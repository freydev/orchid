package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"orchid/internal/agent"
	"orchid/internal/store"
	"orchid/internal/tmux"
)

const idleAfter = 3 * time.Second

// Info is a serializable session summary.
type Info struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Agent    string `json:"agent"`
	Cwd      string `json:"cwd"`
	Project  string `json:"project"`
	Worktree string `json:"worktree"`
	Status    string `json:"status"` // "running" | "requires_action" | "idle" | "stopped"
	Alive     bool   `json:"alive"`
	GridOrder int    `json:"grid_order"`
	Color     string `json:"color,omitempty"`   // accent color id, overrides agent color
	Shell     string `json:"shell,omitempty"`   // shell choice for "shell" sessions
	Command   string `json:"command,omitempty"` // preset command running in this shell session
}

// Manager owns sessions (running, with a live PTY) and records (all, incl. stopped).
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session
	records  map[string]store.Record
	signals  map[string]*signalState
	selfURL  string
	binPath  string
	path     string
	closing  bool
	// shellFor resolves the launchable shell path for a "shell" session given its
	// project and the session's own shell choice. Set via SetShellResolver.
	shellFor func(projectID, sessionShell string) string
}

// signalState is the latest agent-reported activity for a live session.
type signalState struct {
	state string // "running" | "requires_action" | "idle" | "done"
}

func NewManager(path string) *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
		records:  make(map[string]store.Record),
		signals:  make(map[string]*signalState),
		path:     path,
	}
}

// SetEnv configures the values injected into each session's tmux environment so
// the agent's hooks/MCP server can call back. selfURL is the orchid base URL
// (e.g. http://127.0.0.1:8081); binPath is the absolute path to this binary.
func (m *Manager) SetEnv(selfURL, binPath string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.selfURL = selfURL
	m.binPath = binPath
}

// SetShellResolver installs the function that maps a (project, session-shell
// choice) pair to a launchable shell path. When unset, "shell" sessions fall
// back to the global agent.ShellPath().
func (m *Manager) SetShellResolver(f func(projectID, sessionShell string) string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shellFor = f
}

// shellArgvFor returns the argv to launch for a "shell" session, honoring the
// resolver (session -> project -> global). A preset command (rec.Command) runs
// via `shell -c <cmd>`. Caller must not hold m.mu.
func (m *Manager) shellArgvFor(rec store.Record) []string {
	m.mu.Lock()
	f := m.shellFor
	m.mu.Unlock()
	shell := agent.ShellPath()
	if f != nil {
		if p := f(rec.Project, rec.Shell); p != "" {
			shell = p
		}
	}
	if cmd := strings.TrimSpace(rec.Command); cmd != "" {
		return []string{shell, "-c", cmd}
	}
	return []string{shell}
}

// freshArgv builds the launch argv for a record: shell sessions resolve their
// configured shell; other agents use the agent's own command (resume preferred
// when resume is true).
func (m *Manager) freshArgv(ag agent.Agent, rec store.Record, resume bool) []string {
	if rec.Agent == "shell" {
		return m.shellArgvFor(rec)
	}
	if resume {
		if argv := ag.ResumeCommand(rec.Cwd, rec.AgentSessionID); argv != nil {
			return argv
		}
	}
	return ag.NewCommand(rec.Cwd, rec.AgentSessionID)
}

// sessionEnv returns the tmux env entries for a session id.
func (m *Manager) sessionEnv(id string) []string {
	m.mu.Lock()
	url, bin := m.selfURL, m.binPath
	m.mu.Unlock()
	return []string{
		"ORCHID_SESSION=" + id,
		"ORCHID_URL=" + url,
		"ORCHID_BIN=" + bin,
	}
}

func newID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// saveLocked persists all records. Caller must hold m.mu. Errors are logged
// (tmux remains the source of truth), never fatal.
func (m *Manager) saveLocked() {
	recs := make([]store.Record, 0, len(m.records))
	for _, r := range m.records {
		recs = append(recs, r)
	}
	sort.Slice(recs, func(i, j int) bool { return recs[i].ID < recs[j].ID })
	if err := store.Save(m.path, recs); err != nil {
		log.Printf("orchid: save sessions: %v", err)
	}
}

