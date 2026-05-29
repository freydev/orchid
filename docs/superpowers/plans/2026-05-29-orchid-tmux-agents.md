# Orchid tmux-backed Agent Sessions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn orchid sessions into tmux-backed agent sessions — an `Agent` wrapper (claude/codex/shell), each running in a detached tmux session the PTY attaches to, with session records persisted to `~/.config/orchid/sessions.json` and resumable after restart.

**Architecture:** New packages `internal/agent` (interface + registry), `internal/tmux` (CLI wrapper), `internal/store` (JSON persistence). `internal/session` is reworked so the per-session PTY runs `tmux attach -t orchid-<id>` instead of a bare shell; tmux owns the agent process and is the source of truth for liveness. The `Manager` gains persistence and a `Restore`/`Restart` lifecycle: alive tmux → running, dead tmux → retained as `stopped` and resumable.

**Tech Stack:** Go 1.26, `github.com/creack/pty`, `github.com/coder/websocket`, `github.com/google/uuid`, system `tmux` 3.x, vanilla JS frontend.

**Prereqs (verified):** `tmux` 3.6b installed. claude CLI supports `--session-id <uuid>` and `--resume <uuid>`. codex CLI cannot bind a caller-supplied id at start, so codex resume uses `codex resume --last` (documented limitation in the spec). cwd is set via tmux `-c` for all agents.

---

## File Structure

```
internal/agent/agent.go      # Agent interface, registry (Get/Names), ResolveCwd
internal/agent/claude.go     # claudeAgent
internal/agent/codex.go      # codexAgent
internal/agent/shell.go      # shellAgent
internal/agent/agent_test.go
internal/tmux/tmux.go        # Available/NewSession/HasSession/KillSession/AttachArgv
internal/tmux/tmux_test.go
internal/store/store.go      # Record, DefaultPath, Load, Save (atomic)
internal/store/store_test.go
internal/session/session.go  # MODIFIED: tmux-attach PTY, createSession/attachSession
internal/session/manager.go  # MODIFIED: store, records map, Restore, Restart, Create(agent,cwd)
internal/server/api.go       # MODIFIED: POST body {agent,cwd}, restart route
internal/server/server.go    # MODIFIED: register restart route
internal/server/ws.go        # (unchanged)
web/index.html               # MODIFIED: agent select + cwd input
web/app.js                   # MODIFIED: create body, status, restart button
cmd/orchid/main.go           # MODIFIED: tmux check, NewManager(path), Restore at boot
```

---

## Task 1: Add uuid dependency

**Files:** Modify `go.mod`, `go.sum`.

- [ ] **Step 1: Add dependency**

Run:
```bash
cd /Users/andrey/Work/orchid_3
go get github.com/google/uuid@latest
```
Expected: `go.mod` gains `github.com/google/uuid`.

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add google/uuid dependency"
```

---

## Task 2: Agent interface + registry + concrete agents

**Files:**
- Create: `internal/agent/agent.go`, `internal/agent/claude.go`, `internal/agent/codex.go`, `internal/agent/shell.go`
- Test: `internal/agent/agent_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/agent/agent_test.go`:
```go
package agent

import (
	"reflect"
	"testing"
)

func TestClaudeCommands(t *testing.T) {
	a, ok := Get("claude")
	if !ok {
		t.Fatal("claude not registered")
	}
	if a.Name() != "claude" {
		t.Fatalf("name = %q", a.Name())
	}
	got := a.NewCommand("/tmp", "sid-123")
	want := []string{"claude", "--session-id", "sid-123"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NewCommand = %v, want %v", got, want)
	}
	got = a.ResumeCommand("/tmp", "sid-123")
	want = []string{"claude", "--resume", "sid-123"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ResumeCommand = %v, want %v", got, want)
	}
}

func TestCodexCommands(t *testing.T) {
	a, ok := Get("codex")
	if !ok {
		t.Fatal("codex not registered")
	}
	if got := a.NewCommand("/tmp", "ignored"); !reflect.DeepEqual(got, []string{"codex"}) {
		t.Fatalf("NewCommand = %v", got)
	}
	if got := a.ResumeCommand("/tmp", "ignored"); !reflect.DeepEqual(got, []string{"codex", "resume", "--last"}) {
		t.Fatalf("ResumeCommand = %v", got)
	}
}

func TestShellCommands(t *testing.T) {
	t.Setenv("SHELL", "/bin/zsh")
	a, ok := Get("shell")
	if !ok {
		t.Fatal("shell not registered")
	}
	if got := a.NewCommand("/tmp", ""); !reflect.DeepEqual(got, []string{"/bin/zsh"}) {
		t.Fatalf("NewCommand = %v", got)
	}
	if got := a.ResumeCommand("/tmp", ""); got != nil {
		t.Fatalf("ResumeCommand = %v, want nil", got)
	}
}

func TestUnknownAgent(t *testing.T) {
	if _, ok := Get("nope"); ok {
		t.Fatal("expected unknown agent to be absent")
	}
}

func TestNamesSorted(t *testing.T) {
	names := Names()
	want := []string{"claude", "codex", "shell"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("Names = %v, want %v", names, want)
	}
}

