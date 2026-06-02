package agent

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestClaudeCommands(t *testing.T) {
	a, ok := Get("claude")
	if !ok {
		t.Fatal("claude not registered")
	}
	if a.Name() != "claude" {
		t.Fatalf("name = %q", a.Name())
	}
	got := a.NewCommand("/tmp", "sid-123")
	want := []string{"claude", "--session-id", "sid-123"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NewCommand = %v, want %v", got, want)
	}
	got = a.ResumeCommand("/tmp", "sid-123")
	want = []string{"claude", "--resume", "sid-123"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ResumeCommand = %v, want %v", got, want)
	}
}

func TestCodexCommands(t *testing.T) {
	a, ok := Get("codex")
	if !ok {
		t.Fatal("codex not registered")
	}
	// NewCommand starts with "codex" and registers the orchid MCP server via -c.
	got := a.NewCommand("/tmp", "ignored")
	if got[0] != "codex" {
		t.Fatalf("NewCommand[0] = %q", got[0])
	}
	if !containsArg(got, `mcp_servers.orchid.args=["mcp"]`) {
		t.Fatalf("NewCommand missing orchid MCP flag: %v", got)
	}
	// ResumeCommand keeps the `resume --last` subcommand after the -c flags.
	rs := a.ResumeCommand("/tmp", "ignored")
	if rs[len(rs)-2] != "resume" || rs[len(rs)-1] != "--last" {
		t.Fatalf("ResumeCommand tail = %v", rs)
	}
	if !containsArg(rs, `mcp_servers.orchid.args=["mcp"]`) {
		t.Fatalf("ResumeCommand missing orchid MCP flag: %v", rs)
	}
}

func containsArg(argv []string, want string) bool {
	for _, a := range argv {
		if a == want {
			return true
		}
	}
	return false
}

func TestShellCommands(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // no config file -> fall back to $SHELL
	t.Setenv("SHELL", "/bin/zsh")
	a, ok := Get("shell")
	if !ok {
		t.Fatal("shell not registered")
	}
	if got := a.NewCommand("/tmp", ""); !reflect.DeepEqual(got, []string{"/bin/zsh"}) {
		t.Fatalf("NewCommand = %v", got)
	}
	if got := a.ResumeCommand("/tmp", ""); got != nil {
		t.Fatalf("ResumeCommand = %v, want nil", got)
	}
}

func TestShellConfigOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("SHELL", "/bin/zsh")
	od := filepath.Join(dir, "orchid")
	if err := os.MkdirAll(od, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(od, "config.json"), []byte(`{"shell":"/opt/homebrew/bin/fish"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ShellPath(); got != "/opt/homebrew/bin/fish" {
		t.Fatalf("ShellPath = %q, want configured fish (config beats $SHELL)", got)
	}
}

func TestClaudeTitle(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".claude", "projects", "proj")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"type":"user","message":{}}` + "\n" +
		`{"type":"ai-title","aiTitle":"First Title","sessionId":"u1"}` + "\n" +
		`{"type":"assistant","message":{}}` + "\n" +
		`{"type":"ai-title","aiTitle":"Latest Title","sessionId":"u1"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "u1.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ClaudeTitle("u1"); got != "Latest Title" {
		t.Fatalf("ClaudeTitle = %q, want Latest Title", got)
	}
	if got := ClaudeTitle("missing"); got != "" {
		t.Fatalf("missing transcript should be empty, got %q", got)
	}
	if got := ClaudeTitle(""); got != "" {
		t.Fatalf("empty id should be empty, got %q", got)
	}
}

func TestUnknownAgent(t *testing.T) {
	if _, ok := Get("nope"); ok {
		t.Fatal("expected unknown agent to be absent")
	}
}

func TestNamesSorted(t *testing.T) {
	names := Names()
	want := []string{"claude", "codex", "shell"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("Names = %v, want %v", names, want)
	}
}

func TestResolveCwd(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	if got := ResolveCwd(""); got != "/home/tester" {
		t.Fatalf("ResolveCwd(\"\") = %q, want /home/tester", got)
	}
	if got := ResolveCwd("/explicit"); got != "/explicit" {
		t.Fatalf("ResolveCwd = %q", got)
	}
}
