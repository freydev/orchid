/* orchid dashboard — projects -> worktrees -> sessions, with list/grid/terminal views */
const main = document.getElementById("main");
const MAX_GRID = 9;

const AGENT = {
  claude: { label: "Claude", badge: "C", cmd: "claude" },
  codex:  { label: "Codex",  badge: "Cx", cmd: "codex" },
  shell:  { label: "Shell",  badge: ">_", cmd: "shell" },
};
// real shell (e.g. "fish") resolved from the server; overrides AGENT.shell.cmd
async function loadShell() {
  try {
    const g = await (await fetch("/api/shell")).json();
    if (g && g.name) AGENT.shell.cmd = g.name;
  } catch (e) { /* keep fallback */ }
}
const agentMeta = (a) => AGENT[a] || AGENT.shell;
const statusMeta = (s) => ({
  running:         { cls: "ok",   label: "Running",    pulse: true },
  requires_action: { cls: "wait", label: "Needs you",  pulse: true },
  idle:            { cls: "idle", label: "Idle",       pulse: false },
  stopped:         { cls: "stopped", label: "Stopped", pulse: false },
}[s] || { cls: "idle", label: "Idle", pulse: false });

const MONOKAI = {
  background: "#202020", foreground: "#f8f8f2", cursor: "#a6e22e", cursorAccent: "#202020",
  selectionBackground: "rgba(124,156,255,0.30)",
  black: "#272822", brightBlack: "#75715e", red: "#f92672", brightRed: "#f92672",
  green: "#a6e22e", brightGreen: "#a6e22e", yellow: "#e6db74", brightYellow: "#f4bf75",
  blue: "#66d9ef", brightBlue: "#66d9ef", magenta: "#ae81ff", brightMagenta: "#ae81ff",
  cyan: "#a1efe4", brightCyan: "#a1efe4", white: "#f8f8f2", brightWhite: "#f9f8f5",
};

const ICONS = {
  plus:    '<path d="M12 5v14M5 12h14"/>',
  restart: '<path d="M3 12a9 9 0 1 0 3-6.7L3 8"/><path d="M3 3v5h5"/>',
  kill:    '<rect x="6" y="6" width="12" height="12" rx="1.5"/>',
  x:       '<path d="M5 5l14 14M19 5L5 19"/>',
  list:    '<path d="M8 6h12M8 12h12M8 18h12"/><circle cx="4" cy="6" r="1.2"/><circle cx="4" cy="12" r="1.2"/><circle cx="4" cy="18" r="1.2"/>',
  grid:    '<rect x="3.5" y="3.5" width="7.5" height="7.5" rx="1.5"/><rect x="13" y="3.5" width="7.5" height="7.5" rx="1.5"/><rect x="3.5" y="13" width="7.5" height="7.5" rx="1.5"/><rect x="13" y="13" width="7.5" height="7.5" rx="1.5"/>',
  term:    '<rect x="3" y="4" width="18" height="16" rx="2"/><path d="M7 9l3 3-3 3M13 15h4"/>',
  expand:  '<path d="M8 4H4v4"/><path d="M16 4h4v4"/><path d="M16 20h4v-4"/><path d="M8 20H4v-4"/>',
  folder:  '<path d="M3 7a2 2 0 0 1 2-2h4l2 2.5h8a2 2 0 0 1 2 2V18a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7z"/>',
  branch:  '<circle cx="6" cy="6" r="2.4"/><circle cx="6" cy="18" r="2.4"/><circle cx="18" cy="8" r="2.4"/><path d="M6 8.4v7.2M8.3 7.2C12 8 14.5 8.5 15.6 9.8M15.6 9.8c.6.7.9 1.6.9 2.7v1.5"/>',
  chevR:   '<path d="M9 6l6 6-6 6"/>',
  back:    '<path d="M19 12H5M11 18l-6-6 6-6"/>',
  check:   '<path d="M5 12l4.5 4.5L19 7"/>',
  edit:    '<path d="M12 20h9"/><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4 12.5-12.5z"/>',
  trash:   '<path d="M4 7h16M10 11v6M14 11v6M6 7l1 13a2 2 0 0 0 2 2h6a2 2 0 0 0 2-2l1-13M9 7V4h6v3"/>',
  warn:    '<path d="M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0z"/><path d="M12 9v4M12 17h.01"/>',
  power:   '<path d="M12 2.5v9"/><path d="M6 6.5a8.5 8.5 0 1 0 12 0"/>',
  grip:    '<circle cx="9" cy="6" r="1.1"/><circle cx="15" cy="6" r="1.1"/><circle cx="9" cy="12" r="1.1"/><circle cx="15" cy="12" r="1.1"/><circle cx="9" cy="18" r="1.1"/><circle cx="15" cy="18" r="1.1"/>',
  layers:  '<path d="M12 3l8 4.5-8 4.5-8-4.5L12 3z"/><path d="M4 12l8 4.5 8-4.5"/>',
};
const icon = (n, sz = 16) =>
  `<svg width="${sz}" height="${sz}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">${ICONS[n] || ""}</svg>`;

const toastHost = document.createElement("div");
toastHost.className = "toasts";
document.body.appendChild(toastHost);
function toast(text, kind = "info", mono) {
  const t = document.createElement("div");
  t.className = "toast" + (kind === "error" ? " err" : kind === "ok" ? " ok" : "");
  t.innerHTML = icon(kind === "error" ? "warn" : "check", 14) + "<span></span>" + (mono ? '<span class="mono"></span>' : "");
  t.querySelector("span").textContent = text;
  if (mono) t.querySelector(".mono").textContent = mono;
  toastHost.appendChild(t);
  setTimeout(() => { t.classList.add("out"); setTimeout(() => t.remove(), 200); }, kind === "error" ? 4200 : 2600);
}

let projects = [];
let worktrees = [];
let sessions = [];
let cur = { projectId: null, view: "list", activeId: null, grid: [], collapsed: {} };
const panes = new Map();
const removingWt = new Set(); // worktree paths currently being removed (block actions everywhere)

