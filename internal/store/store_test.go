package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultPathUsesConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/cfg")
	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	if got != "/cfg/orchid/sessions.json" {
		t.Fatalf("with XDG_CONFIG_HOME: got %q", got)
	}

	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/home/tester")
	got, err = DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	if got != "/home/tester/.config/orchid/sessions.json" {
		t.Fatalf("fallback: got %q", got)
	}
}

func TestLoadMissingReturnsEmpty(t *testing.T) {
	recs, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("Load missing: %v", err)
	}
	if len(recs) != 0 {
		t.Fatalf("expected empty, got %v", recs)
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "sessions.json")
	in := []Record{
		{ID: "a1", Agent: "claude", Cwd: "/tmp", Tmux: "orchid-a1", AgentSessionID: "u1", Created: time.Unix(1700000000, 0).UTC(), Status: "running"},
		{ID: "b2", Agent: "shell", Cwd: "/x", Tmux: "orchid-b2", AgentSessionID: "u2", Created: time.Unix(1700000001, 0).UTC(), Status: "stopped"},
	}
	if err := Save(path, in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	out, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(out) != 2 || out[0].ID != "a1" || out[1].Status != "stopped" || out[0].AgentSessionID != "u1" {
		t.Fatalf("roundtrip mismatch: %+v", out)
	}
}

func TestSaveAtomicNoTempLeft(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sessions.json")
	if err := Save(path, []Record{{ID: "z"}}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "sessions.json" {
			t.Fatalf("unexpected leftover file: %s", e.Name())
		}
	}
}
