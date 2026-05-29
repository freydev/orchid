# Orchid — CLI Subcommands (Milestone 4)

**Date:** 2026-05-29
**Status:** Approved design

## Context

Today the `orchid` binary only runs the HTTP/WebSocket server in the foreground
(`orchid -host H -port P`). There is no way to run it as a background daemon, to
open the dashboard, or to manage sessions from the shell. This milestone adds a
CLI: daemon lifecycle (`start`/`stop`/`status`), `open`, and session management
(`ls`/`create`/`rm`/`restart`) that talk to the running server's existing HTTP
API.

## Goals

- `orchid serve [-host -port]` — run the server in the foreground (today's
  behavior), now with graceful shutdown on SIGTERM/SIGINT.
- `orchid` (no subcommand) — alias to `serve` (dev/debug, backward compatible).
- `orchid start [-host -port]` — daemonize `serve`, write a runtime file
  (`~/.config/orchid/daemon.json` = pid + addr), wait until the server is
  listening.
- `orchid stop` — SIGTERM the daemon, remove the runtime file.
- `orchid status` — report whether the daemon is running, its addr, and session
  count.
- `orchid open` — open the dashboard URL in the browser.
- `orchid ls` — list sessions (id, agent, status, cwd).
- `orchid create <agent> [cwd]` — create a session, print the new id.
- `orchid rm <id>` — delete a session.
- `orchid restart <id>` — restart a stopped session.

## Non-Goals

- No OS service integration (launchd/systemd).
- No new CLI dependency — stdlib `flag` only.
- No auth / remote control: the CLI assumes a localhost daemon it can read the
  runtime file for. Session commands hit the same localhost API the dashboard
  uses.
- No change to the HTTP/WS API or the session/tmux backend.

## Architecture

```
cmd/orchid/main.go   # parse os.Args[1], route to a subcommand handler; thin
cmd/orchid/cli.go    # subcommand handlers (start/stop/status/open/ls/create/rm/restart/serve)
internal/runtime/    # daemon.json read/write (pid+addr), pid-alive check
internal/client/     # HTTP client over the existing API (List/Create/Kill/Restart)
internal/server/...  # unchanged
internal/session/... # unchanged
```

`serve` is the only subcommand that runs the server; every other command is
either process control (`start`/`stop`/`status`) or an API client
(`ls`/`create`/`rm`/`restart`/`open`/`status` session count).

## Component: command dispatch

`main.go`:
- `route(args []string) (handler func() int, err error)` maps the first arg to a
  handler and parses that subcommand's `flag.FlagSet`. Split out from `main` so
  it is unit-testable without executing. `main` calls `os.Exit(handler())`.
- No arg, or first arg starts with `-` → treat as `serve` (so `orchid -port X`
  still works).
- Unknown subcommand or `-h`/`help` → print usage, exit 2.

Each handler returns an exit code; handlers print user-facing messages to stdout
(success) / stderr (errors).

## Component: internal/runtime

```go
type Info struct {
	PID  int    `json:"pid"`
	Addr string `json:"addr"`
}
func Path() (string, error)            // <config>/orchid/daemon.json (honors XDG_CONFIG_HOME, ~/.config)
func Write(Info) error                 // atomic temp+rename
func Read() (Info, error)              // os.ErrNotExist if absent
func Remove() error                    // ignore-not-exist
func Alive(pid int) bool               // syscall signal 0 probe
```

`Path` mirrors `store.DefaultPath` (`~/.config/orchid`, honoring
`XDG_CONFIG_HOME`). `Alive` uses `syscall.Kill(pid, 0)` (== nil or EPERM → alive).

## Component: internal/client

```go
type Client struct{ addr string; http *http.Client }
func FromRuntime() (*Client, error)  // runtime.Read + Alive; ErrNotRunning if absent/stale (and Remove stale file)
func New(addr string) *Client
func (c *Client) BaseURL() string                                  // "http://"+addr
func (c *Client) List() ([]session.Info, error)                    // GET /api/sessions
func (c *Client) Create(agent, cwd string) (session.Info, error)   // POST /api/sessions {agent,cwd}
func (c *Client) Kill(id string) error                             // DELETE /api/sessions/{id}
func (c *Client) Restart(id string) (session.Info, error)          // POST /api/sessions/{id}/restart

var ErrNotRunning = errors.New("orchid not running (run: orchid start)")
```

