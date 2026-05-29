# Orchid Projects & Git Worktrees Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a project → git-worktree → session hierarchy: projects are git repos (stored in `~/.config/orchid/projects.json`), worktrees come from `git worktree list`, and every session is created in and hard-bound to a worktree's folder; removing a worktree cascades (kill its sessions → `git worktree remove` → `git branch -D`).

**Architecture:** New `internal/project` (projects.json CRUD + `.git` validation) and `internal/gitwt` (git-worktree CLI wrapper, git as source of truth). `session.Manager` becomes worktree-aware: `Create(project, worktree, agent)`, `KillByWorktree`, records carry `Project`+`Worktree`, `Restore` drops legacy records. The server gains project/worktree routes and a remove-worktree cascade. The frontend replaces the flat dashboard with a projects list → project detail (worktree groups) using the already-vendored design CSS.

**Tech Stack:** Go 1.26 stdlib + `os/exec` (git/tmux CLIs), existing `tmux`/`store`/`agent` packages, vanilla JS + underscore + xterm (existing). git is required at runtime.

**Verified git facts:** `git worktree list --porcelain` emits blocks separated by a blank line: `worktree <path>` / `HEAD <sha>` / `branch refs/heads/<b>` (or `detached`/`bare`). The first block is the repo root (main).

---

## File Structure

```
internal/project/project.go + project_test.go   # NEW: projects.json store
internal/gitwt/gitwt.go + gitwt_test.go          # NEW: git worktree CLI wrapper
internal/session/session.go                      # MODIFIED: Record fields used; createSession unchanged (cwd already a param via rec.Cwd)
internal/session/manager.go                      # MODIFIED: Project/Worktree, Create sig, KillByWorktree, Restore drop, List/Info
internal/session/manager_test.go                 # MODIFIED: new Create sig + KillByWorktree test
internal/server/server.go + api.go               # MODIFIED: project store, routes, cascade, create-session change
internal/server/*_test.go                        # MODIFIED: New(mgr,projects) sig, create body
internal/client/client.go + client_test.go       # MODIFIED: Create(project,worktree,agent)
cmd/orchid/cli.go                                # MODIFIED: gitwt preflight, project store wiring, create cmd
web/app.js                                       # MODIFIED: projects list + detail + modals
```

`store.Record` lives in `internal/store/store.go` — Task 3 adds two fields there.

---

## Task 1: internal/project (projects.json store)

**Files:** Create `internal/project/project.go`, `internal/project/project_test.go`.

- [ ] **Step 1: Write the failing test** — `internal/project/project_test.go`:
```go
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
```

- [ ] **Step 2: Run** `go test ./internal/project/ -v` → FAIL (`undefined: New`).

- [ ] **Step 3: Implement** — `internal/project/project.go`:
```go
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
	out := s.projects[:0]
	found := false
	for _, p := range s.projects {
		if p.ID == id {
			found = true
			continue
		}
		out = append(out, p)
	}
	if !found {
		return fmt.Errorf("project %q not found", id)
	}
	s.projects = out
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
```

- [ ] **Step 4: Run** `go test ./internal/project/ -v` → PASS.

- [ ] **Step 5: Commit**
```bash
git add internal/project/
git commit -m "feat: add project store (projects.json, .git validation)"
```

---

## Task 2: internal/gitwt (git worktree wrapper)

**Files:** Create `internal/gitwt/gitwt.go`, `internal/gitwt/gitwt_test.go`.

- [ ] **Step 1: Write the failing test** — `internal/gitwt/gitwt_test.go`:
```go
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
```

- [ ] **Step 2: Run** `go test ./internal/gitwt/ -v` → FAIL (`undefined: parseList`).

- [ ] **Step 3: Implement** — `internal/gitwt/gitwt.go`:
```go
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
```

- [ ] **Step 4: Run** `go test ./internal/gitwt/ -v` → PASS (parse + sani always; Add/List/Remove with real git, else skip).

- [ ] **Step 5: Commit**
```bash
git add internal/gitwt/
git commit -m "feat: add git worktree CLI wrapper"
```

---

## Task 3: Session model — Project/Worktree binding, Create sig, KillByWorktree

**Files:** Modify `internal/store/store.go`, `internal/session/manager.go`, `internal/session/manager_test.go`.

- [ ] **Step 1: Add fields to `store.Record`.** In `internal/store/store.go`, add to the `Record` struct (after `Cwd`):
```go
	Project        string    `json:"project"`
	Worktree       string    `json:"worktree"`
```

- [ ] **Step 2: Rewrite manager_test.go for the new Create signature + KillByWorktree.** Replace `internal/session/manager_test.go` with (the `newTestManager`/`skipNoTmux` helpers stay; Create now takes project+worktree+agent — use a tmp dir as the "worktree"):
```go
package session

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"orchid/internal/tmux"
)

func skipNoTmux(t *testing.T) {
	t.Helper()
	if err := tmux.Available(); err != nil {
		t.Skipf("tmux not available: %v", err)
	}
}

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	m := NewManager(filepath.Join(t.TempDir(), "sessions.json"))
	t.Cleanup(m.KillAll)
	return m
}

func TestManagerCreateGetKill(t *testing.T) {
	skipNoTmux(t)
	m := newTestManager(t)
	s, err := m.Create("p-1", "/tmp", "shell")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer tmux.KillSession(s.Tmux)
	if s.Project != "p-1" || s.Worktree != "/tmp" || s.Cwd != "/tmp" {
		t.Fatalf("binding = %+v", s)
	}
	if _, ok := m.Get(s.ID); !ok {
		t.Fatal("Get failed")
	}
	info := m.List()[0]
	if info.Project != "p-1" || info.Worktree != "/tmp" {
		t.Fatalf("info = %+v", info)
	}
	if err := m.Kill(s.ID); err != nil {
		t.Fatalf("kill: %v", err)
	}
	if len(m.List()) != 0 {
		t.Fatal("expected empty after kill")
	}
}

func TestManagerCreateUnknownAgent(t *testing.T) {
	m := newTestManager(t)
	if _, err := m.Create("p-1", "/tmp", "nope"); err == nil {
		t.Fatal("expected error for unknown agent")
	}
}

func TestKillByWorktree(t *testing.T) {
	skipNoTmux(t)
	m := newTestManager(t)
	a, _ := m.Create("p-1", "/tmp/wtA", "shell")
	b, _ := m.Create("p-1", "/tmp/wtA", "shell")
	c, _ := m.Create("p-1", "/tmp/wtB", "shell")
	defer tmux.KillSession(c.Tmux)
	n := m.KillByWorktree("/tmp/wtA")
	if n != 2 {
		t.Fatalf("killed %d, want 2", n)
	}
	if _, ok := m.Get(a.ID); ok { t.Fatal("a should be gone") }
	if _, ok := m.Get(b.ID); ok { t.Fatal("b should be gone") }
	if _, ok := m.Get(c.ID); !ok { t.Fatal("c should remain") }
}

func TestManagerConcurrentCreate(t *testing.T) {
	skipNoTmux(t)
	m := newTestManager(t)
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := m.Create("p-1", "/tmp", "shell")
			if err != nil { t.Errorf("create: %v", err); return }
			tmux.KillSession(s.Tmux)
		}()
	}
	wg.Wait()
	if len(m.List()) != 6 {
		t.Fatalf("List len = %d, want 6", len(m.List()))
	}
}

var _ = time.Second // keep import if unused
```
(Delete the now-stale `TestSelfExitMarksStopped`/`TestManagerRestoreReconciles`/`TestManagerRestart`/`TestManagerKillUnknown` only if they reference the old `Create(agent,cwd)` signature — actually KEEP them but update each `m.Create("shell","/tmp")` call to `m.Create("p-1","/tmp","shell")`. Do not delete coverage; just fix the signature in every call. Remove the `var _ = time.Second` line if `time` is used by a kept test.)

