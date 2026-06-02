package server

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"syscall"

	"orchid/internal/agent"
	"orchid/internal/config"
)

type settingsOut struct {
	Shell     string   `json:"shell"`      // global default shell choice
	Accent    string   `json:"accent"`     // UI accent color id
	RailWidth int      `json:"rail_width"` // Terminal sidebar width (px)
	Host      string   `json:"host"`       // current bind host
	Port      int      `json:"port"`       // current bind port
	Addr      string   `json:"addr"`       // host:port the daemon is listening on
	Shells    []string `json:"shells"`     // shell names detected on this system
}

type settingsReq struct {
	Shell  string `json:"shell"`
	Accent string `json:"accent"`
	Port   int    `json:"port"`
	Host   string `json:"host"`
}

// defaultAccent is the accent color id used when none is configured.
const defaultAccent = "pink"

// effHost / effPort prefer the persisted config, falling back to how the daemon
// was actually launched.
func (s *Server) effHost() string {
	if h := config.Load().Host; h != "" {
		return h
	}
	if s.host != "" {
		return s.host
	}
	return "127.0.0.1"
}
func (s *Server) effPort() int {
	if p := config.Load().Port; p != 0 {
		return p
	}
	if s.port != 0 {
		return s.port
	}
	return 8081
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	host, port := s.host, s.port
	if host == "" {
		host = s.effHost()
	}
	if port == 0 {
		port = s.effPort()
	}
	accent := config.Load().Accent
	if accent == "" {
		accent = defaultAccent
	}
	writeJSON(w, http.StatusOK, settingsOut{
		Shell:     config.Shell(),
		Accent:    accent,
		RailWidth: config.Load().RailWidth,
		Host:      host,
		Port:      port,
		Addr:      net.JoinHostPort(host, strconv.Itoa(port)),
		Shells:    agent.AvailableShells(),
	})
}

func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var req settingsReq
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	c := config.Load()
	c.Shell = req.Shell
	if req.Accent != "" {
		c.Accent = req.Accent
	}
	if req.Port != 0 {
		c.Port = req.Port
	}
	if req.Host != "" {
		c.Host = req.Host
	}
	if err := config.Save(c); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// report whether a restart is needed to apply the new bind address
	restartNeeded := (req.Port != 0 && req.Port != s.port) || (req.Host != "" && req.Host != s.host)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "restart_needed": restartNeeded})
}

// handleRailWidth persists just the Terminal sidebar width, without touching
// other settings (so frequent drag-saves don't clobber shell/accent).
func (s *Server) handleRailWidth(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Width int `json:"width"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	if req.Width < 160 || req.Width > 600 {
		http.Error(w, "width out of range", http.StatusBadRequest)
		return
	}
	c := config.Load()
	c.RailWidth = req.Width
	if err := config.Save(c); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleRestart relaunches the daemon out-of-process so a new port/host (saved
// via PUT /api/settings) takes effect. It spawns a detached helper that stops
// this daemon and starts a fresh one, then returns immediately. No-op when the
// self path is unknown (e.g. running via `go run` in tests).
func (s *Server) handleRestartDaemon(w http.ResponseWriter, r *http.Request) {
	if s.selfPath == "" {
		http.Error(w, "restart unavailable: server binary path unknown", http.StatusServiceUnavailable)
		return
	}
	host := s.effHost()
	port := strconv.Itoa(s.effPort())
	// sleep lets the HTTP response flush before this daemon is torn down.
	script := fmt.Sprintf("sleep 0.6; %q stop >/dev/null 2>&1; %q start -host %q -port %s >/dev/null 2>&1",
		s.selfPath, s.selfPath, host, port)
	cmd := exec.Command("sh", "-c", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "addr": net.JoinHostPort(host, port)})
}
