# Updating the Cluster

This page covers upgrading services and infrastructure components in a running Globular cluster. It explains how the desired-state model handles upgrades, how infrastructure components are upgraded safely, and how to manage the upgrade lifecycle.

## How Upgrades Work

Upgrading a service in Globular follows the same desired-state model as initial deployment. You publish a new version to the repository, update the desired state, and the platform converges:

```bash
# 1. Publish new version
globular pkg publish my_service-0.0.4-linux_amd64-1.tgz

# 2. Update desired state
globular services desired set my_service 0.0.4

# 3. Platform converges automatically
globular services desired list
```

The controller detects that the desired version (0.0.4) differs from the installed version (0.0.3), creates workflows for each affected node, and the Workflow Service orchestrates the upgrade.

## Service Upgrades

### Single Service Upgrade

```bash
globular services desired set authentication 0.0.2
```

What happens on each target node:
1. **FETCH**: Download `authentication-0.0.2` from MinIO
2. **INSTALL**: Verify checksum, extract new binary to `/usr/local/bin/`
3. **CONFIGURE**: Update configuration if needed
4. **START**: `systemctl restart authentication` — stops old version, starts new
5. **VERIFY**: Health check confirms new version is healthy

During the restart, there is a brief window where the service is unavailable on that node. If other nodes are running the same service, the Envoy gateway routes traffic to healthy instances.

### Multi-Service Upgrade

When upgrading multiple services:

```bash
globular services desired set authentication 0.0.2
globular services desired set rbac 0.0.2
globular services desired set event 0.0.2
```

The controller creates workflows for each service on each node. The concurrency semaphore (default: 3) ensures at most 3 workflows run simultaneously. Services are ordered by priority and dependency.

### Upgrade with Dependencies

If Service B depends on Service A, and both are being upgraded:
- Service A's workflow runs first (lower priority or explicit dependency)
- Service B's workflow blocks until A's health check passes
- Once A is healthy, B's workflow proceeds

```bash
# Both services need upgrades
globular services desired set etcd 3.5.15
globular services desired set authentication 0.0.2  # depends on etcd

# etcd workflows execute first (priority 10)
# authentication workflows execute after etcd is healthy (priority 30)
```

## Infrastructure Upgrades

Infrastructure components (etcd, MinIO, Prometheus, Envoy) require special care because they are critical shared services.

### etcd Upgrades

etcd is the most sensitive component to upgrade because it stores all cluster state. Upgrades are performed one node at a time:

```bash
# Publish new etcd version
globular pkg publish globular-etcd-3.5.15-linux_amd64-1.tgz

# Set desired state
globular services desired set etcd 3.5.15
```

The controller handles etcd upgrades carefully:
1. Upgrade one etcd member at a time (never lose quorum)
2. Restart the member: `systemctl restart etcd`
3. Wait for the member to rejoin and synchronize
4. Verify cluster health: `etcdctl endpoint health`
5. Proceed to the next member

In a 3-node etcd cluster, this means:
- Node 1: upgrade → restart → sync → healthy ✓
- Node 2: upgrade → restart → sync → healthy ✓
- Node 3: upgrade → restart → sync → healthy ✓

At no point does the cluster lose quorum (majority of members).

### MinIO Upgrades

MinIO uses erasure coding across nodes. Upgrades proceed similarly:

```bash
globular services desired set minio 2024.03.15
```

The controller upgrades one MinIO instance at a time, verifying data integrity after each restart.

### Prometheus and Alertmanager

Monitoring infrastructure upgrades are straightforward — they're stateless services that restart cleanly:

```bash
globular services desired set prometheus 2.52.0
globular services desired set alertmanager 0.27.0
```

### Gateway Upgrades

The Envoy gateway handles external traffic. Upgrades use the same workflow but may briefly interrupt external connectivity:

```bash
globular services desired set gateway 0.0.2
```

If multiple gateway instances exist (across nodes), upgrades happen sequentially so at least one gateway is always available.

## Upgrade Monitoring

### Before Upgrading

Check current state and identify what will change:

```bash
# Current state
globular services desired list

# Preview changes
globular services desired diff

# Check repository for available versions
globular pkg info <service-name>
```

### During Upgrade

Monitor convergence:

