/* ============ shared components ============ */
const { useState, useEffect, useRef, useCallback } = React;

/* --- minimal stroke icons --- */
function Icon({ name, size = 16 }) {
  const p = { width: size, height: size, viewBox: "0 0 24 24", fill: "none",
    stroke: "currentColor", strokeWidth: 1.8, strokeLinecap: "round", strokeLinejoin: "round" };
  const paths = {
    plus:    <><path d="M12 5v14M5 12h14"/></>,
    chevR:   <><path d="M9 6l6 6-6 6"/></>,
    chevL:   <><path d="M15 6l-6 6 6 6"/></>,
    back:    <><path d="M19 12H5M11 18l-6-6 6-6"/></>,
    cards:   <><rect x="3" y="4" width="8" height="7" rx="1.5"/><rect x="13" y="4" width="8" height="7" rx="1.5"/><rect x="3" y="14" width="8" height="6" rx="1.5"/><rect x="13" y="14" width="8" height="6" rx="1.5"/></>,
    grid:    <><rect x="3.5" y="3.5" width="7.5" height="7.5" rx="1.5"/><rect x="13" y="3.5" width="7.5" height="7.5" rx="1.5"/><rect x="3.5" y="13" width="7.5" height="7.5" rx="1.5"/><rect x="13" y="13" width="7.5" height="7.5" rx="1.5"/></>,
    term:    <><rect x="3" y="4" width="18" height="16" rx="2"/><path d="M7 9l3 3-3 3M13 15h4"/></>,
    layers:  <><path d="M12 3l8 4.5-8 4.5-8-4.5L12 3z"/><path d="M4 12l8 4.5 8-4.5"/></>,
    x:       <><path d="M6 6l12 12M18 6L6 18"/></>,
    kill:    <><rect x="6" y="6" width="12" height="12" rx="1.5"/></>,
    restart: <><path d="M3 12a9 9 0 1 0 3-6.7L3 8"/><path d="M3 3v5h5"/></>,
    folder:  <><path d="M3 7a2 2 0 0 1 2-2h4l2 2.5h8a2 2 0 0 1 2 2V18a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7z"/></>,
    branch:  <><circle cx="6" cy="6" r="2.4"/><circle cx="6" cy="18" r="2.4"/><circle cx="18" cy="8" r="2.4"/><path d="M6 8.4v7.2M8.3 7.2C12 8 14.5 8.5 15.6 9.8M15.6 9.8c.6.7.9 1.6.9 2.7v1.5"/></>,
    dots:    <><circle cx="5" cy="12" r="1.4"/><circle cx="12" cy="12" r="1.4"/><circle cx="19" cy="12" r="1.4"/></>,
    check:   <><path d="M5 12l4.5 4.5L19 7"/></>,
    list:    <><path d="M8 6h12M8 12h12M8 18h12"/><circle cx="4" cy="6" r="1.2"/><circle cx="4" cy="12" r="1.2"/><circle cx="4" cy="18" r="1.2"/></>,
    expand:  <><path d="M8 4H4v4"/><path d="M16 4h4v4"/><path d="M16 20h4v-4"/><path d="M8 20H4v-4"/></>,
  };
  return <svg {...p}>{paths[name]}</svg>;
}

function AgentBadge({ type, lg }) {
  const m = AGENT_META[type] || AGENT_META.shell;
  return <div className={`agent-badge ${type} ${lg ? "lg" : ""}`}>{m.badge}</div>;
}

function StatusDot({ status }) {
  const s = STATUS_META[status] || STATUS_META.idle;
  return <span className={`dot ${s.cls} ${s.pulse ? "pulse" : ""}`}></span>;
}

function StatusPill({ status }) {
  const s = STATUS_META[status] || STATUS_META.idle;
  return (
    <span className={`status-pill ${s.cls}`}>
      <span className={`dot ${s.cls} ${s.pulse ? "pulse" : ""}`}></span>
      {s.label}
    </span>
  );
}

/* --- toast host --- */
function useToasts() {
  const [toasts, setToasts] = useState([]);
  const push = useCallback((text, mono) => {
    const id = uid("t");
    setToasts((t) => [...t, { id, text, mono }]);
    setTimeout(() => setToasts((t) => t.filter((x) => x.id !== id)), 2600);
  }, []);
  return [toasts, push];
}
function Toasts({ toasts }) {
  return (
    <div className="toasts">
      {toasts.map((t) => (
        <div className="toast" key={t.id}>
          <Icon name="check" size={14} />
          <span>{t.text}</span>
          {t.mono && <span className="mono">{t.mono}</span>}
        </div>
      ))}
    </div>
  );
}

/* --- modal shell --- */
function Modal({ title, desc, children, onClose, footer }) {
  useEffect(() => {
    const h = (e) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", h);
    return () => window.removeEventListener("keydown", h);
  }, [onClose]);
  return (
    <div className="scrim" onMouseDown={onClose}>
      <div className="modal" onMouseDown={(e) => e.stopPropagation()}>
        <div className="m-head">
          <h2>{title}</h2>
          {desc && <p>{desc}</p>}
        </div>
        <div className="m-body">{children}</div>
        {footer && <div className="m-foot">{footer}</div>}
      </div>
    </div>
  );
}

Object.assign(window, { Icon, AgentBadge, StatusDot, StatusPill, useToasts, Toasts, Modal });
