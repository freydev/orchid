// Package gitwt is a thin wrapper over the git worktree CLI.
package gitwt

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Worktree is one entry from `git worktree list`.
type Worktree struct {
	Path   string `json:"path"`
	Branch string `json:"branch"`
	// IsMain marks the project's own folder (shown as "root"). When the project
	// path is itself a linked worktree, that is not git's primary worktree.
	IsMain bool `json:"is_main"`
	// Primary marks git's primary worktree (first listed). git refuses to
	// remove it, so it is never removable even when it isn't the project root.
	Primary bool `json:"-"`
}

// Available reports whether git is usable.
func Available() error {
	if err := exec.Command("git", "--version").Run(); err != nil {
		return fmt.Errorf("git not found: %w", err)
	}
	return nil
}

// List returns the worktrees of repo, the repo folder itself first (IsMain).
// repo may be a linked worktree: then git's primary worktree is listed too,
// but the root is repo, so sessions created in "root" land in repo.
func List(repo string) ([]Worktree, error) {
	out, err := exec.Command("git", "-C", repo, "worktree", "list", "--porcelain").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w: %s", err, out)
	}
	return markRoot(parseList(out), repo), nil
}

// markRoot moves IsMain to the entry whose path is repo (comparing resolved
// paths: git prints symlink-free paths, e.g. /private/var vs /var) and puts it
// first. With no match the git order and primary-as-main are kept.
func markRoot(wts []Worktree, repo string) []Worktree {
	want := canon(repo)
	for i := range wts {
		if canon(wts[i].Path) != want {
			continue
		}
		if i == 0 {
			return wts
		}
		for j := range wts {
			wts[j].IsMain = j == i
		}
		root := wts[i]
		out := append([]Worktree{root}, wts[:i]...)
		return append(out, wts[i+1:]...)
	}
	return wts
}

func canon(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return filepath.Clean(r)
	}
	return filepath.Clean(p)
}

// parseList parses `git worktree list --porcelain` output. Blocks are separated
// by a blank line; the first block is the repo root (main).
func parseList(out []byte) []Worktree {
	var wts []Worktree
	var cur *Worktree
	flush := func() {
		if cur != nil && cur.Path != "" {
			cur.IsMain = len(wts) == 0
			cur.Primary = cur.IsMain
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

// DefaultBranch returns the repo's default branch name (e.g. "main" or
// "master"). It prefers the remote's HEAD, then a local main/master, and
// finally the currently checked-out branch.
func DefaultBranch(repo string) string {
	if out, err := exec.Command("git", "-C", repo, "symbolic-ref", "--short", "refs/remotes/origin/HEAD").Output(); err == nil {
		ref := strings.TrimSpace(string(out)) // e.g. "origin/main"
		if i := strings.IndexByte(ref, '/'); i >= 0 {
			return ref[i+1:]
		}
	}
	for _, b := range []string{"main", "master"} {
		if exec.Command("git", "-C", repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+b).Run() == nil {
			return b
		}
	}
	if out, err := exec.Command("git", "-C", repo, "symbolic-ref", "--short", "HEAD").Output(); err == nil {
		if s := strings.TrimSpace(string(out)); s != "" {
			return s
		}
	}
	return "HEAD"
}

// CurrentBranch returns the branch checked out in repo's main working tree, or
// "" when detached HEAD or not resolvable.
func CurrentBranch(repo string) string {
	out, err := exec.Command("git", "-C", repo, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}
	b := strings.TrimSpace(string(out))
	if b == "HEAD" { // detached
		return ""
	}
	return b
}

// IsRepo reports whether path is usable as a git repository.
func IsRepo(repo string) bool {
	return exec.Command("git", "-C", repo, "rev-parse", "--git-dir").Run() == nil
}

// HasOrigin reports whether the repo has a remote named "origin".
func HasOrigin(repo string) bool {
	out, err := exec.Command("git", "-C", repo, "remote").Output()
	if err != nil {
		return false
	}
	for _, r := range strings.Fields(string(out)) {
		if r == "origin" {
			return true
		}
	}
	return false
}

// resolveBase resolves the start-point for new worktrees: the requested branch
// (or the repo default when empty). When fetch is true and a remote is
// configured, it fetches origin/<base> fresh and branches off origin/<base>;
// otherwise (or if the fetch fails) it branches off the local branch.
func resolveBase(repo, base string, fetch bool) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = DefaultBranch(repo)
	}
	if fetch && HasOrigin(repo) {
		if err := exec.Command("git", "-C", repo, "fetch", "origin", base).Run(); err == nil {
			return "origin/" + base
		}
	}
	return base
}

// BaseRef reports the start-point label a new worktree would branch off, given a
// project's configured base branch ("" -> repo default): "origin/<b>" when a
// remote exists, else "<b>".
func BaseRef(repo, base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = DefaultBranch(repo)
	}
	if HasOrigin(repo) {
		return "origin/" + base
	}
	return base
}

