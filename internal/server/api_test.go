package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"orchid/internal/session"
	"orchid/internal/tmux"
)

func newTestServer(t *testing.T) (*httptest.Server, *session.Manager) {
	t.Helper()
	if err := tmux.Available(); err != nil {
		t.Skipf("tmux not available: %v", err)
	}
	mgr := session.NewManager(filepath.Join(t.TempDir(), "sessions.json"))
	t.Cleanup(mgr.KillAll)
	srv := httptest.NewServer(New(mgr))
	t.Cleanup(srv.Close)
	return srv, mgr
}

func TestAPICreateWithAgentBody(t *testing.T) {
	srv, mgr := newTestServer(t)

	body := bytes.NewBufferString(`{"agent":"shell","cwd":"/tmp"}`)
	resp, err := http.Post(srv.URL+"/api/sessions", "application/json", body)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create status = %d", resp.StatusCode)
	}
	var created session.Info
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	if created.ID == "" || created.Agent != "shell" {
		t.Fatalf("created = %+v", created)
	}
	for _, info := range mgr.List() {
		_ = mgr.Kill(info.ID)
	}
}

func TestAPICreateUnknownAgent400(t *testing.T) {
	srv, _ := newTestServer(t)
	body := bytes.NewBufferString(`{"agent":"nope"}`)
	resp, _ := http.Post(srv.URL+"/api/sessions", "application/json", body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestAPIKillUnknownReturns404(t *testing.T) {
	srv, _ := newTestServer(t)
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/sessions/nope", nil)
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestAPIRestartRunningReturns409(t *testing.T) {
	srv, mgr := newTestServer(t)
	s, err := mgr.Create("shell", "/tmp")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer mgr.Kill(s.ID)

	resp, _ := http.Post(srv.URL+"/api/sessions/"+s.ID+"/restart", "application/json", nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("restart running status = %d, want 409", resp.StatusCode)
	}
	resp.Body.Close()
}
