import { app, refresh } from './state.svelte';
import { api } from './api';

let graphRefresh: (() => void) | null = null;
export function setGraphRefresh(fn: (() => void) | null) { graphRefresh = fn; }

export async function pollStatus() {
  if (app.graph) {
    try { const list = await api.projects(); if (JSON.stringify(list) !== JSON.stringify(app.projects)) app.projects = list; } catch {}
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
