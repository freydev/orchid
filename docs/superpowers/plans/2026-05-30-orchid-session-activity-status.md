# Orchid Session Activity Status Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Report a live activity status for running sessions — `working | idle | requires_action` (plus `stopped`) — from the control-stream's output timestamp (working/idle) and, for claude only, its JSONL transcript tail (requires_action).

**Architecture:** `Session` records a `lastOutput` timestamp in its read loop (free working/idle signal). The `agent.Agent` interface gains `PendingApproval(cwd, agentSessionID)`; claude implements it by locating its `~/.claude/projects/*/<uuid>.jsonl` transcript and checking whether the last message is a tool call with no result. `Manager.List` derives `Info.Status` (dead→stopped; recent output→working; else PendingApproval→requires_action else idle). The frontend maps the 4 states to status pills and patches them in place via a ~3s poll without disturbing live terminals.

**Tech Stack:** Go 1.26 stdlib (`encoding/json`, `os`, `path/filepath`, `time`), existing session/agent packages; vanilla JS frontend.

---

## File Structure

```
internal/session/session.go    # MODIFIED: lastOutput field + Active(); set in readLoop + at creation
internal/session/session_test.go  # + Active test
internal/agent/agent.go        # MODIFIED: PendingApproval on the Agent interface
internal/agent/claude.go       # MODIFIED: PendingApproval (JSONL)
internal/agent/claude_jsonl.go # NEW: transcript locate + tail parse
internal/agent/codex.go        # MODIFIED: PendingApproval -> false
internal/agent/shell.go        # MODIFIED: PendingApproval -> false
internal/agent/claude_jsonl_test.go  # NEW: PendingApproval fixtures
internal/session/manager.go    # MODIFIED: List computes activity status
internal/session/manager_test.go  # + status-resolution test
web/app.js                     # MODIFIED: 4-state statusMeta + data-status + pollStatus
```

---

## Task 1: Session activity timestamp (`lastOutput` + `Active`)

**Files:** Modify `internal/session/session.go`, `internal/session/session_test.go`.

- [ ] **Step 1: Write the failing test** — append to `internal/session/session_test.go`:
```go
func TestSessionActive(t *testing.T) {
	s := newTestSession(t, "active1")
	// fresh session is seeded active
	if !s.Active(3 * time.Second) {
		t.Fatal("fresh session should be active")
	}
	// after writing, still active
	_, _ = s.Write([]byte("echo hi\n"))
	time.Sleep(300 * time.Millisecond)
	if !s.Active(3 * time.Second) {
		t.Fatal("session should be active right after output")
	}
	// with a tiny window it should read inactive once quiet
	time.Sleep(200 * time.Millisecond)
	if s.Active(50 * time.Millisecond) {
		t.Fatal("session should be inactive beyond the window")
	}
}
```
(`newTestSession` and the `time` import already exist in the file.)

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/session/ -run TestSessionActive -v`
Expected: FAIL — `s.Active undefined`.

- [ ] **Step 3: Implement.** In `internal/session/session.go`:

(a) Add the field to the `Session` struct (after `doneOnce sync.Once` or alongside `alive`):
```go
	lastOutput time.Time
```
Add `"time"` to the imports if not present.

(b) Seed it at creation in BOTH `createSession` and `attachSession` — in the `&Session{...}` literal add:
```go
		lastOutput:     time.Now(),
