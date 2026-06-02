package tmux

import (
	"bytes"
	"testing"
	"time"
)

func TestControlAttachStreamSendKeys(t *testing.T) {
	skipIfNoTmux(t)
	name := "orchid-test-ctl"
	_ = KillSession(name)
	if err := NewSession(name, "/tmp", nil, []string{"bash", "--norc"}); err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer KillSession(name)

	ctl, err := Attach(name)
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	defer ctl.Close()
	if ctl.Pane() == "" {
		t.Fatal("pane id not discovered")
	}

	ctl.Start()
	if err := ctl.SendKeys([]byte("echo MARK123\n")); err != nil {
		t.Fatalf("SendKeys: %v", err)
	}

	var acc []byte
	deadline := time.After(6 * time.Second)
	for {
		select {
		case b, ok := <-ctl.Output():
			if !ok {
				t.Fatalf("output closed; acc=%q", acc)
			}
			acc = append(acc, b...)
			if bytes.Contains(acc, []byte("MARK123")) {
				return
			}
		case <-deadline:
			t.Fatalf("MARK123 not seen; acc=%q", acc)
		}
	}
}

func TestControlCaptureReturnsHistory(t *testing.T) {
	skipIfNoTmux(t)
	name := "orchid-test-cap"
	_ = KillSession(name)
	if err := NewSession(name, "/tmp", nil, []string{"bash", "--norc"}); err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer KillSession(name)

	c1, err := Attach(name)
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	c1.Start()
	c1.SendKeys([]byte("echo HISTLINE\n"))
	time.Sleep(1500 * time.Millisecond)
	c1.Close()

	c2, err := Attach(name)
	if err != nil {
		t.Fatalf("Attach2: %v", err)
	}
	defer c2.Close()
	hist, err := c2.Capture(200)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if !bytes.Contains(hist, []byte("HISTLINE")) {
		t.Fatalf("history missing HISTLINE; got %q", hist)
	}
}
