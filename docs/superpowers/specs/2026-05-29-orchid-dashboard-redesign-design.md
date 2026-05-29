# Orchid — Dashboard Redesign (Milestone 5)

**Date:** 2026-05-29
**Status:** Approved design

## Context

A user mocked up a dark, terminal-styled dashboard for orchid in Claude Design and
exported a handoff bundle (saved in this repo at `docs/design/orchid-web/`). The
design assumes a **projects → worktrees → sessions** hierarchy with list/grid/focus
views, mock terminals, modals, toasts, and a tweaks panel.

Our backend is **flat sessions only** — agents `claude`/`codex`/`shell` over real
xterm.js/WebSocket, status `running`/`stopped`, no projects/worktrees and no
"waiting for input" detection. This milestone applies the design's **visual system
to what we actually have** and defers everything else to
`2026-05-29-orchid-dashboard-deferred-spec.md`.

The current frontend (`web/index.html`, `web/app.js`) is a single page: a sidebar
with a create form + session list, and one terminal pane. We restyle it to the
design and add an underscore templating layer to cut hand-built DOM.

## Goals

- Adopt the design's CSS (`docs/design/orchid-web/project/styles.css`) as
  `web/styles.css`; load IBM Plex Sans + JetBrains Mono.
- Topbar: brand (`o` mark + `orchid` + `tmux · v0.5` version) + `codex`/`claude`
  `kbd` chips.
- **List view** — design `s-row` session rows: agent badge, name, `stype` (agent
  cmd + cwd), status pill, hover `restart`/`kill` actions. Click → Focus.
- **Focus view** — design `focus` layout: left `focus-rail` (all sessions, `+` to
  create), right `focus-term` hosting the **real xterm** inside the design's
  `term-head` chrome (agent badge, name, cmd, status pill, restart/kill).
- **Create** — `NewSessionModal`: `choice-grid` agent picker (claude/codex/shell
  with badges) + optional cwd field → existing `POST /api/sessions {agent,cwd}`.
- **Status mapping:** `running` → design `working` (green `ok` dot, pulse);
  `stopped` → `idle`. No `wait`.
- **Agent badges:** claude `C`, codex `Cx`, shell `>_` (per design `AGENT_META`).
- Templating: vendor underscore; use `_.template` for the session-list rows and
  the create modal. xterm stays imperative (it owns its canvas).
- No backend changes (the API already accepts agent + cwd and returns
  `{id,agent,cwd,status,alive}`).

## Non-Goals (deferred — see deferred spec)

Projects, worktrees/git, grid view (multi-terminal), session cards + previews,
"waiting" status, uptime, tweaks panel, toasts, user-given session names/tasks.

## Architecture

The design's prototype is React + Babel; we recreate the **visual output** in
vanilla JS (no React), matching pixels, not structure (per the bundle README).

```
web/
  index.html      # topbar + #main; loads styles.css, fonts, underscore, xterm, app.js
  styles.css      # vendored from the design bundle (verbatim)
  app.js          # state + render via _.template; xterm wiring (real WS) unchanged in behavior
  vendor/
    underscore-min.js   # NEW (templating)
    xterm.js, xterm.css, addon-fit.js   # existing
```

`app.js` structure:
- **State:** `view` (`"list"` | `"focus"`), `activeId`, `sessions` (from
  `GET /api/sessions`), plus the existing xterm/WS singletons for the focused
  terminal.
- **Templates** (`_.template` strings in `app.js`): `rowTpl` (one `s-row`),
  `railItemTpl` (one `rail-item`), `modalTpl` (create modal body). Rendered into
  containers; event handlers bound by delegation after render.
- **Views:** `renderList()` builds `.sess-list` from `rowTpl`; `renderFocus()`
  builds the `.focus` grid (rail from `railItemTpl` + `focus-term` div), then
  opens/attaches the real xterm into `focus-term` exactly as today (WS to
  `/ws/{id}`, `term.onData`→send, resize→`refresh-client`).
- **Status/agent/title helpers** mirror the design's `STATUS_META`/`AGENT_META`
  (running→working, stopped→idle; badges C/Cx/>_).

Data flow unchanged: REST for list/create/kill/restart, WS for the focused
terminal. Selecting a session = build Focus + attach xterm (one active WS, as
today). Switching tears down the old terminal/WS and builds the new.

## Components (design references)

- Topbar: `styles.css` `.topbar/.brand/.kbd`.
- Session row: `styles.css` `.s-row`; markup mirrors `screens.jsx SessionRow`
  (badge / meta / right: status pill + acts).
- Status dot/pill: `.dot`, `.status-pill`; mapping per `data.jsx STATUS_META`.
- Agent badge: `.agent-badge.{claude,codex,shell}`; glyphs per `AGENT_META`.
- Focus layout: `.focus/.focus-rail/.rail-item/.rail-head/.focus-term`;
  `screens.jsx ProjectDetail` focus branch + `terminal.jsx Terminal` head chrome.
- Create modal: `.scrim/.modal/.field/.choice-grid/.choice`; `app.jsx
  NewSessionModal` + `components.jsx Modal`.
- Segmented view switch (List/Terminal): `.seg`; `screens.jsx` tools toolbar
  (we use List + Focus only; no Grid).

## Mapping notes / substitutions

- The design terminal (`terminal.jsx`) renders **mock** line arrays; we replace
  the `.term-body` content with our **real xterm.js** instance. We keep the
  design's `.term-head` chrome around it and the `.term` container styling. The
  design's `.term-prompt` mock input is NOT used (xterm handles input).
- Session display name: we have no user-given name — use the agent label as the
  primary name and `agent cmd · cwd` as the `stype` subtitle; show the short id in
  the rail/secondary text. (User-given names deferred.)
- Version chip text: `tmux · v0.5`.

## Errors & edge cases

- Empty session list → design `.empty` state ("No sessions yet" + Create button).
- Stopped session selected → show a placeholder in `focus-term` ("stopped —
  restart to attach"), restart via the head action (existing behavior, restyled).
- Create with unknown agent → server 400 surfaced inline in the modal.
- Underscore template injection: session ids/cwd are rendered via `_.escape`
  (underscore escapes `<%- %>` interpolations) to avoid HTML injection from cwd.

## Testing

- **Backend:** unchanged — existing Go `-race` suite must stay green (no API or
  handler changes).
- **Frontend:** no JS test harness. Verify with a headless-Chrome (CDP)
  screenshot: build, `start`, create a shell session, screenshot List and Focus,
  confirm the dark terminal look matches the design (surfaces `#0c0c0f`, accent
  `#7c9cff`, IBM Plex/JetBrains fonts, `s-row`/`focus` layout). Manual: create,
  select, type into the real terminal, restart, kill — all still work over WS.

## File layout (changes)

```
web/index.html        # MODIFIED: topbar, fonts, styles.css + underscore includes
web/styles.css        # NEW: vendored design CSS
web/app.js            # MODIFIED: underscore templates + list/focus render; xterm wiring kept
web/vendor/underscore-min.js  # NEW
docs/design/orchid-web/        # design bundle reference (already saved)
```
