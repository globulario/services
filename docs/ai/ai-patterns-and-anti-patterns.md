# AI Patterns and Anti-Patterns

This document catalogs good and bad patterns for AI integration in Globular. Each pattern includes a concrete example showing what to do (or what to avoid) and why.

## Good Patterns

### Pattern 1: Read → Diagnose → Recommend → Execute → Verify

The canonical AI action pattern. Every phase is explicit and auditable.

**Example**: Service authentication is failing health checks.

```
1. READ    — AI queries cluster_get_health, nodeagent_get_service_logs,
             metrics_query for authentication error rate
2. DIAGNOSE — Evidence: exit code 137, dmesg shows OOM kill,
              memory usage grew linearly over 24 hours
              → Root cause: memory leak in token cache
              → Confidence: 0.85
3. RECOMMEND — ACTION_RESTART_SERVICE for authentication on node-2
               Expected outcome: fresh memory allocation, health check passes
4. EXECUTE  — Remediation workflow: resolve → assess → execute → verify
              Via: cluster_doctor → node_agent → systemctl restart
5. VERIFY   — Re-query health check → passes
              Memory usage dropped to baseline
              Job record: SUCCEEDED
```

**Why this is good**: Every step produces observable output. The diagnosis is recorded with evidence. The action flows through a workflow. The outcome is verified. If something goes wrong, every step is inspectable.

### Pattern 2: Workflow-Triggered Remediation

AI actions flow through the standard workflow engine, not through custom execution paths.

**Example**: Restarting a crashed service.

```
AI Executor calls: ProcessIncident(incident_id, tier=AUTO_REMEDIATE)
    │
    ▼
Remediation workflow starts: remediate.doctor.finding
    │
    ├── Step 1: resolve_finding (identify the service and node)
    ├── Step 2: assess_risk (LOW: service restart)
    ├── Step 3: execute_remediation (cluster_doctor → node_agent → systemctl)
    └── Step 4: verify_convergence (re-check doctor finding)
    │
    ▼
Workflow completes: SUCCEEDED
Job record updated in etcd
```

**Why this is good**: The action uses the same workflow engine as service deployment. It has the same audit trail, failure handling, and observability. The workflow is defined in YAML, reviewable, and version-controlled.

### Pattern 3: Typed Actions with Bounded Parameters

AI actions are constrained to a fixed set of typed operations with validated parameters.

**Example**: The action dispatcher in AI Executor:

```go
func (d *actionDispatcher) Dispatch(action ActionType, target Target) error {
    switch action {
    case ACTION_RESTART_SERVICE:
        return d.restartServiceBackend.Execute(target.Node, target.Service)
    case ACTION_DRAIN_ENDPOINT:
        return d.drainEndpointBackend.Execute(target.Endpoint, target.GracePeriod)
    case ACTION_CIRCUIT_BREAKER:
        return d.circuitBreakerBackend.Execute(target.Service, target.State)
    default:
        return fmt.Errorf("unknown action type: %s", action)
    }
}
```

**Why this is good**: AI cannot construct arbitrary commands. It can only choose from a defined set of actions. Each action has a specific backend implementation with its own validation. Adding a new action type requires a code change, not just a prompt.

### Pattern 4: Explicit State Mutation Through APIs

All state changes go through gRPC APIs with RBAC enforcement.

**Example**: Storing a diagnosis result in AI Memory:

```go
// AI Executor stores diagnosis
memoryClient.Store(ctx, &memorypb.StoreRequest{
    Project: "globular-services",
    Type:    memorypb.MemoryType_DEBUG,
    Title:   "Authentication OOM on node-2",
    Content: "Root cause: token cache memory leak...",
    Tags:    "authentication,oom,node-2",
    Metadata: `{"root_cause":"memory-leak","confidence":"0.85"}`,
})
```

**Why this is good**: The memory store RPC goes through the standard gRPC interceptor chain. RBAC checks that the executor has permission. Audit logging records the write. The data is in a well-defined schema, searchable, and queryable.

### Pattern 5: Audit-First Design

Every AI decision produces a durable, queryable record before any action is taken.

**Example**: The AI Executor job store:

