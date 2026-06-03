package agent

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ClaudeSession is a resumable claude conversation on disk.
type ClaudeSession struct {
	ID      string    `json:"id"`
	Title   string    `json:"title"`
	Updated time.Time `json:"updated"`
}

// encodeProjectDir maps a working directory to claude's transcript folder name
// under ~/.claude/projects: every "/" and "." becomes "-".
func encodeProjectDir(cwd string) string {
	return strings.NewReplacer("/", "-", ".", "-").Replace(cwd)
}

// ClaudeSessions lists resumable claude conversations recorded for cwd, newest
// first. Returns nil when none exist or the cwd is blank.
func ClaudeSessions(cwd string) []ClaudeSession {
	cwd = strings.TrimSpace(cwd)
	if cwd == "" {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	dir := filepath.Join(home, ".claude", "projects", encodeProjectDir(cwd))
	matches, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	out := make([]ClaudeSession, 0, len(matches))
	for _, p := range matches {
		st, err := os.Stat(p)
		if err != nil || st.Size() == 0 {
			continue
		}
		out = append(out, ClaudeSession{
			ID:      strings.TrimSuffix(filepath.Base(p), ".jsonl"),
			Title:   titleFromFile(p),
			Updated: st.ModTime().UTC(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Updated.After(out[j].Updated) })
	return out
}
