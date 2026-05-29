# Orchid — Session Activity Status (Milestone 8)

**Date:** 2026-05-30
**Status:** Approved design

## Context

Session status today is binary: `running` (tmux alive) / `stopped` (dead). This
milestone adds a live **activity status** for running sessions —
`working | idle | requires_action` — so the dashboard shows which agents are busy,
idle, or blocked waiting for the user. Implements deferred-spec #5 (the "waiting
for input" status), agent-aware.

Decisions from brainstorming:
- **working vs idle** comes from output activity, read from the control client's
  in-memory `lastOutput` timestamp (free — no polling, no forks, works for every
  agent incl. shell).
- **requires_action** is detected only for **claude**, from its JSONL transcript
  (we set `--session-id <uuid>`, so the file is locatable). codex and shell never
  report requires_action this milestone (codex's transcript id is self-generated
  and hard to locate — deferred).
- Cost scales: the transcript tail-read happens only for **quiet claude** sessions
  (cheap `os.ReadFile` of a bounded tail, no forks), so many idle sessions are
  free.

## Status model

A session's reported status (in `session.Info`, derived — never persisted):

| status            | meaning                                   | when |
|-------------------|-------------------------------------------|------|
| `working`         | producing output recently                 | live & `now − lastOutput < IDLE_AFTER` (~3s) |
| `requires_action` | blocked waiting for user (claude only)     | live & quiet & `agent.PendingApproval` true |
| `idle`            | alive, quiet, not blocked                  | live & quiet & not requires_action |
| `stopped`         | tmux session dead                          | not in the running map |

`store.Record.Status` stays `running|stopped` (persistence unchanged); the
activity status is computed only when building `Info`.

## Components

### internal/session/session.go
- `Session` gains `lastOutput time.Time` (guarded by the existing `mu`).
- In `readLoop`, on every output chunk: set `lastOutput = time.Now()` under `mu`
  (before/with `broadcast`).
- `func (s *Session) Active(within time.Duration) bool` → `time.Since(lastOutput) < within`
  (reads under `mu`).

### internal/agent
- Interface gains `PendingApproval(cwd, agentSessionID string) bool`.
- `claudeAgent.PendingApproval`:
  - Locate the transcript: glob `~/.claude/projects/*/<agentSessionID>.jsonl`
    (uuid is unique); the resolved path may be cached per agentSessionID.
  - Read the **tail** (last ~64 KB), split into lines, JSON-decode each; find the
    last line whose `type` is `user` or `assistant` carrying a `message`.
  - Return **true** iff that last message event is `assistant` with
    `message.stop_reason == "tool_use"` and a `content` block of type `tool_use`,
    i.e. a tool call with no following `tool_result` (a `user`/`tool_result` line
    after it would be the latest event instead). Missing/unreadable file → false.
- `codexAgent.PendingApproval` / `shellAgent.PendingApproval` → `false`.

### internal/session/manager.go
- `const idleAfter = 3 * time.Second`.
- `List` computes `Info.Status` per record: dead (not in `sessions`) → `stopped`;
  else `s.Active(idleAfter)` → `working`; else
  `agent.Get(rec.Agent).PendingApproval(rec.Cwd, rec.AgentSessionID)` →
  `requires_action`; else `idle`. (`Info.Status` is the only thing that changes;
  `Alive` still means "in the running map".)

No background poller, no new tmux/git invocations. Status is derived lazily on
each `List` (driven by the frontend's poll), bounded to quiet-claude tail reads.

## Frontend (web/app.js)

- `statusMeta(s)` becomes 4-state → design palette:
  - `working` → `{cls:"ok", label:"Running", pulse:true}`
  - `requires_action` → `{cls:"wait", label:"Needs input", pulse:true}`
  - `idle` → `{cls:"idle", label:"Idle", pulse:false}`
  - `stopped` → `{cls:"idle", label:"Stopped", pulse:false}`
  (`.dot.wait` / `.status-pill.wait` already exist in the vendored CSS.)
- Status dots/pills get a `data-status="<sessionId>"` marker.
- **Live poll:** while a project is open, a `setInterval(~3000ms)` runs
  `pollStatus()` → `GET /api/sessions` (filter to the project). For each session,
  patch the existing `data-status` dot/pill **in place** (rewrite class + pill
  label) across rendered rows / rail items / grid-pane heads / grid chips.
  It does **not** teardown terminals or re-render. If the set of session ids
  changed (added/removed), fall back to `refresh()`. The interval is cleared when
  returning to the projects list / no project open.

So a claude session flipping to `requires_action` shows an amber pulsing dot
within ~3s with no terminal disturbance.

## Errors & edge cases

- claude transcript missing / mid-rotation / parse error → `PendingApproval` false
  (degrades to `idle`).
- The "no `tool_result` after the trailing `tool_use`" check is structural: an
  auto-approved tool produces a `tool_result` quickly (so the session is `working`,
  not quiet, and this path isn't reached); a human-blocked tool call stays quiet on
  a trailing `tool_use` → `requires_action`. Best-effort; may misclassify exotic
  permission modes (acceptable).
- A session with no output yet (just started) has a zero `lastOutput`; treat a
  zero timestamp as "active" (working) for the first `idleAfter` window, or seed
  `lastOutput` at creation so a brand-new session reads `working` then settles.
- Poll while offline / server gone → fetch fails → skip this tick (no crash);
  next tick retries.

## Testing

- **agent (unit):** `claudeAgent.PendingApproval` against `.jsonl` fixtures under a
  temp `HOME` (`t.Setenv("HOME", …)` + write `~/.claude/projects/p/<uuid>.jsonl`):
  trailing `assistant stop_reason=tool_use`+`tool_use` block → true; trailing
  `assistant end_turn` → false; trailing `user tool_result` → false; missing file
  → false. codex/shell → false.
- **session (unit + integration):** `Active(d)` true immediately after a simulated
  output, false after the window; real tmux+shell: fresh session `working`, then
  `idle` after `idleAfter`.
- **manager (integration):** `List` resolution — dead→stopped; recent-output→
  working; quiet shell→idle; quiet claude with a pending-tool fixture (HOME → a
  fixture dir, matching the session's `AgentSessionID`)→requires_action.
- **frontend:** `node --check`; CDP screenshot of a list with mixed status dots;
  manual — park a claude session on a permission prompt, confirm the pill flips to
  amber "Needs input" within ~3s without the terminal reloading.
- All concurrency under `-race`; tmux/claude-dependent tests skip when unavailable.

## File layout (changes)

```
internal/session/session.go    # MODIFIED: lastOutput + Active()
internal/agent/agent.go        # MODIFIED: PendingApproval on the interface
internal/agent/claude.go       # MODIFIED: PendingApproval via JSONL tail
internal/agent/codex.go        # MODIFIED: PendingApproval -> false
internal/agent/shell.go        # MODIFIED: PendingApproval -> false
internal/agent/claude_jsonl.go # NEW (optional): transcript locate + tail parse helper
internal/agent/*_test.go       # tests for PendingApproval
internal/session/manager.go    # MODIFIED: Info.Status resolution (working/idle/requires_action/stopped)
internal/session/*_test.go     # status resolution + Active tests
web/app.js                     # MODIFIED: 4-state statusMeta + in-place status poll
```
