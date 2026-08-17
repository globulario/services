#!/usr/bin/env bash
set -euo pipefail

# deploy-service.sh — Build, package, and publish a single Globular service.
#
# Usage:
#   ./deploy-service.sh <service_name> [--comment "description of changes"] [--force]
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
# Version comes from the committed source authority
# (golang/build/package-versions.txt, materialized from each service's
# zz_version_generated.go — see docs/design/package-identity-single-authority.md),
# resolved after the package name is known. It used to default to a hardcoded
# "0.0.2", which silently produced a package older than anything deployed: the
# cluster ran ai-memory 1.2.312 while this script built and tried to publish
# 0.0.2, i.e. a downgrade, which node.package_downgrade_requires_force_flag
# forbids. --version still overrides for the hotfix lane.
VERSION=""
PUBLISHER="core@globular.io"
PLATFORM="linux_amd64"
# Repository address: resolve from etcd via the CLI, fall back to mesh routing.
REPOSITORY=""

# ── Parse arguments ───────────────────────────────────────────────────────────

SERVICE=""
COMMENT=""
FORCE_PUBLISH=0

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
        --force)
            # Deliberate replacement of an already-published artifact. See the
            # note above PUBLISH_ARGS for why this is not the default.
            FORCE_PUBLISH=1
            shift
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

# Resolve the version from the committed source authority unless --version was
# given. Registry names are hyphenated (ai-memory) while service dirs are
# underscored (ai_memory). Fail loudly rather than guessing: publishing a version
# nobody declared is how a downgrade reaches the cluster.
if [[ -z "$VERSION" ]]; then
    PKG_VERSIONS="${GOLANG_DIR}/build/package-versions.txt"
    REG_NAME="${SERVICE//_/-}"
    if [[ -f "$PKG_VERSIONS" ]]; then
        VERSION="$(sed -n "s/^${REG_NAME}=//p" "$PKG_VERSIONS" | head -1)"
    fi
    if [[ -z "$VERSION" ]]; then
        echo "ERROR: no version for '${REG_NAME}' in ${PKG_VERSIONS}." >&2
        echo "       Regenerate it (scripts/gen-package-versions-from-source.sh) or pass --version." >&2
        exit 1
    fi
    echo "  Version: ${VERSION} (from committed source authority)"
    echo ""
fi

echo "→ Step 1: Determining build number..."

# The CLI is needed here now (pre-flight resolve), not just at Step 3.
GLOBULAR_CLI="${STAGE_BIN}/globularcli"
if [[ ! -x "$GLOBULAR_CLI" ]]; then
    GLOBULAR_CLI="$(which globular 2>/dev/null || true)"
fi
if [[ -z "$GLOBULAR_CLI" ]]; then
    echo "ERROR: globular CLI not found" >&2
    exit 1
fi
CACHED_TOKEN=""
if [[ -f "${HOME}/.config/globular/token" ]]; then
    CACHED_TOKEN=$(cat "${HOME}/.config/globular/token")
fi

