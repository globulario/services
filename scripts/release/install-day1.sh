#!/usr/bin/env bash
set -euo pipefail

# Globular Day-1 Node Join Script
#
# Run this script ON THE NEW NODE (nuc/dell) after:
#   1. Transferring the release tarball to the new node
#   2. Extracting the tarball
#
# Usage (run as root on the new node):
#   sudo ./install-day1.sh \
#     --controller  10.0.0.63:12000 \
#     --join-token  0ce240e0-a8fa-460a-8175-2b97bee66b94 \
#     [--domain     globular.internal] \
#     [--profiles   core,control-plane,storage] \
#     [--etcd-peer   https://10.0.0.63:2379] \
#     [--minio-addr  10.0.0.63:9000] \
#     [--minio-data-dir /var/lib/globular/minio/data]
#
# The script:
#   1. Bootstraps trust from the controller HTTPS endpoint
#   2. Reads MinIO credentials from etcd over HTTPS
#   3. Downloads cluster PKI material from MinIO on the controller node
#   4. Joins this node to the existing etcd cluster
#   5. Installs and starts the node-agent
#   6. Calls `globular cluster join` to trigger the node.join workflow

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALLER_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
STATE_DIR="/var/lib/globular"
PKG_DIR="$INSTALLER_ROOT/packages"

INSTALLER_BIN="$INSTALLER_ROOT/bin/globular-installer"
if [[ ! -x "$INSTALLER_BIN" ]]; then
  INSTALLER_BIN="$(command -v globular-installer || true)"
fi

# ── Logging ────────────────────────────────────────────────────────────────────
die()        { echo "  ✗ ERROR: $*" >&2; exit 1; }
log_info()   { echo "  → $*"; }
log_success(){ echo "  ✓ $*"; }
log_step()   { echo ""; echo "━━━ $* ━━━"; }
log_substep(){ echo "  • $*"; }

extract_json_string() {
  local json="$1"
  local key="$2"
  sed -n "s/.*\"${key}\"[[:space:]]*:[[:space:]]*\"\\([^\"]*\\)\".*/\\1/p" <<< "$json" | head -n1
}

bootstrap_gateway_ca() {
  local host="$1"
  local port="$2"
  local out="$3"
  local tmp
  tmp="$(mktemp)"
  if ! timeout 15 openssl s_client -showcerts -connect "${host}:${port}" </dev/null >"$tmp" 2>/dev/null; then
    rm -f "$tmp"
    return 1
  fi
  awk '
    /-----BEGIN CERTIFICATE-----/ { capture=1; cert=$0 ORS; next }
    capture { cert=cert $0 ORS }
    /-----END CERTIFICATE-----/ { capture=0; last=cert; cert="" }
    END { if (last != "") printf "%s", last }
  ' "$tmp" >"$out"
  rm -f "$tmp"
  [[ -s "$out" ]]
}

# ── Argument parsing ───────────────────────────────────────────────────────────
CONTROLLER_ADDR=""       # HOST:PORT of existing cluster controller (port 12000)
ETCD_PEER=""             # https://HOST:2379 of an existing etcd peer
MINIO_ADDR=""            # HOST:PORT of existing MinIO (port 9000)
JOIN_TOKEN=""
DOMAIN="globular.internal"
PROFILES="core,control-plane,storage"
MINIO_DATA_DIR="/var/lib/globular/minio/data"
FORCE_REINSTALL="0"
MINIO_ACCESS=""
MINIO_SECRET=""
CA_KEY_FILE=""           # Optional: local path to cluster CA private key (skips MinIO download)

