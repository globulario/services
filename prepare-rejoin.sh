#!/usr/bin/env bash
set -euo pipefail

CACERT=/var/lib/globular/pki/ca.crt
CERT=/var/lib/globular/pki/issued/services/service.crt
KEY=/var/lib/globular/pki/issued/services/service.key
EP=https://127.0.0.1:2379

e() {
  ETCDCTL_API=3 etcdctl --endpoints=$EP --cacert=$CACERT --cert=$CERT --key=$KEY "$@"
}

echo "=== Step 1: Stop controller ==="
systemctl stop globular-cluster-controller.service || true

echo ""
echo "=== Step 2: Remove dell and nuc from etcd ==="
# Remove their installed-state keys
e del "/globular/nodes/8315658d" --prefix
e del "/globular/nodes/e56c03f8" --prefix
# Remove any stale plans
e del "globular/plans/" --prefix
# Remove releases (will be recreated)
e del "/globular/resources/ServiceRelease/" --prefix
e del "/globular/resources/InfrastructureRelease/" --prefix
echo "etcd cleaned"

echo ""
echo "=== Step 3: Remove dell and nuc from controller state ==="
python3 -c "
import json
path = '/var/lib/globular/clustercontroller/state.json'
with open(path) as f:
    state = json.load(f)
nodes = state.get('nodes', {})
to_remove = []
for nid, n in nodes.items():
    hostname = n.get('identity', {}).get('hostname', '')
    if hostname in ('globule-dell', 'globule-nuc'):
        to_remove.append((nid, hostname))
for nid, hostname in to_remove:
    del nodes[nid]
    print(f'Removed {nid} ({hostname})')
print(f'Remaining: {[(nid, n.get(\"identity\",{}).get(\"hostname\",\"?\")) for nid,n in nodes.items()]}')
with open(path, 'w') as f:
    json.dump(state, f, indent=2)
"

echo ""
echo "=== Step 4: Deploy updated gateway (with etcd join support) ==="
cp /home/dave/Documents/github.com/globulario/Globular/gateway /usr/lib/globular/bin/gateway
systemctl restart globular-gateway.service
sleep 2
echo "Gateway restarted"

echo ""
echo "=== Step 5: Deploy updated controller ==="
cp /home/dave/Documents/github.com/globulario/services/golang/cluster_controller_server /usr/lib/globular/bin/cluster_controller_server

echo ""
echo "=== Step 6: Start controller ==="
systemctl start globular-cluster-controller.service
sleep 5

echo ""
echo "=== Step 7: Verify ==="
systemctl status globular-gateway.service --no-pager | head -5
echo ""
systemctl status globular-cluster-controller.service --no-pager | head -5
echo ""
e member list
echo ""

echo "=== Step 8: Create join token ==="
globular cluster token create --duration 1h 2>&1 || echo "(token creation may need the controller to be fully ready — try manually)"

echo ""
echo "=== Ready! ==="
echo ""
echo "On each new node, run:"
echo "  curl -sfL https://10.0.0.63:443/join -k | sudo bash -s -- --token <TOKEN>"
echo ""
