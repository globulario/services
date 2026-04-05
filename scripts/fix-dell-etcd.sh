#!/usr/bin/env bash
set -euo pipefail

cat > /var/lib/globular/config/etcd.yaml <<'EOF'
name: "globule-dell"
data-dir: "/var/lib/globular/etcd"
listen-client-urls: "https://10.0.0.20:2379,https://127.0.0.1:2379"
advertise-client-urls: "https://10.0.0.20:2379"
listen-peer-urls: "https://10.0.0.20:2380"
initial-advertise-peer-urls: "https://10.0.0.20:2380"
initial-cluster: "globule-dell=https://10.0.0.20:2380,globule-ryzen=https://10.0.0.63:2380"
initial-cluster-state: "existing"
initial-cluster-token: "globular-reset-20260320b"

client-transport-security:
  cert-file: /var/lib/globular/pki/issued/services/service.crt
  key-file: /var/lib/globular/pki/issued/services/service.key
  trusted-ca-file: /var/lib/globular/pki/ca.crt

peer-transport-security:
  cert-file: /var/lib/globular/pki/issued/services/service.crt
  key-file: /var/lib/globular/pki/issued/services/service.key
  trusted-ca-file: /var/lib/globular/pki/ca.crt
EOF

chown -R globular:globular /var/lib/globular/etcd /var/lib/globular/config
systemctl start globular-etcd.service
sleep 5

ETCDCTL_API=3 /usr/lib/globular/bin/etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/var/lib/globular/pki/ca.crt \
  --cert=/var/lib/globular/pki/issued/services/service.crt \
  --key=/var/lib/globular/pki/issued/services/service.key \
  endpoint health

echo "=== Member list ==="
ETCDCTL_API=3 /usr/lib/globular/bin/etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/var/lib/globular/pki/ca.crt \
  --cert=/var/lib/globular/pki/issued/services/service.crt \
  --key=/var/lib/globular/pki/issued/services/service.key \
  member list
