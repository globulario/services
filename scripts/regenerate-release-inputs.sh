#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICES_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
PACKAGES_ROOT="${SERVICES_ROOT}/../packages"
GENERATED_ROOT="${SERVICES_ROOT}/generated"
STAGE_BIN="${SERVICES_ROOT}/golang/tools/stage/linux-amd64/usr/local/bin"
GOLANG_ROOT="${SERVICES_ROOT}/golang"
PROTO_DIR="${SERVICES_ROOT}/proto"
WORKFLOW_DEFS="${GOLANG_ROOT}/workflow/definitions"
MANIFEST_PATH="${GENERATED_ROOT}/release-inputs.manifest.json"
VERSIONS_FILE="${SERVICES_ROOT}/golang/build/package-versions.txt"
VERSION="0.0.0-dev"

die() { echo "ERROR: $*" >&2; exit 1; }
info() { echo "  → $*"; }
ok() { echo "  ✓ $*"; }

usage() {
  cat <<'EOF'
Usage:
  bash scripts/regenerate-release-inputs.sh [--version X.Y.Z]

This is the explicit regeneration boundary for services/generated release inputs.
It wipes only known generated release-input subtrees/files, regenerates policy
and specs, rebuilds generated service package templates, and writes a freshness
manifest consumed by scripts/build-release.sh.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --version)
      [[ $# -ge 2 ]] || die "--version requires a value"
      VERSION="${2#v}"
      shift 2
      ;;
    *)
      die "unknown argument: $1"
      ;;
  esac
done

[[ -d "${PACKAGES_ROOT}" ]] || die "packages repo not found at ${PACKAGES_ROOT}"
[[ -d "${STAGE_BIN}" ]] || die "stage binary directory not found at ${STAGE_BIN}"
[[ -x "${STAGE_BIN}/globularcli" ]] || die "globularcli not found at ${STAGE_BIN}/globularcli"
command -v protoc >/dev/null 2>&1 || die "protoc is required to regenerate generated/policy"

PKGMAP="${SERVICES_ROOT}/golang/build/pkg-map.json"

pkgmap_field() {
  local name="$1" field="$2"
  python3 - "${PKGMAP}" "${name}" "${field}" <<'PYEOF'
import json, sys
path, name, field = sys.argv[1:]
with open(path, "r", encoding="utf-8") as f:
    pkgmap = json.load(f)
entry = pkgmap.get(name, {})
val = entry.get(field, "")
if isinstance(val, list):
    print(",".join(str(x) for x in val))
elif val is None:
    print("")
else:
    print(str(val))
PYEOF
}

resolve_package_root() {
  local pkg_name="$1"
  local candidate
  for candidate in "${PACKAGES_ROOT}/${pkg_name}" "${PACKAGES_ROOT}/metadata/${pkg_name}"; do
    if [[ -d "${candidate}" ]]; then
      echo "${candidate}"
      return 0
    fi
  done
  return 1
}

resolve_local_generated_package_source() {
  local pkg_name="$1"
  case "${pkg_name}" in
    globular-cli)
      printf '%s	%s	%s
'         "${GOLANG_ROOT}/globularcli/specs.yaml"         "globular"         "${GOLANG_ROOT}/globularcli"
      ;;
    mcp)
      printf '%s	%s	%s
'         "${GOLANG_ROOT}/mcp/specs.yaml"         "mcp_server"         "${GOLANG_ROOT}/mcp"
      ;;
    *)
      return 1
      ;;
  esac
}

resolve_package_version() {
  local pkg_name="$1"
  python3 - "${VERSIONS_FILE}" "${pkg_name}" "${VERSION}" <<'PYEOF'
import sys
from pathlib import Path
versions_path, pkg_name, default = sys.argv[1:]
path = Path(versions_path)
if path.is_file():
    for raw in path.read_text(encoding='utf-8').splitlines():
        line = raw.split('#', 1)[0].strip()
        if not line or '=' not in line:
            continue
        key, value = [part.strip() for part in line.split('=', 1)]
        if key in {pkg_name, pkg_name.replace('-', '_'), pkg_name.replace('_', '-')}:
            print(value)
            raise SystemExit(0)
print(default)
PYEOF
}

clear_release_inputs() {
  mkdir -p "${GENERATED_ROOT}"
  rm -rf \
    "${GENERATED_ROOT}/specs" \
    "${GENERATED_ROOT}/packages" \
    "${GENERATED_ROOT}/policy" \
    "${GENERATED_ROOT}/payload"
  find "${GENERATED_ROOT}" -maxdepth 1 -type f -name '*.tgz' -delete
  find "${GENERATED_ROOT}" -maxdepth 1 -type d -name '.pkg-staging-*' -exec rm -rf {} +
  rm -f "${MANIFEST_PATH}"
  ok "wiped generated release-input subtrees and top-level package templates"
}

