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

echo "━━━ Configuration ━━━"
echo ""
echo "  Envoy version: ${ENVOY_VERSION}"
echo "  etcd version:  ${ETCD_VERSION}"
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
echo "→ Creating spec symlinks for cluster-controller and node-agent..."
# pkggen.sh converts: clustercontroller_server → clustercontroller → looks for clustercontroller_service.yaml
# But the actual spec is: cluster-controller_service.yaml
# Solution: Create symlinks so pkggen can find them

if [[ -f "${SERVICES_OUTPUT}/specs/cluster-controller_service.yaml" ]]; then
    ln -sf cluster-controller_service.yaml "${SERVICES_OUTPUT}/specs/clustercontroller_service.yaml"
    echo "  ✓ clustercontroller_service.yaml → cluster-controller_service.yaml"
else
    echo "  ⚠ cluster-controller_service.yaml not found"
fi

if [[ -f "${SERVICES_OUTPUT}/specs/node-agent_service.yaml" ]]; then
    ln -sf node-agent_service.yaml "${SERVICES_OUTPUT}/specs/nodeagent_service.yaml"
    echo "  ✓ nodeagent_service.yaml → node-agent_service.yaml"
else
    echo "  ⚠ node-agent_service.yaml not found"
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
if [[ -f "build.sh" ]]; then
    # Build etcd with specific version
    echo "  → Building etcd ${ETCD_VERSION}..."
    bash build.sh --version "${ETCD_VERSION}" etcd

    # Build envoy with specific version
    echo "  → Building envoy ${ENVOY_VERSION}..."
    bash build.sh --version "${ENVOY_VERSION}" envoy

    # Build other infrastructure packages with version 0.0.1
    echo "  → Building other infrastructure packages (0.0.1)..."
    bash build.sh --version "0.0.1" gateway xds minio mc globular_cli

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
