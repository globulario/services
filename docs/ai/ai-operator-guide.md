# AI Operator Guide

This guide explains how to operate, monitor, trust, and constrain the AI subsystem in a Globular cluster. It covers what AI can do, what it cannot do, how to verify AI decisions, and how to disable or adjust AI behavior.

## What AI Can Do

### Observe and Diagnose

The AI subsystem continuously observes cluster state and diagnoses problems:

- **Event monitoring**: AI Watcher subscribes to cluster events (service crashes, health changes, drift detection, workflow failures) and creates incidents
- **Automated diagnosis**: AI Executor gathers evidence from multiple sources (cluster health, logs, metrics, historical incidents) and uses Claude to identify root causes
- **Pattern recognition**: AI Memory stores past incidents and diagnoses, enabling faster diagnosis of recurring problems

### Auto-Remediate (When Configured)

For pre-approved action types, AI can execute remediation automatically:

- Restart a crashed service (Tier 1)
- Clear corrupted local storage (Tier 1)
- Request certificate renewal (Tier 1)

These actions only execute when:
1. The watcher rule specifies `AUTO_REMEDIATE` tier
2. The diagnosis confidence is sufficient
3. The risk level is LOW
4. The action type is in the approved set

### Propose Actions (Requiring Approval)

For higher-risk operations, AI proposes actions and waits for human approval:

- Drain an endpoint from routing
- Open a circuit breaker
- Modify routing weights

The operator reviews the diagnosis, evidence, and proposed action before approving or denying.

### Optimize Routing

AI Router computes dynamic routing policies based on telemetry. In ACTIVE mode, it adjusts endpoint weights to shift traffic away from degrading backends.

## What AI Cannot Do

AI is explicitly prohibited from:

| Prohibited Action | Reason |
|------------------|--------|
| Modify desired state | Operator-owned; deployment decisions are human decisions |
| Add or remove nodes | Cluster membership is admin-controlled |
| Change RBAC permissions | Security boundaries cannot be modified by AI |
| Publish or yank packages | Package lifecycle is developer/CI-controlled |
| Execute shell commands | All actions must be typed and flow through workflows |
| Access file system outside allowlists | Safety boundary enforced by MCP server |
| Act without audit trail | Every action produces a durable job record |
| Override operator disable | AI respects pause/disable commands unconditionally |

## Monitoring AI Behavior

### Check AI Service Status

```bash
# AI Executor status
globular ai executor status
# Output: incidents_processed, diagnoses_completed, actions_executed, actions_failed, uptime

# AI Watcher status
globular ai watcher status
# Output: running/paused, events_received, events_filtered, incidents_created

# AI Router status
globular ai router status
# Output: mode (neutral/observe/active), policies_computed, services_tracked
```

### View Recent Incidents

```bash
# List recent incidents
globular ai watcher incidents --limit 20

# Get incident details
globular ai watcher incident <incident-id>

# List pending approvals
globular ai watcher approvals
```

### View AI Job History

```bash
# List all AI jobs (diagnosis + remediation records)
globular ai executor jobs --limit 20

# Filter by state
globular ai executor jobs --state SUCCEEDED
globular ai executor jobs --state FAILED
globular ai executor jobs --state AWAITING_APPROVAL

# Get full job details
globular ai executor job <incident-id>
# Shows: diagnosis, confidence, evidence, action taken, outcome
```

### View AI Memory

```bash
# List recent memories
globular ai memory list --type DEBUG --limit 10

# Search memories
globular ai memory query --tags "dns,corruption" --text "unclean shutdown"

# Get specific memory
globular ai memory get <memory-id>
```

### Check Routing Policies

```bash
# Current routing policy (all services)
globular ai router policy

# Policy for specific service
globular ai router policy --service authentication

# Router mode
globular ai router status
```

## Approving and Denying AI Actions

When the AI proposes a Tier 2 action, it appears in the pending approvals list:

```bash
# View pending actions
globular ai watcher approvals
# Output:
# INCIDENT       SERVICE           ACTION                CONFIDENCE  RISK
# inc-abc123     authentication    RESTART_SERVICE       0.85        LOW
# inc-def456     postgresql        DRAIN_ENDPOINT        0.72        MEDIUM

# Review the diagnosis before approving
globular ai executor job inc-abc123
# Shows full evidence, root cause analysis, proposed action, expected outcome

# Approve
globular ai approve inc-abc123 --approver admin
# The action executes via remediation workflow

# Deny with reason
globular ai deny inc-def456 --reason "Will investigate manually"
```

## Verifying AI Actions

After an AI action executes, verify the outcome:

```bash
# Check the job result
globular ai executor job <incident-id>
# Shows: state=SUCCEEDED/FAILED, verification result

# Check the workflow that executed the action
globular workflow list --correlation "remediation/<incident-id>"
globular workflow get <run-id>
# Shows: step-by-step execution with timing and errors

# Check cluster health after the action
globular cluster health
globular doctor report --fresh
```

The remediation workflow includes a `verify_convergence` step that automatically re-checks the original finding. If the finding persists after the action, the job is marked FAILED.

## Controlling AI Behavior

### Pause Event Processing

Stop the AI Watcher from creating new incidents:

```bash
globular ai watcher pause
# No new events are processed
# Existing incidents continue their lifecycle

globular ai watcher resume
# Event processing resumes
```

### Set AI to Observe-Only

Ensure no actions are executed, only diagnosis:

```bash
# Set all watcher rules to OBSERVE tier
globular ai watcher set-config --default-tier OBSERVE
```

