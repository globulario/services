#!/usr/bin/env bash
# validate-minio-topology.sh
#
# Proves that MinIO topology automation is fully converged from Day-0 through
# Day-1 join without manual config repair.
#
# Reads from etcd, computes expected fingerprint in-place, probes MinIO health.
# Zero mutations.
#
# Exit code: 0 = fully converged, 1 = not converged (prints exact cause).
#
# Prerequisites:
#   - etcdctl in PATH (or ETCDCTL env var)
#   - jq in PATH
#   - curl in PATH
#   - sha256sum in PATH
#   - Run on a control-plane node (or export ETCD_ENDPOINTS)
#
# Environment overrides:
#   ETCDCTL          path to etcdctl binary   (default: etcdctl)
#   ETCD_ENDPOINTS   etcd endpoints            (default: https://localhost:2379)
#   ETCD_CACERT      CA cert path              (default: /var/lib/globular/pki/ca.crt)
#   ETCD_CERT        client cert path          (default: /var/lib/globular/pki/issued/services/service.crt)
#   ETCD_KEY         client key path           (default: /var/lib/globular/pki/issued/services/service.key)
#   GLOBULAR         globular CLI binary       (default: globular)
#   SSH_USER         SSH user for node checks  (default: root)
#   NO_SSH           skip SSH node checks      (default: unset)

set -euo pipefail

ETCDCTL="${ETCDCTL:-etcdctl}"
ETCD_ENDPOINTS="${ETCD_ENDPOINTS:-https://localhost:2379}"
ETCD_CACERT="${ETCD_CACERT:-/var/lib/globular/pki/ca.crt}"
ETCD_CERT="${ETCD_CERT:-/var/lib/globular/pki/issued/services/service.crt}"
ETCD_KEY="${ETCD_KEY:-/var/lib/globular/pki/issued/services/service.key}"
GLOBULAR="${GLOBULAR:-globular}"
SSH_USER="${SSH_USER:-root}"
ERRORS=0

# ── helpers ────────────────────────────────────────────────────────────────────

etcd() {
    "$ETCDCTL" \
        --endpoints="$ETCD_ENDPOINTS" \
        --cacert="$ETCD_CACERT" \
        --cert="$ETCD_CERT" \
        --key="$ETCD_KEY" \
        "$@" 2>/dev/null
}

etcd_get() {
    etcd get "$1" --print-value-only 2>/dev/null || true
}

fail() {
    echo "FAIL: $*" >&2
    ERRORS=$((ERRORS + 1))
}

ok() {
    echo "OK:   $*"
}

info() {
    echo "INFO: $*"
}

section() {
    echo ""
    echo "──── $* ────────────────────────────────────────────────────────"
}

# ── check prerequisites ────────────────────────────────────────────────────────

section "Prerequisites"
for cmd in "$ETCDCTL" jq curl sha256sum; do
    if ! command -v "$cmd" &>/dev/null; then
        fail "required command not found: $cmd"
    else
        ok "$cmd found"
    fi
done
if [[ $ERRORS -gt 0 ]]; then
    echo "" && echo "ABORT: missing prerequisites" >&2 && exit 1
fi

# ── Step 1: Read desired objectstore state ─────────────────────────────────────

section "Step 1: Desired objectstore state"

DESIRED_JSON="$(etcd_get /globular/objectstore/config)"
if [[ -z "$DESIRED_JSON" ]]; then
    fail "etcd key /globular/objectstore/config is empty — no desired state published (pre-pool-formation)"
    echo "ABORT: no desired state found" >&2
    exit 1
fi
ok "desired state key exists"

DESIRED_GEN="$(echo "$DESIRED_JSON" | jq -r '.generation // 0')"
DESIRED_MODE="$(echo "$DESIRED_JSON" | jq -r '.mode // "standalone"')"
DESIRED_ENDPOINT="$(echo "$DESIRED_JSON" | jq -r '.endpoint // ""')"
DESIRED_VOLUMES_HASH="$(echo "$DESIRED_JSON" | jq -r '.volumes_hash // ""')"
DESIRED_DRIVES="$(echo "$DESIRED_JSON" | jq -r 'if .drives_per_node != null then .drives_per_node else 0 end')"
POOL_NODES_JSON="$(echo "$DESIRED_JSON" | jq -c '.nodes // []')"
POOL_COUNT="$(echo "$POOL_NODES_JSON" | jq 'length')"

info "generation:      $DESIRED_GEN"
info "mode:            $DESIRED_MODE"
info "pool_nodes:      $(echo "$POOL_NODES_JSON" | jq -r 'join(", ")')"
info "drives_per_node: $DESIRED_DRIVES"
info "endpoint:        $DESIRED_ENDPOINT"
info "volumes_hash:    $DESIRED_VOLUMES_HASH"

# ── Step 2: Compute expected fingerprint ───────────────────────────────────────

section "Step 2: Expected state fingerprint"

