# Orchid Graph View Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a global "Graph" view rendering `project → worktree → session` as an interactive `@xyflow/svelte` node graph, where each node mirrors the List view's actions and a session click opens its focus terminal.

**Architecture:** A topbar `List ⇄ Graph` segment toggles a global `app.graph` flag (hash `#/graph`). `GraphPage.svelte` hosts `<SvelteFlow>`, owns expand state + lazily-loaded data (all sessions once; worktrees per project on expand), and builds visible nodes/edges via pure helpers in `lib/graph.ts` (dagre LR layout). Three thin custom node components reuse existing `openModal`/`confirm`/`api` calls — no duplicated business logic. Backend is untouched.

**Tech Stack:** Svelte 5 (runes), TypeScript, `@xyflow/svelte` (v1, Svelte 5), `@dagrejs/dagre`, Rollup, existing Go backend.

**Project conventions (read before starting):**
- No frontend unit tests (YAGNI per spec). Each task's "verification" = `cd web && command npm run build` prints `created dist/bundle.js` with no new errors, plus the stated manual check. (`command npm` bypasses the aikido shim.)
- Svelte 5 runes: `$state`, `$derived`, `$props`, `$effect`. Actions via `use:`.
- Reuse existing modules: `lib/state.svelte` (`app`, `refresh`, `loadProject`, `loadProjects`), `lib/modals.svelte` (`openModal`, `confirm`), `lib/api` (`api`), `lib/router` (`setHash`), `lib/util` (`agentMeta`, `statusMeta`, `STATUS_RANK`, `escHtml`), `lib/theme` (`colorHex`).
- Existing UI components to reuse inside nodes: `components/ui/Badge.svelte`, `components/ui/StatusPill.svelte`, `components/ui/BranchLabel.svelte`, `components/ui/Icon.svelte`.
- Do NOT run `orchid serve` against an isolated HOME (it prunes shared tmux). Build only.
- After the final task, rebuild `dist/` (committed) so `go build` stays node-free.

---

## File structure

| File | Responsibility |
|------|----------------|
| `web/package.json` | add `@xyflow/svelte`, `@dagrejs/dagre` deps |
| `web/src/lib/state.svelte.ts` | `app.graph: boolean` global view flag |
| `web/src/lib/router.ts` | `#/graph` hash read/write |
| `web/src/App.svelte` | topbar List⇄Graph segment; render `<GraphPage>` |
| `web/src/lib/poll.ts` | graph-mode poll (refresh projects + all sessions) |
| `web/src/lib/graph.ts` | pure `buildGraph()` + dagre `layout()` |
| `web/src/components/graph/GraphPage.svelte` | `<SvelteFlow>` host: expand state, data load, node/edge build, relayout |
| `web/src/components/graph/ProjectNode.svelte` | project node + actions + expand caret |
| `web/src/components/graph/WorktreeNode.svelte` | worktree node + actions + expand caret |
| `web/src/components/graph/SessionNode.svelte` | session node + actions + click→terminal |

---

### Task 1: Dependencies, view flag, topbar toggle, routing, empty GraphPage

**Files:**
- Modify: `web/package.json`
- Modify: `web/src/lib/state.svelte.ts`
- Modify: `web/src/lib/router.ts`
- Modify: `web/src/App.svelte`
- Create: `web/src/components/graph/GraphPage.svelte` (placeholder)
- Modify: `web/src/components/ui/Icon.svelte` (add a `graph` icon if missing)

- [ ] **Step 1: Install deps**

Run: `cd /Users/andrey/Hobbies/orchid-svelte/web && command npm install @xyflow/svelte @dagrejs/dagre`
Expected: both added to `package.json` `dependencies`; `node_modules` updated (gitignored).

- [ ] **Step 2: Add the global flag to state**

In `web/src/lib/state.svelte.ts`, inside the `app = $state({ ... })` object add `graph: false` (after `shellName: 'shell',`):

```ts
  shellName: 'shell',
  graph: false, // global Graph view toggle
```

- [ ] **Step 3: Routing for `#/graph`**

In `web/src/lib/router.ts`:

In `setHash()`, at the very top of the function body, handle graph first:

```ts
export function setHash() {
  let h = "#/";
  if (app.graph) {
    h = "#/graph";
  } else if (app.cur.projectId) {
    h = "#/p/" + encodeURIComponent(app.cur.projectId) + "/" + app.cur.view;
    if (app.cur.view === "terminal" && app.cur.activeId) h += "/" + encodeURIComponent(app.cur.activeId);
  }
  if (location.hash !== h) { suppressHash = true; location.hash = h; }
}
```

