package client

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func stub(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New(strings.TrimPrefix(srv.URL, "http://"))
}

func TestList(t *testing.T) {
	c := stub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/api/sessions" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.Write([]byte(`[{"id":"a1","agent":"shell","cwd":"/tmp","status":"running","alive":true}]`))
	})
	infos, err := c.List()
	if err != nil || len(infos) != 1 || infos[0].ID != "a1" || infos[0].Agent != "shell" {
		t.Fatalf("List = %+v, %v", infos, err)
	}
}

func TestCreate(t *testing.T) {
	c := stub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method %s", r.Method)
		}
		w.Write([]byte(`{"id":"b2","agent":"claude","cwd":"/x","status":"running","alive":true}`))
	})
	info, err := c.Create("p-1", "/x", "claude")
	if err != nil || info.ID != "b2" || info.Agent != "claude" {
		t.Fatalf("Create = %+v, %v", info, err)
	}
}

func TestCreateUnknownAgent400(t *testing.T) {
	c := stub(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unknown agent \"nope\"", http.StatusBadRequest)
	})
	if _, err := c.Create("p-1", "", "nope"); err == nil {
		t.Fatal("expected error on 400")
	}
}

func TestKill404(t *testing.T) {
	c := stub(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "session \"x\" not found", http.StatusNotFound)
	})
	if err := c.Kill("x"); err == nil {
		t.Fatal("expected error on 404")
	}
}

func TestRestart409(t *testing.T) {
	c := stub(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "already running", http.StatusConflict)
	})
	if _, err := c.Restart("x"); err == nil {
		t.Fatal("expected error on 409")
	}
}

func TestKillOK(t *testing.T) {
	c := stub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("method %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	if err := c.Kill("a1"); err != nil {
		t.Fatalf("Kill: %v", err)
	}
}
