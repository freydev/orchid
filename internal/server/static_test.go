package server

import (
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"orchid/internal/project"
	"orchid/internal/session"
	"orchid/web"
)

func TestEmbedFSContainsExpectedFiles(t *testing.T) {
	files := []string{"dist/index.html", "dist/bundle.js", "dist/bundle.css", "dist/orchid-logo.svg"}
	for _, f := range files {
		data, err := fs.ReadFile(web.FS, f)
		if err != nil {
			t.Errorf("missing embedded file %s: %v", f, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("embedded file %s is empty", f)
		}
		t.Logf("OK: %s (%d bytes)", f, len(data))
	}
}

func TestStaticFilesServed(t *testing.T) {
	mgr := session.NewManager(t.TempDir() + "/sessions.json")
	defer mgr.KillAll()
	h := New(mgr, project.New(filepath.Join(t.TempDir(), "projects.json")))
	srv := httptest.NewServer(h)
	defer srv.Close()

	tests := []struct {
		path string
		want int
	}{
		{"/bundle.js", 200},
		{"/bundle.css", 200},
		{"/orchid-logo.svg", 200},
		{"/", 200},
		{"/app.js", 404},          // vanilla asset removed by Svelte migration
		{"/vendor/xterm.js", 404}, // xterm now bundled into bundle.js
	}
	for _, tt := range tests {
		resp, err := http.Get(srv.URL + tt.path)
		if err != nil {
			t.Errorf("GET %s: %v", tt.path, err)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != tt.want {
			t.Errorf("GET %s: status %d, want %d", tt.path, resp.StatusCode, tt.want)
		} else {
			t.Logf("OK HTTP GET %s => %d (%d bytes)", tt.path, resp.StatusCode, len(body))
		}
	}

	// Verify index.html loads the Svelte bundle
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "bundle.js") {
		t.Error("index.html does not reference bundle.js")
	}
}
