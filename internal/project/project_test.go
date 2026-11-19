package project

import (
	"os"
	"path/filepath"
	"testing"
)

func gitRepoDir(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	if err := os.MkdirAll(filepath.Join(d, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestAddRequiresGit(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "projects.json"))
	if _, err := s.Add("x", t.TempDir()); err == nil {
		t.Fatal("expected error for non-git dir")
	}
}

func TestAddListGetRemove(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "projects.json"))
	repo := gitRepoDir(t)
	p, err := s.Add("proj", repo)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if p.ID == "" || p.Name != "proj" || p.Path != repo {
		t.Fatalf("project = %+v", p)
	}
	if len(s.List()) != 1 {
		t.Fatalf("List = %v", s.List())
	}
	got, ok := s.Get(p.ID)
	if !ok || got.Path != repo {
		t.Fatalf("Get = %+v %v", got, ok)
	}
	if err := s.Remove(p.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(s.List()) != 0 {
		t.Fatal("expected empty after remove")
	}
}

func TestAddDuplicatePathIdempotent(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "projects.json"))
	repo := gitRepoDir(t)
	p1, _ := s.Add("a", repo)
	p2, err := s.Add("b", repo)
	if err != nil {
		t.Fatalf("Add dup: %v", err)
	}
	if p1.ID != p2.ID || len(s.List()) != 1 {
		t.Fatalf("duplicate path should return existing: %v %v", p1, p2)
	}
}

func TestLoadPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "projects.json")
	repo := gitRepoDir(t)
	s1 := New(path)
	s1.Add("proj", repo)
	s2 := New(path)
	if err := s2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(s2.List()) != 1 || s2.List()[0].Path != repo {
		t.Fatalf("reloaded = %v", s2.List())
	}
}

func TestReorder(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "projects.json"))
	s.projects = []Project{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	if err := s.Reorder([]string{"c", "a"}); err != nil {
		t.Fatal(err)
	}
	got := []string{s.projects[0].ID, s.projects[1].ID, s.projects[2].ID}
	want := []string{"c", "a", "b"} // listed first in given order, unknown ("b") after
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestSetWorktreeOrder(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "projects.json"))
	s.projects = []Project{{ID: "a"}}
	if err := s.SetWorktreeOrder("a", []string{"/x", "/y"}); err != nil {
		t.Fatal(err)
	}
	wt := s.projects[0].WorktreeOrder
	if len(wt) != 2 || wt[0] != "/x" || wt[1] != "/y" {
		t.Fatalf("worktree order = %v", wt)
	}
	if err := s.SetWorktreeOrder("nope", []string{"/z"}); err == nil {
		t.Fatal("expected error for unknown project")
	}
}

func TestResolvePathGlobal(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	cases := map[string]string{
		"~":             home,
		"~/code/x":      filepath.Join(home, "code", "x"),
		"code/x":        filepath.Join(home, "code", "x"), // relative -> home, not CWD
		"/abs/path":     "/abs/path",
		"  ~/y  ":       filepath.Join(home, "y"), // trimmed
		"/abs/../path2": "/path2",                 // cleaned
	}
	for in, want := range cases {
		got, err := resolvePath(in)
		if err != nil {
			t.Errorf("resolvePath(%q) error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("resolvePath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGroupsLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "projects.json")
	s := New(path)
	p, _ := s.Add("proj", gitRepoDir(t))
	if _, err := s.AddGroup(p.ID, "/wt/a", "  "); err == nil {
		t.Fatal("empty name accepted")
	}
	if _, err := s.AddGroup("nope", "/wt/a", "x"); err == nil {
		t.Fatal("unknown project accepted")
	}
	ga, _ := s.AddGroup(p.ID, "/wt/a", "Review")
	gb, _ := s.AddGroup(p.ID, "/wt/b", "Research")
	if err := s.RenameGroup(p.ID, ga.ID, "Reviews"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if err := s.RenameGroup(p.ID, "g-nope", "x"); err == nil {
		t.Fatal("rename unknown group accepted")
	}
	// persisted across reload
	s2 := New(path)
	if err := s2.Load(); err != nil {
		t.Fatal(err)
	}
	got, _ := s2.Get(p.ID)
	if len(got.Groups) != 2 || got.Groups[0].Name != "Reviews" || got.Groups[1].Worktree != "/wt/b" {
		t.Fatalf("groups after reload = %+v", got.Groups)
	}
	// removing a worktree's groups leaves the others
	if err := s.RemoveWorktreeGroups(p.ID, "/wt/a"); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveGroup(p.ID, ga.ID); err == nil {
		t.Fatal("removing already-dropped group succeeded")
	}
	if err := s.RemoveGroup(p.ID, gb.ID); err != nil {
		t.Fatalf("remove: %v", err)
	}
	got, _ = s.Get(p.ID)
	if len(got.Groups) != 0 {
		t.Fatalf("groups remain: %+v", got.Groups)
	}
}
