#!/usr/bin/env bash
set -euo pipefail

# Globular Day-0 Installation Script
#
# Environment Variables:
#   PKG_DIR                  - Package directory (default: internal/assets/packages)
#   INSTALLER_BIN            - Installer binary path (auto-detected)
#   TOLERATE_ALREADY_INSTALLED - Allow already-installed packages (default: 1)
#   FORCE_REINSTALL          - Force overwrite existing binaries even if unchanged (default: 0)
#                              Set to 1 to always reinstall all binaries (useful after rebuild)
#   GLOBULAR_CONFORMANCE     - Conformance test mode (default: warn)
#                              warn: Run tests, log failures, continue installation
#                              fail: Run tests, abort installation on any failure (v1 target)
#                              off:  Skip conformance tests entirely
#
# Conformance tests validate v1.0 invariants:
#   - DNS service reports correct port in metadata
#   - User client certificates exist and are readable
#   - TLS certificate symlinks (server.crt, server.key, ca.crt) exist
#   - DNS service has CAP_NET_BIND_SERVICE for port 53

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALLER_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
STATE_DIR="/var/lib/globular"
PKG_DIR="$INSTALLER_ROOT/internal/assets/packages"

# Respect INSTALLER_BIN if already set by a parent script (e.g. install.sh in the
# release tarball places globular-installer at the tarball root, not in bin/).
# Fall back to the dev/build layout, then PATH.
if [[ -z "${INSTALLER_BIN:-}" ]] || [[ ! -x "${INSTALLER_BIN}" ]]; then
  INSTALLER_BIN="$INSTALLER_ROOT/bin/globular-installer"
  if [[ ! -x "$INSTALLER_BIN" ]]; then
    INSTALLER_BIN="$(command -v globular-installer || true)"
  fi
fi

# Visual symbols for output
die() { echo "  ✗ ERROR: $*" >&2; trace_step "fatal" "die" "$*" 3; exit 1; }
log_info() { echo "  → $*"; }
log_success() { echo "  ✓ $*"; }
log_warn() { echo "  ⚠ $*"; }
log_step() { echo ""; echo "━━━ $* ━━━"; }
log_substep() { echo "  • $*"; }
is_loopback_ip() {
  [[ "$1" =~ ^127\. ]] || [[ "$1" == "::1" ]]
}

declare -a SEED_EXPECTED_MANAGED_PKGS=()

remember_seed_expected_pkg() {
  local pkgfile="${1:-}"
  [[ -n "$pkgfile" ]] || return 0
  local base key existing
  base="$(basename "$pkgfile" .tgz | sed 's/_linux_amd64$//')"
  key="${base%%_*}"
  [[ -n "$key" ]] || return 0
  for existing in "${SEED_EXPECTED_MANAGED_PKGS[@]:-}"; do
    [[ "$existing" == "$key" ]] && return 0
  done
  SEED_EXPECTED_MANAGED_PKGS+=("$key")
}

should_track_seed_inventory_request() {
  local requested="${1:-}"
  case "$requested" in
    mc_*|globular-cli_*|etcdctl_*|rclone_*|restic_*|sctool_*|sha256sum_*|claude_*|codex_*|yt-dlp_*|ffmpeg_*)
      return 1
      ;;
    *)
      return 0
      ;;
  esac
}

count_seed_expected_packages() {
  echo "${#SEED_EXPECTED_MANAGED_PKGS[@]}"
}

count_node_inventory_records() {
  local node_id="${1:-}"
  local etcd_endpoints="${2:-}"
  local ca_path="${3:-}"
  local cert_path="${4:-}"
  local key_path="${5:-}"
  [[ -n "$node_id" ]] || { echo "0"; return 0; }
  etcdctl --endpoints="$etcd_endpoints" \
    --cacert="$ca_path" --cert="$cert_path" --key="$key_path" \
    get "/globular/nodes/${node_id}/packages/" --prefix --keys-only 2>/dev/null \
    | grep -c "^/globular/nodes/${node_id}/packages/" || true
}

resolve_node_agent_state_path() {
  local canonical="/var/lib/globular/node-agent/state.json"
  local legacy="/var/lib/globular/nodeagent/state.json"
  if [[ -f "$canonical" ]]; then
    printf '%s\n' "$canonical"
    return 0
  fi
  if [[ -f "$legacy" ]]; then
    printf '%s\n' "$legacy"
    return 0
  fi
  printf '%s\n' "$canonical"
}

resolve_controller_state_path() {
  local canonical="/var/lib/globular/cluster-controller/state.json"
  local legacy="/var/lib/globular/clustercontroller/state.json"
  if [[ -f "$canonical" ]]; then
    printf '%s\n' "$canonical"
    return 0
  fi
  if [[ -f "$legacy" ]]; then
    printf '%s\n' "$legacy"
    return 0
  fi
  printf '%s\n' "$canonical"
}

read_json_field() {
  local json_path="${1:-}"
  local field="${2:-}"
  [[ -n "$json_path" ]] || return 0
  [[ -n "$field" ]] || return 0
  [[ -f "$json_path" ]] || return 0
  python3 - "$json_path" "$field" <<'PYEOF' 2>/dev/null || true
import json
import sys

path, field = sys.argv[1:]
try:
    with open(path, "r", encoding="utf-8") as fh:
        data = json.load(fh)
except Exception:
    raise SystemExit(0)
value = data.get(field, "")
if value is None:
    value = ""
print(str(value).strip())
PYEOF
}

read_controller_node_seed_status() {
  local state_path="${1:-}"
  local node_id="${2:-}"
  [[ -n "$state_path" ]] || return 0
  [[ -n "$node_id" ]] || return 0
  [[ -f "$state_path" ]] || return 0
  python3 - "$state_path" "$node_id" <<'PYEOF' 2>/dev/null || true
import json
import sys

state_path, node_id = sys.argv[1:]
try:
    with open(state_path, "r", encoding="utf-8") as fh:
        state = json.load(fh)
except Exception:
    raise SystemExit(0)
node = (state.get("nodes") or {}).get(node_id) or {}
last_seen = 1 if node.get("last_seen") else 0
inventory_complete = 1 if node.get("inventory_complete") else 0
installed_versions = node.get("installed_versions") or {}
print(f"{last_seen}\t{inventory_complete}\t{len(installed_versions)}")
PYEOF
}

resolve_registered_service_port() {
  local service_name="${1:-}"
  local etcd_endpoints="${2:-}"
  local ca_path="${3:-}"
  local cert_path="${4:-}"
  local key_path="${5:-}"
  [[ -n "$service_name" ]] || return 0
  etcdctl --endpoints="$etcd_endpoints" \
    --cacert="$ca_path" --cert="$cert_path" --key="$key_path" \
    get /globular/services/ --prefix -w json 2>/dev/null \
    | python3 - "$service_name" <<'PYEOF' 2>/dev/null || true
import base64
import json
import sys

want = sys.argv[1]
try:
    resp = json.load(sys.stdin)
except Exception:
    raise SystemExit(0)

for kv in resp.get("kvs", []):
    try:
        key = base64.b64decode(kv["key"]).decode("utf-8", "replace")
        if not key.endswith("/config"):
            continue
        cfg = json.loads(base64.b64decode(kv.get("value", "")).decode("utf-8", "replace"))
    except Exception:
        continue
    if cfg.get("Name") == want and cfg.get("Port"):
        print(cfg["Port"])
        break
PYEOF
}

classify_auth_login_failure() {
  local output="${1:-}"
  local endpoint="${2:-}"
  local lower first_line
  lower="$(printf '%s' "$output" | tr '[:upper:]' '[:lower:]')"
  first_line="$(printf '%s\n' "$output" | head -n 1 | tr -d '\r')"
  case "$lower" in
    *"connection refused"*|*"deadline exceeded"*|*"context deadline exceeded"*|*"no route to host"*|*"connection reset by peer"*|*"unavailable desc"*)
      printf 'endpoint unreachable at %s' "${endpoint:-<unknown>}"
      ;;
    *"x509:"*|*"tls:"*|*"certificate"*|*"first record does not look like a tls handshake"*)
      printf 'TLS handshake/certificate failure at %s' "${endpoint:-<unknown>}"
      ;;
    *"invalid credentials"*|*"invalid username or password"*|*"wrong password"*|*"permission denied"*|*"unauthenticated"*)
      printf 'bootstrap SA credentials rejected by authentication service'
      ;;
    *"user sa"*' '*"not found"*|*"account not found"*|*"no such user"*)
      printf 'bootstrap SA account is missing or not initialized yet'
      ;;
    *)
      if [[ -n "$first_line" ]]; then
        printf 'unclassified auth failure: %s' "$first_line"
      else
        printf 'auth login returned no token and no diagnostic text'
      fi
      ;;
  esac
}

log_node_agent_seed_diagnostics() {
  local state_path="${1:-}"
  local node_id="${2:-}"
  local expected_count="${3:-0}"
  local observed_count="${4:-0}"
  local controller_endpoint join_token controller_state
  controller_endpoint="$(read_json_field "$state_path" "controller_endpoint")"
  join_token="$(read_json_field "$state_path" "join_token")"
  controller_state="inactive"
  if systemctl is-active --quiet globular-cluster-controller.service 2>/dev/null; then
    controller_state="active"
  fi
  if systemctl is-active --quiet globular-node-agent.service 2>/dev/null; then
    log_substep "node-agent service state: active"
  else
    log_warn "node-agent service is not active"
  fi
  log_substep "cluster-controller service state: ${controller_state}"
  if [[ -n "$state_path" ]]; then
    log_substep "node-agent state path: ${state_path}"
  fi
  if [[ -n "$node_id" ]]; then
    log_substep "node-agent state node_id: ${node_id}"
  else
    log_warn "node-agent state file has no node_id yet"
  fi
  if [[ -n "$controller_endpoint" ]]; then
    log_substep "node-agent controller endpoint: ${controller_endpoint}"
  fi
  if [[ -n "$join_token" ]]; then
    log_substep "node-agent join token present in state file"
  else
    log_warn "node-agent join token missing from state file"
  fi
  log_substep "Observed node-agent inventory before seed timeout: ${observed_count}/${expected_count}"

  local journal_excerpt join_line
  journal_excerpt="$(journalctl -u globular-node-agent.service -n 40 --no-pager 2>/dev/null || true)"
  join_line="$(printf '%s\n' "$journal_excerpt" | grep -F 'RequestJoin failed' | tail -1 || true)"
  if printf '%s\n' "$journal_excerpt" | grep -Fq 'hostname already present'; then
    log_warn "node-agent join is blocked by cluster-controller identity conflict: hostname already present"
  elif [[ -n "$join_line" ]]; then
    log_warn "node-agent join is failing: $(printf '%s' "$join_line" | sed 's/^[[:space:]]*//')"
  else
    log_substep "Recent node-agent journal excerpt:"
    while IFS= read -r line; do
      [[ -n "$line" ]] || continue
      echo "    $line"
    done <<< "$journal_excerpt"
  fi
}

# Returns the routable IP ScyllaDB is listening on, read from scylla.yaml.
# Never returns localhost/127.x — ScyllaDB must bind to a routable IP for
# cluster connectivity. Falls back to the first non-loopback interface IP.
scylla_cql_host() {
  local h
  h=$(grep "^listen_address:" /etc/scylla/scylla.yaml 2>/dev/null \
        | awk '{print $2}' | tr -d "'\"" || true)
  if [[ -z "$h" ]] || [[ "$h" == "localhost" ]] || is_loopback_ip "$h"; then
    h=$(hostname -I | tr ' ' '\n' | grep -v '^\s*$' | grep -v '^::' \
          | while IFS= read -r ip; do
              is_loopback_ip "$ip" || { echo "$ip"; break; }
            done)
  fi
  echo "${h:-}"
}

# Normalize metadata values read from JSON/CLI so sentinel text does not get
# treated as real identity values.
normalize_meta_value() {
  local v="${1:-}"
  v="$(printf '%s' "$v" | tr -d '\r\n' | xargs 2>/dev/null || true)"
  local lower
  lower="$(printf '%s' "$v" | tr '[:upper:]' '[:lower:]')"
  case "$lower" in
    ""|"unknown"|"null"|"none"|"n/a"|"na"|"-")
      echo ""
      ;;
    *)
      echo "$v"
      ;;
  esac
}

has_profile() {
  local want="$1"
  IFS=',' read -ra profiles <<< "$FOUNDING_PROFILES"
  for p in "${profiles[@]}"; do
    p="$(echo "$p" | xargs)"
    [[ "$p" == "$want" ]] && return 0
  done
  return 1
}

# wait_scylla_write_ready HOST
# Waits until ScyllaDB accepts DDL write operations (CREATE KEYSPACE), not just
# reads. The SELECT probe passes while Scylla is still in a pre-write state, so
# services like persistence (which create keyspaces on first start) would fail
# immediately after the read probe succeeded.
wait_scylla_write_ready() {
  local host="${1:-localhost}"
  log_substep "Waiting for ScyllaDB write readiness (DDL probe) on ${host}:9042..."
  local ok=0
  for _i in $(seq 1 60); do
    if cqlsh "$host" 9042 <<'CQL' &>/dev/null
CREATE KEYSPACE IF NOT EXISTS globular_install_probe WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1};
DROP KEYSPACE IF EXISTS globular_install_probe;
CQL
    then
      ok=1
      break
    fi
    sleep 2
  done
  if [[ $ok -eq 1 ]]; then
    log_success "ScyllaDB write-ready (DDL probe passed after ~$((_i * 2))s)"
  else
    log_warn "ScyllaDB DDL probe did not pass after 120s — downstream schema creation may fail on first attempt"
  fi
}

# ── Workflow trace log ─────────────────────────────────────────────────────
# Writes JSON-lines to DAY0_TRACE_LOG. The workflow service imports this on
# startup to create a proper workflow run visible in the admin UI.
DAY0_TRACE_LOG="/var/lib/globular/day0-install.jsonl"
DAY0_TRACE_SEQ=0
DAY0_TRACE_START=$(date +%s%3N)

trace_step() {
  local status="$1" step_key="$2" title="$3" phase="${4:-5}"
  DAY0_TRACE_SEQ=$((DAY0_TRACE_SEQ + 1))
  local now_ms=$(date +%s%3N)
  local dur=$((now_ms - DAY0_TRACE_START))
  printf '{"seq":%d,"key":"%s","title":"%s","status":"%s","phase":%d,"actor":4,"ts":%d,"dur":%d}\n' \
    "$DAY0_TRACE_SEQ" "$step_key" "$title" "$status" "$phase" "$now_ms" "$dur" \
    >> "$DAY0_TRACE_LOG" 2>/dev/null || true
  DAY0_TRACE_START=$now_ms
}

trace_start() {
  mkdir -p "$(dirname "$DAY0_TRACE_LOG")"
  printf '{"type":"run_start","ts":%d,"hostname":"%s"}\n' \
    "$(date +%s%3N)" "$(hostname)" > "$DAY0_TRACE_LOG" 2>/dev/null || true
}

trace_finish() {
  local status="$1" msg="$2"
  printf '{"type":"run_finish","status":"%s","msg":"%s","ts":%d}\n' \
    "$status" "$msg" "$(date +%s%3N)" >> "$DAY0_TRACE_LOG" 2>/dev/null || true
}

trace_start

[[ -d "$PKG_DIR" ]] || die "Package directory not found: $PKG_DIR"
[[ -n "$INSTALLER_BIN" ]] && [[ -x "$INSTALLER_BIN" ]] || die "Installer binary not found; set INSTALLER_BIN or build ./bin/globular-installer"

# Check for root privileges
if [[ $EUID -ne 0 ]]; then
  die "This script must be run as root (use sudo)"
fi

# Ensure /bin/echo exists — apt post-invoke hooks reference it and fail on
# systems where /bin is not symlinked to /usr/bin (minimal installs, containers).
if [[ ! -x /bin/echo ]] && [[ -x /usr/bin/echo ]]; then
  ln -sf /usr/bin/echo /bin/echo
fi

detect_install_cmd() {
  if "$INSTALLER_BIN" pkg --help >/dev/null 2>&1; then
    if "$INSTALLER_BIN" pkg install --help >/dev/null 2>&1; then
      if "$INSTALLER_BIN" pkg install --help 2>&1 | grep -q -- "--package"; then
        echo "pkg_install_flag"; return 0
      fi
      echo "pkg_install_arg"; return 0
    fi
  fi

  if "$INSTALLER_BIN" install --help >/dev/null 2>&1; then
    if "$INSTALLER_BIN" install --help 2>&1 | grep -q -- "--package"; then
      echo "install_flag"; return 0
    fi
    echo "install_arg"; return 0
  fi

  die "Could not detect install command form for $INSTALLER_BIN"
}

detect_uninstall_cmd() {
  if "$INSTALLER_BIN" pkg --help >/dev/null 2>&1; then
    if "$INSTALLER_BIN" pkg uninstall --help >/dev/null 2>&1; then
      if "$INSTALLER_BIN" pkg uninstall --help 2>&1 | grep -q -- "--package"; then
        echo "pkg_uninstall_flag"; return 0
      fi
      echo "pkg_uninstall_arg"; return 0
    fi
  fi

  if "$INSTALLER_BIN" uninstall --help >/dev/null 2>&1; then
    if "$INSTALLER_BIN" uninstall --help 2>&1 | grep -q -- "--package"; then
      echo "uninstall_flag"; return 0
    fi
    echo "uninstall_arg"; return 0
  fi

  if "$INSTALLER_BIN" remove --help >/dev/null 2>&1; then
    if "$INSTALLER_BIN" remove --help 2>&1 | grep -q -- "--package"; then
      echo "remove_flag"; return 0
    fi
    echo "remove_arg"; return 0
  fi

  echo "unknown"
}

INSTALL_MODE="$(detect_install_cmd)"
UNINSTALL_MODE="$(detect_uninstall_cmd)"


TOLERATE_ALREADY_INSTALLED="1"
FORCE_REINSTALL="0"

# Canonical cluster domain — single source of truth for all Day-0 scripts
DOMAIN="globular.internal"

# Founding-node profiles, seeded into the cluster controller's default_profiles.
# Comma-separated; extra workload profiles must be explicit operator intent.
# Override as needed, e.g.:
#   FOUNDING_PROFILES=core,gateway ./install-day0.sh
# The controller's enforceFoundingProfiles() ALWAYS adds the founding quorum
# (control-plane,core,storage) for the first 3 nodes regardless of this.
FOUNDING_PROFILES="${FOUNDING_PROFILES:-core}"
FORCE_FLAG=""
if [[ "$FORCE_REINSTALL" == "1" ]]; then
  FORCE_FLAG="--force"
fi

echo ""
# ── Logo ──────────────────────────────────────────────────────────────────────
# Display logo in terminal if chafa is available; install it silently if missing.
LOGO_FILE="${SCRIPT_DIR}/../assets/logo.png"
if [[ ! -f "$LOGO_FILE" ]]; then
  # Fallback paths
  for p in "$HOME/Pictures/logo.png" "$HOME/pictures/logo.png" /usr/share/globular/logo.png; do
    [[ -f "$p" ]] && LOGO_FILE="$p" && break
  done
fi

