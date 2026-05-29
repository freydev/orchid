package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"orchid/internal/agent"
	"orchid/internal/store"
	"orchid/internal/tmux"
)

// Info is a serializable session summary.
type Info struct {
	ID     string `json:"id"`
	Agent  string `json:"agent"`
	Cwd    string `json:"cwd"`
	Status string `json:"status"` // "running" | "stopped"
	Alive  bool   `json:"alive"`
}

// Manager owns sessions (running, with a live PTY) and records (all, incl. stopped).
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session
	records  map[string]store.Record
	path     string
	closing  bool
}

func NewManager(path string) *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
		records:  make(map[string]store.Record),
		path:     path,
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
	r, ok := m.records[id]
	if !ok {
		return // already removed (e.g. by Kill); nothing to persist
	}
	r.Status = "stopped"
	m.records[id] = r
	m.saveLocked()
}

func (m *Manager) Create(agentName, cwd string) (*Session, error) {
	ag, ok := agent.Get(agentName)
	if !ok {
		return nil, fmt.Errorf("unknown agent %q", agentName)
	}
	id := newID()
	rec := store.Record{
		ID:             id,
		Agent:          agentName,
		Cwd:            agent.ResolveCwd(cwd),
		Tmux:           "orchid-" + id,
		AgentSessionID: uuid.NewString(),
		Created:        time.Now().UTC(),
		Status:         "running",
	}
	s, err := createSession(rec, ag, func() { m.onExit(id) })
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.sessions[id] = s
	m.records[id] = rec
	m.saveLocked()
	m.mu.Unlock()
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
	defer m.mu.Unlock()
	for _, rec := range recs {
		if tmux.HasSession(rec.Tmux) {
			rec.Status = "running"
			id := rec.ID
			s, err := attachSession(rec, func() { m.onExit(id) })
			if err != nil {
				log.Printf("orchid: reattach %s: %v", rec.ID, err)
				rec.Status = "stopped"
			} else {
				m.sessions[rec.ID] = s
			}
		} else {
			rec.Status = "stopped"
		}
		m.records[rec.ID] = rec
	}
	m.saveLocked()
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
	argv := ag.ResumeCommand(rec.Cwd, rec.AgentSessionID)
	if argv == nil {
		argv = ag.NewCommand(rec.Cwd, rec.AgentSessionID)
	}
	if err := tmux.NewSession(rec.Tmux, rec.Cwd, argv); err != nil {
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
	return s, nil
}

func (m *Manager) Get(id string) (*Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	return s, ok
}

func (m *Manager) List() []Info {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Info, 0, len(m.records))
	for _, r := range m.records {
		_, running := m.sessions[r.ID]
		out = append(out, Info{
			ID:     r.ID,
			Agent:  r.Agent,
			Cwd:    r.Cwd,
			Status: r.Status,
			Alive:  running,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Kill terminates a session for good: kill tmux, drop session + record.
func (m *Manager) Kill(id string) error {
	m.mu.Lock()
	rec, ok := m.records[id]
	s := m.sessions[id]
	if ok {
		delete(m.records, id)
		delete(m.sessions, id)
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
