# AI Rules

This document defines the strict operational rules that every AI agent, AI service, and AI-assisted tool must follow when operating within a Globular cluster. These rules are non-negotiable. They reflect the architecture of the platform and are enforced by the gRPC interceptor chain, RBAC, workflow engine, and audit system.

## Rule 1: etcd Is the Single Source of Truth

AI must treat etcd as the sole authoritative source for all cluster state. AI must not:
- Store authoritative cluster state outside etcd
- Use environment variables as a configuration source
- Cache state locally and treat the cache as authoritative
- Assume state based on partial observation without verifying against etcd

**What this means in practice**: When an AI agent needs to know the desired version of a service, it reads `/globular/resources/DesiredService/{name}` from etcd (via the controller API). It does not infer the version from logs, metrics, or previous observations.

AI-specific data (memory, conversation history, learned patterns) lives in ScyllaDB, not etcd. This data is supplementary — it informs AI reasoning but is never treated as cluster truth.

**Status**: Implemented. Enforced by the architecture — AI services use gRPC APIs that read from etcd, not direct etcd access.

## Rule 2: All Mutations Flow Through Workflows

AI must not mutate cluster state through ad-hoc mechanisms. Every state change must go through the workflow engine or an approved gRPC API.

AI must not:
- Execute shell commands directly on nodes
- Modify etcd keys directly (except AI-owned namespaces like `/globular/ai/`)
- Call systemctl, curl, or any OS command to change system state
- Use the Node Agent's ControlService RPC to restart services without a workflow

AI must:
- Trigger remediation through the `remediate.doctor.finding` workflow
- Use the Cluster Controller's APIs for desired-state changes
- Use the AI Executor's structured action dispatch for approved actions

**What this means in practice**: When the AI determines that a service should be restarted, it calls `AI Executor → ProcessIncident → Remediation Workflow → Cluster Doctor → Node Agent`. It does not call `Node Agent → ControlService(restart)` directly.

**Status**: Implemented. The AI Executor routes actions through the cluster doctor's remediation workflow. Direct node manipulation requires explicit RBAC roles that AI service accounts do not have.

## Rule 3: Observe Before Acting

AI must always diagnose before prescribing. The required sequence is:

```
1. OBSERVE  — Gather evidence from cluster state, metrics, logs, and history
2. DIAGNOSE — Analyze evidence to identify root cause
3. RECOMMEND — Propose a specific, typed action with expected outcome
4. [APPROVE] — For Tier 2 actions, wait for human approval
5. EXECUTE  — Carry out the approved action through a workflow
6. VERIFY   — Confirm the action achieved the desired outcome
```

AI must not:
- Skip observation and jump to action based on a trigger event alone
- Act on a single signal without corroborating evidence
- Execute a remediation without first producing a diagnosis record
- Consider an action complete without verifying the outcome

**What this means in practice**: When the AI Watcher detects a `service.exited` event, it does not immediately restart the service. It creates an incident, the AI Executor gathers evidence (cluster health, service logs, historical incidents with the same signature), produces a diagnosis, and only then determines the appropriate action.

**Status**: Implemented. The AI Executor's `ProcessIncident` pipeline follows: detect → diagnose (evidence gathering + Claude analysis) → determine action → execute based on tier → store result.

## Rule 4: Three-Tier Permission Model

AI actions are classified into three tiers with escalating authorization requirements:

| Tier | Name | Authorization | Examples |
|------|------|--------------|----------|
| 0 | OBSERVE | None required | Read cluster state, diagnose incidents, store findings |
| 1 | AUTO_REMEDIATE | Pre-approved by operator config | Restart a crashed service, clear corrupted cache |
| 2 | REQUIRE_APPROVAL | Human approval token required | Drain an endpoint, modify circuit breaker settings |

