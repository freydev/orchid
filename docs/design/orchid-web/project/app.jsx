/* ============ app root ============ */

const TWEAK_DEFAULTS = /*EDITMODE-BEGIN*/{
  "accent": "#7c9cff",
  "density": "comfortable",
  "termFont": 12.5,
  "showPulse": true
}/*EDITMODE-END*/;

// deep clone seed so we can mutate freely
function cloneProjects() {
  return JSON.parse(JSON.stringify(PROJECTS));
}

function shellResponse(cmd) {
  const c = cmd.toLowerCase();
  if (c.startsWith("git status")) return { kind: "dim", text: "On branch main · working tree clean" };
  if (c.startsWith("git ")) return { kind: "ok", text: "  ✓ done" };
  if (c.startsWith("ls")) return { kind: "dim", text: "src/  dist/  package.json  README.md" };
  if (c.startsWith("npm") || c.startsWith("pnpm") || c.startsWith("yarn"))
    return { kind: "ok", text: "  ✓ ok (cached)" };
  if (c.startsWith("cd")) return { kind: "dim", text: "" };
  return { kind: "dim", text: `zsh: command staged — ${cmd}` };
}

function defaultGrid(project) {
  const picks = [];
  // prefer one active session per worktree first
  for (const wt of project.worktrees) {
    if (picks.length >= MAX_GRID) break;
    const s = wt.sessions.find((x) => x.status !== "idle") || wt.sessions[0];
    if (s) picks.push(s.id);
  }
  // fill remaining slots with any other sessions
  if (picks.length < MAX_GRID) {
    for (const wt of project.worktrees) {
      for (const s of wt.sessions) {
        if (picks.length >= MAX_GRID) break;
        if (!picks.includes(s.id)) picks.push(s.id);
      }
    }
  }
  return picks;
}

