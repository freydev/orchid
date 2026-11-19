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
	subs       map[*attachment]struct{} // live WS clients (multiple windows)
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
		subs:           make(map[*attachment]struct{}),
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
	for a := range s.subs {
		closeOnce(a.done)
	}
	s.subs = make(map[*attachment]struct{})
	s.mu.Unlock()
	if s.onExit != nil {
		s.onExit()
	}
}

// broadcast writes output to the ring and fans it out to every live subscriber
// (each open window/tab), so multiple views of the same session stay live.
func (s *Session) broadcast(p []byte) {
	s.ring.Write(p)
	s.mu.Lock()
	subs := make([]*attachment, 0, len(s.subs))
	for a := range s.subs {
		subs = append(subs, a)
	}
	s.mu.Unlock()
	for _, a := range subs {
		cp := append([]byte(nil), p...)
		select {
		case a.ch <- cp:
		case <-a.done:
		}
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

// Size returns the current tmux pane geometry (0,0 if unknown). Used to seed a
// read-only preview pane so redraw-heavy agents (claude/codex) render the same
// screen the real pane has, instead of into a too-small grid.
func (s *Session) Size() (cols, rows uint16) {
	return s.ctl.Size()
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

// Attach registers a new live subscriber. Multiple may be attached at once
// (several windows/tabs viewing the same session). Returns nil if dead.
func (s *Session) Attach() *attachment {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.alive {
		return nil
	}
	a := &attachment{ch: make(chan []byte, 256), done: make(chan struct{})}
	s.subs[a] = struct{}{}
	return a
}

// Detach removes a subscriber.
func (s *Session) Detach(a *attachment) {
	if a == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.subs[a]; ok {
		closeOnce(a.done)
		delete(s.subs, a)
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