const sessName = (s) => ((s.name || "").trim()) || agentMeta(s.agent).label;
// per-session action buttons: alive -> Stop (keep), stopped -> Restart; + rename, kill
function actsHtml(s) {
  const toggle = s.alive
    ? '<button class="btn sm icon ghost" data-act="stop" title="Stop (keep session)">' + icon("power", 22) + "</button>"
    : '<button class="btn sm icon ghost" data-act="restart" title="Restart">' + icon("restart", 18) + "</button>";
  return '<button class="btn sm icon ghost" data-act="rename" title="Rename">' + icon("edit", 18) + "</button>" +
    toggle +
    '<button class="btn sm icon ghost danger" data-act="kill" title="Kill (delete session)">' + icon("x", 22) + "</button>";
}
const rd = (s) => ({ s, m: agentMeta(s.agent), st: statusMeta(s.status), name: sessName(s), acts: actsHtml(s) });
const esc = (x) => _.escape(x == null ? "" : x);

function projDots(p) {
  const list = p.statuses || [];
  if (!list.length) return '<span class="faint" style="font-size:11.5px">no sessions</span>';
  return list.map((st) => { const m = statusMeta(st); return '<span class="dot ' + m.cls + (m.pulse ? " pulse" : "") + '"></span>'; }).join("");
}
const tplProjRow = _.template(
  '<div class="proj-row" data-pid="<%- p.id %>">' +
    '<span class="grip" title="Drag to reorder">' + icon("grip", 14) + "</span>" +
    '<div><div class="name">' + icon("folder", 17) + '<%- p.name %></div>' +
      '<div class="path"><%- p.path %></div></div>' +
    '<div class="proj-meta">' +
      '<div class="stat" title="worktrees">' + icon("layers", 14) + " <%- p.worktrees %></div>" +
      '<div class="stat" title="open sessions">' + icon("term", 14) + " <%- p.sessions %></div>" +
      '<div class="mini-dots"><%= dots %></div>' +
      '<button class="btn sm icon ghost danger proj-del" data-delproj="<%- p.id %>" title="Delete project">' + icon("trash", 18) + "</button>" +
      icon("chevR", 16) +
    "</div>" +
  '</div>'
);
const tplSessRow = _.template(
  '<div class="s-row" data-id="<%- s.id %>">' +
    '<span class="grip" title="Drag to reorder">' + icon("grip", 14) + "</span>" +
    '<div class="agent-badge <%- s.agent %> lg"><%- m.badge %></div>' +
    '<div class="meta"><div class="sname"><%- name %></div><div class="stype"><%- m.cmd %></div></div>' +
    '<div class="right"><div class="stat-w"><span class="status-pill <%- st.cls %>" data-status="<%- s.id %>"><span class="dot <%- st.cls %><%- st.pulse ? " pulse" : "" %>"></span><%- st.label %></span></div>' +
      '<div class="acts"><%= acts %></div></div>' +
  '</div>'
);
const tplRailItem = _.template(
  '<div class="rail-item <%- active ? "active" : "" %>" data-id="<%- s.id %>">' +
    '<span class="rail-drag" title="Drag to reorder"><div class="agent-badge <%- s.agent %>"><%- m.badge %></div></span>' +
    '<div class="meta"><div class="sname"><%- name %></div><div class="stype"><%- m.cmd %></div></div>' +
    '<span class="dot <%- st.cls %><%- st.pulse ? " pulse" : "" %>" data-status="<%- s.id %>"></span>' +
  '</div>'
);

async function loadProjects() { projects = await (await fetch("/api/projects")).json(); }
async function loadProject(pid) {
  worktrees = await (await fetch("/api/projects/" + pid + "/worktrees")).json();
  const all = await (await fetch("/api/sessions")).json();
  sessions = all.filter((s) => s.project === pid);
}

async function refresh() {
  if (!cur.projectId) { await loadProjects(); renderProjects(); return; }
  await loadProject(cur.projectId);
  cur.grid = cur.grid.filter((id) => sessions.some((s) => s.id === id));
  if (cur.view === "terminal" && cur.activeId && sessions.some((s) => s.id === cur.activeId)) {
    updateRail();
  } else {
    renderDetail();
  }
}

function chipHtml(c) {
  const m = agentMeta(c.agent), st = statusMeta(c.status);
  const name = (c.name || "").trim() || m.label;
  const wt = parseBranch(c.worktree || "").rest;
  return '<div class="pc-chip">' +
    '<div class="agent-badge ' + esc(c.agent) + '">' + m.badge + "</div>" +
    '<span class="cname">' + esc(name) + "</span>" +
    '<span class="dot ' + st.cls + (st.pulse ? " pulse" : "") + '"></span>' +
    '<span class="cwt mono">' + esc(wt) + "</span>" +
  "</div>";
}
const STATUS_RANK = { requires_action: 0, running: 1, idle: 2, stopped: 3 };
function projCard(p) {
  const list = p.session_list || [];
  // running first, then requires_action / idle / stopped; keep order within a rank
  const sorted = list.slice().sort((a, b) => (STATUS_RANK[a.status] ?? 9) - (STATUS_RANK[b.status] ?? 9));
  const show = sorted.slice(0, 5);
  const more = list.length - show.length;
  const body = list.length
    ? '<div class="pc-sessions">' + show.map(chipHtml).join("") + (more > 0 ? '<div class="pc-more">+' + more + " more</div>" : "") + "</div>"
    : '<div class="pc-empty">No sessions yet — open to start an agent or shell</div>';
  return '<div class="proj-card" data-pid="' + esc(p.id) + '">' +
    '<div class="pc-head">' +
      '<span class="grip" title="Drag to reorder">' + icon("grip", 14) + "</span>" +
      icon("folder", 18) +
      '<div class="pc-id"><div class="pc-name">' + esc(p.name) + '</div><div class="pc-path">' + esc(p.path) + "</div></div>" +
      '<div class="pc-meta">' +
        '<span class="stat" title="worktrees">' + icon("layers", 14) + " " + p.worktrees + "</span>" +
        '<span class="stat" title="sessions">' + icon("term", 14) + " " + p.sessions + "</span>" +
        '<button class="btn sm icon ghost danger proj-del" data-delproj="' + esc(p.id) + '" title="Delete project">' + icon("x", 22) + "</button>" +
        icon("chevR", 16) +
      "</div>" +
    "</div>" + body +
  "</div>";
}
function renderProjects() {
  teardownAll();
  const all = projects.flatMap((p) => p.session_list || []);
  const running = all.filter((s) => s.status === "running").length;
  const waiting = all.filter((s) => s.status === "requires_action").length;
  const idle = all.filter((s) => s.status === "idle" || s.status === "stopped").length;
  const sum =
    (running ? '<span class="sum ok"><span class="dot ok"></span>' + running + " running</span>" : "") +
    (waiting ? '<span class="sum wait"><span class="dot wait"></span>' + waiting + " waiting</span>" : "") +
    (idle ? '<span class="sum"><span class="dot idle"></span>' + idle + " idle</span>" : "") +
    (all.length === 0 ? "<span>no sessions running</span>" : "");
  const body = projects.length
    ? '<div class="proj-list">' + projects.map(projCard).join("") + "</div>"
    : '<div class="empty"><div class="big">No projects yet</div><div style="margin-bottom:18px">Add a git repository to start.</div>' +
      '<button class="btn primary" id="newProj" style="margin:0 auto">' + icon("plus", 15) + " New project</button></div>";
  main.innerHTML =
    '<div class="wrap"><div class="proj-page"><div class="page-head"><div><h1>Projects</h1>' +
      '<div class="sub">' + sum + "</div></div>" +
      '<button class="btn primary" id="newProj">' + icon("plus", 15) + " New project</button></div>" +
      body + "</div></div>";
  const pl = main.querySelector(".proj-list");
  if (pl) makeSortable(pl, ".proj-card", (el) => el.dataset.pid, orderProjects);
}

