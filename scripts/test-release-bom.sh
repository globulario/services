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
