# Orchid CLI Subcommands Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a CLI to the `orchid` binary — daemon lifecycle (`start`/`stop`/`status`), `open` dashboard, and session management (`ls`/`create`/`rm`/`restart`) — on top of the existing server, with no new dependencies.

**Architecture:** `main.go` routes `os.Args[1]` to subcommand handlers in `cli.go`. `serve` runs the server (today's behavior) with graceful SIGTERM/SIGINT shutdown. `start` daemonizes `serve` and records pid+addr in `internal/runtime` (`~/.config/orchid/daemon.json`). Session/`open`/`status` commands use `internal/client`, a thin HTTP wrapper over the existing API, reading the addr from the runtime file.

**Tech Stack:** Go 1.26 stdlib (`flag`, `os/exec`, `os/signal`, `syscall`, `net/http`); reuses `internal/server`, `internal/session`, `internal/store`.

---

## File Structure

```
internal/runtime/runtime.go      # NEW: Info{PID,Addr}; Path/Write/Read/Remove/Alive
internal/runtime/runtime_test.go # NEW
internal/client/client.go        # NEW: Client over the HTTP API; FromRuntime; ErrNotRunning
internal/client/client_test.go   # NEW (httptest)
cmd/orchid/main.go               # MODIFIED: route() dispatch + os.Exit
cmd/orchid/cli.go                # NEW: serve/start/stop/status/open/ls/create/rm/restart handlers
cmd/orchid/cli_test.go           # NEW: route() table tests
```

---

## Task 1: internal/runtime

**Files:** Create `internal/runtime/runtime.go`, `internal/runtime/runtime_test.go`.

- [ ] **Step 1: Write the failing test** — `internal/runtime/runtime_test.go`:
```go
package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathHonorsXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/cfg")
	got, err := Path()
	if err != nil || got != "/cfg/orchid/daemon.json" {
		t.Fatalf("Path = %q, %v", got, err)
	}
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/home/x")
	got, _ = Path()
	if got != "/home/x/.config/orchid/daemon.json" {
		t.Fatalf("fallback Path = %q", got)
	}
}

func TestWriteReadRemove(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := Write(Info{PID: 4321, Addr: "127.0.0.1:9999"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.PID != 4321 || got.Addr != "127.0.0.1:9999" {
		t.Fatalf("Read = %+v", got)
	}
	if err := Remove(); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := Read(); !os.IsNotExist(err) {
		t.Fatalf("Read after Remove = %v, want not-exist", err)
	}
	if err := Remove(); err != nil { // idempotent
		t.Fatalf("Remove twice: %v", err)
	}
	_ = filepath.Join // keep import if unused after edits
}

func TestAlive(t *testing.T) {
	if !Alive(os.Getpid()) {
		t.Fatal("own pid should be alive")
	}
	if Alive(0) || Alive(-1) {
		t.Fatal("invalid pids should be dead")
	}
	// a very high pid is almost certainly not running
	if Alive(4000000) {
		t.Skip("pid 4000000 unexpectedly alive; skipping")
	}
}
```
Remove the trailing `_ = filepath.Join` line and the `path/filepath` import if `filepath` is unused (it is unused above — so delete both).

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/runtime/ -v`
Expected: FAIL — `undefined: Path`.

- [ ] **Step 3: Write the implementation** — `internal/runtime/runtime.go`:
```go
// Package runtime persists the running orchid daemon's pid and listen address.
package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
)

// Info records a running daemon.
type Info struct {
	PID  int    `json:"pid"`
	Addr string `json:"addr"`
}

// Path returns <config>/orchid/daemon.json (honors XDG_CONFIG_HOME, else ~/.config).
func Path() (string, error) {
	cfg := os.Getenv("XDG_CONFIG_HOME")
	if cfg == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		cfg = filepath.Join(home, ".config")
	}
	return filepath.Join(cfg, "orchid", "daemon.json"), nil
}

