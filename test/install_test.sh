#!/bin/bash
# Test de instalación local — simula lo que hace install.sh por agente,
# usando el binario compilado localmente y scripts/ del repo.
#
# Uso:
#   bash test/install_test.sh
#   bash test/install_test.sh cursor
#   bash test/install_test.sh windsurf

set -e

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MNEMO_BIN="$REPO_ROOT/build/mnemo-test"
PASS=0
FAIL=0

RED='\033[0;31m'
GREEN='\033[0;32m'
RESET='\033[0m'

ok()   { printf "${GREEN}  ✓${RESET} %s\n" "$1"; PASS=$((PASS + 1)); }
fail() { printf "${RED}  ✗${RESET} %s\n" "$1"; FAIL=$((FAIL + 1)); }
info() { printf "  → %s\n" "$1"; }
header() { printf "\n── %s ──\n" "$1"; }

# ── build ─────────────────────────────────────────────────────────────────────

header "Building mnemo"
mkdir -p "$REPO_ROOT/build"
go build -o "$MNEMO_BIN" "$REPO_ROOT/cmd/mnemo/" || { echo "Build failed"; exit 1; }
ok "Binary built: $MNEMO_BIN"

# ── helpers ───────────────────────────────────────────────────────────────────

assert_file_exists() {
  local path="$1" label="$2"
  if [ -f "$path" ]; then
    ok "$label: exists"
  else
    fail "$label: NOT found at $path"
  fi
}

assert_file_contains() {
  local path="$1" text="$2" label="$3"
  if [ -f "$path" ] && grep -qF "$text" "$path"; then
    ok "$label: contains expected text"
  else
    fail "$label: missing '$text' in $path"
  fi
}

assert_file_not_duplicated() {
  local path="$1" text="$2" label="$3"
  local count
  count=$(grep -cF "$text" "$path" 2>/dev/null || echo 0)
  if [ "$count" -le 1 ]; then
    ok "$label: no duplicates ($count occurrence)"
  else
    fail "$label: DUPLICATED ($count occurrences of '$text')"
  fi
}

assert_json_key() {
  local path="$1" key="$2" label="$3"
  local val
  val=$(cat "$path" | "$MNEMO_BIN" json $key 2>/dev/null)
  if [ -n "$val" ]; then
    ok "$label: json.$key = $val"
  else
    fail "$label: json.$key not found in $path"
  fi
}

assert_executable() {
  local path="$1" label="$2"
  if [ -x "$path" ]; then
    ok "$label: is executable"
  else
    fail "$label: NOT executable"
  fi
}

# ── source setup functions ────────────────────────────────────────────────────
# Extraemos solo las funciones de setup de install.sh, sin ejecutar main().

TMP_SCRIPTS="$REPO_ROOT/scripts"

setup_claudecode() {
  local mnemo_bin="$1"
  local claude_dir="$HOME/.claude"
  local mnemo_md="$claude_dir/mnemo.md"
  local claude_md="$claude_dir/CLAUDE.md"
  local reference="@mnemo.md"

  mkdir -p "$claude_dir"
  cp "$TMP_SCRIPTS/claudecode/mnemo.md" "$mnemo_md"

  if [ -f "$claude_md" ] && grep -qF "$reference" "$claude_md" 2>/dev/null; then
    : # already present
  else
    if [ -f "$claude_md" ] && [ -s "$claude_md" ]; then
      tail -c1 "$claude_md" | grep -q $'\n' || printf '\n' >> "$claude_md"
    fi
    printf '%s\n' "$reference" >> "$claude_md"
  fi
}

setup_cursor() {
  local mnemo_bin="$1"
  local hooks_dir="$HOME/.cursor/hooks"
  local mcp_json="$HOME/.cursor/mcp.json"
  local hooks_json="$HOME/.cursor/hooks.json"
  local rules_dir="$HOME/.cursor/rules"

  mkdir -p "$hooks_dir"
  cp "$TMP_SCRIPTS/cursor/hooks/before-submit-prompt.sh" "$hooks_dir/"
  cp "$TMP_SCRIPTS/cursor/hooks/stop.sh" "$hooks_dir/"
  chmod +x "$hooks_dir/before-submit-prompt.sh" "$hooks_dir/stop.sh"

  printf '{"mcpServers":{"mnemo":{"command":"%s","args":["mcp","--tools=agent"]}}}' \
    "$mnemo_bin" | "$mnemo_bin" json-merge "$mcp_json" >/dev/null

  printf '{"version":1,"hooks":{"beforeSubmitPrompt":[{"command":"%s/before-submit-prompt.sh"}],"stop":[{"command":"%s/stop.sh"}]}}' \
    "$hooks_dir" "$hooks_dir" | "$mnemo_bin" json-merge "$hooks_json" >/dev/null

  mkdir -p "$rules_dir"
  cp "$TMP_SCRIPTS/cursor/rules/mnemo.mdc" "$rules_dir/"
}