// BranchExists reports whether a local branch with this name exists.
func BranchExists(repo, branch string) bool {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return false
	}
	return exec.Command("git", "-C", repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch).Run() == nil
}

// Branches lists local branch names (sorted by git's default for-each-ref order).
func Branches(repo string) []string {
	out, err := exec.Command("git", "-C", repo, "for-each-ref", "--format=%(refname:short)", "refs/heads").Output()
	if err != nil {
		return nil
	}
	var names []string
	for _, line := range strings.Split(string(out), "\n") {
		if s := strings.TrimSpace(line); s != "" {
			names = append(names, s)
		}
	}
	return names
}

// Add creates a worktree under <repo>/.orchid/worktrees/<sani(branch)>. If the
// branch already exists it is checked out (base/fetch ignored); otherwise a NEW
// branch is created off base — origin/<base> (fetched fresh) when fetch is true
// and a remote exists, else the local base. base is the chosen start point; ""
// means the repo default branch. Returns the worktree's absolute path with
// symlinks resolved.
func Add(repo, branch, base string, fetch bool) (string, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return "", fmt.Errorf("branch is required")
	}
	path := filepath.Join(repo, ".orchid", "worktrees", sani(branch))
	var args []string
	if BranchExists(repo, branch) {
		// attach the existing branch (no new branch, base/fetch ignored)
		args = []string{"-C", repo, "worktree", "add", path, branch}
	} else {
		args = []string{"-C", repo, "worktree", "add", path, "-b", branch, resolveBase(repo, base, fetch)}
	}
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git worktree add: %w: %s", err, out)
	}
	// Resolve symlinks so the returned path matches git worktree list output.
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	return path, nil
}

// Remove force-removes a worktree (the directory is deleted). A worktree whose
// directory is already gone is unregistered via `git worktree prune`.
func Remove(repo, path string) error {
	if missing(path) {
		out, err := exec.Command("git", "-C", repo, "worktree", "prune").CombinedOutput()
		if err != nil {
			return fmt.Errorf("git worktree prune: %w: %s", err, out)
		}
		return nil
	}
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

// HasWork reports whether removing worktree path (on branch) could lose
// anything: uncommitted or untracked changes, or commits on branch that exist
// nowhere else — not on any remote-tracking ref and not in base (when base
// resolves; "" skips it). Pushed or merged work is therefore not "work".
// Anything it cannot verify (detached HEAD, git errors) counts as work, so
// callers err on the side of keeping.
func HasWork(repo, path, branch, base string) bool {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return true
	}
	// a worktree whose directory is gone has no local changes left to lose
	if !missing(path) {
		out, err := exec.Command("git", "-C", path, "status", "--porcelain").Output()
		if err != nil || len(bytes.TrimSpace(out)) > 0 {
			return true
		}
	}
	args := []string{"-C", repo, "rev-list", "--count", "refs/heads/" + branch, "--not", "--remotes"}
	base = strings.TrimSpace(base)
	if base != "" && exec.Command("git", "-C", repo, "rev-parse", "--verify", "--quiet", base+"^{commit}").Run() == nil {
		args = append(args, base)
	}
	out, err := exec.Command("git", args...).Output()
	return err != nil || strings.TrimSpace(string(out)) != "0"
}

func missing(path string) bool {
	_, err := os.Stat(path)
	return errors.Is(err, fs.ErrNotExist)
}
