package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"

	"orchid/internal/agent"
	"orchid/internal/config"
	"orchid/internal/gitwt"
	"orchid/internal/notify"
	"orchid/internal/project"
	"orchid/internal/session"
)

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.mgr.List())
}

type createReq struct {
	Project   string `json:"project"`
	Worktree  string `json:"worktree"`
	Agent     string `json:"agent"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	Shell     string `json:"shell"`
	Command   string `json:"command"`
	ResumeID  string `json:"resume_id"`
	Dangerous bool   `json:"dangerous"`
	Cwd       string `json:"cwd"` // launch dir override (subdir of worktree)
	Group     string `json:"group"`
}

// sessPatchReq is the PATCH body for a session. Pointer fields distinguish
// "absent" (leave unchanged) from "" (clear): the quick-rename modal sends only
// name; the settings modal sends name+color+shell.
type sessPatchReq struct {
	Name  *string `json:"name"`
	Color *string `json:"color"`
	Shell *string `json:"shell"`
	Group *string `json:"group"`
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
	if !hasWorktree(wts, req.Worktree) {
		http.Error(w, "worktree not found in project", http.StatusBadRequest)
		return
	}
	if req.Group != "" && !groupIn(proj, req.Group, req.Worktree) {
		http.Error(w, "group not found in worktree", http.StatusBadRequest)
		return
	}
	sess, err := s.mgr.Create(req.Project, req.Worktree, req.Agent, session.CreateOpts{
		Name: req.Name, Color: req.Color, Shell: req.Shell, Command: req.Command, ResumeID: req.ResumeID, Dangerous: req.Dangerous, Cwd: req.Cwd, Group: req.Group,
	})
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
		"color": strings.TrimSpace(req.Color), "shell": strings.TrimSpace(req.Shell), "command": strings.TrimSpace(req.Command),
		"group": req.Group,
	})
}

func (s *Server) handleRename(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req sessPatchReq
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	if req.Group != nil && *req.Group != "" {
		// a session may only join a group of its own project + worktree
		var inf *session.Info
		for _, i := range s.mgr.List() {
			if i.ID == id {
				inf = &i
				break
			}
		}
		if inf == nil {
			http.Error(w, "session not found", http.StatusNotFound)
			return
		}
		proj, ok := s.projects.Get(inf.Project)
		if !ok || !groupIn(proj, *req.Group, inf.Worktree) {
			http.Error(w, "group not found in session's worktree", http.StatusBadRequest)
			return
		}
	}
	if err := s.mgr.Update(id, session.SessUpdate{Name: req.Name, Color: req.Color, Shell: req.Shell, Group: req.Group}); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id})
}

// handleClaudeSessions lists resumable claude conversations recorded for the
// ?cwd=<worktree path>, so the New Session modal can offer to import one.
func (s *Server) handleClaudeSessions(w http.ResponseWriter, r *http.Request) {
	cwd := r.URL.Query().Get("cwd")
	writeJSON(w, http.StatusOK, agent.ClaudeSessions(cwd))
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

// handleNotify shows a desktop notification that this session's agent finished.
// Fired by the Stop hook and the MCP notify tool.
// sessionURL builds the dashboard deep link that focuses a session's terminal,
// opened when a desktop notification is clicked. Empty when project is unknown.
// (host/id come from trusted local state, so no escaping is needed.)
func (s *Server) sessionURL(project, id string) string {
	if project == "" {
		return ""
	}
	base := strings.TrimRight(strings.TrimSpace(config.Load().BaseURL), "/")
	if base == "" {
		host := s.host
		if host == "" || host == "0.0.0.0" || host == "::" {
			host = "127.0.0.1"
		}
		base = fmt.Sprintf("http://%s:%d", host, s.port)
	}
	// Use a query (not a #fragment): a PWA "navigate-existing" launch drops the
	// fragment but keeps the query. The frontend converts ?s into the route.
	return base + fmt.Sprintf("/?s=%s~%s", project, id)
}

func (s *Server) handleNotify(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Message string `json:"message"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	name, project := "", ""
	for _, inf := range s.mgr.List() {
		if inf.ID == id {
			name = strings.TrimSpace(inf.Name)
			if name == "" {
				name = inf.Agent
			}
			project = inf.Project
			break
		}
	}
	title := "Orchid"
	if name != "" {
		title = "Orchid · " + name
	}
	body := strings.TrimSpace(req.Message)
	if body == "" {
		body = "Agent finished working"
	}
	notify.Notify(title, body, s.sessionURL(project, id))
	w.WriteHeader(http.StatusNoContent)
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
	Name        string           `json:"name"`
	Path        string           `json:"path"`
	Color       string           `json:"color"`
	Description string           `json:"description"`
	Shell       string           `json:"shell"`
	BaseBranch  string           `json:"base_branch"`
	Commands    []project.Preset `json:"commands"`
}
type updateProjectReq struct {
	Name        string           `json:"name"`
	Color       string           `json:"color"`
	Description string           `json:"description"`
	Shell       string           `json:"shell"`
	BaseBranch  string           `json:"base_branch"`
	Commands    []project.Preset `json:"commands"`
}
type addWorktreeReq struct {
	Branch string `json:"branch"`
	From   string `json:"from"`  // base branch for a new branch; "" -> project base/default
	Fetch  *bool  `json:"fetch"` // fetch origin/<from> first; nil -> true (default)
}
type rmWorktreeReq struct {
	Path string `json:"path"`
}
type wtInfo struct {
	Path    string          `json:"path"`
	Branch  string          `json:"branch"`
	Name    string          `json:"name"`
	IsMain  bool            `json:"is_main"`
	Primary bool            `json:"primary,omitempty"` // git's primary worktree: never removable
	Groups  []project.Group `json:"groups"`            // session groups of this worktree, creation order
}

