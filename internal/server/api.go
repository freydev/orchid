package server

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"orchid/internal/agent"
	"orchid/internal/gitwt"
)

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.mgr.List())
}

type createReq struct {
	Project  string `json:"project"`
	Worktree string `json:"worktree"`
	Agent    string `json:"agent"`
	Name     string `json:"name"`
}

type renameReq struct {
	Name string `json:"name"`
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
	sess, err := s.mgr.Create(req.Project, req.Worktree, req.Agent, req.Name)
	if err != nil {
		if strings.Contains(err.Error(), "unknown agent") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": sess.ID, "name": strings.TrimSpace(req.Name), "agent": sess.Agent, "project": sess.Project,
		"worktree": sess.Worktree, "cwd": sess.Cwd, "status": "running", "alive": true,
	})
}

func (s *Server) handleRename(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req renameReq
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	if err := s.mgr.Rename(id, req.Name); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "name": strings.TrimSpace(req.Name)})
}

// handleShell reports the resolved shell launched for "shell" sessions, so the
// UI can label them with the real shell (e.g. fish) instead of a guess.
func (s *Server) handleShell(w http.ResponseWriter, r *http.Request) {
	p := agent.ShellPath()
	writeJSON(w, http.StatusOK, map[string]string{"path": p, "name": filepath.Base(p)})
}

func (s *Server) handlePrune(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"removed": s.mgr.PruneStopped()})
}

type statusReq struct {
	State string `json:"state"`
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.mgr.Get(id); !ok {
		// Get finds only live sessions; fall back to List for record presence.
		found := false
		for _, info := range s.mgr.List() {
			if info.ID == id {
				found = true
				break
			}
		}
		if !found {
			http.Error(w, "session not found", http.StatusNotFound)
			return
		}
	}
	var req statusReq
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	switch req.State {
	case "running", "done", "requires_action", "idle":
		s.mgr.Signal(id, req.State)
		s.mgr.SyncTitle(id) // a status change is a good moment to pick up claude's title
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "unknown state", http.StatusBadRequest)
	}
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.mgr.Stop(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

type sessChip struct {
	Agent    string `json:"agent"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Worktree string `json:"worktree"` // branch/display name
}
type projOut struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Path        string     `json:"path"`
	Worktrees   int        `json:"worktrees"`
	Sessions    int        `json:"sessions"`
	SessionList []sessChip `json:"session_list"` // worktree-order then session-order
}

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	projs := s.projects.List()
	all := s.mgr.List() // already sorted by (worktree, order, id)
	out := make([]projOut, 0, len(projs))
	for _, p := range projs {
		// worktree paths in saved order + path -> display name
		var wtPaths []string
		wtName := map[string]string{}
		if wts, err := gitwt.List(p.Path); err == nil {
			pos := make(map[string]int, len(p.WorktreeOrder))
			for i, path := range p.WorktreeOrder {
				pos[path] = i
			}
			sort.SliceStable(wts, func(i, j int) bool {
				pi, oki := pos[wts[i].Path]
				pj, okj := pos[wts[j].Path]
				if oki && okj {
					return pi < pj
				}
				return oki && !okj
			})
			for _, wt := range wts {
				wtPaths = append(wtPaths, wt.Path)
				n := wt.Branch
				if wt.IsMain {
					n = "main"
				}
				wtName[wt.Path] = n
			}
		}
		inOrder := make(map[string]bool, len(wtPaths))
		list := []sessChip{}
		for _, path := range wtPaths {
			inOrder[path] = true
			for _, inf := range all {
				if inf.Project == p.ID && inf.Worktree == path {
					list = append(list, sessChip{Agent: inf.Agent, Name: inf.Name, Status: inf.Status, Worktree: wtName[path]})
				}
			}
		}
		for _, inf := range all { // sessions in worktrees git no longer lists
			if inf.Project == p.ID && !inOrder[inf.Worktree] {
				list = append(list, sessChip{Agent: inf.Agent, Name: inf.Name, Status: inf.Status, Worktree: inf.Worktree})
			}
		}
		out = append(out, projOut{
			ID: p.ID, Name: p.Name, Path: p.Path,
			Worktrees: len(wtPaths), Sessions: len(list), SessionList: list,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

type orderReq struct {
	IDs   []string `json:"ids"`
	Paths []string `json:"paths"`
}

func (s *Server) handleProjectsOrder(w http.ResponseWriter, r *http.Request) {
	var req orderReq
	_ = json.NewDecoder(r.Body).Decode(&req)
	if err := s.projects.Reorder(req.IDs); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleWorktreesOrder(w http.ResponseWriter, r *http.Request) {
	var req orderReq
	_ = json.NewDecoder(r.Body).Decode(&req)
	if err := s.projects.SetWorktreeOrder(r.PathValue("id"), req.Paths); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSessionsOrder(w http.ResponseWriter, r *http.Request) {
	var req orderReq
	_ = json.NewDecoder(r.Body).Decode(&req)
	s.mgr.Reorder(req.IDs)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSessionsGridOrder(w http.ResponseWriter, r *http.Request) {
	var req orderReq
	_ = json.NewDecoder(r.Body).Decode(&req)
	s.mgr.ReorderGrid(req.IDs)
	w.WriteHeader(http.StatusNoContent)
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
	id := r.PathValue("id")
	s.mgr.KillByProject(id) // kill + clear the project's sessions; git worktrees untouched
	if err := s.projects.Remove(id); err != nil {
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
	// apply the user's saved worktree order (unknown paths keep git order, after)
	pos := make(map[string]int, len(proj.WorktreeOrder))
	for i, p := range proj.WorktreeOrder {
		pos[p] = i
	}
	sort.SliceStable(wts, func(i, j int) bool {
		pi, oki := pos[wts[i].Path]
		pj, okj := pos[wts[j].Path]
		if oki && okj {
			return pi < pj
		}
		return oki && !okj
	})
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

type gitInfo struct {
	IsRepo        bool   `json:"is_repo"`
	HasOrigin     bool   `json:"has_origin"`
	DefaultBranch string `json:"default_branch"`
	Base          string `json:"base"`
}

func (s *Server) handleGitInfo(w http.ResponseWriter, r *http.Request) {
	proj, ok := s.projects.Get(r.PathValue("id"))
	if !ok {
		http.Error(w, "unknown project", http.StatusNotFound)
		return
	}
	if !gitwt.IsRepo(proj.Path) {
		writeJSON(w, http.StatusOK, gitInfo{IsRepo: false})
		return
	}
	def := gitwt.DefaultBranch(proj.Path)
	hasOrigin := gitwt.HasOrigin(proj.Path)
	base := def
	if hasOrigin {
		base = "origin/" + def
	}
	writeJSON(w, http.StatusOK, gitInfo{IsRepo: true, HasOrigin: hasOrigin, DefaultBranch: def, Base: base})
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
	if !gitwt.IsRepo(proj.Path) {
		http.Error(w, "project is not a git repository", http.StatusBadRequest)
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
