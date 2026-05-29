# Orchid — Web Terminal Dashboard (Milestone 1)

**Date:** 2026-05-29
**Status:** Approved design

## Context

Orchid is a (future) orchestrator for parallel processes/agents, inspired by
[gastown](https://github.com/gastownhall/gastown) but deliberately simpler.
Gastown coordinates multiple AI coding agents over tmux with a Go backend and an
htmx dashboard. We are **not** copying gastown — it is context only for where
this is heading.

This milestone builds the foundation: a Go server that spawns PTY shell sessions
and exposes them in a browser terminal emulator (xterm.js) over WebSocket.
Multiple sessions are supported so each can later become an orchestrated agent.

## Goals

- Single Go binary: serves UI, REST API, and WebSocket.
- Multiple concurrent PTY sessions, managed in memory.
- Browser dashboard: list sessions, create, kill, attach to a live terminal.
- Live interactive I/O and terminal resize.
- Reconnect re-attaches the live shell with recent scrollback replayed.

## Non-Goals (this milestone)

- No authentication. **Localhost only.** The server spawns real shells; it must
  not be exposed to a network. Auth is a later milestone.
- No persistence / database. Sessions are in-memory; restart loses them.
- No agent orchestration, scheduling, tmux, or git integration yet.
- No JS framework, no npm/Vite build step.

## Architecture

```
browser (xterm.js, vanilla JS)
   │  WebSocket  /ws/:sessionId   (binary: PTY bytes both ways; JSON control frames)
   │  HTTP/JSON  /api/sessions     (list / create / kill)
   ▼
Go server (single binary)
   ├─ static handler  → embed.FS (index.html, app.js, xterm.js/css)
   ├─ session manager → map[id]*Session, mutex-guarded
   └─ Session         → creack/pty + os/exec shell, read/write pumps + ring buffer
```

- One binary serves UI + API + WS.
- No DB; sessions live in memory.
- Dead session = closed PTY, removed from map.

### Tech stack

- **Backend:** Go. PTY via `github.com/creack/pty`. WebSocket via
  `github.com/coder/websocket`. Standard `net/http`
  router (Go 1.22+ `http.ServeMux` pattern routing).
- **Static assets:** Go `embed.FS`.
- **Frontend:** vanilla TypeScript-free JS. xterm.js + `xterm-addon-fit`,
  **vendored** into the repo (no CDN), committed for offline single-binary use.

## Session lifecycle

```go
type Session struct {
    ID   string
    Cmd  *exec.Cmd   // user shell: $SHELL, fallback /bin/bash
    Pty  *os.File    // creack/pty master
    ring *ringBuffer // recent output, ~64KB
    mu   sync.Mutex
}
```

- **Create** — `POST /api/sessions`: manager spawns the shell in a PTY, assigns a
  short ID (8-char), stores it in the map, returns `{ "id": "..." }`.
- **List** — `GET /api/sessions`: returns array of `{ id, alive }`.
- **Attach** — `GET /ws/:id`: upgrade to WebSocket. On attach, replay the ring
  buffer, then run two goroutines:
  - PTY→WS: read PTY, write binary WS frames.
  - WS→PTY: read frames; binary → write to PTY; JSON control frame
    `{"type":"resize","cols":N,"rows":N}` → `pty.Setsize`.
  - Only one active WS per session; a new attach kicks the old one.
- **Reconnect** — the PTY outlives the WS. A new WS to the same ID re-attaches
  the live shell and replays the ring buffer.
- **Kill** — `DELETE /api/sessions/:id`: close PTY, `Cmd.Process.Kill()`, remove
  from map.
- **Exit** — shell exits → PTY read returns EOF → session marked dead, active WS
  notified and closed, session dropped from map.

### Ring buffer

Each session keeps the last ~64KB of PTY output in a ring buffer, replayed on
attach so reconnects show recent context instead of a blank screen.

## Frontend

`embed.FS` serves:

- `index.html` — flex split layout. Left: session list (create button, session
  IDs, kill ✕ per row). Right: terminal pane.
- `app.js` — vanilla JS:
  - `fetch` against `/api/sessions` for list/create/kill.
  - On session select: construct `Terminal()`, open WS to `/ws/:id`, wire
    `term.onData → ws.send` and `ws.onmessage → term.write`.
  - `FitAddon` + `ResizeObserver` → send resize control frame on size change.
  - Switching sessions tears down the old `Terminal` + WS and builds a new pair.
- Vendored `xterm.js`, `xterm.css`, `xterm-addon-fit.js`.

## Errors & edge cases

- WS drops → frontend auto-reconnects with backoff; ring buffer replayed.
- Kill session with active WS → server closes the WS; frontend marks it dead.
- Unknown session ID on WS → HTTP 404 before upgrade.
- Bind address/port configurable via flag (`-addr :8080`, default localhost).

## Testing

- **Go unit:** session manager — create / get / kill, concurrent map access
  (run under `-race`).
- **Go integration:** PTY echo — spawn a session, write `echo hi\n`, assert the
  output appears (via ring buffer or PTY read).
- **Manual smoke:** browser — open, type a command, resize, reconnect, kill.

## Project layout (proposed)

```
orchid/
  cmd/orchid/main.go        # flag parsing, server wiring
  internal/session/         # Session, manager, ring buffer
  internal/server/          # http handlers, ws upgrade, pumps
  web/                      # index.html, app.js, vendored xterm assets (embed)
  go.mod
```
