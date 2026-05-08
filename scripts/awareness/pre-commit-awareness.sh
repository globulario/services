#!/usr/bin/env bash
# pre-commit-awareness.sh — Awareness guard before committing Globular code.
#
# Usage:
#   bash scripts/awareness/pre-commit-awareness.sh [--dry-run] [--files "a.go b.go"]
#
# Exit codes:
#   0  clean or warnings only
#   1  critical violation detected
#
# Install as git pre-commit hook:
#   cp scripts/awareness/pre-commit-awareness.sh .git/hooks/pre-commit
#   chmod +x .git/hooks/pre-commit
#
# Or run manually before a push:
#   bash scripts/awareness/pre-commit-awareness.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$REPO_ROOT"

DRY_RUN=false
EXPLICIT_FILES=""

for arg in "$@"; do
  case "$arg" in
    --dry-run) DRY_RUN=true ;;
    --files=*) EXPLICIT_FILES="${arg#--files=}" ;;
    --files) shift; EXPLICIT_FILES="${1:-}" ;;
  esac
done

GLOBULAR="${GLOBULAR:-/usr/lib/globular/bin/globularcli}"
if ! command -v "$GLOBULAR" >/dev/null 2>&1; then
  # Try local build.
  if [ -f "$REPO_ROOT/golang/bin/globular" ]; then
    GLOBULAR="$REPO_ROOT/golang/bin/globular"
  else
    echo "⚠  globular CLI not found — skipping awareness guard" >&2
    exit 0
  fi
fi

# Graph DB path — prefer system install, fall back to repo-relative for dev.
if [ -z "${GRAPH_DB:-}" ]; then
  if [ -f "/var/lib/globular/awareness/graph.db" ]; then
    GRAPH_DB="/var/lib/globular/awareness/graph.db"
  else
    GRAPH_DB="$REPO_ROOT/.globular/awareness/graph.db"
  fi
fi
DOCS_DIR="${DOCS_DIR:-$REPO_ROOT/docs/awareness}"

EXIT_CODE=0
WARNINGS=()
ERRORS=()

log_warn() { WARNINGS+=("$1"); echo "  ⚠  $1" >&2; }
log_error() { ERRORS+=("$1"); echo "  ✗  $1" >&2; EXIT_CODE=1; }
log_ok() { echo "  ✓  $1" >&2; }

echo "AWARENESS PRE-COMMIT GUARD" >&2
echo "" >&2

# ── Step 1: Session start (graph freshness + runtime) ───────────────────────
echo "[ 1/4 ] Session start check..." >&2
SESSION_JSON=$("$GLOBULAR" awareness session-start --repo "$REPO_ROOT" --db "$GRAPH_DB" --output json 2>/dev/null) || {
  log_warn "session-start failed — awareness may be degraded"
  SESSION_JSON="{}"
}

if command -v jq >/dev/null 2>&1; then
  GRAPH_STALE=$(echo "$SESSION_JSON" | jq -r '.graph.stale // false')
  GRAPH_AVAILABLE=$(echo "$SESSION_JSON" | jq -r '.graph.available // false')
  RUNTIME_STATUS=$(echo "$SESSION_JSON" | jq -r '.runtime.status // "unknown"')
else
  GRAPH_STALE=$(echo "$SESSION_JSON" | grep -oE '"stale"\s*:\s*true' | head -1 || true)
  GRAPH_AVAILABLE=$(echo "$SESSION_JSON" | grep -oE '"available"\s*:\s*true' | head -1 || true)
  RUNTIME_STATUS=$(echo "$SESSION_JSON" | grep -oP '"status"\s*:\s*"\K[^"]+' | head -2 | tail -1 || echo "unknown")
fi

if [ "$GRAPH_STALE" = "true" ]; then GRAPH_STALE="true"; else GRAPH_STALE=""; fi
if [ "$GRAPH_AVAILABLE" = "true" ]; then GRAPH_AVAILABLE="true"; else GRAPH_AVAILABLE=""; fi

if [ -z "$GRAPH_AVAILABLE" ]; then
  log_warn "Awareness graph not available — run 'globular awareness build'. Proceeding with degraded checks."
elif [ -n "$GRAPH_STALE" ]; then
  log_warn "Graph stale — some impact checks may be incomplete. Run 'globular awareness build'."
else
  log_ok "Graph available and fresh"
fi

if [ "$RUNTIME_STATUS" = "noop" ]; then
  log_warn "Runtime noop — no live cluster evidence. Static checks only."
fi

# ── Step 2: Changed files impact check ─────────────────────────────────────
echo "" >&2
echo "[ 2/4 ] File impact check..." >&2