while [[ $# -gt 0 ]]; do
  case "$1" in
    --controller)    CONTROLLER_ADDR="$2"; shift 2 ;;
    --etcd-peer)     ETCD_PEER="$2";       shift 2 ;;
    --minio-addr)    MINIO_ADDR="$2";      shift 2 ;;
    --join-token)    JOIN_TOKEN="$2";      shift 2 ;;
    --domain)        DOMAIN="$2";          shift 2 ;;
    --profiles)      PROFILES="$2";        shift 2 ;;
    --minio-data-dir)MINIO_DATA_DIR="$2";  shift 2 ;;
    --minio-access-key) MINIO_ACCESS="$2"; shift 2 ;;
    --minio-secret-key) MINIO_SECRET="$2"; shift 2 ;;
    --ca-key)        CA_KEY_FILE="$2";     shift 2 ;;
    --force)         FORCE_REINSTALL=1;    shift   ;;
    *) die "Unknown argument: $1" ;;
  esac
done

# ── Validate required args ─────────────────────────────────────────────────────
[[ -n "$CONTROLLER_ADDR" ]] || die "--controller HOST:PORT is required (e.g. 10.0.0.63:12000)"
[[ -n "$JOIN_TOKEN"       ]] || die "--join-token TOKEN is required"
CONTROLLER_HOST="${CONTROLLER_ADDR%%:*}"
if [[ -z "$ETCD_PEER" ]]; then
  ETCD_PEER="https://${CONTROLLER_HOST}:2379"
fi
if [[ -z "$MINIO_ADDR" ]]; then
  MINIO_ADDR="${CONTROLLER_HOST}:9000"
fi

# ── Root check ─────────────────────────────────────────────────────────────────
[[ $EUID -eq 0 ]] || die "Must be run as root (sudo)"

# ── Package dir check ──────────────────────────────────────────────────────────
[[ -d "$PKG_DIR"      ]] || die "Package directory not found: $PKG_DIR"
[[ -n "$INSTALLER_BIN" && -x "$INSTALLER_BIN" ]] || \
  die "Installer binary not found; set INSTALLER_BIN or build ./bin/globular-installer"

TOLERATE_ALREADY_INSTALLED="1"
FORCE_FLAG=""
[[ "$FORCE_REINSTALL" == "1" ]] && FORCE_FLAG="--force"
MINIO_DATA_DIR_FLAG="--minio-data-dir $MINIO_DATA_DIR"

# ── Install helper ─────────────────────────────────────────────────────────────
detect_install_cmd() {
  if "$INSTALLER_BIN" pkg install --help >/dev/null 2>&1; then
    echo "pkg_install_arg"; return
  fi
  if "$INSTALLER_BIN" install --help >/dev/null 2>&1; then
    echo "install_arg"; return
  fi
  die "Could not detect install command form for $INSTALLER_BIN"
}

INSTALL_MODE="$(detect_install_cmd)"

run_install() {
  local pkgfile="$1"
  local pkgname
  pkgname="$(basename "$pkgfile" .tgz | sed 's/_linux_amd64$//')"
  local out rc

  log_substep "Installing $pkgname..."
  set +e
  case "$INSTALL_MODE" in
    pkg_install_arg) out="$("$INSTALLER_BIN" pkg install $FORCE_FLAG $MINIO_DATA_DIR_FLAG "$pkgfile" 2>&1)"; rc=$? ;;
    install_arg)     out="$("$INSTALLER_BIN" install $FORCE_FLAG $MINIO_DATA_DIR_FLAG "$pkgfile" 2>&1)"; rc=$? ;;
    *) die "Unknown install mode: $INSTALL_MODE" ;;
  esac
  set -e

  if [[ $rc -ne 0 ]]; then
    if [[ "$TOLERATE_ALREADY_INSTALLED" == "1" ]] && \
       echo "$out" | grep -qiE "already installed|exists|is installed"; then
      log_success "$pkgname (already installed)"
      return 0
    fi
    echo "$out" >&2
    die "Failed to install $pkgname"
  fi
  log_success "$pkgname installed"
}

install_list() {
  local pkg_array=("$@")
  for f in "${pkg_array[@]}"; do
    local path="$PKG_DIR/$f"
    if [[ ! -f "$path" ]]; then
      log_substep "Warning: package not found, skipping: $path"
      continue
    fi
    run_install "$path"
  done
}