// Write atomically persists the daemon info.
func Write(i Info) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".daemon-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	return os.Rename(name, path)
}

// Read returns the persisted daemon info, or an os.IsNotExist error if absent.
func Read() (Info, error) {
	path, err := Path()
	if err != nil {
		return Info{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Info{}, err
	}
	var i Info
	if err := json.Unmarshal(data, &i); err != nil {
		return Info{}, err
	}
	return i, nil
}

// Remove deletes the runtime file; absent file is not an error.
func Remove() error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Alive reports whether a process with pid exists (signal-0 probe).
func Alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/runtime/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/runtime/
git commit -m "feat: add runtime file (daemon pid+addr) helpers"
```

---

## Task 2: internal/client

**Files:** Create `internal/client/client.go`, `internal/client/client_test.go`.

- [ ] **Step 1: Write the failing test** — `internal/client/client_test.go`:
```go
package client

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func stub(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New(strings.TrimPrefix(srv.URL, "http://"))
}

func TestList(t *testing.T) {
	c := stub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/api/sessions" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.Write([]byte(`[{"id":"a1","agent":"shell","cwd":"/tmp","status":"running","alive":true}]`))
	})
	infos, err := c.List()
	if err != nil || len(infos) != 1 || infos[0].ID != "a1" || infos[0].Agent != "shell" {
		t.Fatalf("List = %+v, %v", infos, err)
	}
}

func TestCreate(t *testing.T) {
	c := stub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method %s", r.Method)
		}
		w.Write([]byte(`{"id":"b2","agent":"claude","cwd":"/x","status":"running","alive":true}`))
	})
	info, err := c.Create("claude", "/x")
	if err != nil || info.ID != "b2" || info.Agent != "claude" {
		t.Fatalf("Create = %+v, %v", info, err)
	}
}

func TestCreateUnknownAgent400(t *testing.T) {
	c := stub(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unknown agent \"nope\"", http.StatusBadRequest)
	})
	if _, err := c.Create("nope", ""); err == nil {
		t.Fatal("expected error on 400")
	}
}

func TestKill404(t *testing.T) {
	c := stub(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "session \"x\" not found", http.StatusNotFound)
	})
	if err := c.Kill("x"); err == nil {
		t.Fatal("expected error on 404")
	}
}

func TestRestart409(t *testing.T) {
	c := stub(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "already running", http.StatusConflict)
	})
	if _, err := c.Restart("x"); err == nil {
		t.Fatal("expected error on 409")
	}
}

func TestKillOK(t *testing.T) {
	c := stub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("method %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	if err := c.Kill("a1"); err != nil {
		t.Fatalf("Kill: %v", err)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/client/ -v`
Expected: FAIL — `undefined: New`.

- [ ] **Step 3: Write the implementation** — `internal/client/client.go`:
```go
// Package client is a thin HTTP client over the orchid server's API.
package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"orchid/internal/runtime"
	"orchid/internal/session"
)

// ErrNotRunning is returned when no live daemon is recorded.
var ErrNotRunning = errors.New("orchid not running (run: orchid start)")

type Client struct {
	addr string
	hc   *http.Client
}

func New(addr string) *Client {
	return &Client{addr: addr, hc: &http.Client{Timeout: 10 * time.Second}}
}

// FromRuntime builds a client from the recorded daemon, or ErrNotRunning if the
// runtime file is absent or its pid is dead (a stale file is removed).
func FromRuntime() (*Client, error) {
	info, err := runtime.Read()
	if err != nil {
		return nil, ErrNotRunning
	}
	if !runtime.Alive(info.PID) {
		_ = runtime.Remove()
		return nil, ErrNotRunning
	}
	return New(info.Addr), nil
}

func (c *Client) BaseURL() string { return "http://" + c.addr }

func (c *Client) do(method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, c.BaseURL()+path, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, ErrNotRunning // connection refused etc.
	}
	return resp, nil
}

