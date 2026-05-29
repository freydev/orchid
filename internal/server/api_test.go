package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"orchid/internal/session"
)

func TestAPICreateListKill(t *testing.T) {
	mgr := session.NewManager(t.TempDir() + "/sessions.json")
	defer mgr.KillAll()
	h := New(mgr)
	srv := httptest.NewServer(h)
	defer srv.Close()

	// create
	resp, err := http.Post(srv.URL+"/api/sessions", "application/json", nil)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create status = %d", resp.StatusCode)
	}
	var created session.Info
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	if created.ID == "" {
		t.Fatal("empty id from create")
	}

	// list
	resp, _ = http.Get(srv.URL + "/api/sessions")
	var list []session.Info
	json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	if len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("list = %+v", list)
	}

	// kill
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/sessions/"+created.ID, nil)
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("kill status = %d", resp.StatusCode)
	}
	resp.Body.Close()

	// list empty
	resp, _ = http.Get(srv.URL + "/api/sessions")
	list = nil
	json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	if len(list) != 0 {
		t.Fatalf("expected empty list, got %+v", list)
	}
}

func TestAPIKillUnknownReturns404(t *testing.T) {
	mgr := session.NewManager(t.TempDir() + "/sessions.json")
	h := New(mgr)
	srv := httptest.NewServer(h)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/sessions/nope", nil)
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	resp.Body.Close()
}