type sessChip struct {
	Agent    string `json:"agent"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Worktree string `json:"worktree"`          // branch/display name
	Color    string `json:"color,omitempty"`   // session accent override
	Command  string `json:"command,omitempty"` // preset command, if any
}
type projOut struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Path        string           `json:"path"`
	Color       string           `json:"color,omitempty"`
	Description string           `json:"description,omitempty"`
	Shell       string           `json:"shell,omitempty"`
	BaseBranch  string           `json:"base_branch,omitempty"`
	Commands    []project.Preset `json:"commands,omitempty"`
	Branch      string           `json:"branch,omitempty"` // current branch of the main worktree
	Worktrees   int              `json:"worktrees"`
	Sessions    int              `json:"sessions"`
	SessionList []sessChip       `json:"session_list"`      // worktree-order then session-order
	Problem     string           `json:"problem,omitempty"` // "folder missing" / "not a git repo" — empty when healthy
}

// projectProblem reports why a project's path is unusable, or "" when healthy:
// the folder is gone, or it exists but has no .git entry.
func projectProblem(path string) string {
	fi, err := os.Stat(path)
	if err != nil || !fi.IsDir() {
		return "folder missing"
	}
	if _, err := os.Stat(filepath.Join(path, ".git")); err != nil {
		return "not a git repo"
	}
	return ""
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
					n = "root"
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
					list = append(list, sessChip{Agent: inf.Agent, Name: inf.Name, Status: inf.Status, Worktree: wtName[path], Color: inf.Color, Command: inf.Command})
				}
			}
		}
		for _, inf := range all { // sessions in worktrees git no longer lists
			if inf.Project == p.ID && !inOrder[inf.Worktree] {
				list = append(list, sessChip{Agent: inf.Agent, Name: inf.Name, Status: inf.Status, Worktree: inf.Worktree, Color: inf.Color, Command: inf.Command})
			}
		}
		out = append(out, projOut{
			ID: p.ID, Name: p.Name, Path: p.Path,
			Color: p.Color, Description: p.Description, Shell: p.Shell, BaseBranch: p.BaseBranch, Commands: p.Commands,
			Branch:    gitwt.CurrentBranch(p.Path),
			Worktrees: len(wtPaths), Sessions: len(list), SessionList: list,
			Problem: projectProblem(p.Path),
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
	// apply optional settings chosen at creation
	if req.Color != "" || req.Description != "" || req.Shell != "" || req.BaseBranch != "" || len(req.Commands) > 0 {
		if err := s.projects.Update(p.ID, p.Name, req.Color, req.Description, req.Shell, req.BaseBranch, req.Commands); err == nil {
			p, _ = s.projects.Get(p.ID)
		}
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req updateProjectReq
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	if err := s.projects.Update(id, req.Name, req.Color, req.Description, req.Shell, req.BaseBranch, req.Commands); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	p, _ := s.projects.Get(id)
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
			name = "root"
		}
		groups := []project.Group{}
		for _, g := range proj.Groups {
			if g.Worktree == wt.Path {
				groups = append(groups, g)
			}
		}
		out = append(out, wtInfo{Path: wt.Path, Branch: wt.Branch, Name: name, IsMain: wt.IsMain, Primary: wt.Primary, Groups: groups})
	}
	writeJSON(w, http.StatusOK, out)
}

type gitInfo struct {
	IsRepo        bool     `json:"is_repo"`
	HasOrigin     bool     `json:"has_origin"`
	DefaultBranch string   `json:"default_branch"`
	Base          string   `json:"base"`
	Branches      []string `json:"branches"` // local branches, for the "from" picker
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
	// base reflects the project's configured base branch (falling back to the
	// repo default), so the worktree modal shows the real start point.
	base := gitwt.BaseRef(proj.Path, proj.BaseBranch)
	writeJSON(w, http.StatusOK, gitInfo{IsRepo: true, HasOrigin: hasOrigin, DefaultBranch: def, Base: base, Branches: gitwt.Branches(proj.Path)})
}

// handleDiff returns the diff for ?worktree=<path> of a project: uncommitted
// changes vs HEAD, or HEAD~1..HEAD when the tree is clean.
func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	proj, ok := s.projects.Get(r.PathValue("id"))
	if !ok {
		http.Error(w, "unknown project", http.StatusNotFound)
		return
	}
	wtPath := r.URL.Query().Get("worktree")
	if wtPath == "" {
		http.Error(w, "worktree is required", http.StatusBadRequest)
		return
	}
	// validate the worktree belongs to this project
	wts, err := gitwt.List(proj.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !hasWorktree(wts, wtPath) {
		http.Error(w, "worktree not found in project", http.StatusBadRequest)
		return
	}
	d, err := gitwt.WorktreeDiff(wtPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, d)
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
	// base: per-creation "from" override, else the project's configured base
	base := strings.TrimSpace(req.From)
	if base == "" {
		base = proj.BaseBranch
	}
	fetch := true // default on
	if req.Fetch != nil {
		fetch = *req.Fetch
	}
	path, err := gitwt.Add(proj.Path, req.Branch, base, fetch)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	writeJSON(w, http.StatusOK, wtInfo{Path: path, Branch: req.Branch, Name: req.Branch, IsMain: false, Groups: []project.Group{}})
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
	if target.IsMain || target.Primary {
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
	_ = s.projects.RemoveWorktreeGroups(proj.ID, target.Path)
	w.WriteHeader(http.StatusNoContent)
}

type emptyWt struct {
	Path   string `json:"path"`
	Branch string `json:"branch"`
}

// emptyWorktrees lists the project's removable worktrees whose removal loses
// nothing: no orchid session (running or stopped) is bound to it, and git
// reports no uncommitted/untracked changes and no commits that exist only
// locally (see gitwt.HasWork).
func (s *Server) emptyWorktrees(proj project.Project) ([]gitwt.Worktree, error) {
	wts, err := gitwt.List(proj.Path)
	if err != nil {
		return nil, err
	}
	used := map[string]bool{}
	for _, in := range s.mgr.List() {
		used[in.Worktree] = true
	}
	base := gitwt.BaseRef(proj.Path, proj.BaseBranch)
	// git status on a big repo is slow (~1s each): check candidates in parallel
	work := make([]bool, len(wts))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i, wt := range wts {
		if wt.IsMain || wt.Primary || used[wt.Path] {
			work[i] = true
			continue
		}
		wg.Add(1)
		go func(i int, wt gitwt.Worktree) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			work[i] = gitwt.HasWork(proj.Path, wt.Path, wt.Branch, base)
		}(i, wt)
	}
	wg.Wait()
	var out []gitwt.Worktree
	for i, wt := range wts {
		if !work[i] {
			out = append(out, wt)
		}
	}
	return out, nil
}

func (s *Server) handleEmptyWorktrees(w http.ResponseWriter, r *http.Request) {
	proj, ok := s.projects.Get(r.PathValue("id"))
	if !ok {
		http.Error(w, "unknown project", http.StatusNotFound)
		return
	}
	wts, err := s.emptyWorktrees(proj)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]emptyWt, 0, len(wts))
	for _, wt := range wts {
		out = append(out, emptyWt{Path: wt.Path, Branch: wt.Branch})
	}
	writeJSON(w, http.StatusOK, out)
}

type pruneWtReq struct {
	Paths []string `json:"paths"`
}

// handlePruneWorktrees removes the requested worktrees (and their branches),
// but only those that are still empty right now: the set is recomputed, so a
// worktree that gained a session or changes since the user confirmed is kept.
func (s *Server) handlePruneWorktrees(w http.ResponseWriter, r *http.Request) {
	proj, ok := s.projects.Get(r.PathValue("id"))
	if !ok {
		http.Error(w, "unknown project", http.StatusNotFound)
		return
	}
	var req pruneWtReq
	_ = json.NewDecoder(r.Body).Decode(&req)
	wts, err := s.emptyWorktrees(proj)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	want := map[string]bool{}
	for _, p := range req.Paths {
		want[p] = true
	}
	removed := []string{}
	var errs []string
	for _, wt := range wts {
		if !want[wt.Path] {
			continue
		}
		if err := gitwt.Remove(proj.Path, wt.Path); err != nil {
			errs = append(errs, err.Error())
			continue
		}
		_ = gitwt.DeleteBranch(proj.Path, wt.Branch)
		_ = s.projects.RemoveWorktreeGroups(proj.ID, wt.Path)
		removed = append(removed, wt.Path)
	}
	writeJSON(w, http.StatusOK, map[string]any{"removed": removed, "errors": errs})
}

func hasWorktree(wts []gitwt.Worktree, path string) bool {
	for _, wt := range wts {
		if wt.Path == path {
			return true
		}
	}
	return false
}

// groupIn reports whether gid is a group of proj bound to worktree.
func groupIn(proj project.Project, gid, worktree string) bool {
	for _, g := range proj.Groups {
		if g.ID == gid {
			return g.Worktree == worktree
		}
	}
	return false
}

type groupReq struct {
	Worktree string `json:"worktree"`
	Name     string `json:"name"`
}

func (s *Server) handleAddGroup(w http.ResponseWriter, r *http.Request) {
	proj, ok := s.projects.Get(r.PathValue("id"))
	if !ok {
		http.Error(w, "unknown project", http.StatusNotFound)
		return
	}
	var req groupReq
	_ = json.NewDecoder(r.Body).Decode(&req)
	wts, err := gitwt.List(proj.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !hasWorktree(wts, req.Worktree) {
		http.Error(w, "worktree not found in project", http.StatusBadRequest)
		return
	}
	g, err := s.projects.AddGroup(proj.ID, req.Worktree, req.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (s *Server) handleRenameGroup(w http.ResponseWriter, r *http.Request) {
	var req groupReq
	_ = json.NewDecoder(r.Body).Decode(&req)
	if strings.TrimSpace(req.Name) == "" {
		http.Error(w, "group name is required", http.StatusBadRequest)
		return
	}
	if err := s.projects.RenameGroup(r.PathValue("id"), r.PathValue("gid"), req.Name); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleRemoveGroup deletes a group; its sessions survive, moved to ungrouped.
func (s *Server) handleRemoveGroup(w http.ResponseWriter, r *http.Request) {
	gid := r.PathValue("gid")
	if err := s.projects.RemoveGroup(r.PathValue("id"), gid); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	s.mgr.Ungroup(gid)
	w.WriteHeader(http.StatusNoContent)
}

// handleClearGroup kills every session of a group; the group itself stays.
func (s *Server) handleClearGroup(w http.ResponseWriter, r *http.Request) {
	proj, ok := s.projects.Get(r.PathValue("id"))
	gid := r.PathValue("gid")
	if !ok || !slices.ContainsFunc(proj.Groups, func(g project.Group) bool { return g.ID == gid }) {
		http.Error(w, "group not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"killed": s.mgr.KillByGroup(gid)})
}