setup_windsurf() {
  local mnemo_bin="$1"
  local hooks_dir="$HOME/.codeium/windsurf/hooks"
  local mcp_json="$HOME/.codeium/windsurf/mcp_config.json"
  local hooks_json="$HOME/.codeium/windsurf/hooks.json"
  local memories_dir="$HOME/.codeium/windsurf/memories"
  local global_rules="$memories_dir/global_rules.md"
  local marker="## mnemo — Persistent Memory Protocol"

  mkdir -p "$hooks_dir"
  cp "$TMP_SCRIPTS/windsurf/hooks/pre-user-prompt.sh" "$hooks_dir/"
  cp "$TMP_SCRIPTS/windsurf/hooks/post-cascade-response.sh" "$hooks_dir/"
  chmod +x "$hooks_dir/pre-user-prompt.sh" "$hooks_dir/post-cascade-response.sh"

  printf '{"mcpServers":{"mnemo":{"command":"%s","args":["mcp","--tools=agent"]}}}' \
    "$mnemo_bin" | "$mnemo_bin" json-merge "$mcp_json" >/dev/null

  printf '{"hooks":{"pre_user_prompt":[{"command":"%s/pre-user-prompt.sh"}],"post_cascade_response_with_transcript":[{"command":"%s/post-cascade-response.sh"}]}}' \
    "$hooks_dir" "$hooks_dir" | "$mnemo_bin" json-merge "$hooks_json" >/dev/null

  mkdir -p "$memories_dir"
  if [ -f "$global_rules" ] && grep -qF "$marker" "$global_rules" 2>/dev/null; then
    :
  else
    if [ -f "$global_rules" ] && [ -s "$global_rules" ]; then
      tail -c1 "$global_rules" | grep -q $'\n' || printf '\n' >> "$global_rules"
      printf '\n' >> "$global_rules"
    fi
    cat "$TMP_SCRIPTS/windsurf/templates/global_rules.md" >> "$global_rules"
  fi
}

# ── test: claudecode ──────────────────────────────────────────────────────────

run_test_claudecode() {
  header "claudecode — primera instalación"
  local fake_home
  fake_home=$(mktemp -d)
  HOME="$fake_home" setup_claudecode "$MNEMO_BIN"

  assert_file_exists "$fake_home/.claude/mnemo.md" "mnemo.md"
  assert_file_contains "$fake_home/.claude/mnemo.md" "mnemo — Persistent Memory Protocol" "mnemo.md contenido"
  assert_file_exists "$fake_home/.claude/CLAUDE.md" "CLAUDE.md"
  assert_file_contains "$fake_home/.claude/CLAUDE.md" "@mnemo.md" "CLAUDE.md referencia"

  header "claudecode — idempotencia"
  HOME="$fake_home" setup_claudecode "$MNEMO_BIN"
  assert_file_not_duplicated "$fake_home/.claude/CLAUDE.md" "@mnemo.md" "CLAUDE.md sin duplicados"

  header "claudecode — CLAUDE.md preexistente sin salto de línea"
  local fake_home2
  fake_home2=$(mktemp -d)
  mkdir -p "$fake_home2/.claude"
  printf 'existing content' > "$fake_home2/.claude/CLAUDE.md"  # sin newline final
  HOME="$fake_home2" setup_claudecode "$MNEMO_BIN"
  assert_file_contains "$fake_home2/.claude/CLAUDE.md" "@mnemo.md" "CLAUDE.md appended tras contenido existente"
  assert_file_contains "$fake_home2/.claude/CLAUDE.md" "existing content" "CLAUDE.md conserva contenido existente"

  rm -rf "$fake_home" "$fake_home2"
}

# ── test: cursor ───────────────────────────────────────────────────────────────

