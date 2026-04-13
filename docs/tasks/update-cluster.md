# Task: Update the Cluster

## Overview

Upgrade one or more services to new versions across the cluster. The desired-state model handles rolling upgrades automatically.

Use this when:
- A new service version has been published
- You need to apply a bug fix or security patch
- You want to upgrade infrastructure components (etcd, MinIO, Prometheus)

## Prerequisites

- Cluster is healthy (`globular cluster health`)
- New version is published (`globular pkg info <service>` shows PUBLISHED)
- Authenticated CLI session with operator permissions

## Steps

### Step 1: Check current state

```bash
# Current desired state
globular services desired list

# Available versions
globular pkg info <service>
```

### Step 2: Set the new desired version

```bash
globular services desired set <service> <new-version>
```

Example:
```bash
globular services desired set monitoring 0.0.6
```

For infrastructure upgrades, proceed one service at a time:
```bash
# Upgrade etcd first (most critical)
globular services desired set etcd 3.5.15
# Wait for INSTALLED before proceeding

# Then other infrastructure
globular services desired set prometheus 2.52.0
```

### Step 3: Monitor the upgrade

```bash
# Watch convergence
globular services desired list

# Watch active workflows
globular workflow list --status EXECUTING

# Cluster health during upgrade
globular cluster health
```

### Step 4: Handle failures (if any)

If a service shows FAILED or DEGRADED:

```bash
# Find the failed workflow
globular workflow list --service <service> --status FAILED
globular workflow get <run-id>

# Check the failure class and step
# Then decide: fix and retry, or roll back
```

### Step 5: Roll back if needed

```bash
globular services desired set <service> <previous-version>
```

The platform creates new workflows to install the previous version on all nodes.

## Verification

```bash
# All services at new version
globular services desired list
# Shows: <service> <new-version> N/N INSTALLED

# No drift
globular services repair --dry-run

# Cluster healthy
globular cluster health
```

## Troubleshooting

### Upgrade stuck at APPLYING

```bash
globular workflow list --service <service> --status EXECUTING
globular workflow get <run-id>
# Check which step is running and on which node
```

Common causes:
- **FETCH stuck**: MinIO slow or unreachable. Check MinIO health.
- **START stuck**: Service takes long to start. Check logs.
- **VERIFY stuck**: Health check not responding. Check service logs.

### Service crashes after upgrade

```bash
# Check logs on the affected node
globular node logs --node <node>:11000 --unit <service> --lines 200

# Roll back immediately
globular services desired set <service> <previous-version>

# Yank the bad version to prevent re-deployment
globular pkg yank <service> <bad-version>
```

### etcd upgrade fails

etcd upgrades are sequential (one node at a time to maintain quorum). If one node fails:

```bash
# Check etcd health
globular cluster health

# Check etcd logs on the failed node
globular node logs --node <node>:11000 --unit etcd --lines 100

# If the etcd member is corrupt, remove and re-add:
# (This is handled by the controller automatically in most cases)
```

### Multiple services need upgrading

Upgrade them in dependency order:
```bash
# 1. Infrastructure (etcd, MinIO) — wait for INSTALLED
globular services desired set etcd 3.5.15
# Wait...

# 2. Security (auth, RBAC) — wait for INSTALLED
globular services desired set authentication 0.0.2
# Wait...

# 3. Platform services (event, discovery, repository)
globular services desired set event 0.0.2
globular services desired set repository 0.0.2

# 4. Application services
globular services desired set my_service 0.0.4
```
