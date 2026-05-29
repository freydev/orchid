/* orchid dashboard — vanilla JS + underscore templates over the real xterm/WS backend */
const main = document.getElementById("main");

const AGENT = {
  claude: { label: "Claude", badge: "C", cmd: "claude" },
  codex:  { label: "Codex",  badge: "Cx", cmd: "codex" },
  shell:  { label: "Shell",  badge: ">_", cmd: "zsh" },
};
const agentMeta = (a) => AGENT[a] || AGENT.shell;
const statusMeta = (s) =>
  s === "running"
    ? { cls: "ok", label: "Running", pulse: true }
    : { cls: "idle", label: "Idle", pulse: false };

const ICONS = {
  plus:    '<path d="M12 5v14M5 12h14"/>',
  restart: '<path d="M3 12a9 9 0 1 0 3-6.7L3 8"/><path d="M3 3v5h5"/>',
  kill:    '<rect x="6" y="6" width="12" height="12" rx="1.5"/>',
  list:    '<path d="M8 6h12M8 12h12M8 18h12"/><circle cx="4" cy="6" r="1.2"/><circle cx="4" cy="12" r="1.2"/><circle cx="4" cy="18" r="1.2"/>',
};
const icon = (n, sz = 16) =>
  `<svg width="${sz}" height="${sz}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">${ICONS[n] || ""}</svg>`;

/* ---------- state ---------- */
let sessions = [];
let view = "list";        // "list" | "focus"
let activeId = null;
let term = null, fit = null, ws = null, ro = null;

/* ---------- templates ---------- */
const tplRow = _.template(
  '<div class="s-row" data-id="<%- s.id %>">' +
    '<div class="agent-badge <%- s.agent %> lg"><%- m.badge %></div>' +
    '<div class="meta">' +
      '<div class="sname"><%- m.label %></div>' +
      '<div class="stype"><%- m.cmd %><% if (s.cwd) { %> · <%- s.cwd %><% } %></div>' +
    '</div>' +
    '<div class="right">' +
      '<div class="stat-w"><span class="status-pill <%- st.cls %>"><span class="dot <%- st.cls %><%- st.pulse ? " pulse" : "" %>"></span><%- st.label %></span></div>' +
      '<div class="acts">' +
        '<button class="btn sm icon ghost" data-act="restart" title="Restart">' + icon("restart", 13) + '</button>' +
        '<button class="btn sm icon ghost danger" data-act="kill" title="Kill">' + icon("kill", 12) + '</button>' +
      '</div>' +
    '</div>' +
  '</div>'
);

const tplRailItem = _.template(
  '<div class="rail-item <%- active ? "active" : "" %>" data-id="<%- s.id %>">' +
    '<div class="agent-badge <%- s.agent %>"><%- m.badge %></div>' +
    '<div class="meta"><div class="sname"><%- m.label %></div><div class="stype"><%- m.cmd %></div></div>' +
    '<span class="dot <%- st.cls %><%- st.pulse ? " pulse" : "" %>"></span>' +
  '</div>'
);

const tplModal = _.template(
  '<div class="scrim" id="scrim"><div class="modal">' +
    '<div class="m-head"><h2>New session</h2><p>Runs in the chosen directory (blank = home).</p></div>' +
    '<div class="m-body">' +
      '<div class="field"><label>Type</label><div class="choice-grid">' +
        '<% types.forEach(function (t) { %>' +
        '<button class="choice <%- t.id === sel ? "sel" : "" %>" data-type="<%- t.id %>">' +
          '<div class="agent-badge <%- t.id %> lg"><%- t.badge %></div>' +
          '<span class="lbl"><%- t.label %></span><span class="desc"><%- t.desc %></span>' +
        '</button>' +
        '<% }); %>' +
      '</div></div>' +
      '<div class="field"><label>Working directory (optional)</label><input class="text mono" id="cwd" placeholder="~/code/project" /></div>' +
    '</div>' +
    '<div class="m-foot"><button class="btn ghost" id="cancel">Cancel</button><button class="btn primary" id="start">Start</button></div>' +
  '</div></div>'
);

const rd = (s) => ({ s, m: agentMeta(s.agent), st: statusMeta(s.status) });

/* ---------- data ---------- */
async function refresh() {
  sessions = await (await fetch("/api/sessions")).json();
  if (view === "focus" && activeId != null) updateRail();
  else renderList();
}

/* ---------- list view ---------- */
function renderList() {
  view = "list";
  teardownTerm();
  const body = sessions.length
    ? '<div class="sess-list">' + sessions.map((s) => tplRow(rd(s))).join("") + "</div>"
    : '<div class="empty"><div class="big">No sessions yet</div>' +
      '<div style="margin-bottom:18px">Start an agent or shell.</div>' +
      '<button class="btn primary" id="new" style="margin:0 auto">' + icon("plus", 15) + " Create session</button></div>";
  main.innerHTML =
    '<div class="wrap">' +
      '<div class="page-head"><div><h1>Sessions</h1>' +
        '<div class="sub">' + sessions.length + " session" + (sessions.length === 1 ? "" : "s") + "</div></div>" +
        '<button class="btn primary" id="new">' + icon("plus", 15) + " New session</button>" +
      "</div>" +
      body +
    "</div>";
}

