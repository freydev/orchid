# Orchid Agent Status Signals Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give agent sessions a live, reliable status — `running | requires_action | idle | stopped` — driven by the agent **actively reporting** via Claude Code hooks + an MCP tool, combined server-side with cheap output-activity for shell sessions.

**Architecture:** A bundled Claude plugin fires hooks (`UserPromptSubmit`/`PreToolUse`→running, `Stop`→done, `Notification`→requires_action) that call `orchid signal <state>`, and registers an MCP server (`orchid mcp`) exposing a `set_status` tool the agent calls to assert `requires_action`. Both POST to the orchid HTTP API, which records an in-memory signal per session; `Manager.List` derives the final status (with read-gating so an informational "done" becomes `idle` once the user opens the terminal, while a question stays `requires_action` until the user replies). Session identity flows through tmux env (`ORCHID_SESSION`/`ORCHID_URL`/`ORCHID_BIN`) inherited by hooks and the MCP server.

**Tech Stack:** Go 1.26 stdlib (`net/http`, `encoding/json`, `os/exec`, `bufio`, `time`); existing session/agent/tmux/server packages; vanilla JS frontend; Claude Code plugin (`plugin.json` + `hooks/hooks.json` + `.mcp.json` + a skill).

**Reference:** Design spec `docs/superpowers/specs/2026-06-01-orchid-agent-status-signals-design.md`. Supersedes the discarded approach in `2026-05-30-orchid-session-activity-status-design.md` (whose postmortem proved passive inference cannot distinguish idle from requires_action).

---

## File Structure

```
internal/session/session.go         # MODIFIED: lastOutput field + Active(); set in readLoop + attachSession
internal/session/session_test.go    # + TestSessionActive
internal/tmux/tmux.go               # MODIFIED: NewSession takes env []string -> tmux -e
internal/tmux/tmux_test.go          # + env-injection test
internal/session/manager.go         # MODIFIED: signals map, SetEnv/sessionEnv, Signal/MarkSeen, env into create+restart, List derivation
internal/session/manager_test.go    # + signal/status-resolution tests
internal/statusclient/statusclient.go      # NEW: Post(baseURL,id,state,needsReply) HTTP helper
internal/statusclient/statusclient_test.go # NEW
internal/server/api.go              # MODIFIED: handleStatus + handleSeen
internal/server/server.go           # MODIFIED: routes POST /status, POST /seen
internal/server/api_test.go         # + status/seen handler tests
internal/mcp/mcp.go                 # NEW: stdio JSON-RPC MCP server, set_status tool
internal/mcp/mcp_test.go            # NEW
cmd/orchid/cli.go                   # MODIFIED: route() + signalCmd + mcpCmd; serveCmd sets mgr env
cmd/orchid/cli_test.go              # + signalCmd test (if a test file exists; else new)
web/app.js                          # MODIFIED: 4-state statusMeta, data-status, pollStatus, seen-on-open
plugins/orchid-status/.claude-plugin/plugin.json   # NEW
plugins/orchid-status/hooks/hooks.json             # NEW
plugins/orchid-status/.mcp.json                    # NEW
plugins/orchid-status/bin/signal.sh                # NEW (wrapper: no-op when env absent)
plugins/orchid-status/skills/report-status/SKILL.md# NEW
plugins/orchid-status/README.md                    # NEW
```

---

## Task 1: Session output-activity (`lastOutput` + `Active`)

**Files:** Modify `internal/session/session.go`, `internal/session/session_test.go`.