### Set Router to Passive

Prevent routing changes:

```bash
globular ai router set-mode NEUTRAL
# Router stops computing policies

globular ai router set-mode OBSERVE
# Router computes policies but doesn't apply them (log only)
```

### Disable AI Tool Groups in MCP

Prevent external AI agents from accessing specific tools:

Edit `/var/lib/globular/mcp/config.json`:
```json
{
  "tool_groups": {
    "ai_executor": false,
    "memory": false,
    "cli": false
  }
}
```

Restart the MCP server to apply changes.

### Remove AI Services Entirely

Unassign the AI profile from nodes to stop all AI services:

```bash
# Remove AI services from desired state
globular services desired remove ai_memory
globular services desired remove ai_executor
globular services desired remove ai_watcher
globular services desired remove ai_router
```

The cluster continues operating through its deterministic convergence model.

## Debugging Bad AI Decisions

### Diagnosis Was Wrong

If the AI misdiagnosed an incident:

```bash
# Review the diagnosis
globular ai executor job <incident-id>
# Check: root_cause, confidence, evidence

# Store feedback in AI Memory to prevent recurrence
globular ai memory store \
  --type FEEDBACK \
  --title "Incorrect diagnosis for auth crash" \
  --content "AI diagnosed OOM but actual cause was port conflict. Evidence showed exit code 137 which AI incorrectly attributed to OOM. Check 'address already in use' in logs before assuming OOM." \
  --tags "authentication,diagnosis,correction"
```

The AI Executor reads feedback memories during future diagnoses, reducing the chance of the same mistake.

### Action Made Things Worse

If an AI-triggered action caused additional problems:

```bash
# Check the action and its workflow
globular ai executor job <incident-id>
globular workflow get <workflow-run-id>

# The remediation workflow's verify_convergence step should have caught this
# If it didn't, the verification check may need refinement

# Roll back manually if needed
# (the convergence model will help — set desired state to the correct version)

# Store the lesson
globular ai memory store \
  --type FEEDBACK \
  --title "Restart made auth cascade worse" \
  --content "Restarting authentication during high load caused token cache cold start, cascading to all dependent services. Prefer draining over restart during peak hours." \
  --tags "authentication,restart,cascade,feedback"
```

### Too Many False Positives

If the watcher creates too many incidents for non-issues:

```bash
# Check watcher stats
globular ai watcher status
# High events_filtered vs incidents_created ratio = too sensitive

# Adjust rule thresholds
globular ai watcher set-config \
  --rule health-check-fail \
  --repeat-threshold 5 \
  --cooldown 300
# Now requires 5 occurrences within window, with 5-minute cooldown
```

### AI Actions Too Slow

If diagnosis takes too long:

```bash
# Check executor status
globular ai executor status
# Check: diagnoses_completed count and timing

# If Claude is slow, check the API connection
# The executor will fall back to deterministic rules if API is unavailable
```

## Practical Scenarios

### Scenario 1: Service Crash Auto-Remediation

```
Timeline:
T+0s:    Authentication service crashes on node-2
T+1s:    Event service publishes: service.exited {unit: authentication, node: node-2}
T+1s:    AI Watcher matches rule "service-crash" (Tier 1: AUTO_REMEDIATE)
T+11s:   Batch window closes, incident dispatched to AI Executor
T+12s:   Executor gathers evidence (cluster health, service logs, past incidents)
T+15s:   Claude diagnoses: "OOM kill, memory leak in token cache, confidence 0.85"
T+15s:   Tier 1: Auto-remediation begins
T+16s:   Remediation workflow starts: resolve → assess → execute → verify
T+20s:   Node Agent restarts authentication via systemctl
T+25s:   Health check passes, verification confirms finding resolved
T+25s:   Job record: SUCCEEDED, stored in AI Memory for future reference

Operator verification:
globular ai executor jobs --limit 5
# Shows: inc-xxx SUCCEEDED, authentication restart on node-2
```

### Scenario 2: Routing Anomaly Detection

```
Timeline:
T+0:     Node-3's authentication service P99 latency rises to 500ms (normal: 50ms)
T+5s:    AI Router scoring loop detects anomaly
T+5s:    Router computes new weights: node-1=40, node-2=40, node-3=20
T+5s:    (ACTIVE mode): xDS pushes updated weights to Envoy
T+10s:   Traffic shifts away from node-3
T+30s:   P99 latency stabilizes (degraded node gets less traffic)

Operator observation:
globular ai router policy --service authentication
# Shows: weights shifted, reason: "node-3 P99 3x above average"
```

### Scenario 3: Operator Denies Proposed Action

```
Timeline:
T+0:     AI diagnoses: "PostgreSQL WAL corruption, recommends CLEAR_STORAGE"
T+1s:    Tier 2: Job enters AWAITING_APPROVAL

Operator reviews:
globular ai executor job inc-abc123
# Diagnosis: WAL corruption, confidence 0.72, risk: MEDIUM
# Proposed: CLEAR_STORAGE for /var/lib/postgresql/wal/
# Expected outcome: PostgreSQL restarts with clean WAL

# Operator decides to investigate first
globular ai deny inc-abc123 --reason "Need to check if WAL can be recovered"

# Investigates manually, finds the WAL is recoverable
# Fixes without data loss
```

## What's Next

- [AI Rules](ai-rules.md): Complete constraint specification
- [AI Services](ai-services.md): Detailed service documentation
- [AI Developer Guide](ai-developer-guide.md): Building AI-aware services
