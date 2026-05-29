# Orchid Dashboard — Deferred Design Features

**Date:** 2026-05-29
**Status:** Backlog (not scheduled)

The Claude Design handoff bundle (saved at `docs/design/orchid-web/`, originally
from the design URL `api.anthropic.com/v1/design/h/7GZrcOajpLuny5sepfYLLQ`)
contains a full projects→worktrees→sessions dashboard. Milestone 5
(`2026-05-29-orchid-dashboard-redesign-design.md`) applies only the parts our flat
session backend supports. This document records everything **in the design but not
yet implemented**, each with a pointer to where it lives in the bundle, so it can
be picked up later.

Bundle key files: `docs/design/orchid-web/project/{styles.css, screens.jsx,
app.jsx, components.jsx, terminal.jsx, data.jsx, tweaks-panel.jsx}` and
`README.md` (handoff notes). The chat transcript with the original intent is at
`docs/design/orchid-web/chats/chat1.md`.

## Deferred features

### 1. Projects (drill-in navigation)
A top-level list of projects, each with a folder path; clicking drills into a
project detail screen; sessions/worktrees live under the selected project. Needs a
backend `project` concept (id, name, path) and project-scoped session listing.
- Design: `screens.jsx` `ProjectsList`; create via `app.jsx` `NewProjectModal`;
  detail header/breadcrumb in `screens.jsx` `ProjectDetail`.
- CSS: `styles.css` `.proj-list/.proj-row/.proj-meta/.mini-dots`,
  `.detail-head/.crumb/.detail-title`.

### 2. Worktrees (per-project git working copies)
Tabs/accordion groups under a project, each a git worktree on its own branch with
a "dirty" badge (`±N`). Sessions are created in the selected worktree's folder.
Needs backend git-worktree management (create/list, branch, dirty count) and
session→worktree association.
- Design: `screens.jsx` `ProjectDetail` worktree groups + tabs; create via
  `app.jsx` `NewWorktreeModal`.
- CSS: `styles.css` `.wt-tabs/.wt-tab`, `.wt-group/.wt-group-head/.wt-chev`.

### 3. Grid view (multiple live terminals)
Show up to N (`MAX_GRID = 9`) terminals at once in a responsive grid, with a chip
selector to choose which sessions are visible. Backend already supports concurrent
WebSocket attach to distinct sessions; this is mostly a frontend layout managing N
xterm instances (see the earlier multi-terminal discussion).
- Design: `screens.jsx` grid branch + `GridSelector`; `app.jsx` `defaultGrid`,
  `toggleGrid`, `MAX_GRID`.
- CSS: `styles.css` `.term-grid/.term-pane`, `.grid-sel/.gchip`.

### 4. Session cards + last-lines preview
A card layout (alternative to rows) showing each session with a preview of its
last terminal lines and a footer. Needs the list API to include a scrollback tail
per session (we have the ring server-side; expose last ~N lines in `Info`).
- Design: `styles.css` `.cards/.s-card/.preview/.foot`; cards rendering in the
  grid/list toolbar (`screens.jsx` tools `.seg`).

### 5. "Waiting for input" status
A third status (`wait`, amber pulse) for agents blocked on a user prompt
(e.g. "Proceed? (y/n)"). Needs detection from agent output (parsing the pane for a
prompt) — non-trivial.
- Design: `data.jsx` `STATUS_META.wait`, `waitLines`; `styles.css`
  `.status-pill.wait`, `.dot.wait`.

### 6. Uptime
Per-session `↑ 23m` runtime display. Needs `Info.StartedAt` (we have
`store.Record.Created` server-side — expose it in the API `Info`), then format
client-side.
- Design: `data.jsx` `fmtUptime`; shown in `screens.jsx` `SessionRow`/rail.

### 7. Tweaks panel
A gear panel to adjust accent color, density (compact/comfortable), terminal font
size, and status-pulse toggle, persisted and applied via CSS variables.
- Design: `tweaks-panel.jsx` (whole file); defaults in `app.jsx` `TWEAK_DEFAULTS`;
  CSS vars overridden at runtime (`app.jsx` tweak `useEffect`).

### 8. Toasts
Transient confirmations ("Session started/killed/restarted") bottom-center.
- Design: `components.jsx` `useToasts`/`Toasts`; CSS `styles.css` `.toasts/.toast`.

### 9. User-given session names / tasks
Sessions carry a human name and an optional task description (shown as the row
title + subtitle). We currently key sessions by generated id + agent only. Needs
the create API + record to store a name/task.
- Design: `data.jsx` `mkSession(type,name,...,task)`; create form in `app.jsx`
  `NewSessionModal` "Task / name" field.

## Suggested order (if revisited)
1. Uptime (#6) and Toasts (#8) — tiny, no/low backend.
2. Session names/tasks (#9) — small backend field.
3. Grid view (#3) — frontend, backend ready.
4. Projects (#1) → Worktrees (#2) — the big backend additions, in that order.
5. Session cards/preview (#4), Waiting status (#5), Tweaks (#7) — polish.
