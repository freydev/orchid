// Package tmux is a thin wrapper over the system tmux CLI.
package tmux

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Available reports whether the tmux binary is usable.
func Available() error {
	if _, err := exec.LookPath("tmux"); err != nil {
		return fmt.Errorf("tmux not found in PATH: %w", err)
	}
	if err := exec.Command("tmux", "-V").Run(); err != nil {
		return fmt.Errorf("tmux -V failed: %w", err)
	}
	return nil
}

// NewSession creates a detached tmux session named name, with working directory
// cwd, exporting each "K=V" entry of env into the session, and running argv
// directly (no shell interpolation).
func NewSession(name, cwd string, env []string, argv []string) error {
	args := []string{"new-session", "-d", "-s", name, "-c", cwd}
	for _, e := range env {
		args = append(args, "-e", e)
	}
	args = append(args, "--")
	args = append(args, argv...)
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

// HasSession reports whether a tmux session with the given name exists.
func HasSession(name string) bool {
	return exec.Command("tmux", "has-session", "-t", name).Run() == nil
}

// ListSessions returns the names of all tmux sessions on the default server.
// A missing server (no sessions) yields an empty slice and no error.
func ListSessions() ([]string, error) {
	out, err := exec.Command("tmux", "list-sessions", "-F", "#{session_name}").Output()
	if err != nil {
		return nil, nil // "no server running" / no sessions — not an error here
	}
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			names = append(names, line)
		}
	}
	return names, nil
}

// panePID returns the pid of the first pane's process in session name, or 0.
// tmux runs each pane in its own session/process group (forkpty + setsid), so
// this pid is a process-group leader and `kill(-pid, ...)` hits the whole tree.
func panePID(name string) int {
	out, err := exec.Command("tmux", "list-panes", "-t", name, "-F", "#{pane_pid}").Output()
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if pid, e := strconv.Atoi(strings.TrimSpace(line)); e == nil && pid > 0 {
			return pid
		}
	}
	return 0
}

func pidAlive(pid int) bool { return syscall.Kill(pid, 0) == nil }

// waitGone polls until pid exits or the deadline passes; returns true if gone.
func waitGone(pid int, d time.Duration) bool {
	for end := time.Now().Add(d); time.Now().Before(end); {
		if !pidAlive(pid) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return !pidAlive(pid)
}

// KillSession terminates the named tmux session AND its pane process tree,
// letting the program clean up after itself first. `tmux kill-session` alone
// only sends SIGHUP, which many servers (e.g. puma) trap to reload rather than
// exit — leaving the listening port bound. So signal the pane's process group
// in escalating order, stopping as soon as it exits:
//
//	SIGINT  (like Ctrl-C — best self-cleanup for dev servers/agents)
//	SIGTERM (graceful termination)
//	SIGKILL (force, last resort)
//
// then kill the tmux session for cleanup.
func KillSession(name string) error {
	if pid := panePID(name); pid > 0 {
		_ = syscall.Kill(-pid, syscall.SIGINT)
		if !waitGone(pid, 2*time.Second) {
			_ = syscall.Kill(-pid, syscall.SIGTERM)
			if !waitGone(pid, 3*time.Second) {
				_ = syscall.Kill(-pid, syscall.SIGKILL)
				waitGone(pid, 1*time.Second)
			}
		}
	}
	out, err := exec.Command("tmux", "kill-session", "-t", name).CombinedOutput()
	if err != nil {
		// session may already be gone (process tree exited) — only error if it
		// still exists.
		if HasSession(name) {
			return fmt.Errorf("tmux kill-session: %w: %s", err, out)
		}
	}
	return nil
}
