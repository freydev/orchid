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
	// Shell overrides the program launched for "shell" sessions (e.g.
	// "/opt/homebrew/bin/fish"). Empty falls back to $SHELL, then /bin/bash.
	Shell string `json:"shell"`
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
