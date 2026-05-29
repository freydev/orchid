const listEl = document.getElementById("list");
const newBtn = document.getElementById("new");
const agentSel = document.getElementById("agent");
const cwdInput = document.getElementById("cwd");
const termEl = document.getElementById("term");

let term = null;
let fit = null;
let ws = null;
let activeId = null;

async function fetchSessions() {
  const r = await fetch("/api/sessions");
  return r.json();
}

async function createSession() {
  const r = await fetch("/api/sessions", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ agent: agentSel.value, cwd: cwdInput.value }),
  });
  if (!r.ok) {
    alert("create failed: " + (await r.text()));
    return;
  }
  const s = await r.json();
  cwdInput.value = "";
  await refresh();
  selectSession(s.id);
}

async function killSession(id) {
  await fetch("/api/sessions/" + id, { method: "DELETE" });
  if (id === activeId) teardown();
  await refresh();
}

async function restartSession(id) {
  const r = await fetch("/api/sessions/" + id + "/restart", { method: "POST" });
  if (!r.ok) {
    alert("restart failed: " + (await r.text()));
    return;
  }
  await refresh();
  selectSession(id);
}

function showPlaceholder(text) {
  teardown();
  termEl.innerHTML = '<div id="placeholder">' + text + "</div>";
}

async function refresh() {
  const sessions = await fetchSessions();
  listEl.innerHTML = "";
  for (const s of sessions) {
    const li = document.createElement("li");
    if (s.id === activeId) li.classList.add("active");
    if (s.status !== "running") li.classList.add("dead");

    const meta = document.createElement("div");
    meta.className = "meta";
    const dot = '<span class="dot ' + s.status + '"></span>';
    meta.innerHTML = dot + s.agent + '<span class="sub">' + s.id + "</span>";
    meta.onclick = () => onSelect(s);

    const btns = document.createElement("div");
    btns.className = "btns";
    if (s.status !== "running") {
      const rb = document.createElement("button");
      rb.className = "btn restart";
      rb.textContent = "▶";
      rb.title = "restart";
      rb.onclick = (e) => { e.stopPropagation(); restartSession(s.id); };
      btns.appendChild(rb);
    }
    const kb = document.createElement("button");
    kb.className = "btn kill";
    kb.textContent = "✕";
    kb.title = "delete";
    kb.onclick = (e) => { e.stopPropagation(); killSession(s.id); };
    btns.appendChild(kb);

    li.appendChild(meta);
    li.appendChild(btns);
    listEl.appendChild(li);
  }
}

function onSelect(s) {
  if (s.status !== "running") {
    activeId = s.id;
    showPlaceholder("stopped — restart to attach");
    refresh();
    return;
  }
  selectSession(s.id);
}

function teardown() {
  if (ws) { ws.onclose = null; ws.close(); ws = null; }
  if (term) { term.dispose(); term = null; }
  fit = null;
}

function selectSession(id) {
  teardown();
  termEl.innerHTML = "";
  activeId = id;

  term = new Terminal({ cursorBlink: true, fontSize: 14 });
  fit = new FitAddon.FitAddon();
  term.loadAddon(fit);
  term.open(termEl);
  fit.fit();

  const proto = location.protocol === "https:" ? "wss" : "ws";
  ws = new WebSocket(`${proto}://${location.host}/ws/${id}`);
  ws.binaryType = "arraybuffer";

  ws.onopen = () => sendResize();
  ws.onmessage = (ev) => {
    if (ev.data instanceof ArrayBuffer) term.write(new Uint8Array(ev.data));
  };
  ws.onclose = () => { /* session ended or detached */ };

  term.onData((d) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(new TextEncoder().encode(d));
    }
  });

  const ro = new ResizeObserver(() => { if (fit) { fit.fit(); sendResize(); } });
  ro.observe(termEl);

  refresh();
}

function sendResize() {
  if (!term || !ws || ws.readyState !== WebSocket.OPEN) return;
  ws.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }));
}

newBtn.onclick = createSession;
refresh();
