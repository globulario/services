#!/usr/bin/env bash
set -euo pipefail

echo "=== Stopping ALL services ==="
sudo systemctl stop globular-cluster-controller.service || true
sudo systemctl stop globular-etcd.service || true
sleep 2

echo "=== Deploying FIXED controller binary ==="
sudo cp /home/dave/Documents/github.com/globulario/services/golang/cluster_controller_server /usr/lib/globular/bin/cluster_controller_server
echo "Deployed. Checksums:"
md5sum /home/dave/Documents/github.com/globulario/services/golang/cluster_controller_server /usr/lib/globular/bin/cluster_controller_server

echo "=== Completely wiping etcd data ==="
sudo rm -rf /var/lib/globular/etcd
sudo mkdir -p /var/lib/globular/etcd
sudo chown globular:globular /var/lib/globular/etcd
sudo chmod 0750 /var/lib/globular/etcd

echo "=== Writing single-node etcd config ==="
sudo tee /var/lib/globular/config/etcd.yaml > /dev/null <<'YAML'
name: "globule-ryzen"
data-dir: "/var/lib/globular/etcd"
listen-client-urls: "https://10.0.0.63:2379,https://127.0.0.1:2379"
advertise-client-urls: "https://10.0.0.63:2379"
listen-peer-urls: "https://10.0.0.63:2380"
initial-advertise-peer-urls: "https://10.0.0.63:2380"
initial-cluster: "globule-ryzen=https://10.0.0.63:2380"
initial-cluster-state: "new"
initial-cluster-token: "globular-reset-20260320b"

client-transport-security:
  cert-file: /var/lib/globular/pki/issued/services/service.crt
  key-file: /var/lib/globular/pki/issued/services/service.key
  trusted-ca-file: /var/lib/globular/pki/ca.crt

peer-transport-security:
  cert-file: /var/lib/globular/pki/issued/services/service.crt
  key-file: /var/lib/globular/pki/issued/services/service.key
  trusted-ca-file: /var/lib/globular/pki/ca.crt
YAML

echo "=== Writing single-node etcd endpoints ==="
echo "https://10.0.0.63:2379" | sudo tee /var/lib/globular/config/etcd_endpoints > /dev/null

echo "=== Starting etcd ==="
sudo systemctl start globular-etcd.service
sleep 3

echo "=== Checking etcd member list (must show exactly 1 member) ==="
sudo ETCDCTL_API=3 etcdctl --endpoints=https://127.0.0.1:2379 --cacert=/var/lib/globular/pki/ca.crt --cert=/var/lib/globular/pki/issued/services/service.crt --key=/var/lib/globular/pki/issued/services/service.key member list

echo ""
echo "=== Checking etcd health ==="
sudo ETCDCTL_API=3 etcdctl --endpoints=https://127.0.0.1:2379 --cacert=/var/lib/globular/pki/ca.crt --cert=/var/lib/globular/pki/issued/services/service.crt --key=/var/lib/globular/pki/issued/services/service.key endpoint health

echo ""
echo "=== Starting controller (with fixed binary) ==="
sudo systemctl start globular-cluster-controller.service
sleep 5

echo "=== Controller status ==="
systemctl status globular-cluster-controller.service --no-pager | head -15

echo ""
echo "=== Controller logs (checking no member-add) ==="
journalctl -u globular-cluster-controller.service --since "10 sec ago" --no-pager | grep -E "member-add|etcd|leader|ReportNode" || echo "(no etcd member-add activity - GOOD)"

echo ""
echo "=== Done! ==="
