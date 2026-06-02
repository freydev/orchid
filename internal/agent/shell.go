package agent

import (
	"os"

	"orchid/internal/config"
)

type shellAgent struct{}

func (shellAgent) Name() string { return "shell" }

// ShellPath resolves the shell launched for "shell" sessions: the configured
// shell (~/.config/orchid/config.json "shell"), else $SHELL, else /bin/bash.
func ShellPath() string {
	shell := config.Shell()
	if shell == "" {
		shell = os.Getenv("SHELL")
	}
	if shell == "" {
		shell = "/bin/bash"
	}
	return shell
}

func (shellAgent) NewCommand(cwd, agentSessionID string) []string {
	return []string{ShellPath()}
}

// ResumeCommand returns nil: a shell has no session to resume.
func (shellAgent) ResumeCommand(cwd, agentSessionID string) []string {
	return nil
}