- [ ] **Step 1: Write the failing test** — append to `internal/session/session_test.go`. (If `newTestSession` does not exist in that file, instead place this test in `manager_test.go` style using `m.Create`; but session_test.go already constructs sessions — match its existing helper. Adjust the constructor call to the file's existing pattern.)

```go
func TestSessionActive(t *testing.T) {
	skipNoTmux(t)
	m := newTestManager(t)
	s, err := m.Create("p-1", "/tmp", "shell", "")
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
```

If `session_test.go` lacks `newTestManager`/`skipNoTmux`/`tmux` import, add this test to `internal/session/manager_test.go` instead (those helpers live there) — it exercises the same `Session.Active` method.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/session/ -run TestSessionActive -v`
Expected: FAIL — `s.Active undefined`.

- [ ] **Step 3: Implement.** In `internal/session/session.go`:

(a) Add `"time"` to the import block:
```go
import (
	"sync"
	"time"

	"orchid/internal/agent"
	"orchid/internal/store"
	"orchid/internal/tmux"
)
```

(b) Add the field to the `Session` struct (after `alive bool`):
```go
	mu         sync.Mutex
	alive      bool
	lastOutput time.Time
	sub        *attachment
	doneOnce   sync.Once
```

(c) Seed it in `attachSession`'s returned literal (add the field):
```go
	return &Session{
		ID:             rec.ID,
		Agent:          rec.Agent,
		Cwd:            rec.Cwd,
		Project:        rec.Project,
		Worktree:       rec.Worktree,
		Tmux:           rec.Tmux,
		AgentSessionID: rec.AgentSessionID,
		ctl:            ctl,
		ring:           ring,
		onExit:         onExit,
		alive:          true,
		lastOutput:     time.Now(),
	}, nil
```

(d) Record output time in `readLoop`:
```go
func (s *Session) readLoop() {
	for data := range s.ctl.Output() {
		s.mu.Lock()
		s.lastOutput = time.Now()
		s.mu.Unlock()
		s.broadcast(data)
	}
	s.mu.Lock()
	s.alive = false
	if s.sub != nil {
		closeOnce(s.sub.done)
		s.sub = nil
	}
	s.mu.Unlock()
	if s.onExit != nil {
		s.onExit()
	}
}
```

(e) Add the accessor (after `Alive`):
```go
// Active reports whether pane output was produced within the given window.
func (s *Session) Active(within time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return time.Since(s.lastOutput) < within
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/session/ -run TestSessionActive -race -v`
Expected: PASS (or SKIP without tmux). No races.

- [ ] **Step 5: Commit**

```bash
git add internal/session/session.go internal/session/session_test.go internal/session/manager_test.go
git commit -m "feat: track session output activity (lastOutput + Active)"
```

---

## Task 2: tmux env injection

**Files:** Modify `internal/tmux/tmux.go`, `internal/tmux/tmux_test.go`.

- [ ] **Step 1: Write the failing test** — append to `internal/tmux/tmux_test.go`:

```go
func TestNewSessionInjectsEnv(t *testing.T) {
	if err := Available(); err != nil {
		t.Skipf("tmux not available: %v", err)
	}
	name := "orchid-test-env"
	_ = KillSession(name)
	// run a shell so the session stays alive long enough to inspect env
	if err := NewSession(name, "/tmp", []string{"ORCHID_SESSION=abc123", "ORCHID_URL=http://x"}, []string{"sh"}); err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer KillSession(name)
	out, err := exec.Command("tmux", "show-environment", "-t", name, "ORCHID_SESSION").CombinedOutput()
	if err != nil {
		t.Fatalf("show-environment: %v: %s", err, out)
	}
	if string(out) != "ORCHID_SESSION=abc123\n" {
		t.Fatalf("env = %q, want ORCHID_SESSION=abc123", string(out))
	}
}
```

(`exec` is already imported in tmux_test.go via the existing `show-options` test; if not, add `"os/exec"`.)

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/tmux/ -run TestNewSessionInjectsEnv -v`
Expected: FAIL — `NewSession` signature mismatch (too many arguments).

- [ ] **Step 3: Implement.** In `internal/tmux/tmux.go`, change `NewSession` to accept env and pass each as `-e`:

```go
// NewSession creates a detached tmux session named name, with working directory
// cwd, exporting each "K=V" entry of env into the session, and running argv
// directly (no shell interpolation).
func NewSession(name, cwd string, env []string, argv []string) error {
	args := []string{"new-session", "-d", "-s", name, "-c", cwd}
	for _, e := range env {
		args = append(args, "-e", e)
	}
	args = append(args, "--")
	args = append(args, argv...)
	out, err := exec.Command("tmux", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("tmux new-session: %w: %s", err, out)
	}
	_ = exec.Command("tmux", "set-option", "-t", name, "window-size", "latest").Run()
	return nil
}
```

- [ ] **Step 4: Update the two callers so the package compiles.** In `internal/session/session.go` `createSession`, change the call to thread an `env` parameter (the parameter is added in Task 3; for now make `createSession` accept it):

```go
func createSession(rec store.Record, ag agent.Agent, env []string, onExit func()) (*Session, error) {
	argv := ag.NewCommand(rec.Cwd, rec.AgentSessionID)
	if err := tmux.NewSession(rec.Tmux, rec.Cwd, env, argv); err != nil {
		return nil, err
	}
	return attachSession(rec, onExit)
}
```

In `internal/session/manager.go` `Create`, temporarily pass `nil` (Task 3 replaces it):
```go
	s, err := createSession(rec, ag, nil, func() { m.onExit(id) })
```
In `internal/session/manager.go` `Restart`, change the tmux call:
```go
	if err := tmux.NewSession(rec.Tmux, rec.Cwd, nil, argv); err != nil {
```

- [ ] **Step 5: Run to verify it passes**

Run: `go build ./... && go test ./internal/tmux/ -run TestNewSessionInjectsEnv -v`
Expected: build clean; test PASS (or SKIP without tmux).

- [ ] **Step 6: Commit**

```bash
git add internal/tmux/tmux.go internal/tmux/tmux_test.go internal/session/session.go internal/session/manager.go
git commit -m "feat: tmux NewSession injects session env via -e"
```

---

## Task 3: Manager signals + status derivation + env wiring

**Files:** Modify `internal/session/manager.go`, `internal/session/manager_test.go`.

- [ ] **Step 1: Write the failing test** — append to `internal/session/manager_test.go`:

```go
func TestSignalStatusResolution(t *testing.T) {
	skipNoTmux(t)
	m := newTestManager(t)
	s, err := m.Create("p-1", "/tmp", "shell", "")
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

	// running signal -> running
	m.Signal(id, "running", false)
	if st := status(); st != "running" {
		t.Fatalf("after running signal: %q", st)
	}
	// requires_action -> sticky requires_action even after a read
	m.Signal(id, "requires_action", true)
	m.MarkSeen(id)
	if st := status(); st != "requires_action" {
		t.Fatalf("requires_action should survive a read: %q", st)
	}
	// a running signal (user replied) clears it
	m.Signal(id, "running", false)
	if st := status(); st != "running" {
		t.Fatalf("running should clear requires_action: %q", st)
	}
	// done + unread -> requires_action (attention); done + read -> idle
	m.Signal(id, "done", false)
	if st := status(); st != "requires_action" {
		t.Fatalf("unread done should read as requires_action: %q", st)
	}
	m.MarkSeen(id)
	if st := status(); st != "idle" {
		t.Fatalf("read done should be idle: %q", st)
	}
	// explicit idle
	m.Signal(id, "idle", false)
	if st := status(); st != "idle" {
		t.Fatalf("idle signal: %q", st)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/session/ -run TestSignalStatusResolution -v`
Expected: FAIL — `m.Signal` / `m.MarkSeen` undefined.

- [ ] **Step 3: Implement.** In `internal/session/manager.go`:

(a) Add a const near the top (after the imports):
```go
const idleAfter = 3 * time.Second
```

(b) Add the signal type and Manager fields. Add to the `Manager` struct:
```go
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session
	records  map[string]store.Record
	signals  map[string]*signalState
	selfURL  string
	binPath  string
	path     string
	closing  bool
}

// signalState is the latest agent-reported activity for a live session.
type signalState struct {
	state      string // "running" | "requires_action" | "idle" | "done"
	needsReply bool   // requires_action set by an explicit question/permission
	read       bool   // user has viewed the terminal since the last done/attention
}
```

(c) Initialize the map in `NewManager`:
```go
func NewManager(path string) *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
		records:  make(map[string]store.Record),
		signals:  make(map[string]*signalState),
		path:     path,
	}
}
```

(d) Add env config + helper (after `NewManager`):
```go
// SetEnv configures the values injected into each session's tmux environment so
// the agent's hooks/MCP server can call back. selfURL is the orchid base URL
// (e.g. http://127.0.0.1:8081); binPath is the absolute path to this binary.
func (m *Manager) SetEnv(selfURL, binPath string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.selfURL = selfURL
	m.binPath = binPath
}

// sessionEnv returns the tmux env entries for a session id. Caller need not hold m.mu.
func (m *Manager) sessionEnv(id string) []string {
	m.mu.Lock()
	url, bin := m.selfURL, m.binPath
	m.mu.Unlock()
	return []string{
		"ORCHID_SESSION=" + id,
		"ORCHID_URL=" + url,
		"ORCHID_BIN=" + bin,
	}
}
```

(e) Pass env into create + restart. In `Create`, replace the `createSession(...)` call:
```go
	s, err := createSession(rec, ag, m.sessionEnv(id), func() { m.onExit(id) })
```
In `Restart`, replace the tmux call:
```go
	if err := tmux.NewSession(rec.Tmux, rec.Cwd, m.sessionEnv(id), argv); err != nil {
```

(f) Add the signal mutators (after `Restart` or near `List`):
```go
// Signal records an agent-reported state for a session.
func (m *Manager) Signal(id, state string, needsReply bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.records[id]; !ok {
		return
	}
	sg := m.signals[id]
	if sg == nil {
		sg = &signalState{}
		m.signals[id] = sg
	}
	switch state {
	case "running":
		sg.state = "running"
		sg.needsReply = false
		sg.read = false
	case "done":
		sg.state = "done"
		sg.read = false
	case "requires_action":
		sg.state = "requires_action"
		sg.needsReply = true
		sg.read = false
	case "idle":
		sg.state = "idle"
		sg.needsReply = false
	}
}

// MarkSeen records that the user has viewed a session's terminal.
func (m *Manager) MarkSeen(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if sg := m.signals[id]; sg != nil {
		sg.read = true
	}
}
```

(g) Clean up the signal on kill/exit. In `onExit`, after `delete(m.sessions, id)` add `delete(m.signals, id)`. In `Kill`, inside the `if ok {` block after the deletes add `delete(m.signals, id)`. In `KillByWorktree`, inside the loop after `delete(m.sessions, id)` add `delete(m.signals, id)`.

(h) Rewrite `List` to derive status (snapshot under lock, compute outside). Replace the entire `List` method:
```go
func (m *Manager) List() []Info {
	type ent struct {
		rec store.Record
		s   *Session
		sg  signalState
		has bool
	}
	m.mu.Lock()
	ents := make([]ent, 0, len(m.records))
	for _, r := range m.records {
		e := ent{rec: r, s: m.sessions[r.ID]}
		if sg := m.signals[r.ID]; sg != nil {
			e.sg, e.has = *sg, true
		}
		ents = append(ents, e)
	}
	m.mu.Unlock()

	out := make([]Info, 0, len(ents))
	for _, e := range ents {
		status := resolveStatus(e.rec.Agent, e.s, e.sg, e.has)
		out = append(out, Info{
			ID:       e.rec.ID,
			Name:     e.rec.Name,
			Agent:    e.rec.Agent,
			Cwd:      e.rec.Cwd,
			Project:  e.rec.Project,
			Worktree: e.rec.Worktree,
			Status:   status,
			Alive:    e.s != nil,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// resolveStatus maps a session + its latest signal to a display status.
func resolveStatus(agentName string, s *Session, sg signalState, has bool) string {
	if s == nil {
		return "stopped"
	}
	// No agent signal yet (or shell, which never signals): use output activity.
	if !has || agentName == "shell" {
		if s.Active(idleAfter) {
			return "running"
		}
		return "idle"
	}
	switch sg.state {
	case "running":
		return "running"
	case "requires_action":
		return "requires_action"
	case "done":
		if sg.needsReply || !sg.read {
			return "requires_action"
		}
		return "idle"
	case "idle":
		return "idle"
	default:
		if s.Active(idleAfter) {
			return "running"
		}
		return "idle"
	}
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/session/ -run 'TestSignalStatusResolution|TestList|TestManager|TestKill|TestPrune|TestRestore|TestSelfExit' -race -v`
Expected: PASS. No races. (Existing `TestManagerCreateGetKill` asserts `info.Project`/`Worktree` only — still valid. `TestSelfExitMarksStopped`, if present, still resolves a dead session to `stopped`.)

- [ ] **Step 5: Commit**

```bash
git add internal/session/manager.go internal/session/manager_test.go
git commit -m "feat: manager signal state + read-gated status derivation"
```

---

## Task 4: status HTTP client helper

**Files:** Create `internal/statusclient/statusclient.go`, `internal/statusclient/statusclient_test.go`.

- [ ] **Step 1: Write the failing test** — create `internal/statusclient/statusclient_test.go`:

```go
package statusclient

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPost(t *testing.T) {
	var gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := Post(srv.URL, "sess-1", "requires_action", true); err != nil {
		t.Fatalf("Post: %v", err)
	}
	if gotPath != "/api/sessions/sess-1/status" {
		t.Fatalf("path = %q", gotPath)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(gotBody), &body); err != nil {
		t.Fatalf("body not json: %v", err)
	}
	if body["state"] != "requires_action" || body["needs_reply"] != true {
		t.Fatalf("body = %v", body)
	}
}

func TestPostEmptyBaseURL(t *testing.T) {
	// no base URL (agent outside orchid) -> no-op, no error
	if err := Post("", "x", "running", false); err != nil {
		t.Fatalf("expected nil error for empty baseURL, got %v", err)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/statusclient/ -v`
Expected: FAIL — package/`Post` does not exist.

- [ ] **Step 3: Implement.** Create `internal/statusclient/statusclient.go`:

```go
// Package statusclient posts agent status signals to a running orchid server.
// It is used by the `orchid signal` subcommand and the `orchid mcp` server.
package statusclient

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// Post reports a session's state to the orchid API at baseURL. A blank baseURL
// or id is a no-op (the agent is not running under orchid). Best-effort: a short
// timeout, errors returned but safe to ignore.
func Post(baseURL, id, state string, needsReply bool) error {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" || id == "" {
		return nil
	}
	body, err := json.Marshal(map[string]any{"state": state, "needs_reply": needsReply})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/sessions/"+id+"/status", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c := &http.Client{Timeout: 2 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/statusclient/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/statusclient/
git commit -m "feat: statusclient.Post helper for agent status signals"
```

---

## Task 5: server status + seen endpoints

**Files:** Modify `internal/server/api.go`, `internal/server/server.go`, `internal/server/api_test.go`.

- [ ] **Step 1: Write the failing test** — append to `internal/server/api_test.go`. (It already builds a server with a Manager; reuse its existing setup helper — find how other tests obtain `mgr`/handler and mirror it. The assertions below assume an `httptest` server `ts` and a created session `s`.)

```go
func TestStatusAndSeen(t *testing.T) {
	mgr, ts, s := newServerWithSession(t) // see note below
	defer ts.Close()

	post := func(path, body string) int {
		resp, err := http.Post(ts.URL+path, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("POST %s: %v", path, err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}

	if code := post("/api/sessions/"+s.ID+"/status", `{"state":"requires_action","needs_reply":true}`); code != http.StatusOK {
		t.Fatalf("status code = %d", code)
	}
	if st := statusOf(mgr, s.ID); st != "requires_action" {
		t.Fatalf("after status signal: %q", st)
	}
	if code := post("/api/sessions/"+s.ID+"/seen", ``); code != http.StatusOK {
		t.Fatalf("seen code = %d", code)
	}
	// requires_action survives a read
	if st := statusOf(mgr, s.ID); st != "requires_action" {
		t.Fatalf("requires_action should survive seen: %q", st)
	}
	// unknown id -> 404
	if code := post("/api/sessions/nope/status", `{"state":"running"}`); code != http.StatusNotFound {
		t.Fatalf("unknown id code = %d", code)
	}
}

func statusOf(mgr *session.Manager, id string) string {
	for _, info := range mgr.List() {
		if info.ID == id {
			return info.Status
		}
	}
	return ""
}
```

**Note:** `newServerWithSession` is a helper you add mirroring the existing api_test setup: create a `session.Manager` + `project.Store`, register a project for a tmux-backed repo, `mgr.Create(...)` a `shell` session, wrap `server.New(mgr, ps)` in `httptest.NewServer`, return `(mgr, ts, s)`. If the existing test file already has equivalent setup inline, factor it into this helper (or inline these assertions into the existing test). Skip with `skipNoTmux(t)` since it needs a real session.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/server/ -run TestStatusAndSeen -v`
Expected: FAIL — handlers/routes missing (404 on `/status`).

- [ ] **Step 3: Add handlers.** In `internal/server/api.go` add (near `handlePrune`):

```go
type statusReq struct {
	State     string `json:"state"`
	NeedsReply bool  `json:"needs_reply"`
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.mgr.Get(id); !ok {
		// Get only finds live sessions; fall back to List for record presence.
		found := false
		for _, info := range s.mgr.List() {
			if info.ID == id {
				found = true
				break
			}
		}
		if !found {
			http.Error(w, "session not found", http.StatusNotFound)
			return
		}
	}
	var req statusReq
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	switch req.State {
	case "running", "done", "requires_action", "idle":
		needs := req.NeedsReply || req.State == "requires_action"
		s.mgr.Signal(id, req.State, needs)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "unknown state", http.StatusBadRequest)
	}
}

func (s *Server) handleSeen(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mgr.MarkSeen(id)
	w.WriteHeader(http.StatusNoContent)
}
```

(The test posts and checks `StatusOK`; `204 No Content` satisfies `resp.StatusCode != http.StatusNotFound` paths — adjust the test's expected code to `http.StatusNoContent` for the success cases, or return `http.StatusOK`. Use `http.StatusNoContent` in both handler and test for consistency: change the test's `!= http.StatusOK` to `!= http.StatusNoContent`.)

- [ ] **Step 4: Register routes.** In `internal/server/server.go` `routes()`, add after the restart route:
```go
	s.mux.HandleFunc("POST /api/sessions/{id}/status", s.handleStatus)
	s.mux.HandleFunc("POST /api/sessions/{id}/seen", s.handleSeen)
```

- [ ] **Step 5: Run to verify it passes**

Run: `go test ./internal/server/ -run TestStatusAndSeen -v`
Expected: PASS (or SKIP without tmux).

- [ ] **Step 6: Commit**

```bash
git add internal/server/api.go internal/server/server.go internal/server/api_test.go
git commit -m "feat: POST /status and /seen endpoints drive Manager signals"
```

---

## Task 6: `orchid signal` subcommand + serve env wiring

**Files:** Modify `cmd/orchid/cli.go`; create/append `cmd/orchid/cli_test.go`.

- [ ] **Step 1: Write the failing test** — create `cmd/orchid/cli_test.go` (or append if it exists):

```go
package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./cmd/orchid/ -run TestSignalCmd -v`
Expected: FAIL — `signalCmd` undefined.

- [ ] **Step 3: Implement `signalCmd`.** In `cmd/orchid/cli.go`, add the import `"orchid/internal/statusclient"` to the import block, then add:

```go
// signalCmd posts an agent status signal for the current session. Invoked by the
// claude plugin hooks: `orchid signal running|done|requires_action`. Reads
// ORCHID_SESSION and ORCHID_URL from the environment (injected into the tmux
// session). Always exits 0 so a status hiccup never disrupts the agent.
func signalCmd(args []string) int {
	if len(args) < 1 {
		return 0
	}
	state := args[0]
	switch state {
	case "running", "done", "requires_action", "idle":
	default:
		return 0
	}
	id := os.Getenv("ORCHID_SESSION")
	url := os.Getenv("ORCHID_URL")
	_ = statusclient.Post(url, id, state, state == "requires_action")
	return 0
}
```

- [ ] **Step 4: Register the subcommand.** In `route()`, add cases (before `default`):
```go
	case "signal":
		return func() int { return signalCmd(rest) }, nil
	case "mcp":
		return func() int { return mcpCmd(rest) }, nil
```

Add a stub `mcpCmd` so the package compiles (Task 7 fills it in):
```go
// mcpCmd runs the stdio MCP server (implemented in Task 7).
func mcpCmd(args []string) int { return 0 }
```

- [ ] **Step 5: Wire env into the server.** In `serveCmd`, after `mgr := session.NewManager(sessionsPath())` and before `mgr.Restore()`, set the env so injected sessions can call back:
```go
	mgr := session.NewManager(sessionsPath())
	if self, err := os.Executable(); err == nil {
		mgr.SetEnv("http://"+addr, self)
	}
	if err := mgr.Restore(); err != nil {
		log.Printf("orchid: restore sessions: %v", err)
	}
```

- [ ] **Step 6: Run to verify it passes**

Run: `go build ./... && go test ./cmd/orchid/ -run TestSignalCmd -v`
Expected: build clean; PASS.

- [ ] **Step 7: Commit**

```bash
git add cmd/orchid/cli.go cmd/orchid/cli_test.go
git commit -m "feat: orchid signal subcommand + serve injects session env"
```

---

## Task 7: `orchid mcp` stdio MCP server

**Files:** Create `internal/mcp/mcp.go`, `internal/mcp/mcp_test.go`; modify `cmd/orchid/cli.go` (`mcpCmd`).

Background: MCP over stdio is JSON-RPC 2.0, one JSON object per line. We implement the minimum: `initialize`, `tools/list`, `tools/call`. The single tool `set_status` posts via `statusclient`.

- [ ] **Step 1: Write the failing test** — create `internal/mcp/mcp_test.go`:

```go
package mcp

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServeSetStatus(t *testing.T) {
	var gotState string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		gotState, _ = body["state"].(string)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	t.Setenv("ORCHID_SESSION", "s-1")
	t.Setenv("ORCHID_URL", srv.URL)

	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"set_status","arguments":{"state":"requires_action"}}}`,
	}, "\n") + "\n"

	var out strings.Builder
	if err := Serve(strings.NewReader(in), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	if gotState != "requires_action" {
		t.Fatalf("server received state %q", gotState)
	}
	// three responses, each valid JSON-RPC with matching ids
	sc := bufio.NewScanner(strings.NewReader(out.String()))
	ids := map[float64]bool{}
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var resp map[string]any
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("response not JSON: %q", line)
		}
		if id, ok := resp["id"].(float64); ok {
			ids[id] = true
		}
	}
	for _, want := range []float64{1, 2, 3} {
		if !ids[want] {
			t.Fatalf("missing response for id %v in:\n%s", want, out.String())
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/mcp/ -v`
Expected: FAIL — package/`Serve` missing.

