// Package config loads orchid's user configuration from a JSON file under
// ~/.config/orchid (honoring $XDG_CONFIG_HOME). The file is optional; a missing
// or malformed file yields a zero Config with no error.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Config is orchid's user configuration.
type Config struct {
	// Shell overrides the program launched for "shell" sessions. May be a bare
	// name ("fish") resolved on PATH, or an absolute path
	// ("/opt/homebrew/bin/fish"). Empty falls back to $SHELL, then /bin/bash.
	Shell string `json:"shell"`
	// Host is the listen host the daemon (re)starts on. Empty -> 127.0.0.1.
	Host string `json:"host,omitempty"`
	// Port is the listen port the daemon (re)starts on. Zero -> 8081.
	Port int `json:"port,omitempty"`
	// Accent is the UI accent color id ("pink", "blue"...). Empty -> "pink".
	Accent string `json:"accent,omitempty"`
	// RailWidth is the Terminal tab sidebar width in px. Zero -> UI default.
	RailWidth int `json:"rail_width,omitempty"`
}

// DefaultPath returns the config file path: ~/.config/orchid/config.json
// (or $XDG_CONFIG_HOME/orchid/config.json when set).
func DefaultPath() (string, error) {
	cfg := os.Getenv("XDG_CONFIG_HOME")
	if cfg == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		cfg = filepath.Join(home, ".config")
	}
	return filepath.Join(cfg, "orchid", "config.json"), nil
}

// Load reads the config file. A missing or unreadable/invalid file yields a
// zero Config and no error — config is best-effort, never fatal.
func Load() Config {
	p, err := DefaultPath()
	if err != nil {
		return Config{}
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return Config{}
	}
	var c Config
	_ = json.Unmarshal(data, &c)
	return c
}

// Shell returns the configured shell, trimmed, or "" when unset.
func Shell() string {
	return strings.TrimSpace(Load().Shell)
}

// Save writes the config file atomically (temp + rename), creating the parent
// directory as needed.
func Save(c Config) error {
	p, err := DefaultPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	c.Shell = strings.TrimSpace(c.Shell)
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".config-*.tmp")
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
	return os.Rename(name, p)
}
