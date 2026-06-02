# Orchid Agent Status Signals — Design

**Date:** 2026-06-01
**Supersedes the approach in:** `2026-05-30-orchid-session-activity-status-design.md` (whose postmortem proved passive inference cannot distinguish `idle` from `requires_action`).

## Problem

Orchid sessions show only `running | stopped`. We want a live, reliable activity status for agent sessions — **running**, **requires_action** (the agent needs you), **idle**, **stopped** — so the dashboard surfaces which sessions are waiting on the user.

The prior attempt inferred status from claude's JSONL transcript and screen. The postmortem found this **infeasible**: under auto-approve claude rarely blocks on permission, and "agent asked you a question" is structurally identical to "agent finished and is idle". **Conclusion: the agent must actively report its state.**

## Approach (chosen): hybrid hooks + MCP

The agent reports state through two channels, combined server-side:

1. **Claude Code hooks** (deterministic, event-driven) — a bundled plugin runs a command on lifecycle events:
   - `UserPromptSubmit`, `PreToolUse`, `SessionStart` → **running**
   - `Stop` (turn finished) → **done** (resolves to **idle** — finishing a turn is not "needs you")
   - `Notification` matched by `notification_type`:
     - `idle_prompt` → **idle**
     - everything else (`permission_prompt`, `auth_success`, `elicitation_*`, matched via `^(?!idle_prompt$).*`) → **requires_action**
2. **MCP tool `set_status`** (explicit) — the agent calls it to assert `requires_action` when it has asked the user a question, or to override (`running`/`idle`). A bundled skill instructs the agent to call `set_status("requires_action", message)` whenever it ends a turn asking the user something.

Why hybrid: hooks give reliable, model-independent running/idle; the MCP tool covers the one thing hooks cannot detect under auto-approve — "I asked you a question and need an answer". `requires_action` is raised only by an explicit `set_status` call or a non-idle `Notification`; nothing escalates a plain finished turn to "needs you".

Shell sessions have no plugin; their status stays derived from output activity (`lastOutput` → working/idle), which the postmortem confirmed works cheaply and reliably.

## State model & transitions

Visible statuses in `Info.Status`: `running | requires_action | idle | stopped`.

Per live session the Manager keeps an in-memory **signal record** (not persisted) — just the latest reported state:

```
signalState struct {
  state string // "running" | "requires_action" | "idle" | "done"
}
```

Transition rules (a signal replaces the stored state):

| Event (source) | state |
|---|---|
| `running` (hook: UserPromptSubmit / PreToolUse / SessionStart) | running |
| `done` (hook: Stop) | done |
| `requires_action` (MCP set_status, or non-idle Notification) | requires_action |
| `idle` (MCP set_status, or Notification `idle_prompt`) | idle |

`List` resolves the final status for each session:

1. process dead → **stopped**
2. shell agent (or no signal yet) → activity-based: `lastOutput` within window → **running**, else **idle**
3. state == running → **running**
4. state == requires_action → **requires_action** (sticky; cleared only by a subsequent `running` signal — the user replied → UserPromptSubmit fires)
5. state == done or idle → **idle**

`requires_action` (amber "Needs you") means the agent explicitly asked for input. A plain finished turn (`done`) resolves to `idle`; there is **no** read-gating / unread-attention state. (An earlier revision escalated unread `done` to `requires_action`; dropped because it flagged sessions with no actual question.)

## Components

### 1. Backend (Go)

- **`internal/session/session.go`** — add `lastOutput time.Time` + `Active(within)` (from the prior plan; cheap activity for shell + pre-signal fallback). Seed at create/attach, set in `readLoop`.
- **`internal/session/manager.go`** — `signals map[string]*signalState`; method `Signal(id, state)`; `List` derives status per the rules above (snapshot under lock, compute outside via `resolveStatus`). `SetEnv(selfURL, binPath)` configures env injection; `PruneOrphanTmux()` (startup) kills `orchid-` tmux sessions absent from records.
- **`internal/tmux/tmux.go`** — `NewSession` injects env via `-e`: `ORCHID_SESSION`, `ORCHID_URL`, `ORCHID_BIN`. (tmux supports repeated `-e VAR=val`.) `ListSessions()` lists default-server session names.
- **`internal/server/api.go` + `server.go`** — `POST /api/sessions/{id}/status` `{state}` → `mgr.Signal`.
- **`cmd/orchid`** — two subcommands:
  - `orchid signal <running|done|requires_action|idle>` — reads `ORCHID_SESSION`/`ORCHID_URL` from env, POSTs to `/status`. No curl dependency. Silent, fast, never blocks the agent.
  - `orchid mcp` — stdio MCP server (below).

