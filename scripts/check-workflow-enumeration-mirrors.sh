#!/usr/bin/env bash
# check-workflow-enumeration-mirrors.sh — enforce
# meta.code_must_not_mirror_external_enumerations for the one set where that
# principle has already cost us an outage: workflow definitions.
#
# WHY THIS EXISTS
#
# golang/workflow/definitions/ owns the set of workflow names. Day-0 seeds
# every definition it finds there into etcd. Any Go code that wants "the core
# workflows" must therefore ask the authority — list the etcd prefix, or read
# the directory — never carry its own slice of names.
#
# On 2026-08-18 the node-agent's workflow cache carried exactly such a slice,
# of eight names, each written with a ".yaml" suffix the etcd keys do not have.
# Every lookup missed, so the cache never wrote a file, so a freshly wiped node
# could not resolve node.join, so its workloads installed without receipts and
# cluster-doctor fail-closed at CRITICAL. Five upgrade scenarios failed.
#
# The meta principle had already described this shape, and its own summary
# names the symptom this produced — "workflow not found" — as its worked
# example. It predicted the bug and prevented nothing, because it was written
# down and never made decidable. This script is that principle made decidable
# for one authority.
#
# WHAT IT REFUSES
#
# A Go slice/array literal containing two or more names from the workflow
# definitions directory. Two is already an enumeration; one is a reference.
#
# Deliberately NOT flagged, because they are references rather than mirrors:
#   - a single workflow name (a default, a dispatch target, a log line);
#   - switch cases, which name synthetic workflows that have no YAML at all;
#   - test files, which must be able to construct fixture sets by hand.
#
# A scanner that cannot fire reads exactly like clean code
# (awareness: scanner_zero_findings_conflates_clean_with_dead), so this one is
# proven against the historical defect by check-workflow-enumeration-mirrors's
# own self-test: run with --selftest to confirm it still detects the 2026-08-18
# shape.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEFINITIONS_DIR="$REPO_ROOT/golang/workflow/definitions"
SELFTEST=0
[[ "${1:-}" == "--selftest" ]] && SELFTEST=1

if [[ ! -d "$DEFINITIONS_DIR" ]]; then
	echo "check-workflow-enumeration-mirrors: definitions directory not found: $DEFINITIONS_DIR" >&2
	echo "  The authority moved. Point this check at it rather than deleting the check." >&2
	exit 1
fi

scan() {
	local scan_root="$1"
	python3 - "$DEFINITIONS_DIR" "$scan_root" <<'PY'
import os, re, sys

definitions_dir, scan_root = sys.argv[1], sys.argv[2]

names = {f[:-len(".yaml")] for f in os.listdir(definitions_dir) if f.endswith(".yaml")}
if not names:
    print("check-workflow-enumeration-mirrors: the definitions directory holds no workflows —")
    print("  the authority is empty, so this check cannot fire. Refusing to report clean.")
    sys.exit(1)

# A literal is any []string{...} / []T{...} block. Names may appear bare or
# with the .yaml extension; both forms are mirrors of the same authority.
literal_start = re.compile(r"\[\][A-Za-z0-9_\.\*\[\]]*\{")
quoted = re.compile(r'"([^"]+)"')

violations = []
for dirpath, dirnames, filenames in os.walk(scan_root):
    dirnames[:] = [d for d in dirnames if d not in ("vendor", "node_modules", ".git", "dist")]
    for filename in filenames:
        if not filename.endswith(".go") or filename.endswith("_test.go"):
            continue
        path = os.path.join(dirpath, filename)
        try:
            source = open(path, encoding="utf-8", errors="replace").read()
        except OSError:
            continue

        for match in literal_start.finditer(source):
            depth, index = 0, match.end() - 1
            while index < len(source):
                if source[index] == "{":
                    depth += 1
                elif source[index] == "}":
                    depth -= 1
                    if depth == 0:
                        break
                index += 1
            block = source[match.end():index]
            found = set()
            for literal in quoted.findall(block):
                bare = literal[:-len(".yaml")] if literal.endswith(".yaml") else literal
                if bare in names:
                    found.add(bare)
            if len(found) >= 2:
                line = source[: match.start()].count("\n") + 1
                violations.append((os.path.relpath(path, scan_root), line, sorted(found)))

for path, line, found in violations:
    print(f"  {path}:{line} enumerates {len(found)} workflow names: {', '.join(found)}")
sys.exit(2 if violations else 0)
PY
}

if (( SELFTEST )); then
	# Reconstruct the 2026-08-18 defect and require the scanner to catch it. A
	# gate nobody has watched fail is a decoration.
	fixture_dir="$(mktemp -d)"
	trap 'rm -rf "$fixture_dir"' EXIT
	cat > "$fixture_dir/historical_defect.go" <<'GO'
package main

func fetchWorkflowDefsFromEtcd() {
	knownDefs := []string{
		"day0.bootstrap.yaml",
		"node.bootstrap.yaml",
		"node.join.yaml",
		"cluster.reconcile.yaml",
	}
	_ = knownDefs
}
GO
	echo "== self-test: the 2026-08-18 shape must be detected =="
	if scan "$fixture_dir"; then
		echo "  ✗ the scanner did NOT detect the historical defect — it cannot fire, so a"
		echo "    clean result from it means nothing. Fix the scanner." >&2
		exit 1
	fi
	echo "  ✓ detected"
	exit 0
fi

echo "== workflow enumeration mirrors =="
if scan "$REPO_ROOT/golang"; then
	echo "  ✓ no Go source mirrors the workflow definitions directory"
	exit 0
fi

cat >&2 <<'MSG'

  ✗ A Go slice above enumerates workflow names that golang/workflow/definitions/
    already owns. That slice is a second authority for the set: day0 seeds every
    definition it finds, so the moment a workflow is added the mirror diverges,
    silently, with no compile error and no failing test.

    Derive the set instead — list the etcd prefix (v1alpha1.EtcdWorkflowLister)
    or read the definitions directory.

    See failure_mode
    failure.workflow_cache_asked_etcd_for_filenames_so_a_wiped_node_coul
    and meta.code_must_not_mirror_external_enumerations.
MSG
exit 1
