package gitwt

import (
	"os"
	"os/exec"
	"path/filepath"
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

func TestAddListRemove(t *testing.T) {
	repo := gitRepo(t)
	wts, err := List(repo)
	if err != nil || len(wts) != 1 || !wts[0].IsMain {
		t.Fatalf("initial List = %v, %v", wts, err)
	}
	path, err := Add(repo, "feat/demo")
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
