<script lang="ts">
  import { SvelteFlow, Background, Controls, MiniMap } from '@xyflow/svelte';
  import '@xyflow/svelte/dist/style.css';
  import { app, refresh as refreshApp, loadProject } from '../../lib/state.svelte';
  import { api } from '../../lib/api';
  import type { Session, Worktree } from '../../lib/types';
  import { buildGraph, layout, GRID } from '../../lib/graph';
  import { setHash } from '../../lib/router';
  import { confirm, openModal } from '../../lib/modals.svelte';
  import { escHtml } from '../../lib/util';
  import ProjectNode from './ProjectNode.svelte';
  import WorktreeNode from './WorktreeNode.svelte';
  import SessionNode from './SessionNode.svelte';
  import LeafNode from './LeafNode.svelte';
  import { setGraphRefresh } from '../../lib/poll';

  const nodeTypes = { project: ProjectNode, worktree: WorktreeNode, session: SessionNode, leaf: LeafNode } as any;

  let allSessions = $state<Session[]>([]);
  let worktrees = $state(new Map<string, Worktree[]>());
  let expandedProjects = $state(new Set<string>());
  let expandedWorktrees = $state(new Set<string>());

  let nodes = $state.raw<any[]>([]);
  let edges = $state.raw<any[]>([]);

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
    app.cur = { projectId: pid, activeId: null, collapsed: {} };
    setHash(); refreshApp();
  }
  async function deleteProject(p: any) {
    const ok = await confirm({ title: 'Delete project', body: `Remove <b>${escHtml(p.name)}</b> from Orchid? Git worktrees and files on disk are left untouched.`, confirmLabel: 'Delete' });
    if (!ok) return;
    try { await api.removeProject(p.id); await reload(); } catch {}
  }
  async function renameProject(p: any, name: string) {
    const nm = name.trim();
    if (!nm || nm === p.name) return;
    // send the project's existing settings + new name (backend sets other fields verbatim)
    try {
      await api.updateProject(p.id, {
        name: nm, color: p.color || '', description: p.description || '',
        shell: p.shell || '', base_branch: p.base_branch || '', commands: p.commands || [],
      });
      await reload();
    } catch {}
  }
  async function reload() { allSessions = await api.sessions(); app.projects = await api.projects(); rebuild(); }

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
    try {
      await api.removeWorktree(p.id, wt.path);
      const m = new Map(worktrees); m.delete(p.id); worktrees = m;
      if (expandedProjects.has(p.id)) await loadWorktrees(p.id);
      await reload();
    }
    finally { const d = new Set(app.removingWt); d.delete(wt.path); app.removingWt = d; rebuild(); }
  }

  // Establish project context so graph-opened modals read the right project's
  // worktrees/sessions. loadProject only sets app.worktrees/app.sessions, so we
  // also set app.cur.projectId here. app.graph stays true -> no navigation.
  async function enterCtx(pid: string) { app.cur.projectId = pid; await loadProject(pid); }
  async function openProjectSettings(p: any) { await enterCtx(p.id); openModal('projectSettings', { project: p }); }
  async function newWorktree(p: any)         { await enterCtx(p.id); openModal('newWorktree'); }
  async function openDiff(p: any, wt: any)   { await enterCtx(p.id); openModal('diff', { wt }); }
  async function openNewSession(p: any, wt: any) { await enterCtx(p.id); openModal('newSession', { wt: wt.path }); }
  async function openSessionSettings(p: any, s: any) { await enterCtx(p.id); openModal('sessionSettings', { session: s }); }

  function openSessionTerminal(p: any, s: any) {
    app.graph = false;
    app.cur = { projectId: p.id, activeId: s.id, collapsed: {} };
    setHash(); refreshApp();
  }
  async function killSession(s: any) {
    const ok = await confirm({ title: 'Kill session?', body: `Kill this session? The tmux session and its running process will be terminated.`, confirmLabel: 'Kill' });
    if (!ok) return;
    try { await api.kill(s.id); await reload(); } catch {}
  }
  async function stopSession(s: any)    { try { await api.stop(s.id); }    finally { await reload(); } }
  async function restartSession(s: any) { try { await api.restart(s.id); } finally { await reload(); } }

  const cb = {
    toggleProject, openProject, deleteProject, renameProject,
    toggleWorktree, removeWorktree,
    openSessionTerminal, killSession, stopSession, restartSession,
    openProjectSettings, newWorktree, openDiff, openNewSession, openSessionSettings,
    expandedProjects: () => expandedProjects,
    expandedWorktrees: () => expandedWorktrees,
  };

  function build() {
    return buildGraph({
      projects: app.projects, sessions: allSessions, worktrees,
      expandedProjects, expandedWorktrees,
    });
  }

  // Full relayout: used on expand/collapse/Relayout/initial.
  function rebuild() {
    const g = build();
    nodes = layout(g.nodes, g.edges).map((n) => ({ ...n, data: { ...n.data, cb } }));
    edges = g.edges;
  }

  // Data-only sync: used by the 3s poll. If the structure (node-id set) is
  // unchanged, reuse current positions and skip layout() so drag/zoom survive.
  function syncGraph() {
    const g = build();
    const curIds = new Set(nodes.map((n) => n.id));
    const sameStructure = g.nodes.length === curIds.size && g.nodes.every((n) => curIds.has(n.id));
    if (!sameStructure) { rebuild(); return; }
    const posById = new Map(nodes.map((n) => [n.id, n.position]));
    nodes = g.nodes.map((n) => ({ ...n, position: posById.get(n.id) || n.position, data: { ...n.data, cb } }));
    // edges unchanged when structure is the same — don't reassign (a new array
    // makes xyflow re-render every edge <path>, which flickers).
  }

  async function loadAll() {
    try { if (!app.projects.length) app.projects = await api.projects(); } catch {}
    try { allSessions = await api.sessions(); } catch {}
    // projects + sessions expanded by default: mark all projects expanded, load
    // their worktrees, then expand every worktree so sessions show too.
    expandedProjects = new Set(app.projects.map((p) => p.id));
    await Promise.all(app.projects.filter((p) => !worktrees.has(p.id)).map((p) => loadWorktrees(p.id)));
    const paths: string[] = [];
    for (const wts of worktrees.values()) for (const w of wts) paths.push(w.path);
    expandedWorktrees = new Set(paths);
    rebuild();
  }

  $effect(() => { loadAll(); });

  $effect(() => {
    setGraphRefresh(async () => { try { allSessions = await api.sessions(); } catch {} syncGraph(); });
    return () => setGraphRefresh(null);
  });