# ── Banner ─────────────────────────────────────────────────────────────────────
echo ""
echo "╔════════════════════════════════════════════════════════════════╗"
echo "║          GLOBULAR DAY-1 NODE JOIN                              ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""
log_info "Controller:    $CONTROLLER_ADDR"
log_info "etcd peer:     $ETCD_PEER"
log_info "MinIO addr:    $MINIO_ADDR"
log_info "Domain:        $DOMAIN"
log_info "Profiles:      $PROFILES"
log_info "MinIO data:    $MINIO_DATA_DIR"
log_info "Package dir:   $PKG_DIR"
echo ""

# ─── Detect local node identity ────────────────────────────────────────────────
NODE_IP="$(ip route get 8.8.8.8 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src") print $(i+1); exit}')"
NODE_IP="${NODE_IP:-$(hostname -I 2>/dev/null | awk '{print $1}')}"
NODE_HOSTNAME="$(hostname -s)"
NODE_FQDN="${NODE_HOSTNAME}.${DOMAIN}"

[[ -n "$NODE_IP" ]] || die "Could not detect local node IP"
log_info "Node:          $NODE_HOSTNAME ($NODE_IP)"

# ─── Step 1: Install bootstrap CLI tools ──────────────────────────────────────
# We need mc (to download the CA) and etcdctl (to join the etcd cluster).
# Install these first from the tarball — no network or PKI required.
log_step "Bootstrap CLI Tools (mc + etcdctl)"

MC_PKGS=("mc_0.0.1_linux_amd64.tgz" "etcdctl_3.5.14_linux_amd64.tgz")
for pkg in "${MC_PKGS[@]}"; do
  if [[ -f "$PKG_DIR/$pkg" ]]; then
    run_install "$PKG_DIR/$pkg"
  else
    log_substep "Warning: $pkg not found in $PKG_DIR"
  fi
done

MC_BIN="/usr/local/bin/mc"
ETCDCTL_BIN="/usr/local/bin/etcdctl"

[[ -x "$MC_BIN" ]]     || die "mc not available after bootstrap install"
[[ -x "$ETCDCTL_BIN" ]] || die "etcdctl not available after bootstrap install"

# ─── Step 2: Bootstrap trust from controller HTTPS ────────────────────────────
log_step "Bootstrap Trust from Controller HTTPS"

PKI_DIR="/var/lib/globular/pki"
mkdir -p "$PKI_DIR"
MINIO_CRED_FILE="${STATE_DIR}/minio/credentials"

bootstrap_gateway_ca "$CONTROLLER_HOST" 8443 "${PKI_DIR}/ca.crt" || \
  die "Failed to extract cluster CA certificate from ${CONTROLLER_HOST}:8443"
cp "${PKI_DIR}/ca.crt" "${PKI_DIR}/ca.pem"
chmod 444 "${PKI_DIR}/ca.crt" "${PKI_DIR}/ca.pem"
log_success "Cluster CA bootstrapped from controller HTTPS"

# ─── Step 3: Read MinIO config from etcd ──────────────────────────────────────
log_step "Read MinIO Config from etcd"

MINIO_CFG_JSON="$("$ETCDCTL_BIN" \
  --endpoints="$ETCD_PEER" \
  --cacert="${PKI_DIR}/ca.crt" \
  get /globular/cluster/minio/config --print-value-only 2>/dev/null | tail -n1)" || true

[[ -n "$MINIO_CFG_JSON" ]] || die "Failed to read /globular/cluster/minio/config from etcd"

if [[ -z "$MINIO_ACCESS" ]]; then
  MINIO_ACCESS="$(extract_json_string "$MINIO_CFG_JSON" "access_key")"
fi
if [[ -z "$MINIO_SECRET" ]]; then
  MINIO_SECRET="$(extract_json_string "$MINIO_CFG_JSON" "secret_key")"
