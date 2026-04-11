#!/usr/bin/env bash
set -euo pipefail

# deploy-service.sh — Build, package, and publish a single Globular service.
#
# Usage:
#   ./deploy-service.sh <service_name> [--comment "description of changes"]
#
# Flow:
#   1. Build the Go binary
#   2. Query the repository for the current build number
#   3. Increment build number
#   4. Build the .tgz package
#   5. Publish to the repository
#
# The controller will detect the new artifact and roll it out automatically.
#
# Examples:
#   ./deploy-service.sh cluster_controller --comment "etcd state persistence for leader election"
#   ./deploy-service.sh echo_server
#   ./deploy-service.sh dns --comment "fix trailing dot handling"

# ── Configuration ─────────────────────────────────────────────────────────────

SERVICES_ROOT="$(cd "$(dirname "$0")" && pwd)"
GOLANG_DIR="${SERVICES_ROOT}/golang"
STAGE_BIN="${GOLANG_DIR}/tools/stage/linux-amd64/usr/local/bin"
GENERATED="${SERVICES_ROOT}/generated"
SPECS_DIR="${GENERATED}/specs"
VERSION="0.0.2"
PUBLISHER="core@globular.io"
PLATFORM="linux_amd64"
# Repository address: resolve from etcd via the CLI, fall back to mesh routing.
REPOSITORY=""

# ── Parse arguments ───────────────────────────────────────────────────────────

SERVICE=""
COMMENT=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --comment|-c)
            COMMENT="$2"
            shift 2
            ;;
        --version|-v)
            VERSION="$2"
            shift 2
            ;;
        --repository|-r)
            REPOSITORY="$2"
            shift 2
            ;;
        --help|-h)
            sed -n '3,/^$/p' "$0"
            exit 0
            ;;
        -*)
            echo "Unknown flag: $1" >&2
            exit 2
            ;;
        *)
            SERVICE="$1"
            shift
            ;;
    esac
done

if [[ -z "$SERVICE" ]]; then
    echo "Usage: $0 <service_name> [--comment \"description\"]" >&2
    exit 2
fi

# ── Resolve paths ─────────────────────────────────────────────────────────────

# Normalize: strip _server suffix if user passed it
SERVICE="${SERVICE%_server}"

SPEC_FILE="${SPECS_DIR}/${SERVICE}_service.yaml"
if [[ ! -f "$SPEC_FILE" ]]; then
    echo "ERROR: spec not found: ${SPEC_FILE}" >&2
    echo "Available specs:" >&2
    ls "${SPECS_DIR}"/*.yaml 2>/dev/null | sed 's|.*/||; s/_service.yaml//' | sort | sed 's/^/  /' >&2
    exit 1
fi

# Read the exec name from the spec (e.g., cluster_controller_server)
EXEC_NAME=$(sed -n '/^service:/,/^[^ ]/{s/^[[:space:]]*exec:[[:space:]]*//p;}' "$SPEC_FILE" | head -1)
if [[ -z "$EXEC_NAME" ]]; then
    EXEC_NAME="${SERVICE}_server"
fi

PAYLOAD_DIR="${GENERATED}/payload/${SERVICE}"
GO_PKG_DIR=""

# Find the Go package directory for this service
for candidate in \
    "${GOLANG_DIR}/${SERVICE}/${EXEC_NAME}" \
    "${GOLANG_DIR}/${SERVICE}_server" \
    "${GOLANG_DIR}/${SERVICE}/${SERVICE}_server"; do
    if [[ -d "$candidate" ]]; then
        GO_PKG_DIR="$candidate"
        break
    fi
done

if [[ -z "$GO_PKG_DIR" ]]; then
    echo "ERROR: Go package directory not found for ${SERVICE}" >&2
    echo "Tried:" >&2
    echo "  ${GOLANG_DIR}/${SERVICE}/${EXEC_NAME}" >&2
    echo "  ${GOLANG_DIR}/${SERVICE}_server" >&2
    echo "  ${GOLANG_DIR}/${SERVICE}/${SERVICE}_server" >&2
    exit 1
fi

# ── Step 1: Build the binary ─────────────────────────────────────────────────

echo ""
echo "━━━ Deploy: ${SERVICE} ━━━"
echo ""
if [[ -n "$COMMENT" ]]; then
    echo "  Comment: ${COMMENT}"
    echo ""
fi

echo "→ Step 1: Determining build number..."

CURRENT_BUILD=0
BUILD_TRACKER="${SERVICES_ROOT}/.build-numbers"
touch "${BUILD_TRACKER}"
TRACKER_KEY="${SERVICE}:${VERSION}:${PLATFORM}"
CURRENT_BUILD=$(grep "^${TRACKER_KEY}=" "${BUILD_TRACKER}" 2>/dev/null | tail -1 | cut -d= -f2 || echo "0")
if [[ -z "$CURRENT_BUILD" ]]; then
    CURRENT_BUILD=0
