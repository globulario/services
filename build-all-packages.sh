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
GLOBULAR_BIN="${GLOBULAR_ROOT}/Globular/.bin"
INSTALLER_ASSETS="${GLOBULAR_ROOT}/globular-installer/internal/assets/packages"
SERVICES_STAGE="${SERVICES_ROOT}/golang/tools/stage/linux-amd64/usr/local/bin"
SERVICES_OUTPUT="${SERVICES_ROOT}/generated"

# Versions
ENVOY_VERSION="1.35.3"
ETCD_VERSION="3.5.14"
PROMETHEUS_VERSION="3.5.1"
NODE_EXPORTER_VERSION="1.10.2"
SIDEKICK_VERSION="7.0.0"
RESTIC_VERSION="0.18.1"
SCYLLADB_VERSION="2025.3.1"
SCYLLA_MANAGER_VERSION="3.8.1"
YT_DLP_VERSION="2026.02.21"
FFMPEG_VERSION="7.0.2"
COREUTILS_VERSION="9.4.0"
RCLONE_VERSION="1.73.1"

echo "━━━ Configuration ━━━"
echo ""
echo "  Envoy version:         ${ENVOY_VERSION}"
echo "  etcd version:          ${ETCD_VERSION}"
echo "  Prometheus version:    ${PROMETHEUS_VERSION}"
echo "  Node exporter version: ${NODE_EXPORTER_VERSION}"
echo "  Sidekick version:     ${SIDEKICK_VERSION}"
echo "  ScyllaDB version:     ${SCYLLADB_VERSION}"
echo "  Scylla Manager ver:   ${SCYLLA_MANAGER_VERSION}"
echo "  Restic version:       ${RESTIC_VERSION}"
echo "  yt-dlp version:      ${YT_DLP_VERSION}"
echo "  ffmpeg version:      ${FFMPEG_VERSION}"
echo "  Coreutils version:   ${COREUTILS_VERSION} (sha256sum)"
echo "  Rclone version:       ${RCLONE_VERSION}"
echo ""
echo "  Packages root:        ${PACKAGES_ROOT}"
echo "  Services root:        ${SERVICES_ROOT}"
echo "  Globular binaries:    ${GLOBULAR_BIN}"
echo "  Installer assets:     ${INSTALLER_ASSETS}"
echo ""

# Step 1: Prepare infrastructure binaries
echo "━━━ Step 1: Prepare Infrastructure Binaries ━━━"
echo ""

echo "→ Creating packages/bin directory..."
mkdir -p "${PACKAGES_ROOT}/bin"

echo "→ Copying gateway_server -> gateway..."
if [[ -f "${GLOBULAR_BIN}/gateway_server" ]]; then
    cp "${GLOBULAR_BIN}/gateway_server" "${PACKAGES_ROOT}/bin/gateway"
    chmod +x "${PACKAGES_ROOT}/bin/gateway"
    echo "  ✓ gateway ($(ls -lh "${PACKAGES_ROOT}/bin/gateway" | awk '{print $5}'))"
else
    echo "  ✗ gateway_server not found in ${GLOBULAR_BIN}"
    exit 1
fi

echo "→ Copying xds_server -> xds..."
if [[ -f "${GLOBULAR_BIN}/xds_server" ]]; then
    cp "${GLOBULAR_BIN}/xds_server" "${PACKAGES_ROOT}/bin/xds"
    chmod +x "${PACKAGES_ROOT}/bin/xds"
    echo "  ✓ xds ($(ls -lh "${PACKAGES_ROOT}/bin/xds" | awk '{print $5}'))"
else
    echo "  ✗ xds_server not found in ${GLOBULAR_BIN}"
    exit 1
fi

echo "→ Copying globularcli..."
if [[ -f "${SERVICES_STAGE}/globularcli" ]]; then
    cp "${SERVICES_STAGE}/globularcli" "${PACKAGES_ROOT}/bin/globularcli"
    chmod +x "${PACKAGES_ROOT}/bin/globularcli"
    echo "  ✓ globularcli ($(ls -lh "${PACKAGES_ROOT}/bin/globularcli" | awk '{print $5}'))"
else
    echo "  ✗ globularcli not found in ${SERVICES_STAGE}"
    exit 1
fi

echo ""
echo "→ Checking/downloading Envoy ${ENVOY_VERSION}..."
ENVOY_BIN="${PACKAGES_ROOT}/bin/envoy"
ENVOY_URL="https://github.com/envoyproxy/envoy/releases/download/v${ENVOY_VERSION}/envoy-${ENVOY_VERSION}-linux-x86_64"

if [[ -f "${ENVOY_BIN}" ]]; then
    ENVOY_CURRENT=$(${ENVOY_BIN} --version 2>&1 | grep -oP 'version: \K[0-9.]+' || echo "unknown")
    if [[ "${ENVOY_CURRENT}" == "${ENVOY_VERSION}" ]]; then
        echo "  ✓ envoy ${ENVOY_VERSION} already present"
    else
        echo "  ⚠ envoy version mismatch (${ENVOY_CURRENT}), downloading ${ENVOY_VERSION}..."
        rm -f "${ENVOY_BIN}"
        curl -L "${ENVOY_URL}" -o "${ENVOY_BIN}"
        chmod +x "${ENVOY_BIN}" || true
        echo "  ✓ envoy ${ENVOY_VERSION} downloaded"
    fi