AI must:
- Default to OBSERVE (Tier 0) unless explicitly configured otherwise
- Never escalate its own tier — the tier is set by the operator in the watcher rules
- Automatically downgrade to OBSERVE if the diagnosis confidence is low or the risk is high
- Record the tier, confidence, and risk level in the job record

AI must not:
- Execute Tier 1 actions without being explicitly configured to do so
- Execute Tier 2 actions without a valid approval token
- Treat a previous approval as blanket authorization for future actions

**Status**: Implemented. The AI Watcher rules specify the tier per event pattern. The AI Executor enforces tier checks before action execution. Tier 2 requires a signed approval token validated on execution.

## Rule 5: Actions Must Be Typed and Bounded

AI must not execute free-form commands. Every AI action must be one of a defined set of typed actions:

| Action Type | Scope | Risk |
|-------------|-------|------|
| `ACTION_RESTART_SERVICE` | Single systemd unit on single node | Low |
| `ACTION_CLEAR_STORAGE` | Delete corrupted local data (BadgerDB, WAL) | Low |
| `ACTION_RENEW_CERT` | Request TLS certificate renewal | Low |
| `ACTION_DRAIN_ENDPOINT` | Set endpoint weight to 0 in xDS routing | Medium |
| `ACTION_CIRCUIT_BREAKER` | Open circuit breaker for a service | Medium |
| `ACTION_NOTIFY_ADMIN` | Send notification to operator | None |

AI must not:
- Construct arbitrary shell commands
- Concatenate user input into executable strings
- Execute actions not in the typed action set
- Expand the action set without operator approval and code changes

**What this means in practice**: The AI Executor's `remediator` has a fixed `actionDispatcher` that routes action types to backend implementations. There is no `ACTION_EXEC_SHELL` and there never will be.

**Status**: Implemented. Action types are defined in the proto file and the dispatcher only routes known types. Unknown action types are rejected.

## Rule 6: Every Action Produces an Audit Record

AI must produce a durable, queryable record for every action it takes. The record must include:

- **Incident ID**: Unique identifier linking the event, diagnosis, and action
- **Timestamp**: When the action was taken
- **Tier**: The permission tier under which the action was executed
- **Diagnosis**: The AI's analysis including root cause, confidence, and evidence
- **Action**: The typed action that was taken
- **Outcome**: Whether the action succeeded or failed
- **Verification**: Whether the post-action verification passed

AI must not:
- Take actions without creating a job record
- Delete or modify job records after creation
- Log actions only to stdout (which can be lost)

**Status**: Implemented. AI Executor stores durable job records in etcd at `/globular/ai/jobs/{incident_id}`. Jobs progress through a state machine: DETECTED → DIAGNOSING → DIAGNOSED → [AWAITING_APPROVAL →] EXECUTING → SUCCEEDED/FAILED → CLOSED.

## Rule 7: AI Must Not Invent State

AI must reason only from observable, verifiable evidence. It must not:
- Infer cluster state from incomplete data and treat the inference as fact
- Assume a service is running because it was running 5 minutes ago
- Assume a configuration value exists because it existed in a previous conversation
- Fill in missing data with defaults or assumptions

**What this means in practice**: If the AI needs to know whether etcd is healthy, it queries the cluster health API or the doctor report. It does not assume etcd is healthy because the last health check passed. Stale memory records must be verified against current state before being used for decisions.

**Status**: Architectural principle enforced by design. The AI Executor's evidence gathering phase queries live cluster state, not cached data. AI Memory records are supplementary context, not truth.

## Rule 8: AI Must Respect the 4-Layer State Model

AI must understand and respect the four independent truth layers:

1. **Artifact** (Repository) — Does the version exist?
2. **Desired** (Controller) — What should be running?
3. **Installed** (Node Agent) — What is actually installed?
4. **Runtime** (systemd + health) — Is it running and healthy?

AI must not:
- Collapse these layers (e.g., assume "desired" means "installed")
- Skip layers when diagnosing (e.g., check only runtime without checking if desired matches installed)
- Modify one layer expecting another to automatically follow without a workflow