fi
[[ -n "$MINIO_ACCESS" ]] || die "MinIO access key missing from etcd config"
[[ -n "$MINIO_SECRET" ]] || die "MinIO secret key missing from etcd config"
printf '%s:%s' "$MINIO_ACCESS" "$MINIO_SECRET" > "$MINIO_CRED_FILE"
chmod 600 "$MINIO_CRED_FILE"
log_success "MinIO credentials loaded from etcd and saved to $MINIO_CRED_FILE"

# ─── Step 4: Bootstrap cluster PKI (CA key from MinIO) ────────────────────────
log_step "Bootstrap Cluster PKI"

# Configure mc alias pointing at the controller's MinIO (using IP, not DNS — DNS isn't up yet)
MINIO_ALIAS="cluster"
if ! "$MC_BIN" alias set "$MINIO_ALIAS" "https://${MINIO_ADDR}" \
    "$MINIO_ACCESS" "$MINIO_SECRET" --insecure >/dev/null 2>&1; then
  die "mc alias set failed — check MinIO address ($MINIO_ADDR) and credentials"
fi
log_substep "mc alias set for $MINIO_ADDR (user=$MINIO_ACCESS)"

# CA private key: prefer --ca-key argument, then download from MinIO
if [[ -n "$CA_KEY_FILE" ]]; then
  [[ -f "$CA_KEY_FILE" ]] || die "--ca-key file not found: $CA_KEY_FILE"
  cp "$CA_KEY_FILE" "${PKI_DIR}/ca.key"
  chmod 400 "${PKI_DIR}/ca.key"
  log_success "Cluster CA private key loaded from $CA_KEY_FILE"
elif "$MC_BIN" cp "${MINIO_ALIAS}/globular-config/pki/ca.key" \
    "${PKI_DIR}/ca.key" --insecure >/dev/null 2>&1; then
  chmod 400 "${PKI_DIR}/ca.key"
  log_success "Cluster CA private key downloaded from MinIO"
else
  die "Failed to download CA key from MinIO (${MINIO_ADDR}/globular-config/pki/ca.key)"
fi

# ─── Step 5: Generate service certificates using the cluster CA ────────────────
log_step "TLS Certificate Bootstrap (using cluster CA)"

# setup-tls.sh is idempotent: if the CA already exists and is RSA, it reuses it.
# We downloaded the CA above, so it will skip CA generation and only issue a
# fresh service cert signed by the cluster CA.
if [[ -x "$SCRIPT_DIR/setup-tls.sh" ]]; then
  "$SCRIPT_DIR/setup-tls.sh" || die "TLS setup failed"
  log_success "Service certificates generated (signed by cluster CA)"
else
  die "setup-tls.sh not found"
fi

# ─── Step 6: Generate user client certificates ─────────────────────────────────
log_step "Client Certificate Generation"
if [[ -x "$SCRIPT_DIR/generate-user-client-cert.sh" ]]; then
  if "$SCRIPT_DIR/generate-user-client-cert.sh" root 2>&1; then
    log_success "Root client certificates generated"
  else
    log_substep "Warning: root client certificate generation failed (CLI may not work)"
  fi

  # Also generate for the invoking user
  ORIGINAL_USER=""
  DETECTED_USER=$(stat -c '%U' "$SCRIPT_DIR" 2>/dev/null || echo "")
  [[ -n "$DETECTED_USER" && "$DETECTED_USER" != "root" ]] && ORIGINAL_USER="$DETECTED_USER"

  if [[ -n "$ORIGINAL_USER" ]]; then
    if "$SCRIPT_DIR/generate-user-client-cert.sh" "$ORIGINAL_USER" >/dev/null 2>&1; then
      if [[ -x "$SCRIPT_DIR/fix-client-cert-ownership.sh" ]]; then
        "$SCRIPT_DIR/fix-client-cert-ownership.sh" "$ORIGINAL_USER" >/dev/null 2>&1 || true
      fi
      log_success "User ($ORIGINAL_USER) client certificates generated"
    fi
  fi