func TestResolveCwd(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	if got := ResolveCwd(""); got != "/home/tester" {
		t.Fatalf("ResolveCwd(\"\") = %q, want /home/tester", got)
	}
	if got := ResolveCwd("/explicit"); got != "/explicit" {
		t.Fatalf("ResolveCwd = %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/agent/ -v`
Expected: FAIL — `undefined: Get`.

- [ ] **Step 3: Write the interface + registry**

Create `internal/agent/agent.go`:
```go
package agent

import (
	"os"
	"sort"
)

// Agent describes how to launch a coding agent inside a tmux session.
type Agent interface {
	Name() string
	// NewCommand returns argv to start a fresh agent session bound to the
	// caller-supplied agent session id (so it can be resumed later). Agents
	// without a session concept ignore the id. cwd is informational; the
	// working directory is actually set by tmux's -c flag.
	NewCommand(cwd, agentSessionID string) []string
	// ResumeCommand returns argv to resume a prior session, or nil if the agent
	// has no resume capability (caller then starts fresh).
	ResumeCommand(cwd, agentSessionID string) []string
}

var registry = map[string]Agent{
	"claude": claudeAgent{},
	"codex":  codexAgent{},
	"shell":  shellAgent{},
}

// Get returns the registered agent by name.
func Get(name string) (Agent, bool) {
	a, ok := registry[name]
	return a, ok
}

// Names returns registered agent names, sorted.
func Names() []string {
	out := make([]string, 0, len(registry))
	for n := range registry {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// ResolveCwd returns the working directory for a session: the given cwd, or the
// server default ($HOME) when blank. Role-based defaults can replace this later.
func ResolveCwd(cwd string) string {
	if cwd != "" {
		return cwd
	}
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return "/"
}
```

Create `internal/agent/claude.go`:
```go
package agent

type claudeAgent struct{}

func (claudeAgent) Name() string { return "claude" }

func (claudeAgent) NewCommand(cwd, agentSessionID string) []string {
	return []string{"claude", "--session-id", agentSessionID}
}

func (claudeAgent) ResumeCommand(cwd, agentSessionID string) []string {
	return []string{"claude", "--resume", agentSessionID}
}
```

Create `internal/agent/codex.go`:
```go
package agent

// codexAgent: codex cannot bind a caller-supplied session id at start (it
// generates its own), so the id is ignored and resume uses codex's most-recent
// session. See the milestone-2 spec for this documented limitation.
type codexAgent struct{}

func (codexAgent) Name() string { return "codex" }

func (codexAgent) NewCommand(cwd, agentSessionID string) []string {
	return []string{"codex"}
}

func (codexAgent) ResumeCommand(cwd, agentSessionID string) []string {
	return []string{"codex", "resume", "--last"}
}
```

Create `internal/agent/shell.go`:
```go
package agent

import "os"

type shellAgent struct{}

func (shellAgent) Name() string { return "shell" }

func (shellAgent) NewCommand(cwd, agentSessionID string) []string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	return []string{shell}
}

// ResumeCommand returns nil: a shell has no session to resume.
func (shellAgent) ResumeCommand(cwd, agentSessionID string) []string {
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/agent/ -v`
Expected: PASS (all 6 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/agent/
git commit -m "feat: add agent interface with claude/codex/shell"
```

---

## Task 3: tmux CLI wrapper

**Files:**
- Create: `internal/tmux/tmux.go`
- Test: `internal/tmux/tmux_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/tmux/tmux_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tmux/ -v`
Expected: FAIL — `undefined: Available`.

- [ ] **Step 3: Write the implementation**

Create `internal/tmux/tmux.go`:
```go
// Package tmux is a thin wrapper over the system tmux CLI.
package tmux

import (
	"fmt"
	"os/exec"
)

// Available reports whether the tmux binary is usable.
func Available() error {
	if _, err := exec.LookPath("tmux"); err != nil {
		return fmt.Errorf("tmux not found in PATH: %w", err)
	}
	if err := exec.Command("tmux", "-V").Run(); err != nil {
		return fmt.Errorf("tmux -V failed: %w", err)
	}
	return nil
}

// NewSession creates a detached tmux session named name, with working directory
// cwd, running argv directly (no shell interpolation).
func NewSession(name, cwd string, argv []string) error {
	args := []string{"new-session", "-d", "-s", name, "-c", cwd, "--"}
	args = append(args, argv...)
	out, err := exec.Command("tmux", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("tmux new-session: %w: %s", err, out)
	}
	return nil
}

// HasSession reports whether a tmux session with the given name exists.
func HasSession(name string) bool {
	return exec.Command("tmux", "has-session", "-t", name).Run() == nil
}

// KillSession terminates the named tmux session.
func KillSession(name string) error {
	out, err := exec.Command("tmux", "kill-session", "-t", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("tmux kill-session: %w: %s", err, out)
	}
	return nil
}

// AttachArgv returns the argv that attaches a client to the named session.
func AttachArgv(name string) []string {
	return []string{"tmux", "attach", "-t", name}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tmux/ -v`
Expected: PASS (TestAttachArgv always; TestSessionRoundtrip passes with tmux present, else SKIP).

- [ ] **Step 5: Commit**

```bash
git add internal/tmux/
git commit -m "feat: add tmux CLI wrapper"
```

---

## Task 4: Persistence store

**Files:**
- Create: `internal/store/store.go`
- Test: `internal/store/store_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/store/store_test.go`:
```go
package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMissingReturnsEmpty(t *testing.T) {
	recs, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("Load missing: %v", err)
	}
	if len(recs) != 0 {
		t.Fatalf("expected empty, got %v", recs)
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "sessions.json")
	in := []Record{
		{ID: "a1", Agent: "claude", Cwd: "/tmp", Tmux: "orchid-a1", AgentSessionID: "u1", Created: time.Unix(1700000000, 0).UTC(), Status: "running"},
		{ID: "b2", Agent: "shell", Cwd: "/x", Tmux: "orchid-b2", AgentSessionID: "u2", Created: time.Unix(1700000001, 0).UTC(), Status: "stopped"},
	}
	if err := Save(path, in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	out, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(out) != 2 || out[0].ID != "a1" || out[1].Status != "stopped" || out[0].AgentSessionID != "u1" {
		t.Fatalf("roundtrip mismatch: %+v", out)
	}
}

func TestSaveAtomicNoTempLeft(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sessions.json")
	if err := Save(path, []Record{{ID: "z"}}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "sessions.json" {
			t.Fatalf("unexpected leftover file: %s", e.Name())
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -v`
Expected: FAIL — `undefined: Load`.

- [ ] **Step 3: Write the implementation**

Create `internal/store/store.go`:
```go
// Package store persists session records as JSON.
package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Record is a persisted session. Status is the last-known value; the Manager
// reconciles it against live tmux on load.
type Record struct {
	ID             string    `json:"id"`
	Agent          string    `json:"agent"`
	Cwd            string    `json:"cwd"`
	Tmux           string    `json:"tmux"`
	AgentSessionID string    `json:"agent_session_id"`
	Created        time.Time `json:"created"`
	Status         string    `json:"status"` // "running" | "stopped"
}

// DefaultPath returns <user config dir>/orchid/sessions.json.
func DefaultPath() (string, error) {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg, "orchid", "sessions.json"), nil
}

// Load reads records from path. A missing file yields an empty slice, no error.
func Load(path string) ([]Record, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	var recs []Record
	if err := json.Unmarshal(data, &recs); err != nil {
		return nil, err
	}
	return recs, nil
}

// Save writes records to path atomically (temp file + rename), creating parent
// directories as needed.
func Save(path string, recs []Record) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(recs, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".sessions-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/ -v`
Expected: PASS (all 3 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/store/
git commit -m "feat: add JSON session store with atomic writes"
```

---

## Task 5: Rework Session to attach to tmux

**Files:**
- Modify: `internal/session/session.go`
- Modify: `internal/session/session_test.go` (existing tests call `newSession` — update them)

**Context:** Today `newSession(id, onExit)` runs `$SHELL` directly. We replace it with two constructors that both spawn a PTY running `tmux attach -t <tmux>`. The ring/broadcast/attachment/closeOnce machinery and `readLoop` stay exactly as they are except `readLoop`'s tail (onExit) is unchanged. We add descriptive fields. The existing Session methods (`Write`, `Resize`, `Alive`, `Close`, `Attach`, `Detach`, `Replay`, `Done`, `Ch`) are unchanged.

- [ ] **Step 1: Update existing session tests to the new constructor**

In `internal/session/session_test.go`, the existing tests (`TestSessionEchoesToRing`, `TestSessionAliveFlips`, `TestSessionAttachReceivesOutput`, `TestSessionAttachKicksPrevious`) call `newSession("testN")` or `newSession("testN", nil)`. Replace each `newSession(...)` call with a tmux-backed helper. Add this helper at the top of the file (after imports) and rewrite the four tests to use it:

```go
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
	s, err := createSession(rec, sh, nil)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}
	t.Cleanup(func() { s.Close(); tmux.KillSession(name) })
	return s
}
```

Then change each test's first lines from:
```go
	s, err := newSession("test1")
	if err != nil { t.Fatalf("newSession: %v", err) }
	defer s.Close()
```
to:
```go
	s := newTestSession(t, "test1")
```
(Use distinct ids: test1..test4. Remove the now-unused `err` handling and `defer s.Close()` — cleanup is registered by the helper. Keep the rest of each test body identical.)

- [ ] **Step 2: Run to verify the old constructor is gone / tests fail to compile**

Run: `go test ./internal/session/ -run TestSession -v`
Expected: FAIL — `undefined: createSession` (and `newSession` no longer referenced).

- [ ] **Step 3: Rewrite session.go constructors**

In `internal/session/session.go`, replace the imports and the `newSession` function. New imports block:
```go
import (
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
	"orchid/internal/agent"
	"orchid/internal/store"
	"orchid/internal/tmux"
)
```

Add descriptive fields to the `Session` struct (keep all existing fields; add these after `ID string`):
```go
	Agent          string
	Cwd            string
	Tmux           string
	AgentSessionID string
```

Replace the `newSession(...)` function with:
```go
// createSession creates a NEW detached tmux session running the agent's fresh
// command, then spawns the attach-PTY that views it.
func createSession(rec store.Record, ag agent.Agent, onExit func()) (*Session, error) {
	argv := ag.NewCommand(rec.Cwd, rec.AgentSessionID)
	if err := tmux.NewSession(rec.Tmux, rec.Cwd, argv); err != nil {
		return nil, err
	}
	return attachSession(rec, onExit)
}

// attachSession spawns the attach-PTY for an already-existing tmux session.
func attachSession(rec store.Record, onExit func()) (*Session, error) {
	attach := tmux.AttachArgv(rec.Tmux)
	cmd := exec.Command(attach[0], attach[1:]...)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	f, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}
	s := &Session{
		ID:             rec.ID,
		Agent:          rec.Agent,
		Cwd:            rec.Cwd,
		Tmux:           rec.Tmux,
		AgentSessionID: rec.AgentSessionID,
		cmd:            cmd,
		pty:            f,
		ring:           newRingBuffer(ringSize),
		onExit:         onExit,
		alive:          true,
	}
	go s.readLoop()
	return s, nil
}
```

Leave `readLoop`, `broadcast`, `Write`, `Resize`, `Alive`, `Close`, `Attach`, `Detach`, `Replay`, `closeOnce`, `Done`, `Ch` unchanged.

Note on `Close`: it currently does `pty.Close()` + `cmd.Process.Kill()` + `go cmd.Wait()`. For a tmux-attach PTY, killing the attach client does NOT kill the tmux session (good — agent survives). This is the desired behavior; no change needed.

- [ ] **Step 4: Run tests with the race detector**

Run: `go test ./internal/session/ -run TestSession -race -v`
Expected: PASS (or SKIP if tmux unavailable). No race warnings.

- [ ] **Step 5: Commit**

```bash
git add internal/session/session.go internal/session/session_test.go
git commit -m "feat: back sessions with tmux attach instead of bare shell"
```

---

## Task 6: Rework Manager — store, Restore, Restart, Create(agent, cwd)

**Files:**
- Modify: `internal/session/manager.go`
- Modify: `internal/session/manager_test.go`

**Context:** The Manager now persists records and tracks BOTH running sessions (with a live PTY in `sessions`) and all records (incl. stopped) in `records`. `Info` gains agent/status/cwd. `Create` takes an agent name + cwd. `onExit` marks a record stopped (not removed). New `Restore` and `Restart`. `Kill` removes the record + tmux session. The existing `TestManagerConcurrentCreate` and `TestSelfExitRemovesFromManager` tests must be updated (signatures changed; self-exit now → stopped, not removed).

- [ ] **Step 1: Rewrite manager_test.go**

Replace `internal/session/manager_test.go` with:
```go
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

	// Tell the shell to exit; the tmux session ends, attach client EOFs.
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
	// alive session
	alive, _ := m.Create("shell", "/tmp")
	defer tmux.KillSession(alive.Tmux)
	// a record whose tmux we kill -> should reconcile to stopped
	dead, _ := m.Create("shell", "/tmp")
	tmuxName := dead.Tmux
	// stop tracking by killing the tmux session out from under it
	_ = tmux.KillSession(tmuxName)

	// New manager over the SAME store path.
	m2 := NewManager(m.path)
	t.Cleanup(m2.KillAll)
	if err := m2.Restore(); err != nil {
		t.Fatalf("restore: %v", err)
	}
	defer tmux.KillSession(alive.Tmux)

	var aliveInfo, deadInfo *Info
	for i := range m2.List() {
		info := m2.List()[i]
		switch info.ID {
		case alive.ID:
			aliveInfo = &info
		case dead.ID:
			deadInfo = &info
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

	// Force it to stopped by killing tmux and waiting for onExit.
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

	// Restart an already-running session -> error.
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
			tmux.KillSession(s.Tmux) // best-effort cleanup registered below too
		}()
	}
	wg.Wait()
	if len(m.List()) != 6 {
		t.Fatalf("List len = %d, want 6", len(m.List()))
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/session/ -run TestManager -v`
Expected: FAIL — `NewManager` signature mismatch / `m.path` undefined / `Create` arity.

- [ ] **Step 3: Rewrite manager.go**

Replace `internal/session/manager.go` with:
```go
package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"orchid/internal/agent"
	"orchid/internal/store"
	"orchid/internal/tmux"
)

// Info is a serializable session summary.
type Info struct {
	ID     string `json:"id"`
	Agent  string `json:"agent"`
	Cwd    string `json:"cwd"`
	Status string `json:"status"` // "running" | "stopped"
	Alive  bool   `json:"alive"`
}

// Manager owns sessions (running, with a live PTY) and records (all, incl. stopped).
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session
	records  map[string]store.Record
	path     string
}

func NewManager(path string) *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
		records:  make(map[string]store.Record),
		path:     path,
	}
}

func newID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// saveLocked persists all records. Caller must hold m.mu. Errors are logged
// (tmux remains the source of truth), never fatal.
func (m *Manager) saveLocked() {
	recs := make([]store.Record, 0, len(m.records))
	for _, r := range m.records {
		recs = append(recs, r)
	}
	sort.Slice(recs, func(i, j int) bool { return recs[i].ID < recs[j].ID })
	if err := store.Save(m.path, recs); err != nil {
		log.Printf("orchid: save sessions: %v", err)
	}
}

// onExit marks a session stopped: drop its live PTY entry, flip the record to
// stopped, and persist. The record is retained (resumable).
func (m *Manager) onExit(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, id)
	if r, ok := m.records[id]; ok {
		r.Status = "stopped"
		m.records[id] = r
	}
	m.saveLocked()
}

func (m *Manager) Create(agentName, cwd string) (*Session, error) {
	ag, ok := agent.Get(agentName)
	if !ok {
		return nil, fmt.Errorf("unknown agent %q", agentName)
	}
	id := newID()
	rec := store.Record{
		ID:             id,
		Agent:          agentName,
		Cwd:            agent.ResolveCwd(cwd),
		Tmux:           "orchid-" + id,
		AgentSessionID: uuid.NewString(),
		Created:        time.Now().UTC(),
		Status:         "running",
	}
	s, err := createSession(rec, ag, func() { m.onExit(id) })
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.sessions[id] = s
	m.records[id] = rec
	m.saveLocked()
	m.mu.Unlock()
	return s, nil
}

// Restore loads persisted records and reconciles them against live tmux:
// alive -> running (reattach), dead -> stopped (retained).
func (m *Manager) Restore() error {
	recs, err := store.Load(m.path)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, rec := range recs {
		if tmux.HasSession(rec.Tmux) {
			rec.Status = "running"
			s, err := attachSession(rec, func(id string) func() {
				return func() { m.onExit(id) }
			}(rec.ID))
			if err != nil {
				log.Printf("orchid: reattach %s: %v", rec.ID, err)
				rec.Status = "stopped"
			} else {
				m.sessions[rec.ID] = s
			}
		} else {
			rec.Status = "stopped"
		}
		m.records[rec.ID] = rec
	}
	m.saveLocked()
	return nil
}

// Restart resumes a stopped session into a fresh tmux session.
func (m *Manager) Restart(id string) (*Session, error) {
	m.mu.Lock()
	rec, ok := m.records[id]
	_, running := m.sessions[id]
	m.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("session %q not found", id)
	}
	if running {
		return nil, fmt.Errorf("session %q already running", id)
	}
	ag, ok := agent.Get(rec.Agent)
	if !ok {
		return nil, fmt.Errorf("unknown agent %q", rec.Agent)
	}
	argv := ag.ResumeCommand(rec.Cwd, rec.AgentSessionID)
	if argv == nil {
		argv = ag.NewCommand(rec.Cwd, rec.AgentSessionID)
	}
	if err := tmux.NewSession(rec.Tmux, rec.Cwd, argv); err != nil {
		return nil, err
	}
	s, err := attachSession(rec, func() { m.onExit(id) })
	if err != nil {
		return nil, err
	}
	rec.Status = "running"
	m.mu.Lock()
	m.sessions[id] = s
	m.records[id] = rec
	m.saveLocked()
	m.mu.Unlock()
	return s, nil
}

func (m *Manager) Get(id string) (*Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	return s, ok
}

func (m *Manager) List() []Info {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Info, 0, len(m.records))
	for _, r := range m.records {
		_, running := m.sessions[r.ID]
		out = append(out, Info{
			ID:     r.ID,
			Agent:  r.Agent,
			Cwd:    r.Cwd,
			Status: r.Status,
			Alive:  running,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Kill terminates a session for good: kill tmux, drop session + record.
func (m *Manager) Kill(id string) error {
	m.mu.Lock()
	rec, ok := m.records[id]
	s := m.sessions[id]
	if ok {
		delete(m.records, id)
		delete(m.sessions, id)
	}
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("session %q not found", id)
	}
	if s != nil {
		s.Close()
	}
	_ = tmux.KillSession(rec.Tmux)
	m.mu.Lock()
	m.saveLocked()
	m.mu.Unlock()
	return nil
}

// KillAll closes live PTYs only (shutdown). tmux sessions and records survive
// so they can be restored next boot.
func (m *Manager) KillAll() {
	m.mu.Lock()
	all := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		all = append(all, s)
	}
	m.sessions = make(map[string]*Session)
	m.mu.Unlock()
	for _, s := range all {
		s.Close()
	}
}
```

Note: `attachSession`'s `onExit` parameter is `func()`. In `Restore` we wrap per-id with an immediately-invoked closure to capture `rec.ID` correctly inside the loop.

- [ ] **Step 4: Run tests with the race detector**

Run: `go test ./internal/session/ -race -v`
Expected: PASS (or SKIP without tmux). No race warnings. Note `KillAll` closes PTYs but leaves tmux sessions; tests register `tmux.KillSession` cleanups to avoid leaking tmux sessions across the suite.

- [ ] **Step 5: Commit**

```bash
git add internal/session/manager.go internal/session/manager_test.go
git commit -m "feat: persist sessions, reconcile on restore, support restart"
```

---

## Task 7: API — create body, restart route

**Files:**
- Modify: `internal/server/api.go`
- Modify: `internal/server/server.go`
- Modify: `internal/server/api_test.go`

**Context:** `New(mgr)` already takes `*session.Manager`. `handleCreate` currently calls `mgr.Create()` with no args; now it must read a JSON body `{agent, cwd}`. Add `handleRestart`. The `ws_test.go` and `static_test.go` create managers via `session.NewManager()` — those calls must be updated to `session.NewManager(<temp path>)` since the signature changed.

- [ ] **Step 1: Update server tests for the new manager signature + add API tests**

In `internal/server/api_test.go`, replace the file with:
```go
package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"orchid/internal/session"
	"orchid/internal/tmux"
)

func newTestServer(t *testing.T) (*httptest.Server, *session.Manager) {
	t.Helper()
	if err := tmux.Available(); err != nil {
		t.Skipf("tmux not available: %v", err)
	}
	mgr := session.NewManager(filepath.Join(t.TempDir(), "sessions.json"))
	t.Cleanup(mgr.KillAll)
	srv := httptest.NewServer(New(mgr))
	t.Cleanup(srv.Close)
	return srv, mgr
}

func TestAPICreateWithAgentBody(t *testing.T) {
	srv, mgr := newTestServer(t)

	body := bytes.NewBufferString(`{"agent":"shell","cwd":"/tmp"}`)
	resp, err := http.Post(srv.URL+"/api/sessions", "application/json", body)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create status = %d", resp.StatusCode)
	}
	var created session.Info
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	if created.ID == "" || created.Agent != "shell" {
		t.Fatalf("created = %+v", created)
	}
	// cleanup tmux
	for _, info := range mgr.List() {
		_ = mgr.Kill(info.ID)
	}
}

