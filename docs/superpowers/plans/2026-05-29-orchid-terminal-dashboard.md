# Orchid Web Terminal Dashboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A single Go binary that spawns PTY shell sessions and exposes them as live browser terminals (xterm.js) over WebSocket, with a dashboard to list/create/kill multiple sessions.

**Architecture:** Go `net/http` server serves an embedded vanilla-JS dashboard, a JSON REST API for session management, and a WebSocket endpoint per session. Each session wraps a shell in a PTY (`creack/pty`); a persistent read loop fans output into a ~64KB ring buffer and to the currently-attached WebSocket. Sessions are in-memory, mutex-guarded, no persistence, localhost only.

**Tech Stack:** Go 1.26, `github.com/creack/pty`, `github.com/coder/websocket`, Go `embed`, vanilla JS + vendored `@xterm/xterm` + `@xterm/addon-fit`.

---

## File Structure

```
orchid/
  go.mod
  cmd/orchid/main.go            # flag parsing, wiring, ListenAndServe
  internal/session/ring.go      # ringBuffer
  internal/session/ring_test.go
  internal/session/session.go   # Session: PTY spawn, read loop, attach/broadcast
  internal/session/session_test.go
  internal/session/manager.go   # Manager: create/get/list/kill
  internal/session/manager_test.go
  internal/server/server.go     # http.Handler wiring, static embed
  internal/server/api.go        # /api/sessions handlers
  internal/server/ws.go         # /ws/{id} WebSocket attach + pumps
  web/index.html
  web/app.js
  web/vendor/xterm.js
  web/vendor/xterm.css
  web/vendor/addon-fit.js
  web/embed.go                  # //go:embed FS
```

Responsibilities: `session` package owns all process/PTY state and has zero HTTP knowledge. `server` package owns HTTP/WS and depends on `session`. `web` is static assets + the embed declaration.

---

## Task 1: Project skeleton & dependencies

**Files:**
- Create: `go.mod`
- Create: `cmd/orchid/main.go` (placeholder main)

- [ ] **Step 1: Init module**

Run:
```bash
cd /Users/andrey/Work/orchid_3
go mod init orchid
```

- [ ] **Step 2: Add dependencies**

Run:
```bash
go get github.com/creack/pty@latest
go get github.com/coder/websocket@latest
```
Expected: `go.mod` lists both modules, `go.sum` created.

- [ ] **Step 3: Placeholder main so the module builds**

Create `cmd/orchid/main.go`:
```go
package main

func main() {}
```

- [ ] **Step 4: Verify build**

Run: `go build ./...`
Expected: exit 0, no output.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum cmd/orchid/main.go
git commit -m "chore: init orchid go module and deps"
```

---

## Task 2: Ring buffer

**Files:**
- Create: `internal/session/ring.go`
- Test: `internal/session/ring_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/session/ring_test.go`:
```go
package session

import (
	"bytes"
	"testing"
)

func TestRingBufferKeepsTail(t *testing.T) {
	r := newRingBuffer(4)
	r.Write([]byte("ab"))
	if got := r.Snapshot(); !bytes.Equal(got, []byte("ab")) {
		t.Fatalf("got %q, want %q", got, "ab")
	}
	r.Write([]byte("cdef")) // total "abcdef", max 4 -> keep "cdef"
	if got := r.Snapshot(); !bytes.Equal(got, []byte("cdef")) {
		t.Fatalf("got %q, want %q", got, "cdef")
	}
}

