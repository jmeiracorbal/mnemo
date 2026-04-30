#!/bin/bash
# mnemo — Stop hook for Claude Code plugin

INPUT=$(cat)
SESSION_ID=$(echo "$INPUT" | mnemo json session_id 2>/dev/null)
CWD=$(echo "$INPUT" | mnemo json cwd 2>/dev/null)
[ -z "$CWD" ] && CWD="$(pwd)"

[ -z "$SESSION_ID" ] && exit 0

PROJECT_ROOT=$(git -C "$CWD" rev-parse --show-toplevel 2>/dev/null || echo "$CWD")
[ ! -f "${PROJECT_ROOT}/.mnemo" ] && exit 0

OBS_COUNT=$(mnemo session obs-count "$SESSION_ID" 2>/dev/null)

mnemo session end "$SESSION_ID" 2>/dev/null || true

if [ "${OBS_COUNT:-0}" = "0" ]; then
  printf "\n[mnemo] warning: session ended with 0 memories saved.\n" >&2
fi

exit 0