/* ---------- drag-and-drop reordering ---------- */
const putOrder = (url, body) => fetch(url, { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) }).catch(() => {});
// reorder arr so items whose key is in keys come first in that order (stable for the rest)
function reorderBy(arr, keys, keyFn) {
  const pos = new Map(keys.map((k, i) => [k, i]));
  const at = (x) => (pos.has(keyFn(x)) ? pos.get(keyFn(x)) : Infinity);
  return [...arr].sort((a, b) => at(a) - at(b));
}
function orderProjects(ids) { projects = reorderBy(projects, ids, (p) => p.id); putOrder("/api/projects/order", { ids }); }
function orderWorktrees(paths) { worktrees = reorderBy(worktrees, paths, (w) => w.path); putOrder("/api/projects/" + cur.projectId + "/worktrees/order", { paths }); }
function orderSessions(ids) { sessions = reorderBy(sessions, ids, (s) => s.id); putOrder("/api/sessions/order", { ids }); }
// grid has its own order (grid_order), independent of the List/Terminal session order
function orderGrid(ids) { cur.grid = ids.slice(); putOrder("/api/sessions/grid-order", { ids }); }
function defaultGrid() {
  return sessions.filter((s) => s.alive).slice().sort((a, b) => (a.grid_order || 0) - (b.grid_order || 0)).slice(0, MAX_GRID).map((s) => s.id);
}
// SortableJS-backed reordering (handles 2D grids; drag only via a handle).
// handle defaults to ".grip"; group-level sortables use ".grip-wt" so a nested
// item's grip doesn't also drag its parent group.
function makeSortable(listEl, itemSel, getId, onDrop, handle) {
  if (!window.Sortable || !listEl || Sortable.get(listEl)) return;
  Sortable.create(listEl, {
    draggable: itemSel,
    handle: handle || ".grip",
    animation: 140,
    ghostClass: "drag-ghost",
    fallbackOnBody: true,
    onEnd: () => onDrop([...listEl.querySelectorAll(":scope > " + itemSel)].map(getId)),
  });
}

function segHtml() {
  const b = (v, ic, label) =>
    '<button class="' + (cur.view === v ? "active" : "") + '" data-view="' + v + '">' + icon(ic, 14) + " " + label + "</button>";
  return '<div class="seg">' + b("list", "list", "List") + b("grid", "grid", "Grid") + b("terminal", "term", "Terminal") + "</div>";
}
function detailBarHtml(proj) {
  const ns = sessions.length, nw = worktrees.length;
  const stopped = sessions.filter((s) => !s.alive).length;
  const pruneBtn = stopped
    ? '<button class="btn sm ghost danger" id="prune" title="Remove stopped sessions">' + icon("trash", 14) + " Clear stopped (" + stopped + ")</button>"
    : "";
  return '<div class="detail-bar">' +
    '<button class="db-back" id="backProjects" title="Back to projects">' + icon("back", 15) + "</button>" +
    icon("folder", 16) +
    '<span class="db-name">' + esc(proj.name) + "</span>" +
    '<span class="db-path mono">' + esc(proj.path) + "</span>" +
    '<span class="db-meta mono">' + ns + " " + (ns === 1 ? "session" : "sessions") + " · " + nw + " wt</span>" +
    '<span class="spacer"></span>' + segHtml() + pruneBtn +
    '<button class="btn sm" id="newWt">' + icon("branch", 14) + " Worktree</button>" +
  "</div>";
}

function renderDetail() {
  teardownAll();
  const proj = projects.find((p) => p.id === cur.projectId) || { name: "", path: "" };
  const focus = cur.view === "terminal";
  const cls = focus ? " focus-mode" : "";
  main.innerHTML =
    '<div class="wrap' + cls + '">' +
      detailBarHtml(proj) +
      (cur.view === "list" ? listBodyHtml() : cur.view === "grid" ? gridShellHtml() : focusShellHtml()) +
    "</div>";
  if (cur.view === "grid") mountGrid();
  if (cur.view === "terminal") mountTerminal();
  if (cur.view === "list") {
    const wg = main.querySelector(".wt-groups");
    if (wg) makeSortable(wg, ".wt-group", (el) => el.dataset.wtpath, orderWorktrees, ".grip-wt");
    main.querySelectorAll(".sess-list").forEach((sl) => makeSortable(sl, ".s-row", (el) => el.dataset.id, orderSessions));
  }
}

function listBodyHtml() {
  const groups = worktrees.map((wt) => {
    const rows = sessions.filter((s) => s.worktree === wt.path);
    // default: empty worktrees collapsed, non-empty open; user toggle overrides
    const open = cur.collapsed[wt.path] === undefined ? rows.length > 0 : !cur.collapsed[wt.path];
    const removing = removingWt.has(wt.path);
    const sessHtml = !open ? "" : (rows.length
      ? '<div class="sess-list">' + rows.map((s) => tplSessRow(rd(s))).join("") + "</div>"
      : '<div class="faint" style="font-size:12.5px;padding:4px 2px 6px">no active sessions in this worktree</div>');
    const actions = removing
      ? '<span class="wt-removing"><span class="spinner"></span> Removing…</span>'
      : '<button class="btn ghost sm" data-newsess="' + esc(wt.path) + '">' + icon("plus", 13) + " Session</button>" +
        (wt.is_main ? "" : '<button class="btn ghost sm danger" data-rmwt="' + esc(wt.path) + '" data-branch="' + esc(wt.branch) + '" title="Remove worktree">' + icon("kill", 12) + " Remove</button>");
    return '<div class="wt-group' + (removing ? " removing" : "") + '" data-wtpath="' + esc(wt.path) + '"><div class="wt-group-head clickable" data-wtgroup="' + esc(wt.path) + '">' +
      '<span class="grip grip-wt" title="Drag to reorder">' + icon("grip", 13) + "</span>" +
      '<span class="wt-chev ' + (open ? "open" : "") + '">' + icon("chevR", 14) + "</span>" +
      '<span class="br" title="' + esc(wt.name) + '">' + icon("branch", 13) + branchInline(wt.name) + "</span>" +
      '<span class="count">· ' + rows.length + " session" + (rows.length === 1 ? "" : "s") + "</span><span class=\"spacer\"></span>" +
      actions +
      "</div>" + sessHtml + "</div>";
  }).join("");
  return '<div class="detail-body"><div class="wt-groups">' + groups + "</div>" +
    '<button class="btn ghost sm" id="newWt" style="margin-top:6px">' + icon("plus", 13) + " New worktree</button></div>";
}

