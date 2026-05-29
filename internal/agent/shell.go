package agent

import "os"

type shellAgent struct{}

func (shellAgent) Name() string { return "shell" }

func (shellAgent) NewCommand(cwd, agentSessionID string) []string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	return []string{shell}
}

// ResumeCommand returns nil: a shell has no session to resume.
func (shellAgent) ResumeCommand(cwd, agentSessionID string) []string {
	return nil
}
