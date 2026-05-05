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
[ ! -f "${PROJECT_ROOT}/.mnemo" ] && exit 0

PROJECT=$(realpath "$PROJECT_ROOT" 2>/dev/null | sed "s|^$HOME/||; s|^/||" | tr '/' '-' | tr '[:upper:]' '[:lower:]')
[ -z "$PROJECT" ] && PROJECT=$(basename "$PROJECT_ROOT" | tr '[:upper:]' '[:lower:]')

mnemo capture "$OUTPUT" --session "$SESSION_ID" --project "$PROJECT" 2>/dev/null || true

exit 0
