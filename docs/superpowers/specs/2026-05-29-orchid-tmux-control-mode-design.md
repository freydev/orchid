# Orchid — tmux Control Mode for Native Browser Scrollback (Milestone 3)

**Date:** 2026-05-29
**Status:** Approved design

## Context

Milestone 2 backed each session with a detached tmux session that the browser
viewed through a per-session `tmux attach` pass-through PTY. That `attach` client
drives the outer terminal (xterm.js) as a **full-screen alternate-screen client**
(`\e[?1049h`). The alternate buffer has no scrollback, so:

- The browser's native scrollbar/wheel cannot scroll session history.
- tmux only paints the *current screen* to the client and collapses fast output,
  so the byte-stream ring never holds real history.
- tmux's terminal capability queries (DA1/DA2/XTVERSION/OSC color) leaked to
  xterm, which answered them late → echoed garbage (`1;2c0;276;0c`).

Verified facts driving this milestone:
- `claude` and `codex` do **not** use the alternate screen — they print inline
  in the normal buffer. The blocker to native scrollback was tmux's `attach`,
  not the agents.
- tmux **control mode** (`tmux -CC attach`, as used by iTerm2) streams each
  pane's raw output as a line-based protocol over pipes, instead of driving an
  alt-screen client. Feeding that output into xterm's **normal** buffer gives
  native scrollback while keeping tmux persistence (survives orchid restart).

## Goals

- Replace the `tmux attach` pass-through PTY with a tmux **control-mode client**
  (`tmux -CC attach`) per session. (Verified: `-CC` requires a PTY — `tcgetattr`
  fails over plain pipes — so the control client still runs under a creack/pty
  PTY; we read the line-based control protocol from it and write commands to it.)
- Browser xterm.js uses its **normal** buffer with real scrollback — native
  scrollbar and mouse wheel scroll history with no JS hacks.
- Keep all milestone-2 persistence: sessions survive WS disconnect AND orchid
  restart; `Restore` reconciles against live tmux; `Restart` resumes stopped
  sessions; records persist to `~/.config/orchid/sessions.json`.
- Restore prior scrollback on (re)attach, including output produced before the
  current orchid process started (via `capture-pane`).
- Keep the existing `Agent` abstraction (claude/codex/shell) and HTTP/WS surface
  unchanged on the wire.

## Non-Goals (this milestone)

- No tmux splits/windows in the UI: one session → one window → **one pane**. The
  control-mode parser handles a single pane; layout/window notifications are
  ignored.
- No mouse forwarding to inner apps (wheel scrolls xterm locally). Mouse-for-apps
  can come later.
- No authentication. Localhost only.

## Architecture

```
browser xterm.js  (NORMAL buffer: native scrollback + scrollbar)
   │ WS  binary keystrokes / JSON {type:resize,cols,rows}      (wire unchanged)
   ▼
orchid Session
   ├─ tmux.Control  ── tmux -CC attach -t orchid-<id>  (stdin/stdout pipes)
   │      stdout → control protocol parser → %output bytes ─┐
   │      stdin  ← send-keys -H <hex> / refresh-client -C / capture-pane
   ├─ ring (linear program output, ~1MB) ◄──────────────────┘ + seed via capture
   └─ broadcast → active WS → xterm normal buffer
tmux server  (owns agent processes; survives orchid restart)
```

Why this resolves the milestone-2 problems:
- `%output` carries **raw linear program output**; claude/codex print inline, so
  it accumulates in xterm's normal-buffer scrollback → native scroll works.
- No `tmux attach` → no outer alternate screen, no capability-query handshake →
  the `termfilter` query-stripping and the `mouse on` copy-mode hacks become
  unnecessary and are removed.
- Full-screen apps run *inside* the pane (e.g. vim) still emit their own
  alt-screen sequences within `%output`; xterm handles them per-app correctly,
  reverting to normal scrollback on exit.

## Component: internal/tmux — Control client