fi

# ─── Step 7: Install etcd package (binary + systemd unit only — not started yet)
log_step "Install etcd (not started yet)"
ETCD_PKG="$PKG_DIR/etcd_3.5.14_linux_amd64.tgz"
[[ -f "$ETCD_PKG" ]] || die "etcd package not found: $ETCD_PKG"
run_install "$ETCD_PKG"

# Stop etcd if the package post-install started it (we need to configure it first)
if systemctl is-active --quiet globular-etcd.service 2>/dev/null; then
  log_substep "Stopping auto-started etcd for cluster-join reconfiguration..."
  systemctl stop globular-etcd.service 2>/dev/null || true
fi
# Clean any etcd data dir so we start fresh as a new member
ETCD_DATA_DIR="/var/lib/globular/etcd"
if [[ -d "${ETCD_DATA_DIR}/member" ]]; then
  log_substep "Removing stale etcd data dir for clean cluster join..."
  rm -rf "${ETCD_DATA_DIR}/member"
fi

# ─── Step 8: Register this node with the existing etcd cluster ─────────────────
log_step "Register etcd Peer"

CA_CERT="$PKI_DIR/ca.crt"
SVC_CERT="$PKI_DIR/issued/services/service.crt"
SVC_KEY="$PKI_DIR/issued/services/service.key"

# Wait for the CA files to be readable (setup-tls.sh may have just written them)
for f in "$CA_CERT" "$SVC_CERT" "$SVC_KEY"; do
  [[ -f "$f" ]] || die "Required cert file missing after TLS setup: $f"
done

log_substep "Adding $NODE_HOSTNAME to etcd cluster via $ETCD_PEER ..."

# etcdctl member add returns env vars we need for the join config
MEMBER_ADD_OUTPUT="$("$ETCDCTL_BIN" \
  --endpoints="$ETCD_PEER" \
  --cacert="$CA_CERT" \
  --cert="$SVC_CERT" \
  --key="$SVC_KEY" \
  member add "$NODE_HOSTNAME" \
  --peer-urls="https://${NODE_IP}:2380" 2>&1)" || {
    # member may already be added from a previous aborted attempt — check
    if echo "$MEMBER_ADD_OUTPUT" | grep -qi "already exists\|duplicate\|peerURL"; then
      log_substep "Member already registered — proceeding with existing membership"
    else
      echo "$MEMBER_ADD_OUTPUT" >&2
      die "etcd member add failed"
    fi
  }

log_substep "etcd member add output:"
echo "$MEMBER_ADD_OUTPUT" | sed 's/^/    /'

# Parse the environment variables from the output
ETCD_INITIAL_CLUSTER="$(echo "$MEMBER_ADD_OUTPUT" | grep '^ETCD_INITIAL_CLUSTER=' | cut -d= -f2- | tr -d '"')"
ETCD_INITIAL_CLUSTER_STATE="$(echo "$MEMBER_ADD_OUTPUT" | grep '^ETCD_INITIAL_CLUSTER_STATE=' | cut -d= -f2- | tr -d '"')"
ETCD_INITIAL_CLUSTER_TOKEN="$(echo "$MEMBER_ADD_OUTPUT" | grep '^ETCD_INITIAL_CLUSTER_TOKEN=' | cut -d= -f2- | tr -d '"')"

