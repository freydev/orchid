// Package statusclient posts agent status signals to a running orchid server.
// It is used by the `orchid signal` subcommand and the `orchid mcp` server.
package statusclient

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// Post reports a session's state to the orchid API at baseURL. A blank baseURL
// or id is a no-op (the agent is not running under orchid). Best-effort: a short
// timeout, errors returned but safe to ignore.
func Post(baseURL, id, state string) error {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" || id == "" {
		return nil
	}
	body, err := json.Marshal(map[string]any{"state": state})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/sessions/"+id+"/status", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c := &http.Client{Timeout: 2 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
