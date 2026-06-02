package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"orchid/internal/config"
)

type shellAgent struct{}

func (shellAgent) Name() string { return "shell" }

// ShellPath resolves the shell launched for "shell" sessions: the configured
// shell (~/.config/orchid/config.json "shell"), else $SHELL, else /bin/bash.
// The configured value may be a bare name ("fish") or an absolute path.
func ShellPath() string {
	if p := resolveShell(config.Shell()); p != "" {
		return p
	}
	if p := resolveShell(os.Getenv("SHELL")); p != "" {
		return p
	}
	return "/bin/bash"
}

// ResolveShellName resolves a shell choice (bare name like "fish" or an absolute
// path) to a launchable path. An empty or unresolvable choice falls back to the
// global ShellPath().
func ResolveShellName(choice string) string {
	if p := resolveShell(choice); p != "" {
		return p
	}
	return ShellPath()
}

// resolveShell turns a single choice into a path, or "" when it can't be
// resolved. A "/"-bearing value is treated as a path and must exist; a bare name
// is looked up on PATH.
func resolveShell(choice string) string {
	choice = strings.TrimSpace(choice)
	if choice == "" {
		return ""
	}
	if strings.ContainsRune(choice, '/') {
		if _, err := os.Stat(choice); err == nil {
			return choice
		}
		return ""
	}
	if p, err := exec.LookPath(choice); err == nil {
		return p
	}
	return ""
}

// AvailableShells returns the bare names of shells installed on the system,
// gathered from /etc/shells and a set of common shells found on PATH. Sorted,
// de-duplicated. Used to populate the settings UI's shell picker.
func AvailableShells() []string {
	seen := map[string]bool{}
	var out []string
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		out = append(out, name)
	}
	if data, err := os.ReadFile("/etc/shells"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			add(filepath.Base(line))
		}
	}
	for _, c := range []string{"bash", "zsh", "fish", "sh", "nu", "pwsh"} {
		if _, err := exec.LookPath(c); err == nil {
			add(c)
		}
	}
	sort.Strings(out)
	return out
}

func (shellAgent) NewCommand(cwd, agentSessionID string) []string {
	return []string{ShellPath()}
}

// ResumeCommand returns nil: a shell has no session to resume.
func (shellAgent) ResumeCommand(cwd, agentSessionID string) []string {
	return nil
}
