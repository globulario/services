#!/usr/bin/env bash
# awareness-ci.sh — CI enforcement gate for Globular awareness annotations.
#
# Usage: ./scripts/awareness-ci.sh [--skip-build]
#
# Runs in three phases:
#   1. Build (or verify) the awareness graph.
#   2. Validate all //globular: annotations are well-formed.
#   3. Run the full audit (contracts, required tests, graph drift).
#
# Exits 1 if any ERROR finding is detected.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKIP_BUILD="${1:-}"

cd "$REPO_ROOT"

echo "=== Awareness CI ==="
echo ""

# Phase 1 — Build the graph (unless skipped with --skip-build).
if [[ "$SKIP_BUILD" != "--skip-build" ]]; then
    echo "[1/3] Building awareness graph..."
    globular awareness build --repo "$REPO_ROOT"
    echo ""
fi

# Phase 2 — Annotation syntax check.
echo "[2/3] Validating annotation syntax..."
if ! globular awareness validate-annotations --repo "$REPO_ROOT" --json > /tmp/awareness-annotations.json 2>&1; then
    echo "FAIL: annotation validation errors found"
    cat /tmp/awareness-annotations.json
    exit 1
fi
ANNOTATION_ERRORS=$(jq '.error_count' /tmp/awareness-annotations.json 2>/dev/null || echo "0")
echo "    Annotation errors: $ANNOTATION_ERRORS"
echo ""

# Phase 3 — Full audit (contracts + tests + drift).
echo "[3/3] Running full awareness audit..."
if ! globular awareness audit --repo "$REPO_ROOT" --json > /tmp/awareness-audit.json 2>&1; then
    echo "FAIL: awareness audit errors found"
    cat /tmp/awareness-audit.json | jq '.findings[] | select(.severity == "ERROR")'
    exit 1
fi

AUDIT_ERRORS=$(jq '.error_count' /tmp/awareness-audit.json 2>/dev/null || echo "0")
AUDIT_WARNINGS=$(jq '.warning_count' /tmp/awareness-audit.json 2>/dev/null || echo "0")

echo "    Errors:   $AUDIT_ERRORS"
echo "    Warnings: $AUDIT_WARNINGS"
echo ""

if [[ "$AUDIT_ERRORS" -gt 0 ]]; then
    echo "FAIL — $AUDIT_ERRORS error(s) found."
    jq '.findings[] | select(.severity == "ERROR") | "  [\(.code)] \(.file // ""): \(.message)"' \
        /tmp/awareness-audit.json | tr -d '"'
    exit 1
fi

echo "=== Awareness CI: PASS ==="
