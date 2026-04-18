# Task: Deploy an Application

## Overview

Deploy a service to the Globular cluster using the desired-state model. This task covers publishing a pre-built package and triggering deployment through the convergence system.

Use this when:
- You have a built `.tgz` package ready to deploy
- You want to deploy a new service or upgrade an existing one
- You want the platform to handle installation across all target nodes

## Prerequisites

- A running Globular cluster (`globular cluster health` shows HEALTHY)
- Authenticated CLI session (`globular auth login`)
- A built package file (`.tgz`) or an existing package in the repository

## Steps

### Step 1: Verify the package is in the repository

```bash
globular pkg info <service-name>
```

If the package is not published yet, publish it first:

```bash
globular pkg publish <package-file.tgz>
```

Confirm it shows `PUBLISHED`:
```bash
globular pkg info <service-name>
# NAME          VERSION  BUILD  PLATFORM     STATE       PUBLISHED AT
# my_service    0.0.3    1      linux_amd64  PUBLISHED   2025-04-12 10:30:00
```

### Step 2: Set the desired state

```bash
globular services desired set <service-name> <version>
```

Example:
```bash
globular services desired set my_service 0.0.3 --publisher core@globular.io
```

### Step 3: Monitor deployment progress

```bash
globular services desired list
```

Watch for the status to change:
- `PENDING` → workflows being created
- `APPLYING` → workflows executing on nodes
- `INSTALLED` → all target nodes converged

For detailed workflow progress:
```bash
globular workflow list --service <service-name> --status EXECUTING
```

### Step 4: Wait for convergence

If the status stays at `APPLYING`, check which nodes are still in progress:
```bash
globular workflow list --service <service-name>
```

Typical deployment takes 1-3 minutes per node.

## Verification

Confirm the deployment succeeded:

```bash
# All nodes converged
globular services desired list
# Shows: <service-name> <version> N/N INSTALLED

# No drift
globular cluster get-drift-report
# Shows: INSTALLED for the service on all nodes

# Cluster healthy
globular cluster health
# All nodes healthy, correct service count
```

## Troubleshooting

### Status stuck at APPLYING

```bash
# Check which workflow is in progress
globular workflow list --service <service-name> --status EXECUTING
globular workflow get <run-id>
# Look at the current step — it shows where the workflow is stuck
```

**Common causes**: MinIO down (FETCH phase stuck), service crash on startup (START phase failed), health check timeout (VERIFY phase slow).

### Status shows FAILED

```bash
# Find the failed workflow
globular workflow list --service <service-name> --status FAILED
globular workflow get <run-id>
```

Check the `FailureClass`:
- **REPOSITORY**: Package not found or MinIO unavailable. Check `globular pkg info <service>` and MinIO health.
- **SYSTEMD**: Service binary crashes on startup. Check logs: `globular node logs --node <node>:11000 --unit <service> --lines 100`
- **VALIDATION**: Checksum mismatch. The package may be corrupt. Republish: `globular pkg publish <package.tgz>`
- **NETWORK**: Node can't reach MinIO or controller. Check network connectivity.

### Status shows DEGRADED

Some nodes converged, others failed. Check per-node status:
```bash
globular workflow list --service <service-name>
# Identify which nodes failed and why
```

Fix the failing nodes, or roll back:
```bash
globular services desired set <service-name> <previous-version>
```

### Wrong version deployed

```bash
# Check what's installed
globular services desired list
# If wrong version, set the correct one:
globular services desired set <service-name> <correct-version>
```
