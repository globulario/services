#!/usr/bin/env bash
# awareness-intent-audit-check.sh — CI/pre-commit intent audit check.
#
# Usage:
#   scripts/awareness-intent-audit-check.sh              # strict (fail on violations)
#   scripts/awareness-intent-audit-check.sh --lenient     # lenient (warn only)
#   scripts/awareness-intent-audit-check.sh --strict      # strict (fail on violations + missing tests)
#
# This script runs the intent audit and appends history to
# docs/intent/meta/audit_history.jsonl for regression tracking.
#
# Exit codes:
#   0 — clean (no violations)
#   1 — violations found
#   2 — tool error

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Find the globular binary.
GLOBULAR="${GLOBULAR_BIN:-globular}"
if ! command -v "$GLOBULAR" &>/dev/null; then
  # Try local build.
  if [ -f "$REPO_ROOT/golang/globularcli/globularcli" ]; then
    GLOBULAR="$REPO_ROOT/golang/globularcli/globularcli"
  else
    echo "error: globular binary not found. Set GLOBULAR_BIN or build with 'go build ./globularcli/'" >&2
    exit 2
  fi
fi

# Parse mode.
FAIL_ON="violation"
case "${1:-}" in
  --lenient)  FAIL_ON="none" ;;
  --strict)   FAIL_ON="missing-test" ;;
  *)          FAIL_ON="violation" ;;
esac

HISTORY_FILE="$REPO_ROOT/docs/intent/meta/audit_history.jsonl"

echo "Intent audit: running with --fail-on $FAIL_ON"
exec "$GLOBULAR" awareness intent-audit \
  --fail-on "$FAIL_ON" \
  --history "$HISTORY_FILE" \
  --format text