In `applyHash()`, handle the graph route before the `p` parsing. Replace the start of `applyHash`:

```ts
export async function applyHash() {
  const raw = location.hash.replace(/^#\/?/, "");
  if (raw === "graph") {
    app.graph = true;
    if (!app.projects.length) await loadProjects();
    return;
  }
  app.graph = false;
  const parts = raw ? raw.split("/").map(decodeURIComponent) : [];
  // ... existing body unchanged ...
```

(Keep the rest of `applyHash` exactly as-is after these lines.)

- [ ] **Step 4: Add a `graph` icon**

In `web/src/components/ui/Icon.svelte`, add to the icon map (near `grid`):

```js
    graph:   '<circle cx="5" cy="6" r="2.2"/><circle cx="19" cy="5" r="2.2"/><circle cx="18" cy="18" r="2.2"/><path d="M7.1 6.6 16.9 5.2M7 7.2l9.4 9.6M6.6 8l10.6 9"/>',
```

- [ ] **Step 5: Topbar List⇄Graph segment + render GraphPage**

In `web/src/App.svelte`:

Add the import near the other component imports:

```svelte
  import GraphPage from './components/graph/GraphPage.svelte';
  import { setHash } from './lib/router';
```

Replace the topbar spacer/gear region so the segment sits before the gear:

```svelte
    <div class="spacer"></div>
    <div class="view-seg">
      <button class:sel={!app.graph} title="List view" onclick={() => { app.graph = false; setHash(); }}><Icon name="list" size={15} /></button>
      <button class:sel={app.graph} title="Graph view" onclick={() => { app.graph = true; setHash(); }}><Icon name="graph" size={15} /></button>
    </div>
    <button class="topbar-gear" title="Settings" onclick={() => openModal('globalSettings')}><Icon name="gear" size={18} /></button>
```

Replace the `.main` content branch:

```svelte
  <div class="main">
    {#if app.graph}<GraphPage />{:else if app.cur.projectId}<ProjectDetail />{:else}<ProjectsPage />{/if}
  </div>
```

Add styles inside App.svelte `<style>`:

```css
  .view-seg { display: flex; background: var(--surface-2); border: 1px solid var(--border-2); border-radius: var(--radius-sm); padding: 2px; gap: 2px; }
  .view-seg button { display: inline-flex; align-items: center; justify-content: center; width: 30px; height: 26px; border-radius: var(--radius-sm); color: var(--text-dim); }
  .view-seg button.sel { background: var(--accent-bg); color: var(--accent); }
  .view-seg button:hover:not(.sel) { color: var(--text); }
```

- [ ] **Step 6: Placeholder GraphPage**

Create `web/src/components/graph/GraphPage.svelte`:

```svelte
<script lang="ts">
</script>

<div class="graph-page">graph coming up…</div>

<style>
  .graph-page { height: 100%; display: grid; place-items: center; color: var(--text-faint); font-family: var(--mono); }
</style>
```

- [ ] **Step 7: Build + manual check**

Run: `cd /Users/andrey/Hobbies/orchid-svelte/web && command npm run build`
Expected: `created dist/bundle.js`, no errors.
Manual: toggling the topbar Graph button shows the placeholder; reloading at `#/graph` stays on it; List button returns to projects.

---

### Task 2: Pure graph builder + dagre layout (`lib/graph.ts`)

**Files:**
- Create: `web/src/lib/graph.ts`

This module is pure (no Svelte). It turns app data + expand state into visible
nodes/edges, then positions them with dagre.

- [ ] **Step 1: Implement `lib/graph.ts`**

Create `web/src/lib/graph.ts`:

