#!/usr/bin/env bash
set -euo pipefail

# Validates that a release tarball contains all packages required for Day-0.
# Usage: bash scripts/test-day0-bom.sh [path-to-release-dir]

RELEASE_DIR="${1:-.}"
REGISTRY="${2:-../packages/registry.yaml}"

die() { echo "FAIL: $*" >&2; exit 1; }
pass() { echo "PASS: $*"; }

[[ -f "$RELEASE_DIR/release-index.json" ]] || die "release-index.json not found in $RELEASE_DIR"
[[ -d "$RELEASE_DIR/packages" ]] || die "packages/ directory not found in $RELEASE_DIR"
[[ -f "$REGISTRY" ]] || die "registry.yaml not found at $REGISTRY"

echo "━━━ Day-0 BOM Completeness Check ━━━"
echo "  Release dir: $RELEASE_DIR"
echo "  Registry:    $REGISTRY"
echo ""

python3 <<PYEOF
import json, yaml, os, sys, glob

with open("$REGISTRY") as f:
    registry = yaml.safe_load(f)

index = json.load(open("$RELEASE_DIR/release-index.json"))
idx_by_name = {p["name"]: p for p in index["packages"]}

# Find all .tgz files in packages/
available_tgz = set()
for f in glob.glob("$RELEASE_DIR/packages/*.tgz"):
    # Extract package name from filename: name_version_platform.tgz
    base = os.path.basename(f).rsplit("_", 2)[0] if "_" in os.path.basename(f) else os.path.basename(f)
    available_tgz.add(base)

errors = 0

# Check every day0_required package is present
day0_packages = [p for p in registry["packages"] if p.get("day0_required")]
print(f"Day-0 required packages: {len(day0_packages)}")
print()

for pkg in day0_packages:
    name = pkg["name"]

    # Must be in release index
    if name not in idx_by_name:
        print(f"  FAIL: {name} — missing from release-index.json")
        errors += 1
        continue

    idx_entry = idx_by_name[name]

    # Must have a .tgz in the packages/ directory
    has_tgz = any(name in t for t in available_tgz)
    if not has_tgz:
        # Check if filename from index exists
        fn = idx_entry.get("filename", "")
        if fn and os.path.isfile(f"$RELEASE_DIR/packages/{fn}"):
            has_tgz = True

    if not has_tgz:
        print(f"  FAIL: {name} — in index but .tgz missing from packages/")
        errors += 1
        continue

    version = idx_entry.get("version", "?")
    print(f"  OK:   {name} v{version}")

# Check install.sh exists
if not os.path.isfile("$RELEASE_DIR/install.sh"):
    print(f"\n  FAIL: install.sh missing")
    errors += 1
else:
    print(f"\n  OK:   install.sh present")

# Check globular-installer exists
if not os.path.isfile("$RELEASE_DIR/globular-installer"):
    print(f"  FAIL: globular-installer binary missing")
    errors += 1
else:
    print(f"  OK:   globular-installer present")

# Check workflows exist
wf_dir = "$RELEASE_DIR/workflows"
if not os.path.isdir(wf_dir):
    print(f"  FAIL: workflows/ directory missing")
    errors += 1
else:
    wf_count = len(glob.glob(f"{wf_dir}/*.yaml"))
    print(f"  OK:   {wf_count} workflow definitions")

print()
if errors:
    print(f"{errors} error(s) — Day-0 install would fail")
    sys.exit(1)
else:
    print("All Day-0 requirements satisfied")
PYEOF

echo ""
pass "Day-0 BOM check complete"