```
/globular/ai/jobs/inc-abc123
{
    "state": "JOB_DIAGNOSED",
    "diagnosis": {
        "root_cause": "OOM kill in authentication service",
        "confidence": 0.85,
        "evidence": ["exit code 137", "dmesg OOM entries", "memory growth metrics"],
        "proposed_action": "ACTION_RESTART_SERVICE"
    },
    "tier": "AUTO_REMEDIATE",
    "created_at": 1712937600,
    "diagnosed_at": 1712937615
}
```

The diagnosis is persisted in etcd **before** any action is taken. If the system crashes between diagnosis and execution, the record survives. If the action fails, the record shows what was attempted and why.

**Why this is good**: Operators can always answer "what did AI do and why?" by querying the job store. There is no invisible AI behavior.

### Pattern 6: Deterministic Fallback

AI services degrade gracefully when the LLM is unavailable.

**Example**: AI Executor diagnosis pipeline:

```go
func (d *diagnoser) Diagnose(incident Incident) (Diagnosis, error) {
    // Try Claude first
    diagnosis, err := d.claudeAnalyze(incident)
    if err == nil {
        return diagnosis, nil
    }

    // Fall back to deterministic rules
    return d.deterministicAnalyze(incident)
}
```

The deterministic analyzer uses pattern matching:
- `service.exited` + exit code 137 → "OOM kill"
- `service.exited` + exit code 1 → "Configuration error or binary crash"
- `cluster.health.degraded` → "Check node heartbeats and service health"

**Why this is good**: The cluster never depends on an external API for basic operations. The AI layer enhances diagnosis but is not required. If Anthropic is down, the system still detects and remediates known patterns.

### Pattern 7: Multi-Node Consensus Before Action

AI Executor instances on different nodes coordinate to avoid conflicts.

**Example**: Before restarting a service on node-2:

```go
// Executor on node-1 proposes action to peers
for _, peer := range peers {
    vote, err := peer.ProposeAction(ctx, &ProposeActionRequest{
        ProposedAction: "restart authentication on node-2",
        Target:         "node-2",
        Diagnosis:      diagnosis,
        Tier:           "AUTO_REMEDIATE",
    })
    // Collect votes: APPROVE, REJECT, ABSTAIN, ESCALATE
}

// Majority must approve
if approvalCount > len(peers)/2 {
    proceed()
} else {
    escalateToOperator()
}
```

**Why this is good**: If node-1 and node-3 both see the same incident, they don't both try to restart the service. Peer collaboration prevents duplicate actions and conflicting remediation.

## Anti-Patterns

### Anti-Pattern 1: Shell-First Automation

**What it looks like**:
```go
// BAD: AI constructs a shell command
cmd := fmt.Sprintf("ssh %s systemctl restart %s", node, service)
exec.Command("sh", "-c", cmd).Run()
```

**Why it's bad**: No RBAC check. No audit trail. No workflow tracking. Command injection risk. No verification step. No failure classification.

**What to do instead**: Use the remediation workflow: AI Executor → ProcessIncident → remediate.doctor.finding workflow → Node Agent → systemctl.

### Anti-Pattern 2: Hidden Config Mutation

**What it looks like**:
```go
// BAD: AI writes a config file that changes system behavior
os.WriteFile("/etc/globular/config/override.json", newConfig, 0644)
```

**Why it's bad**: The change is invisible to etcd (source of truth). Other nodes don't see it. The doctor can't detect the drift. Restarting the service may not pick it up. There's no audit record.

**What to do instead**: Modify configuration through the proper etcd-backed config API. The change is visible, replicated, and auditable.

### Anti-Pattern 3: Environment Variable Overrides as Control Plane

**What it looks like**:
```bash
# BAD: AI sets environment variables to control behavior
export DATABASE_HOST=backup-server.example.com
systemctl restart inventory
```

**Why it's bad**: Environment variables are process-scoped and invisible. No other component can verify what value the service is using. The change is lost on restart. etcd doesn't know about it.

**What to do instead**: Change the database endpoint in etcd. The service reads it on next config reload or restart.

### Anti-Pattern 4: Bypassing Workflows

