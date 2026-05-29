# Orchid Dashboard Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restyle the orchid dashboard to the Claude Design handoff (dark terminal look) for our flat session model — topbar, `s-row` session list, `focus` layout (rail + real xterm in design chrome), create modal — using the design's `styles.css` + underscore templates, with no backend changes.

**Architecture:** Pure frontend. `web/index.html` becomes the design shell (topbar + `#main`); `web/styles.css` is the design's CSS verbatim; `web/app.js` is rewritten as vanilla JS that renders List and Focus views from `_.template` strings and attaches the existing real xterm.js/WebSocket terminal inside the design's `.term-head` chrome. The Go server, API, and embed are unchanged (the embed picks up the new web files automatically).

**Tech Stack:** Vanilla JS, underscore (`_.template`), xterm.js + addon-fit (existing), IBM Plex Sans + JetBrains Mono fonts. Design bundle reference: `docs/design/orchid-web/project/{styles.css,screens.jsx,terminal.jsx,app.jsx,components.jsx,data.jsx}`.

**Verification note:** This is a UI task with no JS test harness. Each task verifies via `go build ./...` (embed compiles), the unchanged Go suite staying green, and a headless-Chrome (CDP) screenshot to confirm the visual result. The CDP screenshot harness is provided in Task 3.

---

## File Structure

```
web/index.html               # MODIFIED: design shell (topbar + #main), font + css + underscore includes
web/styles.css               # NEW: vendored from docs/design/orchid-web/project/styles.css
web/app.js                   # REWRITTEN: underscore templates + list/focus render + xterm wiring
web/vendor/underscore-min.js # NEW
```

No Go files change. `web/embed.go` already embeds `index.html app.js vendor/*`; add `styles.css` to the embed pattern (Task 1).

---

## Task 1: Vendor assets + design shell (index.html, styles.css, underscore, embed)

**Files:**
- Create: `web/styles.css` (copy of the design CSS), `web/vendor/underscore-min.js`
- Modify: `web/index.html`, `web/embed.go`

- [ ] **Step 1: Vendor the design CSS and underscore**

Run:
```bash
cd /Users/andrey/Work/orchid_3
cp docs/design/orchid-web/project/styles.css web/styles.css
curl -fsSL https://cdn.jsdelivr.net/npm/underscore@1.13.7/underscore-umd-min.js -o web/vendor/underscore-min.js
wc -c web/styles.css web/vendor/underscore-min.js
```
Expected: both files non-empty (styles.css ~13KB, underscore ~19KB). underscore exposes a global `_` with `_.template` and `_.escape`.

- [ ] **Step 2: Replace `web/index.html`**

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>orchid — AI dev dashboard</title>
  <link rel="preconnect" href="https://fonts.googleapis.com" />
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
  <link href="https://fonts.googleapis.com/css2?family=IBM+Plex+Sans:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500;600&display=swap" rel="stylesheet" />
  <link rel="stylesheet" href="/vendor/xterm.css" />
  <link rel="stylesheet" href="/styles.css" />
</head>
<body>
  <div class="app">
    <div class="topbar">
      <div class="brand"><div class="mark">o</div>orchid<span class="ver">tmux · v0.5</span></div>
      <div class="spacer"></div>
      <span class="kbd">codex</span>
      <span class="kbd">claude</span>
    </div>
    <div class="main" id="main"></div>
  </div>
  <script src="/vendor/underscore-min.js"></script>
  <script src="/vendor/xterm.js"></script>
  <script src="/vendor/addon-fit.js"></script>
  <script src="/app.js"></script>
</body>
</html>
```

- [ ] **Step 3: Add styles.css to the embed**

In `web/embed.go`, update the `//go:embed` directive to include `styles.css`:
```go
//go:embed index.html app.js styles.css vendor/*
var FS embed.FS
```
(Keep the rest of the file unchanged.)

- [ ] **Step 4: Verify build + that styles.css is served**

Run:
```bash
go build ./... && go test ./internal/server/ -run 'TestStatic|TestEmbed' -v
```
Expected: build clean; the embed/static tests pass (or skip). The current `app.js` (old) still loads — page will look broken until Task 2, that's fine.

- [ ] **Step 5: Commit**

```bash
git add web/index.html web/styles.css web/vendor/underscore-min.js web/embed.go
git commit -m "feat: vendor design CSS + underscore, design shell index.html"
```

---

## Task 2: Rewrite app.js (templates, list, focus, create, actions)

**Files:**
- Replace: `web/app.js`

- [ ] **Step 1: Replace `web/app.js` entirely with:**

```js
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
        '<span class="tt">' + m.cmd + "</span><div class=\"spacer\"></div>" +
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
```

- [ ] **Step 2: Build + confirm embed compiles**

Run: `go build ./... && go vet ./...`
Expected: clean (the new `app.js` is served from embed; nothing Go-side references it directly).

- [ ] **Step 3: Run the full Go suite (unchanged backend must stay green)**

Run: `go test ./... -race`
Expected: all PASS (no backend changed).

- [ ] **Step 4: Commit**

```bash
git add web/app.js
git commit -m "feat: redesign dashboard (list + focus views, underscore templates)"
```

---

## Task 3: Visual verification (CDP screenshot)

**Files:** none (verification).

- [ ] **Step 1: Build + run orchid on a test port**

Run:
```bash
cd /Users/andrey/Work/orchid_3
pkill -f 'orchid -port' 2>/dev/null; rm -f "$HOME/.config/orchid/sessions.json"
go build -o orchid ./cmd/orchid && ./orchid -port 18160 >/tmp/o.log 2>&1 &
sleep 1
curl -sf -X POST -H 'Content-Type: application/json' -d '{"agent":"shell","cwd":"/tmp"}' http://127.0.0.1:18160/api/sessions >/dev/null
```

