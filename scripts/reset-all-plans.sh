#!/usr/bin/env bash
set -euo pipefail

CACERT=/var/lib/globular/pki/ca.crt
CERT=/var/lib/globular/pki/issued/services/service.crt
KEY=/var/lib/globular/pki/issued/services/service.key
EP=https://127.0.0.1:2379

echo "=== Stopping controller ==="
systemctl stop globular-cluster-controller.service

echo "=== Listing plan keys (correct prefix) ==="
ETCDCTL_API=3 etcdctl --endpoints=$EP --cacert=$CACERT --cert=$CERT --key=$KEY get globular/plans/ --prefix --keys-only | head -20

echo ""
echo "=== Deleting plan keys ==="
ETCDCTL_API=3 etcdctl --endpoints=$EP --cacert=$CACERT --cert=$CERT --key=$KEY del globular/plans/ --prefix

echo "=== Deleting lock keys ==="
ETCDCTL_API=3 etcdctl --endpoints=$EP --cacert=$CACERT --cert=$CERT --key=$KEY del globular/plans/v1/locks/ --prefix

echo "=== Deleting release objects ==="
ETCDCTL_API=3 etcdctl --endpoints=$EP --cacert=$CACERT --cert=$CERT --key=$KEY del /globular/resources/ServiceRelease/ --prefix
ETCDCTL_API=3 etcdctl --endpoints=$EP --cacert=$CACERT --cert=$CERT --key=$KEY del /globular/resources/InfrastructureRelease/ --prefix

echo "=== Deleting ghost node installed-state ==="
ETCDCTL_API=3 etcdctl --endpoints=$EP --cacert=$CACERT --cert=$CERT --key=$KEY del /globular/nodes/4c2b3cb3-d02a-56d3-93cf-4e2c8728e8a4/ --prefix
ETCDCTL_API=3 etcdctl --endpoints=$EP --cacert=$CACERT --cert=$CERT --key=$KEY del /globular/nodes/814fbbb9-607f-5144-be1a-a863a0bea1e1/ --prefix

echo ""
echo "=== Verifying plans cleaned ==="
COUNT=$(ETCDCTL_API=3 etcdctl --endpoints=$EP --cacert=$CACERT --cert=$CERT --key=$KEY get globular/plans/ --prefix --keys-only | wc -l)
echo "Remaining plan keys: $COUNT"

echo ""
echo "=== Starting controller ==="
systemctl start globular-cluster-controller.service
sleep 10

echo "=== Activity check ==="
journalctl -u globular-cluster-controller.service --since "10 sec ago" --no-pager | grep -iE "wrote plan|import|created with phase|APPLYING" | head -15

echo "=== Done ==="
