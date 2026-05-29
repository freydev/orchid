package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
)

// Info is a serializable session summary.
type Info struct {
	ID    string `json:"id"`
	Alive bool   `json:"alive"`
}

// Manager owns the set of live sessions.
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session
}

func NewManager() *Manager {
	return &Manager{sessions: make(map[string]*Session)}
}

func newID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (m *Manager) remove(id string) {
	m.mu.Lock()
	delete(m.sessions, id)
	m.mu.Unlock()
}

func (m *Manager) Create() (*Session, error) {
	id := newID()
	s, err := newSession(id, func() { m.remove(id) })
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.sessions[id] = s
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
	out := make([]Info, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, Info{ID: s.ID, Alive: s.Alive()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (m *Manager) Kill(id string) error {
	m.mu.Lock()
	s, ok := m.sessions[id]
	if ok {
		delete(m.sessions, id)
	}
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("session %q not found", id)
	}
	s.Close()
	return nil
}

func (m *Manager) KillAll() {
	m.mu.Lock()
	all := m.sessions
	m.sessions = make(map[string]*Session)
	m.mu.Unlock()
	for _, s := range all {
		s.Close()
	}
}