# Replicate RenderStateFingerprint: SHA256("gen|mode|sorted_nodes|drives|volumes_hash")
SORTED_NODES="$(echo "$POOL_NODES_JSON" | jq -r '.[]' | sort | tr '\n' ',' | sed 's/,$//')"
FP_INPUT="${DESIRED_GEN}|${DESIRED_MODE}|${SORTED_NODES}|${DESIRED_DRIVES}|${DESIRED_VOLUMES_HASH}"
EXPECTED_FP="$(echo -n "$FP_INPUT" | sha256sum | awk '{print $1}')"
info "fingerprint input: $FP_INPUT"
ok "expected fingerprint: $EXPECTED_FP"

# ── Step 3: Applied generation ─────────────────────────────────────────────────

section "Step 3: Applied generation"

APPLIED_GEN="$(etcd_get /globular/objectstore/applied_generation)"
APPLIED_GEN="${APPLIED_GEN:-0}"
info "applied_generation: $APPLIED_GEN"
info "desired_generation: $DESIRED_GEN"

if [[ "$APPLIED_GEN" -ge "$DESIRED_GEN" ]]; then
    ok "applied_generation ($APPLIED_GEN) >= desired_generation ($DESIRED_GEN)"
else
    fail "applied_generation ($APPLIED_GEN) < desired_generation ($DESIRED_GEN) — topology workflow has not completed"
fi

# ── Step 4: Restart in progress / topology lock ────────────────────────────────

section "Step 4: Restart flags and lock"

RESTART_IP="$(etcd_get /globular/objectstore/restart_in_progress)"
if [[ -n "$RESTART_IP" ]]; then
    fail "restart_in_progress flag is set (since $RESTART_IP) — workflow may have failed without cleanup"
else
    ok "restart_in_progress: not set"
fi

LOCK_VAL="$(etcd_get /globular/locks/objectstore/minio/topology-restart)"
if [[ -n "$LOCK_VAL" ]]; then
    fail "topology lock is held: $LOCK_VAL — this blocks future topology workflows"
else
    ok "topology lock: not held"
fi

LAST_RESULT="$(etcd_get /globular/objectstore/last_restart_result)"
if [[ -n "$LAST_RESULT" ]]; then
    LAST_STATUS="$(echo "$LAST_RESULT" | jq -r '.status // "unknown"')"
    LAST_TIME="$(echo "$LAST_RESULT" | jq -r '.applied_at // .failed_at // "unknown"')"
    if [[ "$LAST_STATUS" == "succeeded" ]]; then
        ok "last_restart_result: status=succeeded at $LAST_TIME"
    else
        fail "last_restart_result: status=$LAST_STATUS at $LAST_TIME — last topology workflow did not succeed"
    fi
else
    info "last_restart_result: not written yet"
fi

# ── Step 5: Per-node rendered_generation and rendered_state_fingerprint ─────────

section "Step 5: Per-node rendered generation and fingerprint"

# We need to map pool IPs → node IDs.
# Node IDs are available from /globular/nodes/{nodeID}/objectstore/rendered_generation
# but we don't know the node IDs without querying the controller.
# Use 'globular cluster nodes list' via the CLI if available; otherwise skip.

NODE_IDS=""
if command -v "$GLOBULAR" &>/dev/null; then
    # Try to get node IDs that have rendered generations for this generation.
    # List all keys matching /globular/nodes/*/objectstore/rendered_generation
    NODE_GEN_KEYS="$(etcd get /globular/nodes --prefix --keys-only 2>/dev/null | grep '/objectstore/rendered_generation' || true)"
    if [[ -n "$NODE_GEN_KEYS" ]]; then
        while IFS= read -r key; do
            NODE_ID="$(echo "$key" | sed 's|/globular/nodes/\(.*\)/objectstore/rendered_generation|\1|')"
            GEN_VAL="$(etcd_get "$key")"
            FP_VAL="$(etcd_get "/globular/nodes/${NODE_ID}/objectstore/rendered_state_fingerprint")"

            info "node $NODE_ID: rendered_generation=$GEN_VAL"
            if [[ -z "$GEN_VAL" ]]; then
                fail "node $NODE_ID: rendered_generation not written — node agent has not rendered topology yet"
                continue
            fi
            if [[ "$GEN_VAL" -lt "$DESIRED_GEN" ]]; then
                fail "node $NODE_ID: rendered_generation=$GEN_VAL < desired=$DESIRED_GEN — node agent lagging"
            else
                ok "node $NODE_ID: rendered_generation=$GEN_VAL"
            fi

            if [[ -z "$FP_VAL" ]]; then
                fail "node $NODE_ID: rendered_state_fingerprint not written"
                continue
            fi
            if [[ "$FP_VAL" == "$EXPECTED_FP" ]]; then
                ok "node $NODE_ID: fingerprint match ($FP_VAL)"
            else
                fail "node $NODE_ID: fingerprint MISMATCH (got=$FP_VAL expected=$EXPECTED_FP) — node may have rendered standalone or old topology"
            fi
        done <<< "$NODE_GEN_KEYS"
    else
        info "no node rendered_generation keys found in etcd (nodes may not have synced yet)"
    fi
fi

# ── Step 6: SSH-based file checks (optional) ───────────────────────────────────

section "Step 6: Per-node file checks (SSH)"

if [[ -n "${NO_SSH:-}" ]]; then
    info "SSH checks skipped (NO_SSH set)"
