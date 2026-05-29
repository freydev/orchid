// Package project stores orchid projects (git repositories) as JSON.
package project

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Project is a git repository orchid manages.
type Project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// Store is the persisted set of projects.
type Store struct {
	path     string
	mu       sync.Mutex
	projects []Project
}

func New(path string) *Store { return &Store{path: path} }

func newID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return "p-" + hex.EncodeToString(b)
}

// Load reads projects.json (missing file -> empty, no error).
func (s *Store) Load() error {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(data) == 0 {
		s.projects = nil
		return nil
	}
	return json.Unmarshal(data, &s.projects)
}

func (s *Store) List() []Project {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Project, len(s.projects))
	copy(out, s.projects)
	return out
}

func (s *Store) Get(id string) (Project, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.projects {
		if p.ID == id {
			return p, true
		}
	}
	return Project{}, false
}

// Add registers a git repository. The path must contain a .git entry. A path
// already registered returns the existing project (idempotent).
func (s *Store) Add(name, path string) (Project, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Project{}, err
	}
	if _, err := os.Stat(filepath.Join(abs, ".git")); err != nil {
		return Project{}, fmt.Errorf("%s is not a git repository (no .git)", abs)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.projects {
		if p.Path == abs {
			return p, nil
		}
	}
	if name == "" {
		name = filepath.Base(abs)
	}
	p := Project{ID: newID(), Name: name, Path: abs}
	s.projects = append(s.projects, p)
	return p, s.saveLocked()
}

func (s *Store) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fresh := make([]Project, 0, len(s.projects))
	found := false
	for _, p := range s.projects {
		if p.ID == id {
			found = true
			continue
		}
		fresh = append(fresh, p)
	}
	if !found {
		return fmt.Errorf("project %q not found", id)
	}
	s.projects = fresh
	return s.saveLocked()
}

// saveLocked persists atomically (temp + rename). Caller holds s.mu.
func (s *Store) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.projects, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".projects-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	return os.Rename(name, s.path)
}
