# AI Services

This document describes the four AI services in Globular: AI Memory, AI Executor, AI Watcher, and AI Router. For each service, it explains the role, inputs, outputs, decision scope, integration points, and implementation status.

## AI Memory Service

**Port**: 10200 (gRPC), 10201 (gRPC-Web proxy)
**Backend**: ScyllaDB (replication factor 3)
**Status**: Fully implemented

### Role

AI Memory provides persistent, cluster-scoped knowledge storage for AI agents. It stores patterns, root causes, decisions, session context, and operational knowledge that persists across conversations, restarts, and node failures.

### Data Model

Memories are organized by project and type:

| Type | Purpose | Example |
|------|---------|---------|
| `FEEDBACK` | User corrections and confirmed approaches | "Don't restart etcd without checking quorum first" |
| `ARCHITECTURE` | System design knowledge | "DNS service uses ScyllaDB for zone storage" |
| `DECISION` | Design decisions with rationale | "Chose restic over borgbackup for file-level backup" |
| `DEBUG` | Debugging sessions and root causes | "BadgerDB corruption caused by unclean shutdown" |
| `SESSION` | Conversation session summaries | Auto-created by `session_save` |
| `USER` | User profile, preferences, expertise | "User is a senior SRE focused on HA" |
| `PROJECT` | Ongoing work, goals, deadlines | "Merge freeze begins 2026-03-05" |
| `REFERENCE` | Pointers to external resources | "Pipeline bugs tracked in Linear project INGEST" |
| `SCRATCH` | Temporary analysis (auto-expires via TTL) | Intermediate calculation during diagnosis |
| `SKILL` | Operational skill playbooks | Reusable diagnostic/remediation sequences |

Each memory has:
- `id`: UUID (auto-generated)
- `project`: Partition key (required, e.g., "globular-services")
- `tags`: Searchable, AND-filtered (e.g., "dns,corruption,bug")
- `metadata`: Flexible key-value (e.g., `{"root_cause": "unclean-shutdown", "confidence": "high"}`)
- `related_ids`: Bidirectional links to other memories (knowledge graph)
- `reference_count`: Auto-incremented on `Get` (surfaces frequently used knowledge)
- `ttl_seconds`: Auto-expiry (0 = permanent, >0 = ScyllaDB native TTL)

### RPC Surface

| RPC | Purpose | Mutating |
|-----|---------|----------|
| `Store` | Create a memory record | Yes |
| `Query` | Search by type, tags, text (substring match on title+content) | No |
| `Get` | Retrieve by ID (increments reference_count) | Side-effect |
| `Update` | Merge-update (only non-empty fields change) | Yes |
| `Delete` | Remove by ID | Yes |
| `List` | Lightweight summaries (no content field) | No |
| `SaveSession` | Persist conversation context | Yes |
| `ResumeSession` | Fuzzy match on topic+summary | No |
| `Stop` | Graceful shutdown | N/A |

### What It May Change

- Its own ScyllaDB tables (`ai_memory.memories`, `ai_memory.sessions`)
- Nothing else. AI Memory has no access to cluster state, etcd, or service configuration.

### Integration

- **AI Executor**: Stores diagnosis results and learned patterns
- **AI Watcher**: No direct integration (watcher triggers executor, which uses memory)
- **MCP Server**: Exposes all memory RPCs as MCP tools
- **Claude Code**: Reads/writes memory through MCP for cross-conversation knowledge

---

## AI Executor Service

**Port**: 10230 (gRPC), 10231 (gRPC-Web proxy)
**Backend**: etcd (job store), ScyllaDB (conversation history), Anthropic API (diagnosis)
**Status**: Fully implemented

### Role

AI Executor is the "hands and brain" of the AI system. It receives incidents from the AI Watcher (or direct invocation), diagnoses them using Claude or deterministic rules, and executes approved remediation actions through the workflow engine.

### Diagnosis Pipeline

```
Incident received
    │
    ▼
Evidence gathering (parallel):
  ├── Cluster health from controller
  ├── Past incidents from AI Memory
  ├── Deterministic rule matching
  └── Service logs / metrics
    │
    ▼
AI analysis:
  ├── Primary: Claude (Anthropic API) analyzes evidence
  └── Fallback: Deterministic rules if API unavailable
    │
    ▼
Diagnosis produced:
  ├── root_cause (string)
  ├── confidence (0.0 - 1.0)
  ├── proposed_action (typed action)
  └── risk_level (low/medium/high)
    │
    ▼
Tier enforcement:
  ├── Tier 0 (OBSERVE): Store diagnosis, no action
  ├── Tier 1 (AUTO_REMEDIATE): Execute immediately
  └── Tier 2 (REQUIRE_APPROVAL): Wait for human
```

### RPC Surface

**Incident Processing:**

| RPC | Purpose | Mutating |
|-----|---------|----------|
| `ProcessIncident` | Full pipeline: detect → diagnose → act | Yes |
| `GetDiagnosis` | Retrieve diagnosis for incident | No |
| `GetStatus` | Operational stats (incidents processed, actions executed) | No |
| `ListActions` | Recent action history | No |
| `ApproveAction` | Provide approval token for Tier 2 action | Yes |
| `DenyAction` | Reject proposed Tier 2 action | Yes |
| `RetryAction` | Retry a failed action | Yes |
| `GetJob` | Full durable job record | No |
| `ListJobs` | List jobs with state filter | No |

