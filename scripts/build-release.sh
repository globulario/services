#!/usr/bin/env bash
# Local release build — mirrors what GitHub Actions does in release.yml.
#
# Usage:
#   cd /path/to/services
#   bash scripts/build-release.sh [version]
#
# Output:
#   dist/globular-<version>-linux-amd64.tar.gz
#   dist/globular-<version>-linux-amd64.tar.gz.sha256
#
# Requires: go, python3, tar, sha256sum

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICES_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
PACKAGES_ROOT="${SERVICES_ROOT}/../packages"
INSTALLER_ROOT="${SERVICES_ROOT}/../globular-installer"
DIST_DIR="${SERVICES_ROOT}/dist"

VERSION="${1:-0.0.0-dev}"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BOLD='\033[1m'; NC='\033[0m'
die()     { echo -e "${RED}ERROR: $*${NC}" >&2; exit 1; }
ok()      { echo -e "${GREEN}  ✓ $*${NC}"; }
warn()    { echo -e "${YELLOW}  ⚠ $*${NC}"; }
info()    { echo "  → $*"; }
section() { echo ""; echo -e "${BOLD}━━━ $* ━━━${NC}"; echo ""; }

[[ -d "${PACKAGES_ROOT}" ]] || die "packages repo not found at ${PACKAGES_ROOT} — clone it alongside services"

find_source_packages_dir() {
  if compgen -G "${PACKAGES_ROOT}/dist/*.tgz" >/dev/null; then
    echo "${PACKAGES_ROOT}/dist"
    return 0
  fi

  local candidate
  candidate=$(find /tmp "${SERVICES_ROOT}/dist" "${SERVICES_ROOT}/.." -maxdepth 3 -type d -path "*/globular-*-linux-amd64/packages" 2>/dev/null | sort -V | tail -1 || true)
  if [[ -n "${candidate}" ]] && compgen -G "${candidate}/*.tgz" >/dev/null; then
    echo "${candidate}"
    return 0
  fi

  return 1
}

section "Building Release ${VERSION}"

rm -rf "${DIST_DIR}/bin" "${DIST_DIR}/packages" "${DIST_DIR}/globular-${VERSION}-linux-amd64"*
mkdir -p "${DIST_DIR}/bin" "${DIST_DIR}/packages"

# ── Build Go binaries ────────────────────────────────────────────────────────
section "Building Go Services"

LDFLAGS="-X main.version=${VERSION} -X main.buildVersion=${VERSION} -s -w"
cd "${SERVICES_ROOT}/golang"

while IFS='|' read -r target output; do
  target="${target%%#*}"; target="${target// /}"
  output="${output// /}"
  [[ -z "${target}" ]] && continue

  bin_name=$(basename "${output}")
  info "Building ${bin_name}..."
  go build -ldflags "${LDFLAGS}" -o "${DIST_DIR}/bin/${bin_name}" "${target}"
done < build/services.list

cp "${DIST_DIR}/bin/globularcli" "${DIST_DIR}/bin/globular" 2>/dev/null || true

ok "$(ls "${DIST_DIR}/bin/" | wc -l) binaries built"
cd "${SERVICES_ROOT}"

# ── Create service packages ──────────────────────────────────────────────────
section "Creating Service Packages"

declare -A BIN_MAP=(
  [authentication]=authentication_server
  [backup-manager]=backup_manager_server
  [blog]=blog_server
  [catalog]=catalog_server
  [cluster-controller]=cluster_controller_server
  [cluster-doctor]=cluster_doctor_server
  [conversation]=conversation_server
  [discovery]=discovery_server
  [dns]=dns_server
  [echo]=echo_server
  [event]=event_server
  [file]=file_server
  [ldap]=ldap_server
  [log]=log_server
  [mail]=mail_server
  [media]=media_server
  [monitoring]=monitoring_server
  [node-agent]=node_agent_server
  [persistence]=persistence_server
  [rbac]=rbac_server
  [repository]=repository_server
  [resource]=resource_server
  [search]=search_server
  [sql]=sql_server
  [storage]=storage_server
  [title]=title_server
  [torrent]=torrent_server
  [workflow]=workflow_server
  [compute]=compute_server
  [ai-memory]=ai_memory_server
  [ai-executor]=ai_executor_server
  [ai-watcher]=ai_watcher_server
  [ai-router]=ai_router_server
  [globular-cli]=globularcli
  [mcp]=mcp
  [xds]=xds
  [gateway]=gateway
)