else
    echo "  → Downloading envoy ${ENVOY_VERSION}..."
    rm -f "${ENVOY_BIN}"
    curl -L "${ENVOY_URL}" -o "${ENVOY_BIN}"
    chmod +x "${ENVOY_BIN}" || true
    echo "  ✓ envoy ${ENVOY_VERSION} downloaded ($(ls -lh "${ENVOY_BIN}" | awk '{print $5}'))"
fi

echo ""
echo "→ Checking/downloading etcd ${ETCD_VERSION}..."
ETCD_BIN="${PACKAGES_ROOT}/bin/etcd"
ETCD_ARCHIVE="etcd-v${ETCD_VERSION}-linux-amd64.tar.gz"
ETCD_URL="https://github.com/etcd-io/etcd/releases/download/v${ETCD_VERSION}/${ETCD_ARCHIVE}"

if [[ -f "${ETCD_BIN}" ]]; then
    ETCD_CURRENT=$(${ETCD_BIN} --version 2>&1 | grep -oP 'etcd Version: \K[0-9.]+' || echo "unknown")
    if [[ "${ETCD_CURRENT}" == "${ETCD_VERSION}" ]]; then
        echo "  ✓ etcd ${ETCD_VERSION} already present"
    else
        echo "  ⚠ etcd version mismatch (${ETCD_CURRENT}), downloading ${ETCD_VERSION}..."
        rm -f "${ETCD_BIN}" "${PACKAGES_ROOT}/bin/etcdctl"
        cd /tmp
        curl -L "${ETCD_URL}" -o "${ETCD_ARCHIVE}"
        tar xzf "${ETCD_ARCHIVE}"
        cp "etcd-v${ETCD_VERSION}-linux-amd64/etcd" "${ETCD_BIN}"
        cp "etcd-v${ETCD_VERSION}-linux-amd64/etcdctl" "${PACKAGES_ROOT}/bin/etcdctl"
        chmod +x "${ETCD_BIN}" "${PACKAGES_ROOT}/bin/etcdctl" || true
        rm -rf "etcd-v${ETCD_VERSION}-linux-amd64" "${ETCD_ARCHIVE}"
        cd - > /dev/null
        echo "  ✓ etcd ${ETCD_VERSION} downloaded"
    fi
else
    echo "  → Downloading etcd ${ETCD_VERSION}..."
    rm -f "${ETCD_BIN}" "${PACKAGES_ROOT}/bin/etcdctl"
    cd /tmp
    curl -L "${ETCD_URL}" -o "${ETCD_ARCHIVE}"
    tar xzf "${ETCD_ARCHIVE}"
    cp "etcd-v${ETCD_VERSION}-linux-amd64/etcd" "${ETCD_BIN}"
    cp "etcd-v${ETCD_VERSION}-linux-amd64/etcdctl" "${PACKAGES_ROOT}/bin/etcdctl"
    chmod +x "${ETCD_BIN}" "${PACKAGES_ROOT}/bin/etcdctl" || true
    rm -rf "etcd-v${ETCD_VERSION}-linux-amd64" "${ETCD_ARCHIVE}"
    cd - > /dev/null
    echo "  ✓ etcd ${ETCD_VERSION} downloaded ($(ls -lh "${ETCD_BIN}" | awk '{print $5}'))"
fi

echo ""
echo "→ Checking/downloading Prometheus ${PROMETHEUS_VERSION}..."
PROMETHEUS_BIN="${PACKAGES_ROOT}/bin/prometheus"
PROMTOOL_BIN="${PACKAGES_ROOT}/bin/promtool"
PROMETHEUS_ARCHIVE="prometheus-${PROMETHEUS_VERSION}.linux-amd64.tar.gz"
PROMETHEUS_URL="https://github.com/prometheus/prometheus/releases/download/v${PROMETHEUS_VERSION}/${PROMETHEUS_ARCHIVE}"

if [[ -f "${PROMETHEUS_BIN}" ]]; then
    PROM_CURRENT=$(${PROMETHEUS_BIN} --version 2>&1 | grep -oP 'version \K[0-9.]+' || echo "unknown")
    if [[ "${PROM_CURRENT}" == "${PROMETHEUS_VERSION}" ]]; then
        echo "  ✓ prometheus ${PROMETHEUS_VERSION} already present"
    else
        echo "  ⚠ prometheus version mismatch (${PROM_CURRENT}), downloading ${PROMETHEUS_VERSION}..."
        rm -f "${PROMETHEUS_BIN}" "${PROMTOOL_BIN}"
        cd /tmp
        curl -L "${PROMETHEUS_URL}" -o "${PROMETHEUS_ARCHIVE}"
        tar xzf "${PROMETHEUS_ARCHIVE}"
        cp "prometheus-${PROMETHEUS_VERSION}.linux-amd64/prometheus" "${PROMETHEUS_BIN}"
        cp "prometheus-${PROMETHEUS_VERSION}.linux-amd64/promtool" "${PROMTOOL_BIN}"
        chmod +x "${PROMETHEUS_BIN}" "${PROMTOOL_BIN}" || true
        rm -rf "prometheus-${PROMETHEUS_VERSION}.linux-amd64" "${PROMETHEUS_ARCHIVE}"
        cd - > /dev/null
        echo "  ✓ prometheus ${PROMETHEUS_VERSION} downloaded"
    fi