function wtName(path) {
  const wt = worktrees.find((w) => w.path === path);
  return wt ? wt.name : "";
}
function gridShellHtml() {
  const full = cur.grid.length >= MAX_GRID;
  const chips = sessions.map((s) => {
    const on = cur.grid.includes(s.id);
    const disabled = !on && full;
    const st = statusMeta(s.status);
    return '<button class="gchip ' + (on ? "on" : "") + " " + (disabled ? "disabled" : "") + '" data-gchip="' + s.id + '">' +
      '<span class="dot ' + st.cls + '" data-status="' + s.id + '"></span>' + esc(agentMeta(s.agent).label) +
      '<span class="br" title="' + esc(wtName(s.worktree)) + '">' + branchInline(wtName(s.worktree)) + "</span></button>";
  }).join("");
  const selector = '<div class="grid-sel"><span class="lbl">Show in grid:</span>' +
    '<div class="gchips">' + (sessions.length ? chips : '<span class="faint">no sessions</span>') + "</div>" +
    '<span class="cap">' + cur.grid.length + "/" + MAX_GRID + "</span></div>";
  const gridBody = cur.grid.length === 0
    ? '<div class="empty"><div class="big">Select sessions for the grid</div><div>Up to ' + MAX_GRID + " terminals at once.</div></div>"
    : '<div class="term-grid ' + (cur.grid.length === 1 ? "cols-1" : "") + '" id="termGrid"></div>';
  return '<div class="detail-body">' + selector + gridBody + "</div>";
}
function paneHtml(s) {
  const m = agentMeta(s.agent), st = statusMeta(s.status);
  return '<div class="term-pane" data-pane="' + s.id + '"><div class="term" style="flex:1;min-height:0"><div class="term-head">' +
    '<span class="grip" title="Drag to reorder">' + icon("grip", 13) + "</span>" +
    '<div class="agent-badge ' + esc(s.agent) + '">' + m.badge + "</div>" +
    '<div class="term-id"><div class="term-parent">' + icon("branch", 11) + branchInline(wtName(s.worktree)) + '</div><div class="t">' + esc(sessName(s)) + "</div></div>" +
    '<div class="spacer"></div><span class="status-pill ' + st.cls + '" data-status="' + s.id + '"><span class="dot ' + st.cls + (st.pulse ? " pulse" : "") + '"></span>' + st.label + "</span>" +
    '<button class="btn sm icon ghost" data-act="rename" title="Rename">' + icon("edit", 16) + "</button>" +
    '<button class="btn sm icon ghost" data-expand="' + s.id + '" title="Open terminal">' + icon("expand", 16) + "</button></div>" +
    '<div class="pbody" style="flex:1;min-height:0;overflow:hidden;background:var(--term-bg)"></div></div></div>';
}
function mountGrid() {
  const g = document.getElementById("termGrid");
  if (!g) return;
  g.innerHTML = cur.grid.map((id) => sessions.find((s) => s.id === id)).filter(Boolean).map(paneHtml).join("");
  for (const id of cur.grid) {
    const pane = g.querySelector('[data-pane="' + id + '"] .pbody');
    if (pane) attachInto(pane, id, { noResize: true, readonly: true });
  }
  makeSortable(g, ".term-pane", (el) => el.dataset.pane, orderGrid);
}

