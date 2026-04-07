#!/bin/bash
# mnemo installer
# Usage: curl -sSf https://raw.githubusercontent.com/jmeiracorbal/mnemo/main/install.sh | bash
#
# Agent selection (default: claudecode):
#   bash -s -- --agent=cursor
#   bash -s -- --agent=windsurf
#   bash -s -- --agent=all
#
# Environment overrides:
#   MNEMO_AGENT=cursor bash install.sh
#   MNEMO_VERSION=v0.9.0 bash install.sh
#   MNEMO_DRY_RUN=true bash install.sh

set -e

REPO="jmeiracorbal/mnemo"
INSTALL_DIR="${MNEMO_INSTALL_DIR:-$HOME/.local/bin}"
DRY_RUN="${MNEMO_DRY_RUN:-false}"
MNEMO_VERSION="${MNEMO_VERSION:-}"
AGENT="${MNEMO_AGENT:-claudecode}"
TMP_SCRIPTS=""

# ── helpers ────────────────────────────────────────────────────────────────────

info()  { printf "\033[1;34m[mnemo]\033[0m %s\n" "$*"; }
ok()    { printf "\033[1;32m[mnemo]\033[0m %s\n" "$*"; }
err()   { printf "\033[1;31m[mnemo]\033[0m %s\n" "$*" >&2; exit 1; }
warn()  { printf "\033[1;33m[mnemo]\033[0m %s\n" "$*"; }

dry() {
  if [ "$DRY_RUN" = "true" ]; then
    printf "\033[2m  (dry-run) %s\033[0m\n" "$*"
  else
    eval "$@"
  fi
}

# ── detect platform ────────────────────────────────────────────────────────────

detect_platform() {
  local os arch

  case "$(uname -s)" in
    Darwin) os="darwin" ;;
    Linux)  os="linux" ;;
    *)      err "Unsupported OS: $(uname -s)" ;;
  esac

  case "$(uname -m)" in
    arm64|aarch64) arch="arm64" ;;
    x86_64)        arch="amd64" ;;
    *)             err "Unsupported architecture: $(uname -m)" ;;
  esac

  echo "${os}-${arch}"
}

# ── fetch ──────────────────────────────────────────────────────────────────────

fetch() {
  local url="$1" dest="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -sSfL "$url" -o "$dest" 2>/dev/null
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$dest" "$url" 2>/dev/null
  else
    err "curl or wget required"
  fi
}

fetch_stdout() {
  local url="$1"
  if command -v curl >/dev/null 2>&1; then
    curl -sSfL "$url" 2>/dev/null
  else
    wget -qO- "$url" 2>/dev/null
  fi
}

# ── fetch latest version ───────────────────────────────────────────────────────

