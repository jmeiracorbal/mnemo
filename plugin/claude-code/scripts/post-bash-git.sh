#!/bin/bash
# mnemo — PostToolUse hook for Bash tool calls
# Captures git commit events as decision observations. No LLM involvement.

INPUT=$(cat)
CMD=$(echo "$INPUT" | mnemo json tool_input command 2>/dev/null)
CWD=$(echo "$INPUT" | mnemo json cwd 2>/dev/null)

[ -z "$CMD" ] && exit 0

echo "$CMD" | grep -q "git commit" || exit 0

[ -z "$CWD" ] && CWD="$(pwd)"

PROJECT_ROOT=$(git -C "$CWD" rev-parse --show-toplevel 2>/dev/null || echo "$CWD")
MNEMO_FILE="${PROJECT_ROOT}/.mnemo"
[ -f "$MNEMO_FILE" ] && PROJECT=$(mnemo json id < "$MNEMO_FILE" 2>/dev/null)
[ -z "$PROJECT" ] && exit 0

MSG=$(git -C "$PROJECT_ROOT" log -1 --pretty=%B 2>/dev/null | head -5)
[ -z "$MSG" ] && exit 0

SUBJECT=$(echo "$MSG" | head -1)

mnemo save \
  "git commit: ${SUBJECT}" \
  "**What**: Committed changes
**Why**: ${MSG}
**Where**: ${PROJECT_ROOT}" \
  --type decision \
  --project "$PROJECT" \
  2>/dev/null || true

exit 0
