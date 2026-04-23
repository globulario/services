# Cluster Doctor and Invariants

The Cluster Doctor is a specialized service that continuously monitors cluster health, detects problems through invariant checking, classifies findings by severity, and optionally auto-heals low-risk issues. This page covers how the doctor works, what invariants it checks, how to use its diagnostic capabilities, and how auto-healing operates.

## Why the Cluster Doctor Exists

Manual cluster monitoring has limitations:
- Operators can't continuously watch every service on every node
- Some problems are subtle — a version mismatch, a missing endpoint registration, a stale etcd member — and aren't caught by simple health checks
- When problems compound, diagnosing the root cause requires correlating information from multiple sources
- Recovery from routine issues (service stopped, unit disabled) should be automatic

The Cluster Doctor fills these gaps. It runs on the leader node, collects state from all cluster components, evaluates invariants, and produces structured reports with specific remediation steps.

## Architecture

The Cluster Doctor runs as a leader-elected service (port 12005). It operates in three phases:

### 1. Collection

The **collector** gathers a cluster snapshot by querying:
- Cluster Controller: desired state, node registry, release status
- Node Agents: installed packages, unit states, service endpoints
- Workflow Service: active and recent workflow runs
- Prometheus: metric anomalies (optional)

Collection uses a **single-flight pattern** — concurrent requests for the same snapshot share a single fetch, preventing redundant queries. Results are cached with a short TTL (configurable, typically 30 seconds).

### 2. Analysis

The **invariant engine** evaluates the snapshot against a set of rules. Each invariant produces a `PASS`, `FAIL`, or `PENDING` result. Failed invariants produce **Findings**.

### 3. Reporting

The **renderer** builds a structured report from the findings:
- Overall cluster status: HEALTHY, DEGRADED, or CRITICAL
- Findings ranked by severity
- Top 5 issue IDs for prioritization
- Counts by category

## Invariants

Invariants are conditions that must always be true in a healthy cluster. The doctor checks these continuously:

### Service Drift Detection

**Rule**: For every `DesiredService` entry, the installed version on each target node must match the desired version.

**Failure example**:
```
FAIL: monitoring version mismatch on node-3
  Desired: 0.0.6
  Installed: 0.0.5
  Evidence: /globular/nodes/node-3/packages/SERVICE/monitoring
```

**Remediation**: A workflow should be in progress. If not, repair triggers one.

### Unit Running Check

**Rule**: Every installed service should have an active systemd unit.

**Failure example**:
```
FAIL: postgresql unit stopped on node-2
  Evidence: systemctl status = inactive (dead)
  Exit code: 137 (SIGKILL)
  Last active: 1h ago
```

**Remediation**: Restart the unit. If it keeps crashing, investigate logs.

### Network Drift Detection

**Rule**: Every running service should have a matching endpoint registered in etcd.

**Failure example**:
```
FAIL: dns service endpoint missing on node-1
  Evidence: service running on :10006 but no etcd registration at
  /globular/services/dns/instances/node-1
```

**Remediation**: Re-register the endpoint (usually fixed by restarting the service).

### Pending Convergence

**Rule**: Workflows should not be stuck for longer than a threshold (configurable, typically 15 minutes).

**Failure example**:
```
WARN: workflow wf-run-abc123 stuck in EXECUTING for 25 minutes
  Service: postgresql, Node: node-3
  Current step: start_unit (RUNNING for 20 minutes)
```

**Remediation**: Investigate the step. Check node connectivity. Consider canceling and retrying.

### etcd Member Health

**Rule**: All etcd cluster members should be healthy and synchronized.

**Failure example**:
```
WARN: etcd member on node-2 behind leader by 1200 raft entries
  Evidence: etcdctl endpoint status shows applied_index lag
```

**Remediation**: Usually self-resolving. If persistent, the member may need to be removed and re-added.

### Blocked Workflow Detection

**Rule**: No workflows should be blocked for longer than a threshold.

**Failure example**:
```
WARN: workflow wf-run-def456 blocked for 45 minutes
  Blocking dependency: authentication (not yet installed on node-3)
  Trigger: DEPENDENCY_UNBLOCKED expected
```

**Remediation**: Investigate why the dependency is not being installed.

### Service Registration Gap

