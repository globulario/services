# AI in Globular — Overview

## What the AI Layer Is

Globular includes an integrated AI subsystem designed to observe cluster state, diagnose problems, recommend actions, and — under controlled conditions — execute remediation. The AI layer is not an external tool or a chatbot bolted onto the platform. It is an operational component that operates within Globular's architecture: it reads state from etcd, acts through workflows, respects RBAC, and produces auditable records of every decision.

The AI layer consists of four services that form a closed loop:

```
Events/Metrics → AI Watcher (observe) → AI Executor (diagnose + act)
                                              ↓
                                        AI Memory (remember)
                                              ↓
                                        AI Router (optimize routing)
```

**AI Watcher** observes cluster events, filters them through configurable rules, and raises incidents.
**AI Executor** diagnoses incidents using Claude (Anthropic API) or deterministic rules, and executes approved remediation actions.
**AI Memory** stores knowledge persistently in ScyllaDB — patterns, root causes, decisions, and session context — so that AI reasoning improves over time and survives restarts.
**AI Router** computes dynamic routing policies (endpoint weights, circuit breaker settings, drain strategies) based on real-time telemetry.

## Implementation Status

| Component | Status | Backend |
|-----------|--------|---------|
| AI Memory | **Implemented** | ScyllaDB (RF=3), full CRUD + sessions + text search |
| AI Executor | **Implemented** | Anthropic API (primary) + Claude CLI (fallback) + etcd job store |
| AI Watcher | **Implemented** | Event service subscription + configurable rules (12 default) |
| AI Router | **Implemented** | In-memory scoring loop, policy computation every ~5s |
| MCP Server | **Implemented** | 65+ tools via JSON-RPC 2.0, HTTP transport, tool group control |
| Workflow integration | **Implemented** | `ACTOR_AI_DIAGNOSER` in workflow actor enum; doctor remediation workflow |
| xDS integration for router | **Partial** | Router computes policies; xDS watcher integration pending |
| Action execution backends | **Partial** | restart_service, drain_endpoint, circuit_breaker implemented; notify_admin, block_ip pending |

## How AI Differs from Ordinary Automation

Globular already has automation without AI: the convergence model detects drift and dispatches workflows, the cluster doctor checks invariants and proposes remediation, and circuit breakers prevent cascade failures. This automation is deterministic — it follows fixed rules.

The AI layer adds three capabilities that deterministic automation cannot provide:

**1. Diagnosis from partial evidence.** When a service fails, the deterministic system knows it failed and can restart it. The AI layer can analyze logs, metrics, and historical incidents to determine *why* it failed — distinguishing between a memory leak, a configuration error, a dependency timeout, and a binary bug. This diagnosis informs the response: a memory leak needs a resource limit increase, not another restart.

**2. Pattern recognition across time.** The AI memory service stores outcomes of past incidents. When a similar pattern recurs (same service, same error signature, same time of day), the AI can recognize it and apply the previously successful remediation immediately instead of going through the full diagnosis cycle.

**3. Contextual routing optimization.** The AI router observes latency, error rates, and endpoint health over time. It can detect subtle degradation (increasing P99 latency on one node) that doesn't trigger a binary health check failure, and proactively adjust routing weights to shift traffic away from the degrading endpoint.

## How AI Fits into the Architecture

### Relationship to Workflows

AI actions flow through the workflow engine, the same execution backbone used for service deployment and cluster operations. When the AI executor determines that a service needs to be restarted, it does not call systemctl directly. It triggers a remediation workflow:

```
resolve_finding → assess_risk → [require_approval] → execute_remediation → verify_convergence
```

This workflow is defined in YAML (`remediate.doctor.finding.yaml`), executed by the Workflow Service, and recorded with full step-by-step audit. The AI's action is indistinguishable from any other workflow — it has the same observability, the same failure handling, and the same audit trail.

### Relationship to the Convergence Model

The AI layer does not replace the convergence model. It augments it:

- The convergence model handles **expected** state changes (operator sets desired state → workflows install services)
- The AI layer handles **unexpected** state changes (service crashes, anomalies, drift that the convergence model doesn't know how to fix)

The AI layer reads the same four truth layers (Repository → Desired → Installed → Runtime) and never invents a fifth. It proposes actions that the convergence model then executes.

### Relationship to etcd

The AI layer follows the same etcd rules as every other component:
- AI Memory stores knowledge in ScyllaDB (not etcd, to avoid bloating the consensus store)
- AI Executor stores durable job records in etcd at `/globular/ai/jobs/{incident_id}`
- AI Watcher reads event configuration from etcd
- AI Router reads service configuration from etcd

The AI layer never stores authoritative cluster state outside of etcd. ScyllaDB is used for AI-specific data (memory, conversation history) that is valuable but not operationally critical — if ScyllaDB is lost, the cluster still operates normally; only AI memory is affected.

### Relationship to RBAC

All AI services are gRPC services with standard RBAC annotations. AI actions are subject to the same permission model as human actions:

- AI Executor RPCs require appropriate roles (observe, remediate, approve)
- MCP tools enforce read-only access by default
- The 3-tier permission model (OBSERVE, AUTO_REMEDIATE, REQUIRE_APPROVAL) maps to RBAC roles

### Relationship to Observability

The AI layer is both a consumer and producer of observability data:

**Consumer**: AI Watcher subscribes to cluster events. AI Executor queries cluster health, doctor reports, and Prometheus metrics for diagnosis. AI Router reads latency and error-rate metrics.

**Producer**: AI Executor produces durable job records (diagnosis + action + outcome). AI Memory produces searchable knowledge. All AI actions flow through workflows, which produce the standard workflow run audit trail.

## The MCP Interface

The **MCP (Model Context Protocol) Server** is the structured interface through which external AI agents (like Claude Code) interact with the cluster. It exposes 65+ diagnostic tools organized into 32 tool groups, accessible via JSON-RPC 2.0 over HTTP.

The MCP server is **read-only by default**. It provides tools for:
- Cluster health inspection, node status, desired state, drift reports
- Workflow run history and diagnostics
- Backup job status, retention, recovery posture
- RBAC permission queries
- Prometheus metrics and alerts
- AI memory operations (store, query, get, update, delete)
- AI executor status and peer collaboration

Mutating operations (CLI execution, package publish) are gated behind approval workflows and governance checks. The MCP server never bypasses the platform's security model.

## What AI Cannot Do

The AI layer is explicitly constrained:

- It cannot bypass etcd as the source of truth
- It cannot modify cluster state except through workflows and approved APIs
- It cannot execute shell commands directly on nodes
- It cannot invent infrastructure state from partial evidence
- It cannot override RBAC permissions
- It cannot hide its actions from the audit trail
- It cannot take high-risk actions without human approval (Tier 2)

These constraints are not aspirational — they are enforced by the architecture. AI actions flow through gRPC services with interceptors, RBAC checks, and audit logging. There is no backdoor.

See [AI Rules](ai/ai-rules.md) for the complete constraint specification.
