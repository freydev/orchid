#!/usr/bin/env bash
# Report a status to orchid. Args: <state>. No-op when not running under orchid.
[ -n "$ORCHID_BIN" ] && [ -n "$ORCHID_SESSION" ] && "$ORCHID_BIN" signal "$1" >/dev/null 2>&1
exit 0