// onExit marks a session stopped: drop its live PTY entry, flip the record to
// stopped, and persist. The record is retained (resumable). No-op during
// shutdown (KillAll) so still-alive tmux sessions are not falsely marked stopped.
func (m *Manager) onExit(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closing {
		return
	}
	delete(m.sessions, id)
	delete(m.signals, id)
	r, ok := m.records[id]
	if !ok {
		return // already removed (e.g. by Kill); nothing to persist
	}
	r.Status = "stopped"
	m.records[id] = r
	m.saveLocked()
}

// CreateOpts carries the optional per-session settings chosen at creation.
type CreateOpts struct {
	Name    string
	Color   string
	Shell   string // shell choice for "shell" sessions ("" = inherit)
	Command string // preset command to run via `shell -c` (shell sessions only)
}

func (m *Manager) Create(projectID, worktree, agentName string, opts CreateOpts) (*Session, error) {
	ag, ok := agent.Get(agentName)
	if !ok {
		return nil, fmt.Errorf("unknown agent %q", agentName)
	}
	id := newID()
	rec := store.Record{
		ID:             id,
		Name:           strings.TrimSpace(opts.Name),
		Agent:          agentName,
		Cwd:            worktree,
		Project:        projectID,
		Worktree:       worktree,
		Tmux:           "orchid-" + id,
		AgentSessionID: uuid.NewString(),
		Created:        time.Now().UTC(),
		Status:         "running",
		Color:          strings.TrimSpace(opts.Color),
		Shell:          strings.TrimSpace(opts.Shell),
		Command:        strings.TrimSpace(opts.Command),
	}
	s, err := createSession(rec, m.freshArgv(ag, rec, false), m.sessionEnv(id), func() { m.onExit(id) })
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.sessions[id] = s
	m.records[id] = rec
	m.saveLocked()
	m.mu.Unlock()
	s.Start()
	return s, nil
}

// Restore loads persisted records and reconciles them against live tmux:
// alive -> running (reattach), dead -> stopped (retained).
func (m *Manager) Restore() error {
	recs, err := store.Load(m.path)
	if err != nil {
		return err
	}
	m.mu.Lock()
	var started []*Session
	for _, rec := range recs {
		if rec.Project == "" || rec.Worktree == "" {
			continue
		}
		if tmux.HasSession(rec.Tmux) {
			rec.Status = "running"
			id := rec.ID
			s, err := attachSession(rec, func() { m.onExit(id) })
			if err != nil {
				log.Printf("orchid: reattach %s: %v", rec.ID, err)
				rec.Status = "stopped"
			} else {
				m.sessions[rec.ID] = s
				started = append(started, s)
			}
		} else {
			rec.Status = "stopped"
		}
		m.records[rec.ID] = rec
		// restore a persisted requires_action so it survives the restart
		if rec.Signal == "requires_action" {
			m.signals[rec.ID] = &signalState{state: "requires_action"}
		}
	}
	m.saveLocked()
	m.mu.Unlock()
	for _, s := range started {
		s.Start()
	}
	return nil
}

// Restart resumes a stopped session into a fresh tmux session.
func (m *Manager) Restart(id string) (*Session, error) {
	m.mu.Lock()
	rec, ok := m.records[id]
	_, running := m.sessions[id]
	m.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("session %q not found", id)
	}
	if running {
		return nil, fmt.Errorf("session %q already running", id)
	}
	ag, ok := agent.Get(rec.Agent)
	if !ok {
		return nil, fmt.Errorf("unknown agent %q", rec.Agent)
	}
	argv := m.freshArgv(ag, rec, true)
	if err := tmux.NewSession(rec.Tmux, rec.Cwd, m.sessionEnv(id), argv); err != nil {
		if tmux.HasSession(rec.Tmux) {
			return nil, fmt.Errorf("session %q already running", id)
		}
		return nil, err
	}
	s, err := attachSession(rec, func() { m.onExit(id) })
	if err != nil {
		return nil, err
	}
	rec.Status = "running"
	m.mu.Lock()
	m.sessions[id] = s
	m.records[id] = rec
	m.saveLocked()
	m.mu.Unlock()
	s.Start()
	return s, nil
}

