#!/bin/bash
# mnemo installer
# Usage: curl -sSf https://raw.githubusercontent.com/jmeiracorbal/mnemo/main/install.sh | bash
#
# Agent selection (default: auto-detect installed compatible agents):
#   bash -s -- --agent=auto
#   bash -s -- --agent=cursor
#   bash -s -- --agent=codex
#   bash -s -- --agent=opencode
#   bash -s -- --agent=pi
#   bash -s -- --agent=all
#
# Environment overrides:
#   MNEMO_AGENT=cursor bash install.sh
#   MNEMO_VERSION=v1.0.1 bash install.sh
#   MNEMO_DRY_RUN=true bash install.sh

set -e

REPO="jmeiracorbal/mnemo"
INSTALL_DIR="${MNEMO_INSTALL_DIR:-$HOME/.local/bin}"
DRY_RUN="${MNEMO_DRY_RUN:-false}"
MNEMO_VERSION="${MNEMO_VERSION:-}"
AGENT="${MNEMO_AGENT:-auto}"
# Installer-internal Mnemo commands must not recursively offer another update.
export MNEMO_NO_UPDATE_CHECK=1

# State is durable in ~/.mnemo/upgrade.transaction, not in shell variables.
INSTALL_DOWNLOAD=""
INSTALL_CHECKSUM=""
INSTALL_BACKUP=""
INSTALL_MARKER=""
INSTALL_DEST=""
INSTALL_SWITCHED=false
INSTALL_VERIFIED=false
INSTALL_TXN=false
INSTALL_RECOVERY_FAILED=false
INSTALL_RESTORE_AGENT="claudecode"

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