# Fallback: if not parsed from output, build from existing members list
if [[ -z "$ETCD_INITIAL_CLUSTER" ]]; then
  log_substep "Building initial-cluster from member list..."
  MEMBER_LIST="$("$ETCDCTL_BIN" \
    --endpoints="$ETCD_PEER" \
    --cacert="$CA_CERT" \
    --cert="$SVC_CERT" \
    --key="$SVC_KEY" \
    member list --write-out=fields 2>/dev/null)" || true

  # Build comma-separated NAME=PEER_URL list
  ETCD_INITIAL_CLUSTER="$(echo "$MEMBER_LIST" | \
    awk '/PeerURLs/ { url=$NF; sub(/\[/, "", url); sub(/\]/, "", url) }
         /Name/     { name=$NF }
         /PeerURLs/ && name != "" { print name "=" url; name="" }' | \
    paste -sd, -)"

  [[ -n "$ETCD_INITIAL_CLUSTER" ]] || \
    die "Could not determine ETCD_INITIAL_CLUSTER — check etcd connectivity"
  ETCD_INITIAL_CLUSTER_STATE="${ETCD_INITIAL_CLUSTER_STATE:-existing}"
  ETCD_INITIAL_CLUSTER_TOKEN="${ETCD_INITIAL_CLUSTER_TOKEN:-globular-etcd}"
fi

log_substep "ETCD_INITIAL_CLUSTER=$ETCD_INITIAL_CLUSTER"
log_substep "ETCD_INITIAL_CLUSTER_STATE=$ETCD_INITIAL_CLUSTER_STATE"

# ─── Step 9: Write etcd join config ────────────────────────────────────────────
log_step "Configure etcd for Cluster Join"

ETCD_CONFIG_DIR="/var/lib/globular/config"
ETCD_CONFIG="$ETCD_CONFIG_DIR/etcd.yaml"
mkdir -p "$ETCD_CONFIG_DIR"
mkdir -p "$ETCD_DATA_DIR"

cat > "$ETCD_CONFIG" <<EOF
# Generated by install-day1.sh — Day-1 cluster join for $NODE_HOSTNAME
name: ${NODE_HOSTNAME}
data-dir: ${ETCD_DATA_DIR}

listen-peer-urls: https://${NODE_IP}:2380
listen-client-urls: https://${NODE_IP}:2379

initial-advertise-peer-urls: https://${NODE_IP}:2380
advertise-client-urls: https://${NODE_IP}:2379

initial-cluster: ${ETCD_INITIAL_CLUSTER}
initial-cluster-state: ${ETCD_INITIAL_CLUSTER_STATE:-existing}
initial-cluster-token: ${ETCD_INITIAL_CLUSTER_TOKEN:-globular-etcd}

client-transport-security:
  cert-file: ${SVC_CERT}
  key-file: ${SVC_KEY}
  trusted-ca-file: ${CA_CERT}
  client-cert-auth: true

peer-transport-security:
  cert-file: ${SVC_CERT}
  key-file: ${SVC_KEY}
  trusted-ca-file: ${CA_CERT}
  client-cert-auth: true

log-level: warn
EOF

chmod 644 "$ETCD_CONFIG"
if id globular >/dev/null 2>&1; then
  chown globular:globular "$ETCD_CONFIG"
  chown -R globular:globular "$ETCD_DATA_DIR" 2>/dev/null || true
fi
log_success "etcd config written: $ETCD_CONFIG"

# Patch the systemd unit to use this config file (if the unit uses --config-file)
ETCD_UNIT="/etc/systemd/system/globular-etcd.service"
if [[ -f "$ETCD_UNIT" ]]; then
  systemctl daemon-reload
fi

# ─── Step 10: TLS ownership ────────────────────────────────────────────────────
log_step "TLS Ownership Fix"
if id globular >/dev/null 2>&1; then
  chown -R globular:globular "$PKI_DIR"
  mkdir -p /var/lib/globular/.minio/certs
  chown -R globular:globular /var/lib/globular/.minio 2>/dev/null || true
  log_success "TLS files owned by globular:globular"
fi

# ─── Step 11: Start etcd (joining cluster) ─────────────────────────────────────
log_step "Start etcd (joining cluster)"
systemctl daemon-reload
systemctl enable globular-etcd.service 2>/dev/null || true
systemctl start globular-etcd.service || die "Failed to start etcd"

