import { graphlib, layout as dagreLayout } from '@dagrejs/dagre';
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

// fixed node sizes used for dagre + rendering. Keys MUST be width/height: they
// are spread onto each node and read by both dagre layout() and SvelteFlow.
const SIZE: Record<GNodeKind, { width: number; height: number }> = {
  project:  { width: 290, height: 132 },
  worktree: { width: 300, height: 84 },
  session:  { width: 250, height: 74 },
  leaf:     { width: 180, height: 32 },
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

    // Worktrees not loaded yet (fetch in flight or failed): show a single
    // loading leaf and skip the orphan computation, otherwise every session
    // would render as an orphan while the fetch resolves.
    if (!worktrees.has(p.id)) {
      const lid = pid + ':loading';
      nodes.push({ id: lid, type: 'leaf', data: { text: 'loading…' }, position: pos, ...SIZE.leaf });
      edges.push({ id: pid + '->' + lid, source: pid, target: lid });
      continue;
    }

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
      if (!rows.length) continue; // empty worktree: no session edge/leaf drawn
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

// GRID is the alignment step: dagre positions are snapped to it and the canvas
// uses the same snapGrid for dragging, so cards stay grid-aligned.
export const GRID = 16;
const snap = (v: number) => Math.round(v / GRID) * GRID;

// layout assigns LR positions via dagre (snapped to GRID), returns a new array.
// ranksep is wide so columns (project -> worktree -> session) sit far apart.
export function layout(nodes: FlowNode[], edges: FlowEdge[]): FlowNode[] {
  const g = new graphlib.Graph();
  g.setGraph({ rankdir: 'LR', nodesep: 44, ranksep: 130, marginx: 20, marginy: 20 });
  g.setDefaultEdgeLabel(() => ({}));
  for (const n of nodes) g.setNode(n.id, { width: n.width, height: n.height });
  for (const e of edges) g.setEdge(e.source, e.target);
  dagreLayout(g);
  return nodes.map((n) => {
    const d = g.node(n.id);
    return { ...n, position: { x: snap(d.x - (n.width || 0) / 2), y: snap(d.y - (n.height || 0) / 2) } };
  });
}