fetch_latest_version() {
  local version
  version=$(fetch_stdout "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' \
    | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
  [ -z "$version" ] && err "Could not fetch latest release version"
  echo "$version"
}

# ── download and verify binary ─────────────────────────────────────────────────

download_binary() {
  local version="$1" platform="$2"
  local base_url="https://github.com/${REPO}/releases/download/${version}"
  local binary_url="${base_url}/mnemo-${platform}"
  local checksum_url="${base_url}/mnemo-${platform}.sha256"
  local dest="${INSTALL_DIR}/mnemo"

  info "Downloading mnemo ${version} for ${platform}..."

  if [ "$DRY_RUN" = "true" ]; then
    dry "curl -sSfL \"${binary_url}\" -o \"${dest}\""
    dry "curl -sSfL \"${checksum_url}\" | shasum -a 256 -c"
    dry "chmod +x \"${dest}\""
    return
  fi

  mkdir -p "$INSTALL_DIR"

  local tmp
  tmp=$(mktemp)
  trap 'rm -f "$tmp"' EXIT

  fetch "$binary_url" "$tmp" || err "Download failed: ${binary_url}"

  local checksum_file
  checksum_file=$(mktemp)
  trap 'rm -f "$tmp" "$checksum_file"' EXIT

  fetch_stdout "$checksum_url" > "$checksum_file" || err "Checksum download failed: ${checksum_url}"

  # shasum -c expects "hash  filename" — rewrite the path to match $tmp
  local expected_hash
  expected_hash=$(awk '{print $1}' "$checksum_file")
  local actual_hash
  actual_hash=$(shasum -a 256 "$tmp" | awk '{print $1}')

  if [ "$expected_hash" != "$actual_hash" ]; then
    err "Checksum mismatch — aborting. Expected: ${expected_hash}, got: ${actual_hash}"
  fi

  mv "$tmp" "$dest"
  chmod +x "$dest"
  ok "Binary installed: ${dest}"
}

# ── check PATH ─────────────────────────────────────────────────────────────────

check_path() {
  if ! echo "$PATH" | tr ':' '\n' | grep -qx "$INSTALL_DIR"; then
    warn "${INSTALL_DIR} is not in your PATH."
    warn "Add this to your shell profile (~/.zshrc or ~/.bashrc):"
    warn "  export PATH=\"\$HOME/.local/bin:\$PATH\""
  fi
}

# ── download scripts archive ───────────────────────────────────────────────────

download_scripts() {
  local version="$1"
  local archive_url="https://github.com/${REPO}/releases/download/${version}/mnemo-scripts.tar.gz"

  TMP_SCRIPTS=$(mktemp -d)
  trap 'rm -rf "$TMP_SCRIPTS"' EXIT

  info "Downloading scripts archive..."
  local tmp_archive
  tmp_archive=$(mktemp)

  fetch "$archive_url" "$tmp_archive" || err "Scripts archive download failed: ${archive_url}"
  tar -xzf "$tmp_archive" -C "$TMP_SCRIPTS" --strip-components=1
  rm -f "$tmp_archive"
  ok "Scripts ready"
}

# ── setup: Claude Code ─────────────────────────────────────────────────────────

setup_claudecode() {
  local mnemo_bin="$1"
  local claude_dir="$HOME/.claude"
  local mnemo_md="$claude_dir/mnemo.md"
  local claude_md="$claude_dir/CLAUDE.md"
  local reference="@mnemo.md"

  info "Configuring Claude Code..."
  mkdir -p "$claude_dir"

  cp "$TMP_SCRIPTS/claudecode/mnemo.md" "$mnemo_md"
  ok "~/.claude/mnemo.md written"

  if [ -f "$claude_md" ] && grep -qF "$reference" "$claude_md" 2>/dev/null; then
    ok "~/.claude/CLAUDE.md already up to date"
  else
    if [ -f "$claude_md" ] && [ -s "$claude_md" ]; then
      tail -c1 "$claude_md" | grep -q $'\n' || printf '\n' >> "$claude_md"
    fi
    printf '%s\n' "$reference" >> "$claude_md"
    ok "~/.claude/CLAUDE.md updated"
  fi
}

# ── setup: Cursor ──────────────────────────────────────────────────────────────

setup_cursor() {
  local mnemo_bin="$1"
  local hooks_dir="$HOME/.cursor/hooks"
  local mcp_json="$HOME/.cursor/mcp.json"
  local hooks_json="$HOME/.cursor/hooks.json"
  local rules_dir="$HOME/.cursor/rules"

  info "Configuring Cursor..."

  mkdir -p "$hooks_dir"
  cp "$TMP_SCRIPTS/cursor/hooks/before-submit-prompt.sh" "$hooks_dir/"
  cp "$TMP_SCRIPTS/cursor/hooks/stop.sh" "$hooks_dir/"
  chmod +x "$hooks_dir/before-submit-prompt.sh" "$hooks_dir/stop.sh"
  ok "Hook scripts installed to ${hooks_dir}"

  local result
  result=$(printf '{"mcpServers":{"mnemo":{"command":"%s","args":["mcp","--tools=agent"]}}}' \
    "$mnemo_bin" | "$mnemo_bin" json-merge "$mcp_json")
  ok "~/.cursor/mcp.json: ${result}"

  result=$(printf '{"version":1,"hooks":{"beforeSubmitPrompt":[{"command":"%s/before-submit-prompt.sh"}],"stop":[{"command":"%s/stop.sh"}]}}' \
    "$hooks_dir" "$hooks_dir" | "$mnemo_bin" json-merge "$hooks_json")
  ok "~/.cursor/hooks.json: ${result}"

  mkdir -p "$rules_dir"
  cp "$TMP_SCRIPTS/cursor/rules/mnemo.mdc" "$rules_dir/"
  ok "~/.cursor/rules/mnemo.mdc written"
}

# ── setup: Windsurf ────────────────────────────────────────────────────────────

setup_windsurf() {
  local mnemo_bin="$1"
  local hooks_dir="$HOME/.codeium/windsurf/hooks"
  local mcp_json="$HOME/.codeium/windsurf/mcp_config.json"
  local hooks_json="$HOME/.codeium/windsurf/hooks.json"
  local memories_dir="$HOME/.codeium/windsurf/memories"
  local global_rules="$memories_dir/global_rules.md"
  local marker="## mnemo — Persistent Memory Protocol"

  info "Configuring Windsurf..."

  mkdir -p "$hooks_dir"
  cp "$TMP_SCRIPTS/windsurf/hooks/pre-user-prompt.sh" "$hooks_dir/"
  cp "$TMP_SCRIPTS/windsurf/hooks/post-cascade-response.sh" "$hooks_dir/"
  chmod +x "$hooks_dir/pre-user-prompt.sh" "$hooks_dir/post-cascade-response.sh"
  ok "Hook scripts installed to ${hooks_dir}"

  local result
  result=$(printf '{"mcpServers":{"mnemo":{"command":"%s","args":["mcp","--tools=agent"]}}}' \
    "$mnemo_bin" | "$mnemo_bin" json-merge "$mcp_json")
  ok "~/.codeium/windsurf/mcp_config.json: ${result}"

  result=$(printf '{"hooks":{"pre_user_prompt":[{"command":"%s/pre-user-prompt.sh"}],"post_cascade_response_with_transcript":[{"command":"%s/post-cascade-response.sh"}]}}' \
    "$hooks_dir" "$hooks_dir" | "$mnemo_bin" json-merge "$hooks_json")
  ok "~/.codeium/windsurf/hooks.json: ${result}"

  mkdir -p "$memories_dir"
  if [ -f "$global_rules" ] && grep -qF "$marker" "$global_rules" 2>/dev/null; then
    ok "~/.codeium/windsurf/memories/global_rules.md already up to date"
  else
    if [ -f "$global_rules" ] && [ -s "$global_rules" ]; then
      tail -c1 "$global_rules" | grep -q $'\n' || printf '\n' >> "$global_rules"
      printf '\n' >> "$global_rules"
    fi
    cat "$TMP_SCRIPTS/windsurf/templates/global_rules.md" >> "$global_rules"
    ok "~/.codeium/windsurf/memories/global_rules.md updated"
  fi
}

# ── main ───────────────────────────────────────────────────────────────────────

main() {
  # Parse --agent=X from arguments
  for arg in "$@"; do
    case "$arg" in
      --agent=*) AGENT="${arg#--agent=}" ;;
    esac
  done

  [ "$DRY_RUN" = "true" ] && info "Dry-run mode — no changes will be made"

  local platform version
  platform=$(detect_platform)
  version="${MNEMO_VERSION:-$(fetch_latest_version)}"

  info "Latest release: ${version}"

  download_binary "$version" "$platform"
  check_path

  if [ "$DRY_RUN" = "true" ]; then
    info "Dry-run: would configure agent=${AGENT}"
    ok "Done (dry-run)."
    return
  fi

  local mnemo_bin
  mnemo_bin=$(command -v mnemo 2>/dev/null || echo "${INSTALL_DIR}/mnemo")

  download_scripts "$version"

  case "$AGENT" in
    claudecode)
      setup_claudecode "$mnemo_bin"
      ok "Done. Restart Claude Code to activate mnemo."
      ;;
    cursor)
      setup_cursor "$mnemo_bin"
      ok "Done. Restart Cursor to activate mnemo."
      ;;
    windsurf)
      setup_windsurf "$mnemo_bin"
      ok "Done. Restart Windsurf to activate mnemo."
      ;;
    all)
      setup_claudecode "$mnemo_bin"
      setup_cursor "$mnemo_bin"
      setup_windsurf "$mnemo_bin"
      ok "Done. Restart your editors to activate mnemo."
      ;;
    *)
      err "Unknown agent: ${AGENT}. Valid options: claudecode | cursor | windsurf | all"
      ;;
  esac
}

main "$@"