- [ ] **Step 3: Implement.** Create `internal/mcp/mcp.go`:

```go
// Package mcp implements a minimal stdio MCP (Model Context Protocol) server
// exposing a single tool, set_status, that forwards the agent's reported state
// to a running orchid server. JSON-RPC 2.0, one message per line.
package mcp

import (
	"bufio"
	"encoding/json"
	"io"
	"os"

	"orchid/internal/statusclient"
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

const setStatusTool = `{
  "name": "set_status",
  "description": "Report this agent session's status to the orchid dashboard. Call with state=\"requires_action\" when you end your turn asking the user a question or otherwise need their input; state=\"running\" when actively working; state=\"idle\" when finished and nothing is needed.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "state": {"type": "string", "enum": ["running", "idle", "requires_action"]},
      "message": {"type": "string", "description": "Optional short note about what you need."}
    },
    "required": ["state"]
  }
}`

// Serve reads JSON-RPC requests from in and writes responses to out until EOF.
func Serve(in io.Reader, out io.Writer) error {
	enc := json.NewEncoder(out)
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			continue // ignore malformed line
		}
		resp := handle(req)
		if resp != nil {
			if err := enc.Encode(resp); err != nil {
				return err
			}
		}
	}
	return sc.Err()
}

func handle(req rpcRequest) *rpcResponse {
	switch req.Method {
	case "initialize":
		return &rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "orchid", "version": "1.0.0"},
		}}
	case "notifications/initialized":
		return nil // notification, no response
	case "tools/list":
		var tool any
		_ = json.Unmarshal([]byte(setStatusTool), &tool)
		return &rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{
			"tools": []any{tool},
		}}
	case "tools/call":
		return callTool(req)
	default:
		if len(req.ID) == 0 {
			return nil // unknown notification
		}
		return &rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32601, Message: "method not found"}}
	}
}

func callTool(req rpcRequest) *rpcResponse {
	var p struct {
		Name      string `json:"name"`
		Arguments struct {
			State   string `json:"state"`
			Message string `json:"message"`
		} `json:"arguments"`
	}
	_ = json.Unmarshal(req.Params, &p)
	text := "status not updated"
	if p.Name == "set_status" {
		switch p.Arguments.State {
		case "running", "idle", "requires_action":
			_ = statusclient.Post(os.Getenv("ORCHID_URL"), os.Getenv("ORCHID_SESSION"),
				p.Arguments.State, p.Arguments.State == "requires_action")
			text = "status set to " + p.Arguments.State
		}
	}
	return &rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{
		"content": []any{map[string]any{"type": "text", "text": text}},
	}}
}
```