if [[ -f "$LOGO_FILE" ]]; then
  if ! command -v chafa >/dev/null 2>&1; then
    apt-get install -y -qq chafa >/dev/null 2>&1 || true
  fi
  if command -v chafa >/dev/null 2>&1; then
    echo ""
    chafa --size=80x25 --symbols=all --colors=full "$LOGO_FILE" 2>/dev/null || true
    echo ""
  fi
fi

echo "╔════════════════════════════════════════════════════════════════╗"
echo "║          GLOBULAR DAY-0 INSTALLATION                           ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""

# Prompt for MinIO data storage location
# MinIO stores all bucket data under this directory. Choose a path on a
# drive with enough space for your object-store data (backups, media, etc.)
MINIO_DATA_DIR="/var/lib/globular/minio/data"
# Ensure absolute path
if [[ "${MINIO_DATA_DIR:0:1}" != "/" ]]; then
  die "MinIO data directory must be an absolute path: $MINIO_DATA_DIR"
fi
MINIO_DATA_DIR_FLAG="--minio-data-dir $MINIO_DATA_DIR"

log_info "Installer binary: $INSTALLER_BIN"
log_info "Install mode: $INSTALL_MODE"
log_info "Package directory: $PKG_DIR"
log_info "MinIO data directory: $MINIO_DATA_DIR"
log_info "Cluster domain: $DOMAIN"
log_info "Conformance mode: warn"
echo ""

