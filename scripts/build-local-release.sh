#!/usr/bin/env bash
# build-local-release.sh — Build a local release bundle identical in structure
# to what CI (release.yml) produces, without downloading from the internet.
#
# Usage:
#   # Fully automated: rebuild ALL Go-service packages, auto-locate a base bundle,
#   # publish to services/dist/ — no other flags or manual setup needed.
#   bash scripts/build-local-release.sh --version 1.2.267 --all
#
#   bash scripts/build-local-release.sh --version 1.2.139 --prev /tmp/globular-1.2.138-linux-amd64.tar.gz
#   bash scripts/build-local-release.sh --version 1.2.139 --rebuild node-agent,cluster-doctor
#   bash scripts/build-local-release.sh --version 1.2.139 --all --out /some/dir
#
# Flags:
#   --version X.Y.Z   (required) release version to stamp
#   --all             rebuild every Go-service package (all pkg-map go_target entries)
#   --rebuild a,b,c   rebuild only these packages (ignored if --all)
#   --prev <tgz>      base bundle for infra carry-forward (auto-located if omitted)
#   --out <dir>       publish destination (default: services/dist/)
#
# Output: <out>/globular-<version>-linux-amd64.tar.gz (+.sha256) and the extracted dir.
#
# Strategy (mirrors CI release.yml), fully self-contained:
#   1. Auto-locate a base bundle (/tmp, ~/, or --out) for infra binaries + xds/
#      gateway + installer — the parts that cannot be rebuilt locally.
#   2. Build changed/all Go services from CURRENT source with real ldflags.
#   3. Package them from ../packages metadata (either metadata/<name>/ or the
#      flattened <name>/ layout — resolve_meta_dir handles both, so NO packages
#      branch switching is required).
#   4. Copy unchanged (infra) packages forward from the base bundle.
#   5. Generate release-index.json; ship current docs/operational-knowledge;
#      regenerate README.md for this version+commit.
#   6. Assemble, hardlink-dedup internal/assets/packages, pack tarball + sha256.
#   7. Publish to services/dist/ (clears root-owned leftovers first).
#
# Repos assumed at same level as services/:
#   ../packages/   — globulario/packages (any branch: layout auto-detected)
#   ../Globular/   — globulario/Globular (for xds/gateway, only if in rebuild set)
#
set -euo pipefail

SERVICES_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PACKAGES_ROOT_DEFAULT="$SERVICES_ROOT/../packages"
PACKAGES_ROOT="${PACKAGES_ROOT_OVERRIDE:-$PACKAGES_ROOT_DEFAULT}"
PACKAGES_ROOT="$(cd "$PACKAGES_ROOT" 2>/dev/null && pwd)" || { echo "ERROR: packages root not found (${PACKAGES_ROOT_OVERRIDE:-$PACKAGES_ROOT_DEFAULT})"; exit 1; }
GLOBULAR_ROOT="$(cd "$SERVICES_ROOT/../Globular" 2>/dev/null && pwd)" || GLOBULAR_ROOT=""
# Stage bin dir consumed by specgen/pkggen (via regenerate-release-inputs.sh) to
# produce the generated per-service package templates. gRPC-service metadata is no
# longer hand-authored in the packages repo — it is generated into services/generated
# from each binary's --describe (spec) + proto AuthzRule annotations (policy). We
# refresh this dir with freshly-built binaries so generated specs reflect current source.
STAGE_BIN="$SERVICES_ROOT/golang/tools/stage/linux-amd64/usr/local/bin"

# ── Args ──────────────────────────────────────────────────────────────────────
VERSION=""
PREV_TGZ=""
REBUILD_PKGS=""     # comma-separated list; empty = auto-detect via git diff
REBUILD_ALL=0       # --all: rebuild every Go-service package (all pkg-map go_target entries)
OUT_DIR="$SERVICES_ROOT/dist"   # final destination for the bundle + tarball

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)   VERSION="$2";      shift 2 ;;
    --prev)      PREV_TGZ="$2";     shift 2 ;;
    --rebuild)   REBUILD_PKGS="$2"; shift 2 ;;
    --all)       REBUILD_ALL=1;     shift 1 ;;
    --out)       OUT_DIR="$2";      shift 2 ;;
    *) echo "Unknown arg: $1" >&2; exit 1 ;;
  esac
