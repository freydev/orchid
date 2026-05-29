/* ============ screens ============ */

/* ---------- Projects list ---------- */
function ProjectsList({ projects, onOpen, onNew }) {
  return (
    <div className="wrap">
      <div className="page-head">
        <div>
          <h1>Projects</h1>
          <div className="sub">{projects.length} projects · sessions run in the selected worktree's folder</div>
        </div>
        <button className="btn primary" onClick={onNew}>
          <Icon name="plus" size={15} /> New project
        </button>
      </div>

      <div className="proj-list">
        {projects.map((p) => {
          const sessions = p.worktrees.flatMap((w) => w.sessions);
          return (
            <div className="proj-row" key={p.id} onClick={() => onOpen(p)}>
              <div>
                <div className="name">
                  <Icon name="folder" size={17} />
                  {p.name}
                </div>
                <div className="path">{p.path}</div>
              </div>
              <div className="proj-meta">
                <div className="stat" title="worktrees">
                  <Icon name="layers" size={14} /> {p.worktrees.length}
                </div>
                <div className="stat" title="sessions">
                  <Icon name="term" size={14} /> {sessions.length}
                </div>
                <div className="mini-dots">
                  {sessions.length === 0
                    ? <span className="faint" style={{ fontSize: 12 }}>no sessions</span>
                    : sessions.slice(0, 6).map((s) => <StatusDot key={s.id} status={s.status} />)}
                </div>
                <Icon name="chevR" size={16} />
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

/* ---------- session list row ---------- */
function SessionRow({ session, onOpen, onAction }) {
  return (
    <div className="s-row" onClick={() => onOpen(session)}>
      <AgentBadge type={session.type} lg />
      <div className="meta">
        <div className="sname">{session.name}</div>
        <div className="stype">{AGENT_META[session.type].cmd}{session.task ? ` · ${session.task}` : ""}</div>
      </div>
      <div className="right">
        <span className="up">↑ {fmtUptime(session.uptime)}</span>
        <div className="stat-w"><StatusPill status={session.status} /></div>
        <div className="acts" onClick={(e) => e.stopPropagation()}>
          <button className="btn sm icon ghost" title="Restart" onClick={() => onAction("restart", session)}>
            <Icon name="restart" size={13} />
          </button>
          <button className="btn sm icon ghost danger" title="Kill" onClick={() => onAction("kill", session)}>
            <Icon name="kill" size={12} />
          </button>
        </div>
      </div>
    </div>
  );
}

/* ---------- grid selector ---------- */
function GridSelector({ all, gridSel, onToggle }) {
  const full = gridSel.length >= MAX_GRID;
  return (
    <div className="grid-sel">
      <span className="lbl">Show in grid:</span>
      {all.map(({ s, wt }) => {
        const on = gridSel.includes(s.id);
        const disabled = !on && full;
        return (
          <button key={s.id} className={`gchip ${on ? "on" : ""} ${disabled ? "disabled" : ""}`}
            onClick={() => !disabled && onToggle(s.id)}>
            <StatusDot status={s.status} />
            {s.name}
            <span className="br">{wt.name}</span>
          </button>
        );
      })}
      <span className="cap">{gridSel.length}/{MAX_GRID}</span>
    </div>
  );
}

/* ---------- Project detail ---------- */
function ProjectDetail(props) {
  const { project, onBack, onNewWt, onNewSession, onSend, onSessionAction,
          view, setView, focusId, setFocusId, gridSel, onToggleGrid } = props;

  const all = project.worktrees.flatMap((wt) => wt.sessions.map((s) => ({ s, wt })));
  const total = all.length;

  // keep focus target valid
  useEffect(() => {
    if (view === "focus" && !all.find((x) => x.s.id === focusId)) {
      if (all.length) setFocusId(all[0].s.id);
      else setView("list");
    }
  }, [view, focusId, total]);

  const openFocus = (s) => { setFocusId(s.id); setView("focus"); };
  const focused = all.find((x) => x.s.id === focusId);
  const gridSessions = gridSel.map((id) => all.find((x) => x.s.id === id)).filter(Boolean);

  const [collapsed, setCollapsed] = useState({});
  const toggleGroup = (id) => setCollapsed((c) => ({ ...c, [id]: !c[id] }));

  return (
    <div className={"wrap" + (view === "focus" ? " focus-mode" : "")}>
      <div className="detail-head">
        <div className="crumb">
          <button onClick={onBack}>Projects</button>
          <Icon name="chevR" size={13} />
          <span style={{ color: "var(--text-dim)" }}>{project.name}</span>
        </div>
        <div className="detail-title">
          <Icon name="folder" size={20} />
          <h1>{project.name}</h1>
          <span className="path">{project.path}</span>
        </div>
      </div>

      {/* toolbar */}
      <div className="tools">
        <div className="left">
          <span>{total} {total === 1 ? "session" : "sessions"}</span>
          <span className="faint">·</span>
          <span>{project.worktrees.length} worktrees</span>
        </div>
        <div className="row" style={{ gap: 10 }}>
          <div className="seg">
            <button className={view === "list" ? "active" : ""} onClick={() => setView("list")}><Icon name="list" size={14} /> List</button>
            <button className={view === "grid" ? "active" : ""} onClick={() => setView("grid")}><Icon name="grid" size={14} /> Grid</button>
            <button className={view === "focus" ? "active" : ""} onClick={() => total && openFocus(focused?.s || all[0].s)}><Icon name="term" size={14} /> Terminal</button>
          </div>
          <button className="btn" onClick={onNewWt}><Icon name="branch" size={14} /> Worktree</button>
        </div>
      </div>

      {/* content */}
      {total === 0 ? (
        <div className="empty">
          <div className="big">No sessions in this project yet</div>
          <div style={{ marginBottom: 18 }}>Start an agent or shell — it will run in the selected worktree's folder.</div>
          <button className="btn primary" style={{ margin: "0 auto" }} onClick={() => onNewSession(project.worktrees[0])}>
            <Icon name="plus" size={15} /> Create session
          </button>
        </div>
      ) : view === "list" ? (
        <div>
          {project.worktrees.map((wt) => {
            const isOpen = !collapsed[wt.id];
            return (
              <div className="wt-group" key={wt.id}>
                <div className="wt-group-head clickable" onClick={() => toggleGroup(wt.id)}>
                  <span className={`wt-chev ${isOpen ? "open" : ""}`}><Icon name="chevR" size={14} /></span>
                  <span className="br"><Icon name="branch" size={13} />{wt.name}</span>
                  {wt.dirty > 0 && <span className="git-badge">±{wt.dirty}</span>}
                  <span className="count">· {wt.sessions.length} {wt.sessions.length === 1 ? "session" : "sessions"}</span>
                  <span className="spacer"></span>
                  <button className="btn ghost sm" onClick={(e) => { e.stopPropagation(); onNewSession(wt); }}><Icon name="plus" size={13} /> Session</button>
                </div>
                {isOpen && (
                  wt.sessions.length === 0 ? (
                    <div className="faint" style={{ fontSize: 12.5, padding: "4px 2px 6px" }}>no active sessions in this worktree</div>
                  ) : (
                    <div className="sess-list">
                      {wt.sessions.map((s) => <SessionRow key={s.id} session={s} onOpen={openFocus} onAction={onSessionAction} />)}
                    </div>
                  )
                )}
              </div>
            );
          })}
          <button className="btn ghost sm" onClick={onNewWt}><Icon name="plus" size={13} /> New worktree</button>
        </div>
      ) : view === "grid" ? (
        <div>
          <GridSelector all={all} gridSel={gridSel} onToggle={onToggleGrid} />
          {gridSessions.length === 0 ? (
            <div className="empty"><div className="big">Select sessions for the grid</div><div>Up to 4 terminals at once.</div></div>
          ) : (
            <div className={`term-grid ${gridSessions.length === 1 ? "cols-1" : ""}`}>
              {gridSessions.map(({ s, wt }) => (
                <div className="term-pane" key={s.id} onDoubleClick={() => openFocus(s)}>
                  <Terminal session={s} parent={wt.name} showInput={false} onAction={onSessionAction} onExpand={() => openFocus(s)} compact />
                </div>
              ))}
            </div>
          )}
        </div>
      ) : focused ? (
        <div className="focus">
          <div className="focus-rail">
            {project.worktrees.map((wt) => (
              <div key={wt.id}>
                <div className="rail-head">
                  <Icon name="branch" size={11} /> {wt.name}
                  <span className="spacer"></span>
                  <button className="rail-add" title="New session in this worktree" onClick={() => onNewSession(wt)}>+</button>
                </div>
                {wt.sessions.length === 0 ? (
                  <div className="rail-empty">no sessions</div>
                ) : wt.sessions.map((s) => (
                  <div key={s.id} className={`rail-item ${s.id === focused.s.id ? "active" : ""}`} onClick={() => setFocusId(s.id)}>
                    <AgentBadge type={s.type} />
                    <div className="meta">
                      <div className="sname">{s.name}</div>
                      <div className="stype">{AGENT_META[s.type].cmd} · ↑{fmtUptime(s.uptime)}</div>
                    </div>
                    <StatusDot status={s.status} />
                  </div>
                ))}
              </div>
            ))}
          </div>
          <div className="focus-term">
            <Terminal session={focused.s} parent={focused.wt.name} onSend={onSend} onAction={onSessionAction} />
          </div>
        </div>
      ) : null}
    </div>
  );
}

Object.assign(window, { ProjectsList, ProjectDetail, SessionRow, GridSelector });
