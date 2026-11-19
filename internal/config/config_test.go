package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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

func writeConfig(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	od := filepath.Join(dir, "orchid")
	if err := os.MkdirAll(od, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(od, "config.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIdleTimeout(t *testing.T) {
	cases := []struct {
		name string
		body string
		want time.Duration
	}{
		{"unset falls back to the default", `{}`, DefaultIdleTimeoutMin * time.Minute},
		{"explicit value", `{"idle_timeout_min":30}`, 30 * time.Minute},
		{"negative disables reaping", `{"idle_timeout_min":-1}`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writeConfig(t, tc.body)
			if got := IdleTimeout(); got != tc.want {
				t.Fatalf("IdleTimeout() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIdleTimeoutRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := Save(Config{IdleTimeoutMin: 45}); err != nil {
		t.Fatal(err)
	}
	if got := IdleTimeout(); got != 45*time.Minute {
		t.Fatalf("after Save, IdleTimeout() = %v", got)
	}
}
