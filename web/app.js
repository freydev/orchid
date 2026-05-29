/* orchid dashboard — projects -> worktrees -> sessions, over the real xterm/WS backend */
const main = document.getElementById("main");

const AGENT = {
  claude: { label: "Claude", badge: "C", cmd: "claude" },
  codex:  { label: "Codex",  badge: "Cx", cmd: "codex" },
  shell:  { label: "Shell",  badge: ">_", cmd: "zsh" },
};
const agentMeta = (a) => AGENT[a] || AGENT.shell;
const statusMeta = (s) => s === "running"
  ? { cls: "ok", label: "Running", pulse: true }
  : { cls: "idle", label: "Idle", pulse: false };

const MONOKAI = {
  background: "#08080b", foreground: "#f8f8f2", cursor: "#a6e22e", cursorAccent: "#08080b",
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
  list:    '<path d="M8 6h12M8 12h12M8 18h12"/><circle cx="4" cy="6" r="1.2"/><circle cx="4" cy="12" r="1.2"/><circle cx="4" cy="18" r="1.2"/>',
  folder:  '<path d="M3 7a2 2 0 0 1 2-2h4l2 2.5h8a2 2 0 0 1 2 2V18a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7z"/>',
  branch:  '<circle cx="6" cy="6" r="2.4"/><circle cx="6" cy="18" r="2.4"/><circle cx="18" cy="8" r="2.4"/><path d="M6 8.4v7.2M8.3 7.2C12 8 14.5 8.5 15.6 9.8M15.6 9.8c.6.7.9 1.6.9 2.7v1.5"/>',
  chevR:   '<path d="M9 6l6 6-6 6"/>',
};
const icon = (n, sz = 16) =>
  `<svg width="${sz}" height="${sz}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">${ICONS[n] || ""}</svg>`;

/* ---------- state ---------- */
let projects = [];
let cur = { projectId: null, view: "list", activeId: null };
let worktrees = [];
let sessions = [];
let term = null, fit = null, ws = null, ro = null;

/* ---------- templates ---------- */
const tplProjRow = _.template(
  '<div class="proj-row" data-pid="<%- p.id %>">' +
    '<div><div class="name">' + icon("folder", 17) + '<%- p.name %></div>' +
      '<div class="path"><%- p.path %></div></div>' +
    '<div class="proj-meta"><span class="faint">open</span>' + icon("chevR", 16) + '</div>' +
  '</div>'
);
const tplSessRow = _.template(
  '<div class="s-row" data-id="<%- s.id %>">' +
    '<div class="agent-badge <%- s.agent %> lg"><%- m.badge %></div>' +
    '<div class="meta"><div class="sname"><%- m.label %></div><div class="stype"><%- m.cmd %></div></div>' +
    '<div class="right"><div class="stat-w"><span class="status-pill <%- st.cls %>"><span class="dot <%- st.cls %><%- st.pulse ? " pulse" : "" %>"></span><%- st.label %></span></div>' +
      '<div class="acts">' +
        '<button class="btn sm icon ghost" data-act="restart" title="Restart">' + icon("restart", 13) + '</button>' +
        '<button class="btn sm icon ghost danger" data-act="kill" title="Kill">' + icon("kill", 12) + '</button>' +
      '</div></div>' +
  '</div>'
);
const tplRailItem = _.template(
  '<div class="rail-item <%- active ? "active" : "" %>" data-id="<%- s.id %>">' +
    '<div class="agent-badge <%- s.agent %>"><%- m.badge %></div>' +
    '<div class="meta"><div class="sname"><%- m.label %></div><div class="stype"><%- m.cmd %></div></div>' +
    '<span class="dot <%- st.cls %><%- st.pulse ? " pulse" : "" %>"></span>' +
  '</div>'
);

const rd = (s) => ({ s, m: agentMeta(s.agent), st: statusMeta(s.status) });
const esc = (x) => _.escape(x == null ? "" : x);