`session.Info` is reused as the decode target (`id,agent,cwd,status,alive`).
HTTP status mapping: 400→`unknown agent`/bad request, 404→`no such session`,
409→`already running`, non-2xx otherwise → error with body text.

## Subcommand behavior

- **serve** (`-host` default `127.0.0.1`, `-port` default `8081`): tmux preflight,
  resolve store path, `NewManager`+`Restore`, start HTTP server. Install a
  SIGTERM/SIGINT handler that shuts the HTTP server down and calls
  `mgr.KillAll()` (closes control clients; tmux sessions survive), then exits 0.
  (Replaces the current `defer mgr.KillAll()` which never runs on signal.)
- **start** (`-host`/`-port`): if runtime file present and `Alive` → error
  "already running on <addr>" exit 1. Else `exec.Command(self, "serve", "-host",
  H, "-port", P)` with `SysProcAttr{Setsid:true}`, stdout/stderr appended to
  `~/.config/orchid/orchid.log`; do not wait. Poll `net.Dial("tcp", addr)` up to
  3s; on success write `runtime.Info{child.Pid, addr}` and print
  "orchid started on http://addr"; on timeout kill the child, print the last log
  lines, exit 1. `self` from `os.Executable()`.
- **stop**: `FromRuntime` (or read runtime directly); not running → print
  "orchid not running" exit 0. Else `syscall.Kill(pid, SIGTERM)`, poll until pid
  dead (~3s; SIGKILL fallback), `runtime.Remove()`, print "orchid stopped".
- **status**: runtime present + alive → "running on http://addr"; then GET list →
  "N sessions". Absent/stale → "not running" exit 0.
- **open**: `FromRuntime` → open `BaseURL()` with `open` (darwin) / `xdg-open`
  (linux); if no opener, print the URL.
- **ls**: `FromRuntime`.List → aligned table `ID  AGENT  STATUS  CWD` (or
  "no sessions").
- **create `<agent>` `[cwd]`**: missing agent → usage exit 2; `Create` → print id;
  400 → "unknown agent: <name>" exit 1.
- **rm `<id>`**: `Kill`; 404 → "no such session: <id>" exit 1.
- **restart `<id>`**: `Restart` → print "<id> running"; 404/409 → message exit 1.

## Errors & edge cases

- Stale runtime file (pid dead): `FromRuntime`/`status`/`stop` treat as not
  running and remove the file.
- `start` race (file written but server crashes): poll-for-listen guards it; on
  timeout the child is killed and no runtime file is written.
- Non-localhost `-host` on `start`: allowed (advanced); `open`/client still use
  the stored addr. Default stays localhost.
- All API errors surface the server's message; network refused → `ErrNotRunning`.

## Testing

- **internal/runtime (unit):** Write→Read roundtrip; Read missing → ErrNotExist;
  `Alive` true for own pid, false for an unused pid; `Path` shape with/without
  `XDG_CONFIG_HOME`.
- **internal/client (unit, httptest):** spin a stub `http.Handler` returning
  canned JSON/status; assert List/Create/Kill/Restart decode and map
  400/404/409; `FromRuntime` → ErrNotRunning when no file.
- **CLI dispatch (unit):** `route` returns the right handler / usage error for
  each subcommand and for unknown args; `create` with no agent → usage error.
- **serve shutdown (unit):** the signal-shutdown path invokes the injected
  shutdown/KillAll hook exactly once.
- **E2E smoke (real, skip if tmux absent):** `start` → `status` (running, 0
  sessions) → `create shell` (prints id) → `ls` (shows it) → `restart`/`rm` →
  `stop` (daemon gone, runtime file removed); verify a created tmux session
  survives `stop` and reattaches after a second `start`.

## File layout (additions)

```
internal/runtime/runtime.go      # NEW
internal/runtime/runtime_test.go # NEW
internal/client/client.go        # NEW
internal/client/client_test.go   # NEW
cmd/orchid/main.go               # MODIFIED: route() dispatch
cmd/orchid/cli.go                # NEW: subcommand handlers
cmd/orchid/cli_test.go           # NEW: route() table tests
```
