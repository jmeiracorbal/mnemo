#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT
trap 'status=$?; printf "installer test failed at line %s (exit=%s): %s\\n" "$LINENO" "$status" "$BASH_COMMAND" >&2; if [ -f "$fixture/recovered.log" ]; then printf "%s\\n" "-- recovery log --" >&2; cat "$fixture/recovered.log" >&2; fi; exit "$status"' ERR
set -E
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
  controller/preflight) printf 'preflight\n' >> "$MOCK_LOG"; [ "${MOCK_PREFLIGHT_RESULT:-ok}" = ok ] ;;
  controller/stop) printf stopped > "$HOME/active-controller" ;;
  controller/transaction)
    case "$3" in
      begin)
        printf '%s' "$4" | sed 's/^--binary=//' > "$HOME/.mnemo/txn-binary"
        printf '%s' "$5" | sed 's/^--backup=//' > "$HOME/.mnemo/txn-backup"
        printf prepared > "$HOME/.mnemo/upgrade.transaction"
        ;;
      snapshot) printf '%s' "$4" | sed 's/^--release=//' > "$HOME/.mnemo/txn-snapshot" ;;
      schema) printf changed > "$HOME/.mnemo/txn-schema" ;;
      commit) printf committed > "$HOME/.mnemo/upgrade.transaction" ;;
      finalize)
        [ "$(cat "$HOME/.mnemo/upgrade.transaction")" = committed ] || exit 1
        cp "$(cat "$HOME/.mnemo/txn-backup")" "$(cat "$HOME/.mnemo/txn-binary").previous"
        rm -f "$(cat "$HOME/.mnemo/txn-backup")"
        rm -f "$HOME/.mnemo/upgrade.maintenance" "$HOME/.mnemo/upgrade.transaction"
        rm -f "$HOME/.mnemo/txn-binary" "$HOME/.mnemo/txn-backup" "$HOME/.mnemo/txn-snapshot" "$HOME/.mnemo/txn-schema"
        ;;
      recover)
        if [ -f "$HOME/.mnemo/txn-snapshot" ]; then
          release=$(cat "$HOME/.mnemo/txn-snapshot")
          cp "$HOME/.mnemo/recovery/$release.db" "$HOME/.mnemo/memory.db"
        fi
        cp "$(cat "$HOME/.mnemo/txn-backup")" "$(cat "$HOME/.mnemo/txn-binary")"
        rm -f "$(cat "$HOME/.mnemo/txn-binary").upgrading"
        rm -f "$(cat "$HOME/.mnemo/txn-backup")"
        rm -f "$HOME/.mnemo/upgrade.maintenance" "$HOME/.mnemo/upgrade.transaction"
        rm -f "$HOME/.mnemo/txn-binary" "$HOME/.mnemo/txn-backup" "$HOME/.mnemo/txn-snapshot" "$HOME/.mnemo/txn-schema"
        ;;
      *) exit 1 ;;
    esac
    ;;
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
      [ "${MOCK_APPLY_RESULT:-ok}" = ok ] || exit 1
      if [ "${MOCK_KILL_INSTALLER:-no}" = yes ]; then
        kill -KILL "$PPID"
      fi
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
      if [ "${MOCK_KILL_DOWNLOAD:-no}" = yes ]; then
        kill -KILL "$PPID"
      fi
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
  rm -f "$HOME/.mnemo/memory.db" "$MNEMO_INSTALL_DIR/mnemo.schema-changed"
}

check_no_residue() {
  [ ! -e "$MNEMO_INSTALL_DIR/mnemo.upgrading" ]
  [ ! -e "$MNEMO_INSTALL_DIR/mnemo.installing" ]
  [ ! -e "$HOME/.mnemo/upgrade.maintenance" ]
  [ ! -e "$HOME/.mnemo/upgrade.transaction" ]
  [ ! -e "$HOME/.mnemo/upgrade.transaction.writing" ]
  if find "$MNEMO_INSTALL_DIR" -type f \( -name 'mnemo.rollback.*' -o -name 'mnemo.download.*' -o -name 'mnemo.checksum.*' \) | grep -q .; then
    echo 'installer left temporary files behind:' >&2
    find "$MNEMO_INSTALL_DIR" -type f \( -name 'mnemo.rollback.*' -o -name 'mnemo.download.*' -o -name 'mnemo.checksum.*' \) -print >&2
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
grep -q 'Previous Mnemo' "$fixture/failure.log"
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

# A hard crash after migration leaves a durable transaction. The next
# invocation recovers the prior binary and database before preflight.
reset_old
printf original-schema > "$HOME/.mnemo/memory.db"
if MOCK_MIGRATION_RESULT=pending MOCK_KILL_INSTALLER=yes bash "$repo_root/install.sh" > "$fixture/crashed.log" 2>&1; then
  echo 'installer survived the injected hard crash' >&2
  exit 1
fi
[ -e "$HOME/.mnemo/upgrade.transaction" ]
[ -e "$HOME/.mnemo/upgrade.maintenance" ]
if MOCK_PREFLIGHT_RESULT=fail bash "$repo_root/install.sh" > "$fixture/recovered.log" 2>&1; then
  echo 'installer continued despite deliberately failed preflight' >&2
  exit 1
fi
[ "$("$MNEMO_INSTALL_DIR/mnemo" version)" = "mnemo v1.1.0" ]
[ "$(cat "$HOME/.mnemo/memory.db")" = original-schema ]
[ "$(cat "$HOME/active-controller")" = old ]
check_no_residue

# SIGKILL before the journal exists must also leave no download residue after
# the next attempt. The previous executable and SQLite are never changed.
reset_old
printf original-schema > "$HOME/.mnemo/memory.db"
if MOCK_KILL_DOWNLOAD=yes bash "$repo_root/install.sh" > "$fixture/download-crash.log" 2>&1; then
  echo 'installer survived the injected download crash' >&2
  exit 1
fi
[ -e "$MNEMO_INSTALL_DIR/mnemo.installing" ]
[ ! -e "$HOME/.mnemo/upgrade.transaction" ]
if MOCK_PREFLIGHT_RESULT=fail bash "$repo_root/install.sh" > "$fixture/retry-download.log" 2>&1; then
  echo 'retry continued despite failed preflight' >&2
  exit 1
fi
[ "$("$MNEMO_INSTALL_DIR/mnemo" version)" = "mnemo v1.1.0" ]
[ "$(cat "$HOME/.mnemo/memory.db")" = original-schema ]
check_no_residue
