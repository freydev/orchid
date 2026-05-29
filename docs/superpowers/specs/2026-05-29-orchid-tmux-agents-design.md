# Orchid — tmux-backed Agent Sessions (Milestone 2)

**Date:** 2026-05-29
**Status:** Approved design

## Context

Milestone 1 built a Go server that spawns PTY shell sessions and renders them in
a browser via xterm.js. Sessions were bare `$SHELL` processes owned directly by a
per-session PTY, held in memory only.

Milestone 2 turns each session into an **agent session backed by tmux**:

- An **Agent** abstraction (inheritable wrapper) describes how to launch a given
  coding agent. First implementations: `claude`, `codex`, and `shell`.
- Each session runs inside a **detached tmux session**; the per-session PTY is
  now an *attach client* (`tmux attach`) viewing it. tmux owns the agent
  lifecycle, so the agent survives WebSocket disconnects and orchid restarts.
- Sessions are **persisted to JSON** under `~/.config/orchid/sessions.json`,
  including the agent's own session id, so a stopped agent can be **resumed onto
  the same tmux session name** later.

## Goals

- `Agent` interface + registry; concrete `claude`, `codex`, `shell` agents.
- Create a session by choosing an agent (+ optional working directory).
- Run the agent in a detached tmux session named `orchid-<id>`; PTY attaches.
- Generate and store a UUID agent session id at create; bind it to the agent
  (`claude --session-id <uuid>`) so the agent is resumable by that id.
- Persist session records to `~/.config/orchid/sessions.json` (atomic writes).
- On startup, reconcile records against live tmux sessions: alive → reattachable
  `running`; dead → retained as `stopped` (resumable, not dropped).
- Restart/resume a stopped session into a fresh tmux session via the agent's
  resume command.
- Dashboard: choose agent + cwd on create; per-row agent + status; restart and
  delete controls.

## Non-Goals (this milestone)

- No per-agent config beyond the launch/resume argv (no model flags, env, hooks).
  (Agent wrapper deliberately minimal — config is a later milestone.)
- No authentication. **Localhost only.**
- No automatic respawn of dead sessions on startup (dead → `stopped`, user
  triggers restart explicitly).
- No multi-client-per-session (still one active WS per session, new kicks old).

## Architecture

```
browser (xterm.js)
   │ HTTP/JSON /api/sessions (+ /{id}/restart)   WebSocket /ws/{id}
   ▼
Go server
   ├─ internal/server   http + ws (existing, extended)
   ├─ internal/session  Session (PTY=tmux attach), Manager (+ store + Restore)
   ├─ internal/agent    Agent interface + claude/codex/shell registry
   ├─ internal/tmux     thin wrapper over the tmux CLI
   ├─ internal/store    JSON persistence (~/.config/orchid/sessions.json)
   └─ web               dashboard (agent dropdown, status, restart)

tmux daemon (owns agent processes; survives orchid restart)
```

**Key shift from M1:** the PTY no longer owns the agent process. The PTY runs
`tmux attach -t orchid-<id>`; the agent runs inside the tmux session. tmux is the
lifecycle owner and the source of truth for liveness. The M1 ring buffer +
broadcast + single-active-attachment design is otherwise unchanged — the Session
still holds one long-lived attach-PTY + readLoop feeding the ring.

## Component: internal/agent

```go
type Agent interface {
	Name() string
	// NewCommand returns argv to start a fresh agent session bound to the
	// caller-supplied agent session id (so it is resumable later). Agents
	// without a session concept (shell) ignore the id.
	NewCommand(cwd, agentSessionID string) []string
	// ResumeCommand returns argv to resume a prior session by id, or nil if the
	// agent has no resume capability (shell) — caller then starts fresh.
	ResumeCommand(cwd, agentSessionID string) []string
}

func Get(name string) (Agent, bool) // registry lookup
func Names() []string               // for the UI dropdown
func ResolveCwd(cwd string) string  // blank -> server default ($HOME now)
```

Concrete agents:

| Agent  | NewCommand                          | ResumeCommand                   |
|--------|-------------------------------------|---------------------------------|
| claude | `claude --session-id <uuid>`        | `claude --resume <uuid>`        |
| codex  | `codex`                             | `codex resume --last`           |
| shell  | `$SHELL` (fallback `/bin/bash`)     | nil (restart = fresh shell)     |

