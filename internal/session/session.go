package session

import (
	"sync"
	"time"

	"orchid/internal/store"
	"orchid/internal/tmux"
)

const ringSize = 1 << 20 // 1 MiB: ring now holds linear program history

// attachment is a single live WebSocket subscriber.
type attachment struct {
	ch   chan []byte
	done chan struct{}
}

// Session streams one tmux pane (via a control-mode client) to a WS client.
type Session struct {
	ID             string
	Agent          string
	Cwd            string
	Project        string
	Worktree       string
	Tmux           string
	AgentSessionID string

	ctl    *tmux.Control
	ring   *ringBuffer
	onExit func()

	mu         sync.Mutex
	alive      bool
	lastOutput time.Time
	sub        *attachment
	doneOnce   sync.Once
}

// historyLines is how much scrollback to seed from tmux on (re)attach.
const historyLines = 2000

// createSession creates a NEW detached tmux session running argv, then attaches
// a control-mode client.
func createSession(rec store.Record, argv []string, env []string, onExit func()) (*Session, error) {
	if err := tmux.NewSession(rec.Tmux, rec.Cwd, env, argv); err != nil {
		return nil, err
	}
	return attachSession(rec, onExit)
}

// attachSession attaches a control-mode client to an existing tmux session and
// seeds the ring with prior scrollback so reconnects show history.
func attachSession(rec store.Record, onExit func()) (*Session, error) {
	ctl, err := tmux.Attach(rec.Tmux)
	if err != nil {
		return nil, err
	}
	ring := newRingBuffer(ringSize)
	if hist, err := ctl.Capture(historyLines); err == nil && len(hist) > 0 {
		ring.Write(hist)
	}
	return &Session{
		ID:             rec.ID,
		Agent:          rec.Agent,
		Cwd:            rec.Cwd,
		Project:        rec.Project,
		Worktree:       rec.Worktree,
		Tmux:           rec.Tmux,
		AgentSessionID: rec.AgentSessionID,
		ctl:            ctl,
		ring:           ring,
		onExit:         onExit,
		alive:          true,
		lastOutput:     time.Now(),
	}, nil
}

// Start begins streaming pane output. Call once, after registration.
func (s *Session) Start() {
	s.ctl.Start()
	go s.readLoop()
}

func (s *Session) readLoop() {
	for data := range s.ctl.Output() {
		s.mu.Lock()
		s.lastOutput = time.Now()
		s.mu.Unlock()
		s.broadcast(data)
	}
	s.mu.Lock()
	s.alive = false
	if s.sub != nil {
		closeOnce(s.sub.done)
		s.sub = nil
	}
	s.mu.Unlock()
	if s.onExit != nil {
		s.onExit()
	}
}

// broadcast writes output to the ring and the live subscriber (if any).
func (s *Session) broadcast(p []byte) {
	s.ring.Write(p)
	s.mu.Lock()
	a := s.sub
	s.mu.Unlock()
	if a == nil {
		return
	}
	cp := append([]byte(nil), p...)
	select {
	case a.ch <- cp:
	case <-a.done:
	}
}

func (s *Session) Write(p []byte) (int, error) {
	if err := s.ctl.SendKeys(p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (s *Session) Resize(cols, rows uint16) error {
	return s.ctl.Resize(cols, rows)
}

func (s *Session) Alive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.alive
}

// Active reports whether pane output was produced within the given window.
func (s *Session) Active(within time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return time.Since(s.lastOutput) < within
}

// Close terminates the control client. The tmux session survives.
func (s *Session) Close() {
	s.doneOnce.Do(func() { _ = s.ctl.Close() })
}

// Attach registers a new live subscriber, kicking any previous one.
// Returns nil if the session is already dead.
func (s *Session) Attach() *attachment {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.alive {
		return nil
	}
	if s.sub != nil {
		closeOnce(s.sub.done)
	}
	a := &attachment{ch: make(chan []byte, 256), done: make(chan struct{})}
	s.sub = a
	return a
}

// Detach removes a as the current subscriber if it still is one.
func (s *Session) Detach(a *attachment) {
	if a == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sub == a {
		closeOnce(s.sub.done)
		s.sub = nil
	}
}

// Replay returns the current scrollback snapshot for a freshly attached client.
func (s *Session) Replay() []byte {
	return s.ring.Snapshot()
}

// Done returns a channel that is closed when the attachment is invalidated
// (session died or a newer subscriber kicked this one).
func (a *attachment) Done() <-chan struct{} { return a.done }

// Ch returns the channel on which PTY output bytes are delivered.
func (a *attachment) Ch() <-chan []byte { return a.ch }

func closeOnce(ch chan struct{}) {
	select {
	case <-ch:
		// already closed
	default:
		close(ch)
	}
}