**Rule**: Running services should be discoverable via etcd.

**Failure example**:
```
INFO: authentication service running but not registered on node-1
  Evidence: systemctl shows active, etcd has no instance entry
```

**Remediation**: Restart the service to re-register.

### Artifact Integrity

**Rule**: Installed package checksums should match the artifact manifest in the repository.

**Failure example**:
```
CRITICAL: postgresql binary checksum mismatch on node-2
  Installed: sha256:abc123...
  Expected: sha256:def456...
```

**Remediation**: Reinstall the package from the repository.

## Findings

Each invariant failure produces a **Finding** with structured metadata:

```
Finding {
  id: "postgresql_stopped_node2"
  severity: CRITICAL
  category: "unit_stopped"
  entity_ref: "node-2/postgresql"
  evidence: [
    { key: "systemctl_status", value: "inactive (dead)" },
    { key: "exit_code", value: "137" },
    { key: "last_active", value: "2025-04-12T09:15:00Z" }
  ]
  remediation: [
    { action: SYSTEMCTL_RESTART, params: { unit: "postgresql", node_id: "node-2" } }
  ]
}
```

### Severity Levels

| Severity | Meaning | Example |
|----------|---------|---------|
| **CRITICAL** | Immediate action required. Data loss or service outage possible. | Service binary checksum mismatch, etcd quorum lost |
| **ERROR** | Significant problem affecting functionality. | Service stopped, version mismatch on multiple nodes |
| **WARN** | Potential problem that may escalate. | Single node version mismatch, slow etcd sync |
| **INFO** | Informational, no immediate action needed. | Service re-registered, minor endpoint gap |

### Drift Categories

| Category | Description |
|----------|-------------|
| `MISSING_UNIT_FILE` | systemd unit file not found |
| `UNIT_STOPPED` | Unit installed but not running |
| `UNIT_DISABLED` | Unit disabled in systemd |
| `VERSION_MISMATCH` | Installed version ≠ desired version |
| `STATE_HASH_MISMATCH` | Node hash doesn't match expected |
| `ENDPOINT_MISSING` | Service running but not registered in etcd |
| `INVENTORY_INCOMPLETE` | InstalledPackage record missing or incomplete |

## Using the Doctor

### Get a Report

```bash
# Quick report (cached snapshot)
globular doctor report

# Fresh report (collects live data)
globular doctor report --fresh
```

### Explain a Finding

```bash
globular doctor explain <finding-id>
```

This returns:
- **Why it happened**: Analysis of the root cause
- **Evidence**: Data supporting the finding
- **Recommended actions**: Ordered list of remediation steps
- **Risk level**: How risky each remediation action is

### Drift Report

```bash
globular doctor drift-report
```

Focused view of desired-vs-actual mismatches, organized by node and service.

## Auto-Heal

The doctor can automatically remediate certain issues through the **healer loop**.

### Healer Modes

| Mode | Behavior |
|------|----------|
| `observe` | Classify findings only. No actions taken. Default mode. |
| `dry_run` | Show what actions would be taken, but don't execute them. |
| `enforce` | Execute auto-heal actions for low-risk findings. |

Set the mode:
```bash
globular doctor set-mode enforce
```

### Remediation Actions

| Action | Description | Risk | Auto-Healable |
|--------|-------------|------|--------------|
| `SYSTEMCTL_RESTART` | Restart a stopped service | LOW | Yes (in enforce mode) |
| `SYSTEMCTL_START` | Start a disabled service | LOW | Yes (in enforce mode) |
| `SYSTEMCTL_STOP` | Stop a service | LOW | Yes (in enforce mode) |
| `FILE_DELETE` | Delete a file (whitelisted paths only) | LOW | Yes |
| `ETCD_PUT` | Write an etcd key | MEDIUM | No — requires approval |
| `ETCD_DELETE` | Delete an etcd key | HIGH | No — requires approval |
| `PACKAGE_REINSTALL` | Reinstall a package | MEDIUM | No — requires approval |
| `NODE_REMOVE` | Remove a node from the cluster | HIGH | No — requires approval |

### Safety Controls

The healer has multiple safety mechanisms:

**Rate limiting**: Maximum actions per cycle (configurable `healer_max_actions_per_cycle`). Prevents the healer from making too many changes at once.

