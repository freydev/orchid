package session

import (
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
)

const ringSize = 64 * 1024

// attachment is a single live WebSocket subscriber.
type attachment struct {
	ch   chan []byte
	done chan struct{}
}

// Session wraps a shell running in a PTY.
type Session struct {
	ID string

	cmd  *exec.Cmd
	pty  *os.File
	ring *ringBuffer

	mu       sync.Mutex
	alive    bool
	sub      *attachment // currently attached WS, or nil
	doneOnce sync.Once
}

func newSession(id string) (*Session, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	f, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}
	s := &Session{ID: id, cmd: cmd, pty: f, ring: newRingBuffer(ringSize), alive: true}
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
		close(s.sub.done)
		s.sub = nil
	}
	s.mu.Unlock()
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
	})
}
