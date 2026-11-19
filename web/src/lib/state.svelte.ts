import type { Project, Worktree, Session, Settings } from './types';
import { api } from './api';

export const app = $state({
  projects: [] as Project[],
  worktrees: [] as Worktree[],
  sessions: [] as Session[],
  cur: { projectId: null as string|null, activeId: null as string|null,
         collapsed: {} as Record<string,boolean> },
  settings: { shell:'', shells:[] as string[], accent:'purple', rail_width:230, host:'', port:0, addr:'', base_url:'' } as Settings,
  removingWt: new Set<string>(),
  shellName: 'shell',
  graph: false, // global Graph view toggle
});

export async function loadProjects() { app.projects = await api.projects(); }
export async function loadProject(pid: string) {
  app.worktrees = await api.worktrees(pid);
  app.sessions = (await api.sessions()).filter(s => s.project === pid);
}
export async function refresh() {
  if (!app.cur.projectId) { await loadProjects(); return; }
  await loadProject(app.cur.projectId);
}
