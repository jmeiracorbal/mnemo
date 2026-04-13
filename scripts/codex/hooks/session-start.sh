#!/bin/bash
# mnemo — SessionStart hook for Codex CLI
# Fires on session startup and resume.
#
# Input: {
#   "session_id": "...", "cwd": "...", "hook_event_name": "SessionStart"
# }
#
# Output: { "continue": true, "systemMessage": "..." }

HOOKS_DIR="$(dirname "$0")"

INPUT=$(cat)
SESSION_ID=$(echo "$INPUT" | mnemo json session_id 2>/dev/null)
CWD=$(echo "$INPUT" | mnemo json cwd 2>/dev/null)

if [ -z "$SESSION_ID" ]; then
  printf '{"continue":true}\n'
  exit 0
fi

[ -z "$CWD" ] && CWD="$(pwd)"

PROJECT=$(realpath "$CWD" 2>/dev/null | sed "s|^$HOME/||; s|^/||" | tr '/' '-' | tr '[:upper:]' '[:lower:]')
[ -z "$PROJECT" ] && PROJECT=$(basename "$CWD" | tr '[:upper:]' '[:lower:]')

IS_RESUME=$(mnemo session exists "$SESSION_ID" 2>/dev/null)

if [ "$IS_RESUME" = "true" ]; then
  STATUS="[mnemo] Session resumed (project: ${PROJECT})"
else
  mnemo session start "$SESSION_ID" --project "$PROJECT" --dir "$CWD" 2>/dev/null || true
  STATUS="[mnemo] New session started (project: ${PROJECT})"
fi

CONTEXT=$(mnemo context "$PROJECT" 2>/dev/null)
PROTOCOL=$(cat "${HOOKS_DIR}/mnemo-protocol.md" 2>/dev/null)

MSG=$(printf '%s\n\n%s\n\n%s' "$STATUS" "$CONTEXT" "$PROTOCOL")

MSG_JSON=$(printf '%s' "$MSG" | awk 'BEGIN{ORS=""} NR>1{printf "\\n"} {gsub(/\\/, "\\\\"); gsub(/"/, "\\\""); gsub(/\t/, "\\t"); gsub(/\r/, "\\r"); print}')

printf '{"continue":true,"systemMessage":"%s"}\n' "$MSG_JSON"

exit 0
