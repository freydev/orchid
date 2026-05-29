package tmux

import (
	"os/exec"
	"strings"
	"testing"
)

func skipIfNoTmux(t *testing.T) {
	t.Helper()
	if err := Available(); err != nil {
		t.Skipf("tmux not available: %v", err)
	}
}

func TestSessionRoundtrip(t *testing.T) {
	skipIfNoTmux(t)
	name := "orchid-test-roundtrip"
	_ = KillSession(name) // clean any leftover
	defer KillSession(name)

	if HasSession(name) {
		t.Fatal("session should not exist yet")
	}
	if err := NewSession(name, "/tmp", []string{"sleep", "60"}); err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if !HasSession(name) {
		t.Fatal("session should exist after NewSession")
	}
	if err := KillSession(name); err != nil {
		t.Fatalf("KillSession: %v", err)
	}
	if HasSession(name) {
		t.Fatal("session should be gone after KillSession")
	}
}

func TestNewSessionSetsWindowSizeLatest(t *testing.T) {
	skipIfNoTmux(t)
	name := "orchid-test-winsize"
	_ = KillSession(name)
	defer KillSession(name)
	if err := NewSession(name, "/tmp", []string{"sleep", "60"}); err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	out, err := exec.Command("tmux", "show-options", "-t", name, "window-size").CombinedOutput()
	if err != nil {
		t.Fatalf("show-options: %v: %s", err, out)
	}
	if !strings.Contains(string(out), "latest") {
		t.Fatalf("want window-size latest, got %q", out)
	}
}
