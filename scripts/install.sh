#!/usr/bin/env bash
# Globular Day-0 Installer
#
# This script installs Globular from a release tarball. It:
#   1. Creates version-compat symlinks so install-day0.sh can find service
#      packages by their hardcoded names (e.g. node-agent_0.0.1_linux_amd64.tgz)
#   2. Installs the globular CLI to /usr/local/bin/globular
#   3. Delegates to scripts/install-day0.sh for the full Day-0 installation
#
# Usage:
#   cd globular-{VERSION}-linux-amd64
#   sudo bash install.sh
#
# Environment:
#   GLOBULAR_DOMAIN     - Cluster domain (default: globular.internal)
#   MINIO_DATA_DIR      - MinIO data directory (prompted if not set)
#   GLOBULAR_PASSWORD   - Service account password (default: adminadmin)

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

[[ -f "${SCRIPT_DIR}/globular" ]]            || die "globular CLI not found — was the tarball extracted correctly?"
[[ -f "${SCRIPT_DIR}/globular-installer" ]]  || die "globular-installer not found — was the tarball extracted correctly?"
[[ -d "${SCRIPT_DIR}/packages" ]]            || die "packages/ directory not found"
[[ -f "${SCRIPT_DIR}/scripts/install-day0.sh" ]] || die "scripts/install-day0.sh not found"

# Detect the Globular version from the CLI binary
VERSION=$("${SCRIPT_DIR}/globular" version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || echo "")
if [[ -z "${VERSION}" ]]; then
    # Fallback: infer from directory name globular-{VERSION}-linux-amd64
    VERSION=$(basename "${SCRIPT_DIR}" | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || echo "")
fi
[[ -n "${VERSION}" ]] || die "Could not determine release version"
info "Release version: ${VERSION}"

# ── Version-compat symlinks ───────────────────────────────────────────────────
# install-day0.sh has hardcoded package names like "node-agent_0.0.1_linux_amd64.tgz".
# Service packages in this tarball are named "node-agent_{VERSION}_linux_amd64.tgz".
# We create symlinks so both names resolve to the same file.
echo ""
info "Creating version-compat symlinks..."

COMPAT_VERSIONS=("0.0.1" "0.0.2")

cd "${SCRIPT_DIR}/packages"
for pkg in *_${VERSION}_linux_amd64.tgz; do
    [[ -f "$pkg" ]] || continue
    name="${pkg%_${VERSION}_linux_amd64.tgz}"
    for compat_ver in "${COMPAT_VERSIONS[@]}"; do
        compat="${name}_${compat_ver}_linux_amd64.tgz"
        # Only create symlink if the exact compat name doesn't exist as a real file
        if [[ ! -f "$compat" ]]; then
            ln -sf "$pkg" "$compat"
        fi
    done
done
cd "${SCRIPT_DIR}"

ok "Version-compat symlinks created"

# ── Install CLI ───────────────────────────────────────────────────────────────
echo ""
info "Installing globular CLI..."
install -m 755 "${SCRIPT_DIR}/globular" /usr/local/bin/globular
ok "globular → /usr/local/bin/globular"

# ── Run Day-0 installation ────────────────────────────────────────────────────
echo ""
info "Starting Day-0 installation..."
echo ""

export PKG_DIR="${SCRIPT_DIR}/packages"
export INSTALLER_BIN="${SCRIPT_DIR}/globular-installer"

exec "${SCRIPT_DIR}/scripts/install-day0.sh"
