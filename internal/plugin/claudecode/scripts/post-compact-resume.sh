#!/bin/bash
# mnemo — SessionStart (source: compact) hook for Claude Code plugin
# Fires when a new context window starts after compaction.
# The compacted summary was already persisted by the PostCompact hook.

HOOKS_DIR="$(dirname "$0")"

INPUT=$(cat)
SESSION_ID=$(echo "$INPUT" | mnemo json session_id 2>/dev/null)
CWD=$(echo "$INPUT" | mnemo json cwd 2>/dev/null)

[ -z "$SESSION_ID" ] && exit 0
[ -z "$CWD" ] && CWD="$(pwd)"

PROJECT=$(git -C "$CWD" rev-parse --show-toplevel 2>/dev/null | xargs basename 2>/dev/null)
[ -z "$PROJECT" ] && PROJECT=$(git -C "$CWD" remote get-url origin 2>/dev/null | sed 's/\.git$//' | sed 's|.*[/:]||')
[ -z "$PROJECT" ] && PROJECT=$(basename "$CWD")

printf "\n[mnemo] Context restored after compaction (project: %s)\n" "$PROJECT"

CONTEXT=$(mnemo context "$PROJECT" 2>/dev/null)

cat "${HOOKS_DIR}/post-compact-resume-protocol.md"

if [ -n "$CONTEXT" ]; then
  printf "\n%s\n" "$CONTEXT"
fi

exit 0
