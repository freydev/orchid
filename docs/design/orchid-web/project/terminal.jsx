/* ============ terminal ============ */

function TermLine({ ln }) {
  if (ln.prompt) {
    return (
      <div className="ln cmd">
        <span className="p">❯ </span>{ln.text}
      </div>
    );
  }
  return <div className={`ln ${ln.kind || ""}`}>{ln.text || "\u00a0"}</div>;
}

function Terminal({ session, parent, showInput = true, showHead = true, onSend, onAction, onExpand, compact }) {
  const bodyRef = useRef(null);
  const inputRef = useRef(null);
  const [val, setVal] = useState("");

  useEffect(() => {
    const el = bodyRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [session.lines.length, session.id, val]);

  const m = AGENT_META[session.type];
  const working = session.status === "working";
  const isShell = session.type === "shell";

  const submit = (e) => {
    e.preventDefault();
    const v = val.trim();
    if (!v) return;
    onSend && onSend(session, v);
    setVal("");
  };

  return (
    <div className="term" style={{ flex: 1, minHeight: 0 }}>
      {showHead && (
        <div className="term-head">
          <AgentBadge type={session.type} />
          <div className="term-id">
            {parent && <div className="term-parent"><Icon name="branch" size={11} />{parent}</div>}
            <div className="t">{session.name}</div>
          </div>
          <span className="tt">{m.cmd}</span>
          <div className="spacer"></div>
          <StatusPill status={session.status} />
          {onExpand && (
            <button className="btn sm icon ghost" title="Open terminal"
              onClick={onExpand}><Icon name="expand" size={13} /></button>
          )}
          {onAction && (
            <>
              <button className="btn sm icon ghost" title="Перезапустить"
                onClick={() => onAction("restart", session)}><Icon name="restart" size={14} /></button>
              <button className="btn sm icon ghost danger" title="Убить сессию"
                onClick={() => onAction("kill", session)}><Icon name="kill" size={13} /></button>
            </>
          )}
        </div>
      )}
      <div className="term-body" ref={bodyRef}
        onClick={() => inputRef.current && inputRef.current.focus()}>
        {session.lines.map((ln, i) => <TermLine ln={ln} key={i} />)}
        {showInput ? (
          <form className="term-prompt" onSubmit={submit}>
            <span className={`p ${isShell ? "sh" : "ag"}`}>{isShell ? "❯" : "›"}</span>
            <input ref={inputRef} value={val} spellCheck={false} autoComplete="off"
              onChange={(e) => setVal(e.target.value)} autoFocus={!compact} />
          </form>
        ) : working ? (
          <div className="ln"><span className="cursor"></span></div>
        ) : null}
      </div>
    </div>
  );
}

window.Terminal = Terminal;
