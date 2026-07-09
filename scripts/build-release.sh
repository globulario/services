#!/usr/bin/env bash
# Local release build — mirrors what GitHub Actions does in release.yml.
#
# Usage:
#   cd /path/to/services
#   bash scripts/build-release.sh [version] [--bump patch|minor|major] [--full-regenerate]
#
# Output:
#   services/dist/globular-<version>-linux-amd64.tar.gz
#   services/dist/globular-<version>-linux-amd64.tar.gz.sha256
#
# services/dist is disposable release output. Each release build starts by
# removing and recreating it from authoritative inputs. Temporary assembly
# staging lives under services/dist/.staging/ and must not be treated as source
# authority.
#
# IDENTITY MODEL (docs/design/package-identity-single-authority.md):
#   - [version] argument is the PLATFORM release version (bundle name,
#     release-index platform_release). It is NEVER a package version.
#   - Service package versions come from committed zz_version_generated.go
#     files, materialized via scripts/gen-package-versions-from-source.sh.
#     This script does not override them (no -X main.Version in the normal
#     lane) and does not rewrite package.json versions.
#   - Infra/external package versions are owned by the packages repo and
#     pass through byte-identical.
#   - build_id / build_number are repository-admission identity. This script
#     MUST NOT mint them; bundle package.json ships without them and the
#     repository assigns identity at Day-0 admission (deterministic
#     content-derived pins).
#
# Requires: go, python3, tar, sha256sum

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICES_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
PACKAGES_ROOT="${SERVICES_ROOT}/../packages"
INSTALLER_ROOT="${SERVICES_ROOT}/../globular-installer"
GLOBULAR_ROOT="${SERVICES_ROOT}/../Globular"
DIST_DIR="${SERVICES_ROOT}/dist"
DIST_STAGING_DIR="${DIST_DIR}/.staging"
BIN_STAGE_DIR="${DIST_STAGING_DIR}/bin"
PKG_STAGE_DIR="${DIST_STAGING_DIR}/packages"
REGISTRY_YAML="${PACKAGES_ROOT}/registry.yaml"
INSTALLER_STAGE_BIN="${BIN_STAGE_DIR}/globular-installer"

VERSION=""
BUMP_KIND="patch"
EXPLICIT_VERSION=0
FULL_REGENERATE=0
REUSE_GENERATED_RELEASE_INPUTS=0
ALLOW_EXTRACTED_BUNDLE_SOURCES=0
declare -a EXTRACTED_BUNDLE_SOURCE_DIRS=()

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BOLD='\033[1m'; NC='\033[0m'
die()     { echo -e "${RED}ERROR: $*${NC}" >&2; exit 1; }
ok()      { echo -e "${GREEN}  ✓ $*${NC}"; }
warn()    { echo -e "${YELLOW}  ⚠ $*${NC}"; }
info()    { echo "  → $*"; }
section() { echo ""; echo -e "${BOLD}━━━ $* ━━━${NC}"; echo ""; }

usage() {
  cat <<'EOF'
Usage:
  bash scripts/build-release.sh [version] [--bump patch|minor|major] [--full-regenerate] [--allow-extracted-bundle-sources <bundle-or-packages-dir> ...]

Release mode defaults to controlled package sources only:
  - services/generated (generated workspace only)
  - packages/dist (package artifact output validated against current registry-backed staged inputs)

When [version] is omitted, the script derives the next platform release version
from the latest git tag in the services repo after refreshing tags from
`origin`. The default bump is patch.

Use --full-regenerate to wipe and rebuild services/generated release inputs
before assembling the release bundle.

Extracted bundle package dirs are forbidden unless explicitly allowed.
EOF
}

normalize_version() {
  local raw="$1"
  raw="${raw#v}"
  printf '%s\n' "${raw}"
}

validate_release_version() {
  local version="$1"
  [[ "${version}" =~ ^[0-9]+(\.[0-9]+){2}([.-][0-9A-Za-z._-]+)?$ ]] || \
    die "invalid release version '${version}' — expected SemVer-like X.Y.Z"
  [[ "${version}" != "0.0.0-dev" ]] || \
    die "release build refuses platform version 0.0.0-dev — pass an explicit release version or use tag-derived bumping"
}

latest_release_tag() {
  local tag
  while IFS= read -r tag; do
    [[ "${tag}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || continue
    printf '%s\n' "${tag}"
    return 0
  done < <(git -C "${SERVICES_ROOT}" tag --sort=-version:refname)
  return 1
}

refresh_release_tags() {
  git -C "${SERVICES_ROOT}" remote get-url origin >/dev/null 2>&1 || \
    die "cannot refresh release tags: git remote 'origin' is not configured"
  echo "  → Refreshing release tags from origin..." >&2
  git -C "${SERVICES_ROOT}" fetch --tags --force origin >/dev/null 2>&1 || \
    die "failed to refresh release tags from origin; refusing to derive a platform version from stale local tags"
}

bump_release_version() {
  local current="$1" kind="$2"
  python3 - "$current" "$kind" <<'PYEOF'
import sys
version, kind = sys.argv[1:]
parts = version.split(".")
if len(parts) != 3 or not all(p.isdigit() for p in parts):
    raise SystemExit(f"latest release tag must be strict semver X.Y.Z; got {version!r}")
major, minor, patch = (int(p) for p in parts)
if kind == "patch":
    patch += 1
elif kind == "minor":
    minor += 1
    patch = 0
elif kind == "major":
    major += 1
    minor = 0
    patch = 0
else:
    raise SystemExit(f"unsupported bump kind: {kind}")
print(f"{major}.{minor}.{patch}")
PYEOF
}

resolve_release_version() {
  local latest_tag latest_version next_version
  refresh_release_tags
  latest_tag="$(latest_release_tag)" || \
    die "could not determine latest release tag from git; pass an explicit version"
  latest_version="$(normalize_version "${latest_tag}")"
  next_version="$(bump_release_version "${latest_version}" "${BUMP_KIND}")" || \
    die "failed to derive next release version from ${latest_tag}"
  printf '%s\n' "${next_version}"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --bump)
      [[ $# -ge 2 ]] || die "--bump requires patch, minor, or major"
      case "$2" in
        patch|minor|major) BUMP_KIND="$2" ;;
        *) die "unsupported bump kind '$2' — expected patch, minor, or major" ;;
      esac
      shift 2
      ;;
    --full-regenerate)
      FULL_REGENERATE=1
      shift
      ;;
    --allow-extracted-bundle-sources)
      [[ $# -ge 2 ]] || die "--allow-extracted-bundle-sources requires a path"
      ALLOW_EXTRACTED_BUNDLE_SOURCES=1
      EXTRACTED_BUNDLE_SOURCE_DIRS+=("$2")
      shift 2
      ;;
    --*)
      die "unknown flag: $1"
      ;;
    *)
      if (( EXPLICIT_VERSION )); then
        die "unexpected extra argument: $1"
      fi
      VERSION="$(normalize_version "$1")"
      EXPLICIT_VERSION=1
      shift
      ;;
  esac
done

[[ -d "${PACKAGES_ROOT}" ]] || die "packages repo not found at ${PACKAGES_ROOT} — clone it alongside services"
[[ -f "${REGISTRY_YAML}" ]] || die "registry.yaml not found at ${REGISTRY_YAML}"

if (( ! EXPLICIT_VERSION )); then
  VERSION="$(resolve_release_version)"
  info "Auto-selected platform release version ${VERSION} from latest git tag (bump=${BUMP_KIND})"
else
  info "Using explicit platform release version ${VERSION}"
fi
validate_release_version "${VERSION}"

PACKAGE_PUBLISHER="${GLOBULAR_PACKAGE_PUBLISHER:-core@globular.io}"
PACKAGE_VERSION_SUFFIX="${GLOBULAR_PACKAGE_VERSION_SUFFIX:-}"
if [[ "${PACKAGE_PUBLISHER}" != "core@globular.io" && -z "${PACKAGE_VERSION_SUFFIX}" ]]; then
  die "GLOBULAR_PACKAGE_PUBLISHER=${PACKAGE_PUBLISHER} requires GLOBULAR_PACKAGE_VERSION_SUFFIX (e.g. +local.$(hostname -s).1)"