// Stop terminates a session's running process (kills its tmux) but RETAINS the
// record as stopped, so it can be restarted later. Unlike Kill, the session is
// not removed. A stopped session is a no-op.
func (m *Manager) Stop(id string) error {
	m.mu.Lock()
	rec, ok := m.records[id]
	s := m.sessions[id]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("session %q not found", id)
	}
	if s != nil {
		s.Close()
	}
	_ = tmux.KillSession(rec.Tmux)
	m.mu.Lock()
	if r, ok := m.records[id]; ok {
		r.Status = "stopped"
		m.records[id] = r
	}
	delete(m.sessions, id)
	delete(m.signals, id)
	m.saveLocked()
	m.mu.Unlock()
	return nil
}

// Signal records an agent-reported state for a session. Valid states:
// running | done | requires_action | idle.
func (m *Manager) Signal(id, state string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.records[id]
	if !ok {
		return
	}
	switch state {
	case "running", "done", "requires_action", "idle":
		sg := m.signals[id]
		if sg == nil {
			sg = &signalState{}
			m.signals[id] = sg
		}
		sg.state = state
		// persist requires_action so it survives a daemon restart; clear on any
		// other state (the user replied / the agent moved on).
		want := ""
		if state == "requires_action" {
			want = "requires_action"
		}
		if rec.Signal != want {
			rec.Signal = want
			m.records[id] = rec
			m.saveLocked()
		}
	}
}

// SyncTitle fills one session's display name from claude's ai-title when the
// name is empty (claude only). Cheap no-op for named / non-claude sessions.
func (m *Manager) SyncTitle(id string) {
	m.mu.Lock()
	r, ok := m.records[id]
	m.mu.Unlock()
	if !ok || r.Agent != "claude" || strings.TrimSpace(r.Name) != "" || r.AgentSessionID == "" {
		return
	}
	t := agent.ClaudeTitle(r.AgentSessionID)
	if t == "" {
		return
	}
	m.mu.Lock()
	if rr, ok := m.records[id]; ok && strings.TrimSpace(rr.Name) == "" {
		rr.Name = t
		m.records[id] = rr
		m.saveLocked()
	}
	m.mu.Unlock()
}

// SyncTitles fills the display name of any claude session that has no name yet
// with claude's auto-generated session title (the "ai-title" shown by
// `claude -r`). User-set names are never overwritten. Transcript files are read
// outside the lock. Cheap once names are filled (nothing left to do).
func (m *Manager) SyncTitles() {
	m.mu.Lock()
	todo := make(map[string]string) // id -> agentSessionID
	for id, r := range m.records {
		if r.Agent == "claude" && strings.TrimSpace(r.Name) == "" && r.AgentSessionID != "" {
			todo[id] = r.AgentSessionID
		}
	}
	m.mu.Unlock()
	if len(todo) == 0 {
		return
	}
	titles := make(map[string]string)
	for id, asid := range todo {
		if t := agent.ClaudeTitle(asid); t != "" {
			titles[id] = t
		}
	}
	if len(titles) == 0 {
		return
	}
	m.mu.Lock()
	for id, t := range titles {
		if r, ok := m.records[id]; ok && strings.TrimSpace(r.Name) == "" {
			r.Name = t
			m.records[id] = r
		}
	}
	m.saveLocked()
	m.mu.Unlock()
}

// Rename sets a session's display name (empty clears it) and persists.
func (m *Manager) Rename(id, name string) error {
	return m.Update(id, SessUpdate{Name: &name})
}