// worktree name: type-prefix (feature/, fix/, chore/) -> uppercase chip,
// the descriptive part (ticket + description) on its own clamped row.
function parseBranch(name) {
  const i = name.indexOf("/");
  if (i === -1) return { tag: null, rest: name };
  return { tag: name.slice(0, i), rest: name.slice(i + 1) };
}
// inline branch label: [TYPE] chip + descriptive rest (reused in list/grid/term heads)
function branchInline(name) {
  const b = parseBranch(name);
  return (b.tag ? '<span class="b-tag">' + esc(b.tag) + "</span>" : "") +
    '<span class="b-rest">' + esc(b.rest) + "</span>";
}
function railHtml() {
  return worktrees.map((wt) => {
    const rows = sessions.filter((s) => s.worktree === wt.path);
    const items = rows.length
      ? rows.map((s) => tplRailItem({ ...rd(s), active: s.id === cur.activeId })).join("")
      : '<div class="rail-empty">no sessions</div>';
    const wb = parseBranch(wt.name);
    const removing = removingWt.has(wt.path);
    const addBtn = removing
      ? '<span class="spinner" title="Removing worktree…"></span>'
      : '<button class="rail-add" data-newsess="' + esc(wt.path) + '" title="New session in this worktree">+</button>';
    let head;
    const grip = '<span class="rail-drag grip-wt" title="Drag worktree to reorder">' + icon("branch", 11) + "</span>";
    if (wb.tag) {
      // type-prefix -> chip on row 1, descriptive part clamped on row 2
      head =
        '<div class="rail-head" title="' + esc(wt.name) + '">' +
          '<div class="rail-head-top">' + grip +
            '<span class="rail-wt-tag">' + esc(wb.tag) + "</span>" +
            '<span class="spacer"></span>' + addBtn +
          "</div>" +
          '<div class="rail-wt-name">' + esc(wb.rest) + "</div>" +
        "</div>";
    } else {
      // no prefix -> name inline on a single row, no wrap
      head =
        '<div class="rail-head" title="' + esc(wt.name) + '">' +
          '<div class="rail-head-top">' + grip +
            '<span class="rail-wt-name-inline">' + esc(wb.rest) + "</span>" + addBtn +
          "</div>" +
        "</div>";
    }
    return '<div class="rail-group' + (removing ? " removing" : "") + '" data-wtpath="' + esc(wt.path) + '">' + head + '<div class="rail-list">' + items + "</div></div>";
  }).join("");
}
function focusShellHtml() {
  return '<div class="focus-view"><div class="focus-rail" id="rail">' + railHtml() + "</div>" +
    '<div class="focus-term" id="focusTerm"></div></div>';
}
function mountTerminal() {
  if (!cur.activeId || !sessions.some((s) => s.id === cur.activeId)) {
    const first = sessions.find((s) => s.alive) || sessions[0];
    cur.activeId = first ? first.id : null;
  }
  updateRail();
  const host = document.getElementById("focusTerm");
  if (!host) return;
  if (!cur.activeId) { host.innerHTML = '<div id="placeholder">no sessions — create one</div>'; return; }
  host.innerHTML = focusTermChrome(sessions.find((s) => s.id === cur.activeId));
  const body = host.querySelector(".pbody");
  if (body) attachInto(body, cur.activeId);
}
function focusTermChrome(s) {
  const m = agentMeta(s.agent), st = statusMeta(s.status);
  return '<div class="term" style="flex:1;min-height:0"><div class="term-head">' +
    '<div class="agent-badge ' + esc(s.agent) + '">' + m.badge + "</div>" +
    '<div class="term-id"><div class="term-parent">' + icon("branch", 11) + branchInline(wtName(s.worktree)) + '</div><div class="t">' + esc(sessName(s)) + "</div></div>" +
    '<span class="tt">' + m.cmd + '</span><div class="spacer"></div>' +
    '<span class="status-pill ' + st.cls + '" data-status="' + s.id + '"><span class="dot ' + st.cls + (st.pulse ? " pulse" : "") + '"></span>' + st.label + "</span>" +
    actsHtml(s) + "</div>" +
    '<div class="pbody" style="flex:1;min-height:0;overflow:hidden;background:var(--term-bg)"></div></div>';
}
function updateRail() {
  const rail = document.getElementById("rail");
  if (!rail) return;
  rail.innerHTML = '<div class="rail-groups">' + railHtml() + "</div>";
  attachRailDnD();
}
function attachRailDnD() {
  const rail = document.getElementById("rail");
  if (!rail) return;
  const wrap = rail.querySelector(".rail-groups"); // fresh element each render -> fresh listeners
  if (wrap) makeSortable(wrap, ".rail-group", (el) => el.dataset.wtpath, orderWorktrees, ".grip-wt");
  // session sortable lives on a dedicated sub-container (not the draggable group) — mirrors the List tab
  rail.querySelectorAll(".rail-list").forEach((l) => makeSortable(l, ".rail-item", (el) => el.dataset.id, orderSessions, ".rail-drag"));
}

