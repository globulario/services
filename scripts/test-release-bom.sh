#!/usr/bin/env bash
set -euo pipefail

# Validates a release-index.json against the canonical package registry.
# Usage: bash scripts/test-release-bom.sh [path-to-release-index.json]

RELEASE_INDEX="${1:-dist/release-index.json}"
REGISTRY="${2:-../packages/registry.yaml}"

die() { echo "FAIL: $*" >&2; exit 1; }
pass() { echo "PASS: $*"; }
warn() { echo "WARN: $*" >&2; }

[[ -f "$RELEASE_INDEX" ]] || die "release-index.json not found at $RELEASE_INDEX"
[[ -f "$REGISTRY" ]] || die "registry.yaml not found at $REGISTRY"

echo "━━━ Release BOM Validation ━━━"
echo "  Index:    $RELEASE_INDEX"
echo "  Registry: $REGISTRY"
echo ""

ERRORS=0

python3 <<PYEOF
import json, yaml, sys

index = json.load(open("$RELEASE_INDEX"))
with open("$REGISTRY") as f:
    registry = yaml.safe_load(f)

reg_by_name = {p["name"]: p for p in registry["packages"]}
idx_by_name = {p["name"]: p for p in index["packages"]}

errors = 0

# 1. Every registry package should be in the BOM
for name in reg_by_name:
    if name not in idx_by_name:
        print(f"WARN: registry package '{name}' missing from release index")

# 2. Every BOM package should be in the registry
for name in idx_by_name:
    if name not in reg_by_name:
        print(f"FAIL: BOM package '{name}' not in registry")
        errors += 1

# 3. Kind must match
for name, idx_pkg in idx_by_name.items():
    reg_pkg = reg_by_name.get(name)
    if not reg_pkg:
        continue
    idx_kind = idx_pkg.get("kind", "").lower()
    reg_kind = reg_pkg.get("kind", "").lower()
    if idx_kind != reg_kind:
        print(f"FAIL: kind mismatch for '{name}': BOM={idx_kind} registry={reg_kind}")
        errors += 1

# 4. Every package must have a version
for name, pkg in idx_by_name.items():
    if not pkg.get("version"):
        print(f"FAIL: '{name}' has empty version in BOM")
        errors += 1

# 5. Every changed package must have a package_digest
for name, pkg in idx_by_name.items():
    if pkg.get("changed_in_release") and not pkg.get("package_digest"):
        print(f"FAIL: '{name}' is changed but has no package_digest")
        errors += 1

# 6. Every unchanged package must have an origin_release
for name, pkg in idx_by_name.items():
    if not pkg.get("changed_in_release") and not pkg.get("origin_release"):
        print(f"FAIL: '{name}' is unchanged but has no origin_release")
        errors += 1

# 7. Strict v2 install checks.
#
# Single-authority identity model (docs/design/package-identity-single-authority.md):
# build_id / build_number are REPOSITORY-ADMISSION identity. A freshly built
# BOM must NOT carry them (pre-minting is the violation the old check
# accidentally demanded); admission derives deterministic pins from
# artifact_sha256, which IS required here. A legacy BOM that still carries a
# well-formed non-numeric build_id is tolerated (already-published indexes are
# immutable); numeric-only build_ids remain rejected.
schema = index.get("schema_version")
if schema == "globular.repository.index/v2":
    for name, pkg in idx_by_name.items():
        bid = str(pkg.get("build_id", "")).strip()
        if bid.isdigit() and bid != "":
            print(f"FAIL: '{name}' has numeric-only build_id '{bid}' (v2 strict install)")
            errors += 1

        bn = pkg.get("build_number", 0)
        if not isinstance(bn, int) or bn < 0:
            print(f"FAIL: '{name}' has invalid build_number={bn} (must be an int >= 0; identity is assigned at repository admission)")
            errors += 1

        art = str(pkg.get("artifact_sha256", "")).strip()
        if not art:
            print(f"FAIL: '{name}' missing artifact_sha256 (v2 strict install — the content digest is the non-negotiable pin)")
            errors += 1

    # Reject duplicate artifact bytes with conflicting identity tuple.
    seen = {}
    for pkg in index["packages"]:
        digest = str(pkg.get("artifact_sha256", "")).strip().lower()
        if not digest:
            continue
        key = (
            str(pkg.get("publisher", "")).strip().lower(),
            str(pkg.get("name", "")).strip().lower(),
            str(pkg.get("platform", "")).strip().lower(),
            digest,
        )
        ident = (
            str(pkg.get("build_id", "")).strip(),
            str(pkg.get("version", "")).strip(),
            int(pkg.get("build_number", 0) or 0),
        )
        if key in seen and seen[key] != ident:
            print(
                "FAIL: duplicate artifact_sha256 for same publisher/name/platform "
                "with different build_id/version/build_number "
                f"(publisher={key[0]} name={key[1]} platform={key[2]} digest={digest})"
            )
            errors += 1
        else:
            seen[key] = ident

# Summary
total = len(idx_by_name)
changed = sum(1 for p in idx_by_name.values() if p.get("changed_in_release"))
print(f"\nBOM: {total} packages ({changed} changed, {total - changed} unchanged)")

if errors:
    print(f"\n{errors} error(s) found")
    sys.exit(1)
else:
    print("\nAll checks passed")
PYEOF

echo ""
pass "Release BOM validation complete"
