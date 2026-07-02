#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INSTALL_SH="${ROOT}/scripts/install.sh"
DAY0_SH="${ROOT}/scripts/release/install-day0.sh"
HEALTH_SH="${ROOT}/scripts/release/validate-cluster-health.sh"
CERT_FIX_SH="${ROOT}/scripts/release/fix-client-cert-ownership.sh"
CLEAN_SH="${ROOT}/scripts/clean-node.sh"
BOOTSTRAP_DNS_SH="${ROOT}/scripts/release/bootstrap-dns.sh"
RESOLVER_SH="${ROOT}/scripts/release/configure-resolver.sh"
BUILD_RELEASE_SH="${ROOT}/scripts/build-release.sh"
REMOTE_CLEAN_SH="${ROOT}/../Globular/internal/gateway/handlers/cluster/clean-node.sh"

die() { echo "FAIL: $*" >&2; exit 1; }
pass() { echo "PASS: $*"; }

[[ -f "${INSTALL_SH}" ]] || die "missing ${INSTALL_SH}"
[[ -f "${DAY0_SH}" ]] || die "missing ${DAY0_SH}"
[[ -f "${HEALTH_SH}" ]] || die "missing ${HEALTH_SH}"
[[ -f "${CERT_FIX_SH}" ]] || die "missing ${CERT_FIX_SH}"
[[ -f "${CLEAN_SH}" ]] || die "missing ${CLEAN_SH}"
[[ -f "${BOOTSTRAP_DNS_SH}" ]] || die "missing ${BOOTSTRAP_DNS_SH}"
[[ -f "${RESOLVER_SH}" ]] || die "missing ${RESOLVER_SH}"
[[ -f "${BUILD_RELEASE_SH}" ]] || die "missing ${BUILD_RELEASE_SH}"

if rg -n -- "--profile media-server" "${INSTALL_SH}" "${DAY0_SH}" >/dev/null; then
  die "default/example bootstrap guidance must not include --profile media-server"
fi
if rg -n -- "FOUNDING_PROFILES=core,media-server|--profile core --profile media-server --profile gateway" "${BUILD_RELEASE_SH}" >/dev/null; then
  die "release bundle guidance must not imply media-server on founding/bootstrap examples"
fi
pass "bootstrap guidance does not grant media-server implicitly"

rg -n 'release_version_from_bundle' "${INSTALL_SH}" >/dev/null || die "install.sh must define release_version_from_bundle"
rg -n 'release-index.json' "${INSTALL_SH}" >/dev/null || die "install.sh must prefer release-index.json for version reporting"
pass "install.sh version reporting is BOM-first"

rg -n 'AWARENESS_SUMMARY_VERDICT="SKIPPED"' "${HEALTH_SH}" >/dev/null || die "validate-cluster-health.sh must record SKIPPED awareness summary for Day-0"
rg -n 'Awareness evidence verdict: SKIPPED' "${HEALTH_SH}" >/dev/null || die "validate-cluster-health.sh summary must print SKIPPED"
pass "awareness summary is truthful for Day-0 skip"

rg -n 'CLUSTER_DOCTOR_PKG="\$\(resolve_pkg_artifact "cluster-doctor_0\.0\.1_linux_amd64\.tgz" \|\| true\)"' "${DAY0_SH}" >/dev/null || die "cluster-doctor package must resolve via the shared artifact resolver"
if rg -n 'Warning: cluster-doctor package not found at .*cluster-doctor_0\.0\.1' "${DAY0_SH}" >/dev/null; then
  die "cluster-doctor placeholder warning path still references the synthetic 0.0.1 artifact"
fi
pass "cluster-doctor uses resolved artifact identity"

rg -n 'Conformance runner not bundled' "${DAY0_SH}" >/dev/null || die "conformance missing path must say the runner is not bundled"
rg -n 'Optional validation skipped' "${DAY0_SH}" >/dev/null || die "conformance missing path must say optional validation was skipped"
pass "conformance missing path is explicit"