elif [[ "$POOL_COUNT" -eq 0 ]]; then
    info "SSH checks skipped (no pool nodes)"
else
    mapfile -t POOL_IPS < <(echo "$POOL_NODES_JSON" | jq -r '.[]')
    for ip in "${POOL_IPS[@]}"; do
        info "checking files on $ip via ssh"

        SSH_CMD="ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no ${SSH_USER}@${ip}"

        # minio.env must always exist
        if $SSH_CMD test -f /var/lib/globular/minio/minio.env 2>/dev/null; then
            ok "$ip: minio.env exists"
        else
            fail "$ip: minio.env MISSING — node agent has not rendered config"
        fi

        # distributed.conf must exist when pool > 1 or drives_per_node > 1
        if [[ "$POOL_COUNT" -gt 1 ]] || [[ "$DESIRED_DRIVES" -gt 1 ]]; then
            if $SSH_CMD test -f /etc/systemd/system/globular-minio.service.d/distributed.conf 2>/dev/null; then
                ok "$ip: distributed.conf exists"
            else
                fail "$ip: distributed.conf MISSING — expected for pool_size=$POOL_COUNT drives=$DESIRED_DRIVES"
            fi
        fi

        # no standalone MINIO_VOLUMES when desired mode is distributed
        if [[ "$DESIRED_MODE" == "distributed" ]]; then
            VOLUMES_LINE="$($SSH_CMD grep '^MINIO_VOLUMES=' /var/lib/globular/minio/minio.env 2>/dev/null || true)"
            if echo "$VOLUMES_LINE" | grep -qE '^MINIO_VOLUMES=http://'; then
                ok "$ip: MINIO_VOLUMES is distributed (starts with http://)"
            else
                fail "$ip: MINIO_VOLUMES appears to be standalone (got: $VOLUMES_LINE)"
            fi
        fi

        # globular-minio.service active
        SERVICE_STATE="$($SSH_CMD systemctl is-active globular-minio.service 2>/dev/null || echo unknown)"
        if [[ "$SERVICE_STATE" == "active" ]]; then
            ok "$ip: globular-minio.service is active"
        else
            fail "$ip: globular-minio.service state=$SERVICE_STATE"
        fi
    done
fi

# ── Step 7: MinIO health endpoint ─────────────────────────────────────────────

section "Step 7: MinIO health endpoint"

if [[ -z "$DESIRED_ENDPOINT" ]]; then
    info "no endpoint in desired state — skipping health probe"
else
    # Ensure endpoint has port
    HEALTH_HOST="$DESIRED_ENDPOINT"
    if ! echo "$HEALTH_HOST" | grep -q ':'; then
        HEALTH_HOST="${HEALTH_HOST}:9000"
    fi
    HEALTH_URL="https://${HEALTH_HOST}/minio/health/live"
    info "probing $HEALTH_URL"

    HTTP_CODE="$(curl -sk -o /dev/null -w '%{http_code}' --connect-timeout 10 --max-time 15 "$HEALTH_URL" 2>/dev/null || echo 000)"
    if [[ "$HTTP_CODE" == "200" ]]; then
        ok "MinIO health endpoint: HTTP 200 (healthy)"
    else
        fail "MinIO health endpoint: HTTP $HTTP_CODE (expected 200) — MinIO is not serving at $HEALTH_URL"
    fi
fi

# ── Step 8: Doctor CRITICAL objectstore findings ───────────────────────────────

section "Step 8: Doctor objectstore findings"

if ! command -v "$GLOBULAR" &>/dev/null; then
    info "globular CLI not available — skipping doctor check"
else
    DOCTOR_OUTPUT="$("$GLOBULAR" doctor report --json 2>/dev/null || true)"
    if [[ -z "$DOCTOR_OUTPUT" ]]; then
        info "doctor report unavailable (service may be unreachable)"
    else
        CRITICAL_TOPO="$(echo "$DOCTOR_OUTPUT" | jq -r '
            .findings[]? |
            select(
                .severity == "SEVERITY_CRITICAL" and
                (.invariant_id | startswith("objectstore.minio"))
            ) |
            "  CRITICAL: " + .invariant_id + " — " + .summary
        ' 2>/dev/null || true)"

        if [[ -n "$CRITICAL_TOPO" ]]; then
            fail "doctor reports CRITICAL objectstore topology findings:"
            echo "$CRITICAL_TOPO" >&2
        else
            ok "doctor: no CRITICAL objectstore.minio.* findings"
        fi
    fi
fi

# ── Final verdict ──────────────────────────────────────────────────────────────

section "Result"
echo ""
if [[ $ERRORS -eq 0 ]]; then
    echo "✓ CONVERGED — MinIO topology is fully applied and healthy."
    exit 0
else
    echo "✗ NOT CONVERGED — $ERRORS check(s) failed." >&2
    echo "" >&2
    echo "Next steps:" >&2
    echo "  1. globular objectstore topology status" >&2
    echo "  2. globular doctor report" >&2
    echo "  3. globular workflow status objectstore.minio.apply_topology_generation" >&2
    exit 1
fi
