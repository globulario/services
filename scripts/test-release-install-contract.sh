#!/usr/bin/env bash
set -euo pipefail

# Validates the shared release-bundle contract used by both Day-0 installation
# and Day-1 join fallback.
# Usage: bash scripts/test-release-install-contract.sh [path-to-release-dir]

RELEASE_DIR="${1:-.}"

die() { echo "FAIL: $*" >&2; exit 1; }
pass() { echo "PASS: $*"; }

[[ -d "$RELEASE_DIR" ]] || die "release dir not found: $RELEASE_DIR"

echo "━━━ Release Install Contract Check ━━━"
echo "  Release dir: $RELEASE_DIR"
echo ""

errors=0

require_file() {
  local path="$1" label="$2"
  if [[ ! -f "$path" ]]; then
    echo "  FAIL: $label missing ($path)"
    errors=$((errors + 1))
  else
    echo "  OK:   $label present"
  fi
}

require_dir() {
  local path="$1" label="$2"
  if [[ ! -d "$path" ]]; then
    echo "  FAIL: $label missing ($path)"
    errors=$((errors + 1))
  else
    echo "  OK:   $label present"
  fi
}

require_file "$RELEASE_DIR/install.sh" "install.sh"
require_file "$RELEASE_DIR/globular-installer" "globular-installer"
require_file "$RELEASE_DIR/release-index.json" "release-index.json"
require_file "$RELEASE_DIR/scripts/install-day0.sh" "scripts/install-day0.sh"
require_dir "$RELEASE_DIR/packages" "packages/"
require_dir "$RELEASE_DIR/workflows" "workflows/"

if [[ -f "$RELEASE_DIR/install.sh" ]]; then
  if grep -Fq 'install -m 755 "${INSTALLER_BIN}" /usr/lib/globular/bin/globular-installer' "$RELEASE_DIR/install.sh"; then
    echo "  OK:   install.sh persists globular-installer to /usr/lib/globular/bin"
  else
    echo "  FAIL: install.sh does not persist globular-installer for Day-1 fallback"
    errors=$((errors + 1))
  fi
fi

if [[ -f "$RELEASE_DIR/globular-installer" ]] && [[ ! -x "$RELEASE_DIR/globular-installer" ]]; then
  echo "  FAIL: globular-installer is not executable"
  errors=$((errors + 1))
fi

if [[ "$errors" -ne 0 ]]; then
  echo ""
  echo "$errors error(s) — shared Day-0/Day-1 release contract is broken"
  exit 1
fi

echo ""
pass "Shared Day-0/Day-1 release contract satisfied"
