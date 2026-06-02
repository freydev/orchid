package session

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"orchid/internal/store"
	"orchid/internal/tmux"
)

func TestSyncTitles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".claude", "projects", "p")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "asid-1.jsonl"),
		[]byte(`{"type":"ai-title","aiTitle":"My Task","sessionId":"asid-1"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewManager(filepath.Join(t.TempDir(), "sessions.json"))
	m.records["s1"] = store.Record{ID: "s1", Agent: "claude", AgentSessionID: "asid-1"}                // empty name -> fill
	m.records["s2"] = store.Record{ID: "s2", Agent: "claude", AgentSessionID: "asid-1", Name: "mine"}  // user-named -> keep
	m.records["s3"] = store.Record{ID: "s3", Agent: "shell", AgentSessionID: "asid-1"}                 // non-claude -> skip
	m.SyncTitles()
	if m.records["s1"].Name != "My Task" {
		t.Fatalf("s1 name = %q, want My Task", m.records["s1"].Name)
	}
	if m.records["s2"].Name != "mine" {
		t.Fatalf("s2 user name overwritten: %q", m.records["s2"].Name)
	}
	if m.records["s3"].Name != "" {
		t.Fatalf("s3 (shell) should not sync: %q", m.records["s3"].Name)
	}
}

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
	s, err := m.Create("p-1", "/tmp", "shell", CreateOpts{})
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

func TestManagerNameAndRename(t *testing.T) {
	skipNoTmux(t)
	m := newTestManager(t)
	s, err := m.Create("p-1", "/tmp", "shell", CreateOpts{Name: "  build server  "})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer tmux.KillSession(s.Tmux)
	if got := m.List()[0].Name; got != "build server" {
		t.Fatalf("name = %q, want trimmed %q", got, "build server")
	}
	if err := m.Rename(s.ID, "  api  "); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if got := m.List()[0].Name; got != "api" {
		t.Fatalf("renamed name = %q", got)
	}
	if err := m.Rename(s.ID, ""); err != nil {
		t.Fatalf("rename clear: %v", err)
	}
	if got := m.List()[0].Name; got != "" {
		t.Fatalf("cleared name = %q, want empty", got)
	}
	if err := m.Rename("nope", "x"); err == nil {
		t.Fatal("expected error renaming unknown session")
	}
}

func TestPruneStopped(t *testing.T) {
	skipNoTmux(t)
	m := newTestManager(t)
	a, _ := m.Create("p-1", "/tmp", "shell", CreateOpts{})
	b, _ := m.Create("p-1", "/tmp", "shell", CreateOpts{})
	defer tmux.KillSession(a.Tmux)
	defer tmux.KillSession(b.Tmux)
	// Mark b stopped without touching tmux/live map directly: kill its tmux,
	// then flip the record via onExit.
	m.onExit(b.ID)
	if n := m.PruneStopped(); n != 1 {
		t.Fatalf("prune removed %d, want 1", n)
	}
	ids := map[string]bool{}
	for _, info := range m.List() {
		ids[info.ID] = true
	}
	if ids[b.ID] {
		t.Fatal("stopped session b survived prune")
	}
	if !ids[a.ID] {
		t.Fatal("running session a was pruned")
	}
	if n := m.PruneStopped(); n != 0 {
		t.Fatalf("second prune removed %d, want 0", n)
	}
}

func TestOrphanSessions(t *testing.T) {
	names := []string{"orchid-aaa", "orchid-bbb", "other-ccc", "orchid-ddd"}
	known := map[string]bool{"orchid-bbb": true}
	got := orphanSessions(names, known, "orchid-")
	want := []string{"orchid-aaa", "orchid-ddd"}
	if len(got) != len(want) {
		t.Fatalf("orphans = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("orphans = %v, want %v", got, want)
		}
	}
	// non-orchid and known names are never orphans
	for _, n := range got {
		if n == "other-ccc" || n == "orchid-bbb" {
			t.Fatalf("unexpected orphan %q", n)
		}
	}
}

func TestManagerCreateUnknownAgent(t *testing.T) {
	m := newTestManager(t)
	if _, err := m.Create("p-1", "/tmp", "nope", CreateOpts{}); err == nil {
		t.Fatal("expected error for unknown agent")
	}
}

func TestStopRetainsRecord(t *testing.T) {
	skipNoTmux(t)
	m := newTestManager(t)
	s, err := m.Create("p-1", "/tmp", "shell", CreateOpts{})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	id := s.ID
	if err := m.Stop(id); err != nil {
		t.Fatalf("stop: %v", err)
	}
	// record retained, marked stopped, not live, tmux gone
	infos := m.List()
	if len(infos) != 1 || infos[0].ID != id {
		t.Fatalf("record should be retained: %+v", infos)
	}
	if infos[0].Status != "stopped" || infos[0].Alive {
		t.Fatalf("status = %q alive=%v, want stopped/false", infos[0].Status, infos[0].Alive)
	}
	if _, ok := m.Get(id); ok {
		t.Fatal("stopped session should not be live")
	}
	if tmux.HasSession(s.Tmux) {
		t.Fatal("tmux session should be killed")
	}
	// restartable
	if _, err := m.Restart(id); err != nil {
		t.Fatalf("restart after stop: %v", err)
	}
	defer tmux.KillSession(s.Tmux)
	if err := m.Stop("nope"); err == nil {
		t.Fatal("expected error stopping unknown session")
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
	a, _ := m.Create("p-1", "/tmp/wtA", "shell", CreateOpts{})
	b, _ := m.Create("p-1", "/tmp/wtA", "shell", CreateOpts{})
	c, _ := m.Create("p-1", "/tmp/wtB", "shell", CreateOpts{})
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
	s, err := m.Create("p-1", "/tmp", "shell", CreateOpts{})
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
			s, err := m.Create("p-1", "/tmp", "shell", CreateOpts{})
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

func TestSignalStatusResolution(t *testing.T) {
	skipNoTmux(t)
	m := newTestManager(t)
	s, err := m.Create("p-1", "/tmp", "shell", CreateOpts{})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer tmux.KillSession(s.Tmux)
	id := s.ID

	status := func() string {
		for _, info := range m.List() {
			if info.ID == id {
				return info.Status
			}
		}
		t.Fatalf("session %s missing from List", id)
		return ""
	}

	m.Signal(id, "running")
	if st := status(); st != "running" {
		t.Fatalf("after running signal: %q", st)
	}
	// requires_action comes only from an explicit set_status; it is sticky.
	m.Signal(id, "requires_action")
	if st := status(); st != "requires_action" {
		t.Fatalf("requires_action: %q", st)
	}
	// a running signal (the user replied) clears it.
	m.Signal(id, "running")
	if st := status(); st != "running" {
		t.Fatalf("running should clear requires_action: %q", st)
	}
	// done = finished a turn with no explicit question -> idle (not requires_action).
	m.Signal(id, "done")
	if st := status(); st != "idle" {
		t.Fatalf("done with no question should be idle: %q", st)
	}
	m.Signal(id, "idle")
	if st := status(); st != "idle" {
		t.Fatalf("idle signal: %q", st)
	}
}

func TestSessionActive(t *testing.T) {
	skipNoTmux(t)
	m := newTestManager(t)
	s, err := m.Create("p-1", "/tmp", "shell", CreateOpts{})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer tmux.KillSession(s.Tmux)
	// a fresh session emits startup output, so it is active within a generous window
	if !s.Active(5 * time.Second) {
		t.Fatal("fresh session should be active")
	}
	// with a tiny window it reads inactive once output goes quiet
	time.Sleep(300 * time.Millisecond)
	if s.Active(1 * time.Millisecond) {
		t.Fatal("session should be inactive beyond a 1ms window")
	}
}

func TestReorderSessions(t *testing.T) {
	m := NewManager(filepath.Join(t.TempDir(), "sessions.json"))
	m.records["s1"] = store.Record{ID: "s1", Agent: "shell", Worktree: "/w"}
	m.records["s2"] = store.Record{ID: "s2", Agent: "shell", Worktree: "/w"}
	m.records["s3"] = store.Record{ID: "s3", Agent: "shell", Worktree: "/w"}
	m.Reorder([]string{"s3", "s1", "s2"})
	var ids []string
	for _, i := range m.List() {
		ids = append(ids, i.ID)
	}
	want := []string{"s3", "s1", "s2"}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("order = %v, want %v", ids, want)
		}
	}
}

func TestKillByProject(t *testing.T) {
	skipNoTmux(t)
	m := newTestManager(t)
	a, _ := m.Create("p-1", "/tmp/wtA", "shell", CreateOpts{})
	b, _ := m.Create("p-1", "/tmp/wtB", "shell", CreateOpts{})
	c, _ := m.Create("p-2", "/tmp/wtC", "shell", CreateOpts{})
	defer tmux.KillSession(a.Tmux)
	defer tmux.KillSession(b.Tmux)
	defer tmux.KillSession(c.Tmux)
	if n := m.KillByProject("p-1"); n != 2 {
		t.Fatalf("killed %d, want 2", n)
	}
	if _, ok := m.Get(a.ID); ok {
		t.Fatal("a should be gone")
	}
	if _, ok := m.Get(c.ID); !ok {
		t.Fatal("c (other project) should remain")
	}
	if len(m.List()) != 1 {
		t.Fatalf("List len = %d, want 1", len(m.List()))
	}
}

func TestSignalPersistsRequiresAction(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	m := NewManager(path)
	m.records["s1"] = store.Record{ID: "s1", Project: "p", Worktree: "/w", Agent: "claude", Tmux: "orchid-test-nope"}
	m.Signal("s1", "requires_action")
	if m.records["s1"].Signal != "requires_action" {
		t.Fatalf("record signal = %q, want requires_action", m.records["s1"].Signal)
	}
	recs, _ := store.Load(path)
	if len(recs) != 1 || recs[0].Signal != "requires_action" {
		t.Fatalf("not persisted to disk: %+v", recs)
	}
	// a new manager restoring this record seeds the in-memory signal
	m2 := NewManager(path)
	if err := m2.Restore(); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if sg := m2.signals["s1"]; sg == nil || sg.state != "requires_action" {
		t.Fatalf("signal not seeded on restore: %+v", sg)
	}
	// clearing
	m.Signal("s1", "running")
	if m.records["s1"].Signal != "" {
		t.Fatalf("signal not cleared, got %q", m.records["s1"].Signal)
	}
}
