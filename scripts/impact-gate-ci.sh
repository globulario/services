#!/usr/bin/env bash
# impact-gate-ci.sh — CG-5 per-change enforcement.
#
# When a PR changes a Go file that an invariant protects (protects.files /
# implemented_by), that invariant's required_tests MUST run and pass. This
# script resolves the required tests for the changed files, runs exactly those,
# and fails closed if any did not pass.
#
# Advisory until armed: the CI step that calls this is continue-on-error while
# the corpus's required_tests are confirmed runnable in CI; remove that to make
# it a hard gate (the same arm-later pattern used for awg validate/audit).
#
# Environment:
#   AWG_DIR             awareness-graph checkout (default: ../awareness-graph)
#   IMPACT_GATE_BASE    base ref to diff against (default: origin/master)
set -euo pipefail

SVC="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
AWG_DIR="${AWG_DIR:-$SVC/../awareness-graph}"
BASE="${IMPACT_GATE_BASE:-origin/master}"

if [[ ! -d "$AWG_DIR/cmd/awg" ]]; then
    echo "impact-gate: awareness-graph checkout not found at $AWG_DIR; skipping" >&2
    exit 0
fi
run_awg() { ( cd "$AWG_DIR" && go run ./cmd/awg "$@" ); }

git -C "$SVC" fetch -q origin "${BASE#origin/}" 2>/dev/null || true
changed="$(git -C "$SVC" diff --name-only "$BASE"...HEAD -- 'golang/**' 2>/dev/null || true)"
if [[ -z "$changed" ]]; then
    echo "impact-gate: no Go changes vs $BASE — nothing to enforce"
    exit 0
fi
echo "impact-gate: changed Go files:"
echo "$changed" | sed 's/^/  /'

regex="$(run_awg impact-gate -services-repo "$SVC" -changed-files "$changed" -format run)"
if [[ -z "$regex" ]]; then
    echo "impact-gate: no changed file is protected by an invariant with runnable tests"
    exit 0
fi
echo "impact-gate: required tests for changed protected files: $regex"

results="$(mktemp)"
trap 'rm -f "$results"' EXIT
# Run exactly the required tests. Test failures do not abort here — the gate
# verdict is computed from the -json results by impact-gate -ran.
( cd "$SVC/golang" && go test -run "$regex" ./... -json ) > "$results" || true

run_awg impact-gate -services-repo "$SVC" -changed-files "$changed" -ran "$results"