function attachInto(bodyEl, id, opts = {}) {
  teardownPane(id);
  const s = sessions.find((x) => x.id === id);
  if (!s || !s.alive) { bodyEl.innerHTML = '<div id="placeholder">stopped — restart to attach</div>'; return; }
  const ro_ = !!opts.readonly; // grid preview panes: read-only, no cursor
  const theme = ro_ ? Object.assign({}, MONOKAI, { cursor: "rgba(0,0,0,0)", cursorAccent: MONOKAI.background }) : MONOKAI;
  const term = new Terminal({ cursorBlink: !ro_, disableStdin: ro_, fontSize: 13.5, fontFamily: '"Fira Mono", "JetBrains Mono", ui-monospace, Menlo, monospace', scrollback: 5000, theme });
  const fit = new FitAddon.FitAddon();
  term.loadAddon(fit);
  term.open(bodyEl);
  const fitNow = () => {
    fit.fit();
    if (opts.noResize) { try { term.resize(220, term.rows); } catch (_) {} }
  };
  fitNow();
  const proto = location.protocol === "https:" ? "wss" : "ws";
  const ws = new WebSocket(`${proto}://${location.host}/ws/${id}`);
  ws.binaryType = "arraybuffer";
  // Grid panes are size-passive: never push a resize to tmux. With
  // window-size=latest, a tiny grid pane would otherwise shrink the shared
  // pane and break the full-screen/focus rendering of the same session.
  const send = () => { if (opts.noResize) return; if (ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows })); };
  ws.onopen = () => send();
  ws.onmessage = (e) => { if (e.data instanceof ArrayBuffer) term.write(new Uint8Array(e.data)); };
  ws.onclose = () => {};
  term.onData((d) => {
    // Drop xterm's auto-replies to terminal queries (CPR ESC[..R, Device
    // Attributes ESC[..c / ESC[?..c / ESC[>..c) and focus events (ESC[I / ESC[O).
    // In tmux control mode the inner program's queries are answered by tmux, so
    // these spurious replies would otherwise be echoed at the shell prompt.
    const clean = d.replace(/\x1b\[[?>]?[0-9;]*[Rc]|\x1b\[[IO]/g, "");
    if (clean && ws.readyState === WebSocket.OPEN) ws.send(new TextEncoder().encode(clean));
  });
  const ro = new ResizeObserver(() => { fitNow(); send(); });
  ro.observe(bodyEl);
  panes.set(id, { term, fit, ws, ro });
}
function teardownPane(id) {
  const p = panes.get(id);
  if (!p) return;
  if (p.ro) p.ro.disconnect();
  if (p.ws) { p.ws.onclose = null; p.ws.close(); }
  if (p.term) p.term.dispose();
  panes.delete(id);
}
function teardownAll() { for (const id of [...panes.keys()]) teardownPane(id); }

function setView(v) {
  cur.view = v;
  if (v === "grid" && cur.grid.length === 0) {
    cur.grid = defaultGrid();
  }
  setHash();
  renderDetail();
}
function openTerminal(id) { cur.activeId = id; cur.view = "terminal"; setHash(); renderDetail(); }

/* ---------- hash routing: keep URL in sync so refresh restores the page ---------- */
let suppressHash = false;
function setHash() {
  let h = "#/";
  if (cur.projectId) {
    h = "#/p/" + encodeURIComponent(cur.projectId) + "/" + cur.view;
    if (cur.view === "terminal" && cur.activeId) h += "/" + encodeURIComponent(cur.activeId);
  }
  if (location.hash !== h) { suppressHash = true; location.hash = h; }
}
async function applyHash() {
  const raw = location.hash.replace(/^#\/?/, "");
  const parts = raw ? raw.split("/").map(decodeURIComponent) : [];
  teardownAll();
  if (parts[0] !== "p" || !parts[1]) {
    cur = { projectId: null, view: "list", activeId: null, grid: [], collapsed: {} };
    await loadProjects();
    renderProjects();
    return;
  }
  const pid = parts[1];
  const view = (parts[2] === "grid" || parts[2] === "terminal") ? parts[2] : "list";
  const sid = parts[3] || null;
  cur = { projectId: pid, view, activeId: sid, grid: [], collapsed: {} };
  try {
    if (!projects.length) await loadProjects();
    await loadProject(pid);
  } catch (e) {
    // unknown/removed project -> fall back to the list
    cur = { projectId: null, view: "list", activeId: null, grid: [], collapsed: {} };
    await loadProjects();
    renderProjects();
    setHash();
    return;
  }
  if (view === "grid") cur.grid = defaultGrid();
  if (view === "terminal" && (!sid || !sessions.some((s) => s.id === sid))) cur.activeId = null;
  renderDetail();
}
window.addEventListener("hashchange", () => {
  if (suppressHash) { suppressHash = false; return; }
  applyHash();
});

function modal(html, wire) {
  const mount = document.createElement("div");
  mount.innerHTML = '<div class="scrim" id="scrim"><div class="modal">' + html + "</div></div>";
  document.body.appendChild(mount);
  const close = () => { mount.remove(); document.removeEventListener("keydown", onEsc); };
  function onEsc(e) { if (e.key === "Escape") close(); }
  document.addEventListener("keydown", onEsc);
  mount.querySelector("#scrim").addEventListener("mousedown", (e) => { if (e.target.id === "scrim") close(); });
  wire(mount, close);
}

// confirmDialog resolves true on confirm, false on cancel/scrim/Esc. body is raw HTML.
function confirmDialog({ title, body, confirmLabel = "Confirm", danger = true }) {
  return new Promise((resolve) => {
    const mount = document.createElement("div");
    mount.innerHTML = '<div class="scrim" id="cscrim"><div class="modal">' +
      '<div class="m-head"><h2></h2></div>' +
      '<div class="m-body"><p class="confirm-body"></p></div>' +
      '<div class="m-foot"><button class="btn ghost" id="cno">Cancel</button>' +
        '<button class="btn ' + (danger ? "danger-solid" : "primary") + '" id="cyes"></button></div>' +
      "</div></div>";
    document.body.appendChild(mount);
    mount.querySelector("h2").textContent = title;
    mount.querySelector(".confirm-body").innerHTML = body;
    mount.querySelector("#cyes").textContent = confirmLabel;
    const finish = (v) => { mount.remove(); document.removeEventListener("keydown", onKey); resolve(v); };
    function onKey(e) { if (e.key === "Escape") finish(false); else if (e.key === "Enter") finish(true); }
    document.addEventListener("keydown", onKey);
    mount.querySelector("#cscrim").addEventListener("mousedown", (e) => { if (e.target.id === "cscrim") finish(false); });
    mount.querySelector("#cno").onclick = () => finish(false);
    mount.querySelector("#cyes").onclick = () => finish(true);
    mount.querySelector("#cyes").focus();
  });
}
function newProjectModal() {
  modal(
    '<div class="m-head"><h2>New project</h2><p>A git repository directory (must contain .git).</p></div>' +
    '<div class="m-body"><div class="field"><label>Name (optional)</label><input class="text" id="pname" placeholder="my-service" /></div>' +
    '<div class="field"><label>Path</label><input class="text mono" id="ppath" placeholder="~/code/my-service" /></div></div>' +
    '<div class="m-foot"><button class="btn ghost" id="cancel">Cancel</button><button class="btn primary" id="ok">Create</button></div>',
    (mt, close) => {
      mt.querySelector("#cancel").onclick = close;
      mt.querySelector("#ok").onclick = async () => {
        const r = await fetch("/api/projects", { method: "POST", headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ name: mt.querySelector("#pname").value.trim(), path: mt.querySelector("#ppath").value.trim() }) });
        if (!r.ok) { toast("Create project failed: " + (await r.text()).trim(), "error"); return; }
        close(); await refresh(); toast("Project added", "ok");
      };
    });
}
function newWorktreeModal() {
  modal(
    '<div class="m-head"><h2>New worktree</h2><p id="wtDesc">Creates a new branch off the fresh default branch.</p></div>' +
    '<div class="m-body"><div class="field"><label>Branch</label><input class="text mono" id="branch" placeholder="feat/new-thing" /></div>' +
      '<div class="load-status" id="wtStatus" style="display:none"><span class="spinner"></span><span class="step">Working…</span></div>' +
      '<div class="load-err" id="wtErr" style="display:none"></div></div>' +
    '<div class="m-foot"><button class="btn ghost" id="cancel">Cancel</button><button class="btn primary" id="ok" disabled>Create</button></div>',
    (mt, close) => {
      const cancel = mt.querySelector("#cancel"), ok = mt.querySelector("#ok"), input = mt.querySelector("#branch");
      const status = mt.querySelector("#wtStatus"), step = status.querySelector(".step"), errEl = mt.querySelector("#wtErr");
      const desc = mt.querySelector("#wtDesc");
      let busy = false, git = null;
      const setBusy = (on) => { busy = on; ok.disabled = on; cancel.disabled = on; input.disabled = on; };

      // probe git info to show the real base + honest phases
      fetch("/api/projects/" + cur.projectId + "/gitinfo").then((r) => r.json()).then((g) => {
        git = g;
        if (!g.is_repo) {
          desc.textContent = "This project is not a git repository — cannot create a worktree.";
          desc.style.color = "var(--danger)";
          input.disabled = true; ok.disabled = true;
          return;
        }
        ok.disabled = false;
        desc.textContent = g.has_origin
          ? "Branches off " + g.base + " (fetched fresh from origin)."
          : "No origin remote — branches off local " + g.default_branch + ".";
      }).catch(() => { ok.disabled = false; });

      cancel.onclick = () => { if (!busy) close(); };
      const submit = async () => {
        if (busy || ok.disabled) return;
        const branch = input.value.trim();
        if (!branch) { input.focus(); return; }
        errEl.style.display = "none";
        setBusy(true);
        status.style.display = "flex";
        const fetching = git && git.has_origin;
        step.textContent = fetching ? "Fetching origin…" : "Creating worktree…";
        const t = fetching ? setTimeout(() => { step.textContent = "Creating worktree…"; }, 700) : null;
        try {
          const r = await fetch("/api/projects/" + cur.projectId + "/worktrees", { method: "POST", headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ branch }) });
          if (t) clearTimeout(t);
          if (!r.ok) {
            status.style.display = "none";
            errEl.textContent = "Failed: " + (await r.text()).trim();
            errEl.style.display = "block";
            setBusy(false);
            return;
          }
          step.textContent = "Done";
          close(); await refresh();
        } catch (e) {
          if (t) clearTimeout(t);
          status.style.display = "none";
          errEl.textContent = "Failed: " + e.message;
          errEl.style.display = "block";
          setBusy(false);
        }
      };
      ok.onclick = submit;
      input.addEventListener("keydown", (e) => { if (e.key === "Enter") submit(); });
    });
}
function newSessionModal(defaultWt) {
  const types = [{ id: "claude", l: "Claude", b: "C", d: "AI agent" }, { id: "codex", l: "Codex", b: "Cx", d: "AI agent" }, { id: "shell", l: "Shell", b: ">_", d: AGENT.shell.cmd + " shell" }];
  let sel = "claude";
  const opts = worktrees.filter((wt) => !removingWt.has(wt.path)).map((wt) => '<option value="' + esc(wt.path) + '"' + (wt.path === defaultWt ? " selected" : "") + ">" + esc(wt.name) + "</option>").join("");
  const grid = types.map((t) => '<button class="choice" data-type="' + t.id + '"><div class="agent-badge ' + t.id + ' lg">' + t.b + '</div><span class="lbl">' + t.l + '</span><span class="desc">' + t.d + "</span></button>").join("");
  modal(
    '<div class="m-head"><h2>New session</h2></div>' +
    '<div class="m-body"><div class="field"><label>Name (optional)</label><input class="text" id="sname" placeholder="router + sessions API" /></div>' +
    '<div class="field"><label>Worktree</label><select class="text mono" id="wt">' + opts + "</select></div>" +
    '<div class="field"><label>Type</label><div class="choice-grid">' + grid + "</div></div></div>" +
    '<div class="m-foot"><button class="btn ghost" id="cancel">Cancel</button><button class="btn primary" id="ok">Start</button></div>',
    (mt, close) => {
      mt.querySelector("#cancel").onclick = close;
      const mark = () => mt.querySelectorAll(".choice").forEach((x) => x.classList.toggle("sel", x.dataset.type === sel));
      mt.querySelectorAll(".choice").forEach((b) => b.onclick = () => { sel = b.dataset.type; mark(); });
      mark();
      mt.querySelector("#ok").onclick = async () => {
        const r = await fetch("/api/sessions", { method: "POST", headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ project: cur.projectId, worktree: mt.querySelector("#wt").value, agent: sel, name: mt.querySelector("#sname").value.trim() }) });
        if (!r.ok) { toast("Create session failed: " + (await r.text()).trim(), "error"); return; }
        const s = await r.json(); close(); await refresh(); openTerminal(s.id);
      };
    });
}
function renameModal(s) {
  modal(
    '<div class="m-head"><h2>Rename session</h2></div>' +
    '<div class="m-body"><div class="field"><label>Name</label><input class="text" id="sname" placeholder="' + esc(agentMeta(s.agent).label) + '" /></div>' +
      '<div class="faint" style="font-size:11.5px">Leave empty to fall back to the agent name.</div></div>' +
    '<div class="m-foot"><button class="btn ghost" id="cancel">Cancel</button><button class="btn primary" id="ok">Save</button></div>',
    (mt, close) => {
      const input = mt.querySelector("#sname");
      input.value = (s.name || "").trim();
      input.focus(); input.select();
      mt.querySelector("#cancel").onclick = close;
      const save = async () => {
        const name = input.value.trim();
        const r = await fetch("/api/sessions/" + s.id, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ name }) });
        if (!r.ok) { toast("Rename failed: " + (await r.text()).trim(), "error"); return; }
        close(); await refresh(); toast("Renamed", "ok");
      };
      mt.querySelector("#ok").onclick = save;
      input.addEventListener("keydown", (e) => { if (e.key === "Enter") save(); });
    });
}

