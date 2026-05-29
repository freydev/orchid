package server

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"orchid/internal/session"
)

func TestWSEchoAndUnknown404(t *testing.T) {
	mgr := session.NewManager(t.TempDir() + "/sessions.json")
	defer mgr.KillAll()
	h := New(mgr)
	srv := httptest.NewServer(h)
	defer srv.Close()

	sess, _ := mgr.Create("shell", "")
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/" + sess.ID

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "")

	// send a command as a binary frame
	if err := c.Write(ctx, websocket.MessageBinary, []byte("echo ws_marker\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	var acc []byte
	for {
		typ, data, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("read: %v (acc=%q)", err, acc)
		}
		if typ == websocket.MessageBinary {
			acc = append(acc, data...)
			if strings.Contains(string(acc), "ws_marker") {
				break
			}
		}
	}

	// unknown session -> dial should fail with non-101
	badURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/nope"
	if _, _, err := websocket.Dial(ctx, badURL, nil); err == nil {
		t.Fatal("expected dial to unknown session to fail")
	}
}