**What it looks like**:
```go
// BAD: AI calls Node Agent directly to install a package
nodeAgent.ApplyPackageRelease(ctx, &ApplyPackageReleaseRequest{
    PackageName: "postgresql",
    Version:     "0.0.4",
})
```

**Why it's bad**: No workflow record. No step-by-step audit. No failure classification. No semaphore limiting. No circuit breaker. No release tracking. The controller doesn't know the installation happened.

**What to do instead**: Set desired state via `UpsertDesiredService`. The controller creates a workflow that goes through the proper pipeline.

### Anti-Pattern 5: AI Mutating Infrastructure Directly

**What it looks like**:
```go
// BAD: AI modifies etcd cluster membership
etcdClient.MemberAdd(ctx, []string{"https://new-node:2380"})
```

**Why it's bad**: etcd membership changes are one of the most dangerous operations in the cluster. AI does not have the context to safely add or remove etcd members. A bad membership change can cause quorum loss and total cluster failure.

**What to do instead**: etcd membership is managed by the Cluster Controller during node join/remove workflows. AI should never directly modify infrastructure state.

### Anti-Pattern 6: AI Inventing State from Partial Evidence

**What it looks like**:
```
AI observes: Last health check for node-3 was 5 minutes ago
AI assumes: Node-3 is down
AI action: Remove node-3 from cluster

Reality: Node-3 was temporarily unreachable due to a 10-second network blip.
         It recovered 4 minutes ago. Removing it causes data loss.
```

**Why it's bad**: The AI assumed state (node is permanently down) from partial evidence (missed heartbeat). The correct action was to wait for the stale threshold, check if heartbeats resume, and only then diagnose.

**What to do instead**: AI must verify current state before acting. Query the node directly. Check if heartbeats resumed. Wait for the controller's stale threshold (5 minutes). Only diagnose based on verified, current evidence.

### Anti-Pattern 7: AI Acting Without Verification

**What it looks like**:
```
AI diagnoses: Authentication service needs restart
AI executes: Restart authentication
AI status: SUCCEEDED (based on systemctl restart exit code 0)

Reality: The service started but immediately failed its health check.
         Restarting didn't fix the actual problem (port conflict).
```

**Why it's bad**: The AI considered the action successful because the restart command succeeded. But the actual problem persisted.

**What to do instead**: The remediation workflow includes a `verify_convergence` step that re-checks the original finding after action. If the finding persists, the job is marked FAILED, not SUCCEEDED.

### Anti-Pattern 8: Unbounded AI Retry Loops

**What it looks like**:
```
T+0:    AI restarts authentication → fails
T+30s:  AI retries restart → fails
T+60s:  AI retries restart → fails
T+90s:  AI retries restart → fails
...     (continues indefinitely)
```

**Why it's bad**: If the underlying problem isn't a transient failure (it's a port conflict, a binary bug, or a missing dependency), restarting will never work. Unbounded retries waste resources and mask the real problem.

**What to do instead**: The AI Executor's retry policy limits attempts to 3. For deterministic jobs (same input → same failure), retries are disabled. After max retries, the job is marked FAILED and requires operator intervention.

## Summary Table

| Pattern | Type | Key Principle |
|---------|------|--------------|
| Read → Diagnose → Recommend → Execute → Verify | Good | Complete lifecycle with verification |
| Workflow-triggered remediation | Good | Actions through standard engine |
| Typed actions with bounded params | Good | No free-form commands |
| Explicit state mutation through APIs | Good | RBAC + audit + schema |
| Audit-first design | Good | Record before act |
| Deterministic fallback | Good | Graceful degradation without LLM |
| Multi-node consensus | Good | Prevent duplicate/conflicting actions |
| Shell-first automation | Bad | No audit, no RBAC, injection risk |
| Hidden config mutation | Bad | Invisible to source of truth |
| Environment variable overrides | Bad | Process-scoped, invisible, lost on restart |
| Bypassing workflows | Bad | No tracking, no failure handling |
| Direct infrastructure mutation | Bad | Risk of quorum loss, data loss |
| Inventing state from partial evidence | Bad | Assumptions ≠ facts |
| Acting without verification | Bad | Success of command ≠ success of intent |
| Unbounded retry loops | Bad | Masks real problems, wastes resources |