# PRE-FLIGHT: refuse to mint a second build of a version that is already
# published.
#
# The repository resolver requires a package reference to resolve to exactly
# ONE artifact. Publishing build 2 of a version that already has build 1 gives
# that version two build_ids, and the resolver then reports
# repository.identity.version_resolution_ambiguous and the rollout does not
# converge. This script's "increment the build number" behaviour did exactly
# that on every re-deploy of an unchanged version — which is why deploys
# succeeded when the version had been bumped and stalled when it had not.
#
# Observed 2026-08-16: cluster-controller 1.2.317 ended up with build 1 and
# build 2, the second of them blobless, and the rollout stalled.
#
# The fix for "I changed the code" is a new VERSION, which is the identity the
# rest of the platform converges on. Replacing a published artifact in place is
# a deliberate act and needs --force.
RESOLVE_CHECK=$(mktemp)
if "${GLOBULAR_CLI}" repository resolve --name "${SERVICE//_/-}" --version "${VERSION}" \
        --platform "${PLATFORM}" --json \
        ${CACHED_TOKEN:+--token "$CACHED_TOKEN"} >"${RESOLVE_CHECK}" 2>&1; then
    EXISTING_ID=$(python3 -c "
import json,sys
raw=open(sys.argv[1],encoding='utf-8',errors='replace').read()
s,e=raw.find('{'),raw.rfind('}')
print(json.loads(raw[s:e+1]).get('build_id','') if s>=0 and e>s else '')
" "${RESOLVE_CHECK}" 2>/dev/null || echo "")
    if [[ -n "$EXISTING_ID" && "${FORCE_PUBLISH}" != "1" ]]; then
        echo "" >&2
        echo "ERROR: ${SERVICE//_/-} ${VERSION} is already published (build_id=${EXISTING_ID})." >&2
        echo "" >&2
        echo "  Publishing another build of the same version gives it two build_ids," >&2
        echo "  which makes resolution ambiguous and stalls the rollout." >&2
        echo "" >&2
        echo "  If the code changed, bump the version:" >&2
        echo "    scripts/release/... (or edit the service's zz_version_generated.go)," >&2
        echo "    then regenerate golang/build/package-versions.txt" >&2
        echo "" >&2
        echo "  To deliberately replace the published artifact instead:" >&2
        echo "    $0 ${SERVICE} --force" >&2
        rm -f "${RESOLVE_CHECK}"
        exit 1
    fi
elif [[ "${FORCE_PUBLISH}" != "1" ]]; then
    # Not resolvable is the normal case for a new version. Anything else (the
    # repository being unreachable, an ambiguous resolve) is reported so the
    # operator is not publishing blind, but does not block a first publish.
    if grep -qi 'ambiguous' "${RESOLVE_CHECK}"; then
        echo "ERROR: ${SERVICE//_/-} ${VERSION} already resolves ambiguously in the repository." >&2
        echo "       Resolve the duplicate builds before publishing another." >&2
        sed 's/^/    /' "${RESOLVE_CHECK}" >&2
        rm -f "${RESOLVE_CHECK}"
        exit 1
    fi
fi
rm -f "${RESOLVE_CHECK}"

# .build-numbers is a LOCAL cache, not the authority — it lives outside git, so
# a fresh clone or a second machine starts from 0. It is only used to pick the
# next number; the repository decides whether that identity is acceptable, and
# with --force no longer passed unconditionally, a collision now fails loudly
# instead of silently replacing a published artifact.
CURRENT_BUILD=0
BUILD_TRACKER="${SERVICES_ROOT}/.build-numbers"
touch "${BUILD_TRACKER}"
TRACKER_KEY="${SERVICE}:${VERSION}:${PLATFORM}"
CURRENT_BUILD=$(grep "^${TRACKER_KEY}=" "${BUILD_TRACKER}" 2>/dev/null | tail -1 | cut -d= -f2 || echo "0")
if [[ -z "$CURRENT_BUILD" ]]; then
    CURRENT_BUILD=0
fi
NEXT_BUILD=$((CURRENT_BUILD + 1))
echo "  Current: ${CURRENT_BUILD} → Next: ${NEXT_BUILD} (local cache; repository is authority)"

echo ""
echo "→ Step 2: Building binary..."
GO_PKG_REL="./${GO_PKG_DIR#${GOLANG_DIR}/}"
# Inject version and build number via ldflags so the binary knows its identity at runtime.
# -s -w strips the symbol table and DWARF; -trimpath removes local paths. The
# repository refuses a release-channel artifact carrying debug sections
# ("release artifact carries debug section .debug_aranges"), so an unstripped
# build here fails at publish after doing all the work. scripts/build-release.sh
# has always built this way; this lane had not.
LDFLAGS="-X main.Version=${VERSION} -X main.BuildNumberStr=${NEXT_BUILD} -s -w"
(cd "${GOLANG_DIR}" && go build -trimpath -ldflags "${LDFLAGS}" -o "${STAGE_BIN}/${EXEC_NAME}" "${GO_PKG_REL}")
echo "  ✓ Built ${EXEC_NAME} (version=${VERSION}, build=${NEXT_BUILD})"

# Copy to payload
mkdir -p "${PAYLOAD_DIR}/bin"
cp "${STAGE_BIN}/${EXEC_NAME}" "${PAYLOAD_DIR}/bin/${EXEC_NAME}"
echo "  ✓ Staged to payload"

# ── Step 3: Build the package ─────────────────────────────────────────────────

echo ""
echo "→ Step 3: Building package..."

PKG_FILE"${GENERATED}/${SERVICE}_${VERSION}_${PLATFORM//_/_}.tgz"

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
# --force is NOT passed by default. In the CLI it means: if an artifact with
# this identity already exists with different content, DeleteArtifact and then
# re-upload. That is an override of
# invariant.repository.artifact.content_immutable_after_publish, and it is a
# delete-then-write with no rollback — if the delete lands and the re-upload
# fails, the artifact is destroyed or left as a manifest row with no retrievable
# blob (repository.identity.missing_blob_for_published_manifest).
#
# This script used to pass it on every publish, so every routine deploy ran that
# window. Without it, republishing an identity that already exists fails with
# AlreadyExists — which is the correct outcome: it surfaces the build-number
# collision instead of silently overwriting a published artifact. Pass --force
# to this script only when deliberately replacing one.
PUBLISH_ARGS=(
    --file "${ACTUAL_PKG}"
    --repository "${REPOSITORY}"
    --output json
)
if [[ "${FORCE_PUBLISH}" == "1" ]]; then
    echo "  ⚠ --force: an existing artifact with this identity will be deleted and re-uploaded"
    PUBLISH_ARGS+=(--force)
fi
if [[ -n "$CACHED_TOKEN" ]]; then
    PUBLISH_ARGS+=(--token "$CACHED_TOKEN")
fi

PUBLISH_RC=0
"${GLOBULAR_CLI}" pkg publish "${PUBLISH_ARGS[@]}" >"${PUBLISH_LOG}" 2>&1 || PUBLISH_RC=$?
sed 's/^/  /' "${PUBLISH_LOG}"

# The publish gate used to be three greps over the CLI's log text, with the
# exit code discarded by `|| true`. Two of the three patterns matched FAILURE
# output: '"bundle_id"' appears in error payloads too, and 'verify uploaded
# manifest' only ever appears in the message "failed to verify uploaded
# manifest". So a publish whose read-back verification failed was recorded as
# a successful deploy, the script continued to set desired state, and the
# cluster was pointed at an artifact that might have no retrievable blob.
#
# That is how cluster-controller@1.2.317 build 2 reached the manifest without a
# blob on 2026-08-16, which then produced
# repository.identity.version_resolution_ambiguous (one version, two build_ids)
# and left nodes advertising a PUBLISHED artifact they could not serve.
#
# A publish is now successful only if the CLI exited zero AND the JSON says so.
# Nothing is inferred from prose. Failure is fatal here rather than downstream.
if [[ ${PUBLISH_RC} -ne 0 ]]; then
    rm -f "${PUBLISH_LOG}"
    echo "ERROR: publish failed (globular pkg publish exited ${PUBLISH_RC})" >&2
    exit 1
fi
if ! PUBLISH_STATUS=$(python3 - "${PUBLISH_LOG}" <<'PY'
import json, sys
raw = open(sys.argv[1], encoding="utf-8", errors="replace").read()
# The CLI prints a JSON object; tolerate leading/trailing noise by taking the
# outermost braces. A payload we cannot parse is NOT a success.
start, end = raw.find("{"), raw.rfind("}")
if start < 0 or end <= start:
    sys.exit("publish output was not JSON — refusing to treat as success")
try:
    doc = json.loads(raw[start:end + 1])
except Exception as exc:
    sys.exit("publish output was not parseable JSON (%s)" % exc)
status = str(doc.get("status", "")).lower()
if status != "success":
    sys.exit("publish status=%r (want 'success'); error=%r" %
             (status, doc.get("error") or doc.get("message") or "<none>"))
print(doc.get("build_id") or "")
PY
    ); then
    rm -f "${PUBLISH_LOG}"
    echo "ERROR: publish did not report success" >&2
    exit 1
fi
rm -f "${PUBLISH_LOG}"
PUBLISHED_BUILD_ID="${PUBLISH_STATUS}"
echo "  ✓ Publish reported success${PUBLISHED_BUILD_ID:+ (build_id=${PUBLISHED_BUILD_ID})}"

# ── Step 4b: Verify the postcondition, do not assume it ───────────────────────
#
# "Published" is a claim by the writer. The property the cluster depends on is
# that the artifact RESOLVES to exactly one build_id and that its blob can
# actually be served. Ask the repository, and refuse to advance desired state
# if it cannot answer cleanly — a resolver that is ambiguous here becomes a
# stalled rollout later, which is far more expensive to diagnose.
echo ""
echo "→ Step 4b: Verifying the artifact resolves to exactly one build..."

RESOLVE_ARGS=(repository resolve --name "${SERVICE//_/-}" --version "${VERSION}"
              --platform "${PLATFORM}" --json)
if [[ -n "$CACHED_TOKEN" ]]; then
    RESOLVE_ARGS+=(--token "$CACHED_TOKEN")
fi

RESOLVE_LOG=$(mktemp)
if "${GLOBULAR_CLI}" "${RESOLVE_ARGS[@]}" >"${RESOLVE_LOG}" 2>&1; then
    RESOLVED_ID=$(python3 -c "
import json,sys
raw=open(sys.argv[1],encoding='utf-8',errors='replace').read()
s,e=raw.find('{'),raw.rfind('}')
print(json.loads(raw[s:e+1]).get('build_id','') if s>=0 and e>s else '')
" "${RESOLVE_LOG}" 2>/dev/null || echo "")
    if [[ -n "$RESOLVED_ID" ]]; then
        echo "  ✓ Resolves to a single build_id: ${RESOLVED_ID}"
        if [[ -n "$PUBLISHED_BUILD_ID" && "$RESOLVED_ID" != "$PUBLISHED_BUILD_ID" ]]; then
            echo "ERROR: the repository resolves ${SERVICE} ${VERSION} to ${RESOLVED_ID}," >&2
            echo "       but this deploy published ${PUBLISHED_BUILD_ID}. Setting desired" >&2
            echo "       state now would roll out an artifact this run did not build." >&2
            rm -f "${RESOLVE_LOG}"
            exit 1
        fi
    else
        echo "  ⚠ Resolver returned no build_id; see output above"
        sed 's/^/    /' "${RESOLVE_LOG}"
    fi
else
    echo "ERROR: ${SERVICE} ${VERSION} does not resolve to exactly one artifact." >&2
    echo "       This is repository.identity.version_resolution_ambiguous — the" >&2
    echo "       rollout would be non-deterministic. Not setting desired state." >&2
    sed 's/^/    /' "${RESOLVE_LOG}" >&2
    rm -f "${RESOLVE_LOG}"
    exit 1
fi
rm -f "${RESOLVE_LOG}"

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

# Setting desired state IS the deploy. Failing it and then printing "Deployed"
# reports a rollout that was never requested — the operator walks away, nothing
# converges, and the gap is found much later by a doctor finding. Fail loudly.
DESIRED_LOG=$(mktemp)
if "${GLOBULAR_CLI}" "${DESIRED_ARGS[@]}" >"${DESIRED_LOG}" 2>&1; then
    echo "  ✓ Desired state set: ${DESIRED_NAME} → ${VERSION}"
    rm -f "${DESIRED_LOG}"
else
    echo "ERROR: could not set desired state for ${DESIRED_NAME} → ${VERSION}." >&2
    echo "       The artifact is published but nothing will roll it out, so this" >&2
    echo "       deploy is incomplete. Fix the controller, then run:" >&2
    echo "         globular services desired set ${DESIRED_NAME} ${VERSION}" >&2
    sed 's/^/    /' "${DESIRED_LOG}" >&2
    rm -f "${DESIRED_LOG}"
    exit 1
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
    # `hostname -I | awk '{print $1}'` returns whichever address the kernel lists
    # first, which on a control-plane node can be the keepalived VIP — the
    # floating address that may currently live on a different machine
    # (netutil.identity_getter_must_express_vip_ambiguity). Writing the
    # controller target-build through the VIP means the write lands wherever the
    # VIP happens to point, not necessarily this node's etcd.
    #
    # etcdctl talking to the etcd on this same host is the one case where
    # loopback is the correct address rather than a shortcut, so use it.
    ETCD_EP="https://127.0.0.1:2379"
    ETCD_CERTS=(
        --cacert=/var/lib/globular/pki/ca.crt
        --cert=/var/lib/globular/pki/issued/services/service.crt
        --key=/var/lib/globular/pki/issued/services/service.key
    )

    if etcdctl put /globular/system/controller-target-build "${TARGET_JSON}" \
        --endpoints="${ETCD_EP}" "${ETCD_CERTS[@]}" >/dev/null 2>&1; then
        echo "  ✓ Controller target-build: ${VERSION}+${NEXT_BUILD} (checksum=${PKG_CHECKSUM:0:20}...)"
    else
        # Desired state already points at this version. If target-build is not
        # recorded, the controller's self-upgrade path lacks the build identity
        # it converges on, and the deploy is half-applied — exactly the state
        # that looks finished from the console and stalls in the cluster.
        echo "ERROR: could not set controller target-build in etcd." >&2
        echo "       Desired state is already set, so the deploy is half-applied." >&2
        echo "       Complete it with:" >&2
        echo "         etcdctl put /globular/system/controller-target-build '${TARGET_JSON}' \\" >&2
        echo "           --endpoints=${ETCD_EP} --cacert=/var/lib/globular/pki/ca.crt \\" >&2
        echo "           --cert=/var/lib/globular/pki/issued/services/service.crt \\" >&2
        echo "           --key=/var/lib/globular/pki/issued/services/service.key" >&2
        exit 1
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