/* ---------- data ---------- */
async function loadProjects() { projects = await (await fetch("/api/projects")).json(); }
async function loadProject(pid) {
  worktrees = await (await fetch("/api/projects/" + pid + "/worktrees")).json();
  const all = await (await fetch("/api/sessions")).json();
  sessions = all.filter((s) => s.project === pid);
}

async function refresh() {
  if (!cur.projectId) { await loadProjects(); renderProjects(); return; }
  await loadProject(cur.projectId);
  if (cur.view === "focus" && cur.activeId) updateRail();
  else renderDetail();
}

/* ---------- projects list ---------- */
function renderProjects() {
  teardownTerm();
  const body = projects.length
    ? '<div class="proj-list">' + projects.map((p) => tplProjRow({ p })).join("") + "</div>"
    : '<div class="empty"><div class="big">No projects yet</div><div style="margin-bottom:18px">Add a git repository to start.</div>' +
      '<button class="btn primary" id="newProj" style="margin:0 auto">' + icon("plus", 15) + " New project</button></div>";
  main.innerHTML =
    '<div class="wrap"><div class="page-head"><div><h1>Projects</h1>' +
      '<div class="sub">' + projects.length + " project" + (projects.length === 1 ? "" : "s") + "</div></div>" +
      '<button class="btn primary" id="newProj">' + icon("plus", 15) + " New project</button></div>" +
      body + "</div>";
}

/* ---------- project detail ---------- */
function renderDetail() {
  teardownTerm();
  const proj = projects.find((p) => p.id === cur.projectId) || { name: "", path: "" };
  const groups = worktrees.map((wt) => {
    const rows = sessions.filter((s) => s.worktree === wt.path);
    const sessHtml = rows.length
      ? '<div class="sess-list">' + rows.map((s) => tplSessRow(rd(s))).join("") + "</div>"
      : '<div class="faint" style="font-size:12.5px;padding:4px 2px 10px">no sessions</div>';
    const rm = wt.is_main ? "" :
      '<button class="btn ghost sm danger" data-rmwt="' + esc(wt.path) + '" data-branch="' + esc(wt.branch) + '" title="Remove worktree">' + icon("kill", 12) + " Remove</button>";
    return '<div class="wt-group">' +
      '<div class="wt-group-head"><span class="br">' + icon("branch", 13) + esc(wt.name) + "</span>" +
        '<span class="count">· ' + rows.length + " session" + (rows.length === 1 ? "" : "s") + "</span><span class=\"spacer\"></span>" +
        '<button class="btn ghost sm" data-newsess="' + esc(wt.path) + '">' + icon("plus", 13) + " Session</button>" + rm +
      "</div>" + sessHtml + "</div>";
  }).join("");
  main.innerHTML =
    '<div class="wrap"><div class="detail-head">' +
      '<div class="crumb"><button id="backProjects">Projects</button>' + icon("chevR", 13) +
        '<span style="color:var(--text-dim)">' + esc(proj.name) + "</span></div>" +
      '<div class="detail-title">' + icon("folder", 20) + "<h1>" + esc(proj.name) + '</h1><span class="path">' + esc(proj.path) + "</span></div></div>" +
      groups +
      '<button class="btn ghost sm" id="newWt" style="margin-top:6px">' + icon("plus", 13) + " New worktree</button>" +
    "</div>";
}

/* ---------- focus ---------- */
function enterFocus(id) {
  cur.view = "focus"; cur.activeId = id;
  main.innerHTML =
    '<div class="wrap focus-mode"><div class="tools"><div class="left">' +
      '<button class="btn ghost sm" id="backDetail">' + icon("list", 14) + " Back</button></div></div>" +
      '<div class="focus"><div class="focus-rail"><div class="rail-head">Sessions<span class="spacer"></span></div><div id="rail"></div></div>' +
      '<div class="focus-term" id="focusTerm"></div></div></div>';
  updateRail();
  attachTerminal(id);
}
function updateRail() {
  const rail = document.getElementById("rail");
  if (!rail) return;
  rail.innerHTML = sessions.map((s) => tplRailItem({ ...rd(s), active: s.id === cur.activeId })).join("");
}

