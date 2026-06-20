#!/bin/bash
# mnemo — PostToolUse hook for Edit/Write tool calls
# Saves file-change observations deterministically. No LLM involvement.
# Uses topic_key = file-change/<rel_path> so repeated edits upsert, not duplicate.

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | mnemo json tool_name 2>/dev/null)
FILE_PATH=$(echo "$INPUT" | mnemo json tool_input file_path 2>/dev/null)
CWD=$(echo "$INPUT" | mnemo json cwd 2>/dev/null)

[ -z "$FILE_PATH" ] && exit 0
[ -z "$CWD" ] && CWD="$(pwd)"

PROJECT_ROOT=$(git -C "$CWD" rev-parse --show-toplevel 2>/dev/null || echo "$CWD")
MNEMO_FILE="${PROJECT_ROOT}/.mnemo"
[ -f "$MNEMO_FILE" ] && PROJECT=$(mnemo json id < "$MNEMO_FILE" 2>/dev/null)
[ -z "$PROJECT" ] && exit 0

BASENAME=$(basename "$FILE_PATH")
REL_PATH="${FILE_PATH#${PROJECT_ROOT}/}"
TOPIC="file-change/${REL_PATH}"

mnemo save \
  "file changed: ${BASENAME}" \
  "**What**: ${TOOL_NAME} on ${REL_PATH}" \
  --type file-change \
  --project "$PROJECT" \
  --topic "$TOPIC" \
  2>/dev/null || true

exit 0