- [ ] **Step 3: Run** `go test ./internal/session/ -run 'TestManager|TestKillByWorktree' -v` → FAIL (Create arity, `s.Project` undefined).

- [ ] **Step 4: Modify `internal/session/manager.go`.**

(a) `Info` struct — add fields:
```go
type Info struct {
	ID       string `json:"id"`
	Agent    string `json:"agent"`
	Cwd      string `json:"cwd"`
	Project  string `json:"project"`
	Worktree string `json:"worktree"`
	Status   string `json:"status"`
	Alive    bool   `json:"alive"`
}
```

(b) `Create` — new signature, sets project/worktree, cwd = worktree:
```go
func (m *Manager) Create(projectID, worktree, agentName string) (*Session, error) {
	ag, ok := agent.Get(agentName)
	if !ok {
		return nil, fmt.Errorf("unknown agent %q", agentName)
	}
	id := newID()
	rec := store.Record{
		ID:             id,
		Agent:          agentName,
		Cwd:            worktree,
		Project:        projectID,
		Worktree:       worktree,
		Tmux:           "orchid-" + id,
		AgentSessionID: uuid.NewString(),
		Created:        time.Now().UTC(),
		Status:         "running",
	}
	s, err := createSession(rec, ag, func() { m.onExit(id) })
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.sessions[id] = s
	m.records[id] = rec
	m.saveLocked()
	m.mu.Unlock()
	s.Start()
	return s, nil
}
```

(c) `List` — include project/worktree:
```go
		out = append(out, Info{
			ID:       r.ID,
			Agent:    r.Agent,
			Cwd:      r.Cwd,
			Project:  r.Project,
			Worktree: r.Worktree,
			Status:   r.Status,
			Alive:    running,
		})
```

(d) `Restore` — drop legacy records (no project/worktree). At the top of the `for _, rec := range recs` loop body, add:
```go
		if rec.Project == "" || rec.Worktree == "" {
			continue // legacy/unbound record from before projects — drop it
		}
```

(e) Add `KillByWorktree` (after `Kill`):
```go
// KillByWorktree terminates every session bound to the given worktree path
// (running or stopped): kill tmux, drop session + record. Returns the count.
func (m *Manager) KillByWorktree(worktree string) int {
	m.mu.Lock()
	var victims []store.Record
	var live []*Session
	for id, r := range m.records {
		if r.Worktree == worktree {
			victims = append(victims, r)
			if s := m.sessions[id]; s != nil {
				live = append(live, s)
			}
			delete(m.records, id)
			delete(m.sessions, id)
		}
	}
	m.mu.Unlock()
	for _, s := range live {
		s.Close()
	}
	for _, r := range victims {
		_ = tmux.KillSession(r.Tmux)
	}
	m.mu.Lock()
	m.saveLocked()
	m.mu.Unlock()
	return len(victims)
}
```

(f) The session test helper in `session_test.go` (`newTestSession`) calls `createSession(rec, sh, nil)` directly with a `store.Record` — it does NOT use `Manager.Create`, so it is unaffected by the signature change. Leave `session_test.go` as-is.

(g) `Session` exposes `Project`/`Worktree`? The test reads `s.Project`/`s.Worktree`. Add exported fields to the `Session` struct (in `session.go`) and set them in `createSession`/`attachSession` from `rec`:
   - Add `Project string` and `Worktree string` to the `Session` struct (next to `Cwd`).
   - In both `createSession` and `attachSession`, set `Project: rec.Project, Worktree: rec.Worktree` in the `&Session{...}` literal.

- [ ] **Step 5: Run** `go test ./internal/session/ -race` → PASS (all session+manager tests).

- [ ] **Step 6: Commit**
```bash
git add internal/store/store.go internal/session/
git commit -m "feat: bind sessions to project+worktree; add KillByWorktree; drop legacy on restore"
```

---

## Task 4: Server — project store, project/worktree routes, cascade, create-session change

**Files:** Modify `internal/server/server.go`, `internal/server/api.go`, `internal/server/api_test.go`, `internal/server/ws_test.go`, `internal/server/static_test.go`.

