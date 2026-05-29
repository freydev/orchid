package session

import (
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
	"orchid/internal/agent"
	"orchid/internal/store"
	"orchid/internal/tmux"
)

const ringSize = 64 * 1024

// attachment is a single live WebSocket subscriber.
type attachment struct {
	ch   chan []byte
	done chan struct{}
}

// Session wraps a shell running in a PTY.
type Session struct {
	ID             string
	Agent          string
	Cwd            string
	Tmux           string
	AgentSessionID string

	cmd    *exec.Cmd
	pty    *os.File
	ring   *ringBuffer
	onExit func()

	mu       sync.Mutex
	alive    bool
	sub      *attachment // currently attached WS, or nil
	doneOnce sync.Once
}

// createSession creates a NEW detached tmux session running the agent's fresh
// command, then spawns the attach-PTY that views it.
func createSession(rec store.Record, ag agent.Agent, onExit func()) (*Session, error) {
	argv := ag.NewCommand(rec.Cwd, rec.AgentSessionID)
	if err := tmux.NewSession(rec.Tmux, rec.Cwd, argv); err != nil {
		return nil, err
	}
	return attachSession(rec, onExit)
}

// attachSession spawns the attach-PTY for an already-existing tmux session.
func attachSession(rec store.Record, onExit func()) (*Session, error) {
	attach := tmux.AttachArgv(rec.Tmux)
	cmd := exec.Command(attach[0], attach[1:]...)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	f, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}
	s := &Session{
		ID:             rec.ID,
		Agent:          rec.Agent,
		Cwd:            rec.Cwd,
		Tmux:           rec.Tmux,
		AgentSessionID: rec.AgentSessionID,
		cmd:            cmd,
		pty:            f,
		ring:           newRingBuffer(ringSize),
		onExit:         onExit,
		alive:          true,
	}
	go s.readLoop()
	return s, nil
}

func (s *Session) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := s.pty.Read(buf)
		if n > 0 {
			s.broadcast(buf[:n])
		}
		if err != nil {
			break
		}
	}
	s.mu.Lock()
	s.alive = false
	if s.sub != nil {
		closeOnce(s.sub.done)
		s.sub = nil
	}
	s.mu.Unlock()
	s.Close()
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
	return s.pty.Write(p)
}

func (s *Session) Resize(cols, rows uint16) error {
	return pty.Setsize(s.pty, &pty.Winsize{Cols: cols, Rows: rows})
}

func (s *Session) Alive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.alive
}

// Close terminates the shell and PTY.
func (s *Session) Close() {
	s.doneOnce.Do(func() {
		_ = s.pty.Close()
		if s.cmd.Process != nil {
			_ = s.cmd.Process.Kill()
		}
		go func() { _ = s.cmd.Wait() }()
	})
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
