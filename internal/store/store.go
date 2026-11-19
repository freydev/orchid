// Package store persists session records as JSON.
package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Record is a persisted session. Status is the last-known value; the Manager
// reconciles it against live tmux on load.
type Record struct {
	ID             string    `json:"id"`
	Name           string    `json:"name,omitempty"`
	Agent          string    `json:"agent"`
	Cwd            string    `json:"cwd"`
	Project        string    `json:"project"`
	Worktree       string    `json:"worktree"`
	Tmux           string    `json:"tmux"`
	AgentSessionID string    `json:"agent_session_id"`
	Created        time.Time `json:"created"`
	Status         string    `json:"status"` // "running" | "stopped"
	Order          int       `json:"order,omitempty"`
	Signal         string    `json:"signal,omitempty"`    // persisted "requires_action" so it survives restart
	Color          string    `json:"color,omitempty"`     // accent color id ("blue"...), overrides agent color
	Shell          string    `json:"shell,omitempty"`     // shell choice for "shell" sessions ("" = inherit)
	Command        string    `json:"command,omitempty"`   // preset command run in this shell session (via `shell -c`)
	Dangerous      bool      `json:"dangerous,omitempty"` // launch agent with "dangerously skip permissions" (claude/codex); survives restart
	Group          string    `json:"group,omitempty"`     // project group id ("" = ungrouped)
}

// DefaultPath returns the sessions file under ~/.config/orchid (honoring
// $XDG_CONFIG_HOME when set). We use ~/.config explicitly rather than
// os.UserConfigDir() so the location is consistent across platforms (on macOS
// UserConfigDir would be ~/Library/Application Support).
func DefaultPath() (string, error) {
	cfg := os.Getenv("XDG_CONFIG_HOME")
	if cfg == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		cfg = filepath.Join(home, ".config")
	}
	return filepath.Join(cfg, "orchid", "sessions.json"), nil
}

// Load reads records from path. A missing file yields an empty slice, no error.
func Load(path string) ([]Record, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	var recs []Record
	if err := json.Unmarshal(data, &recs); err != nil {
		return nil, err
	}
	return recs, nil
}

// Save writes records to path atomically (temp file + rename), creating parent
// directories as needed.
func Save(path string, recs []Record) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(recs, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".sessions-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}
