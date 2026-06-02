package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"orchid/internal/project"
	"orchid/internal/session"
	"orchid/internal/tmux"
)

func gitRepo(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	for _, a := range [][]string{{"init", "-q"}, {"-c", "user.email=t@t", "-c", "user.name=t", "commit", "--allow-empty", "-q", "-m", "i"}} {
		if out, err := exec.Command("git", append([]string{"-C", d}, a...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", a, err, out)
		}
	}
	return d
}

func newTestServer(t *testing.T) (*httptest.Server, *session.Manager, *project.Store) {
	t.Helper()
	if err := tmux.Available(); err != nil {
		t.Skipf("tmux not available: %v", err)
	}
	if err := exec.Command("git", "--version").Run(); err != nil {
		t.Skipf("git not available")
	}
	mgr := session.NewManager(filepath.Join(t.TempDir(), "sessions.json"))
	t.Cleanup(mgr.KillAll)
	ps := project.New(filepath.Join(t.TempDir(), "projects.json"))
	srv := httptest.NewServer(New(mgr, ps))
	t.Cleanup(srv.Close)
	return srv, mgr, ps
}

func TestAPIProjectsCRUD(t *testing.T) {
	srv, _, _ := newTestServer(t)
	repo := gitRepo(t)
	resp, _ := http.Post(srv.URL+"/api/projects", "application/json", bytes.NewBufferString(`{"name":"p","path":"`+repo+`"}`))
	if resp.StatusCode != 200 {
		t.Fatalf("create project = %d", resp.StatusCode)
	}
	var p project.Project
	json.NewDecoder(resp.Body).Decode(&p)
	resp.Body.Close()
	if p.ID == "" || p.Path != repo {
		t.Fatalf("project = %+v", p)
	}
	resp, _ = http.Post(srv.URL+"/api/projects", "application/json", bytes.NewBufferString(`{"name":"x","path":"`+t.TempDir()+`"}`))
	if resp.StatusCode != 400 {
		t.Fatalf("non-git = %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestAPIWorktreesAndCascade(t *testing.T) {
	srv, mgr, ps := newTestServer(t)
	repo := gitRepo(t)
	p, _ := ps.Add("p", repo)

	resp, _ := http.Get(srv.URL + "/api/projects/" + p.ID + "/worktrees")
	var wts []map[string]any
	json.NewDecoder(resp.Body).Decode(&wts)
	resp.Body.Close()
	if len(wts) != 1 {
		t.Fatalf("worktrees = %v", wts)
	}

	resp, _ = http.Post(srv.URL+"/api/projects/"+p.ID+"/worktrees", "application/json", bytes.NewBufferString(`{"branch":"feat/x"}`))
	if resp.StatusCode != 200 {
		t.Fatalf("add wt = %d", resp.StatusCode)
	}
	var wt map[string]any
	json.NewDecoder(resp.Body).Decode(&wt)
	resp.Body.Close()
	wtPath := wt["path"].(string)

	s, err := mgr.Create(p.ID, wtPath, "shell", session.CreateOpts{})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	_ = s

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/projects/"+p.ID+"/worktrees", bytes.NewBufferString(`{"path":"`+wtPath+`"}`))
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != 204 {
		t.Fatalf("remove wt = %d", resp.StatusCode)
	}
	resp.Body.Close()
	if len(mgr.List()) != 0 {
		t.Fatalf("sessions remain after cascade: %v", mgr.List())
	}
	resp, _ = http.Get(srv.URL + "/api/projects/" + p.ID + "/worktrees")
	wts = nil
	json.NewDecoder(resp.Body).Decode(&wts)
	resp.Body.Close()
	if len(wts) != 1 {
		t.Fatalf("worktree not removed: %v", wts)
	}
}

func TestAPICreateSessionValidation(t *testing.T) {
	srv, _, ps := newTestServer(t)
	repo := gitRepo(t)
	p, _ := ps.Add("p", repo)
	resp, _ := http.Post(srv.URL+"/api/sessions", "application/json", bytes.NewBufferString(`{"project":"nope","worktree":"`+repo+`","agent":"shell"}`))
	if resp.StatusCode != 404 {
		t.Fatalf("unknown project = %d", resp.StatusCode)
	}
	resp.Body.Close()
	resp, _ = http.Post(srv.URL+"/api/sessions", "application/json", bytes.NewBufferString(`{"project":"`+p.ID+`","worktree":"/nope","agent":"shell"}`))
	if resp.StatusCode != 400 {
		t.Fatalf("bad worktree = %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// newServerWithSession mirrors newTestServer, but also registers a project
// pointing at a fresh git repo and creates a live "shell" session in its main
// worktree, returning everything a status test needs.
func newServerWithSession(t *testing.T) (*session.Manager, *httptest.Server, *session.Session) {
	t.Helper()
	ts, mgr, ps := newTestServer(t)
	repo := gitRepo(t)
	p, err := ps.Add("p", repo)
	if err != nil {
		t.Fatalf("add project: %v", err)
	}
	s, err := mgr.Create(p.ID, repo, "shell", session.CreateOpts{})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	return mgr, ts, s
}

func TestStatus(t *testing.T) {
	mgr, ts, s := newServerWithSession(t)
	defer ts.Close()

	post := func(path, body string) int {
		resp, err := http.Post(ts.URL+path, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("POST %s: %v", path, err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}

	if code := post("/api/sessions/"+s.ID+"/status", `{"state":"requires_action"}`); code != http.StatusNoContent {
		t.Fatalf("status code = %d", code)
	}
	if st := statusOf(mgr, s.ID); st != "requires_action" {
		t.Fatalf("after status signal: %q", st)
	}
	// a running signal (user replied) clears it
	if code := post("/api/sessions/"+s.ID+"/status", `{"state":"running"}`); code != http.StatusNoContent {
		t.Fatalf("status code = %d", code)
	}
	if st := statusOf(mgr, s.ID); st != "running" {
		t.Fatalf("running should clear requires_action: %q", st)
	}
	if code := post("/api/sessions/nope/status", `{"state":"running"}`); code != http.StatusNotFound {
		t.Fatalf("unknown id code = %d", code)
	}
}

func statusOf(mgr *session.Manager, id string) string {
	for _, info := range mgr.List() {
		if info.ID == id {
			return info.Status
		}
	}
	return ""
}
