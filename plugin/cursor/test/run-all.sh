#!/bin/bash
# mnemo — Cursor hooks test harness (Cursor 2.6.x format)
# Usage: ./run-all.sh [test-name]
#   test-name: before-submit-prompt | stop | stop-with-transcript
#              (omit to run all)

SCRIPTS_DIR="$(cd "$(dirname "$0")/../scripts" && pwd)"
PASS=0
FAIL=0

run_test() {
  local name="$1"
  local script="$2"
  local payload="$3"

  printf "\n── %s ──────────────────────────────\n" "$name"
  printf "Payload: %s\n" "$payload"
  printf "Output:\n"

  echo "$payload" | bash "$script"
  local exit_code=$?

  if [ $exit_code -eq 0 ] || [ $exit_code -eq 2 ]; then
    printf "\n✓ exit %d\n" "$exit_code"
    PASS=$((PASS + 1))
  else
    printf "\n✗ exit %d (unexpected)\n" "$exit_code"
    FAIL=$((FAIL + 1))
  fi
}

FILTER="${1:-all}"
CWD="$(git -C "$(dirname "$0")" rev-parse --show-toplevel 2>/dev/null || pwd)"
CONV_ID="cursor-test-$(date +%s)"

# ── before-submit-prompt ──────────────────────────────────────────────────────

if [ "$FILTER" = "all" ] || [ "$FILTER" = "before-submit-prompt" ]; then
  run_test "before-submit-prompt — new conversation" \
    "$SCRIPTS_DIR/before-submit-prompt.sh" \
    "{\"conversation_id\":\"$CONV_ID\",\"generation_id\":\"gen-abc\",\"model\":\"default\",\"prompt\":\"hello\",\"attachments\":[],\"hook_event_name\":\"beforeSubmitPrompt\",\"cursor_version\":\"2.6.21\",\"workspace_roots\":[\"$CWD\"],\"user_email\":\"test@example.com\",\"transcript_path\":null}"

  run_test "before-submit-prompt — same conversation (2nd prompt, should skip)" \
    "$SCRIPTS_DIR/before-submit-prompt.sh" \
    "{\"conversation_id\":\"$CONV_ID\",\"generation_id\":\"gen-def\",\"model\":\"default\",\"prompt\":\"follow-up\",\"attachments\":[],\"hook_event_name\":\"beforeSubmitPrompt\",\"cursor_version\":\"2.6.21\",\"workspace_roots\":[\"$CWD\"],\"user_email\":\"test@example.com\",\"transcript_path\":null}"

  run_test "before-submit-prompt — missing conversation_id" \
    "$SCRIPTS_DIR/before-submit-prompt.sh" \
    "{\"model\":\"default\",\"prompt\":\"hello\",\"hook_event_name\":\"beforeSubmitPrompt\",\"workspace_roots\":[\"$CWD\"]}"
fi

# ── stop ──────────────────────────────────────────────────────────────────────

if [ "$FILTER" = "all" ] || [ "$FILTER" = "stop" ]; then
  run_test "stop — completed, no transcript" \
    "$SCRIPTS_DIR/stop.sh" \
    "{\"conversation_id\":\"$CONV_ID\",\"generation_id\":\"gen-abc\",\"model\":\"default\",\"status\":\"completed\",\"loop_count\":3,\"hook_event_name\":\"stop\",\"cursor_version\":\"2.6.21\",\"workspace_roots\":[\"$CWD\"],\"user_email\":\"test@example.com\",\"transcript_path\":null}"

  run_test "stop — missing id" \
    "$SCRIPTS_DIR/stop.sh" \
    "{\"status\":\"completed\",\"hook_event_name\":\"stop\",\"workspace_roots\":[\"$CWD\"]}"
fi

# ── stop with transcript ──────────────────────────────────────────────────────

if [ "$FILTER" = "all" ] || [ "$FILTER" = "stop-with-transcript" ]; then
  CONV_ID2="cursor-transcript-test-$(date +%s)"
  TMPFILE=$(mktemp /tmp/mnemo-cursor-transcript-XXXXXX.jsonl)

  # Simulate a JSONL transcript with assistant messages
  echo '{"role":"user","content":"fix the auth bug"}' >> "$TMPFILE"
  echo '{"role":"assistant","content":"## Key Learnings\n- Fixed null pointer in JWT validation\n- Added expiry check before decode"}' >> "$TMPFILE"
  echo '{"role":"assistant","content":[{"type":"text","text":"Done. Fixed auth middleware by adding nil check."}]}' >> "$TMPFILE"

  # First create the session
  echo "{\"conversation_id\":\"$CONV_ID2\",\"generation_id\":\"gen-t\",\"model\":\"default\",\"prompt\":\"fix\",\"attachments\":[],\"hook_event_name\":\"beforeSubmitPrompt\",\"cursor_version\":\"2.6.21\",\"workspace_roots\":[\"$CWD\"],\"user_email\":\"test@example.com\",\"transcript_path\":null}" \
    | bash "$SCRIPTS_DIR/before-submit-prompt.sh" > /dev/null 2>&1

  run_test "stop — with JSONL transcript (passive capture)" \
    "$SCRIPTS_DIR/stop.sh" \
    "{\"conversation_id\":\"$CONV_ID2\",\"generation_id\":\"gen-t\",\"model\":\"default\",\"status\":\"completed\",\"loop_count\":2,\"hook_event_name\":\"stop\",\"cursor_version\":\"2.6.21\",\"workspace_roots\":[\"$CWD\"],\"user_email\":\"test@example.com\",\"transcript_path\":\"$TMPFILE\"}"

  rm -f "$TMPFILE"
fi

# ── summary ───────────────────────────────────────────────────────────────────

printf "\n══════════════════════════════════════\n"
printf "Results: %d passed, %d failed\n" "$PASS" "$FAIL"
[ $FAIL -eq 0 ] && exit 0 || exit 1