- [ ] **Step 4: Wire `mcpCmd`.** In `cmd/orchid/cli.go`, add import `"orchid/internal/mcp"` and replace the `mcpCmd` stub:
```go
// mcpCmd runs the stdio MCP server used by the claude plugin's .mcp.json.
func mcpCmd(args []string) int {
	if err := mcp.Serve(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "orchid mcp:", err)
		return 1
	}
	return 0
}
```

- [ ] **Step 5: Run to verify it passes**

Run: `go build ./... && go test ./internal/mcp/ -v`
Expected: build clean; PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/mcp/ cmd/orchid/cli.go
git commit -m "feat: orchid mcp stdio server with set_status tool"
```

---

## Task 8: Claude plugin `orchid-status`

**Files:** Create the plugin tree under `plugins/orchid-status/`. (Schema verified against current Claude Code docs: manifest at `.claude-plugin/plugin.json`; `hooks/hooks.json`; `.mcp.json`; `skills/<name>/SKILL.md`. Hook commands run with `shell: bash` and inherit the Claude process env — so `ORCHID_BIN`/`ORCHID_SESSION`/`ORCHID_URL` injected into the tmux pane are visible.)

- [ ] **Step 1: Manifest.** Create `plugins/orchid-status/.claude-plugin/plugin.json`:
```json
{
  "name": "orchid-status",
  "description": "Reports claude session status (running / requires_action / idle) to the orchid dashboard via hooks and an MCP set_status tool.",
  "version": "1.0.0"
}
```

- [ ] **Step 2: Signal wrapper.** Create `plugins/orchid-status/bin/signal.sh` (no-op when not under orchid; never blocks):
```bash
#!/usr/bin/env bash
# Report a status to orchid. Args: <state>. No-op when not running under orchid.
[ -n "$ORCHID_BIN" ] && [ -n "$ORCHID_SESSION" ] && "$ORCHID_BIN" signal "$1" >/dev/null 2>&1
exit 0
```
Then make it executable:
```bash
chmod +x plugins/orchid-status/bin/signal.sh
```

- [ ] **Step 3: Hooks.** Create `plugins/orchid-status/hooks/hooks.json`:
```json
{
  "hooks": {
    "SessionStart": [
      { "matcher": "", "hooks": [
        { "type": "command", "command": "${CLAUDE_PLUGIN_ROOT}/bin/signal.sh running", "async": true, "timeout": 5 }
      ]}
    ],
    "UserPromptSubmit": [
      { "matcher": "", "hooks": [
        { "type": "command", "command": "${CLAUDE_PLUGIN_ROOT}/bin/signal.sh running", "async": true, "timeout": 5 }
      ]}
    ],
    "PreToolUse": [
      { "matcher": "", "hooks": [
        { "type": "command", "command": "${CLAUDE_PLUGIN_ROOT}/bin/signal.sh running", "async": true, "timeout": 5 }
      ]}
    ],
    "Stop": [
      { "matcher": "", "hooks": [
        { "type": "command", "command": "${CLAUDE_PLUGIN_ROOT}/bin/signal.sh done", "async": true, "timeout": 5 }
      ]}
    ],
    "Notification": [
      { "matcher": "", "hooks": [
        { "type": "command", "command": "${CLAUDE_PLUGIN_ROOT}/bin/signal.sh requires_action", "async": true, "timeout": 5 }
      ]}
    ]
  }
}
```

- [ ] **Step 4: MCP registration.** Create `plugins/orchid-status/.mcp.json`. `ORCHID_BIN` is injected into the pane env and inherited by Claude; pass it and the callback vars through to the server:
```json
{
  "mcpServers": {
    "orchid": {
      "type": "stdio",
      "command": "${ORCHID_BIN}",
      "args": ["mcp"],
      "env": {
        "ORCHID_SESSION": "${ORCHID_SESSION}",
        "ORCHID_URL": "${ORCHID_URL}"
      }
    }
  }
}
```

- [ ] **Step 5: Skill.** Create `plugins/orchid-status/skills/report-status/SKILL.md`:
```markdown
---
name: report-status
description: Use when you end a turn by asking the user a question or otherwise need their input — call set_status(requires_action) so the orchid dashboard flags this session as needing attention.
---