- [ ] **Step 1: Update server tests for `New(mgr, projects)` + new create body.** Edit `internal/server/api_test.go` `newTestServer` to construct a project store and pass it; update create tests. Replace `internal/server/api_test.go` with:
```go
package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"orchid/internal/project"
	"orchid/internal/session"
	"orchid/internal/tmux"
)

func gitRepo(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	for _, a := range [][]string{{"init", "-q"}, {"-c", "user.email=t@t", "-c", "user.name=t", "commit", "--allow-empty", "-q", "-m", "i"}} {
		if out, err := exec.Command("git", append([]string{"-C", d}, a...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", a, err, out)
		}
	}
	return d
}

func newTestServer(t *testing.T) (*httptest.Server, *session.Manager, *project.Store) {
	t.Helper()
	if err := tmux.Available(); err != nil {
		t.Skipf("tmux not available: %v", err)
	}
	if err := exec.Command("git", "--version").Run(); err != nil {
		t.Skipf("git not available")
	}
	mgr := session.NewManager(filepath.Join(t.TempDir(), "sessions.json"))
	t.Cleanup(mgr.KillAll)
	ps := project.New(filepath.Join(t.TempDir(), "projects.json"))
	srv := httptest.NewServer(New(mgr, ps))
	t.Cleanup(srv.Close)
	return srv, mgr, ps
}

func TestAPIProjectsCRUD(t *testing.T) {
	srv, _, _ := newTestServer(t)
	repo := gitRepo(t)
	body := bytes.NewBufferString(`{"name":"p","path":"` + repo + `"}`)
	resp, _ := http.Post(srv.URL+"/api/projects", "application/json", body)
	if resp.StatusCode != 200 {
		t.Fatalf("create project = %d", resp.StatusCode)
	}
	var p project.Project
	json.NewDecoder(resp.Body).Decode(&p)
	resp.Body.Close()
	if p.ID == "" || p.Path != repo {
		t.Fatalf("project = %+v", p)
	}
	// non-git dir -> 400
	bad := bytes.NewBufferString(`{"name":"x","path":"` + t.TempDir() + `"}`)
	resp, _ = http.Post(srv.URL+"/api/projects", "application/json", bad)
	if resp.StatusCode != 400 {
		t.Fatalf("non-git = %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestAPIWorktreesAndCascade(t *testing.T) {
	srv, mgr, ps := newTestServer(t)
	repo := gitRepo(t)
	p, _ := ps.Add("p", repo)

	// list -> main only
	resp, _ := http.Get(srv.URL + "/api/projects/" + p.ID + "/worktrees")
	var wts []map[string]any
	json.NewDecoder(resp.Body).Decode(&wts)
	resp.Body.Close()
	if len(wts) != 1 {
		t.Fatalf("worktrees = %v", wts)
	}

	// add worktree
	resp, _ = http.Post(srv.URL+"/api/projects/"+p.ID+"/worktrees", "application/json", bytes.NewBufferString(`{"branch":"feat/x"}`))
	if resp.StatusCode != 200 {
		t.Fatalf("add wt = %d", resp.StatusCode)
	}
	var wt map[string]any
	json.NewDecoder(resp.Body).Decode(&wt)
	resp.Body.Close()
	wtPath := wt["path"].(string)

	// create a session in the worktree
	s, err := mgr.Create(p.ID, wtPath, "shell")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	_ = s

	// cascade-remove the worktree -> session + worktree + branch gone
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/projects/"+p.ID+"/worktrees", bytes.NewBufferString(`{"path":"`+wtPath+`"}`))
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != 204 {
		t.Fatalf("remove wt = %d", resp.StatusCode)
	}
	resp.Body.Close()
	if len(mgr.List()) != 0 {
		t.Fatalf("sessions remain after cascade: %v", mgr.List())
	}
	resp, _ = http.Get(srv.URL + "/api/projects/" + p.ID + "/worktrees")
	wts = nil
	json.NewDecoder(resp.Body).Decode(&wts)
	resp.Body.Close()
	if len(wts) != 1 {
		t.Fatalf("worktree not removed: %v", wts)
	}
}

func TestAPICreateSessionValidation(t *testing.T) {
	srv, _, ps := newTestServer(t)
	repo := gitRepo(t)
	p, _ := ps.Add("p", repo)
	// unknown project
	resp, _ := http.Post(srv.URL+"/api/sessions", "application/json", bytes.NewBufferString(`{"project":"nope","worktree":"`+repo+`","agent":"shell"}`))
	if resp.StatusCode != 404 {
		t.Fatalf("unknown project = %d", resp.StatusCode)
	}
	resp.Body.Close()
	// worktree not in git list
	resp, _ = http.Post(srv.URL+"/api/sessions", "application/json", bytes.NewBufferString(`{"project":"`+p.ID+`","worktree":"/nope","agent":"shell"}`))
	if resp.StatusCode != 400 {
		t.Fatalf("bad worktree = %d", resp.StatusCode)
	}
	resp.Body.Close()
	_ = os.Stat
}
```
Also: in `internal/server/ws_test.go` and `internal/server/static_test.go`, change every `New(mgr)` to `New(mgr, project.New(filepath.Join(t.TempDir(),"projects.json")))` and add the `"orchid/internal/project"` + `"path/filepath"` imports; in `ws_test.go` change the `mgr.Create("shell","/tmp")` call to `mgr.Create("p","/tmp","shell")`.

- [ ] **Step 2: Run** `go test ./internal/server/ -v` → FAIL (`New` arity; routes missing).

- [ ] **Step 3: Rewrite `internal/server/server.go`:**
```go
package server

import (
	"net/http"

	"orchid/internal/project"
	"orchid/internal/session"
	"orchid/web"
)

type Server struct {
	mgr      *session.Manager
	projects *project.Store
	mux      *http.ServeMux
}

func New(mgr *session.Manager, projects *project.Store) http.Handler {
	s := &Server{mgr: mgr, projects: projects, mux: http.NewServeMux()}
	s.routes()
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/sessions", s.handleList)
	s.mux.HandleFunc("POST /api/sessions", s.handleCreate)
	s.mux.HandleFunc("DELETE /api/sessions/{id}", s.handleKill)
	s.mux.HandleFunc("POST /api/sessions/{id}/restart", s.handleRestart)

	s.mux.HandleFunc("GET /api/projects", s.handleProjects)
	s.mux.HandleFunc("POST /api/projects", s.handleAddProject)
	s.mux.HandleFunc("DELETE /api/projects/{id}", s.handleRemoveProject)
	s.mux.HandleFunc("GET /api/projects/{id}/worktrees", s.handleWorktrees)
	s.mux.HandleFunc("POST /api/projects/{id}/worktrees", s.handleAddWorktree)
	s.mux.HandleFunc("DELETE /api/projects/{id}/worktrees", s.handleRemoveWorktree)

	s.mux.HandleFunc("GET /ws/{id}", s.handleWS)
	s.mux.Handle("/", http.FileServer(http.FS(web.FS)))
}
```