func TestRingBufferSnapshotIsCopy(t *testing.T) {
	r := newRingBuffer(8)
	r.Write([]byte("xy"))
	snap := r.Snapshot()
	snap[0] = 'Z'
	if got := r.Snapshot(); !bytes.Equal(got, []byte("xy")) {
		t.Fatalf("snapshot mutation leaked into buffer: %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/session/ -run TestRingBuffer -v`
Expected: FAIL — `undefined: newRingBuffer`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/session/ring.go`:
```go
package session

import "sync"

// ringBuffer keeps the last max bytes written to it.
type ringBuffer struct {
	mu  sync.Mutex
	buf []byte
	max int
}

func newRingBuffer(max int) *ringBuffer {
	return &ringBuffer{max: max}
}

func (r *ringBuffer) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf = append(r.buf, p...)
	if len(r.buf) > r.max {
		r.buf = r.buf[len(r.buf)-r.max:]
	}
	return len(p), nil
}

// Snapshot returns a copy of the current contents.
func (r *ringBuffer) Snapshot() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]byte, len(r.buf))
	copy(out, r.buf)
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/session/ -run TestRingBuffer -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/session/ring.go internal/session/ring_test.go
git commit -m "feat: add ring buffer for session scrollback"
```

---

## Task 3: Session — PTY spawn, read loop, ring

**Files:**
- Create: `internal/session/session.go`
- Test: `internal/session/session_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/session/session_test.go`:
```go
package session

import (
	"bytes"
	"testing"
	"time"
)

