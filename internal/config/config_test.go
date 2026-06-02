package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if got := Shell(); got != "" {
		t.Fatalf("Shell() = %q, want empty for missing file", got)
	}
}

func TestLoadShell(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	od := filepath.Join(dir, "orchid")
	if err := os.MkdirAll(od, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(od, "config.json"), []byte(`{"shell":"  /opt/homebrew/bin/fish  "}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Shell(); got != "/opt/homebrew/bin/fish" {
		t.Fatalf("Shell() = %q, want trimmed fish path", got)
	}
}

func TestLoadInvalid(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	od := filepath.Join(dir, "orchid")
	_ = os.MkdirAll(od, 0o755)
	_ = os.WriteFile(filepath.Join(od, "config.json"), []byte(`{bad json`), 0o644)
	if got := Shell(); got != "" {
		t.Fatalf("Shell() = %q, want empty for invalid file", got)
	}
}