else
    echo "  → Downloading prometheus ${PROMETHEUS_VERSION}..."
    rm -f "${PROMETHEUS_BIN}" "${PROMTOOL_BIN}"
    cd /tmp
    curl -L "${PROMETHEUS_URL}" -o "${PROMETHEUS_ARCHIVE}"
    tar xzf "${PROMETHEUS_ARCHIVE}"
    cp "prometheus-${PROMETHEUS_VERSION}.linux-amd64/prometheus" "${PROMETHEUS_BIN}"
    cp "prometheus-${PROMETHEUS_VERSION}.linux-amd64/promtool" "${PROMTOOL_BIN}"
    chmod +x "${PROMETHEUS_BIN}" "${PROMTOOL_BIN}" || true
    rm -rf "prometheus-${PROMETHEUS_VERSION}.linux-amd64" "${PROMETHEUS_ARCHIVE}"
    cd - > /dev/null
    echo "  ✓ prometheus ${PROMETHEUS_VERSION} downloaded ($(ls -lh "${PROMETHEUS_BIN}" | awk '{print $5}'))"
fi

echo ""
echo "→ Checking/downloading node_exporter ${NODE_EXPORTER_VERSION}..."
NODE_EXPORTER_BIN="${PACKAGES_ROOT}/bin/node_exporter"
NODE_EXPORTER_ARCHIVE="node_exporter-${NODE_EXPORTER_VERSION}.linux-amd64.tar.gz"
NODE_EXPORTER_URL="https://github.com/prometheus/node_exporter/releases/download/v${NODE_EXPORTER_VERSION}/${NODE_EXPORTER_ARCHIVE}"

if [[ -f "${NODE_EXPORTER_BIN}" ]]; then
    NE_CURRENT=$(${NODE_EXPORTER_BIN} --version 2>&1 | grep -oP 'version \K[0-9.]+' || echo "unknown")
    if [[ "${NE_CURRENT}" == "${NODE_EXPORTER_VERSION}" ]]; then
        echo "  ✓ node_exporter ${NODE_EXPORTER_VERSION} already present"
    else
        echo "  ⚠ node_exporter version mismatch (${NE_CURRENT}), downloading ${NODE_EXPORTER_VERSION}..."
        rm -f "${NODE_EXPORTER_BIN}"
        cd /tmp
        curl -L "${NODE_EXPORTER_URL}" -o "${NODE_EXPORTER_ARCHIVE}"
        tar xzf "${NODE_EXPORTER_ARCHIVE}"
        cp "node_exporter-${NODE_EXPORTER_VERSION}.linux-amd64/node_exporter" "${NODE_EXPORTER_BIN}"
        chmod +x "${NODE_EXPORTER_BIN}" || true
        rm -rf "node_exporter-${NODE_EXPORTER_VERSION}.linux-amd64" "${NODE_EXPORTER_ARCHIVE}"
        cd - > /dev/null
        echo "  ✓ node_exporter ${NODE_EXPORTER_VERSION} downloaded"
    fi
else
    echo "  → Downloading node_exporter ${NODE_EXPORTER_VERSION}..."
    rm -f "${NODE_EXPORTER_BIN}"
    cd /tmp
    curl -L "${NODE_EXPORTER_URL}" -o "${NODE_EXPORTER_ARCHIVE}"
    tar xzf "${NODE_EXPORTER_ARCHIVE}"
    cp "node_exporter-${NODE_EXPORTER_VERSION}.linux-amd64/node_exporter" "${NODE_EXPORTER_BIN}"
    chmod +x "${NODE_EXPORTER_BIN}" || true
    rm -rf "node_exporter-${NODE_EXPORTER_VERSION}.linux-amd64" "${NODE_EXPORTER_ARCHIVE}"
    cd - > /dev/null
    echo "  ✓ node_exporter ${NODE_EXPORTER_VERSION} downloaded ($(ls -lh "${NODE_EXPORTER_BIN}" | awk '{print $5}'))"
fi

echo ""
echo "→ Checking/downloading sidekick ${SIDEKICK_VERSION}..."
SIDEKICK_BIN="${PACKAGES_ROOT}/bin/sidekick"
SIDEKICK_URL="https://github.com/minio/sidekick/releases/download/v${SIDEKICK_VERSION}/sidekick-linux-amd64"