done

[[ -z "$VERSION" ]] && { echo "Usage: $0 --version X.Y.Z [--prev <path>.tar.gz] [--rebuild pkg1,pkg2 | --all] [--out <dir>]" >&2; exit 1; }

# --all expands to every package that has a go_target in pkg-map (all locally
# buildable Go services). Infra/third-party packages carry forward from the base
# bundle. Takes precedence over --rebuild.
if (( REBUILD_ALL )); then
  REBUILD_PKGS="$(python3 -c "
import json
d = json.load(open('$SERVICES_ROOT/golang/build/pkg-map.json'))
print(','.join(sorted(n for n, i in d.items() if i.get('go_target'))))
")"
fi

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

# resolve_meta_dir <pkg_name> — a package's metadata (package.json + specs/) lives
# at packages/metadata/<name>/ (pre-migration layout) OR packages/<name>/ (the
# flattened migration layout). Support BOTH so the build works against whichever
# packages branch is checked out, with no git switching. Prints the resolved dir,
# or nothing if neither exists.
resolve_meta_dir() {
  local name="$1"
  if [[ -f "$PACKAGES_ROOT/metadata/$name/package.json" ]]; then
    echo "$PACKAGES_ROOT/metadata/$name"
  elif [[ -f "$PACKAGES_ROOT/$name/package.json" ]]; then
    echo "$PACKAGES_ROOT/$name"
  fi
}

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

  grep -q 'FOUNDING_PROFILES="${FOUNDING_PROFILES:-core,media-server}"' \
    "${release_dir}/scripts/install-day0.sh" \
    || { echo "ERROR: bundled install-day0.sh does not default FOUNDING_PROFILES to core,media-server." >&2; exit 1; }
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
  # Auto-locate a base bundle: newest tarball (of a DIFFERENT version) under /tmp,
  # ~/, or the output dir. The base only supplies third-party infra binaries +
  # xds/gateway + installer that cannot be rebuilt locally; all Go services are
  # rebuilt from current source.
  PREV_TGZ=$(ls /tmp/globular-*.tar.gz "$HOME"/globular-*.tar.gz "$OUT_DIR"/globular-*.tar.gz 2>/dev/null \
             | sort -V | grep -v "globular-${VERSION}-" | tail -1 || true)
fi
[[ -z "$PREV_TGZ" || ! -f "$PREV_TGZ" ]] && { echo "ERROR: No base bundle found. Pass --prev <path>.tar.gz." >&2; exit 1; }
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
            scylla_manager scylla_manager_agent sctool)

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

# A prior `sudo install` (or privileged stage build) can leave root-owned binaries
# in the shared stage bin, which blocks refreshing them below. Reclaim ownership
# best-effort (mirrors the OUT_DIR reclaim at publish time).
if [[ -d "$STAGE_BIN" ]] && find "$STAGE_BIN" ! -user "$(id -un)" -print -quit 2>/dev/null | grep -q .; then
  sudo chown -R "$(id -un):$(id -gn)" "$STAGE_BIN" 2>/dev/null || true
fi

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
  # Mirror the fresh binary into the stage bin so specgen/pkggen (below) derive the
  # generated template's spec from CURRENT source, not a stale staged binary.
  [[ -d "$STAGE_BIN" ]] && cp "$BIN_DIR/$binary" "$STAGE_BIN/$binary"
done

# Also build globular CLI if cli changed
if printf '%s\n' "${CHANGED_NAMES[@]}" | grep -q "globular-cli"; then
  cp "$BIN_DIR/globularcli" "$BIN_DIR/globular" 2>/dev/null || true
  # pkggen/build_extra_template drive `globular pkg build` via STAGE_BIN/globularcli.
  [[ -d "$STAGE_BIN" ]] && cp "$BIN_DIR/globularcli" "$STAGE_BIN/globularcli" 2>/dev/null || true
fi