if [ -n "$EXPLICIT_FILES" ]; then
  CHANGED_FILES="$EXPLICIT_FILES"
else
  # Get staged files from git.
  CHANGED_FILES=$(git diff --cached --name-only --diff-filter=ACM 2>/dev/null || true)
  if [ -z "$CHANGED_FILES" ]; then
    CHANGED_FILES=$(git diff --name-only --diff-filter=ACM HEAD 2>/dev/null || true)
  fi
fi

if [ -z "$CHANGED_FILES" ]; then
  log_warn "No changed files detected — skipping file impact check"
else
  CRITICAL_FILES=()
  for f in $CHANGED_FILES; do
    # Only check Go and awareness YAML files.
    case "$f" in
      *.go|docs/awareness/*.yaml|docs/awareness/knowledge/*.yaml) ;;
      *) continue ;;
    esac

    IMPACT_JSON=$("$GLOBULAR" awareness impact --file "$f" --db "$GRAPH_DB" --output json 2>/dev/null) || {
      log_warn "impact check failed for $f"
      continue
    }

    RISK=$(echo "$IMPACT_JSON" | grep -oP '"risk":"\K[^"]+' | head -1 || echo "unknown")
    FORBIDDEN=$(echo "$IMPACT_JSON" | grep -o '"forbidden_fixes":\[' | head -1 || true)

    if [ -n "$FORBIDDEN" ] && [ "$RISK" = "high" ]; then
      CRITICAL_FILES+=("$f")
      log_warn "High-risk file with forbidden fixes: $f (risk=$RISK)"
    fi
  done

  if [ ${#CRITICAL_FILES[@]} -eq 0 ]; then
    log_ok "File impact check passed"
  fi
fi

# ── Step 3: Scan violations ─────────────────────────────────────────────────
echo "" >&2
echo "[ 3/4 ] Scan violations..." >&2

if [ -n "$CHANGED_FILES" ]; then
  GO_FILES=$(echo "$CHANGED_FILES" | tr ' ' '\n' | grep '\.go$' | tr '\n' ' ' || true)
  if [ -n "$GO_FILES" ]; then
    SCAN_OUTPUT=$("$GLOBULAR" awareness scan-violations --output json 2>/dev/null) || {
      log_warn "scan-violations failed — skipping"
      SCAN_OUTPUT="{}"
    }

    CRITICAL_VIOLATIONS=$(echo "$SCAN_OUTPUT" | grep -c '"severity":"critical"' 2>/dev/null || echo "0")
    if [ "${CRITICAL_VIOLATIONS:-0}" -gt 0 ] 2>/dev/null && [ "$CRITICAL_VIOLATIONS" != "0" ]; then
      log_error "Critical scan violations detected ($CRITICAL_VIOLATIONS). Run 'globular awareness scan-violations' for details."
    else
      log_ok "No critical scan violations"
    fi
  else
    log_ok "No Go files changed — scan not required"
  fi
else
  log_ok "No files to scan"
fi

# ── Step 4: Graph integrity (fast) ─────────────────────────────────────────
echo "" >&2
echo "[ 4/4 ] Graph integrity..." >&2
INTEGRITY_JSON=$("$GLOBULAR" awareness graph-integrity --docs-dir "$DOCS_DIR" --repo-root "$REPO_ROOT" --json 2>/dev/null) || {
  log_warn "graph-integrity check failed — skipping"
  INTEGRITY_JSON="{}"
}

INTEGRITY_EXIT=$(echo "$INTEGRITY_JSON" | grep -oP '"exit_code":\K[0-9]+' | head -1 || echo "0")
if [ "$INTEGRITY_EXIT" -ge 2 ]; then
  log_error "Graph integrity critical violations. Run 'globular awareness graph-integrity' for details."
elif [ "$INTEGRITY_EXIT" -eq 1 ]; then
  log_warn "Graph integrity warnings (non-blocking)"
else
  log_ok "Graph integrity clean"
fi

# ── Summary ─────────────────────────────────────────────────────────────────
echo "" >&2
echo "RESULT:" >&2
if [ "$EXIT_CODE" -ne 0 ]; then
  echo "  ✗ FAILED — ${#ERRORS[@]} critical issue(s) detected" >&2
  for err in "${ERRORS[@]}"; do
    echo "      • $err" >&2
  done
  if $DRY_RUN; then
    echo "  (dry-run: would have blocked commit)" >&2
    exit 0
  fi
  exit 1
elif [ ${#WARNINGS[@]} -gt 0 ]; then
  echo "  ⚠ PASSED with ${#WARNINGS[@]} warning(s)" >&2
  exit 0
else
  echo "  ✓ PASSED — no issues detected" >&2
  exit 0
fi
