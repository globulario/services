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

say "verifying..."
ETCDCTL_API=3 etcdctl --endpoints=10.0.0.63:2379 \
  --cacert="$PKI/ca.crt" \
  --cert="$PKI/issued/services/service.crt" \
  --key="$PKI/issued/services/service.key" \
  --command-timeout=10s endpoint status -w table || true

echo
say "expect: IS LEADER=true and an empty ERRORS column."
say "then:   member list should show exactly one member (globule-ryzen)."
