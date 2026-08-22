#!/usr/bin/env bash
# gen-version.sh — THE single writer of zz_version_generated.go files.
#
# Usage: bash build/gen-version.sh <default-version> [version-overrides-file]
#
# AUTHORITY MODEL (docs/design/package-identity-single-authority.md):
#
#   Committed zz_version_generated.go files ARE the per-package version
#   authority materialized into source. They are COMMITTED and CI-gated —
#   this script is the only tool that writes them. The one-way flow:
#
#     version allocation (deploy pipeline / explicit bump)
#       → this script materializes it into zz_version_generated.go (committed)
#         → binary --version reports it
#           → packaging copies it ONCE into package.json.version
#             → release-index.json (BOM) RECORDS what shipped
#
#   Release builds (scripts/build-release.sh) do NOT call this script and do
#   NOT override versions: they consume the committed zz values via
#   scripts/gen-package-versions-from-source.sh. NEVER stamp the platform
#   release version into package versions:
#
#     Platform release 1.2.52 != every package is version 1.2.52.
#
# Run this script when ALLOCATING a new version for one or more packages
# (deploy pipeline bump, or manual bump before a release), then COMMIT the
# regenerated files.
#
# Override file format (one "key=version" per line):
#   Accepts EITHER of these key formats:
#     pkgname=version                       (e.g. authentication=1.2.43)
#     go_target=version                     (e.g. authentication/authentication_server=1.2.43)
#   Lines starting with # are comments.
#
# The version is written as a var (not const) so -X main.Version=<ver> ldflags
# can override it — that escape is SANCTIONED ONLY for the hotfix/local lane
# (GLOBULAR_PACKAGE_VERSION_SUFFIX set, non-core publisher); normal release
# builds must not pass -X main.Version. The init() sentinel catches
# accidental empty-string builds at runtime.

set -euo pipefail

VERSION="${1:-}"
OVERRIDES_FILE="${2:-}"

if [[ -z "$VERSION" ]]; then
  echo "error: version argument required (e.g. 1.0.31)" >&2
  exit 1
fi

# Validate semver-ish (must start with digit)
if ! [[ "$VERSION" =~ ^[0-9] ]]; then
  echo "error: version '$VERSION' must start with a digit" >&2
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GOLANG_ROOT="$(dirname "$SCRIPT_DIR")"

# Load overrides if provided.
# Accepts both formats:
#   go_target=version  (authentication/authentication_server=1.2.43)
#   pkgname=version    (authentication=1.2.43)  — matches the shipped target identity
declare -A VERSION_OVERRIDES=()
if [[ -n "$OVERRIDES_FILE" && -f "$OVERRIDES_FILE" ]]; then
  while IFS='=' read -r pkg_suffix pkg_version; do
    pkg_suffix="$(echo "$pkg_suffix" | tr -d ' ')"
    # Strip inline comments and trailing whitespace from value.
    pkg_version="$(echo "$pkg_version" | sed 's/#.*//' | tr -d ' ')"
    [[ -z "$pkg_suffix" || "$pkg_suffix" == \#* ]] && continue
    [[ -z "$pkg_version" ]] && continue
    VERSION_OVERRIDES["$pkg_suffix"]="$pkg_version"
  done < "$OVERRIDES_FILE"
  echo "gen-version: loaded ${#VERSION_OVERRIDES[@]} overrides from $OVERRIDES_FILE"
fi

# Read package directories from services.list
PKGS=()
while IFS='|' read -r target _; do
  target="$(echo "$target" | tr -d ' ')"
  [[ -z "$target" || "$target" == \#* ]] && continue
  # target is e.g. ./authentication/authentication_server
  pkg_dir="$GOLANG_ROOT/${target#./}"
  [[ -d "$pkg_dir" ]] && PKGS+=("$pkg_dir")
done < "$SCRIPT_DIR/services.list"

COUNT=0
OVERRIDDEN=0
for pkg_dir in "${PKGS[@]}"; do
  # Determine version for this package.
  pkg_version="$VERSION"
  # Check for override. Try four representations of one package identity:
  #   1. Full go_target path: authentication/authentication_server
  #   2. Target leaf without _server: authentication
  #   3. Hyphenated target leaf: cluster-controller or globular-oci-runner
  #   4. Registry package name: globular-cli (differs from the derivations)
  rel_path="${pkg_dir#$GOLANG_ROOT/}"
  target_leaf="${rel_path##*/}"
  pkg_name="${target_leaf%_server}"
  pkg_name_hyphen="${pkg_name//_/-}"
  # 4. Registry package name. Identity follows the target LEAF, and for one
  #    target the registry name differs from every mechanical derivation:
  #    globularcli ships as globular-cli. Mirrors pkg_name_for_target in
  #    scripts/gen-package-versions-from-source.sh — keep the two in step.
  case "${pkg_name}" in
    globularcli) pkg_registry_name="globular-cli" ;;
    *)           pkg_registry_name="${pkg_name_hyphen}" ;;
  esac
  if [[ ${#VERSION_OVERRIDES[@]} -gt 0 ]]; then
    if [[ -n "${VERSION_OVERRIDES[$rel_path]+x}" ]]; then
      pkg_version="${VERSION_OVERRIDES[$rel_path]}"
      OVERRIDDEN=$((OVERRIDDEN + 1))
    elif [[ -n "${VERSION_OVERRIDES[$pkg_name]+x}" ]]; then
      pkg_version="${VERSION_OVERRIDES[$pkg_name]}"
      OVERRIDDEN=$((OVERRIDDEN + 1))
    elif [[ -n "${VERSION_OVERRIDES[$pkg_name_hyphen]+x}" ]]; then
      pkg_version="${VERSION_OVERRIDES[$pkg_name_hyphen]}"
      OVERRIDDEN=$((OVERRIDDEN + 1))
    elif [[ -n "${VERSION_OVERRIDES[$pkg_registry_name]+x}" ]]; then
      pkg_version="${VERSION_OVERRIDES[$pkg_registry_name]}"
      OVERRIDDEN=$((OVERRIDDEN + 1))
    else
      # No override matched, so this package silently takes the DEFAULT version.
      # That is how globularcli got bumped by a build that changed no CLI source:
      # its registry name is globular-cli while every key form derived above is
      # globularcli, so the lookup missed and the default applied. A miss that
      # lands on a plausible value is indistinguishable from a hit — say so.
      if [[ -n "${OVERRIDES_FILE}" ]]; then
        echo "gen-version: WARNING no override matched '${pkg_name}' (tried ${rel_path}, ${pkg_name}, ${pkg_name_hyphen}, ${pkg_registry_name}); using default ${VERSION}" >&2
      fi
    fi
  fi

  cat > "$pkg_dir/zz_version_generated.go" <<GOFILE
// Code generated by build/gen-version.sh — DO NOT EDIT.
// Version is set at build time via gen-version.sh or -X main.Version=<ver> ldflags.
package main

// Version is the release version injected at build time.
// var (not const) so -X main.Version=<ver> ldflags can override when needed.
var Version = "${pkg_version}"

func init() {
	// Runtime sentinel: catches stale generated files in CI where the version
	// is expected but the binary shipped with an empty string.
	if Version == "" {
		panic("globular: Version is empty — inject with gen-version.sh or -X main.Version=<ver>")
	}
}
GOFILE
  COUNT=$((COUNT + 1))
done

echo "gen-version: wrote versions into ${COUNT} packages (${OVERRIDDEN} overridden, default=${VERSION})"
