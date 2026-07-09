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
#   4. Every shipped package has a committed, well-formed, non-dev version
#      (zz_version_generated.go contract).
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
)

# 1. No local build_id minting.
for f in "${RELEASE_SCRIPTS[@]}"; do
  if grep -nE 'uuid\.uuid4|uuid\.uuid1|uuidgen' "$f" >/dev/null; then
    err "$(basename "$f") mints UUIDs — build_id is repository-admission identity only"
  fi
done
ok "no local build_id minting in release scripts"

# 2. No local build_number minting.
for f in "${RELEASE_SCRIPTS[@]}"; do
  if grep -nE 'BUILD_NUMBER=.*date \+%s|build_number.*date \+%s' "$f" >/dev/null; then
    err "$(basename "$f") mints timestamp build_numbers — build_number is repository-admission identity only"
  fi
done
ok "no local build_number minting in release scripts"

# 3. No platform-version override of package versions in the release build.
if grep -nE -- '-X main\.Version=\$\{VERSION\}' "${SERVICES_ROOT}/scripts/build-release.sh" >/dev/null; then
  err "build-release.sh injects the PLATFORM version into binaries — service versions come from committed zz files"
fi
ok "no platform-version ldflags override in build-release.sh"

# 4. Committed per-package versions valid (zz contract).
if ! bash "${SCRIPT_DIR}/gen-package-versions-from-source.sh" --check; then
  err "committed zz_version_generated.go contract violated"
fi

# 5. Retired version_source class.
if [[ -f "${PACKAGES_ROOT}/registry.yaml" ]] && grep -nE '^\s*version_source: platform$' "${PACKAGES_ROOT}/registry.yaml" >/dev/null; then
  err "registry.yaml still classifies packages as version_source: platform — retired; use 'code' or 'self'"
fi
ok "registry.yaml carries no retired 'platform' version_source"

if (( FAIL )); then
  echo "" >&2
  echo "See docs/design/package-identity-single-authority.md for the authority model." >&2
  exit 1
fi
echo "identity gate: all checks passed"
