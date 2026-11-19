package agent

import (
	"bufio"
	"encoding/json"
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
	Cwd     string    `json:"cwd"` // working dir the conversation ran in (may be a subdir)
}

// encodeProjectDir maps a working directory to claude's transcript folder name
// under ~/.claude/projects: every "/" and "." becomes "-".
func encodeProjectDir(cwd string) string {
	return strings.NewReplacer("/", "-", ".", "-").Replace(cwd)
}

// cwdFromFile returns the working directory recorded in a claude transcript (the
// "cwd" field appears on message entries, not the first line). "" if not found.
func cwdFromFile(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 16<<20) // transcript lines can be large
	for i := 0; i < 50 && sc.Scan(); i++ {
		var e struct {
			Cwd string `json:"cwd"`
		}
		if json.Unmarshal(sc.Bytes(), &e) == nil && e.Cwd != "" {
			return e.Cwd
		}
	}
	return ""
}

// ClaudeSessions lists resumable claude conversations recorded for root AND any
// nested directory under it, newest first. Claude stores transcripts per launch
// cwd, so a conversation started in a project subfolder lives in a sibling
// ~/.claude/projects/<enc> folder; we match those by encoded-name prefix and
// confirm with each transcript's real cwd (the encoding is lossy — "a/b" and
// "a-b" collide — so the cwd check removes false positives). Returns nil when
// none exist or root is blank.
func ClaudeSessions(root string) []ClaudeSession {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	projects := filepath.Join(home, ".claude", "projects")
	enc := encodeProjectDir(root)
	entries, _ := os.ReadDir(projects)

	out := make([]ClaudeSession, 0)
	for _, de := range entries {
		if !de.IsDir() || !strings.HasPrefix(de.Name(), enc) {
			continue
		}
		exactRoot := de.Name() == enc
		matches, _ := filepath.Glob(filepath.Join(projects, de.Name(), "*.jsonl"))
		for _, p := range matches {
			st, err := os.Stat(p)
			if err != nil || st.Size() == 0 {
				continue
			}
			cwd := cwdFromFile(p)
			// Keep sessions whose real cwd is root or a descendant. For the exact
			// root folder, keep even when cwd can't be read (legacy/empty).
			under := cwd == root || strings.HasPrefix(cwd, root+string(filepath.Separator))
			if !under && !(exactRoot && cwd == "") {
				continue
			}
			if cwd == "" {
				cwd = root
			}
			out = append(out, ClaudeSession{
				ID:      strings.TrimSuffix(filepath.Base(p), ".jsonl"),
				Title:   titleFromFile(p),
				Updated: st.ModTime().UTC(),
				Cwd:     cwd,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Updated.After(out[j].Updated) })
	return out
}
