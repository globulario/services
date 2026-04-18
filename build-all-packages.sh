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

# ── Step 1: Prepare infrastructure binaries ───────────────────────────────
# Copies Go binaries from Globular build output and downloads third-party
# binaries (envoy, etcd, prometheus, etc.) into packages/bin/.
# Versions are read from spec metadata — this step only ensures binaries exist.

echo "━━━ Step 1: Prepare Infrastructure Binaries ━━━"
echo ""

mkdir -p "${PACKAGES_ROOT}/bin"

# Helper: copy a Go binary from the Globular build output.
copy_go_bin() {
    local src="$1" dst="$2" label="$3"
    if [[ -f "${src}" ]]; then
        cp "${src}" "${PACKAGES_ROOT}/bin/${dst}"
        chmod +x "${PACKAGES_ROOT}/bin/${dst}"
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

# Helper: download a binary if version mismatches or missing.
ensure_binary() {
    local bin="$1" version="$2" check_cmd="$3" download_fn="$4"
    local current=""
    if [[ -f "${bin}" ]]; then
        current=$(eval "${check_cmd}" 2>&1 || echo "unknown")
        if [[ "${current}" == "${version}" ]]; then
            echo "  ✓ $(basename "${bin}") ${version} already present"
            return 0
        fi
        echo "  ⚠ $(basename "${bin}") version mismatch (${current}), downloading ${version}..."
    else
        echo "  → Downloading $(basename "${bin}") ${version}..."
    fi
    eval "${download_fn}"
    echo "  ✓ $(basename "${bin}") ${version} ($(ls -lh "${bin}" | awk '{print $5}'))"
}

# Go binaries from the stage directory (built by generateCode.sh)
echo "→ Copying Go binaries from stage..."
copy_go_bin "${SERVICES_STAGE}/gateway" "gateway" "gateway"
copy_go_bin "${SERVICES_STAGE}/xds" "xds" "xds"
copy_go_bin "${SERVICES_STAGE}/globularcli" "globularcli" "globularcli"

# Read versions from specs (single source of truth)
ENVOY_VERSION=$(spec_version "${PACKAGES_ROOT}/specs/envoy_service.yaml")
ETCD_VERSION=$(spec_version "${PACKAGES_ROOT}/specs/etcd_service.yaml")
PROMETHEUS_VERSION=$(spec_version "${PACKAGES_ROOT}/specs/prometheus_service.yaml")
ALERTMANAGER_VERSION=$(spec_version "${PACKAGES_ROOT}/specs/alertmanager_service.yaml")
NODE_EXPORTER_VERSION=$(spec_version "${PACKAGES_ROOT}/specs/node_exporter_service.yaml")
SIDEKICK_VERSION=$(spec_version "${PACKAGES_ROOT}/specs/sidekick_service.yaml")
RESTIC_VERSION=$(spec_version "${PACKAGES_ROOT}/specs/restic_cmd.yaml")
RCLONE_VERSION=$(spec_version "${PACKAGES_ROOT}/specs/rclone_cmd.yaml")
YT_DLP_VERSION=$(spec_version "${PACKAGES_ROOT}/specs/yt_dlp_cmd.yaml")
FFMPEG_VERSION=$(spec_version "${PACKAGES_ROOT}/specs/ffmpeg_cmd.yaml")
COREUTILS_VERSION=$(spec_version "${PACKAGES_ROOT}/specs/sha256sum_cmd.yaml")

echo ""
echo "→ Third-party binary versions (from spec metadata):"
echo "  envoy=${ENVOY_VERSION} etcd=${ETCD_VERSION} prometheus=${PROMETHEUS_VERSION} alertmanager=${ALERTMANAGER_VERSION}"
echo "  node_exporter=${NODE_EXPORTER_VERSION} sidekick=${SIDEKICK_VERSION}"
echo "  restic=${RESTIC_VERSION} rclone=${RCLONE_VERSION} yt-dlp=${YT_DLP_VERSION}"
echo "  ffmpeg=${FFMPEG_VERSION} sha256sum/coreutils=${COREUTILS_VERSION}"
echo ""

# Envoy
echo "→ Envoy ${ENVOY_VERSION}..."
ENVOY_BIN="${PACKAGES_ROOT}/bin/envoy"
ensure_binary "${ENVOY_BIN}" "${ENVOY_VERSION}" \
    "${ENVOY_BIN} --version 2>&1 | grep -oP 'version: \K[0-9.]+'" \
    "curl -sL 'https://github.com/envoyproxy/envoy/releases/download/v${ENVOY_VERSION}/envoy-${ENVOY_VERSION}-linux-x86_64' -o '${ENVOY_BIN}' && chmod +x '${ENVOY_BIN}'"

# etcd + etcdctl
echo "→ etcd ${ETCD_VERSION}..."
ETCD_BIN="${PACKAGES_ROOT}/bin/etcd"
ensure_binary "${ETCD_BIN}" "${ETCD_VERSION}" \
    "${ETCD_BIN} --version 2>&1 | grep -oP 'etcd Version: \K[0-9.]+'" \
    "cd /tmp && curl -sL 'https://github.com/etcd-io/etcd/releases/download/v${ETCD_VERSION}/etcd-v${ETCD_VERSION}-linux-amd64.tar.gz' -o etcd.tgz && tar xzf etcd.tgz && cp etcd-v${ETCD_VERSION}-linux-amd64/etcd '${PACKAGES_ROOT}/bin/etcd' && cp etcd-v${ETCD_VERSION}-linux-amd64/etcdctl '${PACKAGES_ROOT}/bin/etcdctl' && chmod +x '${PACKAGES_ROOT}/bin/etcd' '${PACKAGES_ROOT}/bin/etcdctl' && rm -rf etcd-v${ETCD_VERSION}-linux-amd64 etcd.tgz && cd ->/dev/null"

# Prometheus + promtool
echo "→ Prometheus ${PROMETHEUS_VERSION}..."
PROMETHEUS_BIN="${PACKAGES_ROOT}/bin/prometheus"
ensure_binary "${PROMETHEUS_BIN}" "${PROMETHEUS_VERSION}" \
    "${PROMETHEUS_BIN} --version 2>&1 | grep -oP 'version \K[0-9.]+'" \
    "cd /tmp && curl -sL 'https://github.com/prometheus/prometheus/releases/download/v${PROMETHEUS_VERSION}/prometheus-${PROMETHEUS_VERSION}.linux-amd64.tar.gz' -o prom.tgz && tar xzf prom.tgz && cp prometheus-${PROMETHEUS_VERSION}.linux-amd64/prometheus '${PACKAGES_ROOT}/bin/prometheus' && cp prometheus-${PROMETHEUS_VERSION}.linux-amd64/promtool '${PACKAGES_ROOT}/bin/promtool' && chmod +x '${PACKAGES_ROOT}/bin/prometheus' '${PACKAGES_ROOT}/bin/promtool' && rm -rf prometheus-${PROMETHEUS_VERSION}.linux-amd64 prom.tgz && cd ->/dev/null"

# Alertmanager + amtool
echo "→ Alertmanager ${ALERTMANAGER_VERSION}..."
ALERTMANAGER_BIN="${PACKAGES_ROOT}/bin/alertmanager"
ensure_binary "${ALERTMANAGER_BIN}" "${ALERTMANAGER_VERSION}" \
    "${ALERTMANAGER_BIN} --version 2>&1 | grep -oP 'version \K[0-9.]+'" \
    "cd /tmp && curl -sL 'https://github.com/prometheus/alertmanager/releases/download/v${ALERTMANAGER_VERSION}/alertmanager-${ALERTMANAGER_VERSION}.linux-amd64.tar.gz' -o am.tgz && tar xzf am.tgz && cp alertmanager-${ALERTMANAGER_VERSION}.linux-amd64/alertmanager '${PACKAGES_ROOT}/bin/alertmanager' && cp alertmanager-${ALERTMANAGER_VERSION}.linux-amd64/amtool '${PACKAGES_ROOT}/bin/amtool' && chmod +x '${PACKAGES_ROOT}/bin/alertmanager' '${PACKAGES_ROOT}/bin/amtool' && rm -rf alertmanager-${ALERTMANAGER_VERSION}.linux-amd64 am.tgz && cd ->/dev/null"

# Node exporter
echo "→ node_exporter ${NODE_EXPORTER_VERSION}..."
NODE_EXPORTER_BIN="${PACKAGES_ROOT}/bin/node_exporter"
ensure_binary "${NODE_EXPORTER_BIN}" "${NODE_EXPORTER_VERSION}" \
    "${NODE_EXPORTER_BIN} --version 2>&1 | grep -oP 'version \K[0-9.]+'" \
    "cd /tmp && curl -sL 'https://github.com/prometheus/node_exporter/releases/download/v${NODE_EXPORTER_VERSION}/node_exporter-${NODE_EXPORTER_VERSION}.linux-amd64.tar.gz' -o ne.tgz && tar xzf ne.tgz && cp node_exporter-${NODE_EXPORTER_VERSION}.linux-amd64/node_exporter '${PACKAGES_ROOT}/bin/node_exporter' && chmod +x '${PACKAGES_ROOT}/bin/node_exporter' && rm -rf node_exporter-${NODE_EXPORTER_VERSION}.linux-amd64 ne.tgz && cd ->/dev/null"

# Sidekick
echo "→ sidekick ${SIDEKICK_VERSION}..."
SIDEKICK_BIN="${PACKAGES_ROOT}/bin/sidekick"
ensure_binary "${SIDEKICK_BIN}" "${SIDEKICK_VERSION}" \
    "${SIDEKICK_BIN} --version 2>&1 | grep -oP 'version: \K[0-9.]+'" \
    "curl -sL 'https://github.com/minio/sidekick/releases/latest/download/sidekick-linux-amd64' -o '${SIDEKICK_BIN}' && chmod +x '${SIDEKICK_BIN}'"

# MCP server — should already be in stage from generateCode.sh
if [[ -f "${SERVICES_STAGE}/mcp" ]]; then
    copy_go_bin "${SERVICES_STAGE}/mcp" "mcp" "mcp"
elif [[ -x "${PACKAGES_ROOT}/bin/mcp" ]]; then
    echo "  ✓ mcp already in packages/bin/ ($(ls -lh "${PACKAGES_ROOT}/bin/mcp" | awk '{print $5}'))"
else
    echo "→ Building mcp server (fallback)..."
    (cd "${SERVICES_ROOT}/golang" && GOOS=linux GOARCH=amd64 go build -o "${PACKAGES_ROOT}/bin/mcp" ./mcp)
    chmod +x "${PACKAGES_ROOT}/bin/mcp"
    echo "  ✓ mcp ($(ls -lh "${PACKAGES_ROOT}/bin/mcp" | awk '{print $5}'))"
fi

echo ""

# ── Step 2: Build infrastructure packages ─────────────────────────────────
# Versions come from spec metadata — one command builds all 22 infra packages.
echo "━━━ Step 2: Build Infrastructure Packages ━━━"
echo ""

cd "${PACKAGES_ROOT}"

rm -rf "${DIST_DIR}"
mkdir -p "${DIST_DIR}"

export GLOBULAR_BIN="${PACKAGES_ROOT}/bin/globularcli"
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
            cluster_controller node_agent; do
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
if [[ -f "golang/globularcli/tools/pkggen/pkggen.sh" ]]; then
    bash golang/globularcli/tools/pkggen/pkggen.sh \
        --globular "${SERVICES_STAGE}/globularcli" \
        --bin-dir "${SERVICES_STAGE}" \
        --gen-root "${SERVICES_OUTPUT}" \
        --out "${DIST_DIR}" \
        --publisher "core@globular.io" \
        --platform "linux_amd64"
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
    for pkg in "${DIST_DIR}"/*.tgz; do
        if [[ -f "${pkg}" ]]; then
            name=$(basename "${pkg}")
            if "${GLOBULARCLI}" pkg publish --repository "${REPO_ADDR}" --file "${pkg}" --force >/dev/null 2>&1; then
                echo "  ✓ ${name}"
                PUBLISHED=$((PUBLISHED + 1))
            else
                echo "  ✗ ${name} (publish failed — repository may be unavailable)"
            fi
        fi
    done
    echo ""
    echo "  ✓ ${PUBLISHED} packages published to repository"
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
