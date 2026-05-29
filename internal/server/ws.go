package server

import (
	"encoding/json"
	"net/http"

	"github.com/coder/websocket"
)

type controlMsg struct {
	Type string `json:"type"`
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sess, ok := s.mgr.Get(id)
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// localhost dashboard; relax origin checks (localhost only, no auth).
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}
	defer c.Close(websocket.StatusInternalError, "closing")

	ctx := r.Context()

	att := sess.Attach()
	if att == nil {
		c.Close(websocket.StatusGoingAway, "session dead")
		return
	}
	defer sess.Detach(att)

	// Replay scrollback first.
	if replay := sess.Replay(); len(replay) > 0 {
		if err := c.Write(ctx, websocket.MessageBinary, replay); err != nil {
			return
		}
	}

	// PTY -> WS
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-att.Done():
				c.Close(websocket.StatusGoingAway, "detached")
				return
			case b := <-att.Ch():
				if err := c.Write(ctx, websocket.MessageBinary, b); err != nil {
					return
				}
			}
		}
	}()

	// WS -> PTY (also handles control frames)
	for {
		typ, data, err := c.Read(ctx)
		if err != nil {
			return
		}
		if typ == websocket.MessageText {
			var m controlMsg
			if json.Unmarshal(data, &m) == nil && m.Type == "resize" {
				_ = sess.Resize(m.Cols, m.Rows)
			}
			continue
		}
		if _, err := sess.Write(data); err != nil {
			return
		}
	}
}