materialize_bundled_policy_manifests() {
  local policy_root="/var/lib/globular/policy/services"
  local source_dir=""
  local candidate service policy_file copied=0

  for candidate in \
    "$INSTALLER_ROOT/generated/payload" \
    "$INSTALLER_ROOT/payload" \
    "$INSTALLER_ROOT/../generated/payload" \
    "$SCRIPT_DIR/../generated/payload" \
    "$SCRIPT_DIR/../../generated/payload"; do
    if [[ -d "$candidate" ]]; then
      source_dir="$candidate"
      break
    fi
  done

  if [[ -z "$source_dir" ]]; then
    log_warn "No bundled service policy payload found; RBAC semantic action mapping may be unavailable until packages install"
    return 0
  fi

  mkdir -p "$policy_root"
  for candidate in "$source_dir"/*; do
    [[ -d "$candidate/policy" ]] || continue
    service="$(basename "$candidate")"
    mkdir -p "$policy_root/$service"
    for policy_file in permissions.generated.json roles.generated.json; do
      if [[ -f "$candidate/policy/$policy_file" ]]; then
        install -m 0644 "$candidate/policy/$policy_file" "$policy_root/$service/$policy_file"
        copied=$((copied + 1))
      fi
    done
  done

  chown -R globular:globular "$policy_root" 2>/dev/null || true
  log_success "Service RBAC policy manifests materialized: ${copied} files from ${source_dir}"
}

log_step "Service RBAC Policy Manifests"
materialize_bundled_policy_manifests

# Stop any cluster agents that might be running from a previous Day-0.
# The node-agent's reconciliation loop will stop services (like MinIO) while
# the installer is configuring them, causing bucket-provisioning to fail.
# On a fresh node these services don't exist yet, so this is always safe.
for _svc in globular-node-agent.service globular-cluster-controller.service; do
  if systemctl is-active --quiet "$_svc" 2>/dev/null; then
    log_substep "Stopping $_svc (will be restarted at end of Day-0)..."
    systemctl stop "$_svc" 2>/dev/null || true
  fi
done

# TLS MUST be set up BEFORE any packages are installed
log_step "TLS Certificate Bootstrap"
trace_step "running" "phase.tls" "TLS Certificate Bootstrap" 6
if [[ -x "$SCRIPT_DIR/setup-tls.sh" ]]; then
  "$SCRIPT_DIR/setup-tls.sh" || die "TLS setup failed"
  log_success "TLS certificates generated (RSA)"
else
  die "setup-tls.sh not found or not executable"
fi

# Register the Globular CA in the system trust store so that tools using
# the OS certificate bundle (curl, MCP server, Go http.DefaultTransport, etc.)
# trust Globular's TLS certificates without needing --insecure flags.
CA_SRC="${STATE_DIR}/pki/ca.crt"
CA_DST="/usr/local/share/ca-certificates/globular-ca.crt"
if [[ -f "$CA_SRC" ]]; then
  cp "$CA_SRC" "$CA_DST"
  chmod 644 "$CA_DST"
  update-ca-certificates --fresh >/dev/null 2>&1 || update-ca-certificates >/dev/null 2>&1 || true
  log_success "Globular CA registered in system trust store (${CA_DST})"

  # Also copy to each user's ~/.config/globular/ca.crt so tools that use
  # desktop tooling can always read it without
  # needing group membership or directory traversal into /var/lib/globular/pki/.
  for _uh in /root /home/*; do
    [[ -d "$_uh" ]] || continue
    _ca_user_dir="$_uh/.config/globular"
    mkdir -p "$_ca_user_dir"
    cp "$CA_SRC" "$_ca_user_dir/ca.crt"
    chmod 644 "$_ca_user_dir/ca.crt"
    _owner=$(stat -c '%U' "$_uh")
    chown "$_owner:$_owner" "$_ca_user_dir/ca.crt" 2>/dev/null || true
  done
  log_success "Globular CA copied to user .config/globular/ directories"
else
  log_warn "CA not found at ${CA_SRC} — skipping system trust store registration"
fi

# Generate root/admin client certificates for CLI and service-to-service communication
log_step "Client Certificate Generation"
if [[ -x "$SCRIPT_DIR/generate-user-client-cert.sh" ]]; then
  if "$SCRIPT_DIR/generate-user-client-cert.sh" root 2>&1 | tee /tmp/client-cert-root.log; then
    log_success "Root client certificates generated"
  else
    die "Root client certificate generation failed (check /tmp/client-cert-root.log) - CLI will not work without this"
  fi

  ORIGINAL_USER=""
  if [[ -d "$SCRIPT_DIR" ]]; then
    DETECTED_USER=$(stat -c '%U' "$SCRIPT_DIR" 2>/dev/null || echo "")
    if [[ -n "$DETECTED_USER" ]] && [[ "$DETECTED_USER" != "root" ]]; then
      ORIGINAL_USER="$DETECTED_USER"
      log_info "Detected installer user from directory ownership: $ORIGINAL_USER"
    fi
  fi

  if [[ -n "$ORIGINAL_USER" ]]; then
    if "$SCRIPT_DIR/generate-user-client-cert.sh" "$ORIGINAL_USER" 2>&1 | tee "/tmp/client-cert-$ORIGINAL_USER.log"; then
      # Fix ownership of generated certificates
      if [[ -x "$SCRIPT_DIR/fix-client-cert-ownership.sh" ]]; then
        "$SCRIPT_DIR/fix-client-cert-ownership.sh" "$ORIGINAL_USER" 2>&1 | tee "/tmp/client-cert-fix-$ORIGINAL_USER.log" || true
      fi
      log_success "User ($ORIGINAL_USER) client certificates generated"
    else
      die "User ($ORIGINAL_USER) client certificate generation failed (check /tmp/client-cert-$ORIGINAL_USER.log) - CLI will not work without this"
    fi
  else
    log_info "No non-root user detected, skipping user client certificate generation"
  fi
else
  die "generate-user-client-cert.sh not found - CLI will not work without client certificates"
fi

# Note: ScyllaDB TLS is configured in the "ScyllaDB Database" section below
# (both fresh-install and already-installed paths run setup-scylla-tls.sh)

install_from_extracted_spec() {
  local pkgfile="$1"
  local staging spec out rc
  staging="$(mktemp -d)"
  cleanup() { rm -rf "$staging"; }
  trap cleanup RETURN

  tar -xzf "$pkgfile" -C "$staging"

  spec=""
  if [[ -d "$staging/specs" ]]; then
    spec="$(find "$staging/specs" -maxdepth 1 -type f \( -name "*.yaml" -o -name "*.yml" -o -name "*.json" \) | head -n 1)"
  fi

  if [[ -z "${spec:-}" || ! -f "$spec" ]]; then
    echo "    ✗ Could not locate embedded spec in package: $pkgfile" >&2
    return 2
  fi

  set +e
  # shellcheck disable=SC2086
  out="$("$INSTALLER_BIN" install --staging-dir "$staging" --spec "$spec" $FORCE_FLAG $MINIO_DATA_DIR_FLAG 2>&1)"
  rc=$?
  set -e

  if [[ $rc -ne 0 ]]; then
    echo "$out" >&2
    return $rc
  fi
  return 0
}

run_install() {
  local pkgfile="$1"
  local pkgname="$(basename "$pkgfile" .tgz | sed 's/_linux_amd64$//')"
  local out rc

  log_substep "Installing $pkgname..."
  log_substep "  CMD: $INSTALLER_BIN install $FORCE_FLAG $MINIO_DATA_DIR_FLAG $pkgfile"

  set +e
  # shellcheck disable=SC2086
  # NOTE: flags MUST come before positional args (Go flag package stops at first non-flag)
  case "$INSTALL_MODE" in
    pkg_install_flag) out="$("$INSTALLER_BIN" pkg install --package "$pkgfile" $FORCE_FLAG $MINIO_DATA_DIR_FLAG 2>&1)"; rc=$? ;;
    pkg_install_arg)  out="$("$INSTALLER_BIN" pkg install $FORCE_FLAG $MINIO_DATA_DIR_FLAG "$pkgfile" 2>&1)"; rc=$? ;;
    install_flag)     out="$("$INSTALLER_BIN" install --package "$pkgfile" $FORCE_FLAG $MINIO_DATA_DIR_FLAG 2>&1)"; rc=$? ;;
    install_arg)      out="$("$INSTALLER_BIN" install $FORCE_FLAG $MINIO_DATA_DIR_FLAG "$pkgfile" 2>&1)"; rc=$? ;;
    *) die "Unknown install mode: $INSTALL_MODE" ;;
  esac
  set -e

  if [[ $rc -ne 0 ]] && echo "$out" | grep -qiE "using spec default|missing files definition"; then
    set +e
    out="$(install_from_extracted_spec "$pkgfile" 2>&1)"; rc=$?
    set -e
    if [[ $rc -ne 0 ]]; then
      echo "$out" >&2
      trace_step "failed" "install.$pkgname" "Install $pkgname (spec fallback failed)"
      die "Failed to install $pkgname"
    fi
    log_success "$pkgname installed"
    trace_step "ok" "install.$pkgname" "Install $pkgname (spec fallback)"
    return 0
  fi

  if [[ $rc -ne 0 ]]; then
    if [[ "$TOLERATE_ALREADY_INSTALLED" == "1" ]] && echo "$out" | grep -qiE "already installed|exists|is installed"; then
      log_success "$pkgname (already installed)"
      trace_step "ok" "install.$pkgname" "Install $pkgname (already installed)"
      return 0
    fi
    echo "$out" >&2
    trace_step "failed" "install.$pkgname" "Install $pkgname failed"
    die "Failed to install $pkgname"
  fi

  log_success "$pkgname installed"
  trace_step "ok" "install.$pkgname" "Install $pkgname"
}

install_list() {
  local pkg_array=("$@")
  for f in "${pkg_array[@]}"; do
    local path=""
    path="$(resolve_pkg_artifact "$f" || true)"
    if [[ -z "$path" ]]; then
      log_substep "Warning: package not found, skipping: $PKG_DIR/$f"
      continue
    fi
    run_install "$path"
    if should_track_seed_inventory_request "$f"; then
      remember_seed_expected_pkg "$path"
    fi
  done
}

resolve_pkg_artifact() {
  local requested="$1"
  local exact="$PKG_DIR/$requested"
  if [[ -f "$exact" ]]; then
    printf '%s\n' "$exact"
    return 0
  fi
  local prefix="${requested%%_*}"
  local match=""
  match=$(ls "$PKG_DIR/${prefix}_"*"_linux_amd64.tgz" 2>/dev/null | head -1) || match=""
  if [[ -n "$match" ]] && [[ -f "$match" ]]; then
    log_substep "Resolved $requested → $(basename "$match")" >&2
    printf '%s\n' "$match"
    return 0
  fi
  return 1
}

SCYLLADB_PKG="scylladb_2025.3.8_linux_amd64.tgz"  # canonical fallback name for logging

# Resolve scylladb package by prefix — version-agnostic, same approach as install_list().
# Also searches /var/lib/globular/packages/ (where join-script stages packages) as a
# secondary location so Day-0 re-runs on an already-joined node find the artifact.
_resolve_scylladb_pkg() {
  local dirs=("$PKG_DIR" "/var/lib/globular/packages")
  for dir in "${dirs[@]}"; do
    [[ -d "$dir" ]] || continue
    local match
    match=$(ls "$dir/scylladb_"*"_linux_amd64.tgz" 2>/dev/null | head -1) || match=""
    if [[ -n "$match" ]] && [[ -f "$match" ]]; then
      echo "$match"
      return 0
    fi
  done
  return 1
}
SCYLLADB_PKG_PATH=$(_resolve_scylladb_pkg || true)

BOOTSTRAP_MINIO_PKGS=(
  # sha256sum is installed first so the /usr/local/bin/sha256sum wrapper is
  # valid for the rest of the installation (and for any subsequent upgrade
  # verification). Without this, a post-wipe reinstall hits a stale wrapper
  # pointing at the wiped /usr/lib/globular/bin/sha256sum and breaks.
  "sha256sum_9.4.0_linux_amd64.tgz"
  "etcd_3.5.14_linux_amd64.tgz"
  "minio_RELEASE.2025-09-07T16-13-09Z_linux_amd64.tgz"
)

DATA_LAYER_PKGS=(
  "persistence_0.0.1_linux_amd64.tgz"
)

BOOTSTRAP_REST_PKGS=(
  "xds_0.0.1_linux_amd64.tgz"
  # gateway must come before envoy: envoy's ExecStartPre waits for
  # /run/globular/envoy/envoy-bootstrap.json which gateway writes on startup.
  "gateway_0.0.1_linux_amd64.tgz"
  "envoy_1.35.3_linux_amd64.tgz"
  "node-agent_0.0.1_linux_amd64.tgz"
  "cluster-controller_0.0.1_linux_amd64.tgz"
  "cluster-doctor_0.0.1_linux_amd64.tgz"
)

CONTROL_PLANE_PKGS=(
  "resource_0.0.1_linux_amd64.tgz"
  "rbac_0.0.1_linux_amd64.tgz"
  "authentication_0.0.1_linux_amd64.tgz"
  # DNS must be installed before repository so it gets its default port (10006).
  # The PortAllocator assigns ports in first-come order; repository would otherwise
  # grab 10006 first and force DNS to reallocate to 10007, breaking bootstrap-dns.sh.
  "dns_0.0.1_linux_amd64.tgz"
  "repository_0.0.1_linux_amd64.tgz"
)

OPS_PKGS=(
  "sidekick_7.0.0_linux_amd64.tgz"
  "node-exporter_1.10.2_linux_amd64.tgz"
  "prometheus_3.5.1_linux_amd64.tgz"
  "alertmanager_0.28.1_linux_amd64.tgz"
  "monitoring_0.0.1_linux_amd64.tgz"
  "event_0.0.1_linux_amd64.tgz"
  "log_0.0.1_linux_amd64.tgz"
  "backup-manager_0.0.1_linux_amd64.tgz"
  "mcp_0.0.2_linux_amd64.tgz"
  "ai-memory_0.0.1_linux_amd64.tgz"
  "ai-watcher_0.0.1_linux_amd64.tgz"
  "ai-executor_0.0.1_linux_amd64.tgz"
  "ai-router_0.0.1_linux_amd64.tgz"
  "workflow_0.0.1_linux_amd64.tgz"
  "scylla-manager-agent_3.11.1_linux_amd64.tgz"
  "scylla-manager_3.11.1_linux_amd64.tgz"
)

COMMON_WORKLOAD_PKGS=(
  "file_0.0.1_linux_amd64.tgz"
  "search_0.0.1_linux_amd64.tgz"
)

MEDIA_WORKLOAD_PKGS=(
  "media_0.0.1_linux_amd64.tgz"
  "title_0.0.1_linux_amd64.tgz"
  "torrent_0.0.1_linux_amd64.tgz"
)

COMMON_CMDS_PKGS=(
  "mc_0.0.1_linux_amd64.tgz"
  "globular-cli_0.0.1_linux_amd64.tgz"
  "etcdctl_3.5.14_linux_amd64.tgz"
  "rclone_1.73.1_linux_amd64.tgz"
  "restic_0.18.1_linux_amd64.tgz"
  "sctool_3.11.1_linux_amd64.tgz"
  "sha256sum_9.4.0_linux_amd64.tgz"
  "claude_2.1.177_linux_amd64.tgz"
  "codex_0.142.3_linux_amd64.tgz"
)

MEDIA_CMDS_PKGS=(
  "yt-dlp_2026.2.21_linux_amd64.tgz"
  "ffmpeg_7.0.2_linux_amd64.tgz"
)

# Phase 2: Enable bootstrap mode for Day-0 installation
# Security Fix #4: Create JSON state file with explicit timestamps
# This enables 4-level secured bootstrap mode:
# - Time-bounded (30 minutes from now, explicit in file)
# - Loopback-only
# - Method allowlisted (essential Day-0 methods only)
# - Explicit enablement (this file with 0600 permissions)
BOOTSTRAP_FLAG="/var/lib/globular/bootstrap.enabled"
log_substep "Enabling bootstrap mode (30-minute window)..."
mkdir -p "$(dirname "$BOOTSTRAP_FLAG")"

# Create JSON state file with explicit timestamps (not relying on mtime)
ENABLED_AT=$(date +%s)
EXPIRES_AT=$((ENABLED_AT + 1800))  # 30 minutes = 1800 seconds
NONCE=$(openssl rand -hex 16 2>/dev/null || echo "fallback-nonce-$$")

cat > "$BOOTSTRAP_FLAG" <<EOF
{
  "enabled_at_unix": $ENABLED_AT,
  "expires_at_unix": $EXPIRES_AT,
  "nonce": "$NONCE",
  "created_by": "install-day0.sh",
  "version": "1.0"
}
EOF

# Set secure permissions: 0600, globular-owned so services running as globular can read it.
# The bootstrap gate allows both root-owned and globular-owned files.
chmod 0600 "$BOOTSTRAP_FLAG"
if id globular >/dev/null 2>&1; then
  chown globular:globular "$BOOTSTRAP_FLAG" 2>/dev/null || true
else
  chown root:root "$BOOTSTRAP_FLAG" 2>/dev/null || chown 0:0 "$BOOTSTRAP_FLAG"
fi

log_success "Bootstrap mode enabled: $BOOTSTRAP_FLAG (expires: $(date -d @$EXPIRES_AT '+%Y-%m-%d %H:%M:%S' 2>/dev/null || date -r $EXPIRES_AT '+%Y-%m-%d %H:%M:%S' 2>/dev/null || echo 'in 30 minutes'))"

# Write bootstrap sa credential file for non-interactive artifact publishing.
# Permissions 0600, root-owned. Deleted in Phase 5 cleanup.
# On Day-0 the sa account always starts with the default password (adminadmin).
# No interactive prompt needed — it would block unattended installs for no benefit.
BOOTSTRAP_SA_CRED="/var/lib/globular/.bootstrap-sa-password"
BOOTSTRAP_PASSWORD="adminadmin"
if [[ -n "${BOOTSTRAP_PASSWORD}" ]]; then
  printf '%s' "$BOOTSTRAP_PASSWORD" > "$BOOTSTRAP_SA_CRED"
  chmod 0600 "$BOOTSTRAP_SA_CRED"
  chown root:root "$BOOTSTRAP_SA_CRED" 2>/dev/null || true
fi

log_step "ScyllaDB Database"
if systemctl list-unit-files 2>/dev/null | grep -q "^scylla-server.service"; then
  log_success "ScyllaDB packages already installed"

  # Ensure data dirs exist (may have been wiped for a clean reinstall)
  if [[ ! -d /var/lib/scylla/data ]] || [[ ! -d /var/lib/scylla/commitlog ]]; then
    log_substep "ScyllaDB data directories missing — recreating..."
    mkdir -p /var/lib/scylla/data /var/lib/scylla/commitlog
    chown -R scylla:scylla /var/lib/scylla
    log_success "ScyllaDB data directories recreated"
  fi

  # TLS setup is handled by scylladb package post-install script when present.
  # Fallback to external script for packages built without embedded scripts.
  if [[ -f /etc/scylla/tls/server.crt ]]; then
    log_substep "ScyllaDB TLS already configured"
  elif [[ -x "$SCRIPT_DIR/setup-scylla-tls.sh" ]]; then
    log_substep "Configuring ScyllaDB TLS (fallback)..."
    "$SCRIPT_DIR/setup-scylla-tls.sh" || die "ScyllaDB TLS setup failed"
    log_success "ScyllaDB configured with TLS"
  fi

  # Validate scylla.yaml: if missing or has loopback listen_address, the running
  # config is unusable. Re-run the bundled post-install to regenerate it.
  _SCYLLA_YAML_OK=false
  if [[ -f /etc/scylla/scylla.yaml ]]; then
    _yaml_ip=$(grep "^listen_address:" /etc/scylla/scylla.yaml | awk '{print $2}' | tr -d "'\""  )
    if [[ -n "$_yaml_ip" ]] && [[ "$_yaml_ip" != "localhost" ]] && ! is_loopback_ip "$_yaml_ip"; then
      _SCYLLA_YAML_OK=true
    fi
  fi
  if [[ "${_SCYLLA_YAML_OK}" == "false" ]]; then
    log_substep "scylla.yaml missing or has loopback address — reinstalling scylladb package to regenerate..."
    systemctl stop scylla-server.service 2>/dev/null || true
    _scylla_pkg=$(_resolve_scylladb_pkg || true)
    if [[ -n "${_scylla_pkg}" ]] && [[ -f "${_scylla_pkg}" ]]; then
      rm -f /etc/scylla/scylla.yaml
      export SCYLLA_INSTALL_INTENT="initial-node"
      export SCYLLA_BOOTSTRAP_INTENT="first-node"
      run_install "${_scylla_pkg}"
      unset SCYLLA_INSTALL_INTENT SCYLLA_BOOTSTRAP_INTENT
      log_success "scylla.yaml regenerated"
    else
      die "scylla.yaml invalid and no bundled scylladb package found to fix it"
    fi
  fi

  # Wait for ScyllaDB to be ready
  if ! systemctl is-active --quiet scylla-server.service; then
    systemctl start scylla-server.service || log_substep "Warning: failed to start scylla-server"
  fi
  SCYLLA_CQL_HOST=$(scylla_cql_host)
  log_substep "Waiting for ScyllaDB CQL port (${SCYLLA_CQL_HOST}:9042)..."
  SCYLLA_READY=0
  for i in $(seq 1 90); do
    if cqlsh "$SCYLLA_CQL_HOST" 9042 -e "SELECT now() FROM system.local" &>/dev/null; then
      SCYLLA_READY=1
      break
    fi
    sleep 1
  done
  if [[ $SCYLLA_READY -eq 1 ]]; then
    log_success "ScyllaDB ready (took ${i}s)"
    wait_scylla_write_ready "$SCYLLA_CQL_HOST"
  else
    echo "" >&2
    echo "━━━ scylla-server journal (last 20 lines) ━━━" >&2
    journalctl -xeu scylla-server.service --no-pager -n 20 >&2 || true
    die "ScyllaDB not accepting CQL on ${SCYLLA_CQL_HOST}:9042 after 90s"
  fi
else
  log_substep "ScyllaDB not found — installing..."

  # Pre-install: wipe stale ScyllaDB state from a previous failed Day-0 attempt.
  # The ownership file is only written after a SUCCESSFUL post-install. If it
  # doesn't exist but scylla data does, the last install failed — safe to wipe.
  # This runs unconditionally in install-day0.sh so post-install always starts
  # clean, regardless of which package version's post-install is bundled.
  _SCYLLA_OWNERSHIP="/var/lib/globular/state/scylladb/ownership.json"
  if ! systemctl is-active --quiet scylla-server.service 2>/dev/null; then
    if [[ -d /var/lib/scylla/data ]] && [[ ! -f "$_SCYLLA_OWNERSHIP" ]]; then
      log_substep "Wiping stale ScyllaDB data from previous failed bootstrap..."
      systemctl stop scylla-server.service 2>/dev/null || true
      rm -rf /var/lib/scylla/data /var/lib/scylla/commitlog \
             /var/lib/scylla/hints /var/lib/scylla/view_hints \
             /etc/scylla/scylla.yaml
      log_success "Stale state cleared"
    fi
  fi

  # Install the ScyllaDB Globular package via bundled .deb files (no internet needed).
  # Falls back to direct apt install only when no scylladb_*.tgz is found anywhere.
  if [[ -n "${SCYLLADB_PKG_PATH:-}" ]] && [[ -f "$SCYLLADB_PKG_PATH" ]]; then
    log_substep "Using bundled package: $(basename "$SCYLLADB_PKG_PATH")"
    # Always wipe scylla.yaml so post-install writes a fresh one with the correct
    # node IP. A stale yaml from a previous failed attempt may have listen_address
    # and rpc_address set to 'localhost', causing CQL to bind on 127.0.0.1 only.
    rm -f /etc/scylla/scylla.yaml
    export SCYLLA_INSTALL_INTENT="initial-node"
    export SCYLLA_BOOTSTRAP_INTENT="first-node"
    run_install "$SCYLLADB_PKG_PATH"
    remember_seed_expected_pkg "$SCYLLADB_PKG_PATH"
    unset SCYLLA_INSTALL_INTENT SCYLLA_BOOTSTRAP_INTENT
  else
    log_substep "Warning: no scylladb package found in $PKG_DIR or /var/lib/globular/packages, attempting direct apt install..."
    # Only import GPG key and configure apt repo when falling back to direct apt install
    mkdir -p /etc/apt/keyrings
    if [[ ! -f /etc/apt/keyrings/scylladb.gpg ]]; then
      log_substep "Importing ScyllaDB GPG key (A43E06657BAC99E3)..."
      # Import via keyserver — the old URL-based .gpg download is no longer published
      # by ScyllaDB. Using the Ubuntu keyserver with the known key ID is stable.
      gpg --homedir /tmp --no-default-keyring \
          --keyring /etc/apt/keyrings/scylladb.gpg \
          --keyserver hkp://keyserver.ubuntu.com \
          --recv-keys A43E06657BAC99E3
      chmod 644 /etc/apt/keyrings/scylladb.gpg
      log_success "ScyllaDB GPG key imported"
    fi
    if [[ ! -f /etc/apt/sources.list.d/scylla.list ]]; then
      echo "deb [arch=amd64,arm64 signed-by=/etc/apt/keyrings/scylladb.gpg] https://downloads.scylladb.com/downloads/scylla/deb/debian-ubuntu/scylladb-2025.3 stable main" \
        > /etc/apt/sources.list.d/scylla.list
    fi
    apt-get update -qq
    DEBIAN_FRONTEND=noninteractive apt-get install -y -qq scylla scylla-server scylla-conf scylla-cqlsh scylla-python3
  fi

  # TLS, data dirs, and scylla.yaml are configured by the scylladb package
  # post-install script when present. Fallback to external script for packages
  # built without embedded scripts.
  if [[ -f /etc/scylla/tls/server.crt ]] && [[ -f /etc/scylla/scylla.yaml ]]; then
    log_substep "ScyllaDB TLS/config already configured (by package post-install)"
  elif [[ -x "$SCRIPT_DIR/setup-scylla-tls.sh" ]]; then
    log_substep "Configuring ScyllaDB (TLS, data dirs, scylla.yaml)..."
    "$SCRIPT_DIR/setup-scylla-tls.sh" || die "ScyllaDB TLS setup failed"
    log_success "ScyllaDB configured and started"
  else
    die "ScyllaDB TLS not configured and setup-scylla-tls.sh not found"
  fi

  # Enable service for boot
  systemctl enable scylla-server.service 2>/dev/null || true

  # ScyllaDB MUST be running before continuing. Downstream services (persistence,
  # scylla-manager) all require CQL — a non-running scylla is a hard failure.
  if ! systemctl is-active --quiet scylla-server.service; then
    log_substep "Starting ScyllaDB service..."
    if ! systemctl start scylla-server.service 2>/dev/null; then
      echo "" >&2
      echo "━━━ scylla-server journal (last 40 lines) ━━━" >&2
      journalctl -xeu scylla-server.service --no-pager -n 40 >&2 || true
      echo "━━━ /etc/scylla/scylla.yaml ━━━" >&2
      cat /etc/scylla/scylla.yaml 2>/dev/null >&2 || echo "(not found)" >&2
      die "ScyllaDB service failed to start — see diagnostics above"
    fi
  fi

  # Wait for ScyllaDB to accept CQL (can take 30-90s on first start).
  SCYLLA_CQL_HOST=$(scylla_cql_host)
  log_substep "Waiting for ScyllaDB to accept CQL connections (${SCYLLA_CQL_HOST}:9042)..."
  SCYLLA_READY=0
  for i in $(seq 1 90); do
    if cqlsh "$SCYLLA_CQL_HOST" 9042 -e "SELECT now() FROM system.local" &>/dev/null; then
      SCYLLA_READY=1
      break
    fi
    sleep 1
  done
  if [[ $SCYLLA_READY -eq 1 ]]; then
    log_success "ScyllaDB installed and ready (took ${i}s)"
    wait_scylla_write_ready "$SCYLLA_CQL_HOST"
  else
    echo "" >&2
    echo "━━━ scylla-server journal (last 20 lines) ━━━" >&2
    journalctl -xeu scylla-server.service --no-pager -n 20 >&2 || true
    die "ScyllaDB not accepting CQL on ${SCYLLA_CQL_HOST}:9042 after 90s"
  fi
fi

# TLS ownership fix: certs were generated as root during TLS Bootstrap but
# infrastructure services (etcd, gateway, etc.) run as the globular user.
# Fix ownership NOW, before any globular-user service tries to read them.
if id globular >/dev/null 2>&1 && [[ -d /var/lib/globular/pki ]]; then
  chown -R globular:globular /var/lib/globular/pki
  # chown -R resets directory execute bits via root's umask — restore them so
  # globular and world can traverse into pki/ subdirs (issued/, issued/services/).
  chmod o+rx /var/lib/globular /var/lib/globular/pki 2>/dev/null || true
  find /var/lib/globular/pki -type d -exec chmod o+rx {} + 2>/dev/null || true
  log_substep "TLS files ownership set to globular:globular (pre-infra)"
fi

log_step "Infrastructure Layer (etcd + minio)"
trace_step "running" "phase.infra" "Infrastructure Layer" 5
install_list "${BOOTSTRAP_MINIO_PKGS[@]}"

# If the user chose a custom MinIO data directory, ensure the directory exists.
# Do NOT patch the installed systemd unit or env file here: the installer
# already renders both from the authoritative {{.MinioDataDir}} template input.
# Post-install `sed -i` rewrites poison the canonical installed_state receipt
# and create permanent unit_file_drift findings for globular-minio.service.
DEFAULT_MINIO_PATH="/var/lib/globular/minio/data"
if [[ "$MINIO_DATA_DIR" != "$DEFAULT_MINIO_PATH" ]]; then
  log_substep "Using custom MinIO data directory: $MINIO_DATA_DIR"

  # Create the custom data directory
  mkdir -p "$MINIO_DATA_DIR"
  chown globular:globular "$MINIO_DATA_DIR"
  chmod 0700 "$MINIO_DATA_DIR"
  log_substep "Created $MINIO_DATA_DIR"

  log_success "MinIO data directory prepared at $MINIO_DATA_DIR"
fi

log_step "TLS Ownership Fix"
log_substep "Setting TLS file ownership to globular user..."
if id globular >/dev/null 2>&1; then
  # Create backup data directory outside the cluster dir so backups survive uninstall/disk failure
  mkdir -p /var/backups/globular
  chown globular:globular /var/backups/globular
  log_success "Backup data directory created at /var/backups/globular"

  # INV-PKI-1: Use canonical PKI paths only
  chown -R globular:globular /var/lib/globular/pki /var/lib/globular/.minio 2>/dev/null || true
  log_success "TLS files ownership set to globular:globular"

  # Allow the gateway to read systemd journal (needed for journal.ReadUnit API)
  if getent group systemd-journal >/dev/null 2>&1; then
    usermod -aG systemd-journal globular
    log_success "globular user added to systemd-journal group"
  fi

  # Allow scylla-manager-agent (running as globular) to manage ScyllaDB snapshots
  if getent group scylla >/dev/null 2>&1; then
    usermod -aG scylla globular
    # Set default ACLs so new snapshot files/dirs are group-writable by scylla group
    if command -v setfacl >/dev/null 2>&1 && [[ -d /var/lib/scylla/data ]]; then
      setfacl -R -m g:scylla:rwX /var/lib/scylla/data
      setfacl -R -d -m g:scylla:rwX /var/lib/scylla/data
    fi
    log_success "globular user added to scylla group (snapshot management)"
  fi

  # Sudoers: allow globular user to manage Globular services and restore operations.
  # The backup-manager runs as globular and needs to:
  # - Restart all services after restore (re-register etcd ports, fix node reachability)
  # - Stop/start ScyllaDB workload services during schema restore
  # - Fix sstable upload dir ownership for ScyllaDB restore
  log_substep "Installing sudoers rules for globular user..."
  cat > /etc/sudoers.d/globular <<'SUDOERS'
# Manage any globular-* systemd service.
# Needed by: backup-manager (post-restore restarts), node-agent (stop/start
# workload services during ScyllaDB schema restore, etcd stop/start).
globular ALL=(root) NOPASSWD: /usr/bin/systemctl stop globular-*.service
globular ALL=(root) NOPASSWD: /usr/bin/systemctl start globular-*.service
globular ALL=(root) NOPASSWD: /usr/bin/systemctl restart globular-*.service

# ScyllaDB sstable upload dir ownership fix during restore (safety net).
# The scylla-manager-agent runs as scylla so this rarely fires, but guards
# against edge cases where files end up with wrong ownership.
globular ALL=(root) NOPASSWD: /usr/bin/find /var/lib/scylla/data *
globular ALL=(root) NOPASSWD: /usr/bin/bash -c find /var/lib/scylla/data *
SUDOERS
  chmod 0440 /etc/sudoers.d/globular
  log_success "Sudoers rules installed for globular user"

  # Restart services that depend on TLS certificates
  log_substep "Restarting services to apply TLS ownership changes..."
  systemctl restart globular-etcd.service 2>/dev/null || true
  systemctl restart globular-minio.service 2>/dev/null || true
  sleep 3  # Wait for services to restart with correct cert permissions
  log_success "Services restarted with correct TLS ownership"
else
  log_substep "Warning: globular user not found, skipping ownership fix"
fi

log_step "MinIO Configuration"
# Contract, credentials, and TLS symlinks are handled by the minio package
# pre-start.sh script when present. Fallback to external script for packages
# built without embedded scripts.
if [[ -f /var/lib/globular/objectstore/minio.json ]]; then
  log_success "MinIO contract configured (by package pre-start script)"
elif [[ -x "$SCRIPT_DIR/setup-minio-contract.sh" ]]; then
  "$SCRIPT_DIR/setup-minio-contract.sh"
  log_success "MinIO contract configured (fallback)"
else
  die "MinIO contract not found and setup-minio-contract.sh not available"
fi

# Verify TLS symlinks exist (created by pre-start.sh or setup-tls.sh).
# Without these, MinIO runs in HTTP mode — which breaks the HTTPS-only cluster.
if [[ ! -L /var/lib/globular/.minio/certs/public.crt ]]; then
  log_substep "Warning: MinIO TLS cert symlink missing — MinIO may be running in HTTP mode"
  log_substep "Expected: /var/lib/globular/.minio/certs/public.crt → PKI service cert"
fi

log_substep "Verifying MinIO systemd unit..."
MINIO_UNIT="/etc/systemd/system/globular-minio.service"
if [[ ! -f "$MINIO_UNIT" ]]; then
  die "MinIO unit not installed at $MINIO_UNIT"
fi
if grep -q "{{" "$MINIO_UNIT"; then
  die "MinIO unit contains unrendered template placeholders"
fi
if ! systemd-analyze verify "$MINIO_UNIT" 2>&1 | grep -v "Transaction order is cyclic" > /dev/null; then
  : # Ignore systemd-analyze errors (they're often spurious)
fi

# Ensure MinIO is running. The installer's start_services step already started
# it if the package had a spec with that step, but handle the case where the
# installer didn't start it (old packages or install failures).
log_substep "Ensuring MinIO service is running..."
systemctl daemon-reload
if ! systemctl is-active --quiet globular-minio.service; then
  systemctl start globular-minio.service || die "Failed to start MinIO service"
  log_success "MinIO service started"
else
  log_success "MinIO service already running"
fi

log_step "CLI Tools (needed for bucket provisioning)"
install_list "${COMMON_CMDS_PKGS[@]}"
if has_profile "media-server"; then
  install_list "${MEDIA_CMDS_PKGS[@]}"
fi

# Seed etcd Tier-0 keys so services that cannot use DNS can find infrastructure.
# These keys must be written BEFORE the cluster controller starts, which reads
# them during initProjections and publishMinioConfigLocked.
log_step "Seed Tier-0 etcd keys (ScyllaDB hosts + MinIO config)"
_NODE_IP_LOCAL=$(hostname -I | awk '{print $1}')
_ETCD_ENDPOINTS="${ETCD_ENDPOINTS:-https://${_NODE_IP_LOCAL}:2379}"
_CA_CERT="/var/lib/globular/pki/ca.crt"
_CERT="/var/lib/globular/pki/issued/services/service.crt"
_KEY="/var/lib/globular/pki/issued/services/service.key"

# --- ScyllaDB hosts ---
# Detect ScyllaDB listen IP from scylla.yaml (same logic as the readiness check above).
_SCYLLA_IP=$(grep "^listen_address:" /etc/scylla/scylla.yaml 2>/dev/null | awk '{print $2}' | tr -d "'\"" || true)
if [[ -z "$_SCYLLA_IP" ]]; then
  _SCYLLA_IP="$_NODE_IP_LOCAL"
fi
if [[ -n "$_SCYLLA_IP" ]] && ! is_loopback_ip "$_SCYLLA_IP"; then
  if etcdctl --endpoints="$_ETCD_ENDPOINTS" \
      --cacert="$_CA_CERT" --cert="$_CERT" --key="$_KEY" \
      put "/globular/cluster/scylla/hosts" "[\"$_SCYLLA_IP\"]" >/dev/null 2>&1; then
    log_success "ScyllaDB hosts seeded in etcd: [$_SCYLLA_IP]"
  else
    log_substep "Warning: could not seed ScyllaDB hosts in etcd (will retry at runtime)"
  fi
else
  log_substep "Warning: could not determine ScyllaDB listen IP, skipping scylla hosts seed"
fi

# --- MinIO config ---
# Read credentials from the credentials file (written by setup-minio-contract.sh).
# This ensures the cluster controller uses the same credentials MinIO was initialized with.
_MINIO_CRED_FILE="/var/lib/globular/minio/credentials"
if [[ -f "$_MINIO_CRED_FILE" ]]; then
  _MINIO_AK=$(cut -d: -f1 "$_MINIO_CRED_FILE")
  _MINIO_SK=$(cut -d: -f2- "$_MINIO_CRED_FILE")
  if [[ -n "$_MINIO_AK" && -n "$_MINIO_SK" ]]; then
    _MINIO_ETCD_VAL="{\"endpoint\":\"minio.${DOMAIN}:9000\",\"access_key\":\"$_MINIO_AK\",\"secret_key\":\"$_MINIO_SK\",\"secure\":true,\"bucket\":\"globular\",\"prefix\":\"${DOMAIN}\",\"webroot_dir\":\"webroot\"}"
    if etcdctl --endpoints="$_ETCD_ENDPOINTS" \
        --cacert="$_CA_CERT" --cert="$_CERT" --key="$_KEY" \
        put "/globular/cluster/minio/config" "$_MINIO_ETCD_VAL" >/dev/null 2>&1; then
      log_success "MinIO config seeded in etcd (access_key=$_MINIO_AK endpoint=minio.${DOMAIN}:9000)"
    else
      log_substep "Warning: could not seed MinIO config in etcd (will be written by cluster controller)"
    fi
  fi
else
  log_substep "Warning: MinIO credentials file not found — MinIO config not seeded"
fi

log_step "MinIO Bucket Provisioning"
# On new packages, the post-install.sh script already created buckets during
# install_list above. The external scripts are idempotent — safe to re-run.
# On old packages (no embedded scripts), these are the primary bucket creators.
if [[ -x "$SCRIPT_DIR/ensure-minio-buckets.sh" ]]; then
  "$SCRIPT_DIR/ensure-minio-buckets.sh"
  log_success "MinIO buckets provisioned"
else
  log_substep "ensure-minio-buckets.sh not found — buckets handled by package post-install"
fi

log_step "Cluster Config (shared via MinIO)"
# Create the cluster config bucket and upload critical shared files.
# These are available to all nodes via MinIO — survives any single node loss.
MC_BIN="/usr/local/bin/mc"
MINIO_ALIAS="local"
if [[ -x "$MC_BIN" ]]; then
  # Read credentials from the canonical credentials file (written by setup-minio-contract.sh).
  # minio.json stores auth.mode=file — the AccessKey/SecretKey fields are NOT directly in it.
  # Fallback to default credentials only if the file is missing.
  MINIO_ENDPOINT="https://$(hostname -I | awk '{print $1}'):9000"
  _CRED_FILE="/var/lib/globular/minio/credentials"
  if [[ -f "$_CRED_FILE" ]]; then
    MINIO_ACCESS="$(cut -d: -f1 "$_CRED_FILE")"
    MINIO_SECRET="$(cut -d: -f2- "$_CRED_FILE")"
  else
    MINIO_ACCESS="globular"
    MINIO_SECRET="globularadmin"
  fi
  # Trust Globular CA in mc so MinIO TLS is verified without --insecure.
  mkdir -p ~/.mc/certs/CAs
  cp /var/lib/globular/pki/ca.crt ~/.mc/certs/CAs/globular-ca.crt 2>/dev/null || true

  if "$MC_BIN" alias set "$MINIO_ALIAS" "$MINIO_ENDPOINT" "$MINIO_ACCESS" "$MINIO_SECRET" 2>/dev/null; then
    log_substep "mc alias configured (user=$MINIO_ACCESS)"
  else
    log_substep "Warning: mc alias set failed — cluster config upload skipped"
  fi

  # Create config bucket.
  "$MC_BIN" mb --ignore-existing "${MINIO_ALIAS}/globular-config" 2>/dev/null || true

  # Upload CA certificate and key (cluster-wide PKI).
  if [[ -f /var/lib/globular/pki/ca.pem ]]; then
    "$MC_BIN" cp /var/lib/globular/pki/ca.pem "${MINIO_ALIAS}/globular-config/pki/ca.pem" 2>/dev/null && \
      log_success "CA certificate uploaded to MinIO (cluster-shared)" || \
      log_substep "Warning: CA cert upload to MinIO failed (non-fatal)"
  fi
  if [[ -f /var/lib/globular/pki/ca.key ]]; then
    "$MC_BIN" cp /var/lib/globular/pki/ca.key "${MINIO_ALIAS}/globular-config/pki/ca.key" 2>/dev/null && \
      log_success "CA key uploaded to MinIO (cluster-shared)" || \
      log_substep "Warning: CA key upload to MinIO failed (non-fatal)"
  fi

  # Upload RBAC cluster roles if present.
  if [[ -f /var/lib/globular/policy/rbac/cluster-roles.json ]]; then
    "$MC_BIN" cp /var/lib/globular/policy/rbac/cluster-roles.json \
      "${MINIO_ALIAS}/globular-config/policy/rbac/cluster-roles.json" 2>/dev/null || true
    log_success "RBAC policies uploaded to MinIO"
  fi

  # Upload AI operational rules (CLUSTER_CLAUDE.md → ai/CLAUDE.md in MinIO).
  # The ai_executor reads this via config.GetClusterConfig("ai/CLAUDE.md").
  CLAUDE_MD="${SCRIPT_DIR}/CLUSTER_CLAUDE.md"
  if [[ -f "$CLAUDE_MD" ]]; then
    "$MC_BIN" cp "$CLAUDE_MD" "${MINIO_ALIAS}/globular-config/ai/CLAUDE.md" 2>/dev/null && \
      log_success "AI operational rules uploaded to MinIO (cluster-shared)" || \
      log_substep "Warning: CLAUDE.md upload to MinIO failed (non-fatal)"
  fi
else
  log_substep "Warning: mc not found — cluster config sharing deferred"
fi

log_step "Data Layer (persistence)"
install_list "${DATA_LAYER_PKGS[@]}"

log_step "MinIO Bucket Setup"
if [[ -x "$SCRIPT_DIR/setup-minio.sh" ]]; then
  "$SCRIPT_DIR/setup-minio.sh"
  log_success "MinIO buckets configured"
else
  log_substep "setup-minio.sh not found — bucket setup handled by package post-install"
fi

# Ensure local webroot exists immediately on Day-0.
# Gateway serves /var/lib/globular/webroot directly, while webroot-sync may run
# later. Materialize the bundled assets now to avoid first-boot 404s.
log_step "Webroot Local Materialization"
WEBROOT_DST="/var/lib/globular/webroot"
WEBROOT_SRC=""
for _wr in \
    "$INSTALLER_ROOT/webroot" \
    "$SCRIPT_DIR/../webroot" \
    "$SCRIPT_DIR/../../webroot" \
    "/usr/lib/globular/webroot" \
    "/opt/globular/webroot"; do
  if [[ -f "$_wr/index.html" ]]; then
    WEBROOT_SRC="$_wr"
    break
  fi
done
if [[ -z "$WEBROOT_SRC" ]]; then
  die "No bundled webroot found (expected index.html under installer webroot paths)"
fi
mkdir -p "$WEBROOT_DST"
cp -a "${WEBROOT_SRC}/." "$WEBROOT_DST/"
if [[ ! -f "${WEBROOT_DST}/logo.png" ]]; then
  for _logo in "$INSTALLER_ROOT/assets/logo.png" "$SCRIPT_DIR/../assets/logo.png" "$SCRIPT_DIR/../../assets/logo.png"; do
    if [[ -f "$_logo" ]]; then
      cp -f "$_logo" "${WEBROOT_DST}/logo.png"
      break
    fi
  done
fi
find "$WEBROOT_DST" -type d -exec chmod 755 {} \; 2>/dev/null || true
find "$WEBROOT_DST" -type f -exec chmod 644 {} \; 2>/dev/null || true
if id globular >/dev/null 2>&1; then
  chown -R globular:globular "$WEBROOT_DST" 2>/dev/null || true
fi
[[ -f "${WEBROOT_DST}/index.html" ]] || die "webroot materialization failed: missing ${WEBROOT_DST}/index.html"
[[ -f "${WEBROOT_DST}/logo.png" ]] || die "webroot materialization failed: missing ${WEBROOT_DST}/logo.png"
log_success "Local webroot ready at ${WEBROOT_DST} (source: ${WEBROOT_SRC})"

# Ensure gateway serves with canonical cluster identity from Day-0 onward.
# Older/default package configs may keep Domain=localhost and Protocol=http.
log_step "Gateway Runtime Config"
GATEWAY_CFG="/var/lib/globular/services/gateway/config.json"
if [[ -f "$GATEWAY_CFG" ]]; then
  _GW_DOMAIN="$(hostname).${DOMAIN}"
  if python3 - "$GATEWAY_CFG" "$_GW_DOMAIN" <<'PY'
import json, sys
p, domain = sys.argv[1], sys.argv[2]
with open(p, "r", encoding="utf-8") as f:
    d = json.load(f)
changed = False
if d.get("Domain") != domain:
    d["Domain"] = domain
    changed = True
if str(d.get("Protocol", "")).lower() != "https":
    d["Protocol"] = "https"
    changed = True
if changed:
    with open(p, "w", encoding="utf-8") as f:
        json.dump(d, f, indent=2)
        f.write("\n")
print("changed" if changed else "unchanged")
PY
  then
    log_substep "Gateway config normalized (domain=${_GW_DOMAIN}, protocol=https)"
    systemctl restart globular-gateway.service 2>/dev/null || true
  else
    log_substep "Warning: failed to normalize gateway config"
  fi
else
  log_substep "Gateway config not found at $GATEWAY_CFG yet; expected until gateway writes its runtime config"
fi

# ── Workflow definitions (always required) ────────────────────────────────
# Copy workflow YAML files to /var/lib/globular/workflows/ unconditionally.
# The cluster controller reads these at startup to seed etcd. Without them
# the cluster cannot reconcile or deploy packages.
log_step "Workflow Definitions"

WORKFLOW_DEFS_SRC="${SCRIPT_DIR}/../workflows"
if [[ ! -d "$WORKFLOW_DEFS_SRC" ]]; then
  WORKFLOW_DEFS_SRC="${SCRIPT_DIR}/../../services/golang/workflow/definitions"
fi
if [[ -d "$WORKFLOW_DEFS_SRC" ]]; then
  mkdir -p /var/lib/globular/workflows
  cp "$WORKFLOW_DEFS_SRC"/*.yaml /var/lib/globular/workflows/
  chown -R globular:globular /var/lib/globular/workflows 2>/dev/null || true
  log_success "Workflow definitions deployed to /var/lib/globular/workflows/ ($(ls "$WORKFLOW_DEFS_SRC"/*.yaml | wc -l) files)"
else
  log_warn "Workflow definitions not found — controller cannot seed etcd on startup"
  log_warn "Manually copy *.yaml files to /var/lib/globular/workflows/ before starting"
fi

# ── Workflow-driven installation ─────────────────────────────────────────
# If USE_WORKFLOW=1, install the node-agent and delegate all remaining
# package installation to the day0.bootstrap workflow.
USE_WORKFLOW="${USE_WORKFLOW:-0}"
if [[ "$USE_WORKFLOW" == "1" ]]; then
  log_step "Workflow-Driven Bootstrap"

  # Install node-agent (the workflow runner).
  NODE_AGENT_PKG="$PKG_DIR/node-agent_0.0.1_linux_amd64.tgz"
  if [[ -f "$NODE_AGENT_PKG" ]]; then
    run_install "$NODE_AGENT_PKG"
  else
    die "node-agent package not found at $NODE_AGENT_PKG"
  fi

  # Copy all .tgz packages to the local fallback directory so the workflow
  # can install them without needing the repository service (which isn't
  # running yet during Day-0).
  log_substep "Staging packages for local install..."
  mkdir -p /var/lib/globular/packages
  cp "$PKG_DIR"/*.tgz /var/lib/globular/packages/ 2>/dev/null || true
  chown -R globular:globular /var/lib/globular/packages 2>/dev/null || true
  PKG_COUNT=$(ls /var/lib/globular/packages/*.tgz 2>/dev/null | wc -l)
  log_success "$PKG_COUNT packages staged in /var/lib/globular/packages/"

  # Globular configuration (Protocol=https) — needed before node-agent starts.
  if [[ -x "$SCRIPT_DIR/setup-config.sh" ]]; then
    "$SCRIPT_DIR/setup-config.sh"
    log_success "Configuration set to HTTPS"
  fi

  # Start the node-agent.
  log_substep "Starting node-agent..."
  systemctl enable globular-node-agent.service 2>/dev/null || true
  systemctl start globular-node-agent.service || die "Failed to start node-agent"

  # Resolve the node-agent's actual port from the installed systemd unit.
  # Never hardcode 11000 — the port lives in the unit file, not in this script.
  _NA_IP=$(ip route get 8.8.8.8 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src") print $(i+1); exit}')
  _NA_IP="${_NA_IP:-$(hostname -I | awk '{print $1}')}"
  _NA_PORT=$(grep -oP '(?<=--port[= ])\d+' /etc/systemd/system/globular-node-agent.service 2>/dev/null | head -1 || true)
  _NA_PORT="${_NA_PORT:-$(grep -oP '(?<=--port[= ])\d+' /etc/systemd/system/globular-node-agent.service.d/*.conf 2>/dev/null | head -1 || true)}"
  [[ -n "$_NA_PORT" ]] || die "Could not determine node-agent port from installed systemd unit"

  # Wait for node-agent to be ready on its routable IP.
  log_substep "Waiting for node-agent to be ready on ${_NA_IP}:${_NA_PORT}..."
  for i in $(seq 1 30); do
    if timeout 2 bash -c "echo >/dev/tcp/${_NA_IP}/${_NA_PORT}" 2>/dev/null; then
      log_success "Node-agent ready on ${_NA_IP}:${_NA_PORT}"
      break
    fi
    if [[ $i -eq 30 ]]; then
      die "Node-agent not ready after 60 seconds"
    fi
    sleep 2
  done

  # Detect local hostname and IP for workflow inputs.
  NODE_HOSTNAME="$(hostname)"
  NODE_IP="$(ip route get 8.8.8.8 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src") print $(i+1); exit}')"
  NODE_IP="${NODE_IP:-$(hostname -I 2>/dev/null | awk '{print $1}')}"

  # Trigger the day0.bootstrap workflow via gRPC to the local node-agent.
  # The node-agent uses TLS with the cluster CA.
  CA_CERT="/var/lib/globular/pki/ca.crt"
  SVC_CERT="/var/lib/globular/pki/issued/services/service.crt"
  SVC_KEY="/var/lib/globular/pki/issued/services/service.key"

  NODE_ID="$(cat /var/lib/globular/nodeagent/node_id 2>/dev/null || echo 'bootstrap')"

  BOOTSTRAP_NODE_PROFILES_JSON=$(printf '%s' "$FOUNDING_PROFILES" \
    | jq -Rc 'split(",") | map(gsub("^\\s+|\\s+$";"")) | map(select(length>0))')
  if [[ -z "$BOOTSTRAP_NODE_PROFILES_JSON" || "$BOOTSTRAP_NODE_PROFILES_JSON" == "[]" ]]; then
    BOOTSTRAP_NODE_PROFILES_JSON='["core"]'
  fi

  GRPC_REQUEST=$(jq -cn \
    --arg workflow_name "day0.bootstrap" \
    --arg cluster_id "$DOMAIN" \
    --arg bootstrap_node_id "$NODE_ID" \
    --arg bootstrap_node_hostname "$NODE_HOSTNAME" \
    --arg domain "$DOMAIN" \
    --argjson bootstrap_node_profiles "$BOOTSTRAP_NODE_PROFILES_JSON" \
    '{
      workflow_name: $workflow_name,
      inputs: {
        cluster_id: $cluster_id,
        bootstrap_node_id: $bootstrap_node_id,
        bootstrap_node_hostname: $bootstrap_node_hostname,
        domain: $domain,
        bootstrap_node_profiles: $bootstrap_node_profiles
      }
    }')

  log_substep "Triggering day0.bootstrap workflow..."
  log_substep "  Request: $GRPC_REQUEST"

  GRPCURL="$(command -v grpcurl 2>/dev/null || true)"
  if [[ -n "$GRPCURL" ]]; then
    "$GRPCURL" -insecure \
      -cert "$SVC_CERT" -key "$SVC_KEY" \
      -d "$GRPC_REQUEST" \
      -max-time 1800 \
      "${_NA_IP}:${_NA_PORT}" node_agent.NodeAgentService/RunWorkflow 2>&1 | while IFS= read -r line; do
        echo "  [workflow] $line"
      done
    WORKFLOW_RC=${PIPESTATUS[0]}
  else
    # No grpcurl — try the globular CLI with a RunWorkflow-equivalent command.
    GLOBULAR_CLI="/usr/local/bin/globular"
    if [[ ! -x "$GLOBULAR_CLI" ]]; then
      GLOBULAR_CLI="/usr/lib/globular/bin/globular"
    fi
    if [[ ! -x "$GLOBULAR_CLI" ]]; then
      GLOBULAR_CLI="$(command -v globular 2>/dev/null || true)"
    fi
    if [[ -n "$GLOBULAR_CLI" ]] && [[ -x "$GLOBULAR_CLI" ]]; then
      "$GLOBULAR_CLI" --insecure --timeout 1800s workflow run day0.bootstrap \
        --node "${_NA_IP}:${_NA_PORT}" 2>&1 | while IFS= read -r line; do
          echo "  [workflow] $line"
        done
      WORKFLOW_RC=${PIPESTATUS[0]}
    else
      die "Neither grpcurl nor globular CLI available to trigger workflow"
    fi
  fi

  if [[ "${WORKFLOW_RC:-1}" -ne 0 ]]; then
    log_substep "Warning: Workflow returned non-zero exit code: ${WORKFLOW_RC}"
    log_substep "Falling back to manual installation..."
    USE_WORKFLOW=0
    # Fall through to manual installation below.
  else
    log_success "Day-0 bootstrap workflow completed successfully"
    trace_finish "ok" "Day-0 installation via workflow completed"
    exit 0
  fi
fi

# ── Manual Installation (legacy or workflow fallback) ────────────────────
# This path runs if USE_WORKFLOW=0 or if the workflow failed and fell back.

log_step "Globular Configuration (Protocol=https)"
if [[ -x "$SCRIPT_DIR/setup-config.sh" ]]; then
  "$SCRIPT_DIR/setup-config.sh"
  log_success "Configuration set to HTTPS"

  # Ensure network.json has https protocol and correct permissions.
  # network.json may exist from a previous run with Protocol=http.
  if [[ -f /var/lib/globular/network.json ]]; then
    CURRENT_NET_PROTO=$(jq -r '.Protocol // "http"' /var/lib/globular/network.json 2>/dev/null)
    if [[ "$CURRENT_NET_PROTO" != "https" ]]; then
      jq '.Protocol = "https"' /var/lib/globular/network.json > /var/lib/globular/network.json.tmp \
        && mv /var/lib/globular/network.json.tmp /var/lib/globular/network.json
      log_substep "Updated network.json Protocol to https"
    fi
    chmod 644 /var/lib/globular/network.json
    log_substep "Set network.json permissions to 644"
  else
    cat > /var/lib/globular/network.json <<EOF
{
  "Domain": "${DOMAIN}",
  "Protocol": "https",
  "PortHTTP": 80,
  "PortHTTPS": 443
}
EOF
    chmod 644 /var/lib/globular/network.json
    log_substep "Materialized bootstrap network.json for local runtime checks"
  fi

  # CRITICAL: Regenerate client certificates now that domain is configured
  # Initial certs were generated before config.json had the final cluster domain.
  log_substep "Regenerating client certificates with configured domain..."

  # Regenerate root client certificates
  if "$SCRIPT_DIR/generate-user-client-cert.sh" root >/dev/null 2>&1; then
    log_substep "Root client certificates regenerated for configured domain"
  fi

  # Regenerate user client certificates if we have a detected user
  if [[ -n "${ORIGINAL_USER:-}" ]] && [[ "$ORIGINAL_USER" != "root" ]]; then
    if "$SCRIPT_DIR/generate-user-client-cert.sh" "$ORIGINAL_USER" >/dev/null 2>&1; then
      if [[ -x "$SCRIPT_DIR/fix-client-cert-ownership.sh" ]]; then
        "$SCRIPT_DIR/fix-client-cert-ownership.sh" "$ORIGINAL_USER" >/dev/null 2>&1 || true
      fi
      log_substep "User ($ORIGINAL_USER) client certificates regenerated for configured domain"
    fi
  fi
else
  log_substep "Warning: setup-config.sh not found (Protocol may default to HTTP)"
fi

log_step "Bootstrap Services (xds, envoy, gateway, agents)"
install_list "${BOOTSTRAP_REST_PKGS[@]}"

# Explicitly ensure cluster-doctor is installed and running (common omission)
CLUSTER_DOCTOR_PKG="$(resolve_pkg_artifact "cluster-doctor_0.0.1_linux_amd64.tgz" || true)"
if [[ -n "$CLUSTER_DOCTOR_PKG" ]] && [[ -f "$CLUSTER_DOCTOR_PKG" ]]; then
  systemctl daemon-reload >/dev/null 2>&1 || true
  if systemctl cat globular-cluster-doctor.service >/dev/null 2>&1 || \
     systemctl status globular-cluster-doctor.service >/dev/null 2>&1; then
    log_substep "cluster-doctor unit present"
  else
    log_substep "cluster-doctor unit missing after daemon-reload; reinstalling from package..."
    run_install "$CLUSTER_DOCTOR_PKG"
    systemctl daemon-reload >/dev/null 2>&1 || true
  fi

  if ! systemctl is-active --quiet globular-cluster-doctor.service 2>/dev/null; then
    log_substep "Starting globular-cluster-doctor.service..."
    systemctl enable globular-cluster-doctor.service >/dev/null 2>&1 || true
    systemctl start globular-cluster-doctor.service || log_substep "Warning: failed to start cluster-doctor (check logs)"
  fi
else
  log_substep "Warning: cluster-doctor package artifact not found in bundled packages"
fi

# Restart xDS to ensure it picks up the HTTPS configuration
log_substep "Restarting xDS service to apply HTTPS configuration..."
if systemctl is-active --quiet globular-xds.service; then
  systemctl restart globular-xds.service
  sleep 3  # Wait for xDS to regenerate Envoy config
  log_success "xDS restarted with HTTPS config"
fi

# Restart Envoy to pick up the new configuration from xDS
log_substep "Restarting Envoy with HTTPS configuration..."
if systemctl is-active --quiet globular-envoy.service; then
  systemctl restart globular-envoy.service
  sleep 3  # Wait for Envoy to start with new config
  log_success "Envoy restarted on port 8443 (HTTPS)"
fi

log_step "Control Plane Services"
trace_step "running" "phase.control-plane" "Control Plane Services" 5

# Add /etc/hosts entry for <hostname>.globular.internal so the DNS service can
# resolve the etcd endpoint (globule-ryzen.globular.internal:2379) before the
# cluster DNS resolver is running. This is a Day-0 bootstrap necessity only —
# once DNS is bootstrapped the system resolver handles this.
_NODE_IP=$(hostname -I | awk '{print $1}')
_NODE_SHORT=$(hostname -s)
_NODE_FQDN="${_NODE_SHORT}.${DOMAIN}"
if [[ -n "$_NODE_IP" ]] && ! grep -qF "$_NODE_FQDN" /etc/hosts 2>/dev/null; then
  echo "$_NODE_IP  $_NODE_FQDN  $_NODE_SHORT" >> /etc/hosts
  log_substep "Added /etc/hosts: $_NODE_IP → $_NODE_FQDN (bootstrap DNS resolution)"
fi

install_list "${CONTROL_PLANE_PKGS[@]}"

# Set cluster_domain in cluster controller config so the admin UI and DNS
# reconciler know the canonical domain from the very first boot.
CC_CONFIG_DIR="/var/lib/globular/cluster-controller"
CC_CONFIG_FILE="${CC_CONFIG_DIR}/config.json"
mkdir -p "${CC_CONFIG_DIR}"
# Build default_profiles JSON array from the comma-separated FOUNDING_PROFILES
# (e.g. "core,media-server" -> ["core","media-server"]). Empty entries dropped.
DEFAULT_PROFILES_JSON=$(printf '%s' "$FOUNDING_PROFILES" \
  | jq -Rc 'split(",") | map(gsub("^\\s+|\\s+$";"")) | map(select(length>0))')
if [[ -z "$DEFAULT_PROFILES_JSON" || "$DEFAULT_PROFILES_JSON" == "[]" ]]; then
  DEFAULT_PROFILES_JSON='["core"]'
fi
log_substep "Founding profiles seed: default_profiles=${DEFAULT_PROFILES_JSON}"
if [[ -f "${CC_CONFIG_FILE}" ]]; then
  # Merge cluster_domain into existing config; seed default_profiles only if the
  # key is absent (//=) so re-runs never clobber an operator's later change.
  jq --arg d "$DOMAIN" --argjson p "$DEFAULT_PROFILES_JSON" \
    '.cluster_domain = $d | (.default_profiles //= $p)' \
    "${CC_CONFIG_FILE}" > "${CC_CONFIG_FILE}.tmp"
  mv "${CC_CONFIG_FILE}.tmp" "${CC_CONFIG_FILE}"
else
  # Seed cluster_domain and default_profiles — omit port so the controller uses
  # its own built-in default. The port is read from etcd after first start.
  cat > "${CC_CONFIG_FILE}" <<CCEOF
{
  "cluster_domain": "${DOMAIN}",
  "default_profiles": ${DEFAULT_PROFILES_JSON}
}
CCEOF
fi
chmod 644 "${CC_CONFIG_FILE}"
if id globular >/dev/null 2>&1; then
  chown globular:globular "${CC_CONFIG_FILE}"
fi
log_success "Cluster controller config: cluster_domain=${DOMAIN}"

# Restart cluster controller to pick up the domain
if systemctl is-active --quiet globular-cluster-controller.service 2>/dev/null; then
  systemctl restart globular-cluster-controller.service
  log_substep "Restarted cluster controller with cluster_domain"
fi

log_step "System Resolver Configuration (Day-0)"
if [[ -x "$SCRIPT_DIR/configure-resolver.sh" ]]; then
  RESOLVER_LOG="/tmp/configure-resolver-$(date +%Y%m%d-%H%M%S).log"
  set +e
  "$SCRIPT_DIR/configure-resolver.sh" --configure-only 2>&1 | tee "$RESOLVER_LOG"
  resolver_rc=${PIPESTATUS[0]}
  set -e

  if [[ $resolver_rc -ne 0 ]]; then
    die "configure-resolver.sh failed (see $RESOLVER_LOG)"
  fi

  if grep -q "VERIFY_RESULT=SKIPPED" "$RESOLVER_LOG"; then
    log_substep "System resolver configured; verification deferred until DNS bootstrap seeds records"
  elif grep -q "VERIFY_RESULT=FAIL" "$RESOLVER_LOG"; then
    log_substep "Warning: DNS resolver verification FAILED unexpectedly during configure-only phase (see $RESOLVER_LOG)"
  elif grep -q "VERIFY_RESULT=PASS" "$RESOLVER_LOG"; then
    log_success "System resolver configured for ${DOMAIN}"
  else
    log_substep "Warning: configure-resolver.sh completed without VERIFY_RESULT marker (see $RESOLVER_LOG)"
  fi
else
  log_substep "Warning: configure-resolver.sh not found, DNS system resolver not configured"
fi

log_step "DNS Bootstrap (Day-0)"

# Ensure globular CLI is callable as "globular" in PATH for bootstrap-dns.sh.
# install.sh normally does this; when install-day0.sh is run directly it may be missing.
_GLOBULAR_CLI_PATH="/usr/local/bin/globular"
if [[ ! -x "$_GLOBULAR_CLI_PATH" ]]; then
  _GLOBULAR_CLI_PATH="/usr/lib/globular/bin/globular"
fi
if [[ -x "$_GLOBULAR_CLI_PATH" ]] && [[ ! -x "/usr/local/bin/globular" ]]; then
  ln -sf "$_GLOBULAR_CLI_PATH" /usr/local/bin/globular
fi

# Ensure etcd is running — DNS depends on it and etcd may have been rate-limited
# by systemd if TLS certs were briefly unreadable during regeneration.
if ! systemctl is-active --quiet globular-etcd.service 2>/dev/null; then
  log_substep "etcd not running — resetting and restarting..."
  systemctl reset-failed globular-etcd.service 2>/dev/null || true
  chown -R globular:globular /var/lib/globular/pki 2>/dev/null || true
  systemctl start globular-etcd.service 2>/dev/null || true
  sleep 3
  if systemctl is-active --quiet globular-etcd.service 2>/dev/null; then
    log_success "etcd recovered"
  else
    log_substep "Warning: etcd still not running — DNS bootstrap may fail"
  fi
fi

# DNS zone/record registration is now handled by the dns package post-install script.
# On Day 0, the dns service package includes scripts/post-install.sh which runs
# after health_checks pass. Fallback to external script if post-install didn't run.
if [[ -x "$SCRIPT_DIR/bootstrap-dns.sh" ]]; then
  # Verify DNS records exist; if not, run the legacy bootstrap script.
  if command -v dig >/dev/null 2>&1 && dig @"${_NODE_IP}" +short "api.${DOMAIN}" 2>/dev/null | grep -q .; then
    log_success "DNS records already initialized (by package post-install)"
  else
    log_substep "DNS records missing — running bootstrap-dns.sh..."
    "$SCRIPT_DIR/bootstrap-dns.sh"
    log_success "DNS records initialized (n0, api)"
  fi
else
  log_substep "Warning: bootstrap-dns.sh not found, DNS records not initialized"
fi

log_step "System Resolver Verification (post-bootstrap)"
if [[ -x "$SCRIPT_DIR/configure-resolver.sh" ]]; then
  RESOLVER_VERIFY_LOG="/tmp/configure-resolver-verify-$(date +%Y%m%d-%H%M%S).log"
  set +e
  "$SCRIPT_DIR/configure-resolver.sh" --verify-only 2>&1 | tee "$RESOLVER_VERIFY_LOG"
  resolver_verify_rc=${PIPESTATUS[0]}
  set -e

  if [[ $resolver_verify_rc -ne 0 ]]; then
    die "configure-resolver.sh --verify-only failed (see $RESOLVER_VERIFY_LOG)"
  fi

  if grep -q "VERIFY_RESULT=PASS" "$RESOLVER_VERIFY_LOG"; then
    log_success "System resolver verified for ${DOMAIN} after DNS bootstrap"
  elif grep -q "VERIFY_RESULT=FAIL" "$RESOLVER_VERIFY_LOG"; then
    log_warn "DNS resolver verification FAILED after DNS bootstrap (see $RESOLVER_VERIFY_LOG)"
  else
    log_warn "configure-resolver.sh --verify-only completed without VERIFY_RESULT marker (see $RESOLVER_VERIFY_LOG)"
  fi
else
  log_warn "configure-resolver.sh not found, post-bootstrap DNS resolver verification skipped"
fi

log_step "RBAC Bootstrap Seed"
trace_step "running" "phase.rbac_seed" "RBAC Bootstrap Seed" 4
if [[ -x "${GLOBULAR_CLI:-}" || -x /usr/local/bin/globular || -x /usr/lib/globular/bin/globular ]]; then
  _RBAC_CLI="${GLOBULAR_CLI:-}"
  if [[ -z "$_RBAC_CLI" || ! -x "$_RBAC_CLI" ]]; then
    if [[ -x /usr/local/bin/globular ]]; then
      _RBAC_CLI="/usr/local/bin/globular"
    else
      _RBAC_CLI="/usr/lib/globular/bin/globular"
    fi
  fi
  _RBAC_IP="${_NODE_IP:-$(ip route get 8.8.8.8 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src") print $(i+1); exit}')}"
  _RBAC_IP="${_RBAC_IP:-$(hostname -I 2>/dev/null | awk '{print $1}')}"
  _RBAC_ETCD="${ETCD_ENDPOINTS:-https://${_RBAC_IP}:2379}"
  _RBAC_CA="/var/lib/globular/pki/ca.crt"
  _RBAC_CERT="/var/lib/globular/pki/issued/services/service.crt"
  _RBAC_KEY="/var/lib/globular/pki/issued/services/service.key"
  _RBAC_AUTH_PORT="$(resolve_registered_service_port "authentication.AuthenticationService" "$_RBAC_ETCD" "$_RBAC_CA" "$_RBAC_CERT" "$_RBAC_KEY")"
  _RBAC_AUTH_DIRECT=""
  if [[ -n "$_RBAC_AUTH_PORT" ]]; then
    _RBAC_AUTH_DIRECT="${_RBAC_IP}:${_RBAC_AUTH_PORT}"
  else
    _RBAC_AUTH_PORT="$(sudo ss -tlnp 2>/dev/null | awk '/authentication_/{match($4,/[^:]+$/); print substr($4,RSTART,RLENGTH)}' | head -1 || true)"
    [[ -n "$_RBAC_AUTH_PORT" ]] && _RBAC_AUTH_DIRECT="${_RBAC_IP}:${_RBAC_AUTH_PORT}"
  fi
  _RBAC_PASSWORD="${BOOTSTRAP_PASSWORD:-}"
  if [[ -z "$_RBAC_PASSWORD" && -f "${BOOTSTRAP_SA_CRED:-/var/lib/globular/.bootstrap-sa-password}" ]]; then
    _RBAC_PASSWORD="$(cat "${BOOTSTRAP_SA_CRED:-/var/lib/globular/.bootstrap-sa-password}" 2>/dev/null || true)"
  fi
  _RBAC_PASSWORD="${_RBAC_PASSWORD:-adminadmin}"
  _RBAC_TOKEN=""
  if [[ -n "$_RBAC_AUTH_DIRECT" ]]; then
    _RBAC_AUTH_OUT="$(HOME=/root "$_RBAC_CLI" --timeout 10s --insecure --auth "$_RBAC_AUTH_DIRECT" auth login --user sa --password "$_RBAC_PASSWORD" 2>&1 || true)"
    _RBAC_TOKEN="$(printf '%s\n' "$_RBAC_AUTH_OUT" | awk -F'Token: ' '/^Token:/ {print $2; exit}')"
    if [[ -z "$_RBAC_TOKEN" ]]; then
      log_warn "RBAC seed auth login failed via ${_RBAC_AUTH_DIRECT}: $(classify_auth_login_failure "$_RBAC_AUTH_OUT" "$_RBAC_AUTH_DIRECT")"
    fi
  else
    log_warn "authentication service not discoverable — RBAC seed will try bootstrap gate without token"
  fi
  _RBAC_ARGS=(--timeout 60s --insecure)
  [[ -n "$_RBAC_TOKEN" ]] && _RBAC_ARGS+=(--token "$_RBAC_TOKEN")
  if "$_RBAC_CLI" "${_RBAC_ARGS[@]}" rbac seed --force; then
    log_success "RBAC service-account bindings and roles seeded"
  else
    log_warn "RBAC seed failed — workflow recorder may report role_binding_denied"
  fi
  _RBAC_HOSTNAME="$(hostname -s 2>/dev/null || hostname 2>/dev/null || true)"
  if [[ -n "$_RBAC_HOSTNAME" ]]; then
    if "$_RBAC_CLI" "${_RBAC_ARGS[@]}" rbac bind --subject "$_RBAC_HOSTNAME" --role globular-node-executor; then
      log_success "RBAC bound founding node identity ${_RBAC_HOSTNAME} to globular-node-executor"
    else
      log_warn "RBAC founding-node executor binding failed for ${_RBAC_HOSTNAME}"
    fi
  fi
else
  log_warn "globular CLI not found — skipping RBAC bootstrap seed"
fi

# Remove bootstrap flag — DNS bootstrap is done, and the expired flag would
# cause every subsequent gRPC call to fail on first attempt (bootstrap_expired
# denial before falling through to normal auth), making publish very slow.
rm -f "${BOOTSTRAP_FLAG}" 2>/dev/null

log_step "Operations Services"
trace_step "running" "phase.ops" "Operations Services" 5
install_list "${OPS_PKGS[@]}"

# Ensure MCP server is running — it's required by ai-watcher/ai-executor/ai-router.
# The installer may not start it if the binary exits cleanly before deps are ready,
# so kick a restart here. The package spec already ships Restart=always so systemd
# keeps it alive once deps are reachable.
#
# NOTE: do NOT `sed -i` the unit file here. The package installer writes a
# .sha256 sidecar next to the unit at install time and node-agent's hash drift
# detector compares it to the live file every sweep. Any post-install edit
# (sed, override drop-in we forget to checksum, etc.) breaks the sidecar and
# produces a permanent globular-mcp.service hash_drift finding.
if [[ -f /etc/systemd/system/globular-mcp.service ]]; then
  systemctl restart globular-mcp.service 2>/dev/null || true
  log_substep "MCP server restarted"
fi

# Configure scylla-manager-agent — MinIO S3 backup creds + scylla api block.
# NOTE: auth_token, https/prometheus/debug ports, and scylla api_address are
# owned by the node-agent reconciler (see
# golang/node_agent/node_agent_server/scylla_manager_agent_config.go). It
# derives a cluster-wide token from sha256(domain|ca_hash) so every agent
# shares the same value — required for `sctool cluster add` to authenticate
# against every host. Installing a per-node UUID here would silently win the
# race against the reconciler and break manager → agent auth on the 4
# non-coordinator hosts. The reconciler also overrides any port choice here.
#
# Likewise the local `sctool cluster add` block this script used to run is
# gone — backup_manager's ensureScyllaRegistered handles registration once
# the agent has a stable token, using the manager's actual HTTPS endpoint
# (read from /var/lib/globular/scylla-manager/scylla-manager.yaml).
AGENT_CONFIG="/var/lib/globular/scylla-manager-agent/scylla-manager-agent.yaml"

# Detect ScyllaDB listen address (needed for S3 endpoint hint to MinIO)
SCYLLA_IP=""
if [[ -f /etc/scylla/scylla.yaml ]]; then
  # Strip quotes and whitespace from value (YAML may wrap IPs in single/double quotes)
  SCYLLA_IP=$(grep "^listen_address:" /etc/scylla/scylla.yaml 2>/dev/null | awk '{print $2}' | tr -d "'\"" || true)
fi
if [[ -z "$SCYLLA_IP" ]]; then
  SCYLLA_IP=$(ip route get 8.8.8.8 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src") print $(i+1); exit}')
fi
if [[ -z "$SCYLLA_IP" ]]; then
  SCYLLA_IP=$(hostname -I | awk '{print $1}')
fi

if [[ -f "$AGENT_CONFIG" ]] && ! grep -q "^s3:" "$AGENT_CONFIG"; then
  log_substep "Configuring scylla-manager-agent S3 access..."

  # Read MinIO credentials for S3 backup access
  MINIO_CRED_FILE="${STATE_DIR}/minio/credentials"
  AGENT_S3_BLOCK=""
  if [[ -f "$MINIO_CRED_FILE" ]]; then
    if IFS=":" read -r MINIO_AK MINIO_SK < "$MINIO_CRED_FILE" && [[ -n "$MINIO_AK" && -n "$MINIO_SK" ]]; then
      AGENT_S3_BLOCK="
# MinIO S3 access for ScyllaDB backups (auto-configured by install-day0.sh)
s3:
  access_key_id: ${MINIO_AK}
  secret_access_key: ${MINIO_SK}
  provider: Minio
  region: us-east-1
  endpoint: https://${SCYLLA_IP}:9000

# Skip TLS verification for internal MinIO with self-signed certs
rclone:
  insecure_skip_verify: true"
      log_substep "MinIO S3 credentials prepared for agent config"
    else
      log_substep "Warning: could not parse MinIO credentials from $MINIO_CRED_FILE"
    fi
  else
    log_substep "Warning: MinIO credentials file not found at $MINIO_CRED_FILE — agent will not have S3 access"
  fi

  # Append the S3 block. Token + ports + scylla block come from the node-agent
  # reconciler on its first heartbeat after this script finishes. Until then
  # the agent runs on its built-in defaults — the reconciler restarts it once
  # the canonical values are written.
  if [[ -n "$AGENT_S3_BLOCK" ]]; then
    printf "%s\n" "$AGENT_S3_BLOCK" >> "$AGENT_CONFIG"
    chown scylla:globular "$AGENT_CONFIG"
    chmod 0640 "$AGENT_CONFIG"
    log_success "scylla-manager-agent S3 config appended (endpoint=https://${SCYLLA_IP}:9000)"
  fi
fi

# Always ensure the agent config directory is owned by scylla (agent runs as scylla)
# with globular group so backup-manager can read it.
if [[ -d /var/lib/globular/scylla-manager-agent ]]; then
  chmod 0750 /var/lib/globular/scylla-manager-agent
  chown scylla:globular /var/lib/globular/scylla-manager-agent
  # Fix ownership of files inside too.
  chown scylla:globular /var/lib/globular/scylla-manager-agent/* 2>/dev/null || true
fi

log_step "Workload Services"
trace_step "running" "phase.workloads" "Workload Services" 5
install_list "${COMMON_WORKLOAD_PKGS[@]}"
if has_profile "media-server"; then
  install_list "${MEDIA_WORKLOAD_PKGS[@]}"
fi

# Run conformance tests
# Day-0 always runs in warn mode for now.
CONFORMANCE_MODE="warn"

if [[ "$CONFORMANCE_MODE" != "off" ]]; then
  log_step "Conformance Tests (mode: $CONFORMANCE_MODE)"
  CONFORMANCE_SCRIPT="$SCRIPT_DIR/../tests/conformance/run.sh"

  if [[ -x "$CONFORMANCE_SCRIPT" ]]; then
    log_substep "Running v1.0 conformance checks..."

    # Run conformance and capture exit code
    CONFORMANCE_LOG="/tmp/globular-conformance-$(date +%Y%m%d-%H%M%S).log"
    if "$CONFORMANCE_SCRIPT" 2>&1 | tee "$CONFORMANCE_LOG"; then
      log_success "All conformance tests passed!"
    else
      CONFORMANCE_EXIT=$?
      echo ""
      echo "╔════════════════════════════════════════════════════════════════╗"
      echo "║          ⚠  CONFORMANCE FAILED                                 ║"
      echo "╚════════════════════════════════════════════════════════════════╝"
      echo ""
      log_info "Some conformance tests failed (exit code: $CONFORMANCE_EXIT)"
      log_info "Full log: $CONFORMANCE_LOG"
      log_info "Run manually: sudo $CONFORMANCE_SCRIPT"
      echo ""

      if [[ "$CONFORMANCE_MODE" == "fail" ]]; then
        log_warn "Conformance violations detected"
      else
        # warn mode: continue but alert user
        log_info "⚠  Installation will continue (warn mode)"
        echo ""
      fi
    fi
  else
    log_substep "Conformance runner not bundled: $CONFORMANCE_SCRIPT"
    log_substep "Optional validation skipped (GLOBULAR_CONFORMANCE=$CONFORMANCE_MODE)"

    if [[ "$CONFORMANCE_MODE" == "fail" ]]; then
      die "Conformance runner required but not bundled: $CONFORMANCE_SCRIPT"
    fi
  fi
else
  log_substep "Conformance tests disabled"
fi

# Cluster Health Validation
log_step "Cluster Health Validation"
trace_step "running" "phase.health" "Cluster Health Validation" 8
VALIDATION_SCRIPT="$SCRIPT_DIR/validate-cluster-health.sh"

if [[ -x "$VALIDATION_SCRIPT" ]]; then
  log_substep "Running comprehensive cluster health checks..."
  echo ""

  # Run validation and capture exit code
  if "$VALIDATION_SCRIPT" --day0; then
    VALIDATION_PASSED=1
  else
    VALIDATION_PASSED=0
    VALIDATION_EXIT=$?
    echo ""
    echo "╔════════════════════════════════════════════════════════════════╗"
    echo "║          ⚠  CLUSTER HEALTH VALIDATION FAILED                   ║"
    echo "╚════════════════════════════════════════════════════════════════╝"
    echo ""
    log_info "Cluster health validation failed (exit code: $VALIDATION_EXIT)"
    log_info "Some services may not be running correctly"
    log_info "Review the validation output above for details"
    log_info "Common fixes:"
    log_info "  - Check service logs: journalctl -u globular-<service> -n 50"
    log_info "  - Restart failed services: systemctl restart globular-<service>"
    log_info "  - Re-run validation: sudo $VALIDATION_SCRIPT"
    echo ""
    die "Installation validation failed - cluster is not healthy"
  fi
else
  log_substep "Warning: Validation script not found: $VALIDATION_SCRIPT"
  log_substep "Skipping cluster health validation"
  VALIDATION_PASSED=0
fi

# Locate the Globular CLI.
# install.sh installs it at /usr/local/bin/globular from the tarball root before
# this script runs. The globular-cli package also installs it at $PREFIX/bin/globular.
# The binary is named "globular" (not "globularcli") per the package spec entrypoint.
GLOBULAR_CLI="/usr/local/bin/globular"
if [[ ! -x "$GLOBULAR_CLI" ]]; then
  GLOBULAR_CLI="/usr/lib/globular/bin/globular"
fi

# ── Day-0 Join Token ─────────────────────────────────────────────────────────
# Generate a shared join token so the first node can self-register with
# the cluster controller during bootstrap. Both services must share the
# same token: controller stores it in state.JoinTokens, node-agent passes
# it in RequestJoin.
log_step "Day-0 Join Token provisioning"
DAY0_TOKEN=$(cat /proc/sys/kernel/random/uuid 2>/dev/null || uuidgen)

# Write token to controller config
CC_CONFIG="/var/lib/globular/cluster-controller/config.json"
if [[ -f "$CC_CONFIG" ]]; then
  jq --arg tok "$DAY0_TOKEN" '.join_token = $tok' "$CC_CONFIG" > "${CC_CONFIG}.tmp" \
    && mv "${CC_CONFIG}.tmp" "$CC_CONFIG"
  log_substep "Join token written to controller config"
else
  mkdir -p "$(dirname "$CC_CONFIG")"
  echo "{\"join_token\": \"$DAY0_TOKEN\", \"port\": 12000}" > "$CC_CONFIG"
  log_substep "Controller config created with join token"
fi

# Persist the token in the node-agent state file so startup does not rely on
# any environment variable or systemd drop-in.
NA_STATE="$(resolve_node_agent_state_path)"
mkdir -p "$(dirname "$NA_STATE")"
if [[ -f "$NA_STATE" ]] && command -v jq >/dev/null 2>&1; then
  jq --arg tok "$DAY0_TOKEN" '.join_token = $tok' "$NA_STATE" > "${NA_STATE}.tmp" \
    && mv "${NA_STATE}.tmp" "$NA_STATE"
else
  cat > "$NA_STATE" <<EOF
{
  "join_token": "${DAY0_TOKEN}"
}
EOF
fi
chmod 0600 "$NA_STATE"
log_substep "Join token written to node-agent state file (${NA_STATE})"

# Fix controller state if it has stale protocol=http from a previous run.
CC_STATE="/var/lib/globular/clustercontroller/state.json"
if [[ -f "$CC_STATE" ]]; then
  STATE_PROTO=$(jq -r '.cluster_network_spec.protocol // "http"' "$CC_STATE" 2>/dev/null)
  if [[ "$STATE_PROTO" != "https" ]]; then
    jq '.cluster_network_spec.protocol = "https"' "$CC_STATE" > "${CC_STATE}.tmp" \
      && mv "${CC_STATE}.tmp" "$CC_STATE"
    log_substep "Fixed controller state protocol to https"
  fi
fi

# Reload systemd and restart only the controller so it picks up the join token.
# The node-agent is enabled but NOT started here — the operator starts it explicitly.
systemctl restart globular-cluster-controller
log_substep "Restarted cluster controller with shared join token"
# Give controller time to re-initialize with the new token.
sleep 5
log_success "Day-0 join token provisioned"

# ── Final Service Stabilization ──────────────────────────────────────────────
# Restart the cluster controller so it picks up fresh gRPC connections to all
# services. During install, services are started/stopped/restarted in sequence
# which leaves the controller with stale cached connections. A final restart
# ensures clean connectivity now that everything is stable.
log_step "Final Service Stabilization"
systemctl restart globular-cluster-controller.service 2>/dev/null || true
systemctl restart globular-gateway.service 2>/dev/null || true
sleep 3
log_success "Controller and gateway restarted with fresh connections"

echo ""
# ── AI credential access + MCP auto-configuration ────────────────────────────
# Seed the globular service user's own AI credential paths.
# Must run AFTER package installation (which creates the globular user).
INSTALLER_USER="${ORIGINAL_USER:-}"
if [[ -n "$INSTALLER_USER" ]] && id globular >/dev/null 2>&1; then
  INSTALLER_HOME=$(eval echo "~$INSTALLER_USER")
  GLOBULAR_HOME=$(eval echo "~globular")
  AI_BACKEND_SEEDED=0

  # ── AI credentials ──────────────────────────────────────────────────────
  CLAUDE_CREDS="$INSTALLER_HOME/.claude/.credentials.json"
  if [[ -f "$CLAUDE_CREDS" ]]; then
    install -d -o globular -g globular -m 0700 "$GLOBULAR_HOME/.claude"
    install -o globular -g globular -m 0600 "$CLAUDE_CREDS" "$GLOBULAR_HOME/.claude/.credentials.json"
    AI_BACKEND_SEEDED=1
    log_success "Seeded Claude credentials for globular service user"
  fi

  CODEX_AUTH="$INSTALLER_HOME/.codex/auth.json"
  if [[ -f "$CODEX_AUTH" ]]; then
    install -d -o globular -g globular -m 0700 "$GLOBULAR_HOME/.codex"
    install -o globular -g globular -m 0600 "$CODEX_AUTH" "$GLOBULAR_HOME/.codex/auth.json"
    AI_BACKEND_SEEDED=1
    log_success "Seeded Codex credentials for globular service user"
  fi

  if [[ "$AI_BACKEND_SEEDED" -eq 1 ]]; then
    # Restart ai_executor so it picks up the seeded service-user credentials
    # and syncs them to etcd during startup.
    systemctl restart globular-ai-executor.service 2>/dev/null || true
    log_success "AI backend credentials activated for ai_executor"
  fi

  # ── MCP server endpoint ─────────────────────────────────────────────────
  # Write ~/.claude/.mcp.json if not already pointing at this node's MCP.
  _MCP_JSON="$INSTALLER_HOME/.claude/.mcp.json"
  _NODE_IP=$(ip route get 8.8.8.8 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src") print $(i+1); exit}')
  _NODE_IP="${_NODE_IP:-$(hostname -I | awk '{print $1}')}"
  # Read MCP port from the installed systemd unit — never hardcode it.
  _MCP_PORT=$(grep -oP '(?<=--port[= ])\d+' /etc/systemd/system/globular-mcp.service 2>/dev/null | head -1 || true)
  if [[ -n "$_MCP_PORT" ]] && [[ -n "$_NODE_IP" ]] && command -v python3 >/dev/null 2>&1; then
    _MCP_URL="https://${_NODE_IP}:${_MCP_PORT}/mcp"
    python3 - "$_MCP_JSON" "$_MCP_URL" <<'PYEOF'
import json, sys, os
path, url = sys.argv[1], sys.argv[2]
cfg = {}
if os.path.exists(path):
    try:
        cfg = json.load(open(path))
    except Exception:
        cfg = {}
cfg.setdefault("mcpServers", {})["globular"] = {"type": "http", "url": url}
with open(path, "w") as f:
    json.dump(cfg, f, indent=2)
PYEOF
    chown "$INSTALLER_USER:$INSTALLER_USER" "$_MCP_JSON" 2>/dev/null || true
    log_success "Claude Code MCP endpoint set to ${_MCP_URL}"
  fi
fi

# ── Publish bootstrap artifacts to repository (Layer 1) ──────────────────────
# Populates the Repository catalog so the cluster can manage upgrades,
# new-node joins, and desired-state resolution. Idempotent — skips packages
# ── Copy release-index.json to state directory ────────────────────────────────
# The release-index.json is the authoritative BOM for this platform release.
# It is read by:
#   - ensure-bootstrap-artifacts.sh to determine SYNC_TAG
#   - gateway join_binaries.go to serve exact BOM package versions to joining nodes
# Without it, these paths fall back to legacy (latest published) behavior.
for _ri in \
    "$INSTALLER_ROOT/release-index.json" \
    "$INSTALLER_ROOT/internal/assets/release-index.json" \
    "$PKG_DIR/../release-index.json"; do
  if [[ -f "$_ri" ]]; then
    cp "$_ri" "${STATE_DIR}/release-index.json"
    chmod 0644 "${STATE_DIR}/release-index.json"
    # Log the platform release identity.
    _bom_tag=$(python3 -c "import json; d=json.load(open('${STATE_DIR}/release-index.json')); print(d.get('release_tag',''))" 2>/dev/null || true)
    _bom_plat=$(python3 -c "import json; d=json.load(open('${STATE_DIR}/release-index.json')); print(d.get('platform_release',''))" 2>/dev/null || true)
    log_success "Installed release-index.json to ${STATE_DIR}/ (tag=${_bom_tag:-unknown} platform_release=${_bom_plat:-unknown})"
    break
  fi
done
if [[ ! -f "${STATE_DIR}/release-index.json" ]]; then
  log_warn "release-index.json not found in installer bundle"
  log_warn "Day-1 join binaries will resolve to latest published (non-deterministic)."
  log_warn "Include release-index.json in the installer bundle for deterministic installs."
fi

# ── Copy package artifacts to local distribution directory ─────────────────────
# Packages are served to joining nodes via the gateway's /join/packages/ endpoint.
# This replaces the MinIO publish step — local disk is the sole package authority.
log_step "Copying Package Artifacts to /var/lib/globular/packages/"
DIST_PKG_DIR="${STATE_DIR}/packages"
mkdir -p "${DIST_PKG_DIR}"
_copied=0
_skipped=0
for _tgz in "$PKG_DIR/"*.tgz; do
  [[ -f "$_tgz" ]] || continue
  _dest="${DIST_PKG_DIR}/$(basename "$_tgz")"
  if [[ -f "$_dest" ]]; then
    _skipped=$((_skipped + 1))
  else
    cp "$_tgz" "$_dest" && _copied=$((_copied + 1)) || \
      log_warn "Failed to copy $(basename "$_tgz") to ${DIST_PKG_DIR}"
  fi
done
chown -R globular:globular "${DIST_PKG_DIR}" 2>/dev/null || true
log_success "Package distribution ready: ${_copied} copied, ${_skipped} already present (${DIST_PKG_DIR})"

# ── Register packages in repository (Layer 1) ────────────────────────────────
# Delegates to ensure-bootstrap-artifacts.sh which:
#   - Discovers the repository endpoint from etcd (no DNS dependency)
#   - Publishes all CORE_PACKAGES with checksum-based idempotency
#   - Registers the upstream source (GitHub releases) for Day-1+ sync
#   - Syncs the full release BOM from the upstream so the catalog is complete
# Non-fatal: if the script fails, Day-0 continues with whatever was published.
log_step "Registering Package Artifacts in Repository"
if [[ -x "$SCRIPT_DIR/ensure-bootstrap-artifacts.sh" ]]; then
  if bash "$SCRIPT_DIR/ensure-bootstrap-artifacts.sh" "$PKG_DIR" "$GLOBULAR_CLI"; then
    log_success "Bootstrap artifacts registered"
  else
    log_warn "ensure-bootstrap-artifacts.sh returned non-zero (non-fatal) — some packages may be missing from repository"
    log_warn "Re-run manually: bash $SCRIPT_DIR/ensure-bootstrap-artifacts.sh $PKG_DIR"
  fi
else
  log_warn "ensure-bootstrap-artifacts.sh not found at $SCRIPT_DIR — falling back to direct publish"
  if [[ -x "$GLOBULAR_CLI" ]]; then
    for _i in $(seq 1 30); do
      if "$GLOBULAR_CLI" pkg publish \
          --dir "$PKG_DIR" \
          --repository repository.globular.internal \
          >/dev/null 2>&1; then
        log_success "All packages registered in repository (fallback)"
        break
      fi
      [[ $_i -eq 30 ]] && log_warn "Could not register packages after 30 attempts" || sleep 2
    done
  else
    log_warn "globular CLI not found — skipping repository registration"
  fi
fi

# ── Seed desired state from installed packages (Layer 2) ─────────────────────
# The controller knows what is in the repository (Layer 1). We seed Layer 2
# (DesiredService) from the full installed inventory (Layer 3) so reconcileNodes
# can materialize infra desired state and the cluster becomes self-managing.
#
# The node agent may have started before all packages were deployed and only
# scanned a partial inventory. Restart it now to force a complete rescan before
# seeding so all 38+ packages are reported.
#
# DNS is not yet set up at this stage, so we connect directly to the controller
# using the local IP and CA certificate — no mesh routing needed here.
log_step "Seeding Desired State from Installed Packages"
if [[ -x "$GLOBULAR_CLI" ]]; then
  # Restart node agent to ensure full inventory is reported.
  log_substep "Restarting node agent to force full inventory scan..."
  systemctl restart globular-node-agent 2>/dev/null || true

  # Resolve controller address from etcd — the controller is running by this point and has
  # registered its address and port. Never use a hardcoded port.
  _SEED_IP="$(ip route get 8.8.8.8 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src") print $(i+1); exit}')"
  _SEED_IP="${_SEED_IP:-$(hostname -I 2>/dev/null | awk '{print $1}')}"
  _SEED_CA="/var/lib/globular/pki/ca.crt"
  _SEED_CERT="/var/lib/globular/pki/issued/services/service.crt"
  _SEED_KEY="/var/lib/globular/pki/issued/services/service.key"
  _SEED_ETCD="${ETCD_ENDPOINTS:-https://${_SEED_IP}:2379}"
  _CTRL_PORT_ETCD=$(etcdctl --endpoints="$_SEED_ETCD" \
    --cacert="$_SEED_CA" --cert="$_SEED_CERT" --key="$_SEED_KEY" \
    get "/globular/services/cluster_controller.ClusterControllerService/config" \
    --print-value-only 2>/dev/null | \
    grep -oP '"Port"\s*:\s*\K[0-9]+' | head -1 || true)
  if [[ -z "$_CTRL_PORT_ETCD" ]]; then
    # etcd lookup failed (controller not yet registered) — fall back to unit file
    _CTRL_PORT_ETCD=$(grep -oP '(?<=--port[= ])\d+' /etc/systemd/system/globular-cluster-controller.service 2>/dev/null | head -1 || true)
  fi
  [[ -n "$_CTRL_PORT_ETCD" ]] || die "Could not determine cluster controller port from etcd or unit file"
  _SEED_CTRL="${_SEED_IP}:${_CTRL_PORT_ETCD}"
  _SEED_STATE_PATH="$(resolve_node_agent_state_path)"
  _SEED_CONTROLLER_STATE_PATH="$(resolve_controller_state_path)"
  _SEED_NODE_ID=""
  _SEED_EXPECTED_COUNT="$(count_seed_expected_packages)"
  _SEED_LAST_NODE_ID=""

  # Wait for the controller-owned node record to show both last_seen and
  # inventory_complete. Seeding too early only captures a partial subset of the
  # install we just completed.
  log_substep "Waiting for controller-owned node heartbeat and inventory_complete..."
  log_substep "Expected managed package records from this Day-0 run: ${_SEED_EXPECTED_COUNT}"
  log_substep "Watching node-agent state path: ${_SEED_STATE_PATH}"
  log_substep "Watching controller state path: ${_SEED_CONTROLLER_STATE_PATH}"
  _HB_WAIT=0
  _HB_MAX=120
  _SEED_HEARTBEAT_MISSING=1
  _SEED_OBSERVED_COUNT=0
  _SEED_LAST_COUNT=-1
  _SEED_INVENTORY_COMPLETE=0
  while [[ $_HB_WAIT -lt $_HB_MAX ]]; do
    _SEED_STATE_PATH="$(resolve_node_agent_state_path)"
    _SEED_CONTROLLER_STATE_PATH="$(resolve_controller_state_path)"
    _SEED_NODE_ID="$(read_json_field "$_SEED_STATE_PATH" "node_id")"
    if [[ -n "$_SEED_NODE_ID" ]] && [[ "$_SEED_NODE_ID" != "$_SEED_LAST_NODE_ID" ]]; then
      log_substep "Using node-agent state node_id for heartbeat watch: ${_SEED_NODE_ID}"
      _SEED_LAST_NODE_ID="$_SEED_NODE_ID"
    fi
    if [[ -n "$_SEED_NODE_ID" ]]; then
      _SEED_STATUS="$(read_controller_node_seed_status "$_SEED_CONTROLLER_STATE_PATH" "$_SEED_NODE_ID")"
    else
      _SEED_STATUS=""
    fi
    if [[ -n "$_SEED_STATUS" ]]; then
      IFS=$'\t' read -r _SEED_HAS_LAST_SEEN _SEED_HAS_INVENTORY_COMPLETE _SEED_CONTROLLER_COUNT <<< "$_SEED_STATUS"
    else
      _SEED_HAS_LAST_SEEN=0
      _SEED_HAS_INVENTORY_COMPLETE=0
      _SEED_CONTROLLER_COUNT=0
    fi
    if [[ "${_SEED_HAS_LAST_SEEN:-0}" == "1" ]]; then
      _SEED_HEARTBEAT_MISSING=0
      _SEED_OBSERVED_COUNT="${_SEED_CONTROLLER_COUNT:-0}"
      if [[ "$_SEED_OBSERVED_COUNT" != "$_SEED_LAST_COUNT" ]]; then
        log_substep "Observed controller inventory after ${_HB_WAIT}s: ${_SEED_OBSERVED_COUNT} managed services recorded"
        _SEED_LAST_COUNT="$_SEED_OBSERVED_COUNT"
      fi
      if [[ "${_SEED_HAS_INVENTORY_COMPLETE:-0}" == "1" ]]; then
        _SEED_INVENTORY_COMPLETE=1
        log_substep "Heartbeat and inventory_complete detected after ${_HB_WAIT}s"
        break
      fi
    fi
    sleep 3
    _HB_WAIT=$((_HB_WAIT + 3))
  done
  if [[ $_HB_WAIT -ge $_HB_MAX ]]; then
    if [[ "${_SEED_HEARTBEAT_MISSING:-1}" -eq 1 ]]; then
      log_warn "No heartbeat detected after ${_HB_MAX}s for node-agent state node_id '${_SEED_NODE_ID:-<unset>}' — seed may partially fail (safe to re-run)"
      log_node_agent_seed_diagnostics "$_SEED_STATE_PATH" "$_SEED_NODE_ID" "$_SEED_EXPECTED_COUNT" "$_SEED_OBSERVED_COUNT"
    else
      log_warn "Controller state did not mark inventory_complete after ${_HB_MAX}s — observed ${_SEED_OBSERVED_COUNT} managed services"
    fi
  fi

  # Seed with retry: transient heartbeat timing can still cause partial failures.
  # Up to 3 attempts with a short pause between each. The seed is idempotent —
  # already-imported entries are skipped, so retries only pick up failures.
  _SEED_ATTEMPTS=3
  _SEED_OK=false
  for _attempt in $(seq 1 $_SEED_ATTEMPTS); do
    log_substep "Running 'globular services seed' (attempt ${_attempt}/${_SEED_ATTEMPTS}, controller=${_SEED_CTRL})..."
    _SEED_OUT="$("$GLOBULAR_CLI" services seed \
        --controller "${_SEED_CTRL}" \
        --ca "${_SEED_CA}" 2>&1)" && _SEED_RC=0 || _SEED_RC=$?
    while IFS= read -r line; do
      echo "  [seed] $line"
    done <<< "$_SEED_OUT"
    if [[ $_SEED_RC -eq 0 ]]; then
      log_success "Desired state seeded from installed packages"
      _SEEDED_COUNT="$(printf '%s\n' "$_SEED_OUT" | sed -n 's/.*Desired state seeded with \([0-9][0-9]*\) service.*/\1/p' | head -1)"
      if [[ "${_SEED_HEARTBEAT_MISSING:-0}" -eq 1 ]]; then
        if [[ -n "$_SEEDED_COUNT" ]]; then
          log_warn "Desired state seeding completed without a heartbeat; seeded ${_SEEDED_COUNT} service(s), result may be partial"
        else
          log_warn "Desired state seeding completed without a heartbeat; result may be partial"
        fi
      elif [[ "${_SEED_INVENTORY_COMPLETE:-0}" -ne 1 ]]; then
        if [[ -n "$_SEEDED_COUNT" ]]; then
          log_warn "Desired state seeding completed before controller inventory_complete; seeded ${_SEEDED_COUNT} service(s), observed ${_SEED_OBSERVED_COUNT} managed services"
        else
          log_warn "Desired state seeding completed before controller inventory_complete; observed ${_SEED_OBSERVED_COUNT} managed services"
        fi
      fi
      _SEED_OK=true
      break
    fi
    if [[ $_attempt -lt $_SEED_ATTEMPTS ]]; then
      log_substep "Seed attempt ${_attempt} had failures — retrying in 10s..."
      sleep 10
    fi
  done
  if [[ "$_SEED_OK" != "true" ]]; then
    log_warn "services seed did not fully succeed after ${_SEED_ATTEMPTS} attempts"
    log_warn "Re-run manually after bootstrap: globular services seed --controller ${_SEED_CTRL} --ca ${_SEED_CA}"
  fi
else
  log_warn "globular CLI not found at $GLOBULAR_CLI — skipping desired state seed"
  log_warn "Run manually after bootstrap: globular services seed"
fi

# ── Initialize AI operational memory (ops-knowledge seed) ───────────────────
# Day-0 must preload operational knowledge so AI agents have baseline runbooks,
# invariants and known failure modes before first intervention.
log_step "Initializing AI Operational Memory"
if [[ -x "$GLOBULAR_CLI" ]]; then
  # Prefer seeded entries from the installed awareness bundle (release artifact).
  # Fallback to source-tree docs path for dev/checkouts.
  OPS_KNOWLEDGE_DIR="/var/lib/globular/awareness/current/ops-knowledge"
  if [[ ! -d "$OPS_KNOWLEDGE_DIR" ]]; then
    _AWARENESS_BUNDLE=""
    for _ab in \
      "$INSTALLER_ROOT/awareness"/awareness-bundle-*.tar.gz \
      "/var/lib/globular/awareness"/awareness-bundle-*.tar.gz; do
      if [[ -f "$_ab" ]]; then
        _AWARENESS_BUNDLE="$_ab"
        break
      fi
    done
    if [[ -n "$_AWARENESS_BUNDLE" ]]; then
      _AW_VERSION=""
      _AW_BUILD_ID=""
      _AW_FILENAME_BUILD_ID=""
      _AW_MANIFEST_BUILD_ID=""
      _AW_INDEX_BUILD_ID=""
      if [[ -f "${STATE_DIR}/release-index.json" ]]; then
        _AW_VERSION=$(python3 -c "import json; d=json.load(open('${STATE_DIR}/release-index.json')); print((d.get('platform_release') or d.get('release_tag','').lstrip('v')).strip())" 2>/dev/null || true)
      fi
      _AW_VERSION="$(normalize_meta_value "$_AW_VERSION")"
      if [[ -z "$_AW_VERSION" ]]; then
        _AW_VERSION=$(basename "$_AWARENESS_BUNDLE" | sed -n 's/^awareness-bundle-\([0-9][0-9.]*\)-[A-Za-z0-9._-]\+\.tar\.gz$/\1/p')
      fi
      _AW_FILENAME_BUILD_ID=$(basename "$_AWARENESS_BUNDLE" | sed -n 's/^awareness-bundle-[0-9][0-9.]*-\([A-Za-z0-9._-]\+\)\.tar\.gz$/\1/p')
      _AW_MANIFEST=""
      _AW_EMBEDDED_VERSION=""
      _AW_EMBEDDED_BUILD_ID=""
      if [[ -f "$(dirname "$_AWARENESS_BUNDLE")/manifest.json" ]]; then
        _AW_MANIFEST="$(dirname "$_AWARENESS_BUNDLE")/manifest.json"
      elif [[ -f "${_AWARENESS_BUNDLE}.manifest.json" ]]; then
        _AW_MANIFEST="${_AWARENESS_BUNDLE}.manifest.json"
      elif [[ -f "${_AWARENESS_BUNDLE%.tar.gz}.manifest.json" ]]; then
        _AW_MANIFEST="${_AWARENESS_BUNDLE%.tar.gz}.manifest.json"
      fi
      if [[ -n "$_AW_MANIFEST" ]]; then
        _AW_MANIFEST_BUILD_ID="$(python3 -c "import json; d=json.load(open('${_AW_MANIFEST}')); print((d.get('build_id') or '').strip())" 2>/dev/null || true)"
        _AW_MANIFEST_BUILD_ID="$(normalize_meta_value "$_AW_MANIFEST_BUILD_ID")"
        [[ -n "$_AW_MANIFEST_BUILD_ID" ]] && _AW_BUILD_ID="$_AW_MANIFEST_BUILD_ID"
      fi
      if [[ -z "$_AW_BUILD_ID" && -n "$_AW_FILENAME_BUILD_ID" ]]; then
        log_warn "Awareness manifest build_id unavailable; not passing --build-id (filename token='${_AW_FILENAME_BUILD_ID}')"
      fi
      # Fallback: extract metadata from manifest.json embedded in the bundle.
      # This avoids depending on sidecar files and provides a full UUID build_id.
      if [[ -z "$_AW_BUILD_ID" ]]; then
        _AW_EMBEDDED_VERSION="$(tar -xOzf "$_AWARENESS_BUNDLE" manifest.json 2>/dev/null | python3 -c "import json,sys; d=json.load(sys.stdin); print((d.get('version') or '').strip())" 2>/dev/null || true)"
        _AW_EMBEDDED_BUILD_ID="$(tar -xOzf "$_AWARENESS_BUNDLE" manifest.json 2>/dev/null | python3 -c "import json,sys; d=json.load(sys.stdin); print((d.get('build_id') or '').strip())" 2>/dev/null || true)"
        _AW_EMBEDDED_VERSION="$(normalize_meta_value "$_AW_EMBEDDED_VERSION")"
        _AW_EMBEDDED_BUILD_ID="$(normalize_meta_value "$_AW_EMBEDDED_BUILD_ID")"
        if [[ -n "$_AW_EMBEDDED_BUILD_ID" ]]; then
          _AW_BUILD_ID="$_AW_EMBEDDED_BUILD_ID"
          log_substep "Using embedded awareness manifest build_id from bundle payload"
        fi
        if [[ -z "$_AW_VERSION" && -n "$_AW_EMBEDDED_VERSION" ]]; then
          _AW_VERSION="$_AW_EMBEDDED_VERSION"
        fi
      fi
      if [[ -f "${STATE_DIR}/release-index.json" ]]; then
        _AW_INDEX_BUILD_ID="$(python3 -c "import json; d=json.load(open('${STATE_DIR}/release-index.json')); print((d.get('build_id') or '').strip())" 2>/dev/null || true)"
        _AW_INDEX_BUILD_ID="$(normalize_meta_value "$_AW_INDEX_BUILD_ID")"
      fi

      _AWARENESS_INSTALL_ARGS=()
      [[ -n "$_AW_VERSION" ]] && _AWARENESS_INSTALL_ARGS+=(--version "$_AW_VERSION")
      [[ -n "$_AW_BUILD_ID" ]] && _AWARENESS_INSTALL_ARGS+=(--build-id "$_AW_BUILD_ID")
      [[ -n "$_AW_MANIFEST" ]] && _AWARENESS_INSTALL_ARGS+=(--manifest "$_AW_MANIFEST")
      _AW_TEMP_RELEASE_INDEX=""
      if [[ -f "${STATE_DIR}/release-index.json" ]]; then
        if [[ -n "$_AW_INDEX_BUILD_ID" && -n "$_AW_BUILD_ID" && "$_AW_INDEX_BUILD_ID" == "$_AW_BUILD_ID" ]]; then
          _AWARENESS_INSTALL_ARGS+=(--release-index "${STATE_DIR}/release-index.json")
        elif [[ -z "$_AW_INDEX_BUILD_ID" ]]; then
          log_warn "Skipping --release-index for awareness install: release-index build_id unavailable"
        else
          log_warn "Skipping --release-index for awareness install: build_id mismatch (release-index='${_AW_INDEX_BUILD_ID:-unknown}' manifest='${_AW_MANIFEST_BUILD_ID:-unknown}')"
        fi
      fi
      # The awareness installer defaults to /var/lib/globular/release-index.json
      # when --release-index is omitted. If that file is malformed or missing
      # build metadata, install fails even with explicit --version/--build-id.
      # Provide a minimal verified release-index when host metadata is unusable.
      if [[ -n "$_AW_VERSION" && -n "$_AW_BUILD_ID" && ! " ${_AWARENESS_INSTALL_ARGS[*]} " =~ " --release-index " ]]; then
        _AW_TEMP_RELEASE_INDEX="$(mktemp /tmp/awareness-release-index.XXXXXX.json)"
        cat >"$_AW_TEMP_RELEASE_INDEX" <<EOF
{"platform_release":"${_AW_VERSION}","release_tag":"v${_AW_VERSION}","build_id":"${_AW_BUILD_ID}"}
EOF
        _AWARENESS_INSTALL_ARGS+=(--release-index "$_AW_TEMP_RELEASE_INDEX")
        log_substep "Using synthetic release-index for awareness install"
      fi

      log_substep "Installing awareness bundle from ${_AWARENESS_BUNDLE}..."
      if "$GLOBULAR_CLI" awareness install "$_AWARENESS_BUNDLE" \
          "${_AWARENESS_INSTALL_ARGS[@]}" \
          2>&1 | while IFS= read -r line; do echo "  [awareness-install] $line"; done; then
        log_success "Awareness bundle installed"
      else
        [[ -n "$_AW_TEMP_RELEASE_INDEX" ]] && rm -f "$_AW_TEMP_RELEASE_INDEX" 2>/dev/null || true
        die "awareness bundle install failed"
      fi
      [[ -n "$_AW_TEMP_RELEASE_INDEX" ]] && rm -f "$_AW_TEMP_RELEASE_INDEX" 2>/dev/null || true
    fi
  fi
  if [[ ! -d "$OPS_KNOWLEDGE_DIR" ]]; then
    if [[ -d "$INSTALLER_ROOT/docs/operational-knowledge" ]]; then
      OPS_KNOWLEDGE_DIR="$INSTALLER_ROOT/docs/operational-knowledge"
    elif [[ -d "docs/operational-knowledge" ]]; then
      OPS_KNOWLEDGE_DIR="$(pwd)/docs/operational-knowledge"
    else
      log_warn "operational-knowledge directory not found — skipping ops-knowledge seed (ai-memory will be seeded at day-1)"
      OPS_KNOWLEDGE_DIR=""
    fi
  fi
  if [[ -z "$OPS_KNOWLEDGE_DIR" ]]; then
    : # skip — warned above
  else
  log_substep "Operational knowledge source: ${OPS_KNOWLEDGE_DIR}"

  BOOTSTRAP_SA_CRED="${BOOTSTRAP_SA_CRED:-/var/lib/globular/.bootstrap-sa-password}"
  BOOTSTRAP_PASSWORD=""
  if [[ -f "$BOOTSTRAP_SA_CRED" ]]; then
    BOOTSTRAP_PASSWORD="$(cat "$BOOTSTRAP_SA_CRED" 2>/dev/null || true)"
  fi
  BOOTSTRAP_PASSWORD="${BOOTSTRAP_PASSWORD:-adminadmin}"
  _OPS_CA="/var/lib/globular/pki/ca.crt"

  # ai-memory is installed earlier in this run and registers its port in
  # etcd asynchronously after start. Discover the port by scanning the
  # UUID-keyed /globular/services/<uuid>/config entries and matching
  # Name=="ai_memory.AiMemoryService". Retry briefly to absorb the gap
  # between systemd-start and etcd-self-registration; only fall back to
  # the day-1 retry path if ai-memory has not registered after that window.
  _OPS_SKIP_SEED=0

  # Day-0 DNS may not be ready yet. Resolve AI-memory endpoint directly
  # (etcd first, then unit file, then local IP fallback) and pass --memory.
  _OPS_IP="${_SEED_IP:-$(ip route get 8.8.8.8 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src") print $(i+1); exit}')}"
  _OPS_IP="${_OPS_IP:-$(hostname -I 2>/dev/null | awk '{print $1}')}"
  _OPS_CERT="/var/lib/globular/pki/issued/services/service.crt"
  _OPS_KEY="/var/lib/globular/pki/issued/services/service.key"
  _OPS_ETCD="${ETCD_ENDPOINTS:-https://${_OPS_IP}:2379}"

  _OPS_MEM_PORT=""
  for _ops_port_try in $(seq 1 30); do
    _OPS_MEM_PORT="$(resolve_registered_service_port "ai_memory.AiMemoryService" "$_OPS_ETCD" "$_OPS_CA" "$_OPS_CERT" "$_OPS_KEY")"
    if [[ -n "$_OPS_MEM_PORT" ]]; then
      break
    fi
    sleep 2
  done

  if [[ -z "$_OPS_MEM_PORT" ]]; then
    log_warn "ai-memory did not register a port in etcd after 60s — ops-knowledge seed deferred to day-1 retry"
    _OPS_SKIP_SEED=1
  else
    log_substep "ai-memory registered at port ${_OPS_MEM_PORT} (etcd)"
  fi

  # Auth is only needed when ai-memory is reachable.
  _OPS_TOKEN=""
  _OPS_AUTH_PORT=""
  _OPS_AUTH_ENDPOINT=""
  if [[ "$_OPS_SKIP_SEED" -eq 0 ]]; then
    _OPS_AUTH_PORT="$(resolve_registered_service_port "authentication.AuthenticationService" "$_OPS_ETCD" "$_OPS_CA" "$_OPS_CERT" "$_OPS_KEY")"
    if [[ -z "$_OPS_AUTH_PORT" ]]; then
      _OPS_AUTH_PORT="$(grep -oP '(?<=--port[= ])\d+' /etc/systemd/system/globular-authentication.service 2>/dev/null | head -1 || true)"
    fi
    if [[ -n "$_OPS_AUTH_PORT" ]]; then
      _OPS_AUTH_ENDPOINT="${_OPS_IP}:${_OPS_AUTH_PORT}"
      log_substep "Authentication service endpoint for ops seed: ${_OPS_AUTH_ENDPOINT}"
    else
      log_warn "Could not resolve authentication service port for ops seed — authentication readiness will be classified from CLI output only"
    fi
    log_substep "Authenticating as bootstrap SA user for ops-knowledge seed..."
    for _ops_auth_try in $(seq 1 5); do
      _LOGIN_ARGS=(--ca "$_OPS_CA")
      if [[ -n "$_OPS_AUTH_ENDPOINT" ]]; then
        _LOGIN_ARGS+=(--auth "$_OPS_AUTH_ENDPOINT")
      fi
      _LOGIN_OUT="$("$GLOBULAR_CLI" "${_LOGIN_ARGS[@]}" auth login --user sa --password "$BOOTSTRAP_PASSWORD" 2>&1 || true)"
      _OPS_TOKEN="$(echo "$_LOGIN_OUT" | sed -n 's/^Token: //p' | head -n1 || true)"
      if [[ -z "$_OPS_TOKEN" && -f /root/.config/globular/token ]]; then
        _OPS_TOKEN="$(cat /root/.config/globular/token 2>/dev/null || true)"
      fi
      if [[ -n "$_OPS_TOKEN" ]]; then
        break
      fi
      log_substep "Auth not ready for ops seed (attempt ${_ops_auth_try}/5): $(classify_auth_login_failure "$_LOGIN_OUT" "$_OPS_AUTH_ENDPOINT")"
      sleep 2
    done
    if [[ -z "$_OPS_TOKEN" ]]; then
      if systemctl is-active --quiet globular-authentication.service 2>/dev/null; then
        log_substep "authentication service state: active"
      else
        log_warn "authentication service is not active"
      fi
      log_warn "Failed to get auth token for ops-knowledge seed — $(classify_auth_login_failure "$_LOGIN_OUT" "$_OPS_AUTH_ENDPOINT"). Seed deferred to day-1."
      _OPS_SKIP_SEED=1
    fi
  fi

  if [[ "$_OPS_SKIP_SEED" -eq 0 ]]; then
    _OPS_MEMORY="${_OPS_IP}:${_OPS_MEM_PORT}"
    log_substep "Using direct ai-memory endpoint for seed: ${_OPS_MEMORY}"

    log_substep "Seeding operational knowledge into AI memory..."
    _ops_seed_ok=0
    for _ops_seed_try in $(seq 1 5); do
      if "$GLOBULAR_CLI" ops-knowledge seed --dir "$OPS_KNOWLEDGE_DIR" --memory "$_OPS_MEMORY" --token "$_OPS_TOKEN" \
          2>&1 | while IFS= read -r line; do echo "  [ops-seed] $line"; done; then
        _ops_seed_ok=1
        break
      fi
      log_substep "ops-knowledge seed retry ${_ops_seed_try}/5..."
      sleep 2
    done
    if [[ "$_ops_seed_ok" -ne 1 ]]; then
      log_warn "ops-knowledge seed failed — will retry at day-1 when ai-memory is running"
      _OPS_SKIP_SEED=1
    else
      log_success "Operational knowledge seed completed"

      log_substep "Verifying seeded knowledge integrity..."
      if "$GLOBULAR_CLI" ops-knowledge verify --dir "$OPS_KNOWLEDGE_DIR" --memory "$_OPS_MEMORY" --token "$_OPS_TOKEN" \
          2>&1 | while IFS= read -r line; do echo "  [ops-verify] $line"; done; then
        log_success "Operational knowledge integrity verified"
      else
        log_warn "ops-knowledge verify reported drift — seed may be incomplete, will retry at day-1"
        _OPS_SKIP_SEED=1
      fi
    fi
  fi

  if [[ "$_OPS_SKIP_SEED" -eq 0 ]]; then
    log_substep "Validating AI operational-awareness availability..."
    _OPS_LIST_RAW="$("$GLOBULAR_CLI" ops-knowledge list --memory "$_OPS_MEMORY" --token "$_OPS_TOKEN" 2>/dev/null || true)"
    _OPS_COUNT="$(printf '%s\n' "$_OPS_LIST_RAW" | awk 'NR>1 {c++} END {print c+0}')"
    if [[ "${_OPS_COUNT:-0}" -lt 10 ]]; then
      log_warn "Operational knowledge appears incomplete (entries=${_OPS_COUNT:-0}) — will retry at day-1"
    fi
    if ! printf '%s\n' "$_OPS_LIST_RAW" | grep -q "ops.role.ai-memory"; then
      # Keep day-0 robust across ops-knowledge schema/output evolution.
      # verify+count already prove seeded awareness is available.
      _OPS_LIST_JSON="$("$GLOBULAR_CLI" --output json ops-knowledge list --memory "$_OPS_MEMORY" --token "$_OPS_TOKEN" 2>/dev/null || true)"
      if printf '%s\n' "$_OPS_LIST_JSON" | grep -Eq '"id"[[:space:]]*:[[:space:]]*"ops\.role\.ai-memory"|"ops\.role\.ai-memory"'; then
        log_substep "Confirmed ai-memory role entry via JSON output"
      else
        log_warn "Could not find explicit ops.role.ai-memory entry in list output; proceeding because seed+verify passed"
      fi
    fi
    log_success "AI operational-awareness available (${_OPS_COUNT} entries loaded)"
  else
    log_substep "Ops-knowledge seed skipped at day-0 — run 'globular ops-knowledge seed' after day-1 workloads start"
  fi
  fi # end: OPS_KNOWLEDGE_DIR non-empty