# Reporting status to orchid

This session runs inside the orchid dashboard. Running/idle status is tracked automatically by hooks — you do not need to report those.

**The one thing to do manually:** whenever you finish a turn by **asking the user a question** or otherwise **need their input to continue**, call the `set_status` MCP tool:

- `set_status(state="requires_action", message="<short note on what you need>")`

This makes orchid show an amber "Needs you" status until the user replies. If you are simply done with a task and need nothing, do not call it — the hooks resolve that to idle once the user reads your message.
```

- [ ] **Step 6: README.** Create `plugins/orchid-status/README.md`:
```markdown
# orchid-status plugin

Reports claude session status to the orchid dashboard.

- **Hooks** (`hooks/hooks.json`): `UserPromptSubmit`/`PreToolUse`/`SessionStart` → `running`, `Stop` → `done`, `Notification` → `requires_action`. They run `bin/signal.sh`, which calls `$ORCHID_BIN signal <state>` and no-ops when not under orchid.
- **MCP** (`.mcp.json`): registers the `orchid` server (`$ORCHID_BIN mcp`) exposing `set_status` so the agent can explicitly raise `requires_action` when it asks you a question.

orchid injects `ORCHID_SESSION`, `ORCHID_URL`, `ORCHID_BIN` into each session's tmux environment; the hooks and MCP server inherit them.

