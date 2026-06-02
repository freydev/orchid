#!/usr/bin/env bash
# Show a desktop notification that the agent finished. No-op when not running
# under orchid. Optional arg: a short message.
[ -n "$ORCHID_BIN" ] && [ -n "$ORCHID_SESSION" ] && "$ORCHID_BIN" notify "$1" >/dev/null 2>&1
exit 0
