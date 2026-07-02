#!/usr/bin/env bash
# build-local-release.sh — Build a local release bundle identical in structure
# to what CI (release.yml) produces, without downloading from the internet.
#
# Usage:
#   bash scripts/build-local-release.sh --version 1.2.139
#   bash scripts/build-local-release.sh --version 1.2.139 --prev /tmp/globular-1.2.138-linux-amd64.tar.gz
#   bash scripts/build-local-release.sh --version 1.2.139 --rebuild node-agent,cluster-doctor
#
# Output: /tmp/globular-<version>-linux-amd64.tar.gz  (+.sha256)
#
# Strategy (mirrors CI release.yml):
#   1. Extract infra binaries from previous bundle packages (no internet needed)
#   2. Build changed Go services with real ldflags (-X main.Version=...)
#   3. Rebuild changed packages via packages/build.sh pattern
#   4. Copy unchanged packages from previous bundle
#   5. Generate release-index.json
#   6. Assemble bundle: globular, globular-installer, scripts/, webroot/,
#      workflows/, docs/, packages/, release-index.json
#   7. Pack tarball + sha256
#
# Repos assumed at same level as services/:
#   ../packages/   — globulario/packages
#   ../Globular/   — globulario/Globular (for xds/gateway)
#
set -euo pipefail

SERVICES_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PACKAGES_ROOT="$(cd "$SERVICES_ROOT/../packages" 2>/dev/null && pwd)" || { echo "ERROR: ../packages not found"; exit 1; }
GLOBULAR_ROOT="$(cd "$SERVICES_ROOT/../Globular" 2>/dev/null && pwd)" || GLOBULAR_ROOT=""

# ── Args ──────────────────────────────────────────────────────────────────────
VERSION=""
PREV_TGZ=""
REBUILD_PKGS=""     # comma-separated list; empty = auto-detect via git diff

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)   VERSION="$2";      shift 2 ;;
    --prev)      PREV_TGZ="$2";     shift 2 ;;
    --rebuild)   REBUILD_PKGS="$2"; shift 2 ;;
    *) echo "Unknown arg: $1" >&2; exit 1 ;;
  esac
done

[[ -z "$VERSION" ]] && { echo "Usage: $0 --version X.Y.Z [--prev <path>.tar.gz] [--rebuild pkg1,pkg2]" >&2; exit 1; }

OUT_TGZ="/tmp/globular-${VERSION}-linux-amd64.tar.gz"
DIST_DIR="/tmp/globular-${VERSION}-linux-amd64"
WORK="$(mktemp -d /tmp/globular-build-XXXXXX)"
trap 'rm -rf "$WORK"' EXIT

BIN_DIR="$WORK/bin"
PKG_OUT="$WORK/packages"
mkdir -p "$BIN_DIR" "$PKG_OUT"

# install-day0.sh reads packages from internal/assets/packages/
BUNDLE_PKG_DIR="$DIST_DIR/internal/assets/packages"

log()  { echo "  $*"; }
step() { echo ""; echo "=== $* ==="; }

elf_needs_release_strip() {
  local bin="$1"
  [[ -f "$bin" ]] || return 1
  file -b "$bin" 2>/dev/null | grep -q '^ELF' || return 1
  readelf -S "$bin" 2>/dev/null | grep -Eq '\.(debug_|zdebug_|symtab)\b'
}

strip_release_binary() {
  local bin="$1" label="${2:-$(basename "$1")}"
  [[ -f "$bin" ]] || return 0
  if ! elf_needs_release_strip "$bin"; then
    return 0
  fi
  if ! command -v strip >/dev/null 2>&1; then
    echo "ERROR: $label contains release-forbidden debug sections and 'strip' is unavailable." >&2
    exit 1
  fi
  log "stripping $label for release-channel packaging"
  strip --strip-debug --strip-unneeded "$bin"
  chmod +x "$bin"
  if elf_needs_release_strip "$bin"; then
    echo "ERROR: $label still contains release-forbidden debug sections after strip." >&2
    exit 1
  fi
}

