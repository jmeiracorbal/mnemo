#!/bin/bash
# mnemo — Stop hook for Claude Code plugin

INPUT=$(cat)
SESSION_ID=$(echo "$INPUT" | mnemo json session_id 2>/dev/null)
CWD=$(echo "$INPUT" | mnemo json cwd 2>/dev/null)
[ -z "$CWD" ] && CWD="$(pwd)"

[ -z "$SESSION_ID" ] && exit 0

PROJECT_ROOT=$(git -C "$CWD" rev-parse --show-toplevel 2>/dev/null || echo "$CWD")
MNEMO_FILE="${PROJECT_ROOT}/.mnemo"
[ -f "$MNEMO_FILE" ] && PROJECT=$(mnemo json id < "$MNEMO_FILE" 2>/dev/null)
[ -z "$PROJECT" ] && exit 0

mnemo session end "$SESSION_ID" >/dev/null 2>&1 || true

exit 0
