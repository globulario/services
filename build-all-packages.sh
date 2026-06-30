#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "╔════════════════════════════════════════════════════════════════╗"
echo "║     REBUILD AND REPACK ALL PACKAGES                            ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""

# Paths
GLOBULAR_ROOT="/home/dave/Documents/github.com/globulario"
PACKAGES_ROOT="${GLOBULAR_ROOT}/packages"
SERVICES_ROOT="${GLOBULAR_ROOT}/services"
# Gateway and xds are now built by generateCode.sh into the stage directory,
# alongside all other services. No longer need the Globular/.bin path.
INSTALLER_ASSETS="${GLOBULAR_ROOT}/globular-installer/internal/assets/packages"
SERVICES_STAGE="${SERVICES_ROOT}/golang/tools/stage/linux-amd64/usr/local/bin"
SERVICES_OUTPUT="${SERVICES_ROOT}/generated"

# The single output directory for all built packages (infra + services).
DIST_DIR="${PACKAGES_ROOT}/dist"

echo "━━━ Configuration ━━━"
echo ""
echo "  Packages root:        ${PACKAGES_ROOT}"
echo "  Services root:        ${SERVICES_ROOT}"
echo "  Stage directory:      ${SERVICES_STAGE}"
echo "  Dist directory:       ${DIST_DIR}"
echo "  Installer assets:     ${INSTALLER_ASSETS}"
echo ""

echo "━━━ Generated Workspace ━━━"
echo ""
mkdir -p "${SERVICES_OUTPUT}"
# services/generated is a disposable build workspace. Rebuild the transient
# spec + package outputs each run so stale artifacts cannot masquerade as
# current release input. Preserve generated/policy and workflow payload inputs
# produced by generateCode.sh; they are regenerated earlier in the full build flow.
rm -rf "${SERVICES_OUTPUT}/specs" "${SERVICES_OUTPUT}/packages"
find "${SERVICES_OUTPUT}" -maxdepth 1 -type f -name '*.tgz' -delete
find "${SERVICES_OUTPUT}" -maxdepth 1 -type d -name '.pkg-staging-*' -exec rm -rf {} +
echo "  ✓ Reset transient outputs under ${SERVICES_OUTPUT}"
echo ""

# ── Step 1: Prepare infrastructure binaries ───────────────────────────────
# Copies Go binaries from Globular build output and downloads third-party
# binaries (envoy, etcd, prometheus, etc.) into packages/bin/.
# Versions are read from spec metadata — this step only ensures binaries exist.

echo "━━━ Step 1: Prepare Infrastructure Binaries ━━━"
echo ""

# packages/bin is binary staging only. Recreate it from declared outputs at the
# staging boundary so stale aliases or orphaned build artifacts cannot survive.
rm -rf "${PACKAGES_ROOT}/bin"
mkdir -p "${PACKAGES_ROOT}/bin"
printf '#!/bin/sh\nexit 0\n' > "${PACKAGES_ROOT}/bin/noop"
chmod +x "${PACKAGES_ROOT}/bin/noop"
echo "  ✓ Reset packages/bin staging and recreated shared noop sentinel"

# Helper: copy a Go binary from the Globular build output.
copy_go_bin() {
    local src="$1" dst="$2" label="$3"
    if [[ -f "${src}" ]]; then
        cp "${src}" "${PACKAGES_ROOT}/bin/${dst}"
        chmod +x "${PACKAGES_ROOT}/bin/${dst}"
        strip_release_binary "${PACKAGES_ROOT}/bin/${dst}" "${label}"
        echo "  ✓ ${label} ($(ls -lh "${PACKAGES_ROOT}/bin/${dst}" | awk '{print $5}'))"
    else
        echo "  ✗ ${label} not found at ${src}"
        exit 1
    fi
}

# Helper: read metadata.version from a spec file.
spec_version() {
    sed -n '/^metadata:/,/^[^ ]/{ s/^[[:space:]]\{1,\}version:[[:space:]]*\(.*\)/\1/p; }' "$1" | head -1 | sed 's/^"\(.*\)"$/\1/'
}