# Wait for etcd to join and accept client connections
log_substep "Waiting for etcd to accept local client connections..."
ETCD_READY=0
for i in $(seq 1 60); do
  if "$ETCDCTL_BIN" \
      --endpoints="https://${NODE_IP}:2379" \
      --cacert="$CA_CERT" --cert="$SVC_CERT" --key="$SVC_KEY" \
      endpoint health >/dev/null 2>&1; then
    ETCD_READY=1
    break
  fi
  sleep 2
done
[[ $ETCD_READY -eq 1 ]] || die "etcd failed to join cluster after 120s — check: journalctl -u globular-etcd"
log_success "etcd joined cluster (took $((i*2))s)"

# ─── Step 12: Install node-agent ───────────────────────────────────────────────
log_step "Install node-agent"
NODE_AGENT_PKG="$PKG_DIR/node-agent_0.0.1_linux_amd64.tgz"
[[ -f "$NODE_AGENT_PKG" ]] || die "node-agent package not found: $NODE_AGENT_PKG"
run_install "$NODE_AGENT_PKG"

# ─── Step 13: Globular config (Protocol=https) ─────────────────────────────────
log_step "Globular Configuration (Protocol=https)"
if [[ -x "$SCRIPT_DIR/setup-config.sh" ]]; then
  "$SCRIPT_DIR/setup-config.sh"
  log_success "Configuration set to HTTPS"
fi

# ─── Step 14: /etc/hosts bootstrap entries ────────────────────────────────────
log_step "/etc/hosts Bootstrap Entries"

# Add this node's own FQDN
if ! grep -qF "$NODE_FQDN" /etc/hosts 2>/dev/null; then
  echo "$NODE_IP  $NODE_FQDN  $NODE_HOSTNAME" >> /etc/hosts
  log_substep "Added $NODE_IP → $NODE_FQDN"
fi

# Add controller node FQDN (needed for service discovery before DNS is up)
CONTROLLER_FQDN="${CONTROLLER_HOST}.${DOMAIN}"
if ! grep -qF "$CONTROLLER_FQDN" /etc/hosts 2>/dev/null; then
  echo "$CONTROLLER_HOST  $CONTROLLER_FQDN  ${CONTROLLER_HOST%%.*}" >> /etc/hosts
  log_substep "Added $CONTROLLER_HOST → $CONTROLLER_FQDN"
fi

# Add minio.<domain> pointing at controller (MinIO lives on the controller node at Day-1)
MINIO_FQDN="minio.${DOMAIN}"
MINIO_HOST="${MINIO_ADDR%%:*}"
if ! grep -qF "$MINIO_FQDN" /etc/hosts 2>/dev/null; then
  echo "$MINIO_HOST  $MINIO_FQDN" >> /etc/hosts
  log_substep "Added $MINIO_HOST → $MINIO_FQDN"
fi

log_success "/etc/hosts updated"

# ─── Step 15: System DNS resolver ─────────────────────────────────────────────
log_step "System Resolver Configuration"
if [[ -x "$SCRIPT_DIR/configure-resolver.sh" ]]; then
  set +e
  "$SCRIPT_DIR/configure-resolver.sh" 2>&1
  resolver_rc=$?
  set -e
  if [[ $resolver_rc -ne 0 ]]; then
    log_substep "Warning: configure-resolver.sh failed (DNS will resolve via /etc/hosts until DNS service is installed)"
  else
    log_success "System resolver configured for ${DOMAIN}"
  fi
else
  log_substep "configure-resolver.sh not found — DNS resolver config skipped"
fi

# ─── Step 16: Start node-agent ─────────────────────────────────────────────────
log_step "Start node-agent"

# node-agent waits for $SVC_CERT to exist (ExecStartPre) — it does, we generated it above.
systemctl enable globular-node-agent.service 2>/dev/null || true
systemctl start globular-node-agent.service || die "Failed to start node-agent"

