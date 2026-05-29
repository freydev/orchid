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
