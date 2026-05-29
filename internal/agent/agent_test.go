package agent

import (
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
	if got := a.NewCommand("/tmp", "ignored"); !reflect.DeepEqual(got, []string{"codex"}) {
		t.Fatalf("NewCommand = %v", got)
	}
	if got := a.ResumeCommand("/tmp", "ignored"); !reflect.DeepEqual(got, []string{"codex", "resume", "--last"}) {
		t.Fatalf("ResumeCommand = %v", got)
	}
}

func TestShellCommands(t *testing.T) {
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