if [[ -f "${SIDEKICK_BIN}" ]]; then
    SK_CURRENT=$(${SIDEKICK_BIN} --version 2>&1 | grep -oP '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || echo "unknown")
    if [[ "${SK_CURRENT}" == "${SIDEKICK_VERSION}" ]]; then
        echo "  ✓ sidekick ${SIDEKICK_VERSION} already present"
    else
        echo "  ⚠ sidekick version mismatch (${SK_CURRENT}), downloading ${SIDEKICK_VERSION}..."
        rm -f "${SIDEKICK_BIN}"
        curl -L "${SIDEKICK_URL}" -o "${SIDEKICK_BIN}"
        chmod +x "${SIDEKICK_BIN}"
        echo "  ✓ sidekick ${SIDEKICK_VERSION} downloaded"
    fi
else
    echo "  → Downloading sidekick ${SIDEKICK_VERSION}..."
    curl -L "${SIDEKICK_URL}" -o "${SIDEKICK_BIN}"
    chmod +x "${SIDEKICK_BIN}"
    echo "  ✓ sidekick ${SIDEKICK_VERSION} downloaded ($(ls -lh "${SIDEKICK_BIN}" | awk '{print $5}'))"
fi

echo ""

echo "→ Checking/downloading scylla-manager-agent ${SCYLLA_MANAGER_VERSION}..."
SCYLLA_AGENT_BIN="${PACKAGES_ROOT}/bin/scylla_manager_agent"
SCYLLA_AGENT_DEB="scylla-manager-agent_${SCYLLA_MANAGER_VERSION}.linux_amd64.deb"
SCYLLA_AGENT_URL="https://github.com/scylladb/scylla-manager/releases/download/v${SCYLLA_MANAGER_VERSION}/${SCYLLA_AGENT_DEB}"

if [[ -f "${SCYLLA_AGENT_BIN}" ]]; then
    SA_CURRENT=$(${SCYLLA_AGENT_BIN} --version 2>&1 | head -1 || echo "unknown")
    if [[ "${SA_CURRENT}" == "${SCYLLA_MANAGER_VERSION}" ]]; then
        echo "  ✓ scylla-manager-agent ${SCYLLA_MANAGER_VERSION} already present"
    else
        echo "  ⚠ scylla-manager-agent version mismatch (${SA_CURRENT}), downloading ${SCYLLA_MANAGER_VERSION}..."
        rm -f "${SCYLLA_AGENT_BIN}"
        cd /tmp
        curl -L "${SCYLLA_AGENT_URL}" -o "${SCYLLA_AGENT_DEB}"
        mkdir -p scylla-agent-extract
        dpkg-deb -x "${SCYLLA_AGENT_DEB}" scylla-agent-extract/
        cp scylla-agent-extract/usr/bin/scylla-manager-agent "${SCYLLA_AGENT_BIN}"
        chmod +x "${SCYLLA_AGENT_BIN}"
        rm -rf scylla-agent-extract "${SCYLLA_AGENT_DEB}"
        cd - > /dev/null
        echo "  ✓ scylla-manager-agent ${SCYLLA_MANAGER_VERSION} downloaded"
    fi
else
    echo "  → Downloading scylla-manager-agent ${SCYLLA_MANAGER_VERSION}..."
    cd /tmp
    curl -L "${SCYLLA_AGENT_URL}" -o "${SCYLLA_AGENT_DEB}"
    mkdir -p scylla-agent-extract
    dpkg-deb -x "${SCYLLA_AGENT_DEB}" scylla-agent-extract/
    cp scylla-agent-extract/usr/bin/scylla-manager-agent "${SCYLLA_AGENT_BIN}"
    chmod +x "${SCYLLA_AGENT_BIN}"
    rm -rf scylla-agent-extract "${SCYLLA_AGENT_DEB}"
    cd - > /dev/null
    echo "  ✓ scylla-manager-agent ${SCYLLA_MANAGER_VERSION} downloaded ($(ls -lh "${SCYLLA_AGENT_BIN}" | awk '{print $5}'))"
fi

echo ""
echo "→ Checking/downloading scylla-manager ${SCYLLA_MANAGER_VERSION}..."
SCYLLA_MGR_BIN="${PACKAGES_ROOT}/bin/scylla_manager"
SCYLLA_MGR_DEB="scylla-manager-server_${SCYLLA_MANAGER_VERSION}.linux_amd64.deb"
SCYLLA_MGR_URL="https://github.com/scylladb/scylla-manager/releases/download/v${SCYLLA_MANAGER_VERSION}/${SCYLLA_MGR_DEB}"

if [[ -f "${SCYLLA_MGR_BIN}" ]]; then
    SM_CURRENT=$(${SCYLLA_MGR_BIN} --version 2>&1 | head -1 || echo "unknown")
    if [[ "${SM_CURRENT}" == "${SCYLLA_MANAGER_VERSION}" ]]; then
        echo "  ✓ scylla-manager ${SCYLLA_MANAGER_VERSION} already present"
    else
        echo "  ⚠ scylla-manager version mismatch (${SM_CURRENT}), downloading ${SCYLLA_MANAGER_VERSION}..."
        rm -f "${SCYLLA_MGR_BIN}"
        cd /tmp
        curl -L "${SCYLLA_MGR_URL}" -o "${SCYLLA_MGR_DEB}"
        mkdir -p scylla-mgr-extract
        dpkg-deb -x "${SCYLLA_MGR_DEB}" scylla-mgr-extract/
        cp scylla-mgr-extract/usr/bin/scylla-manager "${SCYLLA_MGR_BIN}"
        chmod +x "${SCYLLA_MGR_BIN}"
        rm -rf scylla-mgr-extract "${SCYLLA_MGR_DEB}"
        cd - > /dev/null
        echo "  ✓ scylla-manager ${SCYLLA_MANAGER_VERSION} downloaded"
    fi
else
    echo "  → Downloading scylla-manager ${SCYLLA_MANAGER_VERSION}..."
    cd /tmp
    curl -L "${SCYLLA_MGR_URL}" -o "${SCYLLA_MGR_DEB}"
    mkdir -p scylla-mgr-extract
    dpkg-deb -x "${SCYLLA_MGR_DEB}" scylla-mgr-extract/
    cp scylla-mgr-extract/usr/bin/scylla-manager "${SCYLLA_MGR_BIN}"
    chmod +x "${SCYLLA_MGR_BIN}"
    rm -rf scylla-mgr-extract "${SCYLLA_MGR_DEB}"
    cd - > /dev/null
    echo "  ✓ scylla-manager ${SCYLLA_MANAGER_VERSION} downloaded ($(ls -lh "${SCYLLA_MGR_BIN}" | awk '{print $5}'))"
fi

echo ""
echo "→ Checking/downloading sctool ${SCYLLA_MANAGER_VERSION}..."
SCTOOL_BIN="${PACKAGES_ROOT}/bin/sctool"
SCTOOL_DEB="scylla-manager-client_${SCYLLA_MANAGER_VERSION}.linux_amd64.deb"
SCTOOL_URL="https://github.com/scylladb/scylla-manager/releases/download/v${SCYLLA_MANAGER_VERSION}/${SCTOOL_DEB}"

if [[ -f "${SCTOOL_BIN}" ]]; then
    SC_CURRENT=$(${SCTOOL_BIN} version 2>&1 | grep -oP 'Client version: \K[0-9.]+' || echo "unknown")
    if [[ "${SC_CURRENT}" == "${SCYLLA_MANAGER_VERSION}" ]]; then
        echo "  ✓ sctool ${SCYLLA_MANAGER_VERSION} already present"
    else
        echo "  ⚠ sctool version mismatch (${SC_CURRENT}), downloading ${SCYLLA_MANAGER_VERSION}..."
        rm -f "${SCTOOL_BIN}"
        cd /tmp
        curl -L "${SCTOOL_URL}" -o "${SCTOOL_DEB}"
        mkdir -p sctool-extract
        dpkg-deb -x "${SCTOOL_DEB}" sctool-extract/
        cp sctool-extract/usr/bin/sctool "${SCTOOL_BIN}"
        chmod +x "${SCTOOL_BIN}"
        rm -rf sctool-extract "${SCTOOL_DEB}"
        cd - > /dev/null
        echo "  ✓ sctool ${SCYLLA_MANAGER_VERSION} downloaded"
    fi
else
    echo "  → Downloading sctool ${SCYLLA_MANAGER_VERSION}..."
    cd /tmp
    curl -L "${SCTOOL_URL}" -o "${SCTOOL_DEB}"
    mkdir -p sctool-extract
    dpkg-deb -x "${SCTOOL_DEB}" sctool-extract/
    cp sctool-extract/usr/bin/sctool "${SCTOOL_BIN}"
    chmod +x "${SCTOOL_BIN}"
    rm -rf sctool-extract "${SCTOOL_DEB}"
    cd - > /dev/null
    echo "  ✓ sctool ${SCYLLA_MANAGER_VERSION} downloaded ($(ls -lh "${SCTOOL_BIN}" | awk '{print $5}'))"
fi

echo ""
echo "→ Checking/downloading yt-dlp ${YT_DLP_VERSION}..."
YT_DLP_BIN="${PACKAGES_ROOT}/bin/yt-dlp"
YT_DLP_URL="https://github.com/yt-dlp/yt-dlp/releases/download/${YT_DLP_VERSION}/yt-dlp_linux"

if [[ -f "${YT_DLP_BIN}" ]]; then
    YD_CURRENT=$(${YT_DLP_BIN} --version 2>&1 || echo "unknown")
    if [[ "${YD_CURRENT}" == "${YT_DLP_VERSION}" ]]; then
        echo "  ✓ yt-dlp ${YT_DLP_VERSION} already present"
    else
        echo "  ⚠ yt-dlp version mismatch (${YD_CURRENT}), downloading ${YT_DLP_VERSION}..."
        rm -f "${YT_DLP_BIN}"
        curl -L "${YT_DLP_URL}" -o "${YT_DLP_BIN}"
        chmod +x "${YT_DLP_BIN}"
        echo "  ✓ yt-dlp ${YT_DLP_VERSION} downloaded"
    fi
else
    echo "  → Downloading yt-dlp ${YT_DLP_VERSION}..."
    curl -L "${YT_DLP_URL}" -o "${YT_DLP_BIN}"
    chmod +x "${YT_DLP_BIN}"
    echo "  ✓ yt-dlp ${YT_DLP_VERSION} downloaded ($(ls -lh "${YT_DLP_BIN}" | awk '{print $5}'))"
fi

echo ""
echo "→ Checking/downloading ffmpeg ${FFMPEG_VERSION} (static)..."
FFMPEG_BIN="${PACKAGES_ROOT}/bin/ffmpeg"
FFMPEG_URL="https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-amd64-static.tar.xz"

if [[ -f "${FFMPEG_BIN}" ]]; then
    FF_CURRENT=$(${FFMPEG_BIN} -version 2>&1 | head -1 | grep -oP 'version \K[0-9.]+' || echo "unknown")
    if [[ "${FF_CURRENT}" == "${FFMPEG_VERSION}" ]]; then
        echo "  ✓ ffmpeg ${FFMPEG_VERSION} already present"
    else
        echo "  ⚠ ffmpeg version mismatch (${FF_CURRENT}), downloading ${FFMPEG_VERSION}..."
        rm -f "${FFMPEG_BIN}"
        cd /tmp
        curl -L "${FFMPEG_URL}" -o ffmpeg-release-amd64-static.tar.xz
        tar xf ffmpeg-release-amd64-static.tar.xz
        cp ffmpeg-*-amd64-static/ffmpeg "${FFMPEG_BIN}"
        chmod +x "${FFMPEG_BIN}"
        rm -rf ffmpeg-*-amd64-static ffmpeg-release-amd64-static.tar.xz
        cd - > /dev/null
        echo "  ✓ ffmpeg ${FFMPEG_VERSION} downloaded"
    fi
else
    echo "  → Downloading ffmpeg ${FFMPEG_VERSION} (static)..."
    cd /tmp
    curl -L "${FFMPEG_URL}" -o ffmpeg-release-amd64-static.tar.xz
    tar xf ffmpeg-release-amd64-static.tar.xz
    cp ffmpeg-*-amd64-static/ffmpeg "${FFMPEG_BIN}"
    chmod +x "${FFMPEG_BIN}"
    rm -rf ffmpeg-*-amd64-static ffmpeg-release-amd64-static.tar.xz
    cd - > /dev/null
    echo "  ✓ ffmpeg ${FFMPEG_VERSION} downloaded ($(ls -lh "${FFMPEG_BIN}" | awk '{print $5}'))"
fi

echo ""
echo "→ Copying sha256sum from system (coreutils ${COREUTILS_VERSION})..."
SHA256SUM_BIN="${PACKAGES_ROOT}/bin/sha256sum"
if [[ -f "${SHA256SUM_BIN}" ]]; then
    echo "  ✓ sha256sum already present"
else
    if [[ -x /usr/bin/sha256sum ]]; then
        cp /usr/bin/sha256sum "${SHA256SUM_BIN}"
        chmod +x "${SHA256SUM_BIN}"
        echo "  ✓ sha256sum copied from /usr/bin/sha256sum"
    else
        echo "  ✗ sha256sum not found on system"
        exit 1
    fi
fi

echo ""
echo "→ Checking/downloading restic ${RESTIC_VERSION}..."
RESTIC_BIN="${PACKAGES_ROOT}/bin/restic"
RESTIC_ARCHIVE="restic_${RESTIC_VERSION}_linux_amd64.bz2"
RESTIC_URL="https://github.com/restic/restic/releases/download/v${RESTIC_VERSION}/${RESTIC_ARCHIVE}"

if [[ -f "${RESTIC_BIN}" ]]; then
    RS_CURRENT=$(${RESTIC_BIN} version 2>&1 | grep -oP 'restic \K[0-9.]+' || echo "unknown")
    if [[ "${RS_CURRENT}" == "${RESTIC_VERSION}" ]]; then
        echo "  ✓ restic ${RESTIC_VERSION} already present"
    else
        echo "  ⚠ restic version mismatch (${RS_CURRENT}), downloading ${RESTIC_VERSION}..."
        rm -f "${RESTIC_BIN}"
        cd /tmp
        curl -L "${RESTIC_URL}" -o "${RESTIC_ARCHIVE}"
        bunzip2 -f "${RESTIC_ARCHIVE}"
        cp "restic_${RESTIC_VERSION}_linux_amd64" "${RESTIC_BIN}"
        chmod +x "${RESTIC_BIN}"
        rm -f "restic_${RESTIC_VERSION}_linux_amd64"
        cd - > /dev/null
        echo "  ✓ restic ${RESTIC_VERSION} downloaded"
    fi
else
    echo "  → Downloading restic ${RESTIC_VERSION}..."
    cd /tmp
    curl -L "${RESTIC_URL}" -o "${RESTIC_ARCHIVE}"
    bunzip2 -f "${RESTIC_ARCHIVE}"
    cp "restic_${RESTIC_VERSION}_linux_amd64" "${RESTIC_BIN}"
    chmod +x "${RESTIC_BIN}"
    rm -f "restic_${RESTIC_VERSION}_linux_amd64"
    cd - > /dev/null
    echo "  ✓ restic ${RESTIC_VERSION} downloaded ($(ls -lh "${RESTIC_BIN}" | awk '{print $5}'))"
fi

echo ""
echo "→ Checking/downloading rclone ${RCLONE_VERSION}..."
RCLONE_BIN="${PACKAGES_ROOT}/bin/rclone"
RCLONE_ZIP="rclone-v${RCLONE_VERSION}-linux-amd64.zip"
RCLONE_URL="https://downloads.rclone.org/v${RCLONE_VERSION}/${RCLONE_ZIP}"

if [[ -f "${RCLONE_BIN}" ]]; then
    RC_CURRENT=$(${RCLONE_BIN} --version 2>&1 | head -1 | grep -oP 'v\K[0-9.]+' || echo "unknown")
    if [[ "${RC_CURRENT}" == "${RCLONE_VERSION}" ]]; then
        echo "  ✓ rclone ${RCLONE_VERSION} already present"
    else
        echo "  ⚠ rclone version mismatch (${RC_CURRENT}), downloading ${RCLONE_VERSION}..."
        rm -f "${RCLONE_BIN}"
        cd /tmp
        curl -L "${RCLONE_URL}" -o "${RCLONE_ZIP}"
        unzip -o "${RCLONE_ZIP}" "rclone-v${RCLONE_VERSION}-linux-amd64/rclone"
        cp "rclone-v${RCLONE_VERSION}-linux-amd64/rclone" "${RCLONE_BIN}"
        chmod +x "${RCLONE_BIN}"
        rm -rf "rclone-v${RCLONE_VERSION}-linux-amd64" "${RCLONE_ZIP}"
        cd - > /dev/null
        echo "  ✓ rclone ${RCLONE_VERSION} downloaded"
    fi
else
    echo "  → Downloading rclone ${RCLONE_VERSION}..."
    cd /tmp
    curl -L "${RCLONE_URL}" -o "${RCLONE_ZIP}"
    unzip -o "${RCLONE_ZIP}" "rclone-v${RCLONE_VERSION}-linux-amd64/rclone"
    cp "rclone-v${RCLONE_VERSION}-linux-amd64/rclone" "${RCLONE_BIN}"
    chmod +x "${RCLONE_BIN}"
    rm -rf "rclone-v${RCLONE_VERSION}-linux-amd64" "${RCLONE_ZIP}"
    cd - > /dev/null
    echo "  ✓ rclone ${RCLONE_VERSION} downloaded ($(ls -lh "${RCLONE_BIN}" | awk '{print $5}'))"
fi

echo ""

# Step 2: Build infrastructure packages
echo "━━━ Step 2: Build Infrastructure Packages ━━━"
echo ""

cd "${PACKAGES_ROOT}"

echo "→ Cleaning old packages from packages/out/..."
if [[ -d "${PACKAGES_ROOT}/out" ]]; then
    rm -f "${PACKAGES_ROOT}/out"/*.tgz
    echo "  ✓ Old packages removed"
else
    mkdir -p "${PACKAGES_ROOT}/out"
    echo "  ✓ Output directory created"
fi

echo ""
echo "→ Running packages/build.sh..."
# Update GLOBULAR_BIN to point to the globularcli binary for packages/build.sh
export GLOBULAR_BIN="${PACKAGES_ROOT}/bin/globularcli"
if [[ -f "build.sh" ]]; then
    # Build scylladb with specific version
    echo "  → Building scylladb ${SCYLLADB_VERSION}..."
    bash build.sh --version "${SCYLLADB_VERSION}" scylladb

    # Build sctool (scylla-manager-client) with same version as scylla-manager
    echo "  → Building sctool ${SCYLLA_MANAGER_VERSION}..."
    bash build.sh --version "${SCYLLA_MANAGER_VERSION}" sctool

    # Build scylla-manager-agent with same version as scylla-manager
    echo "  → Building scylla-manager-agent ${SCYLLA_MANAGER_VERSION}..."
    bash build.sh --version "${SCYLLA_MANAGER_VERSION}" scylla_manager_agent

    # Build scylla-manager with specific version
    echo "  → Building scylla-manager ${SCYLLA_MANAGER_VERSION}..."
    bash build.sh --version "${SCYLLA_MANAGER_VERSION}" scylla_manager

    # Build etcd with specific version
    echo "  → Building etcd ${ETCD_VERSION}..."
    bash build.sh --version "${ETCD_VERSION}" etcd

    # Build envoy with specific version
    echo "  → Building envoy ${ENVOY_VERSION}..."
    bash build.sh --version "${ENVOY_VERSION}" envoy

    # Build prometheus with specific version
    echo "  → Building prometheus ${PROMETHEUS_VERSION}..."
    bash build.sh --version "${PROMETHEUS_VERSION}" prometheus

    # Build node_exporter with specific version
    echo "  → Building node_exporter ${NODE_EXPORTER_VERSION}..."
    bash build.sh --version "${NODE_EXPORTER_VERSION}" node_exporter

    # Build sidekick with specific version
    echo "  → Building sidekick ${SIDEKICK_VERSION}..."
    bash build.sh --version "${SIDEKICK_VERSION}" sidekick

    # Build etcdctl with same version as etcd
    echo "  → Building etcdctl ${ETCD_VERSION}..."
    bash build.sh --version "${ETCD_VERSION}" etcdctl

    # Build yt-dlp (convert date version to semver: 2026.02.21 -> 2026.2.21)
    YT_DLP_SEMVER="${YT_DLP_VERSION//\.0/.}"
    YT_DLP_SEMVER="${YT_DLP_SEMVER#.}"
    echo "  → Building yt-dlp ${YT_DLP_SEMVER}..."
    bash build.sh --version "${YT_DLP_SEMVER}" yt_dlp

    # Build ffmpeg with specific version
    echo "  → Building ffmpeg ${FFMPEG_VERSION}..."
    bash build.sh --version "${FFMPEG_VERSION}" ffmpeg

    # Build sha256sum with coreutils version
    echo "  → Building sha256sum ${COREUTILS_VERSION}..."
    bash build.sh --version "${COREUTILS_VERSION}" sha256sum

    # Build restic with specific version
    echo "  → Building restic ${RESTIC_VERSION}..."
    bash build.sh --version "${RESTIC_VERSION}" restic

    # Build rclone with specific version
    echo "  → Building rclone ${RCLONE_VERSION}..."
    bash build.sh --version "${RCLONE_VERSION}" rclone

    # Build other infrastructure packages with version 0.0.1
    echo "  → Building other infrastructure packages (0.0.1)..."
    bash build.sh --version "0.0.1" gateway xds minio mc globular_cli keepalived

    echo ""
    echo "  ✓ Infrastructure packages built"
else
    echo "  ✗ build.sh not found in ${PACKAGES_ROOT}"
    exit 1
fi

echo ""

# Step 3: Build service packages
echo "━━━ Step 3: Build Service Packages ━━━"
echo ""

cd "${SERVICES_ROOT}"

# Remove legacy binary names from the stage directory so specgen/pkggen don't
# produce duplicate packages (e.g. both clustercontroller and cluster_controller).
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

# Clean previous service packages to avoid stale names
echo ""
echo "→ Cleaning old service packages from ${SERVICES_OUTPUT}/packages..."
rm -f "${SERVICES_OUTPUT}/packages"/*.tgz 2>/dev/null || true

echo ""
echo "→ Step 3b: Build service packages..."
if [[ -f "golang/globularcli/tools/pkggen/pkggen.sh" ]]; then
    bash golang/globularcli/tools/pkggen/pkggen.sh \
        --globular "${SERVICES_STAGE}/globularcli" \
        --bin-dir "${SERVICES_STAGE}" \
        --gen-root "${SERVICES_OUTPUT}" \
        --out "${SERVICES_OUTPUT}/packages" \
        --version "0.0.1" \
        --publisher "core@globular.io" \
        --platform "linux_amd64"
    echo "  ✓ Service packages built"
else
    echo "  ✗ pkggen.sh not found"
    exit 1
fi

echo ""

# Step 4: Copy all packages to installer assets
echo "━━━ Step 4: Copy Packages to Installer Assets ━━━"
echo ""

echo "→ Cleaning old packages from installer assets..."
if [[ -d "${INSTALLER_ASSETS}" ]]; then
    rm -f "${INSTALLER_ASSETS}"/*.tgz
    echo "  ✓ Old packages removed from installer assets"
else
    mkdir -p "${INSTALLER_ASSETS}"
    echo "  ✓ Installer assets directory created"
fi

echo ""

echo "→ Copying infrastructure packages..."
INFRA_COUNT=0
if [[ -d "${PACKAGES_ROOT}/out" ]]; then
    for pkg in "${PACKAGES_ROOT}"/out/*.tgz; do
        if [[ -f "${pkg}" ]]; then
            cp "${pkg}" "${INSTALLER_ASSETS}/"
            basename "${pkg}"
            INFRA_COUNT=$((INFRA_COUNT + 1))
        fi
    done
    echo "  ✓ ${INFRA_COUNT} infrastructure packages copied"
else
    echo "  ⚠ No infrastructure packages found in ${PACKAGES_ROOT}/out"
fi

echo ""
echo "→ Copying service packages..."
SERVICE_COUNT=0
if [[ -d "${SERVICES_OUTPUT}/packages" ]]; then
    for pkg in "${SERVICES_OUTPUT}"/packages/*.tgz; do
        if [[ -f "${pkg}" ]]; then
            cp "${pkg}" "${INSTALLER_ASSETS}/"
            basename "${pkg}"
            SERVICE_COUNT=$((SERVICE_COUNT + 1))
        fi
    done
    echo "  ✓ ${SERVICE_COUNT} service packages copied"
else
    echo "  ⚠ No service packages found in ${SERVICES_OUTPUT}/packages"
fi

echo ""

# Step 5: Summary
echo "━━━ Step 5: Package Summary ━━━"
echo ""

TOTAL_PACKAGES=$((INFRA_COUNT + SERVICE_COUNT))

echo "Packages in installer assets:"
ls -lh "${INSTALLER_ASSETS}"/*.tgz 2>/dev/null | awk '{print "  " $9 " (" $5 ")"}' | sed 's|.*/||' || echo "  (none)"

echo ""
echo "╔════════════════════════════════════════════════════════════════╗"

if [[ ${TOTAL_PACKAGES} -gt 0 ]]; then
    echo "║     ✓ ALL PACKAGES REBUILT AND REPACKED                       ║"
    echo "╚════════════════════════════════════════════════════════════════╝"
    echo ""
    echo "Summary:"
    echo "  Infrastructure packages: ${INFRA_COUNT}"
    echo "  Service packages:        ${SERVICE_COUNT}"
    echo "  Total packages:          ${TOTAL_PACKAGES}"
else
    echo "║     ⚠ NO PACKAGES FOUND                                        ║"
    echo "╚════════════════════════════════════════════════════════════════╝"
    exit 1
fi

echo ""
echo "Next steps:"
echo "  1. Test installation: cd globular-installer && sudo ./install.sh"
echo "  2. Verify all services start correctly"
echo "  3. Check certificate discovery works with new binaries"
echo ""
