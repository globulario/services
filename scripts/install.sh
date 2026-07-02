#!/usr/bin/env bash
# Globular release wrapper entrypoint.
#
# services owns release publication and ships this wrapper in the release
# tarball. The actual Day-0 installation workflow authority remains in
# globular-installer/scripts/install-day0.sh.
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
# Optional — set the founding node's profiles (comma-separated). The quorum
# profiles (control-plane,core,storage) are always enforced; this adds explicit
# workload profiles from day-0. Example:
#   sudo FOUNDING_PROFILES=core,gateway bash install.sh
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

PKG_DIR="${SCRIPT_DIR}/packages"
INSTALLER_BIN="${SCRIPT_DIR}/globular-installer"
MINIO_DATA_DIR="/var/lib/globular/minio/data"
STATE_DIR="/var/lib/globular"
GLOBULAR_DOMAIN="globular.internal"

# Founding-node profiles, forwarded to install-day0.sh (comma-separated). The
# controller always enforces the quorum trio (control-plane,core,storage).
# Extra workload profiles must be explicit operator intent.
# Override via the environment, e.g.:
#   sudo FOUNDING_PROFILES=core,gateway bash install.sh
export FOUNDING_PROFILES="${FOUNDING_PROFILES:-core}"

release_version_from_bundle() {
  local index="${SCRIPT_DIR}/release-index.json"
  [[ -f "${index}" ]] || return 1
  python3 - "${index}" <<'PYEOF'
import json, sys
path = sys.argv[1]
try:
    data = json.load(open(path, "r", encoding="utf-8"))
except Exception:
    raise SystemExit(1)
value = (data.get("platform_release") or data.get("release_tag") or "").strip()
if value.lower().startswith("v"):
    value = value[1:]
if not value:
    raise SystemExit(1)
print(value)
PYEOF
}

print_bootstrap_profiles() {
  local profile trimmed
  local -a bootstrap_profiles=("core")
  IFS=',' read -ra _profiles <<< "${FOUNDING_PROFILES:-core}"
  for profile in "${_profiles[@]}"; do
    trimmed="$(echo "${profile}" | xargs)"
    case "${trimmed}" in
      ""|"core"|"control-plane"|"storage")
        continue
        ;;
    esac
    bootstrap_profiles+=("${trimmed}")
  done
  local idx last
  last=$((${#bootstrap_profiles[@]} - 1))
  for idx in "${!bootstrap_profiles[@]}"; do
    if [[ "${idx}" -lt "${last}" ]]; then
      echo "    --profile ${bootstrap_profiles[$idx]} \\"
    else
      echo "    --profile ${bootstrap_profiles[$idx]}"
    fi
  done
}

mkdir -p "${SCRIPT_DIR}/bin" "${SCRIPT_DIR}/internal/assets"
ln -sf "${INSTALLER_BIN}" "${SCRIPT_DIR}/bin/globular-installer"
ln -sfn "${PKG_DIR}" "${SCRIPT_DIR}/internal/assets/packages"

# ── Install CLI ───────────────────────────────────────────────────────────────
info "Installing globular CLI..."
install -m 755 "${SCRIPT_DIR}/globular" /usr/local/bin/globular
ok "globular → /usr/local/bin/globular"

VERSION="$(release_version_from_bundle 2>/dev/null || true)"
if [[ -z "${VERSION}" ]]; then
  VERSION=$("${SCRIPT_DIR}/globular" version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || echo "unknown")
fi
info "Release version: ${VERSION}"

# ── Run Day-0 installation ────────────────────────────────────────────────────
echo ""
info "Starting Day-0 installation..."
info "Founding profiles: ${FOUNDING_PROFILES} (quorum control-plane,core,storage always enforced)"
echo ""

"${SCRIPT_DIR}/scripts/install-day0.sh"

# Print a corrected bootstrap command using a detected node-agent gRPC port.
# This supersedes stale script output that may omit the port suffix.
NODE_IP="$(ip route get 8.8.8.8 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src"){print $(i+1); exit}}')"
if [[ -z "${NODE_IP}" ]]; then
  NODE_IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
fi
if [[ -z "${NODE_IP}" ]]; then
  die "Could not determine a routable node IP for the bootstrap command"
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
print_bootstrap_profiles
echo "  Add any additional explicit workload profiles only if you intend to run them."
echo ""
