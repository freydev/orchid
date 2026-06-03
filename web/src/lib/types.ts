export type View = 'list' | 'grid' | 'terminal';
export type Status = 'running' | 'requires_action' | 'idle' | 'stopped';

export interface SessChip { agent: string; name: string; status: Status; worktree: string; color?: string; command?: string; }
export interface Preset { id: string; label: string; cmd: string; }
export interface Project {
  id: string; name: string; path: string; color?: string; description?: string;
  shell?: string; base_branch?: string; commands?: Preset[]; branch?: string;
  worktrees: number; sessions: number; session_list: SessChip[];
}
export interface Worktree { path: string; branch: string; name: string; is_main: boolean; }
export interface Session {
  id: string; name: string; agent: string; cwd: string; project: string; worktree: string;
  status: Status; alive: boolean; grid_order: number; color?: string; shell?: string; command?: string;
}
export interface GitInfo { is_repo: boolean; has_origin: boolean; default_branch: string; base: string; branches: string[]; }
export interface DiffLine { t: 'add' | 'del' | 'ctx'; text: string; }
export interface DiffHunk { header: string; lines: DiffLine[]; }
export interface DiffFile { path: string; status: 'modified'|'added'|'deleted'|'renamed'; add: number; del: number; hunks: DiffHunk[]; }
export interface Diff { mode: 'uncommitted' | 'last_commit'; ref: string; sub: string; files: DiffFile[]; }
export interface Settings { shell: string; accent: string; rail_width: number; host: string; port: number; addr: string; shells: string[]; }
export interface ClaudeSession { id: string; title: string; updated: string; }
export interface CreateSessionReq { project: string; worktree: string; agent: string; name?: string; color?: string; shell?: string; command?: string; resume_id?: string; }
