#!/bin/bash
# mnemo — pre_user_prompt hook for Windsurf
# Fires before every prompt. First occurrence of a trajectory_id creates the
# session, emits general context, and searches for memories relevant to the
# opening prompt.
#
# Input: {
#   "agent_action_name": "pre_user_prompt",
#   "trajectory_id": "...", "execution_id": "...", "timestamp": "...",
#   "tool_info": { "user_prompt": "..." }
# }

INPUT=$(cat)
TRAJECTORY_ID=$(echo "$INPUT" | mnemo json trajectory_id 2>/dev/null)
PROMPT=$(echo "$INPUT" | mnemo json tool_info user_prompt 2>/dev/null)

[ -z "$TRAJECTORY_ID" ] && exit 0

WORKSPACE="$(pwd)"
PROJECT=$(git -C "$WORKSPACE" rev-parse --show-toplevel 2>/dev/null | xargs basename 2>/dev/null)
[ -z "$PROJECT" ] && PROJECT=$(git -C "$WORKSPACE" remote get-url origin 2>/dev/null | sed 's/\.git$//' | sed 's|.*[/:]||')
[ -z "$PROJECT" ] && PROJECT=$(basename "$WORKSPACE")

# Only act on the first prompt of a conversation (session start)
IS_KNOWN=$(mnemo session exists "$TRAJECTORY_ID" 2>/dev/null)
[ "$IS_KNOWN" = "true" ] && exit 0

# New conversation — register session and emit general context
mnemo session start "$TRAJECTORY_ID" --project "$PROJECT" --dir "$WORKSPACE" 2>/dev/null || true
printf "\n[mnemo] New session started (project: %s)\n" "$PROJECT"

CONTEXT=$(mnemo context "$PROJECT" 2>/dev/null)
if [ -n "$CONTEXT" ]; then
  printf "\n%s\n" "$CONTEXT"
fi

# Prompt-specific search: only if prompt has meaningful content (>20 chars)
PROMPT_LEN=${#PROMPT}
if [ "$PROMPT_LEN" -gt 20 ]; then
  SEARCH_QUERY=$(echo "$PROMPT" | cut -c1-100)
  SEARCH_RESULTS=$(mnemo search "$SEARCH_QUERY" --project "$PROJECT" --limit 3 2>/dev/null)
  if [ -n "$SEARCH_RESULTS" ] && ! echo "$SEARCH_RESULTS" | grep -q "^No memories found"; then
    printf "\n[mnemo] Relevant memories for this prompt:\n%s\n" "$SEARCH_RESULTS"
  fi
fi

exit 0
