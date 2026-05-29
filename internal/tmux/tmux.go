// Package tmux is a thin wrapper over the system tmux CLI.
package tmux

import (
	"fmt"
	"os/exec"
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
// cwd, running argv directly (no shell interpolation).
func NewSession(name, cwd string, argv []string) error {
	args := []string{"new-session", "-d", "-s", name, "-c", cwd, "--"}
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

// KillSession terminates the named tmux session.
func KillSession(name string) error {
	out, err := exec.Command("tmux", "kill-session", "-t", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("tmux kill-session: %w: %s", err, out)
	}
	return nil
}