### Use

orchid loads this plugin automatically for claude sessions by launching claude with `--plugin-dir <path-to>/plugins/orchid-status` (see Task 9 / follow-up). For manual testing:

    claude --plugin-dir ./plugins/orchid-status
```

- [ ] **Step 7: Validate JSON.** Run:
```bash
for f in plugins/orchid-status/.claude-plugin/plugin.json plugins/orchid-status/hooks/hooks.json plugins/orchid-status/.mcp.json; do node -e "JSON.parse(require('fs').readFileSync('$f','utf8'))" && echo "ok $f"; done
```
Expected: `ok` for all three.

- [ ] **Step 8: Commit**
```bash
git add plugins/orchid-status/
git commit -m "feat: orchid-status claude plugin (hooks + MCP set_status)"
```

**Note on launching claude with the plugin:** for the plugin to load, the claude agent must be started with `--plugin-dir <abs path>/plugins/orchid-status`. The `claudeAgent.NewCommand`/`ResumeCommand` in `internal/agent/claude.go` currently returns `["claude", ...]`. Wiring the `--plugin-dir` flag (resolved from the orchid install location, e.g. an env var `ORCHID_PLUGIN_DIR` set alongside the others, or `claude config`/marketplace install) is a small follow-up; capture it as a beads issue if not done here. The plugin itself is complete and testable via `--plugin-dir` manually.

---

## Task 9: Frontend — 4-state pills, live poll, seen-on-open

**Files:** Modify `web/app.js`.

- [ ] **Step 1: 4-state `statusMeta`.** Replace the current `statusMeta` near the top of `web/app.js`:
```js
const statusMeta = (s) => ({
  running:         { cls: "ok",   label: "Running",    pulse: true },
  requires_action: { cls: "wait", label: "Needs you",  pulse: true },
  idle:            { cls: "idle", label: "Idle",       pulse: false },
  stopped:         { cls: "idle", label: "Stopped",    pulse: false },
}[s] || { cls: "idle", label: "Idle", pulse: false });
```

- [ ] **Step 2: Tag indicators with `data-status`.** Add a `data-status="<id>"` attribute to each status indicator so the poller patches them in place:
  - `tplSessRow`: change the status-pill opening tag to
    `'<span class="status-pill <%- st.cls %>" data-status="<%- s.id %>">'`
  - `tplRailItem`: change the trailing dot to
    `'<span class="dot <%- st.cls %><%- st.pulse ? " pulse" : "" %>" data-status="<%- s.id %>"></span>'`
  - `paneHtml` (grid): in the status-pill span add `data-status="' + s.id + '"`.
  - `focusTermChrome`: in the status-pill span add `data-status="' + s.id + '"`.
  - `gridShellHtml` chips: change each chip dot to `'<span class="dot ' + st.cls + '" data-status="' + s.id + '"></span>'`.

- [ ] **Step 3: Seen-on-open.** Add a helper and call it where a terminal is opened. Add near `openTerminal`:
```js
function markSeen(id) {
  if (!id) return;
  fetch("/api/sessions/" + id + "/seen", { method: "POST" }).catch(() => {});
}
```
In `openTerminal(id)` add `markSeen(id);` as the first line. In `mountTerminal()` after `cur.activeId` is finalized add `markSeen(cur.activeId);`. In the rail-item click handler (where `cur.activeId = rail.dataset.id`) add `markSeen(cur.activeId);`.

- [ ] **Step 4: Live poll.** Add near the bottom of `web/app.js`, before `loadShell().finally(refresh);`:
```js
async function pollStatus() {
  if (!cur.projectId) return;
  let list;
  try {
    list = (await (await fetch("/api/sessions")).json()).filter((s) => s.project === cur.projectId);
  } catch (e) {
    return; // transient; retry next tick
  }
  const a = list.map((s) => s.id).sort().join(",");
  const b = sessions.map((s) => s.id).sort().join(",");
  if (a !== b) { await refresh(); return; } // session set changed -> full re-render
  sessions = list;
  for (const s of list) {
    const st = statusMeta(s.status);
    document.querySelectorAll('[data-status="' + s.id + '"]').forEach((el) => {
      if (el.classList.contains("status-pill")) {
        el.className = "status-pill " + st.cls + (s.name ? "" : "");
        el.setAttribute("data-status", s.id);
        el.innerHTML = '<span class="dot ' + st.cls + (st.pulse ? " pulse" : "") + '"></span>' + st.label;
      } else {
        el.className = "dot " + st.cls + (st.pulse ? " pulse" : "");
        el.setAttribute("data-status", s.id);
      }
    });
  }
}
setInterval(pollStatus, 3000);
```

- [ ] **Step 5: Verify build + JS syntax.**

Run: `go build ./... && node --check web/app.js`
Expected: clean. Also confirm CSS already defines `.status-pill.wait` and `.dot.wait` (it does — used by the original design).

- [ ] **Step 6: Commit**
```bash
git add web/app.js
git commit -m "feat: 4-state status pills, live poll, seen-on-open"
```

---

## Task 10: Gate + manual verification

**Files:** none.

- [ ] **Step 1: Full suite + vet + race.**

Run: `go build ./... && go vet ./... && go test ./... -race`
Expected: all PASS (git/tmux/claude-dependent tests may SKIP). No races.

- [ ] **Step 2: Rebuild the binary.**

Run: `go build -o orchid ./cmd/orchid`
Expected: builds. (The running daemon must be restarted to pick this up — `./orchid stop; ./orchid start`.)

- [ ] **Step 3: Manual end-to-end (shell).** Start orchid, open the dashboard, create a `shell` session. Its pill shows **Running** briefly then **Idle** (~3s). No plugin involved — confirms the activity fallback.

- [ ] **Step 4: Manual end-to-end (claude + plugin).** Ensure claude is launched with the plugin (`--plugin-dir <repo>/plugins/orchid-status`; via the follow-up wiring or by temporarily pointing `claudeAgent` at it). Create a claude session:
  - While it works/runs tools → **Running**.
  - Ask it something that makes it end its turn with a question; it calls `set_status(requires_action)` (per the skill) → pill turns amber **Needs you** and stays until you reply.
  - When it finishes a task needing nothing and you open its terminal → **Idle**.
  - The live terminal is NOT reloaded across pill changes (poll patches in place).

- [ ] **Step 5: No leftover sessions.**

Run: `tmux ls 2>/dev/null | grep '^orchid-' && echo CHECK || echo clean`
Expected: only sessions you intend to keep; clean up test sessions + tmux.

- [ ] **Step 6: File follow-up.** If `--plugin-dir` wiring into `claudeAgent` was deferred (Task 8 note), create a beads issue: "Launch claude with orchid-status plugin (--plugin-dir / marketplace) so status signals work without manual flags."

---

## Self-Review Notes

- **Spec coverage:** activity fallback (T1); env transport (T2, T6); signal state + read-gated derivation incl. the exact transition table — running clears requires_action, done+unread=requires_action, done+read=idle, requires_action sticky (T3); HTTP forwarding helper (T4); `/status`+`/seen` endpoints (T5); `orchid signal` + serve env wiring (T6); `orchid mcp` set_status (T7); plugin manifest/hooks/.mcp.json/skill/wrapper (T8); 4-state pills + in-place poll + seen-on-open (T9); gate + manual incl. plugin (T10). Out-of-scope items (no persistence, codex via fallback, loopback trust) honored.
- **Type consistency:** `Session.lastOutput`/`Active(within)`; `tmux.NewSession(name, cwd, env, argv)` (callers updated T2); `Manager.signals map[string]*signalState{state,needsReply,read}`, `Signal(id,state,needsReply)`, `MarkSeen(id)`, `SetEnv(selfURL,binPath)`, `sessionEnv(id)`, `resolveStatus(...)`; `statusclient.Post(baseURL,id,state,needsReply)`; server `statusReq{State,NeedsReply}`, `handleStatus`/`handleSeen`, routes `POST /status`+`/seen`; `signalCmd`/`mcpCmd`; `mcp.Serve(in,out)` + `set_status`; frontend `statusMeta` keys ∈ {running,requires_action,idle,stopped} match `Info.Status`; `data-status` attribute consistent across templates and `pollStatus`; `markSeen(id)`.
- **Known soft spots (flagged, not silent):** `--plugin-dir` wiring into `claudeAgent` is deferred to a follow-up (T8 note, T10 step 6) — the plugin is complete and manually testable. `Notification` fires on more than permission (incl. idle-60s); we still map it to requires_action, but the primary requires_action signal is the explicit `set_status` call (per the design decision), so over-eager Notification only nudges toward "needs you", which a subsequent `running` (user reply) clears. The status `204` vs `200` mismatch in T5 is called out inline (use `204` in both handler and test).
```
