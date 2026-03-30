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

# ─── Ensure ~/.cursor/rules/mnemo.mdc exists ─────────────────────────────────
CURSOR_RULES_DIR="${HOME}/.cursor/rules"
MNEMO_MDC="${CURSOR_RULES_DIR}/mnemo.mdc"

if [ ! -f "$MNEMO_MDC" ]; then
  mkdir -p "$CURSOR_RULES_DIR"
  cat > "$MNEMO_MDC" << 'MDCEOF'
---
description: mnemo persistent memory protocol
alwaysApply: true
---

## mnemo — Persistent Memory Protocol

You have access to mnemo memory tools (mem_save, mem_search, mem_context, mem_session_summary).

### PROACTIVE SAVE — do NOT wait for user to ask
Call `mem_save` IMMEDIATELY after ANY of these:
- Decision made (architecture, convention, workflow, tool choice)
- Bug fixed (include root cause)
- Convention or workflow documented/updated
- Non-obvious discovery, gotcha, or edge case found
- Pattern established (naming, structure, approach)
- User preference or constraint learned
- Feature implemented with non-obvious approach

**Self-check after EVERY task**: "Did I just make a decision, fix a bug, learn something, or establish a convention? If yes → mem_save NOW."

### SEARCH MEMORY when:
- User asks to recall anything
- Starting work on something that might have been done before
- User mentions a topic you have no context on

### SESSION CLOSE — MANDATORY, no exceptions
`mem_session_summary` is NOT optional. It is the final step of every session.
Call it before ANY response that signals completion ("done", "listo", "ready", "finished", "completed").
Fields: Goal, Discoveries, Accomplished, Next Steps, Relevant Files.

If nothing was accomplished: call it anyway with Goal and Next Steps.
No session ends without `mem_session_summary`.
MDCEOF
fi
# ─────────────────────────────────────────────────────────────────────────────

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
