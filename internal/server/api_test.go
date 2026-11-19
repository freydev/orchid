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

	"orchid/internal/config"
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

func TestAPISettingsIdleTimeout(t *testing.T) {
	srv, _, _ := newTestServer(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	get := func() settingsOut {
		var out settingsOut
		resp, err := http.Get(srv.URL + "/api/settings")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		json.NewDecoder(resp.Body).Decode(&out)
		return out
	}
	put := func(body string) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/settings", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("PUT /api/settings = %d", resp.StatusCode)
		}
	}

	// an unset config reports the default rather than a bare 0
	if got := get().IdleTimeoutMin; got != config.DefaultIdleTimeoutMin {
		t.Fatalf("default idle_timeout_min = %d, want %d", got, config.DefaultIdleTimeoutMin)
	}
	put(`{"idle_timeout_min":45}`)
	if got := get().IdleTimeoutMin; got != 45 {
		t.Fatalf("after PUT 45, idle_timeout_min = %d", got)
	}
	// a client that omits the field must not clobber the stored value
	put(`{"accent":"blue"}`)
	if got := get().IdleTimeoutMin; got != 45 {
		t.Fatalf("omitted field clobbered the value: %d", got)
	}
	// negative means "off" and must survive the round trip verbatim
	put(`{"idle_timeout_min":-1}`)
	if got := get().IdleTimeoutMin; got != -1 {
		t.Fatalf("after PUT -1, idle_timeout_min = %d", got)
	}
}

func TestAPIGroups(t *testing.T) {
	srv, mgr, ps := newTestServer(t)
	repo := gitRepo(t)
	p, _ := ps.Add("p", repo)
	base := srv.URL + "/api/projects/" + p.ID

	do := func(method, url, body string) *http.Response {
		t.Helper()
		req, _ := http.NewRequest(method, url, bytes.NewBufferString(body))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	worktrees := func() []map[string]any {
		resp := do(http.MethodGet, base+"/worktrees", "")
		defer resp.Body.Close()
		var out []map[string]any
		json.NewDecoder(resp.Body).Decode(&out)
		return out
	}
	root := worktrees()[0]["path"].(string)

	if resp := do(http.MethodPost, base+"/groups", `{"worktree":"/nope","name":"x"}`); resp.StatusCode != 400 {
		t.Fatalf("bad worktree = %d", resp.StatusCode)
	}
	if resp := do(http.MethodPost, base+"/groups", `{"worktree":"`+root+`","name":" "}`); resp.StatusCode != 400 {
		t.Fatalf("empty name = %d", resp.StatusCode)
	}
	resp := do(http.MethodPost, base+"/groups", `{"worktree":"`+root+`","name":"Review"}`)
	var g project.Group
	json.NewDecoder(resp.Body).Decode(&g)
	resp.Body.Close()
	if resp.StatusCode != 200 || g.ID == "" {
		t.Fatalf("create group = %d %+v", resp.StatusCode, g)
	}
	if gs := worktrees()[0]["groups"].([]any); len(gs) != 1 {
		t.Fatalf("worktree groups = %v", gs)
	}

	// session created in the group; another moved in via PATCH; a third ungrouped
	resp = do(http.MethodPost, srv.URL+"/api/sessions", `{"project":"`+p.ID+`","worktree":"`+root+`","agent":"shell","group":"`+g.ID+`"}`)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("create in group = %d", resp.StatusCode)
	}
	if resp := do(http.MethodPost, srv.URL+"/api/sessions", `{"project":"`+p.ID+`","worktree":"`+root+`","agent":"shell","group":"g-nope"}`); resp.StatusCode != 400 {
		t.Fatalf("unknown group = %d", resp.StatusCode)
	}
	s2, _ := mgr.Create(p.ID, root, "shell", session.CreateOpts{})
	s3, _ := mgr.Create(p.ID, root, "shell", session.CreateOpts{})
	if resp := do(http.MethodPatch, srv.URL+"/api/sessions/"+s2.ID, `{"group":"`+g.ID+`"}`); resp.StatusCode != 200 {
		t.Fatalf("move to group = %d", resp.StatusCode)
	}
	if resp := do(http.MethodPatch, srv.URL+"/api/sessions/"+s3.ID, `{"group":"g-nope"}`); resp.StatusCode != 400 {
		t.Fatalf("move to unknown group = %d", resp.StatusCode)
	}

	// clear kills only the group's sessions; the group survives
	resp = do(http.MethodPost, base+"/groups/"+g.ID+"/clear", "")
	var cleared map[string]int
	json.NewDecoder(resp.Body).Decode(&cleared)
	resp.Body.Close()
	if cleared["killed"] != 2 {
		t.Fatalf("cleared = %v", cleared)
	}
	if l := mgr.List(); len(l) != 1 || l[0].ID != s3.ID {
		t.Fatalf("sessions after clear = %+v", l)
	}
	if gs := worktrees()[0]["groups"].([]any); len(gs) != 1 {
		t.Fatalf("group dropped by clear: %v", gs)
	}

	// rename, then delete: sessions survive, ungrouped
	if resp := do(http.MethodPatch, base+"/groups/"+g.ID, `{"name":"Reviews"}`); resp.StatusCode != 204 {
		t.Fatalf("rename = %d", resp.StatusCode)
	}
	do(http.MethodPatch, srv.URL+"/api/sessions/"+s3.ID, `{"group":"`+g.ID+`"}`).Body.Close()
	if resp := do(http.MethodDelete, base+"/groups/"+g.ID, ""); resp.StatusCode != 204 {
		t.Fatalf("delete = %d", resp.StatusCode)
	}
	if l := mgr.List(); len(l) != 1 || l[0].Group != "" {
		t.Fatalf("sessions after delete = %+v", l)
	}
	if gs := worktrees()[0]["groups"].([]any); len(gs) != 0 {
		t.Fatalf("group not deleted: %v", gs)
	}
}

