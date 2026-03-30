#!/bin/bash
# mnemo — PostCompact hook for Claude Code plugin
# Injects memory protocol and context after compaction so the agent
# persists the compacted summary and recovers session state.

HOOKS_DIR="$(dirname "$0")"

INPUT=$(cat)
SESSION_ID=$(echo "$INPUT" | mnemo json session_id 2>/dev/null)
CWD=$(echo "$INPUT" | mnemo json cwd 2>/dev/null)

[ -z "$CWD" ] && CWD="$(pwd)"

PROJECT=$(git -C "$CWD" rev-parse --show-toplevel 2>/dev/null | xargs basename 2>/dev/null)
[ -z "$PROJECT" ] && PROJECT=$(git -C "$CWD" remote get-url origin 2>/dev/null | sed 's/\.git$//' | sed 's|.*[/:]||')
[ -z "$PROJECT" ] && PROJECT=$(basename "$CWD")

if [ -n "$SESSION_ID" ] && [ -n "$PROJECT" ]; then
  mnemo session start "$SESSION_ID" --project "$PROJECT" --dir "$CWD" 2>/dev/null || true
fi

CONTEXT=$(mnemo context "$PROJECT" 2>/dev/null)

cat "${HOOKS_DIR}/post-compact-protocol-header.md"

printf "\n1. FIRST: Call mem_session_summary with the content of the compacted summary above. Use project: '%s'.\n" "$PROJECT"
printf "   This preserves what was accomplished before compaction.\n\n"
printf "2. THEN: Call mem_context with project: '%s' to recover recent session history and observations.\n" "$PROJECT"
printf "   Read the returned context carefully — it tells you what was being worked on.\n\n"

cat "${HOOKS_DIR}/post-compact-protocol-footer.md"

if [ -n "$CONTEXT" ]; then
  printf "\n%s\n" "$CONTEXT"
fi

exit 0
