#!/bin/bash
# mnemo installer
# Usage: curl -sSf https://raw.githubusercontent.com/jmeiracorbal/mnemo/main/install.sh | bash

set -e

REPO="jmeiracorbal/mnemo"
INSTALL_DIR="${MNEMO_INSTALL_DIR:-$HOME/.local/bin}"
DRY_RUN="${MNEMO_DRY_RUN:-false}"
MNEMO_VERSION="${MNEMO_VERSION:-}"

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

# ── fetch latest version ───────────────────────────────────────────────────────

fetch_latest_version() {
  local version

  if command -v curl >/dev/null 2>&1; then
    version=$(curl -sSf "https://api.github.com/repos/${REPO}/releases/latest" \
      | python3 -c "import sys,json; print(json.load(sys.stdin)['tag_name'])" 2>/dev/null)
  elif command -v wget >/dev/null 2>&1; then
    version=$(wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" \
      | python3 -c "import sys,json; print(json.load(sys.stdin)['tag_name'])" 2>/dev/null)
  else
    err "curl or wget required"
  fi

  [ -z "$version" ] && err "Could not fetch latest release version"
  echo "$version"
}

# ── download binary ────────────────────────────────────────────────────────────

download_binary() {
  local version="$1" platform="$2"
  local url="https://github.com/${REPO}/releases/download/${version}/mnemo-${platform}"
  local dest="${INSTALL_DIR}/mnemo"

  info "Downloading mnemo ${version} for ${platform}..."

  if [ "$DRY_RUN" = "true" ]; then
    dry "curl -sSfL \"${url}\" -o \"${dest}\""
    dry "chmod +x \"${dest}\""
    return
  fi

  mkdir -p "$INSTALL_DIR"

  if command -v curl >/dev/null 2>&1; then
    curl -sSfL "$url" -o "$dest" || err "Download failed: ${url}"
  else
    wget -qO "$dest" "$url" || err "Download failed: ${url}"
  fi

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

# ── run setup ─────────────────────────────────────────────────────────────────

run_setup() {
  if [ "$DRY_RUN" = "true" ]; then
    dry "mnemo setup"
    return
  fi

  if ! command -v mnemo >/dev/null 2>&1 && ! [ -x "${INSTALL_DIR}/mnemo" ]; then
    warn "mnemo not found in PATH — skipping setup. Run 'mnemo setup' manually after adding ${INSTALL_DIR} to PATH."
    return
  fi

  local mnemo_bin
  mnemo_bin=$(command -v mnemo 2>/dev/null || echo "${INSTALL_DIR}/mnemo")

  info "Running mnemo setup..."
  "$mnemo_bin" setup
}

# ── main ───────────────────────────────────────────────────────────────────────

main() {
  [ "$DRY_RUN" = "true" ] && info "Dry-run mode — no changes will be made"

  local platform version
  platform=$(detect_platform)
  version="${MNEMO_VERSION:-$(fetch_latest_version)}"

  info "Latest release: ${version}"

  download_binary "$version" "$platform"
  check_path
  run_setup

  ok "Done. Restart Claude Code to activate mnemo."
}

main "$@"
