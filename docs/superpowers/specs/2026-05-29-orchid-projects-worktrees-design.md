# Orchid — Projects & Git Worktrees (Milestone 6)

**Date:** 2026-05-29
**Status:** Approved design

## Context

Until now orchid manages a flat list of tmux-backed sessions, each created with a
free-form `cwd`. This milestone adds the **project → worktree → session**
hierarchy the design always assumed (deferred-spec items #1 and #2): a project is
a git repository directory; worktrees are real `git worktree`s; every session is
created in — and hard-bound to — a worktree's folder.

This implements deferred features #1 (projects) and #2 (worktrees) from
`2026-05-29-orchid-dashboard-deferred-spec.md`, plus the session→worktree binding.
Backend (Go) + frontend (the design's `ProjectsList` → `ProjectDetail`, replacing
the flat dashboard).

## Requirements (from the user)

- Create a **project** linked to a directory that must contain `.git`.
- Create **worktrees** via `git worktree`; the root worktree is named `main`; new
  worktrees live in `<repo>/.orchid/worktrees/`.
- Worktrees are read from `git worktree list` (git is the source of truth).
- Each session, at creation, uses the folder of its worktree; a session is **hard
  bound** to a worktree. The root folder (`main`) cannot be changed.
- Projects are stored in a separate JSON file.
- A worktree can be **removed**, which **deletes its branch and closes+kills all
  its sessions** first.

## Decisions

- **Clean break (B):** sessions now require a project + worktree. On load, records
  lacking a `Project`/`Worktree` binding are dropped (the existing flat
  `sessions.json` is disposable dev state).
- **Git is truth (A):** worktrees are never persisted; always derived from
  `git worktree list` per project. A session binds to a worktree by its absolute
  **path**; `cwd` = that path. `main` = the repo root path.
- **New-branch-from-HEAD (A):** worktree create = `git worktree add
  <repo>/.orchid/worktrees/<sani(branch)> -b <branch>` (off current HEAD). The
  branch field is a NEW branch name (placeholder `feat/new-thing`); it is not
  prefilled with `master` (that would fail).
- **Remove-worktree cascade:** kill all sessions with `Worktree==path` → `git
  worktree remove --force <path>` → `git branch -D <branch>`. `main` is protected.

## Data model

```
~/.config/orchid/projects.json  = [{ "id","name","path" }]   (path = repo root with .git; immutable)
~/.config/orchid/sessions.json  = records (now incl. project + worktree)
git (per project)               = worktree source of truth (git worktree list)
```

- **Project** `{ID, Name, Path}` — `Path` absolute, contains `.git`.
- **Worktree** (derived) `{Path, Branch, IsMain}` — `IsMain` = repo root, shown as
  `main`, not removable. New worktrees: `<repo>/.orchid/worktrees/<sani(branch)>`
  where `sani` replaces `/` with `-`.
- **store.Record** gains `Project string` (project id) + `Worktree string` (abs
  worktree path). `Cwd` = `Worktree`.

## Components

### internal/project (NEW)
Projects store, atomic JSON (mirrors `internal/store`/`internal/runtime`):
```go
type Project struct { ID, Name, Path string }     // json: id,name,path
type Store struct { /* path, mu, projects */ }
func New(path string) *Store
func (s *Store) Load() error                       // missing file -> empty
func (s *Store) List() []Project
func (s *Store) Get(id string) (Project, bool)
func (s *Store) Add(name, path string) (Project, error)  // requires <path>/.git; if path already a project, return existing (idempotent)
func (s *Store) Remove(id string) error
```
`.git` validation: `os.Stat(filepath.Join(path, ".git"))` exists (dir for normal
repos, file for worktrees/submodules — accept either). `path` is made absolute.

### internal/gitwt (NEW)
Thin `git` CLI wrapper (mirrors `internal/tmux`):
```go
type Worktree struct { Path, Branch string; IsMain bool }   // json: path,branch,is_main
func Available() error                                       // git --version
func List(repo string) ([]Worktree, error)                  // git -C repo worktree list --porcelain
func Add(repo, branch string) (string, error)               // worktree add <repo>/.orchid/worktrees/<sani> -b branch ; returns abs path
func Remove(repo, path string) error                        // git -C repo worktree remove --force path
func DeleteBranch(repo, branch string) error                // git -C repo branch -D branch
```
`List` parses porcelain blocks (`worktree <path>`, `HEAD <sha>`, `branch
refs/heads/<b>`, `bare`, `detached`). The first non-bare entry (repo root) →
`IsMain=true`, `Branch` from its head. `Add` creates `.orchid/worktrees/` as
needed; sanitizes the branch into the folder name.

### internal/session (MODIFIED)
- `Record` + `Info` gain `Project`, `Worktree` (json `project`, `worktree`).
- `Manager.Create(projectID, worktreePath, agentName) (*Session, error)` — sets
  `Cwd = worktreePath`, stores project + worktree. (Replaces `Create(agent,cwd)`.)
- `Manager.KillByWorktree(path string) int` — kills every session (running or
  stopped) whose `Worktree == path` (tmux kill + drop records + save); returns
  count.
- `Restore` drops records lacking `Project`/`Worktree`.
- `List`/`Info` include `Project`, `Worktree`.

### internal/server (MODIFIED)
`New(mgr, projects)` gains the project store; a `removeWorktree(projID, path)`
helper orchestrates the cascade. New/changed routes below.

## API

```
GET    /api/projects                           -> [{id,name,path}]
POST   /api/projects        {name,path}         -> 200 {id,name,path}; 400 no .git / bad path; duplicate path returns the existing project (idempotent 200)
DELETE /api/projects/{id}                        -> 204; 404 unknown   (json only; on-disk untouched)

GET    /api/projects/{id}/worktrees             -> [{path,branch,name,is_main}]   (git worktree list; name=main|branch)
POST   /api/projects/{id}/worktrees {branch}    -> 200 {path,branch,is_main:false}; 409 branch exists; 400 bad/empty; 404 project
DELETE /api/projects/{id}/worktrees {path}      -> 204 cascade (kill sessions -> remove worktree -> delete branch); 400 if main; 404

POST   /api/sessions   {project,worktree,agent} -> 200 Info; 404 unknown project; 400 worktree not in git list / unknown agent
GET    /api/sessions                             -> []Info (incl. project, worktree)
DELETE /api/sessions/{id}                         -> 204 (kill, unchanged)
POST   /api/sessions/{id}/restart                 -> 200/404/409 (unchanged)
GET    /ws/{id}                                   -> unchanged
```

- The worktree DELETE carries `path` in a JSON body (paths contain slashes, unfit
  for a path param). The handler validates the path is a real worktree of the
  project and not `IsMain`.
- Create-session validates: project exists; `worktree` is present in that
  project's live `git worktree list` (rejects stale/arbitrary cwd); agent known.

## Frontend (replaces flat dashboard)

Nav state `{projectId, view}` (`view`: `"list"` | `"focus"`). New underscore
templates; reuses already-vendored design CSS (`.proj-*`, `.wt-*`, `.crumb`,
modals, `.choice-grid`).

- **Projects list** (no project): design `proj-row` per project — folder icon,
  name, path, worktree count + session count + status dots; `+ New project`.
- **Project detail**: breadcrumb (`Projects › name`), title + path. **Worktree
  groups** (`wt-group` accordion): head = branch (`main` first) + session count +
  `+ Session` + `✕ Remove` (omitted for `main`); body = the worktree's `s-row`
  sessions (restart/kill). `+ New worktree` at the bottom.
- **Focus**: clicking a session opens the existing Focus view (rail = the
  project's sessions across worktrees) with the real xterm (Monokai) — unchanged.
- **Modals**: New project (name + path); New worktree (branch); New session
  (agent choice-grid + worktree `<select>` defaulting to the clicked group) →
  `POST {project,worktree,agent}`.
- **Remove worktree**: confirm ("kills N sessions, removes the worktree, deletes
  branch `<b>`") → `DELETE …/worktrees {path}`.
- Data: `GET /api/projects`; on open → `GET …/worktrees` + `GET /api/sessions`
  (filtered to the project, grouped client-side by `worktree` path).

## Errors & edge cases

- `git` missing → `gitwt.Available()` checked at startup (alongside the tmux
  preflight) → fatal with install hint.
- Add project: relative path → made absolute; missing `.git` → 400; duplicate
  path → return the existing project (idempotent, 200).
- Add worktree: existing branch / sanitized-path collision → git error → 409.
- Remove worktree on `main` → 400. Dirty tree handled by `--force` (sessions are
  cascade-killed first regardless).
- Create session: unknown project → 404; worktree absent from live list → 400;
  unknown agent → 400.
- `git branch -D` failing after the worktree is removed → logged, non-fatal.
- Restore drops records with no project/worktree, and records whose worktree path
  is no longer in the project's `git worktree list`.

## Testing

- **project (unit):** `Add` validates `.git` (fake repo dir), List/Get/Remove,
  atomic save, dedupe; missing file → empty.
- **gitwt (integration, real git, skip if absent):** in a tmpdir repo with a
  commit — `List` (main only, then after `Add` two); `Add` creates
  `.orchid/worktrees/<sani>` on a new branch off HEAD; `Remove` + `DeleteBranch`;
  `IsMain` detection; porcelain parse unit test on canned output.
- **session (real git + tmux):** `Create(project,worktree,agent)` runs in the
  worktree dir (assert via a shell `pwd` through the ring); `KillByWorktree`
  kills all matching; `Restore` drops legacy records.
- **server (httptest + real git/tmux):** projects CRUD; worktree create/list;
  **cascade remove** (create session in a worktree → DELETE worktree → session
  gone + worktree gone + branch gone); create-session 404/400 validation.
- **frontend:** CDP screenshots — projects list, project detail (worktree
  groups), create modals; manual: create project → worktree → session → terminal.
- All concurrency under `-race`; git/tmux tests skip when unavailable and clean up
  temp repos + `orchid-*` tmux sessions.

## File layout (changes)

```
internal/project/project.go + _test.go    # NEW
internal/gitwt/gitwt.go + _test.go         # NEW
internal/session/manager.go, session.go    # MODIFIED: project/worktree on Record/Info, Create sig, KillByWorktree, Restore drop
internal/server/server.go, api.go          # MODIFIED: project store, project/worktree routes, cascade, create-session change
cmd/orchid/cli.go                          # MODIFIED: gitwt preflight + project store wiring into server.New
internal/client/client.go                  # MODIFIED: Create signature -> Create(project,worktree,agent); add Projects()/Worktrees() if CLI needs them
cmd/orchid/cli.go                          # MODIFIED (create cmd): `orchid create <project> <worktree> <agent>` (the old `create <agent> [cwd]` no longer compiles against the new client.Create); `ls` unaffected
web/index.html, web/app.js                 # MODIFIED: projects list + detail screens, modals
```
