# Debugging Failures

This page covers how to diagnose and resolve problems in a Globular cluster. It explains the diagnostic tools available, common failure patterns, and systematic approaches to troubleshooting.

## Diagnostic Approach

When something goes wrong in a Globular cluster, the 4-layer state model provides a structured diagnostic framework. Instead of guessing, you systematically check each layer:

1. **Layer 1 (Repository)**: Does the artifact exist? Is it PUBLISHED? Is the checksum valid?
2. **Layer 2 (Desired)**: Is the desired state set correctly? Does the version match what's in the repository?
3. **Layer 3 (Installed)**: Is the correct version installed on the node? Does the checksum match?
4. **Layer 4 (Runtime)**: Is the service running? Is it passing health checks?

The `repair --dry-run` command performs this comparison automatically:

```bash
globular services repair --dry-run
```

This shows the status of every service across all four layers, immediately highlighting where the problem lies.

## Workflow Diagnostics

Most problems in Globular surface as workflow failures. When a deployment or upgrade fails, the workflow records exactly what went wrong.

### Listing Failed Workflows

```bash
# All failed workflows
globular workflow list --status FAILED

# Failed workflows for a specific service
globular workflow list --service postgresql --status FAILED

# Failed workflows on a specific node
globular workflow list --node node-abc123 --status FAILED
```

### Examining a Failed Workflow

```bash
globular workflow get <run-id>
```

Output:
```
Run ID:         wf-run-abc123
Correlation:    service/postgresql/node-ghi789
Status:         FAILED
Trigger:        DESIRED_DRIFT
Failure Class:  SYSTEMD
Retry Count:    2
Started:        2025-04-12T10:30:00Z
Ended:          2025-04-12T10:35:22Z

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

This tells you:
- The artifact was found and valid (steps 1-3 passed)
- The binary was installed correctly (step 4 passed)
- Configuration was written (step 5 passed)
- The systemd unit failed to start (step 6 failed)
- The failure class is SYSTEMD — the binary crashed after starting

### Following the Retry Chain

Workflows form a chain when retried:

```bash
# The parent_run_id links retries to the original
globular workflow get <parent-run-id>
# Shows the original failure

globular workflow get <retry-run-id>
# Shows: parent_run_id = <parent-run-id>, retry_count = 1
```

## Service Logs

### Checking Service Logs

When a service fails to start or crashes, check its journal output:

```bash
# Via the Node Agent (remote, works from any machine):
globular node logs --node node-abc123:11000 --unit postgresql --lines 100

# Via the Node Agent search (pattern matching):
globular node search-logs --node node-abc123:11000 --unit postgresql --pattern "error|fatal|panic"
```

The Node Agent's `GetServiceLogs` and `SearchServiceLogs` RPCs wrap journalctl, providing remote access to service logs without SSH.

### Common Log Patterns

**Port conflict**:
```
listen tcp :5432: bind: address already in use
```
Another process is using the service's port. Check with `ss -tlnp | grep <port>`.

**Missing dependency**:
```
failed to connect to etcd: context deadline exceeded
```
The service depends on etcd, which isn't reachable. Check if etcd is running and the endpoint in etcd is correct.

**Configuration error**:
```
config validation failed: missing required field "database_url"
```
A required configuration key is missing from etcd. Check the service's config in etcd.

**Certificate error**:
```
tls: failed to verify certificate: x509: certificate signed by unknown authority
```
The service doesn't trust the cluster CA. Certificate may need re-provisioning.

## Cluster Doctor

The Cluster Doctor is an automated diagnostic tool that continuously monitors cluster health and identifies problems.

### Getting a Doctor Report

```bash
# Cached report (fast, uses recent snapshot)
globular doctor report

# Fresh report (slower, collects live data from all nodes)
globular doctor report --fresh
```

Output:
```
CLUSTER STATUS: DEGRADED

FINDINGS:
  CRITICAL  postgresql unit stopped on node-2
            Evidence: systemctl status = inactive (dead)
            Remediation: systemctl restart postgresql

  WARN      monitoring service version mismatch on node-3
            Evidence: desired=0.0.6, installed=0.0.5
            Remediation: workflow pending, will auto-resolve

  INFO      etcd member node-4 not yet synced
            Evidence: raft applied index behind leader by 1200
            Remediation: wait for sync (typically < 5 minutes)

