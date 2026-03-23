#!/usr/bin/env bash
set -euo pipefail

STATE=/var/lib/globular/clustercontroller/state.json
GHOST="814fbbb9-607f-5144-be1a-a863a0bea1e1"

CACERT=/var/lib/globular/pki/ca.crt
CERT=/var/lib/globular/pki/issued/services/service.crt
KEY=/var/lib/globular/pki/issued/services/service.key
EP=https://127.0.0.1:2379

e() {
  ETCDCTL_API=3 etcdctl --endpoints=$EP --cacert=$CACERT --cert=$CERT --key=$KEY "$@"
}

echo "=== Stopping controller ==="
systemctl stop globular-cluster-controller.service || true

echo "=== Removing ghost nuc from state ==="
python3 << 'PYEOF'
import json
path = "/var/lib/globular/clustercontroller/state.json"
with open(path) as f:
    state = json.load(f)
ghost = "814fbbb9-607f-5144-be1a-a863a0bea1e1"
if ghost in state.get("nodes", {}):
    del state["nodes"][ghost]
    print("Removed ghost " + ghost)
else:
    print("Ghost not found")
remaining = [(nid, n.get("identity",{}).get("hostname","?")) for nid,n in state.get("nodes",{}).items()]
print("Remaining:", remaining)
with open(path, "w") as f:
    json.dump(state, f, indent=2)
PYEOF

echo "=== Cleaning etcd ==="
e del "globular/plans/" --prefix
e del "/globular/resources/ServiceRelease/" --prefix
e del "/globular/resources/InfrastructureRelease/" --prefix
e del "/globular/nodes/$GHOST" --prefix

echo "=== Starting controller ==="
systemctl start globular-cluster-controller.service
sleep 10

echo "=== Check ==="
journalctl -u globular-cluster-controller.service --since "10 sec ago" --no-pager | grep -iE "created with phase|import" | head -10

echo "=== Done ==="
