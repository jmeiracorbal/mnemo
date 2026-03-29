#!/bin/bash
# mnemo — SessionStart (source: compact) hook for Claude Code plugin
# Fires when a new context window starts after compaction.
# The compacted summary was already persisted by the PostCompact hook.

INPUT=$(cat)
SESSION_ID=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('session_id',''))" 2>/dev/null)
CWD=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('cwd',''))" 2>/dev/null)

[ -z "$SESSION_ID" ] && exit 0
[ -z "$CWD" ] && CWD="$(pwd)"

PROJECT=$(git -C "$CWD" rev-parse --show-toplevel 2>/dev/null | xargs basename 2>/dev/null)
[ -z "$PROJECT" ] && PROJECT=$(git -C "$CWD" remote get-url origin 2>/dev/null | sed 's/\.git$//' | sed 's|.*[/:]||')
[ -z "$PROJECT" ] && PROJECT=$(basename "$CWD")

printf "\n[mnemo] Context restored after compaction (project: %s)\n" "$PROJECT"

CONTEXT=$(mnemo context "$PROJECT" 2>/dev/null)

cat <<'PROTOCOL'

### FIRST ACTION — load memory tools
Memory tools are deferred and must be loaded before use. Call ToolSearch NOW with:
select:mcp__mnemo__mem_save,mcp__mnemo__mem_context,mcp__mnemo__mem_search,mcp__mnemo__mem_session_summary,mcp__mnemo__mem_session_end

### POST-COMPACTION RECOVERY — context window was just reset
The compacted summary above contains what happened before compaction.
The summary was already persisted to mnemo by the PostCompact hook.

Recovery steps (do BEFORE responding to user):
1. Call mem_context with the current project to recover recent session history
2. If you need detail on a specific topic, call mem_search with relevant keywords
3. Only THEN continue working on what the user asked
PROTOCOL

if [ -n "$CONTEXT" ]; then
  printf "\n%s\n" "$CONTEXT"
fi

exit 0