func TestAPICreateUnknownAgent400(t *testing.T) {
	srv, _ := newTestServer(t)
	body := bytes.NewBufferString(`{"agent":"nope"}`)
	resp, _ := http.Post(srv.URL+"/api/sessions", "application/json", body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestAPIKillUnknownReturns404(t *testing.T) {
	srv, _ := newTestServer(t)
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/sessions/nope", nil)
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestAPIRestartRunningReturns409(t *testing.T) {
	srv, mgr := newTestServer(t)
	s, err := mgr.Create("shell", "/tmp")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer mgr.Kill(s.ID)

	resp, _ := http.Post(srv.URL+"/api/sessions/"+s.ID+"/restart", "application/json", nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("restart running status = %d, want 409", resp.StatusCode)
	}
	resp.Body.Close()
}
```

Also update `internal/server/ws_test.go` and `internal/server/static_test.go`: change every `session.NewManager()` call to `session.NewManager(filepath.Join(t.TempDir(), "sessions.json"))` and add `"path/filepath"` to imports. In `ws_test.go`, the test calls `mgr.Create()` — change to `mgr.Create("shell", "/tmp")` and add a `tmux.Available()` skip guard at the top of the test plus `"orchid/internal/tmux"` import; defer `mgr.Kill` for created sessions.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/server/ -v`
Expected: FAIL — compile errors (`mgr.Create` arity, `NewManager` arity, `handleRestart` route missing).

- [ ] **Step 3: Update server.go routes**

In `internal/server/server.go` `routes()`, add after the DELETE line:
```go
	s.mux.HandleFunc("POST /api/sessions/{id}/restart", s.handleRestart)
```

- [ ] **Step 4: Update api.go**

Replace `handleCreate` and add `handleRestart` in `internal/server/api.go`. Final file:
```go
package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.mgr.List())
}

type createReq struct {
	Agent string `json:"agent"`
	Cwd   string `json:"cwd"`
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createReq
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req) // empty body -> zero struct
	}
	if req.Agent == "" {
		http.Error(w, "missing agent", http.StatusBadRequest)
		return
	}
	sess, err := s.mgr.Create(req.Agent, req.Cwd)
	if err != nil {
		if strings.Contains(err.Error(), "unknown agent") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":     sess.ID,
		"agent":  sess.Agent,
		"cwd":    sess.Cwd,
		"status": "running",
		"alive":  true,
	})
}

func (s *Server) handleKill(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.mgr.Kill(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sess, err := s.mgr.Restart(id)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "not found"):
			http.Error(w, err.Error(), http.StatusNotFound)
		case strings.Contains(err.Error(), "already running"):
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":     sess.ID,
		"agent":  sess.Agent,
		"cwd":    sess.Cwd,
		"status": "running",
		"alive":  true,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

var _ = errors.New // (kept only if errors becomes unused after edits; remove if unused)
```
Remove the trailing `var _ = errors.New` line and the `"errors"` import if `errors` ends up unused (it is unused above — so DELETE both the import and that line). The real imports needed are `encoding/json`, `net/http`, `strings`.

- [ ] **Step 5: Run tests (race)**

Run: `go test ./internal/server/ -race -v`
Expected: PASS (or SKIP without tmux). No race warnings.

- [ ] **Step 6: Commit**

```bash
git add internal/server/
git commit -m "feat: API create-with-agent body and restart endpoint"
```

---

## Task 8: Frontend — agent select, status, restart

**Files:**
- Modify: `web/index.html`
- Modify: `web/app.js`

- [ ] **Step 1: Update index.html**

In `web/index.html`, replace the `<button id="new">+ New session</button>` line with a small create form:
```html
    <div id="create">
      <select id="agent">
        <option value="claude">claude</option>
        <option value="codex">codex</option>
        <option value="shell">shell</option>
      </select>
      <input id="cwd" type="text" placeholder="cwd (optional)" />
      <button id="new">+ New</button>
    </div>
```
And add to the `<style>` block:
```css
    #create { padding: 8px; display: flex; flex-direction: column; gap: 4px; border-bottom: 1px solid #333; }
    #create select, #create input, #create button { padding: 6px; }
    #list li .meta { display: flex; flex-direction: column; }
    #list li .sub { font-size: 11px; opacity: 0.6; }
    #list .dot { width: 8px; height: 8px; border-radius: 50%; display: inline-block; margin-right: 6px; }
    #list .dot.running { background: #6f6; }
    #list .dot.stopped { background: #888; }
    #list .btns { display: flex; gap: 4px; }
    #list .btn { border: none; background: transparent; cursor: pointer; font-size: 14px; }
    #list .restart { color: #6cf; }
    #list .kill { color: #f66; }
    #placeholder { color: #666; padding: 20px; font-family: system-ui, sans-serif; }
```

- [ ] **Step 2: Update app.js**

Replace `web/app.js` with:
```js
const listEl = document.getElementById("list");
const newBtn = document.getElementById("new");
const agentSel = document.getElementById("agent");
const cwdInput = document.getElementById("cwd");
const termEl = document.getElementById("term");

let term = null;
let fit = null;
let ws = null;
let activeId = null;

async function fetchSessions() {
  const r = await fetch("/api/sessions");
  return r.json();
}

async function createSession() {
  const r = await fetch("/api/sessions", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ agent: agentSel.value, cwd: cwdInput.value }),
  });
  if (!r.ok) {
    alert("create failed: " + (await r.text()));
    return;
  }
  const s = await r.json();
  cwdInput.value = "";
  await refresh();
  selectSession(s.id);
}

async function killSession(id) {
  await fetch("/api/sessions/" + id, { method: "DELETE" });
  if (id === activeId) teardown();
  await refresh();
}

async function restartSession(id) {
  const r = await fetch("/api/sessions/" + id + "/restart", { method: "POST" });
  if (!r.ok) {
    alert("restart failed: " + (await r.text()));
    return;
  }
  await refresh();
  selectSession(id);
}

function showPlaceholder(text) {
  teardown();
  termEl.innerHTML = '<div id="placeholder">' + text + "</div>";
}

async function refresh() {
  const sessions = await fetchSessions();
  listEl.innerHTML = "";
  for (const s of sessions) {
    const li = document.createElement("li");
    if (s.id === activeId) li.classList.add("active");
    if (s.status !== "running") li.classList.add("dead");

    const meta = document.createElement("div");
    meta.className = "meta";
    const dot = '<span class="dot ' + s.status + '"></span>';
    meta.innerHTML = dot + s.agent + '<span class="sub">' + s.id + "</span>";
    meta.onclick = () => onSelect(s);

    const btns = document.createElement("div");
    btns.className = "btns";
    if (s.status !== "running") {
      const rb = document.createElement("button");
      rb.className = "btn restart";
      rb.textContent = "▶";
      rb.title = "restart";
      rb.onclick = (e) => { e.stopPropagation(); restartSession(s.id); };
      btns.appendChild(rb);
    }
    const kb = document.createElement("button");
    kb.className = "btn kill";
    kb.textContent = "✕";
    kb.title = "delete";
    kb.onclick = (e) => { e.stopPropagation(); killSession(s.id); };
    btns.appendChild(kb);

    li.appendChild(meta);
    li.appendChild(btns);
    listEl.appendChild(li);
  }
}

function onSelect(s) {
  if (s.status !== "running") {
    activeId = s.id;
    showPlaceholder("stopped — restart to attach");
    refresh();
    return;
  }
  selectSession(s.id);
}

function teardown() {
  if (ws) { ws.onclose = null; ws.close(); ws = null; }
  if (term) { term.dispose(); term = null; }
  fit = null;
}

function selectSession(id) {
  teardown();
  termEl.innerHTML = "";
  activeId = id;

  term = new Terminal({ cursorBlink: true, fontSize: 14 });
  fit = new FitAddon.FitAddon();
  term.loadAddon(fit);
  term.open(termEl);
  fit.fit();

  const proto = location.protocol === "https:" ? "wss" : "ws";
  ws = new WebSocket(`${proto}://${location.host}/ws/${id}`);
  ws.binaryType = "arraybuffer";

  ws.onopen = () => sendResize();
  ws.onmessage = (ev) => {
    if (ev.data instanceof ArrayBuffer) term.write(new Uint8Array(ev.data));
  };
  ws.onclose = () => { /* session ended or detached */ };

  term.onData((d) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(new TextEncoder().encode(d));
    }
  });

  const ro = new ResizeObserver(() => { if (fit) { fit.fit(); sendResize(); } });
  ro.observe(termEl);

  refresh();
}

