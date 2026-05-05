#!/bin/bash
# mnemo — Stop hook for Codex CLI
# Fires when the agent stops. Reads transcript for passive capture,
# then closes the mnemo session.
#
# Input: {
#   "session_id": "...", "transcript_path": "...", "cwd": "...",
#   "hook_event_name": "Stop", "reason": "...", "turn_id": "..."
# }
#
# Output: { "continue": true }

INPUT=$(cat)
SESSION_ID=$(echo "$INPUT" | mnemo json session_id 2>/dev/null)
TRANSCRIPT_PATH=$(echo "$INPUT" | mnemo json transcript_path 2>/dev/null)
CWD=$(echo "$INPUT" | mnemo json cwd 2>/dev/null)

if [ -z "$SESSION_ID" ]; then
  printf '{"continue":true}\n'
  exit 0
fi

[ -z "$CWD" ] && CWD="$(pwd)"

PROJECT_ROOT=$(git -C "$CWD" rev-parse --show-toplevel 2>/dev/null || echo "$CWD")

PROJECT=$(realpath "$PROJECT_ROOT" 2>/dev/null | sed "s|^$HOME/||; s|^/||" | tr '/' '-' | tr '[:upper:]' '[:lower:]')
[ -z "$PROJECT" ] && PROJECT=$(basename "$PROJECT_ROOT" | tr '[:upper:]' '[:lower:]')

# Passive capture only if project has mnemo configured
if [ -f "${PROJECT_ROOT}/.mnemo" ]; then
  if [ -n "$TRANSCRIPT_PATH" ] && [ -f "$TRANSCRIPT_PATH" ]; then
    CONTENT=$(mnemo extract-transcript "$TRANSCRIPT_PATH" 2>/dev/null)
    if [ -z "$CONTENT" ]; then
      CONTENT=$(cat "$TRANSCRIPT_PATH" 2>/dev/null)
    fi

    if [ -n "$CONTENT" ]; then
      mnemo capture "$CONTENT" --session "$SESSION_ID" --project "$PROJECT" 2>/dev/null || true
    fi
  fi
fi

OBS_COUNT=$(mnemo session obs-count "$SESSION_ID" 2>/dev/null)
mnemo session end "$SESSION_ID" 2>/dev/null || true

if [ "${OBS_COUNT:-0}" = "0" ]; then
  printf '{"continue":true,"systemMessage":"[mnemo] warning: session ended with 0 memories saved."}\n'
  exit 0
fi

printf '{"continue":true}\n'
exit 0
