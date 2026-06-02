package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	exec "os/exec"
	"os/signal"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"orchid/internal/agent"
	"orchid/internal/client"
	"orchid/internal/config"
	"orchid/internal/gitwt"
	"orchid/internal/mcp"
	"orchid/internal/project"
	"orchid/internal/runtime"
	"orchid/internal/server"
	"orchid/internal/session"
	"orchid/internal/statusclient"
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
	case "signal":
		return func() int { return signalCmd(rest) }, nil
	case "notify":
		return func() int { return notifyCmd(rest) }, nil
	case "mcp":
		return func() int { return mcpCmd(rest) }, nil
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

func projectsPath() string {
	cfg, _ := os.UserConfigDir()
	return filepath.Join(cfg, "orchid", "projects.json")
}

func hostPort(args []string) (string, int, error) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	host := fs.String("host", "127.0.0.1", "listen host (localhost only by default)")
	port := fs.Int("port", 8081, "listen port")
	if err := fs.Parse(args); err != nil {
		return "", 0, err
	}
	// Saved settings (config.json) fill in any value not explicitly passed as a
	// flag, so `orchid start` honors the port/host chosen in the settings UI.
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	cfg := config.Load()
	if !set["host"] && cfg.Host != "" {
		*host = cfg.Host
	}
	if !set["port"] && cfg.Port != 0 {
		*port = cfg.Port
	}
	return *host, *port, nil
}

// signalCmd posts an agent status signal for the current session. Invoked by the
// claude plugin hooks: `orchid signal running|done|requires_action`. Reads
// ORCHID_SESSION and ORCHID_URL from the environment (injected into the tmux
// session). Always exits 0 so a status hiccup never disrupts the agent.
func signalCmd(args []string) int {
	if len(args) < 1 {
		return 0
	}
	state := args[0]
	switch state {
	case "running", "done", "requires_action", "idle":
	default:
		return 0
	}
	id := os.Getenv("ORCHID_SESSION")
	url := os.Getenv("ORCHID_URL")
	_ = statusclient.Post(url, id, state)
	return 0
}

// notifyCmd asks orchid to show a desktop notification that this session's agent
// finished. Invoked by the claude plugin's Stop hook: `orchid notify [message]`.
// Reads ORCHID_SESSION and ORCHID_URL from the environment. Always exits 0.
func notifyCmd(args []string) int {
	msg := ""
	if len(args) > 0 {
		msg = strings.Join(args, " ")
	}
	_ = statusclient.Notify(os.Getenv("ORCHID_URL"), os.Getenv("ORCHID_SESSION"), msg)
	return 0
}

// mcpCmd runs the stdio MCP server used by the claude plugin's .mcp.json.
func mcpCmd(args []string) int {
	if err := mcp.Serve(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "orchid mcp:", err)
		return 1
	}
	return 0
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
	if err := gitwt.Available(); err != nil {
		log.Printf("orchid requires git: %v", err)
		return 1
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	self, _ := os.Executable()
	mgr := session.NewManager(sessionsPath())
	if self != "" {
		mgr.SetEnv("http://"+addr, self)
		if os.Getenv("ORCHID_PLUGIN_DIR") == "" {
			cand := filepath.Join(filepath.Dir(self), "plugins", "orchid-status")
			if st, err := os.Stat(cand); err == nil && st.IsDir() {
				_ = os.Setenv("ORCHID_PLUGIN_DIR", cand)
			}
		}
	}
	ps := project.New(projectsPath())
	if err := ps.Load(); err != nil {
		log.Printf("orchid: restore projects: %v", err)
	}
	// Resolve a "shell" session's program: session choice -> project shell ->
	// global default, then to an absolute path.
	mgr.SetShellResolver(func(projectID, sessionShell string) string {
		choice := strings.TrimSpace(sessionShell)
		if choice == "" {
			if p, ok := ps.Get(projectID); ok {
				choice = strings.TrimSpace(p.Shell)
			}
		}
		return agent.ResolveShellName(choice)
	})
	if err := mgr.Restore(); err != nil {
		log.Printf("orchid: restore sessions: %v", err)
	}
	if n := mgr.PruneOrphanTmux(); n > 0 {
		log.Printf("orchid: pruned %d orphan tmux session(s) not in sessions.json", n)
	}
	mgr.SyncTitles() // backfill claude session titles at startup
	handler := server.New(mgr, ps)
	handler.SetRuntime(self, host, port)
	srv := &http.Server{Addr: addr, Handler: handler}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()

	log.Printf("orchid listening on http://%s (sessions: %s)", addr, sessionsPath())
	err = srv.ListenAndServe()
	mgr.KillAll()
	if err != nil && err != http.ErrServerClosed {
		log.Print(err)
		return 1
	}
	return 0
}

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
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "orchid:", err)
		return 1
	}
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

	exited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(exited) }()

	listening := false
	for i := 0; i < 30; i++ {
		select {
		case <-exited:
			fmt.Fprintf(os.Stderr, "orchid: server exited during startup; see %s\n", logPath)
			return 1
		default:
		}
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
	// Final check: child still alive before we record it.
	select {
	case <-exited:
		fmt.Fprintf(os.Stderr, "orchid: server exited during startup; see %s\n", logPath)
		return 1
	default:
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
func runtimeGOOS() string { return goruntime.GOOS }

func openCmd(args []string) int {
	c, err := client.FromRuntime()
	if err != nil {
		fmt.Fprintln(os.Stderr, "orchid:", err)
		return 1
	}
	url := c.BaseURL()
	var opener string
	switch runtimeGOOS() {
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
	if len(args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: orchid create <project> <worktree> <agent>")
		return 2
	}
	c, err := client.FromRuntime()
	if err != nil {
		fmt.Fprintln(os.Stderr, "orchid:", err)
		return 1
	}
	info, err := c.Create(args[0], args[1], args[2])
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
