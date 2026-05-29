package session

import (
	"bytes"
	"testing"
	"time"
)

func TestSessionEchoesToRing(t *testing.T) {
	s, err := newSession("test1")
	if err != nil {
		t.Fatalf("newSession: %v", err)
	}
	defer s.Close()

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
	s, err := newSession("test2")
	if err != nil {
		t.Fatalf("newSession: %v", err)
	}
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