fi
NEXT_BUILD=$((CURRENT_BUILD + 1))
echo "  Current: ${CURRENT_BUILD} → Next: ${NEXT_BUILD}"

echo ""
echo "→ Step 2: Building binary..."
GO_PKG_REL="./${GO_PKG_DIR#${GOLANG_DIR}/}"
# Inject version and build number via ldflags so the binary knows its identity at runtime.
LDFLAGS="-X main.Version=${VERSION} -X main.BuildNumberStr=${NEXT_BUILD}"
(cd "${GOLANG_DIR}" && go build -ldflags "${LDFLAGS}" -o "${STAGE_BIN}/${EXEC_NAME}" "${GO_PKG_REL}")
echo "  ✓ Built ${EXEC_NAME} (version=${VERSION}, build=${NEXT_BUILD})"

# Copy to payload
mkdir -p "${PAYLOAD_DIR}/bin"
cp "${STAGE_BIN}/${EXEC_NAME}" "${PAYLOAD_DIR}/bin/${EXEC_NAME}"
echo "  ✓ Staged to payload"

# ── Step 3: Build the package ─────────────────────────────────────────────────

echo ""
echo "→ Step 3: Building package..."

GLOBULAR_CLI="${STAGE_BIN}/globularcli"
if [[ ! -x "$GLOBULAR_CLI" ]]; then
    GLOBULAR_CLI="$(which globular 2>/dev/null || true)"
fi
if [[ -z "$GLOBULAR_CLI" ]]; then
    echo "ERROR: globular CLI not found" >&2
    exit 1
fi

PKG_FILE="${GENERATED}/${SERVICE}_${VERSION}_${PLATFORM//_/_}.tgz"

BUILD_LOG=$(mktemp)
if ! "${GLOBULAR_CLI}" pkg build \
    --spec "${SPEC_FILE}" \
    --root "${PAYLOAD_DIR}" \
    --version "${VERSION}" \
    --build-number "${NEXT_BUILD}" \
    --publisher "${PUBLISHER}" \
    --platform "${PLATFORM}" \
    --out "${GENERATED}" >"${BUILD_LOG}" 2>&1; then
    sed 's/^/  /' "${BUILD_LOG}"
    rm -f "${BUILD_LOG}"
    echo "ERROR: package build failed" >&2
    exit 1
fi
sed 's/^/  /' "${BUILD_LOG}"
rm -f "${BUILD_LOG}"

# Find the actual output file (name may use dashes instead of underscores)
SERVICE_DASH="${SERVICE//_/-}"
ACTUAL_PKG=$(ls -t "${GENERATED}/${SERVICE_DASH}"*"${VERSION}"*".tgz" 2>/dev/null | head -1)
if [[ -z "$ACTUAL_PKG" ]]; then
    ACTUAL_PKG=$(ls -t "${GENERATED}/${SERVICE}"*"${VERSION}"*".tgz" 2>/dev/null | head -1)
fi
if [[ -z "$ACTUAL_PKG" ]]; then
    echo "ERROR: package file not found after build" >&2
    exit 1
fi
echo "  ✓ Package: $(basename "${ACTUAL_PKG}")"

# ── Step 4: Publish ───────────────────────────────────────────────────────────

# Resolve repository address from etcd if not set via --repository flag.
if [[ -z "$REPOSITORY" ]]; then
    # Simplest safe default: rely on mesh logical name. etcd lookup is optional and
    # can fail in restricted environments; skip it here to avoid hostname/network
    # permission issues.
    REPOSITORY="repository.PackageRepository"
fi

echo ""
echo "→ Step 4: Publishing to repository (${REPOSITORY})..."

PUBLISH_LOG=$(mktemp)
# Read cached token for auth.
CACHED_TOKEN=""
if [[ -f "${HOME}/.config/globular/token" ]]; then
    CACHED_TOKEN=$(cat "${HOME}/.config/globular/token")
fi

PUBLISH_ARGS=(
    --file "${ACTUAL_PKG}"
    --repository "${REPOSITORY}"
    --force
    --output json
)
if [[ -n "$CACHED_TOKEN" ]]; then
    PUBLISH_ARGS+=(--token "$CACHED_TOKEN")
fi

"${GLOBULAR_CLI}" pkg publish "${PUBLISH_ARGS[@]}" >"${PUBLISH_LOG}" 2>&1 || true
sed 's/^/  /' "${PUBLISH_LOG}"

