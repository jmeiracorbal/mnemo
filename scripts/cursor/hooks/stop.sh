#!/bin/bash
# mnemo — stop hook for Cursor 2.6+
# Fires when a conversation completes. Reads transcript_path (JSONL) for
# passive capture, then closes the mnemo session.
#
# Input: {
#   "conversation_id": "...", "generation_id": "...", "status": "completed|aborted|error",
#   "loop_count": N, "workspace_roots": ["..."],
#   "transcript_path": "/path/to/{conversation_id}.jsonl"
# }

INPUT=$(cat)
CONVERSATION_ID=$(echo "$INPUT" | mnemo json conversation_id 2>/dev/null)
TRANSCRIPT_PATH=$(echo "$INPUT" | mnemo json transcript_path 2>/dev/null)
WORKSPACE=$(echo "$INPUT" | mnemo json workspace_roots 0 2>/dev/null)

[ -z "$CONVERSATION_ID" ] && exit 0
[ -z "$WORKSPACE" ] && WORKSPACE="$(pwd)"

PROJECT_ROOT=$(git -C "$WORKSPACE" rev-parse --show-toplevel 2>/dev/null || echo "$WORKSPACE")
MNEMO_FILE="${PROJECT_ROOT}/.mnemo"
[ -f "$MNEMO_FILE" ] && PROJECT=$(mnemo json id < "$MNEMO_FILE" 2>/dev/null)
[ -z "$PROJECT" ] && exit 0

# Passive capture from transcript if available
if [ -n "$TRANSCRIPT_PATH" ] && [ -f "$TRANSCRIPT_PATH" ]; then
  CONTENT=$(mnemo extract-transcript "$TRANSCRIPT_PATH" 2>/dev/null)

  if [ -n "$CONTENT" ]; then
    printf '%s' "$CONTENT" | mnemo capture - --session "$CONVERSATION_ID" --project "$PROJECT" 2>/dev/null || true
  fi
fi

OBS_COUNT=$(mnemo session obs-count "$CONVERSATION_ID" 2>/dev/null)
mnemo session end "$CONVERSATION_ID" 2>/dev/null || true

if [ "${OBS_COUNT:-0}" = "0" ]; then
  printf "\n[mnemo] warning: session ended with 0 memories saved.\n" >&2
fi

exit 0