# Rebuild xds/gateway if Globular repo changed
if [[ -n "$GLOBULAR_ROOT" ]]; then
  for gname in xds gateway; do
    if printf '%s\n' "${CHANGED_NAMES[@]}" | grep -q "^${gname}$"; then
      log "Building $gname..."
      GLDFLAGS="-X main.Version=${VERSION} -X main.BuildVersion=${VERSION} -s -w"
      go build -trimpath -ldflags "$GLDFLAGS" -o "$BIN_DIR/$gname" "$GLOBULAR_ROOT/cmd/$gname"
      [[ -d "$STAGE_BIN" ]] && cp "$BIN_DIR/$gname" "$STAGE_BIN/$gname"
    fi
  done
fi

# ── Regenerate service package templates (services/generated) ─────────────────
# gRPC-service package metadata is NO LONGER hand-authored in the packages repo
# (packages commit "remove gRPC-service metadata (generated, not hand-authored
# here)"). It is derived from source: spec from each binary's --describe (specgen)
# and RBAC policy from proto AuthzRule annotations (authzgen). regenerate-release-
# inputs.sh is the sanctioned regeneration boundary — it (re)builds generated/policy,
# generated/specs, and generated/<name>_<version>_linux_amd64.tgz templates from the
# stage bin we just refreshed. build-release.sh (CI) uses the exact same boundary.
# We then consume those templates below, swapping in our freshly-built binaries.
step "Regenerate service package templates (services/generated)"
GENERATED_ROOT="$SERVICES_ROOT/generated"
POLICY_ROOT="$GENERATED_ROOT/policy"
if [[ ! -x "$STAGE_BIN/globularcli" ]]; then
  echo "ERROR: stage bin not populated at $STAGE_BIN (needs globularcli + service binaries)." >&2
  echo "       Build the stage set first (e.g. 'make stage' / build-release stage step) then re-run." >&2
  exit 1
fi
command -v protoc >/dev/null 2>&1 || { echo "ERROR: protoc required to regenerate service package templates. Run generateCode.sh / install protoc first." >&2; exit 1; }
bash "$SERVICES_ROOT/scripts/regenerate-release-inputs.sh" --version "$VERSION"
log "generated $(ls "$GENERATED_ROOT"/*.tgz 2>/dev/null | wc -l) service package templates under $GENERATED_ROOT"

# ── Build changed packages ────────────────────────────────────────────────────
step "Build changed packages"
BUILD_NUMBER=$(date +%s)   # local builds use unix timestamp as build_number
BUILD_ID=$(python3 -c "import uuid; print(uuid.uuid4())")

# Re-stamp a package.json in place with this build's identity. Always sets
# version/build_id/build_number. entrypoint_checksum is set only when a checksum is
# supplied (fresh binary swapped in); an empty checksum leaves the template's own
# entrypoint/entrypoint_checksum untouched (staged binary kept as-is).
restamp_package_json() {
  python3 - "$1" "$2" "$3" "$4" "$5" <<'PYEOF'
import json, sys
path, version, build_id, build_number, checksum = sys.argv[1:]
d = json.load(open(path))
d['version'] = version
d['build_id'] = build_id
d['build_number'] = int(build_number)
if checksum:
    d['entrypoint_checksum'] = checksum
json.dump(d, open(path, 'w'), indent=2)
PYEOF
}

