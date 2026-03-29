#!/bin/bash
# mnemo — beforeShellExecution hook for Cursor 2.6+
# Fires before every shell command the agent executes.
# Saves commands that signal meaningful decisions or milestones to mnemo memory.
# Always exits allowing the command to run.
#
# Input: {
#   "command": "...", "cwd": "...", "sandbox": false,
#   "conversation_id": "...", "workspace_roots": ["..."],
#   "hook_event_name": "beforeShellExecution"
# }

INPUT=$(cat)
COMMAND=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('command',''))" 2>/dev/null)
CWD=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('cwd',''))" 2>/dev/null)
CONVERSATION_ID=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('conversation_id',''))" 2>/dev/null)
WORKSPACE=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); roots=d.get('workspace_roots',[]); print(roots[0] if roots else '')" 2>/dev/null)

[ -z "$COMMAND" ] && exit 0
[ -z "$CWD" ] && CWD="${WORKSPACE:-$(pwd)}"

PROJECT=$(git -C "$CWD" rev-parse --show-toplevel 2>/dev/null | xargs basename 2>/dev/null)
[ -z "$PROJECT" ] && PROJECT=$(git -C "$CWD" remote get-url origin 2>/dev/null | sed 's/\.git$//' | sed 's|.*[/:]||')
[ -z "$PROJECT" ] && PROJECT=$(basename "$CWD")

# Patterns that signal a meaningful decision or milestone worth remembering
MEMORABLE=false
MEMORY_TYPE="decision"

case "$COMMAND" in
  git\ tag*|git\ push\ origin\ v*)
    MEMORABLE=true
    MEMORY_TYPE="decision"
    ;;
  git\ push\ --force*|git\ reset\ --hard*)
    MEMORABLE=true
    MEMORY_TYPE="decision"
    ;;
  npm\ publish*|npx\ publish*)
    MEMORABLE=true
    MEMORY_TYPE="decision"
    ;;
  go\ build*|go\ install*)
    MEMORABLE=true
    MEMORY_TYPE="pattern"
    ;;
  make\ *|cargo\ build*|cargo\ release*)
    MEMORABLE=true
    MEMORY_TYPE="pattern"
    ;;
  *migrate*|*migration*|*db:migrate*|*alembic\ upgrade*)
    MEMORABLE=true
    MEMORY_TYPE="decision"
    ;;
  docker\ build*|docker\ push*|docker-compose\ up*)
    MEMORABLE=true
    MEMORY_TYPE="pattern"
    ;;
esac

if [ "$MEMORABLE" = "true" ]; then
  TITLE="Shell: $COMMAND"
  CONTENT="**What**: Ran \`$COMMAND\`
**Where**: $CWD (project: $PROJECT)"
  mnemo save "$TITLE" "$CONTENT" --type "$MEMORY_TYPE" --project "$PROJECT" --session "$CONVERSATION_ID" 2>/dev/null || true
fi

exit 0
