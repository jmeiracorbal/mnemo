#!/bin/bash
# mnemo — Stop hook for Claude Code plugin

INPUT=$(cat)
SESSION_ID=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('session_id',''))" 2>/dev/null)

[ -z "$SESSION_ID" ] && exit 0

OBS_COUNT=$(mnemo session obs-count "$SESSION_ID" 2>/dev/null)

mnemo session end "$SESSION_ID" 2>/dev/null || true

if [ "${OBS_COUNT:-0}" = "0" ]; then
  printf "\n[mnemo] warning: session ended with 0 memories saved.\n" >&2
fi

exit 0
