#!/usr/bin/env bash
set -euo pipefail

# Reset etcd to a clean single-node cluster.
# Use after quorum loss from a failed multi-node expansion.
#
# What this does:
#   1. Stops etcd
#   2. Wipes data directory
#   3. Writes fresh single-node config with routable IP + new cluster token
#   4. Starts etcd
#   5. Restarts all globular services (they re-register in the new etcd)

if [[ $EUID -ne 0 ]]; then
  echo "Error: run as root (sudo)" >&2
  exit 1
fi

STATE_DIR="/var/lib/globular"
ETCD_CONFIG="${STATE_DIR}/config/etcd.yaml"
ETCD_DATA="${STATE_DIR}/etcd"
CA_CERT="${STATE_DIR}/pki/ca.crt"
SVC_CERT="${STATE_DIR}/pki/issued/services/service.crt"
SVC_KEY="${STATE_DIR}/pki/issued/services/service.key"

# Detect hostname and routable IP
HOSTNAME=$(hostname | sed 's/[^a-zA-Z0-9_-]/-/g; s/^-//; s/-$//')
NODE_IP=$(ip route get 8.8.8.8 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src") print $(i+1); exit}')
if [[ -z "$NODE_IP" ]]; then
  NODE_IP=$(hostname -I | awk '{print $1}')
fi
if [[ -z "$NODE_IP" ]]; then
  echo "Error: cannot detect routable IP" >&2
  exit 1
fi

# Generate unique cluster token to avoid stale raft state
TOKEN="globular-etcd-$(date +%s)"

echo "━━━ Reset etcd to single-node ━━━"
echo "  Node:    ${HOSTNAME} (${NODE_IP})"
echo "  Token:   ${TOKEN}"
echo ""

# 1. Stop etcd
echo "  → Stopping etcd..."
systemctl stop globular-etcd.service 2>/dev/null || true
sleep 1

# 2. Wipe data
echo "  → Wiping etcd data directory..."
rm -rf "${ETCD_DATA}"
mkdir -p "${ETCD_DATA}"
chown globular:globular "${ETCD_DATA}"
chmod 0750 "${ETCD_DATA}"

# 3. Write fresh config
echo "  → Writing fresh single-node config..."
mkdir -p "$(dirname "${ETCD_CONFIG}")"
cat > "${ETCD_CONFIG}" <<EOF
name: "${HOSTNAME}"
data-dir: "${ETCD_DATA}"
listen-client-urls: "https://${NODE_IP}:2379,https://127.0.0.1:2379"
advertise-client-urls: "https://${NODE_IP}:2379"
listen-peer-urls: "https://${NODE_IP}:2380"
initial-advertise-peer-urls: "https://${NODE_IP}:2380"
initial-cluster: "${HOSTNAME}=https://${NODE_IP}:2380"
initial-cluster-state: "new"
initial-cluster-token: "${TOKEN}"

client-transport-security:
  cert-file: ${SVC_CERT}
  key-file: ${SVC_KEY}

peer-transport-security:
  cert-file: ${SVC_CERT}
  key-file: ${SVC_KEY}
  trusted-ca-file: ${CA_CERT}
EOF
chown globular:globular "${ETCD_CONFIG}"

# 4. Start etcd
echo "  → Starting etcd..."
systemctl start globular-etcd.service
sleep 3

# 5. Verify
echo "  → Verifying..."
if ETCDCTL_API=3 etcdctl \
  --endpoints="https://127.0.0.1:2379" \
  --cacert="${CA_CERT}" \
  endpoint health 2>/dev/null | grep -q "is healthy"; then
  echo "  ✓ etcd healthy"
else
  echo "  ✗ etcd not healthy — check: journalctl -u globular-etcd.service -n 20"
  exit 1
fi

ETCDCTL_API=3 etcdctl \
  --endpoints="https://127.0.0.1:2379" \
  --cacert="${CA_CERT}" \
  member list

# 6. Restart all services
echo ""
echo "  → Restarting all globular services (re-register in fresh etcd)..."
systemctl restart 'globular-*.service'
sleep 5
echo "  ✓ All services restarted"

echo ""
echo "  ✓ etcd reset complete — single-node cluster ready"
echo "  ✓ Listening on ${NODE_IP}:2379 (routable) + 127.0.0.1:2379 (loopback)"
echo ""
