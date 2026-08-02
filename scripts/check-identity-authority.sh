#!/usr/bin/env bash
# check-identity-authority.sh — CI gate for the single-authority package
# identity model (docs/design/package-identity-single-authority.md).
#
# Asserts, at the source level, that the release pipeline cannot regress into
# local identity minting or platform-version stamping:
#   1. No local build_id minting (uuid) in release scripts.
#   2. No local build_number minting (date +%s) in release scripts.
#   3. No platform-version override of service package versions
#      (-X main.Version=${VERSION}) in the release build.
#   4. Every available shipped package has a committed, well-formed, non-dev
#      version (zz_version_generated.go contract). Cross-repo packages are
#      explicitly reported as not evaluated when Globular is not checked out.
#   5. registry.yaml carries no retired 'version_source: platform' class.
#
# Run: bash scripts/check-identity-authority.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICES_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
PACKAGES_ROOT="${SERVICES_ROOT}/../packages"
FAIL=0

err() { echo "IDENTITY-GATE FAIL: $*" >&2; FAIL=1; }
ok()  { echo "  ✓ $*"; }

RELEASE_SCRIPTS=(
  "${SERVICES_ROOT}/scripts/build-release.sh"
  "${SERVICES_ROOT}/scripts/regenerate-release-inputs.sh"
  "${SERVICES_ROOT}/.github/workflows/release.yml"
)

# 1. No local build_id minting.
for f in "${RELEASE_SCRIPTS[@]}"; do
  if grep -nE 'uuid\.uuid[147]|_uuid\.uuid4|uuidgen|per_pkg_build_id' "$f" >/dev/null; then
    err "$(basename "$f") mints UUIDs — build_id is repository-admission identity only"
  fi
done
ok "no local build_id minting in release scripts"

# 2. No local build_number minting.
for f in "${RELEASE_SCRIPTS[@]}"; do
  if grep -nE 'BUILD_NUMBER=.*date \+%s|build_number.*date \+%s|BUILD_NUMBER=.*github\.run_number' "$f" >/dev/null; then
    err "$(basename "$f") mints timestamp build_numbers — build_number is repository-admission identity only"
  fi
done
ok "no local build_number minting in release scripts"

# 3. No platform-version override of package versions in any release path.
for f in "${RELEASE_SCRIPTS[@]}"; do
  if grep -nE -- '-X main\.Version=\$\{VERSION\}|pkg_version[[:space:]]*=[[:space:]]*version|gen-version\.sh "\$\{VERSION\}"' "$f" >/dev/null; then
    err "$(basename "$f") stamps the platform version into package identity — package versions come from committed source authority"
  fi
done
if (( ! FAIL )); then
  ok "release paths preserve committed per-package versions"
fi

# 4. Committed per-package versions valid (zz contract).
if ! bash "${SCRIPT_DIR}/gen-package-versions-from-source.sh" --check; then
  err "committed zz_version_generated.go contract violated"
else
  ok "available committed package versions satisfy the source-authority contract"
fi

# 5. Retired version_source class.
if [[ ! -f "${PACKAGES_ROOT}/registry.yaml" ]]; then
  err "packages registry unavailable at ${PACKAGES_ROOT}/registry.yaml — checkout globulario/packages beside services"
elif grep -nE '^\s*version_source: platform$' "${PACKAGES_ROOT}/registry.yaml" >/dev/null; then
  err "registry.yaml still classifies packages as version_source: platform — retired; use 'code' or 'self'"
else
  ok "registry.yaml carries no retired 'platform' version_source"
fi

if (( FAIL )); then
  echo "" >&2
  echo "See docs/design/package-identity-single-authority.md for the authority model." >&2
  exit 1
fi
echo "identity gate: all checks passed"
