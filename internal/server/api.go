package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"orchid/internal/gitwt"
)

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.mgr.List())
}

type createReq struct {
	Project  string `json:"project"`
	Worktree string `json:"worktree"`
	Agent    string `json:"agent"`
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createReq
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	if req.Agent == "" || req.Project == "" || req.Worktree == "" {
		http.Error(w, "project, worktree and agent are required", http.StatusBadRequest)
		return
	}
	proj, ok := s.projects.Get(req.Project)
	if !ok {
		http.Error(w, "unknown project", http.StatusNotFound)
		return
	}
	wts, err := gitwt.List(proj.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	valid := false
	for _, wt := range wts {
		if wt.Path == req.Worktree {
			valid = true
			break
		}
	}
	if !valid {
		http.Error(w, "worktree not found in project", http.StatusBadRequest)
		return
	}
	sess, err := s.mgr.Create(req.Project, req.Worktree, req.Agent)
	if err != nil {
		if strings.Contains(err.Error(), "unknown agent") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": sess.ID, "agent": sess.Agent, "project": sess.Project,
		"worktree": sess.Worktree, "cwd": sess.Cwd, "status": "running", "alive": true,
	})
}

func (s *Server) handleKill(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.mgr.Kill(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sess, err := s.mgr.Restart(id)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "not found"):
			http.Error(w, err.Error(), http.StatusNotFound)
		case strings.Contains(err.Error(), "already running"):
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":     sess.ID,
		"agent":  sess.Agent,
		"cwd":    sess.Cwd,
		"status": "running",
		"alive":  true,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type addProjectReq struct {
	Name string `json:"name"`
	Path string `json:"path"`
}
type addWorktreeReq struct {
	Branch string `json:"branch"`
}
type rmWorktreeReq struct {
	Path string `json:"path"`
}
type wtInfo struct {
	Path   string `json:"path"`
	Branch string `json:"branch"`
	Name   string `json:"name"`
	IsMain bool   `json:"is_main"`
}

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.projects.List())
}

func (s *Server) handleAddProject(w http.ResponseWriter, r *http.Request) {
	var req addProjectReq
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Path == "" {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}
	p, err := s.projects.Add(req.Name, req.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleRemoveProject(w http.ResponseWriter, r *http.Request) {
	if err := s.projects.Remove(r.PathValue("id")); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleWorktrees(w http.ResponseWriter, r *http.Request) {
	proj, ok := s.projects.Get(r.PathValue("id"))
	if !ok {
		http.Error(w, "unknown project", http.StatusNotFound)
		return
	}
	wts, err := gitwt.List(proj.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]wtInfo, 0, len(wts))
	for _, wt := range wts {
		name := wt.Branch
		if wt.IsMain {
			name = "main"
		}
		out = append(out, wtInfo{Path: wt.Path, Branch: wt.Branch, Name: name, IsMain: wt.IsMain})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleAddWorktree(w http.ResponseWriter, r *http.Request) {
	proj, ok := s.projects.Get(r.PathValue("id"))
	if !ok {
		http.Error(w, "unknown project", http.StatusNotFound)
		return
	}
	var req addWorktreeReq
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Branch == "" {
		http.Error(w, "branch is required", http.StatusBadRequest)
		return
	}
	path, err := gitwt.Add(proj.Path, req.Branch)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	writeJSON(w, http.StatusOK, wtInfo{Path: path, Branch: req.Branch, Name: req.Branch, IsMain: false})
}

func (s *Server) handleRemoveWorktree(w http.ResponseWriter, r *http.Request) {
	proj, ok := s.projects.Get(r.PathValue("id"))
	if !ok {
		http.Error(w, "unknown project", http.StatusNotFound)
		return
	}
	var req rmWorktreeReq
	_ = json.NewDecoder(r.Body).Decode(&req)
	wts, err := gitwt.List(proj.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var target *gitwt.Worktree
	for i := range wts {
		if wts[i].Path == req.Path {
			target = &wts[i]
			break
		}
	}
	if target == nil {
		http.Error(w, "worktree not found", http.StatusNotFound)
		return
	}
	if target.IsMain {
		http.Error(w, "cannot remove the main worktree", http.StatusBadRequest)
		return
	}
	s.mgr.KillByWorktree(target.Path)
	if err := gitwt.Remove(proj.Path, target.Path); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if target.Branch != "" {
		_ = gitwt.DeleteBranch(proj.Path, target.Branch)
	}
	w.WriteHeader(http.StatusNoContent)
}