```

(c) In `readLoop`, record output time on each chunk:
```go
func (s *Session) readLoop() {
	for data := range s.ctl.Output() {
		s.mu.Lock()
		s.lastOutput = time.Now()
		s.mu.Unlock()
		s.broadcast(data)
	}
	// ... unchanged tail (alive=false, closeOnce, onExit) ...
}
```

(d) Add the accessor:
```go
// Active reports whether output was produced within the given window.
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
git add internal/session/session.go internal/session/session_test.go
git commit -m "feat: track session output activity (lastOutput + Active)"
```

---

## Task 2: claude PendingApproval via JSONL

**Files:** Modify `internal/agent/agent.go`, `internal/agent/claude.go`, `internal/agent/codex.go`, `internal/agent/shell.go`; create `internal/agent/claude_jsonl.go`, `internal/agent/claude_jsonl_test.go`.

- [ ] **Step 1: Write the failing test** — create `internal/agent/claude_jsonl_test.go`:
```go
package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTranscript(t *testing.T, uuid, body string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".claude", "projects", "some-proj")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, uuid+".jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestClaudePendingApproval(t *testing.T) {
	c := claudeAgent{}

	// trailing assistant tool_use with no result -> pending
	writeTranscript(t, "u1",
		`{"type":"user","message":{"role":"user","content":[{"type":"text"}]}}`+"\n"+
		`{"type":"assistant","message":{"role":"assistant","stop_reason":"tool_use","content":[{"type":"tool_use"}]}}`+"\n"+
		`{"type":"permission-mode"}`+"\n")
	if !c.PendingApproval("/x", "u1") {
		t.Fatal("trailing tool_use without result should be pending")
	}

	// trailing assistant end_turn -> not pending
	writeTranscript(t, "u2",
		`{"type":"assistant","message":{"role":"assistant","stop_reason":"tool_use","content":[{"type":"tool_use"}]}}`+"\n"+
		`{"type":"user","message":{"role":"user","content":[{"type":"tool_result"}]}}`+"\n"+
		`{"type":"assistant","message":{"role":"assistant","stop_reason":"end_turn","content":[{"type":"text"}]}}`+"\n")
	if c.PendingApproval("/x", "u2") {
		t.Fatal("trailing end_turn should not be pending")
	}

	// trailing tool_result -> not pending
	writeTranscript(t, "u3",
		`{"type":"assistant","message":{"role":"assistant","stop_reason":"tool_use","content":[{"type":"tool_use"}]}}`+"\n"+
		`{"type":"user","message":{"role":"user","content":[{"type":"tool_result"}]}}`+"\n")
	if c.PendingApproval("/x", "u3") {
		t.Fatal("trailing tool_result should not be pending")
	}

	// missing file -> not pending
	t.Setenv("HOME", t.TempDir())
	if c.PendingApproval("/x", "nope") {
		t.Fatal("missing transcript should not be pending")
	}
}

func TestNonClaudePendingApproval(t *testing.T) {
	if codexAgent{}.PendingApproval("/x", "u1") {
		t.Fatal("codex should never be pending")
	}
	if (shellAgent{}).PendingApproval("/x", "u1") {
		t.Fatal("shell should never be pending")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/agent/ -run PendingApproval -v`
Expected: FAIL — `c.PendingApproval undefined` / interface not satisfied.

- [ ] **Step 3: Add `PendingApproval` to the interface.** In `internal/agent/agent.go`, add to the `Agent` interface:
```go
	// PendingApproval reports whether the agent is blocked waiting for user
	// approval/input, inferred from its transcript. Agents without a detectable
	// signal return false.
	PendingApproval(cwd, agentSessionID string) bool
```

- [ ] **Step 4: Implement the claude detector.** Create `internal/agent/claude_jsonl.go`:
```go
package agent

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
)

// findClaudeTranscript locates ~/.claude/projects/*/<id>.jsonl (uuid is unique).
func findClaudeTranscript(agentSessionID string) string {
	home, err := os.UserHomeDir()
	if err != nil || agentSessionID == "" {
		return ""
	}
	m, _ := filepath.Glob(filepath.Join(home, ".claude", "projects", "*", agentSessionID+".jsonl"))
	if len(m) > 0 {
		return m[0]
	}
	return ""
}

// tailBytes returns up to the last n bytes of the file.
func tailBytes(path string, n int64) []byte {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil
	}
	off := int64(0)
	if st.Size() > n {
		off = st.Size() - n
	}
	buf := make([]byte, st.Size()-off)
	if _, err := f.ReadAt(buf, off); err != nil {
		return nil
	}
	return buf
}

type claudeEvent struct {
	Type    string `json:"type"`
	Message *struct {
		Role       string `json:"role"`
		StopReason string `json:"stop_reason"`
		Content    []struct {
			Type string `json:"type"`
		} `json:"content"`
	} `json:"message"`
}

