package agent

import "os"

type claudeAgent struct{}

func (claudeAgent) Name() string { return "claude" }

// withPluginDir appends --plugin-dir <ORCHID_PLUGIN_DIR> when that env var is
// set, so claude loads the bundled orchid-status plugin (hooks + MCP).
func withPluginDir(argv []string) []string {
	if d := os.Getenv("ORCHID_PLUGIN_DIR"); d != "" {
		return append(argv, "--plugin-dir", d)
	}
	return argv
}

// withSkipPerms appends --dangerously-skip-permissions when opts ask for it.
func withSkipPerms(argv []string, opts Options) []string {
	if opts.SkipPermissions {
		return append(argv, "--dangerously-skip-permissions")
	}
	return argv
}

func (claudeAgent) NewCommand(cwd, agentSessionID string, opts Options) []string {
	return withSkipPerms(withPluginDir([]string{"claude", "--session-id", agentSessionID}), opts)
}

func (claudeAgent) ResumeCommand(cwd, agentSessionID string, opts Options) []string {
	return withSkipPerms(withPluginDir([]string{"claude", "--resume", agentSessionID}), opts)
}