```bash
# Overall progress
globular services desired list

# Active workflows
globular workflow list --status EXECUTING

# Per-node status
globular cluster health

# Specific workflow details
globular workflow get <run-id>
```

### After Upgrade

Verify everything converged:

```bash
# All services should show INSTALLED
globular services desired list

# No drift
globular services repair --dry-run

# Cluster healthy
globular cluster health
```

## Handling Upgrade Failures

### Automatic Retry

If an upgrade fails due to a transient issue (network, MinIO restart), the workflow retries automatically:
- **NETWORK** failures: Retry with exponential backoff
- **REPOSITORY** failures: Retry with backoff
- **SYSTEMD** failures: One retry, then fail

### Manual Rollback

If the new version is fundamentally broken:

```bash
# Roll back to previous version
globular services desired set my_service 0.0.3

# The controller creates workflows to install 0.0.3
# Nodes running the broken 0.0.4 get downgraded
# Nodes that failed to install 0.0.4 (still on 0.0.3) see no change
```

### Partial Failure

If some nodes upgraded successfully and others failed:

```bash
globular services desired list
# my_service  0.0.4  2/3  DEGRADED

# Options:
# 1. Wait — the 5-minute backoff will trigger another attempt
# 2. Investigate the failed node:
globular workflow list --service my_service --node <failed-node> --status FAILED
globular workflow get <run-id>
# 3. Fix the underlying issue and let convergence retry
# 4. Roll back: globular services desired set my_service 0.0.3
```

## Batch Upgrades

### Upgrading All Services

To upgrade all services to their latest published versions:

```bash
# For each service, find the latest version and set desired state
for svc in authentication rbac event discovery repository; do
  latest=$(globular pkg info $svc | head -1 | awk '{print $2}')
  globular services desired set $svc $latest
done
```

### Platform-Wide Upgrade

When upgrading the entire Globular platform (all services from a new build):

```bash
# 1. Build all packages from the new source
./build-all-packages.sh

# 2. Packages are automatically published to the repository (Stage 4)

# 3. Update desired state for each service
globular services desired set authentication 0.0.2
globular services desired set rbac 0.0.2
# ... for each service

# 4. Infrastructure upgrades (one at a time, ordered)
globular services desired set etcd 3.5.15
# Wait for completion...
globular services desired set minio 2024.03.15
# Wait for completion...
```

## Practical Scenarios

### Scenario 1: Minor Version Bump

Upgrading the monitoring service from 0.0.5 to 0.0.6:

```bash
globular pkg publish globular-monitoring-0.0.6-linux_amd64-1.tgz
globular services desired set monitoring 0.0.6

# Monitor
globular services desired list
# monitoring  0.0.6  APPLYING → INSTALLED (1-2 minutes)

# Deprecate old version
globular pkg deprecate monitoring 0.0.5
```

### Scenario 2: Emergency Rollback

A critical authentication bug is discovered after upgrading:

```bash
# Current state
globular services desired list
# authentication  0.0.3  3/3  INSTALLED  (this is the broken version)

# Immediate rollback
globular services desired set authentication 0.0.2

# Monitor — should converge within 2-3 minutes
globular services desired list
# authentication  0.0.2  3/3  INSTALLED

# Yank the broken version to prevent re-deployment
globular pkg yank authentication 0.0.3
```

### Scenario 3: etcd Cluster Upgrade

Upgrading etcd in a 3-node cluster from 3.5.14 to 3.5.15:

```bash
# Verify cluster health before starting
globular cluster health
# All 3 nodes healthy, etcd quorum intact

# Publish and upgrade
globular pkg publish globular-etcd-3.5.15-linux_amd64-1.tgz
globular services desired set etcd 3.5.15

# The controller upgrades one node at a time:
# Node 1: restart etcd → rejoin → sync → healthy ✓ (quorum: 2/3)
# Node 2: restart etcd → rejoin → sync → healthy ✓ (quorum: 2/3)
# Node 3: restart etcd → rejoin → sync → healthy ✓ (quorum: 3/3)

# Monitor
globular services desired list
# etcd  3.5.15  1/3  APPLYING
# ...
# etcd  3.5.15  3/3  INSTALLED

globular cluster health
# All nodes healthy, etcd quorum intact
```

## What's Next

- [Debugging Failures](debugging-failures.md): Diagnose deployment and service problems
- [Observability](observability.md): Prometheus, logs, and workflow monitoring