func TestAPIPruneEmptyWorktrees(t *testing.T) {
	srv, mgr, ps := newTestServer(t)
	repo := gitRepo(t)
	p, _ := ps.Add("p", repo)
	base := srv.URL + "/api/projects/" + p.ID + "/worktrees"

	add := func(branch string) string {
		resp, _ := http.Post(base, "application/json", bytes.NewBufferString(`{"branch":"`+branch+`"}`))
		if resp.StatusCode != 200 {
			t.Fatalf("add wt %s = %d", branch, resp.StatusCode)
		}
		var wt map[string]any
		json.NewDecoder(resp.Body).Decode(&wt)
		resp.Body.Close()
		return wt["path"].(string)
	}
	empty, withSess, dirty := add("feat/empty"), add("feat/sess"), add("feat/dirty")
	if _, err := mgr.Create(p.ID, withSess, "shell", session.CreateOpts{}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if out, err := exec.Command("touch", filepath.Join(dirty, "wip.txt")).CombinedOutput(); err != nil {
		t.Fatalf("touch: %v %s", err, out)
	}

	resp, _ := http.Get(base + "/empty")
	var got []map[string]any
	json.NewDecoder(resp.Body).Decode(&got)
	resp.Body.Close()
	if len(got) != 1 || got[0]["path"] != empty || got[0]["branch"] != "feat/empty" {
		t.Fatalf("empty = %v, want only %s", got, empty)
	}

	// prune re-verifies every path: only the empty one goes
	body, _ := json.Marshal(map[string]any{"paths": []string{empty, withSess, dirty, repo}})
	resp, _ = http.Post(base+"/prune", "application/json", bytes.NewReader(body))
	var res struct {
		Removed []string `json:"removed"`
	}
	json.NewDecoder(resp.Body).Decode(&res)
	resp.Body.Close()
	if len(res.Removed) != 1 || res.Removed[0] != empty {
		t.Fatalf("removed = %v, want [%s]", res.Removed, empty)
	}
	if out, _ := exec.Command("git", "-C", repo, "branch", "--list", "feat/empty").Output(); strings.TrimSpace(string(out)) != "" {
		t.Fatalf("branch feat/empty not deleted")
	}
	resp, _ = http.Get(base)
	var wts []map[string]any
	json.NewDecoder(resp.Body).Decode(&wts)
	resp.Body.Close()
	if len(wts) != 3 {
		t.Fatalf("worktrees after prune = %d, want 3 (root, sess, dirty)", len(wts))
	}
}
