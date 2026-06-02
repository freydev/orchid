---
name: report-status
description: Use throughout every turn in an orchid session — call set_status to report running before work, requires_action when you need the user, and idle when finished, so the dashboard stays accurate.
---

# Reporting status to orchid

This session runs inside the orchid dashboard, which exposes a `set_status` MCP tool. Lifecycle hooks already report most transitions automatically, but call `set_status` yourself to keep the dashboard accurate and cover gaps:

- `set_status(state="running")` — right before you start working (running tools, editing files, executing commands).
- `set_status(state="requires_action", message="<short note on what you need>")` — whenever you end a turn needing the user's answer, decision, or approval. The dashboard shows an amber "Needs you" until they reply.
- `set_status(state="idle")` — as soon as you finish and need nothing further from the user. Always emit this at the end of a turn that needs no input.

Report promptly. `requires_action` is sticky until you next report `running` (i.e. the user replied and you resumed work).