### 2. MCP server — `orchid mcp`

Minimal stdio JSON-RPC 2.0 implementing MCP: `initialize`, `tools/list`, `tools/call`. One tool:

- **`set_status`** — params `{ "state": "running"|"idle"|"requires_action", "message"?: string }`. On call, POSTs `{state}` to `$ORCHID_URL/api/sessions/$ORCHID_SESSION/status`. Returns a short text confirmation. Reads `ORCHID_SESSION`/`ORCHID_URL` from env (inherited from the tmux pane).

Implemented in `internal/mcp` (pure stdlib JSON-RPC over stdin/stdout — no third-party MCP SDK needed for a one-tool server).

### 3. Claude plugin — `plugins/orchid-status/`

A Claude Code plugin directory bundling:

- **`.claude-plugin/plugin.json`** — manifest (name, version, description).
- **`hooks/hooks.json`** — maps events to `${CLAUDE_PLUGIN_ROOT}`-relative or `$ORCHID_BIN` commands:
  - `UserPromptSubmit`, `PreToolUse`, `SessionStart` → `orchid signal running`
  - `Stop` → `orchid signal done`
  - `Notification` matcher `idle_prompt` → `orchid signal idle`
  - `Notification` matcher `^(?!idle_prompt$).*` (everything else) → `orchid signal requires_action`
- **`.mcp.json`** — registers the orchid MCP server: command `orchid mcp` (or `$ORCHID_BIN mcp`).
- **`skills/report-status/SKILL.md`** — instructs the agent: when you end a turn by asking the user a question or otherwise need their input, call the `set_status` tool with `requires_action`; when you resume work, you don't need to call it (the hooks handle running). Keep it short; the hooks do the heavy lifting.

The hooks call `$ORCHID_BIN signal …`; `ORCHID_BIN` is the absolute path to the running orchid binary, injected into the session env so the plugin works without orchid being on `$PATH`.

## Frontend

- **4-state `statusMeta`** in `web/app.js`: `running`→ok "Running", `requires_action`→wait "Needs you", `idle`→idle "Idle", `stopped`→idle "Stopped".
- **Live poll** — a ~3s `GET /api/sessions` that patches status pills/dots **in place** via `data-status="<id>"` without tearing down live terminals; full re-render only when the session set changes.
- Attachability is gated on `Info.alive`, not on `status == "running"` (idle/requires_action sessions are alive and openable).

## Error handling

- `orchid signal` / MCP forwarding: best-effort. Network/JSON errors are swallowed (exit 0) so a status hiccup never blocks or fails the agent.
- Missing env (`ORCHID_SESSION` unset, e.g. agent run outside orchid): `signal`/`set_status` no-op silently.
- Unknown `{id}` on `/status`: 404, ignored by caller.
- Unknown `state` value: 400; the signal is ignored.

## Testing

- **session:** `Active(window)`.
- **manager:** `Signal` + `List` status resolution (running; requires_action sticky, cleared by running; done→idle; idle; dead=stopped; shell activity fallback). `orphanSessions` pure logic.
- **mcp:** JSON-RPC `initialize`/`tools/list`/`tools/call set_status` round-trip against a stub HTTP server; env-driven forwarding; missing-env no-op.
- **cmd:** `orchid signal` posts the right body; missing env exits 0 without posting.
- **server:** `/status` handler updates Manager; bad input handled.
- **frontend:** build + `node --check`; manual/CDP for live pill flips without terminal reload.
- **plugin:** schema-validate `plugin.json`/`hooks.json`/`.mcp.json` shape against current Claude Code plugin docs.

## Out of scope / YAGNI

- No persistence of signal status (`store.Record.Status` stays `running|stopped`; signals are ephemeral, rebuilt as sessions report).
- No per-message history of statuses; only the latest.
- codex: no first-party hook integration in this iteration (codex sessions fall back to activity-based working/idle, same as shell). The plugin targets claude.
- No auth on the local status endpoints (loopback-only, same trust model as the existing API).

## Notes (resolved during implementation)

- Claude Code **plugin schema** confirmed: manifest `.claude-plugin/plugin.json`; `hooks/hooks.json`; root `.mcp.json`; `skills/<name>/SKILL.md`. Hook commands run with `shell: bash` and inherit the Claude process env, so the tmux-injected `ORCHID_*` vars reach them. `Notification` `matcher` filters on `notification_type` and supports JS regex (negative lookahead).
- Plugin launch: `claudeAgent` appends `--plugin-dir $ORCHID_PLUGIN_DIR` when set; `serveCmd` auto-discovers `<binary-dir>/plugins/orchid-status`. The plugin is loaded per orchid-launched session — nothing is installed into the user's global claude config.
