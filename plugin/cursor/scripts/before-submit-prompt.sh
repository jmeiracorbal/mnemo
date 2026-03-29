#!/bin/bash
# mnemo — beforeSubmitPrompt hook for Cursor 2.6+
# Fires before every prompt. We use it as session start by tracking
# conversation_id — first time we see it we create the session, emit general
# context, and search for memories relevant to the opening prompt.
#
# Input: {
#   "conversation_id": "...", "generation_id": "...", "prompt": "...",
#   "workspace_roots": ["..."], "transcript_path": null|"...",
#   "hook_event_name": "beforeSubmitPrompt"
# }

INPUT=$(cat)
CONVERSATION_ID=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('conversation_id',''))" 2>/dev/null)
WORKSPACE=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); roots=d.get('workspace_roots',[]); print(roots[0] if roots else '')" 2>/dev/null)
PROMPT=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('prompt',''))" 2>/dev/null)

[ -z "$CONVERSATION_ID" ] && exit 0
[ -z "$WORKSPACE" ] && WORKSPACE="$(pwd)"

PROJECT=$(git -C "$WORKSPACE" rev-parse --show-toplevel 2>/dev/null | xargs basename 2>/dev/null)
[ -z "$PROJECT" ] && PROJECT=$(git -C "$WORKSPACE" remote get-url origin 2>/dev/null | sed 's/\.git$//' | sed 's|.*[/:]||')
[ -z "$PROJECT" ] && PROJECT=$(basename "$WORKSPACE")

# Only act on the first prompt of a conversation (session start)
IS_KNOWN=$(mnemo session exists "$CONVERSATION_ID" 2>/dev/null)
[ "$IS_KNOWN" = "true" ] && exit 0

# New conversation — register session and emit general context
mnemo session start "$CONVERSATION_ID" --project "$PROJECT" --dir "$WORKSPACE" 2>/dev/null || true
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
