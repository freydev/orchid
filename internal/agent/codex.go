package agent

import "os"

// codexAgent: codex cannot bind a caller-supplied session id at start (it
// generates its own), so the id is ignored and resume uses codex's most-recent
// session. See the milestone-2 spec for this documented limitation.
type codexAgent struct{}

func (codexAgent) Name() string { return "codex" }

// codexStatusInstructions tells codex (which has no lifecycle hooks like claude)
// to self-report its status via the orchid set_status MCP tool. Injected as
// developer instructions per session, so no repo AGENTS.md is touched.
const codexStatusInstructions = "Status reporting (orchid): this session runs inside the orchid dashboard, which exposes set_status and notify_done MCP tools. Report your state so the dashboard stays accurate: call set_status(state=\"running\") right before you start working (running tools, editing files, executing commands); call set_status(state=\"requires_action\") whenever you end a turn needing the user's answer, decision, or approval; call set_status(state=\"idle\") as soon as you finish and need nothing further from the user. Always emit set_status(idle) at the end of a turn that needs no input. When you finish a task and want the user's attention (e.g. they may have stepped away), also call notify_done with a short message summarizing what you completed — this shows a desktop notification on the user's machine."

// codexFlags registers the orchid MCP server (set_status tool) and injects the
// status-reporting developer instructions for this codex session via `-c`
// overrides. codex inherits the tmux pane env, so the spawned `orchid mcp`
// server reads ORCHID_SESSION/ORCHID_URL itself — no static env is baked into
// config. Empty if the orchid binary path can't be resolved.
func codexFlags() []string {
	self, err := os.Executable()
	if err != nil {
		return nil
	}
	return []string{
		"-c", `mcp_servers.orchid.command="` + self + `"`,
		"-c", `mcp_servers.orchid.args=["mcp"]`,
		"-c", `developer_instructions="` + tomlEscape(codexStatusInstructions) + `"`,
	}
}

// tomlEscape escapes a string for a TOML basic ("...") value.
func tomlEscape(s string) string {
	var b []byte
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\\':
			b = append(b, '\\', '\\')
		case '"':
			b = append(b, '\\', '"')
		case '\n':
			b = append(b, '\\', 'n')
		case '\t':
			b = append(b, '\\', 't')
		default:
			b = append(b, c)
		}
	}
	return string(b)
}

func (codexAgent) NewCommand(cwd, agentSessionID string) []string {
	return append([]string{"codex"}, codexFlags()...)
}

func (codexAgent) ResumeCommand(cwd, agentSessionID string) []string {
	// `-c` is a global option and must precede the subcommand.
	return append(append([]string{"codex"}, codexFlags()...), "resume", "--last")
}