`NewSession` stays (creates the detached tmux session) but drops the `mouse on`
call; it additionally sets `window-size latest` for orchid sessions so that
`refresh-client -C <cols>x<rows>` from the control client drives the pane size.
(Verified: `window-size manual` makes tmux ignore `refresh-client -C`; `latest`
resizes correctly.)

```go
// Control is a tmux control-mode client (tmux -CC attach) for one session.
type Control struct {
	cmd  *exec.Cmd
	in   io.WriteCloser
	out  io.ReadCloser
	pane string          // "%N"
	ch   chan []byte     // decoded %output
	// internal: reply-block buffering, done signaling
}

func Attach(name string) (*Control, error) // start tmux -CC attach -t name; discover pane id
func (c *Control) Output() <-chan []byte    // decoded %output bytes (closed on exit)
func (c *Control) SendKeys(b []byte) error  // send-keys -t <pane> -H <hexpairs>
func (c *Control) Resize(cols, rows uint16) error // refresh-client -C <cols>x<rows>
func (c *Control) Capture(lines int) ([]byte, error) // capture-pane -e -p -J -S -<lines> -t <pane>
func (c *Control) Close() error             // close stdin, kill client (tmux session survives)
```

Control-mode protocol parsing (line-based on stdout; `bufio.Scanner` with a
raised buffer limit for long `%output` lines):