</script>

<div class="graph-page">
  <button class="relayout" title="Re-layout" onclick={rebuild}>Relayout</button>
  <SvelteFlow bind:nodes bind:edges {nodeTypes} fitView nodesConnectable={false} nodesDraggable={false} snapGrid={[GRID, GRID]}>
    <Background />
    <MiniMap />
    <Controls />
  </SvelteFlow>
</div>

<style>
  .graph-page { height: 100%; width: 100%; position: relative; }
  .relayout { position: absolute; top: 12px; right: 12px; z-index: 5; padding: 5px 12px; font-size: 12px; border: 1px solid var(--border-2); border-radius: var(--radius-sm); background: var(--surface-2); color: var(--text-dim); }
  .relayout:hover { color: var(--text); }
  /* match the dark theme to xterm/app surfaces */
  .graph-page :global(.svelte-flow) { background: var(--bg); }

  /* tree "spreads" on expand/collapse: existing nodes slide to their new dagre
     positions. (Edges briefly point at the final positions while nodes slide —
     transient and acceptable. This is a transform on the FLOW NODE wrapper, not
     on .gnode, so it doesn't break handle measurement.) Off mid-drag. */
  .graph-page :global(.svelte-flow__node) { transition: transform .3s cubic-bezier(.2,.7,.25,1); }
  .graph-page :global(.svelte-flow__node.dragging) { transition: none; }

  /* NOTE: we deliberately do NOT CSS-transition the node's position transform —
     SvelteFlow computes edge paths from the node positions in state (which jump
     to the new layout instantly), so a CSS slide desyncs the edges mid-anim.
     Nodes snap to position; only the content animates.

     A NEW node's content pops in (scale + fade) — drawn on the inner .gnode so
     it doesn't move the node position. Only newly-created nodes animate
     (existing ones are reused by id, so it doesn't replay). */
  /* IMPORTANT: NO transform here. A transform on .gnode (it contains the
     connection Handles, and `both` fill leaves it applied permanently) makes
     .gnode a containing block and skews xyflow's handle-offset measurement, so
     edges anchor at the card center. Use opacity + a box-shadow ring pulse,
     which don't affect layout/measurement. */
  .graph-page :global(.gnode) { animation: gnode-in .22s ease both, gnode-ring .7s ease-out; }
  @keyframes gnode-in { from { opacity: 0; } to { opacity: 1; } }
  @keyframes gnode-ring {
    0%   { box-shadow: 0 0 0 2px color-mix(in srgb, var(--accent) 75%, transparent); }
    100% { box-shadow: 0 0 0 9px transparent; }
  }
  /* NOTE: no entrance animation on edges — xyflow re-renders the edge <path>
     whenever a node moves or `edges` is reassigned (every 3s poll), which
     replays the animation and makes edges flicker. Edges just appear. */
</style>