function App() {
  const [t, setTweak] = useTweaks(TWEAK_DEFAULTS);
  const projectsRef = useRef(cloneProjects());
  const [, force] = useState(0);
  const bump = useCallback(() => force((n) => n + 1), []);

  const [cur, setCur] = useState({ projectId: null, view: "list", focusId: null, gridSel: [] });
  const [modal, setModal] = useState(null); // {kind, ctx}
  const [toasts, pushToast] = useToasts();

  const projects = projectsRef.current;
  const project = projects.find((p) => p.id === cur.projectId) || null;

  /* ---- apply tweaks to CSS vars ---- */
  useEffect(() => {
    const r = document.documentElement.style;
    r.setProperty("--accent", t.accent);
    r.setProperty("--accent-dim", t.accent);
    r.setProperty("--accent-bg", t.accent + "20");
    r.setProperty("--accent-bg-2", t.accent + "2e");
    const compact = t.density === "compact";
    r.setProperty("--row-pad", compact ? "10px" : "14px");
    r.setProperty("--gap", compact ? "11px" : "16px");
    document.documentElement.classList.toggle("no-pulse", !t.showPulse);
  }, [t.accent, t.density, t.showPulse]);

  useEffect(() => {
    document.querySelectorAll(".term-body").forEach((el) => { el.style.fontSize = t.termFont + "px"; });
  });

  /* ---- live streaming into working sessions ---- */
  useEffect(() => {
    const iv = setInterval(() => {
      let changed = false;
      projects.forEach((p) => p.worktrees.forEach((w) => w.sessions.forEach((s) => {
        if (s.status === "idle") return;
        s.uptime += 1;
        if (s.status === "working" && Math.random() < 0.55) {
          const pool = STREAM[s.type];
          const line = pool[Math.floor(Math.random() * pool.length)];
          s.lines.push({ ...line });
          if (s.lines.length > 60) s.lines.splice(0, s.lines.length - 60);
          changed = true;
        }
      })));
      if (changed) bump(); else force((n) => n + 1); // tick uptime each cycle
    }, 2400);
    return () => clearInterval(iv);
  }, []);

  /* ---- navigation ---- */
  const openProject = (p) => setCur({ projectId: p.id, view: "list", focusId: null, gridSel: defaultGrid(p) });
  const backHome = () => setCur({ projectId: null, view: "list", focusId: null, gridSel: [] });
  const setView = (view) => setCur((c) => ({ ...c, view }));
  const setFocusId = (focusId) => setCur((c) => ({ ...c, focusId }));
  const toggleGrid = (id) => setCur((c) => {
    if (c.gridSel.includes(id)) return { ...c, gridSel: c.gridSel.filter((x) => x !== id) };
    if (c.gridSel.length >= MAX_GRID) return c;
    return { ...c, gridSel: [...c.gridSel, id] };
  });

  /* ---- actions ---- */
  const send = (session, text) => {
    const ls = session.lines;
    if (session.type === "shell") {
      ls.push({ prompt: true, text });
      ls.push(shellResponse(text));
    } else {
      ls.push({ kind: "acc", text: "› " + text });
      const wasWaiting = session.status === "wait";
      session.status = "working";
      ls.push({ kind: "dim", text: session.type === "codex" ? "thinking…" : "" });
      setTimeout(() => {
        ls.push({ kind: "agent", text: wasWaiting ? "Got it, continuing." : "On it — starting the task." });
        bump();
      }, 850);
    }
    bump();
  };

  const sessionAction = (action, session) => {
    const wt = project.worktrees.find((w) => w.sessions.includes(session));
    if (!wt) return;
    if (action === "kill") {
      wt.sessions = wt.sessions.filter((s) => s.id !== session.id);
      pushToast("Session killed", session.name);
      setCur((c) => ({
        ...c,
        gridSel: c.gridSel.filter((id) => id !== session.id),
        focusId: c.focusId === session.id ? (wt.sessions[0]?.id || null) : c.focusId,
        view: c.focusId === session.id && project.worktrees.every((w) => w.sessions.length === 0) ? "list" : c.view,
      }));
      bump();
    } else if (action === "restart") {
      session.lines = [
        { kind: "dim", text: `restarting ${AGENT_META[session.type].cmd}…` },
        { kind: "ok", text: "  ✓ session ready" },
      ];
      session.status = session.type === "shell" ? "idle" : "working";
      session.uptime = 0;
      pushToast("Session restarted", session.name);
      bump();
    }
  };

  /* ---- modal submit ---- */
  const createProject = (name, path) => {
    const p = { id: uid("p"), name, path: path || `~/code/${name}`,
      worktrees: [{ id: uid("wt"), name: "main", branch: "main", dirty: 0, sessions: [] }] };
    projects.unshift(p);
    setModal(null);
    openProject(p);
    pushToast("Project created", name);
  };

  const createSession = (wt, type, name) => {
    const starter = {
      claude: [{ kind: "dim", text: "claude · session started" }, { kind: "agent", text: "Ready to help. What are we working on?" }],
      codex:  [{ kind: "dim", text: "codex · session started" }, { kind: "agent", text: "Project context loaded. Waiting for a task." }],
      shell:  [{ kind: "dim", text: `zsh · ${project.path}${wt.name !== "main" ? "/" + wt.name : ""}` }],
    }[type];
    const s = mkSession(type, name || (type === "shell" ? "shell" : "new task"),
      type === "shell" ? "idle" : "working", 0, starter);
    wt.sessions.push(s);
    setModal(null);
    setCur((c) => ({ ...c, view: "focus", focusId: s.id, gridSel: c.gridSel.length < MAX_GRID ? [...c.gridSel, s.id] : c.gridSel }));
    pushToast("Session started", `${AGENT_META[type].cmd} · ${wt.name}`);
  };

  const createWorktree = (name, branch) => {
    const wt = { id: uid("wt"), name, branch: branch || name, dirty: 0, sessions: [] };
    project.worktrees.push(wt);
    setModal(null);
    setCur((c) => ({ ...c, view: "list" }));
    pushToast("Worktree created", branch || name);
  };

  return (
    <div className="app">
      <div className="topbar">
        <div className="brand">
          <div className="mark">o</div>
          orchid
          <span className="ver">tmux · v0.4</span>
        </div>
        <div className="spacer"></div>
        <span className="kbd">codex</span>
        <span className="kbd">claude</span>
      </div>

      <div className="main">
        {!project ? (
          <ProjectsList projects={projects} onOpen={openProject} onNew={() => setModal({ kind: "project" })} />
        ) : (
          <ProjectDetail
            project={project}
            view={cur.view}
            focusId={cur.focusId}
            gridSel={cur.gridSel}
            setView={setView}
            setFocusId={setFocusId}
            onToggleGrid={toggleGrid}
            onBack={backHome}
            onNewWt={() => setModal({ kind: "worktree" })}
            onNewSession={(wt) => setModal({ kind: "session", wt })}
            onSend={send}
            onSessionAction={sessionAction}
          />
        )}
      </div>

      {modal?.kind === "project" && <NewProjectModal onClose={() => setModal(null)} onCreate={createProject} />}
      {modal?.kind === "session" && <NewSessionModal wt={modal.wt} project={project} onClose={() => setModal(null)} onCreate={createSession} />}
      {modal?.kind === "worktree" && <NewWorktreeModal project={project} onClose={() => setModal(null)} onCreate={createWorktree} />}

      <Toasts toasts={toasts} />

      <TweaksPanel>
        <TweakSection label="Appearance" />
        <TweakColor label="Accent" value={t.accent}
          options={["#7c9cff", "#5fd39a", "#e9a23b", "#c98bdb", "#e57373"]}
          onChange={(v) => setTweak("accent", v)} />
        <TweakRadio label="Density" value={t.density}
          options={["compact", "comfortable"]} onChange={(v) => setTweak("density", v)} />
        <TweakSection label="Terminal" />
        <TweakSlider label="Font size" value={t.termFont} min={11} max={16} step={0.5} unit="px"
          onChange={(v) => setTweak("termFont", v)} />
        <TweakToggle label="Status pulse" value={t.showPulse} onChange={(v) => setTweak("showPulse", v)} />
      </TweaksPanel>
    </div>
  );
}

