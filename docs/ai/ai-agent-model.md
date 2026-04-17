# AI Agent Model

This document defines what an AI agent is in Globular, what it can observe, what tools it may use, what actions it can take, and the boundaries within which it must operate.

## What an AI Agent Is

In Globular, an AI agent is any software component that uses reasoning (LLM-based or rule-based) to observe cluster state, diagnose problems, recommend actions, or execute remediation. This includes:

- **Claude Code** interacting with the cluster through the MCP server
- **AI Executor** performing automated incident diagnosis and remediation
- **AI Watcher** detecting and classifying events for AI processing
- **AI Router** computing dynamic routing policies
- **Custom agents** built by developers using Globular's APIs

An AI agent is NOT a privileged system component. It operates at the same level as a human operator — it authenticates, it has RBAC roles, its actions are audited, and it can be constrained or disabled.

## Agent Lifecycle

Every AI agent interaction follows a five-phase lifecycle:

```
OBSERVE → REASON → RECOMMEND → [EXECUTE] → VERIFY
```

### Phase 1: Observation

The agent gathers evidence from cluster state. It may read:

**Cluster state** (via controller APIs / MCP tools):
- Cluster health: `cluster_get_health`
- Node status: `cluster_list_nodes`, `cluster_get_node_full_status`
- Desired state: `cluster_get_desired_state`
- Drift report: `cluster_get_drift_report`
- Convergence detail: `cluster_get_convergence_detail`
- Operational snapshot: `cluster_get_operational_snapshot`

**Diagnostics** (via doctor / MCP tools):
- Doctor report: `cluster_get_doctor_report`
- Finding explanation: `cluster_explain_finding`
- Reconciliation status: `cluster_get_reconcile_status`

**Service state** (via node agent / MCP tools):
- Installed packages: `nodeagent_list_installed_packages`
- Service logs: `nodeagent_get_service_logs`, `nodeagent_search_logs`
- Certificate status: `nodeagent_get_certificate_status`
- Node inventory: `nodeagent_get_inventory`

**Package state** (via repository / MCP tools):
- Artifact manifests: `repository_get_artifact_manifest`
- Artifact versions: `repository_get_artifact_versions`
- Artifact search: `repository_search_artifacts`

**Workflow state** (via workflow service / MCP tools):
- Workflow runs: `workflow_list_runs`, `workflow_get_run`
- Service workflow status: `workflow_get_service_status`
- Workflow diagnostics: `workflow_diagnose`

**Metrics** (via monitoring / MCP tools):
- Instant queries: `metrics_query`
- Range queries: `metrics_query_range`
- Alert status: `metrics_alerts`
- Recording/alerting rules: `metrics_rules`
- Scrape target status: `metrics_targets`

**Backup state** (via backup manager / MCP tools):
- Backup list and details: `backup_list_backups`, `backup_get_backup`
- Job status: `backup_list_jobs`, `backup_get_job`
- Recovery posture: `backup_get_recovery_posture`
- Retention status: `backup_get_retention_status`

**RBAC state** (via RBAC service / MCP tools):
- Permission checks: `rbac_validate_access`, `rbac_validate_action`
- Role bindings: `rbac_list_role_bindings`
- Resource permissions: `rbac_get_resource_permissions`

**AI knowledge** (via AI Memory / MCP tools):
- Past incidents: `memory_query` with tags and type filters
- Historical diagnoses: `memory_get` for specific records
- Session context: `session_resume` for prior conversation state

**Infrastructure** (via etcd / MCP tools):
- Configuration keys: `etcd_get`
- Service configuration: `service_config_get`, `service_config_list`
- Schema descriptions: `schema_describe`, `schema_list`

### What Agents Must Not Observe

Agents must not:
- Read raw credentials from etcd or files
- Access other tenants' data without RBAC authorization
- Read file system paths outside configured allowlists
- Query databases without connection/collection allowlists

These restrictions are enforced by the MCP server's safety module and RBAC.

### Phase 2: Reasoning

The agent analyzes the gathered evidence to form a diagnosis. Reasoning may be:

**LLM-based** (AI Executor with Claude):
- Evidence is formatted into a structured prompt
- Claude analyzes the evidence and returns: root_cause, confidence (0-1), proposed_action, risk_level
- The prompt includes cluster context, historical incidents, and the specific symptoms

**Rule-based** (deterministic fallback):
- Event pattern → known root cause mapping
- Configured in the AI Watcher's default rules
- Used when Claude is unavailable or for well-known patterns

**Hybrid** (preferred):
- Deterministic rules handle common cases immediately (service crash → restart)
- LLM handles novel or complex cases (multi-service failure, performance degradation)

### What Agents Must Not Do During Reasoning

Agents must not:
- Treat inferences as facts — all conclusions must be labeled with confidence
- Hallucinate cluster state that wasn't observed
- Ignore contradictory evidence
- Use stale memory records without verifying them against current state

### Phase 3: Recommendation

The agent produces a typed recommendation:

```
Recommendation {
    incident_id: "inc-abc123"
    diagnosis: {
        root_cause: "Service authentication OOM killed due to memory leak in token cache"
        confidence: 0.85
        evidence: ["exit code 137", "dmesg OOM entries", "memory growth in metrics"]
    }
    proposed_action: ACTION_RESTART_SERVICE
    target: { node: "node-2", service: "authentication" }
    risk_level: LOW
    expected_outcome: "Service restarts with fresh memory allocation"
    tier: AUTO_REMEDIATE
}
```

