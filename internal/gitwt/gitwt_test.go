package gitwt

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func parseFixture() string {
	return "worktree /repo\nHEAD abc\nbranch refs/heads/master\n\n" +
		"worktree /repo/.orchid/worktrees/feat-x\nHEAD def\nbranch refs/heads/feat/x\n\n"
}

func TestParseList(t *testing.T) {
	wts := parseList([]byte(parseFixture()))
	if len(wts) != 2 {
		t.Fatalf("got %d worktrees", len(wts))
	}
	if !wts[0].IsMain || wts[0].Path != "/repo" || wts[0].Branch != "master" {
		t.Fatalf("main = %+v", wts[0])
	}
	if wts[1].IsMain || wts[1].Branch != "feat/x" {
		t.Fatalf("wt2 = %+v", wts[1])
	}
}

func TestSani(t *testing.T) {
	if sani("feat/new-thing") != "feat-new-thing" {
		t.Fatalf("sani = %q", sani("feat/new-thing"))
	}
}

func gitRepo(t *testing.T) string {
	t.Helper()
	if err := Available(); err != nil {
		t.Skipf("git not available: %v", err)
	}
	d := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "--allow-empty", "-q", "-m", "init"},
	} {
		c := exec.Command("git", append([]string{"-C", d}, args...)...)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	return d
}

func git(t *testing.T, repo string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}

// New worktrees must branch off the default branch, not whatever HEAD happens
// to be checked out.
func TestAddOffDefaultBranch(t *testing.T) {
	repo := gitRepo(t)
	def := DefaultBranch(repo)
	defHead := strings.TrimSpace(git(t, repo, "rev-parse", def))

	// Move HEAD onto a side branch with an extra commit ahead of default.
	git(t, repo, "checkout", "-q", "-b", "side")
	git(t, repo, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "--allow-empty", "-q", "-m", "side")
	sideHead := strings.TrimSpace(git(t, repo, "rev-parse", "HEAD"))
	if sideHead == defHead {
		t.Fatal("side commit did not advance HEAD")
	}

	path, err := Add(repo, "feat/demo", "", true)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	got := strings.TrimSpace(git(t, repo, "-C", path, "rev-parse", "HEAD"))
	if got != defHead {
		t.Fatalf("worktree based on %s, want default %s (side was %s)", got, defHead, sideHead)
	}
}

func TestGitInfoNoOrigin(t *testing.T) {
	repo := gitRepo(t)
	if !IsRepo(repo) {
		t.Fatal("IsRepo = false for a git repo")
	}
	if HasOrigin(repo) {
		t.Fatal("HasOrigin = true with no remote configured")
	}
	if DefaultBranch(repo) == "" || DefaultBranch(repo) == "HEAD" {
		t.Fatalf("DefaultBranch = %q, want a real branch", DefaultBranch(repo))
	}
}

func TestIsRepoNonRepo(t *testing.T) {
	if Available() != nil {
		t.Skip("git not available")
	}
	if IsRepo(t.TempDir()) {
		t.Fatal("IsRepo = true for a non-git directory")
	}
}

func TestAddListRemove(t *testing.T) {
	repo := gitRepo(t)
	wts, err := List(repo)
	if err != nil || len(wts) != 1 || !wts[0].IsMain {
		t.Fatalf("initial List = %v, %v", wts, err)
	}
	path, err := Add(repo, "feat/demo", "", true)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if filepath.Base(path) != "feat-demo" {
		t.Fatalf("path = %q", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("worktree dir missing: %v", err)
	}
	wts, _ = List(repo)
	if len(wts) != 2 {
		t.Fatalf("after add List = %v", wts)
	}
	if err := Remove(repo, path); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if err := DeleteBranch(repo, "feat/demo"); err != nil {
		t.Fatalf("DeleteBranch: %v", err)
	}
	wts, _ = List(repo)
	if len(wts) != 1 {
		t.Fatalf("after remove List = %v", wts)
	}
}

func TestMarkRootLinkedWorktree(t *testing.T) {
	wts := []Worktree{
		{Path: "/repo", Branch: "master", IsMain: true, Primary: true},
		{Path: "/repo-svelte", Branch: "svelte"},
		{Path: "/repo/.orchid/worktrees/feat-x", Branch: "feat/x"},
	}
	got := markRoot(wts, "/repo-svelte")
	if got[0].Path != "/repo-svelte" || !got[0].IsMain || got[0].Primary {
		t.Fatalf("root = %+v", got[0])
	}
	if got[1].Path != "/repo" || got[1].IsMain || !got[1].Primary {
		t.Fatalf("primary = %+v", got[1])
	}
	if len(got) != 3 || got[2].IsMain {
		t.Fatalf("got %+v", got)
	}
	// project at the primary: unchanged
	same := markRoot([]Worktree{{Path: "/repo", IsMain: true, Primary: true}, {Path: "/b"}}, "/repo")
	if !same[0].IsMain || same[1].IsMain {
		t.Fatalf("unchanged case = %+v", same)
	}
}

func TestHasWork(t *testing.T) {
	repo := gitRepo(t)
	base := strings.TrimSpace(git(t, repo, "rev-parse", "--abbrev-ref", "HEAD"))
	wt, err := Add(repo, "feat/empty", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if HasWork(repo, wt, "feat/empty", base) {
		t.Fatal("fresh worktree reported as having work")
	}
	// untracked file counts as work
	if err := os.WriteFile(filepath.Join(wt, "new.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !HasWork(repo, wt, "feat/empty", base) {
		t.Fatal("untracked file not detected")
	}
	// committed but not on base counts as work
	git(t, wt, "add", ".")
	git(t, wt, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "-m", "c")
	if !HasWork(repo, wt, "feat/empty", base) {
		t.Fatal("commit ahead of base not detected")
	}
	// once base contains the commit, nothing is lost
	git(t, repo, "merge", "-q", "--ff-only", "feat/empty")
	if HasWork(repo, wt, "feat/empty", base) {
		t.Fatal("merged branch still reported as having work")
	}
	// detached HEAD: be safe
	if !HasWork(repo, wt, "", base) {
		t.Fatal("empty branch must count as work")
	}
	// a pushed commit lives on a remote ref -> not work, even if base lacks it
	git(t, wt, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "--allow-empty", "-q", "-m", "c2")
	if !HasWork(repo, wt, "feat/empty", "no-such-branch") {
		t.Fatal("unpushed commit with unresolvable base not detected")
	}
	remote := t.TempDir()
	git(t, remote, "init", "-q", "--bare")
	git(t, repo, "remote", "add", "origin", remote)
	git(t, repo, "push", "-q", "origin", "feat/empty")
	if HasWork(repo, wt, "feat/empty", base) {
		t.Fatal("pushed branch still reported as having work")
	}
}

func TestHasWorkMissingDirAndRemove(t *testing.T) {
	repo := gitRepo(t)
	base := strings.TrimSpace(git(t, repo, "rev-parse", "--abbrev-ref", "HEAD"))
	wt, err := Add(repo, "feat/gone", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(wt); err != nil {
		t.Fatal(err)
	}
	if HasWork(repo, wt, "feat/gone", base) {
		t.Fatal("worktree with deleted dir and no unique commits reported as work")
	}
	if err := Remove(repo, wt); err != nil {
		t.Fatalf("Remove missing: %v", err)
	}
	if strings.Contains(git(t, repo, "worktree", "list"), "feat/gone") {
		t.Fatal("stale worktree entry not pruned")
	}
}
