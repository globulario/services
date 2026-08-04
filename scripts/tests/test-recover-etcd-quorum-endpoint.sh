#!/usr/bin/env bash
# Regression tests for recover-etcd-quorum.sh endpoint resolution (F9).
#
# The script previously hardcoded 10.0.0.63:2379 — the advertise URL of the one
# host where the original incident occurred — and ended verification with
# `|| true`, so a failed recovery on any node still printed success guidance and
# exited 0. These tests pin resolution from local etcd config and the failure
# propagation.
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT_UNDER_TEST="${ROOT}/scripts/recover-etcd-quorum.sh"
PASS=0; FAIL=0
ok(){ echo "  ok   $1"; PASS=$((PASS+1)); }
bad(){ echo "  FAIL $1"; FAIL=$((FAIL+1)); }

# Extract resolve_client_endpoint() so it can be exercised without running the
# destructive recovery body (which stops etcd and rewrites raft membership).
FN="$(sed -n '/^resolve_client_endpoint()/,/^}/p' "$SCRIPT_UNDER_TEST")"
[[ -n "$FN" ]] || { echo "FAIL: resolve_client_endpoint not found"; exit 1; }
eval "$FN"

mk(){ local f; f="$(mktemp)"; printf '%s\n' "$1" > "$f"; printf '%s' "$f"; }

# 1. configured IPv4 endpoint
c=$(mk 'advertise-client-urls: "10.0.0.9:2379"')
[[ "$(resolve_client_endpoint "$c")" == "10.0.0.9:2379" ]] && ok "IPv4 endpoint" || bad "IPv4 endpoint"

# 2. configured hostname endpoint
c=$(mk 'advertise-client-urls: "globule-nuc.globular.internal:2379"')
[[ "$(resolve_client_endpoint "$c")" == "globule-nuc.globular.internal:2379" ]] && ok "hostname endpoint" || bad "hostname endpoint"

# 3. configured HTTPS endpoint
c=$(mk 'advertise-client-urls: "https://10.0.0.20:2379"')
[[ "$(resolve_client_endpoint "$c")" == "https://10.0.0.20:2379" ]] && ok "https endpoint" || bad "https endpoint"

# 4. falls back to listen-client-urls when advertise is absent
c=$(mk 'listen-client-urls: "https://10.0.0.8:2379"')
[[ "$(resolve_client_endpoint "$c")" == "https://10.0.0.8:2379" ]] && ok "listen-client-urls fallback" || bad "listen-client-urls fallback"

# 5. missing endpoint -> nonzero
c=$(mk 'name: "globular-etcd"')
resolve_client_endpoint "$c" >/dev/null 2>&1; [[ $? -ne 0 ]] && ok "missing endpoint fails" || bad "missing endpoint fails"

# 6. malformed endpoint -> nonzero
c=$(mk 'advertise-client-urls: "not-a-valid-endpoint"')
resolve_client_endpoint "$c" >/dev/null 2>&1; [[ $? -ne 0 ]] && ok "malformed endpoint fails" || bad "malformed endpoint fails"

# 7. unreadable config -> nonzero
resolve_client_endpoint "/nonexistent/etcd.yaml" >/dev/null 2>&1; [[ $? -ne 0 ]] && ok "unreadable config fails" || bad "unreadable config fails"

# 8. no incident-specific host remains in executable code
if grep -nE '^[^#]*10\.0\.0\.63' "$SCRIPT_UNDER_TEST" >/dev/null 2>&1; then
  bad "hardcoded 10.0.0.63 still in executable code"
else ok "no hardcoded incident endpoint in code"; fi

# 9. verification failure must NOT be swallowed
if grep -qE 'endpoint status -w table \|\| true' "$SCRIPT_UNDER_TEST"; then
  bad "verification still swallowed by || true"
else ok "verification failure propagates"; fi

# 10. success guidance must be gated behind the verification
if grep -q 'exit 1' "$SCRIPT_UNDER_TEST" && \
   awk '/say "verifying\.\.\."/,0' "$SCRIPT_UNDER_TEST" | grep -q 'ERROR: endpoint status failed'; then
  ok "failed verification exits before success guidance"
else bad "failed verification does not exit before success guidance"; fi

echo "  ---- passed=$PASS failed=$FAIL"
[[ $FAIL -eq 0 ]]