run_test_cursor() {
  header "cursor — primera instalación"
  local fake_home
  fake_home=$(mktemp -d)
  HOME="$fake_home" setup_cursor "$MNEMO_BIN"

  assert_file_exists "$fake_home/.cursor/hooks/before-submit-prompt.sh" "before-submit-prompt.sh"
  assert_file_exists "$fake_home/.cursor/hooks/stop.sh" "stop.sh"
  assert_executable "$fake_home/.cursor/hooks/before-submit-prompt.sh" "before-submit-prompt.sh"
  assert_executable "$fake_home/.cursor/hooks/stop.sh" "stop.sh"
  assert_file_exists "$fake_home/.cursor/mcp.json" "mcp.json"
  assert_json_key "$fake_home/.cursor/mcp.json" "mcpServers mnemo command" "mcp.json mnemo.command"
  assert_file_exists "$fake_home/.cursor/hooks.json" "hooks.json"
  assert_json_key "$fake_home/.cursor/hooks.json" "version" "hooks.json version"
  assert_file_contains "$fake_home/.cursor/hooks.json" "$fake_home/.cursor/hooks/before-submit-prompt.sh" "hooks.json referencia before-submit-prompt.sh"
  assert_file_contains "$fake_home/.cursor/hooks.json" "$fake_home/.cursor/hooks/stop.sh" "hooks.json referencia stop.sh"
  assert_file_exists "$fake_home/.cursor/rules/mnemo.mdc" "mnemo.mdc"
  assert_file_contains "$fake_home/.cursor/rules/mnemo.mdc" "alwaysApply: true" "mnemo.mdc frontmatter"

  header "cursor — idempotencia"
  HOME="$fake_home" setup_cursor "$MNEMO_BIN"
  local before_count
  before_count=$(grep -c "before-submit-prompt.sh" "$fake_home/.cursor/hooks.json" 2>/dev/null || echo 0)
  if [ "$before_count" -le 1 ]; then
    ok "hooks.json sin duplicados de before-submit-prompt.sh ($before_count)"
  else
    fail "hooks.json DUPLICADO: $before_count entradas de before-submit-prompt.sh"
  fi

  header "cursor — mcp.json preexistente con otra entrada"
  local fake_home2
  fake_home2=$(mktemp -d)
  mkdir -p "$fake_home2/.cursor"
  printf '{"mcpServers":{"other":{"command":"something"}}}' > "$fake_home2/.cursor/mcp.json"
  HOME="$fake_home2" setup_cursor "$MNEMO_BIN"
  assert_json_key "$fake_home2/.cursor/mcp.json" "mcpServers other command" "mcp.json preserva entrada existente"
  assert_json_key "$fake_home2/.cursor/mcp.json" "mcpServers mnemo command" "mcp.json añade mnemo"

  rm -rf "$fake_home" "$fake_home2"
}

# ── test: windsurf ─────────────────────────────────────────────────────────────

run_test_windsurf() {
  header "windsurf — primera instalación"
  local fake_home
  fake_home=$(mktemp -d)
  HOME="$fake_home" setup_windsurf "$MNEMO_BIN"

  assert_file_exists "$fake_home/.codeium/windsurf/hooks/pre-user-prompt.sh" "pre-user-prompt.sh"
  assert_file_exists "$fake_home/.codeium/windsurf/hooks/post-cascade-response.sh" "post-cascade-response.sh"
  assert_executable "$fake_home/.codeium/windsurf/hooks/pre-user-prompt.sh" "pre-user-prompt.sh"
  assert_executable "$fake_home/.codeium/windsurf/hooks/post-cascade-response.sh" "post-cascade-response.sh"
  assert_file_exists "$fake_home/.codeium/windsurf/mcp_config.json" "mcp_config.json"
  assert_json_key "$fake_home/.codeium/windsurf/mcp_config.json" "mcpServers mnemo command" "mcp_config.json mnemo.command"
  assert_file_exists "$fake_home/.codeium/windsurf/hooks.json" "hooks.json"
  assert_file_contains "$fake_home/.codeium/windsurf/hooks.json" "$fake_home/.codeium/windsurf/hooks/pre-user-prompt.sh" "hooks.json referencia pre-user-prompt.sh"
  assert_file_contains "$fake_home/.codeium/windsurf/hooks.json" "$fake_home/.codeium/windsurf/hooks/post-cascade-response.sh" "hooks.json referencia post-cascade-response.sh"
  assert_file_exists "$fake_home/.codeium/windsurf/memories/global_rules.md" "global_rules.md"
  assert_file_contains "$fake_home/.codeium/windsurf/memories/global_rules.md" "mnemo — Persistent Memory Protocol" "global_rules.md contenido"

  header "windsurf — idempotencia"
  HOME="$fake_home" setup_windsurf "$MNEMO_BIN"
  assert_file_not_duplicated "$fake_home/.codeium/windsurf/memories/global_rules.md" "mnemo — Persistent Memory Protocol" "global_rules.md sin duplicados"

  header "windsurf — global_rules.md preexistente"
  local fake_home2
  fake_home2=$(mktemp -d)
  mkdir -p "$fake_home2/.codeium/windsurf/memories"
  printf '# My existing rules\n\nSome content.' > "$fake_home2/.codeium/windsurf/memories/global_rules.md"
  HOME="$fake_home2" setup_windsurf "$MNEMO_BIN"
  assert_file_contains "$fake_home2/.codeium/windsurf/memories/global_rules.md" "My existing rules" "global_rules.md conserva contenido existente"
  assert_file_contains "$fake_home2/.codeium/windsurf/memories/global_rules.md" "mnemo — Persistent Memory Protocol" "global_rules.md añade protocolo"

  rm -rf "$fake_home" "$fake_home2"
}