/* ---------- terminal ---------- */
function attachTerminal(id) {
  teardownTerm();
  const host = document.getElementById("focusTerm");
  if (!host) return;
  const s = sessions.find((x) => x.id === id);
  if (!s) { host.innerHTML = ""; return; }
  const m = agentMeta(s.agent), st = statusMeta(s.status);
  host.innerHTML =
    '<div class="term" style="flex:1;min-height:0"><div class="term-head">' +
      '<div class="agent-badge ' + esc(s.agent) + '">' + m.badge + "</div>" +
      '<div class="term-id"><div class="t">' + esc(m.label) + "</div></div>" +
      '<span class="tt">' + m.cmd + '</span><div class="spacer"></div>' +
      '<span class="status-pill ' + st.cls + '"><span class="dot ' + st.cls + (st.pulse ? " pulse" : "") + '"></span>' + st.label + "</span>" +
      '<button class="btn sm icon ghost" data-act="restart" title="Restart">' + icon("restart", 14) + "</button>" +
      '<button class="btn sm icon ghost danger" data-act="kill" title="Kill">' + icon("kill", 13) + "</button>" +
    "</div><div id=\"termBody\" style=\"flex:1;min-height:0;overflow:hidden;background:#08080b\"></div></div>";
  const tb = document.getElementById("termBody");
  if (s.status !== "running") { tb.innerHTML = '<div id="placeholder">stopped — restart to attach</div>'; return; }
  term = new Terminal({ cursorBlink: true, fontSize: 13, scrollback: 5000, theme: MONOKAI });
  fit = new FitAddon.FitAddon(); term.loadAddon(fit); term.open(tb); fit.fit();
  const proto = location.protocol === "https:" ? "wss" : "ws";
  ws = new WebSocket(`${proto}://${location.host}/ws/${id}`); ws.binaryType = "arraybuffer";
  ws.onopen = () => sendResize();
  ws.onmessage = (e) => { if (e.data instanceof ArrayBuffer) term.write(new Uint8Array(e.data)); };
  ws.onclose = () => {};
  term.onData((d) => { if (ws && ws.readyState === WebSocket.OPEN) ws.send(new TextEncoder().encode(d)); });
  ro = new ResizeObserver(() => { if (fit) { fit.fit(); sendResize(); } });
  ro.observe(tb);
}
function teardownTerm() {
  if (ro) { ro.disconnect(); ro = null; }
  if (ws) { ws.onclose = null; ws.close(); ws = null; }
  if (term) { term.dispose(); term = null; }
  fit = null;
}
function sendResize() { if (term && ws && ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows })); }