func errFromResp(resp *http.Response) error {
	b, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("%s", strings.TrimSpace(string(b)))
}

func (c *Client) List() ([]session.Info, error) {
	resp, err := c.do(http.MethodGet, "/api/sessions", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errFromResp(resp)
	}
	var infos []session.Info
	if err := json.NewDecoder(resp.Body).Decode(&infos); err != nil {
		return nil, err
	}
	return infos, nil
}

func (c *Client) Create(agent, cwd string) (session.Info, error) {
	body, _ := json.Marshal(map[string]string{"agent": agent, "cwd": cwd})
	resp, err := c.do(http.MethodPost, "/api/sessions", bytes.NewReader(body))
	if err != nil {
		return session.Info{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return session.Info{}, errFromResp(resp)
	}
	var info session.Info
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return session.Info{}, err
	}
	return info, nil
}

func (c *Client) Kill(id string) error {
	resp, err := c.do(http.MethodDelete, "/api/sessions/"+id, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return errFromResp(resp)
	}
	return nil
}

func (c *Client) Restart(id string) (session.Info, error) {
	resp, err := c.do(http.MethodPost, "/api/sessions/"+id+"/restart", nil)
	if err != nil {
		return session.Info{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return session.Info{}, errFromResp(resp)
	}
	var info session.Info
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return session.Info{}, err
	}
	return info, nil
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/client/ -v`
Expected: PASS (all 6 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/client/
git commit -m "feat: add HTTP client over orchid API"
```

---

## Task 3: route() dispatch + serve handler

**Files:** Modify `cmd/orchid/main.go`; create `cmd/orchid/cli.go`, `cmd/orchid/cli_test.go`.

**Context:** Current `main.go` is the server body. We move it into a `serve(host,port)` handler with graceful shutdown, and make `main` a thin router. The non-serve subcommands are stubbed in this task (return a "not implemented" code) and filled in Tasks 4–5 — EXCEPT the router and `serve` must work now. `cli_test.go` tests `route` dispatch.

- [ ] **Step 1: Write the failing test** — `cmd/orchid/cli_test.go`:
```go
package main

import "testing"

func TestRouteKnownCommands(t *testing.T) {
	for _, name := range []string{"serve", "start", "stop", "status", "open", "ls", "create", "rm", "restart"} {
		h, err := route([]string{name})
		if err != nil || h == nil {
			t.Fatalf("route(%q) = %v, %v; want handler", name, h, err)
		}
	}
}

func TestRouteNoArgsIsServe(t *testing.T) {
	h, err := route(nil)
	if err != nil || h == nil {
		t.Fatalf("route(nil) = %v, %v; want serve handler", h, err)
	}
}

func TestRouteFlagsAreServe(t *testing.T) {
	h, err := route([]string{"-port", "9000"})
	if err != nil || h == nil {
		t.Fatalf("route(-port) = %v, %v; want serve handler", h, err)
	}
}

func TestRouteUnknown(t *testing.T) {
	if _, err := route([]string{"bogus"}); err == nil {
		t.Fatal("unknown command should error")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./cmd/orchid/ -v`
Expected: FAIL — `undefined: route`.

- [ ] **Step 3: Replace `cmd/orchid/main.go`:**
```go
package main

import (
	"fmt"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	handler, err := route(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "orchid:", err)
		usage()
		return 2
	}
	return handler()
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: orchid <command> [args]

commands:
  serve [-host H -port P]   run the server in the foreground
  start [-host H -port P]   start the server as a background daemon
  stop                      stop the daemon
  status                    show daemon status and session count
  open                      open the dashboard in a browser
  ls                        list sessions
  create <agent> [cwd]      create a session
  rm <id>                   delete a session
  restart <id>              restart a stopped session
`)
}
```

- [ ] **Step 4: Create `cmd/orchid/cli.go` with the router and `serve`:**
```go
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"orchid/internal/server"
	"orchid/internal/session"
	"orchid/internal/store"
	"orchid/internal/tmux"
)

