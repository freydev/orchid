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
	mgr.KillAll()
	if err != nil && err != http.ErrServerClosed {
		log.Print(err)
		return 1
	}
	return 0
}

// --- stubs filled in later tasks ---
func startCmd(args []string) int   { fmt.Fprintln(os.Stderr, "not implemented"); return 1 }
func stopCmd(args []string) int    { fmt.Fprintln(os.Stderr, "not implemented"); return 1 }
func statusCmd(args []string) int  { fmt.Fprintln(os.Stderr, "not implemented"); return 1 }
func openCmd(args []string) int    { fmt.Fprintln(os.Stderr, "not implemented"); return 1 }
func lsCmd(args []string) int      { fmt.Fprintln(os.Stderr, "not implemented"); return 1 }
func createCmd(args []string) int  { fmt.Fprintln(os.Stderr, "not implemented"); return 1 }
func rmCmd(args []string) int      { fmt.Fprintln(os.Stderr, "not implemented"); return 1 }
func restartCmd(args []string) int { fmt.Fprintln(os.Stderr, "not implemented"); return 1 }
