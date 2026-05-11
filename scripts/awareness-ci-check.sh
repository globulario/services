#!/usr/bin/env bash
# awareness-ci-check.sh — coverage-ratchet gate for the awareness graph.
#
# This is a stub, not a full CI workflow. It runs `globular awareness
# meta-check` with the current-state-compatible thresholds so a regression
# in coverage is immediately visible. It does NOT add a GitHub Actions
# workflow; that decision is left for whoever wires this up.
#
# Pair with scripts/awareness-ci.sh (which checks annotations + audit).
# This script handles the orthogonal coverage axis.
#
# Usage:
#   ./scripts/awareness-ci-check.sh [--repo <path>] [--db <path>] [--strict]
#
# Modes:
#   Default — exit non-zero only on regression below the floors below.
#   --strict — also fail on any orphan FailureMode (--orphans-fail) and on
#             any unknown-role YAML in docs/awareness.
#
# Exit codes:
#   0  All ratchets satisfied; coverage has not regressed.
#   1  Ratchet violated (regression). Read stderr for the specific floor.
#   2  Tooling error (graph missing, CLI not on PATH, etc.).
#
# Honest defaults: thresholds match the current cluster state so this gate
# does not suddenly fail because coverage is honestly limited. When coverage
# improves, raise the floors here in the SAME PR that adds the new
# enforcement; the script ratchets, it doesn't aspire.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DB_PATH=""
STRICT=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --repo)   REPO_ROOT="$2"; shift 2 ;;
        --db)     DB_PATH="$2";   shift 2 ;;
        --strict) STRICT=1;       shift   ;;
        -h|--help)
            sed -n '2,28p' "$0"
            exit 0
            ;;
        *)
            echo "unknown flag: $1" >&2
            exit 2
            ;;
    esac
done

# Pinned floors. These match the cluster state on 2026-05-10 and earn the
# right to ratchet up by being the actual current count, not an aspiration.
# When you raise these, do it in the same PR that adds the enforcement that
# justifies the new floor.
MIN_WELL_COVERED=10
MIN_DETECTED=23
BASELINE_ORPHANS=0

# Locate the CLI. Prefer the canonical system install (kept current by
# package install) over $PATH (which may carry a stale developer build).
# Override with GLOBULARCLI env var for non-standard layouts.
GLOBULARCLI="${GLOBULARCLI:-}"
if [[ -z "$GLOBULARCLI" ]]; then
    if [[ -x "/usr/lib/globular/bin/globularcli" ]]; then
        GLOBULARCLI="/usr/lib/globular/bin/globularcli"
    elif command -v globularcli >/dev/null 2>&1; then
        GLOBULARCLI="$(command -v globularcli)"
    elif [[ -x "$REPO_ROOT/globularcli" ]]; then
        GLOBULARCLI="$REPO_ROOT/globularcli"
    else
        echo "awareness-ci-check: globularcli not found at /usr/lib/globular/bin/, on PATH, or in repo root" >&2
        echo "  install: go build -o globularcli ./golang/globularcli" >&2
        echo "  override: GLOBULARCLI=/path/to/globularcli ./scripts/awareness-ci-check.sh" >&2
        exit 2
    fi
fi

ARGS=(
    awareness meta-check
    --repo "$REPO_ROOT"
    --min-well-covered "$MIN_WELL_COVERED"
    --min-detected "$MIN_DETECTED"
    --baseline-orphans "$BASELINE_ORPHANS"
    --critical-orphans-fail
)

if [[ -n "$DB_PATH" ]]; then
    ARGS+=(--db "$DB_PATH")
fi

if [[ -n "$STRICT" ]]; then
    ARGS+=(--orphans-fail --max-orphans 0)
fi

echo "=== Awareness coverage-ratchet check ==="
echo "  repo:               $REPO_ROOT"
echo "  min_well_covered:   $MIN_WELL_COVERED"
echo "  min_detected:       $MIN_DETECTED"
echo "  baseline_orphans:   $BASELINE_ORPHANS"
echo "  critical_orphans:   fail"
[[ -n "$STRICT" ]] && echo "  mode:               strict"
echo ""

set +e
"$GLOBULARCLI" "${ARGS[@]}"
status=$?
set -e

if [[ $status -ne 0 ]]; then
    echo "" >&2
    echo "awareness-ci-check: coverage ratchet violated (exit $status)" >&2
    echo "  Review the meta-check output above for the specific floor that regressed." >&2
    echo "  If the regression is intentional and earned (a failure_mode was deliberately deprecated)," >&2
    echo "  lower the corresponding floor in scripts/awareness-ci-check.sh in the same PR." >&2
    exit 1
fi

echo ""
echo "awareness-ci-check: coverage ratchets satisfied."
