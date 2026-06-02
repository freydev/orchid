# Orchid KB Store — Design

**Date:** 2026-05-30
**Status:** Approved (design)
**Package:** `internal/kb`

## Purpose

A simple embedded knowledge + task store for agents. Two concerns in one
store:

1. **Tasks** — title, description, metadata; create / update / close; with
   many-to-many blocking relationships between tasks.
2. **Knowledge (KV)** — arbitrary `key → string` records, independent of tasks
   (notes, facts agents persist).

Consumed as a Go package plus an `orchid task` / `orchid kv` CLI namespace.

## Backend

`go.etcd.io/bbolt` — pure-Go, no CGO, ACID, B+tree (lmdb-like semantics).
lmdb itself was rejected: its Go bindings require CGO and a system `liblmdb`,
which would break the project's current pure-Go cross-compilation.

- Single DB file at `~/.config/orchid/kb.db` (honoring `$XDG_CONFIG_HOME`,
  following the pattern in `internal/store.DefaultPath`).
- Task IDs are UUIDs via `github.com/google/uuid` (already a dependency).

## Data Model

```go
type Task struct {
    ID      string            `json:"id"`
    Title   string            `json:"title"`
    Desc    string            `json:"desc"`
    Meta    map[string]string `json:"meta"`
    Status  string            `json:"status"` // "open" | "closed"
    Created time.Time         `json:"created"`
    Updated time.Time         `json:"updated"`
}
```

`blocked` is **not stored**. It is derived: a task `T` is blocked iff there
exists an edge `B → T` where `B.Status == "open"`.

### Buckets

| Bucket   | Key                          | Value          |
|----------|------------------------------|----------------|
| `tasks`  | `id`                         | `Task` as JSON |
| `blocks` | `blockerID \x00 blockedID`   | empty          |
| `kv`     | `key`                        | string bytes   |

`blocks` edges are stored once per direction. Many-to-many is expressed by
multiple edges; lookups scan the bucket (`Blockers`/`Blocks` filter by the
relevant half of the composite key).

## Package API

```go
type Store struct { /* wraps *bbolt.DB */ }

func Open(path string) (*Store, error)
func (s *Store) Close() error

// Tasks
func (s *Store) CreateTask(title, desc string, meta map[string]string) (Task, error)
func (s *Store) UpdateTask(id, title, desc string, meta map[string]string) (Task, error) // bumps Updated
func (s *Store) CloseTask(id string) error          // ErrBlocked if any open blocker exists
func (s *Store) GetTask(id string) (Task, error)    // ErrNotFound
func (s *Store) ListTasks(status string) ([]Task, error) // "" = all
func (s *Store) ReadyTasks() ([]Task, error)        // open AND no open blockers

// Blocking (many-to-many)
func (s *Store) Block(blockerID, blockedID string) error   // ErrCycle on transitive cycle
func (s *Store) Unblock(blockerID, blockedID string) error
func (s *Store) Blockers(id string) ([]string, error)      // who blocks id (open+closed)
func (s *Store) Blocks(id string) ([]string, error)        // whom id blocks
func (s *Store) IsBlocked(id string) (bool, error)         // derived

// Knowledge
func (s *Store) KVSet(key, val string) error
func (s *Store) KVGet(key string) (string, error)          // ErrNotFound
func (s *Store) KVDelete(key string) error
func (s *Store) KVList(prefix string) ([]string, error)    // "" = all keys
```

### Sentinel errors

```go
var (
    ErrNotFound = errors.New("kb: not found")
    ErrBlocked  = errors.New("kb: task has open blockers")
    ErrCycle    = errors.New("kb: blocking cycle")
)
```

## Semantics

- **Status model:** explicit field is `open`/`closed` only. "Blocked" is always
  derived from edges + blocker status, never stored.
- **Close guard:** `CloseTask` fails with `ErrBlocked` while any blocker of the
  task is still `open`. Closing a blocker auto-unblocks its targets (because
  blocked-ness is derived).
- **Ready:** `ReadyTasks` returns tasks that are `open` and have no `open`
  blocker — the actionable set (mirrors `bd ready`).
- **Cycle guard:** `Block(b, t)` is rejected with `ErrCycle` if `t` already
  (transitively) blocks `b`. Without this, a cycle of open tasks is an
  unbreakable deadlock under the close guard. Detection is a DFS over `blocks`
  edges within the write transaction.
- **Unblock:** removes an edge directly — for edges added by mistake or
  dependencies that no longer hold while the blocker stays open.
- **Block validation:** both task IDs must exist (`ErrNotFound` otherwise);
  self-edge (`b == t`) rejected as `ErrCycle`.

## CLI

New `task` and `kv` namespaces routed from `cmd/orchid/cli.go` (the flat
`create` / `status` / `rm` names are already taken by session commands).

```
orchid task create --title T [--desc D] [--meta k=v,k2=v2]
orchid task update <id> [--title T] [--desc D] [--meta k=v,...]
orchid task close   <id>
orchid task show    <id>           # fields + blockers + blocks + blocked?
orchid task ls      [--status open|closed]
orchid task ready
orchid task block   <blocker> <blocked>
orchid task unblock <blocker> <blocked>

orchid kv set <key> <value>
orchid kv get <key>
orchid kv del <key>
orchid kv ls  [prefix]
```

- `--meta` parses comma-separated `k=v` pairs.
- `task show` prints fields plus derived blocked state and both edge lists.
- Output uses `text/tabwriter`, matching existing `lsCmd`/`statusCmd`.
- `route()` gains `case "task"` and `case "kv"` delegating to sub-routers.

## Testing

- `internal/kb/kb_test.go` — table-driven, temp-dir DB (pattern of existing
  `*_test.go`):
  - task CRUD + `Updated` bump
  - close guard (`ErrBlocked`) and auto-unblock on blocker close
  - many-to-many: one blocker → many blocked, many blockers → one blocked
  - `Blockers` / `Blocks` / `IsBlocked` (derived)
  - `ReadyTasks` excludes blocked + closed
  - cycle guard (`ErrCycle`, incl. self-edge and transitive)
  - `Unblock` removes only the named edge
  - KV set/get/delete/list + `ErrNotFound`
- `cmd/orchid/cli_test.go` — argument parsing for the new subcommands
  (`--meta` parsing, routing).

## Files

| File                       | Change                               |
|----------------------------|--------------------------------------|
| `internal/kb/kb.go`        | `Store`, `Open`/`Close`, buckets, errors |
| `internal/kb/task.go`      | task ops + blocking + ready (optional split) |
| `internal/kb/kv.go`        | KV ops (optional split)              |
| `internal/kb/kb_test.go`   | tests                                |
| `cmd/orchid/cli.go`        | `task`/`kv` routing + handlers       |
| `cmd/orchid/cli_test.go`   | CLI parsing tests                    |
| `go.mod` / `go.sum`        | add `go.etcd.io/bbolt`               |

## Out of Scope (YAGNI)

- HTTP API / server endpoints (CLI + package only this iteration).
- Linking KV records to tasks (KV is standalone).
- Task priorities, assignees, due dates.
- Concurrency beyond bbolt's single-writer transaction model.