- [ ] **Step 4: Replace create + add project/worktree handlers in `internal/server/api.go`.** Change `handleCreate` and add the new handlers + cascade. Set the imports to include `gitwt`, `project`, `strings`. New `handleCreate`:
```go
type createReq struct {
	Project  string `json:"project"`
	Worktree string `json:"worktree"`
	Agent    string `json:"agent"`
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createReq
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	if req.Agent == "" || req.Project == "" || req.Worktree == "" {
		http.Error(w, "project, worktree and agent are required", http.StatusBadRequest)
		return
	}
	proj, ok := s.projects.Get(req.Project)
	if !ok {
		http.Error(w, "unknown project", http.StatusNotFound)
		return
	}
	wts, err := gitwt.List(proj.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	valid := false
	for _, wt := range wts {
		if wt.Path == req.Worktree {
			valid = true
			break
		}
	}
	if !valid {
		http.Error(w, "worktree not found in project", http.StatusBadRequest)
		return
	}
	sess, err := s.mgr.Create(req.Project, req.Worktree, req.Agent)
	if err != nil {
		if strings.Contains(err.Error(), "unknown agent") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": sess.ID, "agent": sess.Agent, "project": sess.Project,
		"worktree": sess.Worktree, "cwd": sess.Cwd, "status": "running", "alive": true,
	})
}
```
Add the project/worktree handlers (same file):
```go
type addProjectReq struct{ Name, Path string }
type addWorktreeReq struct{ Branch string }
type rmWorktreeReq struct{ Path string }

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.projects.List())
}

func (s *Server) handleAddProject(w http.ResponseWriter, r *http.Request) {
	var req addProjectReq
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Path == "" {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}
	p, err := s.projects.Add(req.Name, req.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleRemoveProject(w http.ResponseWriter, r *http.Request) {
	if err := s.projects.Remove(r.PathValue("id")); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type wtInfo struct {
	Path   string `json:"path"`
	Branch string `json:"branch"`
	Name   string `json:"name"`
	IsMain bool   `json:"is_main"`
}

func (s *Server) handleWorktrees(w http.ResponseWriter, r *http.Request) {
	proj, ok := s.projects.Get(r.PathValue("id"))
	if !ok {
		http.Error(w, "unknown project", http.StatusNotFound)
		return
	}
	wts, err := gitwt.List(proj.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]wtInfo, 0, len(wts))
	for _, wt := range wts {
		name := wt.Branch
		if wt.IsMain {
			name = "main"
		}
		out = append(out, wtInfo{Path: wt.Path, Branch: wt.Branch, Name: name, IsMain: wt.IsMain})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleAddWorktree(w http.ResponseWriter, r *http.Request) {
	proj, ok := s.projects.Get(r.PathValue("id"))
	if !ok {
		http.Error(w, "unknown project", http.StatusNotFound)
		return
	}
	var req addWorktreeReq
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Branch == "" {
		http.Error(w, "branch is required", http.StatusBadRequest)
		return
	}
	path, err := gitwt.Add(proj.Path, req.Branch)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	writeJSON(w, http.StatusOK, wtInfo{Path: path, Branch: req.Branch, Name: req.Branch, IsMain: false})
}

func (s *Server) handleRemoveWorktree(w http.ResponseWriter, r *http.Request) {
	proj, ok := s.projects.Get(r.PathValue("id"))
	if !ok {
		http.Error(w, "unknown project", http.StatusNotFound)
		return
	}
	var req rmWorktreeReq
	_ = json.NewDecoder(r.Body).Decode(&req)
	wts, err := gitwt.List(proj.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var target *gitwtWorktree
	for i := range wts {
		if wts[i].Path == req.Path {
			target = &wts[i]
			break
		}
	}
	if target == nil {
		http.Error(w, "worktree not found", http.StatusNotFound)
		return
	}
	if target.IsMain {
		http.Error(w, "cannot remove the main worktree", http.StatusBadRequest)
		return
	}
	// cascade: kill sessions in this worktree, remove worktree, delete branch.
	s.mgr.KillByWorktree(target.Path)
	if err := gitwt.Remove(proj.Path, target.Path); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if target.Branch != "" {
		_ = gitwt.DeleteBranch(proj.Path, target.Branch) // non-fatal
	}
	w.WriteHeader(http.StatusNoContent)
}
```
Note: `gitwtWorktree` above must be `gitwt.Worktree` — use that type (the alias is illustrative). Add imports `orchid/internal/gitwt` (and keep `encoding/json`, `net/http`, `strings`). Remove `_ = os.Stat`-style leftovers.

- [ ] **Step 5: Run** `go test ./internal/server/ -race -v` → PASS (or SKIP without git/tmux).

- [ ] **Step 6: Commit**
```bash
git add internal/server/
git commit -m "feat: project + worktree API with remove-worktree cascade; worktree-bound create"
```

---

## Task 5: Wire CLI/client (compile fixes) + git preflight + project store

**Files:** Modify `internal/client/client.go`, `internal/client/client_test.go`, `cmd/orchid/cli.go`.

