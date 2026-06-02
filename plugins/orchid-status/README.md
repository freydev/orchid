# orchid-status plugin

Reports claude session status to the orchid dashboard.

- **Hooks** (`hooks/hooks.json`): `UserPromptSubmit`/`PreToolUse`/`SessionStart` -> `running`, `Stop` -> `done`, `Notification` -> `requires_action`. They run `bin/signal.sh`, which calls `$ORCHID_BIN signal <state>` and no-ops when not under orchid.
- **MCP** (`.mcp.json`): registers the `orchid` server (`$ORCHID_BIN mcp`) exposing `set_status` so the agent can explicitly raise `requires_action` when it asks you a question.

orchid injects `ORCHID_SESSION`, `ORCHID_URL`, `ORCHID_BIN` into each session's tmux environment; the hooks and MCP server inherit them.

### Use

orchid loads this plugin for claude sessions by launching claude with `--plugin-dir <path-to>/plugins/orchid-status`. For manual testing:

    claude --plugin-dir ./plugins/orchid-status
