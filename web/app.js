const listEl = document.getElementById("list");
const newBtn = document.getElementById("new");

let term = null;
let fit = null;
let ws = null;
let activeId = null;

async function fetchSessions() {
  const r = await fetch("/api/sessions");
  return r.json();
}

async function createSession() {
  const r = await fetch("/api/sessions", { method: "POST" });
  const s = await r.json();
  await refresh();
  selectSession(s.id);
}

async function killSession(id) {
  await fetch("/api/sessions/" + id, { method: "DELETE" });
  if (id === activeId) teardown();
  await refresh();
}

async function refresh() {
  const sessions = await fetchSessions();
  listEl.innerHTML = "";
  for (const s of sessions) {
    const li = document.createElement("li");
    li.textContent = s.id;
    if (s.id === activeId) li.classList.add("active");
    if (!s.alive) li.classList.add("dead");
    li.onclick = () => selectSession(s.id);
    const kill = document.createElement("button");
    kill.className = "kill";
    kill.textContent = "✕";
    kill.onclick = (e) => { e.stopPropagation(); killSession(s.id); };
    li.appendChild(kill);
    listEl.appendChild(li);
  }
}

function teardown() {
  if (ws) { ws.onclose = null; ws.close(); ws = null; }
  if (term) { term.dispose(); term = null; }
  fit = null;
  activeId = null;
}

function selectSession(id) {
  if (id === activeId) return;
  teardown();
  activeId = id;

  term = new Terminal({ cursorBlink: true, fontSize: 14 });
  fit = new FitAddon.FitAddon();
  term.loadAddon(fit);
  term.open(document.getElementById("term"));
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
  ro.observe(document.getElementById("term"));

  refresh();
}

function sendResize() {
  if (!term || !ws || ws.readyState !== WebSocket.OPEN) return;
  ws.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }));
}

newBtn.onclick = createSession;
refresh();
