#!/usr/bin/env bash
set -euo pipefail

# Defaults
BIN_DIR="/home/dave/Documents/github.com/globulario/services/golang/tools/stage/linux-amd64/usr/local/bin"
GEN_ROOT="$(pwd)/generated"
OUT_DIR="$(pwd)/generated/packages"
SCRIPTS_DIR=""
VERSION="0.0.1"
VERSIONS_FILE=""
PUBLISHER="core@globular.io"
PLATFORM="$(go env GOOS)_$(go env GOARCH)"
GLOBULAR_BIN="${GLOBULAR_BIN:-globular}"

usage() {
  cat <<EOF
Usage: $0 [options]

Builds one package per *_server binary by assembling a per-service payload root and invoking:
  globular pkg build --spec <spec> --root <payload-root> --version <ver> --out <out>

Per-package versions (recommended for releases):
  --versions-file <path>   File with one "svcname=version" per line (e.g. authentication=1.2.43).
                           Overrides --version for packages listed. Packages not listed use
                           the --version default. Use this to preserve BOM version identity.

  Without --versions-file, all packages get the same --version (a single platform stamp —
  this is WRONG for unchanged packages in a mixed-version BOM release).

Common options:
  --globular <path>     Path to globular CLI binary (default: env GLOBULAR_BIN or 'globular')
  --bin-dir <path>      Directory containing *_server binaries
  --gen-root <path>     Directory containing generated/specs and generated/config
  --scripts-dir <path>  Directory containing per-service post-install scripts
  --out <path>          Output packages directory
  --version <ver>       Default version for packages not listed in --versions-file
  --publisher <id>      Publisher (default: core@globular.io)
  --platform <goos_goarch> Platform (default: current go env)

Example (BOM-correct release build):
  $0 --globular ./globularcli \\
     --bin-dir /path/to/stage/bin \\
     --gen-root ./generated \\
     --out ./generated/packages \\
     --versions-file build/package-versions.txt
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
    --scripts-dir) SCRIPTS_DIR="$2"; shift 2 ;;
    --out) OUT_DIR="$2"; shift 2 ;;
    --version) VERSION="$2"; shift 2 ;;
    --versions-file) VERSIONS_FILE="$2"; shift 2 ;;
    --publisher) PUBLISHER="$2"; shift 2 ;;
    --platform) PLATFORM="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown arg: $1" >&2; usage; exit 2 ;;
  esac
done

# Load per-package versions. Format: one "svcname=version" per line.
# Keys are service names (e.g. "authentication", not "authentication_server").
# Lines prefixed with # are comments.
declare -A PKG_VERSIONS=()
if [[ -n "${VERSIONS_FILE}" && -f "${VERSIONS_FILE}" ]]; then
  while IFS='=' read -r key val; do
    key="$(echo "$key" | tr -d ' ')"
    val="$(echo "$val" | tr -d ' ')"
    [[ -z "$key" || "$key" == \#* ]] && continue
    # Strip path prefix if caller passed gen-version.sh format (authentication/authentication_server).
    key="${key%%/*}"
    PKG_VERSIONS["$key"]="$val"
  done < "${VERSIONS_FILE}"
  echo "pkggen: loaded ${#PKG_VERSIONS[@]} per-package versions from ${VERSIONS_FILE}"
elif [[ -n "${VERSIONS_FILE}" ]]; then
  echo "ERROR: --versions-file not found: ${VERSIONS_FILE}" >&2
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
  echo "${exe%_server}"
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

  cp -L "${exe_path}" "${root}/bin/${exe}"
  cp -a "${spec_src}" "${root}/specs/${svc}_service.yaml"

  # Copy config if it exists
  if [[ -f "${cfg_src}" ]]; then
    mkdir -p "${root}/config/${svc}"
    cp -a "${cfg_src}" "${root}/config/${svc}/config.json"
  fi

  # Copy data directory if it exists (e.g. workflow definitions)
  local data_src="${PAYLOAD_DIR}/${svc}/data"
  if [[ -d "${data_src}" ]]; then
    cp -a "${data_src}" "${root}/data"
  fi

  # Copy generated authorization policy files if they exist.
  # These land at /var/lib/globular/policy/services/{svc}/ on the node,
  # enabling the runtime resolver to resolve gRPC method paths to action keys.
  local policy_src="${GEN_ROOT}/policy/${svc}"
  if [[ -d "${policy_src}" ]]; then
    mkdir -p "${root}/policy"
    for f in permissions.generated.json roles.generated.json; do
      if [[ -f "${policy_src}/${f}" ]]; then
        cp -a "${policy_src}/${f}" "${root}/policy/${f}"
      fi
    done
  fi

  # Resolve per-package version: versions file wins over global --version default.
  local pkg_version="${VERSION}"
  if [[ ${#PKG_VERSIONS[@]} -gt 0 && -n "${PKG_VERSIONS[$svc]+x}" ]]; then
    pkg_version="${PKG_VERSIONS[$svc]}"
  fi

  echo "==> pkg build ${svc} (${exe}) version=${pkg_version}"
  local scripts_flag=""
  if [[ -n "${SCRIPTS_DIR}" ]]; then
    scripts_flag="--scripts-dir ${SCRIPTS_DIR}"
  fi
  # shellcheck disable=SC2086
  "${GLOBULAR_BIN}" pkg build \
    --spec "${root}/specs/${svc}_service.yaml" \
    --root "${root}" \
    ${scripts_flag} \
    --version "${pkg_version}" \
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