# spec_path <spec_file> — resolve a spec to its single source of truth under
# metadata/<name>/specs/. There is no top-level packages/specs/ dir anymore
# (removed in the 2026-06 spec source-of-truth consolidation). The package name
# is the spec filename minus _service.yaml/_cmd.yaml with underscores→hyphens,
# matching the metadata/ dir naming.
spec_path() {
    local file="$1" name
    name="$(echo "${file}" | sed 's/_service\.yaml$//; s/_cmd\.yaml$//' | tr '_' '-')"
    echo "${PACKAGES_ROOT}/metadata/${name}/specs/${file}"
}

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
    if ! command -v strip >/dev/null 2>&1; then
        echo "  ✗ ${label} contains release-forbidden debug sections and 'strip' is unavailable"
        exit 1
    fi
    echo "  → Stripping ${label} for release-channel packaging..."
    strip --strip-debug --strip-unneeded "${bin}"
    chmod +x "${bin}"
    if elf_needs_release_strip "${bin}"; then
        echo "  ✗ ${label} still contains release-forbidden debug sections after strip"
        exit 1
    fi
    echo "  ✓ ${label} stripped for release channel"
}

normalize_release_binaries() {
    local label_prefix="$1"
    shift
    local bin
    for bin in "$@"; do
        [[ -n "${bin}" ]] || continue
        strip_release_binary "${bin}" "${label_prefix}:$(basename "${bin}")"
    done
}