# ── test: json-merge directo ──────────────────────────────────────────────────

run_test_json_merge() {
  header "json-merge — comportamiento básico"
  local tmp
  tmp=$(mktemp)
  rm "$tmp"  # queremos que no exista

  printf '{"a":1}' | "$MNEMO_BIN" json-merge "$tmp" | grep -q "updated" && ok "nueva creación" || fail "nueva creación"
  printf '{"a":1}' | "$MNEMO_BIN" json-merge "$tmp" | grep -q "already up to date" && ok "idempotencia" || fail "idempotencia"
  printf '{"b":2}' | "$MNEMO_BIN" json-merge "$tmp" >/dev/null
  val=$(cat "$tmp" | "$MNEMO_BIN" json a)
  [ "$val" = "1" ] && ok "clave existente preservada" || fail "clave existente perdida (got: $val)"
  val=$(cat "$tmp" | "$MNEMO_BIN" json b)
  [ "$val" = "2" ] && ok "clave nueva añadida" || fail "clave nueva no añadida (got: $val)"

  rm -f "$tmp"
}

# ── validate shipped scripts ─────────────────────────────────────────────────

validate_shipped_scripts() {
  header "shipped scripts — existence check"
  assert_file_exists "$REPO_ROOT/scripts/claudecode/mnemo.md" "claudecode: mnemo.md"
  assert_file_exists "$REPO_ROOT/scripts/cursor/hooks/before-submit-prompt.sh" "cursor: before-submit-prompt.sh"
  assert_file_exists "$REPO_ROOT/scripts/cursor/hooks/stop.sh" "cursor: stop.sh"
  assert_file_exists "$REPO_ROOT/scripts/cursor/rules/mnemo.mdc" "cursor: mnemo.mdc"
  assert_file_exists "$REPO_ROOT/scripts/windsurf/hooks/pre-user-prompt.sh" "windsurf: pre-user-prompt.sh"
  assert_file_exists "$REPO_ROOT/scripts/windsurf/hooks/post-cascade-response.sh" "windsurf: post-cascade-response.sh"
  assert_file_exists "$REPO_ROOT/scripts/windsurf/templates/global_rules.md" "windsurf: global_rules.md"
}

# ── run ───────────────────────────────────────────────────────────────────────

FILTER="${1:-all}"

validate_shipped_scripts
run_test_json_merge

case "$FILTER" in
  all)
    run_test_claudecode
    run_test_cursor
    run_test_windsurf
    ;;
  claudecode) run_test_claudecode ;;
  cursor)     run_test_cursor ;;
  windsurf)   run_test_windsurf ;;
  *)
    echo "Uso: $0 [all|claudecode|cursor|windsurf]"
    exit 1
    ;;
esac

# ── summary ───────────────────────────────────────────────────────────────────

printf "\n══════════════════════════════════\n"
printf "Resultados: %d pasados, %d fallidos\n" "$PASS" "$FAIL"
[ $FAIL -eq 0 ] && exit 0 || exit 1
