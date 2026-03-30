#!/bin/bash
# mnemo — Windsurf hooks test harness
# Usage: ./run-all.sh [test-name]
#   test-name: pre-user-prompt | post-cascade-response | post-cascade-with-transcript
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
TRAJ_ID="windsurf-test-$(date +%s)"

# ── pre_user_prompt ───────────────────────────────────────────────────────────

if [ "$FILTER" = "all" ] || [ "$FILTER" = "pre-user-prompt" ]; then
  run_test "pre_user_prompt — new conversation" \
    "$SCRIPTS_DIR/pre-user-prompt.sh" \
    "{\"agent_action_name\":\"pre_user_prompt\",\"trajectory_id\":\"$TRAJ_ID\",\"execution_id\":\"exec-abc\",\"timestamp\":\"2026-03-30T10:00:00Z\",\"tool_info\":{\"user_prompt\":\"hello\"}}"

  run_test "pre_user_prompt — same conversation (2nd prompt, should skip)" \
    "$SCRIPTS_DIR/pre-user-prompt.sh" \
    "{\"agent_action_name\":\"pre_user_prompt\",\"trajectory_id\":\"$TRAJ_ID\",\"execution_id\":\"exec-def\",\"timestamp\":\"2026-03-30T10:00:01Z\",\"tool_info\":{\"user_prompt\":\"follow-up\"}}"

  run_test "pre_user_prompt — missing trajectory_id" \
    "$SCRIPTS_DIR/pre-user-prompt.sh" \
    "{\"agent_action_name\":\"pre_user_prompt\",\"execution_id\":\"exec-xyz\",\"timestamp\":\"2026-03-30T10:00:02Z\",\"tool_info\":{\"user_prompt\":\"hello\"}}"

  run_test "pre_user_prompt — long prompt triggers search" \
    "$SCRIPTS_DIR/pre-user-prompt.sh" \
    "{\"agent_action_name\":\"pre_user_prompt\",\"trajectory_id\":\"windsurf-search-$(date +%s)\",\"execution_id\":\"exec-ghi\",\"timestamp\":\"2026-03-30T10:00:03Z\",\"tool_info\":{\"user_prompt\":\"how do we handle memory persistence across sessions in mnemo\"}}"
fi

# ── post_cascade_response_with_transcript ─────────────────────────────────────

if [ "$FILTER" = "all" ] || [ "$FILTER" = "post-cascade-response" ]; then
  run_test "post_cascade_response — no transcript" \
    "$SCRIPTS_DIR/post-cascade-response.sh" \
    "{\"agent_action_name\":\"post_cascade_response_with_transcript\",\"trajectory_id\":\"$TRAJ_ID\",\"execution_id\":\"exec-abc\",\"timestamp\":\"2026-03-30T10:01:00Z\",\"tool_info\":{\"transcript_path\":\"\"}}"

  run_test "post_cascade_response — missing trajectory_id" \
    "$SCRIPTS_DIR/post-cascade-response.sh" \
    "{\"agent_action_name\":\"post_cascade_response_with_transcript\",\"execution_id\":\"exec-abc\",\"timestamp\":\"2026-03-30T10:01:01Z\",\"tool_info\":{\"transcript_path\":\"\"}}"
fi

# ── post_cascade_response with real transcript ────────────────────────────────

if [ "$FILTER" = "all" ] || [ "$FILTER" = "post-cascade-with-transcript" ]; then
  TRAJ_ID2="windsurf-transcript-test-$(date +%s)"
  TMPFILE=$(mktemp /tmp/mnemo-windsurf-transcript-XXXXXX.jsonl)

  echo '{"role":"user","content":"fix the auth bug"}' >> "$TMPFILE"
  echo '{"role":"assistant","content":"## Key Learnings\n- Fixed null pointer in JWT validation\n- Added expiry check before decode"}' >> "$TMPFILE"
  echo '{"role":"assistant","content":[{"type":"text","text":"Done. Fixed auth middleware by adding nil check."}]}' >> "$TMPFILE"

  # First create the session
  echo "{\"agent_action_name\":\"pre_user_prompt\",\"trajectory_id\":\"$TRAJ_ID2\",\"execution_id\":\"exec-t\",\"timestamp\":\"2026-03-30T10:00:00Z\",\"tool_info\":{\"user_prompt\":\"fix\"}}" \
    | bash "$SCRIPTS_DIR/pre-user-prompt.sh" > /dev/null 2>&1

  run_test "post_cascade_response — with JSONL transcript (passive capture)" \
    "$SCRIPTS_DIR/post-cascade-response.sh" \
    "{\"agent_action_name\":\"post_cascade_response_with_transcript\",\"trajectory_id\":\"$TRAJ_ID2\",\"execution_id\":\"exec-t\",\"timestamp\":\"2026-03-30T10:01:00Z\",\"tool_info\":{\"transcript_path\":\"$TMPFILE\"}}"

  rm -f "$TMPFILE"
fi

# ── summary ───────────────────────────────────────────────────────────────────

printf "\n══════════════════════════════════════\n"
printf "Results: %d passed, %d failed\n" "$PASS" "$FAIL"
[ $FAIL -eq 0 ] && exit 0 || exit 1
