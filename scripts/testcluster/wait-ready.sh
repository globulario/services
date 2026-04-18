#!/usr/bin/env bash
# wait-ready.sh — poll the containerized cluster until all health gates pass.
#
# Gates (in order):
#   1. All five containers are running
#   2. etcd cluster is healthy (3-member quorum)
#   3. Cluster controller is reachable (port 12000)
#   4. Workflow service is reachable (port 10004)
#   5. Node agent has registered at least 3 nodes
#
# Usage: wait-ready.sh [timeout_seconds]   default: 300
set -euo pipefail

TIMEOUT="${1:-300}"
INTERVAL=5
ELAPSED=0

CONTROLLER_CONTAINER="globular-node-1"
ETCD_ENDPOINTS="https://10.10.0.11:2379,https://10.10.0.12:2379,https://10.10.0.13:2379"
ETCD_CACERT="/var/lib/globular/pki/ca.crt"
ETCD_CERT="/var/lib/globular/pki/issued/services/service.crt"
ETCD_KEY="/var/lib/globular/pki/issued/services/service.key"

log() { echo "[$(date '+%H:%M:%S')] $*"; }

wait_for() {
    local desc="$1"
    local check_fn="$2"
    local start=$ELAPSED
    while ! $check_fn 2>/dev/null; do
        if [[ $ELAPSED -ge $TIMEOUT ]]; then
            echo "FAIL: timeout waiting for: $desc (${TIMEOUT}s elapsed)"
            exit 1
        fi
        sleep $INTERVAL
        ELAPSED=$(( ELAPSED + INTERVAL ))
        local waited=$(( ELAPSED - start ))
        log "  waiting for $desc ... ${waited}s"
    done
    log "  OK: $desc"
}

# ── Gate 1: all containers running ──────────────────────────────────────────

check_containers() {
    local expected=("globular-node-1" "globular-node-2" "globular-node-3"
                    "globular-node-4" "globular-node-5" "globular-scylladb")
    for c in "${expected[@]}"; do
        local state
        state=$(docker inspect --format '{{.State.Status}}' "$c" 2>/dev/null) || return 1
        [[ "$state" == "running" ]] || return 1
    done
    return 0
}

# ── Gate 2: etcd cluster quorum ─────────────────────────────────────────────

check_etcd() {
    docker exec "$CONTROLLER_CONTAINER" \
        etcdctl \
        --endpoints="$ETCD_ENDPOINTS" \
        --cacert="$ETCD_CACERT" \
        --cert="$ETCD_CERT" \
        --key="$ETCD_KEY" \
        endpoint health --cluster \
        2>/dev/null | grep -qc "is healthy"
    # grep -qc returns 0 if at least 1 match; we need ≥ 2 of 3 members healthy
    local count
    count=$(docker exec "$CONTROLLER_CONTAINER" \
        etcdctl \
        --endpoints="$ETCD_ENDPOINTS" \
        --cacert="$ETCD_CACERT" \
        --cert="$ETCD_CERT" \
        --key="$ETCD_KEY" \
        endpoint health --cluster 2>/dev/null | grep -c "is healthy" || true)
    [[ "$count" -ge 2 ]]
}

# ── Gate 3: cluster controller responding ────────────────────────────────────

check_controller() {
    # controller registers itself in etcd when ready
    docker exec "$CONTROLLER_CONTAINER" \
        etcdctl \
        --endpoints="https://10.10.0.11:2379" \
        --cacert="$ETCD_CACERT" \
        --cert="$ETCD_CERT" \
        --key="$ETCD_KEY" \
        get /globular/clustercontroller/leader \
        --print-value-only 2>/dev/null | grep -q .
}

# ── Gate 4: workflow service ready ───────────────────────────────────────────

check_workflow() {
    # workflow writes a heartbeat / service registration when it connects
    docker exec "$CONTROLLER_CONTAINER" \
        etcdctl \
        --endpoints="https://10.10.0.11:2379" \
        --cacert="$ETCD_CACERT" \
        --cert="$ETCD_CERT" \
        --key="$ETCD_KEY" \
        get /globular/services/workflow/config \
        --print-value-only 2>/dev/null | grep -q port
}

# ── Gate 5: ≥3 nodes registered ─────────────────────────────────────────────

check_nodes() {
    local count
    count=$(docker exec "$CONTROLLER_CONTAINER" \
        etcdctl \
        --endpoints="https://10.10.0.11:2379" \
        --cacert="$ETCD_CACERT" \
        --cert="$ETCD_CERT" \
        --key="$ETCD_KEY" \
        get /globular/nodes/ --prefix --keys-only 2>/dev/null \
        | grep -c '/status$' || true)
    [[ "$count" -ge 3 ]]
}

# ── Main ──────────────────────────────────────────────────────────────────────

log "Waiting for cluster readiness (timeout: ${TIMEOUT}s)..."

wait_for "all containers running"        check_containers
wait_for "etcd quorum (≥2 of 3)"         check_etcd
wait_for "cluster controller elected"    check_controller
wait_for "workflow service registered"   check_workflow
wait_for "≥3 nodes registered"           check_nodes

log "Cluster is ready. Total wait: ${ELAPSED}s"