// SessUpdate carries optional session edits. A nil field is left unchanged; a
// non-nil field is applied verbatim (so a pointer to "" clears it).
type SessUpdate struct {
	Name  *string
	Color *string
	Shell *string
}

// Update applies the non-nil fields of u to a session record and persists.
func (m *Manager) Update(id string, u SessUpdate) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.records[id]
	if !ok {
		return fmt.Errorf("session %q not found", id)
	}
	if u.Name != nil {
		rec.Name = strings.TrimSpace(*u.Name)
	}
	if u.Color != nil {
		rec.Color = strings.TrimSpace(*u.Color)
	}
	if u.Shell != nil {
		rec.Shell = strings.TrimSpace(*u.Shell)
	}
	m.records[id] = rec
	m.saveLocked()
	return nil
}

// Reorder sets the display Order of the given session ids to their position in
// the list (used for drag-and-drop within a worktree) and persists.
func (m *Manager) Reorder(ids []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	changed := false
	for i, id := range ids {
		if r, ok := m.records[id]; ok && r.Order != i {
			r.Order = i
			m.records[id] = r
			changed = true
		}
	}
	if changed {
		m.saveLocked()
	}
}

// ReorderGrid sets the grid-display GridOrder of the given session ids to their
// position (a separate order from Reorder, used by the Grid tab) and persists.
func (m *Manager) ReorderGrid(ids []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	changed := false
	for i, id := range ids {
		if r, ok := m.records[id]; ok && r.GridOrder != i {
			r.GridOrder = i
			m.records[id] = r
			changed = true
		}
	}
	if changed {
		m.saveLocked()
	}
}

func (m *Manager) Get(id string) (*Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	return s, ok
}

func (m *Manager) List() []Info {
	type ent struct {
		rec store.Record
		s   *Session
		sg  signalState
		has bool
	}
	m.mu.Lock()
	ents := make([]ent, 0, len(m.records))
	for _, r := range m.records {
		e := ent{rec: r, s: m.sessions[r.ID]}
		if sg := m.signals[r.ID]; sg != nil {
			e.sg, e.has = *sg, true
		}
		ents = append(ents, e)
	}
	m.mu.Unlock()

	// stable display order: worktree, then user Order, then id
	sort.Slice(ents, func(i, j int) bool {
		a, b := ents[i].rec, ents[j].rec
		if a.Worktree != b.Worktree {
			return a.Worktree < b.Worktree
		}
		if a.Order != b.Order {
			return a.Order < b.Order
		}
		return a.ID < b.ID
	})
	out := make([]Info, 0, len(ents))
	for _, e := range ents {
		status := resolveStatus(e.rec.Agent, e.s, e.sg, e.has)
		out = append(out, Info{
			ID:       e.rec.ID,
			Name:     e.rec.Name,
			Agent:    e.rec.Agent,
			Cwd:      e.rec.Cwd,
			Project:   e.rec.Project,
			Worktree:  e.rec.Worktree,
			Status:    status,
			Alive:     e.s != nil,
			GridOrder: e.rec.GridOrder,
			Color:     e.rec.Color,
			Shell:     e.rec.Shell,
			Command:   e.rec.Command,
		})
	}
	return out
}

// resolveStatus maps a session + its latest signal to a display status.
func resolveStatus(agentName string, s *Session, sg signalState, has bool) string {
	if s == nil {
		return "stopped"
	}
	if !has {
		if s.Active(idleAfter) {
			return "running"
		}
		return "idle"
	}
	switch sg.state {
	case "running":
		return "running"
	case "requires_action":
		// requires_action is raised only by an explicit agent set_status call;
		// it stays until the agent next reports running (the user replied).
		return "requires_action"
	case "done", "idle":
		// finished a turn with no explicit question -> idle (not "needs you").
		return "idle"
	default:
		if s.Active(idleAfter) {
			return "running"
		}
		return "idle"
	}
}