CERT_OUT="$(SUDO_USER=nobody USER=root bash "${CERT_FIX_SH}" 2>&1 || true)"
printf '%s\n' "${CERT_OUT}" | rg -n '^WARNING: Certificate directory not found' >/dev/null || die "cert ownership helper must downgrade missing-dir to WARNING"
if printf '%s\n' "${CERT_OUT}" | rg -n '^ERROR: Certificate directory not found' >/dev/null; then
  die "cert ownership helper must not emit ERROR for missing expected directory"
fi
pass "cert ownership helper missing-dir path is non-fatal and truthful"

TMP_TLS_ROOT="$(mktemp -d)"
cleanup() { rm -rf "${TMP_TLS_ROOT}"; }
trap cleanup EXIT
mkdir -p "${TMP_TLS_ROOT}/globular.internal"
openssl req -x509 -nodes -newkey rsa:2048 \
  -keyout "${TMP_TLS_ROOT}/globular.internal/ca.key" \
  -out "${TMP_TLS_ROOT}/globular.internal/ca.crt" \
  -subj "/CN=test-ca" >/dev/null 2>&1
openssl req -nodes -newkey rsa:2048 \
  -keyout "${TMP_TLS_ROOT}/globular.internal/client.key" \
  -out "${TMP_TLS_ROOT}/globular.internal/client.csr" \
  -subj "/CN=test-client" >/dev/null 2>&1
openssl x509 -req \
  -in "${TMP_TLS_ROOT}/globular.internal/client.csr" \
  -CA "${TMP_TLS_ROOT}/globular.internal/ca.crt" \
  -CAkey "${TMP_TLS_ROOT}/globular.internal/ca.key" \
  -CAcreateserial \
  -out "${TMP_TLS_ROOT}/globular.internal/client.crt" \
  -days 1 >/dev/null 2>&1
cp "${TMP_TLS_ROOT}/globular.internal/client.key" "${TMP_TLS_ROOT}/globular.internal/client.pem"
if [[ "$(id -u)" -eq 0 ]]; then
  pass "cert ownership helper success-path test skipped under root-only execution"
else
  CERT_SUCCESS_OUT="$(SUDO_USER=root USER=root GLOBULAR_CERT_HOME_OVERRIDE="${TMP_TLS_ROOT}" bash "${CERT_FIX_SH}" "$(id -un)" 2>&1)"
  printf '%s\n' "${CERT_SUCCESS_OUT}" | rg -n "Cert Directory: ${TMP_TLS_ROOT}/globular.internal" >/dev/null || die "cert ownership helper must discover an existing globular.internal cert directory"
  pass "cert ownership helper discovers the generated globular.internal cert directory"
fi

rg -n 'count_scylla_up_nodes\(\)' "${CLEAN_SH}" >/dev/null || die "clean-node.sh must use a single-output Scylla counter helper"
rg -n "awk '/\\^U\\[NL\\] / \\{n\\+\\+\\} END \\{print n\\+0\\}'" "${CLEAN_SH}" >/dev/null || die "clean-node.sh must use awk counting for Scylla decommission peers"
if rg -n 'grep -cE "\^U\[NL\] " \|\| echo "0"' "${CLEAN_SH}" >/dev/null; then
  die "clean-node.sh must not use grep -c plus fallback echo for Scylla peer counts"
fi
rg -n 'resolve_node_agent_state_path\(\)' "${CLEAN_SH}" >/dev/null || die "clean-node.sh must resolve the canonical node-agent state path"
rg -n '_STATE_FILE="\$\(resolve_node_agent_state_path\)"' "${CLEAN_SH}" >/dev/null || die "clean-node.sh must read node identity from the canonical node-agent state path"
rg -n "d.get\\('node_id', ''\\)\\.strip\\(\\)" "${CLEAN_SH}" >/dev/null || die "clean-node.sh must read the lowercase node_id field from state.json"
if rg -n "d.get\\('NodeID', ''\\)" "${CLEAN_SH}" >/dev/null; then
  die "clean-node.sh must not use the stale NodeID field name"
