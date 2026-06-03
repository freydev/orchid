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
  selectionBackground: "rgba(232,139,176,0.30)",
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
  diff:    '<path d="M5 4h9l5 5v10a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1z"/><path d="M13.5 4v5h5"/><path d="M9 13h4M11 11v4"/><path d="M9 18h4"/>',
  gear:    '<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09a1.65 1.65 0 0 0-1-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09a1.65 1.65 0 0 0 1.51-1 1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>',
};
// curated accent colors for projects/sessions (mirrors the backend palette)
const DOT_COLORS = [
  { id: "blue", hex: "#7c9cff" }, { id: "green", hex: "#5fd39a" },
  { id: "amber", hex: "#e9a23b" }, { id: "purple", hex: "#c98bdb" },
  { id: "red", hex: "#e57373" }, { id: "cyan", hex: "#5fc8d3" },
  { id: "pink", hex: "#e88bb0" }, { id: "slate", hex: "#8a93a6" },
  { id: "monokai", hex: "#f92672" },
];
const colorHex = (id) => (DOT_COLORS.find((c) => c.id === id) || {}).hex || null;
const DEFAULT_ACCENT = "pink";
// apply an accent color id to the live CSS variables (derives dim + bg tints)
// Re-tint the whole TUI palette toward the accent hue: surfaces, borders and
// text are gray levels mixed toward the accent (the "phosphor" cast follows the
// accent), while wait/idle/danger stay semantic.
// canonical Monokai Classic palette — warm olive surfaces + pink accent, not a
// single-hue tint (that's what makes it look like monokai).
function applyMonokai() {
  const root = document.documentElement.style;
  const v = {
    "--bg": "#1e1f1c", "--surface": "#272822", "--surface-2": "#2e2f28", "--surface-3": "#383a32",
    "--border": "#41423b", "--border-2": "#565849",
    "--text": "#f8f8f2", "--text-dim": "#a59f85", "--text-faint": "#75715e",
    "--accent": "#f92672", "--accent-dim": "#c01f5b",
    "--accent-bg": "rgba(249,38,114,0.14)", "--accent-bg-2": "rgba(249,38,114,0.22)",
    "--ok": "#a6e22e", "--ok-bg": "rgba(166,226,46,0.14)",
    "--wait": "#e6db74", "--wait-bg": "rgba(230,219,116,0.15)",
    "--idle": "#75715e", "--danger": "#f92672",
    "--term-bg": "#272822",
  };
  for (const k in v) root.setProperty(k, v[k]);
  if (typeof MONOKAI !== "undefined") { MONOKAI.background = "#272822"; MONOKAI.cursorAccent = "#272822"; }
  if (typeof panes !== "undefined") {
    for (const p of panes.values()) {
      if (p.term) { try { p.term.options.theme = Object.assign({}, p.term.options.theme, { background: "#272822", cursorAccent: "#272822" }); } catch (_) {} }
    }
  }
}

function applyAccent(id) {
  if (id === "monokai") { applyMonokai(); return; }
  const hex = colorHex(id) || colorHex(DEFAULT_ACCENT);
  const h = hex.replace("#", "");
  const r = parseInt(h.slice(0, 2), 16), g = parseInt(h.slice(2, 4), 16), b = parseInt(h.slice(4, 6), 16);
  const root = document.documentElement.style;
  const hx = (a) => "#" + a.map((x) => Math.max(0, Math.min(255, Math.round(x))).toString(16).padStart(2, "0")).join("");
  // mix a neutral gray `level` toward the accent by weight w (0..1)
  const tint = (level, w) => hx([level + (r - level) * w, level + (g - level) * w, level + (b - level) * w]);

  root.setProperty("--accent", hex);
  root.setProperty("--accent-dim", hx([r * 0.72, g * 0.72, b * 0.72]));
  root.setProperty("--accent-bg", "rgba(" + r + "," + g + "," + b + ",0.12)");
  root.setProperty("--accent-bg-2", "rgba(" + r + "," + g + "," + b + ",0.20)");
  // surfaces — near-black with a faint accent cast
  root.setProperty("--bg", tint(7, 0.05));
  root.setProperty("--surface", tint(13, 0.06));
  root.setProperty("--surface-2", tint(20, 0.09));
  root.setProperty("--surface-3", tint(28, 0.12));
  root.setProperty("--border", tint(42, 0.18));
  root.setProperty("--border-2", tint(62, 0.27));
  // text — accent-tinted phosphor
  root.setProperty("--text", tint(206, 0.10));
  root.setProperty("--text-dim", tint(135, 0.18));
  root.setProperty("--text-faint", tint(82, 0.20));
  // "running"/ok follows the accent
  root.setProperty("--ok", hex);
  root.setProperty("--ok-bg", "rgba(" + r + "," + g + "," + b + ",0.12)");

  // terminal background: a very dark tint of the accent
  const t = (c) => Math.round(8 + c * 0.09);
  const termbg = hx([t(r), t(g), t(b)]);
  root.setProperty("--term-bg", termbg);
  if (typeof MONOKAI !== "undefined") { MONOKAI.background = termbg; MONOKAI.cursorAccent = termbg; }
  if (typeof panes !== "undefined") {
    for (const p of panes.values()) {
      if (p.term) { try { p.term.options.theme = Object.assign({}, p.term.options.theme, { background: termbg, cursorAccent: termbg }); } catch (_) {} }
    }
  }
}
const pcolStyle = (id) => { const h = colorHex(id); return h ? ' style="--pcol:' + h + '"' : ""; };
const scolStyle = (id) => { const h = colorHex(id); return h ? ' style="--scol:' + h + '"' : ""; };
// global settings cache (resolved shell, system shells, bind addr)
let appSettings = { shell: "", shells: [], host: "", port: 0, addr: "" };
async function loadSettings() {
  try { appSettings = await (await fetch("/api/settings")).json(); } catch (e) { /* keep defaults */ }
  applyAccent(appSettings.accent || DEFAULT_ACCENT);
  applyRailWidth(appSettings.rail_width || 230);
}