# An update cannot report failure after a database change without recovering
# the previous executable and SQLite first. No second binary-only rollback path
# is permitted for an existing installation.
finish_install() {
  local status=$?
  trap - EXIT

  if [ "$INSTALL_VERIFIED" != "true" ] && [ "$INSTALL_TXN" = "true" ]; then
    warn "The release was not committed; restoring the previous installation."
    local recovery_bin="$INSTALL_DOWNLOAD"
    if [ "$INSTALL_SWITCHED" = "true" ]; then
      recovery_bin="$INSTALL_DEST"
    fi
    if ! "$recovery_bin" controller stop >/dev/null 2>&1; then
      INSTALL_RECOVERY_FAILED=true
      printf '[mnemo] CRITICAL: could not stop the controller; SQLite recovery is fenced.\n' >&2
    elif ! "$recovery_bin" controller transaction recover --force; then
      INSTALL_RECOVERY_FAILED=true
      printf '[mnemo] CRITICAL: release recovery failed; the journal, backup and write fence are preserved.\n' >&2
    else
      INSTALL_TXN=false
      INSTALL_BACKUP=""
      if ! "$INSTALL_DEST" setup refresh --agent="$INSTALL_RESTORE_AGENT" --mnemo-bin="$INSTALL_DEST"; then
        printf '[mnemo] CRITICAL: previous controller could not restart after recovery.\n' >&2
      else
        local attempt restored=false
        for attempt in 1 2 3 4 5 6 7 8 9 10; do
          if "$INSTALL_DEST" stats >/dev/null 2>&1; then
            restored=true
            break
          fi
          sleep 1
        done
        if [ "$restored" = "true" ]; then
          info "Previous Mnemo executable and SQLite state recovered."
        else
          printf '[mnemo] CRITICAL: recovered controller health is unconfirmed.\n' >&2
        fi
      fi
    fi
    status=1
  elif [ "$INSTALL_VERIFIED" != "true" ] && [ "$INSTALL_SWITCHED" = "true" ]; then
    # There is no previous executable on a first-time installation.
    rm -f -- "$INSTALL_DEST"
    status=1
  fi
  [ -z "$INSTALL_DOWNLOAD" ] || rm -f -- "$INSTALL_DOWNLOAD"
  [ -z "$INSTALL_CHECKSUM" ] || rm -f -- "$INSTALL_CHECKSUM"
  if [ "$INSTALL_RECOVERY_FAILED" != "true" ]; then
    [ -z "$INSTALL_BACKUP" ] || rm -f -- "$INSTALL_BACKUP"
    [ -z "$INSTALL_MARKER" ] || rm -f -- "$INSTALL_MARKER"
  else
    status=1
  fi
  exit "$status"
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

  INSTALL_DEST="$dest"
  local marker="${dest}.upgrading"
  local prior_pid=""
  if [ -e "$marker" ]; then
    prior_pid=$(cat "$marker") || err "Cannot inspect an existing upgrade marker."
    case "$prior_pid" in
      ''|*[!0-9]*) err "Invalid upgrade marker; manual recovery required." ;;
    esac
    if kill -0 "$prior_pid" 2>/dev/null; then
      err "Another Mnemo installer is still running (PID $prior_pid)."
    fi
    # If the journal already exists, its recovery owns this marker and backup.
    # Otherwise the former installer died before starting the transaction.
    if [ ! -e "$HOME/.mnemo/upgrade.transaction" ]; then
      rm -f -- "$marker" "${dest}.rollback.${prior_pid}" ||
        err "Cannot clean up an interrupted pre-transaction download."
    fi
  fi
  # PID-addressed artifacts let an interrupted run clean only its own files.
  # Never remove an artifact belonging to an active concurrent installer.
  if [ -n "$prior_pid" ]; then
    rm -f -- "${dest}.download.${prior_pid}" "${dest}.checksum.${prior_pid}" ||
      err "Cannot clean interrupted download artifacts."
  fi
  local installer_pid="$"
  INSTALL_DOWNLOAD="${dest}.download.${installer_pid}"
  INSTALL_CHECKSUM="${dest}.checksum.${installer_pid}"
  ( umask 077; set -C; : > "$INSTALL_DOWNLOAD" ) ||
    err "Download artifact already exists for this installer."
  trap finish_install EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM
  fetch "$binary_url" "$INSTALL_DOWNLOAD" || err "Download failed: ${binary_url}"

  ( umask 077; set -C; : > "$INSTALL_CHECKSUM" ) ||
    err "Checksum artifact already exists for this installer."
  fetch_stdout "$checksum_url" > "$INSTALL_CHECKSUM" || err "Checksum download failed: ${checksum_url}"

  # shasum -c expects "hash  filename" — rewrite the path to match $tmp
  local expected_hash
  expected_hash=$(awk '{print $1}' "$INSTALL_CHECKSUM")
  local actual_hash
  actual_hash=$(shasum -a 256 "$INSTALL_DOWNLOAD" | awk '{print $1}')

  if [ "$expected_hash" != "$actual_hash" ]; then
    err "Checksum mismatch — aborting. Expected: ${expected_hash}, got: ${actual_hash}"
  fi

  chmod +x "$INSTALL_DOWNLOAD"
  # Recover abandoned installs before checking the old controller or reserving
  # a new transaction. An active concurrent installer is never overridden.
  if [ -f "$HOME/.mnemo/upgrade.transaction" ]; then
    "$INSTALL_DOWNLOAD" controller transaction recover || err "Unresolved Mnemo update; recovery was not possible."
    "$dest" setup refresh --agent="$INSTALL_RESTORE_AGENT" --mnemo-bin="$dest" || err "Could not restart recovered Mnemo."
  fi
  # Every upgrade waits for a stable, acknowledged durable queue before
  # replacing the controller executable. Active agent/Mnemo sessions stay open.
  if [ -f "$HOME/.mnemo/config.toml" ]; then
    info "Waiting for pending memory saves to complete before restarting the controller..."
    if ! "$INSTALL_DOWNLOAD" controller preflight; then
      err "Update canceled: pending saves did not finish or the controller is unavailable. The running controller and installed binary are unchanged."
    fi
  fi

  if [ -f "$dest" ]; then
    # Exclusive marker prevents concurrent installers and differentiates an
    # in-progress transaction from failures after a successful upgrade.
    local marker="${dest}.upgrading"
    local marker_pid="$"
    if ! ( set -C; echo "$marker_pid" > "$marker" ) 2>/dev/null; then
      err "Another Mnemo upgrade is already running or left an unresolved upgrade marker: ${marker}"
    fi
    INSTALL_MARKER="$marker"
    INSTALL_BACKUP="${dest}.rollback.${marker_pid}"
    ( umask 077; set -C; : > "$INSTALL_BACKUP" ) ||
      err "Rollback backup already exists for this installer."
    cp -p -- "$dest" "$INSTALL_BACKUP" || err "Could not preserve existing Mnemo executable."
  fi

  # Stop all database writers before the snapshot. During the subsequent
  # start-and-verify phase, the new controller will NAK real durable deliveries
  # until the installer commits the release. Active agent sessions stay open.
  if [ -f "$dest" ]; then
    "$INSTALL_DOWNLOAD" controller transaction begin --binary="$dest" --backup="$INSTALL_BACKUP" ||
      err "Could not persist the upgrade transaction."
    INSTALL_TXN=true
    if ! ( set -C; echo "$version" > "$HOME/.mnemo/upgrade.maintenance" ) 2>/dev/null; then
      err "An SQLite recovery transaction is already active."
    fi
    "$INSTALL_DOWNLOAD" controller stop || err "Could not stop the previous controller safely."
  fi

  if [ -f "$HOME/.mnemo/memory.db" ]; then
    if [ "$INSTALL_TXN" != "true" ]; then
      err "Existing SQLite database has no recoverable previous executable."
    fi
    # The restore point belongs to the *installed* release, not the target.
    # Include an attempt identifier so retries never overwrite earlier data.
    local previous_release
    previous_release=$("$dest" version | awk '{print $2}')
    local snapshot_id="${previous_release}-$(date -u +%Y%m%dT%H%M%S)-$$"
    "$INSTALL_DOWNLOAD" db recovery snapshot --release="$snapshot_id" || err "Could not preserve the pre-upgrade SQLite state."
    "$INSTALL_DOWNLOAD" controller transaction snapshot --release="$snapshot_id" ||
      err "Could not record the verified SQLite recovery point."
    if ! "$INSTALL_DOWNLOAD" db migrate --check >/dev/null 2>&1; then
      "$INSTALL_DOWNLOAD" db migrate || err "SQLite migration failed; restoring the pre-upgrade recovery point."
      "$INSTALL_DOWNLOAD" controller transaction schema --changed ||
        err "Could not record the schema compatibility boundary."
    fi
  fi

  # Both files live on the same filesystem, so the executable switch is atomic.
  mv -f -- "$INSTALL_DOWNLOAD" "$dest" || err "Could not activate downloaded Mnemo binary."
  INSTALL_DOWNLOAD=""
  INSTALL_SWITCHED=true
  info "New binary activated; verifying the managed service before committing..."
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
  INSTALL_RESTORE_AGENT="$agent"
  "$mnemo_bin" setup refresh --agent="$agent" --mnemo-bin="$mnemo_bin"
  ok "Global ${agent} setup refreshed. Run 'mnemo init --agent=${agent}' in projects that should use mnemo."
}