regenerate_policy() {
  local descriptor_out="${GENERATED_ROOT}/policy/descriptor.pb"
  info "Generating authz policy descriptors..."
  mkdir -p "${GENERATED_ROOT}/policy"
  mapfile -t protos < <(find "${PROTO_DIR}" -maxdepth 1 -name '*.proto' | sort)
  [[ ${#protos[@]} -gt 0 ]] || die "no proto files found under ${PROTO_DIR}"
  protoc -I "${PROTO_DIR}" --descriptor_set_out="${descriptor_out}" --include_imports "${protos[@]}"
  (
    cd "${GOLANG_ROOT}"
    GOCACHE="${GOCACHE:-/tmp/.cache/go-build}" go run ./globularcli/tools/authzgen \
      -descriptor "${descriptor_out}" \
      -out "${GENERATED_ROOT}/policy"
  )
  ok "regenerated generated/policy"
}

sync_workflow_payload() {
  local dst="${GENERATED_ROOT}/payload/workflow/definitions"
  info "Syncing workflow payload definitions..."
  mkdir -p "${dst}"
  cp "${WORKFLOW_DEFS}"/*.yaml "${dst}/"
  ok "regenerated generated/payload/workflow/definitions"
}

regenerate_specs() {
  info "Generating service specs..."
  bash "${GOLANG_ROOT}/globularcli/tools/specgen/specgen.sh" "${STAGE_BIN}" "${GENERATED_ROOT}"
  ok "regenerated generated/specs"
}

build_extra_template() {
  local pkg_name="$1" src_bin="$2"
  local metadata_dir spec_src entrypoint local_root tmpdir scripts_arg=()
  local -a spec_candidates
  local local_source=""

  [[ -x "${src_bin}" ]] || die "required stage binary missing for ${pkg_name}: ${src_bin}"

  if local_source="$(resolve_local_generated_package_source "${pkg_name}")"; then
    IFS=$'	' read -r spec_src entrypoint local_root <<< "${local_source}"
    metadata_dir="${local_root}"
    [[ -f "${spec_src}" ]] || die "local spec missing for ${pkg_name}: ${spec_src}"
  else
    metadata_dir="$(resolve_package_root "${pkg_name}")" || die "metadata directory missing for ${pkg_name}: ${PACKAGES_ROOT}/${pkg_name} or ${PACKAGES_ROOT}/metadata/${pkg_name}"
    mapfile -t spec_candidates < <(find "${metadata_dir}/specs" -maxdepth 1 -name '*.yaml' | sort)
    [[ ${#spec_candidates[@]} -eq 1 ]] || die "expected exactly one spec for ${pkg_name} under ${metadata_dir}/specs"
    spec_src="${spec_candidates[0]}"
    entrypoint="$(python3 -c 'import json,sys; print((json.load(open(sys.argv[1], encoding="utf-8")).get("entrypoint") or "").split("/")[-1])' "${metadata_dir}/package.json")"
    [[ -n "${entrypoint}" ]] || die "package.json entrypoint missing for ${pkg_name}"
  fi
  local pkg_version="$(resolve_package_version "${pkg_name}")"

  tmpdir="$(mktemp -d "${GENERATED_ROOT}/.pkg-staging-${pkg_name//\//-}.XXXXXX")"
  mkdir -p "${tmpdir}/bin" "${tmpdir}/specs"
  cp "${src_bin}" "${tmpdir}/bin/${entrypoint}"
  chmod 755 "${tmpdir}/bin/${entrypoint}"
  cp "${spec_src}" "${tmpdir}/specs/$(basename "${spec_src}")"

  if [[ -d "${metadata_dir}/data" ]]; then
    cp -a "${metadata_dir}/data" "${tmpdir}/data"
  fi
  if [[ -d "${metadata_dir}/scripts" ]]; then
    scripts_arg=(--scripts-dir "${metadata_dir}/scripts")
  fi

  "${STAGE_BIN}/globularcli" pkg build \
    --spec "${tmpdir}/specs/$(basename "${spec_src}")" \
    --root "${tmpdir}" \
    "${scripts_arg[@]}" \
    --version "${pkg_version}" \
    --publisher "core@globular.io" \
    --platform "linux_amd64" \
    --out "${GENERATED_ROOT}" \
    --skip-missing-config=true \
    --skip-missing-systemd=true >/dev/null
  rm -rf "${tmpdir}"
  ok "regenerated template ${pkg_name}_${pkg_version}_linux_amd64.tgz"
}

regenerate_service_templates() {
  info "Generating service package templates..."
  bash "${GOLANG_ROOT}/globularcli/tools/pkggen/pkggen.sh" \
    --globular "${STAGE_BIN}/globularcli" \
    --bin-dir "${STAGE_BIN}" \
    --gen-root "${GENERATED_ROOT}" \
    --out "${GENERATED_ROOT}" \
    --version "${VERSION}" \
    --versions-file "${VERSIONS_FILE}" \
    --publisher "core@globular.io" \
    --platform "linux_amd64"

  build_extra_template "globular-cli" "${STAGE_BIN}/globularcli"
  build_extra_template "mcp" "${STAGE_BIN}/mcp"
  build_extra_template "gateway" "${STAGE_BIN}/gateway"
  build_extra_template "xds" "${STAGE_BIN}/xds"
}

validate_generated_inputs() {
  echo "  → Validating regenerated release inputs..." >&2
  python3 - "${PKGMAP}" "${GENERATED_ROOT}" "${VERSION}" "${VERSIONS_FILE}" <<'PYEOF'
import glob, json, os, sys
from pathlib import Path

pkgmap_path, generated_root, version, versions_file = sys.argv[1:]
with open(pkgmap_path, "r", encoding="utf-8") as f:
    pkgmap = json.load(f)

overrides = {}
path = Path(versions_file)
if path.is_file():
    for raw in path.read_text(encoding="utf-8").splitlines():
        line = raw.split("#", 1)[0].strip()
        if not line or "=" not in line:
            continue
        key, value = [part.strip() for part in line.split("=", 1)]
        overrides[key] = value

def expected_version(name: str) -> str:
    aliases = {
        name,
        name.replace("-", "_"),
        name.replace("_", "-"),
    }
    for alias in aliases:
        if alias in overrides:
            return overrides[alias]
    return version

# Platform-versioned packages: those with a go_target OR in the special set
required = set()
for name, pkg in pkgmap.items():
    go_target = str(pkg.get("go_target") or "").strip()
    is_platform = pkg.get("platform_version", True)
    if is_platform and (go_target or name in {"globular-cli", "mcp", "gateway", "xds"}):
        required.add(name)

actual = set()
for path in sorted(glob.glob(os.path.join(generated_root, "*.tgz"))):
    base = os.path.basename(path)
    if not base.endswith("_linux_amd64.tgz"):
        raise SystemExit(f"unexpected generated template name: {base}")
    stem = base[:-len("_linux_amd64.tgz")]
    try:
        name, file_version = stem.rsplit("_", 1)
    except ValueError:
        raise SystemExit(f"unexpected generated template naming format: {base}")
    if name not in pkgmap:
        raise SystemExit(f"generated template {base} is not in pkg-map.json")
    expected = expected_version(name)
    if file_version != expected:
        raise SystemExit(f"generated template {base} has version {file_version}, expected {expected}")
    actual.add(name)

missing = sorted(required - actual)
extra = sorted(actual - required)
if missing:
    raise SystemExit("missing regenerated templates: " + ", ".join(missing))
if extra:
    raise SystemExit("unexpected regenerated templates: " + ", ".join(extra))
print(len(actual))
PYEOF
}

write_manifest() {
  local package_count="$1"
  python3 - "${GENERATED_ROOT}" "${MANIFEST_PATH}" "${VERSION}" "${STAGE_BIN}" "${package_count}" <<'PYEOF'
import glob, hashlib, json, os, sys
from datetime import datetime, timezone

root, manifest_path, version, stage_bin, package_count = sys.argv[1:]
packages = []
for path in sorted(glob.glob(os.path.join(root, "*.tgz"))):
    raw = open(path, "rb").read()
    packages.append({
        "filename": os.path.basename(path),
        "sha256": hashlib.sha256(raw).hexdigest(),
    })
manifest = {
    "schema": "globular.release-inputs/v1",
    "generated_at": datetime.now(timezone.utc).isoformat(),
    "version": version,
    "source_stage_bin": stage_bin,
    "package_count": int(package_count),
    "packages": packages,
}
with open(manifest_path, "w", encoding="utf-8") as f:
    json.dump(manifest, f, indent=2)
PYEOF
  ok "wrote ${MANIFEST_PATH}"
}

clear_release_inputs
regenerate_policy
sync_workflow_payload
regenerate_specs
regenerate_service_templates
PACKAGE_COUNT="$(validate_generated_inputs)"
write_manifest "${PACKAGE_COUNT}"
ok "regenerated ${PACKAGE_COUNT} release-input templates under ${GENERATED_ROOT}"