fi
rg -n 'controller removal start' "${CLEAN_SH}" >/dev/null || die "clean-node.sh must log controller removal start"
rg -n 'controller removal success' "${CLEAN_SH}" >/dev/null || die "clean-node.sh must log controller removal success"
rg -n 'service stop start' "${CLEAN_SH}" >/dev/null || die "clean-node.sh must log service stop start"
rg -n 'data wipe start' "${CLEAN_SH}" >/dev/null || die "clean-node.sh must log data wipe start"
rg -n 'package/state wipe start' "${CLEAN_SH}" >/dev/null || die "clean-node.sh must log package/state wipe start"
rg -n 'cleanup complete' "${CLEAN_SH}" >/dev/null || die "clean-node.sh must log cleanup complete"
pass "Scylla cleanup count and cleanup identity reads are canonical"

TMP_CLEAN_ROOT="$(mktemp -d)"
TMP_CLEAN_BIN="${TMP_CLEAN_ROOT}/bin"
TMP_CLEAN_STATE="${TMP_CLEAN_ROOT}/state"
mkdir -p "${TMP_CLEAN_BIN}" "${TMP_CLEAN_STATE}/node-agent"
cat > "${TMP_CLEAN_STATE}/node-agent/state.json" <<'EOF'
{"node_id":"test-node-id","controller_endpoint":"https://globular.internal:12000"}
EOF
cat > "${TMP_CLEAN_BIN}/hostname" <<'EOF'
#!/usr/bin/env bash
if [[ "${1:-}" == "-I" ]]; then
  echo "10.0.0.63"
else
  echo "test-node"
fi
EOF
cat > "${TMP_CLEAN_BIN}/curl" <<'EOF'
#!/usr/bin/env bash
printf '200'
EOF
cat > "${TMP_CLEAN_BIN}/systemctl" <<'EOF'
#!/usr/bin/env bash
case "${1:-}" in
  is-active)
    exit 1
    ;;
  list-units|list-timers)
    exit 0
    ;;
  *)
    exit 0
    ;;
esac
EOF
chmod +x "${TMP_CLEAN_BIN}/hostname" "${TMP_CLEAN_BIN}/curl" "${TMP_CLEAN_BIN}/systemctl"
CLEAN_FLOW_OUT="$(
  PATH="${TMP_CLEAN_BIN}:$PATH" \
  GLOBULAR_CLEAN_NODE_TEST_ALLOW_NON_ROOT=1 \
  GLOBULAR_CLEAN_NODE_TEST_STOP_AFTER=service_stop_start \
  GLOBULAR_SKIP_AI_BACKUP=1 \
  GLOBULAR_STATE_DIR_OVERRIDE="${TMP_CLEAN_STATE}" \
  bash "${CLEAN_SH}" --force 2>&1
)"
printf '%s\n' "${CLEAN_FLOW_OUT}" | rg -n 'controller removal success' >/dev/null || \
  die "clean-node.sh regression harness must reach controller removal success"
printf '%s\n' "${CLEAN_FLOW_OUT}" | rg -n 'service stop start' >/dev/null || \
  die "clean-node.sh regression harness must continue beyond RemoveNode to service stop start"
pass "clean-node.sh continues past successful RemoveNode"
rm -rf "${TMP_CLEAN_ROOT}"

rg -n 'systemctl daemon-reload' "${DAY0_SH}" >/dev/null || die "install-day0.sh must daemon-reload before cluster-doctor unit detection"
rg -n 'systemctl cat globular-cluster-doctor\.service' "${DAY0_SH}" >/dev/null || die "install-day0.sh must use systemctl cat to detect the cluster-doctor unit truthfully"
if rg -n 'systemctl list-unit-files \| grep -q "\^globular-cluster-doctor\.service"' "${DAY0_SH}" >/dev/null; then
  die "install-day0.sh must not rely on list-unit-files alone for cluster-doctor detection"
