/* hash routing: keep URL in sync so refresh restores the page — ported from web/app.js */
import { app, loadProjects, loadProject, MAX_GRID } from './state.svelte';

// grid has its own order (grid_order); default grid = alive sessions by grid_order, capped at MAX_GRID
function defaultGrid(): string[] {
  return app.sessions
    .filter((s) => s.alive)
    .slice()
    .sort((a, b) => (a.grid_order || 0) - (b.grid_order || 0))
    .slice(0, MAX_GRID)
    .map((s) => s.id);
}

let suppressHash = false;

export function setHash() {
  let h = "#/";
  if (app.cur.projectId) {
    h = "#/p/" + encodeURIComponent(app.cur.projectId) + "/" + app.cur.view;
    if (app.cur.view === "terminal" && app.cur.activeId) h += "/" + encodeURIComponent(app.cur.activeId);
  }
  if (location.hash !== h) { suppressHash = true; location.hash = h; }
}

export async function applyHash() {
  const raw = location.hash.replace(/^#\/?/, "");
  const parts = raw ? raw.split("/").map(decodeURIComponent) : [];
  if (parts[0] !== "p" || !parts[1]) {
    app.cur = { projectId: null, view: "list", activeId: null, grid: [], collapsed: {} };
    await loadProjects();
    return;
  }
  const pid = parts[1];
  const view = (parts[2] === "grid" || parts[2] === "terminal") ? parts[2] : "list";
  const sid = parts[3] || null;
  app.cur = { projectId: pid, view, activeId: sid, grid: [], collapsed: {} };
  try {
    if (!app.projects.length) await loadProjects();
    await loadProject(pid);
  } catch (e) {
    // unknown/removed project -> fall back to the list
    app.cur = { projectId: null, view: "list", activeId: null, grid: [], collapsed: {} };
    await loadProjects();
    setHash();
    return;
  }
  if (view === "grid") app.cur.grid = defaultGrid();
  if (view === "terminal" && (!sid || !app.sessions.some((s) => s.id === sid))) app.cur.activeId = null;
}

export function start() {
  window.addEventListener("hashchange", () => {
    if (suppressHash) { suppressHash = false; return; }
    applyHash();
  });
  applyHash();
}