- [ ] **Step 1: Update `client.Create` signature.** In `internal/client/client.go`, change `Create`:
```go
func (c *Client) Create(project, worktree, agent string) (session.Info, error) {
	body, _ := json.Marshal(map[string]string{"project": project, "worktree": worktree, "agent": agent})
	resp, err := c.do(http.MethodPost, "/api/sessions", bytes.NewReader(body))
	if err != nil {
		return session.Info{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return session.Info{}, errFromResp(resp)
	}
	var info session.Info
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return session.Info{}, err
	}
	return info, nil
}
```
In `internal/client/client_test.go`, update `TestCreate` and `TestCreateUnknownAgent400` to call `c.Create("p-1", "/tmp", "claude")` / `c.Create("p-1","/tmp","nope")` (the stub handlers don't inspect the body, so only the call site changes).

- [ ] **Step 2: Update `cmd/orchid/cli.go`.**

(a) In `serveCmd`, add a git preflight next to the tmux one and wire the project store into `server.New`:
```go
	if err := tmux.Available(); err != nil {
		log.Printf("orchid requires tmux: %v (install tmux and retry)", err)
		return 1
	}
	if err := gitwt.Available(); err != nil {
		log.Printf("orchid requires git: %v", err)
		return 1
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	mgr := session.NewManager(sessionsPath())
	if err := mgr.Restore(); err != nil {
		log.Printf("orchid: restore sessions: %v", err)
	}
	ps := project.New(projectsPath())
	if err := ps.Load(); err != nil {
		log.Printf("orchid: load projects: %v", err)
	}
	srv := &http.Server{Addr: addr, Handler: server.New(mgr, ps)}
```
Add a `projectsPath()` helper next to `sessionsPath()`:
```go
func projectsPath() string {
	return filepath.Join(filepath.Dir(sessionsPath()), "projects.json")
}
```
Add imports `orchid/internal/gitwt` and `orchid/internal/project`.

(b) Update `createCmd` to the new signature (`orchid create <project> <worktree> <agent>`):
```go
func createCmd(args []string) int {
	if len(args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: orchid create <project> <worktree> <agent>")
		return 2
	}
	c, err := client.FromRuntime()
	if err != nil {
		fmt.Fprintln(os.Stderr, "orchid:", err)
		return 1
	}
	info, err := c.Create(args[0], args[1], args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, "orchid:", err)
		return 1
	}
	fmt.Println(info.ID)
	return 0
}
```

- [ ] **Step 3: Build + vet + test**

Run: `go build ./... && go vet ./... && go test ./... -race`
Expected: all PASS (or git/tmux-dependent tests skip).

- [ ] **Step 4: Commit**
```bash
git add internal/client/ cmd/orchid/cli.go
git commit -m "feat: wire git preflight + project store; client/CLI create takes project+worktree"
```

---

## Task 6: Frontend — projects list + project detail + modals

**Files:** Replace `web/app.js`.

**Context:** The design CSS (`.proj-*`, `.wt-*`, `.crumb`, `.choice-grid`, modal classes) is already vendored. The current `app.js` (M5) renders a flat list + focus. This task replaces it with: a projects list (home), a project detail (worktree accordion groups of sessions), the Focus terminal (unchanged behavior), and three modals (project / worktree / session). The terminal + Monokai theme + xterm wiring are reused verbatim.

- [ ] **Step 1: Replace `web/app.js`** with the following. (It keeps the `AGENT`, `MONOKAI`, `icon`, terminal-attach, and focus logic from M5; adds project/worktree screens + modals + a `cur` nav state.)

```js
/* orchid dashboard — projects -> worktrees -> sessions, over the real xterm/WS backend */
const main = document.getElementById("main");

const AGENT = {
  claude: { label: "Claude", badge: "C", cmd: "claude" },
  codex:  { label: "Codex",  badge: "Cx", cmd: "codex" },
  shell:  { label: "Shell",  badge: ">_", cmd: "zsh" },
};
const agentMeta = (a) => AGENT[a] || AGENT.shell;
const statusMeta = (s) => s === "running"
  ? { cls: "ok", label: "Running", pulse: true }
  : { cls: "idle", label: "Idle", pulse: false };

const MONOKAI = {
  background: "#08080b", foreground: "#f8f8f2", cursor: "#a6e22e", cursorAccent: "#08080b",
  selectionBackground: "rgba(124,156,255,0.30)",
  black: "#272822", brightBlack: "#75715e", red: "#f92672", brightRed: "#f92672",
  green: "#a6e22e", brightGreen: "#a6e22e", yellow: "#e6db74", brightYellow: "#f4bf75",
  blue: "#66d9ef", brightBlue: "#66d9ef", magenta: "#ae81ff", brightMagenta: "#ae81ff",
  cyan: "#a1efe4", brightCyan: "#a1efe4", white: "#f8f8f2", brightWhite: "#f9f8f5",
};

const ICONS = {
  plus:    '<path d="M12 5v14M5 12h14"/>',
  restart: '<path d="M3 12a9 9 0 1 0 3-6.7L3 8"/><path d="M3 3v5h5"/>',
  kill:    '<rect x="6" y="6" width="12" height="12" rx="1.5"/>',
  list:    '<path d="M8 6h12M8 12h12M8 18h12"/><circle cx="4" cy="6" r="1.2"/><circle cx="4" cy="12" r="1.2"/><circle cx="4" cy="18" r="1.2"/>',
  folder:  '<path d="M3 7a2 2 0 0 1 2-2h4l2 2.5h8a2 2 0 0 1 2 2V18a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7z"/>',
  branch:  '<circle cx="6" cy="6" r="2.4"/><circle cx="6" cy="18" r="2.4"/><circle cx="18" cy="8" r="2.4"/><path d="M6 8.4v7.2M8.3 7.2C12 8 14.5 8.5 15.6 9.8M15.6 9.8c.6.7.9 1.6.9 2.7v1.5"/>',
  chevR:   '<path d="M9 6l6 6-6 6"/>',
};
const icon = (n, sz = 16) =>
  `<svg width="${sz}" height="${sz}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">${ICONS[n] || ""}</svg>`;

/* ---------- state ---------- */
let projects = [];
let cur = { projectId: null, view: "list", activeId: null }; // view: list | focus
let worktrees = [];   // for the open project
let sessions = [];    // for the open project (filtered)
let term = null, fit = null, ws = null, ro = null;

/* ---------- templates ---------- */
const tplProjRow = _.template(
  '<div class="proj-row" data-pid="<%- p.id %>">' +
    '<div><div class="name">' + icon("folder", 17) + '<%- p.name %></div>' +
      '<div class="path"><%- p.path %></div></div>' +
    '<div class="proj-meta"><span class="faint">open</span>' + icon("chevR", 16) + '</div>' +
  '</div>'
);
const tplSessRow = _.template(
  '<div class="s-row" data-id="<%- s.id %>">' +
    '<div class="agent-badge <%- s.agent %> lg"><%- m.badge %></div>' +
    '<div class="meta"><div class="sname"><%- m.label %></div><div class="stype"><%- m.cmd %></div></div>' +
    '<div class="right"><div class="stat-w"><span class="status-pill <%- st.cls %>"><span class="dot <%- st.cls %><%- st.pulse ? " pulse" : "" %>"></span><%- st.label %></span></div>' +
      '<div class="acts">' +
        '<button class="btn sm icon ghost" data-act="restart" title="Restart">' + icon("restart", 13) + '</button>' +
        '<button class="btn sm icon ghost danger" data-act="kill" title="Kill">' + icon("kill", 12) + '</button>' +
      '</div></div>' +
  '</div>'
);
const tplRailItem = _.template(
  '<div class="rail-item <%- active ? "active" : "" %>" data-id="<%- s.id %>">' +
    '<div class="agent-badge <%- s.agent %>"><%- m.badge %></div>' +
    '<div class="meta"><div class="sname"><%- m.label %></div><div class="stype"><%- m.cmd %></div></div>' +
    '<span class="dot <%- st.cls %><%- st.pulse ? " pulse" : "" %>"></span>' +
  '</div>'
);

const rd = (s) => ({ s, m: agentMeta(s.agent), st: statusMeta(s.status) });
const esc = (x) => _.escape(x == null ? "" : x);

/* ---------- data ---------- */
async function loadProjects() { projects = await (await fetch("/api/projects")).json(); }
async function loadProject(pid) {
  worktrees = await (await fetch("/api/projects/" + pid + "/worktrees")).json();
  const all = await (await fetch("/api/sessions")).json();
  sessions = all.filter((s) => s.project === pid);
}

async function refresh() {
  if (!cur.projectId) { await loadProjects(); renderProjects(); return; }
  await loadProject(cur.projectId);
  if (cur.view === "focus" && cur.activeId) updateRail();
  else renderDetail();
}

/* ---------- projects list ---------- */
function renderProjects() {
  teardownTerm();
  const body = projects.length
    ? '<div class="proj-list">' + projects.map((p) => tplProjRow({ p })).join("") + "</div>"
    : '<div class="empty"><div class="big">No projects yet</div><div style="margin-bottom:18px">Add a git repository to start.</div>' +
      '<button class="btn primary" id="newProj" style="margin:0 auto">' + icon("plus", 15) + " New project</button></div>";
  main.innerHTML =
    '<div class="wrap"><div class="page-head"><div><h1>Projects</h1>' +
      '<div class="sub">' + projects.length + " project" + (projects.length === 1 ? "" : "s") + "</div></div>" +
      '<button class="btn primary" id="newProj">' + icon("plus", 15) + " New project</button></div>" +
      body + "</div>";
}

/* ---------- project detail ---------- */
function renderDetail() {
  teardownTerm();
  const proj = projects.find((p) => p.id === cur.projectId) || { name: "", path: "" };
  const groups = worktrees.map((wt) => {
    const rows = sessions.filter((s) => s.worktree === wt.path);
    const sessHtml = rows.length
      ? '<div class="sess-list">' + rows.map((s) => tplSessRow(rd(s))).join("") + "</div>"
      : '<div class="faint" style="font-size:12.5px;padding:4px 2px 10px">no sessions</div>';
    const rm = wt.is_main ? "" :
      '<button class="btn ghost sm danger" data-rmwt="' + esc(wt.path) + '" data-branch="' + esc(wt.branch) + '" title="Remove worktree">' + icon("kill", 12) + " Remove</button>";
    return '<div class="wt-group">' +
      '<div class="wt-group-head"><span class="br">' + icon("branch", 13) + esc(wt.name) + "</span>" +
        '<span class="count">· ' + rows.length + " session" + (rows.length === 1 ? "" : "s") + "</span><span class=\"spacer\"></span>" +
        '<button class="btn ghost sm" data-newsess="' + esc(wt.path) + '">' + icon("plus", 13) + " Session</button>" + rm +
      "</div>" + sessHtml + "</div>";
  }).join("");
  main.innerHTML =
    '<div class="wrap"><div class="detail-head">' +
      '<div class="crumb"><button id="backProjects">Projects</button>' + icon("chevR", 13) +
        '<span style="color:var(--text-dim)">' + esc(proj.name) + "</span></div>" +
      '<div class="detail-title">' + icon("folder", 20) + "<h1>" + esc(proj.name) + '</h1><span class="path">' + esc(proj.path) + "</span></div></div>" +
      groups +
      '<button class="btn ghost sm" id="newWt" style="margin-top:6px">' + icon("plus", 13) + " New worktree</button>" +
    "</div>";
}

/* ---------- focus ---------- */
function enterFocus(id) {
  cur.view = "focus"; cur.activeId = id;
  main.innerHTML =
    '<div class="wrap focus-mode"><div class="tools"><div class="left">' +
      '<button class="btn ghost sm" id="backDetail">' + icon("list", 14) + " Back</button></div></div>" +
      '<div class="focus"><div class="focus-rail"><div class="rail-head">Sessions<span class="spacer"></span></div><div id="rail"></div></div>' +
      '<div class="focus-term" id="focusTerm"></div></div></div>';
  updateRail();
  attachTerminal(id);
}
function updateRail() {
  const rail = document.getElementById("rail");
  if (!rail) return;
  rail.innerHTML = sessions.map((s) => tplRailItem({ ...rd(s), active: s.id === cur.activeId })).join("");
}

/* ---------- terminal (unchanged from M5) ---------- */
function attachTerminal(id) {
  teardownTerm();
  const host = document.getElementById("focusTerm");
  if (!host) return;
  const s = sessions.find((x) => x.id === id);
  if (!s) { host.innerHTML = ""; return; }
  const m = agentMeta(s.agent), st = statusMeta(s.status);
  host.innerHTML =
    '<div class="term" style="flex:1;min-height:0"><div class="term-head">' +
      '<div class="agent-badge ' + esc(s.agent) + '">' + m.badge + "</div>" +
      '<div class="term-id"><div class="t">' + esc(m.label) + "</div></div>" +
      '<span class="tt">' + m.cmd + '</span><div class="spacer"></div>' +
      '<span class="status-pill ' + st.cls + '"><span class="dot ' + st.cls + (st.pulse ? " pulse" : "") + '"></span>' + st.label + "</span>" +
      '<button class="btn sm icon ghost" data-act="restart" title="Restart">' + icon("restart", 14) + "</button>" +
      '<button class="btn sm icon ghost danger" data-act="kill" title="Kill">' + icon("kill", 13) + "</button>" +
    "</div><div id=\"termBody\" style=\"flex:1;min-height:0;overflow:hidden;background:#08080b\"></div></div>";
  const tb = document.getElementById("termBody");
  if (s.status !== "running") { tb.innerHTML = '<div id="placeholder">stopped — restart to attach</div>'; return; }
  term = new Terminal({ cursorBlink: true, fontSize: 13, scrollback: 5000, theme: MONOKAI });
  fit = new FitAddon.FitAddon(); term.loadAddon(fit); term.open(tb); fit.fit();
  const proto = location.protocol === "https:" ? "wss" : "ws";
  ws = new WebSocket(`${proto}://${location.host}/ws/${id}`); ws.binaryType = "arraybuffer";
  ws.onopen = () => sendResize();
  ws.onmessage = (e) => { if (e.data instanceof ArrayBuffer) term.write(new Uint8Array(e.data)); };
  ws.onclose = () => {};
  term.onData((d) => { if (ws && ws.readyState === WebSocket.OPEN) ws.send(new TextEncoder().encode(d)); });
  ro = new ResizeObserver(() => { if (fit) { fit.fit(); sendResize(); } });
  ro.observe(tb);
}
function teardownTerm() {
  if (ro) { ro.disconnect(); ro = null; }
  if (ws) { ws.onclose = null; ws.close(); ws = null; }
  if (term) { term.dispose(); term = null; }
  fit = null;
}
function sendResize() { if (term && ws && ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows })); }

