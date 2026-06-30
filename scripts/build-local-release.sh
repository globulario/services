#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICES_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
PACKAGES_ROOT="${SERVICES_ROOT}/../packages"
EXTERNAL_MANIFEST="${PACKAGES_ROOT}/sources/external-artifacts.json"
EXTERNAL_BUILDER="${PACKAGES_ROOT}/scripts/build-external-packages.py"
RELEASE_BUILDER="${SERVICES_ROOT}/scripts/build-release.sh"
VERSION=""
FULL_REGENERATE=1
BUILD_EXTERNAL=1

usage() {
  cat <<'EOF'
Usage:
  bash scripts/build-local-release.sh --version X.Y.Z [--no-full-regenerate] [--skip-external]

Builds a full local release into services/dist from current source trees only.
No previous release bundle is used.

Phases:
  1. Rebuild external/self-versioned package tgz files into ../packages/dist
  2. Run services/scripts/build-release.sh to assemble services/dist

Outputs:
  dist/globular-<version>-linux-amd64/
  dist/globular-<version>-linux-amd64.tar.gz
  dist/globular-<version>-linux-amd64.tar.gz.sha256
EOF
}

die() { echo "ERROR: $*" >&2; exit 1; }
info() { echo "  -> $*"; }
ok() { echo "  OK $*"; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)
      [[ $# -ge 2 ]] || die "--version requires a value"
      VERSION="${2#v}"
      shift 2
      ;;
    --no-full-regenerate)
      FULL_REGENERATE=0
      shift
      ;;
    --skip-external)
      BUILD_EXTERNAL=0
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "unknown argument: $1"
      ;;
  esac
done

[[ -n "${VERSION}" ]] || die "--version is required"
[[ "${VERSION}" =~ ^[0-9]+(\.[0-9]+){2}([.-][0-9A-Za-z._-]+)?$ ]] || die "invalid version '${VERSION}'"
[[ -d "${PACKAGES_ROOT}" ]] || die "packages repo not found at ${PACKAGES_ROOT}"
[[ -f "${EXTERNAL_MANIFEST}" ]] || die "external artifact manifest not found at ${EXTERNAL_MANIFEST}"
[[ -f "${RELEASE_BUILDER}" ]] || die "release builder not found at ${RELEASE_BUILDER}"
[[ -f "${EXTERNAL_BUILDER}" ]] || die "external package builder not found at ${EXTERNAL_BUILDER}"
command -v python3 >/dev/null 2>&1 || die "python3 is required"
command -v bash >/dev/null 2>&1 || die "bash is required"

if (( BUILD_EXTERNAL )); then
  info "Cleaning prior external package artifacts from ${PACKAGES_ROOT}/dist"
  python3 - "${PACKAGES_ROOT}" "${EXTERNAL_MANIFEST}" <<'PYEOF'
import json
import sys
from pathlib import Path

repo_root = Path(sys.argv[1])
manifest_path = Path(sys.argv[2])
dist_dir = repo_root / 'dist'
dist_dir.mkdir(parents=True, exist_ok=True)
with manifest_path.open('r', encoding='utf-8') as f:
    manifest = json.load(f)
for name in sorted((manifest.get('packages') or {}).keys()):
    for stale in dist_dir.glob(f'{name}_*_linux_amd64.tgz'):
        stale.unlink()
PYEOF

  info "Building external/self-versioned package artifacts into ${PACKAGES_ROOT}/dist"
  python3 "${EXTERNAL_BUILDER}"     --repo-root "${PACKAGES_ROOT}"     --manifest "sources/external-artifacts.json"     --out "dist"
  ok "external package artifacts rebuilt"
fi

release_args=("${VERSION}")
if (( FULL_REGENERATE )); then
  release_args+=(--full-regenerate)
fi

info "Assembling local release into ${SERVICES_ROOT}/dist"
bash "${RELEASE_BUILDER}" "${release_args[@]}"

RELEASE_NAME="globular-${VERSION}-linux-amd64"
RELEASE_DIR="${SERVICES_ROOT}/dist/${RELEASE_NAME}"
RELEASE_TGZ="${SERVICES_ROOT}/dist/${RELEASE_NAME}.tar.gz"
RELEASE_SHA="${SERVICES_ROOT}/dist/${RELEASE_NAME}.tar.gz.sha256"

[[ -d "${RELEASE_DIR}" ]] || die "release directory missing after build: ${RELEASE_DIR}"
[[ -f "${RELEASE_TGZ}" ]] || die "release tarball missing after build: ${RELEASE_TGZ}"
[[ -f "${RELEASE_SHA}" ]] || die "release checksum missing after build: ${RELEASE_SHA}"

ok "local release ready"
echo "  dir: ${RELEASE_DIR}"
echo "  tgz: ${RELEASE_TGZ}"
echo "  sha: ${RELEASE_SHA}"