TOP ISSUES: [postgresql_stopped, monitoring_version_mismatch]
FINDINGS: 3 (1 critical, 1 warning, 1 info)
```

### Explaining a Finding

Get detailed explanation and remediation steps for a specific finding:

```bash
globular doctor explain <finding-id>
```

Output:
```
FINDING: postgresql_stopped

Why this happened:
  The postgresql systemd unit on node-2 is in 'inactive (dead)' state.
  This could be caused by:
  - Service binary crash (check journalctl)
  - Out of memory (check dmesg for OOM killer)
  - Manual stop (check audit log)
  - Dependency failure (check etcd connectivity)

Evidence:
  - systemctl status postgresql = inactive (dead)
  - Last active: 2025-04-12T09:15:00Z (1h ago)
  - Exit code: 137 (SIGKILL — likely OOM)

Recommended actions:
  1. Check journal: journalctl -u postgresql --since "1 hour ago"
  2. Check system memory: free -h, dmesg | grep -i oom
  3. If OOM: increase memory limits in systemd unit
  4. Restart: systemctl restart postgresql
  5. Verify: check health endpoint
```

### Drift Report

The drift report specifically focuses on desired-vs-actual mismatches:

```bash
globular doctor drift-report
```

Output:
```
DRIFT ITEMS:
  NODE       SERVICE       DESIRED   INSTALLED  CATEGORY
  node-2     postgresql    0.0.3     0.0.3      UNIT_STOPPED
  node-3     monitoring    0.0.6     0.0.5      VERSION_MISMATCH
  node-1     dns           0.0.1     0.0.1      ENDPOINT_MISSING
```

**Drift categories**:
- `MISSING_UNIT_FILE`: systemd unit file doesn't exist
- `UNIT_STOPPED`: Service installed but not running
- `UNIT_DISABLED`: Service disabled in systemd
- `VERSION_MISMATCH`: Installed version differs from desired
- `STATE_HASH_MISMATCH`: Node's applied hash doesn't match expected
- `ENDPOINT_MISSING`: Service running but not registered in etcd
- `INVENTORY_INCOMPLETE`: InstalledPackage record missing or incomplete

### Auto-Heal

The Cluster Doctor can automatically remediate certain issues. The healer loop runs on the leader doctor and operates in three modes:

**observe** (default): Classify findings only, no action taken
```bash
globular doctor set-mode observe
```

**dry_run**: Show what actions would be taken, but don't execute
```bash
globular doctor set-mode dry_run
```

**enforce**: Execute auto-heal actions for low-risk findings
```bash
globular doctor set-mode enforce
```

Auto-healable actions:
- `SYSTEMCTL_RESTART`: Restart a stopped service (RISK_LOW)
- `SYSTEMCTL_START`: Start a disabled service (RISK_LOW)

Actions requiring manual approval:
- `ETCD_PUT` / `ETCD_DELETE`: Modify etcd state (RISK_MEDIUM/HIGH)
- `PACKAGE_REINSTALL`: Reinstall a package (RISK_MEDIUM)
- `NODE_REMOVE`: Remove a node (RISK_HIGH)

The healer is rate-limited (configurable `max_actions_per_cycle`) and has a circuit breaker that stops after 3 consecutive failures.

### Heal History

View the audit trail of auto-heal actions:

```bash
globular doctor heal-history
```

Output:
```
TIME                  ACTION              TARGET                  STATUS
2025-04-12 10:30:00   SYSTEMCTL_RESTART   postgresql@node-2       EXECUTED ✓
2025-04-12 10:25:00   SYSTEMCTL_RESTART   monitoring@node-3       EXECUTED ✓
2025-04-12 10:20:00   SYSTEMCTL_START     dns@node-1              EXECUTED ✓
```

## Common Failure Patterns

### Pattern 1: Service Crashes After Upgrade

**Symptoms**: Service enters FAILED state immediately after upgrade. Workflow shows `FailureClass: SYSTEMD` at the START phase.

**Diagnosis**:
```bash
# Check workflow
globular workflow list --service <service> --status FAILED
globular workflow get <run-id>

# Check logs
globular node logs --node <node>:11000 --unit <service> --lines 200
```

**Common causes**:
- Binary incompatibility (wrong platform, missing library)
- Configuration change required by new version
- Port conflict with another service
- Insufficient memory

**Resolution**: Fix the root cause, or roll back:
```bash
globular services desired set <service> <previous-version>
```

### Pattern 2: Fetch Failures

**Symptoms**: Workflow fails at FETCH phase with `FailureClass: REPOSITORY` or `FailureClass: NETWORK`.

**Diagnosis**:
```bash
# Check MinIO health
globular cluster health
# Look for MinIO in the service list