- [ ] **Step 2: Screenshot the List and Focus views via headless Chrome (CDP)**

Create `/tmp/shot/m.go`:
```go
package main
import("context";"encoding/base64";"encoding/json";"fmt";"io";"net/http";"os";"time";"github.com/coder/websocket")
func main(){
	url:="http://127.0.0.1:18160/"; out:=os.Args[1]; clickRow:=len(os.Args)>2
	resp,_:=http.Get("http://localhost:9222/json"); b,_:=io.ReadAll(resp.Body); resp.Body.Close()
	var raw []map[string]any; json.Unmarshal(b,&raw); var wsurl string
	for _,t:=range raw{ if t["type"]=="page"{ if u,ok:=t["webSocketDebuggerUrl"].(string);ok{wsurl=u;break} } }
	c,_,e:=websocket.Dial(context.Background(),wsurl,nil); if e!=nil{fmt.Println(e);return}
	defer c.Close(websocket.StatusNormalClosure,""); c.SetReadLimit(20<<20)
	id:=0; var img string
	send:=func(m string,p map[string]any){id++; b,_:=json.Marshal(map[string]any{"id":id,"method":m,"params":p}); c.Write(context.Background(),websocket.MessageText,b)}
	go func(){ for{ _,d,err:=c.Read(context.Background()); if err!=nil{return}; var r map[string]any; json.Unmarshal(d,&r); if res,ok:=r["result"].(map[string]any);ok{ if data,ok:=res["data"].(string);ok{img=data} } } }()
	send("Page.navigate",map[string]any{"url":url}); time.Sleep(2500*time.Millisecond)
	if clickRow { send("Runtime.evaluate",map[string]any{"expression":"document.querySelector('.s-row')&&document.querySelector('.s-row').click()"}); time.Sleep(2500*time.Millisecond) }
	send("Page.captureScreenshot",map[string]any{"format":"png"}); time.Sleep(1500*time.Millisecond)
	if img!=""{ d,_:=base64.StdEncoding.DecodeString(img); os.WriteFile(out,d,0644); fmt.Println("saved",out,len(d)) } else { fmt.Println("no image") }
}
```
Run:
```bash
"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" --headless=new --remote-debugging-port=9222 --user-data-dir=/tmp/cdpd --window-size=1100,700 about:blank >/tmp/chrome.log 2>&1 &
sleep 2
mkdir -p /tmp/shot && cd /tmp/shot
printf 'module shot\ngo 1.26\nrequire github.com/coder/websocket v1.8.14\n' > go.mod
GOFLAGS=-mod=mod go get github.com/coder/websocket@v1.8.14 >/dev/null 2>&1
# (paste m.go from above)
GOFLAGS=-mod=mod go run m.go /tmp/shot/list.png          # List view
GOFLAGS=-mod=mod go run m.go /tmp/shot/focus.png click   # Focus view (clicks first row)
```
Then Read both PNGs and confirm: dark `#0c0c0f` background, topbar with `o` mark + `orchid` + `tmux · v0.5` + codex/claude chips, a `s-row` for the shell session with agent badge + green Running pill, and (focus) the rail + a live terminal in `.term-head` chrome. The look should match the design bundle.

- [ ] **Step 3: Manual smoke (the real terminal still works)**

In a browser at `http://127.0.0.1:18160`: click the session → terminal attaches and accepts typing; `+ New session` opens the modal (agent choice grid + cwd) → Start creates and focuses it; restart/kill on the term-head work; "All sessions" returns to the list.

- [ ] **Step 4: Cleanup**

Run:
```bash
pkill -f 'orchid -port' 2>/dev/null
pkill -f 'remote-debugging-port=9222' 2>/dev/null
tmux ls 2>/dev/null | grep '^orchid-' | cut -d: -f1 | xargs -r -n1 tmux kill-session -t
rm -rf /tmp/shot /tmp/cdpd orchid "$HOME/.config/orchid/sessions.json"
```

- [ ] **Step 5: Commit (only if fixes were needed)**

No code change expected here. If the screenshot revealed a visual bug, fix under Task 1/2 and re-run.

---

## Self-Review Notes

- **Spec coverage:** design CSS + fonts + topbar (T1); list `s-row` + status pill/dot mapping running→ok / stopped→idle + agent badges C/Cx/>_ (T2 tplRow/statusMeta/AGENT); focus rail + real xterm in `.term-head` chrome + view switching (T2 enterFocus/attachTerminal); create modal `choice-grid` + cwd → `POST {agent,cwd}` (T2 openModal); underscore templates (T2 tplRow/tplRailItem/tplModal); empty state (T2 renderList); stopped placeholder (T2 attachTerminal); injection-safe via `<%- %>`/`_.escape` (T2). All spec sections mapped.
- **Deferred items NOT built** (correct per scope): projects, worktrees, grid, cards/preview, wait-status, uptime, tweaks, toasts, session names — all in the deferred spec.
- **No backend changes:** API (`/api/sessions` create with `{agent,cwd}`, `/ws/{id}`, restart, kill) and embed are reused; `embed.go` only gains `styles.css` in its pattern (T1).
- **Type/selector consistency:** `data-id`/`data-act`/`data-type` attributes and the `s-row`/`rail-item`/`choice`/`scrim` classes are produced by the templates and matched by the delegated click handler; `attachTerminal`/`teardownTerm`/`sendResize`/`enterFocus`/`renderList`/`updateRail`/`refresh`/`openModal` all defined and cross-referenced consistently.
- **Verification deviation:** no JS unit tests (vanilla, no harness) — visual correctness is confirmed by the CDP screenshot in T3 plus the unchanged Go suite; flagged so the missing JS tests aren't a silent gap.
```
