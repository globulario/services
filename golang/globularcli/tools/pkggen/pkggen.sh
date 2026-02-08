#!/usr/bin/env bash
set -euo pipefail

# Defaults
BIN_DIR="/home/dave/Documents/github.com/globulario/services/golang/tools/stage/linux-amd64/usr/local/bin"
GEN_ROOT="$(pwd)/generated"
OUT_DIR="$(pwd)/generated/packages"
VERSION="0.0.1"
PUBLISHER="core@globular.io"
PLATFORM="$(go env GOOS)_$(go env GOARCH)"
GLOBULAR_BIN="${GLOBULAR_BIN:-globular}"

usage() {
  cat <<EOF
Usage: $0 [options]

Builds one package per *_server binary by assembling a per-service payload root and invoking:
  globular pkg build --spec <spec> --root <payload-root> --version <ver> --out <out>

Required:
  --version <ver>

Common options:
  --globular <path>     Path to globular CLI binary (default: env GLOBULAR_BIN or 'globular')
  --bin-dir <path>      Directory containing *_server binaries
  --gen-root <path>     Directory containing generated/specs and generated/config
  --out <path>          Output packages directory
  --publisher <id>      Publisher (default: core@globular.io)
  --platform <goos_goarch> Platform (default: current go env)

Example:
  $0 --globular ./globularcli \\
     --bin-dir /path/to/stage/bin \\
     --gen-root ./generated \\
     --out ./generated/packages \\
     --version 0.0.1
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

while [[ $# -gt 0 ]]; do
  case "$1" in
    --globular) GLOBULAR_BIN="$2"; shift 2 ;;
    --bin-dir) BIN_DIR="$2"; shift 2 ;;
    --gen-root) GEN_ROOT="$2"; shift 2 ;;
    --out) OUT_DIR="$2"; shift 2 ;;
    --version) VERSION="$2"; shift 2 ;;
    --publisher) PUBLISHER="$2"; shift 2 ;;
    --platform) PLATFORM="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown arg: $1" >&2; usage; exit 2 ;;
  esac
done

if [[ -z "${VERSION}" ]]; then
  echo "ERROR: --version is required" >&2
  exit 2
fi

if [[ ! -d "${BIN_DIR}" ]]; then
  echo "ERROR: --bin-dir not found: ${BIN_DIR}" >&2
  exit 2
fi

SPECS_DIR="${GEN_ROOT}/specs"
CONFIG_DIR="${GEN_ROOT}/config"
PAYLOAD_DIR="${GEN_ROOT}/payload"

mkdir -p "${OUT_DIR}" "${PAYLOAD_DIR}"

svc_name_from_exe() {
  local exe="$1"
  local base="${exe%_server}"

  # Special cases for services with compound names
  case "${base}" in
    clustercontroller) echo "cluster-controller" ;;
    nodeagent) echo "node-agent" ;;
    *) echo "${base//_/-}" ;;
  esac
}

build_one() {
  local exe_path="$1"
  local exe
  exe="$(basename "${exe_path}")"
  local svc
  svc="$(svc_name_from_exe "${exe}")"

  local spec_src="${SPECS_DIR}/${svc}_service.yaml"
  local cfg_src="${CONFIG_DIR}/${svc}/config.json"

  if [[ ! -f "${spec_src}" ]]; then
    echo "WARN: missing spec for ${svc}: ${spec_src} (run make specgen first); skipping"
    return 0
  fi

  local root="${PAYLOAD_DIR}/${svc}"
  rm -rf "${root}"
  mkdir -p "${root}/bin" "${root}/specs"

  cp -a "${exe_path}" "${root}/bin/${exe}"
  cp -a "${spec_src}" "${root}/specs/${svc}_service.yaml"

  # Copy config if it exists
  if [[ -f "${cfg_src}" ]]; then
    mkdir -p "${root}/config/${svc}"
    cp -a "${cfg_src}" "${root}/config/${svc}/config.json"
  fi

  echo "==> pkg build ${svc} (${exe})"
  "${GLOBULAR_BIN}" pkg build \
    --spec "${root}/specs/${svc}_service.yaml" \
    --root "${root}" \
    --version "${VERSION}" \
    --publisher "${PUBLISHER}" \
    --platform "${PLATFORM}" \
    --out "${OUT_DIR}" \
    --skip-missing-config=true \
    --skip-missing-systemd=true
}

shopt -s nullglob
bins=( "${BIN_DIR}"/*_server )
if [[ ${#bins[@]} -eq 0 ]]; then
  echo "ERROR: no *_server binaries found in ${BIN_DIR}" >&2
  exit 1
fi

for exe_path in "${bins[@]}"; do
  [[ -x "${exe_path}" ]] || continue
  build_one "${exe_path}"
done

echo "Done. Packages in: ${OUT_DIR}"
