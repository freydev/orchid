# Orchid tmux Control Mode (Native Scrollback) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace each session's `tmux attach` pass-through PTY with a tmux control-mode client (`tmux -CC attach`), so program output streams into xterm.js's normal buffer — giving native browser scrollback/scrollbar — while keeping tmux persistence (survives orchid restart).

**Architecture:** A `tmux.Control` type runs `tmux -CC attach` under a creack/pty PTY, parses the line-based control protocol (`%output`, `%begin/%end`, `%exit`), exposes decoded program output on a channel, and sends input via `send-keys -H` (hex) and resize via `refresh-client -C`. `Session` swaps its attach-PTY for a `Control`; the ring (now linear history, enlarged) feeds xterm. The DA-query `termfilter` and tmux `mouse on` hacks are removed (control mode makes them unnecessary).

**Tech Stack:** Go 1.26, `github.com/creack/pty`, `github.com/coder/websocket`, system tmux 3.x (control mode `-CC`), vanilla JS + xterm.js.

**Verified protocol facts (from a spike against tmux 3.6b):**
- `tmux -CC attach` needs a PTY (`tcgetattr` fails over pipes).
- Output is `\r\n`-terminated lines; the stream opens with a `\x1bP1000p` DCS marker (strip it) and ends with `\x1b\\`.
- Program output: `%output %<pane> <data>` where `<data>` is octal-escaped (`\015`=CR, `\012`=LF, `\\`=backslash).
- Command replies arrive as `%begin <ts> <num> <flags>` … lines … `%end <ts> <num> <flags>` (or `%error …`). **Every** command (send-keys, refresh-client, …) produces such a block.
- `list-panes -F '#{pane_id}'` reply content is a line like `%3` (the pane id).
- `send-keys -t <pane> -H <hh hh …>` injects raw bytes (space-separated hex pairs).
- `refresh-client -C <cols>x<rows>` resizes the pane **only** when the session option `window-size` is `latest` (default); `manual` ignores it.
- `capture-pane -t <pane> -e -p -J -S -<N>` returns scrollback (with escapes) inside a `%begin/%end` block.
- Notifications to ignore: `%session-changed`, `%window-add`, `%window-renamed`, `%window-close`, `%layout-change`, `%client-session-changed`, `%client-detached`, `%pane-mode-changed`, `%unlinked-window-*`, `%sessions-changed`, `%continue`, `%pause`.

