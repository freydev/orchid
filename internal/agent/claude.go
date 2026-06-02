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

func (claudeAgent) NewCommand(cwd, agentSessionID string) []string {
	return withPluginDir([]string{"claude", "--session-id", agentSessionID})
}

func (claudeAgent) ResumeCommand(cwd, agentSessionID string) []string {
	return withPluginDir([]string{"claude", "--resume", agentSessionID})
}
