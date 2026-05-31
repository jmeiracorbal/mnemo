#!/bin/bash
# mnemo — SubagentStop hook for Claude Code plugin
# Extracts learnings from subagent output (async, does not block).

INPUT=$(cat)
SESSION_ID=$(echo "$INPUT" | mnemo json session_id 2>/dev/null)
CWD=$(echo "$INPUT" | mnemo json cwd 2>/dev/null)
OUTPUT=$(echo "$INPUT" | mnemo json stdout 2>/dev/null)

[ -z "$OUTPUT" ] && exit 0

[ -z "$CWD" ] && CWD="$(pwd)"

PROJECT_ROOT=$(git -C "$CWD" rev-parse --show-toplevel 2>/dev/null || echo "$CWD")
MNEMO_FILE="${PROJECT_ROOT}/.mnemo"
[ -f "$MNEMO_FILE" ] && PROJECT=$(mnemo json id < "$MNEMO_FILE" 2>/dev/null)
[ -z "$PROJECT" ] && exit 0

mnemo capture "$OUTPUT" --session "$SESSION_ID" --project "$PROJECT" 2>/dev/null || true

exit 0
