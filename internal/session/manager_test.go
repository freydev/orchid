package session

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"orchid/internal/tmux"
)

func skipNoTmux(t *testing.T) {
	t.Helper()
	if err := tmux.Available(); err != nil {
		t.Skipf("tmux not available: %v", err)
	}
}

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sessions.json")
	m := NewManager(path)
	t.Cleanup(m.KillAll)
	return m
}

func TestManagerCreateGetKill(t *testing.T) {
	skipNoTmux(t)
	m := newTestManager(t)
	s, err := m.Create("shell", "/tmp")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer tmux.KillSession(s.Tmux)

	got, ok := m.Get(s.ID)
	if !ok || got.ID != s.ID {
		t.Fatalf("Get(%q) = %v, %v", s.ID, got, ok)
	}
	if len(m.List()) != 1 {
		t.Fatalf("List len = %d, want 1", len(m.List()))
	}
	if m.List()[0].Agent != "shell" || m.List()[0].Status != "running" {
		t.Fatalf("info = %+v", m.List()[0])
	}
	if err := m.Kill(s.ID); err != nil {
		t.Fatalf("kill: %v", err)
	}
	if len(m.List()) != 0 {
		t.Fatalf("expected empty after kill, got %+v", m.List())
	}
}

func TestManagerCreateUnknownAgent(t *testing.T) {
	m := newTestManager(t)
	if _, err := m.Create("nope", "/tmp"); err == nil {
		t.Fatal("expected error for unknown agent")
	}
}

func TestManagerKillUnknown(t *testing.T) {
	m := newTestManager(t)
	if err := m.Kill("nope"); err == nil {
		t.Fatal("expected error killing unknown session")
	}
}

func TestSelfExitMarksStopped(t *testing.T) {
	skipNoTmux(t)
	m := newTestManager(t)
	s, err := m.Create("shell", "/tmp")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer tmux.KillSession(s.Tmux)

	if _, err := s.Write([]byte("exit\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	deadline := time.After(8 * time.Second)
	for {
		infos := m.List()
		if len(infos) == 1 && infos[0].Status == "stopped" {
			if _, ok := m.Get(s.ID); ok {
				t.Fatal("stopped session should not be in running map")
			}
			return
		}
		select {
		case <-deadline:
			t.Fatalf("session not marked stopped; list=%+v", m.List())
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func TestManagerRestoreReconciles(t *testing.T) {
	skipNoTmux(t)
	m := newTestManager(t)
	alive, _ := m.Create("shell", "/tmp")
	defer tmux.KillSession(alive.Tmux)
	dead, _ := m.Create("shell", "/tmp")
	_ = tmux.KillSession(dead.Tmux)

	m2 := NewManager(m.path)
	t.Cleanup(m2.KillAll)
	if err := m2.Restore(); err != nil {
		t.Fatalf("restore: %v", err)
	}

	var aliveInfo, deadInfo *Info
	infos := m2.List()
	for i := range infos {
		switch infos[i].ID {
		case alive.ID:
			aliveInfo = &infos[i]
		case dead.ID:
			deadInfo = &infos[i]
		}
	}
	if aliveInfo == nil || aliveInfo.Status != "running" {
		t.Fatalf("alive session not running after restore: %+v", aliveInfo)
	}
	if deadInfo == nil || deadInfo.Status != "stopped" {
		t.Fatalf("dead session not stopped after restore: %+v", deadInfo)
	}
}

func TestManagerRestart(t *testing.T) {
	skipNoTmux(t)
	m := newTestManager(t)
	s, _ := m.Create("shell", "/tmp")
	tmuxName := s.Tmux
	defer tmux.KillSession(tmuxName)

	_ = tmux.KillSession(tmuxName)
	deadline := time.After(8 * time.Second)
	for {
		if len(m.List()) == 1 && m.List()[0].Status == "stopped" {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("did not reach stopped; list=%+v", m.List())
		case <-time.After(100 * time.Millisecond):
		}
	}

	rs, err := m.Restart(s.ID)
	if err != nil {
		t.Fatalf("restart: %v", err)
	}
	defer tmux.KillSession(rs.Tmux)
	if _, ok := m.Get(s.ID); !ok {
		t.Fatal("restarted session not in running map")
	}
	if m.List()[0].Status != "running" {
		t.Fatalf("status after restart = %q", m.List()[0].Status)
	}

	if _, err := m.Restart(s.ID); err == nil {
		t.Fatal("expected error restarting a running session")
	}
}

func TestManagerConcurrentCreate(t *testing.T) {
	skipNoTmux(t)
	m := newTestManager(t)
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := m.Create("shell", "/tmp")
			if err != nil {
				t.Errorf("create: %v", err)
				return
			}
			tmux.KillSession(s.Tmux)
		}()
	}
	wg.Wait()
	if len(m.List()) != 6 {
		t.Fatalf("List len = %d, want 6", len(m.List()))
	}
}
