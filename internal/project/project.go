// Package project stores orchid projects (git repositories) as JSON.
package project

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Preset is a named shell command a project can launch as a session.
type Preset struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Cmd   string `json:"cmd"`
}

// Project is a git repository orchid manages.
type Project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
	// Color is the project accent color id ("blue", "green"...), or "".
	Color string `json:"color,omitempty"`
	// Description is a free-form note shown on the project card.
	Description string `json:"description,omitempty"`
	// Shell overrides the default shell for this project's "shell" sessions
	// (bare name or path). Empty inherits the global default.
	Shell string `json:"shell,omitempty"`
	// BaseBranch is the start point for new worktrees. Empty -> repo default.
	BaseBranch string `json:"base_branch,omitempty"`
	// Commands are named command presets selectable when starting a shell session.
	Commands []Preset `json:"commands,omitempty"`
	// WorktreeOrder is the user's preferred display order of worktree paths.
	// Worktrees absent from this list sort after, in git order.
	WorktreeOrder []string `json:"worktree_order,omitempty"`
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

// Update sets a project's editable settings. Name is applied only when
// non-empty (a project must keep a name); the other fields are set verbatim,
// so passing "" clears them.
func (s *Store) Update(id, name, color, description, shell, baseBranch string, commands []Preset) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.projects {
		if s.projects[i].ID == id {
			if n := strings.TrimSpace(name); n != "" {
				s.projects[i].Name = n
			}
			s.projects[i].Color = strings.TrimSpace(color)
			s.projects[i].Description = strings.TrimSpace(description)
			s.projects[i].Shell = strings.TrimSpace(shell)
			s.projects[i].BaseBranch = strings.TrimSpace(baseBranch)
			s.projects[i].Commands = sanitizePresets(commands)
			return s.saveLocked()
		}
	}
	return fmt.Errorf("project %q not found", id)
}

// sanitizePresets drops presets with an empty label or cmd and trims fields.
func sanitizePresets(in []Preset) []Preset {
	var out []Preset
	for _, p := range in {
		label, cmd := strings.TrimSpace(p.Label), strings.TrimSpace(p.Cmd)
		if label == "" || cmd == "" {
			continue
		}
		id := strings.TrimSpace(p.ID)
		if id == "" {
			id = newID()
		}
		out = append(out, Preset{ID: id, Label: label, Cmd: cmd})
	}
	return out
}

// Reorder sets the project display order to match ids. Projects whose id is not
// in ids keep their relative order and sort after the listed ones.
func (s *Store) Reorder(ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	pos := make(map[string]int, len(ids))
	for i, id := range ids {
		pos[id] = i
	}
	sort.SliceStable(s.projects, func(i, j int) bool {
		pi, oki := pos[s.projects[i].ID]
		pj, okj := pos[s.projects[j].ID]
		if oki && okj {
			return pi < pj
		}
		return oki && !okj // known (listed) before unknown
	})
	return s.saveLocked()
}

// SetWorktreeOrder records the user's worktree display order (by path) for a project.
func (s *Store) SetWorktreeOrder(id string, paths []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.projects {
		if s.projects[i].ID == id {
			s.projects[i].WorktreeOrder = paths
			return s.saveLocked()
		}
	}
	return fmt.Errorf("project %q not found", id)
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
