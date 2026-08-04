#!/usr/bin/env bash
# gen-package-versions-from-source.sh — materialize per-package versions from
# committed source authority (zz_version_generated.go) into an overrides file.
#
# AUTHORITY MODEL (docs/design/package-identity-single-authority.md):
#   The committed zz_version_generated.go in each service main package is the
#   per-package version authority. This script READS those files (it never
#   writes them — gen-version.sh is the single writer) and emits
#   golang/build/package-versions.txt in the pkggen/gen-version overrides
#   format (`name=version`, registry hyphenated names).
#
#   Consumers: scripts/build-release.sh, scripts/regenerate-release-inputs.sh,
#   CI identity gate.
#
# Usage:
#   bash scripts/gen-package-versions-from-source.sh [--out FILE] [--check]
#
#   --out FILE   write the overrides file (default golang/build/package-versions.txt)
#   --check      validate only: every available shipped package has a committed,
#                well-formed, non-dev version; exit non-zero otherwise. If the
#                sibling Globular checkout is absent, xds/gateway are reported
#                as not evaluated. A present checkout is always validated strictly.
#
# Materialization (without --check) requires the sibling Globular repo because
# xds and gateway versions are part of the generated release input.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICES_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
GOLANG_ROOT="${SERVICES_ROOT}/golang"
GLOBULAR_ROOT="${SERVICES_ROOT}/../Globular"
OUT_FILE="${GOLANG_ROOT}/build/package-versions.txt"
CHECK_ONLY=0
SKIPPED_EXTERNAL=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --out) OUT_FILE="$2"; shift 2 ;;
    --check) CHECK_ONLY=1; shift ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done

die() { echo "ERROR: $*" >&2; exit 1; }

# Extract `var Version = "x.y.z"` from a zz_version_generated.go file.
zz_version() {
  local file="$1"
  [[ -f "${file}" ]] || return 1
  sed -n 's/^var Version = "\(.*\)"$/\1/p' "${file}" | head -1
}

# Registry package name from a golang/<dir> service directory name.
pkg_name_for_dir() {
  local dir="$1"
  case "${dir}" in
    globularcli) echo "globular-cli" ;;
    *) echo "${dir//_/-}" ;;
  esac
}

validate_version() {
  local name="$1" ver="$2"
  [[ -n "${ver}" ]] || die "${name}: empty version in zz_version_generated.go"
  # Reject the WHOLE 0.0.0-* placeholder family, not just 0.0.0-dev.
  # 0.0.0-ci was published as the committed version of all 33 packages by
  # commit 52342307: CI ran `gen-version.sh 0.0.0-ci` against the tracked
  # checkout and a `git add -A` captured it. A check naming one placeholder
  # cannot stop the next one, so the shape is rejected rather than the value.
  case "${ver}" in
    0.0.0-*|0.0.0)
      die "${name}: version '${ver}' is a placeholder — committed package versions must be real allocated versions (see docs/design/package-identity-single-authority.md)" ;;
  esac
  [[ "${ver}" =~ ^[0-9]+(\.[0-9]+){2}([+-][0-9A-Za-z._-]+)?$ ]] || die "${name}: invalid version '${ver}'"
}

declare -A VERSIONS=()

# 1) services.list targets (includes mcp; excludes xds/gateway which live in Globular).
while IFS='|' read -r target _; do
  target="${target%%#*}"; target="${target// /}"
  [[ -z "${target}" ]] && continue
  rel="${target#./}"                      # e.g. authentication/authentication_server, mcp
  svc_dir="${rel%%/*}"                    # e.g. authentication, mcp
  name="$(pkg_name_for_dir "${svc_dir}")"
  zz="${GOLANG_ROOT}/${rel}/zz_version_generated.go"
  ver="$(zz_version "${zz}" || true)"
  [[ -n "${ver}" ]] || die "${name}: missing ${zz} — run golang/build/gen-version.sh and commit the result"
  validate_version "${name}" "${ver}"
  VERSIONS["${name}"]="${ver}"
done < "${GOLANG_ROOT}/build/services.list"

# 2) globular-cli (not in services.list under that name in all branches — ensure present).
if [[ -z "${VERSIONS[globular-cli]+x}" ]]; then
  ver="$(zz_version "${GOLANG_ROOT}/globularcli/zz_version_generated.go" || true)"
  [[ -n "${ver}" ]] || die "globular-cli: missing golang/globularcli/zz_version_generated.go"
  validate_version "globular-cli" "${ver}"
  VERSIONS["globular-cli"]="${ver}"
fi

# 3) xds + gateway from the sibling Globular repo.
# A source-only CI checkout can validate this repository without also cloning
# Globular. That absence is reported, not confused with a missing authority file.
# Once the sibling root exists, however, both files are mandatory. Release-input
# materialization is never allowed to omit these packages.
if [[ ! -d "${GLOBULAR_ROOT}" ]]; then
  if (( CHECK_ONLY )); then
    SKIPPED_EXTERNAL=2
    echo "gen-package-versions: NOTICE: sibling Globular checkout absent at ${GLOBULAR_ROOT}; xds/gateway version authority not evaluated"
  else
    die "Globular repo absent at ${GLOBULAR_ROOT} — required to materialize xds/gateway versions"
  fi
else
  for pair in "xds:cmd/xds" "gateway:cmd/gateway"; do
    name="${pair%%:*}"; sub="${pair#*:}"
    zz="${GLOBULAR_ROOT}/${sub}/zz_version_generated.go"
    ver="$(zz_version "${zz}" || true)"
    [[ -n "${ver}" ]] || die "${name}: missing ${zz} — the present Globular repo must carry a committed version file for ${name}"
    validate_version "${name}" "${ver}"
    VERSIONS["${name}"]="${ver}"
  done
fi

if (( CHECK_ONLY )); then
  if (( SKIPPED_EXTERNAL > 0 )); then
    echo "gen-package-versions: ${#VERSIONS[@]} packages valid; ${SKIPPED_EXTERNAL} cross-repo packages not evaluated"
  else
    echo "gen-package-versions: ${#VERSIONS[@]} packages carry valid committed versions"
  fi
  exit 0
fi

{
  echo "# Code generated by scripts/gen-package-versions-from-source.sh — DO NOT EDIT."
  echo "# Per-package versions materialized from committed zz_version_generated.go files."
  echo "# Authority: docs/design/package-identity-single-authority.md"
  for name in $(printf '%s\n' "${!VERSIONS[@]}" | sort); do
    echo "${name}=${VERSIONS[${name}]}"
  done
} > "${OUT_FILE}"
echo "gen-package-versions: wrote ${#VERSIONS[@]} package versions to ${OUT_FILE}"
