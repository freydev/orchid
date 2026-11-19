package agent

import (
	"reflect"
	"testing"
)

func TestClaudePluginDir(t *testing.T) {
	c := claudeAgent{}

	t.Setenv("ORCHID_PLUGIN_DIR", "")
	if got := c.NewCommand("/x", "sid", Options{}); !reflect.DeepEqual(got, []string{"claude", "--session-id", "sid"}) {
		t.Fatalf("no-plugin NewCommand = %v", got)
	}
	if got := c.ResumeCommand("/x", "sid", Options{}); !reflect.DeepEqual(got, []string{"claude", "--resume", "sid"}) {
		t.Fatalf("no-plugin ResumeCommand = %v", got)
	}

	t.Setenv("ORCHID_PLUGIN_DIR", "/opt/orchid/plugins/orchid-status")
	if got := c.NewCommand("/x", "sid", Options{}); !reflect.DeepEqual(got, []string{"claude", "--session-id", "sid", "--plugin-dir", "/opt/orchid/plugins/orchid-status"}) {
		t.Fatalf("plugin NewCommand = %v", got)
	}
	if got := c.ResumeCommand("/x", "sid", Options{}); !reflect.DeepEqual(got, []string{"claude", "--resume", "sid", "--plugin-dir", "/opt/orchid/plugins/orchid-status"}) {
		t.Fatalf("plugin ResumeCommand = %v", got)
	}
}

func TestClaudeSkipPermissions(t *testing.T) {
	c := claudeAgent{}
	t.Setenv("ORCHID_PLUGIN_DIR", "")
	opts := Options{SkipPermissions: true}
	if got := c.NewCommand("/x", "sid", opts); !reflect.DeepEqual(got, []string{"claude", "--session-id", "sid", "--dangerously-skip-permissions"}) {
		t.Fatalf("skip NewCommand = %v", got)
	}
	if got := c.ResumeCommand("/x", "sid", opts); !reflect.DeepEqual(got, []string{"claude", "--resume", "sid", "--dangerously-skip-permissions"}) {
		t.Fatalf("skip ResumeCommand = %v", got)
	}
}

func TestCodexSkipPermissions(t *testing.T) {
	a := codexAgent{}
	got := a.NewCommand("/x", "sid", Options{SkipPermissions: true})
	if len(got) == 0 || got[len(got)-1] != "--dangerously-bypass-approvals-and-sandbox" {
		t.Fatalf("codex skip NewCommand missing bypass flag: %v", got)
	}
	rs := a.ResumeCommand("/x", "sid", Options{SkipPermissions: true})
	// bypass flag must precede the `resume` subcommand
	var bypassAt, resumeAt = -1, -1
	for i, s := range rs {
		if s == "--dangerously-bypass-approvals-and-sandbox" {
			bypassAt = i
		}
		if s == "resume" {
			resumeAt = i
		}
	}
	if bypassAt < 0 || resumeAt < 0 || bypassAt > resumeAt {
		t.Fatalf("codex skip ResumeCommand order wrong: %v", rs)
	}
}