// claudePending reports whether the transcript ends on an assistant tool call
// that has no following tool_result (i.e. blocked awaiting approval).
func claudePending(agentSessionID string) bool {
	path := findClaudeTranscript(agentSessionID)
	if path == "" {
		return false
	}
	data := tailBytes(path, 64*1024)
	lines := bytes.Split(data, []byte("\n"))
	for i := len(lines) - 1; i >= 0; i-- {
		line := bytes.TrimSpace(lines[i])
		if len(line) == 0 {
			continue
		}
		var ev claudeEvent
		if json.Unmarshal(line, &ev) != nil {
			continue // partial leading line or non-JSON control line
		}
		if (ev.Type != "user" && ev.Type != "assistant") || ev.Message == nil {
			continue // skip control events (mode/permission-mode/ai-title/...)
		}
		// last conversation message found
		if ev.Type == "assistant" && ev.Message.StopReason == "tool_use" {
			for _, c := range ev.Message.Content {
				if c.Type == "tool_use" {
					return true
				}
			}
		}
		return false
	}
	return false
}
```

- [ ] **Step 5: Wire the methods.** In `internal/agent/claude.go` add:
```go
func (claudeAgent) PendingApproval(cwd, agentSessionID string) bool {
	return claudePending(agentSessionID)
}
```
In `internal/agent/codex.go` add:
```go
func (codexAgent) PendingApproval(cwd, agentSessionID string) bool { return false }
```
In `internal/agent/shell.go` add:
```go
func (shellAgent) PendingApproval(cwd, agentSessionID string) bool { return false }
```

- [ ] **Step 6: Run to verify it passes**

Run: `go test ./internal/agent/ -v`
Expected: PASS (all agent tests incl. the new PendingApproval ones).

- [ ] **Step 7: Commit**

```bash
git add internal/agent/
git commit -m "feat: claude PendingApproval via JSONL transcript tail"
```

---

## Task 3: Manager derives activity status in List

**Files:** Modify `internal/session/manager.go`, `internal/session/manager_test.go`.

- [ ] **Step 1: Write the failing test** — append to `internal/session/manager_test.go`:
```go
func TestListActivityStatus(t *testing.T) {
	skipNoTmux(t)
	m := newTestManager(t)
	s, err := m.Create("p-1", "/tmp", "shell")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer tmux.KillSession(s.Tmux)

	// fresh shell session: working (recent startup output) then idle
	if st := m.List()[0].Status; st != "working" && st != "idle" {
		t.Fatalf("unexpected initial status %q", st)
	}
	// wait past the idle window with no input -> idle (shell never requires_action)
	time.Sleep(3500 * time.Millisecond)
	if st := m.List()[0].Status; st != "idle" {
		t.Fatalf("status = %q, want idle", st)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/session/ -run TestListActivityStatus -v`
Expected: FAIL — status is the persisted `running`, not `idle`/`working`.

- [ ] **Step 3: Implement.** In `internal/session/manager.go`:

(a) Add the import `"orchid/internal/agent"` (already imported) and a const near the top:
```go
const idleAfter = 3 * time.Second
```

(b) Replace `List` to snapshot under the lock, then derive status outside it (so the per-session transcript read doesn't hold `m.mu`):
```go
func (m *Manager) List() []Info {
	type ent struct {
		rec store.Record
		s   *Session
	}
	m.mu.Lock()
	ents := make([]ent, 0, len(m.records))
	for _, r := range m.records {
		ents = append(ents, ent{r, m.sessions[r.ID]})
	}
	m.mu.Unlock()

	out := make([]Info, 0, len(ents))
	for _, e := range ents {
		status := "stopped"
		if e.s != nil {
			if e.s.Active(idleAfter) {
				status = "working"
			} else if ag, ok := agent.Get(e.rec.Agent); ok && ag.PendingApproval(e.rec.Cwd, e.rec.AgentSessionID) {
				status = "requires_action"
			} else {
				status = "idle"
			}
		}
		out = append(out, Info{
			ID:       e.rec.ID,
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
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/session/ -run 'TestList|TestManager|TestKill|TestSelfExit|TestRestore' -race -v`
Expected: PASS. No races. (Note: `TestSelfExitMarksStopped` asserts a *stopped* record after the shell exits — still valid: a dead session resolves to `stopped`.)

- [ ] **Step 5: Commit**

```bash
git add internal/session/manager.go internal/session/manager_test.go
git commit -m "feat: derive working/idle/requires_action status in List"
```

---

## Task 4: Frontend — 4-state pills + live status poll

**Files:** Modify `web/app.js`.

- [ ] **Step 1: 4-state `statusMeta`.** Replace the `statusMeta` definition near the top of `web/app.js`:
```js
const statusMeta = (s) => ({
  working:         { cls: "ok",   label: "Running",     pulse: true },
  requires_action: { cls: "wait", label: "Needs input", pulse: true },
  idle:            { cls: "idle", label: "Idle",        pulse: false },
  stopped:         { cls: "idle", label: "Stopped",     pulse: false },
}[s] || { cls: "idle", label: "Idle", pulse: false });
```

- [ ] **Step 2: Tag status dots/pills with `data-status`.** In the templates and chrome builders, add a `data-status="<id>"` attribute to each status indicator so the poller can find them:
  - `tplSessRow` — change the status-pill span opening tag to include the id:
    `'<span class="status-pill <%- st.cls %>" data-status="<%- s.id %>">'`
  - `tplRailItem` — change the trailing dot to:
    `'<span class="dot <%- st.cls %><%- st.pulse ? " pulse" : "" %>" data-status="<%- s.id %>"></span>'`
  - `paneHtml` (grid) — change the status-pill span to include `data-status="' + s.id + '"`.
  - `focusTermChrome` — change the status-pill span to include `data-status="' + s.id + '"`.
  - `gridShellHtml` chips — change each chip's dot to `'<span class="dot ' + st.cls + '" data-status="' + s.id + '"></span>'`.

  (Each is a one-attribute addition to the existing element; keep everything else identical.)

- [ ] **Step 3: Add `pollStatus()` + interval.** Add near the bottom of `web/app.js`, before the final `refresh();`:
```js
async function pollStatus() {
  if (!cur.projectId) return;
  let list;
  try {
    list = (await (await fetch("/api/sessions")).json()).filter((s) => s.project === cur.projectId);
  } catch (e) {
    return; // server unreachable this tick; retry next
  }
  const a = list.map((s) => s.id).sort().join(",");
  const b = sessions.map((s) => s.id).sort().join(",");
  if (a !== b) { await refresh(); return; } // session set changed -> full re-render
  sessions = list;
  for (const s of list) {
    const st = statusMeta(s.status);
    document.querySelectorAll('[data-status="' + s.id + '"]').forEach((el) => {
      if (el.classList.contains("status-pill")) {
        el.className = "status-pill " + st.cls;
        el.innerHTML = '<span class="dot ' + st.cls + (st.pulse ? " pulse" : "") + '"></span>' + st.label;
      } else {
        el.className = "dot " + st.cls + (st.pulse ? " pulse" : "");
      }
    });
  }
}
setInterval(pollStatus, 3000);
```

- [ ] **Step 4: Verify build + JS syntax**

Run: `go build ./... && node --check web/app.js`
Expected: clean.

- [ ] **Step 5: Commit**

```bash
git add web/app.js
git commit -m "feat: 4-state status pills + live in-place status poll"
```

---

## Task 5: Gate + verification

**Files:** none.

- [ ] **Step 1: Full suite**

Run: `go build ./... && go vet ./... && go test ./... -race`
Expected: all PASS (or git/tmux/claude tests skip). No races.

- [ ] **Step 2: Manual / CDP**

Run orchid, add this repo as a project, create a `shell` session: in the dashboard its pill shows **Running** briefly then **Idle** (~3s). If `claude` is installed, create a claude session and trigger a permission prompt (e.g. ask it to run a command needing approval) — within ~3s the pill flips to amber **Needs input**, and the live terminal is NOT reloaded. CDP screenshot of a list with mixed dots is sufficient evidence; clean up sessions + tmux + config after.

- [ ] **Step 3: No leftover**

Run: `tmux ls 2>/dev/null | grep '^orchid-' && echo LEAK || echo clean`
Expected: `clean`.

---

## Self-Review Notes

- **Spec coverage:** lastOutput + Active (T1); claude PendingApproval via JSONL locate/tail/parse, codex+shell false, interface method (T2); List derives working/idle/requires_action/stopped, idleAfter, snapshot-then-compute (T3); 4-state statusMeta + data-status tagging + in-place pollStatus + 3s interval + set-change fallback to refresh (T4); gate (T5). All spec sections mapped.
- **Cost:** activity is an in-memory timestamp; `PendingApproval` (glob + 64 KB tail read) runs only for **quiet, live** sessions in `List`, and only claude actually reads a file — codex/shell return immediately. Frontend poll is one `GET` per 3s while a project is open.
- **Type consistency:** `Session.lastOutput`/`Active(within)`; `agent.PendingApproval(cwd, agentSessionID)` on all three agents + interface; `claudePending`/`findClaudeTranscript`/`tailBytes`/`claudeEvent` in claude_jsonl.go; `Info.Status` ∈ {working,idle,requires_action,stopped}; frontend `statusMeta` keys match exactly; `data-status` attribute consistent across templates and `pollStatus`.
- **Persistence unchanged:** `store.Record.Status` stays `running|stopped`; activity status is `Info`-only (derived).
- **Verification deviation:** requires_action is best-effort (structural JSONL heuristic) and lacks an automated end-to-end test against a real blocked claude (would need a live permission prompt) — covered by unit fixtures (T2) + manual (T5). Flagged so it's not a silent gap.
```
