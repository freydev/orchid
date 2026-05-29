package agent

type claudeAgent struct{}

func (claudeAgent) Name() string { return "claude" }

func (claudeAgent) NewCommand(cwd, agentSessionID string) []string {
	return []string{"claude", "--session-id", agentSessionID}
}

func (claudeAgent) ResumeCommand(cwd, agentSessionID string) []string {
	return []string{"claude", "--resume", agentSessionID}
}