- `%output %<pane> <data>` → octal-unescape `data` (`\NNN` octal, `\\` →`\`) →
  send bytes on `ch`.
- `%begin <ts> <num> <flags>` … `%end <ts> <num> <flags>` / `%error …` → collect
  the lines in between as a command reply (used by pane discovery and `Capture`).
- `%exit [reason]` → control session ending → close `ch`.
- `%window-add`, `%layout-change`, `%session-changed`, `%client-session-changed`,
  `%client-detached`, `%pane-mode-changed`, `%unlinked-window-*`, `%begin`-less
  unknown `%…` lines → ignored.

Pane discovery: immediately after attach, write `list-panes -t <name> -F '#{pane_id}'`
and read the `%begin/%end` reply to set `c.pane`.

Commands (one per line, written to `in`):
- keystrokes: `send-keys -t <pane> -H <hh hh …>` (space-separated hex byte pairs;
  handles arbitrary control/escape bytes from xterm).
- resize: `refresh-client -C <cols>x<rows>`.
- history: `capture-pane -t <pane> -e -p -J -S -<lines>` (escape sequences kept,
  joined wrapped lines), reply read from the `%begin/%end` block.

Octal-unescape and the protocol line classifier are pure functions, unit-tested
without tmux.

## Component: internal/session

`Session` swaps the attach-PTY (`cmd`/`pty`) for `ctl *tmux.Control`. The
ring/attachment/broadcast/`Attach`/`Detach`/`Replay`/`closeOnce` machinery is
unchanged except the ring capacity grows (`ringSize` 64 KiB → 1 MiB) since it now
holds linear history.

- `readLoop` ranges over `ctl.Output()`, `broadcast`ing each chunk (then, on
  channel close, runs the existing dead/stopped cleanup + `onExit`).
- `Write(b)` → `ctl.SendKeys(b)`; `Resize` → `ctl.Resize`; `Close` → `ctl.Close`;
  `Alive` mirrors whether the control client is live.
- `createSession(rec, ag, onExit)`: `tmux.NewSession(...)` then `tmux.Attach` then
  build `Session`; caller `Start()`s it (unchanged register-before-Start order).
- `attachSession(rec, onExit)`: `tmux.Attach`; **seed history** by writing
  `ctl.Capture(historyLines)` output into the ring before `Start()` so reattached
  (Restore/Restart) sessions show prior scrollback. `historyLines` ≈ 2000.
- `Start()` unchanged (`go s.readLoop()`).

Deletions:
- `termfilter.go` and `termfilter_test.go` (DA-query garbage cannot occur in
  control mode).
- The `mouse on` call and `TestNewSessionEnablesMouse` (native scroll replaces
  tmux copy-mode).

## Component: internal/session — Manager

Surface unchanged: `NewManager(path)`, `Create(agent,cwd)`, `Restore`,
`Restart(id)`, `Get`, `List`, `Kill`, `KillAll`, `onExit`, `closing`. Records,
store, and reconcile logic (HasSession → running/stopped, dead retained) are
identical. Only the per-session backend (control client vs PTY) changes, behind
`createSession`/`attachSession`.

## Component: web

`app.js`:
- `new Terminal({ scrollback: 5000, cursorBlink: true, fontSize: 14 })`.
- FitAddon retained: `fit.fit()` + `ResizeObserver` → send `{type:"resize",cols,rows}`
  (server → `refresh-client -C`).
- Native wheel/scrollbar scroll the normal-buffer scrollback. No custom wheel
  handling, no row-onclick changes (the row-click fix from milestone 2 stays).
- Session switch: teardown + new `Terminal` + WS; `Replay()` paints history into
  scrollback.

HTTP/WS wire protocol is unchanged (binary keystrokes, JSON resize, binary
output, `/api/sessions` + `/ws/{id}` + `/restart`).

## Errors & edge cases

- `tmux -CC attach` fails / session gone → control client exits → `%exit` or EOF
  → `Output()` closes → readLoop ends → record marked `stopped` (existing path).
- Unknown protocol lines ignored; a malformed `%output` escape decodes
  best-effort and logs once.
- `send-keys`/`refresh-client`/`capture-pane` → tmux `%error` reply → logged,
  non-fatal.
- Pane-id discovery failure → `Attach` returns error → creation/restore marks the
  session failed/stopped.
- `Capture` seed failure on reattach → logged; continue with empty history.
- `refresh-client -C` requires `window-size latest` (set at session create); with
  `manual` tmux ignores the control client's size.
- The control protocol stream opens with a `\x1bP1000p` DCS marker and closes with
  `\x1b\\` (ST); the parser strips a leading `\x1bP…p` and ignores the ST.

## Testing

- **tmux.Control unit (no tmux):** octal-unescape (`\033`, `\\`, literal,
  multibyte UTF-8 bytes); protocol classifier over canned `%output`/`%begin`/
  `%end`/`%error`/`%exit` lines → asserts decoded bytes and events; hex encoding
  of `send-keys -H`.
- **tmux.Control integration (real tmux, skip if absent):** attach to a shell
  session, `SendKeys("echo MARK\n")`, read `Output()` until "MARK"; `Capture`
  returns earlier lines; `Resize` then verify pane size via `list-panes`.
- **session/manager integration:** create→write echo→see output; self-exit →
  stopped; `Restore` reconcile (live→running, dead→stopped); `Restart`
  stopped→running; **seed-on-restore**: pre-create a tmux session with known
  output, `Restore`, assert `Replay()` contains it.
- Remove `termfilter_test.go` and `TestNewSessionEnablesMouse`.
- All concurrency tests run under `-race`. tmux-touching tests skip when
  `tmux.Available()` fails and clean up `orchid-*` sessions.

## File layout (changes)

```
internal/tmux/control.go        # NEW: Control client + protocol parser + unescape
internal/tmux/control_test.go   # NEW
internal/tmux/tmux.go           # MODIFIED: NewSession sets window-size manual, drops mouse on
internal/tmux/tmux_test.go      # MODIFIED: drop TestNewSessionEnablesMouse
internal/session/session.go     # MODIFIED: ctl *tmux.Control, capture-seed, bigger ring
internal/session/termfilter.go      # DELETED
internal/session/termfilter_test.go # DELETED
internal/session/session_test.go    # MODIFIED: control-mode helper
internal/session/manager.go     # (unchanged surface; wiring only if needed)
web/app.js                      # MODIFIED: scrollback option; remove any wheel hacks
```