/* ---------- modals ---------- */
function NewProjectModal({ onClose, onCreate }) {
  const [name, setName] = useState("");
  const [path, setPath] = useState("");
  return (
    <Modal title="New project" desc="The folder where sessions and worktrees will run."
      onClose={onClose}
      footer={<>
        <button className="btn ghost" onClick={onClose}>Cancel</button>
        <button className="btn primary" disabled={!name.trim()} onClick={() => name.trim() && onCreate(name.trim(), path.trim())}>Create</button>
      </>}>
      <div className="field">
        <label>Name</label>
        <input className="text" autoFocus value={name} onChange={(e) => setName(e.target.value)} placeholder="my-service" />
      </div>
      <div className="field">
        <label>Path</label>
        <input className="text mono" value={path} onChange={(e) => setPath(e.target.value)} placeholder={name ? `~/code/${name}` : "~/code/my-service"} />
      </div>
    </Modal>
  );
}

function NewSessionModal({ wt, project, onClose, onCreate }) {
  const [type, setType] = useState("claude");
  const [name, setName] = useState("");
  const [wtId, setWtId] = useState(wt.id);
  const target = project.worktrees.find((w) => w.id === wtId) || project.worktrees[0];
  const types = [
    { id: "claude", desc: "AI agent" },
    { id: "codex", desc: "AI agent" },
    { id: "shell", desc: "zsh shell" },
  ];
  const dir = `${project.path}${target.name !== "main" ? "/" + target.name : ""}`;
  return (
    <Modal title="New session" desc={<>Runs in <span className="mono" style={{ color: "var(--text-dim)" }}>{dir}</span></>}
      onClose={onClose}
      footer={<>
        <button className="btn ghost" onClick={onClose}>Cancel</button>
        <button className="btn primary" onClick={() => onCreate(target, type, name.trim())}>Start</button>
      </>}>
      <div className="field">
        <label>Worktree</label>
        <select className="text mono" value={wtId} onChange={(e) => setWtId(e.target.value)}>
          {project.worktrees.map((w) => (
            <option key={w.id} value={w.id}>{w.name}{w.dirty > 0 ? `  (±${w.dirty})` : ""}</option>
          ))}
        </select>
      </div>
      <div className="field">
        <label>Type</label>
        <div className="choice-grid">
          {types.map((x) => (
            <button key={x.id} className={`choice ${type === x.id ? "sel" : ""}`} onClick={() => setType(x.id)}>
              <AgentBadge type={x.id} lg />
              <span className="lbl">{AGENT_META[x.id].label}</span>
              <span className="desc">{x.desc}</span>
            </button>
          ))}
        </div>
      </div>
      <div className="field">
        <label>{type === "shell" ? "Name (optional)" : "Task / name"}</label>
        <input className="text" autoFocus value={name} onChange={(e) => setName(e.target.value)}
          placeholder={type === "shell" ? "shell" : "e.g. fix auth bug"} />
      </div>
    </Modal>
  );
}

function NewWorktreeModal({ project, onClose, onCreate }) {
  const [name, setName] = useState("");
  const [branch, setBranch] = useState("");
  return (
    <Modal title="New worktree" desc={<>A separate working copy of <span className="mono" style={{ color: "var(--text-dim)" }}>{project.name}</span> on its own branch.</>}
      onClose={onClose}
      footer={<>
        <button className="btn ghost" onClick={onClose}>Cancel</button>
        <button className="btn primary" disabled={!branch.trim()} onClick={() => branch.trim() && onCreate(name.trim() || branch.trim(), branch.trim())}>Create</button>
      </>}>
      <div className="field">
        <label>Branch</label>
        <input className="text mono" autoFocus value={branch} onChange={(e) => setBranch(e.target.value)} placeholder="feat/new-thing" />
      </div>
      <div className="field">
        <label>Tab label (optional)</label>
        <input className="text" value={name} onChange={(e) => setName(e.target.value)} placeholder={branch || "feat/new-thing"} />
      </div>
    </Modal>
  );
}

Object.assign(window, { App, NewProjectModal, NewSessionModal, NewWorktreeModal });

ReactDOM.createRoot(document.getElementById("root")).render(<App />);
