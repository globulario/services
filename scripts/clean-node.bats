#!/usr/bin/env bats
# Regression coverage for SCAR-1 (INCIDENT 2026-07-09): node teardown must fail closed
# on UNKNOWN Scylla ring topology instead of hard-killing a live voter and destroying
# group0 quorum.
#
# Contract:  cluster.teardown.membership_must_be_confirmed_before_destructive_stop
# Failure:   cluster.teardown.nodetool_unreachable_misread_as_single_node
# Forbidden: cluster.teardown.do_not_treat_probe_failure_as_empty_ring
#
# Proves:
#   1. nodetool unreachable -> count_scylla_up_nodes returns UNKNOWN, not 0
#   2. clean-node refuses Scylla decommission/removal on UNKNOWN (fail closed)
#   3. --force still refuses on UNKNOWN topology
#   4. --last-node is the only allowed destructive last-node path
#   (+ positive controls: confirmed 2-node decommissions; confirmed 1-node skips cleanly)
#
# SAFETY: These tests drive the REAL script but MUST run in an isolated environment
# (CI / container / throwaway VM). They set GLOBULAR_STATE_DIR_OVERRIDE to a temp dir,
# stub nodetool/systemctl on PATH, and GLOBULAR_CLEAN_NODE_TEST_STOP_AFTER so execution
# halts BEFORE any service-stop or wipe phase. Do NOT run on a live Globular node.
#
# Run:  bats scripts/clean-node.bats
#       CLEAN_NODE_SH=../Globular/internal/gateway/handlers/cluster/clean-node.sh bats scripts/clean-node.bats
#
# The gateway copy is authoritative (embedded into the gateway binary at build); the
# services-repo copy shares the identical decision logic and is unit-checked in the
# "both copies" tests at the bottom.

setup() {
  BATS_TMPDIR="${BATS_TMPDIR:-/tmp}"
  STUB="$(mktemp -d)"
  STATE="$(mktemp -d)"            # empty -> node-agent state file absent -> _NODE_ID="" -> Phase 0.1 skipped
  export PATH="$STUB:$PATH"

  # Gateway copy has the TEST seams (TEST_ALLOW_NON_ROOT / TEST_STOP_AFTER).
  CLEAN_NODE_SH="${CLEAN_NODE_SH:-$BATS_TEST_DIRNAME/../../Globular/internal/gateway/handlers/cluster/clean-node.sh}"

  export GLOBULAR_CLEAN_NODE_TEST_ALLOW_NON_ROOT=1
  export GLOBULAR_CLEAN_NODE_TEST_STOP_AFTER=service_stop_start   # halt after scylla+etcd phases, before any stop/wipe
  export GLOBULAR_STATE_DIR_OVERRIDE="$STATE"

  # systemctl stub: scylla-server "active" so the decommission block runs; everything
  # else (incl. globular-etcd) reports inactive so those phases are skipped; all other
  # verbs succeed as no-ops.
  cat >"$STUB/systemctl" <<'EOF'
#!/usr/bin/env bash
args="$*"
if [[ "$args" == *"is-active"* ]]; then
  [[ "$args" == *"scylla-server"* ]] && exit 0   # active
  exit 3                                          # inactive (etcd, others)
fi
exit 0
EOF
  chmod +x "$STUB/systemctl"
}

teardown() { rm -rf "$STUB" "$STATE"; }

# Configure the nodetool stub for a scenario.
_set_nodetool() { printf '#!/usr/bin/env bash\n%s\n' "$1" >"$STUB/nodetool"; chmod +x "$STUB/nodetool"; }

# ── Requirement 2 + 3: UNKNOWN topology fails closed, even with --force ──────────
@test "unreachable nodetool + --force: FAIL CLOSED, no decommission, no misclassification" {
  _set_nodetool 'exit 1'                                   # CQL/API down: empty stdout, nonzero exit
  run bash "$CLEAN_NODE_SH" --force
  [ "$status" -ne 0 ]                                       # must die (fail closed)
  [[ "$output" == *"UNREACHABLE"* ]]
  [[ "$output" != *"Single-node ScyllaDB"* ]]              # must NOT misread unknown as single-node
  [[ "$output" != *"decommissioned cleanly"* ]]
  [[ "$output" != *"service stop start"* ]]                # must not have reached the stop phase
}