else
  log_warn "globular CLI not found at $GLOBULAR_CLI — cannot initialize AI operational memory. Ops-knowledge seed skipped."
fi

# ── Scylla Manager: ensure HTTPS and register Scylla cluster ─────────────────
# HTTPS: patch the running scylla-manager.yaml to add https:/tls lines if the
# service started without them (config was created before this step existed).
# Registration: register the local Scylla node with scylla-manager so backup,
# repair, and restore are available. Idempotent — skipped if already registered.
SM_CFG="${STATE_DIR}/scylla-manager/scylla-manager.yaml"
SM_CERT="${STATE_DIR}/pki/issued/services/service.crt"
SM_KEY="${STATE_DIR}/pki/issued/services/service.key"
SCTOOL_BIN="/usr/lib/globular/bin/sctool"
AGENT_CONFIG="${STATE_DIR}/scylla-manager-agent/scylla-manager-agent.yaml"

log_step "Scylla Manager"

# 1. Patch existing config to add HTTPS if missing
if [[ -f "$SM_CFG" ]] && ! grep -q "^https:" "$SM_CFG"; then
  SM_HOST=$(grep -E '^http:' "$SM_CFG" | awk '{print $2}' | cut -d: -f1 | tr -d ' ')
  if [[ -n "$SM_HOST" && -f "$SM_CERT" && -f "$SM_KEY" ]]; then
    log_substep "Adding HTTPS to scylla-manager config (${SM_HOST}:5443)..."
    sed -i \
      -e "/^http:/a https: ${SM_HOST}:5443" \
      -e "/^https:/a tls_cert_file: $SM_CERT" \
      -e "/^tls_cert_file:/a tls_key_file: $SM_KEY" \
      "$SM_CFG"
    chown globular:globular "$SM_CFG"
    log_success "scylla-manager HTTPS configured (port 5443)"
    # Restart to open the HTTPS listener
    if systemctl is-active --quiet globular-scylla-manager.service 2>/dev/null; then
      log_substep "Restarting scylla-manager to apply HTTPS config..."
      systemctl restart globular-scylla-manager.service
      for _i in $(seq 1 15); do
        ss -lnt | grep -q ":5443 " && break
        sleep 1
      done
      ss -lnt | grep -q ":5443 " && log_success "scylla-manager HTTPS listener up" \
        || log_warn "scylla-manager :5443 not yet visible — may still be starting"
    fi
  else
    log_substep "Skipping HTTPS patch: host='${SM_HOST}' cert=$(test -f "$SM_CERT" && echo ok || echo missing)"
  fi