func TestSessionEchoesToRing(t *testing.T) {
	s, err := newSession("test1")
	if err != nil {
		t.Fatalf("newSession: %v", err)
	}
	defer s.Close()

	if _, err := s.Write([]byte("echo orchid_marker\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	deadline := time.After(5 * time.Second)
	for {
		if bytes.Contains(s.ring.Snapshot(), []byte("orchid_marker")) {
			return // success
		}
		select {
		case <-deadline:
			t.Fatalf("marker not found in ring; got %q", s.ring.Snapshot())
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func TestSessionAliveFlips(t *testing.T) {
	s, err := newSession("test2")
	if err != nil {
		t.Fatalf("newSession: %v", err)
	}
	if !s.Alive() {
		t.Fatal("expected alive immediately after start")
	}
	s.Close()
	deadline := time.After(5 * time.Second)
	for s.Alive() {
		select {
		case <-deadline:
			t.Fatal("session still alive after Close")
		case <-time.After(50 * time.Millisecond):
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/session/ -run TestSession -v`
Expected: FAIL — `undefined: newSession`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/session/session.go`:
```go
package session

import (
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
)

const ringSize = 64 * 1024

// attachment is a single live WebSocket subscriber.
type attachment struct {
	ch   chan []byte
	done chan struct{}
}

// Session wraps a shell running in a PTY.
type Session struct {
	ID string

	cmd  *exec.Cmd
	pty  *os.File
	ring *ringBuffer

	mu       sync.Mutex
	alive    bool
	sub      *attachment // currently attached WS, or nil
	doneOnce sync.Once
}

func newSession(id string) (*Session, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	f, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}
	s := &Session{ID: id, cmd: cmd, pty: f, ring: newRingBuffer(ringSize), alive: true}
	go s.readLoop()
	return s, nil
}

func (s *Session) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := s.pty.Read(buf)
		if n > 0 {
			s.broadcast(buf[:n])
		}
		if err != nil {
			break
		}
	}
	s.mu.Lock()
	s.alive = false
	if s.sub != nil {
		close(s.sub.done)
		s.sub = nil
	}
	s.mu.Unlock()
}

// broadcast writes output to the ring and the live subscriber (if any).
func (s *Session) broadcast(p []byte) {
	s.ring.Write(p)
	s.mu.Lock()
	a := s.sub
	s.mu.Unlock()
	if a == nil {
		return
	}
	cp := append([]byte(nil), p...)
	select {
	case a.ch <- cp:
	case <-a.done:
	}
}

func (s *Session) Write(p []byte) (int, error) {
	return s.pty.Write(p)
}

func (s *Session) Resize(cols, rows uint16) error {
	return pty.Setsize(s.pty, &pty.Winsize{Cols: cols, Rows: rows})
}

func (s *Session) Alive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.alive
}

// Close terminates the shell and PTY.
func (s *Session) Close() {
	s.doneOnce.Do(func() {
		_ = s.pty.Close()
		if s.cmd.Process != nil {
			_ = s.cmd.Process.Kill()
		}
	})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/session/ -run TestSession -v`
Expected: PASS (both tests).

- [ ] **Step 5: Commit**

```bash
git add internal/session/session.go internal/session/session_test.go
git commit -m "feat: add PTY-backed session with read loop"
```

---

## Task 4: Session attach / detach (live subscriber)

**Files:**
- Modify: `internal/session/session.go` (add `Attach`, `Detach`, `Replay`)
- Test: `internal/session/session_test.go` (add attach test)

- [ ] **Step 1: Write the failing test**

Append to `internal/session/session_test.go`:
```go
func TestSessionAttachReceivesOutput(t *testing.T) {
	s, err := newSession("test3")
	if err != nil {
		t.Fatalf("newSession: %v", err)
	}
	defer s.Close()

	a := s.Attach()
	defer s.Detach(a)

	if _, err := s.Write([]byte("echo attach_marker\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	var acc []byte
	deadline := time.After(5 * time.Second)
	for {
		select {
		case b := <-a.ch:
			acc = append(acc, b...)
			if bytes.Contains(acc, []byte("attach_marker")) {
				return
			}
		case <-deadline:
			t.Fatalf("marker not received; got %q", acc)
		}
	}
}

func TestSessionAttachKicksPrevious(t *testing.T) {
	s, err := newSession("test4")
	if err != nil {
		t.Fatalf("newSession: %v", err)
	}
	defer s.Close()

	a1 := s.Attach()
	a2 := s.Attach() // should kick a1

	select {
	case <-a1.done: // expected
	case <-time.After(2 * time.Second):
		t.Fatal("previous attachment was not kicked")
	}
	s.Detach(a2)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/session/ -run TestSessionAttach -v`
Expected: FAIL — `s.Attach undefined`.

- [ ] **Step 3: Write minimal implementation**

Add to `internal/session/session.go`:
```go
// Attach registers a new live subscriber, kicking any previous one.
// Returns nil if the session is already dead.
func (s *Session) Attach() *attachment {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.alive {
		return nil
	}
	if s.sub != nil {
		close(s.sub.done)
	}
	a := &attachment{ch: make(chan []byte, 256), done: make(chan struct{})}
	s.sub = a
	return a
}

// Detach removes a as the current subscriber if it still is one.
func (s *Session) Detach(a *attachment) {
	if a == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sub == a {
		close(s.sub.done)
		s.sub = nil
	}
}

// Replay returns the current scrollback snapshot for a freshly attached client.
func (s *Session) Replay() []byte {
	return s.ring.Snapshot()
}
```

Note: `Detach` may double-close `done` if the read loop already closed it on death. Guard with a closed-channel check by making `done` close idempotent — replace the two `close(s.sub.done)` sites and the read-loop close with this helper added to `session.go`:
```go
func closeOnce(ch chan struct{}) {
	select {
	case <-ch:
		// already closed
	default:
		close(ch)
	}
}
```
Then in `readLoop`, `Attach`, and `Detach`, replace each `close(... .done)` / `close(s.sub.done)` with `closeOnce(<that channel>)`. (This race-safe close pattern is required because `done` can be closed from both the read loop and Detach.)

- [ ] **Step 4: Run tests with the race detector**

Run: `go test ./internal/session/ -run TestSession -race -v`
Expected: PASS, no race warnings.

- [ ] **Step 5: Commit**

```bash
git add internal/session/session.go internal/session/session_test.go
git commit -m "feat: add session attach/detach for live WS subscribers"
```

---

## Task 5: Manager

**Files:**
- Create: `internal/session/manager.go`
- Test: `internal/session/manager_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/session/manager_test.go`:
```go
package session

import (
	"sync"
	"testing"
)

func TestManagerCreateGetKill(t *testing.T) {
	m := NewManager()
	s, err := m.Create()
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer m.KillAll()

	got, ok := m.Get(s.ID)
	if !ok || got.ID != s.ID {
		t.Fatalf("Get(%q) = %v, %v", s.ID, got, ok)
	}
	if len(m.List()) != 1 {
		t.Fatalf("List len = %d, want 1", len(m.List()))
	}
	if err := m.Kill(s.ID); err != nil {
		t.Fatalf("kill: %v", err)
	}
	if _, ok := m.Get(s.ID); ok {
		t.Fatal("session still present after Kill")
	}
}

func TestManagerKillUnknown(t *testing.T) {
	m := NewManager()
	if err := m.Kill("nope"); err == nil {
		t.Fatal("expected error killing unknown session")
	}
}

func TestManagerConcurrentCreate(t *testing.T) {
	m := NewManager()
	defer m.KillAll()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := m.Create(); err != nil {
				t.Errorf("create: %v", err)
			}
		}()
	}
	wg.Wait()
	if len(m.List()) != 10 {
		t.Fatalf("List len = %d, want 10", len(m.List()))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/session/ -run TestManager -v`
Expected: FAIL — `undefined: NewManager`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/session/manager.go`:
```go
package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
)

// Info is a serializable session summary.
type Info struct {
	ID    string `json:"id"`
	Alive bool   `json:"alive"`
}

// Manager owns the set of live sessions.
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session
}

func NewManager() *Manager {
	return &Manager{sessions: make(map[string]*Session)}
}

func newID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (m *Manager) Create() (*Session, error) {
	id := newID()
	s, err := newSession(id)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.sessions[id] = s
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
	out := make([]Info, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, Info{ID: s.ID, Alive: s.Alive()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (m *Manager) Kill(id string) error {
	m.mu.Lock()
	s, ok := m.sessions[id]
	if ok {
		delete(m.sessions, id)
	}
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("session %q not found", id)
	}
	s.Close()
	return nil
}

func (m *Manager) KillAll() {
	m.mu.Lock()
	all := m.sessions
	m.sessions = make(map[string]*Session)
	m.mu.Unlock()
	for _, s := range all {
		s.Close()
	}
}
```

- [ ] **Step 4: Run tests with the race detector**

Run: `go test ./internal/session/ -race -v`
Expected: PASS (all session + manager tests), no race warnings.

- [ ] **Step 5: Commit**

```bash
git add internal/session/manager.go internal/session/manager_test.go
git commit -m "feat: add session manager (create/get/list/kill)"
```

---

## Task 6: HTTP API handlers

**Files:**
- Create: `internal/server/server.go`
- Create: `internal/server/api.go`
- Test: `internal/server/api_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/server/api_test.go`:
```go
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"orchid/internal/session"
)

func TestAPICreateListKill(t *testing.T) {
	mgr := session.NewManager()
	defer mgr.KillAll()
	h := New(mgr)
	srv := httptest.NewServer(h)
	defer srv.Close()

	// create
	resp, err := http.Post(srv.URL+"/api/sessions", "application/json", nil)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create status = %d", resp.StatusCode)
	}
	var created session.Info
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	if created.ID == "" {
		t.Fatal("empty id from create")
	}

	// list
	resp, _ = http.Get(srv.URL + "/api/sessions")
	var list []session.Info
	json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	if len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("list = %+v", list)
	}

	// kill
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/sessions/"+created.ID, nil)
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("kill status = %d", resp.StatusCode)
	}
	resp.Body.Close()

	// list empty
	resp, _ = http.Get(srv.URL + "/api/sessions")
	list = nil
	json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	if len(list) != 0 {
		t.Fatalf("expected empty list, got %+v", list)
	}
}

func TestAPIKillUnknownReturns404(t *testing.T) {
	mgr := session.NewManager()
	h := New(mgr)
	srv := httptest.NewServer(h)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/sessions/nope", nil)
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	resp.Body.Close()
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestAPI -v`
Expected: FAIL — `undefined: New`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/server/server.go`:
```go
package server

import (
	"net/http"

	"orchid/internal/session"
)

// Server wires the HTTP API and (later) WebSocket + static assets.
type Server struct {
	mgr *session.Manager
	mux *http.ServeMux
}

// New builds the HTTP handler. Static asset wiring is added in Task 8.
func New(mgr *session.Manager) http.Handler {
	s := &Server{mgr: mgr, mux: http.NewServeMux()}
	s.routes()
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/sessions", s.handleList)
	s.mux.HandleFunc("POST /api/sessions", s.handleCreate)
	s.mux.HandleFunc("DELETE /api/sessions/{id}", s.handleKill)
}
```

Create `internal/server/api.go`:
```go
package server

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.mgr.List())
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	sess, err := s.mgr.Create()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": sess.ID, "alive": sess.Alive()})
}

func (s *Server) handleKill(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.mgr.Kill(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/server/ -run TestAPI -v`
Expected: PASS (both tests).

- [ ] **Step 5: Commit**

```bash
git add internal/server/server.go internal/server/api.go internal/server/api_test.go
git commit -m "feat: add session REST API handlers"
```

---

## Task 7: WebSocket attach endpoint

**Files:**
- Create: `internal/server/ws.go`
- Modify: `internal/server/server.go` (register `/ws/{id}` route)
- Test: `internal/server/ws_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/server/ws_test.go`:
```go
package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"orchid/internal/session"
)

func TestWSEchoAndUnknown404(t *testing.T) {
	mgr := session.NewManager()
	defer mgr.KillAll()
	h := New(mgr)
	srv := httptest.NewServer(h)
	defer srv.Close()

	sess, _ := mgr.Create()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/" + sess.ID

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "")

	// send a command as a binary frame
	if err := c.Write(ctx, websocket.MessageBinary, []byte("echo ws_marker\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	var acc []byte
	for {
		typ, data, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("read: %v (acc=%q)", err, acc)
		}
		if typ == websocket.MessageBinary {
			acc = append(acc, data...)
			if strings.Contains(string(acc), "ws_marker") {
				break
			}
		}
	}

	// unknown session -> dial should fail with non-101
	badURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/nope"
	if _, _, err := websocket.Dial(ctx, badURL, nil); err == nil {
		t.Fatal("expected dial to unknown session to fail")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestWS -v`
Expected: FAIL — `404` route not handled / handshake fails because route missing.

- [ ] **Step 3: Register the route**

In `internal/server/server.go`, add to `routes()`:
```go
	s.mux.HandleFunc("GET /ws/{id}", s.handleWS)
```

- [ ] **Step 4: Write the WebSocket handler**

Create `internal/server/ws.go`:
```go
package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/coder/websocket"
)

type controlMsg struct {
	Type string `json:"type"`
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sess, ok := s.mgr.Get(id)
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// localhost dashboard; relax origin checks (localhost only, no auth).
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}
	defer c.Close(websocket.StatusInternalError, "closing")

	ctx := r.Context()

	att := sess.Attach()
	if att == nil {
		c.Close(websocket.StatusGoingAway, "session dead")
		return
	}
	defer sess.Detach(att)

	// Replay scrollback first.
	if replay := sess.Replay(); len(replay) > 0 {
		if err := c.Write(ctx, websocket.MessageBinary, replay); err != nil {
			return
		}
	}

	// PTY -> WS
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-att.done:
				c.Close(websocket.StatusGoingAway, "detached")
				return
			case b := <-att.ch:
				if err := c.Write(ctx, websocket.MessageBinary, b); err != nil {
					return
				}
			}
		}
	}()

	// WS -> PTY (also handles control frames)
	for {
		typ, data, err := c.Read(ctx)
		if err != nil {
			return
		}
		if typ == websocket.MessageText {
			var m controlMsg
			if json.Unmarshal(data, &m) == nil && m.Type == "resize" {
				_ = sess.Resize(m.Cols, m.Rows)
			}
			continue
		}
		if _, err := sess.Write(data); err != nil {
			return
		}
	}
}

var _ = context.Background // keep import if unused after edits
```

Note: remove the trailing `var _ = context.Background` line if `context` is already used (it is, via `ctx`); it is only a guard against an unused import during incremental edits.

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/server/ -run TestWS -race -v`
Expected: PASS, no race warnings.

- [ ] **Step 6: Commit**

```bash
git add internal/server/ws.go internal/server/server.go internal/server/ws_test.go
git commit -m "feat: add WebSocket attach endpoint with resize + replay"
```

---

## Task 8: Frontend (vendored xterm, dashboard, embed)

**Files:**
- Create: `web/vendor/xterm.js`, `web/vendor/xterm.css`, `web/vendor/addon-fit.js` (downloaded)
- Create: `web/index.html`
- Create: `web/app.js`
- Create: `web/embed.go`
- Modify: `internal/server/server.go` (serve static FS)

- [ ] **Step 1: Vendor xterm assets**

Run:
```bash
mkdir -p web/vendor
curl -fsSL https://cdn.jsdelivr.net/npm/@xterm/xterm@5.5.0/lib/xterm.js -o web/vendor/xterm.js
curl -fsSL https://cdn.jsdelivr.net/npm/@xterm/xterm@5.5.0/css/xterm.css -o web/vendor/xterm.css
curl -fsSL https://cdn.jsdelivr.net/npm/@xterm/addon-fit@0.10.0/lib/addon-fit.js -o web/vendor/addon-fit.js
```
Expected: three non-empty files in `web/vendor/`. Verify: `wc -c web/vendor/*` shows non-zero sizes.

Note: these UMD builds expose globals `window.Terminal` (xterm.js) and `window.FitAddon.FitAddon` (addon-fit).

- [ ] **Step 2: Write index.html**

Create `web/index.html`:
```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Orchid</title>
  <link rel="stylesheet" href="/vendor/xterm.css" />
  <style>
    * { box-sizing: border-box; }
    html, body { height: 100%; margin: 0; font-family: system-ui, sans-serif; }
    #app { display: flex; height: 100%; }
    #sidebar { width: 240px; background: #1a1a1a; color: #ddd; display: flex; flex-direction: column; }
    #sidebar header { padding: 12px; font-weight: 600; border-bottom: 1px solid #333; }
    #new { margin: 8px; padding: 8px; cursor: pointer; }
    #list { flex: 1; overflow-y: auto; list-style: none; margin: 0; padding: 0; }
    #list li { display: flex; align-items: center; justify-content: space-between; padding: 8px 12px; cursor: pointer; }
    #list li.active { background: #333; }
    #list li.dead { opacity: 0.5; }
    #list .kill { border: none; background: transparent; color: #f66; cursor: pointer; font-size: 14px; }
    #main { flex: 1; background: #000; }
    #term { height: 100%; width: 100%; }
  </style>
</head>
<body>
  <div id="app">
    <aside id="sidebar">
      <header>Orchid</header>
      <button id="new">+ New session</button>
      <ul id="list"></ul>
    </aside>
    <main id="main"><div id="term"></div></main>
  </div>
  <script src="/vendor/xterm.js"></script>
  <script src="/vendor/addon-fit.js"></script>
  <script src="/app.js"></script>
</body>
</html>
```

- [ ] **Step 3: Write app.js**

Create `web/app.js`:
```js
const listEl = document.getElementById("list");
const newBtn = document.getElementById("new");

let term = null;
let fit = null;
let ws = null;
let activeId = null;

async function fetchSessions() {
  const r = await fetch("/api/sessions");
  return r.json();
}

async function createSession() {
  const r = await fetch("/api/sessions", { method: "POST" });
  const s = await r.json();
  await refresh();
  selectSession(s.id);
}

async function killSession(id) {
  await fetch("/api/sessions/" + id, { method: "DELETE" });
  if (id === activeId) teardown();
  await refresh();
}

async function refresh() {
  const sessions = await fetchSessions();
  listEl.innerHTML = "";
  for (const s of sessions) {
    const li = document.createElement("li");
    li.textContent = s.id;
    if (s.id === activeId) li.classList.add("active");
    if (!s.alive) li.classList.add("dead");
    li.onclick = () => selectSession(s.id);
    const kill = document.createElement("button");
    kill.className = "kill";
    kill.textContent = "✕";
    kill.onclick = (e) => { e.stopPropagation(); killSession(s.id); };
    li.appendChild(kill);
    listEl.appendChild(li);
  }
}

function teardown() {
  if (ws) { ws.onclose = null; ws.close(); ws = null; }
  if (term) { term.dispose(); term = null; }
  fit = null;
  activeId = null;
}

function selectSession(id) {
  if (id === activeId) return;
  teardown();
  activeId = id;

  term = new Terminal({ cursorBlink: true, fontSize: 14 });
  fit = new FitAddon.FitAddon();
  term.loadAddon(fit);
  term.open(document.getElementById("term"));
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
  ro.observe(document.getElementById("term"));

  refresh();
}

function sendResize() {
  if (!term || !ws || ws.readyState !== WebSocket.OPEN) return;
  ws.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }));
}

newBtn.onclick = createSession;
refresh();
```

- [ ] **Step 4: Write the embed declaration and serve static**

Create `web/embed.go`:
```go
package web

import "embed"

//go:embed index.html app.js vendor/*
var FS embed.FS
```

Modify `internal/server/server.go` — add the import and a static route. Update the file to:
```go
package server

import (
	"io/fs"
	"net/http"

	"orchid/internal/session"
	"orchid/web"
)

type Server struct {
	mgr *session.Manager
	mux *http.ServeMux
}

func New(mgr *session.Manager) http.Handler {
	s := &Server{mgr: mgr, mux: http.NewServeMux()}
	s.routes()
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/sessions", s.handleList)
	s.mux.HandleFunc("POST /api/sessions", s.handleCreate)
	s.mux.HandleFunc("DELETE /api/sessions/{id}", s.handleKill)
	s.mux.HandleFunc("GET /ws/{id}", s.handleWS)

	static, _ := fs.Sub(web.FS, ".")
	s.mux.Handle("/", http.FileServer(http.FS(static)))
}
```

- [ ] **Step 5: Verify build and existing tests still pass**

Run: `go build ./... && go test ./... -race`
Expected: build clean, all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add web/ internal/server/server.go
git commit -m "feat: add embedded vanilla dashboard with xterm.js"
```

---

## Task 9: main.go wiring & manual smoke

**Files:**
- Modify: `cmd/orchid/main.go`

- [ ] **Step 1: Implement main**

Replace `cmd/orchid/main.go`:
```go
package main

import (
	"flag"
	"log"
	"net/http"

	"orchid/internal/server"
	"orchid/internal/session"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "listen address (localhost only by default)")
	flag.Parse()

	mgr := session.NewManager()
	defer mgr.KillAll()

	h := server.New(mgr)
	log.Printf("orchid listening on http://%s", *addr)
	if err := http.ListenAndServe(*addr, h); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 2: Build**

Run: `go build -o orchid ./cmd/orchid`
Expected: binary `./orchid` created, exit 0.

- [ ] **Step 3: Manual smoke test**

Run: `./orchid` then open `http://127.0.0.1:8080`. Verify:
1. Click "+ New session" → a session ID appears in the sidebar and a terminal opens.
2. Type `ls` + Enter → output renders.
3. Resize the browser window → terminal reflows, shell `tput cols` reflects new width.
4. Reload the page, re-select the session → recent scrollback replays.
5. Click ✕ on a session → it is removed; killing the active one clears the terminal.

Expected: all five behaviors work. Stop with Ctrl-C.

- [ ] **Step 4: Commit**

```bash
git add cmd/orchid/main.go
git commit -m "feat: wire orchid main with addr flag"
```

---

## Self-Review Notes

- **Spec coverage:** in-memory multi-session (T5), PTY shell (T3), WS binary I/O + JSON resize control (T7), ring-buffer replay on reconnect (T2/T4/T7), one-active-WS-kicks-old (T4/T7), REST list/create/kill (T6), unknown-id 404 (T6/T7), embedded vanilla dashboard + vendored xterm (T8), `-addr` localhost-default flag (T9). All spec sections mapped.
- **No auth / localhost only:** enforced by default `-addr 127.0.0.1:8080`; `InsecureSkipVerify` on WS accept is acceptable only because of the localhost binding (documented in code comment).
- **Type consistency:** `Info{ID,Alive}`, `Session.ID/Alive()/Write/Resize/Close/Attach/Detach/Replay`, `attachment{ch,done}`, `Manager.Create/Get/List/Kill/KillAll`, `server.New(mgr)`, `controlMsg{Type,Cols,Rows}` used consistently across tasks.
- **Frontend reconnect/backoff:** spec mentions auto-reconnect with backoff as an edge case; this milestone implements manual reselect-to-reattach (re-select rebuilds WS) and leaves automatic backoff reconnect as a deliberate follow-up — not required for the five smoke behaviors. Noted here so it is not a silent gap.
```