**Circuit breaker**: Stops after 3 consecutive failures. If auto-heal actions are failing, something is fundamentally wrong and human intervention is needed.

**Leadership gating**: The healer only runs on the leader doctor instance. This prevents multiple doctor instances from executing the same actions.

**Audit trail**: Every action is recorded in both an in-memory ring buffer and a persistent file. The complete history is queryable:

```bash
globular doctor heal-history
```

### Approval Workflow

For high-risk actions, the doctor uses a structured approval workflow:

1. **Resolve**: Doctor identifies the problem and proposes a remediation
2. **Assess**: Doctor evaluates the risk and generates a `HealDecision` with disposition (AUTO, PROPOSE, OBSERVE)
3. **Approve**: For PROPOSE disposition, an operator reviews and approves with an approval token
4. **Execute**: The approved action is executed via the `ExecuteRemediation` RPC
5. **Verify**: Doctor re-checks the invariant to confirm the fix worked

```bash
# Execute a proposed remediation
globular doctor execute-remediation <finding-id> --approval-token <token>
```

Or use the workflow-based approach:
```bash
# Start a full remediation workflow
globular doctor start-remediation-workflow <finding-id>
# This goes through: resolve → assess → approve → execute → verify
```

## Freshness Contract

The doctor's reports include metadata about data freshness:

```
ReportHeader {
  source: "cluster-doctor (leader)"
  observed_at: "2025-04-12T10:30:00Z"
  snapshot_age_seconds: 12
  cache_hit: true
  cache_ttl_seconds: 30
}
```

- **CACHED mode**: Returns the most recent snapshot. Fast (< 1 second) but may be up to TTL seconds old.
- **FRESH mode**: Forces a new collection from all nodes. Slower (5-30 seconds) but guaranteed current.

The snapshot age is computed server-side to avoid clock skew between the doctor and the client.

## Practical Scenarios

### Scenario 1: Routine Health Check

```bash
# Quick check
globular doctor report
# CLUSTER STATUS: HEALTHY
# FINDINGS: 0
# All invariants: PASS

# Nothing to do — cluster is healthy
```

### Scenario 2: Service Down After Reboot

Node-2 rebooted and some services didn't auto-start:

```bash
globular doctor report --fresh
# CLUSTER STATUS: DEGRADED
# FINDINGS: 3
#   ERROR  postgresql unit stopped on node-2
#   ERROR  redis unit stopped on node-2
#   WARN   monitoring unit stopped on node-2

# With auto-heal in enforce mode:
# The healer automatically restarts all three services
# within the next healer cycle (typically < 1 minute)

# Check heal history
globular doctor heal-history
# 2025-04-12 10:30:00  SYSTEMCTL_RESTART  postgresql@node-2  EXECUTED ✓
# 2025-04-12 10:30:01  SYSTEMCTL_RESTART  redis@node-2       EXECUTED ✓
# 2025-04-12 10:30:02  SYSTEMCTL_RESTART  monitoring@node-2  EXECUTED ✓

# Verify
globular doctor report
# CLUSTER STATUS: HEALTHY
```

### Scenario 3: Investigating a Complex Failure

Multiple issues are interacting:

```bash
globular doctor report --fresh
# CLUSTER STATUS: CRITICAL
# FINDINGS: 5
#   CRITICAL  etcd member unhealthy on node-3
#   ERROR     authentication not responding on node-3
#   ERROR     rbac service down on node-3
#   WARN      3 services version mismatch on node-3
#   INFO      workflow wf-abc blocked (dependency: authentication)

# The root cause is likely the etcd issue — everything cascades from there
globular doctor explain etcd_unhealthy_node3
# Evidence: etcd on node-3 cannot reach peers
# Root cause: network interface down on node-3
# Remediation: Check network on node-3

# Fix the root cause
# (fix network on node-3)

# After network is restored:
globular doctor report --fresh
# etcd syncs → auth starts → rbac starts → versions converge → workflow unblocks
# CLUSTER STATUS: HEALTHY (may take a few minutes for full convergence)
```

## What's Next

- [Network and Routing](network-and-routing.md): Envoy gateway, xDS routing, and DNS
- [Certificate Lifecycle](certificate-lifecycle.md): Certificate management and rotation