// route maps args to a handler returning an exit code. The first arg selects the
// subcommand; no args or a leading flag means `serve`.
func route(args []string) (func() int, error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return func() int { return serveCmd(args) }, nil
	}
	rest := args[1:]
	switch args[0] {
	case "serve":
		return func() int { return serveCmd(rest) }, nil
	case "start":
		return func() int { return startCmd(rest) }, nil
	case "stop":
		return func() int { return stopCmd(rest) }, nil
	case "status":
		return func() int { return statusCmd(rest) }, nil
	case "open":
		return func() int { return openCmd(rest) }, nil
	case "ls":
		return func() int { return lsCmd(rest) }, nil
	case "create":
		return func() int { return createCmd(rest) }, nil
	case "rm":
		return func() int { return rmCmd(rest) }, nil
	case "restart":
		return func() int { return restartCmd(rest) }, nil
	default:
		return nil, fmt.Errorf("unknown command %q", args[0])
	}
}

func sessionsPath() string {
	p, err := store.DefaultPath()
	if err != nil {
		cfg, _ := os.UserConfigDir()
		p = filepath.Join(cfg, "orchid", "sessions.json")
	}
	return p
}

func hostPort(args []string) (string, int, error) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	host := fs.String("host", "127.0.0.1", "listen host (localhost only by default)")
	port := fs.Int("port", 8081, "listen port")
	if err := fs.Parse(args); err != nil {
		return "", 0, err
	}
	return *host, *port, nil
}

