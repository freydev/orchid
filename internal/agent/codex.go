package agent

// codexAgent: codex cannot bind a caller-supplied session id at start (it
// generates its own), so the id is ignored and resume uses codex's most-recent
// session. See the milestone-2 spec for this documented limitation.
type codexAgent struct{}

func (codexAgent) Name() string { return "codex" }

func (codexAgent) NewCommand(cwd, agentSessionID string) []string {
	return []string{"codex"}
}

func (codexAgent) ResumeCommand(cwd, agentSessionID string) []string {
	return []string{"codex", "resume", "--last"}
}
