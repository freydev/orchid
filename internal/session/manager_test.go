package session

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"orchid/internal/store"
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
	m := NewManager(filepath.Join(t.TempDir(), "sessions.json"))
	t.Cleanup(m.KillAll)
	return m
}

func TestManagerCreateGetKill(t *testing.T) {
	skipNoTmux(t)
	m := newTestManager(t)
	s, err := m.Create("p-1", "/tmp", "shell")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer tmux.KillSession(s.Tmux)
	if s.Project != "p-1" || s.Worktree != "/tmp" || s.Cwd != "/tmp" {
		t.Fatalf("binding = %+v", s)
	}
	if _, ok := m.Get(s.ID); !ok {
		t.Fatal("Get failed")
	}
	info := m.List()[0]
	if info.Project != "p-1" || info.Worktree != "/tmp" {
		t.Fatalf("info = %+v", info)
	}
	if err := m.Kill(s.ID); err != nil {
		t.Fatalf("kill: %v", err)
	}
	if len(m.List()) != 0 {
		t.Fatal("expected empty after kill")
	}
}

func TestManagerCreateUnknownAgent(t *testing.T) {
	m := newTestManager(t)
	if _, err := m.Create("p-1", "/tmp", "nope"); err == nil {
		t.Fatal("expected error for unknown agent")
	}
}

func TestManagerKillUnknown(t *testing.T) {
	m := newTestManager(t)
	if err := m.Kill("nope"); err == nil {
		t.Fatal("expected error killing unknown session")
	}
}

func TestKillByWorktree(t *testing.T) {
	skipNoTmux(t)
	m := newTestManager(t)
	a, _ := m.Create("p-1", "/tmp/wtA", "shell")
	b, _ := m.Create("p-1", "/tmp/wtA", "shell")
	c, _ := m.Create("p-1", "/tmp/wtB", "shell")
	defer tmux.KillSession(a.Tmux)
	defer tmux.KillSession(b.Tmux)
	defer tmux.KillSession(c.Tmux)
	n := m.KillByWorktree("/tmp/wtA")
	if n != 2 {
		t.Fatalf("killed %d, want 2", n)
	}
	if _, ok := m.Get(a.ID); ok {
		t.Fatal("a should be gone")
	}
	if _, ok := m.Get(b.ID); ok {
		t.Fatal("b should be gone")
	}
	if _, ok := m.Get(c.ID); !ok {
		t.Fatal("c should remain")
	}
}

func TestSelfExitMarksStopped(t *testing.T) {
	skipNoTmux(t)
	m := newTestManager(t)
	s, err := m.Create("p-1", "/tmp", "shell")
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
			t.Fatalf("not stopped; list=%+v", m.List())
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func TestRestoreDropsLegacy(t *testing.T) {
	skipNoTmux(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "sessions.json")

	// Write two records: one bound (project+worktree set), one legacy (no project/worktree).
	// The bound record has a fake Tmux name that won't exist — Restore will mark it stopped
	// but still retain it. The legacy one must be dropped entirely.
	bound := store.Record{
		ID:       "aabbccdd",
		Agent:    "shell",
		Cwd:      "/tmp",
		Project:  "p-bound",
		Worktree: "/tmp",
		Tmux:     "orchid-aabbccdd-nonexistent",
		Status:   "running",
	}
	legacy := store.Record{
		ID:     "11223344",
		Agent:  "shell",
		Cwd:    "/tmp",
		Tmux:   "orchid-11223344-nonexistent",
		Status: "running",
		// Project and Worktree intentionally empty
	}
	if err := store.Save(path, []store.Record{bound, legacy}); err != nil {
		t.Fatalf("save: %v", err)
	}

	m := NewManager(path)
	t.Cleanup(m.KillAll)
	if err := m.Restore(); err != nil {
		t.Fatalf("restore: %v", err)
	}

	list := m.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 record after restore (legacy dropped), got %d: %+v", len(list), list)
	}
	if list[0].ID != bound.ID {
		t.Fatalf("surviving record ID = %q, want %q", list[0].ID, bound.ID)
	}
	if list[0].Project != "p-bound" {
		t.Fatalf("surviving record Project = %q, want %q", list[0].Project, "p-bound")
	}
	// Tmux session doesn't exist, so it must be stopped (not running).
	if list[0].Status != "stopped" {
		t.Fatalf("surviving record Status = %q, want %q", list[0].Status, "stopped")
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
			s, err := m.Create("p-1", "/tmp", "shell")
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