function sendResize() {
  if (!term || !ws || ws.readyState !== WebSocket.OPEN) return;
  ws.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }));
}

newBtn.onclick = createSession;
refresh();
```

- [ ] **Step 3: Verify build + embed still serves**

Run: `go build ./... && go test ./internal/server/ -run TestStatic -v`
Expected: build clean; static test passes (or skips). The frontend is static — no JS test harness; manual verification happens in Task 10.

- [ ] **Step 4: Commit**

```bash
git add web/
git commit -m "feat: dashboard agent select, status, restart controls"
```

---

## Task 9: Wire main — tmux check, store path, Restore

**Files:**
- Modify: `cmd/orchid/main.go`

- [ ] **Step 1: Replace main.go**

Replace `cmd/orchid/main.go` with:
```go
package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"strconv"

	"orchid/internal/server"
	"orchid/internal/session"
	"orchid/internal/store"
	"orchid/internal/tmux"
)

func main() {
	host := flag.String("host", "127.0.0.1", "listen host (localhost only by default)")
	port := flag.Int("port", 8081, "listen port")
	flag.Parse()

	if err := tmux.Available(); err != nil {
		log.Fatalf("orchid requires tmux: %v (install tmux and retry)", err)
	}

	path, err := store.DefaultPath()
	if err != nil {
		log.Fatalf("orchid: resolve config path: %v", err)
	}

	mgr := session.NewManager(path)
	if err := mgr.Restore(); err != nil {
		log.Printf("orchid: restore sessions: %v", err)
	}
	defer mgr.KillAll()

	addr := net.JoinHostPort(*host, strconv.Itoa(*port))
	h := server.New(mgr)
	log.Printf("orchid listening on http://%s (sessions: %s)", addr, path)
	if err := http.ListenAndServe(addr, h); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 2: Build**

Run: `go build -o orchid ./cmd/orchid`
Expected: binary built, exit 0.

- [ ] **Step 3: Full verification**

Run: `go build ./... && go vet ./... && go test ./... -race`
Expected: all green (tmux-dependent tests run since tmux is present).

- [ ] **Step 4: Commit**

```bash
git add cmd/orchid/main.go
git commit -m "feat: tmux preflight, persistent store path, restore on boot"
```

---

## Task 10: End-to-end smoke test

**Files:** none (verification only).

- [ ] **Step 1: Build and run**

Run:
```bash
go build -o orchid ./cmd/orchid
./orchid -port 18090 &
sleep 1
```

- [ ] **Step 2: Create a shell session and verify tmux + persistence**

Run:
```bash
ID=$(curl -sf -X POST -H 'Content-Type: application/json' -d '{"agent":"shell","cwd":"/tmp"}' http://127.0.0.1:18090/api/sessions | sed 's/.*"id":"\([^"]*\)".*/\1/')
echo "id=$ID"
tmux has-session -t "orchid-$ID" && echo "tmux session exists"
curl -sf http://127.0.0.1:18090/api/sessions; echo
cat "${XDG_CONFIG_HOME:-$HOME/.config}/orchid/sessions.json"
```
Expected: a non-empty id; `tmux has-session` succeeds; list shows the session running; the JSON file contains the record with `agent_session_id` and `status: running`.

- [ ] **Step 3: Verify restart-after-stop**

Run:
```bash
tmux kill-session -t "orchid-$ID"   # simulate agent exit
sleep 1
curl -sf http://127.0.0.1:18090/api/sessions; echo   # expect status: stopped
curl -sf -X POST http://127.0.0.1:18090/api/sessions/$ID/restart; echo  # expect status: running
tmux has-session -t "orchid-$ID" && echo "tmux session back"
```
Expected: list shows `stopped` after kill; restart returns running; tmux session exists again.

- [ ] **Step 4: Verify restart survives orchid restart (Restore)**

Run:
```bash
kill %1 2>/dev/null; sleep 1          # stop orchid (tmux session survives)
./orchid -port 18090 &
sleep 1
curl -sf http://127.0.0.1:18090/api/sessions; echo   # session reattached as running
```
Expected: the session reappears as `running` (reattached from tmux) after orchid restart.

- [ ] **Step 5: Browser smoke (manual)**

Open `http://127.0.0.1:18090`. Verify: create a `shell` session (agent dropdown), terminal opens and accepts input; create a `claude` session (if `claude` is on PATH) and confirm it boots in the terminal; stop a session via `tmux kill-session`, refresh, confirm it shows stopped with a ▶ restart button that re-attaches. Then:
```bash
kill %1 2>/dev/null
# clean up any leftover test tmux sessions
tmux ls 2>/dev/null | grep '^orchid-' | cut -d: -f1 | xargs -r -n1 tmux kill-session -t
```

- [ ] **Step 6: Commit (if any cleanup/docs changed; otherwise skip)**

No code change expected in this task. If the smoke test surfaced a bug, fix it under the relevant task and re-run.

---

## Self-Review Notes

- **Spec coverage:** Agent interface + registry + claude/codex/shell (T2); tmux wrapper (T3); JSON store at `~/.config` with atomic writes (T4); PTY-as-tmux-attach Session with create/attach constructors (T5); Manager persistence + Restore reconcile (alive→running, dead→stopped) + Restart + onExit→stopped + Kill removes record (T6); API create-with-agent body + 400 unknown + restart 200/404/409 (T7); dashboard agent select + cwd + status + restart (T8); main tmux preflight + store path + Restore at boot (T9); E2E incl. survive-restart (T10). All spec sections mapped.
- **codex limitation:** encoded per the verified spec — `NewCommand` returns `["codex"]` (id ignored), `ResumeCommand` returns `["codex","resume","--last"]`. Documented in codex.go and the spec.
- **Type consistency:** `store.Record` fields (ID/Agent/Cwd/Tmux/AgentSessionID/Created/Status) used consistently across store/session/manager; `Info{ID,Agent,Cwd,Status,Alive}` in manager + returned by API/used in app.js; `createSession(rec, ag, onExit)` / `attachSession(rec, onExit)` signatures match callers in manager; `agent.Get/Names/ResolveCwd`, `tmux.Available/NewSession/HasSession/KillSession/AttachArgv`, `store.DefaultPath/Load/Save` consistent across tasks.
- **Test isolation:** all tmux-touching tests skip when `tmux.Available()` fails and register `tmux.KillSession` cleanups; `KillAll` deliberately leaves tmux sessions alive (restore semantics), so tests must kill tmux sessions explicitly — accounted for in cleanups.
- **Deliberate deviation:** the unused-import guard lines (`var _ = errors.New`) are explicitly called out to be removed; flagged so they are not left in.
```
