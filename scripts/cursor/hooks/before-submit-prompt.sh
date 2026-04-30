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
CONVERSATION_ID=$(echo "$INPUT" | mnemo json conversation_id 2>/dev/null)
WORKSPACE=$(echo "$INPUT" | mnemo json workspace_roots 0 2>/dev/null)
PROMPT=$(echo "$INPUT" | mnemo json prompt 2>/dev/null)

[ -z "$CONVERSATION_ID" ] && exit 0
[ -z "$WORKSPACE" ] && WORKSPACE="$(pwd)"

PROJECT_ROOT=$(git -C "$WORKSPACE" rev-parse --show-toplevel 2>/dev/null || echo "$WORKSPACE")
[ ! -f "${PROJECT_ROOT}/.mnemo" ] && exit 0

PROJECT=$(realpath "$PROJECT_ROOT" 2>/dev/null | sed "s|^$HOME/||; s|^/||" | tr '/' '-' | tr '[:upper:]' '[:lower:]')
[ -z "$PROJECT" ] && PROJECT=$(basename "$PROJECT_ROOT" | tr '[:upper:]' '[:lower:]')

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