main.addEventListener("click", async (e) => {
  const act = e.target.closest("[data-act]");
  if (act) {
    e.stopPropagation();
    const holder = act.closest("[data-id]") || act.closest("[data-pane]");
    const id = holder ? (holder.dataset.id || holder.dataset.pane) : cur.activeId;
    if (act.dataset.act === "rename") {
      const s = sessions.find((x) => x.id === id);
      if (s) renameModal(s);
      return;
    }
    if (act.dataset.act === "kill") {
      const s = sessions.find((x) => x.id === id);
      const label = s ? agentMeta(s.agent).label : "this session";
      const ok = await confirmDialog({ title: "Kill session?",
        body: "Kill <strong>" + esc(label) + "</strong>? The tmux session and its running process will be terminated.",
        confirmLabel: "Kill" });
      if (!ok) return;
      const r = await fetch("/api/sessions/" + id, { method: "DELETE" });
      if (!r.ok) { toast("Kill failed: " + (await r.text()).trim(), "error"); return; }
      if (id === cur.activeId && cur.view === "terminal") { cur.activeId = null; }
      cur.grid = cur.grid.filter((g) => g !== id);
      await refresh();
    } else if (act.dataset.act === "stop") {
      const r = await fetch("/api/sessions/" + id + "/stop", { method: "POST" });
      if (!r.ok) { toast("Stop failed: " + (await r.text()).trim(), "error"); return; }
      cur.grid = cur.grid.filter((g) => g !== id);
      await refresh();
      if (cur.view === "terminal") renderDetail();
      toast("Session stopped", "ok");
    } else if (act.dataset.act === "restart") {
      const r = await fetch("/api/sessions/" + id + "/restart", { method: "POST" });
      if (!r.ok) { toast("Restart failed: " + (await r.text()).trim(), "error"); return; }
      await refresh();
      if (cur.view === "terminal") renderDetail();
    }
    return;
  }
  const view = e.target.closest("[data-view]");
  if (view) { setView(view.dataset.view); return; }
  const gchip = e.target.closest("[data-gchip]");
  if (gchip && !gchip.classList.contains("disabled")) {
    const id = gchip.dataset.gchip;
    cur.grid = cur.grid.includes(id) ? cur.grid.filter((g) => g !== id) : (cur.grid.length < MAX_GRID ? [...cur.grid, id] : cur.grid);
    renderDetail();
    return;
  }
  const expand = e.target.closest("[data-expand]");
  if (expand) { openTerminal(expand.dataset.expand); return; }
  const rmwt = e.target.closest("[data-rmwt]");
  if (rmwt) {
    const path = rmwt.dataset.rmwt, branch = rmwt.dataset.branch;
    if (removingWt.has(path)) return;
    const n = sessions.filter((s) => s.worktree === path).length;
    const ok = await confirmDialog({ title: "Remove worktree?",
      body: "Remove <strong>" + esc(branch) + "</strong>? This kills " + n + " session" + (n === 1 ? "" : "s") +
        ", removes the worktree, and deletes the branch. This cannot be undone.",
      confirmLabel: "Remove" });
    if (!ok) return;
    removingWt.add(path);   // block this worktree's actions everywhere while it removes
    renderDetail();
    let r, err;
    try {
      r = await fetch("/api/projects/" + cur.projectId + "/worktrees", { method: "DELETE", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ path }) });
    } catch (e) { err = e; }
    removingWt.delete(path);
    if (err) { toast("Remove worktree failed: " + err.message, "error"); renderDetail(); return; }
    if (!r.ok) { toast("Remove worktree failed: " + (await r.text()).trim(), "error"); await refresh(); return; }
    await refresh(); toast("Worktree removed", "ok");
    return;
  }
  const ns = e.target.closest("[data-newsess]");
  if (ns) { if (!removingWt.has(ns.dataset.newsess)) newSessionModal(ns.dataset.newsess); return; }
  const wtg = e.target.closest("[data-wtgroup]");
  if (wtg) {
    const p = wtg.dataset.wtgroup;
    const rows = sessions.filter((s) => s.worktree === p).length;
    const open = cur.collapsed[p] === undefined ? rows > 0 : !cur.collapsed[p];
    cur.collapsed[p] = open; // collapse if currently open, expand if collapsed
    renderDetail();
    return;
  }
  if (e.target.closest("#newProj")) { newProjectModal(); return; }
  if (e.target.closest("#newWt")) { newWorktreeModal(); return; }
  if (e.target.closest("#prune")) {
    const n = sessions.filter((s) => !s.alive).length;
    if (!n) return;
    const ok = await confirmDialog({ title: "Clear stopped sessions?",
      body: "Remove <strong>" + n + "</strong> stopped session" + (n === 1 ? "" : "s") +
        "? Their saved state is discarded. Running sessions are untouched.",
      confirmLabel: "Clear" });
    if (!ok) return;
    const r = await fetch("/api/sessions/prune", { method: "POST" });
    if (!r.ok) { toast("Clear failed: " + (await r.text()).trim(), "error"); return; }
    const j = await r.json();
    cur.grid = cur.grid.filter((id) => sessions.some((s) => s.id === id && s.alive));
    if (cur.activeId && !sessions.some((s) => s.id === cur.activeId && s.alive)) cur.activeId = null;
    await refresh();
    toast("Removed " + j.removed + " stopped session" + (j.removed === 1 ? "" : "s"), "ok");
    return;
  }
  if (e.target.closest("#backProjects")) { teardownAll(); cur = { projectId: null, view: "list", activeId: null, grid: [], collapsed: {} }; setHash(); renderProjects(); return; }
  const rail = e.target.closest(".rail-item");
  if (rail) { cur.activeId = rail.dataset.id; setHash(); updateRail(); const host = document.getElementById("focusTerm"); if (host) { teardownAll(); host.innerHTML = focusTermChrome(sessions.find((s) => s.id === cur.activeId)); const b = host.querySelector(".pbody"); if (b) attachInto(b, cur.activeId); } return; }
  const srow = e.target.closest(".s-row");
  if (srow) { openTerminal(srow.dataset.id); return; }
  const delproj = e.target.closest("[data-delproj]");
  if (delproj) {
    e.stopPropagation();
    const id = delproj.dataset.delproj;
    const p = projects.find((x) => x.id === id);
    const n = p ? (p.sessions || 0) : 0;
    const ok = await confirmDialog({ title: "Delete project?",
      body: "Delete <strong>" + esc(p ? p.name : "") + "</strong>? Kills " + n + " open session" + (n === 1 ? "" : "s") +
        " and removes them. The git worktrees are left untouched.",
      confirmLabel: "Delete" });
    if (!ok) return;
    const r = await fetch("/api/projects/" + id, { method: "DELETE" });
    if (!r.ok) { toast("Delete failed: " + (await r.text()).trim(), "error"); return; }
    await loadProjects(); renderProjects();
    toast("Project deleted", "ok");
    return;
  }
  const prow = e.target.closest(".proj-card");
  if (prow) { cur = { projectId: prow.dataset.pid, view: "list", activeId: null, grid: [], collapsed: {} }; setHash(); refresh(); return; }
});