# Check if the artifact exists
globular pkg info <service>
# Verify the version is PUBLISHED
```

**Common causes**:
- MinIO is down or unreachable
- Artifact was yanked or revoked
- Network partition between nodes
- DNS resolution failure

**Resolution**: If MinIO is down, wait for it to recover (workflows will auto-retry). If the artifact was removed, republish it.

### Pattern 3: Dependency Deadlock

**Symptoms**: Multiple services stuck in BLOCKED state, none progressing.

**Diagnosis**:
```bash
globular workflow list --status BLOCKED

# Check each blocked workflow
globular workflow get <run-id>
# Look at the dependency that's blocking
```

**Common causes**:
- Circular dependency (A needs B, B needs A)
- Missing dependency (service not in desired state)
- Dependency failed and isn't retrying

**Resolution**:
- Circular dependencies are reported as errors — fix the spec files
- Missing dependencies: `globular services desired set <dependency> <version>`
- Failed dependencies: investigate and fix the root cause

### Pattern 4: Checksum Mismatch

**Symptoms**: Workflow fails at INSTALL phase with `FailureClass: VALIDATION`.

**Diagnosis**:
```bash
globular workflow get <run-id>
# Step "verify_checksum" shows FAILED
# Error: "checksum mismatch: expected sha256:abc..., got sha256:def..."
```

**Common causes**:
- Corrupt download (network issue during fetch)
- Artifact was replaced in MinIO after manifest was written
- Man-in-the-middle attack (unlikely but possible)

**Resolution**:
```bash
# Clear the cached artifact and retry
globular node clear-cache --node <node>:11000 --package <service>

# If the artifact itself is corrupt, republish
globular pkg publish <corrected-package.tgz>
```

### Pattern 5: Node Unreachable

**Symptoms**: Node shows `unreachable` or `unknown` in cluster health. Heartbeats have stopped.

**Diagnosis**:
```bash
globular cluster health
# Shows: node-xyz  unreachable  5m ago

# Check from the controller's perspective
globular cluster nodes list
# Shows last heartbeat timestamp
```

**Common causes**:
- Node is down (hardware failure, reboot, power loss)
- Network partition
- Node Agent crashed
- Firewall rule blocking port 11000 or 12000

**Resolution**:
- If the node is physically accessible: check power, network, OS status
- If the Node Agent crashed: `sudo systemctl restart globular-node-agent`
- If network partition: check routing, firewall rules, DNS
- If hardware failure: [remove the node](adding-nodes.md#removing-a-node) and replace

## Practical Scenarios

### Scenario 1: Investigating a Degraded Cluster

```bash
# Start with cluster health
globular cluster health
# CLUSTER STATUS: DEGRADED
# node-2: unhealthy (last seen 3m ago)

# Get doctor report
globular doctor report --fresh
# CRITICAL: 3 services down on node-2

# Check if it's a node issue or service issue
globular node logs --node node-2:11000 --unit globular-node-agent --lines 50
# If this times out, the node agent itself is down

# If node agent responds, check individual services
globular node logs --node node-2:11000 --unit authentication --lines 50
globular node logs --node node-2:11000 --unit rbac --lines 50

# Fix: restart the affected services
# (or let auto-heal do it if enabled)
```

### Scenario 2: Post-Deployment Verification

After deploying a new version, verify everything is healthy:

```bash
# 1. Check desired state alignment
globular services desired list
# All services should show INSTALLED

# 2. Check for drift
globular services repair --dry-run
# Should show no issues

# 3. Run doctor
globular doctor report --fresh
# Should show HEALTHY

# 4. Check workflow history for any issues
globular workflow list --status FAILED
# Should be empty (or only old failures)
```

### Scenario 3: Resolving a Stuck Workflow

A workflow has been in EXECUTING state for an unusually long time:

```bash
# Check the workflow
globular workflow get <run-id>
# Shows: step "start_unit" RUNNING for 10 minutes (abnormal)

# Check the node
globular cluster health
# If the node is healthy, the step might be stuck

# Check service logs on the node
globular node logs --node <node>:11000 --unit <service> --lines 100

# If the service is in a startup loop:
# It might be waiting for a dependency that's not resolving
# Check etcd for the dependency's endpoint
```

## What's Next

- [Observability](observability.md): Prometheus metrics, log aggregation, and monitoring
- [Backup and Restore](backup-and-restore.md): Protect your cluster data