fi
pass "cluster-doctor unit detection is not based on stale list-unit-files output"

rg -n -- '--configure-only' "${DAY0_SH}" "${RESOLVER_SH}" >/dev/null || die "resolver configuration must support a configure-only phase"
rg -n -- '--verify-only' "${DAY0_SH}" "${RESOLVER_SH}" >/dev/null || die "resolver verification must run after bootstrap in verify-only mode"
rg -n 'VERIFY_RESULT="SKIPPED"' "${RESOLVER_SH}" >/dev/null || die "configure-resolver.sh must mark deferred verification as SKIPPED"
pass "resolver configuration and verification are split across the bootstrap boundary"

if rg -n 'Waiting for ScyllaDB to accept writes' "${BOOTSTRAP_DNS_SH}" >/dev/null; then
  die "bootstrap-dns.sh must not wait on a separate Scylla read gate before the authoritative DNS write probe"
fi
rg -n 'DNS database write-readiness is the authority' "${BOOTSTRAP_DNS_SH}" >/dev/null || die "bootstrap-dns.sh must document the DNS write-readiness authority"
pass "bootstrap-dns readiness is anchored to the DNS write path"

rg -n 'Desired state seeding completed without a heartbeat; seeded .* service\(s\), result may be partial' "${DAY0_SH}" >/dev/null || die "heartbeat-missing seed path must report partial seeding clearly"
rg -n 'Expected managed package records from this Day-0 run' "${DAY0_SH}" >/dev/null || die "desired-state seed path must log the expected managed inventory count"
rg -n 'Observed node-agent inventory after .* managed package records' "${DAY0_SH}" >/dev/null || die "desired-state seed path must log observed inventory counts while waiting"
rg -n 'partial node-agent inventory; observed .* managed package records' "${DAY0_SH}" >/dev/null || die "desired-state seed path must report observed/expected counts when seeding stays partial"
rg -n 'NA_STATE="\$\(resolve_node_agent_state_path\)"' "${DAY0_SH}" >/dev/null || die "install-day0.sh must write the join token to the canonical node-agent state path"
if rg -n 'NA_STATE="/var/lib/globular/nodeagent/state.json"' "${DAY0_SH}" >/dev/null; then
  die "install-day0.sh must not hardcode the legacy nodeagent state path"
fi
rg -n 'Using node-agent state node_id for heartbeat watch' "${DAY0_SH}" >/dev/null || die "install-day0.sh must watch the controller heartbeat using node-agent state node_id"
rg -n 'node-agent join is blocked by cluster-controller identity conflict: hostname already present' "${DAY0_SH}" >/dev/null || die "install-day0.sh must classify hostname-identity conflicts in heartbeat diagnostics"
pass "desired-state seed path reports partial state with explicit inventory counts and node-agent identity diagnostics"

rg -n 'resolve_registered_service_port "authentication\.AuthenticationService"' "${DAY0_SH}" >/dev/null || die "ops seed must resolve the authentication service endpoint directly"
rg -n -F '_LOGIN_ARGS+=(--auth "$_OPS_AUTH_ENDPOINT")' "${DAY0_SH}" >/dev/null || die "ops seed auth login must target the resolved authentication endpoint"
rg -n 'classify_auth_login_failure' "${DAY0_SH}" >/dev/null || die "ops seed must classify auth readiness failures truthfully"
pass "ops-knowledge seed auth readiness is endpoint-aware and diagnostic"

rg -n 'verify_embedded_clean_script_authority' "${BUILD_RELEASE_SH}" >/dev/null || die "build-release.sh must guard against stale embedded /clean payloads"
if [[ -f "${REMOTE_CLEAN_SH}" ]]; then
  cmp -s "${CLEAN_SH}" "${REMOTE_CLEAN_SH}" || die "embedded /clean payload drift detected between services and sibling Globular copy"
  pass "embedded /clean payload matches the authoritative cleanup script"
else
  pass "embedded /clean payload check skipped because sibling Globular repo is absent"
fi