**Status**: Implemented. The AI diagnostic pipeline queries all four layers via the cluster controller, node agent, and doctor APIs.

## Rule 9: AI Must Distinguish Observation, Recommendation, and Execution

AI must clearly separate three modes of operation in its outputs:

- **Observation**: "Service X is not running on node Y" — a factual statement derived from cluster state
- **Recommendation**: "Service X should be restarted because the exit code indicates OOM" — an analysis with proposed action
- **Execution**: "Restarted service X on node Y via workflow wf-run-abc123" — a completed action with audit trail

AI must not:
- Present recommendations as facts
- Present observations as actions taken
- Execute without explicitly transitioning from recommendation to execution

**Status**: Implemented. The AI Executor's job state machine enforces this: DIAGNOSED (observation + recommendation) → AWAITING_APPROVAL or EXECUTING (explicit transition) → SUCCEEDED/FAILED (execution outcome).

## Rule 10: AI Must Fail Safe

When the AI subsystem itself fails, the cluster must continue operating normally:

- If AI Memory (ScyllaDB) is down, the cluster operates without AI memory — no degradation of core services
- If AI Executor is down, incidents are not diagnosed but the convergence model still handles drift
- If AI Watcher is down, events are not AI-processed but the doctor and convergence model still function
- If the Anthropic API is unavailable, the AI Executor falls back to deterministic rules

AI services are **not** in the critical path of cluster operations. They are supplementary services that enhance operations but are never required for basic functionality.

**Status**: Implemented. AI services are optional (not in the `core` profile by default). The AI Executor has a deterministic fallback when Claude is unavailable. The cluster doctor operates independently of AI services.

## Rule 11: AI Must Be Disableable

Operators must be able to disable or constrain AI behavior at any time:

- **Pause the watcher**: `globular ai watcher pause` — stops event processing
- **Set mode to OBSERVE**: Ensures no actions are taken, only diagnosis
- **Disable tool groups**: MCP config allows disabling any tool group
- **Remove AI profile**: Unassigning the AI profile from nodes stops all AI services

AI must not resist being disabled. There is no "override" or "emergency mode" where AI acts against operator instructions.

**Status**: Implemented. AI Watcher has Pause/Resume RPCs. AI Router has mode switching (NEUTRAL/OBSERVE/ACTIVE). MCP tool groups can be disabled in config.

## Rule 12: AI Must Operate Within RBAC Boundaries

AI services authenticate with service account tokens and are subject to the same RBAC enforcement as all other services:

- AI Executor's service account has permissions for diagnosis and approved remediation — not for cluster membership changes, desired-state modification, or package publishing
- MCP tools enforce read-only access by default
- Escalated operations (CLI execution, package operations) require explicit governor approval

AI must not:
- Attempt to bypass RBAC by using a different authentication method
- Accumulate permissions over time
- Request permissions beyond what its role allows

**Status**: Implemented. AI services use standard gRPC interceptors with JWT authentication and RBAC checks.

## Summary

| Rule | Enforcement | Status |
|------|-------------|--------|
| etcd is source of truth | Architecture (gRPC APIs read from etcd) | Implemented |
| Mutations through workflows | Remediation workflow pipeline | Implemented |
| Observe before acting | ProcessIncident pipeline | Implemented |
| Three-tier permissions | Tier checks in executor | Implemented |
| Typed, bounded actions | Action dispatcher with fixed types | Implemented |
| Audit records for every action | etcd job store | Implemented |
| No invented state | Live evidence gathering | Implemented |
| Respect 4-layer model | Multi-layer diagnostic queries | Implemented |
| Separate observation/recommendation/execution | Job state machine | Implemented |
| Fail safe | Optional services, deterministic fallback | Implemented |
| Disableable | Pause/Resume, mode switching, tool groups | Implemented |
| RBAC boundaries | gRPC interceptors, service account roles | Implemented |
