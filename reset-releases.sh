#!/usr/bin/env bash
set -euo pipefail

CACERT=/var/lib/globular/pki/ca.crt
CERT=/var/lib/globular/pki/issued/services/service.crt
KEY=/var/lib/globular/pki/issued/services/service.key
EP=https://127.0.0.1:2379

echo "=== Stopping controller ==="
systemctl stop globular-cluster-controller.service

echo "=== Deleting all release objects ==="
ETCDCTL_API=3 etcdctl --endpoints=$EP --cacert=$CACERT --cert=$CERT --key=$KEY del /globular/resources/ServiceRelease/ --prefix
ETCDCTL_API=3 etcdctl --endpoints=$EP --cacert=$CACERT --cert=$CERT --key=$KEY del /globular/resources/InfrastructureRelease/ --prefix

echo "=== Deleting all plan data ==="
ETCDCTL_API=3 etcdctl --endpoints=$EP --cacert=$CACERT --cert=$CERT --key=$KEY del /globular/plans/ --prefix

echo "=== Starting controller ==="
systemctl start globular-cluster-controller.service
sleep 10

echo "=== Checking releases ==="
journalctl -u globular-cluster-controller.service --since "10 sec ago" --no-pager | grep -iE "wrote plan|APPLYING|import|created with phase" | head -20

echo "=== Done ==="