```ts
import Dagre from '@dagrejs/dagre';
import type { Project, Worktree, Session } from './types';

export type GNodeKind = 'project' | 'worktree' | 'session' | 'leaf';

export interface GraphInput {
  projects: Project[];
  sessions: Session[];                       // ALL sessions (unfiltered)
  worktrees: Map<string, Worktree[]>;        // projectId -> worktrees (loaded on expand)
  expandedProjects: Set<string>;
  expandedWorktrees: Set<string>;            // worktree paths
}

export interface FlowNode {
  id: string;
  type: GNodeKind;
  data: any;
  position: { x: number; y: number };
  width?: number;
  height?: number;
}
export interface FlowEdge { id: string; source: string; target: string; }

// fixed node sizes used for dagre + rendering
const SIZE: Record<GNodeKind, { w: number; h: number }> = {
  project:  { w: 240, h: 76 },
  worktree: { w: 230, h: 64 },
  session:  { w: 250, h: 74 },
  leaf:     { w: 180, h: 32 },
};

// buildGraph returns only the nodes/edges visible for the current expand state.
export function buildGraph(input: GraphInput): { nodes: FlowNode[]; edges: FlowEdge[] } {
  const { projects, sessions, worktrees, expandedProjects, expandedWorktrees } = input;
  const nodes: FlowNode[] = [];
  const edges: FlowEdge[] = [];
  const pos = { x: 0, y: 0 };

  for (const p of projects) {
    const pid = 'p:' + p.id;
    nodes.push({ id: pid, type: 'project', data: { project: p }, position: pos, ...SIZE.project });
    if (!expandedProjects.has(p.id)) continue;

    const wts = worktrees.get(p.id) || [];
    if (!wts.length) {
      const lid = pid + ':empty';
      nodes.push({ id: lid, type: 'leaf', data: { text: 'no worktrees' }, position: pos, ...SIZE.leaf });
      edges.push({ id: pid + '->' + lid, source: pid, target: lid });
      continue;
    }
    for (const wt of wts) {
      const wid = 'w:' + p.id + ':' + wt.path;
      nodes.push({ id: wid, type: 'worktree', data: { project: p, wt }, position: pos, ...SIZE.worktree });
      edges.push({ id: pid + '->' + wid, source: pid, target: wid });
      if (!expandedWorktrees.has(wt.path)) continue;

      const rows = sessions.filter((s) => s.worktree === wt.path && s.project === p.id);
      if (!rows.length) {
        const lid = wid + ':empty';
        nodes.push({ id: lid, type: 'leaf', data: { text: 'no sessions' }, position: pos, ...SIZE.leaf });
        edges.push({ id: wid + '->' + lid, source: wid, target: lid });
        continue;
      }
      for (const s of rows) {
        const sid = 's:' + s.id;
        nodes.push({ id: sid, type: 'session', data: { project: p, session: s }, position: pos, ...SIZE.session });
        edges.push({ id: wid + '->' + sid, source: wid, target: sid });
      }
    }

    // orphan sessions: in this project but whose worktree isn't in the list
    const known = new Set(wts.map((w) => w.path));
    const orphans = sessions.filter((s) => s.project === p.id && !known.has(s.worktree));
    for (const s of orphans) {
      const sid = 's:' + s.id;
      nodes.push({ id: sid, type: 'session', data: { project: p, session: s, orphan: true }, position: pos, ...SIZE.session });
      edges.push({ id: pid + '->' + sid, source: pid, target: sid });
    }
  }
  return { nodes, edges };
}

// layout assigns LR positions via dagre, returns a new nodes array.
export function layout(nodes: FlowNode[], edges: FlowEdge[]): FlowNode[] {
  const g = new Dagre.graphlib.Graph();
  g.setGraph({ rankdir: 'LR', nodesep: 18, ranksep: 70, marginx: 20, marginy: 20 });
  g.setDefaultEdgeLabel(() => ({}));
  for (const n of nodes) g.setNode(n.id, { width: n.width, height: n.height });
  for (const e of edges) g.setEdge(e.source, e.target);
  Dagre.layout(g);
  return nodes.map((n) => {
    const d = g.node(n.id);
    return { ...n, position: { x: d.x - (n.width || 0) / 2, y: d.y - (n.height || 0) / 2 } };
  });
}
```

- [ ] **Step 2: Build**

Run: `cd /Users/andrey/Hobbies/orchid-svelte/web && command npm run build`
Expected: `created dist/bundle.js`, no errors (module compiles; unused until Task 3).

---

### Task 3: GraphPage with SvelteFlow + project nodes (collapsed) + data load

**Files:**
- Modify: `web/src/components/graph/GraphPage.svelte`
- Create: `web/src/components/graph/ProjectNode.svelte` (minimal, expanded in Task 4)

- [ ] **Step 1: Minimal ProjectNode**

Create `web/src/components/graph/ProjectNode.svelte`:

```svelte
<script lang="ts">
  import { Handle, Position } from '@xyflow/svelte';
  let { data }: { data: any } = $props();
</script>

<div class="gnode project">
  <div class="title">{data.project.name}</div>
  <Handle type="source" position={Position.Right} />
</div>

<style>
  .gnode { background: var(--surface); border: 1px solid var(--border-2); border-radius: var(--radius-sm); padding: 10px 12px; box-sizing: border-box; width: 240px; height: 76px; }
  .title { font-weight: 600; font-size: 13.5px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
</style>
```

