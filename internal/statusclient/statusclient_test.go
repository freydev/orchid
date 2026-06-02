package statusclient

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPost(t *testing.T) {
	var gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := Post(srv.URL, "sess-1", "requires_action"); err != nil {
		t.Fatalf("Post: %v", err)
	}
	if gotPath != "/api/sessions/sess-1/status" {
		t.Fatalf("path = %q", gotPath)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(gotBody), &body); err != nil {
		t.Fatalf("body not json: %v", err)
	}
	if body["state"] != "requires_action" {
		t.Fatalf("body = %v", body)
	}
}

func TestPostEmptyBaseURL(t *testing.T) {
	if err := Post("", "x", "running"); err != nil {
		t.Fatalf("expected nil error for empty baseURL, got %v", err)
	}
}
