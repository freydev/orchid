import type { Project, Worktree, Session, GitInfo, Diff, Settings, ClaudeSession, CreateSessionReq } from './types';

async function j<T>(r: Response): Promise<T> { if (!r.ok) throw new Error((await r.text()).trim() || ('HTTP '+r.status)); return r.json() as Promise<T>; }
async function ok(r: Response): Promise<void> { if (!r.ok) throw new Error((await r.text()).trim() || ('HTTP '+r.status)); }
const put = (u:string,b:unknown) => fetch(u,{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify(b)});
const post = (u:string,b?:unknown) => fetch(u,{method:'POST',headers:{'Content-Type':'application/json'},body:b===undefined?undefined:JSON.stringify(b)});
const patch = (u:string,b:unknown) => fetch(u,{method:'PATCH',headers:{'Content-Type':'application/json'},body:JSON.stringify(b)});

export const api = {
  projects: () => fetch('/api/projects').then(j<Project[]>),
  worktrees: (pid:string) => fetch(`/api/projects/${pid}/worktrees`).then(j<Worktree[]>),
  sessions: () => fetch('/api/sessions').then(j<Session[]>),
  shell: () => fetch('/api/shell').then(j<{path:string;name:string}>),
  settings: () => fetch('/api/settings').then(j<Settings>),
  gitInfo: (pid:string) => fetch(`/api/projects/${pid}/gitinfo`).then(j<GitInfo>),
  diff: (pid:string, wt:string) => fetch(`/api/projects/${pid}/diff?worktree=${encodeURIComponent(wt)}`).then(j<Diff>),
  claudeSessions: (cwd:string) => fetch(`/api/claude-sessions?cwd=${encodeURIComponent(cwd)}`).then(j<ClaudeSession[]>),

  createSession: (b:CreateSessionReq) => post('/api/sessions', b).then(j<Session & {command?:string}>),
  updateSession: (id:string, b:{name?:string;color?:string;shell?:string;group?:string}) => patch(`/api/sessions/${id}`, b).then(ok),
  stop: (id:string) => post(`/api/sessions/${id}/stop`).then(ok),
  kill: (id:string) => fetch(`/api/sessions/${id}`,{method:'DELETE'}).then(ok),
  restart: (id:string) => post(`/api/sessions/${id}/restart`).then(ok),
  prune: () => post('/api/sessions/prune').then(j<{removed:number}>),

  addProject: (b:Record<string,unknown>) => post('/api/projects', b).then(j<Project>),
  updateProject: (id:string, b:Record<string,unknown>) => patch(`/api/projects/${id}`, b).then(j<Project>),
  removeProject: (id:string) => fetch(`/api/projects/${id}`,{method:'DELETE'}).then(ok),
  addWorktree: (pid:string, b:{branch:string;from?:string;fetch?:boolean}) => post(`/api/projects/${pid}/worktrees`, b).then(ok),
  emptyWorktrees: (pid:string) => fetch(`/api/projects/${pid}/worktrees/empty`).then(j<{path:string; branch:string}[]>),
  pruneWorktrees: (pid:string, paths:string[]) => post(`/api/projects/${pid}/worktrees/prune`,{paths}).then(j<{removed:string[]; errors:string[]|null}>),
  removeWorktree: (pid:string, path:string) => fetch(`/api/projects/${pid}/worktrees`,{method:'DELETE',headers:{'Content-Type':'application/json'},body:JSON.stringify({path})}).then(ok),

  addGroup: (pid:string, worktree:string, name:string) => post(`/api/projects/${pid}/groups`, {worktree, name}).then(j<{id:string}>),
  renameGroup: (pid:string, gid:string, name:string) => patch(`/api/projects/${pid}/groups/${gid}`, {name}).then(ok),
  removeGroup: (pid:string, gid:string) => fetch(`/api/projects/${pid}/groups/${gid}`,{method:'DELETE'}).then(ok),
  clearGroup: (pid:string, gid:string) => post(`/api/projects/${pid}/groups/${gid}/clear`).then(j<{killed:number}>),

  orderProjects: (ids:string[]) => put('/api/projects/order',{ids}),
  orderSessions: (ids:string[]) => put('/api/sessions/order',{ids}),
  orderWorktrees: (pid:string, paths:string[]) => put(`/api/projects/${pid}/worktrees/order`,{paths}),

  putSettings: (b:{shell:string;accent:string;port:number;base_url?:string;idle_timeout_min?:number}) => put('/api/settings', b).then(j<{ok:boolean;restart_needed:boolean}>),
  railWidth: (width:number) => put('/api/settings/rail-width',{width}),
  restartDaemon: () => post('/api/restart').then(j<{ok:boolean;addr:string}>),
  notify: (id:string, message='') => post(`/api/sessions/${id}/notify`,{message}).then(ok),
};