/* ---------- modals ---------- */
function modal(html, wire) {
  const mount = document.createElement("div");
  mount.innerHTML = '<div class="scrim" id="scrim"><div class="modal">' + html + "</div></div>";
  document.body.appendChild(mount);
  const close = () => { mount.remove(); document.removeEventListener("keydown", onEsc); };
  function onEsc(e) { if (e.key === "Escape") close(); }
  document.addEventListener("keydown", onEsc);
  mount.querySelector("#scrim").addEventListener("mousedown", (e) => { if (e.target.id === "scrim") close(); });
  wire(mount, close);
}
function newProjectModal() {
  modal(
    '<div class="m-head"><h2>New project</h2><p>A git repository directory (must contain .git).</p></div>' +
    '<div class="m-body"><div class="field"><label>Name (optional)</label><input class="text" id="pname" placeholder="my-service" /></div>' +
    '<div class="field"><label>Path</label><input class="text mono" id="ppath" placeholder="~/code/my-service" /></div></div>' +
    '<div class="m-foot"><button class="btn ghost" id="cancel">Cancel</button><button class="btn primary" id="ok">Create</button></div>',
    (mt, close) => {
      mt.querySelector("#cancel").onclick = close;
      mt.querySelector("#ok").onclick = async () => {
        const r = await fetch("/api/projects", { method: "POST", headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ name: mt.querySelector("#pname").value.trim(), path: mt.querySelector("#ppath").value.trim() }) });
        if (!r.ok) { alert("create failed: " + (await r.text())); return; }
        close(); await refresh();
      };
    });
}
function newWorktreeModal() {
  modal(
    '<div class="m-head"><h2>New worktree</h2><p>Creates a new branch off the current HEAD.</p></div>' +
    '<div class="m-body"><div class="field"><label>Branch</label><input class="text mono" id="branch" placeholder="feat/new-thing" /></div></div>' +
    '<div class="m-foot"><button class="btn ghost" id="cancel">Cancel</button><button class="btn primary" id="ok">Create</button></div>',
    (mt, close) => {
      mt.querySelector("#cancel").onclick = close;
      mt.querySelector("#ok").onclick = async () => {
        const r = await fetch("/api/projects/" + cur.projectId + "/worktrees", { method: "POST", headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ branch: mt.querySelector("#branch").value.trim() }) });
        if (!r.ok) { alert("create failed: " + (await r.text())); return; }
        close(); await refresh();
      };
    });
}
function newSessionModal(defaultWt) {
  const types = [{ id: "claude", l: "Claude", b: "C", d: "AI agent" }, { id: "codex", l: "Codex", b: "Cx", d: "AI agent" }, { id: "shell", l: "Shell", b: ">_", d: "zsh shell" }];
  let sel = "claude";
  const opts = worktrees.map((wt) => '<option value="' + esc(wt.path) + '"' + (wt.path === defaultWt ? " selected" : "") + ">" + esc(wt.name) + "</option>").join("");
  const grid = types.map((t) => '<button class="choice" data-type="' + t.id + '"><div class="agent-badge ' + t.id + ' lg">' + t.b + '</div><span class="lbl">' + t.l + '</span><span class="desc">' + t.d + "</span></button>").join("");
  modal(
    '<div class="m-head"><h2>New session</h2></div>' +
    '<div class="m-body"><div class="field"><label>Worktree</label><select class="text mono" id="wt">' + opts + "</select></div>" +
    '<div class="field"><label>Type</label><div class="choice-grid">' + grid + "</div></div></div>" +
    '<div class="m-foot"><button class="btn ghost" id="cancel">Cancel</button><button class="btn primary" id="ok">Start</button></div>',
    (mt, close) => {
      mt.querySelector("#cancel").onclick = close;
      const mark = () => mt.querySelectorAll(".choice").forEach((x) => x.classList.toggle("sel", x.dataset.type === sel));
      mt.querySelectorAll(".choice").forEach((b) => b.onclick = () => { sel = b.dataset.type; mark(); });
      mark();
      mt.querySelector("#ok").onclick = async () => {
        const r = await fetch("/api/sessions", { method: "POST", headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ project: cur.projectId, worktree: mt.querySelector("#wt").value, agent: sel }) });
        if (!r.ok) { alert("create failed: " + (await r.text())); return; }
        const s = await r.json(); close(); await refresh(); enterFocus(s.id);
      };
    });
}

