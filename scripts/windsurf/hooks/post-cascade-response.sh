#!/bin/bash
# mnemo — post_cascade_response_with_transcript hook for Windsurf
# Fires after a conversation response. Reads transcript JSONL for passive
# capture, then closes the mnemo session.
#
# Input: {
#   "agent_action_name": "post_cascade_response_with_transcript",
#   "trajectory_id": "...", "execution_id": "...", "timestamp": "...",
#   "tool_info": { "transcript_path": "/path/to/{trajectory_id}.jsonl" }
# }

INPUT=$(cat)
TRAJECTORY_ID=$(echo "$INPUT" | mnemo json trajectory_id 2>/dev/null)
TRANSCRIPT_PATH=$(echo "$INPUT" | mnemo json tool_info transcript_path 2>/dev/null)

[ -z "$TRAJECTORY_ID" ] && exit 0

WORKSPACE="$(pwd)"
PROJECT=$(realpath "$WORKSPACE" 2>/dev/null | sed "s|^$HOME/||; s|^/||" | tr '/' '-' | tr '[:upper:]' '[:lower:]')
[ -z "$PROJECT" ] && PROJECT=$(basename "$WORKSPACE" | tr '[:upper:]' '[:lower:]')

# Passive capture from transcript if available
if [ -n "$TRANSCRIPT_PATH" ] && [ -f "$TRANSCRIPT_PATH" ]; then
  CONTENT=$(mnemo extract-transcript "$TRANSCRIPT_PATH" 2>/dev/null)

  if [ -n "$CONTENT" ]; then
    mnemo capture "$CONTENT" --session "$TRAJECTORY_ID" --project "$PROJECT" 2>/dev/null || true
  fi
fi

OBS_COUNT=$(mnemo session obs-count "$TRAJECTORY_ID" 2>/dev/null)
mnemo session end "$TRAJECTORY_ID" 2>/dev/null || true

if [ "${OBS_COUNT:-0}" = "0" ]; then
  printf "\n[mnemo] warning: session ended with 0 memories saved.\n" >&2
fi

exit 0
