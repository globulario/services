#!/usr/bin/env bash
# run-tests.sh — bring up the containerized integration cluster, run tests, tear down.
#
# Usage:
#   scripts/testcluster/run-tests.sh              # all integration test suites
#   scripts/testcluster/run-tests.sh smoke        # smoke tests only
#   scripts/testcluster/run-tests.sh reconcile    # reconciliation scenario tests
#   scripts/testcluster/run-tests.sh migration    # Scylla migration tests
#   scripts/testcluster/run-tests.sh release      # package rollout tests
#
# Environment:
#   QUICKSTART_DIR   path to globular-quickstart repo (default: ../globular-quickstart
#                    relative to this services repo)
#   SKIP_TEARDOWN=1  leave cluster running after tests (for debugging)
#   SKIP_BUILD=1     skip docker image rebuild (use existing image)
#   CLUSTER_TIMEOUT  seconds to wait for cluster readiness (default: 300)
#
# The cluster is ALWAYS torn down on exit (success or failure) unless
# SKIP_TEARDOWN=1 is set. This prevents the real production cluster from
# being touched — the test target is the containerized cluster only.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICES_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
QUICKSTART_DIR="${QUICKSTART_DIR:-$(cd "$SERVICES_DIR/../globular-quickstart" 2>/dev/null && pwd || true)}"
SUITE="${1:-all}"
CLUSTER_TIMEOUT="${CLUSTER_TIMEOUT:-300}"

log()  { echo "[$(date '+%H:%M:%S')] $*"; }
fail() { echo "[$(date '+%H:%M:%S')] FAIL: $*" >&2; exit 1; }

# ── Preflight ──────────────────────────────────────────────────────────────

if [[ -z "$QUICKSTART_DIR" || ! -d "$QUICKSTART_DIR" ]]; then
    fail "globular-quickstart not found. Set QUICKSTART_DIR or clone it next to the services repo.
    Expected: $SERVICES_DIR/../globular-quickstart"
fi

for tool in docker; do
    command -v "$tool" >/dev/null 2>&1 || fail "$tool is required but not installed"
done

if ! docker info >/dev/null 2>&1; then
    fail "Docker daemon is not running"
fi

log "Using quickstart at: $QUICKSTART_DIR"
log "Test suite: $SUITE"

# ── Cleanup on exit ────────────────────────────────────────────────────────

CLUSTER_STARTED=0
teardown() {
    if [[ $CLUSTER_STARTED -eq 1 && "${SKIP_TEARDOWN:-0}" != "1" ]]; then
        log "Tearing down cluster..."
        cd "$QUICKSTART_DIR" && docker compose down -v --remove-orphans 2>/dev/null || true
        log "Cluster removed."
    elif [[ "${SKIP_TEARDOWN:-0}" == "1" ]]; then
        log "SKIP_TEARDOWN=1 — cluster left running at:"
        log "  Gateway:    https://localhost:10443"
        log "  Prometheus: http://localhost:19090"
        log "  Shell:      cd $QUICKSTART_DIR && make shell N=1"
    fi
}
trap teardown EXIT

# ── Build + start cluster ──────────────────────────────────────────────────

log "Collecting binaries into quickstart build context..."
# Collect freshly-built binaries from the stage directory into the quickstart
STAGE_BIN="$SERVICES_DIR/golang/tools/stage/linux-amd64/usr/local/bin"
if [[ -d "$STAGE_BIN" ]]; then
    mkdir -p "$QUICKSTART_DIR/binaries"
    # Copy only files that exist in the stage (don't fail if some are missing)
    find "$STAGE_BIN" -maxdepth 1 -type f -exec cp -f {} "$QUICKSTART_DIR/binaries/" \; 2>/dev/null || true
    log "  Copied binaries from $STAGE_BIN"
else
    log "  Warning: stage dir not found ($STAGE_BIN), relying on existing quickstart binaries"
fi

log "Starting containerized cluster..."
cd "$QUICKSTART_DIR"

if [[ "${SKIP_BUILD:-0}" != "1" ]]; then
    log "  Building Docker image..."
    docker build -q -t globulario/globular-node:latest . || fail "docker build failed"
fi

docker compose up -d --remove-orphans
CLUSTER_STARTED=1
log "  Containers started."

# ── Wait for readiness ─────────────────────────────────────────────────────

log "Waiting for cluster to be ready..."
bash "$SCRIPT_DIR/wait-ready.sh" "$CLUSTER_TIMEOUT"

# ── Run tests ─────────────────────────────────────────────────────────────

export GLOBULAR_TEST_CLUSTER=1
export GLOBULAR_TEST_ETCD_ENDPOINT="https://10.10.0.11:2379"
export GLOBULAR_TEST_CONTAINER="globular-node-1"

# Map suite name → Go test run pattern
declare -A SUITE_PATTERNS=(
    [smoke]="TestIntegrationSmoke"
    [reconcile]="TestIntegrationReconcile"
    [migration]="TestIntegrationMigration"
    [release]="TestIntegrationRelease"
    [all]="TestIntegration"
)

PATTERN="${SUITE_PATTERNS[$SUITE]:-TestIntegration}"
log "Running test suite '$SUITE' (pattern: $PATTERN)..."

cd "$SERVICES_DIR/golang"
go test \
    -tags integration \
    ./testcluster/... \
    -run "$PATTERN" \
    -v \
    -count=1 \
    -timeout 20m \
    2>&1

log "Integration tests complete."
