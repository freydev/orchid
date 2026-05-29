# Orchid — View Switcher + Multi-Terminal Grid (Milestone 7)

**Date:** 2026-05-30
**Status:** Approved design

## Context

The project-detail screen (milestone 6) shows worktree groups of sessions (list)
and a focus terminal reached by clicking a session. The Claude Design has a
segmented view switcher `[List | Grid | Terminal]` in the detail toolbar and keeps
the project header above every view. This milestone adds that switcher, the
**multi-terminal Grid** view (deferred-spec #3), and restores the project header
on the Terminal view. Frontend-only — the backend already supports concurrent
WebSocket attach to distinct sessions.

Implements deferred-spec #3 (grid view). Builds on the M6 frontend
(`web/app.js`).

## Goals

- A `.seg` switcher `[List | Grid | Terminal]` in the project-detail toolbar;
  `cur.view ∈ {list, grid, terminal}`; active button highlighted.
- The project header (breadcrumb · folder · title · path) + the seg toolbar render
  in **all three** views (the "terminal-tab header").
- **Grid view**: a `GridSelector` (chips for the project's sessions, toggle which
  appear, cap **4** = `MAX_GRID`, `N/4` counter, over-cap chips disabled) above a
  responsive `term-grid` of `.term-pane`s, each hosting a **live xterm** (real WS).
  Default selection = first 4 running sessions. Double-click a pane → Terminal
  view focused on it.
- **Terminal view**: the existing focus layout (session rail + one live xterm in
  `term-head` chrome), now under the persistent header/seg.
- Multi-terminal lifecycle: a `panes` map replaces the single terminal globals;
  one `attachInto(host, id)` builds an xterm+WS, reused by grid panes and focus;
  `teardownAll()` disposes every pane on view switch / re-render.

## Non-Goals

- No backend changes (concurrent WS already supported; create/kill/restart/list
  unchanged).
- No grid drag-reorder, no per-pane resize splitter (CSS auto-grid only).
- Cap is fixed at 4 (not user-configurable this milestone).

## Architecture (web/app.js)

State adds `cur.view` (`list|grid|terminal`) and `cur.grid` (array of selected
session ids, ≤ `MAX_GRID=4`). The single `term/ws/fit/ro` globals are replaced by:

```
panes = Map(id -> { term, fit, ws, ro })   // live terminals (grid panes OR the focus one)
attachInto(hostEl, id)                       // build xterm + WS for session id into hostEl; register in panes
teardownPane(id)                             // dispose one pane (RO disconnect, ws.onclose=null+close, term.dispose)
teardownAll()                                // teardownPane for every entry
```

`renderDetail()` builds the persistent shell once:
```
detail-head (crumb + title + path)
tools: <seg List|Grid|Terminal>  + New worktree
#body   ← renderBody() fills per cur.view
```
- `renderBody()` switches on `cur.view`:
  - `list`  → worktree groups (current M6 markup).
  - `grid`  → `GridSelector` chips + `term-grid`; for each selected id, a
    `.term-pane` with `term-head` + a host div → `attachInto`.
  - `terminal` → `focus` (rail + `focus-term`); `attachInto(focusTerm, activeId)`.
- Every `renderBody()`/view switch calls `teardownAll()` first.

Data flow unchanged: `GET /api/sessions` (filtered to project) + `GET …/worktrees`;
each live pane opens its own `/ws/{id}` and sends resize via `refresh-client -C`.

## Components (design references)

- Seg switcher: `styles.css .seg`; icons `list`/`grid`/`term` from `components.jsx`.
- Grid selector: `.grid-sel/.gchip` (`screens.jsx GridSelector`); cap label `N/4`.
- Grid: `.term-grid/.term-pane` (`screens.jsx` grid branch); `cols-1` for a single
  pane; `MAX_GRID = 4`.
- Term pane chrome: `.term-head` (agent badge, name, cmd, status pill, an
  expand→Terminal button); the live xterm fills the bare body (`overflow:hidden`,
  Monokai) as in the current focus terminal.
- Terminal view: `.focus/.focus-rail/.focus-term` (unchanged), now beneath the
  persistent header/seg.

## Grid behavior

- Selector lists every session in the project (`status dot · agent · worktree`).
  Toggling adds/removes from `cur.grid`; at 4 selected, unselected chips are
  `.disabled`. Default `cur.grid` = first ≤4 running sessions when entering grid
  with an empty selection.
- 0 selected → empty state ("Select sessions for the grid · up to 4").
- Stopped session selected → pane shows the "stopped — restart to attach"
  placeholder, no WS opened.
- Re-rendering the grid (e.g. after refresh) tears down and rebuilds panes for the
  current selection (simple + correct; acceptable at cap 4).

## Errors & edge cases

- View switch always `teardownAll()` first → no leaked WS/ResizeObservers.
- A pane whose session was killed elsewhere: next `refresh()` drops it from
  sessions; the grid rebuild omits it (and prunes it from `cur.grid`).
- Restart/kill from a pane/term-head: same delegated handlers as today; after the
  action, `refresh()` + rebuild current view.
- Resize: each pane has its own ResizeObserver → its own `refresh-client -C` (tmux
  sizes per attached control client).

## Testing

- Frontend, no JS harness: `node --check web/app.js`; CDP screenshots of all three
  views — List (worktree groups), Grid (≥2 live terminals + selector chips),
  Terminal (header + rail + terminal). Manual: switch List↔Grid↔Terminal, toggle
  chips (cap 4 enforced), type into multiple grid terminals simultaneously, verify
  no console errors and the open-WS count equals the selected-pane count; leaving
  grid closes those WS (teardownAll).
- Backend unchanged → existing Go `-race` suite must stay green.

## File layout (changes)

```
web/app.js   # MODIFIED: cur.view/cur.grid, panes map + attachInto/teardownAll,
             #           renderDetail shell + renderBody (list|grid|terminal),
             #           GridSelector, term-grid panes, seg switcher, persistent header
```
Only `web/app.js` changes (design CSS already vendored; index.html unchanged).
