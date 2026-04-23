#!/usr/bin/env bash
# Globular Installer
#
# Installs all Globular components (infrastructure, services, PKI) from the
# release tarball. After this completes, start the node agent and bootstrap
# the cluster.
#
# Usage:
#   VERSION="1.0.17"
#   curl -LO "https://github.com/globulario/services/releases/download/v${VERSION}/globular-${VERSION}-linux-amd64.tar.gz"
#   curl -LO "https://github.com/globulario/services/releases/download/v${VERSION}/globular-${VERSION}-linux-amd64.tar.gz.sha256"
#   /usr/bin/sha256sum -c "globular-${VERSION}-linux-amd64.tar.gz.sha256"
#   tar xzf "globular-${VERSION}-linux-amd64.tar.gz"
#   cd "globular-${VERSION}-linux-amd64"
#   sudo bash install.sh
#
# After installation, the installer prints the exact bootstrap command with the
# node's routable IP and the actual node-agent port. Example:
#   sudo systemctl start globular-node-agent
#   globular cluster bootstrap \
#     --node <node-ip>:<node-agent-port> \
#     --domain <your-domain> \
#     --profile core \
#     --profile gateway

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
BOLD='\033[1m'
NC='\033[0m'

die()  { echo -e "${RED}✗ ERROR: $*${NC}" >&2; exit 1; }
ok()   { echo -e "${GREEN}  ✓ $*${NC}"; }
info() { echo "  → $*"; }

echo ""
echo -e "${BOLD}╔══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BOLD}║           GLOBULAR INSTALLATION                              ║${NC}"
echo -e "${BOLD}╚══════════════════════════════════════════════════════════════╝${NC}"
echo ""

# ── Prerequisites ─────────────────────────────────────────────────────────────
[[ $EUID -eq 0 ]] || die "Must be run as root: sudo bash install.sh"

ARCH="$(uname -m)"
[[ "${ARCH}" == "x86_64" ]] || die "Only linux/amd64 is supported (detected: ${ARCH})"

command -v systemctl >/dev/null 2>&1 || die "systemd is required"
command -v tar       >/dev/null 2>&1 || die "tar is required"

[[ -f "${SCRIPT_DIR}/globular" ]]           || die "globular CLI not found — was the tarball extracted correctly?"
[[ -f "${SCRIPT_DIR}/globular-installer" ]] || die "globular-installer not found — was the tarball extracted correctly?"
[[ -d "${SCRIPT_DIR}/packages" ]]           || die "packages/ directory not found"
[[ -f "${SCRIPT_DIR}/scripts/install-day0.sh" ]] || die "scripts/install-day0.sh not found"

# ── Environment for install-day0.sh ──────────────────────────────────────────
# PKG_DIR: where .tgz packages live in the tarball
export PKG_DIR="${SCRIPT_DIR}/packages"

# INSTALLER_BIN: the globular-installer Go binary bundled in the tarball
export INSTALLER_BIN="${SCRIPT_DIR}/globular-installer"

# MINIO_DATA_DIR: default to /var/lib/globular/minio/data (non-interactive)
# Override with: sudo MINIO_DATA_DIR=/data/minio bash install.sh
export MINIO_DATA_DIR="${MINIO_DATA_DIR:-/var/lib/globular/minio/data}"

# STATE_DIR: canonical Globular state root used by Day-0 scripts
# (some installer script revisions read it before setting a default).
export STATE_DIR="${STATE_DIR:-/var/lib/globular}"

# GLOBULAR_DOMAIN: internal cluster domain (used for etcd/MinIO config keys)
# The external/operator domain is set at bootstrap time via --domain flag
export GLOBULAR_DOMAIN="${GLOBULAR_DOMAIN:-globular.internal}"

# ── Install CLI ───────────────────────────────────────────────────────────────
info "Installing globular CLI..."
install -m 755 "${SCRIPT_DIR}/globular" /usr/local/bin/globular
ok "globular → /usr/local/bin/globular"

VERSION=$("${SCRIPT_DIR}/globular" version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || echo "unknown")
info "Release version: ${VERSION}"

# ── Run Day-0 installation ────────────────────────────────────────────────────
echo ""
info "Starting Day-0 installation..."
echo ""

"${SCRIPT_DIR}/scripts/install-day0.sh"

# Print a corrected bootstrap command using a detected node-agent gRPC port.
# This supersedes stale script output that may omit the port suffix.
NODE_IP="$(ip route get 8.8.8.8 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src"){print $(i+1); exit}}')"
if [[ -z "${NODE_IP}" ]]; then
  NODE_IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
fi
if [[ -z "${NODE_IP}" ]]; then
  NODE_IP="127.0.0.1"
fi

NODE_AGENT_PORT="$(ss -ltnp 2>/dev/null | awk '/node_agent_serv/ {split($4,a,":"); p=a[length(a)]; if(p ~ /^[0-9]+$/){print p}}' | grep -E '^11000$' | head -n1)"
if [[ -z "${NODE_AGENT_PORT}" ]]; then
  NODE_AGENT_PORT="$(ss -ltnp 2>/dev/null | awk '/node_agent_serv/ {split($4,a,":"); p=a[length(a)]; if(p ~ /^[0-9]+$/){print p}}' | head -n1)"
fi
if [[ -z "${NODE_AGENT_PORT}" ]]; then
  NODE_AGENT_PORT="11000"
fi

echo ""
echo "Corrected bootstrap command:"
echo "  globular cluster bootstrap \\"
echo "    --node ${NODE_IP}:${NODE_AGENT_PORT} \\"
echo "    --domain <your-domain> \\"
echo "    --profile core \\"
echo "    --profile gateway"
echo ""