**codex limitation (verified against codex CLI):** codex interactive has no flag
to bind a caller-supplied session id at fresh start — it generates its own id —
and `codex resume <id>` would need an id we never observe. So codex ignores the
generated `agentSessionID` and resumes via `codex resume --last` (the most recent
codex session). This is a known imperfection (it resumes codex's globally-latest
session, which is usually but not guaranteed to be this orchid session). claude
binds and resumes a specific id as specified; shell has no resume. cwd is set for
every agent via tmux's `-c` flag (no agent needs a `--cd` argument).

cwd resolution lives in `ResolveCwd` (single place): empty string → `$HOME`.
Designed so role-based defaults can replace this later without touching callers.

## Component: internal/tmux

Thin wrapper over the `tmux` CLI (via `os/exec`):

```go
func Available() error                                   // `tmux -V`; error if missing
func NewSession(name, cwd string, argv []string) error   // new-session -d -s name -c cwd -- argv...
func HasSession(name string) bool                        // has-session -t name (exit 0)
func KillSession(name string) error                      // kill-session -t name
func AttachArgv(name string) []string                    // ["tmux","attach","-t",name]
```

`NewSession` passes `argv` so tmux execs the agent directly (no shell quoting
issues). `-c cwd` sets the working directory. Session names are validated to
`orchid-<id>` (hex id) — no user input flows into tmux target names.

## Component: internal/store

```go
type Record struct {
	ID             string    `json:"id"`
	Agent          string    `json:"agent"`
	Cwd            string    `json:"cwd"`
	Tmux           string    `json:"tmux"`
	AgentSessionID string    `json:"agent_session_id"`
	Created        time.Time `json:"created"`
	Status         string    `json:"status"` // "running" | "stopped"
}

func DefaultPath() (string, error)            // <os.UserConfigDir>/orchid/sessions.json
func Load(path string) ([]Record, error)      // missing file -> nil, nil
func Save(path string, recs []Record) error   // mkdir -p; write temp; rename (atomic)
```

`Status` in the JSON is the last-known value; on load the Manager reconciles it
against live tmux (see Restore). Save is best-effort: failures are logged, not
fatal (tmux remains source of truth).

## Component: internal/session (changes)

`Session` gains fields: `Agent string`, `Cwd string`, `Tmux string`,
`AgentSessionID string`, `Status string`. The PTY command becomes the tmux
attach client.

Two constructors replace the single `newSession`:

```go
// createSession: create a NEW detached tmux session running the agent, then
// spawn the attach-PTY.
func createSession(rec store.Record, ag agent.Agent, onExit func()) (*Session, error)
// attachSession: tmux session already exists (reconcile / restart) — just spawn
// the attach-PTY.
func attachSession(rec store.Record, onExit func()) (*Session, error)
```

Both end by: `pty.Start(exec.Command("tmux","attach","-t",rec.Tmux))`, start
`readLoop`, set `alive=true`. The existing ring/broadcast/attachment/closeOnce
machinery is reused verbatim.

Liveness: when the agent exits, its tmux session ends, the attach client gets
EOF, `readLoop` ends → `onExit` fires. `onExit` marks the record `stopped`
(NOT removed) and persists. `Alive()` continues to mirror readLoop state.

### Manager (changes)

```go
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session   // running sessions (have a live PTY)
	records  map[string]store.Record // ALL records incl. stopped
	path     string                // store path
}

func NewManager(path string) *Manager
func (m *Manager) Restore() error                    // load + reconcile against tmux
func (m *Manager) Create(agentName, cwd string) (*Session, error)
func (m *Manager) Restart(id string) (*Session, error) // stopped -> running
func (m *Manager) Get(id string) (*Session, bool)
func (m *Manager) List() []Info                      // running + stopped, sorted
func (m *Manager) Kill(id string) error              // tmux kill + remove record
func (m *Manager) KillAll()                          // close PTYs only (tmux survives)
```

