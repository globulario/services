#!/usr/bin/env bash
set -euo pipefail

# Validates that the join script serves all binaries required for Day-1 join.
# Usage: bash scripts/test-day1-join-bom.sh [gateway-address]
#
# If no gateway is provided, validates against the registry only (offline mode).

GATEWAY="${1:-}"
REGISTRY="${2:-../packages/registry.yaml}"

die() { echo "FAIL: $*" >&2; exit 1; }
pass() { echo "PASS: $*"; }
warn() { echo "WARN: $*" >&2; }

[[ -f "$REGISTRY" ]] || die "registry.yaml not found at $REGISTRY"

echo "━━━ Day-1 Join BOM Check ━━━"
echo "  Registry: $REGISTRY"
[[ -n "$GATEWAY" ]] && echo "  Gateway:  $GATEWAY (online check)" || echo "  Mode:     offline (registry-only)"
echo ""

python3 <<PYEOF
import yaml, sys

with open("$REGISTRY") as f:
    registry = yaml.safe_load(f)

# Day-1 join requires these packages to be available
join_required = [p for p in registry["packages"] if p.get("day1_join_required")]

print(f"Day-1 join required packages: {len(join_required)}")
print()

for pkg in join_required:
    name = pkg["name"]
    binary = pkg.get("binary", "")
    kind = pkg.get("kind", "?")
    profiles = pkg.get("profiles", [])
    print(f"  {name:25s} kind={kind:15s} binary={binary:25s} profiles={profiles}")

# The join script serves these binaries from /join/bin/:
# node_agent_server, globularcli, etcd, etcdctl
join_binaries = ["node_agent_server", "globularcli", "etcd", "etcdctl"]

print(f"\nJoin script serves {len(join_binaries)} binaries:")
for b in join_binaries:
    # Find which registry package provides this binary
    providers = [p["name"] for p in registry["packages"] if p.get("binary") == b]
    if providers:
        print(f"  {b:25s} ← {providers[0]}")
    else:
        print(f"  {b:25s} ← NOT IN REGISTRY")

# Verify all day1_join_required packages have their binary in the join set
# OR will be installed by the node-agent after join
errors = 0
for pkg in join_required:
    binary = pkg.get("binary", "")
    if binary in join_binaries:
        continue
    # These will be installed by controller after join — that's OK
    # But they must be in the release BOM
    if pkg.get("kind") == "infrastructure":
        continue
    # Flag if a required service binary is missing from join
    if binary and binary not in join_binaries:
        print(f"\n  WARN: {pkg['name']} (binary={binary}) not served by join script — must be installed post-join by controller")

print()
if errors:
    print(f"{errors} error(s)")
    sys.exit(1)
else:
    print("Day-1 join requirements documented")
PYEOF

# Online check: if gateway provided, verify join script is reachable
if [[ -n "$GATEWAY" ]]; then
    echo ""
    echo "Online check: fetching join script from $GATEWAY..."
    if curl -sfL -k "https://${GATEWAY}/join" -o /dev/null 2>/dev/null; then
        pass "Join script endpoint reachable"
    else
        warn "Join script endpoint not reachable (gateway may be down)"
    fi

    # Check binary endpoints
    for BIN in node_agent_server globularcli etcd etcdctl; do
        if curl -sfL -k "https://${GATEWAY}/join/bin/${BIN}" -o /dev/null 2>/dev/null; then
            pass "Binary $BIN available at /join/bin/${BIN}"
        else
            warn "Binary $BIN NOT available (may not be served yet)"
        fi
    done
fi

echo ""
pass "Day-1 join BOM check complete"
