package tmux

import "testing"

func skipIfNoTmux(t *testing.T) {
	t.Helper()
	if err := Available(); err != nil {
		t.Skipf("tmux not available: %v", err)
	}
}

func TestAttachArgv(t *testing.T) {
	got := AttachArgv("orchid-abc")
	want := []string{"tmux", "attach", "-t", "orchid-abc"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
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
