package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"orchid/internal/session"
)

func TestRouteKnownCommands(t *testing.T) {
	for _, name := range []string{"serve", "start", "stop", "status", "open", "ls", "create", "rm", "restart"} {
		h, err := route([]string{name})
		if err != nil || h == nil {
			t.Fatalf("route(%q): got nil=%v err=%v; want handler", name, h == nil, err)
		}
	}
}

func TestRouteNoArgsIsServe(t *testing.T) {
	h, err := route(nil)
	if err != nil || h == nil {
		t.Fatalf("route(nil): got nil=%v err=%v; want serve handler", h == nil, err)
	}
}

func TestRouteFlagsAreServe(t *testing.T) {
	h, err := route([]string{"-port", "9000"})
	if err != nil || h == nil {
		t.Fatalf("route(-port): got nil=%v err=%v; want serve handler", h == nil, err)
	}
}

func TestRouteUnknown(t *testing.T) {
	if _, err := route([]string{"bogus"}); err == nil {
		t.Fatal("unknown command should error")
	}
}

func TestSignalCmdPosts(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	t.Setenv("ORCHID_SESSION", "s-1")
	t.Setenv("ORCHID_URL", srv.URL)
	if code := signalCmd([]string{"requires_action"}); code != 0 {
		t.Fatalf("signalCmd code = %d", code)
	}
	if path != "/api/sessions/s-1/status" {
		t.Fatalf("path = %q", path)
	}
}

func TestSignalCmdNoEnvIsNoop(t *testing.T) {
	t.Setenv("ORCHID_SESSION", "")
	t.Setenv("ORCHID_URL", "")
	if code := signalCmd([]string{"running"}); code != 0 {
		t.Fatalf("expected exit 0 with no env, got %d", code)
	}
}

func TestAnyAgentRunning(t *testing.T) {
	cases := []struct {
		name string
		list []session.Info
		want bool
	}{
		{"empty", nil, false},
		{"agent running", []session.Info{{Agent: "claude", Status: "running"}}, true},
		{"agent idle", []session.Info{{Agent: "codex", Status: "idle"}, {Agent: "claude", Status: "requires_action"}}, false},
		{"only shell running", []session.Info{{Agent: "shell", Status: "running"}}, false},
	}
	for _, c := range cases {
		if got := anyAgentRunning(c.list); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}