# consume_generated_template <tpl> <pkg_name> <binary>: assemble the shipped package
# from a generated template — extract package.json/specs/policy/config/data, drop in
# our freshly-built binary (falling back to the staged one), re-stamp identity. Mirrors
# build-release.sh's generated-current-release path.
consume_generated_template() {
  local tpl="$1" pkg_name="$2" binary="$3"
  local per_pkg_build_id fresh_bin="" checksum_arg="" TMPROOT="$WORK/root-${pkg_name}"
  per_pkg_build_id=$(python3 -c "import uuid; print(uuid.uuid4())")
  if [[ -n "$binary" && "$binary" != "none" && "$binary" != "noop" ]]; then
    if [[ -f "$BIN_DIR/$binary" ]]; then fresh_bin="$BIN_DIR/$binary"
    elif [[ -f "$STAGE_BIN/$binary" ]]; then fresh_bin="$STAGE_BIN/$binary"; fi
  fi
  rm -rf "$TMPROOT"; mkdir -p "$TMPROOT"
  if [[ -n "$fresh_bin" ]]; then
    # keep everything but the (stale, staged) binary; swap in our fresh build
    tar -C "$TMPROOT" -xf "$tpl" --exclude='bin/*'
    mkdir -p "$TMPROOT/bin"
    cp "$fresh_bin" "$TMPROOT/bin/$binary"
    chmod +x "$TMPROOT/bin/$binary"
    checksum_arg="sha256:$(sha256sum "$TMPROOT/bin/$binary" | awk '{print $1}')"
  else
    # no rebuildable binary (e.g. gateway/xds not in this rebuild set) — ship the
    # template's staged binary + its checksum verbatim; only re-stamp identity.
    tar -C "$TMPROOT" -xf "$tpl"
  fi
  restamp_package_json "$TMPROOT/package.json" "$VERSION" "$per_pkg_build_id" "$BUILD_NUMBER" "$checksum_arg"
  tar -C "$TMPROOT" -czf "$PKG_OUT/${pkg_name}_${VERSION}_linux_amd64.tgz" .
  rm -rf "$TMPROOT"
  log "Packaged $pkg_name v${VERSION} (from generated template)"
}

