# Failure Scenarios and Recovery

This page catalogs common failure scenarios in a Globular cluster, explains how the platform detects and responds to each one, and provides specific recovery procedures.

## Failure Detection

Globular detects failures through multiple mechanisms:

- **Heartbeats**: Node Agents send periodic heartbeats to the controller (default: 30 seconds). If no heartbeat is received within the stale threshold (5 minutes), the node is marked unhealthy.
- **Health checks**: The Envoy gateway probes service health endpoints. Unhealthy services are removed from the routing table.
- **Workflow failures**: When a workflow step fails, the failure is classified and recorded.
- **Hash-based drift detection**: The controller compares each node's `AppliedServicesHash` against the expected state.
- **Cluster Doctor**: Continuous invariant checking detects configuration drift, stopped units, version mismatches, and missing endpoints.

## Infrastructure Failures

### etcd Unavailable

**Symptoms**: Services fail to read or write configuration. New desired-state commands fail. Service discovery returns stale endpoints.

**Detection**: Controller health check fails to reach etcd. Node Agent etcd probes return unhealthy.

**Impact**: Critical — etcd is the backbone of all cluster state. Without etcd, the controller cannot evaluate desired state, dispatch workflows, or manage membership. Existing running services continue (they don't need etcd for gRPC serving) but cannot be managed.

**Recovery**:
```bash
# Check etcd member status
globular cluster health
# Look for etcd members

# On the affected node:
journalctl -u etcd --no-pager -n 100

# Common causes:
# - Disk full: df -h /var/lib/etcd/
# - Corrupt WAL: etcdctl endpoint status
# - Port conflict: ss -tlnp | grep 2379

# If etcd data is corrupt on one member (3-node cluster):
# Remove and re-add the member
# etcd will replicate from healthy members

# If all etcd members are down:
# Restore from backup (see Backup and Restore)
globular backup restore <latest-backup> --provider etcd
```

### MinIO Down

**Symptoms**: Package downloads fail (FETCH phase). Backup uploads fail. Artifact publishing fails.

**Detection**: Workflow steps fail with `FailureClass: REPOSITORY`. Backup jobs fail with provider errors.

**Impact**: Medium — existing running services are unaffected. New deployments, upgrades, and backups are blocked until MinIO recovers.

**Recovery**:
```bash
# Check MinIO status
journalctl -u minio --no-pager -n 50

# Common causes:
# - Disk space exhausted
# - Erasure coding degradation (too many nodes down)
# - Certificate expired

# Restart MinIO
sudo systemctl restart minio

# If MinIO data is corrupted, it can be restored from backup
globular backup restore <backup-id> --provider minio
```

### Prometheus/Alertmanager Down

**Symptoms**: Metrics queries return errors. Alerts stop firing. Dashboard data is missing.

**Detection**: Monitoring service gRPC calls fail. MCP metrics tools return errors.

**Impact**: Low — this does not affect service operation. Only observability is degraded. The cluster continues to function normally; you just can't see the metrics.

**Recovery**:
```bash
# Restart Prometheus
sudo systemctl restart prometheus

# Restart Alertmanager
sudo systemctl restart alertmanager

# Check if Prometheus has data gaps
globular metrics query --expr 'up'
```

## Service Failures

### Service Crash Loop

**Symptoms**: A service repeatedly starts and crashes. systemd shows rapid restart cycles. Health checks fail intermittently.

**Detection**: Node Agent heartbeat shows unit in `activating` or `failed` state. Workflow VERIFY phase fails with `FailureClass: SYSTEMD`.

**Root causes**:
- **Binary bug**: The service crashes on startup due to a code error
- **Configuration error**: Missing or invalid configuration in etcd
- **Port conflict**: Another process is using the service's port
- **Resource exhaustion**: Out of memory, too many open files
- **Dependency unavailable**: Required service (etcd, auth) not reachable

**Diagnosis**:
```bash
# Check service status
sudo systemctl status <service>

# Check recent logs
globular node logs --node <node>:11000 --unit <service> --lines 200

# Search for error patterns
globular node search-logs --node <node>:11000 --unit <service> --pattern "panic|fatal|error" --severity ERROR

# Check system resources
free -h           # Memory
df -h             # Disk
ulimit -n         # File descriptors
```

**Recovery**:
```bash
# If it's a binary bug: roll back to previous version
globular services desired set <service> <previous-version>

# If it's a configuration error: fix the config in etcd
# (use MCP etcd tools or globular CLI)

# If it's a port conflict: find and stop the conflicting process
ss -tlnp | grep <port>

# If it's resource exhaustion: increase limits in the systemd unit
```

### Service Health Check Failure

**Symptoms**: Service is running (systemd reports active) but fails gRPC health checks. Gateway removes it from routing.

**Detection**: Envoy health check fails. Doctor report shows `UNIT_STOPPED` or endpoint unhealthy.

**Root causes**:
- **Deadlocked thread**: The service is alive but not processing requests
- **Dependency timeout**: The service is waiting for an unresponsive dependency
- **TLS certificate mismatch**: Health check fails TLS handshake
- **Overloaded**: Too many concurrent requests, not responding in time

**Diagnosis**:
```bash
# Check if the process is running
globular node logs --node <node>:11000 --unit <service> --lines 50

# Check for goroutine leaks (if the service has a debug endpoint)
# Check gRPC connection state
```

**Recovery**:
```bash
# Restart the service
globular node control --node <node>:11000 --unit <service> --action restart

# Or let the doctor auto-heal (if enforce mode is enabled)
```

### Authentication Service Down

**Symptoms**: All authenticated operations fail. New tokens cannot be issued. Token validation fails for services using JWT.

**Detection**: Interceptor chain reports authentication failures. gRPC calls return `Unauthenticated` status.

**Impact**: Critical — without authentication, operators cannot manage the cluster and services cannot validate inter-service tokens.

**Recovery**:
```bash
# This is a priority-1 recovery
# Check the service
journalctl -u authentication --no-pager -n 100

# Restart
sudo systemctl restart authentication

# If the binary is broken, roll back
globular services desired set authentication <previous-version>

# Note: During auth outage, the RBAC interceptor falls back to local
# cluster-roles.json, allowing basic operations to continue
```

## Node Failures

### Choosing the right recovery path

Not all node failures require the same response. Use the escalation ladder:

| Node state | Right action |
|------------|-------------|
| Services crashed, node agent alive | `globular doctor heal` — auto-repair, no human steps |
| Specific artifacts corrupt or wrong version | `globular node repair` — targeted reinstall of specific packages |
| Node agent crashed but OS is intact | `sudo systemctl restart globular-node-agent` on the node |
| Node unreachable but expected back | Wait — the reconciler auto-converges when it reconnects |
| **OS cannot be trusted, disk corrupt, hardware replaced** | **`globular node recover full-reseed`** — full wipe + rebuild |

The full-reseed workflow is last resort. It captures an inventory snapshot, fences the reconciler, waits for a human to wipe and reprovision the node, then reinstalls every artifact in deterministic bootstrap order and verifies each one. See [Node Full-Reseed Recovery](node-recovery.md) for the complete guide.

### Node Completely Down

**Symptoms**: All services on the node are unreachable. Heartbeats stop. Gateway removes all backends on that node.

**Detection**: Controller marks node as unreachable after heartbeat stale threshold (5 minutes).

**Impact**: Depends on cluster size and service redundancy. In a 3-node cluster with services on multiple nodes, other nodes handle the load. In a single-node cluster, this is a complete outage.

**Recovery**:
```bash
# Check if the node is physically accessible
ping <node-ip>
ssh <node>

# If the node is reachable but services are down:
sudo systemctl restart globular-node-agent
# The node agent will restart all managed services

# If the OS is intact but the node is misconfigured or has partial state:
globular node recover full-reseed \
  --node-id <node-id> \
  --reason "node down, partial state" \
  --dry-run                        # review plan first
# See: docs/operators/node-recovery.md

# If hardware failure (replacement machine):
# 1. Remove the node from the cluster
globular cluster nodes remove <node-id>

# 2. Provision replacement hardware
# 3. Join the new node with the same profiles
globular cluster token create --expires 2h
globular cluster join --node <new-node>:11000 --controller <controller>:12000 --join-token <token>
globular cluster requests approve <req-id> --profile <same-profiles>
```

### Node Disk Corruption / OS Untrustworthy

**Symptoms**: Services fail with filesystem errors. Node agent reports unexpected state. Package checksums fail. `dmesg` shows I/O errors.

**Detection**: Node agent reports CORRUPTED artifact state. Heartbeats are erratic. Doctor invariants fail on the affected node.

**Impact**: The node cannot be trusted. Any running services on it may be serving corrupt data or behaving incorrectly.

**Recovery**: This is the primary use case for full-reseed. The node's filesystem cannot be trusted so the only safe recovery is a complete wipe and rebuild.

```bash
# Step 1: Capture a snapshot while the node is still responding (if possible)
globular node snapshot create \
  --node-id <node-id> \
  --reason "pre-wipe snapshot, disk corruption detected"

# Step 2: Dry-run to validate the plan
globular node recover full-reseed \
  --node-id <node-id> \
  --reason "disk corruption — I/O errors on /var" \
  --snapshot-id <snapshot-id-from-step-1> \
  --dry-run

# Step 3: Dispatch the workflow
globular node recover full-reseed \
  --node-id <node-id> \
  --reason "disk corruption — I/O errors on /var" \
  --snapshot-id <snapshot-id-from-step-1>

# Step 4: Wipe and reinstall the OS (your action, not automated)
# Step 5: Acknowledge reprovision
globular node recover ack-reprovision \
  --node-id <node-id> \
  --workflow-id <workflow-id>

# Full procedure: see docs/operators/node-recovery.md
```

### Node Agent Crash

**Symptoms**: The node agent stops responding. Services on the node continue running but cannot be managed. Heartbeats stop.

**Detection**: Controller marks node as stale/unreachable. Workflow steps targeting this node time out.

**Recovery**:
```bash
# Restart the node agent
sudo systemctl restart globular-node-agent

# The agent will:
# 1. Reconnect to the controller
# 2. Send a heartbeat with current state
# 3. Resume processing workflow steps
# 4. Report any state changes that occurred during the outage
```

### Network Partition

**Symptoms**: Some nodes cannot reach other nodes. Split-brain behavior where different parts of the cluster disagree on state.

**Detection**: Heartbeat failures for multiple nodes simultaneously. etcd cluster health shows partitioned members.

**Impact**: Nodes in the majority partition continue normally (they have etcd quorum). Nodes in the minority partition:
- Lose etcd write capability (no quorum)
- Controller instances resign leadership
- Running services continue but cannot be managed
- New deployments are impossible

**Recovery**:
```bash
# The partition heals automatically when network connectivity is restored
# etcd re-synchronizes
# Nodes resume heartbeats
# Controller re-evaluates desired state

# If the partition persists:
# Check network infrastructure (switches, firewalls, routing)
# Check DNS resolution between nodes
# Check firewall rules for ports 2379, 2380, 11000, 12000
```

## Workflow Failures

### Stuck Workflow

**Symptoms**: A workflow has been in EXECUTING state for an abnormally long time.

**Detection**: Workflow list shows old EXECUTING entries. Doctor reports `workflow_stuck` invariant failure.

**Recovery**:
```bash
# Check the workflow details
globular workflow get <run-id>
# Identify which step is stuck

# If the step is targeting a node:
# Check node connectivity
globular cluster health

# If the node is unreachable, the step will eventually timeout
# The workflow will fail and can be retried

# If the workflow service itself is stuck:
sudo systemctl restart globular-workflow
```

### Reconciliation Storm

**Symptoms**: Controller dispatches many workflows rapidly. Node agents are overwhelmed. CPU and network usage spike.

**Detection**: Workflow list shows many PENDING/EXECUTING entries. Controller logs show rapid dispatch. Circuit breaker metrics increase.

**Cause**: Usually triggered by a large desired-state change while the cluster is unhealthy, causing cascading failures and retries.

**Built-in protection**: Globular has three safeguards:
1. **Semaphore**: Limits concurrent workflows (default: 3)
2. **Circuit breaker**: Pauses dispatch when the Workflow Service is overwhelmed
3. **5-minute backoff**: Failed releases wait before retrying

**Recovery**:
```bash
# The circuit breaker should have already engaged
# Wait for the breaker to cool down (30 seconds)

# If the storm persists, identify the root cause:
globular workflow list --status FAILED
# Check the most common failure class

# Fix the root cause (usually an infrastructure issue)
# Then let convergence resume naturally
```

## Practical Scenarios

### Scenario 1: Node-2 Loses Power

In a 3-node cluster, node-2 loses power unexpectedly:

```
Timeline:
T+0s:    Power lost on node-2
T+30s:   Controller misses node-2's heartbeat
T+5m:    Controller marks node-2 as unreachable
T+5m:    Doctor reports: CRITICAL "node-2 unreachable"

Automatic response:
- etcd: Quorum maintained (2/3 members)
- MinIO: Erasure coding handles missing node
- Gateway: Removes node-2 backends from rotation
- Controller: No workflows dispatched to node-2
- Services: Traffic routes to node-1 and node-3

Manual response:
T+30m:   Node-2 power restored
T+30m:   Node agent starts, sends heartbeat
T+31m:   Controller processes heartbeat, marks node-2 as healthy
T+31m:   Controller detects any drift on node-2
T+32m:   Convergence workflows bring node-2 to desired state
```

### Scenario 2: Bad Deployment Cascading Failure

A new version of the authentication service has a bug that causes it to reject all tokens:

```
Timeline:
T+0:     globular services desired set authentication 0.0.3
T+1m:    Workflow installs 0.0.3 on node-1
T+1m:    Auth service starts, passes health check (it starts OK)
T+2m:    Workflow installs 0.0.3 on node-2 and node-3
T+3m:    All nodes running 0.0.3 → all token validations fail
T+3m:    Every gRPC call to any service fails with Unauthenticated
T+3m:    RBAC interceptor falls back to local cluster-roles.json
T+4m:    Operator notices (alerts fire, or direct observation)

Recovery:
# RBAC fallback keeps basic operations working
globular services desired set authentication 0.0.2
# Workflows install 0.0.2 on all nodes
# Auth service starts accepting tokens again
# Cluster returns to normal

# Yank the bad version
globular pkg yank authentication 0.0.3
```

### Scenario 3: etcd Disk Full

etcd's data directory fills the disk:

```
Timeline:
T+0:     /var/lib/etcd/ fills disk
T+0:     etcd stops accepting writes
T+1m:    Controller cannot update state
T+1m:    Node agents cannot register packages
T+2m:    Doctor reports: CRITICAL "etcd write failure"

Recovery:
# On the affected node:
# 1. Check disk usage
df -h /var/lib/etcd/

# 2. Compact etcd history (removes old revisions)
etcdctl compact $(etcdctl endpoint status --write-out="json" | jq '.header.revision')
etcdctl defrag

# 3. If still full, clean up large keys
# Check for accumulation in any key prefix

# 4. Increase disk space if needed

# 5. Verify etcd health
etcdctl endpoint health
```

## Additional Failure Patterns

### Artifact Stuck in VERIFIED

**Symptoms**: A published package shows `VERIFIED` state but never transitions to `PUBLISHED`. Deployments targeting this version fail because the artifact is not yet available.

**Detection**: `globular pkg info <service>` shows the artifact in VERIFIED state. Desired-state workflows fail at the DECISION phase with "artifact not in PUBLISHED state."

**Cause**: The publish pipeline completed checksum verification (STAGING → VERIFIED) but failed to transition to PUBLISHED. This can happen if:
- The Repository service crashed between the verification and publish steps
- etcd was temporarily unavailable during the state transition
- The publisher's RBAC permissions allowed upload but not the final publish transition

**Recovery**:
```bash
# Check the artifact state
globular pkg info <service>
# Shows: VERIFIED (should be PUBLISHED)

# Option 1: Re-publish the package
globular pkg publish <package.tgz>
# The repository detects the existing artifact and retries the transition

# Option 2: If the archive is intact in MinIO but the manifest is stuck,
# check the repository service logs
globular node logs --node <repo-node>:11000 --unit repository --lines 100

# Verify etcd connectivity
globular cluster health
```

### Leader Mismatch

**Symptoms**: Two controller instances both believe they are the leader. Conflicting desired-state decisions. Duplicate workflow dispatches. Inconsistent node health assessments.

**Detection**: Controller logs show leadership acquisition on multiple nodes simultaneously. Workflow list shows duplicate correlation IDs. Doctor report may show conflicting convergence states.

**Cause**: This is a split-brain scenario, typically caused by:
- Network partition where both sides can reach etcd (rare — etcd prevents this via quorum)
- etcd lease renewal succeeded on both sides due to clock skew
- A zombie leader that held the lease through a long GC pause, while a new leader was elected

**Recovery**:
```bash
# Check which controllers think they're leader
globular node logs --node <node-1>:11000 --unit controller --pattern "acquired leadership"
globular node logs --node <node-2>:11000 --unit controller --pattern "acquired leadership"

# The liveness watchdog should detect and resolve this automatically
# If it doesn't, manually restart the controller on one node:
globular node control --node <stale-leader>:11000 --unit controller --action restart

# The restarted controller becomes standby
# The other controller continues as sole leader

# Verify single leader
globular cluster health
# Should show one leader
```

**Prevention**: The controller's liveness watchdog monitors processing activity and resigns leadership if the instance is not actually handling requests. In normal operation, etcd's lease mechanism prevents true split-brain.

### Service Unreachable

**Symptoms**: Calls to a specific service fail with "connection refused" or "deadline exceeded." The service appears in `desired list` as INSTALLED but cannot be reached by other services or the gateway.

**Detection**: Gateway health checks fail for the service. Other services that depend on it report errors. Doctor report shows `ENDPOINT_MISSING` drift.

**Cause**:
- The service process crashed but systemd hasn't restarted it yet (or restart limit reached)
- The service is running but bound to the wrong interface (should be 0.0.0.0)
- The service's endpoint registration in etcd is stale (pointing to an old IP/port)
- A firewall rule is blocking the service port
- TLS certificate mismatch (service certificate doesn't match the hostname clients use)

**Recovery**:
```bash
# 1. Check if the service is running
globular node control --node <node>:11000 --unit <service> --action status

# 2. If not running, restart it
globular node control --node <node>:11000 --unit <service> --action restart

# 3. If running but unreachable, check the endpoint in etcd
# The endpoint should match the service's actual address

# 4. Check logs for binding errors
globular node logs --node <node>:11000 --unit <service> --lines 100
# Look for: "bind: address already in use" or "tls: bad certificate"

# 5. Check TLS certificates
globular node certificate-status --node <node>:11000

# 6. Check network
# From another node, verify connectivity to the service port
```

### Node Not Reporting Heartbeat

**Symptoms**: A node shows as `unreachable` or `unknown` in cluster health. Last heartbeat timestamp is stale (> 5 minutes old).

**Detection**: `globular cluster health` shows the node with a stale "last seen" time. Doctor reports `node_unreachable` finding.

**Cause**:
- The node is physically down (power loss, hardware failure)
- The Node Agent crashed and hasn't restarted
- Network partition between the node and the controller
- The node's clock is significantly skewed (heartbeats appear to be from the future/past)
- The controller is overwhelmed and not processing heartbeats fast enough

**Recovery**:
```bash
# 1. Check if the node is reachable
ping <node-ip>

# 2. If reachable, check the node agent
ssh <node> sudo systemctl status globular-node-agent

# 3. If the agent is down, restart it
ssh <node> sudo systemctl restart globular-node-agent

# 4. If the agent is running but heartbeats aren't arriving,
# check network connectivity to the controller
ssh <node> curl -k https://<controller-ip>:12000
# If this fails, check firewalls and routing

# 5. If the node is permanently lost
globular cluster nodes remove <node-id>
# Then provision a replacement (see Adding Nodes)

# 6. After recovery, verify heartbeat is flowing
globular cluster health
# Node should show "healthy" with recent "last seen" timestamp
```

## What's Next

- [Cluster Doctor and Invariants](operators/cluster-doctor.md): Automated health analysis
- [Network and Routing](operators/network-and-routing.md): Gateway, xDS, and service discovery
