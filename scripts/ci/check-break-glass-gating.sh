#!/usr/bin/env bash
# check-break-glass-gating.sh — RT-4 scripts coverage gate.
#
# The principle-check scanner is Go-source-based and cannot see bash. This is the
# lightweight complement: every script that mutates owner-owned cluster state
# BEHIND the controller — deleting/putting etcd keys under
# /globular/{resources,plans,nodes}, or rewriting the controller's
# clustercontroller/state.json — MUST be gated by scripts/lib/break-glass.sh
# (source it and call break_glass_guard before any mutation). A new ungated
# owner-mutating recovery script fails CI.
#
# Out of scope (correctly NOT flagged): build scripts (release-index json.dump),
# node-local clean (nodeagent/state.json, run on the node being cleaned, not
# behind the controller), and read-only etcdctl get.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."  # → scripts/

# Owner-owned etcd mutation: a del/put of a controller-owned prefix. Matches both
# the literal `etcdctl ... del /globular/...` and the `e del "/globular/..."`
# wrapper form; leading slash and quotes optional.
OWNER_ETCD='(del|put)[[:space:]]+["'\''=]*/?globular/(resources|plans|nodes)'

# Controller state.json rewrite (NOT node-agent's nodeagent/state.json).
CONTROLLER_STATE='clustercontroller/state\.json'

fail=0
flagged=0
for f in *.sh; do
	mutates=0
	if grep -Eq "$OWNER_ETCD" "$f"; then
		mutates=1
	fi
	if grep -q "$CONTROLLER_STATE" "$f" && grep -Eq 'json\.dump|>[[:space:]]*"?[^"]*state\.json' "$f"; then
		mutates=1
	fi
	[ "$mutates" -eq 1 ] || continue

	flagged=$((flagged + 1))
	if grep -q 'break_glass_guard' "$f"; then
		echo "  ok (gated):   $f"
	else
		echo "  ✗ UNGATED:    $f — mutates owner-owned state behind the controller without break_glass_guard"
		echo "                source \"\$(dirname \"\${BASH_SOURCE[0]}\")/lib/break-glass.sh\" and call break_glass_guard before any mutation"
		fail=1
	fi
done

echo "break-glass gating: ${flagged} owner-state-mutating script(s) checked"
if [ "$fail" -eq 0 ]; then
	echo "all owner-state-mutating scripts are gated ✓"
fi
exit "$fail"
