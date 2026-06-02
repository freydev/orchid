package server

import (
	"net/http"

	"orchid/internal/project"
	"orchid/internal/session"
	"orchid/web"
)

type Server struct {
	mgr      *session.Manager
	projects *project.Store
	mux      *http.ServeMux
}

func New(mgr *session.Manager, projects *project.Store) http.Handler {
	s := &Server{mgr: mgr, projects: projects, mux: http.NewServeMux()}
	s.routes()
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/sessions", s.handleList)
	s.mux.HandleFunc("GET /api/shell", s.handleShell)
	s.mux.HandleFunc("POST /api/sessions", s.handleCreate)
	s.mux.HandleFunc("POST /api/sessions/prune", s.handlePrune)
	s.mux.HandleFunc("DELETE /api/sessions/{id}", s.handleKill)
	s.mux.HandleFunc("PATCH /api/sessions/{id}", s.handleRename)
	s.mux.HandleFunc("POST /api/sessions/{id}/restart", s.handleRestart)
	s.mux.HandleFunc("POST /api/sessions/{id}/stop", s.handleStop)
	s.mux.HandleFunc("POST /api/sessions/{id}/status", s.handleStatus)

	s.mux.HandleFunc("GET /api/projects", s.handleProjects)
	s.mux.HandleFunc("POST /api/projects", s.handleAddProject)
	s.mux.HandleFunc("PUT /api/projects/order", s.handleProjectsOrder)
	s.mux.HandleFunc("PUT /api/sessions/order", s.handleSessionsOrder)
	s.mux.HandleFunc("PUT /api/sessions/grid-order", s.handleSessionsGridOrder)
	s.mux.HandleFunc("PUT /api/projects/{id}/worktrees/order", s.handleWorktreesOrder)
	s.mux.HandleFunc("DELETE /api/projects/{id}", s.handleRemoveProject)
	s.mux.HandleFunc("GET /api/projects/{id}/gitinfo", s.handleGitInfo)
	s.mux.HandleFunc("GET /api/projects/{id}/worktrees", s.handleWorktrees)
	s.mux.HandleFunc("POST /api/projects/{id}/worktrees", s.handleAddWorktree)
	s.mux.HandleFunc("DELETE /api/projects/{id}/worktrees", s.handleRemoveWorktree)

	s.mux.HandleFunc("GET /ws/{id}", s.handleWS)
	s.mux.Handle("/", http.FileServer(http.FS(web.FS)))
}
