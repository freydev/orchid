package session

import (
	"sync"
	"testing"
	"time"
)

func TestManagerCreateGetKill(t *testing.T) {
	m := NewManager()
	s, err := m.Create()
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer m.KillAll()

	got, ok := m.Get(s.ID)
	if !ok || got.ID != s.ID {
		t.Fatalf("Get(%q) = %v, %v", s.ID, got, ok)
	}
	if len(m.List()) != 1 {
		t.Fatalf("List len = %d, want 1", len(m.List()))
	}
	if err := m.Kill(s.ID); err != nil {
		t.Fatalf("kill: %v", err)
	}
	if _, ok := m.Get(s.ID); ok {
		t.Fatal("session still present after Kill")
	}
}

func TestManagerKillUnknown(t *testing.T) {
	m := NewManager()
	if err := m.Kill("nope"); err == nil {
		t.Fatal("expected error killing unknown session")
	}
}

func TestManagerConcurrentCreate(t *testing.T) {
	m := NewManager()
	defer m.KillAll()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := m.Create(); err != nil {
				t.Errorf("create: %v", err)
			}
		}()
	}
	wg.Wait()
	if len(m.List()) != 10 {
		t.Fatalf("List len = %d, want 10", len(m.List()))
	}
}

func TestSelfExitRemovesFromManager(t *testing.T) {
	m := NewManager()
	defer m.KillAll()
	s, err := m.Create()
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := s.Write([]byte("exit\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	deadline := time.After(5 * time.Second)
	for {
		if _, ok := m.Get(s.ID); !ok {
			return // removed — success
		}
		select {
		case <-deadline:
			t.Fatal("session not removed from manager after shell exit")
		case <-time.After(50 * time.Millisecond):
		}
	}
}