/* ---------- Terminal sidebar (rail) resizing ---------- */
const RAIL_MIN = 180, RAIL_MAX = 520;
let railWidth = 230;
function applyRailWidth(px) {
  railWidth = Math.max(RAIL_MIN, Math.min(RAIL_MAX, Math.round(px) || 230));
  document.documentElement.style.setProperty("--rail-w", railWidth + "px");
}
function persistRailWidth(px) {
  appSettings.rail_width = Math.round(px);
  fetch("/api/settings/rail-width", { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ width: Math.round(px) }) }).catch(() => {});
}
function startRailResize(e) {
  const view = e.target.closest(".focus-view");
  if (!view) return;
  e.preventDefault();
  const cs = getComputedStyle(view);
  const left = view.getBoundingClientRect().left + (parseFloat(cs.paddingLeft) || 0);
  e.target.classList.add("dragging");
  document.body.style.cursor = "col-resize";
  document.body.style.userSelect = "none";
  const move = (ev) => applyRailWidth(ev.clientX - left);
  const up = () => {
    document.removeEventListener("mousemove", move);
    document.removeEventListener("mouseup", up);
    e.target.classList.remove("dragging");
    document.body.style.cursor = "";
    document.body.style.userSelect = "";
    for (const p of panes.values()) { try { p.fit.fit(); } catch (_) {} } // reflow xterm to new width
    persistRailWidth(railWidth);
  };
  document.addEventListener("mousemove", move);
  document.addEventListener("mouseup", up);
}
document.addEventListener("mousedown", (e) => { if (e.target.id === "railResize") startRailResize(e); });
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
// agent badge, optionally with a session color marker (overrides agent color)
function badgeHtml(agent, color, extraCls) {
  const m = agentMeta(agent);
  const inner = '<div class="agent-badge ' + esc(agent) + (extraCls ? " " + extraCls : "") + '">' + m.badge + "</div>";
  if (!color) return inner;
  return '<span class="s-badge-wrap">' + inner + '<span class="s-color" style="background:' + colorHex(color) + '"></span></span>';
}
// per-session action buttons: settings, then Stop (alive) / Restart (stopped), kill
function actsHtml(s) {
  const toggle = s.alive
    ? '<button class="btn sm icon ghost" data-act="stop" title="Stop (keep session)">' + icon("power", 22) + "</button>"
    : '<button class="btn sm icon ghost" data-act="restart" title="Restart">' + icon("restart", 18) + "</button>";
  return '<button class="btn sm icon ghost" data-act="settings" title="Session settings">' + icon("gear", 18) + "</button>" +
    toggle +
    '<button class="btn sm icon ghost danger" data-act="kill" title="Kill (delete session)">' + icon("x", 22) + "</button>";
}
// the command/program label for a session: preset cmd > session shell choice > agent cmd
const sessCmd = (s) => (s.agent === "shell" && (s.shell || "").trim()) ? shellName(s.shell) : agentMeta(s.agent).cmd;
// session subtitle: a "$ <command>" tag for preset sessions, else the program
const stypeHtml = (s) => s.command ? '<span class="s-cmd-tag">$ ' + esc(s.command) + "</span>" : esc(sessCmd(s));
const rd = (s) => ({ s, m: agentMeta(s.agent), st: statusMeta(s.status), name: sessName(s), acts: actsHtml(s),
  badgeLg: badgeHtml(s.agent, s.color, "lg"), badgeSm: badgeHtml(s.agent, s.color, ""), scol: scolStyle(s.color), stype: stypeHtml(s), cmd: sessCmd(s) });
const esc = (x) => _.escape(x == null ? "" : x);
// compact relative time ("3m ago") for a timestamp string
function relTime(iso) {
  const t = Date.parse(iso);
  if (!t) return "";
  const s = Math.max(0, (Date.now() - t) / 1000);
  if (s < 60) return "just now";
  if (s < 3600) return Math.floor(s / 60) + "m ago";
  if (s < 86400) return Math.floor(s / 3600) + "h ago";
  return Math.floor(s / 86400) + "d ago";
}

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
  '<div class="s-row" data-id="<%- s.id %>"<%= scol %>>' +
    '<span class="grip" title="Drag to reorder">' + icon("grip", 14) + "</span>" +
    "<%= badgeLg %>" +
    '<div class="meta"><div class="sname"><%- name %></div><div class="stype"><%= stype %></div></div>' +
    '<div class="right"><div class="stat-w"><span class="status-pill <%- st.cls %>" data-status="<%- s.id %>"><span class="dot <%- st.cls %><%- st.pulse ? " pulse" : "" %>"></span><%- st.label %></span></div>' +
      '<div class="acts"><%= acts %></div></div>' +
  '</div>'
);
const tplRailItem = _.template(
  '<div class="rail-item <%- active ? "active" : "" %>" data-id="<%- s.id %>"<%= scol %>>' +
    '<span class="rail-drag" title="Drag to reorder"><%= badgeSm %></span>' +
    '<div class="meta"><div class="sname"><%- name %></div><div class="stype"><%- cmd %></div></div>' +
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
    badgeHtml(c.agent, c.color, "") +
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
  const desc = (p.description || "").trim() ? '<div class="pc-desc">' + esc(p.description) + "</div>" : "";
  const branch = (p.branch || "").trim() ? '<span class="pc-branch" title="current branch">' + icon("branch", 12) + branchInline(p.branch) + "</span>" : "";
  return '<div class="proj-card" data-pid="' + esc(p.id) + '"' + pcolStyle(p.color) + ">" +
    '<div class="pc-head">' +
      '<span class="grip" title="Drag to reorder">' + icon("grip", 14) + "</span>" +
      (colorHex(p.color) ? '<span class="pc-dot"></span>' : icon("folder", 18)) +
      '<div class="pc-id"><div class="pc-name">' + esc(p.name) + branch + '</div><div class="pc-path">' + esc(p.path) + "</div>" + desc + "</div>" +
      '<div class="pc-meta">' +
        '<span class="stat" title="worktrees">' + icon("layers", 14) + " " + p.worktrees + "</span>" +
        '<span class="stat" title="sessions">' + icon("term", 14) + " " + p.sessions + "</span>" +
        '<button class="pc-gear" data-projset="' + esc(p.id) + '" title="Project settings">' + icon("gear", 18) + "</button>" +
        '<button class="btn sm icon ghost danger proj-del" data-delproj="' + esc(p.id) + '" title="Delete project">' + icon("x", 22) + "</button>" +
        icon("chevR", 16) +
      "</div>" +
    "</div>" + body +
  "</div>";
}
// bottom status bar (tmux/vim style): mode · context · session counts
const statusbarEl = document.getElementById("statusbar");
function updateStatusbar() {
  if (!statusbarEl) return;
  let list, ctx;
  if (cur.projectId) {
    list = sessions;
    const proj = projects.find((p) => p.id === cur.projectId);
    ctx = '<span class="accent">' + esc(proj ? proj.name : "project") + "</span> · " + esc(cur.view);
  } else {
    list = projects.flatMap((p) => p.session_list || []);
    ctx = projects.length + " project" + (projects.length === 1 ? "" : "s");
  }
  const running = list.filter((s) => s.status === "running").length;
  const waiting = list.filter((s) => s.status === "requires_action").length;
  const idle = list.filter((s) => s.status === "idle" || s.status === "stopped").length;
  statusbarEl.innerHTML =
    '<div class="sb-seg sb-mode">orchid</div>' +
    '<div class="sb-seg sb-ctx">' + ctx + "</div>" +
    '<div class="spacer"></div>' +
    '<div class="sb-counts">' +
      '<span class="sb-count ok"><span class="dot ok"></span>' + running + " run</span>" +
      '<span class="sb-count wait"><span class="dot wait"></span>' + waiting + " wait</span>" +
      '<span class="sb-count idle"><span class="dot idle"></span>' + idle + " idle</span>" +
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
  updateStatusbar();
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
  return '<div class="seg">' + b("list", "list", "Board") + b("grid", "grid", "Grid") + b("terminal", "term", "Terminal") + "</div>";
}
function detailBarHtml(proj) {
  const ns = sessions.length, nw = worktrees.length;
  const stopped = sessions.filter((s) => !s.alive).length;
  const pruneBtn = stopped
    ? '<button class="btn sm ghost danger" id="prune" title="Remove stopped sessions">' + icon("trash", 14) + " Clear stopped (" + stopped + ")</button>"
    : "";
  return '<div class="detail-bar"' + pcolStyle(proj.color) + ">" +
    '<button class="db-back" id="backProjects" title="Back to projects">' + icon("back", 15) + "</button>" +
    (colorHex(proj.color) ? '<span class="pc-dot"></span>' : icon("folder", 16)) +
    '<span class="db-name">' + esc(proj.name) + "</span>" +
    ((proj.branch || "").trim() ? '<span class="db-branch" title="current branch">' + icon("branch", 12) + branchInline(proj.branch) + "</span>" : "") +
    '<span class="db-path mono">' + esc(proj.path) + "</span>" +
    '<span class="db-meta mono">' + ns + " " + (ns === 1 ? "session" : "sessions") + " · " + nw + " wt</span>" +
    '<span class="spacer"></span>' + segHtml() + pruneBtn +
    '<button class="btn sm icon ghost" id="projSet" title="Project settings">' + icon("gear", 19) + "</button>" +
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
    main.querySelectorAll(".cards").forEach((cl) => makeSortable(cl, ".s-card", (el) => el.dataset.id, orderSessions, ".sc-grip"));
    mountListPreviews();
  }
  updateStatusbar();
}

