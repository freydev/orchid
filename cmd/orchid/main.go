package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"orchid/internal/server"
	"orchid/internal/session"
	"orchid/internal/store"
	"orchid/internal/tmux"
)

func main() {
	host := flag.String("host", "127.0.0.1", "listen host (localhost only by default)")
	port := flag.Int("port", 8081, "listen port")
	flag.Parse()

	if err := tmux.Available(); err != nil {
		log.Fatalf("orchid requires tmux: %v (install tmux and retry)", err)
	}

	addr := net.JoinHostPort(*host, strconv.Itoa(*port))

	sessPath, err := store.DefaultPath()
	if err != nil {
		cfg, _ := os.UserConfigDir()
		sessPath = filepath.Join(cfg, "orchid", "sessions.json")
	}
	mgr := session.NewManager(sessPath)
	if err := mgr.Restore(); err != nil {
		log.Printf("orchid: restore sessions: %v", err)
	}
	defer mgr.KillAll()

	h := server.New(mgr)
	log.Printf("orchid listening on http://%s (sessions: %s)", addr, sessPath)
	if err := http.ListenAndServe(addr, h); err != nil {
		log.Fatal(err)
	}
}