- [ ] **Step 2: GraphPage hosting SvelteFlow**

Replace `web/src/components/graph/GraphPage.svelte`:

```svelte
<script lang="ts">
  import { SvelteFlow, Background, Controls, MiniMap } from '@xyflow/svelte';
  import '@xyflow/svelte/dist/style.css';
  import { app } from '../../lib/state.svelte';
  import { api } from '../../lib/api';
  import type { Session, Worktree } from '../../lib/types';
  import { buildGraph, layout } from '../../lib/graph';
  import ProjectNode from './ProjectNode.svelte';

  const nodeTypes = { project: ProjectNode } as any;

  let allSessions = $state<Session[]>([]);
  let worktrees = $state(new Map<string, Worktree[]>());
  let expandedProjects = $state(new Set<string>());
  let expandedWorktrees = $state(new Set<string>());

  let nodes = $state<any[]>([]);
  let edges = $state<any[]>([]);

  function rebuild() {
    const g = buildGraph({
      projects: app.projects, sessions: allSessions, worktrees,
      expandedProjects, expandedWorktrees,
    });
    nodes = layout(g.nodes, g.edges);
    edges = g.edges;
  }

  async function loadAll() {
    try { if (!app.projects.length) app.projects = await api.projects(); } catch {}
    try { allSessions = await api.sessions(); } catch {}
    rebuild();
  }

  $effect(() => { loadAll(); });
</script>

<div class="graph-page">
  <SvelteFlow bind:nodes bind:edges {nodeTypes} fitView>
    <Background />
    <MiniMap />
    <Controls />
  </SvelteFlow>
</div>

<style>
  .graph-page { height: 100%; width: 100%; }
  /* match the dark theme to xterm/app surfaces */
  .graph-page :global(.svelte-flow) { background: var(--bg); }
</style>
```

- [ ] **Step 3: Build + manual check**

Run: `cd /Users/andrey/Hobbies/orchid-svelte/web && command npm run build`
Expected: `created dist/bundle.js`, no errors.
Manual: Graph tab shows one node per project laid out left-to-right; pan/zoom/minimap/controls work. (Exact `@xyflow/svelte` import names: if `bind:nodes`/`nodeTypes` differ in the installed version, consult `node_modules/@xyflow/svelte` README and adapt — the data model `{id,type,data,position}` is stable.)

---

### Task 4: ProjectNode — actions + expand caret + status summary

**Files:**
- Modify: `web/src/components/graph/ProjectNode.svelte`
- Modify: `web/src/components/graph/GraphPage.svelte` (pass callbacks via node `data`)

- [ ] **Step 1: Pass interaction callbacks through node data**

In `GraphPage.svelte`, change `buildGraph` data to include shared callbacks. Add these functions in `<script>`:

```ts
  import { setHash } from '../../lib/router';
  import { openModal, confirm } from '../../lib/modals.svelte';
  import { escHtml } from '../../lib/util';
  import { refresh as refreshApp } from '../../lib/state.svelte';

  function toggleProject(id: string) {
    const next = new Set(expandedProjects);
    if (next.has(id)) { next.delete(id); } else { next.add(id); if (!worktrees.has(id)) loadWorktrees(id); }
    expandedProjects = next; rebuild();
  }
  async function loadWorktrees(pid: string) {
    try { const wts = await api.worktrees(pid); const m = new Map(worktrees); m.set(pid, wts); worktrees = m; rebuild(); } catch {}
  }
  function openProject(pid: string) {
    app.graph = false;
    app.cur = { projectId: pid, view: 'list', activeId: null, grid: [], collapsed: {} };
    setHash(); refreshApp();
  }
  async function deleteProject(p: any) {
    const ok = await confirm({ title: 'Delete project', body: `Remove <b>${escHtml(p.name)}</b> from Orchid? Git worktrees and files on disk are left untouched.`, confirmLabel: 'Delete' });
    if (!ok) return;
    try { await api.removeProject(p.id); await reload(); } catch {}
  }
  async function reload() { allSessions = await api.sessions(); app.projects = await api.projects(); rebuild(); }
```

Pass them via a single `cb` object merged into the build (simplest: attach to `app`-independent closure). Update `rebuild()` to inject callbacks into each node's data:

