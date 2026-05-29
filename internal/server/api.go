package server

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.mgr.List())
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	sess, err := s.mgr.Create()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": sess.ID, "alive": sess.Alive()})
}

func (s *Server) handleKill(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.mgr.Kill(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
