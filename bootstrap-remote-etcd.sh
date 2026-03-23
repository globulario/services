#!/usr/bin/env bash
set -euo pipefail

# Bootstrap remote nodes with etcd endpoint pointing to ryzen.
# This breaks the chicken-and-egg: node agents need etcd to receive plans,
# but etcd endpoints are delivered via plans.

ETCD_ENDPOINT="https://10.0.0.63:2379"
REMOTE_NODES=("10.0.0.20" "10.0.0.214")  # dell and nuc IPs — adjust if different

for node in "${REMOTE_NODES[@]}"; do
    echo "=== Bootstrapping $node ==="
    ssh -o StrictHostKeyChecking=no "$node" "
        sudo mkdir -p /var/lib/globular/config
        echo '$ETCD_ENDPOINT' | sudo tee /var/lib/globular/config/etcd_endpoints > /dev/null
        sudo systemctl restart globular-node-agent.service
        echo 'Done: etcd_endpoints written, node-agent restarted'
    " 2>&1 || echo "FAILED to reach $node — try manually"
    echo ""
done

echo "=== Waiting 10s for agents to reconnect ==="
sleep 10

echo "=== Checking controller logs ==="
journalctl -u globular-cluster-controller.service --since "15 sec ago" --no-pager | grep -iE "poll-plan|wrote plan|ReportNode" | head -10 || echo "(no plan activity yet — give it more time)"
