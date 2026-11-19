# Orchid Graph View — Design

**Status:** Design approved (brainstorming). Next: implementation plan.

**Goal:** A new global "Graph" view that renders all orchid entities as an
interactive node graph — `project → worktree → session` — using
`@xyflow/svelte`. Every node carries the same actions as its counterpart in the
normal List view (reusing existing modals/handlers). Clicking a session node
opens that session's focus terminal.

**Why:** The List view is per-project drill-in. A graph gives a single global
picture of all projects, their worktrees, and live sessions at a glance, with
the same controls inline.

---

## Decisions (locked)

| Area | Decision |
|------|----------|
| Library | **`@xyflow/svelte`** (Svelte 5 flow), layout via **`@dagrejs/dagre`** |
| v1 hierarchy | **project → worktree → session** (no "task" layer — deferred) |
| Session click | **Navigate** to that session's focus Terminal (like a Board card click); not inline xterm |
| Layout | Auto LR tree (dagre). Collapsible: projects collapsed by default → expand to worktrees → expand worktree to sessions. Nodes draggable; positions **not** persisted in v1; a **Relayout** button re-runs dagre |
| Entry | Topbar **List ⇄ Graph** segment toggling a global view; hash `#/graph` |
| Node actions | Mirror the List view, reusing existing `openModal` / `confirm` / `api` |
| Backend | **Unchanged** — graph is built from existing `/api/projects`, `/api/sessions`, `/api/projects/{id}/worktrees` |
| Tests | No unit tests (matches project). Acceptance = manual parity checklist |

---

## State & entry

- New global flag `app.graph: boolean` in `lib/state.svelte.ts`.
- Topbar (`App.svelte`): a `List ⇄ Graph` segment (two icon buttons). Toggling
  sets `app.graph` and updates the hash.
- `router.ts`:
  - `setHash()` writes `#/graph` when `app.graph` is true (independent of
    project drill-in; graph is global).
  - `applyHash()` recognizes `#/graph` → sets `app.graph = true`.
- `App.svelte` render: `{#if app.graph}<GraphPage/>{:else if app.cur.projectId}<ProjectDetail/>{:else}<ProjectsPage/>{/if}`.
- Leaving graph (toggle to List) clears `app.graph` and restores the prior hash.

## Data flow (lazy, matches collapse-on-demand)

- On entering Graph: ensure `app.projects` loaded (`api.projects()`) and load
  **all** sessions once via `api.sessions()` (unfiltered) into a graph-local
  list (do not disturb `app.sessions`, which the List view scopes per project).
- Expand state held in `GraphPage`:
  - `expandedProjects: Set<string>` (project ids),
  - `expandedWorktrees: Set<string>` (worktree paths).
- Worktrees are fetched **on first project expand** via `api.worktrees(pid)` and
  cached in a `Map<projectId, Worktree[]>`. Collapsed projects fetch nothing.
- Session nodes under a worktree = all-sessions filtered by `session.worktree === wt.path`.
- Polling: while `app.graph`, a graph poll refreshes `api.projects()` + the
  all-sessions list (so status pills/counts stay live). Reuse/extend the
  existing 3 s interval; in graph mode it must load all sessions, not the
  project-scoped set.

## Components

- **`components/graph/GraphPage.svelte`** — owns `<SvelteFlow>`, expand state,
  the all-sessions list + worktree cache, builds `nodes`/`edges` via `lib/graph.ts`,
  runs dagre on change, renders a **Relayout** button and a legend. Registers the
  three custom node types.
- **`lib/graph.ts`** — pure helpers:
  - `buildGraph(projects, allSessions, worktreesByProject, expandedProjects, expandedWorktrees)` → `{ nodes, edges }` (only visible nodes per expand state).
  - `layout(nodes, edges)` → positions via dagre (`rankdir: 'LR'`), returns nodes with `position`.
- **`components/graph/ProjectNode.svelte`** — project title (→ drill-in: set
  `app.graph=false`, open its detail), status summary (run/wait/idle from
  `session_list`), gear → `openModal('projectSettings',{project})`, **+ Worktree**
  → `openModal('newWorktree')` (after selecting the project context), delete →
  `confirm` + `api.removeProject`. Expand/collapse caret (toggles
  `expandedProjects`). Source handle to worktrees.
- **`components/graph/WorktreeNode.svelte`** — branch/name label, **Diff** →
  `openModal('diff',{wt})`, **+ Session** → `openModal('newSession',{wt: wt.path})`,
  **Remove** (confirm + `api.removeWorktree`, hidden for `is_main`), removing
  spinner via `app.removingWt`. Expand/collapse caret (toggles
  `expandedWorktrees`). Target handle from project, source handle to sessions.
- **`components/graph/SessionNode.svelte`** — agent `Badge` (+color),
  name, `StatusPill`, gear → `openModal('sessionSettings',{session})`,
  stop/restart (`api.stop`/`api.restart`), kill (`confirm` + `api.kill`); the
  node body click → set `app.graph=false`, focus that project + terminal +
  `activeId=session.id`, `setHash()`. Target handle from worktree.

All actions reuse the existing modal/confirm/api code paths — no duplicated
business logic. The node components are thin wrappers around the same calls the
List components make.

## Layout

- `@dagrejs/dagre`: a directed graph, `rankdir: 'LR'`, node width/height per
  type (fixed sizes), `nodesep`/`ranksep` tuned for readability.
- Recompute on: expand/collapse, data refresh that changes the visible set, and
  the **Relayout** button.
- Nodes are draggable (SvelteFlow default); drag positions are transient and
  reset on the next dagre run. Persistence is out of scope for v1.
- Pan/zoom, minimap, and background grid from SvelteFlow.

## Cross-context actions

- Drill-in (project title / session click) leaves the graph: set
  `app.graph = false`, set `app.cur` appropriately, `setHash()`.
- After create/remove/kill the graph rebuilds from refreshed data (re-fetch
  projects + all sessions; invalidate the affected project's worktree cache).

## Edge cases

- A session whose `worktree` is not in its project's worktree list (orphan):
  show it under the project node in an "other" group; never drop it silently.
- Empty project (no worktrees) / empty worktree (no sessions): expand shows a
  faint "none" leaf, consistent with the List view.
- Large graphs: collapsed-by-default projects keep the initial render small;
  expansion is on demand.

## Dependencies

- Add `@xyflow/svelte` and `@dagrejs/dagre` to `web/package.json`; they bundle
  via the existing rollup `node-resolve`/`commonjs` setup. Import SvelteFlow's
  CSS (`@xyflow/svelte/dist/style.css`) once (in `GraphPage` or `main.ts`).
- Rebuild `web/dist` (committed) so `go build` stays node-free.

## Acceptance — manual parity checklist

- Topbar List⇄Graph toggles; `#/graph` survives refresh.
- Graph shows all projects collapsed; expand → worktrees; expand worktree → sessions.
- Status pills/counts update live (poll).
- Project node: drill-in, settings, +worktree, delete all work via existing flows.
- Worktree node: diff, +session, remove (confirm, hidden for main), removing spinner.
- Session node: settings, stop/restart, kill (confirm), color/badge/status; click → focus terminal of the right project/session.
- Relayout re-runs dagre; drag works; pan/zoom/minimap work.
- Create/remove/kill reflect in the graph after refresh.

## Out of scope (v1)

Task layer (project→task→worktree), inline live terminals in nodes, persisted
node positions, backend changes, multi-select/bulk ops.