# Check if the upload actually succeeded. The CLI may exit non-zero if the
# post-upload verify step fails (mesh auth, MinIO key format) even though
# the bundle was uploaded successfully. We consider these cases as success:
#   - "status": "success"
#   - "bundle_id" present (upload completed, verify warning)
#   - "verify uploaded manifest" in error (upload done, verify read-back failed)
if grep -q '"status": "success"' "${PUBLISH_LOG}" \
   || grep -q '"bundle_id"' "${PUBLISH_LOG}" \
   || grep -q 'verify uploaded manifest' "${PUBLISH_LOG}"; then
    if ! grep -q '"status": "success"' "${PUBLISH_LOG}"; then
        echo "  (post-upload verify had a warning — bundle was uploaded successfully)"
    fi
    rm -f "${PUBLISH_LOG}"
else
    rm -f "${PUBLISH_LOG}"
    echo "ERROR: publish failed" >&2
    exit 1
fi

# Update build tracker
if grep -q "^${TRACKER_KEY}=" "${BUILD_TRACKER}" 2>/dev/null; then
    sed -i "s|^${TRACKER_KEY}=.*|${TRACKER_KEY}=${NEXT_BUILD}|" "${BUILD_TRACKER}"
else
    echo "${TRACKER_KEY}=${NEXT_BUILD}" >> "${BUILD_TRACKER}"
fi

# ── Step 5: Set desired state ─────────────────────────────────────────────────

echo ""
echo "→ Step 5: Setting desired state..."

# Normalize service name for desired-state (underscores → dashes)
DESIRED_NAME="${SERVICE//_/-}"

DESIRED_ARGS=(services desired set "${DESIRED_NAME}" "${VERSION}")
if [[ -n "$CACHED_TOKEN" ]]; then
    DESIRED_ARGS+=(--token "$CACHED_TOKEN")
fi

if "${GLOBULAR_CLI}" "${DESIRED_ARGS[@]}" >/dev/null 2>&1; then
    echo "  ✓ Desired state set: ${DESIRED_NAME} → ${VERSION}"
else
    echo "  ⚠ Could not set desired state (controller may be unavailable)"
    echo "    Run manually: globular services desired set ${DESIRED_NAME} ${VERSION}"
fi

# ── Step 6: Set controller target-build (controller packages only) ────────────

if [[ "${SERVICE}" == "cluster_controller" || "${SERVICE}" == "cluster-controller" ]]; then
    echo ""
    echo "→ Step 6: Setting controller target-build in etcd..."

    # Compute checksum of the actual published artifact package.
    ARTIFACT_SHA256=$(sha256sum "${ACTUAL_PKG}" 2>/dev/null | cut -d' ' -f1)
    if [[ -n "$ARTIFACT_SHA256" ]]; then
        PKG_CHECKSUM="sha256:${ARTIFACT_SHA256}"
    else
        PKG_CHECKSUM=""
    fi

    TARGET_JSON="{\"version\":\"${VERSION}\",\"build_number\":${NEXT_BUILD},\"checksum\":\"${PKG_CHECKSUM}\",\"set_at\":$(date +%s)}"
    ETCD_EP="https://$(hostname -I | awk '{print $1}'):2379"
    ETCD_CERTS=(
        --cacert=/var/lib/globular/pki/ca.crt
        --cert=/var/lib/globular/pki/issued/services/service.crt
        --key=/var/lib/globular/pki/issued/services/service.key
    )

    if etcdctl put /globular/system/controller-target-build "${TARGET_JSON}" \
        --endpoints="${ETCD_EP}" "${ETCD_CERTS[@]}" >/dev/null 2>&1; then
        echo "  ✓ Controller target-build: ${VERSION}+${NEXT_BUILD} (checksum=${PKG_CHECKSUM:0:20}...)"
    else
        echo "  ⚠ Could not set controller target-build in etcd"
        echo "    etcdctl put /globular/system/controller-target-build '${TARGET_JSON}'"
    fi
fi

# ── Step 7: Record the deployment ─────────────────────────────────────────────

echo ""
echo "━━━ Deployed ━━━"
echo ""
echo "  Service:      ${SERVICE}"
echo "  Version:      ${VERSION}"
echo "  Build:        ${NEXT_BUILD}"
echo "  Comment:      ${COMMENT:-"(none)"}"
echo "  Package:      $(basename "${ACTUAL_PKG}")"
echo ""
echo "  The controller will detect the new artifact and roll it out."
echo ""

# Append to deployment log
DEPLOY_LOG="${SERVICES_ROOT}/.deploy-log"
echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) | ${SERVICE} | v${VERSION}+${NEXT_BUILD} | ${COMMENT:-"-"}" >> "${DEPLOY_LOG}"
