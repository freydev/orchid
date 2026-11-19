package server

import (
	"io/fs"
	"mime"
	"net/http"

	"orchid/internal/project"
	"orchid/internal/session"
	"orchid/web"
)

func init() {
	// Go's default mime table lacks .webmanifest; without this the PWA manifest
	// is served as octet-stream and some browsers reject it.
	_ = mime.AddExtensionType(".webmanifest", "application/manifest+json")
}

type Server struct {
	mgr      *session.Manager
	projects *project.Store
	mux      *http.ServeMux
	// runtime info for the settings page / self-restart
	selfPath string
	host     string
	port     int
}

func New(mgr *session.Manager, projects *project.Store) *Server {
	s := &Server{mgr: mgr, projects: projects, mux: http.NewServeMux()}
	s.routes()
	return s
}

// ServeHTTP makes Server an http.Handler delegating to its mux.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

// SetRuntime records how the daemon was launched so the settings page can show
// the bind address and the restart endpoint can relaunch with a new port.
func (s *Server) SetRuntime(selfPath, host string, port int) {
	s.selfPath = selfPath
	s.host = host
	s.port = port
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/sessions", s.handleList)
	s.mux.HandleFunc("GET /api/shell", s.handleShell)
	s.mux.HandleFunc("GET /api/claude-sessions", s.handleClaudeSessions)
	s.mux.HandleFunc("POST /api/sessions", s.handleCreate)
	s.mux.HandleFunc("POST /api/sessions/prune", s.handlePrune)
	s.mux.HandleFunc("DELETE /api/sessions/{id}", s.handleKill)
	s.mux.HandleFunc("PATCH /api/sessions/{id}", s.handleRename)
	s.mux.HandleFunc("POST /api/sessions/{id}/restart", s.handleRestart)
	s.mux.HandleFunc("POST /api/sessions/{id}/stop", s.handleStop)
	s.mux.HandleFunc("POST /api/sessions/{id}/status", s.handleStatus)
	s.mux.HandleFunc("POST /api/sessions/{id}/notify", s.handleNotify)

	s.mux.HandleFunc("GET /api/settings", s.handleGetSettings)
	s.mux.HandleFunc("PUT /api/settings", s.handlePutSettings)
	s.mux.HandleFunc("PUT /api/settings/rail-width", s.handleRailWidth)
	s.mux.HandleFunc("POST /api/restart", s.handleRestartDaemon)

	s.mux.HandleFunc("GET /api/projects", s.handleProjects)
	s.mux.HandleFunc("POST /api/projects", s.handleAddProject)
	s.mux.HandleFunc("PATCH /api/projects/{id}", s.handleUpdateProject)
	s.mux.HandleFunc("PUT /api/projects/order", s.handleProjectsOrder)
	s.mux.HandleFunc("PUT /api/sessions/order", s.handleSessionsOrder)
	s.mux.HandleFunc("PUT /api/projects/{id}/worktrees/order", s.handleWorktreesOrder)
	s.mux.HandleFunc("DELETE /api/projects/{id}", s.handleRemoveProject)
	s.mux.HandleFunc("GET /api/projects/{id}/gitinfo", s.handleGitInfo)
	s.mux.HandleFunc("GET /api/projects/{id}/diff", s.handleDiff)
	s.mux.HandleFunc("GET /api/projects/{id}/worktrees", s.handleWorktrees)
	s.mux.HandleFunc("POST /api/projects/{id}/worktrees", s.handleAddWorktree)
	s.mux.HandleFunc("DELETE /api/projects/{id}/worktrees", s.handleRemoveWorktree)
	s.mux.HandleFunc("GET /api/projects/{id}/worktrees/empty", s.handleEmptyWorktrees)
	s.mux.HandleFunc("POST /api/projects/{id}/worktrees/prune", s.handlePruneWorktrees)
	s.mux.HandleFunc("POST /api/projects/{id}/groups", s.handleAddGroup)
	s.mux.HandleFunc("PATCH /api/projects/{id}/groups/{gid}", s.handleRenameGroup)
	s.mux.HandleFunc("DELETE /api/projects/{id}/groups/{gid}", s.handleRemoveGroup)
	s.mux.HandleFunc("POST /api/projects/{id}/groups/{gid}/clear", s.handleClearGroup)

	s.mux.HandleFunc("GET /ws/{id}", s.handleWS)
	sub, _ := fs.Sub(web.FS, "dist")
	s.mux.Handle("/", http.FileServer(http.FS(sub)))
}