// session card (Board view): badge+name+status head, live read-only xterm
// preview, footer with worktree branch + actions.
function sCardHtml(s) {
  const st = statusMeta(s.status);
  const preview = s.alive
    ? '<div class="preview pbody" data-pane="' + s.id + '"></div>'
    : '<div class="preview stopped"><span class="faint">stopped — restart to attach</span></div>';
  const foot = sessBranchChip(s) || '<span class="up">' + esc(wtName(s.worktree)) + "</span>";
  return '<div class="s-card" data-id="' + s.id + '"' + scolStyle(s.color) + ">" +
    '<div class="head">' +
      '<span class="sc-grip" title="Drag to reorder">' + badgeHtml(s.agent, s.color, "lg") + "</span>" +
      '<div class="meta"><div class="sname">' + esc(sessName(s)) + '</div><div class="stype">' + stypeHtml(s) + "</div></div>" +
      '<span class="status-pill ' + st.cls + '" data-status="' + s.id + '"><span class="dot ' + st.cls + (st.pulse ? " pulse" : "") + '"></span>' + st.label + "</span>" +
    "</div>" +
    preview +
    '<div class="foot">' + foot + '<div class="acts">' + actsHtml(s) + "</div></div>" +
  "</div>";
}
function mountListPreviews() {
  for (const s of sessions) {
    if (!s.alive) continue;
    const pane = main.querySelector('.s-card[data-id="' + s.id + '"] .pbody');
    if (pane) attachInto(pane, s.id, { noResize: true, readonly: true });
  }
}

