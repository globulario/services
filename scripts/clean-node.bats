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

# ── Requirement 1 (unit): probe targets the node IP, UNKNOWN on unreachable — BOTH copies ─
# The ScyllaDB admin REST API binds to api_address = the node's cluster IP (never
# localhost — this is a cluster). nodetool defaults to 127.0.0.1:10000, which nothing
# serves, so the probe MUST target the resolved node address via `-h`. These tests
# extract ONLY the topology-count logic (no main flow, no destructive commands).
@test "gateway copy: count_scylla_up_nodes probes the node IP via -h, UNKNOWN on unreachable, counts otherwise" {
  local gw="$BATS_TEST_DIRNAME/../../Globular/internal/gateway/handlers/cluster/clean-node.sh"
  # pull in the helper log_warn (no-op) + the function body, then call it with a node IP
  run bash -c '
    log_warn() { :; }
    '"$(sed -n "/^count_scylla_up_nodes()/,/^}/p" "$gw")"'
    # nodetool answers ONLY when targeted at the node IP via -h; the localhost default fails.
    nodetool() { [[ "$1" == "-h" && "$2" == "10.0.0.63" ]] && printf "UN a\nUN b\n"; return 0; }
    [[ "$(count_scylla_up_nodes 10.0.0.63)" == "2" ]] || { echo "node-IP probe not counted"; exit 1; }
    # A target the API is not served on (e.g. localhost) -> unreachable -> UNKNOWN, never 0.
    [[ "$(count_scylla_up_nodes 127.0.0.1)" == "UNKNOWN" ]] || { echo "unreachable!=UNKNOWN"; exit 1; }
    nodetool() { [[ "$1" == "-h" && "$2" == "10.0.0.63" ]] && printf "UN a\n"; return 0; }
    [[ "$(count_scylla_up_nodes 10.0.0.63)" == "1" ]] || { echo "1-node count wrong"; exit 1; }
    echo OK'
  [ "$status" -eq 0 ]
  [[ "$output" == *"OK"* ]]
}

@test "services copy: inline probe targets the node IP via -h, UNKNOWN on unreachable, never 0" {
  local svc="$BATS_TEST_DIRNAME/clean-node.sh"
  # The services copy computes _SCYLLA_UP inline. Assert it probes via `nodetool -h "$_NODE_IP"`
  # (the resolved node address) and replicate the probe->classify snippet.
  grep -q 'nodetool -h "\$_NODE_IP" status' "$svc"
  grep -q '_SCYLLA_UP="UNKNOWN"' "$svc"
  run bash -c '
    _NODE_IP=10.0.0.63
    # nodetool answers only at the node IP; the localhost default would fail.
    nodetool() { [[ "$1" == "-h" && "$2" == "10.0.0.63" ]] && printf "UN a\nUN b\n"; return 0; }
    _NT_OUT="$(nodetool -h "$_NODE_IP" status 2>/dev/null || true)"
    if [[ -z "$_NT_OUT" ]]; then _SCYLLA_UP="UNKNOWN"; else _SCYLLA_UP=$(printf "%s\n" "$_NT_OUT" | awk "/^U[NL] / {n++} END {print n+0}"); fi
    [[ "$_SCYLLA_UP" == "2" ]] || { echo "node-IP probe not counted"; exit 1; }
    # Unreachable target -> UNKNOWN, never 0.
    nodetool() { return 1; }
    _NT_OUT="$(nodetool -h "$_NODE_IP" status 2>/dev/null || true)"
    if [[ -z "$_NT_OUT" ]]; then _SCYLLA_UP="UNKNOWN"; else _SCYLLA_UP=$(printf "%s\n" "$_NT_OUT" | awk "/^U[NL] / {n++} END {print n+0}"); fi
    [[ "$_SCYLLA_UP" == "UNKNOWN" ]] && echo OK'
  [ "$status" -eq 0 ]
  [[ "$output" == *"OK"* ]]
}

# ── Guard: neither copy probes scylla on localhost, nor collapses a failed probe to 0 ─
@test "both copies probe via nodetool -h (node IP); no bare localhost probe, no fail-open to 0" {
  local gw="$BATS_TEST_DIRNAME/../../Globular/internal/gateway/handlers/cluster/clean-node.sh"
  local svc="$BATS_TEST_DIRNAME/clean-node.sh"
  # Every real probe must go through -h (the resolved node address).
  grep -q 'nodetool -h' "$gw"
  grep -q 'nodetool -h' "$svc"
  # No command-position bare `nodetool status|decommission` (would hit the 127.0.0.1 default).
  run grep -nE '(\$\(|if |;[[:space:]]*)nodetool (status|decommission)' "$gw" "$svc"
  [ "$status" -ne 0 ]
  # No fail-open: a failed probe must never be collapsed to 0.
  run grep -nE 'nodetool[^|]*status[^|]*\|\|[[:space:]]*(echo|printf)[[:space:]]+.?0' "$gw" "$svc"
  [ "$status" -ne 0 ]
}