Recommendations must:
- Reference specific evidence (not vague descriptions)
- Use typed action constants (not free-form strings)
- Include a confidence level
- Include a risk assessment
- Specify the expected outcome (for verification)

### Phase 4: Execution (Conditional)

Execution depends on the tier:

**Tier 0 (OBSERVE)**: No execution. The recommendation is stored in AI Memory for future reference.

**Tier 1 (AUTO_REMEDIATE)**: The action executes automatically through the remediation workflow:
```
AI Executor → ProcessIncident → Remediation Workflow →
    resolve_finding → assess_risk → execute_remediation → verify_convergence
```

**Tier 2 (REQUIRE_APPROVAL)**: The recommendation is presented to a human operator. Execution proceeds only after an explicit approval token is provided:
```bash
globular ai approve <incident-id> --approver admin
```

### What Agents Must Not Do During Execution

Agents must not:
- Execute actions directly (SSH, systemctl, etcd put)
- Bypass the workflow engine
- Execute without a job record
- Execute Tier 2 actions without approval
- Execute actions not in the typed action set

### Phase 5: Verification

After execution, the agent verifies the outcome:

1. Re-query the cluster state affected by the action
2. Compare against the expected outcome
3. Record the verification result in the job record
4. If verification fails, the job is marked FAILED (not automatically retried at a different tier)

The remediation workflow includes a `verify_convergence` step that re-runs the doctor check to confirm the original finding is resolved.

## Agent Boundaries

### What Agents May Read

| Data Source | Access Method | Restriction |
|------------|---------------|-------------|
| Cluster health | Controller API / MCP | None (read-only) |
| Node status | Node Agent API / MCP | RBAC: node-read role |
| Service logs | Node Agent API / MCP | RBAC: node-read role |
| Desired state | Controller API / MCP | RBAC: cluster-read role |
| Workflow history | Workflow API / MCP | RBAC: workflow-read role |
| Prometheus metrics | Monitoring API / MCP | RBAC: monitoring-read role |
| Backup status | Backup API / MCP | RBAC: backup-read role |
| RBAC state | RBAC API / MCP | RBAC: rbac-read role |
| AI Memory | Memory API / MCP | Project-scoped |
| etcd keys | etcd API / MCP | Read-only, scoped |
| File system | File tools / MCP | Allowlisted paths only |

### What Agents May Write

| Data Target | Access Method | Restriction |
|------------|---------------|-------------|
| AI Memory | Memory API / MCP | Project-scoped, AI-owned namespace |
| AI Job records | Executor API | AI-owned etcd namespace `/globular/ai/` |
| Incident diagnosis | Executor ProcessIncident | Through structured pipeline |
| Remediation actions | Executor → Workflow | Through remediation workflow, tier-gated |

### What Agents Must Never Write

| Data Target | Reason |
|------------|--------|
| Desired state (`/globular/resources/DesiredService/`) | Operator-owned; AI may recommend but not change |
| Node packages (`/globular/nodes/{id}/packages/`) | Node Agent-owned; modified only by workflows |
| Service configuration (`/globular/services/`) | Service-owned; modified only through config APIs |
| etcd cluster membership | Controller-owned; requires admin role |
| File system (binaries, configs, certs) | Node Agent-owned; modified only by workflows |
| RBAC role bindings | Admin-owned; AI cannot escalate permissions |

## Interaction Map

<img src="/docs/assets/diagrams/ai-agent-map.svg" alt="AI agent interaction map" style="width:100%;max-width:850px">

## Multi-Agent Coordination

### Peer Collaboration

AI Executor instances running on multiple nodes coordinate through peer RPCs:

- **Ping**: Discovery — "Are you alive? Is Claude available?"
- **ShareObservation**: Evidence sharing — "I see X on my node, do you see it too?"
- **ProposeAction**: Consensus — "I want to restart service Y, do you agree?"
- **NotifyActionTaken**: Coordination — "I restarted service Y, here's the result"

The `ProposeAction` RPC returns a vote: APPROVE, REJECT, ABSTAIN, or ESCALATE. This prevents multiple executors from taking conflicting actions simultaneously.

### Claude Code + AI Executor

Claude Code (external agent) and AI Executor (internal agent) can collaborate:

1. Claude Code observes cluster state through MCP tools
2. Claude Code sends a prompt to AI Executor via `ai_executor_send_prompt`
3. AI Executor processes the prompt with its cluster context
4. AI Executor returns diagnosis or recommendation
5. Claude Code presents the result to the operator

This separation ensures that Claude Code's reasoning benefits from the executor's direct cluster access without giving Claude Code direct mutation capability.

## Failure Cases

### Agent Lost Connection to MCP

If the MCP connection drops, the agent loses access to cluster state. The agent must not:
- Continue making recommendations based on stale data
- Assume the last-known state is still current
- Queue actions for execution when connection is restored (state may have changed)

### AI Service Unavailable

If AI Memory, Executor, or Watcher is down:
- The cluster continues operating through its deterministic convergence model
- MCP tools that depend on AI services return appropriate errors
- The agent should inform the operator that AI capabilities are degraded

### Conflicting Diagnoses

If multiple agents (or multiple executor instances) produce different diagnoses for the same incident:
- The peer voting mechanism resolves conflicts (majority wins)
- If no majority, the action escalates to REQUIRE_APPROVAL
- The operator makes the final decision

### Diagnosis Confidence Too Low

If the AI Executor's diagnosis has confidence < 0.5:
- The action automatically downgrades to OBSERVE tier
- The diagnosis is stored in AI Memory for pattern accumulation
- No remediation is attempted
- The operator is notified of the low-confidence finding