- **Create**: validate agent exists; resolve cwd; `id := newID()`;
  `agentSessionID := newUUID()`; `tmux := "orchid-"+id`; build Record (running);
  `createSession`; store map + records; Save.
- **Restore**: `store.Load`; for each record: `tmux.HasSession(rec.Tmux)`? yes →
  `attachSession`, status running; no → status stopped (keep record, no PTY).
  Save reconciled list.
- **Restart**: record must exist and be stopped (else 409). Build resume argv via
  `agent.ResumeCommand` (nil → NewCommand fresh). `tmux.NewSession(...)`;
  `attachSession`; status running; Save.
- **onExit(id)**: mark record stopped; drop from `sessions` map; Save. (Session
  remains listed as stopped.)
- **Kill**: `tmux.KillSession`; remove from both maps; Save.
- **KillAll** (shutdown): close PTYs only; leave tmux + records intact.

`Info` gains `Agent`, `Status`, `Cwd` fields.

## Component: internal/server (changes)

- `POST /api/sessions` — body `{"agent":"claude","cwd":"/path"}` (cwd optional) →
  `Manager.Create` → `{id, agent, status, cwd}`. Unknown agent → 400.
- `GET /api/sessions` — `[]Info` incl. agent/status/cwd.
- `POST /api/sessions/{id}/restart` — `Manager.Restart`; 200 on success, 404
  unknown, 409 if already running.
- `DELETE /api/sessions/{id}` — `Manager.Kill`; 204 / 404.
- `GET /ws/{id}` — unchanged; if session not running (stopped/unknown) → 404.

## Component: web (changes)

- Create form: agent `<select>` populated from a static list (claude/codex/shell)
  + optional cwd `<input>`. POST JSON body.
- Sidebar rows show `agent · id` and a status dot (running/stopped). Running rows:
  ✕ delete. Stopped rows: ▶ restart + ✕ delete. Selecting a stopped session shows
  a placeholder ("stopped — restart to attach") instead of opening a WS.
- Restart button → `POST /{id}/restart` → on success select + attach.

## Errors & edge cases

- tmux missing → `tmux.Available()` at boot → `log.Fatal` with install hint.
- Agent binary not on PATH → tmux session dies immediately → record becomes
  `stopped` via onExit; UI shows stopped. No crash.
- Store write failure → logged, non-fatal.
- Restart a running session → 409. Restart a shell (nil ResumeCommand) → fresh.
- tmux session name is always server-generated `orchid-<hexid>`; user cwd is
  passed via `-c` (not interpolated into a shell), avoiding injection.

## Testing

- **agent** (unit, no exec): argv for claude/codex/shell, new vs resume, and
  `ResolveCwd("")` → `$HOME`.
- **tmux** (integration, real tmux): `Available`; `NewSession`/`HasSession`/
  `KillSession`/`AttachArgv` roundtrip running `$SHELL`/`sleep`. `t.Skip` if
  `Available()` fails. Always `KillSession` in cleanup.
- **store** (unit): `Load` missing → empty; `Save`+`Load` roundtrip; atomic temp
  file removed after rename; `DefaultPath` shape.
- **session/manager** (integration, real tmux + shell agent): create→attach→write
  echo→kill; self-exit (`exit\n`) → record becomes stopped + PTY gone; `Restore`
  reconcile (pre-create a tmux session → Restore attaches it; kill one → Restore
  marks stopped); `Restart` stopped→running.
- claude/codex binaries are NOT required for any test (only argv unit-tested).
- All concurrency tests run under `-race`.

## File layout (additions)

```
internal/agent/agent.go        # interface, registry, ResolveCwd
internal/agent/claude.go       # claude agent
internal/agent/codex.go        # codex agent
internal/agent/shell.go        # shell agent
internal/agent/agent_test.go
internal/tmux/tmux.go
internal/tmux/tmux_test.go
internal/store/store.go
internal/store/store_test.go
internal/session/session.go    # modified: tmux attach PTY, two constructors
internal/session/manager.go    # modified: store, Restore, Restart, records map
internal/server/api.go         # modified: create body, restart route
web/app.js, web/index.html     # modified: agent select, status, restart
cmd/orchid/main.go             # modified: tmux check, store path, Restore at boot
```
