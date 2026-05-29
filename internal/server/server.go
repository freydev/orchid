package server

import (
	"net/http"

	"orchid/internal/session"
)

// Server wires the HTTP API and (later) WebSocket + static assets.
type Server struct {
	mgr *session.Manager
	mux *http.ServeMux
}

// New builds the HTTP handler. Static asset wiring is added in Task 8.
func New(mgr *session.Manager) http.Handler {
	s := &Server{mgr: mgr, mux: http.NewServeMux()}
	s.routes()
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/sessions", s.handleList)
	s.mux.HandleFunc("POST /api/sessions", s.handleCreate)
	s.mux.HandleFunc("DELETE /api/sessions/{id}", s.handleKill)
	s.mux.HandleFunc("GET /ws/{id}", s.handleWS)
}
