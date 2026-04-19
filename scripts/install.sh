#!/usr/bin/env bash
# Globular Installer
#
# Downloads, extracts, and registers the Globular node agent so you can
# bootstrap a cluster in the next step.
#
# Usage:
#   curl -LO https://github.com/globulario/services/releases/download/v0.1.0/globular-0.1.0-linux-amd64.tar.gz
#   tar xzf globular-0.1.0-linux-amd64.tar.gz
#   cd globular-0.1.0-linux-amd64
#   sudo bash install.sh
#
# After installation:
#   sudo systemctl start globular-node-agent
#   globular cluster bootstrap --domain <your-domain> --profile core --profile gateway
#
# See: https://globular.io/docs/operators/installation

set -euo pipefail

INSTALL_PREFIX="/usr/local"
STATE_DIR="/var/lib/globular"
PKG_CACHE_DIR="${STATE_DIR}/packages"
SYSTEMD_DIR="/etc/systemd/system"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BOLD='\033[1m'
NC='\033[0m'

die()     { echo -e "${RED}✗ ERROR: $*${NC}" >&2; exit 1; }
ok()      { echo -e "${GREEN}  ✓ $*${NC}"; }
warn()    { echo -e "${YELLOW}  ⚠ $*${NC}"; }
info()    { echo "  → $*"; }
section() { echo ""; echo -e "${BOLD}━━━ $* ━━━${NC}"; echo ""; }

echo ""
echo -e "${BOLD}╔══════════════════════════════════════════════╗${NC}"
echo -e "${BOLD}║           Globular Installer                 ║${NC}"
echo -e "${BOLD}╚══════════════════════════════════════════════╝${NC}"
echo ""

# ── Prerequisites ──────────────────────────────────────────────────────────
section "Checking Prerequisites"

[[ $EUID -eq 0 ]] || die "Must be run as root: sudo bash install.sh"

ARCH="$(uname -m)"
[[ "${ARCH}" == "x86_64" ]] || die "Only linux/amd64 is supported (detected: ${ARCH})"

command -v systemctl >/dev/null 2>&1 || die "systemd is required"
command -v tar       >/dev/null 2>&1 || die "tar is required"

# Verify the installer was extracted from a proper release tarball
[[ -f "${SCRIPT_DIR}/globular" ]]      || die "globular CLI not found in ${SCRIPT_DIR} — was the tarball extracted correctly?"
[[ -d "${SCRIPT_DIR}/packages" ]]      || die "packages/ directory not found in ${SCRIPT_DIR}"

# Node-agent package must be present
NODE_AGENT_PKG=$(ls "${SCRIPT_DIR}/packages/node-agent_"*"_linux_amd64.tgz" 2>/dev/null | head -1)
[[ -n "${NODE_AGENT_PKG}" ]] || die "node-agent package not found in ${SCRIPT_DIR}/packages/"

# Disk space: need 500 MB free in /var/lib
FREE_MB=$(df -m /var/lib 2>/dev/null | awk 'NR==2 {print $4}' || echo 9999)
[[ "${FREE_MB}" -ge 500 ]] || die "Need at least 500 MB free in /var/lib (have ${FREE_MB} MB)"

ok "Platform: linux/amd64"
ok "systemd present"
ok "Disk space OK (${FREE_MB} MB free)"

# ── Directories ────────────────────────────────────────────────────────────
section "Setting Up Directories"

mkdir -p "${INSTALL_PREFIX}/bin"
mkdir -p "${STATE_DIR}"
mkdir -p "${PKG_CACHE_DIR}"

ok "${STATE_DIR}/ created"
ok "${PKG_CACHE_DIR}/ created"

# ── Install CLI ────────────────────────────────────────────────────────────
section "Installing Globular CLI"

if [[ -f "${INSTALL_PREFIX}/bin/globular" ]]; then
    cp "${INSTALL_PREFIX}/bin/globular" "${INSTALL_PREFIX}/bin/globular.bak"
    info "Backed up existing CLI to globular.bak"
fi

install -m 755 "${SCRIPT_DIR}/globular" "${INSTALL_PREFIX}/bin/globular"
ok "globular → ${INSTALL_PREFIX}/bin/globular"

VERSION=$("${INSTALL_PREFIX}/bin/globular" version 2>/dev/null || echo "unknown")
info "Installed version: ${VERSION}"

# ── Install Node Agent ─────────────────────────────────────────────────────
section "Installing Node Agent"

info "Extracting node_agent_server from $(basename "${NODE_AGENT_PKG}")..."
tar -xOf "${NODE_AGENT_PKG}" bin/node_agent_server > "${INSTALL_PREFIX}/bin/node_agent_server"
chmod 755 "${INSTALL_PREFIX}/bin/node_agent_server"
ok "node_agent_server → ${INSTALL_PREFIX}/bin/node_agent_server"

# ── Copy service packages to local cache ──────────────────────────────────
section "Copying Service Packages"

count=0
for pkg in "${SCRIPT_DIR}/packages/"*.tgz; do
    name=$(basename "${pkg}")
    cp "${pkg}" "${PKG_CACHE_DIR}/${name}"
    count=$((count + 1))
done
ok "${count} packages cached in ${PKG_CACHE_DIR}/"

info "Infrastructure packages (etcd, MinIO, Envoy, ScyllaDB) will be"
info "downloaded automatically during bootstrap."

# ── Create globular system user ────────────────────────────────────────────
section "Creating System User"

if ! id -u globular >/dev/null 2>&1; then
    useradd --system --no-create-home --shell /usr/sbin/nologin \
            --home-dir "${STATE_DIR}" globular
    ok "Created system user: globular"
else
    ok "System user 'globular' already exists"
fi

chown -R globular:globular "${STATE_DIR}" 2>/dev/null || true

# ── Install systemd unit ───────────────────────────────────────────────────
section "Installing systemd Service"

# Write a bootstrap-safe node-agent unit.
# The TLS cert check is omitted here — it gets replaced with the full unit
# when the node-agent installs its own package during Day-0 bootstrap.
cat > "${SYSTEMD_DIR}/globular-node-agent.service" <<UNIT
[Unit]
Description=Globular Node Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
Group=root
WorkingDirectory=${STATE_DIR}/node_agent
ExecStartPre=/bin/sh -c 'mkdir -p ${STATE_DIR}/node_agent'
ExecStart=${INSTALL_PREFIX}/bin/node_agent_server
Restart=always
RestartSec=5
StartLimitIntervalSec=300
StartLimitBurst=10
LimitNOFILE=524288

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable globular-node-agent.service 2>/dev/null
ok "globular-node-agent.service installed and enabled"

# ── Done ───────────────────────────────────────────────────────────────────
section "Installation Complete"

echo "Globular is ready. Bootstrap your first node:"
echo ""
echo "  1. Start the node agent:"
echo "       sudo systemctl start globular-node-agent"
echo ""
echo "     Verify it's running:"
echo "       sudo systemctl status globular-node-agent"
echo ""
echo "  2. Bootstrap the cluster (in another terminal):"
echo "       globular cluster bootstrap \\"
echo "         --node localhost:11000 \\"
echo "         --domain <your-domain> \\"
echo "         --profile core \\"
echo "         --profile gateway"
echo ""
echo "     Example for a single-node homelab:"
echo "       globular cluster bootstrap \\"
echo "         --node localhost:11000 \\"
echo "         --domain mycluster.local \\"
echo "         --profile core --profile gateway --profile storage"
echo ""
echo "  Documentation: https://globular.io/docs/operators/installation"
echo ""