/* ---------- event delegation ---------- */
main.addEventListener("click", async (e) => {
  const act = e.target.closest("[data-act]");
  if (act) {
    e.stopPropagation();
    const holder = act.closest("[data-id]");
    const id = holder ? holder.dataset.id : cur.activeId;
    if (act.dataset.act === "kill") {
      await fetch("/api/sessions/" + id, { method: "DELETE" });
      if (id === cur.activeId) { teardownTerm(); cur.view = "list"; cur.activeId = null; }
      await refresh();
    } else if (act.dataset.act === "restart") {
      const r = await fetch("/api/sessions/" + id + "/restart", { method: "POST" });
      if (!r.ok) { alert("restart failed: " + (await r.text())); return; }
      await refresh();
      if (cur.view === "focus") attachTerminal(id);
    }
    return;
  }
  const rmwt = e.target.closest("[data-rmwt]");
  if (rmwt) {
    const path = rmwt.dataset.rmwt, branch = rmwt.dataset.branch;
    const n = sessions.filter((s) => s.worktree === path).length;
    if (!confirm(`Remove worktree "${branch}"? Kills ${n} session(s), removes the worktree, deletes the branch.`)) return;
    await fetch("/api/projects/" + cur.projectId + "/worktrees", { method: "DELETE", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ path }) });
    await refresh();
    return;
  }
  const ns = e.target.closest("[data-newsess]");
  if (ns) { newSessionModal(ns.dataset.newsess); return; }
  if (e.target.closest("#newProj")) { newProjectModal(); return; }
  if (e.target.closest("#newWt")) { newWorktreeModal(); return; }
  if (e.target.closest("#backProjects")) { cur = { projectId: null, view: "list", activeId: null }; renderProjects(); return; }
  if (e.target.closest("#backDetail")) { cur.view = "list"; cur.activeId = null; renderDetail(); return; }
  const rail = e.target.closest(".rail-item");
  if (rail) { cur.activeId = rail.dataset.id; updateRail(); attachTerminal(cur.activeId); return; }
  const srow = e.target.closest(".s-row");
  if (srow) { enterFocus(srow.dataset.id); return; }
  const prow = e.target.closest(".proj-row");
  if (prow) { cur = { projectId: prow.dataset.pid, view: "list", activeId: null }; refresh(); return; }
});

