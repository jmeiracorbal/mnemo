#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT
mkdir -p "$fixture/home/.mnemo" "$fixture/bin" "$fixture/mock-bin"
printf '[events]\nport = 4222\n' > "$fixture/home/.mnemo/config.toml"

cat > "$fixture/old" <<'OLD'
#!/usr/bin/env bash
case "$1/$2" in
  setup/refresh) printf old > "$HOME/active-controller" ;;
  stats/) [ "$(cat "$HOME/active-controller" 2>/dev/null)" = old ] ;;
  version/) printf 'mnemo v1.1.0\n' ;;
  *) exit 1 ;;
esac
OLD

cat > "$fixture/new" <<'NEW'
#!/usr/bin/env bash
case "$1/$2" in
  controller/preflight) printf 'preflight\n' >> "$MOCK_LOG" ;;
  controller/stop) printf stopped > "$HOME/active-controller" ;;
  controller/verify)
    printf 'verify\n' >> "$MOCK_LOG"
    [ "${MOCK_VERIFY_RESULT:-ok}" = ok ] && [ "$(cat "$HOME/active-controller" 2>/dev/null)" = new ]
    ;;
  db/recovery)
    release="${4#--release=}"
    mkdir -p "$HOME/.mnemo/recovery"
    case "$3" in
      snapshot)
        [ "${MOCK_SNAPSHOT_RESULT:-ok}" = ok ] || exit 1
        cp "$HOME/.mnemo/memory.db" "$HOME/.mnemo/recovery/$release.db"
        ;;
      restore) cp "$HOME/.mnemo/recovery/$release.db" "$HOME/.mnemo/memory.db" ;;
      *) exit 1 ;;
    esac
    ;;
  db/migrate)
    if [ "${3:-}" = "--check" ]; then
      [ "${MOCK_MIGRATION_RESULT:-ready}" = ready ] || exit 2
    else
      printf migrated-schema > "$HOME/.mnemo/memory.db"
      [ "${MOCK_APPLY_RESULT:-ok}" = ok ]
    fi
    ;;
  setup/refresh) printf new > "$HOME/active-controller" ;;
  stats/) [ "$(cat "$HOME/active-controller" 2>/dev/null)" = new ] ;;
  version/) printf 'mnemo v99.0.0\n' ;;
  *) exit 1 ;;
esac
NEW
chmod +x "$fixture/old" "$fixture/new"

cat > "$fixture/mock-bin/curl" <<'CURL'
#!/usr/bin/env bash
case "$1" in
  -sSfI) exit 0 ;;
  -sSfL)
    if [ "${3:-}" = "-o" ]; then
      cp "$MOCK_RELEASE_BIN" "$4"
    else
      shasum -a 256 "$MOCK_RELEASE_BIN"
    fi
    ;;
  *) exit 1 ;;
esac
CURL
chmod +x "$fixture/mock-bin/curl"

export HOME="$fixture/home"
export MNEMO_INSTALL_DIR="$fixture/bin"
export MNEMO_VERSION=v99.0.0
export MNEMO_AGENT=codex
export MOCK_RELEASE_BIN="$fixture/new"
export MOCK_LOG="$fixture/log"
export PATH="$fixture/mock-bin:$PATH"

reset_old() {
  cp "$fixture/old" "$MNEMO_INSTALL_DIR/mnemo"
  printf old > "$HOME/active-controller"
  rm -f "$HOME/.mnemo/memory.db"
}

check_no_residue() {
  [ ! -e "$MNEMO_INSTALL_DIR/mnemo.upgrading" ]
  [ ! -e "$HOME/.mnemo/upgrade.maintenance" ]
  if find "$MNEMO_INSTALL_DIR" -type f \( -name 'mnemo.rollback.*' -o -name 'mnemo.download.*' -o -name 'mnemo.checksum.*' \) | grep -q .; then
    echo 'installer left temporary files behind' >&2
    exit 1
  fi
}

# No database: the new controller must be verified without user action.
reset_old
MOCK_VERIFY_RESULT=ok bash "$repo_root/install.sh" >/dev/null
[ "$("$MNEMO_INSTALL_DIR/mnemo" version)" = "mnemo v99.0.0" ]
[ "$(cat "$HOME/active-controller")" = new ]
[ "$("$MNEMO_INSTALL_DIR/mnemo.previous" version)" = "mnemo v1.1.0" ]
grep -q '^preflight$' "$MOCK_LOG"
grep -q '^verify$' "$MOCK_LOG"
check_no_residue

# A failed controller verification restores the prior binary and service.
reset_old
if MOCK_VERIFY_RESULT=fail bash "$repo_root/install.sh" > "$fixture/failure.log" 2>&1; then
  echo 'installer succeeded despite failed controller verification' >&2
  exit 1
fi
[ "$("$MNEMO_INSTALL_DIR/mnemo" version)" = "mnemo v1.1.0" ]
[ "$(cat "$HOME/active-controller")" = old ]
grep -q 'previous Mnemo' "$fixture/failure.log"
check_no_residue

# Pending SQLite migrations are now allowed: snapshot first, migrate, verify,
# then remove the write fence so queued durable writes can be processed.
reset_old
printf original-schema > "$HOME/.mnemo/memory.db"
MOCK_MIGRATION_RESULT=pending bash "$repo_root/install.sh" >/dev/null
[ "$(cat "$HOME/.mnemo/memory.db")" = migrated-schema ]
[ "$(cat "$HOME/active-controller")" = new ]
[ "$(find "$HOME/.mnemo/recovery" -type f -name 'v1.1.0-*.db' | wc -l | tr -d ' ')" -ge 1 ]
check_no_residue

# Failure after a schema migration must recover the exact pre-upgrade SQLite
# content and previous executable before allowing user writes again.
reset_old
printf original-schema > "$HOME/.mnemo/memory.db"
if MOCK_MIGRATION_RESULT=pending MOCK_VERIFY_RESULT=fail bash "$repo_root/install.sh" > "$fixture/db-rollback.log" 2>&1; then
  echo 'installer succeeded despite failed post-migration verification' >&2
  exit 1
fi
[ "$("$MNEMO_INSTALL_DIR/mnemo" version)" = "mnemo v1.1.0" ]
[ "$(cat "$HOME/.mnemo/memory.db")" = original-schema ]
[ "$(cat "$HOME/active-controller")" = old ]
check_no_residue

# A partial migration failure is recovered without starting the new service.
reset_old
printf original-schema > "$HOME/.mnemo/memory.db"
if MOCK_MIGRATION_RESULT=pending MOCK_APPLY_RESULT=fail bash "$repo_root/install.sh" > "$fixture/db-failure.log" 2>&1; then
  echo 'installer succeeded despite failed SQLite migration' >&2
  exit 1
fi
[ "$(cat "$HOME/.mnemo/memory.db")" = original-schema ]
[ "$(cat "$HOME/active-controller")" = old ]
check_no_residue