normalize_release_bin_dir() {
  local bin
  for bin in "$BIN_DIR"/*; do
    [[ -f "$bin" ]] || continue
    strip_release_binary "$bin" "bin/$(basename "$bin")"
  done
}

validate_release_bundle() {
  local release_dir="$1"
  local pkg_dir="${release_dir}/packages"
  local prefix tgz tmpdir entrypoint

  grep -q 'FOUNDING_PROFILES="${FOUNDING_PROFILES:-core}"' \
    "${release_dir}/scripts/install-day0.sh" \
    || { echo "ERROR: bundled install-day0.sh does not default FOUNDING_PROFILES to core." >&2; exit 1; }
  python3 "${SERVICES_ROOT}/scripts/validate-day0-package-contract.py" \
    "${release_dir}/scripts/install-day0.sh" "${PACKAGES_ROOT}/registry.yaml" >/dev/null

  for prefix in prometheus node-exporter scylla-manager scylla-manager-agent sctool; do
    tgz=$(find "${pkg_dir}" -maxdepth 1 -name "${prefix}_*_linux_amd64.tgz" | sort -V | tail -1 || true)
    [[ -n "${tgz}" ]] || continue
    tmpdir=$(mktemp -d)
    tar -xzf "${tgz}" -C "${tmpdir}"
    entrypoint="$(sed -n 's/.*"entrypoint"[[:space:]]*:[[:space:]]*"bin\/\([^"]*\)".*/\1/p' "${tmpdir}/package.json" | head -1)"
    if [[ -n "${entrypoint}" && -f "${tmpdir}/bin/${entrypoint}" ]] && elf_needs_release_strip "${tmpdir}/bin/${entrypoint}"; then
      rm -rf "${tmpdir}"
      echo "ERROR: bundled package $(basename "${tgz}") still contains release-forbidden debug sections." >&2
      exit 1
    fi
    rm -rf "${tmpdir}"
  done
}