elif [[ -f "$SM_CFG" ]]; then
  log_substep "scylla-manager HTTPS already configured"
else
  log_substep "scylla-manager config not yet created — configure script will add HTTPS on first start"
fi

# 2. Cluster registration is DEFERRED to backup_manager.ensureScyllaRegistered.
#
# Do NOT run `sctool cluster add` here. At day-0 the scylla-manager-agent on
# every node still runs on its built-in defaults — the cluster-wide auth_token
# (sha256(domain|ca_hash)) and the non-conflicting agent HTTPS port (5612) are
# written by the node-agent reconciler on its first heartbeat AFTER this script
# finishes (golang/node_agent/node_agent_server/scylla_manager_agent_config.go).
# Registering now makes scylla-manager run a connectivity check against every
# ring member; peers whose agent has not yet been reconciled still answer on
# their default :10001 with a mismatched/empty token, so the add fails with
# "<peer-ip>: agent [HTTP 401] unauthorized". This is a re-introduced regression
# of the local-registration block that the agent-config note above documents as
# removed on purpose.
#
# backup_manager.ensureScyllaRegistered performs the add idempotently once the
# token + port are stable across all agents, reading the manager's real HTTPS
# endpoint from scylla-manager.yaml.
log_substep "Scylla cluster registration deferred to backup_manager (runs after agent token/port reconcile)"

