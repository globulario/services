#!/usr/bin/env bash
# awareness-mcp-smoke.sh
#
# Smoke test for default awareness-first workflow.
# Uses CLI preflight equivalent for MCP when direct MCP JSON-RPC shell setup
# is impractical in CI shells.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TASK="fix annotation validator false positives in test fixtures"

# Use installed globular when it supports awareness; otherwise fallback to local CLI.
if globular awareness preflight --help >/dev/null 2>&1; then
  GLOBULAR_CMD=(globular)
else
  mkdir -p /tmp/gocache /tmp/gomodcache
  GLOBULAR_CMD=(env GOCACHE=/tmp/gocache GOMODCACHE=/tmp/gomodcache go run "$REPO_ROOT/golang/globularcli")
fi

TMP_JSON="$(mktemp)"
trap 'rm -f "$TMP_JSON"' EXIT

"${GLOBULAR_CMD[@]}" awareness preflight \
  --repo "$REPO_ROOT" \
  --task "$TASK" \
  --format json >"$TMP_JSON"

grep -q 'awareness.annotation_scanner.production_source_only' "$TMP_JSON" || {
  echo "FAIL: invariant awareness.annotation_scanner.production_source_only not found in preflight output"
  exit 1
}
grep -q 'validate_globular_annotations_inside_test_fixtures' "$TMP_JSON" || {
  echo "FAIL: forbidden fix validate_globular_annotations_inside_test_fixtures not found in preflight output"
  exit 1
}
grep -q 'TestValidateAnnotationsSkipsTestFiles' "$TMP_JSON" || {
  echo "FAIL: required test TestValidateAnnotationsSkipsTestFiles not found in preflight output"
  exit 1
}

echo "awareness-mcp-smoke: PASS"