/* ---------- focus view ---------- */
function enterFocus(id) {
  view = "focus";
  activeId = id;
  main.innerHTML =
    '<div class="wrap focus-mode">' +
      '<div class="tools"><div class="left">' +
        '<button class="btn ghost sm" id="backList">' + icon("list", 14) + " All sessions</button>" +
      "</div></div>" +
      '<div class="focus">' +
        '<div class="focus-rail"><div class="rail-head">Sessions<span class="spacer"></span>' +
          '<button class="rail-add" id="railAdd">+</button></div><div id="rail"></div></div>' +
        '<div class="focus-term" id="focusTerm"></div>' +
      "</div>" +
    "</div>";
  updateRail();
  attachTerminal(id);
}

function updateRail() {
  const rail = document.getElementById("rail");
  if (!rail) return;
  rail.innerHTML = sessions.map((s) => tplRailItem({ ...rd(s), active: s.id === activeId })).join("");
}

/* ---------- terminal (real xterm/WS) ---------- */
function attachTerminal(id) {
  teardownTerm();
  const host = document.getElementById("focusTerm");
  if (!host) return;
  const s = sessions.find((x) => x.id === id);
  if (!s) { host.innerHTML = ""; return; }
  const m = agentMeta(s.agent), st = statusMeta(s.status);
  host.innerHTML =
    '<div class="term" style="flex:1;min-height:0">' +
      '<div class="term-head">' +
        '<div class="agent-badge ' + s.agent + '">' + m.badge + "</div>" +
        '<div class="term-id"><div class="t">' + _.escape(m.label) + "</div></div>" +
        '<span class="tt">' + m.cmd + '</span><div class="spacer"></div>' +
        '<span class="status-pill ' + st.cls + '"><span class="dot ' + st.cls + (st.pulse ? " pulse" : "") + '"></span>' + st.label + "</span>" +
        '<button class="btn sm icon ghost" data-act="restart" title="Restart">' + icon("restart", 14) + "</button>" +
        '<button class="btn sm icon ghost danger" data-act="kill" title="Kill">' + icon("kill", 13) + "</button>" +
      "</div>" +
      '<div class="term-body" id="termBody"></div>' +
    "</div>";
  const tb = document.getElementById("termBody");
  if (s.status !== "running") {
    tb.innerHTML = '<div id="placeholder">stopped — restart to attach</div>';
    return;
  }
  term = new Terminal({ cursorBlink: true, fontSize: 13, scrollback: 5000, theme: { background: "#08080b" } });
  fit = new FitAddon.FitAddon();
  term.loadAddon(fit);
  term.open(tb);
  fit.fit();
  const proto = location.protocol === "https:" ? "wss" : "ws";
  ws = new WebSocket(`${proto}://${location.host}/ws/${id}`);
  ws.binaryType = "arraybuffer";
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

function sendResize() {
  if (term && ws && ws.readyState === WebSocket.OPEN)
    ws.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }));
}

/* ---------- create modal ---------- */
function openModal() {
  const types = [
    { id: "claude", label: "Claude", badge: "C", desc: "AI agent" },
    { id: "codex", label: "Codex", badge: "Cx", desc: "AI agent" },
    { id: "shell", label: "Shell", badge: ">_", desc: "zsh shell" },
  ];
  let sel = "claude";
  const mount = document.createElement("div");
  mount.innerHTML = tplModal({ types, sel });
  document.body.appendChild(mount);
  const close = () => { mount.remove(); document.removeEventListener("keydown", onEsc); };
  function onEsc(ev) { if (ev.key === "Escape") close(); }
  document.addEventListener("keydown", onEsc);
  mount.querySelector("#scrim").addEventListener("mousedown", (e) => { if (e.target.id === "scrim") close(); });
  mount.querySelector("#cancel").onclick = close;
  mount.querySelectorAll(".choice").forEach((b) => {
    b.onclick = () => { sel = b.dataset.type; mount.querySelectorAll(".choice").forEach((x) => x.classList.toggle("sel", x.dataset.type === sel)); };
  });
  mount.querySelector("#start").onclick = async () => {
    const cwd = mount.querySelector("#cwd").value.trim();
    const r = await fetch("/api/sessions", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ agent: sel, cwd }) });
    if (!r.ok) { alert("create failed: " + (await r.text())); return; }
    const s = await r.json();
    close();
    await refresh();
    enterFocus(s.id);
  };
}

/* ---------- event delegation ---------- */
main.addEventListener("click", async (e) => {
  const actBtn = e.target.closest("[data-act]");
  if (actBtn) {
    e.stopPropagation();
    const holder = actBtn.closest("[data-id]");
    const id = holder ? holder.dataset.id : activeId;
    if (actBtn.dataset.act === "kill") {
      await fetch("/api/sessions/" + id, { method: "DELETE" });
      if (id === activeId) { teardownTerm(); view = "list"; }
      await refresh();
    } else if (actBtn.dataset.act === "restart") {
      const r = await fetch("/api/sessions/" + id + "/restart", { method: "POST" });
      if (!r.ok) { alert("restart failed: " + (await r.text())); return; }
      await refresh();
      if (view === "focus") attachTerminal(id);
    }
    return;
  }
  if (e.target.closest("#new")) { openModal(); return; }
  if (e.target.closest("#railAdd")) { openModal(); return; }
  if (e.target.closest("#backList")) { renderList(); return; }
  const railItem = e.target.closest(".rail-item");
  if (railItem) { activeId = railItem.dataset.id; updateRail(); attachTerminal(activeId); return; }
  const row = e.target.closest(".s-row");
  if (row) { enterFocus(row.dataset.id); return; }
});

refresh();
