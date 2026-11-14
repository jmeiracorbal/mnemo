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
  setup/refresh) printf 'old' > "$HOME/active-controller" ;;
  stats/) [ "$(cat "$HOME/active-controller" 2>/dev/null)" = old ] ;;
  version/) printf 'old\n' ;;
  *) exit 1 ;;
esac
OLD

cat > "$fixture/new" <<'NEW'
#!/usr/bin/env bash
case "$1/$2" in
  controller/preflight) printf 'preflight\n' >> "$MOCK_LOG" ;;
  db/migrate) [ "${MOCK_MIGRATION_RESULT:-ready}" = ready ] ;;
  controller/verify)
    printf 'verify\n' >> "$MOCK_LOG"
    [ "${MOCK_VERIFY_RESULT:-ok}" = ok ] && [ "$(cat "$HOME/active-controller" 2>/dev/null)" = new ]
    ;;
  setup/refresh) printf 'new' > "$HOME/active-controller" ;;
  version/) printf 'new\n' ;;
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

check_no_residue() {
  if find "$MNEMO_INSTALL_DIR" -type f \( -name 'mnemo.rollback.*' -o -name 'mnemo.download.*' -o -name 'mnemo.checksum.*' \) | grep -q .; then
    echo 'installer left temporary files behind' >&2
    exit 1
  fi
}

cp "$fixture/old" "$MNEMO_INSTALL_DIR/mnemo"
printf old > "$HOME/active-controller"
MOCK_VERIFY_RESULT=ok bash "$repo_root/install.sh" >/dev/null
[ "$("$MNEMO_INSTALL_DIR/mnemo" version)" = new ]
[ "$(cat "$HOME/active-controller")" = new ]
grep -q '^preflight$' "$MOCK_LOG"
grep -q '^verify$' "$MOCK_LOG"
check_no_residue

cp "$fixture/old" "$MNEMO_INSTALL_DIR/mnemo"
printf old > "$HOME/active-controller"
if MOCK_VERIFY_RESULT=fail bash "$repo_root/install.sh" > "$fixture/failure.log" 2>&1; then
  echo 'installer succeeded despite a failed controller verification' >&2
  exit 1
fi
[ "$("$MNEMO_INSTALL_DIR/mnemo" version)" = old ]
[ "$(cat "$HOME/active-controller")" = old ]
grep -q 'Previous Mnemo version restored and responding' "$fixture/failure.log"
check_no_residue

# Unsafe SQLite migrations must fail before replacing the binary.
cp "$fixture/old" "$MNEMO_INSTALL_DIR/mnemo"
printf old > "$HOME/active-controller"
touch "$HOME/.mnemo/memory.db"
if MOCK_MIGRATION_RESULT=pending bash "$repo_root/install.sh" > "$fixture/migration.log" 2>&1; then
  echo 'installer replaced a binary despite an unsafe schema migration' >&2
  exit 1
fi
[ "$("$MNEMO_INSTALL_DIR/mnemo" version)" = old ]
[ "$(cat "$HOME/active-controller")" = old ]
grep -q 'requires a database migration' "$fixture/migration.log"
check_no_residue