for pkg_name in "${CHANGED_NAMES[@]}"; do
  go_target=$(python3 -c "
import json
d=json.load(open('$PKG_MAP'))
print(d.get('$pkg_name',{}).get('go_target',''))
" 2>/dev/null)
  binary=$(python3 -c "
import json
d=json.load(open('$PKG_MAP'))
print(d.get('$pkg_name',{}).get('binary',''))
" 2>/dev/null)
  gen_tpl="$GENERATED_ROOT/${pkg_name}_${VERSION}_linux_amd64.tgz"

  # Preferred path: a generated template (Go services + globular-cli/mcp/gateway/xds).
  # gRPC-service metadata is generated, never carried in the packages repo, so a
  # locally-built Go service MUST have a template — a missing one is a regeneration
  # bug, not a package to silently skip.
  if [[ -f "$gen_tpl" ]]; then
    if [[ -n "$go_target" && ( -z "$binary" || ( "$binary" != "none" && "$binary" != "noop" && ! -f "$BIN_DIR/$binary" ) ) ]]; then
      echo "ERROR: $pkg_name has go_target but freshly-built binary '$binary' is missing in $BIN_DIR." >&2
      exit 1
    fi
    consume_generated_template "$gen_tpl" "$pkg_name" "$binary"
    continue
  fi

  if [[ -n "$go_target" ]]; then
    echo "ERROR: no generated template for Go service '$pkg_name' at $gen_tpl." >&2
    echo "       regenerate-release-inputs.sh must produce one; refusing to ship a bundle missing a service." >&2
    exit 1
  fi

  # ── Legacy fallback: static/infra packages whose metadata is still hand-authored
  #    in the packages repo (etcd, keepalived, scylladb, …). Used when an infra spec
  #    changes; the binary comes from the base-bundle carry-forward (BIN_DIR).
  meta_dir="$(resolve_meta_dir "$pkg_name")"
  spec_file="$meta_dir/specs/${pkg_name//-/_}_service.yaml"
  [[ -f "$spec_file" ]] || spec_file="$meta_dir/specs/${pkg_name//-/_}_cmd.yaml"
  [[ -n "$meta_dir" && -d "$meta_dir" ]] || { log "SKIP $pkg_name: no generated template and no metadata dir (carried forward from base bundle)"; continue; }
  [[ -f "$spec_file" ]] || { log "SKIP $pkg_name: no spec yaml"; continue; }
  [[ -z "$binary" ]] && { log "SKIP $pkg_name: not in pkg-map"; continue; }

  no_entrypoint=0
  if [[ "$binary" == "none" || "$binary" == "noop" ]]; then
    no_entrypoint=1
  elif [[ ! -f "$BIN_DIR/$binary" ]]; then
    log "SKIP $pkg_name: binary $binary not found (carried forward from base bundle)"; continue
  fi

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
  if [[ "$no_entrypoint" -eq 0 ]]; then
    cp "$BIN_DIR/$binary" "$TMPROOT/bin/$binary"
    chmod +x "$TMPROOT/bin/$binary"
  fi
  cp "$spec_file" "$TMPROOT/specs/$(basename "$spec_file")"

  policy_src="$POLICY_ROOT/${pkg_name//-/_}"
  if [[ -d "$policy_src" ]]; then
    mkdir -p "$TMPROOT/policy"
    for pf in permissions.generated.json roles.generated.json; do
      [[ -f "$policy_src/$pf" ]] && cp -a "$policy_src/$pf" "$TMPROOT/policy/$pf"
    done
  fi

  if [[ "$no_entrypoint" -eq 1 ]]; then
    checksum_arg=""
  else
    checksum_arg="sha256:$(sha256sum "$TMPROOT/bin/$binary" | awk '{print $1}')"
  fi
  python3 - "$TMPROOT/package.json" "$pkg_version" "$per_pkg_build_id" "$BUILD_NUMBER" "$checksum_arg" <<'PYEOF'
import json, sys
path, version, build_id, build_number, checksum = sys.argv[1:]
d = json.load(open(path))
d['version'] = version
d['build_id'] = build_id
d['build_number'] = int(build_number)
if checksum:
    d['entrypoint_checksum'] = checksum
else:
    d['entrypoint'] = 'none'
    d['entrypoint_checksum'] = ''
json.dump(d, open(path, 'w'), indent=2)
PYEOF

  out_tgz="$PKG_OUT/${pkg_name}_${pkg_version}_linux_amd64.tgz"
  tar -C "$TMPROOT" -czf "$out_tgz" .
  rm -rf "$TMPROOT"
  log "Packaged $pkg_name v${pkg_version} (legacy metadata)"
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

# Packages — install-day0.sh reads from internal/assets/packages/, and packages/
# is a legacy mirror some scripts use. Populate packages/ once, then HARDLINK the
# internal/assets/packages/ copies to it so the ~700MB package set is stored once,
# not twice (matches the reference bundle layout; keeps the tarball ~750MB, not 1.5GB).
cp "$PKG_OUT/"*.tgz "$DIST_DIR/packages/"
for f in "$DIST_DIR/packages/"*.tgz; do
  ln -f "$f" "$BUNDLE_PKG_DIR/$(basename "$f")"
done

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

# install.sh at root must come from current source. Copying it from the previous
# bundle reintroduces stale bootstrap policy text and behavior.
cp "$SERVICES_ROOT/scripts/install.sh" "$DIST_DIR/install.sh"
chmod +x "$DIST_DIR/install.sh"

# webroot and docs from previous bundle; workflows come from current source.
for d in webroot docs; do
  [[ -d "$PREV_DIR/$d" ]] && cp -r "$PREV_DIR/$d" "$DIST_DIR/$d"
done
mkdir -p "$DIST_DIR/workflows"
cp "$SERVICES_ROOT/golang/workflow/definitions/"*.yaml "$DIST_DIR/workflows/"

# Ship the CURRENT ops-knowledge corpus (seeds ai-memory on the installed node),
# not the prev bundle's stale copy. Authored source is docs/operational-knowledge.
if [[ -d "$SERVICES_ROOT/docs/operational-knowledge" ]]; then
  rm -rf "$DIST_DIR/docs/operational-knowledge"
  mkdir -p "$DIST_DIR/docs"
  cp -a "$SERVICES_ROOT/docs/operational-knowledge" "$DIST_DIR/docs/operational-knowledge"
fi

# README.md — regenerate for THIS version + the exact source commit it was built
# from (the prev bundle's README names the old version).
SRC_COMMIT="$(git -C "$SERVICES_ROOT" rev-parse HEAD 2>/dev/null || echo unknown)"
PKG_COUNT="$(ls "$DIST_DIR/packages/"*.tgz 2>/dev/null | wc -l | tr -d ' ')"
cat > "$DIST_DIR/README.md" <<HEREDOC
# Globular ${VERSION}

An open-source microservices platform for self-hosted distributed systems.
No containers. No Kubernetes. No cloud required.

## Install

\`\`\`bash
sudo bash install.sh
\`\`\`

The first node always comes up with the quorum profiles
(\`control-plane\`, \`core\`, \`storage\`). To add a workload profile from
day-0, pass \`FOUNDING_PROFILES\` (comma-separated) through \`sudo\`:

\`\`\`bash
sudo FOUNDING_PROFILES=core,media-server bash install.sh
\`\`\`

## What's Included

- \`globular\` — CLI tool
- \`globular-installer\` — Day-0 package installer
- \`scripts/\` — Bootstrap scripts (TLS, minio contract, DNS, etc.)
- \`packages/\` — ${PKG_COUNT} packages (composition lockfile)
- \`release-index.json\` — BOM (Bill of Materials) for this platform release
- \`install.sh\` — Installer entry point

## BOM Release Model

This release is a **composition lockfile**. Each package keeps its own version.
Only packages whose content changed are rebuilt for this release; unchanged
packages reference their origin release via release-index.json.

## Built From

Repository: https://github.com/globulario/services
Tag:        v${VERSION}
Commit:     ${SRC_COMMIT}
HEREDOC
log "README.md written (v${VERSION}, commit ${SRC_COMMIT:0:12})"

validate_release_bundle "$DIST_DIR"

# ── Pack tarball ──────────────────────────────────────────────────────────────
step "Pack tarball"
rm -f "$OUT_TGZ" "${OUT_TGZ}.sha256"
tar -C /tmp -czf "$OUT_TGZ" "globular-${VERSION}-linux-amd64/"
sha256sum "$OUT_TGZ" > "${OUT_TGZ}.sha256"

# ── Publish to the output directory (default: services/dist) ──────────────────
# A previous `sudo install` can leave root-owned files in an existing bundle dir;
# clear them (best-effort sudo) so the copy is not blocked.
step "Publish to $OUT_DIR"
if [[ -e "$OUT_DIR" && -n "$(find "$OUT_DIR" ! -user "$(id -un)" -print -quit 2>/dev/null)" ]]; then
  sudo chown -R "$(id -un):$(id -gn)" "$OUT_DIR" 2>/dev/null || true
fi
mkdir -p "$OUT_DIR"
rm -rf "$OUT_DIR/globular-${VERSION}-linux-amd64" \
       "$OUT_DIR/globular-${VERSION}-linux-amd64.tar.gz" \
       "$OUT_DIR/globular-${VERSION}-linux-amd64.tar.gz.sha256" \
       "$OUT_DIR/.staging"
cp "$OUT_TGZ" "$OUT_DIR/"
cp "${OUT_TGZ}.sha256" "$OUT_DIR/"
cp -a "$DIST_DIR" "$OUT_DIR/globular-${VERSION}-linux-amd64"
# Keep the sha256 file's path relative so `sha256sum -c` works from $OUT_DIR.
( cd "$OUT_DIR" && sha256sum "globular-${VERSION}-linux-amd64.tar.gz" > "globular-${VERSION}-linux-amd64.tar.gz.sha256" )

SIZE=$(du -sh "$OUT_DIR/globular-${VERSION}-linux-amd64.tar.gz" | awk '{print $1}')
echo ""
echo "╔════════════════════════════════════════════════════════════════╗"
echo "║  ✓ LOCAL RELEASE BUILT                                         ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""
echo "  Bundle : $OUT_DIR/globular-${VERSION}-linux-amd64.tar.gz  ($SIZE)"
echo "  SHA256 : $(awk '{print $1}' "$OUT_DIR/globular-${VERSION}-linux-amd64.tar.gz.sha256")"
echo "  Dir    : $OUT_DIR/globular-${VERSION}-linux-amd64"
echo ""
echo "  To install:"
echo "    cd $OUT_DIR/globular-${VERSION}-linux-amd64 && sudo bash install.sh 2>&1 | tee /tmp/day0-test.log"
echo "    echo \"EXIT: \$?\""
