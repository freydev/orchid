package agent

import (
	"os"
	"sort"
)

// Options carries per-session launch toggles chosen by the user.
type Options struct {
	// SkipPermissions launches the agent with its "dangerously skip
	// permissions" flag (claude: --dangerously-skip-permissions; codex:
	// --dangerously-bypass-approvals-and-sandbox). Ignored by shell.
	SkipPermissions bool
}

// Agent describes how to launch a coding agent inside a tmux session.
type Agent interface {
	Name() string
	// NewCommand returns argv to start a fresh agent session bound to the
	// caller-supplied agent session id (so it can be resumed later). Agents
	// without a session concept ignore the id. cwd is informational; the
	// working directory is actually set by tmux's -c flag.
	NewCommand(cwd, agentSessionID string, opts Options) []string
	// ResumeCommand returns argv to resume a prior session, or nil if the agent
	// has no resume capability (caller then starts fresh).
	ResumeCommand(cwd, agentSessionID string, opts Options) []string
}

var registry = map[string]Agent{
	"claude": claudeAgent{},
	"codex":  codexAgent{},
	"shell":  shellAgent{},
}

// Get returns the registered agent by name.
func Get(name string) (Agent, bool) {
	a, ok := registry[name]
	return a, ok
}

// Names returns registered agent names, sorted.
func Names() []string {
	out := make([]string, 0, len(registry))
	for n := range registry {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// ResolveCwd returns the working directory for a session: the given cwd, or the
// server default ($HOME) when blank. Role-based defaults can replace this later.
func ResolveCwd(cwd string) string {
	if cwd != "" {
		return cwd
	}
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return "/"
}
