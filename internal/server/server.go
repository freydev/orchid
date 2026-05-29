package server

import (
	"net/http"

	"orchid/internal/session"
	"orchid/web"
)

type Server struct {
	mgr *session.Manager
	mux *http.ServeMux
}

func New(mgr *session.Manager) http.Handler {
	s := &Server{mgr: mgr, mux: http.NewServeMux()}
	s.routes()
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/sessions", s.handleList)
	s.mux.HandleFunc("POST /api/sessions", s.handleCreate)
	s.mux.HandleFunc("DELETE /api/sessions/{id}", s.handleKill)
	s.mux.HandleFunc("POST /api/sessions/{id}/restart", s.handleRestart)
	s.mux.HandleFunc("GET /ws/{id}", s.handleWS)

	s.mux.Handle("/", http.FileServer(http.FS(web.FS)))
}
