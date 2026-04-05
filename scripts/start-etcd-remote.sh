#!/usr/bin/env bash
set -euo pipefail

# Run this on each remote node (dell, nuc) to create and start the etcd service.
# The config file was already written by the join script.

INSTALL_DIR="/usr/lib/globular/bin"
STATE_DIR="/var/lib/globular"
SYSTEMD_DIR="/etc/systemd/system"

if [[ ! -f "${STATE_DIR}/config/etcd.yaml" ]]; then
  echo "ERROR: etcd config not found at ${STATE_DIR}/config/etcd.yaml"
  exit 1
fi

if [[ ! -f "${INSTALL_DIR}/etcd" ]]; then
  echo "ERROR: etcd binary not found at ${INSTALL_DIR}/etcd"
  exit 1
fi

echo "=== Creating etcd systemd unit ==="
cat > "${SYSTEMD_DIR}/globular-etcd.service" <<UNIT
[Unit]
Description=Globular etcd
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=globular
Group=globular
ExecStartPre=/usr/bin/mkdir -p ${STATE_DIR}/etcd
ExecStartPre=/usr/bin/chown globular:globular ${STATE_DIR}/etcd
ExecStartPre=/usr/bin/chmod 0750 ${STATE_DIR}/etcd
ExecStart=${INSTALL_DIR}/etcd --config-file ${STATE_DIR}/config/etcd.yaml
Restart=on-failure
RestartSec=5
LimitNOFILE=524288

[Install]
WantedBy=multi-user.target
UNIT

echo "=== Adding this node as etcd member ==="
BOOTSTRAP_ETCD="https://10.0.0.63:2379"
CACERT="${STATE_DIR}/pki/ca.crt"
NODE_IP=$(hostname -I | awk '{print $1}')
NODE_NAME=$(hostname | sed 's/[^a-zA-Z0-9_-]/-/g')

CERT="${STATE_DIR}/pki/issued/services/service.crt"
KEY="${STATE_DIR}/pki/issued/services/service.key"

ETCDCTL_API=3 "${INSTALL_DIR}/etcdctl" \
  --endpoints="${BOOTSTRAP_ETCD}" \
  --cacert="${CACERT}" \
  --cert="${CERT}" \
  --key="${KEY}" \
  member add "${NODE_NAME}" --peer-urls="https://${NODE_IP}:2380" 2>&1 || echo "(may already be added)"

echo "=== Starting etcd ==="
mkdir -p "${STATE_DIR}/etcd"
chown -R globular:globular "${STATE_DIR}/etcd" "${STATE_DIR}/config"
systemctl daemon-reload
systemctl enable globular-etcd.service
systemctl start globular-etcd.service
sleep 5

echo "=== Checking health ==="
ETCDCTL_API=3 "${INSTALL_DIR}/etcdctl" \
  --endpoints="https://127.0.0.1:2379" \
  --cacert="${CACERT}" \
  --cert="${CERT}" \
  --key="${KEY}" \
  endpoint health 2>&1 || echo "etcd not healthy yet — check: journalctl -u globular-etcd.service"

echo "=== Done ==="