```ts
  const cb = { toggleProject, openProject, deleteProject, expandedProjects: () => expandedProjects };
  function rebuild() {
    const g = buildGraph({ projects: app.projects, sessions: allSessions, worktrees, expandedProjects, expandedWorktrees });
    nodes = layout(g.nodes, g.edges).map((n) => ({ ...n, data: { ...n.data, cb } }));
    edges = g.edges;
  }
```

- [ ] **Step 2: Full ProjectNode**

Replace `web/src/components/graph/ProjectNode.svelte`:

```svelte
<script lang="ts">
  import { Handle, Position } from '@xyflow/svelte';
  import { app } from '../../lib/state.svelte';
  import Icon from '../ui/Icon.svelte';
  let { data }: { data: any } = $props();
  const p = $derived(data.project);
  const expanded = $derived(data.cb.expandedProjects().has(p.id));
  const list = $derived(p.session_list || []);
  const run = $derived(list.filter((s: any) => s.status === 'running').length);
  const wait = $derived(list.filter((s: any) => s.status === 'requires_action').length);
  const idle = $derived(list.filter((s: any) => s.status === 'idle' || s.status === 'stopped').length);
</script>

<div class="gnode project">
  <div class="row">
    <button class="caret" class:open={expanded} title="Expand" onclick={() => data.cb.toggleProject(p.id)}><Icon name="chevR" size={13} /></button>
    <button class="title" title="Open project" onclick={() => data.cb.openProject(p.id)}>{p.name}</button>
    <button class="ic" title="Project settings" onclick={(e) => { e.stopPropagation(); import('../../lib/modals.svelte').then(m => m.openModal('projectSettings', { project: p })); }}><Icon name="gear" size={15} /></button>
    <button class="ic danger" title="Delete project" onclick={() => data.cb.deleteProject(p)}><Icon name="x" size={16} /></button>
  </div>
  <div class="meta">
    <span class="c ok">{run} run</span><span class="c wait">{wait} wait</span><span class="c idle">{idle} idle</span>
  </div>
  <Handle type="source" position={Position.Right} />
</div>

<style>
  .gnode { background: var(--surface); border: 1px solid var(--border-2); border-radius: var(--radius-sm); padding: 8px 10px; box-sizing: border-box; width: 240px; height: 76px; display: flex; flex-direction: column; gap: 6px; }
  .row { display: flex; align-items: center; gap: 6px; }
  .caret { color: var(--text-faint); transition: transform .15s; display: inline-flex; }
  .caret.open { transform: rotate(90deg); }
  .title { flex: 1; font-weight: 600; font-size: 13px; text-align: left; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .title:hover { color: var(--accent); }
  .ic { color: var(--text-faint); display: inline-flex; } .ic:hover { color: var(--text); } .ic.danger:hover { color: var(--danger); }
  .meta { display: flex; gap: 8px; font-size: 10.5px; }
  .c.ok { color: var(--ok); } .c.wait { color: var(--wait); } .c.idle { color: var(--text-dim); }
</style>
```

(Note: `openModal` for project settings is called via dynamic import only to avoid an extra top-level import; you may instead add `import { openModal } from '../../lib/modals.svelte'` at the top and call `openModal('projectSettings', { project: p })` directly — preferred.)

- [ ] **Step 3: Build + manual check**

Run: `cd /Users/andrey/Hobbies/orchid-svelte/web && command npm run build`
Expected: `created dist/bundle.js`, no errors.
Manual: project node shows name + run/wait/idle counts; caret toggles (worktrees appear in Task 5); gear opens project settings; delete confirms + removes; title drills into the project (leaves graph).

---

### Task 5: Worktree nodes — expand, render, actions

**Files:**
- Create: `web/src/components/graph/WorktreeNode.svelte`
- Modify: `web/src/components/graph/GraphPage.svelte` (register type + worktree callbacks)

- [ ] **Step 1: Worktree callbacks in GraphPage**

Add to `GraphPage.svelte` `<script>`:

```ts
  function toggleWorktree(path: string) {
    const next = new Set(expandedWorktrees);
    next.has(path) ? next.delete(path) : next.add(path);
    expandedWorktrees = next; rebuild();
  }
  async function removeWorktree(p: any, wt: any) {
    if (app.removingWt.has(wt.path)) return;
    const ok = await confirm({ title: 'Remove worktree?', body: `Remove <b>${escHtml(wt.branch)}</b>? This kills its sessions, removes the worktree, and deletes the branch. This cannot be undone.`, confirmLabel: 'Remove' });
    if (!ok) return;
    const add = new Set(app.removingWt); add.add(wt.path); app.removingWt = add;
    try { await api.removeWorktree(p.id, wt.path); await reload(); }
    finally { const d = new Set(app.removingWt); d.delete(wt.path); app.removingWt = d; rebuild(); }
  }
```

