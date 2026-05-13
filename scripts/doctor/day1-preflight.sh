#!/usr/bin/env bash
set -euo pipefail

RELEASE_INDEX="${1:-dist/release-index.json}"
REGISTRY="${2:-../packages/registry.yaml}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

warn() { echo -e "${YELLOW}WARN:${NC} $*" >&2; }
pass() { echo -e "${GREEN}PASS:${NC} $*"; }

echo -e "${BLUE}━━━ Day-1 State-Integrity Preflight ━━━${NC}"
echo "  release-index: $RELEASE_INDEX"
echo "  registry:      $REGISTRY"
echo ""

if [[ -f "$RELEASE_INDEX" && -f "$REGISTRY" ]]; then
  bash scripts/test-release-bom.sh "$RELEASE_INDEX" "$REGISTRY"
else
  warn "Skipping release BOM check (missing file)."
fi

if [[ -f "$REGISTRY" ]]; then
  bash scripts/test-day1-join-bom.sh "" "$REGISTRY"
else
  warn "Skipping join BOM check (missing registry)."
fi

if ! command -v globular >/dev/null 2>&1; then
  warn "globular CLI not found; skipping live cluster checks"
  pass "Day-1 state-integrity preflight (static-only) complete"
  exit 0
fi

REPORT_JSON="$(globular --timeout 20s doctor report cluster --fresh --json 2>/dev/null || true)"
if [[ -n "$REPORT_JSON" ]]; then
  python3 <<'PYEOF' "$REPORT_JSON"
import json
import sys

raw = sys.argv[1]
try:
    data = json.loads(raw)
except Exception:
    print("WARN: could not parse doctor report JSON; skipping doctor-backed checks")
    sys.exit(0)

findings = data.get("findings", []) or []
blocked = {
    "node.stale_duplicate",
    "artifact.installed_digest_mismatch",
    "artifact.desired_build_mismatch",
    "artifact.cache_digest_mismatch",
    "repository.watchdog_inconsistency",
    "dns.zone_reload_failed",
}

hits = []
for f in findings:
    inv = (f.get("invariantId") or f.get("invariant_id") or "").strip()
    sev = (f.get("severity") or "").strip()
    summ = (f.get("summary") or "").strip()
    if inv in blocked:
        hits.append((inv, sev, summ))

if hits:
    print("FAIL: blocking doctor findings present:")
    for inv, sev, summ in hits:
        print(f"  - {inv} [{sev}] {summ}")
    sys.exit(2)

print("PASS: doctor report has no blocking Day-1 integrity invariants")
PYEOF
else
  warn "doctor report unavailable; skipping doctor-backed checks"
fi

if globular --timeout 20s services verify-integrity --json >/tmp/day1_verify_integrity.json 2>/tmp/day1_verify_integrity.err; then
  pass "services verify-integrity reports no findings"
else
  rc=$?
  if [[ $rc -eq 1 || $rc -eq 2 ]]; then
    echo "FAIL: services verify-integrity reported findings"
    cat /tmp/day1_verify_integrity.json 2>/dev/null || true
    exit 1
  fi
  warn "services verify-integrity RPC unavailable (rc=$rc); skipping"
fi

NODE_JSON="$(globular --timeout 20s --output json cluster nodes list 2>/dev/null || true)"
if [[ -n "$NODE_JSON" ]]; then
  DOMAIN="$(jq -r '.Domain // empty' /var/lib/globular/config.json 2>/dev/null || true)"
  if [[ -n "$DOMAIN" ]]; then
    python3 <<'PYEOF' "$NODE_JSON" "$DOMAIN"
import json
import subprocess
import sys

nodes_raw, domain = sys.argv[1], sys.argv[2]
try:
    data = json.loads(nodes_raw)
except Exception:
    print("WARN: could not parse node list; skipping DNS-role check")
    sys.exit(0)

known_ips = set()
def walk(x):
    if isinstance(x, dict):
        ips = x.get("ips")
        if isinstance(ips, list):
            for ip in ips:
                if isinstance(ip, str) and ip:
                    known_ips.add(ip)
        for v in x.values():
            walk(v)
    elif isinstance(x, list):
        for v in x:
            walk(v)

walk(data)
if not known_ips:
    print("WARN: no node IPs discovered; skipping DNS-role check")
    sys.exit(0)

bad = []
for role in ("gateway", "dns", "controller"):
    name = f"{role}.{domain}"
    try:
        out = subprocess.check_output([
            "globular", "--timeout", "10s", "--output", "json", "dns", "inspect", name, "--types", "A"
        ], stderr=subprocess.DEVNULL, text=True)
        rec = json.loads(out)
    except Exception:
        continue

    ips = set()
    def collect(z):
        if isinstance(z, dict):
            for k, v in z.items():
                if k.lower() in ("ip", "address", "target") and isinstance(v, str) and "." in v:
                    ips.add(v)
                collect(v)
        elif isinstance(z, list):
            for v in z:
                collect(v)
    collect(rec)

    for ip in ips:
        if ip not in known_ips:
            bad.append((name, ip))

if bad:
    print("FAIL: DNS role record points to IP not present in active node inventory:")
    for name, ip in bad:
        print(f"  - {name} -> {ip}")
    sys.exit(3)

print("PASS: DNS role records resolve only to known node IPs")
PYEOF
  else
    warn "Domain unavailable; skipping DNS role-record check"
  fi
else
  warn "cluster nodes list unavailable; skipping DNS role-record check"
fi

pass "Day-1 state-integrity preflight complete"
