import type { Status } from './types';
export const AGENT: Record<string,{label:string;badge:string;cmd:string}> = {
  claude: { label:'Claude', badge:'C', cmd:'claude' },
  codex:  { label:'Codex',  badge:'Cx', cmd:'codex' },
  shell:  { label:'Shell',  badge:'>_', cmd:'shell' },
};
export const agentMeta = (a:string) => AGENT[a] ?? AGENT.shell;
export const statusMeta = (s:Status) => ({
  running:{cls:'ok',label:'Running',pulse:true}, requires_action:{cls:'wait',label:'Needs you',pulse:true},
  idle:{cls:'idle',label:'Idle',pulse:false}, stopped:{cls:'stopped',label:'Stopped',pulse:false},
}[s] ?? {cls:'idle',label:'Idle',pulse:false});
export const STATUS_RANK: Record<string,number> = { requires_action:0, running:1, idle:2, stopped:3 };
export const parseBranch = (name:string) => { const i=name.indexOf('/'); return i<0?{tag:null as string|null,rest:name}:{tag:name.slice(0,i),rest:name.slice(i+1)}; };
export const shellName = (x?:string) => { x=(x||'').trim(); return x?(x.split('/').pop()||x):''; };
export function relTime(iso:string){ const t=Date.parse(iso); if(!t) return ''; const s=Math.max(0,(Date.now()-t)/1000);
  if(s<60) return 'just now'; if(s<3600) return Math.floor(s/60)+'m ago'; if(s<86400) return Math.floor(s/3600)+'h ago'; return Math.floor(s/86400)+'d ago'; }
// Reorder `arr` so its items follow the order of `keys` (by key). Items whose
// key isn't in `keys` sink to the end, preserving relative order (stable sort).
export function reorderBy<T>(arr:T[], keys:string[], keyFn:(x:T)=>string):T[]{
  const pos = new Map(keys.map((k,i)=>[k,i] as const));
  const at = (x:T) => pos.has(keyFn(x)) ? pos.get(keyFn(x))! : Infinity;
  return [...arr].sort((a,b)=>at(a)-at(b));
}
