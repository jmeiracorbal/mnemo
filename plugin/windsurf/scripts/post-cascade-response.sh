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
TRAJECTORY_ID=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('trajectory_id',''))" 2>/dev/null)
TRANSCRIPT_PATH=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('tool_info',{}).get('transcript_path',''))" 2>/dev/null)

[ -z "$TRAJECTORY_ID" ] && exit 0

WORKSPACE="$(pwd)"
PROJECT=$(git -C "$WORKSPACE" rev-parse --show-toplevel 2>/dev/null | xargs basename 2>/dev/null)
[ -z "$PROJECT" ] && PROJECT=$(git -C "$WORKSPACE" remote get-url origin 2>/dev/null | sed 's/\.git$//' | sed 's|.*[/:]||')
[ -z "$PROJECT" ] && PROJECT=$(basename "$WORKSPACE")

# Passive capture from transcript if available
if [ -n "$TRANSCRIPT_PATH" ] && [ -f "$TRANSCRIPT_PATH" ]; then
  CONTENT=$(python3 -c "
import sys, json
lines = []
for line in open('$TRANSCRIPT_PATH'):
    try:
        msg = json.loads(line)
        role = msg.get('role','')
        content = msg.get('content','')
        if role == 'assistant' and content:
            if isinstance(content, list):
                for block in content:
                    if isinstance(block, dict) and block.get('type') == 'text':
                        lines.append(block.get('text',''))
            elif isinstance(content, str):
                lines.append(content)
    except:
        pass
print('\n'.join(lines))
" 2>/dev/null)

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
