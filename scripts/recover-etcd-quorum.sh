#!/usr/bin/env bash
# Restore a single-node etcd that has lost quorum to a phantom voter.
#
# Symptom this fixes:
#   etcdctl endpoint status -> IS LEADER=false, ERRORS="etcdserver: no leader"
#   every etcd write hangs; gateway/controller/doctor all crawl.
#
# Cause: a failed Day-1 join left a second voter registered for a node that no
# longer exists, so quorum (2 of 2) can never be met. The surviving member
# cannot remove the dead one, because MemberRemove itself needs quorum.
#
# --force-new-cluster rewrites the raft member list to this node alone. The
# data-dir is preserved; only membership is rewritten. A timestamped backup is
# taken first.
set -euo pipefail

ETCD_BIN=/usr/lib/globular/bin/etcd
ETCD_CFG=/var/lib/globular/config/etcd.yaml
ETCD_DATA=/var/lib/globular/etcd
PKI=/var/lib/globular/pki

say() { printf '  %s\n' "$*"; }

say "stopping globular-etcd..."
systemctl stop globular-etcd.service

BACKUP="${ETCD_DATA}.bak-$(date +%s)"
cp -a "$ETCD_DATA" "$BACKUP"
say "backup: $BACKUP"

say "starting etcd with --force-new-cluster (20s)..."
sudo -u globular "$ETCD_BIN" --config-file "$ETCD_CFG" --force-new-cluster \
  > /tmp/etcd-force-new-cluster.log 2>&1 &
FNC_PID=$!
sleep 20

if grep -qiE "became leader|elected leader|raft.node: .* became leader" /tmp/etcd-force-new-cluster.log; then
  say "leader elected under force-new-cluster"
else
  say "WARNING: no leader line found — check /tmp/etcd-force-new-cluster.log"
fi

say "stopping the temporary instance..."
kill "$FNC_PID" 2>/dev/null || true
wait "$FNC_PID" 2>/dev/null || true
sleep 3

say "starting globular-etcd normally..."
systemctl start globular-etcd.service
sleep 12

# Resolve the client endpoint from THIS node's etcd configuration.
#
# This previously hardcoded 10.0.0.63:2379 — the advertise URL of the one host
# where the original incident happened. On any other node the script verified a
# machine it had not just recovered, and `|| true` swallowed the failure, so a
# failed recovery still printed success guidance and exited 0.
resolve_client_endpoint() {
  local cfg="$1" url=""
  [[ -r "$cfg" ]] || return 1
  # advertise-client-urls is what clients are told to dial; fall back to the
  # listen URL when only that is set. First entry wins for a comma-separated list.
  for key in advertise-client-urls listen-client-urls; do
    url="$(sed -nE "s/^[[:space:]]*${key}:[[:space:]]*[\"']?([^\"',]+).*/\1/p" "$cfg" | head -n1)"
    [[ -n "$url" ]] && break
  done
  [[ -n "$url" ]] || return 1
  # Reject anything that is not a usable endpoint rather than dialing garbage.
  [[ "$url" =~ ^(https?://)?[A-Za-z0-9._-]+:[0-9]+$ ]] || return 2
  printf '%s' "$url"
}

say "resolving client endpoint from ${ETCD_CFG}..."
if ! ETCD_ENDPOINT="$(resolve_client_endpoint "$ETCD_CFG")"; then
  case $? in
    2) say "ERROR: malformed client URL in ${ETCD_CFG}" ;;
    *) say "ERROR: no advertise-client-urls/listen-client-urls found in ${ETCD_CFG}" ;;
  esac
  exit 1
fi
say "client endpoint: ${ETCD_ENDPOINT}"

say "verifying..."
# NO `|| true`. A failed verification means the recovery did not work, and the
# caller must see a nonzero exit instead of success guidance.
if ! ETCDCTL_API=3 etcdctl --endpoints="$ETCD_ENDPOINT" \
  --cacert="$PKI/ca.crt" \
  --cert="$PKI/issued/services/service.crt" \
  --key="$PKI/issued/services/service.key" \
  --command-timeout=10s endpoint status -w table; then
  say "ERROR: endpoint status failed against ${ETCD_ENDPOINT} — recovery did NOT complete."
  say "inspect /tmp/etcd-force-new-cluster.log and the globular-etcd unit before retrying."
  exit 1
fi

echo
say "expect: IS LEADER=true and an empty ERRORS column."
say "then:   member list should show exactly one member for this node."