refresh();
```

- [ ] **Step 2: Build + JS syntax**

Run: `go build ./... && node --check web/app.js`
Expected: clean.

- [ ] **Step 3: Commit**
```bash
git add web/app.js
git commit -m "feat: projects list + project detail (worktree groups) + modals"
```

---

## Task 7: Full gate + E2E smoke

- [ ] **Step 1: Whole repo**

Run: `go build ./... && go vet ./... && go test ./... -race`
Expected: all PASS (git+tmux present).

- [ ] **Step 2: E2E (real git + tmux)**

Run:
```bash
rm -f "$HOME/.config/orchid/sessions.json" "$HOME/.config/orchid/projects.json"
go build -o orchid ./cmd/orchid && ./orchid -port 18170 >/tmp/o.log 2>&1 &
sleep 1
# use this repo as a project (it has .git)
PID=$(curl -sf -X POST -H 'Content-Type: application/json' -d '{"name":"orchid","path":"'"$PWD"'"}' http://127.0.0.1:18170/api/projects | sed 's/.*"id":"\([^"]*\)".*/\1/')
echo "project=$PID"
curl -sf http://127.0.0.1:18170/api/projects/$PID/worktrees; echo                       # main only
WT=$(curl -sf -X POST -H 'Content-Type: application/json' -d '{"branch":"feat/orchid-e2e"}' http://127.0.0.1:18170/api/projects/$PID/worktrees | sed 's/.*"path":"\([^"]*\)".*/\1/')
echo "worktree=$WT"; ls -d "$WT"                                                          # dir exists under .orchid/worktrees
SID=$(curl -sf -X POST -H 'Content-Type: application/json' -d '{"project":"'"$PID"'","worktree":"'"$WT"'","agent":"shell"}' http://127.0.0.1:18170/api/sessions | sed 's/.*"id":"\([^"]*\)".*/\1/')
echo "session=$SID"; tmux has-session -t "orchid-$SID" && echo "tmux ok"
# cascade-remove the worktree -> session + dir + branch gone
curl -sf -X DELETE -H 'Content-Type: application/json' -d '{"path":"'"$WT"'"}' http://127.0.0.1:18170/api/projects/$PID/worktrees
echo "after remove:"; curl -sf http://127.0.0.1:18170/api/sessions; echo                  # session gone
test ! -d "$WT" && echo "worktree dir removed"
git branch --list feat/orchid-e2e | grep -q . && echo "BRANCH STILL EXISTS (bad)" || echo "branch deleted"
tmux has-session -t "orchid-$SID" 2>/dev/null && echo "TMUX STILL EXISTS (bad)" || echo "tmux session gone"
```
Expected: project created; worktrees=main only; worktree dir created under `.orchid/worktrees/feat-orchid-e2e`; session created + tmux ok; after DELETE — session list empty, worktree dir removed, branch deleted, tmux session gone.

- [ ] **Step 3: Browser check (manual, optional CDP screenshot)**

Open `http://127.0.0.1:18170`: Projects list → add project (this repo) → open → see `main` group → New worktree → New session (worktree select) → terminal attaches → Remove worktree confirms + cascades.

- [ ] **Step 4: Cleanup**
```bash
pkill -f 'orchid -port' 2>/dev/null
git worktree list --porcelain | grep -q '.orchid/worktrees' && git worktree remove --force .orchid/worktrees/* 2>/dev/null
git branch --list 'feat/orchid-e2e' | grep -q . && git branch -D feat/orchid-e2e 2>/dev/null
tmux ls 2>/dev/null | grep '^orchid-' | cut -d: -f1 | xargs -r -n1 tmux kill-session -t
rm -rf orchid "$HOME/.config/orchid/sessions.json" "$HOME/.config/orchid/projects.json" .orchid
```

- [ ] **Step 5: Commit (only if fixes were needed)**

No code change expected here.

---

## Self-Review Notes

- **Spec coverage:** projects.json store + `.git` validation + idempotent dup (T1); gitwt List/Add(-b from HEAD)/Remove/DeleteBranch + porcelain parse + `.orchid/worktrees` (T2); Record/Info `Project`+`Worktree`, `Create(project,worktree,agent)`, `KillByWorktree`, Restore drops legacy (T3); project + worktree routes, create-session validation (project exists + worktree in git list + agent), remove-worktree cascade, `main` protected (T4); git preflight + project store wiring + client/CLI create sig (T5); projects list + detail (worktree groups) + 3 modals + remove-worktree confirm (T6); gate + E2E incl. cascade (T7). All spec sections mapped.
- **Type consistency:** `project.Project{ID,Name,Path}`, `project.Store` methods; `gitwt.Worktree{Path,Branch,IsMain}`, `gitwt.List/Add/Remove/DeleteBranch/Available`; `store.Record`+`session.Info`+`Session` gain `Project,Worktree`; `Manager.Create(projectID,worktree,agentName)` + `KillByWorktree(worktree)`; `server.New(mgr,projects)`; `client.Create(project,worktree,agent)`; create JSON body `{project,worktree,agent}`; worktree JSON `{path,branch,name,is_main}`. Consistent across tasks.
- **Clean break:** Restore drops records lacking project/worktree (T3); old flat sessions.json is discarded — documented.
- **Frontend verification:** no JS tests; E2E (T7) exercises the API end-to-end incl. cascade; browser/CDP is manual. Flagged so it's not a silent gap.
- **gitwt.Worktree alias note:** Task 4's `gitwtWorktree` placeholder must be written as `gitwt.Worktree` in the real code — called out inline.
```