function listBodyHtml() {
  const groups = worktrees.map((wt) => {
    const rows = sessions.filter((s) => s.worktree === wt.path);
    // default: empty worktrees collapsed, non-empty open; user toggle overrides
    const open = cur.collapsed[wt.path] === undefined ? rows.length > 0 : !cur.collapsed[wt.path];
    const removing = removingWt.has(wt.path);
    const sessHtml = !open ? "" : (rows.length
      ? '<div class="cards">' + rows.map(sCardHtml).join("") + "</div>"
      : '<div class="faint" style="font-size:12.5px;padding:4px 2px 6px">no active sessions in this worktree</div>');
    const actions = removing
      ? '<span class="wt-removing"><span class="spinner"></span> Removing…</span>'
      : '<button class="btn ghost sm" data-diff="' + esc(wt.path) + '" title="View diff">' + icon("diff", 14) + " Diff</button>" +
        '<button class="btn ghost sm" data-newsess="' + esc(wt.path) + '">' + icon("plus", 13) + " Session</button>" +
        (wt.is_main ? "" : '<button class="btn ghost sm danger" data-rmwt="' + esc(wt.path) + '" data-branch="' + esc(wt.branch) + '" title="Remove worktree">' + icon("x", 14) + " Remove</button>");
    return '<div class="wt-group' + (removing ? " removing" : "") + '" data-wtpath="' + esc(wt.path) + '"><div class="wt-group-head clickable" data-wtgroup="' + esc(wt.path) + '">' +
      '<span class="grip grip-wt" title="Drag to reorder">' + icon("grip", 13) + "</span>" +
      '<span class="wt-chev ' + (open ? "open" : "") + '">' + icon("chevR", 14) + "</span>" +
      '<span class="br" title="' + esc(wt.name) + '">' + icon("branch", 13) + branchInline(wt.name) + "</span>" +
      ((wt.branch || "").trim() ? '<span class="wt-branch" title="git branch">' + icon("branch", 11) + esc(wt.branch) + "</span>" : "") +
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
// chip showing the git branch of a session's worktree (empty when unknown)
function sessBranchChip(s) {
  const wt = worktrees.find((w) => w.path === s.worktree);
  const b = wt && (wt.branch || "").trim();
  return b ? '<span class="wt-branch" title="git branch">' + icon("branch", 11) + esc(wt.branch) + "</span>" : "";
}
function gridShellHtml() {
  const full = cur.grid.length >= MAX_GRID;
  const chips = sessions.map((s) => {
    const on = cur.grid.includes(s.id);
    const disabled = !on && full;
    const st = statusMeta(s.status);
    return '<button class="gchip ' + (on ? "on" : "") + " " + (disabled ? "disabled" : "") + '" data-gchip="' + s.id + '">' +
      '<span class="dot ' + st.cls + '" data-status="' + s.id + '"></span>' + esc(sessName(s)) +
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
  return '<div class="term-pane" data-pane="' + s.id + '"><div class="term" style="flex:1;min-height:0"><div class="term-head"' + scolStyle(s.color) + ">" +
    '<span class="grip" title="Drag to reorder">' + icon("grip", 13) + "</span>" +
    badgeHtml(s.agent, s.color, "") +
    '<div class="term-id"><div class="term-parent">' + icon("branch", 11) + branchInline(wtName(s.worktree)) + '</div><div class="t">' + esc(sessName(s)) + "</div></div>" +
    '<div class="spacer"></div><span class="status-pill ' + st.cls + '" data-status="' + s.id + '"><span class="dot ' + st.cls + (st.pulse ? " pulse" : "") + '"></span>' + st.label + "</span>" +
    '<button class="btn sm icon ghost" data-act="settings" title="Session settings">' + icon("gear", 16) + "</button>" +
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
      : '<button class="rail-icon" data-diff="' + esc(wt.path) + '" title="View diff">' + icon("diff", 13) + "</button>" +
        '<button class="rail-add" data-newsess="' + esc(wt.path) + '" title="New session in this worktree">+</button>';
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
    '<div class="rail-resizer" id="railResize" title="Drag to resize"></div>' +
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
  return '<div class="term" style="flex:1;min-height:0"><div class="term-head"' + scolStyle(s.color) + ">" +
    badgeHtml(s.agent, s.color, "") +
    '<div class="term-id"><div class="term-parent">' + icon("branch", 11) + branchInline(wtName(s.worktree)) + '</div><div class="t-line"><div class="t">' + esc(sessName(s)) + "</div>" + sessBranchChip(s) + "</div></div>" +
    '<span class="tt">' + (s.command ? '<span class="s-cmd-tag">$ ' + esc(s.command) + "</span>" : esc(sessCmd(s))) + '</span><div class="spacer"></div>' +
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
  // Shift+Enter inserts a newline instead of submitting. We deliver the newline
  // as a one-character bracketed paste (CSI 200~ <CR> CSI 201~): claude, codex
  // and bracketed-paste-aware shells (fish/zsh) insert pasted newlines literally
  // instead of executing the line. We capture the keydown on the container and
  // stopImmediatePropagation so xterm never also emits its default CR (a trailing
  // bare CR would submit the just-inserted newline).
  if (!ro_) {
    bodyEl.addEventListener("keydown", (e) => {
      if (e.key === "Enter" && e.shiftKey && !e.ctrlKey && !e.altKey && !e.metaKey) {
        e.preventDefault();
        e.stopImmediatePropagation();
        if (ws.readyState === WebSocket.OPEN) ws.send(new TextEncoder().encode("\x1b[200~\r\x1b[201~"));
      }
    }, true);
  }
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
/* ---------- worktree diff viewer ---------- */
const DSTAT = { modified: "M", added: "A", deleted: "D", renamed: "R" };
function diffFileHtml(f, collapsed) {
  const hunks = (f.hunks || []).map((h) =>
    '<div class="dhunk"><div class="dhunk-head mono">' + esc(h.header) + "</div>" +
    h.lines.map((ln) => {
      const g = ln.t === "add" ? "+" : ln.t === "del" ? "−" : " ";
      return '<div class="dline ' + ln.t + '"><span class="dgutter">' + g + '</span><span class="dcode mono">' + (esc(ln.text) || " ") + "</span></div>";
    }).join("") + "</div>").join("");
  return '<div class="dfile">' +
    '<div class="dfile-head" data-dtoggle>' +
      '<span class="dfile-chev' + (collapsed ? "" : " open") + '">' + icon("chevR", 13) + "</span>" +
      '<span class="dstat ' + esc(f.status) + '">' + (DSTAT[f.status] || "M") + "</span>" +
      '<span class="dpath mono">' + esc(f.path) + "</span><span class=\"spacer\"></span>" +
      (f.add > 0 ? '<span class="dadd">+' + f.add + "</span>" : "") +
      (f.del > 0 ? '<span class="ddel">−' + f.del + "</span>" : "") +
    "</div>" +
    '<div class="dhunks"' + (collapsed ? ' style="display:none"' : "") + ">" + hunks + "</div></div>";
}
function diffModal(wt) {
  const mount = document.createElement("div");
  mount.innerHTML = '<div class="scrim" id="dscrim"><div class="diff-modal">' +
    '<div class="diff-head" id="dhead"><div class="dh-id"><div class="dh-title">' + icon("diff", 16) + " Diff</div>" +
      '<div class="dh-wt mono">' + icon("branch", 12) + " " + esc(wt.name) + "</div></div><span class=\"spacer\"></span>" +
      '<button class="btn sm icon ghost" id="dclose" title="Close">' + icon("x", 18) + "</button></div>" +
    '<div class="diff-body" id="dbody"><div class="diff-empty">Loading…</div></div></div></div>';
  document.body.appendChild(mount);
  const close = () => { mount.remove(); document.removeEventListener("keydown", onKey); };
  function onKey(e) { if (e.key === "Escape") close(); }
  document.addEventListener("keydown", onKey);
  mount.querySelector("#dscrim").addEventListener("mousedown", (e) => { if (e.target.id === "dscrim") close(); });
  mount.querySelector("#dclose").onclick = close;
  // collapse/expand a file
  mount.querySelector("#dbody").addEventListener("click", (e) => {
    const h = e.target.closest("[data-dtoggle]");
    if (!h) return;
    const file = h.closest(".dfile");
    const open = !h.querySelector(".dfile-chev").classList.contains("open");
    h.querySelector(".dfile-chev").classList.toggle("open", open);
    file.querySelector(".dhunks").style.display = open ? "" : "none";
  });
  fetch("/api/projects/" + cur.projectId + "/diff?worktree=" + encodeURIComponent(wt.path))
    .then((r) => r.ok ? r.json() : Promise.reject(new Error("HTTP " + r.status)))
    .then((d) => {
      const tot = (d.files || []).reduce((a, f) => ({ add: a.add + f.add, del: a.del + f.del }), { add: 0, del: 0 });
      const modeLbl = d.mode === "uncommitted" ? "uncommitted" : "last commit";
      mount.querySelector("#dhead").innerHTML =
        '<div class="dh-id"><div class="dh-title">' + icon("diff", 16) + " Diff" +
          '<span class="dh-mode ' + esc(d.mode).replace(/_/g, "-") + '">' + modeLbl + "</span></div>" +
          '<div class="dh-wt mono">' + icon("branch", 12) + " " + esc(wt.name) + "</div></div>" +
        '<div class="dh-meta"><span class="dh-ref mono">' + esc(d.ref) + '</span><span class="dh-sub">' + esc(d.sub) + "</span></div>" +
        '<span class="spacer"></span><div class="dh-totals"><span class="dadd">+' + tot.add + '</span><span class="ddel">−' + tot.del + "</span></div>" +
        '<button class="btn sm icon ghost" id="dclose" title="Close">' + icon("x", 18) + "</button>";
      mount.querySelector("#dclose").onclick = close;
      // collapse files by default when there are many, to keep the list scannable
      const collapseAll = (d.files || []).length > 4;
      mount.querySelector("#dbody").innerHTML = (d.files || []).length
        ? d.files.map((f) => diffFileHtml(f, collapseAll)).join("")
        : '<div class="diff-empty">No changes.</div>';
    })
    .catch((e) => { mount.querySelector("#dbody").innerHTML = '<div class="diff-empty">Failed to load diff: ' + esc(e.message) + "</div>"; });
}

function newProjectModal() {
  let color = "";
  modal(
    '<div class="m-head"><h2>New project</h2><p>A git repository directory (must contain .git).</p></div>' +
    '<div class="m-body">' +
      fieldHtml("Name (optional)", null, '<input class="text" id="pname" placeholder="my-service" />') +
      fieldHtml("Path", null, '<input class="text mono" id="ppath" placeholder="~/code/my-service" />') +
      fieldHtml("Color", null, swatchesHtml(color, true, "No color")) +
      fieldHtml("Description", null, '<textarea class="text area" id="pdesc" rows="2" placeholder="What is this project?"></textarea>') +
      '<div class="frow">' +
        fieldHtml("Shell", "default for sessions", shellSelectHtml("pshell", globalShellName())) +
        fieldHtml("Base branch", "for new worktrees", '<input class="text mono" id="pbase" placeholder="main" />') +
      "</div>" +
    "</div>" +
    '<div class="m-foot"><button class="btn ghost" id="cancel">Cancel</button><button class="btn primary" id="ok">Create</button></div>',
    (mt, close) => {
      wireSwatches(mt, (c) => { color = c; });
      mt.querySelector("#cancel").onclick = close;
      mt.querySelector("#ok").onclick = async () => {
        const r = await fetch("/api/projects", { method: "POST", headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            name: mt.querySelector("#pname").value.trim(), path: mt.querySelector("#ppath").value.trim(),
            color, description: mt.querySelector("#pdesc").value.trim(),
            shell: mt.querySelector("#pshell").value, base_branch: mt.querySelector("#pbase").value.trim(),
          }) });
        if (!r.ok) { toast("Create project failed: " + (await r.text()).trim(), "error"); return; }
        close(); await refresh(); toast("Project added", "ok");
      };
    });
}
function newWorktreeModal() {
  modal(
    '<div class="m-head"><h2>New worktree</h2><p id="wtDesc">Creates a new branch off the fresh default branch.</p></div>' +
    '<div class="m-body"><div class="field"><label>Branch</label><input class="text mono" id="branch" placeholder="feat/new-thing" /></div>' +
      '<div class="field" id="fromField" style="display:none"><label>From branch<span class="fhint">base for the new branch</span></label><select class="text mono" id="from"></select></div>' +
      '<label class="checkrow" id="fetchRow" style="display:none"><input type="checkbox" id="fetch" checked /><span>Fetch <code>origin/&lt;branch&gt;</code> first<span class="fhint">off — branch off the local copy</span></span></label>' +
      '<div class="load-status" id="wtStatus" style="display:none"><span class="spinner"></span><span class="step">Working…</span></div>' +
      '<div class="load-err" id="wtErr" style="display:none"></div></div>' +
    '<div class="m-foot"><button class="btn ghost" id="cancel">Cancel</button><button class="btn primary" id="ok" disabled>Create</button></div>',
    (mt, close) => {
      const cancel = mt.querySelector("#cancel"), ok = mt.querySelector("#ok"), input = mt.querySelector("#branch");
      const status = mt.querySelector("#wtStatus"), step = status.querySelector(".step"), errEl = mt.querySelector("#wtErr");
      const desc = mt.querySelector("#wtDesc"), fromField = mt.querySelector("#fromField"), fromSel = mt.querySelector("#from");
      const fetchRow = mt.querySelector("#fetchRow"), fetchBox = mt.querySelector("#fetch");
      let busy = false, git = null;
      const setBusy = (on) => { busy = on; ok.disabled = on; cancel.disabled = on; input.disabled = on; fromSel.disabled = on; fetchBox.disabled = on; };

      // reflect whether the typed branch already exists: existing -> checkout (base
      // irrelevant); new -> branch off the chosen "from", optionally fetched.
      const sync = () => {
        if (!git || !git.is_repo) return;
        const name = input.value.trim();
        const exists = (git.branches || []).includes(name);
        fromField.style.opacity = exists ? ".45" : "";
        fetchRow.style.opacity = exists ? ".45" : "";
        fromSel.disabled = exists || busy;
        fetchBox.disabled = exists || busy || !git.has_origin;
        if (exists) {
          desc.textContent = "Branch " + name + " exists — it will be checked out into the worktree.";
          return;
        }
        const base = fromSel.value || git.default_branch;
        const willFetch = git.has_origin && fetchBox.checked;
        desc.textContent = willFetch
          ? "New branch off origin/" + base + " (fetched fresh)."
          : "New branch off local " + base + ".";
      };

      // probe git info to show the real base + populate the "from" picker
      fetch("/api/projects/" + cur.projectId + "/gitinfo").then((r) => r.json()).then((g) => {
        git = g;
        if (!g.is_repo) {
          desc.textContent = "This project is not a git repository — cannot create a worktree.";
          desc.style.color = "var(--danger)";
          input.disabled = true; ok.disabled = true;
          return;
        }
        const branches = g.branches || [];
        // preselect the project's configured base branch, else the repo default
        const proj = projects.find((p) => p.id === cur.projectId) || {};
        const want = (proj.base_branch || "").trim() || g.default_branch;
        if (branches.length) {
          const sel = branches.includes(want) ? want : g.default_branch;
          fromSel.innerHTML = branches.map((b) => '<option value="' + esc(b) + '"' + (b === sel ? " selected" : "") + ">" + esc(b) + "</option>").join("");
          fromField.style.display = "";
        }
        if (g.has_origin) fetchRow.style.display = "flex";
        ok.disabled = false;
        sync();
      }).catch(() => { ok.disabled = false; });

      input.addEventListener("input", sync);
      fromSel.addEventListener("change", sync);
      fetchBox.addEventListener("change", sync);
      cancel.onclick = () => { if (!busy) close(); };
      const submit = async () => {
        if (busy || ok.disabled) return;
        const branch = input.value.trim();
        if (!branch) { input.focus(); return; }
        const exists = git && (git.branches || []).includes(branch);
        const doFetch = !!(git && git.has_origin && !exists && fetchBox.checked);
        errEl.style.display = "none";
        setBusy(true);
        status.style.display = "flex";
        step.textContent = doFetch ? "Fetching origin…" : "Creating worktree…";
        const t = doFetch ? setTimeout(() => { step.textContent = "Creating worktree…"; }, 700) : null;
        try {
          const r = await fetch("/api/projects/" + cur.projectId + "/worktrees", { method: "POST", headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ branch, from: fromSel.value, fetch: fetchBox.checked }) });
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
  let color = "", cmdId = "";
  // new shell sessions are preselected with the project's shell (falling back to global)
  const proj = projects.find((p) => p.id === cur.projectId) || {};
  const defShell = shellName(proj.shell) || globalShellName();
  const presets = proj.commands || [];
  const presetChips = '<button type="button" class="cmd-chip sel" data-cmd="">' + icon("term", 14) + " Plain shell</button>" +
    presets.map((p) => '<button type="button" class="cmd-chip" data-cmd="' + esc(p.id) + '"><span class="cc-label">' + esc(p.label) + '</span><span class="cc-cmd mono">' + esc(p.cmd) + "</span></button>").join("");
  const presetBody = presets.length
    ? '<div class="cmd-presets">' + presetChips + "</div>"
    : '<div class="cmd-presets">' + presetChips + '</div><div class="cmd-empty">No presets yet — add them in project settings.</div>';
  modal(
    '<div class="m-head"><h2>New session</h2></div>' +
    '<div class="m-body">' +
      fieldHtml("Name (optional)", null, '<input class="text" id="sname" placeholder="router + sessions API" />') +
      fieldHtml("Worktree", null, '<select class="text mono" id="wt">' + opts + "</select>") +
      fieldHtml("Type", null, '<div class="choice-grid">' + grid + "</div>") +
      fieldHtml("Color", null, swatchesHtml(color, true, "Agent default")) +
      '<div class="field claude-field" id="resumeField" style="display:none"><label>Resume Claude session<span class="fhint">import an existing conversation</span></label><select class="text" id="resume"><option value="">Start new conversation</option></select></div>' +
      '<div class="field shell-field" id="cmdField" style="display:none"><label>Command preset<span class="fhint">opens a tmux session running it</span></label>' + presetBody + "</div>" +
      '<div class="field shell-field" id="shellField" style="display:none">' +
        '<label>Shell<span class="fhint">for this session</span></label>' +
        shellSelectHtml("sshell", defShell) + "</div>" +
    "</div>" +
    '<div class="m-foot"><button class="btn ghost" id="cancel">Cancel</button><button class="btn primary" id="ok">Start</button></div>',
    (mt, close) => {
      mt.querySelector("#cancel").onclick = close;
      const shellField = mt.querySelector("#shellField"), cmdField = mt.querySelector("#cmdField");
      const resumeField = mt.querySelector("#resumeField"), resumeSel = mt.querySelector("#resume"), wtSel = mt.querySelector("#wt");
      const nameInput = mt.querySelector("#sname");
      let resumeId = "", resumeLoadedCwd = null, resumeSessions = [];
      // load resumable claude conversations for the selected worktree
      const loadResume = async (cwd) => {
        if (cwd === resumeLoadedCwd) return;
        resumeLoadedCwd = cwd; resumeId = "";
        try { resumeSessions = (await (await fetch("/api/claude-sessions?cwd=" + encodeURIComponent(cwd))).json()) || []; }
        catch (e) { resumeSessions = []; }
        resumeSel.innerHTML = '<option value="">Start new conversation</option>' +
          resumeSessions.map((s) => '<option value="' + esc(s.id) + '">' + esc((s.title || ("session " + s.id.slice(0, 8))) + " · " + relTime(s.updated)) + "</option>").join("");
      };
      const mark = () => {
        mt.querySelectorAll(".choice").forEach((x) => x.classList.toggle("sel", x.dataset.type === sel));
        const isShell = sel === "shell", isClaude = sel === "claude";
        shellField.style.display = isShell ? "" : "none";
        cmdField.style.display = isShell ? "" : "none";
        resumeField.style.display = isClaude ? "" : "none";
        if (isClaude) loadResume(wtSel.value);
      };
      wtSel.addEventListener("change", () => { if (sel === "claude") loadResume(wtSel.value); });
      resumeSel.addEventListener("change", () => {
        resumeId = resumeSel.value;
        const s = resumeSessions.find((x) => x.id === resumeId);
        if (resumeId && s && s.title && !nameInput.value.trim()) nameInput.value = s.title;
      });
      mt.querySelectorAll(".choice").forEach((b) => b.onclick = () => { sel = b.dataset.type; mark(); });
      // preset chip selection: updates cmdId + name placeholder
      mt.querySelectorAll(".cmd-chip").forEach((c) => c.onclick = () => {
        cmdId = c.dataset.cmd;
        mt.querySelectorAll(".cmd-chip").forEach((x) => x.classList.toggle("sel", x === c));
        const chosen = presets.find((p) => p.id === cmdId);
        nameInput.placeholder = chosen ? chosen.label : "router + sessions API";
      });
      wireSwatches(mt, (c) => { color = c; });
      mark();
      mt.querySelector("#ok").onclick = async () => {
        const chosen = sel === "shell" ? presets.find((p) => p.id === cmdId) : null;
        const name = nameInput.value.trim() || (chosen ? chosen.label : "");
        const body = { project: cur.projectId, worktree: mt.querySelector("#wt").value, agent: sel, name, color };
        if (sel === "shell") { body.shell = mt.querySelector("#sshell").value; if (chosen) body.command = chosen.cmd; }
        if (sel === "claude" && resumeId) body.resume_id = resumeId;
        const r = await fetch("/api/sessions", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
        if (!r.ok) { toast("Create session failed: " + (await r.text()).trim(), "error"); return; }
        const s = await r.json(); close(); await refresh(); openTerminal(s.id);
      };
    });
}
/* ---------- settings: reusable form-control builders ---------- */
function fieldHtml(label, hint, control) {
  return '<div class="field"><label>' + esc(label) + (hint ? '<span class="fhint">' + esc(hint) + "</span>" : "") + "</label>" + control + "</div>";
}
// color swatches; allowNone adds an "inherit/none" chip (value "")
function swatchesHtml(value, allowNone, noneLabel) {
  const none = allowNone
    ? '<button type="button" class="swatch none' + (!value ? " sel" : "") + '" data-color="" title="' + esc(noneLabel || "Default") + '">' + icon("x", 12) + "</button>"
    : "";
  const sw = DOT_COLORS.map((c) =>
    '<button type="button" class="swatch' + (value === c.id ? " sel" : "") + '" data-color="' + c.id + '" style="--sw:' + c.hex + '" title="' + c.id + '"></button>').join("");
  return '<div class="swatches" data-swatches>' + none + sw + "</div>";
}
// bare shell name from a name-or-path ("/opt/homebrew/bin/fish" -> "fish")
const shellName = (x) => { x = (x || "").trim(); return x ? (x.split("/").pop() || x) : ""; };
// the resolved global shell name (from /api/shell), with sane fallbacks
const globalShellName = () => shellName(AGENT.shell.cmd) || (appSettings.shells && appSettings.shells[0]) || "bash";
// a concrete shell <select>, value preselected (added to the list if unknown)
function shellSelectHtml(id, value) {
  const names = (appSettings.shells && appSettings.shells.length) ? appSettings.shells : ["zsh", "bash", "fish", "sh"];
  const v = shellName(value);
  const opts = names.map((n) => '<option value="' + esc(n) + '"' + (v === n ? " selected" : "") + ">" + esc(n) + "</option>");
  if (v && !names.includes(v)) opts.unshift('<option value="' + esc(v) + '" selected>' + esc(v) + "</option>");
  return '<select class="text" id="' + id + '">' + opts.join("") + "</select>";
}
// attach swatch click behavior; onPick(colorId) fires with "" for none
function wireSwatches(mt, onPick) {
  const box = mt.querySelector("[data-swatches]");
  if (!box) return;
  box.addEventListener("click", (e) => {
    const b = e.target.closest(".swatch");
    if (!b) return;
    box.querySelectorAll(".swatch").forEach((x) => x.classList.remove("sel"));
    b.classList.add("sel");
    onPick(b.dataset.color || "");
  });
}

/* ---------- settings modals: global / project / session ---------- */
function globalSettingsModal() {
  const shellSel = shellSelectHtml("gshell", appSettings.shell || globalShellName());
  let accent = appSettings.accent || DEFAULT_ACCENT;
  modal(
    '<div class="m-head"><h2>Settings</h2><p>Global defaults. Projects and sessions can override these.</p></div>' +
    '<div class="m-body">' +
      '<div class="set-scope">' + icon("gear", 13) + " Global</div>" +
      fieldHtml("Accent color", "UI theme", swatchesHtml(accent, false)) +
      fieldHtml("Default shell", "for new terminals", shellSel) +
      fieldHtml("Server port", "restart to apply",
        '<div class="port-row"><input class="text mono" id="gport" value="' + esc(appSettings.port || "") + '" placeholder="8081" />' +
        '<button class="btn icon" id="grestart" title="Restart the orchid daemon">' + icon("restart", 17) + "</button></div>") +
      '<div class="set-soon mono">listening on ' + esc(appSettings.addr || "?") + "</div>" +
    "</div>" +
    '<div class="m-foot"><button class="btn ghost" id="cancel">Cancel</button><button class="btn primary" id="ok">Save</button></div>',
    (mt, close) => {
      mt.querySelector("#cancel").onclick = close;
      wireSwatches(mt, (c) => { accent = c || DEFAULT_ACCENT; });
      const persist = async () => {
        const shell = mt.querySelector("#gshell").value;
        const port = parseInt(mt.querySelector("#gport").value.trim(), 10) || 0;
        const r = await fetch("/api/settings", { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ shell, port, accent }) });
        if (!r.ok) { toast("Save settings failed: " + (await r.text()).trim(), "error"); return null; }
        const j = await r.json();
        await loadSettings(); await loadShell();
        return j;
      };
      mt.querySelector("#ok").onclick = async () => { const j = await persist(); if (j) { close(); toast("Settings saved", "ok"); } };
      mt.querySelector("#grestart").onclick = async () => {
        const j = await persist();
        if (!j) return;
        const ok = await confirmDialog({ title: "Restart server?",
          body: "The orchid daemon will restart" + (j.restart_needed ? " on the new port" : "") + ". Running sessions are kept (tmux); reload the page once it's back.",
          confirmLabel: "Restart", danger: false });
        if (!ok) return;
        const rr = await fetch("/api/restart", { method: "POST" });
        if (!rr.ok) { toast("Restart failed: " + (await rr.text()).trim(), "error"); return; }
        const res = await rr.json();
        close();
        toast("Restarting — reload " + (res.addr || "the page") + " shortly", "ok");
      };
    });
}

function projectSettingsModal(p) {
  let color = p.color || "";
  modal(
    '<div class="m-head"><h2>Project settings</h2><p class="mono">' + esc(p.path) + "</p></div>" +
    '<div class="m-body">' +
      '<div class="set-scope proj">' + icon("folder", 13) + " " + esc(p.name) + "</div>" +
      fieldHtml("Name", null, '<input class="text" id="pname" />') +
      fieldHtml("Color", null, swatchesHtml(color, true, "No color")) +
      fieldHtml("Description", null, '<textarea class="text area" id="pdesc" rows="2" placeholder="What is this project?"></textarea>') +
      '<div class="frow">' +
        fieldHtml("Shell", "default for sessions", shellSelectHtml("pshell", p.shell || globalShellName())) +
        fieldHtml("Base branch", "for new worktrees", '<select class="text mono" id="pbase"></select>') +
      "</div>" +
      '<div class="field"><label>Command presets<span class="fhint">selectable when starting a shell</span></label>' +
        '<div class="cmd-editor" id="cmdEditor"></div>' +
        '<button type="button" class="btn sm cmd-add" id="cmdAdd">' + icon("plus", 14) + " Add command</button></div>" +
    "</div>" +
    '<div class="m-foot"><button class="btn ghost" id="cancel">Cancel</button><button class="btn primary" id="ok">Save</button></div>',
    (mt, close) => {
      mt.querySelector("#pname").value = p.name || "";
      mt.querySelector("#pdesc").value = p.description || "";
      // command-presets editor: rows of (label, cmd) managed in the DOM
      const cmdEditor = mt.querySelector("#cmdEditor");
      const cmdRow = (c) => {
        const row = document.createElement("div");
        row.className = "cmd-row";
        row.dataset.cid = c.id || "";
        row.innerHTML = '<input class="text cmd-label-in" placeholder="label" />' +
          '<input class="text mono cmd-cmd-in" placeholder="npm run dev" spellcheck="false" />' +
          '<button type="button" class="btn sm icon ghost danger" title="Remove">' + icon("x", 18) + "</button>";
        row.querySelector(".cmd-label-in").value = c.label || "";
        row.querySelector(".cmd-cmd-in").value = c.cmd || "";
        row.querySelector("button").onclick = () => { row.remove(); syncEmpty(); };
        return row;
      };
      const syncEmpty = () => {
        let e = cmdEditor.querySelector(".cmd-empty");
        if (!cmdEditor.querySelector(".cmd-row")) { if (!e) { e = document.createElement("div"); e.className = "cmd-empty"; e.textContent = "No command presets. Add one to start tmux sessions running it."; cmdEditor.appendChild(e); } }
        else if (e) e.remove();
      };
      (p.commands || []).forEach((c) => cmdEditor.appendChild(cmdRow(c)));
      syncEmpty();
      mt.querySelector("#cmdAdd").onclick = () => { cmdEditor.appendChild(cmdRow({})); syncEmpty(); cmdEditor.querySelector(".cmd-row:last-child .cmd-label-in").focus(); };
      // base-branch picker: (repo default) + local branches; preselect saved value
      const pbase = mt.querySelector("#pbase");
      const want = (p.base_branch || "").trim();
      const fillBase = (branches) => {
        const set = new Set(branches);
        const opts = ['<option value=""' + (!want ? " selected" : "") + ">(repo default)</option>"];
        if (want && !set.has(want)) opts.push('<option value="' + esc(want) + '" selected>' + esc(want) + "</option>");
        for (const b of branches) opts.push('<option value="' + esc(b) + '"' + (b === want ? " selected" : "") + ">" + esc(b) + "</option>");
        pbase.innerHTML = opts.join("");
      };
      fillBase([]);
      fetch("/api/projects/" + p.id + "/gitinfo").then((r) => r.json()).then((g) => { if (g.is_repo) fillBase(g.branches || []); }).catch(() => {});
      wireSwatches(mt, (c) => { color = c; });
      mt.querySelector("#cancel").onclick = close;
      mt.querySelector("#ok").onclick = async () => {
        const name = mt.querySelector("#pname").value.trim();
        if (!name) { mt.querySelector("#pname").focus(); return; }
        const commands = [...cmdEditor.querySelectorAll(".cmd-row")].map((row) => ({
          id: row.dataset.cid || "", label: row.querySelector(".cmd-label-in").value.trim(), cmd: row.querySelector(".cmd-cmd-in").value.trim(),
        })).filter((c) => c.label && c.cmd);
        const body = { name, color, description: mt.querySelector("#pdesc").value.trim(), shell: mt.querySelector("#pshell").value, base_branch: mt.querySelector("#pbase").value.trim(), commands };
        const r = await fetch("/api/projects/" + p.id, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
        if (!r.ok) { toast("Save failed: " + (await r.text()).trim(), "error"); return; }
        close();
        await loadProjects();
        if (cur.projectId) await refresh(); else renderProjects();
        toast("Project saved", "ok");
      };
    });
}

function sessionSettingsModal(s) {
  let color = s.color || "";
  const isShell = s.agent === "shell";
  // preselect the session's own shell, else project, else global
  const proj = projects.find((p) => p.id === s.project) || {};
  const curShell = shellName(s.shell) || shellName(proj.shell) || globalShellName();
  const shellField = isShell
    ? fieldHtml("Shell", "for this session", shellSelectHtml("sshell", curShell))
    : "";
  modal(
    '<div class="m-head"><h2>Session settings</h2><p><span class="mono">' + esc(agentMeta(s.agent).cmd) + "</span></p></div>" +
    '<div class="m-body">' +
      '<div class="set-scope sess">' + badgeHtml(s.agent, s.color, "") + " " + esc(sessName(s)) + "</div>" +
      fieldHtml("Name", null, '<input class="text" id="sname" placeholder="' + esc(agentMeta(s.agent).label) + '" />') +
      fieldHtml("Color", "overrides agent color", swatchesHtml(color, true, "Agent default")) +
      shellField +
      (isShell ? '<div class="faint" style="font-size:11.5px">Shell changes apply on the next restart.</div>' : "") +
    "</div>" +
    '<div class="m-foot"><button class="btn ghost" id="cancel">Cancel</button><button class="btn primary" id="ok">Save</button></div>',
    (mt, close) => {
      mt.querySelector("#sname").value = (s.name || "").trim();
      wireSwatches(mt, (c) => { color = c; });
      mt.querySelector("#cancel").onclick = close;
      const input = mt.querySelector("#sname");
      input.focus(); input.select();
      mt.querySelector("#ok").onclick = async () => {
        const body = { name: input.value.trim(), color };
        if (isShell) body.shell = mt.querySelector("#sshell").value;
        const r = await fetch("/api/sessions/" + s.id, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
        if (!r.ok) { toast("Save failed: " + (await r.text()).trim(), "error"); return; }
        close(); await refresh();
        if (cur.view === "terminal") renderDetail();
        toast("Session saved", "ok");
      };
    });
}

main.addEventListener("click", async (e) => {
  const act = e.target.closest("[data-act]");
  if (act) {
    e.stopPropagation();
    const holder = act.closest("[data-id]") || act.closest("[data-pane]");
    const id = holder ? (holder.dataset.id || holder.dataset.pane) : cur.activeId;
    if (act.dataset.act === "settings") {
      const s = sessions.find((x) => x.id === id);
      if (s) sessionSettingsModal(s);
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
  // diff button lives inside the clickable worktree head — check it first so it
  // doesn't also toggle the group's collapse.
  const dbtn0 = e.target.closest("[data-diff]");
  if (dbtn0) {
    const wt = worktrees.find((w) => w.path === dbtn0.dataset.diff);
    if (wt) diffModal(wt);
    return;
  }
  const wtg = e.target.closest("[data-wtgroup]");
  if (wtg) {
    const p = wtg.dataset.wtgroup;
    const rows = sessions.filter((s) => s.worktree === p).length;
    const open = cur.collapsed[p] === undefined ? rows > 0 : !cur.collapsed[p];
    cur.collapsed[p] = open; // collapse if currently open, expand if collapsed
    renderDetail();
    return;
  }
  const projset = e.target.closest("[data-projset]");
  if (projset) {
    e.stopPropagation();
    const p = projects.find((x) => x.id === projset.dataset.projset);
    if (p) projectSettingsModal(p);
    return;
  }
  if (e.target.closest("#projSet")) {
    const p = projects.find((x) => x.id === cur.projectId);
    if (p) projectSettingsModal(p);
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
  const scard = e.target.closest(".s-card");
  if (scard && !e.target.closest("button") && !e.target.closest(".grip")) { openTerminal(scard.dataset.id); return; }
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
  updateStatusbar();
}
setInterval(pollStatus, 3000);

// global settings gear lives in the topbar (outside #main); wire it directly
const globalSetBtn = document.getElementById("globalSet");
if (globalSetBtn) {
  globalSetBtn.innerHTML = icon("gear", 18);
  globalSetBtn.addEventListener("click", globalSettingsModal);
}

Promise.all([loadShell(), loadSettings()]).finally(applyHash);
