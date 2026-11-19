/* hash routing: keep URL in sync so refresh restores the page — ported from web/app.js */
import { app, loadProjects, loadProject } from './state.svelte';

let suppressHash = false;

export function setHash() {
  let h = "#/";
  if (app.graph) {
    h = "#/graph";
  } else if (app.cur.projectId) {
    h = "#/p/" + encodeURIComponent(app.cur.projectId) + "/terminal";
    if (app.cur.activeId) h += "/" + encodeURIComponent(app.cur.activeId);
  }
  if (location.hash !== h) { suppressHash = true; location.hash = h; }
}

export async function applyHash() {
  const raw = location.hash.replace(/^#\/?/, "");
  if (raw === "graph") {
    app.graph = true;
    if (!app.projects.length) await loadProjects();
    return;
  }
  app.graph = false;
  const parts = raw ? raw.split("/").map(decodeURIComponent) : [];
  if (parts[0] !== "p" || !parts[1]) {
    app.cur = { projectId: null, activeId: null, collapsed: {} };
    await loadProjects();
    return;
  }
  const pid = parts[1];
  // #/p/<pid>/terminal/<sid>; legacy #/p/<pid>/list|grid links land on the terminal view
  const sid = (parts[2] === "terminal" && parts[3]) || null;
  app.cur = { projectId: pid, activeId: sid, collapsed: {} };
  try {
    if (!app.projects.length) await loadProjects();
    await loadProject(pid);
  } catch (e) {
    // unknown/removed project -> fall back to the list
    app.cur = { projectId: null, activeId: null, collapsed: {} };
    await loadProjects();
    setHash();
    return;
  }
  if (!sid || !app.sessions.some((s) => s.id === sid)) app.cur.activeId = null;
}

export async function start() {
  window.addEventListener("hashchange", () => {
    if (suppressHash) { suppressHash = false; return; }
    applyHash();
  });
  // await so the project + its sessions finish loading before the app mounts —
  // otherwise a deep-linked terminal mounts with empty app.sessions and the
  // Terminal effect clears the activeId before the session arrives.
  await applyHash();
}
