#!/usr/bin/env bash
set -euo pipefail

STATE=/var/lib/globular/clustercontroller/state.json
GHOST1="4c2b3cb3-d02a-56d3-93cf-4e2c8728e8a4"
GHOST2="814fbbb9-607f-5144-be1a-a863a0bea1e1"

echo "=== Stopping controller ==="
sudo systemctl stop globular-cluster-controller.service

echo "=== Backing up state ==="
sudo cp "$STATE" "${STATE}.bak-$(date +%s)"

echo "=== Removing ghost nodes from state ==="
sudo python3 -c "
import json
with open('$STATE') as f:
    state = json.load(f)
nodes = state.get('nodes', {})
for gid in ['$GHOST1', '$GHOST2']:
    if gid in nodes:
        hostname = nodes[gid].get('identity', {}).get('hostname', 'unknown')
        del nodes[gid]
        print(f'Removed ghost node {gid} ({hostname})')
    else:
        print(f'Ghost node {gid} not found (already removed)')
remaining = [(nid, n.get('identity', {}).get('hostname', '?')) for nid, n in nodes.items()]
print(f'Remaining nodes: {remaining}')
with open('$STATE', 'w') as f:
    json.dump(state, f, indent=2)
"

echo ""
echo "=== Deploying latest controller binary ==="
sudo cp /home/dave/Documents/github.com/globulario/services/golang/cluster_controller_server /usr/lib/globular/bin/cluster_controller_server

echo "=== Starting controller ==="
sudo systemctl start globular-cluster-controller.service
sleep 5

echo "=== Controller status ==="
systemctl status globular-cluster-controller.service --no-pager | head -10

echo ""
echo "=== Checking auto-import ==="
journalctl -u globular-cluster-controller.service --since "10 sec ago" --no-pager | grep -iE "autoImport|import|leader|node" | head -10

echo ""
echo "=== Done! ==="
