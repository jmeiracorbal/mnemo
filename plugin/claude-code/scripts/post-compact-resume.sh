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

PROJECT_ROOT=$(git -C "$CWD" rev-parse --show-toplevel 2>/dev/null || echo "$CWD")
[ ! -f "${PROJECT_ROOT}/.mnemo" ] && exit 0

PROJECT=$(realpath "$PROJECT_ROOT" 2>/dev/null | sed "s|^$HOME/||; s|^/||" | tr '/' '-' | tr '[:upper:]' '[:lower:]')
[ -z "$PROJECT" ] && PROJECT=$(basename "$PROJECT_ROOT" | tr '[:upper:]' '[:lower:]')

printf "\n[mnemo] Context restored after compaction (project: %s)\n" "$PROJECT"

CONTEXT=$(mnemo context "$PROJECT" 2>/dev/null)

cat "${HOOKS_DIR}/post-compact-resume-protocol.md"

if [ -n "$CONTEXT" ]; then
  printf "\n%s\n" "$CONTEXT"
fi

exit 0