Extend the `cb` object:

```ts
  const cb = {
    toggleProject, openProject, deleteProject,
    toggleWorktree, removeWorktree,
    expandedProjects: () => expandedProjects,
    expandedWorktrees: () => expandedWorktrees,
  };
```

Register the node type + import:

```ts
  import WorktreeNode from './WorktreeNode.svelte';
  const nodeTypes = { project: ProjectNode, worktree: WorktreeNode } as any;
```

- [ ] **Step 2: WorktreeNode**

Create `web/src/components/graph/WorktreeNode.svelte`:

```svelte
<script lang="ts">
  import { Handle, Position } from '@xyflow/svelte';
  import { app } from '../../lib/state.svelte';
  import { openModal } from '../../lib/modals.svelte';
  import Icon from '../ui/Icon.svelte';
  import BranchLabel from '../ui/BranchLabel.svelte';
  let { data }: { data: any } = $props();
  const p = $derived(data.project);
  const wt = $derived(data.wt);
  const expanded = $derived(data.cb.expandedWorktrees().has(wt.path));
  const removing = $derived(app.removingWt.has(wt.path));
</script>

<div class="gnode worktree" class:removing>
  <Handle type="target" position={Position.Left} />
  <div class="row">
    <button class="caret" class:open={expanded} title="Expand" onclick={() => data.cb.toggleWorktree(wt.path)}><Icon name="chevR" size={12} /></button>
    <span class="br"><Icon name="branch" size={12} /><BranchLabel name={wt.name} /></span>
    <button class="ic" title="View diff" onclick={() => openModal('diff', { wt })}><Icon name="diff" size={14} /></button>
    <button class="ic" title="New session" onclick={() => openModal('newSession', { wt: wt.path })}><Icon name="plus" size={13} /></button>
    {#if !wt.is_main}
      <button class="ic danger" title="Remove worktree" onclick={() => data.cb.removeWorktree(p, wt)}><Icon name="x" size={14} /></button>
    {/if}
  </div>
  <Handle type="source" position={Position.Right} />
</div>

<style>
  .gnode { background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-sm); padding: 9px 10px; box-sizing: border-box; width: 230px; height: 64px; display: flex; align-items: center; }
  .gnode.removing { opacity: .55; pointer-events: none; }
  .row { display: flex; align-items: center; gap: 6px; width: 100%; }
  .caret { color: var(--text-faint); transition: transform .15s; display: inline-flex; } .caret.open { transform: rotate(90deg); }
  .br { flex: 1; display: inline-flex; align-items: center; gap: 5px; min-width: 0; font-family: var(--mono); font-size: 11px; }
  .br :global(.b-rest) { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .ic { color: var(--text-faint); display: inline-flex; } .ic:hover { color: var(--text); } .ic.danger:hover { color: var(--danger); }
</style>
```

- [ ] **Step 3: Build + manual check**

Run: `cd /Users/andrey/Hobbies/orchid-svelte/web && command npm run build`
Expected: `created dist/bundle.js`, no errors.
Manual: expanding a project fetches + shows its worktree nodes (edges project→worktree); diff opens; +session opens the modal with the worktree preselected; remove confirms + removes (main hidden); caret toggles sessions (Task 6).

---

### Task 6: Session nodes — render, actions, click→terminal

**Files:**
- Create: `web/src/components/graph/SessionNode.svelte`
- Modify: `web/src/components/graph/GraphPage.svelte` (register type + session callbacks)

- [ ] **Step 1: Session callbacks in GraphPage**

Add to `GraphPage.svelte` `<script>`:

```ts
  function openSessionTerminal(p: any, s: any) {
    app.graph = false;
    app.cur = { projectId: p.id, view: 'terminal', activeId: s.id, grid: [], collapsed: {} };
    setHash(); refreshApp();
  }
  async function killSession(s: any) {
    const ok = await confirm({ title: 'Kill session?', body: `Kill this session? The tmux session and its running process will be terminated.`, confirmLabel: 'Kill' });
    if (!ok) return;
    try { await api.kill(s.id); await reload(); } catch {}
  }
  async function stopSession(s: any)    { try { await api.stop(s.id); }    finally { await reload(); } }
  async function restartSession(s: any) { try { await api.restart(s.id); } finally { await reload(); } }
```

