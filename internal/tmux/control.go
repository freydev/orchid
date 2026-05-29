package tmux

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
)

// Control is a tmux control-mode client (tmux -CC attach) for one session.
// Lifecycle: Attach (synchronous setup: discover pane, optional Capture), then
// Start (begins streaming %output on Output()).
type Control struct {
	cmd  *exec.Cmd
	pty  *os.File
	br   *bufio.Reader
	pane string

	writeMu sync.Mutex
	ch      chan []byte
	started bool
}

// Attach starts `tmux -CC attach -t name` under a PTY and discovers the pane id.
func Attach(name string) (*Control, error) {
	cmd := exec.Command("tmux", "-CC", "attach", "-t", name)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	f, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}
	c := &Control{
		cmd: cmd, pty: f,
		br: bufio.NewReaderSize(f, 1<<20),
		ch: make(chan []byte, 256),
	}
	pane, err := c.discoverPane(name)
	if err != nil {
		_ = c.Close()
		return nil, err
	}
	c.pane = pane
	return c, nil
}

func (c *Control) Pane() string { return c.pane }

func (c *Control) readLine() ([]byte, error) {
	line, err := c.br.ReadBytes('\n')
	if len(line) == 0 {
		return nil, err
	}
	line = bytes.TrimRight(line, "\r\n")
	return stripDCSPrefix(line), err
}

func (c *Control) writeCmd(format string, args ...any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err := fmt.Fprintf(c.pty, format+"\n", args...)
	return err
}

// readReplyBlock reads lines until the next %end/%error, returning content lines
// between %begin and %end. Setup-phase only (no concurrent reader).
func (c *Control) readReplyBlock() ([][]byte, error) {
	var content [][]byte
	inBlock := false
	for {
		line, err := c.readLine()
		if line != nil {
			if k, ok := beginEnd(line); ok {
				if k == lineBegin {
					inBlock = true
					content = nil
					continue
				}
				return content, nil
			}
			if inBlock {
				content = append(content, line)
			}
		}
		if err != nil {
			return content, err
		}
	}
}

func (c *Control) discoverPane(name string) (string, error) {
	if err := c.writeCmd("list-panes -t %s -F \"#{pane_id}\"", name); err != nil {
		return "", err
	}
	for attempts := 0; attempts < 50; attempts++ {
		block, err := c.readReplyBlock()
		for _, l := range block {
			if len(l) >= 2 && l[0] == '%' && l[1] >= '0' && l[1] <= '9' {
				return string(l), nil
			}
		}
		if err != nil {
			return "", fmt.Errorf("pane discovery: %w", err)
		}
	}
	return "", fmt.Errorf("pane id not found for %s", name)
}

// Capture returns up to lines of scrollback (with escapes). Setup-phase only.
func (c *Control) Capture(lines int) ([]byte, error) {
	if c.started {
		return nil, fmt.Errorf("Capture must be called before Start")
	}
	if err := c.writeCmd("capture-pane -t %s -e -p -J -S -%d", c.pane, lines); err != nil {
		return nil, err
	}
	block, err := c.readReplyBlock()
	if err != nil && len(block) == 0 {
		return nil, err
	}
	// Drop the blank rows capture-pane pads to the full pane height, so seeding
	// doesn't push the prompt down with empty space above it.
	return trimTrailingBlankLines(bytes.Join(block, []byte("\r\n"))), nil
}

// Start launches the streaming goroutine: %output -> Output() channel.
func (c *Control) Start() {
	c.started = true
	go func() {
		defer close(c.ch)
		for {
			line, err := c.readLine()
			if line != nil {
				if pane, data, ok := parseOutput(line); ok && pane == c.pane && len(data) > 0 {
					c.ch <- data
				}
			}
			if err != nil {
				return
			}
		}
	}()
}

func (c *Control) Output() <-chan []byte { return c.ch }

// SendKeys injects raw bytes into the pane.
func (c *Control) SendKeys(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	return c.writeCmd("send-keys -t %s -H %s", c.pane, hexKeys(b))
}

// Resize sets the pane size via the control client.
func (c *Control) Resize(cols, rows uint16) error {
	if cols == 0 || rows == 0 {
		return nil
	}
	return c.writeCmd("refresh-client -C %dx%d", cols, rows)
}

// Close terminates the control client. The tmux session survives.
func (c *Control) Close() error {
	_ = c.pty.Close()
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	go func() { _ = c.cmd.Wait() }()
	return nil
}
