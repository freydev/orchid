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
	Agent          string    `json:"agent"`
	Cwd            string    `json:"cwd"`
	Tmux           string    `json:"tmux"`
	AgentSessionID string    `json:"agent_session_id"`
	Created        time.Time `json:"created"`
	Status         string    `json:"status"` // "running" | "stopped"
}

// DefaultPath returns <user config dir>/orchid/sessions.json.
func DefaultPath() (string, error) {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", err
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
