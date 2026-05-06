#!/usr/bin/env bash
# awareness-ci.sh — CI enforcement gate for Globular awareness annotations.
#
# Usage:
#   ./scripts/awareness-ci.sh [--skip-build] [--strict]
#
# Modes:
#   Default  — fail on ERROR; suppress known warning backlog; print grouped summary.
#   --strict — additionally fail on unsuppressed warnings, expired suppressions,
#              max_count violations, and invalid suppression entries.
#
# Phases:
#   1. Build (or verify) the awareness graph.
#   2. Validate all //globular: annotations are well-formed.
#   3. Run the full audit (contracts, required tests, graph drift) with suppressions.
#
# Exits 1 if any ERROR finding is detected, or if --strict conditions are violated.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKIP_BUILD=""
STRICT=""
for arg in "$@"; do
    case "$arg" in
        --skip-build) SKIP_BUILD=1 ;;
        --strict)     STRICT=1     ;;
    esac
done

SUPPRESSIONS="$REPO_ROOT/docs/awareness/audit_suppressions.yaml"

cd "$REPO_ROOT"

echo "=== Awareness CI ==="
echo ""

# Phase 1 — Build the graph (unless skipped with --skip-build).
if [[ -z "$SKIP_BUILD" ]]; then
    echo "[1/3] Building awareness graph..."
    globular awareness build --repo "$REPO_ROOT"
    echo ""
fi

# Phase 2 — Annotation syntax check (fast, no graph required).
echo "[2/3] Validating annotation syntax..."
if ! globular awareness validate-annotations --repo "$REPO_ROOT" --json > /tmp/awareness-annotations.json 2>&1; then
    echo "FAIL: annotation validation errors found"
    cat /tmp/awareness-annotations.json
    exit 1
fi
ANNOTATION_ERRORS=$(jq '.error_count' /tmp/awareness-annotations.json 2>/dev/null || echo "0")
echo "    Annotation errors: $ANNOTATION_ERRORS"
echo ""

# Phase 3 — Full audit with suppressions.
echo "[3/3] Running full awareness audit..."

AUDIT_FLAGS=(
    --repo "$REPO_ROOT"
    --suppressions "$SUPPRESSIONS"
    --json
)

if [[ -n "$STRICT" ]]; then
    AUDIT_FLAGS+=(--strict)
fi

if ! globular awareness audit "${AUDIT_FLAGS[@]}" > /tmp/awareness-audit.json 2>&1; then
    AUDIT_ERRORS=$(jq '.error_count // 0' /tmp/awareness-audit.json 2>/dev/null || echo "unknown")
    AUDIT_WARNINGS=$(jq '.warning_count // 0' /tmp/awareness-audit.json 2>/dev/null || echo "unknown")
    AUDIT_SUPPRESSED=$(jq '.suppressed_count // 0' /tmp/awareness-audit.json 2>/dev/null || echo "unknown")

    echo ""
    echo "FAIL — Awareness audit did not pass."
    echo "    Errors:     $AUDIT_ERRORS"
    echo "    Warnings:   $AUDIT_WARNINGS (unsuppressed)"
    echo "    Suppressed: $AUDIT_SUPPRESSED"
    echo ""

    # Print ERROR findings.
    jq -r '.findings[]? | select(.severity == "ERROR") | "  [ERROR] \(.file // ""): \(.message)"' \
        /tmp/awareness-audit.json 2>/dev/null || true

    # In strict mode, also print max_count violations and expired suppressions.
    if [[ -n "$STRICT" ]]; then
        jq -r '.max_count_violations[]? | "  [MAX_COUNT] \(.suppression_id): \(.actual_count) found, max=\(.max_count)"' \
            /tmp/awareness-audit.json 2>/dev/null || true
        jq -r '.expired_suppressions[]? | "  [EXPIRED] \(.)"' \
            /tmp/awareness-audit.json 2>/dev/null || true
    fi

    exit 1
fi

AUDIT_ERRORS=$(jq '.error_count // 0' /tmp/awareness-audit.json 2>/dev/null || echo "0")
AUDIT_WARNINGS=$(jq '.warning_count // 0' /tmp/awareness-audit.json 2>/dev/null || echo "0")
AUDIT_SUPPRESSED=$(jq '.suppressed_count // 0' /tmp/awareness-audit.json 2>/dev/null || echo "0")

echo "    Errors:     $AUDIT_ERRORS"
echo "    Warnings:   $AUDIT_WARNINGS (unsuppressed)"
echo "    Suppressed: $AUDIT_SUPPRESSED (see docs/awareness/audit_suppressions.yaml)"
echo ""

if [[ "$AUDIT_WARNINGS" -gt 0 ]]; then
    echo "NOTICE — $AUDIT_WARNINGS unsuppressed warning(s). Review with:"
    echo "    globular awareness audit --repo '$REPO_ROOT' --suppressions '$SUPPRESSIONS'"
    echo ""
fi

echo "=== Awareness CI: PASS ==="