repack_previous_package_if_needed() {
  local tgz="$1" fname="$2"
  local tmpdir="$WORK/repack-${fname%.tgz}"
  local entrypoint checksum
  mkdir -p "$tmpdir"
  tar -xzf "$tgz" -C "$tmpdir"
  entrypoint="$(sed -n 's/.*"entrypoint"[[:space:]]*:[[:space:]]*"bin\/\([^"]*\)".*/\1/p' "$tmpdir/package.json" | head -1)"
  if [[ -z "$entrypoint" || ! -f "$tmpdir/bin/$entrypoint" ]]; then
    cp "$tgz" "$PKG_OUT/$fname"
    rm -rf "$tmpdir"
    return 0
  fi
  if ! elf_needs_release_strip "$tmpdir/bin/$entrypoint"; then
    cp "$tgz" "$PKG_OUT/$fname"
    rm -rf "$tmpdir"
    return 0
  fi

  strip_release_binary "$tmpdir/bin/$entrypoint" "${fname}:${entrypoint}"
  checksum=$(sha256sum "$tmpdir/bin/$entrypoint" | awk '{print $1}')
  python3 - "$tmpdir/package.json" "$BUILD_NUMBER" "sha256:$checksum" <<'PYEOF'
import json, sys, uuid
path, build_number, checksum = sys.argv[1:]
data = json.load(open(path))
data["build_id"] = str(uuid.uuid4())
data["build_number"] = int(build_number)
data["entrypoint_checksum"] = checksum
json.dump(data, open(path, "w"), indent=2)
PYEOF
  tar -C "$tmpdir" -czf "$PKG_OUT/$fname" .
  rm -rf "$tmpdir"
  log "repacked inherited package with stripped entrypoint: $fname"
}

# ── Find previous bundle ──────────────────────────────────────────────────────
step "Locate previous bundle"
if [[ -z "$PREV_TGZ" ]]; then
  PREV_TGZ=$(ls /tmp/globular-*.tar.gz 2>/dev/null | sort -V | grep -v "${VERSION}" | tail -1 || true)
fi
[[ -z "$PREV_TGZ" || ! -f "$PREV_TGZ" ]] && { echo "ERROR: No previous bundle found. Pass --prev <path>." >&2; exit 1; }
log "Using previous bundle: $PREV_TGZ"

PREV_DIR="$WORK/prev"
mkdir -p "$PREV_DIR"
tar -xzf "$PREV_TGZ" -C "$PREV_DIR" --strip-components=1
PREV_INDEX="$PREV_DIR/release-index.json"
[[ -f "$PREV_INDEX" ]] || { echo "ERROR: No release-index.json in previous bundle" >&2; exit 1; }
PREV_VERSION=$(python3 -c "import json; print(json.load(open('$PREV_INDEX'))['platform_release'])")
log "Previous version: $PREV_VERSION"

# ── Extract infra binaries from previous packages ─────────────────────────────
step "Extract infra binaries from previous bundle"
INFRA_PKGS=(etcd etcdctl envoy minio mc prometheus alertmanager node_exporter
            sidekick restic rclone sha256sum yt-dlp ffmpeg
            scylla_manager scylla_manager_agent sctool noop)

for tgz in "$PREV_DIR/packages/"*.tgz; do
  [[ -f "$tgz" ]] || continue
  # Skip extracted dev sidecars (oxigraph, awareness-graph) — not cluster
  # packages since the AWG extraction; don't pull their binaries into bin/.
  case "$(basename "$tgz")" in
    oxigraph_*|awareness-graph_*) continue ;;
  esac
  TMPX="$WORK/extract-$(basename "$tgz" .tgz)"
  mkdir -p "$TMPX"
  tar -xzf "$tgz" -C "$TMPX" ./bin/ 2>/dev/null || true
  for bin in "$TMPX/bin/"*; do
    [[ -f "$bin" ]] || continue
    bname="$(basename "$bin")"
    [[ ! -f "$BIN_DIR/$bname" ]] && { cp "$bin" "$BIN_DIR/$bname"; chmod +x "$BIN_DIR/$bname"; log "  binary: $bname"; }
  done
  rm -rf "$TMPX"
done

# Also grab globular-installer and globular from prev bundle root
for f in globular globular-installer; do
  [[ -f "$PREV_DIR/$f" && ! -f "$BIN_DIR/$f" ]] && { cp "$PREV_DIR/$f" "$BIN_DIR/$f"; chmod +x "$BIN_DIR/$f"; }
done
normalize_release_bin_dir

# ── Determine which packages to rebuild ───────────────────────────────────────
step "Determine changed packages"
PREV_TAG="v${PREV_VERSION}"
FORCED_CHANGED_FILE="$WORK/forced-packages.txt"

if [[ -n "$REBUILD_PKGS" ]]; then
  IFS=',' read -ra CHANGED_NAMES <<< "$REBUILD_PKGS"
  log "Manual rebuild list: ${CHANGED_NAMES[*]}"
else
  # Auto-detect via git diff against previous tag
  CHANGED_NAMES=()
  if git -C "$SERVICES_ROOT" rev-parse "$PREV_TAG" >/dev/null 2>&1; then
    CHANGED_FILES=$(git -C "$SERVICES_ROOT" diff --name-only "$PREV_TAG" HEAD 2>/dev/null)
    PKG_SPEC_CHANGES=$(git -C "$PACKAGES_ROOT" diff --name-only "$PREV_TAG" HEAD 2>/dev/null || \
                       git -C "$PACKAGES_ROOT" diff --name-only HEAD~1 HEAD 2>/dev/null || true)
  else
    log "Tag $PREV_TAG not found — treating all Go services as changed"
    CHANGED_FILES="golang/"
    PKG_SPEC_CHANGES=""
  fi

  PKG_MAP="$SERVICES_ROOT/golang/build/pkg-map.json"
  # Map changed go source dirs → package names
  while IFS='|' read -r go_target pkg_name; do
    go_target="${go_target#./}"
    if echo "$CHANGED_FILES" | grep -q "golang/${go_target%%/*}/"; then
      CHANGED_NAMES+=("$pkg_name")
    fi
  done < <(python3 -c "
import json
d=json.load(open('$PKG_MAP'))
for name,info in d.items():
    t=info.get('go_target','')
    if t: print(f'{t}|{name}')
")

  # Also detect changed package specs
  while IFS= read -r f; do
    spec_name=$(basename "$f" | sed 's/_service\.yaml$//' | sed 's/_cmd\.yaml$//' | tr '_' '-')
    python3 -c "import json; d=json.load(open('$PKG_MAP')); exit(0 if '$spec_name' in d else 1)" 2>/dev/null && \
      CHANGED_NAMES+=("$spec_name")
  done < <(echo "$PKG_SPEC_CHANGES" | grep "specs/.*\.yaml" || true)

  # Deduplicate
  mapfile -t CHANGED_NAMES < <(printf '%s\n' "${CHANGED_NAMES[@]}" | sort -u)

  bash "$SERVICES_ROOT/scripts/release/cluster-controller-release-guard.sh" detect \
    --services-root "$SERVICES_ROOT" \
    --packages-root "$PACKAGES_ROOT" \
    --prev-tag "$PREV_TAG" \
    --output "$FORCED_CHANGED_FILE" || true

  if [[ -f "$FORCED_CHANGED_FILE" ]]; then
    while IFS='|' read -r forced_name _reason; do
      [[ -n "${forced_name}" ]] || continue
      CHANGED_NAMES+=("$forced_name")
    done < "$FORCED_CHANGED_FILE"
    mapfile -t CHANGED_NAMES < <(printf '%s\n' "${CHANGED_NAMES[@]}" | sort -u)
  fi

  log "Auto-detected changed: ${CHANGED_NAMES[*]:-none}"
fi

# ── Build Go version files ────────────────────────────────────────────────────
step "Generate version files (v${VERSION})"
cd "$SERVICES_ROOT/golang"
bash build/gen-version.sh "$VERSION"

# ── Build changed Go services ─────────────────────────────────────────────────
step "Build changed Go services"
LDFLAGS="-s -w"
PKG_MAP="$SERVICES_ROOT/golang/build/pkg-map.json"

for pkg_name in "${CHANGED_NAMES[@]}"; do
  go_target=$(python3 -c "
import json
d=json.load(open('$PKG_MAP'))
info=d.get('$pkg_name',{})
print(info.get('go_target',''))
" 2>/dev/null)
  [[ -z "$go_target" ]] && continue

  binary=$(python3 -c "
import json
d=json.load(open('$PKG_MAP'))
print(d.get('$pkg_name',{}).get('binary',''))
" 2>/dev/null)
  [[ -z "$binary" ]] && continue

  log "Building $binary ($pkg_name)..."
  go build -trimpath -ldflags "$LDFLAGS" -o "$BIN_DIR/$binary" "./$go_target"
done

# Also build globular CLI if cli changed
if printf '%s\n' "${CHANGED_NAMES[@]}" | grep -q "globular-cli"; then
  cp "$BIN_DIR/globularcli" "$BIN_DIR/globular" 2>/dev/null || true
fi

# Rebuild xds/gateway if Globular repo changed
if [[ -n "$GLOBULAR_ROOT" ]]; then
  for gname in xds gateway; do
    if printf '%s\n' "${CHANGED_NAMES[@]}" | grep -q "^${gname}$"; then
      log "Building $gname..."
      GLDFLAGS="-X main.Version=${VERSION} -X main.BuildVersion=${VERSION} -s -w"
      go build -trimpath -ldflags "$GLDFLAGS" -o "$BIN_DIR/$gname" "$GLOBULAR_ROOT/cmd/$gname"
    fi
  done
fi

# ── Build changed packages ────────────────────────────────────────────────────
step "Build changed packages"
BUILD_NUMBER=$(date +%s)   # local builds use unix timestamp as build_number
BUILD_ID=$(python3 -c "import uuid; print(uuid.uuid4())")

for pkg_name in "${CHANGED_NAMES[@]}"; do
  meta_dir="$PACKAGES_ROOT/metadata/$pkg_name"
  # Single source of truth: metadata/<name>/specs/ (top-level specs/ was removed
  # in the 2026-06 spec consolidation).
  spec_file="$meta_dir/specs/${pkg_name//-/_}_service.yaml"
  [[ -f "$spec_file" ]] || spec_file="$meta_dir/specs/${pkg_name//-/_}_cmd.yaml"

  [[ -d "$meta_dir" ]] || { log "SKIP $pkg_name: no metadata dir"; continue; }
  [[ -f "$spec_file" ]] || { log "SKIP $pkg_name: no spec yaml"; continue; }

  binary=$(python3 -c "
import json
d=json.load(open('$PKG_MAP'))
print(d.get('$pkg_name',{}).get('binary',''))
" 2>/dev/null)
  [[ -z "$binary" ]] && { log "SKIP $pkg_name: not in pkg-map"; continue; }
  [[ -f "$BIN_DIR/$binary" ]] || { log "SKIP $pkg_name: binary $binary not found"; continue; }

  # Determine version
  platform_version=$(python3 -c "
import json
d=json.load(open('$PKG_MAP'))
pv=d.get('$pkg_name',{}).get('platform_version', True)
print('true' if pv is not False else 'false')
")
  if [[ "$platform_version" == "true" ]]; then
    pkg_version="$VERSION"
  else
    pkg_version=$(python3 -c "
import json
d=json.load(open('$meta_dir/package.json'))
print(d.get('version','0.0.0'))
")
  fi

  per_pkg_build_id=$(python3 -c "import uuid; print(uuid.uuid4())")

  TMPROOT="$WORK/root-${pkg_name}"
  rm -rf "$TMPROOT"
  cp -a "$meta_dir" "$TMPROOT"
  mkdir -p "$TMPROOT/bin" "$TMPROOT/specs"
  cp "$BIN_DIR/$binary" "$TMPROOT/bin/$binary"
  chmod +x "$TMPROOT/bin/$binary"
  cp "$spec_file" "$TMPROOT/specs/$(basename "$spec_file")"

  # Stamp package.json
  checksum=$(sha256sum "$TMPROOT/bin/$binary" | awk '{print $1}')
  python3 - "$TMPROOT/package.json" "$pkg_version" "$per_pkg_build_id" "$BUILD_NUMBER" "sha256:$checksum" <<'PYEOF'
import json, sys
path, version, build_id, build_number, checksum = sys.argv[1:]
d = json.load(open(path))
d['version'] = version
d['build_id'] = build_id
d['build_number'] = int(build_number)
d['entrypoint_checksum'] = checksum
json.dump(d, open(path, 'w'), indent=2)
PYEOF

  out_tgz="$PKG_OUT/${pkg_name}_${pkg_version}_linux_amd64.tgz"
  tar -C "$TMPROOT" -czf "$out_tgz" .
  rm -rf "$TMPROOT"
  log "Packaged $pkg_name v${pkg_version}"
done

# ── Copy unchanged packages from previous bundle ──────────────────────────────
step "Copy unchanged packages"
CHANGED_SET=$(printf '%s\n' "${CHANGED_NAMES[@]}")

for tgz in "$PREV_DIR/packages/"*.tgz; do
  [[ -f "$tgz" ]] || continue
  fname="$(basename "$tgz")"
  # Extract package name: strip _<version>_<platform>.tgz (version starts with digit)
  pkg_name=$(python3 -c "
import re, sys
fname = '$fname'
m = re.match(r'^(.+?)_(\d+[\.\d]+.*)_(linux_amd64)\.tgz\$', fname)
print(m.group(1) if m else fname)
")
  # Extracted dev sidecars — NOT cluster packages since the AWG extraction.
  # Mirrors CI release.yml, which intentionally does not ship oxigraph or
  # awareness-graph ("not a cluster package since v1.2.211"). A stale pre-
  # extraction base bundle (e.g. <1.2.211 passed via --prev) still carries them;
  # never carry them forward into a cluster bundle / Day-0 install set.
  case "$pkg_name" in
    oxigraph|awareness-graph)
      log "SKIP (extracted dev sidecar, not a cluster package): $pkg_name"
      continue ;;
  esac
  if echo "$CHANGED_SET" | grep -qx "$pkg_name"; then
    log "SKIP (rebuilt): $pkg_name"
    continue
  fi
  # Don't copy if we already built a new version
  if ls "$PKG_OUT/${pkg_name}_"*.tgz 2>/dev/null | grep -q .; then
    log "SKIP (new version built): $pkg_name"
    continue
  fi
  repack_previous_package_if_needed "$tgz" "$fname"
done
log "$(ls "$PKG_OUT"/*.tgz | wc -l) total packages"

# ── Generate release-index.json ───────────────────────────────────────────────
step "Generate release-index.json"
python3 - "$PKG_OUT" "$PREV_INDEX" "$VERSION" "$BUILD_NUMBER" "$BUILD_ID" <<'PYEOF'
import json, hashlib, tarfile, glob, os, sys
from datetime import datetime, timezone

pkg_out, prev_index_path, version, build_number, build_id = sys.argv[1:]
prev = json.load(open(prev_index_path))
prev_by_name = {p['name']: p for p in prev.get('packages', [])}

entries = []
for tgz_path in sorted(glob.glob(f"{pkg_out}/*.tgz")):
    fname = os.path.basename(tgz_path)
    # Parse name from filename: <name>_<version>_<platform>.tgz
    # The platform is linux_amd64 (it contains an internal underscore), so the
    # bare name is everything before the LAST THREE underscore tokens
    # (version, "linux", "amd64") — rsplit 3, not 2. The package.json name is
    # the authority; the filename parse is only a fallback.
    parts = fname.rsplit('_', 3)
    name = parts[0]

    with tarfile.open(tgz_path) as tf:
        pj = json.loads(tf.extractfile('./package.json').read())
    if pj.get('name'):
        name = pj['name']

    raw = open(tgz_path, 'rb').read()
    pkg_digest = f"sha256:{hashlib.sha256(raw).hexdigest()}"

    prev_entry = prev_by_name.get(name, {})
    changed = (pj['build_id'] != prev_entry.get('build_id', ''))

    entry = {
        'name':                    name,
        'kind':                    pj.get('kind', prev_entry.get('kind', 'service')),
        'version':                 pj['version'],
        'build_number':            pj['build_number'],
        'build_id':                pj['build_id'],
        'platform':                pj.get('platform', 'linux_amd64'),
        'publisher':               pj.get('publisher', 'core@globular.io'),
        'channel':                 pj.get('channel', prev_entry.get('channel', 'stable')),
        'package_contract_digest': prev_entry.get('package_contract_digest', pkg_digest),
        'package_digest':          pkg_digest,
        'entrypoint_checksum':     pj.get('entrypoint_checksum', ''),
        'asset_url':               f"file://{tgz_path}",
        'filename':                fname,
        'origin_release':          f"v{version}",
        'changed_in_release':      changed,
        'profiles':                prev_entry.get('profiles', []),
    }
    entries.append(entry)

index = {
    'schema_version':         2,
    'platform_release':       version,
    'release_tag':            f"v{version}",
    'publisher':              'core@globular.io',
    'generated_at':           datetime.now(timezone.utc).isoformat(),
    'package_digest_algorithm': 'sha256',
    'referenced_releases':    [],
    'force_full_rebuild':     False,
    'force_full_rebuild_reason': '',
    'packages':               entries,
}

out = f"{pkg_out}/../release-index.json"
json.dump(index, open(out, 'w'), indent=2)
print(f"  release-index.json: {len(entries)} packages")
PYEOF

# ── Assemble bundle directory ─────────────────────────────────────────────────
step "Assemble bundle: $DIST_DIR"
rm -rf "$DIST_DIR"
mkdir -p "$BUNDLE_PKG_DIR" "$DIST_DIR/packages" "$DIST_DIR/scripts" "$DIST_DIR/bin"

# globular CLI
if [[ -f "$BIN_DIR/globular" ]]; then
  cp "$BIN_DIR/globular" "$DIST_DIR/globular"
  chmod +x "$DIST_DIR/globular"
elif [[ -f "$PREV_DIR/globular" ]]; then
  cp "$PREV_DIR/globular" "$DIST_DIR/globular"
  chmod +x "$DIST_DIR/globular"
fi

# globular-installer — install-day0.sh looks in bin/ first, then root
if [[ -f "$BIN_DIR/globular-installer" ]]; then
  cp "$BIN_DIR/globular-installer" "$DIST_DIR/bin/globular-installer"
  cp "$BIN_DIR/globular-installer" "$DIST_DIR/globular-installer"
elif [[ -f "$PREV_DIR/globular-installer" ]]; then
  cp "$PREV_DIR/globular-installer" "$DIST_DIR/bin/globular-installer"
  cp "$PREV_DIR/globular-installer" "$DIST_DIR/globular-installer"
fi

# Packages — install-day0.sh reads from internal/assets/packages/
# Also mirror to packages/ (legacy path some scripts use)
cp "$PKG_OUT/"*.tgz "$BUNDLE_PKG_DIR/"
cp "$PKG_OUT/"*.tgz "$DIST_DIR/packages/"

# release-index.json
cp "$WORK/release-index.json" "$DIST_DIR/release-index.json"

# Scripts (use our current source — this is the whole point)
cp -r "$SERVICES_ROOT/scripts/release/"* "$DIST_DIR/scripts/"
# Extra files/dirs from previous bundle scripts/ not present in our source
for f in "$PREV_DIR/scripts/"*; do
  bname="$(basename "$f")"
  [[ -e "$DIST_DIR/scripts/$bname" ]] && continue
  if [[ -d "$f" ]]; then
    cp -r "$f" "$DIST_DIR/scripts/$bname"
  else
    cp "$f" "$DIST_DIR/scripts/$bname"
  fi
done

# install.sh at root
[[ -f "$PREV_DIR/install.sh" ]] && cp "$PREV_DIR/install.sh" "$DIST_DIR/install.sh"

# webroot, workflows, docs from previous bundle
for d in webroot workflows docs; do
  [[ -d "$PREV_DIR/$d" ]] && cp -r "$PREV_DIR/$d" "$DIST_DIR/$d"
done

validate_release_bundle "$DIST_DIR"

# ── Pack tarball ──────────────────────────────────────────────────────────────
step "Pack tarball"
rm -f "$OUT_TGZ" "${OUT_TGZ}.sha256"
tar -C /tmp -czf "$OUT_TGZ" "globular-${VERSION}-linux-amd64/"
sha256sum "$OUT_TGZ" > "${OUT_TGZ}.sha256"

SIZE=$(du -sh "$OUT_TGZ" | awk '{print $1}')
echo ""
echo "╔════════════════════════════════════════════════════════════════╗"
echo "║  ✓ LOCAL RELEASE BUILT                                         ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""
echo "  Bundle : $OUT_TGZ  ($SIZE)"
echo "  SHA256 : $(cat "${OUT_TGZ}.sha256" | awk '{print $1}')"
echo "  Dir    : $DIST_DIR"
echo ""
echo "  To install:"
echo "    sudo bash $DIST_DIR/scripts/install-day0.sh 2>&1 | tee /tmp/day0-test.log"
echo "    echo \"EXIT: \$?\""
