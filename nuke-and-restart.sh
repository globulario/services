#!/usr/bin/env bash
set -euo pipefail

CACERT=/var/lib/globular/pki/ca.crt
CERT=/var/lib/globular/pki/issued/services/service.crt
KEY=/var/lib/globular/pki/issued/services/service.key
EP=https://127.0.0.1:2379

e() {
  ETCDCTL_API=3 etcdctl --endpoints=$EP --cacert=$CACERT --cert=$CERT --key=$KEY "$@"
}

echo "=== Stopping controller ==="
systemctl stop globular-cluster-controller.service || true

echo "=== ALL keys in etcd ==="
e get "" --prefix --keys-only

echo ""
echo "=== Nuking plans ==="
e del "globular/plans/" --prefix
e del "/globular/plans/" --prefix

echo "=== Nuking releases ==="
e del "/globular/resources/ServiceRelease/" --prefix
e del "/globular/resources/InfrastructureRelease/" --prefix

echo "=== Nuking ghost installed-state ==="
e del "/globular/nodes/4c2b3cb3" --prefix
e del "/globular/nodes/814fbbb9" --prefix

echo ""
echo "=== Remaining keys ==="
e get "" --prefix --keys-only | wc -l
echo "keys remaining"

echo ""
echo "=== Starting controller ==="
systemctl start globular-cluster-controller.service
sleep 12

echo "=== Check ==="
journalctl -u globular-cluster-controller.service --since "12 sec ago" --no-pager | grep -iE "created with phase|wrote plan|import|APPLYING" | head -15

echo "=== Done ==="