/* ---------- modals ---------- */
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
        if (!r.ok) { alert("create failed: " + (await r.text())); return; }
        close(); await refresh();
      };
    });
}
function newWorktreeModal() {
  modal(
    '<div class="m-head"><h2>New worktree</h2><p>Creates a new branch off the current HEAD.</p></div>' +
    '<div class="m-body"><div class="field"><label>Branch</label><input class="text mono" id="branch" placeholder="feat/new-thing" /></div></div>' +
    '<div class="m-foot"><button class="btn ghost" id="cancel">Cancel</button><button class="btn primary" id="ok">Create</button></div>',
    (mt, close) => {
      mt.querySelector("#cancel").onclick = close;
      mt.querySelector("#ok").onclick = async () => {
        const r = await fetch("/api/projects/" + cur.projectId + "/worktrees", { method: "POST", headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ branch: mt.querySelector("#branch").value.trim() }) });
        if (!r.ok) { alert("create failed: " + (await r.text())); return; }
        close(); await refresh();
      };
    });
}
function newSessionModal(defaultWt) {
  const types = [{ id: "claude", l: "Claude", b: "C", d: "AI agent" }, { id: "codex", l: "Codex", b: "Cx", d: "AI agent" }, { id: "shell", l: "Shell", b: ">_", d: "zsh shell" }];
  let sel = "claude";
  const opts = worktrees.map((wt) => '<option value="' + esc(wt.path) + '"' + (wt.path === defaultWt ? " selected" : "") + ">" + esc(wt.name) + "</option>").join("");
  const grid = types.map((t) => '<button class="choice" data-type="' + t.id + '"><div class="agent-badge ' + t.id + ' lg">' + t.b + '</div><span class="lbl">' + t.l + '</span><span class="desc">' + t.d + "</span></button>").join("");
  modal(
    '<div class="m-head"><h2>New session</h2></div>' +
    '<div class="m-body"><div class="field"><label>Worktree</label><select class="text mono" id="wt">' + opts + "</select></div>" +
    '<div class="field"><label>Type</label><div class="choice-grid">' + grid + "</div></div></div>" +
    '<div class="m-foot"><button class="btn ghost" id="cancel">Cancel</button><button class="btn primary" id="ok">Start</button></div>',
    (mt, close) => {
      mt.querySelector("#cancel").onclick = close;
      const mark = () => mt.querySelectorAll(".choice").forEach((x) => x.classList.toggle("sel", x.dataset.type === sel));
      mt.querySelectorAll(".choice").forEach((b) => b.onclick = () => { sel = b.dataset.type; mark(); });
      mark();
      mt.querySelector("#ok").onclick = async () => {
        const r = await fetch("/api/sessions", { method: "POST", headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ project: cur.projectId, worktree: mt.querySelector("#wt").value, agent: sel }) });
        if (!r.ok) { alert("create failed: " + (await r.text())); return; }
        const s = await r.json(); close(); await refresh(); enterFocus(s.id);
      };
    });
}

/* ---------- event delegation ---------- */
main.addEventListener("click", async (e) => {
  const act = e.target.closest("[data-act]");
  if (act) {
    e.stopPropagation();
    const holder = act.closest("[data-id]");
    const id = holder ? holder.dataset.id : cur.activeId;
    if (act.dataset.act === "kill") {
      await fetch("/api/sessions/" + id, { method: "DELETE" });
      if (id === cur.activeId) { teardownTerm(); cur.view = "list"; cur.activeId = null; }
      await refresh();
    } else if (act.dataset.act === "restart") {
      const r = await fetch("/api/sessions/" + id + "/restart", { method: "POST" });
      if (!r.ok) { alert("restart failed: " + (await r.text())); return; }
      await refresh();
      if (cur.view === "focus") attachTerminal(id);
    }
    return;
  }
  const rmwt = e.target.closest("[data-rmwt]");
  if (rmwt) {
    const path = rmwt.dataset.rmwt, branch = rmwt.dataset.branch;
    const n = sessions.filter((s) => s.worktree === path).length;
    if (!confirm(`Remove worktree "${branch}"? Kills ${n} session(s), removes the worktree, deletes the branch.`)) return;
    await fetch("/api/projects/" + cur.projectId + "/worktrees", { method: "DELETE", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ path }) });
    await refresh();
    return;
  }
  const ns = e.target.closest("[data-newsess]");
  if (ns) { newSessionModal(ns.dataset.newsess); return; }
  if (e.target.closest("#newProj")) { newProjectModal(); return; }
  if (e.target.closest("#newWt")) { newWorktreeModal(); return; }
  if (e.target.closest("#backProjects")) { cur = { projectId: null, view: "list", activeId: null }; renderProjects(); return; }
  if (e.target.closest("#backDetail")) { cur.view = "list"; cur.activeId = null; renderDetail(); return; }
  const rail = e.target.closest(".rail-item");
  if (rail) { cur.activeId = rail.dataset.id; updateRail(); attachTerminal(cur.activeId); return; }
  const srow = e.target.closest(".s-row");
  if (srow) { enterFocus(srow.dataset.id); return; }
  const prow = e.target.closest(".proj-row");
  if (prow) { cur = { projectId: prow.dataset.pid, view: "list", activeId: null }; refresh(); return; }
});

refresh();
