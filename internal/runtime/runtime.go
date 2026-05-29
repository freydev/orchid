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