SOURCE_PACKAGES_DIR="$(find_source_packages_dir)" || die "no source packages found; expected ${PACKAGES_ROOT}/dist/*.tgz or an extracted globular release"
SOURCE_RELEASE_DIR="$(cd "${SOURCE_PACKAGES_DIR}/.." && pwd)"
info "Using source packages from ${SOURCE_PACKAGES_DIR}"

copied_external=0
for src_pkg in "${SOURCE_PACKAGES_DIR}"/*.tgz; do
  base="$(basename "${src_pkg}")"
  pkg_name="${base%_*_linux_amd64.tgz}"
  if [[ -n "${BIN_MAP[${pkg_name}]+x}" ]]; then
    continue
  fi
  cp "${src_pkg}" "${DIST_DIR}/packages/${base}"
  copied_external=$((copied_external + 1))
done
ok "${copied_external} external/unchanged packages copied"

pkg_count=0
for pkg_name in "${!BIN_MAP[@]}"; do
  bin_name="${BIN_MAP[${pkg_name}]}"
  bin_path="${DIST_DIR}/bin/${bin_name}"

  if [[ ! -f "${bin_path}" ]]; then
    src_pkg=$(find "${SOURCE_PACKAGES_DIR}" -maxdepth 1 -name "${pkg_name}_*_linux_amd64.tgz" 2>/dev/null | sort -V | tail -1 || true)
    if [[ -n "${src_pkg}" ]]; then
      warn "Carrying forward ${pkg_name} ($(basename "${src_pkg}"); ${bin_name} not built)"
      cp "${src_pkg}" "${DIST_DIR}/packages/$(basename "${src_pkg}")"
    else
      warn "Skipping ${pkg_name} (${bin_name} not built and no source package found)"
    fi
    continue
  fi

  src_pkg=$(find "${SOURCE_PACKAGES_DIR}" -maxdepth 1 -name "${pkg_name}_*_linux_amd64.tgz" 2>/dev/null | sort -V | tail -1 || true)
  if [[ -z "${src_pkg}" ]]; then
    warn "Skipping ${pkg_name} (no source package in packages/dist/)"
    continue
  fi

  info "Packaging ${pkg_name} v${VERSION}..."

  tmpdir=$(mktemp -d)
  tar -C "${tmpdir}" -xf "${src_pkg}" --exclude='bin/*'
  mkdir -p "${tmpdir}/bin"
  cp "${bin_path}" "${tmpdir}/bin/${bin_name}"
  chmod 755 "${tmpdir}/bin/${bin_name}"

  CHECKSUM="sha256:$(sha256sum "${bin_path}" | awk '{print $1}')"

  python3 - "${tmpdir}/package.json" "${VERSION}" "${CHECKSUM}" <<'PYEOF'
import json, sys
path, version, checksum = sys.argv[1], sys.argv[2], sys.argv[3]
with open(path) as f:
    d = json.load(f)
d['version'] = version
d['entrypoint_checksum'] = checksum
with open(path, 'w') as f:
    json.dump(d, f, indent=2)
PYEOF

  out_file="${DIST_DIR}/packages/${pkg_name}_${VERSION}_linux_amd64.tgz"
  tar -C "${tmpdir}" -czf "${out_file}" .
  rm -rf "${tmpdir}"
  pkg_count=$((pkg_count + 1))
done

ok "${pkg_count} packages created"

# ── Assemble release tarball ─────────────────────────────────────────────────
section "Assembling Release Tarball"

RELEASE_NAME="globular-${VERSION}-linux-amd64"
RELEASE_DIR="${DIST_DIR}/${RELEASE_NAME}"

mkdir -p "${RELEASE_DIR}/packages"
mkdir -p "${RELEASE_DIR}/scripts" "${RELEASE_DIR}/workflows"

cp "${DIST_DIR}/bin/globular"   "${RELEASE_DIR}/globular"
chmod 755 "${RELEASE_DIR}/globular"

if [[ -x "${INSTALLER_ROOT}/globular-installer" ]]; then
  cp "${INSTALLER_ROOT}/globular-installer" "${RELEASE_DIR}/globular-installer"
elif [[ -x "${INSTALLER_ROOT}/bin/globular-installer" ]]; then
  cp "${INSTALLER_ROOT}/bin/globular-installer" "${RELEASE_DIR}/globular-installer"
else
  die "globular-installer binary not found in ${INSTALLER_ROOT}"
fi
chmod 755 "${RELEASE_DIR}/globular-installer"

cp "${DIST_DIR}/packages/"*.tgz "${RELEASE_DIR}/packages/"
cp "${SCRIPT_DIR}/install.sh"   "${RELEASE_DIR}/install.sh"
chmod +x "${RELEASE_DIR}/install.sh"

if [[ -d "${INSTALLER_ROOT}/scripts" ]]; then
  cp -a "${INSTALLER_ROOT}/scripts/." "${RELEASE_DIR}/scripts/"
elif [[ -d "${SOURCE_RELEASE_DIR}/scripts" ]]; then
  cp -a "${SOURCE_RELEASE_DIR}/scripts/." "${RELEASE_DIR}/scripts/"
else
  die "installer scripts not found"
fi
chmod +x "${RELEASE_DIR}/scripts/"*.sh 2>/dev/null || true

cp "${SERVICES_ROOT}/golang/workflow/definitions/"*.yaml "${RELEASE_DIR}/workflows/"

if [[ -d "${SOURCE_RELEASE_DIR}/webroot" ]]; then
  cp -a "${SOURCE_RELEASE_DIR}/webroot" "${RELEASE_DIR}/webroot"
fi

(cd "${RELEASE_DIR}/packages" && sha256sum *.tgz > SHA256SUMS)

cat > "${RELEASE_DIR}/README.md" <<HEREDOC
# Globular ${VERSION}

## Install

\`\`\`bash
sudo bash install.sh
\`\`\`

## Next Steps

\`\`\`bash
sudo systemctl start globular-node-agent
globular cluster bootstrap --node localhost:11000 --domain <your-domain> --profile core --profile gateway
\`\`\`

Full guide: https://globular.io/docs/operators/installation
HEREDOC

cd "${DIST_DIR}"
tar czf "${RELEASE_NAME}.tar.gz" "${RELEASE_NAME}/"
sha256sum "${RELEASE_NAME}.tar.gz" > "${RELEASE_NAME}.tar.gz.sha256"

section "Done"
echo "Release tarball: ${DIST_DIR}/${RELEASE_NAME}.tar.gz"
echo "Size:            $(du -sh "${DIST_DIR}/${RELEASE_NAME}.tar.gz" | cut -f1)"
echo "Packages:        ${pkg_count}"
echo ""
echo "Contents:"
tar tzf "${DIST_DIR}/${RELEASE_NAME}.tar.gz" | head -10 || true
echo "  ..."
echo ""
echo "To test the installer:"
echo "  mkdir /tmp/globular-test && tar xzf ${DIST_DIR}/${RELEASE_NAME}.tar.gz -C /tmp/globular-test"
echo "  sudo bash /tmp/globular-test/${RELEASE_NAME}/install.sh"
