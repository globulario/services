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

elf_needs_release_strip() {
  local bin="$1"
  [[ -f "${bin}" ]] || return 1
  file -b "${bin}" 2>/dev/null | grep -q '^ELF' || return 1
  readelf -S "${bin}" 2>/dev/null | grep -Eq '\.(debug_|zdebug_|symtab)\b'
}

strip_release_binary() {
  local bin="$1" label="${2:-$(basename "$1")}"
  [[ -f "${bin}" ]] || return 0
  if ! elf_needs_release_strip "${bin}"; then
    return 0
  fi
  command -v strip >/dev/null 2>&1 || die "${label} contains release-forbidden debug sections and 'strip' is unavailable"
  info "Stripping ${label} for release-channel packaging..."
  strip --strip-debug --strip-unneeded "${bin}"
  chmod +x "${bin}"
  if elf_needs_release_strip "${bin}"; then
    die "${label} still contains release-forbidden debug sections after strip"
  fi
}

repack_package_if_needed() {
  local src_pkg="$1" dest_pkg="$2"
  local tmpdir entrypoint checksum
  tmpdir=$(mktemp -d)
  tar -xzf "${src_pkg}" -C "${tmpdir}"
  entrypoint="$(sed -n 's/.*"entrypoint"[[:space:]]*:[[:space:]]*"bin\/\([^"]*\)".*/\1/p' "${tmpdir}/package.json" | head -1)"
  if [[ -n "${entrypoint}" && -f "${tmpdir}/bin/${entrypoint}" ]] && elf_needs_release_strip "${tmpdir}/bin/${entrypoint}"; then
    strip_release_binary "${tmpdir}/bin/${entrypoint}" "$(basename "${src_pkg}"):${entrypoint}"
    checksum=$(sha256sum "${tmpdir}/bin/${entrypoint}" | awk '{print $1}')
    python3 - "${tmpdir}/package.json" "sha256:${checksum}" <<'PYEOF'
import json, sys, uuid
path, checksum = sys.argv[1:]
with open(path) as f:
    data = json.load(f)
data["build_id"] = str(uuid.uuid4())
data["entrypoint_checksum"] = checksum
with open(path, "w") as f:
    json.dump(data, f, indent=2)
PYEOF
    tar -C "${tmpdir}" -czf "${dest_pkg}" .
    ok "Repacked stripped artifact: $(basename "${dest_pkg}")"
  else
    cp "${src_pkg}" "${dest_pkg}"
  fi
  rm -rf "${tmpdir}"
}

validate_release_bundle() {
  local release_dir="$1"
  local pkg_dir="${release_dir}/packages"
  local prefix tgz tmpdir entrypoint

  grep -q 'FOUNDING_PROFILES="${FOUNDING_PROFILES:-core,media-server}"' \
    "${release_dir}/scripts/install-day0.sh" \
    || die "release bundle install-day0.sh does not default FOUNDING_PROFILES to core,media-server"

  for prefix in prometheus node-exporter scylla-manager scylla-manager-agent sctool; do
    tgz=$(find "${pkg_dir}" -maxdepth 1 -name "${prefix}_*_linux_amd64.tgz" | sort -V | tail -1 || true)
    [[ -n "${tgz}" ]] || continue
    tmpdir=$(mktemp -d)
    tar -xzf "${tgz}" -C "${tmpdir}"
    entrypoint="$(sed -n 's/.*"entrypoint"[[:space:]]*:[[:space:]]*"bin\/\([^"]*\)".*/\1/p' "${tmpdir}/package.json" | head -1)"
    if [[ -n "${entrypoint}" && -f "${tmpdir}/bin/${entrypoint}" ]] && elf_needs_release_strip "${tmpdir}/bin/${entrypoint}"; then
      rm -rf "${tmpdir}"
      die "release bundle package $(basename "${tgz}") still carries release-forbidden debug sections"
    fi
    rm -rf "${tmpdir}"
  done
}

collect_source_package_dirs() {
  local -a dirs=()
  local candidate

  if compgen -G "${PACKAGES_ROOT}/dist/*.tgz" >/dev/null; then
    dirs+=("${PACKAGES_ROOT}/dist")
  fi
  if compgen -G "${SERVICES_ROOT}/generated/*.tgz" >/dev/null; then
    dirs+=("${SERVICES_ROOT}/generated")
  fi
  while IFS= read -r candidate; do
    [[ -n "${candidate}" ]] || continue
    dirs+=("${candidate}")
  done < <(find /tmp "${SERVICES_ROOT}/dist" "${SERVICES_ROOT}/.." -maxdepth 3 -type d -path "*/globular-*-linux-amd64/packages" 2>/dev/null | sort -uV)

  if [[ ${#dirs[@]} -eq 0 ]]; then
    return 1
  fi

  printf '%s\n' "${dirs[@]}"
}

find_source_package() {
  local pkg_name="$1"
  local -a dirs=("${@:2}")
  local dir match
  for dir in "${dirs[@]}"; do
    match=$(find "${dir}" -maxdepth 1 -name "${pkg_name}_*_linux_amd64.tgz" 2>/dev/null | sort -V | tail -1 || true)
    if [[ -n "${match}" ]]; then
      echo "${match}"
      return 0
    fi
  done
  return 1
}

section "Building Release ${VERSION}"

rm -rf "${DIST_DIR}/bin" "${DIST_DIR}/packages" "${DIST_DIR}/globular-${VERSION}-linux-amd64"*
mkdir -p "${DIST_DIR}/bin" "${DIST_DIR}/packages"

# ── Build Go binaries ────────────────────────────────────────────────────────
section "Building Go Services"

LDFLAGS="-X main.Version=${VERSION} -s -w"
cd "${SERVICES_ROOT}/golang"

while IFS='|' read -r target output; do
  target="${target%%#*}"; target="${target// /}"
  output="${output// /}"
  [[ -z "${target}" ]] && continue

  bin_name=$(basename "${output}")
  info "Building ${bin_name}..."
  go build -trimpath -ldflags "${LDFLAGS}" -o "${DIST_DIR}/bin/${bin_name}" "${target}"
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
  [ai-memory]=ai_memory_server
  [ai-executor]=ai_executor_server
  [ai-watcher]=ai_watcher_server
  [ai-router]=ai_router_server
  [globular-cli]=globularcli
  [mcp]=mcp
  [xds]=xds
  [gateway]=gateway
)

mapfile -t SOURCE_PACKAGE_DIRS < <(collect_source_package_dirs) || die "no source packages found; expected generated/*.tgz, ${PACKAGES_ROOT}/dist/*.tgz, or an extracted globular release"
SOURCE_RELEASE_DIR="$(cd "${SOURCE_PACKAGE_DIRS[0]}/.." && pwd)"
info "Using source packages from:"
for dir in "${SOURCE_PACKAGE_DIRS[@]}"; do
  info "  - ${dir}"
done

copied_external=0
declare -A seen_external=()
for dir in "${SOURCE_PACKAGE_DIRS[@]}"; do
  for src_pkg in "${dir}"/*.tgz; do
    [[ -e "${src_pkg}" ]] || continue
    base="$(basename "${src_pkg}")"
    pkg_name="${base%_*_linux_amd64.tgz}"
    if [[ -n "${BIN_MAP[${pkg_name}]+x}" ]]; then
      continue
    fi
    if [[ -n "${seen_external[${base}]+x}" ]]; then
      continue
    fi
    repack_package_if_needed "${src_pkg}" "${DIST_DIR}/packages/${base}"
    seen_external["${base}"]=1
    copied_external=$((copied_external + 1))
  done
done
ok "${copied_external} external/unchanged packages copied"

pkg_count=0
for pkg_name in "${!BIN_MAP[@]}"; do
  bin_name="${BIN_MAP[${pkg_name}]}"
  bin_path="${DIST_DIR}/bin/${bin_name}"

  if [[ ! -f "${bin_path}" ]]; then
    src_pkg="$(find_source_package "${pkg_name}" "${SOURCE_PACKAGE_DIRS[@]}" || true)"
    if [[ -n "${src_pkg}" ]]; then
      warn "Carrying forward ${pkg_name} ($(basename "${src_pkg}"); ${bin_name} not built)"
      repack_package_if_needed "${src_pkg}" "${DIST_DIR}/packages/$(basename "${src_pkg}")"
    else
      warn "Skipping ${pkg_name} (${bin_name} not built and no source package found)"
    fi
    continue
  fi

  src_pkg="$(find_source_package "${pkg_name}" "${SOURCE_PACKAGE_DIRS[@]}" || true)"
  if [[ -z "${src_pkg}" ]]; then
    die "missing source package template for ${pkg_name} (${bin_name} was built, but no ${pkg_name}_*_linux_amd64.tgz was found in any source package directory)"
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

if [[ -d "${SERVICES_ROOT}/scripts/release" ]]; then
  cp -a "${SERVICES_ROOT}/scripts/release/." "${RELEASE_DIR}/scripts/"
fi
chmod +x "${RELEASE_DIR}/scripts/"*.sh 2>/dev/null || true

cp "${SERVICES_ROOT}/golang/workflow/definitions/"*.yaml "${RELEASE_DIR}/workflows/"

if [[ -d "${SERVICES_ROOT}/webroot" ]]; then
  cp -a "${SERVICES_ROOT}/webroot" "${RELEASE_DIR}/webroot"
elif [[ -d "${SOURCE_RELEASE_DIR}/webroot" ]]; then
  cp -a "${SOURCE_RELEASE_DIR}/webroot" "${RELEASE_DIR}/webroot"
fi

(cd "${RELEASE_DIR}/packages" && sha256sum *.tgz > SHA256SUMS)
validate_release_bundle "${RELEASE_DIR}"

cat > "${RELEASE_DIR}/README.md" <<HEREDOC
# Globular ${VERSION}

## Install

\`\`\`bash
sudo bash install.sh
\`\`\`

The first node always comes up with the quorum profiles plus the media workload
profile (\`control-plane\`, \`core\`, \`storage\`, \`media-server\`). To
override or extend the day-0 workload profiles, pass \`FOUNDING_PROFILES\`
(comma-separated) through \`sudo\`:

\`\`\`bash
sudo FOUNDING_PROFILES=core,media-server bash install.sh
\`\`\`

## Next Steps

\`\`\`bash
sudo systemctl start globular-node-agent
globular cluster bootstrap --node <routable-node-ip>:11000 --domain <your-domain> --profile core --profile gateway
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
