#!/bin/bash
# mnemo installer
# Usage: curl -sSf https://raw.githubusercontent.com/jmeiracorbal/mnemo/main/install.sh | bash
#
# Agent selection (default: auto-detect installed compatible agents):
#   bash -s -- --agent=auto
#   bash -s -- --agent=cursor
#   bash -s -- --agent=windsurf
#   bash -s -- --agent=codex
#   bash -s -- --agent=opencode
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
AGENT="${MNEMO_AGENT:-auto}"

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

probe_url() {
  local url="$1"
  if command -v curl >/dev/null 2>&1; then
    curl -sSfI "$url" >/dev/null 2>&1
  else
    wget -q --spider "$url" 2>/dev/null
  fi
}

# ── version compatibility check ───────────────────────────────────────────────

check_version_compat() {
  local version="$1" platform="$2"
  local base_url="https://github.com/${REPO}/releases/download/${version}"

  info "Checking compatibility of pinned version ${version}..."

  if ! probe_url "${base_url}/mnemo-${platform}.sha256"; then
    err "Release ${version} does not ship a binary for ${platform}. Unset MNEMO_VERSION to use the latest release."
  fi

  ok "Release ${version} is compatible."
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

# ── setup delegation ───────────────────────────────────────────────────────────

setup_agent() {
  local agent="$1" mnemo_bin="$2"

  info "Configuring ${agent}..."
  "$mnemo_bin" setup refresh --agent="$agent" --mnemo-bin="$mnemo_bin"
  ok "Global ${agent} setup refreshed. Run 'mnemo init --agent=${agent}' in projects that should use mnemo."
}


# ── agent detection ───────────────────────────────────────────────────────────

agent_detected() {
  case "$1" in
    claudecode)
      command -v claude >/dev/null 2>&1 || [ -d "$HOME/.claude" ] || [ -f "$HOME/.claude.json" ]
      ;;
    cursor)
      command -v cursor >/dev/null 2>&1 || [ -d "$HOME/.cursor" ]
      ;;
    windsurf)
      command -v windsurf >/dev/null 2>&1 || [ -d "$HOME/.codeium/windsurf" ]
      ;;
    codex)
      command -v codex >/dev/null 2>&1 || [ -d "$HOME/.codex" ]
      ;;
    opencode)
      command -v opencode >/dev/null 2>&1 || [ -d "$HOME/.config/opencode" ]
      ;;
    *) return 1 ;;
  esac
}

detect_agents() {
  local found=""
  local agent
  for agent in claudecode cursor windsurf codex opencode; do
    if agent_detected "$agent"; then
      found="$found $agent"
    fi
  done
  # CodeGraph falls back to Claude when auto-detection finds nothing.
  if [ -z "$found" ]; then
    found=" claudecode"
  fi
  echo "$found"
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

  if [ -n "${MNEMO_VERSION}" ]; then
    if [ "$DRY_RUN" = "true" ]; then
      info "Dry-run: would check compatibility of pinned version ${version}"
    else
      check_version_compat "$version" "$platform"
    fi
  fi

  download_binary "$version" "$platform"
  check_path

  if [ "$DRY_RUN" = "true" ]; then
    info "Dry-run: would configure agent=${AGENT}"
    ok "Done (dry-run)."
    return
  fi

  local mnemo_bin="${INSTALL_DIR}/mnemo"
  if ! [ -x "$mnemo_bin" ]; then
    mnemo_bin=$(command -v mnemo 2>/dev/null) || err "mnemo not found in ${INSTALL_DIR} or PATH"
    warn "Using mnemo from PATH: ${mnemo_bin} (expected: ${INSTALL_DIR}/mnemo)"
  fi

  case "$AGENT" in
    auto)
      local detected
      detected=$(detect_agents)
      info "Auto-detected agents:${detected}"
      for selected in $detected; do
        setup_agent "$selected" "$mnemo_bin"
      done
      ok "Done. Run 'mnemo init --agent=all' in projects that should use mnemo."
      ;;
    all)
      for selected in claudecode cursor windsurf codex opencode; do
        setup_agent "$selected" "$mnemo_bin"
      done
      ok "Done. Run 'mnemo init --agent=all' in projects that should use mnemo."
      ;;
    claudecode|cursor|windsurf|codex|opencode)
      setup_agent "$AGENT" "$mnemo_bin"
      ok "Done. Run 'mnemo init --agent=${AGENT}' in projects that should use mnemo."
      ;;
    *)
      err "Unknown agent: ${AGENT}. Valid options: auto | claudecode | cursor | windsurf | codex | opencode | all"
      ;;
  esac
}

main "$@"
