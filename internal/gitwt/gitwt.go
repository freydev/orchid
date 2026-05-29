// Package gitwt is a thin wrapper over the git worktree CLI.
package gitwt

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Worktree is one entry from `git worktree list`.
type Worktree struct {
	Path   string `json:"path"`
	Branch string `json:"branch"`
	IsMain bool   `json:"is_main"`
}

// Available reports whether git is usable.
func Available() error {
	if err := exec.Command("git", "--version").Run(); err != nil {
		return fmt.Errorf("git not found: %w", err)
	}
	return nil
}

// List returns the worktrees of repo (first entry = main).
func List(repo string) ([]Worktree, error) {
	out, err := exec.Command("git", "-C", repo, "worktree", "list", "--porcelain").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w: %s", err, out)
	}
	return parseList(out), nil
}

// parseList parses `git worktree list --porcelain` output. Blocks are separated
// by a blank line; the first block is the repo root (main).
func parseList(out []byte) []Worktree {
	var wts []Worktree
	var cur *Worktree
	flush := func() {
		if cur != nil && cur.Path != "" {
			cur.IsMain = len(wts) == 0
			wts = append(wts, *cur)
		}
		cur = nil
	}
	for _, line := range strings.Split(string(bytes.TrimSpace(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			flush()
			continue
		}
		if cur == nil {
			cur = &Worktree{}
		}
		switch {
		case strings.HasPrefix(line, "worktree "):
			cur.Path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "branch "):
			cur.Branch = strings.TrimPrefix(strings.TrimPrefix(line, "branch "), "refs/heads/")
		}
	}
	flush()
	return wts
}

func sani(branch string) string { return strings.ReplaceAll(branch, "/", "-") }

// Add creates a new worktree on a NEW branch (off current HEAD) under
// <repo>/.orchid/worktrees/<sani(branch)>. Returns the worktree's absolute path.
func Add(repo, branch string) (string, error) {
	if strings.TrimSpace(branch) == "" {
		return "", fmt.Errorf("branch is required")
	}
	path := filepath.Join(repo, ".orchid", "worktrees", sani(branch))
	out, err := exec.Command("git", "-C", repo, "worktree", "add", path, "-b", branch).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git worktree add: %w: %s", err, out)
	}
	return path, nil
}

// Remove force-removes a worktree (the directory is deleted).
func Remove(repo, path string) error {
	out, err := exec.Command("git", "-C", repo, "worktree", "remove", "--force", path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree remove: %w: %s", err, out)
	}
	return nil
}

// DeleteBranch force-deletes a branch.
func DeleteBranch(repo, branch string) error {
	out, err := exec.Command("git", "-C", repo, "branch", "-D", branch).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git branch -D: %w: %s", err, out)
	}
	return nil
}
