package session

import (
	"bytes"
	"testing"
	"time"

	"orchid/internal/agent"
	"orchid/internal/store"
	"orchid/internal/tmux"
)

// newTestSession creates a real tmux-backed shell session for tests, skipping
// if tmux is unavailable. It registers cleanup that kills the tmux session.
func newTestSession(t *testing.T, id string) *Session {
	t.Helper()
	if err := tmux.Available(); err != nil {
		t.Skipf("tmux not available: %v", err)
	}
	name := "orchid-" + id
	_ = tmux.KillSession(name)
	rec := store.Record{ID: id, Agent: "shell", Cwd: "/tmp", Tmux: name, Status: "running"}
	sh, _ := agent.Get("shell")
	s, err := createSession(rec, sh.NewCommand(rec.Cwd, rec.AgentSessionID, agent.Options{}), nil, nil)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}
	s.Start()
	t.Cleanup(func() { s.Close(); tmux.KillSession(name) })
	return s
}

func TestSessionEchoesToRing(t *testing.T) {
	s := newTestSession(t, "test1")

	if _, err := s.Write([]byte("echo orchid_marker\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	deadline := time.After(5 * time.Second)
	for {
		if bytes.Contains(s.ring.Snapshot(), []byte("orchid_marker")) {
			return // success
		}
		select {
		case <-deadline:
			t.Fatalf("marker not found in ring; got %q", s.ring.Snapshot())
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func TestSessionAliveFlips(t *testing.T) {
	s := newTestSession(t, "test2")

	if !s.Alive() {
		t.Fatal("expected alive immediately after start")
	}
	s.Close()
	deadline := time.After(5 * time.Second)
	for s.Alive() {
		select {
		case <-deadline:
			t.Fatal("session still alive after Close")
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func TestSessionAttachReceivesOutput(t *testing.T) {
	s := newTestSession(t, "test3")

	a := s.Attach()
	defer s.Detach(a)

	if _, err := s.Write([]byte("echo attach_marker\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	var acc []byte
	deadline := time.After(5 * time.Second)
	for {
		select {
		case b := <-a.ch:
			acc = append(acc, b...)
			if bytes.Contains(acc, []byte("attach_marker")) {
				return
			}
		case <-deadline:
			t.Fatalf("marker not received; got %q", acc)
		}
	}
}

func TestSessionMultipleAttachments(t *testing.T) {
	s := newTestSession(t, "test4")

	a1 := s.Attach()
	a2 := s.Attach() // both stay live (multiple windows)

	// a1 must NOT be kicked when a2 attaches.
	select {
	case <-a1.done:
		t.Fatal("first attachment was kicked; multiple should coexist")
	case <-time.After(200 * time.Millisecond):
	}

	// detaching a2 leaves a1 live.
	s.Detach(a2)
	select {
	case <-a2.done: // expected
	case <-time.After(2 * time.Second):
		t.Fatal("detached attachment was not closed")
	}
	select {
	case <-a1.done:
		t.Fatal("detaching one closed the other")
	case <-time.After(200 * time.Millisecond):
	}
	s.Detach(a1)
}
