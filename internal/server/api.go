package server

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.mgr.List())
}

type createReq struct {
	Agent string `json:"agent"`
	Cwd   string `json:"cwd"`
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createReq
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req) // empty body -> zero struct
	}
	if req.Agent == "" {
		http.Error(w, "missing agent", http.StatusBadRequest)
		return
	}
	sess, err := s.mgr.Create(req.Agent, req.Cwd)
	if err != nil {
		if strings.Contains(err.Error(), "unknown agent") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