# ── Clean up legacy underscore-named dirs and transient install files ─────────
# Package specs now use hyphenated canonical names (ai-executor, node-agent, …).
# Old installs or prior runs may have left empty underscore-named dirs behind.
# Remove them only if they are empty — non-empty dirs are left for operator review.
for _legacy_dir in \
  ai_executor ai_memory ai_router ai_watcher \
  backup_manager cluster_controller cluster_doctor clusterdoctor node_agent nodeagent; do
  _p="${STATE_DIR}/${_legacy_dir}"
  if [[ -d "$_p" ]] && [[ -z "$(ls -A "$_p" 2>/dev/null)" ]]; then
    rmdir "$_p" && log_info "Removed empty legacy dir: ${_p}"
  fi
done
# Remove stale config.json backup files left from format migrations.
find "${STATE_DIR}" -maxdepth 1 -name 'config.json.bak.*' -delete 2>/dev/null || true

# ── Restore preserved AI memory (from a prior clean-node) ────────────────────
# clean-node.sh exports the ai_memory / behavioral_memory ScyllaDB keyspaces to
# /var/backups/globular/ai-memory-snapshot before wiping a node. If that
# snapshot is present, re-import it now: ScyllaDB and the ai-memory schema are
# up (the ops-knowledge seed above already exercised them), so COPY FROM
# layers the preserved operator/agent memories on top of the seeded baseline
# (COPY = upsert by primary key). This closes the clean → day-0 round-trip.
#
# Best-effort: never fails the install. Idempotent — the snapshot is archived
# aside on success so re-running day-0 does not double-import. Counter tables
# (governance stats) are not COPY-restorable and degrade with a warning.
# Opt out with: GLOBULAR_SKIP_AI_RESTORE=1
AI_RESTORE_DIR="/var/backups/globular/ai-memory-snapshot"
if [[ "${GLOBULAR_SKIP_AI_RESTORE:-0}" != "1" ]] && [[ -f "${AI_RESTORE_DIR}/.saved_at" ]]; then
  log_step "Restoring preserved AI Memory"
  _R_HOST="${SCYLLA_CQL_HOST:-$(scylla_cql_host)}"
  [[ -z "$_R_HOST" ]] && _R_HOST="127.0.0.1"
  if command -v cqlsh >/dev/null 2>&1 && cqlsh "$_R_HOST" 9042 -e "SELECT now() FROM system.local" >/dev/null 2>&1; then
    log_substep "Restoring from ${AI_RESTORE_DIR} (saved $(cat "${AI_RESTORE_DIR}/.saved_at" 2>/dev/null) on $(cat "${AI_RESTORE_DIR}/.source_host" 2>/dev/null || echo unknown))"
    _restored=0
    for _ks in ai_memory behavioral_memory; do
      [[ -d "${AI_RESTORE_DIR}/${_ks}" ]] || continue
      # Ensure schema exists. The service normally creates it on first start;
      # apply the captured schema as a safety net (errors on already-existing
      # objects are ignored — the live RF/definition wins).
      if [[ -f "${AI_RESTORE_DIR}/${_ks}/schema.cql" ]]; then
        cqlsh "$_R_HOST" 9042 -f "${AI_RESTORE_DIR}/${_ks}/schema.cql" >/dev/null 2>&1 || true
      fi
      for _csv in "${AI_RESTORE_DIR}/${_ks}"/*.csv; do
        [[ -f "$_csv" ]] || continue
        _tbl="$(basename "$_csv" .csv)"
        if cqlsh "$_R_HOST" 9042 -e "COPY ${_ks}.${_tbl} FROM '${_csv}' WITH HEADER=true;" >/dev/null 2>&1; then
          log_success "Restored ${_ks}.${_tbl}"
          _restored=$((_restored + 1))
        else
          log_warn "Could not restore ${_ks}.${_tbl} (table absent or counter type — skipped)"
        fi
      done
    done
    if [[ "$_restored" -gt 0 ]]; then
      mv "$AI_RESTORE_DIR" "${AI_RESTORE_DIR}.restored-$(date -u +%Y%m%d%H%M%S)" 2>/dev/null || true
      log_success "AI memory restore complete (${_restored} table(s)); snapshot archived alongside"
    else
      log_warn "AI memory snapshot present but no tables restored — left in place at ${AI_RESTORE_DIR}"
    fi
  else
    log_warn "ScyllaDB CQL not reachable — skipping AI memory restore (snapshot kept at ${AI_RESTORE_DIR})"
  fi
fi

# ── Final permission hardening ───────────────────────────────────────────────
# Package installation can chown/chmod state dirs. Re-enforce the permissions
# that matter for non-root tooling (Claude Code MCP, CLI as regular user):
#   - /var/lib/globular and pki/ must be world-traversable (o+x) so that
#     world-readable files inside (ca.crt, ca.pem) are actually reachable.
#   - Private keys stay 400 (owner-read only).
if [[ -d "${STATE_DIR}/pki" ]]; then
  chmod o+x "${STATE_DIR}" "${STATE_DIR}/pki" 2>/dev/null || true
fi

echo ""
echo "╔════════════════════════════════════════════════════════════════╗"
echo "║          ✓ INSTALLATION COMPLETE                               ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""
log_success "Infrastructure ready. Bootstrap mode is active — start the node agent and run bootstrap."
trace_finish "ok" "Day-0 installation complete"

# Resolve the actual node IP and node-agent port from the installed systemd unit.
# Never use loopback — the bootstrap command must use the routable IP
# so the controller can reach back. Never hardcode the port — read it from the unit.
_BOOTSTRAP_IP=$(ip route get 8.8.8.8 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src") print $(i+1); exit}')
_BOOTSTRAP_IP="${_BOOTSTRAP_IP:-$(hostname -I | awk '{print $1}')}"
_NA_UNIT_PORT=$(grep -oP '(?<=--port[= ])\d+' /etc/systemd/system/globular-node-agent.service 2>/dev/null | head -1 || true)
_NA_UNIT_PORT="${_NA_UNIT_PORT:-$(grep -oP '(?<=--port[= ])\d+' /etc/systemd/system/globular-node-agent.service.d/*.conf 2>/dev/null | head -1 || true)}"
_NA_UNIT_PORT="${_NA_UNIT_PORT:-$(ss -ltnp 2>/dev/null | awk '/node_agent_serv/ {split($4,a,":"); p=a[length(a)]; if(p ~ /^[0-9]+$/){print p; exit}}' || true)}"
if [[ -z "$_NA_UNIT_PORT" ]]; then
  log_warn "Could not determine node-agent port from unit file or ss — replace <node-agent-port> in the command below"
  _NA_UNIT_PORT="<node-agent-port>"
fi
_BOOTSTRAP_NODE="${_BOOTSTRAP_IP}:${_NA_UNIT_PORT}"

echo ""
echo "  Next steps:"
echo ""
echo "  1. Start the node agent:"
echo "       sudo systemctl start globular-node-agent"
echo ""
echo "     Verify it is running:"
echo "       sudo systemctl status globular-node-agent"
echo ""
echo "  2. Bootstrap this node (in another terminal):"
echo "       globular cluster bootstrap \\"
echo "         --node ${_BOOTSTRAP_NODE} \\"
echo "         --domain <your-domain> \\"
echo "         --profile core"
echo ""
echo "     Example for a single-node cluster:"
echo "       globular cluster bootstrap \\"
echo "         --node ${_BOOTSTRAP_NODE} \\"
echo "         --domain mycluster.local \\"
echo "         --profile core --profile storage"
echo ""
echo "  After bootstrap, add more nodes with:"
echo "       curl -sfL https://<gateway>:8443/join -k | sudo bash -s -- --token <token>"
echo ""