**Conversation:**

| RPC | Purpose | Mutating |
|-----|---------|----------|
| `SendPrompt` | Send prompt to Claude (streaming response) | Yes (creates conversation) |
| `GetConversation` | Retrieve conversation history | No |
| `ListConversations` | List conversations for a user | No |
| `DeleteConversation` | Remove conversation | Yes |

**Peer Collaboration:**

| RPC | Purpose | Mutating |
|-----|---------|----------|
| `Ping` | Health check + AI availability | No |
| `ShareObservation` | Share evidence for peer confirmation | No (read) |
| `ProposeAction` | Request peer vote on proposed action | No (advisory) |
| `NotifyActionTaken` | Inform peers of completed action | No (advisory) |

### What It May Change

- etcd keys under `/globular/ai/jobs/` (job records)
- ScyllaDB conversation tables (conversation history)
- AI Memory (stores diagnoses and learned patterns)
- Cluster state via remediation workflows (restart services, drain endpoints, etc.) — only through the workflow engine, never directly

### What It Must Never Change Directly

- Desired state (`/globular/resources/DesiredService/`)
- Node packages (`/globular/nodes/`)
- Service configuration (`/globular/services/`)
- RBAC bindings
- File system on any node

### Claude Integration

The executor connects to Claude via two backends:

**Anthropic API** (primary):
- Auth: OAuth (enterprise) or API key
- Model: `claude-opus-4-1` (configurable)
- Max tokens: 4096 (configurable)
- Hot-reload: Credentials watched every 5 minutes, swapped without restart

**Claude CLI** (fallback):
- Used when API is unavailable
- Same diagnostic prompts, different transport

### Job State Machine

```
JOB_DETECTED → JOB_DIAGNOSING → JOB_DIAGNOSED →
    ├── Tier 0: → JOB_CLOSED
    ├── Tier 1: → JOB_EXECUTING → JOB_SUCCEEDED / JOB_FAILED → JOB_CLOSED
    └── Tier 2: → JOB_AWAITING_APPROVAL →
                    ├── Approved: → JOB_APPROVED → JOB_EXECUTING → ...
                    ├── Denied: → JOB_DENIED → JOB_CLOSED
                    └── Expired: → JOB_EXPIRED → JOB_CLOSED
```

### Action Types

| Action | What It Does | How |
|--------|-------------|-----|
| `ACTION_RESTART_SERVICE` | Restart a systemd unit | Via cluster doctor → remediation workflow |
| `ACTION_CLEAR_STORAGE` | Delete corrupted local data | Via cluster doctor remediation |
| `ACTION_RENEW_CERT` | Request TLS certificate renewal | Via node agent |
| `ACTION_DRAIN_ENDPOINT` | Set endpoint weight to 0 in xDS | Via AI Router or direct xDS config |
| `ACTION_CIRCUIT_BREAKER` | Open circuit breaker for service | Via AI Router |
| `ACTION_NOTIFY_ADMIN` | Send operator notification | Planned (not fully implemented) |
| `ACTION_BLOCK_IP` | Add to network blocklist | Planned (not fully implemented) |

---

## AI Watcher Service

**Port**: 10210 (gRPC), 10211 (gRPC-Web proxy)
**Backend**: In-memory event batching + etcd for rule configuration
**Status**: Fully implemented

### Role

AI Watcher is the observation layer. It subscribes to cluster events, filters them through configurable rules, batches related events, and dispatches incidents to the AI Executor for diagnosis.

### Event Processing Pipeline

```
Event Service (pub/sub)
    │
    ▼
Subscribe to topics:
  cluster.*, service.*, node.*, alert.*, operation.*, workflow.*
    │
    ▼
Rule matching:
  Pattern match (glob), severity filter, repeat threshold, cooldown
    │
    ▼
Batch window (default 10s):
  Collect related events, fire once window closes
    │
    ▼
Dispatch to AI Executor:
  ProcessIncident(incident_id, tier, trigger_event, metadata)
```

### Default Rules (12 shipped)

| Rule | Event Pattern | Tier | Behavior |
|------|--------------|------|----------|
| `service-crash` | `service.exited` | AUTO_REMEDIATE | Restart crashed service |
| `service-restart-exhausted` | `service.restart_failed` | AUTO_REMEDIATE | Diagnose restart failure |
| `health-check-fail` | `cluster.health.degraded` | OBSERVE | Diagnose health degradation |
| `drift-detected` | `cluster.drift.*` | OBSERVE | Diagnose state drift |
| `convergence-stalled` | `operation.stalled` | OBSERVE | Diagnose stalled convergence |
| `workflow-run-failed` | `workflow.run.failed` | OBSERVE | Diagnose workflow failure |
| `workflow-step-failed` | `workflow.step.failed` | OBSERVE | Diagnose step failure |
| `cert-expiry-warning` | `node.cert.expiring` | OBSERVE | Track certificate expiry |
| `doctor-finding` | `cluster.finding.*` | OBSERVE | Process doctor findings |
| `auth-denial` | `alert.auth.denied` (3x) | OBSERVE | Detect auth anomalies |
| `brute-force-detect` | `alert.auth.failed` (5x) | OBSERVE | Detect brute force |
| `error-rate-spike` | `alert.error.spike` | OBSERVE | Diagnose error spikes |

