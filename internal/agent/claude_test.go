package agent

import (
	"reflect"
	"testing"
)

func TestClaudePluginDir(t *testing.T) {
	c := claudeAgent{}

	t.Setenv("ORCHID_PLUGIN_DIR", "")
	if got := c.NewCommand("/x", "sid"); !reflect.DeepEqual(got, []string{"claude", "--session-id", "sid"}) {
		t.Fatalf("no-plugin NewCommand = %v", got)
	}
	if got := c.ResumeCommand("/x", "sid"); !reflect.DeepEqual(got, []string{"claude", "--resume", "sid"}) {
		t.Fatalf("no-plugin ResumeCommand = %v", got)
	}

	t.Setenv("ORCHID_PLUGIN_DIR", "/opt/orchid/plugins/orchid-status")
	if got := c.NewCommand("/x", "sid"); !reflect.DeepEqual(got, []string{"claude", "--session-id", "sid", "--plugin-dir", "/opt/orchid/plugins/orchid-status"}) {
		t.Fatalf("plugin NewCommand = %v", got)
	}
	if got := c.ResumeCommand("/x", "sid"); !reflect.DeepEqual(got, []string{"claude", "--resume", "sid", "--plugin-dir", "/opt/orchid/plugins/orchid-status"}) {
		t.Fatalf("plugin ResumeCommand = %v", got)
	}
}