**Reply-correlation strategy (avoids matching replies to commands):**
- *Setup phase* (synchronous, before streaming): the only commands whose **content** we need are `list-panes` (pane id) and `capture-pane` (history). Run them synchronously during attach, reading until their `%end`.
- *Streaming phase* (one goroutine): route `%output` → channel; **ignore all `%begin/%end` blocks** (they're just acks for send-keys/refresh-client) and all other notifications; `%exit`/EOF closes the channel.

---

## File Structure

```
internal/tmux/tmux.go            # MODIFIED: NewSession sets `window-size latest`, drops `mouse on`
internal/tmux/tmux_test.go       # MODIFIED: drop TestNewSessionEnablesMouse; add window-size test
internal/tmux/protocol.go        # NEW: pure helpers — unescapeOutput, classifyLine, hexKeys
internal/tmux/protocol_test.go   # NEW
internal/tmux/control.go         # NEW: Control client (Attach/Output/SendKeys/Resize/Capture/Close)
internal/tmux/control_test.go    # NEW (integration, real tmux)
internal/session/session.go      # MODIFIED: ctl *tmux.Control replaces cmd/pty; capture-seed; ring 1MiB
internal/session/session_test.go # MODIFIED: control-mode test helper
internal/session/termfilter.go        # DELETED
internal/session/termfilter_test.go   # DELETED
web/app.js                       # MODIFIED: Terminal({scrollback:5000})
```

`internal/session/manager.go` is unchanged (its surface and reconcile logic don't change; it calls `createSession`/`attachSession` which change internally).

---

## Task 1: tmux NewSession — window-size latest, drop mouse

**Files:** Modify `internal/tmux/tmux.go`, `internal/tmux/tmux_test.go`.

- [ ] **Step 1: Replace the mouse test with a window-size test.** In `internal/tmux/tmux_test.go`, delete `TestNewSessionEnablesMouse` and add:
```go
func TestNewSessionSetsWindowSizeLatest(t *testing.T) {
	skipIfNoTmux(t)
	name := "orchid-test-winsize"
	_ = KillSession(name)
	defer KillSession(name)
	if err := NewSession(name, "/tmp", []string{"sleep", "60"}); err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	out, err := exec.Command("tmux", "show-options", "-t", name, "window-size").CombinedOutput()
	if err != nil {
		t.Fatalf("show-options: %v: %s", err, out)
	}
	if !strings.Contains(string(out), "latest") {
		t.Fatalf("want window-size latest, got %q", out)
	}
}
```
(Keep the existing `os/exec` and `strings` imports added previously.)

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/tmux/ -run 'WindowSize|Mouse' -v`
Expected: FAIL — `TestNewSessionEnablesMouse` undefined is gone; `TestNewSessionSetsWindowSizeLatest` fails (`window-size` not `latest`).

- [ ] **Step 3: Update NewSession.** In `internal/tmux/tmux.go`, replace the `set-option … mouse on` line with `window-size latest`:
```go
	out, err := exec.Command("tmux", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("tmux new-session: %w: %s", err, out)
	}
	// window-size latest lets the control client's refresh-client -C drive the
	// pane size. (mouse mode is intentionally NOT set: native xterm scrollback
	// replaces tmux copy-mode under control mode.)
	_ = exec.Command("tmux", "set-option", "-t", name, "window-size", "latest").Run()
	return nil
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/tmux/ -v`
Expected: PASS (`TestAttachArgv`, `TestSessionRoundtrip`, `TestNewSessionSetsWindowSizeLatest`).

- [ ] **Step 5: Commit**

```bash
git add internal/tmux/tmux.go internal/tmux/tmux_test.go
git commit -m "feat: set window-size latest, drop mouse mode (prep for control mode)"
```

---

## Task 2: Control protocol pure helpers

**Files:** Create `internal/tmux/protocol.go`, `internal/tmux/protocol_test.go`.

- [ ] **Step 1: Write the failing test** — create `internal/tmux/protocol_test.go`:
```go
package tmux

import (
	"bytes"
	"testing"
)

func TestUnescapeOutput(t *testing.T) {
	cases := map[string]string{
		`echo MARK\015\012`: "echo MARK\r\n",
		`a\\b`:              `a\b`,
		`\033[32mhi\033[m`:  "\x1b[32mhi\x1b[m",
		`plain`:             "plain",
	}
	for in, want := range cases {
		if got := unescapeOutput([]byte(in)); !bytes.Equal(got, []byte(want)) {
			t.Fatalf("unescapeOutput(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestClassifyLine(t *testing.T) {
	pane, data, ok := parseOutput([]byte("%output %3 hello\\015"))
	if !ok || pane != "%3" || string(data) != "hello\r" {
		t.Fatalf("parseOutput = %q,%q,%v", pane, data, ok)
	}
	if _, _, ok := parseOutput([]byte("%layout-change @3 x")); ok {
		t.Fatal("non-%output should not parse as output")
	}
	if k, ok := beginEnd([]byte("%begin 1 2 1")); !ok || k != lineBegin {
		t.Fatalf("begin classify = %v,%v", k, ok)
	}
	if k, ok := beginEnd([]byte("%end 1 2 1")); !ok || k != lineEnd {
		t.Fatalf("end classify = %v,%v", k, ok)
	}
	if k, ok := beginEnd([]byte("%error 1 2 1")); !ok || k != lineEnd {
		t.Fatalf("error should classify as end, got %v,%v", k, ok)
	}
	if _, ok := beginEnd([]byte("%output %1 x")); ok {
		t.Fatal("%output should not be begin/end")
	}
}

func TestStripDCS(t *testing.T) {
	if got := stripDCSPrefix([]byte("\x1bP1000p%begin 1 2 0")); string(got) != "%begin 1 2 0" {
		t.Fatalf("stripDCSPrefix = %q", got)
	}
	if got := stripDCSPrefix([]byte("%output %1 x")); string(got) != "%output %1 x" {
		t.Fatalf("no-DCS line altered: %q", got)
	}
}

func TestHexKeys(t *testing.T) {
	if got := hexKeys([]byte("A\n")); got != "41 0a" {
		t.Fatalf("hexKeys = %q, want '41 0a'", got)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/tmux/ -run 'Unescape|Classify|StripDCS|HexKeys' -v`
Expected: FAIL — `undefined: unescapeOutput` etc.

- [ ] **Step 3: Write the implementation** — create `internal/tmux/protocol.go`:
```go
package tmux

import (
	"bytes"
	"fmt"
	"strings"
)

type lineKind int

const (
	lineBegin lineKind = iota
	lineEnd
)

// stripDCSPrefix removes a leading control-mode DCS marker (ESC P … p) that tmux
// emits at the start of the stream, leaving the protocol line intact.
func stripDCSPrefix(line []byte) []byte {
	if len(line) >= 2 && line[0] == 0x1b && line[1] == 'P' {
		if i := bytes.IndexByte(line, 'p'); i >= 0 {
			return line[i+1:]
		}
	}
	return line
}

// parseOutput returns the pane id and decoded bytes for a "%output %<pane> <data>"
// line. ok is false for any other line.
func parseOutput(line []byte) (pane string, data []byte, ok bool) {
	const pfx = "%output "
	if !bytes.HasPrefix(line, []byte(pfx)) {
		return "", nil, false
	}
	rest := line[len(pfx):]
	sp := bytes.IndexByte(rest, ' ')
	if sp < 0 {
		return string(rest), nil, true // pane with empty output
	}
	return string(rest[:sp]), unescapeOutput(rest[sp+1:]), true
}

// beginEnd classifies "%begin"/"%end"/"%error" lines (reply-block delimiters).
func beginEnd(line []byte) (lineKind, bool) {
	switch {
	case bytes.HasPrefix(line, []byte("%begin")):
		return lineBegin, true
	case bytes.HasPrefix(line, []byte("%end")), bytes.HasPrefix(line, []byte("%error")):
		return lineEnd, true
	}
	return 0, false
}

// unescapeOutput decodes tmux %output escaping: \NNN octal and \\ backslash.
func unescapeOutput(in []byte) []byte {
	out := make([]byte, 0, len(in))
	for i := 0; i < len(in); i++ {
		if in[i] != '\\' || i+1 >= len(in) {
			out = append(out, in[i])
			continue
		}
		if in[i+1] == '\\' {
			out = append(out, '\\')
			i++
			continue
		}
		// up to 3 octal digits
		if isOctal(in[i+1]) {
			val := 0
			j := i + 1
			for j < len(in) && j <= i+3 && isOctal(in[j]) {
				val = val*8 + int(in[j]-'0')
				j++
			}
			out = append(out, byte(val))
			i = j - 1
			continue
		}
		out = append(out, in[i])
	}
	return out
}

func isOctal(b byte) bool { return b >= '0' && b <= '7' }

// hexKeys formats bytes as space-separated lowercase hex pairs for send-keys -H.
func hexKeys(b []byte) string {
	parts := make([]string, len(b))
	for i, c := range b {
		parts[i] = fmt.Sprintf("%02x", c)
	}
	return strings.Join(parts, " ")
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/tmux/ -run 'Unescape|Classify|StripDCS|HexKeys' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tmux/protocol.go internal/tmux/protocol_test.go
git commit -m "feat: tmux control-mode protocol helpers (unescape, classify, hexkeys)"
```

---

## Task 3: Control client

**Files:** Create `internal/tmux/control.go`, `internal/tmux/control_test.go`.

- [ ] **Step 1: Write the failing test** — create `internal/tmux/control_test.go`:
```go
package tmux

import (
	"bytes"
	"testing"
	"time"
)

func TestControlAttachStreamSendKeys(t *testing.T) {
	skipIfNoTmux(t)
	name := "orchid-test-ctl"
	_ = KillSession(name)
	if err := NewSession(name, "/tmp", []string{"bash", "--norc"}); err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer KillSession(name)

	ctl, err := Attach(name)
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	defer ctl.Close()
	if ctl.Pane() == "" {
		t.Fatal("pane id not discovered")
	}

	ctl.Start()
	if err := ctl.SendKeys([]byte("echo MARK123\n")); err != nil {
		t.Fatalf("SendKeys: %v", err)
	}

	var acc []byte
	deadline := time.After(6 * time.Second)
	for {
		select {
		case b, ok := <-ctl.Output():
			if !ok {
				t.Fatalf("output closed; acc=%q", acc)
			}
			acc = append(acc, b...)
			if bytes.Contains(acc, []byte("MARK123")) {
				return
			}
		case <-deadline:
			t.Fatalf("MARK123 not seen; acc=%q", acc)
		}
	}
}

func TestControlCaptureReturnsHistory(t *testing.T) {
	skipIfNoTmux(t)
	name := "orchid-test-cap"
	_ = KillSession(name)
	if err := NewSession(name, "/tmp", []string{"bash", "--norc"}); err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer KillSession(name)

	// Produce output via a throwaway control client.
	c1, err := Attach(name)
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	c1.Start()
	c1.SendKeys([]byte("echo HISTLINE\n"))
	time.Sleep(1500 * time.Millisecond)
	c1.Close()

	// Fresh client captures history before streaming.
	c2, err := Attach(name)
	if err != nil {
		t.Fatalf("Attach2: %v", err)
	}
	defer c2.Close()
	hist, err := c2.Capture(200)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if !bytes.Contains(hist, []byte("HISTLINE")) {
		t.Fatalf("history missing HISTLINE; got %q", hist)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/tmux/ -run TestControl -v`
Expected: FAIL — `undefined: Attach`.

- [ ] **Step 3: Write the implementation** — create `internal/tmux/control.go`:
```go
package tmux

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
)

// Control is a tmux control-mode client (tmux -CC attach) for one session.
// Lifecycle: Attach (synchronous setup: discover pane, optional Capture), then
// Start (begins streaming %output on Output()).
type Control struct {
	cmd  *exec.Cmd
	pty  *os.File
	br   *bufio.Reader
	pane string

	writeMu sync.Mutex
	ch      chan []byte
	started bool
}

// Attach starts `tmux -CC attach -t name` under a PTY and discovers the pane id.
func Attach(name string) (*Control, error) {
	cmd := exec.Command("tmux", "-CC", "attach", "-t", name)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	f, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}
	c := &Control{
		cmd: cmd, pty: f,
		br: bufio.NewReaderSize(f, 1<<20),
		ch: make(chan []byte, 256),
	}
	pane, err := c.discoverPane(name)
	if err != nil {
		_ = c.Close()
		return nil, err
	}
	c.pane = pane
	return c, nil
}

func (c *Control) Pane() string { return c.pane }

// readLine reads one CR/LF-terminated protocol line, stripping the trailing
// \r\n and any leading DCS marker.
func (c *Control) readLine() ([]byte, error) {
	line, err := c.br.ReadBytes('\n')
	if len(line) == 0 {
		return nil, err
	}
	line = bytes.TrimRight(line, "\r\n")
	return stripDCSPrefix(line), err
}

func (c *Control) writeCmd(format string, args ...any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err := fmt.Fprintf(c.pty, format+"\n", args...)
	return err
}

// readReplyBlock reads lines until the next %end/%error, returning the content
// lines between %begin and %end. It must be called only during setup (before
// Start), so no concurrent reader competes for the PTY.
func (c *Control) readReplyBlock() ([][]byte, error) {
	var content [][]byte
	inBlock := false
	for {
		line, err := c.readLine()
		if line != nil {
			if k, ok := beginEnd(line); ok {
				if k == lineBegin {
					inBlock = true
					content = nil
					continue
				}
				return content, nil // lineEnd
			}
			if inBlock {
				content = append(content, line)
			}
		}
		if err != nil {
			return content, err
		}
	}
}

func (c *Control) discoverPane(name string) (string, error) {
	if err := c.writeCmd("list-panes -t %s -F \"#{pane_id}\"", name); err != nil {
		return "", err
	}
	// Scan reply blocks until we find a pane-id line ("%<digits>").
	for attempts := 0; attempts < 50; attempts++ {
		block, err := c.readReplyBlock()
		for _, l := range block {
			if len(l) >= 2 && l[0] == '%' && l[1] >= '0' && l[1] <= '9' {
				return string(l), nil
			}
		}
		if err != nil {
			return "", fmt.Errorf("pane discovery: %w", err)
		}
	}
	return "", fmt.Errorf("pane id not found for %s", name)
}

// Capture returns up to lines of scrollback (with escapes). Setup-phase only.
func (c *Control) Capture(lines int) ([]byte, error) {
	if c.started {
		return nil, fmt.Errorf("Capture must be called before Start")
	}
	if err := c.writeCmd("capture-pane -t %s -e -p -J -S -%d", c.pane, lines); err != nil {
		return nil, err
	}
	block, err := c.readReplyBlock()
	if err != nil && len(block) == 0 {
		return nil, err
	}
	return bytes.Join(block, []byte("\r\n")), nil
}

// Start launches the streaming goroutine: %output → Output() channel.
func (c *Control) Start() {
	c.started = true
	go func() {
		defer close(c.ch)
		for {
			line, err := c.readLine()
			if line != nil {
				if pane, data, ok := parseOutput(line); ok && pane == c.pane && len(data) > 0 {
					c.ch <- data
				}
				// %begin/%end acks and all other notifications are ignored.
			}
			if err != nil {
				return
			}
		}
	}()
}

func (c *Control) Output() <-chan []byte { return c.ch }

// SendKeys injects raw bytes into the pane.
func (c *Control) SendKeys(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	return c.writeCmd("send-keys -t %s -H %s", c.pane, hexKeys(b))
}

// Resize sets the pane size via the control client.
func (c *Control) Resize(cols, rows uint16) error {
	if cols == 0 || rows == 0 {
		return nil
	}
	return c.writeCmd("refresh-client -C %dx%d", cols, rows)
}

// Close terminates the control client. The tmux session itself survives.
func (c *Control) Close() error {
	_ = c.pty.Close()
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	go func() { _ = c.cmd.Wait() }()
	return nil
}
```

- [ ] **Step 4: Run to verify it passes (race)**

Run: `go test ./internal/tmux/ -run TestControl -race -v`
Expected: PASS (both control tests). No race warnings. Confirm no leftover sessions: `tmux ls 2>/dev/null | grep orchid-test || echo clean`.

- [ ] **Step 5: Commit**

```bash
git add internal/tmux/control.go internal/tmux/control_test.go
git commit -m "feat: tmux control-mode client (attach, stream, send-keys, resize, capture)"
```

---

## Task 4: Rework Session onto the Control client

**Files:** Modify `internal/session/session.go`; delete `internal/session/termfilter.go` + `internal/session/termfilter_test.go`; modify `internal/session/session_test.go`.

**Context:** `Session` currently holds `cmd *exec.Cmd`, `pty *os.File`, `qf queryFilter`, and the constructors run `tmux attach` via creack/pty. Replace those with `ctl *tmux.Control`. The ring/`attachment`/`broadcast`/`Attach`/`Detach`/`Replay`/`closeOnce`/`Done`/`Ch`/`Start` machinery stays. `Start()` is still called by the Manager after registering the session.

- [ ] **Step 1: Delete the obsolete termfilter**

Run:
```bash
git rm internal/session/termfilter.go internal/session/termfilter_test.go
```

- [ ] **Step 2: Update the session test helper.** In `internal/session/session_test.go`, the helper `newTestSession` currently builds via `createSession(rec, sh, nil)`; keep that call but it must `Start()` the session (unchanged) — verify the helper still ends with `s.Start()` if present, otherwise add it. Replace the helper body with:
```go
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
	s.Start()
	t.Cleanup(func() { s.Close(); tmux.KillSession(name) })
	return s
}
```
(If the existing helper already matches, leave it. The four `TestSession*` bodies are unchanged.)

- [ ] **Step 3: Run to verify it fails to compile**

Run: `go build ./internal/session/`
Expected: FAIL — `qf` / `queryFilter` undefined (termfilter deleted) and the constructors still reference `pty`. This drives the rewrite.

- [ ] **Step 4: Rewrite session.go fields, constructors, and readLoop.**

Replace the import block of `internal/session/session.go` with:
```go
import (
	"sync"

	"orchid/internal/agent"
	"orchid/internal/store"
	"orchid/internal/tmux"
)
```

Change `ringSize` and the `Session` struct:
```go
const ringSize = 1 << 20 // 1 MiB: ring now holds linear program history

// Session streams one tmux pane (via a control-mode client) to a WS client.
type Session struct {
	ID             string
	Agent          string
	Cwd            string
	Tmux           string
	AgentSessionID string

	ctl    *tmux.Control
	ring   *ringBuffer
	onExit func()

	mu       sync.Mutex
	alive    bool
	sub      *attachment
	doneOnce sync.Once
}
```

Replace `createSession` and `attachSession` with:
```go
// historyLines is how much scrollback to seed from tmux on (re)attach.
const historyLines = 2000

// createSession creates a NEW detached tmux session running the agent's fresh
// command, then attaches a control-mode client.
func createSession(rec store.Record, ag agent.Agent, onExit func()) (*Session, error) {
	argv := ag.NewCommand(rec.Cwd, rec.AgentSessionID)
	if err := tmux.NewSession(rec.Tmux, rec.Cwd, argv); err != nil {
		return nil, err
	}
	return attachSession(rec, onExit)
}

// attachSession attaches a control-mode client to an existing tmux session and
// seeds the ring with prior scrollback so reconnects show history.
func attachSession(rec store.Record, onExit func()) (*Session, error) {
	ctl, err := tmux.Attach(rec.Tmux)
	if err != nil {
		return nil, err
	}
	ring := newRingBuffer(ringSize)
	if hist, err := ctl.Capture(historyLines); err == nil && len(hist) > 0 {
		ring.Write(hist)
	}
	return &Session{
		ID:             rec.ID,
		Agent:          rec.Agent,
		Cwd:            rec.Cwd,
		Tmux:           rec.Tmux,
		AgentSessionID: rec.AgentSessionID,
		ctl:            ctl,
		ring:           ring,
		onExit:         onExit,
		alive:          true,
	}, nil
}
```

Replace `Start`/`readLoop` to consume the control channel, and update `Write`/`Resize`/`Close`:
```go
// Start begins streaming pane output. Call once, after registration.
func (s *Session) Start() {
	s.ctl.Start()
	go s.readLoop()
}

func (s *Session) readLoop() {
	for data := range s.ctl.Output() {
		s.broadcast(data)
	}
	// Output channel closed => control client exited => session dead.
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

func (s *Session) Write(p []byte) (int, error) {
	if err := s.ctl.SendKeys(p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (s *Session) Resize(cols, rows uint16) error {
	return s.ctl.Resize(cols, rows)
}

func (s *Session) Close() {
	s.doneOnce.Do(func() { _ = s.ctl.Close() })
}
```

Leave `broadcast`, `Attach`, `Detach`, `Replay`, `closeOnce`, `Done`, `Ch`, `Alive` unchanged. Remove the old `import` of `os`, `os/exec`, `creack/pty` and the `qf`/filter usage (gone with the rewrite above).

- [ ] **Step 5: Run tests (race)**

Run: `go test ./internal/session/ -race -v`
Expected: PASS (all `TestSession*` + manager tests + ring/attachment) or SKIP without tmux. No races. The manager tests now exercise control mode end-to-end.

- [ ] **Step 6: Commit**

```bash
git add internal/session/
git commit -m "feat: drive sessions via tmux control client; seed history; remove termfilter"
```

---

## Task 5: Frontend — enable scrollback

**Files:** Modify `web/app.js`.

- [ ] **Step 1: Enable xterm scrollback.** In `web/app.js`, change the Terminal constructor:
```js
  term = new Terminal({ cursorBlink: true, fontSize: 14, scrollback: 5000 });
```
No other JS changes: there is no custom wheel handler to remove (wheel was handled tmux-side via the now-removed `mouse on`). The native scrollbar/wheel now scroll the normal-buffer scrollback. The whole-row click handler from the previous milestone stays.

- [ ] **Step 2: Verify build + embed still serves**

Run: `go build ./... && go test ./internal/server/ -run 'TestStatic|TestEmbed' -v`
Expected: build clean; static/embed tests PASS (or skip).

- [ ] **Step 3: Commit**

```bash
git add web/app.js
git commit -m "feat: enable xterm scrollback for native browser scrolling"
```

---

## Task 6: Full build, vet, race

**Files:** none (gate).

- [ ] **Step 1: Whole repo**

Run: `go build ./... && go vet ./... && go test ./... -race`
Expected: all packages PASS, no races. `internal/session` and `internal/tmux` integration tests run (tmux present).

- [ ] **Step 2: Confirm no leftover test tmux sessions**

Run: `tmux ls 2>/dev/null | grep '^orchid-test' && echo LEAK || echo clean`
Expected: `clean`.

---

## Task 7: End-to-end smoke

**Files:** none (verification).

- [ ] **Step 1: Build and run**

Run:
```bash
rm -f "$HOME/.config/orchid/sessions.json"
go build -o orchid ./cmd/orchid
./orchid -port 18120 &
sleep 1
```

- [ ] **Step 2: Create a shell session, produce history, verify output flows**

Run:
```bash
ID=$(curl -sf -X POST -H 'Content-Type: application/json' -d '{"agent":"shell","cwd":"/tmp"}' http://127.0.0.1:18120/api/sessions | sed 's/.*"id":"\([^"]*\)".*/\1/')
echo "id=$ID"; tmux has-session -t "orchid-$ID" && echo "tmux ok"
```
Expected: non-empty id; tmux session exists.

- [ ] **Step 3: Browser check (manual)**

Open `http://127.0.0.1:18120`. In a shell session run `seq 1 500` (or `claude`). Verify:
1. Output renders; typing works; resize reflows (FitAddon).
2. **Mouse wheel / scrollbar scroll back through history natively** (no shell-history paging, no `1;2c0;276;0c` garbage).
3. Run `claude` (if installed) — its inline output is scrollable with the browser scrollbar.
4. Switch to another session and back — recent history is present (ring replay).

- [ ] **Step 4: Verify restart-survival + history reseed**

Run:
```bash
kill %1; sleep 1
./orchid -port 18120 &
sleep 1
curl -sf http://127.0.0.1:18120/api/sessions; echo   # session reattached, running
```
Then reopen the browser, select the session, confirm prior scrollback is restored (seeded via capture-pane).
Cleanup:
```bash
kill %1 2>/dev/null
tmux ls 2>/dev/null | grep '^orchid-' | cut -d: -f1 | xargs -r -n1 tmux kill-session -t
```

- [ ] **Step 5: Commit (only if fixes were needed)**

No code change expected here. If the smoke surfaced a bug, fix under the relevant task and re-run.

---

## Self-Review Notes

- **Spec coverage:** control-mode client over PTY (T3); protocol parse/unescape/hex (T2); `window-size latest` + drop mouse (T1); Session on control client + capture-seed + 1 MiB ring + termfilter removal (T4); native scrollback via `scrollback:5000` (T5); persistence/reconcile unchanged (Manager untouched, exercised in T4/T6); E2E incl. restart-survival + history reseed + native scroll (T7). All spec sections mapped.
- **Reply correlation:** only `list-panes` and `capture-pane` need reply content, both during synchronous setup (before `Start`); streaming phase ignores all `%begin/%end` acks — matches the spec's strategy and the verified protocol.
- **Type consistency:** `tmux.Control` methods (`Attach`/`Pane`/`Start`/`Output`/`SendKeys`/`Resize`/`Capture`/`Close`) used consistently in session.go; `parseOutput`/`unescapeOutput`/`beginEnd`/`stripDCSPrefix`/`hexKeys` shared between protocol.go and control.go; `createSession`/`attachSession` signatures unchanged so manager.go compiles untouched; `Session.Start()` still called by manager after registration.
- **Removed hacks:** `termfilter.go`/tests deleted (T4); `mouse on` replaced by `window-size latest` (T1). Both verified unnecessary under control mode.
- **Known limitation (documented in spec):** single pane only; tmux splits/windows not surfaced.
```