# Wait for node-agent to be ready on :11000
log_substep "Waiting for node-agent on :11000..."
NODE_AGENT_READY=0
for i in $(seq 1 30); do
  if timeout 2 bash -c "echo >/dev/tcp/${NODE_IP}/11000" 2>/dev/null; then
    NODE_AGENT_READY=1
    break
  fi
  sleep 2
done
[[ $NODE_AGENT_READY -eq 1 ]] || die "node-agent not ready after 60s — check: journalctl -u globular-node-agent"
log_success "node-agent ready on :11000"

# ─── Step 17: Join the cluster ─────────────────────────────────────────────────
log_step "Cluster Join (triggering node.join workflow)"

GLOBULAR_CLI="/usr/lib/globular/bin/globularcli"
[[ -x "$GLOBULAR_CLI" ]] || GLOBULAR_CLI="$(command -v globular 2>/dev/null || true)"
[[ -n "$GLOBULAR_CLI" && -x "$GLOBULAR_CLI" ]] || die "globular CLI not found (is globular-cli package installed?)"

# Install globular-cli if not yet present (it may have been included already)
if [[ ! -x "$GLOBULAR_CLI" ]]; then
  CLI_PKG="$PKG_DIR/globular-cli_0.0.1_linux_amd64.tgz"
  [[ -f "$CLI_PKG" ]] && run_install "$CLI_PKG"
  GLOBULAR_CLI="$(command -v globular 2>/dev/null || true)"
fi

[[ -n "$GLOBULAR_CLI" && -x "$GLOBULAR_CLI" ]] || die "globular CLI still not found after install"

log_substep "Running: globular cluster join"
log_substep "  Controller: $CONTROLLER_ADDR"
log_substep "  Node-agent: ${NODE_IP}:11000"
log_substep "  Profiles:   $PROFILES"

# Convert comma-separated profiles to repeated --profile flags
PROFILE_FLAGS=""
IFS=',' read -ra PROFILE_ARRAY <<< "$PROFILES"
for p in "${PROFILE_ARRAY[@]}"; do
  PROFILE_FLAGS="$PROFILE_FLAGS --profile $p"
done

set +e
JOIN_OUTPUT="$("$GLOBULAR_CLI" cluster join \
  --controller "$CONTROLLER_ADDR" \
  --node "${NODE_IP}:11000" \
  --join-token "$JOIN_TOKEN" \
  $PROFILE_FLAGS \
  2>&1)"
JOIN_RC=$?
set -e

echo "$JOIN_OUTPUT" | sed 's/^/  [join] /'

if [[ $JOIN_RC -ne 0 ]]; then
  echo ""
  log_substep "Warning: cluster join returned non-zero (rc=$JOIN_RC)"
  log_substep "The node-agent and etcd are running — the controller may still approve the join."
  log_substep "Check status: globular cluster nodes"
  log_substep "Or check logs: journalctl -u globular-node-agent -f"
else
  log_success "Cluster join request sent — node.join workflow triggered"
fi

# ─── Summary ───────────────────────────────────────────────────────────────────
echo ""
echo "╔════════════════════════════════════════════════════════════════╗"
echo "║          DAY-1 BOOTSTRAP COMPLETE                              ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""
log_info "Node $NODE_HOSTNAME ($NODE_IP) has:"
log_info "  ✓ etcd joined the cluster"
log_info "  ✓ node-agent running on :11000"
log_info "  ✓ node.join workflow triggered"
echo ""
log_info "The node.join workflow will install all remaining packages."
log_info "Monitor progress from any cluster node:"
echo ""
echo "    globular workflow list --controller $CONTROLLER_ADDR"
echo "    globular cluster nodes --controller $CONTROLLER_ADDR"
echo ""
log_info "Logs on this node:"
echo ""
echo "    journalctl -u globular-node-agent -f"
echo "    journalctl -u globular-etcd -f"
echo ""
