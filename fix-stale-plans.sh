#!/usr/bin/env bash
set -euo pipefail

CACERT=/var/lib/globular/pki/ca.crt
CERT=/var/lib/globular/pki/issued/services/service.crt
KEY=/var/lib/globular/pki/issued/services/service.key
EP=https://127.0.0.1:2379

echo "=== Listing all globular keys ==="
ETCDCTL_API=3 etcdctl --endpoints=$EP --cacert=$CACERT --cert=$CERT --key=$KEY get /globular/ --prefix --keys-only | head -40

echo ""
echo "=== Deleting plan keys ==="
ETCDCTL_API=3 etcdctl --endpoints=$EP --cacert=$CACERT --cert=$CERT --key=$KEY del /globular/plans/ --prefix
echo ""

echo "=== Deleting release keys ==="
ETCDCTL_API=3 etcdctl --endpoints=$EP --cacert=$CACERT --cert=$CERT --key=$KEY del /globular/resources/ServiceRelease/ --prefix
ETCDCTL_API=3 etcdctl --endpoints=$EP --cacert=$CACERT --cert=$CERT --key=$KEY del /globular/resources/InfrastructureRelease/ --prefix
echo ""

echo "=== Starting controller ==="
systemctl start globular-cluster-controller.service
sleep 10

echo "=== Checking activity ==="
journalctl -u globular-cluster-controller.service --since "10 sec ago" --no-pager | grep -iE "wrote plan|import|APPLYING|AVAILABLE|leader" | head -15

echo "=== Done! ==="