Extend `cb`:

```ts
  const cb = {
    toggleProject, openProject, deleteProject,
    toggleWorktree, removeWorktree,
    openSessionTerminal, killSession, stopSession, restartSession,
    expandedProjects: () => expandedProjects,
    expandedWorktrees: () => expandedWorktrees,
  };
```

Register the type:

```ts
  import SessionNode from './SessionNode.svelte';
  const nodeTypes = { project: ProjectNode, worktree: WorktreeNode, session: SessionNode } as any;
```

- [ ] **Step 2: SessionNode**

Create `web/src/components/graph/SessionNode.svelte`:

```svelte
<script lang="ts">
  import { Handle, Position } from '@xyflow/svelte';
  import { openModal } from '../../lib/modals.svelte';
  import { agentMeta } from '../../lib/util';
  import Badge from '../ui/Badge.svelte';
  import StatusPill from '../ui/StatusPill.svelte';
  import Icon from '../ui/Icon.svelte';
  let { data }: { data: any } = $props();
  const p = $derived(data.project);
  const s = $derived(data.session);
  const name = $derived(((s.name || '').trim()) || agentMeta(s.agent).label);
</script>

<div class="gnode session">
  <Handle type="target" position={Position.Left} />
  <div class="head">
    <Badge agent={s.agent} color={s.color} />
    <button class="name" title="Open terminal" onclick={() => data.cb.openSessionTerminal(p, s)}>{name}</button>
    <StatusPill status={s.status} id={s.id} />
  </div>
  <div class="acts">
    <button class="ic" title="Session settings" onclick={() => openModal('sessionSettings', { session: s })}><Icon name="gear" size={15} /></button>
    {#if s.alive}
      <button class="ic" title="Stop" onclick={() => data.cb.stopSession(s)}><Icon name="power" size={18} /></button>
    {:else}
      <button class="ic" title="Restart" onclick={() => data.cb.restartSession(s)}><Icon name="restart" size={15} /></button>
    {/if}
    <button class="ic danger" title="Kill" onclick={() => data.cb.killSession(s)}><Icon name="x" size={16} /></button>
  </div>
</div>

<style>
  .gnode { background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-sm); padding: 8px 10px; box-sizing: border-box; width: 250px; height: 74px; display: flex; flex-direction: column; gap: 6px; }
  .head { display: flex; align-items: center; gap: 8px; }
  .name { flex: 1; font-size: 12.5px; font-weight: 600; text-align: left; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .name:hover { color: var(--accent); }
  .acts { display: flex; gap: 6px; justify-content: flex-end; }
  .ic { color: var(--text-faint); display: inline-flex; } .ic:hover { color: var(--text); } .ic.danger:hover { color: var(--danger); }
</style>
```

- [ ] **Step 3: Build + manual check**

Run: `cd /Users/andrey/Hobbies/orchid-svelte/web && command npm run build`
Expected: `created dist/bundle.js`, no errors.
Manual: expanding a worktree shows its session nodes (edges worktree→session); name click opens the focus terminal of that session in its project (leaves graph, correct session active); gear/stop/restart/kill work; status pill matches.

---

### Task 7: Relayout button + live polling + leaf node + final wiring

**Files:**
- Modify: `web/src/components/graph/GraphPage.svelte`
- Create: `web/src/components/graph/LeafNode.svelte`
- Modify: `web/src/lib/poll.ts`

- [ ] **Step 1: LeafNode (empty placeholders)**

Create `web/src/components/graph/LeafNode.svelte`:

```svelte
<script lang="ts">
  import { Handle, Position } from '@xyflow/svelte';
  let { data }: { data: any } = $props();
</script>

<div class="leaf"><Handle type="target" position={Position.Left} />{data.text}</div>

<style>
  .leaf { background: transparent; color: var(--text-faint); font-size: 11px; font-family: var(--mono); padding: 6px 8px; box-sizing: border-box; width: 180px; height: 32px; display: flex; align-items: center; }
</style>
```

Register it in `GraphPage.svelte`:

```ts
  import LeafNode from './LeafNode.svelte';
  const nodeTypes = { project: ProjectNode, worktree: WorktreeNode, session: SessionNode, leaf: LeafNode } as any;
```

- [ ] **Step 2: Relayout button**

In `GraphPage.svelte` template, add a toolbar over the flow:

```svelte
<div class="graph-page">
  <button class="relayout" title="Re-layout" onclick={rebuild}>Relayout</button>
  <SvelteFlow bind:nodes bind:edges {nodeTypes} fitView>
    <Background />
    <MiniMap />
    <Controls />
  </SvelteFlow>
</div>
```

