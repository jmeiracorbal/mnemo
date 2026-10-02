#!/bin/bash
# mnemo — PreToolUse hook for Claude Code plugin
# Capa-1 deterministic guardrail: blocks mnemo write tools before any MCP call
# is attempted when the current directory has no .mnemo marker or its project
# id does not match the tool call's project argument.

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | mnemo json tool_name 2>/dev/null)

case "$TOOL_NAME" in
  mcp__mnemo__mem_save|\
  mcp__mnemo__mem_save_prompt|\
  mcp__mnemo__mem_session_summary|\
  mcp__mnemo__mem_capture_passive)
    ;;
  *)
    exit 0
    ;;
esac

CWD=$(echo "$INPUT" | mnemo json cwd 2>/dev/null)
[ -z "$CWD" ] && CWD="$(pwd)"
PROJECT_ARG=$(echo "$INPUT" | mnemo json tool_input project 2>/dev/null)

PROJECT_ROOT=$(git -C "$CWD" rev-parse --show-toplevel 2>/dev/null || echo "$CWD")
MARKER_ID=$(mnemo json id < "${PROJECT_ROOT}/.mnemo" 2>/dev/null)

if [ -z "$MARKER_ID" ]; then
  printf '{"action":"block","message":"mnemo: project not initialized at %s — run mnemo init to activate persistent memory."}\n' "$PROJECT_ROOT"
  exit 0
fi

if [ -n "$PROJECT_ARG" ] && [ "$MARKER_ID" != "$PROJECT_ARG" ]; then
  printf '{"action":"block","message":"mnemo: project id mismatch — .mnemo has %s but request carries %s."}\n' "$MARKER_ID" "$PROJECT_ARG"
  exit 0
fi

exit 0
