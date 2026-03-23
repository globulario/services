#!/usr/bin/env bash
set -euo pipefail

# Fix remote node agents: add REPOSITORY_ADDRESS to their systemd unit
# and restart. This is a one-time fix — future joins will include it.

NODES=("10.0.0.20" "10.0.0.214")
REPO_ADDR="10.0.0.63:443"

for node in "${NODES[@]}"; do
    echo "=== Fixing $node ==="
    ssh "dave@${node}" "
        # Add REPOSITORY_ADDRESS to the main systemd unit (not a drop-in)
        if ! grep -q 'REPOSITORY_ADDRESS' /etc/systemd/system/globular-node-agent.service 2>/dev/null; then
            sudo sed -i '/^Environment=ETCD_ENDPOINTS/a Environment=REPOSITORY_ADDRESS=${REPO_ADDR}' /etc/systemd/system/globular-node-agent.service
        else
            echo 'REPOSITORY_ADDRESS already in unit file'
        fi
        # Also update drop-in if it exists with wrong value
        if [ -f /etc/systemd/system/globular-node-agent.service.d/repository.conf ]; then
            echo '[Service]' | sudo tee /etc/systemd/system/globular-node-agent.service.d/repository.conf > /dev/null
            echo 'Environment=REPOSITORY_ADDRESS=${REPO_ADDR}' | sudo tee -a /etc/systemd/system/globular-node-agent.service.d/repository.conf > /dev/null
        fi
        sudo systemctl daemon-reload
        sudo systemctl restart globular-node-agent.service
        echo 'Done'
        sudo systemctl show globular-node-agent.service -p Environment | grep REPO
    " 2>&1 || echo "FAILED on $node"
    echo ""
done

echo "=== Resetting stale plans on ryzen ==="
sudo bash /home/dave/Documents/github.com/globulario/services/nuke-and-restart.sh

echo "=== All done ==="
