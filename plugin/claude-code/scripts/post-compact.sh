#!/bin/bash
# mnemo — PostCompact hook for Claude Code plugin
# Injects memory protocol and context after compaction so the agent
# persists the compacted summary and recovers session state.

HOOKS_DIR="$(dirname "$0")"

INPUT=$(cat)
SESSION_ID=$(echo "$INPUT" | mnemo json session_id 2>/dev/null)
CWD=$(echo "$INPUT" | mnemo json cwd 2>/dev/null)

[ -z "$CWD" ] && CWD="$(pwd)"

PROJECT_ROOT=$(git -C "$CWD" rev-parse --show-toplevel 2>/dev/null || echo "$CWD")
[ ! -f "${PROJECT_ROOT}/.mnemo" ] && exit 0

PROJECT=$(realpath "$PROJECT_ROOT" 2>/dev/null | sed "s|^$HOME/||; s|^/||" | tr '/' '-' | tr '[:upper:]' '[:lower:]')
[ -z "$PROJECT" ] && PROJECT=$(basename "$PROJECT_ROOT" | tr '[:upper:]' '[:lower:]')

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
