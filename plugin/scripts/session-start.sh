#!/bin/bash
# mnemo — SessionStart hook for Claude Code plugin
# Claude Code passes hook input via stdin as JSON:
#   { "session_id": "...", "cwd": "..." }

INPUT=$(cat)
SESSION_ID=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('session_id',''))" 2>/dev/null)
CWD=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('cwd',''))" 2>/dev/null)

[ -z "$SESSION_ID" ] && exit 0
[ -z "$CWD" ] && CWD="$(pwd)"

# Detect project: prefer git root directory name, fallback to remote repo name, then cwd basename
PROJECT=$(git -C "$CWD" rev-parse --show-toplevel 2>/dev/null | xargs basename 2>/dev/null)
[ -z "$PROJECT" ] && PROJECT=$(git -C "$CWD" remote get-url origin 2>/dev/null | sed 's/\.git$//' | sed 's|.*[/:]||')
[ -z "$PROJECT" ] && PROJECT=$(basename "$CWD")

IS_RESUME=$(mnemo session exists "$SESSION_ID" 2>/dev/null)

if [ "$IS_RESUME" = "true" ]; then
  printf "\n[mnemo] Session resumed (project: %s)\n" "$PROJECT"
else
  mnemo session start "$SESSION_ID" --project "$PROJECT" --dir "$CWD" 2>/dev/null || true
  printf "\n[mnemo] New session started (project: %s)\n" "$PROJECT"
fi

CONTEXT=$(mnemo context "$PROJECT" 2>/dev/null)
if [ -n "$CONTEXT" ]; then
  printf "\n%s\n" "$CONTEXT"
fi

cat <<'PROTOCOL'

### FIRST ACTION — load memory tools
Memory tools are deferred and must be loaded before use. Call ToolSearch NOW with:
select:mcp__mnemo__mem_save,mcp__mnemo__mem_context,mcp__mnemo__mem_search,mcp__mnemo__mem_session_summary,mcp__mnemo__mem_session_end
PROTOCOL

exit 0