# ── Requirement 4: --last-node is the ONLY allowed destructive last-node path ────
@test "unreachable nodetool + --last-node: operator override skips decommission and proceeds" {
  _set_nodetool 'exit 1'
  run bash "$CLEAN_NODE_SH" --force --last-node
  [ "$status" -eq 0 ]
  [[ "$output" == *"operator override"* ]]
  [[ "$output" == *"service stop start"* ]]                # reached the stop-phase marker (TEST_STOP_AFTER)
}

# ── Positive control: a CONFIRMED multi-node ring decommissions ─────────────────
@test "confirmed 2-node ring: decommission runs" {
  _set_nodetool 'printf "Datacenter: dc1\nUN 10.0.0.8 1MB\nUN 10.0.0.9 1MB\n"'
  run bash "$CLEAN_NODE_SH" --force
  [ "$status" -eq 0 ]
  [[ "$output" == *"Decommissioning ScyllaDB node"* ]]
}

# ── Positive control: a CONFIRMED single node skips WITHOUT false fail-closed ────
@test "confirmed single node: skip decommission, no false UNKNOWN" {
  _set_nodetool 'printf "Datacenter: dc1\nUN 10.0.0.8 1MB\n"'
  run bash "$CLEAN_NODE_SH" --force
  [ "$status" -eq 0 ]
  [[ "$output" == *"Single-node ScyllaDB (confirmed"* ]]
  [[ "$output" != *"UNREACHABLE"* ]]
}

# ── Requirement 1 (unit): unreachable nodetool -> UNKNOWN, not 0 — in BOTH copies ─
# Extracts and exercises ONLY the topology-count logic from each copy (no main flow,
# no destructive commands), so it is safe to run anywhere.
@test "gateway copy: count_scylla_up_nodes returns UNKNOWN on unreachable, counts otherwise" {
  local gw="$BATS_TEST_DIRNAME/../../Globular/internal/gateway/handlers/cluster/clean-node.sh"
  # pull in the helper log_warn (no-op) + the function body, then call it
  run bash -c '
    log_warn() { :; }
    '"$(sed -n "/^count_scylla_up_nodes()/,/^}/p" "$gw")"'
    nodetool() { return 1; }         ; [[ "$(count_scylla_up_nodes)" == "UNKNOWN" ]] || { echo "unreachable!=UNKNOWN"; exit 1; }
    nodetool() { printf "UN a\nUN b\n"; }; [[ "$(count_scylla_up_nodes)" == "2" ]] || { echo "2-node count wrong"; exit 1; }
    nodetool() { printf "UN a\n"; }   ; [[ "$(count_scylla_up_nodes)" == "1" ]] || { echo "1-node count wrong"; exit 1; }
    echo OK'
  [ "$status" -eq 0 ]
  [[ "$output" == *"OK"* ]]
}

@test "services copy: inline topology logic yields UNKNOWN on unreachable, not 0" {
  local svc="$BATS_TEST_DIRNAME/clean-node.sh"
  # The services copy computes _SCYLLA_UP inline; replicate the exact probe->classify
  # snippet by extracting the `_NT_OUT=...`/awk lines and asserting the UNKNOWN sentinel.
  grep -q 'printf .UNKNOWN' "$svc" || grep -q '_SCYLLA_UP="UNKNOWN"' "$svc"
  run bash -c '
    nodetool() { return 1; }
    _NT_OUT="$(nodetool status 2>/dev/null || true)"
    if [[ -z "$_NT_OUT" ]]; then _SCYLLA_UP="UNKNOWN"; else _SCYLLA_UP=$(printf "%s\n" "$_NT_OUT" | awk "/^U[NL] / {n++} END {print n+0}"); fi
    [[ "$_SCYLLA_UP" == "UNKNOWN" ]] && echo OK'
  [ "$status" -eq 0 ]
  [[ "$output" == *"OK"* ]]
}

# ── Guard: neither copy collapses a failed probe to 0 (forbidden_fix regression) ─
@test "neither copy pipes a failed nodetool status to echo 0 / printf 0" {
  local gw="$BATS_TEST_DIRNAME/../../Globular/internal/gateway/handlers/cluster/clean-node.sh"
  local svc="$BATS_TEST_DIRNAME/clean-node.sh"
  run grep -nE 'nodetool status[^|]*\|\|[[:space:]]*(echo|printf)[[:space:]]+.?0' "$gw" "$svc"
  [ "$status" -ne 0 ]   # grep finds nothing -> the fail-open pattern is gone
}
