#!/bin/bash
# mnemo — SubagentStop hook for Claude Code plugin
# Extracts learnings from subagent output (async, does not block).

INPUT=$(cat)
SESSION_ID=$(echo "$INPUT" | mnemo json session_id 2>/dev/null)
CWD=$(echo "$INPUT" | mnemo json cwd 2>/dev/null)
OUTPUT=$(echo "$INPUT" | mnemo json stdout 2>/dev/null)

[ -z "$OUTPUT" ] && exit 0

[ -z "$CWD" ] && CWD="$(pwd)"
PROJECT=$(git -C "$CWD" rev-parse --show-toplevel 2>/dev/null | xargs basename 2>/dev/null)
[ -z "$PROJECT" ] && PROJECT=$(git -C "$CWD" remote get-url origin 2>/dev/null | sed 's/\.git$//' | sed 's|.*[/:]||')
[ -z "$PROJECT" ] && PROJECT=$(basename "$CWD")

mnemo capture "$OUTPUT" --session "$SESSION_ID" --project "$PROJECT" 2>/dev/null || true

exit 0