// Kill terminates a session for good: kill tmux, drop session + record.
func (m *Manager) Kill(id string) error {
	m.mu.Lock()
	rec, ok := m.records[id]
	s := m.sessions[id]
	if ok {
		delete(m.records, id)
		delete(m.sessions, id)
		delete(m.signals, id)
	}
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("session %q not found", id)
	}
	if s != nil {
		s.Close()
	}
	_ = tmux.KillSession(rec.Tmux)
	m.mu.Lock()
	m.saveLocked()
	m.mu.Unlock()
	return nil
}

// PruneStopped drops every stopped (not-alive) session record and persists.
// Running sessions are left untouched. Returns the number removed.
func (m *Manager) PruneStopped() int {
	m.mu.Lock()
	var victims []store.Record
	for id, r := range m.records {
		if _, running := m.sessions[id]; running {
			continue
		}
		if r.Status == "stopped" {
			victims = append(victims, r)
			delete(m.records, id)
		}
	}
	if len(victims) > 0 {
		m.saveLocked()
	}
	m.mu.Unlock()
	for _, r := range victims {
		_ = tmux.KillSession(r.Tmux) // defensive; stopped sessions have no tmux
	}
	return len(victims)
}

// PruneOrphanTmux kills leftover "orchid-" tmux sessions that have no record
// (e.g. from a crashed or replaced daemon). Returns the number killed. Intended
// to run once at startup, after Restore.
func (m *Manager) PruneOrphanTmux() int {
	m.mu.Lock()
	known := make(map[string]bool, len(m.records))
	for _, r := range m.records {
		known[r.Tmux] = true
	}
	m.mu.Unlock()
	names, _ := tmux.ListSessions()
	n := 0
	for _, name := range orphanSessions(names, known, "orchid-") {
		if err := tmux.KillSession(name); err == nil {
			n++
		}
	}
	return n
}

// orphanSessions returns the prefix-matching names not present in known.
func orphanSessions(names []string, known map[string]bool, prefix string) []string {
	var out []string
	for _, name := range names {
		if strings.HasPrefix(name, prefix) && !known[name] {
			out = append(out, name)
		}
	}
	return out
}

// KillByProject terminates every session (running or stopped) of a project:
// kills tmux, drops session + record. Git worktrees are NOT touched. Returns count.
func (m *Manager) KillByProject(projectID string) int {
	m.mu.Lock()
	var victims []store.Record
	var live []*Session
	for id, r := range m.records {
		if r.Project == projectID {
			victims = append(victims, r)
			if s := m.sessions[id]; s != nil {
				live = append(live, s)
			}
			delete(m.records, id)
			delete(m.sessions, id)
			delete(m.signals, id)
		}
	}
	m.mu.Unlock()
	for _, s := range live {
		s.Close()
	}
	for _, r := range victims {
		_ = tmux.KillSession(r.Tmux)
	}
	m.mu.Lock()
	m.saveLocked()
	m.mu.Unlock()
	return len(victims)
}

// KillByWorktree terminates every session (running or stopped) bound to the
// given worktree path: kill tmux, drop session + record. Returns the count.
func (m *Manager) KillByWorktree(worktree string) int {
	m.mu.Lock()
	var victims []store.Record
	var live []*Session
	for id, r := range m.records {
		if r.Worktree == worktree {
			victims = append(victims, r)
			if s := m.sessions[id]; s != nil {
				live = append(live, s)
			}
			delete(m.records, id)
			delete(m.sessions, id)
			delete(m.signals, id)
		}
	}
	m.mu.Unlock()
	for _, s := range live {
		s.Close()
	}
	for _, r := range victims {
		_ = tmux.KillSession(r.Tmux)
	}
	m.mu.Lock()
	m.saveLocked()
	m.mu.Unlock()
	return len(victims)
}

// KillAll closes live PTYs only (shutdown). tmux sessions and records survive
// so they can be restored next boot.
func (m *Manager) KillAll() {
	m.mu.Lock()
	m.closing = true
	all := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		all = append(all, s)
	}
	m.sessions = make(map[string]*Session)
	m.mu.Unlock()
	for _, s := range all {
		s.Close()
	}
}