# ── agent detection ───────────────────────────────────────────────────────────

agent_detected() {
  case "$1" in
    claudecode)
      command -v claude >/dev/null 2>&1 || [ -d "${CLAUDE_CONFIG_DIR:-$HOME/.claude}" ] || [ -f "${CLAUDE_CONFIG_DIR:-$HOME}/.claude.json" ]
      ;;
    cursor)
      command -v cursor >/dev/null 2>&1 || [ -d "$HOME/.cursor" ]
      ;;
    codex)
      command -v codex >/dev/null 2>&1 || [ -d "$HOME/.codex" ]
      ;;
    opencode)
      command -v opencode >/dev/null 2>&1 || [ -d "$HOME/.config/opencode" ]
      ;;
    pi)
      command -v pi >/dev/null 2>&1 || [ -d "$HOME/.pi/agent" ]
      ;;
    *) return 1 ;;
  esac
}

detect_agents() {
  local found=""
  local agent
  for agent in claudecode cursor codex opencode pi; do
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
      for selected in claudecode cursor codex opencode pi; do
        setup_agent "$selected" "$mnemo_bin"
      done
      ok "Done. Run 'mnemo init --agent=all' in projects that should use mnemo."
      ;;
    claudecode|cursor|codex|opencode|pi)
      setup_agent "$AGENT" "$mnemo_bin"
      ok "Done. Run 'mnemo init --agent=${AGENT}' in projects that should use mnemo."
      ;;
    *)
      err "Unknown agent: ${AGENT}. Valid options: auto | claudecode | cursor | codex | opencode | pi | all"
      ;;
  esac
  # This functional probe is fully automatic and must pass before cleanup
  # removes the previous executable. It exercises durable events, commands,
  # and a real SQLite read through the newly restarted controller.
  if ! "$mnemo_bin" controller verify; then
    err "The new Mnemo controller failed functional verification. Rolling back."
  fi
  if [ "$INSTALL_TXN" = "true" ]; then
    # Commit is durable before releasing the write fence. If the process dies
    # during cleanup, the next controller sees a committed journal and will not
    # replay an old SQLite snapshot over acknowledged new writes.
    "$mnemo_bin" controller transaction commit || err "Could not commit release transaction."
    INSTALL_VERIFIED=true
    "$mnemo_bin" controller transaction finalize || err "Could not finalize release transaction."
    INSTALL_TXN=false
  else
    INSTALL_VERIFIED=true
  fi
  ok "Controller verified; upgrade committed."
}

main "$@"