// grid panes are read-only previews: a click opens the session in the Terminal
// tab (ignore clicks on header buttons / the drag grip).
main.addEventListener("click", (e) => {
  if (e.target.closest("button") || e.target.closest(".grip")) return;
  const pane = e.target.closest("[data-pane]");
  if (pane) openTerminal(pane.dataset.pane);
});

async function pollStatus() {
  if (!cur.projectId) {
    // projects list: refresh counts + status dots when anything changed
    try {
      const list = await (await fetch("/api/projects")).json();
      if (JSON.stringify(list) !== JSON.stringify(projects)) { projects = list; renderProjects(); }
    } catch (e) { /* transient */ }
    return;
  }
  let list;
  try {
    list = (await (await fetch("/api/sessions")).json()).filter((s) => s.project === cur.projectId);
  } catch (e) {
    return; // transient; retry next tick
  }
  const a = list.map((s) => s.id).sort().join(",");
  const b = sessions.map((s) => s.id).sort().join(",");
  if (a !== b) { await refresh(); return; } // session set changed -> full re-render
  sessions = list;
  for (const s of list) {
    const st = statusMeta(s.status);
    document.querySelectorAll('[data-status="' + s.id + '"]').forEach((el) => {
      if (el.classList.contains("status-pill")) {
        el.className = "status-pill " + st.cls;
        el.setAttribute("data-status", s.id);
        el.innerHTML = '<span class="dot ' + st.cls + (st.pulse ? " pulse" : "") + '"></span>' + st.label;
      } else {
        el.className = "dot " + st.cls + (st.pulse ? " pulse" : "");
        el.setAttribute("data-status", s.id);
      }
    });
  }
}
setInterval(pollStatus, 3000);

loadShell().finally(applyHash);
