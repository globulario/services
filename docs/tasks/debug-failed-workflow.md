# Task: Debug a Failed Workflow

## Overview

Diagnose why a deployment, upgrade, or repair workflow failed. Identify the root cause and resolve it.

Use this when:
- A service shows FAILED or DEGRADED in `services desired list`
- A workflow is stuck in EXECUTING for too long
- You need to understand why a service isn't converging

## Prerequisites

- Access to the Globular CLI
- Authenticated session

## Steps

### Step 1: Find the failed workflow

```bash
# List failed workflows
globular workflow list --status FAILED

# Or for a specific service
globular workflow list --service <service> --status FAILED

# Or for a specific node
globular workflow list --node <node-id> --status FAILED
```

Output:
```
RUN ID          SERVICE      NODE         STATUS  TRIGGER        STARTED
wf-run-abc123   postgresql   node-ghi789  FAILED  DESIRED_DRIFT  10m ago
```

### Step 2: Examine the workflow details

```bash
globular workflow get wf-run-abc123
```

Output:
```
Run ID:         wf-run-abc123
Correlation:    service/postgresql/node-ghi789
Status:         FAILED
Failure Class:  SYSTEMD
Retry Count:    2
Trigger:        DESIRED_DRIFT

STEPS:
  1. resolve_artifact     SUCCEEDED   0.5s
  2. fetch_package        SUCCEEDED  12.3s
  3. verify_checksum      SUCCEEDED   0.1s
  4. install_binary       SUCCEEDED   2.1s
  5. configure_service    SUCCEEDED   0.8s
  6. start_unit           FAILED      5.0s
     Error: "unit postgresql exited with status 1 after 3.2s"
  7. verify_health        SKIPPED
```

Note the **failed step** and the **failure class**.

### Step 3: Diagnose based on failure class

#### SYSTEMD — Service crashed on startup

```bash
# Check service logs on the affected node
globular node logs --node <node>:11000 --unit <service> --lines 200

# Search for specific errors
globular node search-logs --node <node>:11000 --unit <service> --pattern "error|panic|fatal"
```

Common causes: port conflict, missing dependency, configuration error, binary bug.

#### REPOSITORY — Package not found or unavailable

```bash
# Check if the artifact exists
globular pkg info <service>

# Check MinIO health
globular cluster health
```

Common causes: MinIO down, artifact yanked/revoked, wrong version specified.

#### NETWORK — Connectivity failure

```bash
# Check node connectivity
globular cluster health
# Is the affected node reachable?

# Check specific endpoints
globular node logs --node <node>:11000 --unit <service> --pattern "connection refused|timeout"
```

Common causes: firewall rules, DNS failure, MinIO unreachable.

#### VALIDATION — Checksum or integrity failure

```bash
# The downloaded package doesn't match the manifest
# Clear the cache and retry
globular node clear-cache --node <node>:11000 --package <service>
```

If it keeps failing, the artifact may be corrupt. Republish:
```bash
globular pkg publish <corrected-package.tgz>
```

#### DEPENDENCY — Upstream service unavailable

```bash
# Check which dependency is missing
# The workflow error message usually names it

# Check if the dependency is installed
globular services desired list
# Look for the dependency service

# If not installed, deploy it first
globular services desired set <dependency> <version>
```

#### CONFIG — Configuration error

```bash
# Check etcd for the service's config
# Look for missing or invalid values

# The error message usually indicates which config key is missing
```

### Step 4: Fix the root cause

Based on your diagnosis:

- **Binary bug**: Roll back: `globular services desired set <service> <previous-version>`
- **Port conflict**: Find and stop the conflicting process on the node
- **Missing dependency**: Deploy the dependency first
- **Corrupt package**: Republish the package
- **Network issue**: Fix the network and wait for auto-retry (5-minute backoff)
- **Config error**: Fix the configuration in etcd

### Step 5: Trigger retry (or wait)

After fixing the root cause:

```bash
# Option 1: Wait for automatic retry (5-minute backoff after FAILED)
# The reconciler will create a new workflow automatically

# Option 2: Force immediate retry
globular services apply-desired

# Option 3: Remove and re-set desired state (creates fresh workflow)
globular services desired remove <service>
globular services desired set <service> <version>
```

## Verification

```bash
# New workflow should succeed
globular workflow list --service <service>
# Shows: latest run SUCCEEDED

# Service is now installed
globular services desired list
# Shows: <service> INSTALLED

# No drift
globular services repair --dry-run
```

## Troubleshooting

### Workflow keeps failing after fix

Check if there are multiple failure causes:
```bash
globular workflow list --service <service>
# Look at the most recent run's error, not just the original
```

### Can't find the workflow

The workflow may have been superseded:
```bash
# List all workflows for the service (including superseded)
globular workflow list --service <service>
```

### Workflow stuck in BLOCKED

A dependency hasn't been met. Check what's blocking:
```bash
globular workflow get <run-id>
# The error message names the blocking dependency
```

### Multiple services failing

Use the doctor for a comprehensive view:
```bash
globular doctor report --fresh
# Shows all findings with severity and remediation
```
