import type { Project, Worktree, Session, Settings, View } from './types';
import { api } from './api';

export const app = $state({
  projects: [] as Project[],
  worktrees: [] as Worktree[],
  sessions: [] as Session[],
  cur: { projectId: null as string|null, view: 'list' as View, activeId: null as string|null,
         grid: [] as string[], collapsed: {} as Record<string,boolean> },
  settings: { shell:'', shells:[] as string[], accent:'purple', rail_width:230, host:'', port:0, addr:'' } as Settings,
  removingWt: new Set<string>(),
  shellName: 'shell',
});

export async function loadProjects() { app.projects = await api.projects(); }
export async function loadProject(pid: string) {
  app.worktrees = await api.worktrees(pid);
  app.sessions = (await api.sessions()).filter(s => s.project === pid);
}
export async function refresh() {
  if (!app.cur.projectId) { await loadProjects(); return; }
  await loadProject(app.cur.projectId);
  app.cur.grid = app.cur.grid.filter(id => app.sessions.some(s => s.id === id));
}
export const MAX_GRID = 9;
