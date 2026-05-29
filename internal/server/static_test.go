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
	files := []string{"index.html", "app.js", "vendor/xterm.js", "vendor/xterm.css", "vendor/addon-fit.js"}
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
		{"/vendor/xterm.js", 200},
		{"/vendor/xterm.css", 200},
		{"/vendor/addon-fit.js", 200},
		{"/app.js", 200},
		{"/", 200},
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

	// Verify index.html references xterm.js
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	count := strings.Count(string(body), "xterm.js")
	if count == 0 {
		t.Error("index.html does not reference xterm.js")
	} else {
		t.Logf("OK index.html xterm.js reference count: %d", count)
	}
}