Add style:

```css
  .graph-page { position: relative; }
  .relayout { position: absolute; top: 12px; right: 12px; z-index: 5; padding: 5px 12px; font-size: 12px; border: 1px solid var(--border-2); border-radius: var(--radius-sm); background: var(--surface-2); color: var(--text-dim); }
  .relayout:hover { color: var(--text); }
```

- [ ] **Step 3: Graph-mode polling (live status)**

In `web/src/lib/poll.ts`, make graph mode refresh projects + all sessions and re-trigger a rebuild. Add a module-level callback the GraphPage registers:

```ts
import { app, refresh } from './state.svelte';
import { api } from './api';

let graphRefresh: (() => void) | null = null;
export function setGraphRefresh(fn: (() => void) | null) { graphRefresh = fn; }

export async function pollStatus() {
  if (app.graph) {
    try { app.projects = await api.projects(); } catch {}
    graphRefresh?.();
    return;
  }
  if (!app.cur.projectId) {
    try { const list = await api.projects(); if (JSON.stringify(list) !== JSON.stringify(app.projects)) app.projects = list; } catch {}
    return;
  }
  let list;
  try { list = (await api.sessions()).filter(s => s.project === app.cur.projectId); } catch { return; }
  const a = list.map(s => s.id).sort().join(',');
  const b = app.sessions.map(s => s.id).sort().join(',');
  if (a !== b) { await refresh(); return; }
  app.sessions = list;
}
```

In `GraphPage.svelte`, register a refresh that reloads sessions + rebuilds, and clean up:

```ts
  import { setGraphRefresh } from '../../lib/poll';
  $effect(() => {
    setGraphRefresh(async () => { try { allSessions = await api.sessions(); } catch {} rebuild(); });
    return () => setGraphRefresh(null);
  });
```

- [ ] **Step 4: Build + manual check**

Run: `cd /Users/andrey/Hobbies/orchid-svelte/web && command npm run build`
Expected: `created dist/bundle.js`, no errors.
Manual: empty project/worktree shows a faint "none" leaf; Relayout re-runs dagre; status pills/counts update within ~3 s as sessions change; creating/removing/killing reflects after the action's `reload()`.

---

### Task 8: Final parity pass + rebuild dist + Go build

**Files:**
- Modify: `web/dist/*` (rebuilt artifacts)

- [ ] **Step 1: Full frontend build**

Run: `cd /Users/andrey/Hobbies/orchid-svelte/web && command npm run build`
Expected: `created dist/bundle.js` + `bundle.css`; only pre-existing unused-selector warnings (`.detail-body`, `.toast.out`).

- [ ] **Step 2: Go build (embed includes new dist)**

Run: `cd /Users/andrey/Hobbies/orchid-svelte && go build -o orchid ./cmd/orchid`
Expected: builds, no errors.

- [ ] **Step 3: Manual acceptance checklist (vs spec)**

Verify each: topbar List⇄Graph toggle; `#/graph` survives reload; projects collapsed → expand worktrees → expand sessions; live status; project node (drill-in/settings/+worktree/delete); worktree node (diff/+session/remove, main hidden, removing spinner); session node (settings/stop/restart/kill, color/badge/status, click→focus terminal of the right session); Relayout; pan/zoom/minimap; create/remove/kill reflected; orphan sessions shown under project.

- [ ] **Step 4: Commit (only if the active git profile authorizes it)**

```bash
git add web/package.json web/src docs/superpowers && git commit -m "feat(web): global Graph view (project→worktree→session) via @xyflow/svelte"
```

---

## Notes for the implementer

- `@xyflow/svelte` v1 targets Svelte 5. The exact prop names (`bind:nodes`, `bind:edges`, `nodeTypes`, `fitView`) and `Handle`/`Position` exports are per its README — if the installed version differs, read `node_modules/@xyflow/svelte/README.md` and adapt the wiring; the `{id,type,data,position,width,height}` node shape and `{id,source,target}` edge shape used by `lib/graph.ts` are stable across versions.
- Node action handlers intentionally mirror the List components (`SessionCard.svelte`, `WtGroup.svelte`, `ProjectCard.svelte`, `DetailBar.svelte`) — same `openModal`/`confirm`/`api` calls. If any of those modules' signatures changed, match the current ones.
- Keep node components thin; all state (expand sets, caches) lives in `GraphPage`, passed down through node `data.cb`.