### RPC Surface

| RPC | Purpose | Mutating |
|-----|---------|----------|
| `GetConfig` | Retrieve current watcher configuration | No |
| `SetConfig` | Update rules, topics, batch window | Yes |
| `GetStatus` | Stats (events received/filtered, incidents created) | No |
| `Pause` | Stop event processing | Yes |
| `Resume` | Resume event processing | Yes |
| `GetIncidents` | List detected incidents | No |
| `GetIncident` | Get specific incident | No |
| `ApproveAction` | Route approval to AI Executor | Yes |
| `DenyAction` | Route denial to AI Executor | Yes |
| `GetPendingApprovals` | List incidents awaiting approval | No |
| `Stop` | Graceful shutdown | N/A |

### What It May Change

- In-memory incident tracking state
- etcd-stored rule configuration

### What It Must Never Change Directly

- Cluster state of any kind
- Service configuration
- It only reads events and dispatches incidents — all actions go through AI Executor

---

## AI Router Service

**Port**: 10220 (gRPC), 10221 (gRPC-Web proxy)
**Backend**: In-memory policy cache (scoring loop every ~5 seconds)
**Status**: Implemented (xDS integration partial)

### Role

AI Router computes dynamic routing policies — endpoint weights, circuit breaker settings, drain strategies — based on real-time metrics and anomaly signals. It does not sit in the request path. It shapes the xDS configuration that Envoy uses for traffic routing.

### Modes

| Mode | Behavior |
|------|----------|
| `NEUTRAL` | No routing opinion. Default passthrough. |
| `OBSERVE` | Compute policies and log them, but don't apply to xDS. |
| `ACTIVE` | Compute and apply policies via xDS. |

### Policy Output

For each tracked service, the router computes:

```
RoutingPolicy {
  weights: { "192.168.1.10:10101": 80, "192.168.1.11:10101": 20 }
  drain: [{ endpoint: "192.168.1.12:10101", reason: "high_p99", grace_ms: 30000 }]
  circuit_breaker: { max_connections: 1024, max_pending: 128 }
  outlier_detection: { consecutive_5xx: 3, ejection_time_ms: 30000 }
  retry_policy: { retry_on: "5xx", num_retries: 2 }
  service_class: STATELESS_UNARY
  confidence: 0.8
  reasons: ["endpoint 192.168.1.12 P99 latency 3x above average"]
}
```

### Service Classifications

| Class | Characteristics | Routing Behavior |
|-------|----------------|-----------------|
| `STATELESS_UNARY` | Per-request balancing, fast drain | Quick weight adjustments |
| `STREAM_HEAVY` | Long-lived streams | Slow drain with grace period |
| `CONTROL_PLANE` | Must always be reachable | Minimum weight (never zero) |
| `DEPLOYMENT_SENSITIVE` | Needs warm-up | Gradual weight increase after restart |

### RPC Surface

| RPC | Purpose | Mutating |
|-----|---------|----------|
| `GetRoutingPolicy` | Current policy for one or all services | No |
| `GetStatus` | Mode, policies computed/applied, services tracked | No |
| `SetMode` | Switch between NEUTRAL/OBSERVE/ACTIVE | Yes |
| `GetServiceClassifications` | Map of service → class | No |
| `Stop` | Graceful shutdown | N/A |

### What It May Change

- In-memory routing policy cache
- xDS configuration (when in ACTIVE mode) — through the xDS server API, not directly

### What It Must Never Change Directly

- Service configuration in etcd
- Node state
- Desired state

### Integration Status

- **Policy computation**: Implemented (scoring loop every ~5s)
- **Anomaly tracking**: Implemented (anomalyTracker struct)
- **Drain management**: Implemented (stream-aware graceful removal)
- **xDS push**: Partial — router computes policies, but the xDS server integration to consume `GetRoutingPolicy` on each sync is pending
- **AI Memory**: Implemented — stores historical routing patterns for learning

---

## Deployment

### Service Order

1. **AI Memory** (10200) — Start first; no dependencies
2. **AI Watcher** (10210) — Depends on Event Service
3. **AI Executor** (10230) — Depends on AI Memory; optional Anthropic API key
4. **AI Router** (10220) — No dependencies; optional xDS integration

### Profile

AI services are typically assigned to an `ai` or `monitoring` profile. They are **not** in the `core` profile — the cluster operates without them.

### High Availability

- **AI Memory**: ScyllaDB replication (RF=3) across nodes
- **AI Executor**: Multi-instance with peer collaboration; executors coordinate to avoid duplicate actions
- **AI Watcher**: Single instance per cluster (lightweight; no clustering needed)
- **AI Router**: Single instance per cluster; policies cached and served via RPC