// serveCmd runs the server in the foreground with graceful shutdown.
func serveCmd(args []string) int {
	host, port, err := hostPort(args)
	if err != nil {
		return 2
	}
	if err := tmux.Available(); err != nil {
		log.Printf("orchid requires tmux: %v (install tmux and retry)", err)
		return 1
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	mgr := session.NewManager(sessionsPath())
	if err := mgr.Restore(); err != nil {
		log.Printf("orchid: restore sessions: %v", err)
	}
	srv := &http.Server{Addr: addr, Handler: server.New(mgr)}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()

	log.Printf("orchid listening on http://%s (sessions: %s)", addr, sessionsPath())
	err = srv.ListenAndServe()
	mgr.KillAll() // closes control clients; tmux sessions survive
	if err != nil && err != http.ErrServerClosed {
		log.Print(err)
		return 1
	}
	return 0
}

// --- stubs filled in Tasks 4-5 ---
func startCmd(args []string) int   { fmt.Fprintln(os.Stderr, "not implemented"); return 1 }
func stopCmd(args []string) int    { fmt.Fprintln(os.Stderr, "not implemented"); return 1 }
func statusCmd(args []string) int  { fmt.Fprintln(os.Stderr, "not implemented"); return 1 }
func openCmd(args []string) int    { fmt.Fprintln(os.Stderr, "not implemented"); return 1 }
func lsCmd(args []string) int      { fmt.Fprintln(os.Stderr, "not implemented"); return 1 }
func createCmd(args []string) int  { fmt.Fprintln(os.Stderr, "not implemented"); return 1 }
func rmCmd(args []string) int      { fmt.Fprintln(os.Stderr, "not implemented"); return 1 }
func restartCmd(args []string) int { fmt.Fprintln(os.Stderr, "not implemented"); return 1 }
```

- [ ] **Step 5: Run tests + build**

Run: `go build ./... && go test ./cmd/orchid/ -v`
Expected: build clean; route tests PASS.

- [ ] **Step 6: Manual: foreground serve still works**

Run: `go build -o orchid ./cmd/orchid && ./orchid serve -port 18140 & sleep 1; curl -sf -o /dev/null -w '%{http_code}\n' http://127.0.0.1:18140/; kill %1`
Expected: `200`. (Also `./orchid -port ...` works via the no-subcommand path.)

- [ ] **Step 7: Commit**

```bash
git add cmd/orchid/
git commit -m "feat: CLI router + serve with graceful shutdown"
```

---

## Task 4: start / stop / status

**Files:** Modify `cmd/orchid/cli.go` (replace the three stubs).

- [ ] **Step 1: Replace the `startCmd`, `stopCmd`, `statusCmd` stubs** in `cmd/orchid/cli.go` with:
```go
func startCmd(args []string) int {
	host, port, err := hostPort(args)
	if err != nil {
		return 2
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	if info, err := runtime.Read(); err == nil && runtime.Alive(info.PID) {
		fmt.Fprintf(os.Stderr, "orchid: already running on http://%s\n", info.Addr)
		return 1
	}

	self, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "orchid:", err)
		return 1
	}
	logPath := filepath.Join(filepath.Dir(sessionsPath()), "orchid.log")
	logf, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Fprintln(os.Stderr, "orchid:", err)
		return 1
	}
	defer logf.Close()

	cmd := exec.Command(self, "serve", "-host", host, "-port", strconv.Itoa(port))
	cmd.Stdout = logf
	cmd.Stderr = logf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "orchid:", err)
		return 1
	}

	// Wait until the server is listening (up to ~3s).
	listening := false
	for i := 0; i < 30; i++ {
		conn, derr := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if derr == nil {
			conn.Close()
			listening = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !listening {
		_ = cmd.Process.Kill()
		fmt.Fprintf(os.Stderr, "orchid: server did not start; see %s\n", logPath)
		return 1
	}
	if err := runtime.Write(runtime.Info{PID: cmd.Process.Pid, Addr: addr}); err != nil {
		fmt.Fprintln(os.Stderr, "orchid:", err)
		return 1
	}
	fmt.Printf("orchid started on http://%s\n", addr)
	return 0
}

func stopCmd(args []string) int {
	info, err := runtime.Read()
	if err != nil || !runtime.Alive(info.PID) {
		_ = runtime.Remove()
		fmt.Println("orchid not running")
		return 0
	}
	if err := syscall.Kill(info.PID, syscall.SIGTERM); err != nil {
		fmt.Fprintln(os.Stderr, "orchid:", err)
		return 1
	}
	for i := 0; i < 30 && runtime.Alive(info.PID); i++ {
		time.Sleep(100 * time.Millisecond)
	}
	if runtime.Alive(info.PID) {
		_ = syscall.Kill(info.PID, syscall.SIGKILL)
	}
	_ = runtime.Remove()
	fmt.Println("orchid stopped")
	return 0
}

func statusCmd(args []string) int {
	info, err := runtime.Read()
	if err != nil || !runtime.Alive(info.PID) {
		_ = runtime.Remove()
		fmt.Println("orchid: not running")
		return 0
	}
	fmt.Printf("orchid: running on http://%s (pid %d)\n", info.Addr, info.PID)
	if c, err := client.FromRuntime(); err == nil {
		if sessions, err := c.List(); err == nil {
			fmt.Printf("sessions: %d\n", len(sessions))
		}
	}
	return 0
}
```

Add the imports `exec "os/exec"`, `"time"`, `"orchid/internal/runtime"`, `"orchid/internal/client"` to `cli.go` (use `os/exec` as `exec`). Remove the `startCmd/stopCmd/statusCmd` stub lines.

- [ ] **Step 2: Build**

Run: `go build ./...`
Expected: clean.

- [ ] **Step 3: Manual start/stop/status cycle**

Run:
```bash
go build -o orchid ./cmd/orchid
./orchid start -port 18141
./orchid status
./orchid stop
./orchid status
```
Expected: "started on http://127.0.0.1:18141"; "running on ... (pid N)" + "sessions: 0"; "stopped"; "not running". After stop, `~/.config/orchid/daemon.json` is gone (`ls "$HOME/.config/orchid/"`).

- [ ] **Step 4: Commit**

```bash
git add cmd/orchid/cli.go
git commit -m "feat: orchid start/stop/status daemon commands"
```

---

## Task 5: open / ls / create / rm / restart

**Files:** Modify `cmd/orchid/cli.go` (replace the five stubs).

- [ ] **Step 1: Replace the `openCmd`, `lsCmd`, `createCmd`, `rmCmd`, `restartCmd` stubs** in `cmd/orchid/cli.go` with:
```go
func openCmd(args []string) int {
	c, err := client.FromRuntime()
	if err != nil {
		fmt.Fprintln(os.Stderr, "orchid:", err)
		return 1
	}
	url := c.BaseURL()
	var opener string
	switch goos := runtimeGOOS(); goos {
	case "darwin":
		opener = "open"
	default:
		opener = "xdg-open"
	}
	if _, err := exec.LookPath(opener); err != nil {
		fmt.Println(url)
		return 0
	}
	if err := exec.Command(opener, url).Start(); err != nil {
		fmt.Println(url)
	}
	return 0
}

func lsCmd(args []string) int {
	c, err := client.FromRuntime()
	if err != nil {
		fmt.Fprintln(os.Stderr, "orchid:", err)
		return 1
	}
	infos, err := c.List()
	if err != nil {
		fmt.Fprintln(os.Stderr, "orchid:", err)
		return 1
	}
	if len(infos) == 0 {
		fmt.Println("no sessions")
		return 0
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tAGENT\tSTATUS\tCWD")
	for _, s := range infos {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.ID, s.Agent, s.Status, s.Cwd)
	}
	w.Flush()
	return 0
}

func createCmd(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: orchid create <agent> [cwd]")
		return 2
	}
	cwd := ""
	if len(args) >= 2 {
		cwd = args[1]
	}
	c, err := client.FromRuntime()
	if err != nil {
		fmt.Fprintln(os.Stderr, "orchid:", err)
		return 1
	}
	info, err := c.Create(args[0], cwd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "orchid:", err)
		return 1
	}
	fmt.Println(info.ID)
	return 0
}

func rmCmd(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: orchid rm <id>")
		return 2
	}
	c, err := client.FromRuntime()
	if err != nil {
		fmt.Fprintln(os.Stderr, "orchid:", err)
		return 1
	}
	if err := c.Kill(args[0]); err != nil {
		fmt.Fprintln(os.Stderr, "orchid:", err)
		return 1
	}
	fmt.Printf("%s removed\n", args[0])
	return 0
}

func restartCmd(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: orchid restart <id>")
		return 2
	}
	c, err := client.FromRuntime()
	if err != nil {
		fmt.Fprintln(os.Stderr, "orchid:", err)
		return 1
	}
	info, err := c.Restart(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "orchid:", err)
		return 1
	}
	fmt.Printf("%s %s\n", info.ID, info.Status)
	return 0
}
```

Add imports `"text/tabwriter"` and `"runtime"` to `cli.go`. Because the package already imports `orchid/internal/runtime` (named `runtime` by default), ALIAS the stdlib one: import stdlib as `goruntime "runtime"` and define `func runtimeGOOS() string { return goruntime.GOOS }`. (This avoids the import-name clash with `orchid/internal/runtime`.)

- [ ] **Step 2: Build + vet**

Run: `go build ./... && go vet ./...`
Expected: clean (no import clash).

- [ ] **Step 3: Manual full cycle**

Run:
```bash
go build -o orchid ./cmd/orchid
./orchid start -port 18142
./orchid create shell /tmp
./orchid ls
ID=$(./orchid ls | awk 'NR==2{print $1}')
./orchid restart "$ID"   # likely 409 already running -> error message, exit 1 (expected for a running session)
./orchid rm "$ID"
./orchid ls               # no sessions
./orchid stop
```
Expected: create prints an id; ls shows it; rm removes it; final ls "no sessions"; stop "stopped". Clean up any leftover tmux: `tmux ls 2>/dev/null | grep '^orchid-' | cut -d: -f1 | xargs -r -n1 tmux kill-session -t`.

- [ ] **Step 4: Commit**

```bash
git add cmd/orchid/cli.go
git commit -m "feat: orchid open/ls/create/rm/restart session commands"
```

---

## Task 6: Full build, vet, race

- [ ] **Step 1: Whole repo**

Run: `go build ./... && go vet ./... && go test ./... -race`
Expected: all PASS, no races. New packages `internal/runtime`, `internal/client`, `cmd/orchid` covered.

- [ ] **Step 2: No leftover test tmux**

Run: `tmux ls 2>/dev/null | grep '^orchid-' && echo LEAK || echo clean`
Expected: `clean`.

---

## Task 7: End-to-end daemon smoke

**Files:** none (verification).

- [ ] **Step 1: Lifecycle + persistence**

Run:
```bash
rm -f "$HOME/.config/orchid/daemon.json" "$HOME/.config/orchid/sessions.json"
go build -o orchid ./cmd/orchid
./orchid start -port 18143
./orchid status                         # running, 0 sessions
ID=$(./orchid create shell /tmp); echo "created $ID"
tmux has-session -t "orchid-$ID" && echo "tmux ok"
./orchid ls
./orchid stop                           # daemon gone
test ! -f "$HOME/.config/orchid/daemon.json" && echo "runtime file removed"
tmux has-session -t "orchid-$ID" && echo "tmux session SURVIVED stop (persistence)"
./orchid start -port 18143              # restart daemon
./orchid ls                             # session reattached (running)
./orchid rm "$ID"
./orchid stop
```
Expected: status running→0; create prints id; tmux session exists and SURVIVES `stop`; runtime file removed on stop; after second `start`, `ls` shows the session reattached; `rm` then `stop` clean. Final cleanup:
```bash
tmux ls 2>/dev/null | grep '^orchid-' | cut -d: -f1 | xargs -r -n1 tmux kill-session -t
rm -f orchid
```

- [ ] **Step 2: Commit (only if fixes were needed)**

No code change expected. Fix under the relevant task and re-run if the smoke surfaced a bug.

---

## Self-Review Notes

- **Spec coverage:** runtime file pid+addr (T1); HTTP client + error mapping + ErrNotRunning (T2); router + `serve` with graceful SIGTERM/SIGINT shutdown + no-arg/flag→serve alias (T3); `start` daemonize/wait-for-listen, `stop` SIGTERM+remove, `status` (T4); `open`/`ls`/`create`/`rm`/`restart` (T5); build/vet/race gate (T6); E2E incl. stop-survives-tmux + restart-reattach (T7). All spec sections mapped.
- **Import-clash note:** `cli.go` imports both `orchid/internal/runtime` (as `runtime`) and stdlib `runtime` (aliased `goruntime`) — explicitly handled in T5 to avoid a collision; also `os/exec` imported as `exec`.
- **Type consistency:** `runtime.Info{PID,Addr}`, `runtime.Path/Write/Read/Remove/Alive`, `client.New/FromRuntime/BaseURL/List/Create/Kill/Restart`, `client.ErrNotRunning`, `session.Info{ID,Agent,Cwd,Status,Alive}` used consistently across client/cli; subcommand handlers all `func([]string) int`.
- **Deviation from spec testing:** the spec listed a unit test for the serve signal-shutdown hook; ListenAndServe + signals don't unit-test cleanly, so graceful shutdown + tmux survival is covered by the T7 E2E (`stop` then verify daemon gone, runtime file removed, tmux session survived) instead. Flagged so it isn't a silent gap.
- **YAGNI:** stdlib only; no daemon supervision/respawn; `start` is fire-and-poll, not a supervisor.
```