fi
if [[ -n "${PACKAGE_VERSION_SUFFIX}" && ! "${PACKAGE_VERSION_SUFFIX}" =~ ^[+-][A-Za-z0-9][A-Za-z0-9._-]*$ ]]; then
  die "invalid GLOBULAR_PACKAGE_VERSION_SUFFIX=${PACKAGE_VERSION_SUFFIX}; expected +local.name.1 or -hotfix.name.1"
fi
if [[ -n "${PACKAGE_VERSION_SUFFIX}" || "${PACKAGE_PUBLISHER}" != "core@globular.io" ]]; then
  info "Package identity lane: publisher=${PACKAGE_PUBLISHER} suffix=${PACKAGE_VERSION_SUFFIX} (platform_release=${VERSION})"
fi

# ── Per-package version authority ────────────────────────────────────────────
# Committed zz_version_generated.go files are the per-package version
# authority. Materialize them once; every packaging/validation step below
# consumes this map. The platform VERSION is never a package version.
PACKAGE_VERSIONS_FILE="${SERVICES_ROOT}/golang/build/package-versions.txt"
bash "${SCRIPT_DIR}/gen-package-versions-from-source.sh" --out "${PACKAGE_VERSIONS_FILE}"
declare -A PKG_VERSIONS=()
while IFS='=' read -r pv_name pv_ver; do
  [[ -z "${pv_name}" || "${pv_name}" == \#* ]] && continue
  PKG_VERSIONS["${pv_name}"]="${pv_ver}"
done < "${PACKAGE_VERSIONS_FILE}"

# pkg_version_of <registry-name> — committed source version + optional
# hotfix/local suffix lane. Dies for unknown packages (fail-safe).
pkg_version_of() {
  local name="$1"
  [[ -n "${PKG_VERSIONS[${name}]+x}" ]] || die "no committed version for package '${name}' — run golang/build/gen-version.sh and commit zz_version_generated.go"
  printf '%s%s\n' "${PKG_VERSIONS[${name}]}" "${PACKAGE_VERSION_SUFFIX}"
}

elf_needs_release_strip() {
  local bin="$1"
  [[ -f "${bin}" ]] || return 1
  file -b "${bin}" 2>/dev/null | grep -q '^ELF' || return 1
  readelf -S "${bin}" 2>/dev/null | grep -Eq '\.(debug_|zdebug_|symtab)\b'
}

strip_release_binary() {
  local bin="$1" label="${2:-$(basename "$1")}"
  [[ -f "${bin}" ]] || return 0
  if ! elf_needs_release_strip "${bin}"; then
    return 0
  fi
  command -v strip >/dev/null 2>&1 || die "${label} contains release-forbidden debug sections and 'strip' is unavailable"
  info "Stripping ${label} for release-channel packaging..."
  strip --strip-debug --strip-unneeded "${bin}"
  chmod +x "${bin}"
  if elf_needs_release_strip "${bin}"; then
    die "${label} still contains release-forbidden debug sections after strip"
  fi
}

registry_json_field() {
  local name="$1" field="$2"
  python3 - "$REGISTRY_YAML" "$name" "$field" <<'PYEOF'
import sys, yaml
path, name, field = sys.argv[1:]
with open(path, "r", encoding="utf-8") as f:
    reg = yaml.safe_load(f) or {}
for pkg in reg.get("packages", []):
    if pkg.get("name") == name:
        val = pkg.get(field, "")
        if isinstance(val, list):
            print(",".join(str(x) for x in val))
        elif val is None:
            print("")
        else:
            print(str(val))
        sys.exit(0)
print("")
PYEOF
}

normalize_bundle_packages_dir() {
  local candidate="$1"
  if [[ -d "${candidate}/packages" ]]; then
    echo "${candidate}/packages"
  else
    echo "${candidate}"
  fi
}

validate_generated_release_inputs() {
  local manifest_path="${SERVICES_ROOT}/generated/release-inputs.manifest.json"
  python3 - "${SERVICES_ROOT}/generated" "${manifest_path}" "${REGISTRY_YAML}" "${VERSION}" "${PACKAGE_VERSIONS_FILE}" <<'PYEOF'
import glob, json, os, sys, tarfile
import yaml

generated_root, manifest_path, registry_path, version, versions_file = sys.argv[1:]

# Per-package version authority (committed zz files, materialized).
pkg_versions = {}
with open(versions_file, "r", encoding="utf-8") as f:
    for line in f:
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        k, _, v = line.partition("=")
        pkg_versions[k.strip()] = v.strip()
files = sorted(glob.glob(os.path.join(generated_root, "*.tgz")))
if not files:
    sys.exit(0)
if not os.path.isfile(manifest_path):
    raise SystemExit(f"generated release inputs present but manifest missing: {manifest_path}")
with open(manifest_path, "r", encoding="utf-8") as f:
    manifest = json.load(f)
if manifest.get("schema") != "globular.release-inputs/v1":
    raise SystemExit(f"{manifest_path}: schema must be globular.release-inputs/v1")
if str(manifest.get("version") or "").strip() != version:
    raise SystemExit(f"{manifest_path}: version {manifest.get('version')!r} does not match release version {version!r}; rerun --full-regenerate")
if int(manifest.get("package_count") or 0) != len(files):
    raise SystemExit(f"{manifest_path}: package_count={manifest.get('package_count')} does not match generated template count {len(files)}")

with open(registry_path, "r", encoding="utf-8") as f:
    reg = yaml.safe_load(f) or {}
registry = {str(pkg.get("name") or "").strip() for pkg in reg.get("packages", []) if str(pkg.get("name") or "").strip()}
manifest_files = {pkg.get("filename") for pkg in manifest.get("packages", []) if pkg.get("filename")}
actual_files = {os.path.basename(path) for path in files}
if manifest_files != actual_files:
    raise SystemExit(f"{manifest_path}: manifest package list does not match current generated/*.tgz files")

for path in files:
    base = os.path.basename(path)
    try:
        with tarfile.open(path, "r:gz") as tf:
            member = None
            for cand in ("package.json", "./package.json"):
                try:
                    member = tf.getmember(cand)
                    break
                except KeyError:
                    continue
            if member is None:
                raise SystemExit(f"{base}: package.json not found")
            pkg = json.loads(tf.extractfile(member).read())
    except tarfile.TarError as exc:
        raise SystemExit(f"{base}: invalid generated package template: {exc}")

    name = str(pkg.get("name") or "").strip()
    file_version = str(pkg.get("version") or "").strip()
    if not name or name not in registry:
        raise SystemExit(f"{base}: generated package template is not registry-backed")
    expected = pkg_versions.get(name)
    if expected is None:
        raise SystemExit(f"{base}: no committed version authority for {name!r} — run gen-version.sh and commit zz_version_generated.go")
    if file_version != expected:
        raise SystemExit(f"{base}: generated package template version {file_version!r} does not match committed package version {expected!r}; rerun --full-regenerate")
PYEOF
}

collect_source_package_dirs() {
  local -a dirs=()
  local candidate normalized

  if compgen -G "${SERVICES_ROOT}/generated/*.tgz" >/dev/null; then
    validate_generated_release_inputs || die "services/generated release inputs failed validation"
    dirs+=("${SERVICES_ROOT}/generated")
  fi
  if compgen -G "${PACKAGES_ROOT}/dist/*.tgz" >/dev/null; then
    dirs+=("${PACKAGES_ROOT}/dist")
  fi
  if (( ALLOW_EXTRACTED_BUNDLE_SOURCES )); then
    for candidate in "${EXTRACTED_BUNDLE_SOURCE_DIRS[@]}"; do
      normalized="$(normalize_bundle_packages_dir "${candidate}")"
      [[ -d "${normalized}" ]] || die "extracted bundle source dir not found: ${candidate}"
      [[ -f "${normalized}/../release-index.json" ]] || die "explicit extracted bundle source missing sibling release-index.json: ${normalized}"
      dirs+=("${normalized}")
    done
  fi

  if [[ ${#dirs[@]} -eq 0 ]]; then
    return 1
  fi

  printf '%s\n' "${dirs[@]}"
}

list_external_registry_packages() {
  python3 - "${REGISTRY_YAML}" <<'PYEOF'
import sys, yaml

with open(sys.argv[1], "r", encoding="utf-8") as f:
    reg = yaml.safe_load(f) or {}

generated = {"globular-cli", "gateway", "xds"}
for pkg in reg.get("packages", []):
    name = str(pkg.get("name") or "").strip()
    go_target = str(pkg.get("go_target") or "").strip()
    if not name:
        continue
    if go_target or name in generated:
        continue
    print(name)
PYEOF
}

find_source_package() {
  local pkg_name="$1"
  shift
  local -a dirs=("$@")
  local dir match found=""
  for dir in "${dirs[@]}"; do
    match=$(find "${dir}" -maxdepth 1 -name "${pkg_name}_*_linux_amd64.tgz" 2>/dev/null | sort -V | tail -1 || true)
    [[ -n "${match}" ]] || continue
    if [[ -n "${found}" && "${found}" != "${match}" ]]; then
      die "ambiguous source package for ${pkg_name}: ${found} and ${match}"
    fi
    found="${match}"
  done
  [[ -n "${found}" ]] || return 1
  echo "${found}"
}

ensure_generated_source_package_template() {
  local pkg_name="$1" bin_name="$2"
  local metadata_dir spec_src policy_dir tmpdir out_pkg
  local -a spec_candidates
  local -a scripts_args=()

  out_pkg="$(find_source_package "${pkg_name}" "${GENERATED_PKG_DIR}" || true)"
  if [[ -n "${out_pkg}" ]]; then
    echo "${out_pkg}"
    return 0
  fi

  metadata_dir="${PACKAGES_ROOT}/metadata/${pkg_name}"
  [[ -d "${metadata_dir}" ]] || die "cannot synthesize package template for ${pkg_name}: metadata dir missing at ${metadata_dir}"

  mapfile -t spec_candidates < <(find "${metadata_dir}/specs" -maxdepth 1 -name '*.yaml' | sort)
  [[ ${#spec_candidates[@]} -eq 1 ]] || die "cannot synthesize package template for ${pkg_name}: expected exactly one canonical spec under ${metadata_dir}/specs"
  spec_src="${spec_candidates[0]}"

  [[ -x "${BIN_STAGE_DIR}/globular" ]] || die "cannot synthesize package template for ${pkg_name}: staged globular CLI missing at ${BIN_STAGE_DIR}/globular"
  [[ -f "${BIN_STAGE_DIR}/${bin_name}" ]] || die "cannot synthesize package template for ${pkg_name}: staged binary missing at ${BIN_STAGE_DIR}/${bin_name}"

  tmpdir="$(mktemp -d)"
  mkdir -p "${tmpdir}/bin" "${tmpdir}/specs"
  cp "${BIN_STAGE_DIR}/${bin_name}" "${tmpdir}/bin/${bin_name}"
  chmod 755 "${tmpdir}/bin/${bin_name}"
  cp "${spec_src}" "${tmpdir}/specs/$(basename "${spec_src}")"

  if [[ -d "${metadata_dir}/data" ]]; then
    cp -a "${metadata_dir}/data" "${tmpdir}/data"
  fi

  policy_dir="${SERVICES_ROOT}/generated/policy/${pkg_name//-/_}"
  if [[ -d "${policy_dir}" ]]; then
    mkdir -p "${tmpdir}/policy"
    for policy_file in permissions.generated.json roles.generated.json; do
      if [[ -f "${policy_dir}/${policy_file}" ]]; then
        cp -a "${policy_dir}/${policy_file}" "${tmpdir}/policy/${policy_file}"
      fi
    done
  fi

  if [[ -d "${metadata_dir}/scripts" ]]; then
    scripts_args=(--scripts-dir "${metadata_dir}/scripts")
  fi

  mkdir -p "${GENERATED_PKG_DIR}"
  "${BIN_STAGE_DIR}/globular" pkg build \
    --spec "${tmpdir}/specs/$(basename "${spec_src}")" \
    --root "${tmpdir}" \
    "${scripts_args[@]}" \
    --version "$(pkg_version_of "${pkg_name}")" \
    --publisher "${PACKAGE_PUBLISHER}" \
    --platform "linux_amd64" \
    --out "${GENERATED_PKG_DIR}" \
    --skip-missing-config=true \
    --skip-missing-systemd=true >/dev/null
  rm -rf "${tmpdir}"

  out_pkg="$(find_source_package "${pkg_name}" "${GENERATED_PKG_DIR}" || true)"
  [[ -n "${out_pkg}" ]] || die "failed to synthesize generated source package template for ${pkg_name}"
  echo "${out_pkg}"
}

extract_package_field() {
  local pkg="$1" field="$2"
  python3 - "${pkg}" "${field}" <<'PYEOF'
import json, sys, tarfile
pkg, field = sys.argv[1:]
with tarfile.open(pkg, "r:gz") as tf:
    member = None
    for cand in ("package.json", "./package.json"):
        try:
            member = tf.getmember(cand)
            break
        except KeyError:
            continue
    if member is None:
        raise SystemExit(f"{pkg}: package.json not found")
    raw = tf.extractfile(member).read()
data = json.loads(raw)
print(data.get(field, ""))
PYEOF
}

classify_source_dir() {
  local dir="$1"
  case "${dir}" in
    "${SERVICES_ROOT}/generated") echo "generated-current-release" ;;
    "${PACKAGES_ROOT}/dist") echo "input-built-current-release" ;;
    *) echo "explicit-carry-forward" ;;
  esac
}

validate_input_built_artifact() {
  local src_pkg="$1"
  local name entrypoint entry_bin checksum staged_bin expected_bin go_target tmpdir extracted_bin metadata_bin current_input

  name="$(extract_package_field "${src_pkg}" name)"
  entrypoint="$(extract_package_field "${src_pkg}" entrypoint)"
  checksum="$(extract_package_field "${src_pkg}" entrypoint_checksum)"
  expected_bin="$(registry_json_field "${name}" binary)"
  go_target="$(registry_json_field "${name}" go_target)"

  [[ -n "${name}" ]] || die "package ${src_pkg} is missing package.json.name"
  [[ -n "${expected_bin}" ]] || die "package ${src_pkg} (${name}) is not in registry.yaml"
  [[ -z "${go_target}" ]] || die "release build must not source ${name} from packages/dist; it is generated from current service source"

  # Binary-less packages intentionally carry manifest entrypoint "none" even
  # when the registry binary names the runtime executable that will be installed
  # later by dpkg or fetch-at-install logic (e.g. sctool, scylladb wrappers).
  if [[ "${entrypoint}" == "none" || "${entrypoint}" == "noop" || "${expected_bin}" == "none" || "${expected_bin}" == "noop" ]]; then
    return 0
  fi

  entry_bin="$(basename "${entrypoint}")"
  [[ "${entry_bin}" == "${expected_bin}" ]] || die "package ${src_pkg} entrypoint ${entrypoint} does not match registry binary ${expected_bin}"

  staged_bin="${PACKAGES_ROOT}/bin/${expected_bin}"
  metadata_bin="${PACKAGES_ROOT}/metadata/${name}/bin/${expected_bin}"
  current_input="${staged_bin}"
  if [[ ! -f "${current_input}" ]]; then
    current_input="${metadata_bin}"
  fi
  [[ -f "${current_input}" ]] || die "package ${name} has dist artifact ${src_pkg} but neither current staged input ${staged_bin} nor package-local wrapper ${metadata_bin} exists; explicit carry-forward classification required"
  if elf_needs_release_strip "${current_input}"; then
    die "package ${name} has stripped dist artifact ${src_pkg} but current input ${current_input} still carries forbidden debug sections; refusing stale-output carry-forward"
  fi

  tmpdir=$(mktemp -d)
  tar -xzf "${src_pkg}" -C "${tmpdir}"
  extracted_bin="${tmpdir}/${entrypoint}"
  [[ -f "${extracted_bin}" ]] || die "package ${src_pkg} is missing entrypoint payload ${entrypoint}"
  if elf_needs_release_strip "${extracted_bin}"; then
    rm -rf "${tmpdir}"
    die "package ${src_pkg} still carries release-forbidden debug sections"
  fi

  local input_sha artifact_sha
  input_sha="$(sha256sum "${current_input}" | awk '{print $1}')"
  artifact_sha="$(sha256sum "${extracted_bin}" | awk '{print $1}')"
  [[ "sha256:${artifact_sha}" == "${checksum}" ]] || die "package ${src_pkg} has package.json entrypoint_checksum=${checksum}, but packaged entrypoint hashes to sha256:${artifact_sha}"
  [[ "${input_sha}" == "${artifact_sha}" ]] || die "package ${src_pkg} entrypoint does not match current input ${current_input}; stale dist artifact detected"
  rm -rf "${tmpdir}"
}

validate_carry_forward_artifact() {
  local src_pkg="$1" source_dir="$2"
  local name artifact_sha entrypoint checksum tmpdir extracted_bin

  (( ALLOW_EXTRACTED_BUNDLE_SOURCES )) || die "explicit carry-forward artifact ${src_pkg} provided without --allow-extracted-bundle-sources"
  name="$(extract_package_field "${src_pkg}" name)"
  [[ -n "${name}" ]] || die "carry-forward artifact ${src_pkg} is missing package.json.name"
  [[ -f "${source_dir}/../release-index.json" ]] || die "carry-forward source ${source_dir} is missing sibling release-index.json"

  artifact_sha="sha256:$(sha256sum "${src_pkg}" | awk '{print $1}')"
  python3 - "${source_dir}/../release-index.json" "${name}" "${artifact_sha}" <<'PYEOF'
import json, sys
path, name, digest = sys.argv[1:]
with open(path, "r", encoding="utf-8") as f:
    idx = json.load(f)
for pkg in idx.get("packages", []):
    if pkg.get("name") != name:
        continue
    if str(pkg.get("artifact_sha256") or pkg.get("package_digest") or "").strip() != digest:
        print(f"artifact digest mismatch for {name}: index has {pkg.get('artifact_sha256') or pkg.get('package_digest')}, file has {digest}", file=sys.stderr)
        sys.exit(1)
    if not pkg.get("origin_release"):
        print(f"carry-forward package {name} has no origin_release in source release-index", file=sys.stderr)
        sys.exit(1)
    sys.exit(0)
print(f"package {name} not found in source release-index", file=sys.stderr)
sys.exit(1)
PYEOF

  entrypoint="$(extract_package_field "${src_pkg}" entrypoint)"
  checksum="$(extract_package_field "${src_pkg}" entrypoint_checksum)"
  tmpdir=$(mktemp -d)
  tar -xzf "${src_pkg}" -C "${tmpdir}"
  extracted_bin="${tmpdir}/${entrypoint}"
  if [[ -f "${extracted_bin}" ]]; then
    if elf_needs_release_strip "${extracted_bin}"; then
      rm -rf "${tmpdir}"
      die "carry-forward artifact ${src_pkg} still carries release-forbidden debug sections"
    fi
    [[ "sha256:$(sha256sum "${extracted_bin}" | awk '{print $1}')" == "${checksum}" ]] || {
      rm -rf "${tmpdir}"
      die "carry-forward artifact ${src_pkg} has mismatched entrypoint checksum"
    }
  fi
  rm -rf "${tmpdir}"
}

# validate_package_payload_sanity — READ-ONLY payload checks. The old
# sanitize_package_payload silently PATCHED payloads (scylla-manager
# ExecStartPre) and re-compressed every external package. Payload fixes belong
# in the authority source (packages repo); the release build only refuses
# known-bad content. No extraction, no rewrite, no re-compression.
validate_package_payload_sanity() {
  local pkg_path="$1"
  case "$(basename "${pkg_path}")" in
    scylla-manager_*.tgz)
      python3 - "${pkg_path}" <<'PYEOF'
import sys, tarfile
pkg_path = sys.argv[1]
legacy = "ExecStartPre=/bin/sh -c 'if [ -x"
spec_fixed = "ExecStartPre={{.Prefix}}/bin/scylla-manager-configure"
unit_fixed_a = "ExecStartPre=/usr/lib/globular/bin/scylla-manager-configure"
hook_spec = "ExecStartPost=-+{{.Prefix}}/bin/scylla-manager-register-cluster"
hook_unit = "ExecStartPost=-+/usr/lib/globular/bin/scylla-manager-register-cluster"
with tarfile.open(pkg_path, "r:gz") as tf:
    for member in tf.getmembers():
        name = member.name.lstrip("./")
        if name == "specs/scylla_manager_service.yaml":
            text = tf.extractfile(member).read().decode("utf-8")
            if legacy in text:
                raise SystemExit(f"{pkg_path}: {name} carries the legacy scylla-manager ExecStartPre — fix it in the packages repo source (packages/metadata/scylla-manager/), not at bundle time")
            if spec_fixed not in text:
                raise SystemExit(f"{pkg_path}: {name} missing fixed scylla-manager ExecStartPre")
            if hook_spec not in text:
                raise SystemExit(f"{pkg_path}: {name} missing scylla-manager ExecStartPost registration hook")
        elif name == "systemd/globular-scylla-manager.service":
            text = tf.extractfile(member).read().decode("utf-8")
            if legacy in text:
                raise SystemExit(f"{pkg_path}: {name} carries the legacy scylla-manager ExecStartPre — fix it in the packages repo source, not at bundle time")
            if unit_fixed_a not in text and spec_fixed not in text:
                raise SystemExit(f"{pkg_path}: {name} missing fixed scylla-manager ExecStartPre")
            if hook_unit not in text and hook_spec not in text:
                raise SystemExit(f"{pkg_path}: {name} missing scylla-manager ExecStartPost registration hook")
PYEOF
      ;;
  esac
}

validate_package_systemd_units() {
  local pkg_path="$1"
  python3 - "${pkg_path}" <<'PYEOF'
import re
import sys
import tarfile

pkg_path = sys.argv[1]
unsafe = []
exec_re = re.compile(r"^Exec(?:Start|StartPre|StartPost|Reload|Stop|StopPost)=(.*)$")
percent_re = re.compile(r"(^|[^%])%(?!%)")

with tarfile.open(pkg_path, "r:gz") as tf:
    for member in tf.getmembers():
        name = member.name.lstrip("./")
        if not name.startswith("systemd/") or not name.endswith(".service"):
            continue
        raw = tf.extractfile(member).read().decode("utf-8")
        for lineno, line in enumerate(raw.splitlines(), start=1):
            m = exec_re.match(line)
            if not m:
                continue
            cmd = m.group(1)
            if "/bin/sh -c" not in cmd:
                continue
            if percent_re.search(cmd):
                unsafe.append(f"{name}:{lineno}: inline shell contains unescaped % and will be rewritten by systemd")

if unsafe:
    raise SystemExit("\n".join(unsafe))
PYEOF
}

validate_release_index_against_packages() {
  local release_dir="$1"
  python3 - "${release_dir}/release-index.json" "${release_dir}/packages" "${REGISTRY_YAML}" <<'PYEOF'
import glob, hashlib, json, os, sys
import yaml

index_path, pkg_dir, registry_path = sys.argv[1:]
with open(index_path, "r", encoding="utf-8") as f:
    idx = json.load(f)
with open(registry_path, "r", encoding="utf-8") as f:
    registry = yaml.safe_load(f) or {}
entries = idx.get("packages", [])
files = sorted(glob.glob(os.path.join(pkg_dir, "*.tgz")))
if len(entries) != len(files):
    print(f"release-index count mismatch: {len(entries)} entries vs {len(files)} package files", file=sys.stderr)
    sys.exit(1)
by_name = {p.get("name"): p for p in entries}
if len(by_name) != len(entries):
    print("release-index contains duplicate package names", file=sys.stderr)
    sys.exit(1)
registry_names = sorted(pkg.get("name") for pkg in (registry.get("packages") or []) if pkg.get("name"))
index_names = sorted(by_name.keys())
missing = [name for name in registry_names if name not in by_name]
extra = [name for name in index_names if name not in registry_names]
if missing:
    print(f"release-index is missing registry packages: {', '.join(missing)}", file=sys.stderr)
    sys.exit(1)
if extra:
    print(f"release-index contains packages not present in registry.yaml: {', '.join(extra)}", file=sys.stderr)
    sys.exit(1)
for path in files:
    raw = open(path, "rb").read()
    digest = "sha256:" + hashlib.sha256(raw).hexdigest()
    base = os.path.basename(path).rsplit("_", 3)[0]
    pkg = by_name.get(base)
    if not pkg:
        print(f"package file {os.path.basename(path)} missing from release-index", file=sys.stderr)
        sys.exit(1)
    if str(pkg.get("artifact_sha256", "")).strip() != digest:
        print(f"artifact_sha256 mismatch for {base}: index={pkg.get('artifact_sha256')} actual={digest}", file=sys.stderr)
        sys.exit(1)
PYEOF
}

generate_release_index() {
  local pkg_dir="$1" out="$2" provenance_file="$3"
  python3 - "${pkg_dir}" "${out}" "${VERSION}" "${REGISTRY_YAML}" "${provenance_file}" "${PACKAGE_PUBLISHER}" <<'PYEOF'
import glob, hashlib, json, os, sys, tarfile
from datetime import datetime, timezone
import yaml

pkg_dir, out_path, version, registry_path, provenance_path, publisher = sys.argv[1:]
with open(registry_path, "r", encoding="utf-8") as f:
    registry = yaml.safe_load(f) or {}
reg_by_name = {pkg["name"]: pkg for pkg in registry.get("packages", [])}

prov = {}
with open(provenance_path, "r", encoding="utf-8") as f:
    for line in f:
        line = line.rstrip("\n")
        if not line:
            continue
        name, prov_class, origin_release, source_path = line.split("\t", 3)
        prov[name] = {
            "provenance_class": prov_class,
            "origin_release": origin_release,
            "source_path": source_path,
        }

entries = []
for tgz_path in sorted(glob.glob(os.path.join(pkg_dir, "*.tgz"))):
    with tarfile.open(tgz_path, "r:gz") as tf:
        member = None
        for cand in ("package.json", "./package.json"):
            try:
                member = tf.getmember(cand)
                break
            except KeyError:
                continue
        if member is None:
            raise SystemExit(f"{tgz_path}: package.json not found")
        pkg_json = json.loads(tf.extractfile(member).read())
    name = pkg_json.get("name") or os.path.basename(tgz_path).rsplit("_", 3)[0]
    reg = reg_by_name.get(name)
    if not reg:
        raise SystemExit(f"release package {name} not present in registry.yaml")
    raw = open(tgz_path, "rb").read()
    digest = "sha256:" + hashlib.sha256(raw).hexdigest()
    p = prov.get(name)
    if not p:
        raise SystemExit(f"missing provenance entry for {name}")
    entries.append({
        "name": name,
        "kind": str(reg.get("kind", pkg_json.get("type", ""))).lower(),
        "version": pkg_json.get("version", ""),
        # Repository-admission identity is NEVER pre-filled by the release
        # build. Admission derives deterministic pins from artifact_sha256
        # (docs/design/package-identity-single-authority.md).
        "build_number": 0,
        "build_id": "",
        "platform": pkg_json.get("platform", "linux_amd64"),
        "publisher": pkg_json.get("publisher", reg.get("publisher_id", "core@globular.io")),
        "publisher_id": pkg_json.get("publisher", reg.get("publisher_id", "core@globular.io")),
        "entrypoint_checksum": pkg_json.get("entrypoint_checksum", ""),
        "artifact_sha256": digest,
        "package_digest": digest,
        "package_contract_digest": digest,
        "filename": os.path.basename(tgz_path),
        "asset_url": f"packages/{os.path.basename(tgz_path)}",
        "profiles": reg.get("profiles", []),
        "origin_release": p["origin_release"],
        "changed_in_release": p["provenance_class"] != "explicit-carry-forward",
        "provenance_class": p["provenance_class"],
        "source_path": p["source_path"],
    })

idx = {
    "schema_version": "globular.repository.index/v2",
    "platform_release": version,
    "release_tag": f"v{version}",
    "publisher": publisher,
    "generated_at": datetime.now(timezone.utc).isoformat(),
    "package_digest_algorithm": "sha256",
    "packages": entries,
}
with open(out_path, "w", encoding="utf-8") as f:
    json.dump(idx, f, indent=2)
PYEOF
}

# validate_release_identity_gate <release_dir> — the single-authority gate:
# no pre-admission repository identity anywhere in the bundle index; current-
# release package versions match the committed source authority.
validate_release_identity_gate() {
  local release_dir="$1"
  python3 - "${release_dir}/release-index.json" "${release_dir}/packages" "${PACKAGE_VERSIONS_FILE}" "${PACKAGE_VERSION_SUFFIX}" <<'PYEOF'
import json, os, sys, tarfile

index_path, pkg_dir, versions_file, suffix = sys.argv[1:]
with open(index_path, "r", encoding="utf-8") as f:
    idx = json.load(f)

pkg_versions = {}
with open(versions_file, "r", encoding="utf-8") as f:
    for line in f:
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        k, _, v = line.partition("=")
        pkg_versions[k.strip()] = v.strip()

errors = []
for pkg in idx.get("packages", []):
    name = pkg.get("name", "?")
    # 1) Index never carries pre-admission repository identity.
    if str(pkg.get("build_id") or "").strip():
        errors.append(f"{name}: release-index carries pre-admission build_id")
    if int(pkg.get("build_number") or 0) > 0:
        errors.append(f"{name}: release-index carries pre-admission build_number")
    carry_forward = pkg.get("provenance_class") == "explicit-carry-forward"
    # 2) Current-release service packages: version equals committed authority.
    if name in pkg_versions and not carry_forward:
        expected = pkg_versions[name] + suffix
        if str(pkg.get("version") or "") != expected:
            errors.append(f"{name}: version {pkg.get('version')!r} != committed source version {expected!r}")
    # 3) package.json inside current-release artifacts carries no identity.
    if not carry_forward:
        tgz = os.path.join(pkg_dir, pkg.get("filename") or "")
        if os.path.isfile(tgz):
            with tarfile.open(tgz, "r:gz") as tf:
                member = None
                for cand in ("package.json", "./package.json"):
                    try:
                        member = tf.getmember(cand)
                        break
                    except KeyError:
                        continue
                if member is not None:
                    pj = json.loads(tf.extractfile(member).read())
                    if str(pj.get("build_id") or "").strip():
                        errors.append(f"{name}: package.json carries pre-admission build_id")
                    if int(pj.get("build_number") or 0) > 0:
                        errors.append(f"{name}: package.json carries pre-admission build_number")

if errors:
    raise SystemExit("identity gate FAILED:\n  " + "\n  ".join(errors))
print(f"  → identity gate: {len(idx.get('packages', []))} packages clean (no pre-admission identity; versions match committed source)")
PYEOF
}

validate_release_bundle_dir() {
  local release_dir="$1"
  local pkg_dir="${release_dir}/packages"
  local prefix tgz tmpdir entrypoint

  grep -q 'FOUNDING_PROFILES="${FOUNDING_PROFILES:-core,media-server}"' \
    "${release_dir}/scripts/install-day0.sh" \
    || die "release bundle install-day0.sh does not default FOUNDING_PROFILES to core,media-server"
  [[ -f "${release_dir}/release-index.json" ]] || die "release bundle is missing release-index.json"
  python3 "${SERVICES_ROOT}/scripts/validate-day0-package-contract.py" \
    "${release_dir}/scripts/install-day0.sh" "${REGISTRY_YAML}" >/dev/null

  validate_release_index_against_packages "${release_dir}"
  validate_release_identity_gate "${release_dir}"
  bash "${SERVICES_ROOT}/scripts/test-release-bom.sh" "${release_dir}/release-index.json" "${REGISTRY_YAML}" >/dev/null

  while IFS= read -r tgz; do
    [[ -n "${tgz}" ]] || continue
    tmpdir=$(mktemp -d)
    tar -xzf "${tgz}" -C "${tmpdir}"
    entrypoint="$(sed -n 's/.*"entrypoint"[[:space:]]*:[[:space:]]*"bin\/\([^"]*\)".*/\1/p' "${tmpdir}/package.json" | head -1)"
    if [[ -n "${entrypoint}" && -f "${tmpdir}/bin/${entrypoint}" ]] && elf_needs_release_strip "${tmpdir}/bin/${entrypoint}"; then
      rm -rf "${tmpdir}"
      die "release bundle package $(basename "${tgz}") still carries release-forbidden debug sections"
    fi
    rm -rf "${tmpdir}"
  done < <(find "${pkg_dir}" -maxdepth 1 -name '*.tgz' | sort)

  python3 - "${release_dir}/release-index.json" <<'PYEOF'
import json, sys
path = sys.argv[1]
targets = ["node-exporter", "prometheus", "scylla-manager", "scylla-manager-agent", "sctool"]
with open(path, "r", encoding="utf-8") as f:
    idx = json.load(f)
by_name = {p["name"]: p for p in idx.get("packages", [])}
for name in targets:
    pkg = by_name.get(name)
    if not pkg:
        raise SystemExit(f"sensitive package missing from release-index: {name}")
    print(f"  → provenance {name}: {pkg.get('provenance_class')} (origin_release={pkg.get('origin_release')}, file={pkg.get('filename')})")
PYEOF
}

# registry_name_for_target_dir <golang-subdir> — registry package name for a
# services.list target's top-level directory (mirrors gen-package-versions).
registry_name_for_target_dir() {
  local dir="$1"
  case "${dir}" in
    globularcli) echo "globular-cli" ;;
    *) echo "${dir//_/-}" ;;
  esac
}

# ldflags_for <registry-name> — normal lane: NO version injection (the
# committed zz_version_generated.go is the version authority baked at
# compile). Hotfix/local lane (suffix set): the SANCTIONED ldflags escape so
# the binary reports version+suffix.
ldflags_for() {
  local name="$1"
  if [[ -n "${PACKAGE_VERSION_SUFFIX}" ]]; then
    printf '%s\n' "-X main.Version=$(pkg_version_of "${name}") -s -w"
  else
    printf '%s\n' "-s -w"
  fi
}

stage_release_binaries() {
  section "Building Go Services"

  cd "${SERVICES_ROOT}/golang"

  while IFS='|' read -r target output; do
    target="${target%%#*}"; target="${target// /}"
    output="${output// /}"
    [[ -z "${target}" ]] && continue

    bin_name=$(basename "${output}")
    svc_dir="${target#./}"; svc_dir="${svc_dir%%/*}"
    pkg_reg_name="$(registry_name_for_target_dir "${svc_dir}")"
    info "Building ${bin_name} (version $(pkg_version_of "${pkg_reg_name}") from committed source)..."
    go build -trimpath -ldflags "$(ldflags_for "${pkg_reg_name}")" -o "${BIN_STAGE_DIR}/${bin_name}" "${target}"
  done < build/services.list

  cp "${BIN_STAGE_DIR}/globularcli" "${BIN_STAGE_DIR}/globular" 2>/dev/null || true
  cp "${BIN_STAGE_DIR}/mcp" "${BIN_STAGE_DIR}/mcp_server" 2>/dev/null || true

  [[ -d "${GLOBULAR_ROOT}" ]] || die "Globular repo not found at ${GLOBULAR_ROOT} — required to build xds and gateway"
  info "Building xds and gateway from sibling Globular repo..."
  (
    cd "${GLOBULAR_ROOT}"
    go build -trimpath -ldflags "$(ldflags_for xds)" -o "${BIN_STAGE_DIR}/xds" ./cmd/xds
    go build -trimpath -ldflags "$(ldflags_for gateway)" -o "${BIN_STAGE_DIR}/gateway" ./cmd/gateway
  )

  [[ -d "${INSTALLER_ROOT}" ]] || die "globular-installer repo not found at ${INSTALLER_ROOT}"
  info "Validating installer mirrors before building installer binary..."
  make -C "${INSTALLER_ROOT}" check-specs >/dev/null
  info "Building globular-installer from sibling source repo..."
  (
    cd "${INSTALLER_ROOT}"
    installer_cache_dir="$(mktemp -d)"
    trap 'rm -rf "${installer_cache_dir}"' EXIT
    GOCACHE="${installer_cache_dir}" go build -buildvcs=false -trimpath -o "${INSTALLER_STAGE_BIN}" ./cmd/globular-installer
  )

  ok "$(ls "${BIN_STAGE_DIR}/" | wc -l) staged release binaries built"
  rm -f "${BIN_STAGE_DIR}/compute_server" "${BIN_STAGE_DIR}/discovery_server"
  cd "${SERVICES_ROOT}"
}

section "Building Release ${VERSION}"

rm -rf "${DIST_DIR}"
mkdir -p "${BIN_STAGE_DIR}" "${PKG_STAGE_DIR}"

if (( FULL_REGENERATE )); then
  stage_release_binaries
  info "Running full regeneration for services/generated release inputs..."
  bash "${SERVICES_ROOT}/scripts/regenerate-release-inputs.sh" --version "${VERSION}" --bin-dir "${BIN_STAGE_DIR}"
  REUSE_GENERATED_RELEASE_INPUTS=1
fi

# ── Build Go binaries into release staging ───────────────────────────────────
if (( ! REUSE_GENERATED_RELEASE_INPUTS )); then
  stage_release_binaries
fi

# ── Create service packages ──────────────────────────────────────────────────
section "Creating Service Packages"

declare -A BIN_MAP=(
  [authentication]=authentication_server
  [backup-manager]=backup_manager_server
  [blog]=blog_server
  [catalog]=catalog_server
  [cluster-controller]=cluster_controller_server
  [cluster-doctor]=cluster_doctor_server
  [conversation]=conversation_server
  [dns]=dns_server
  [echo]=echo_server
  [event]=event_server
  [file]=file_server
  [ldap]=ldap_server
  [log]=log_server
  [mail]=mail_server
  [media]=media_server
  [monitoring]=monitoring_server
  [node-agent]=node_agent_server
  [persistence]=persistence_server
  [rbac]=rbac_server
  [repository]=repository_server
  [resource]=resource_server
  [search]=search_server
  [sql]=sql_server
  [storage]=storage_server
  [title]=title_server
  [torrent]=torrent_server
  [workflow]=workflow_server
  [ai-memory]=ai_memory_server
  [ai-executor]=ai_executor_server
  [ai-watcher]=ai_watcher_server
  [ai-router]=ai_router_server
  [globular-cli]=globular
  [mcp]=mcp_server
  [xds]=xds
  [gateway]=gateway
)

SOURCE_PACKAGE_DIRS_RAW="$(collect_source_package_dirs)" || die "no source packages found; expected services/generated/*.tgz, packages/dist/*.tgz, or an explicitly allowed extracted bundle source"
mapfile -t SOURCE_PACKAGE_DIRS <<< "${SOURCE_PACKAGE_DIRS_RAW}"
info "Using source packages from:"
for dir in "${SOURCE_PACKAGE_DIRS[@]}"; do
  info "  - ${dir}"
done

GENERATED_PKG_DIR="${SERVICES_ROOT}/generated"
EXTRACTED_SOURCE_RELEASE_DIR=""
for dir in "${SOURCE_PACKAGE_DIRS[@]}"; do
  if [[ "$(classify_source_dir "${dir}")" == "explicit-carry-forward" ]]; then
    EXTRACTED_SOURCE_RELEASE_DIR="$(cd "${dir}/.." && pwd)"
    break
  fi
done
PROVENANCE_FILE="$(mktemp)"
trap 'rm -f "${PROVENANCE_FILE}"' EXIT

# NO local identity minting. build_id and build_number are repository-admission
# identity (assigned at Day-0 seed / steady-state upload). The release build
# ships packages exactly as their authority produced them:
#   - external/infra packages: byte-identical copies from packages/dist
#   - service packages: built ONCE by pkg build with the committed version
# There is no stamp_package_identity step — deleting compensation, not
# improving it (see docs/design/package-identity-single-authority.md).

# assert_no_local_identity <pkg.tgz> — release gate: bundle packages must not
# carry pre-admission repository identity.
assert_no_local_identity() {
  local pkg="$1"
  python3 - "${pkg}" <<'PYEOF'
import json, sys, tarfile
pkg = sys.argv[1]
with tarfile.open(pkg, "r:gz") as tf:
    member = None
    for cand in ("package.json", "./package.json"):
        try:
            member = tf.getmember(cand)
            break
        except KeyError:
            continue
    if member is None:
        raise SystemExit(f"{pkg}: package.json not found")
    data = json.loads(tf.extractfile(member).read())
build_id = str(data.get("build_id") or "").strip()
build_number = int(data.get("build_number") or 0)
if build_id:
    raise SystemExit(f"{pkg}: carries pre-admission build_id={build_id!r} — repository admission is the only build_id authority")
if build_number > 0:
    raise SystemExit(f"{pkg}: carries pre-admission build_number={build_number} — repository admission is the only build_number authority")
PYEOF
}

sync_policy_payload() {
  local pkg_root="$1" pkg_name="$2"
  local policy_dir
  [[ -n "${pkg_name}" ]] || return 0
  policy_dir="${SERVICES_ROOT}/generated/policy/${pkg_name//-/_}"
  [[ -d "${policy_dir}" ]] || return 0
  mkdir -p "${pkg_root}/policy"
  for policy_file in permissions.generated.json roles.generated.json; do
    if [[ -f "${policy_dir}/${policy_file}" ]]; then
      cp -a "${policy_dir}/${policy_file}" "${pkg_root}/policy/${policy_file}"
    fi
  done
}

copied_external=0
declare -A seen_external=()
for dir in "${SOURCE_PACKAGE_DIRS[@]}"; do
  source_class="$(classify_source_dir "${dir}")"
  if [[ "${source_class}" == "generated-current-release" ]]; then
    continue
  fi
  for src_pkg in "${dir}"/*.tgz; do
    [[ -e "${src_pkg}" ]] || continue
    pkg_name="$(extract_package_field "${src_pkg}" name)"
    [[ -n "${pkg_name}" ]] || die "cannot determine package name for ${src_pkg}"
    if [[ -n "${BIN_MAP[${pkg_name}]+x}" ]]; then
      continue
    fi
    if [[ -n "${seen_external[${pkg_name}]+x}" ]]; then
      die "duplicate external package candidate for ${pkg_name}: ${seen_external[${pkg_name}]} and ${src_pkg}"
    fi
    case "${source_class}" in
      input-built-current-release)
        validate_input_built_artifact "${src_pkg}"
        ;;
      explicit-carry-forward)
        validate_carry_forward_artifact "${src_pkg}" "${dir}"
        ;;
      *)
        die "unexpected external package source class ${source_class} for ${src_pkg}"
        ;;
    esac
    # External/infra packages pass through BYTE-IDENTICAL — their version is
    # externally owned (packages repo) and identity is repository-admission
    # business. No restamping, no re-compression.
    staged_pkg="${PKG_STAGE_DIR}/${pkg_name}_$(extract_package_field "${src_pkg}" version)_linux_amd64.tgz"
    cp "${src_pkg}" "${staged_pkg}"
    validate_package_payload_sanity "${staged_pkg}"
    validate_package_systemd_units "${staged_pkg}" || die "unsafe systemd unit content detected in $(basename "${staged_pkg}")"
    if [[ "${source_class}" != "explicit-carry-forward" ]]; then
      assert_no_local_identity "${staged_pkg}"
    fi
    if [[ "${source_class}" == "explicit-carry-forward" ]]; then
      origin_release="$(python3 - "${dir}/../release-index.json" "${pkg_name}" <<'PYEOF'
import json, sys
path, name = sys.argv[1:]
with open(path, "r", encoding="utf-8") as f:
    idx = json.load(f)
for pkg in idx.get("packages", []):
    if pkg.get("name") == name:
        print(pkg.get("origin_release") or "")
        sys.exit(0)
print("")
PYEOF
)"
    else
      origin_release="v${VERSION}"
    fi
    printf '%s\t%s\t%s\t%s\n' "${pkg_name}" "${source_class}" "${origin_release}" "${staged_pkg}" >> "${PROVENANCE_FILE}"
    seen_external["${pkg_name}"]="${src_pkg}"
    copied_external=$((copied_external + 1))
  done
done

mapfile -t EXPECTED_EXTERNAL_PACKAGES < <(list_external_registry_packages)
missing_external=()
for pkg_name in "${EXPECTED_EXTERNAL_PACKAGES[@]}"; do
  [[ -n "${seen_external[${pkg_name}]+x}" ]] && continue
  missing_external+=("${pkg_name}")
done
if (( ${#missing_external[@]} > 0 )); then
  die "missing source package(s) for external registry packages: ${missing_external[*]} — rebuild packages/dist (for wrapper packages like sctool, ensure packages/build.sh runs after metadata changes)"
fi
ok "${copied_external} external/unchanged packages copied"

pkg_count=0
for pkg_name in "${!BIN_MAP[@]}"; do
  bin_name="${BIN_MAP[${pkg_name}]}"

  src_pkg="$(find_source_package "${pkg_name}" "${GENERATED_PKG_DIR}" || true)"
  if [[ -z "${src_pkg}" ]]; then
    if (( REUSE_GENERATED_RELEASE_INPUTS )); then
      die "missing generated source package template for ${pkg_name} in ${GENERATED_PKG_DIR}; full regeneration should have produced it"
    fi
    bin_path="${BIN_STAGE_DIR}/${bin_name}"
    [[ -f "${bin_path}" ]] || die "missing built binary for ${pkg_name}: expected ${bin_path}"
    info "Synthesizing package template for ${pkg_name} from canonical metadata..."
    src_pkg="$(ensure_generated_source_package_template "${pkg_name}" "${bin_name}")"
  fi
  [[ -n "${src_pkg}" ]] || die "missing generated source package template for ${pkg_name} in ${GENERATED_PKG_DIR}"

  pkg_ver="$(pkg_version_of "${pkg_name}")"
  info "Packaging ${pkg_name} v${pkg_ver} (committed source version)..."
  out_file="${PKG_STAGE_DIR}/${pkg_name}_${pkg_ver}_linux_amd64.tgz"

  if (( REUSE_GENERATED_RELEASE_INPUTS )); then
    # Regenerated templates are already complete (built by pkg build with the
    # committed version, policy payload, and entrypoint checksum). Pure copy —
    # no identity mutation, no re-compression.
    cp "${src_pkg}" "${out_file}"
    validate_package_systemd_units "${out_file}" || die "unsafe systemd unit content detected in $(basename "${out_file}")"
    assert_no_local_identity "${out_file}"
    printf '%s\t%s\t%s\t%s\n' "${pkg_name}" "generated-current-release" "v${VERSION}" "${src_pkg}" >> "${PROVENANCE_FILE}"
    pkg_count=$((pkg_count + 1))
    continue
  fi

  bin_path="${BIN_STAGE_DIR}/${bin_name}"
  [[ -f "${bin_path}" ]] || die "missing built binary for ${pkg_name}: expected ${bin_path}"

  tmpdir=$(mktemp -d)
  tar -C "${tmpdir}" -xf "${src_pkg}" --exclude='bin/*'
  sync_policy_payload "${tmpdir}" "${pkg_name}"
  mkdir -p "${tmpdir}/bin"
  cp "${bin_path}" "${tmpdir}/bin/${bin_name}"
  chmod 755 "${tmpdir}/bin/${bin_name}"

  CHECKSUM="sha256:$(sha256sum "${bin_path}" | awk '{print $1}')"

  # Packaging writes package.json ONCE, before the single compression:
  # version copied from committed source authority, entrypoint_checksum is
  # payload truth computed here. Repository-admission identity (build_id /
  # build_number) is actively stripped — never minted locally.
  python3 - "${tmpdir}/package.json" "${pkg_ver}" "${CHECKSUM}" "${PACKAGE_PUBLISHER}" <<'PYEOF'
import json, sys
path, version, checksum, publisher = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4]
with open(path) as f:
    d = json.load(f)
d['version'] = version
d['publisher'] = publisher
d['entrypoint_checksum'] = checksum
d.pop('build_number', None)
d.pop('build_id', None)
with open(path, 'w') as f:
    json.dump(d, f, indent=2)
PYEOF
  tar -C "${tmpdir}" -czf "${out_file}" .
  rm -rf "${tmpdir}"
  validate_package_systemd_units "${out_file}" || die "unsafe systemd unit content detected in $(basename "${out_file}")"
  assert_no_local_identity "${out_file}"
  printf '%s\t%s\t%s\t%s\n' "${pkg_name}" "generated-current-release" "v${VERSION}" "${src_pkg}" >> "${PROVENANCE_FILE}"
  pkg_count=$((pkg_count + 1))
done

ok "${pkg_count} packages created"

# ── Assemble release tarball ─────────────────────────────────────────────────
section "Assembling Release Tarball"

RELEASE_NAME="globular-${VERSION}-linux-amd64"
RELEASE_DIR="${DIST_DIR}/${RELEASE_NAME}"

mkdir -p "${RELEASE_DIR}/packages"
mkdir -p "${RELEASE_DIR}/scripts" "${RELEASE_DIR}/workflows"

cp "${BIN_STAGE_DIR}/globular"   "${RELEASE_DIR}/globular"
chmod 755 "${RELEASE_DIR}/globular"

[[ -x "${INSTALLER_STAGE_BIN}" ]] || die "release-staged installer binary missing at ${INSTALLER_STAGE_BIN}"
cp "${INSTALLER_STAGE_BIN}" "${RELEASE_DIR}/globular-installer"
chmod 755 "${RELEASE_DIR}/globular-installer"

cp "${PKG_STAGE_DIR}/"*.tgz "${RELEASE_DIR}/packages/"
cp "${SCRIPT_DIR}/install.sh"   "${RELEASE_DIR}/install.sh"
chmod +x "${RELEASE_DIR}/install.sh"

if [[ -d "${INSTALLER_ROOT}/scripts" ]]; then
  cp -a "${INSTALLER_ROOT}/scripts/." "${RELEASE_DIR}/scripts/"
elif [[ -n "${EXTRACTED_SOURCE_RELEASE_DIR}" && -d "${EXTRACTED_SOURCE_RELEASE_DIR}/scripts" ]]; then
  cp -a "${EXTRACTED_SOURCE_RELEASE_DIR}/scripts/." "${RELEASE_DIR}/scripts/"
else
  die "installer scripts not found"
fi

if [[ -d "${SERVICES_ROOT}/scripts/release" ]]; then
  cp -a "${SERVICES_ROOT}/scripts/release/." "${RELEASE_DIR}/scripts/"
fi
chmod +x "${RELEASE_DIR}/scripts/"*.sh 2>/dev/null || true

cp "${SERVICES_ROOT}/golang/workflow/definitions/"*.yaml "${RELEASE_DIR}/workflows/"

# services/webroot is the authored release webroot source. When assembling
# from a previously extracted release bundle, re-use that bundled copy.
if [[ -d "${SERVICES_ROOT}/webroot" ]]; then
  cp -a "${SERVICES_ROOT}/webroot" "${RELEASE_DIR}/webroot"
elif [[ -n "${EXTRACTED_SOURCE_RELEASE_DIR}" && -d "${EXTRACTED_SOURCE_RELEASE_DIR}/webroot" ]]; then
  cp -a "${EXTRACTED_SOURCE_RELEASE_DIR}/webroot" "${RELEASE_DIR}/webroot"
fi

# docs/operational-knowledge is the ops-knowledge corpus that seeds ai-memory on
# the installed node. It ships in the bundle (matching prior releases) so the
# target can (re)generate ai-memory from the authored source, not a stale copy.
if [[ -d "${SERVICES_ROOT}/docs/operational-knowledge" ]]; then
  mkdir -p "${RELEASE_DIR}/docs"
  cp -a "${SERVICES_ROOT}/docs/operational-knowledge" "${RELEASE_DIR}/docs/operational-knowledge"
elif [[ -n "${EXTRACTED_SOURCE_RELEASE_DIR}" && -d "${EXTRACTED_SOURCE_RELEASE_DIR}/docs/operational-knowledge" ]]; then
  mkdir -p "${RELEASE_DIR}/docs"
  cp -a "${EXTRACTED_SOURCE_RELEASE_DIR}/docs/operational-knowledge" "${RELEASE_DIR}/docs/operational-knowledge"
fi

generate_release_index "${RELEASE_DIR}/packages" "${RELEASE_DIR}/release-index.json" "${PROVENANCE_FILE}"
(cd "${RELEASE_DIR}/packages" && sha256sum *.tgz > SHA256SUMS)
validate_release_bundle_dir "${RELEASE_DIR}"

cat > "${RELEASE_DIR}/README.md" <<HEREDOC
# Globular ${VERSION}

## Install

\`\`\`bash
sudo bash install.sh
\`\`\`

The first node always comes up with the quorum profiles plus the media workload
profile (\`control-plane\`, \`core\`, \`storage\`, \`media-server\`). To
override or extend the day-0 workload profiles, pass \`FOUNDING_PROFILES\`
(comma-separated) through \`sudo\`:

\`\`\`bash
sudo FOUNDING_PROFILES=core,media-server bash install.sh
\`\`\`

## Next Steps

\`\`\`bash
sudo systemctl start globular-node-agent
globular cluster bootstrap --node <routable-node-ip>:11000 --domain <your-domain> --profile core --profile media-server --profile gateway
\`\`\`

Full guide: https://globular.io/docs/operators/installation
HEREDOC

cd "${DIST_DIR}"
tar czf "${RELEASE_NAME}.tar.gz" "${RELEASE_NAME}/"
sha256sum "${RELEASE_NAME}.tar.gz" > "${RELEASE_NAME}.tar.gz.sha256"

EXTRACT_VALIDATE_DIR="$(mktemp -d)"
tar xzf "${RELEASE_NAME}.tar.gz" -C "${EXTRACT_VALIDATE_DIR}"
validate_release_bundle_dir "${EXTRACT_VALIDATE_DIR}/${RELEASE_NAME}"
rm -rf "${EXTRACT_VALIDATE_DIR}"

section "Done"
echo "Release tarball: ${DIST_DIR}/${RELEASE_NAME}.tar.gz"
echo "Size:            $(du -sh "${DIST_DIR}/${RELEASE_NAME}.tar.gz" | cut -f1)"
echo "Packages:        ${pkg_count}"
echo ""
echo "Contents:"
tar tzf "${DIST_DIR}/${RELEASE_NAME}.tar.gz" | head -10 || true
echo "  ..."
echo ""
echo "To test the installer:"
echo "  mkdir /tmp/globular-test && tar xzf ${DIST_DIR}/${RELEASE_NAME}.tar.gz -C /tmp/globular-test"
echo "  sudo bash /tmp/globular-test/${RELEASE_NAME}/install.sh"
