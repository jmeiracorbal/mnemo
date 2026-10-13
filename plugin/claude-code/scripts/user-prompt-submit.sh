#!/bin/bash
# mnemo — UserPromptSubmit hook for Claude Code plugin
# Re-emits deferred tool loading on the first user message and periodic save nudges.
#
# Input: {
#   "session_id": "...", "cwd": "...", "prompt": "...",
#   "hook_event_name": "UserPromptSubmit"
# }

HOOKS_DIR="$(dirname "$0")"

INPUT=$(cat)
SESSION_ID=$(echo "$INPUT" | mnemo json session_id 2>/dev/null)
CWD=$(echo "$INPUT" | mnemo json cwd 2>/dev/null)

[ -z "$SESSION_ID" ] && exit 0
[ -z "$CWD" ] && CWD="$(pwd)"

PROJECT_ROOT=$(git -C "$CWD" rev-parse --show-toplevel 2>/dev/null || echo "$CWD")
MNEMO_FILE="${PROJECT_ROOT}/.mnemo"
[ -f "$MNEMO_FILE" ] && PROJECT=$(mnemo json id < "$MNEMO_FILE" 2>/dev/null)
[ -z "$PROJECT" ] && exit 0

STATE_DIR="${XDG_STATE_HOME:-$HOME/.local/state}/mnemo/user-prompt"
mkdir -p "$STATE_DIR"
STATE_FILE="${STATE_DIR}/${SESSION_ID}"
NOW=$(date +%s)

PROMPT_COUNT=0
LAST_NUDGE=0
if [ -f "$STATE_FILE" ]; then
  # shellcheck disable=SC1090
  read -r PROMPT_COUNT LAST_NUDGE < "$STATE_FILE" 2>/dev/null || true
fi
PROMPT_COUNT=$((PROMPT_COUNT + 1))

OUTPUT=""
if [ "$PROMPT_COUNT" -eq 1 ]; then
  OUTPUT=$(cat "${HOOKS_DIR}/session-start-protocol.md" 2>/dev/null)
elif [ "$((NOW - LAST_NUDGE))" -ge 900 ]; then
  LAST_NUDGE=$NOW
  OUTPUT="[mnemo] Reminder: call mem_save after significant decisions, fixes, or discoveries in this session."
fi

printf '%s %s\n' "$PROMPT_COUNT" "$LAST_NUDGE" > "$STATE_FILE"

if [ -n "$OUTPUT" ]; then
  printf '%s\n' "$OUTPUT"
fi

exit 0