# Helper: download a binary if version mismatches or missing.
ensure_binary() {
    local bin="$1" version="$2" check_cmd="$3" download_fn="$4"
    shift 4
    local normalize_bins=("$@")
    if [[ ${#normalize_bins[@]} -eq 0 ]]; then
        normalize_bins=("${bin}")
    fi
    local current=""
    if [[ -f "${bin}" ]]; then
        current=$(eval "${check_cmd}" 2>&1 || echo "unknown")
        if [[ "${current}" == "${version}" ]]; then
            echo "  ✓ $(basename "${bin}") ${version} already present"
            normalize_release_binaries "$(basename "${bin}")" "${normalize_bins[@]}"
            return 0
        fi
        echo "  ⚠ $(basename "${bin}") version mismatch (${current}), downloading ${version}..."
    else
        echo "  → Downloading $(basename "${bin}") ${version}..."
    fi
    eval "${download_fn}"
    normalize_release_binaries "$(basename "${bin}")" "${normalize_bins[@]}"
    echo "  ✓ $(basename "${bin}") ${version} ($(ls -lh "${bin}" | awk '{print $5}'))"
}

stage_local_binary() {
    local src="$1" dst="$2" expected_version="$3" check_cmd="$4" label="$5"
    if [[ -z "${src}" || ! -x "${src}" ]]; then
        echo "  ✗ ${label} source binary not found or not executable: ${src:-<empty>}"
        exit 1
    fi
    local current=""
    current=$(eval "${check_cmd}" 2>/dev/null | head -1 | tr -d '\r' || true)
    if [[ -z "${current}" ]]; then
        echo "  ✗ ${label} version probe returned empty output"
        exit 1
    fi
    if [[ "${current}" != "${expected_version}" ]]; then
        echo "  ✗ ${label} version drift: expected '${expected_version}', got '${current}' from ${src}"
        echo "    registry-backed spec metadata is authoritative; update the spec or stage the correct binary."
        exit 1
    fi
    cp "${src}" "${dst}"
    chmod +x "${dst}"
    strip_release_binary "${dst}" "${label}"
    echo "  ✓ ${label} ${expected_version} ($(ls -lh "${dst}" | awk '{print $5}'))"
}

# Go binaries from the stage directory (built by generateCode.sh)
echo "→ Copying Go binaries from stage..."
copy_go_bin "${SERVICES_STAGE}/gateway" "gateway" "gateway"
copy_go_bin "${SERVICES_STAGE}/xds" "xds" "xds"
rm -f "${PACKAGES_ROOT}/bin/globularcli"
copy_go_bin "${SERVICES_STAGE}/globularcli" "globular" "globular (cli install name)"

# Read versions from specs (single source of truth)
ENVOY_VERSION=$(spec_version "$(spec_path envoy_service.yaml)")
ETCD_VERSION=$(spec_version "$(spec_path etcd_service.yaml)")
PROMETHEUS_VERSION=$(spec_version "$(spec_path prometheus_service.yaml)")
ALERTMANAGER_VERSION=$(spec_version "$(spec_path alertmanager_service.yaml)")
NODE_EXPORTER_VERSION=$(spec_version "$(spec_path node_exporter_service.yaml)")
SIDEKICK_VERSION=$(spec_version "$(spec_path sidekick_service.yaml)")
RESTIC_VERSION=$(spec_version "$(spec_path restic_cmd.yaml)")
RCLONE_VERSION=$(spec_version "$(spec_path rclone_cmd.yaml)")
MC_VERSION=$(spec_version "$(spec_path mc_cmd.yaml)")
YT_DLP_VERSION=$(spec_version "$(spec_path yt_dlp_cmd.yaml)")
FFMPEG_VERSION=$(spec_version "$(spec_path ffmpeg_cmd.yaml)")
COREUTILS_VERSION=$(spec_version "$(spec_path sha256sum_cmd.yaml)")
MINIO_VERSION=$(spec_version "$(spec_path minio_service.yaml)")
SCYLLA_MANAGER_VERSION=$(spec_version "$(spec_path scylla_manager_service.yaml)")
SCYLLA_MANAGER_AGENT_VERSION=$(spec_version "$(spec_path scylla_manager_agent_service.yaml)")
SCTOOL_VERSION=$(spec_version "$(spec_path sctool_cmd.yaml)")

echo ""
echo "→ Third-party binary versions (from spec metadata):"
echo "  envoy=${ENVOY_VERSION} etcd=${ETCD_VERSION} prometheus=${PROMETHEUS_VERSION} alertmanager=${ALERTMANAGER_VERSION}"
echo "  node_exporter=${NODE_EXPORTER_VERSION} sidekick=${SIDEKICK_VERSION}"
echo "  restic=${RESTIC_VERSION} rclone=${RCLONE_VERSION} mc=${MC_VERSION} yt-dlp=${YT_DLP_VERSION}"
echo "  ffmpeg=${FFMPEG_VERSION} sha256sum/coreutils=${COREUTILS_VERSION}"
echo "  minio=${MINIO_VERSION} scylla-manager=${SCYLLA_MANAGER_VERSION} scylla-manager-agent=${SCYLLA_MANAGER_AGENT_VERSION} sctool=${SCTOOL_VERSION}"
echo ""

# Envoy
echo "→ Envoy ${ENVOY_VERSION}..."
ENVOY_BIN="${PACKAGES_ROOT}/bin/envoy"
ensure_binary "${ENVOY_BIN}" "${ENVOY_VERSION}" \
    "${ENVOY_BIN} --version 2>&1 | grep -oP 'version: \K[0-9.]+'" \
    "curl -sL 'https://github.com/envoyproxy/envoy/releases/download/v${ENVOY_VERSION}/envoy-${ENVOY_VERSION}-linux-x86_64' -o '${ENVOY_BIN}' && chmod +x '${ENVOY_BIN}'" \
    "${ENVOY_BIN}"

# etcd + etcdctl
echo "→ etcd ${ETCD_VERSION}..."
ETCD_BIN="${PACKAGES_ROOT}/bin/etcd"
ensure_binary "${ETCD_BIN}" "${ETCD_VERSION}" \
    "${ETCD_BIN} --version 2>&1 | grep -oP 'etcd Version: \K[0-9.]+'" \
    "cd /tmp && curl -sL 'https://github.com/etcd-io/etcd/releases/download/v${ETCD_VERSION}/etcd-v${ETCD_VERSION}-linux-amd64.tar.gz' -o etcd.tgz && tar xzf etcd.tgz && cp etcd-v${ETCD_VERSION}-linux-amd64/etcd '${PACKAGES_ROOT}/bin/etcd' && cp etcd-v${ETCD_VERSION}-linux-amd64/etcdctl '${PACKAGES_ROOT}/bin/etcdctl' && chmod +x '${PACKAGES_ROOT}/bin/etcd' '${PACKAGES_ROOT}/bin/etcdctl' && rm -rf etcd-v${ETCD_VERSION}-linux-amd64 etcd.tgz && cd ->/dev/null" \
    "${PACKAGES_ROOT}/bin/etcd" "${PACKAGES_ROOT}/bin/etcdctl"

# Prometheus + promtool
echo "→ Prometheus ${PROMETHEUS_VERSION}..."
PROMETHEUS_BIN="${PACKAGES_ROOT}/bin/prometheus"
ensure_binary "${PROMETHEUS_BIN}" "${PROMETHEUS_VERSION}" \
    "${PROMETHEUS_BIN} --version 2>&1 | grep -oP 'version \K[0-9.]+'" \
    "cd /tmp && curl -sL 'https://github.com/prometheus/prometheus/releases/download/v${PROMETHEUS_VERSION}/prometheus-${PROMETHEUS_VERSION}.linux-amd64.tar.gz' -o prom.tgz && tar xzf prom.tgz && cp prometheus-${PROMETHEUS_VERSION}.linux-amd64/prometheus '${PACKAGES_ROOT}/bin/prometheus' && cp prometheus-${PROMETHEUS_VERSION}.linux-amd64/promtool '${PACKAGES_ROOT}/bin/promtool' && chmod +x '${PACKAGES_ROOT}/bin/prometheus' '${PACKAGES_ROOT}/bin/promtool' && rm -rf prometheus-${PROMETHEUS_VERSION}.linux-amd64 prom.tgz && cd ->/dev/null" \
    "${PACKAGES_ROOT}/bin/prometheus" "${PACKAGES_ROOT}/bin/promtool"

# Alertmanager + amtool
echo "→ Alertmanager ${ALERTMANAGER_VERSION}..."
ALERTMANAGER_BIN="${PACKAGES_ROOT}/bin/alertmanager"
ensure_binary "${ALERTMANAGER_BIN}" "${ALERTMANAGER_VERSION}" \
    "${ALERTMANAGER_BIN} --version 2>&1 | grep -oP 'version \K[0-9.]+'" \
    "cd /tmp && curl -sL 'https://github.com/prometheus/alertmanager/releases/download/v${ALERTMANAGER_VERSION}/alertmanager-${ALERTMANAGER_VERSION}.linux-amd64.tar.gz' -o am.tgz && tar xzf am.tgz && cp alertmanager-${ALERTMANAGER_VERSION}.linux-amd64/alertmanager '${PACKAGES_ROOT}/bin/alertmanager' && cp alertmanager-${ALERTMANAGER_VERSION}.linux-amd64/amtool '${PACKAGES_ROOT}/bin/amtool' && chmod +x '${PACKAGES_ROOT}/bin/alertmanager' '${PACKAGES_ROOT}/bin/amtool' && rm -rf alertmanager-${ALERTMANAGER_VERSION}.linux-amd64 am.tgz && cd ->/dev/null" \
    "${PACKAGES_ROOT}/bin/alertmanager" "${PACKAGES_ROOT}/bin/amtool"

# Node exporter
echo "→ node_exporter ${NODE_EXPORTER_VERSION}..."
NODE_EXPORTER_BIN="${PACKAGES_ROOT}/bin/node_exporter"
ensure_binary "${NODE_EXPORTER_BIN}" "${NODE_EXPORTER_VERSION}" \
    "${NODE_EXPORTER_BIN} --version 2>&1 | grep -oP 'version \K[0-9.]+'" \
    "cd /tmp && curl -sL 'https://github.com/prometheus/node_exporter/releases/download/v${NODE_EXPORTER_VERSION}/node_exporter-${NODE_EXPORTER_VERSION}.linux-amd64.tar.gz' -o ne.tgz && tar xzf ne.tgz && cp node_exporter-${NODE_EXPORTER_VERSION}.linux-amd64/node_exporter '${PACKAGES_ROOT}/bin/node_exporter' && chmod +x '${PACKAGES_ROOT}/bin/node_exporter' && rm -rf node_exporter-${NODE_EXPORTER_VERSION}.linux-amd64 ne.tgz && cd ->/dev/null" \
    "${PACKAGES_ROOT}/bin/node_exporter"

# Sidekick
echo "→ sidekick ${SIDEKICK_VERSION}..."
SIDEKICK_BIN="${PACKAGES_ROOT}/bin/sidekick"
ensure_binary "${SIDEKICK_BIN}" "${SIDEKICK_VERSION}" \
    "${SIDEKICK_BIN} --version 2>&1 | grep -oP 'version: \K[0-9.]+'" \
    "curl -sL 'https://github.com/minio/sidekick/releases/latest/download/sidekick-linux-amd64' -o '${SIDEKICK_BIN}' && chmod +x '${SIDEKICK_BIN}'" \
    "${SIDEKICK_BIN}"

# MinIO + Scylla Manager toolchain come from external upstream packaging, but
# the versions are still governed by the canonical package specs. Stage only
# binaries whose self-reported version exactly matches the spec.
echo "→ minio ${MINIO_VERSION}..."
MINIO_SRC="${MINIO_BIN:-/usr/bin/minio}"
stage_local_binary "${MINIO_SRC}" "${PACKAGES_ROOT}/bin/minio" "${MINIO_VERSION}" \
    "${MINIO_SRC} --version 2>&1 | head -1 | grep -o 'RELEASE\\.[^ ]*'" \
    "minio"

echo "→ scylla-manager ${SCYLLA_MANAGER_VERSION}..."
SCYLLA_MANAGER_SRC="${SCYLLA_MANAGER_BIN:-$(command -v scylla-manager 2>/dev/null || true)}"
stage_local_binary "${SCYLLA_MANAGER_SRC}" "${PACKAGES_ROOT}/bin/scylla_manager" "${SCYLLA_MANAGER_VERSION}" \
    "${SCYLLA_MANAGER_SRC} --version 2>&1 | head -1" \
    "scylla_manager"

echo "→ scylla-manager-agent ${SCYLLA_MANAGER_AGENT_VERSION}..."
SCYLLA_MANAGER_AGENT_SRC="${SCYLLA_MANAGER_AGENT_BIN:-$(command -v scylla-manager-agent 2>/dev/null || true)}"
stage_local_binary "${SCYLLA_MANAGER_AGENT_SRC}" "${PACKAGES_ROOT}/bin/scylla_manager_agent" "${SCYLLA_MANAGER_AGENT_VERSION}" \
    "${SCYLLA_MANAGER_AGENT_SRC} --version 2>&1 | head -1" \
    "scylla_manager_agent"

echo "→ sctool ${SCTOOL_VERSION}..."
SCTOOL_SRC="${SCTOOL_BIN:-$(command -v sctool 2>/dev/null || true)}"
stage_local_binary "${SCTOOL_SRC}" "${PACKAGES_ROOT}/bin/sctool" "${SCTOOL_VERSION}" \
    "${SCTOOL_SRC} version 2>&1 | sed -n 's/^Client version: //p' | head -1" \
    "sctool"

echo "→ restic ${RESTIC_VERSION}..."
RESTIC_BIN="${PACKAGES_ROOT}/bin/restic"
ensure_binary "${RESTIC_BIN}" "${RESTIC_VERSION}" \
    "${RESTIC_BIN} version 2>&1 | awk 'NR==1{print \$2}'" \
    "cd /tmp && curl -sL 'https://github.com/restic/restic/releases/download/v${RESTIC_VERSION}/restic_${RESTIC_VERSION}_linux_amd64.bz2' -o restic.bz2 && bunzip2 -f restic.bz2 && mv restic '${RESTIC_BIN}' && chmod +x '${RESTIC_BIN}' && cd ->/dev/null" \
    "${RESTIC_BIN}"

echo "→ rclone ${RCLONE_VERSION}..."
RCLONE_BIN="${PACKAGES_ROOT}/bin/rclone"
ensure_binary "${RCLONE_BIN}" "${RCLONE_VERSION}" \
    "${RCLONE_BIN} version 2>&1 | awk 'NR==1{sub(/^v/,\"\",\$2); print \$2}'" \
    "cd /tmp && rm -rf rclone-v${RCLONE_VERSION}-linux-amd64 rclone.zip && curl -sL 'https://downloads.rclone.org/v${RCLONE_VERSION}/rclone-v${RCLONE_VERSION}-linux-amd64.zip' -o rclone.zip && python3 -c \"import zipfile; z=zipfile.ZipFile('rclone.zip'); name=[n for n in z.namelist() if n.endswith('/rclone')][0]; z.extract(name,'.')\" && cp rclone-v${RCLONE_VERSION}-linux-amd64/rclone '${RCLONE_BIN}' && chmod +x '${RCLONE_BIN}' && rm -rf rclone-v${RCLONE_VERSION}-linux-amd64 rclone.zip && cd ->/dev/null" \
    "${RCLONE_BIN}"

echo "→ mc ${MC_VERSION}..."
MC_BIN="${PACKAGES_ROOT}/bin/mc"
ensure_binary "${MC_BIN}" "${MC_VERSION}" \
    "${MC_BIN} --version 2>&1 | head -1 | grep -o 'RELEASE\\.[^ ]*'" \
    "curl -sL 'https://dl.min.io/client/mc/release/linux-amd64/mc' -o '${MC_BIN}' && chmod +x '${MC_BIN}'" \
    "${MC_BIN}"

echo "→ yt-dlp ${YT_DLP_VERSION}..."
YT_DLP_BIN="${PACKAGES_ROOT}/bin/yt-dlp"
ensure_binary "${YT_DLP_BIN}" "${YT_DLP_VERSION}" \
    "${YT_DLP_BIN} --version 2>&1 | head -1" \
    "cp /usr/bin/yt-dlp '${YT_DLP_BIN}' && chmod +x '${YT_DLP_BIN}'" \
    "${YT_DLP_BIN}"

echo "→ ffmpeg ${FFMPEG_VERSION}..."
FFMPEG_BIN="${PACKAGES_ROOT}/bin/ffmpeg"
ensure_binary "${FFMPEG_BIN}" "${FFMPEG_VERSION}" \
    "${FFMPEG_BIN} -version 2>&1 | head -1 | sed -n 's/^ffmpeg version \\([^ -]*\\).*/\\1/p'" \
    "cd /tmp && rm -rf ffmpeg-* ffmpeg.tar.xz && curl -sL 'https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-amd64-static.tar.xz' -o ffmpeg.tar.xz && tar -xJf ffmpeg.tar.xz && cp \$(find ffmpeg-* -type f -name ffmpeg | head -1) '${FFMPEG_BIN}' && chmod +x '${FFMPEG_BIN}' && rm -rf ffmpeg-* ffmpeg.tar.xz && cd ->/dev/null" \
    "${FFMPEG_BIN}"

echo "→ sha256sum ${COREUTILS_VERSION}..."
SHA256SUM_BIN="${PACKAGES_ROOT}/bin/sha256sum"
ensure_binary "${SHA256SUM_BIN}" "${COREUTILS_VERSION}" \
    "${SHA256SUM_BIN} --version 2>&1 | head -1 | awk '{print \$4}'" \
    "cp /usr/bin/sha256sum '${SHA256SUM_BIN}' && chmod +x '${SHA256SUM_BIN}'" \
    "${SHA256SUM_BIN}"

# MCP server — built from ./mcp, packaged as mcp_server.
# Do not reuse the historical packages/bin/mcp alias; metadata/registry/specs
# require mcp_server and the old name has repeatedly reintroduced payload drift.
rm -f "${PACKAGES_ROOT}/bin/mcp"
if [[ -f "${SERVICES_STAGE}/mcp" ]]; then
    copy_go_bin "${SERVICES_STAGE}/mcp" "mcp_server" "mcp_server"
elif [[ -x "${PACKAGES_ROOT}/bin/mcp_server" ]]; then
    echo "  ✓ mcp_server already in packages/bin/ ($(ls -lh "${PACKAGES_ROOT}/bin/mcp_server" | awk '{print $5}'))"
else
    echo "→ Building mcp server (fallback)..."
    (cd "${SERVICES_ROOT}/golang" && GOOS=linux GOARCH=amd64 go build -o "${PACKAGES_ROOT}/bin/mcp_server" ./mcp)
    chmod +x "${PACKAGES_ROOT}/bin/mcp_server"
    strip_release_binary "${PACKAGES_ROOT}/bin/mcp_server" "mcp_server"
    echo "  ✓ mcp_server ($(ls -lh "${PACKAGES_ROOT}/bin/mcp_server" | awk '{print $5}'))"
fi

# Bundle intent graph nodes into the MCP package payload so they are deployed
# to /var/lib/globular/intent/ on every node that installs the MCP package.
# The data/intent/ directory inside the payload root is bundled automatically
# by 'globular pkg build' and extracted as PACKAGE_ROOT/data/intent/ during
# install, where the post-install.sh script copies them to /var/lib/globular/intent/.
MCP_INTENT_SRC="${SERVICES_ROOT}/docs/intent"
MCP_DATA_DIR="${PACKAGES_ROOT}/metadata/mcp/data/intent"
if [[ -d "${MCP_INTENT_SRC}" ]]; then
    mkdir -p "${MCP_DATA_DIR}"
    cp -a "${MCP_INTENT_SRC}/." "${MCP_DATA_DIR}/"
    echo "  ✓ intent nodes bundled into mcp payload ($(ls "${MCP_DATA_DIR}" | wc -l) files)"
else
    echo "  ⚠ intent source not found at ${MCP_INTENT_SRC} — skipping intent bundle"
fi

# Claude CLI — NOTHING to bundle here. As of the fetch-at-install redesign
# (packages commit 31fb144), the claude package is a wrapper: its spec has
# entrypoint=noop and ships only the shared `noop` sentinel in the payload.
# The real ~250MB proprietary binary is fetched + sha256-verified at INSTALL
# time by scripts/install-claude.sh and lands at /usr/local/bin/claude (where
# ai_executor probes first), NOT /usr/lib/globular/bin. Do not copy or npm-install
# claude into packages/bin/ — packages/build.sh bundles bin/noop via the spec
# entrypoint and the binary in packages/bin/claude is ignored.
echo "→ claude: wrapper package (no payload binary; fetched at install)"

echo ""

# ── Step 2: Build infrastructure packages ─────────────────────────────────
# Versions come from spec metadata — one command builds all 22 infra packages.
echo "━━━ Step 2: Build Infrastructure Packages ━━━"
echo ""

cd "${PACKAGES_ROOT}"

rm -rf "${DIST_DIR}"
mkdir -p "${DIST_DIR}"

export GLOBULAR_BIN="${PACKAGES_ROOT}/bin/globular"
echo "→ Running packages/build.sh (all 22 specs, versions from metadata)..."
bash build.sh --out "${DIST_DIR}"

INFRA_COUNT=$(ls "${DIST_DIR}"/*.tgz 2>/dev/null | wc -l)
echo ""
echo "  ✓ ${INFRA_COUNT} infrastructure packages built"

echo ""

# ── Step 3: Build service packages ────────────────────────────────────────
echo "━━━ Step 3: Build Service Packages ━━━"
echo ""

cd "${SERVICES_ROOT}"

# Remove legacy binary names from the stage directory
echo "→ Removing legacy binary names from stage directory..."
for _old in clustercontroller_server clusterdoctor_server nodeagent_server \
            cluster_controller node_agent compute_server discovery_server; do
    if [[ -e "${SERVICES_STAGE}/${_old}" || -L "${SERVICES_STAGE}/${_old}" ]]; then
        rm -f "${SERVICES_STAGE}/${_old}"
        echo "  removed ${_old}"
    fi
done
echo ""

echo "→ Step 3a: Generate service specs..."
if [[ -f "golang/globularcli/tools/specgen/specgen.sh" ]]; then
    bash golang/globularcli/tools/specgen/specgen.sh \
        "${SERVICES_STAGE}" \
        "${SERVICES_OUTPUT}"
    echo "  ✓ Specs generated"
else
    echo "  ✗ specgen.sh not found"
    exit 1
fi

echo ""
echo "→ Step 3b: Build service packages..."

# Resolve the per-package versions file.
# Priority: VERSIONS_FILE env var → build/package-versions.txt → release-index.json auto-generate.
# Never hardcode a single platform version for all packages — that violates the BOM invariant.
VERSIONS_FILE="${VERSIONS_FILE:-}"
if [[ -z "${VERSIONS_FILE}" && -f "${SERVICES_ROOT}/golang/build/package-versions.txt" ]]; then
    VERSIONS_FILE="${SERVICES_ROOT}/golang/build/package-versions.txt"
fi
if [[ -z "${VERSIONS_FILE}" ]]; then
    # Auto-generate from active release-index if available.
    RELEASE_INDEX="/var/lib/globular/release-index.json"
    GEN_VERSIONS_FILE="${SERVICES_ROOT}/golang/build/package-versions.txt"
    if [[ -f "${RELEASE_INDEX}" ]]; then
        echo "→ Generating package-versions.txt from ${RELEASE_INDEX}..."
        python3 - <<PYEOF
import json, sys
with open("${RELEASE_INDEX}") as f:
    idx = json.load(f)
lines = []
for p in idx.get("packages", []):
    name = p.get("name", "")
    ver  = p.get("version", "")
    if name and ver:
        lines.append(f"{name}={ver}")
lines.sort()
with open("${GEN_VERSIONS_FILE}", "w") as f:
    f.write(f"# Auto-generated from {RELEASE_INDEX} — do not edit by hand\n")
    f.write(f"# platform_release: {idx.get('platform_release','?')}\n")
    for l in lines:
        f.write(l + "\n")
print(f"  wrote {len(lines)} entries to ${GEN_VERSIONS_FILE}")
PYEOF
        VERSIONS_FILE="${GEN_VERSIONS_FILE}"
    else
        echo "ERROR: no VERSIONS_FILE provided and ${RELEASE_INDEX} not found." >&2
        echo "       Set VERSIONS_FILE=<path> or create golang/build/package-versions.txt" >&2
        echo "       with one 'svcname=version' per line matching the active BOM." >&2
        exit 1
    fi
fi
echo "  using versions file: ${VERSIONS_FILE}"

if [[ -f "golang/globularcli/tools/pkggen/pkggen.sh" ]]; then
    bash golang/globularcli/tools/pkggen/pkggen.sh \
        --globular "${SERVICES_STAGE}/globularcli" \
        --bin-dir "${SERVICES_STAGE}" \
        --gen-root "${SERVICES_OUTPUT}" \
        --out "${DIST_DIR}" \
        --publisher "core@globular.io" \
        --platform "linux_amd64" \
        --versions-file "${VERSIONS_FILE}"
    echo "  ✓ Service packages built"
else
    echo "  ✗ pkggen.sh not found"
    exit 1
fi

SERVICE_COUNT=$(ls "${DIST_DIR}"/*.tgz 2>/dev/null | wc -l)
SERVICE_COUNT=$((SERVICE_COUNT - INFRA_COUNT))
echo ""
echo "  ✓ ${SERVICE_COUNT} service packages built"

echo ""

# ── Step 4: Publish packages to repository (if running) ───────────────────
echo "━━━ Step 4: Publish Packages to Repository ━━━"
echo ""

REPO_ADDR="${GLOBULAR_REPO_ADDR:-localhost:443}"
GLOBULARCLI="${SERVICES_STAGE}/globularcli"
PUBLISHED=0

if [[ -x "${GLOBULARCLI}" ]]; then
    echo "→ Publishing ${DIST_DIR}/*.tgz to repository at ${REPO_ADDR}..."
    SKIPPED=0
    for pkg in "${DIST_DIR}"/*.tgz; do
        if [[ -f "${pkg}" ]]; then
            name=$(basename "${pkg}")
            # Capture stderr to detect "already published" vs real failures.
            # Do NOT use --force: re-publishing a version that already exists
            # generates a new build_id for an identical artifact, which causes
            # build_id drift across the 4 layers (desired updates, installed
            # stays on old build_id → reconciler re-installs identical binaries).
            out=$("${GLOBULARCLI}" pkg publish --repository "${REPO_ADDR}" --file "${pkg}" 2>&1)
            rc=$?
            if [[ ${rc} -eq 0 ]]; then
                echo "  ✓ ${name}"
                PUBLISHED=$((PUBLISHED + 1))
            elif echo "${out}" | grep -qi "already.published\|already_exists\|AlreadyExists\|already exists"; then
                echo "  = ${name} (already published — skipped)"
                SKIPPED=$((SKIPPED + 1))
                PUBLISHED=$((PUBLISHED + 1))
            else
                echo "  ✗ ${name} (publish failed — repository may be unavailable)"
            fi
        fi
    done
    echo ""
    echo "  ✓ ${PUBLISHED} packages published to repository (${SKIPPED} already existed, skipped)"
else
    echo "  ⚠ globularcli not found — skipping repository publish"
    echo "  → Packages are in ${DIST_DIR}/ for manual publish"
fi

echo ""

# ── Step 5: Copy all packages to installer assets ─────────────────────────
echo "━━━ Step 5: Copy Packages to Installer Assets ━━━"
echo ""

echo "→ Syncing dist/ to installer assets..."
if [[ -d "${INSTALLER_ASSETS}" ]]; then
    rm -f "${INSTALLER_ASSETS}"/*.tgz
else
    mkdir -p "${INSTALLER_ASSETS}"
fi

TOTAL=0
for pkg in "${DIST_DIR}"/*.tgz; do
    if [[ -f "${pkg}" ]]; then
        cp "${pkg}" "${INSTALLER_ASSETS}/"
        TOTAL=$((TOTAL + 1))
    fi
done
echo "  ✓ ${TOTAL} packages copied to installer"

echo ""

# ── Step 6: Summary ───────────────────────────────────────────────────────
echo "━━━ Step 6: Package Summary ━━━"
echo ""

echo "Packages in dist/:"
ls "${DIST_DIR}"/*.tgz 2>/dev/null | sed 's|.*/||' | sort
echo ""

echo "╔════════════════════════════════════════════════════════════════╗"
if [[ ${TOTAL} -gt 0 ]]; then
    echo "║     ✓ ALL PACKAGES REBUILT AND REPACKED                       ║"
    echo "╚════════════════════════════════════════════════════════════════╝"
    echo ""
    echo "Summary:"
    echo "  Infrastructure packages: ${INFRA_COUNT}"
    echo "  Service packages:        ${SERVICE_COUNT}"
    echo "  Total packages:          ${TOTAL}"
else
    echo "║     ⚠ NO PACKAGES FOUND                                        ║"
    echo "╚════════════════════════════════════════════════════════════════╝"
    exit 1
fi

echo ""
echo "Next steps:"
echo "  1. Test installation: cd globular-installer && sudo ./scripts/install-day0.sh"
echo "  2. Verify all services start correctly"
echo ""
