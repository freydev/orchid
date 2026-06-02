package agent

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
)

// claudeTranscript locates ~/.claude/projects/*/<id>.jsonl (the uuid is unique).
func claudeTranscript(agentSessionID string) string {
	if agentSessionID == "" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	m, _ := filepath.Glob(filepath.Join(home, ".claude", "projects", "*", agentSessionID+".jsonl"))
	if len(m) > 0 {
		return m[0]
	}
	return ""
}

// ClaudeTitle returns claude's auto-generated session title — the "ai-title"
// shown by `claude -r` — for the given agent session id, or "" if none is
// found. It reads only the tail of the transcript (titles are re-emitted
// throughout the conversation, so a recent one is in the last chunk).
func ClaudeTitle(agentSessionID string) string {
	path := claudeTranscript(agentSessionID)
	if path == "" {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return ""
	}
	const tail = 64 * 1024
	off := int64(0)
	if st.Size() > tail {
		off = st.Size() - tail
	}
	buf := make([]byte, st.Size()-off)
	if _, err := f.ReadAt(buf, off); err != nil && err != io.EOF {
		return ""
	}
	title := ""
	for _, line := range bytes.Split(buf, []byte("\n")) {
		if !bytes.Contains(line, []byte(`"ai-title"`)) {
			continue // skip non-title lines (and the partial leading line)
		}
		var ev struct {
			Type    string `json:"type"`
			AiTitle string `json:"aiTitle"`
		}
		if json.Unmarshal(bytes.TrimSpace(line), &ev) == nil && ev.Type == "ai-title" && ev.AiTitle != "" {
			title = ev.AiTitle // keep the last one
		}
	}
	return title
}
