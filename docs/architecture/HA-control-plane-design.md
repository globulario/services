# High-Availability Control-Plane Design

**Status:** Design freeze — approved before implementation begins.

---

## 1. Service Classification by HA Class

### Class A — Quorum/State Stores

| Service | Justification |
|---------|---------------|
| etcd | 3-node Raft quorum. Survives 1 node loss. Already HA. |
| ScyllaDB | Multi-node replication (RF=3). Survives 1 node loss. Already HA. |
| MinIO | Erasure-coded distributed storage. Survives 1 node loss. Already HA. |

### Class B — Leader-Elected Control-Plane Singleton

| Service | Justification |
|---------|---------------|
| cluster-controller | Mutates desired state, drives reconciliation, manages node lifecycle. Must have exactly one active leader. **Already has etcd-based election** via `concurrency.Election` with 15s TTL lease. Followers watch leader addr. |
| cluster-doctor | Produces authoritative findings, triggers remediation workflows. Must have exactly one active authority to prevent duplicate findings and conflicting remediations. **No leader election today.** |

### Class C — Resumable Execution Service

| Service | Justification |
|---------|---------------|
| workflow-service | Executes centralized workflows. Runs are stateful (ScyllaDB-persisted). If executor dies mid-run, the run must be resumable by another instance. **No run ownership or resumption today.** |

### Class D — Stateless Multi-Instance Services

| Service | Justification |
|---------|---------------|
| gateway | HTTP/gRPC proxy. Pure routing. Multiple instances safe. |
| repository | Artifact catalog backed by MinIO. Read-heavy, stateless. |
| authentication | Backed by persistence service + etcd. Stateless RPC handlers. |
| rbac | Backed by etcd. Stateless policy evaluation. |
| resource | Backed by etcd. Stateless CRUD. |
| persistence | Backed by ScyllaDB/MongoDB. Stateless query proxy. |
| discovery | Backed by etcd service registry. Stateless lookup. |
| dns | Backed by etcd zone data. Stateless resolver. |
| event | Pub/sub broker. Stateless message relay. |
| file | Backed by MinIO. Stateless file operations. |
| media | Backed by MinIO + ScyllaDB. Stateless transcoding. |
| search | Backed by bleve/ScyllaDB. Stateless indexing and query. |
| title | Backed by ScyllaDB. Stateless metadata. |
| torrent | Stateless download management. |
| log | Stateless log aggregation. |
| monitoring | Stateless Prometheus proxy. |
| mcp | Stateless MCP tool server. |
| ai-memory | Backed by ScyllaDB. Stateless memory CRUD. |
| ai-executor | Stateless prompt execution (connects to external LLMs). |
| ai-watcher | Event consumer. Stateless reaction engine. |
| ai-router | Stateless LLM routing. |
| backup-manager | Cluster-wide backup coordination. Uses etcd mutex for exclusive ops. Stateless between jobs. |

### Class E — Node-Local Agents

| Service | Justification |
|---------|---------------|
| node-agent | Local to each node. Manages packages, reports status, executes workflows. Not leader-elected. |
| Envoy | Local proxy per node. Configured via xDS. |
| xds | Generates Envoy config from etcd service registry. Local to node. |
| Prometheus | Per-node metrics scraper. Local storage + remote-write capable. |
| node-exporter | Per-node hardware metrics. |
| sidekick | MinIO sidecar. Local to MinIO node. |

---

## 2. HA Contract Per Class

### Class B — Leader-Elected Singletons

**Ownership model:** Exactly one leader at a time. Leadership proven by etcd lease. Followers may serve read traffic but MUST NOT mutate authoritative state.

**Failover semantics:**
- Leader loss detected by etcd lease expiry (TTL: 15 seconds)
- New leader elected within TTL + campaign latency (~20 seconds worst case)
- New leader reloads state from etcd before enabling write paths
- No manual intervention required

**Correctness condition:**
- Two leaders MUST NEVER act concurrently on write paths
- Fencing: all state mutations use etcd transactions with lease-attached keys
- Stale leader that lost its lease MUST stop mutating before the new leader starts

**Bounded interruption:** Reconciliation pauses for up to 20 seconds during failover. Finding production pauses for same duration.

**Allowed degradation:** Delayed reconciliation, delayed findings during failover.

**Must never happen:**
- Two controllers both writing desired state
- Two doctors both producing findings simultaneously
- Split-brain where both believe they are leader

### Class C — Resumable Execution

**Ownership model:** Each workflow run has exactly one executor-owner at a time. Ownership proven by executor lease in ScyllaDB (heartbeat row).

**Failover semantics:**
- Executor heartbeats its owned runs every 10 seconds
- If heartbeat is missed for 30 seconds, run is declared orphaned
- Any healthy executor may claim an orphaned run and resume from the last completed step
- Resume is idempotent: completed steps are skipped, in-progress step is re-executed

**Correctness condition:**
- A run MUST NOT be owned by two executors simultaneously
- Claim uses ScyllaDB lightweight transaction (IF executor_id = NULL OR stale)
- Actor callbacks are idempotent by design (same input → same output)

**Bounded interruption:** Orphaned runs resume within 30s heartbeat timeout + 10s scan interval = ~40 seconds.

**Allowed degradation:** In-progress step re-executed on resume (idempotent). Brief gap in step recording.

**Must never happen:**
- Two executors running the same step of the same run simultaneously
- A run permanently orphaned with no recovery path
- Resume corrupting accumulated step outputs

### Class D — Stateless Multi-Instance

**Ownership model:** No ownership. Any instance can serve any request. Load balanced by Envoy.

**Failover semantics:**
- Instance death detected by Envoy active health check (gRPC health probe, 5s interval)
- Dead endpoint removed from Envoy routing within 15 seconds (3 failed probes)
- Remaining instances absorb traffic automatically
- No state migration needed

**Correctness condition:** Reduced capacity is acceptable; loss of correctness is not. All state is in Class A stores.

**Bounded interruption:** Individual requests may fail during the health check window (~5-15 seconds). Clients retry.

**Must never happen:**
- Traffic routed to a dead endpoint for more than 15 seconds
- Loss of all instances of a service with no detection

### Class E — Node-Local

**Ownership model:** One instance per node. Not replicated across nodes.

**Failover semantics:** If the node dies, its agents die. Higher layers (controller, doctor) detect node absence via heartbeat timeout (2 minutes) and reason about it.

**Must never happen:** A Class B or C service assuming a node-agent is reachable when the node is down.

---

## 3. Required Mechanisms Per Class

### Class B — Controller Leader Election (Hardening)

**Current state:** Controller already uses `concurrency.Election` with 15s TTL. This is solid. What's missing:

| Mechanism | Status | Action needed |
|-----------|--------|---------------|
| etcd lease-based election | Exists | None |
| Fencing token / epoch | Missing | Add epoch counter to state mutations |
| Lease renewal | Exists (etcd session) | None |
| Failover timeout | ~20s | Acceptable |
| Follower behavior | Idle | Allow read-only RPCs on followers |
| State reload on takeover | Exists (`reloadStateFromEtcd`) | None |
| Graceful resignation | Exists (`election.Resign`) | None |

**Fencing implementation:**
```
On becoming leader:
  1. Increment /globular/clustercontroller/epoch (atomic CAS)
  2. Store epoch in srv.leaderEpoch
  3. All state-mutating operations include epoch in etcd transactions
  4. If transaction fails (epoch mismatch), stop and re-campaign
```

**Follower read RPCs:** `GetClusterInfo`, `GetNodeList`, `ListDesiredState` can be served by followers reading from etcd. Only write RPCs (`SetNodeProfiles`, `ApproveJoinRequest`, release mutations) require leader.

### Class B — Doctor Leader Election (New)

**Current state:** No leader election. Single instance assumed.

**Required implementation:**
- Same `concurrency.Election` pattern as controller
- Election prefix: `/globular/cluster_doctor/leader`
- TTL: 15 seconds (same as controller)
- On becoming leader: start finding production loop
- On losing leadership: stop finding production, clear cached findings
- Followers: serve `ExplainFinding` from cached findings (read-only), but do NOT produce new findings

**Key invariant:** Only the leader calls `GetClusterReport` with `FRESHNESS_FRESH`. Followers may serve cached/stale findings but must declare `source: "follower"` in the freshness header.

### Class C — Workflow Executor Ownership (New)

**Current state:** No run ownership. Single executor assumed.

**Required implementation:**

**Executor lease table (ScyllaDB):**
```sql
CREATE TABLE IF NOT EXISTS globular_workflows.executor_leases (
    run_id       text PRIMARY KEY,
    executor_id  text,
    heartbeat_at bigint,
    started_at   bigint
);
```

**Lifecycle:**
1. `ExecuteWorkflow` entry: claim run via LWT:
   ```sql
   INSERT INTO executor_leases (run_id, executor_id, heartbeat_at, started_at)
   VALUES (?, ?, ?, ?) IF NOT EXISTS
   ```
2. During execution: heartbeat every 10s:
   ```sql
   UPDATE executor_leases SET heartbeat_at = ? WHERE run_id = ? IF executor_id = ?
   ```
3. On completion: delete lease row
4. Orphan scanner (background goroutine, every 15s): find rows where `now - heartbeat_at > 30s`
5. Claim orphan via LWT:
   ```sql
   UPDATE executor_leases SET executor_id = ?, heartbeat_at = ?
   WHERE run_id = ? IF heartbeat_at < ?
   ```
6. Resume: load run state from `workflow_runs` + `workflow_steps`, skip completed steps, re-execute current step

**Resume semantics:**
- Completed steps (SUCCEEDED/FAILED/SKIPPED): skip
- In-progress step (RUNNING): re-execute from the beginning (actor callbacks are idempotent)
- Pending steps: execute normally
- The engine's existing DAG walker handles this naturally — just set completed steps' status before starting

#### Step Resumability Classes

Not all workflow actions are equally safe to re-execute. Each action falls
into one of three resumability classes:

| Class | Meaning | Examples | Resume rule |
|-------|---------|----------|-------------|
| **R-safe** | Fully idempotent. Re-execution produces identical result. | `doctor.resolve_finding`, `doctor.assess_risk`, `controller.reconcile.scan_drift`, `controller.release.select_package_targets`, `node.verify_package_installed`, `node.verify_services_active` | Re-execute unconditionally |
| **R-guarded** | Side-effecting but guarded by external state. Re-execution is safe because the target service rejects duplicates or the operation is naturally idempotent. | `controller.bootstrap.set_phase` (phase already set → no-op), `node.install_package` (already installed → no-op), `node.restart_package_service` (restart is idempotent), `doctor.execute_remediation` (audit check prevents double-execute), `controller.release.mark_node_succeeded` (etcd CAS) | Re-execute; handler detects duplicate and returns success |
| **R-check** | Side-effecting with no built-in guard. Must check completion before re-executing. | `node.uninstall_package` (repeated uninstall is safe but wastes time), `installer.bootstrap_dns` (zone already exists → error without guard) | Check step output in ScyllaDB first. If step recorded SUCCEEDED, skip. If RUNNING or no record, re-execute. |

**Rule:** All existing actor handlers in `engine/actors*.go` are R-safe or
R-guarded by design. The structured-action model (typed, audited, gated)
ensures side effects are naturally idempotent. New actions added in the
future MUST declare their resumability class in their registration comment.

**Actions that are NOT resumable:**
None in the current codebase. If a non-idempotent action is ever introduced
(e.g., sending an external notification), it must be wrapped in a
completion-check guard or moved to R-check class.

#### Terminal Hook Replay Rules

When a workflow is resumed after executor crash, the `onFailure` and
`onSuccess` hooks need special handling:

| Scenario | Hook rule |
|----------|-----------|
| Run crashed before any step completed | Resume from beginning. No hooks fired yet. Run hooks at the natural end. |
| Run crashed mid-step, step had no `onFailure` | Re-execute step. Normal hook behavior on completion. |
| Run crashed after step `onFailure` fired | Step `onFailure` is R-safe (e.g., `mark_item_failed` is idempotent). Re-execute the step; if it fails again, the hook fires again safely. |
| Run crashed after all steps completed but before run `onSuccess` | Check run status in ScyllaDB. If all steps SUCCEEDED but run not marked SUCCEEDED, fire `onSuccess` hook and mark run terminal. |
| Run crashed after run `onFailure` fired | Check run status. If already marked FAILED, do not re-fire. If not marked, re-fire `onFailure` (idempotent) and mark terminal. |

**Key invariant:** Hooks are idempotent. `doctor.mark_failed` logs a warning
(safe to repeat). `controller.reconcile.emit_completed` publishes an event
(duplicate events are harmless). `controller.release.mark_failed` patches
a release phase (etcd CAS prevents double-transition).

**Implementation:** The resume logic checks `workflow_runs.status`:
- If SUCCEEDED or FAILED → run is already terminal, skip entirely
- If EXECUTING → resume from last completed step
- If PENDING → start from beginning

### Follower Read Semantics

#### Controller Followers

| RPC | Follower behavior | Justification |
|-----|-------------------|---------------|
| `GetClusterInfo` | Serve from etcd (read-only) | Cluster metadata doesn't change during failover |
| `ListNodes` | Serve from etcd state snapshot | Node list is etcd-authoritative, not leader-only |
| `GetNodeList` | Serve from etcd state snapshot | Same as ListNodes |
| `ResolveNode` | Serve from ScyllaDB projection + etcd fallback | Already designed for non-leader access |
| `GetClusterHealth` | Serve from etcd state (best-effort) | Health is derived from heartbeat timestamps in etcd |
| `SetNodeProfiles` | **Reject** → forward to leader | Write operation, leader-only |
| `ApproveJoinRequest` | **Reject** → forward to leader | Write operation, leader-only |
| `SetDesiredVersion` | **Reject** → forward to leader | Write operation, leader-only |
| All release mutations | **Reject** → forward to leader | Write operation, leader-only |

**Rejection behavior:** Return `codes.Unavailable` with metadata
`X-Leader-Addr: <addr>` so the client can retry against the leader directly.
This is NOT automatic proxying — the client decides whether to follow the
redirect.

**State freshness on followers:** Followers read from etcd, not from the
leader's in-memory cache. This means follower reads are at most one etcd
sync behind (~milliseconds), which is acceptable for read RPCs.

#### Doctor Followers

| RPC | Follower behavior | Justification |
|-----|-------------------|---------------|
| `GetClusterReport` | Serve cached findings from last leader snapshot. Set `freshness_mode: CACHED`, `source: "follower"`. **Never** force-fresh on followers. | Prevents duplicate upstream scans. Caller sees it's not authoritative. |
| `GetNodeReport` | Same: cached, `source: "follower"` | Same |
| `GetDriftReport` | Same: cached, `source: "follower"` | Same |
| `ExplainFinding` | Serve from local findings cache | Already reads from cache, same on leader or follower |
| `ExecuteRemediation` | **Reject** → return error | Side-effecting, leader-only |
| `StartRemediationWorkflow` | **Reject** → return error | Triggers workflow, leader-only |

**Finding cache on followers:** Followers subscribe to the leader's finding
events (via the event service) and maintain a local cache. This provides
read availability during failover with explicit staleness disclosure. If
the event subscription lapses, the follower's cache ages and the freshness
header reflects this honestly.

### Class D — Health-Aware Multi-Instance Routing

**Current state:** Services register in etcd (`/globular/services/{id}/instances/{mac}`). xDS reads this but doesn't do active health checking.

**Required implementation:**

**gRPC health service:** Every Class D service must implement the standard gRPC Health Checking Protocol (`grpc.health.v1.Health`). This is a single method: `Check(HealthCheckRequest) → HealthCheckResponse`.

**xDS integration:** The xDS server must:
1. Read all instances for a service from etcd
2. Generate Envoy EDS (Endpoint Discovery Service) with all healthy instances
3. Configure Envoy active health checking:
   - Protocol: gRPC health check
   - Interval: 5 seconds
   - Unhealthy threshold: 3 consecutive failures (15s to remove)
   - Healthy threshold: 1 success (immediate re-add)

**Endpoint withdrawal:** When a node-agent stops reporting (heartbeat timeout), the controller must:
1. Mark the node's instances as unhealthy in etcd
2. xDS picks up the change and removes endpoints from Envoy

### Ingress HA (Class D special case)

**Current state:** Single Envoy gateway per node. If the gateway node dies, ingress is lost.

**Required implementation:**
- keepalived with VRRP for a shared Virtual IP (VIP) across gateway nodes
- VIP floats to a healthy gateway node on failure
- Detection: VRRP advertisement interval 1s, dead interval 3s
- This is strictly north-south (ingress) HA — not the answer to service-level HA

---

## 4. Explicit HA Invariants

These are testable via failover drills.

### I-1: Controller Leadership
> Loss of the controller leader node must not stop reconciliation for more than 20 seconds. A new leader must be elected and resume reconciliation without manual intervention.

### I-2: Controller Fencing
> Two controller instances must never both believe they are leader and mutate desired state simultaneously. Fencing via epoch prevents stale-leader writes.

### I-3: Doctor Authority
> Only one doctor instance may produce findings at a time. Followers serve cached data with explicit `source: "follower"` disclosure.

### I-4: Workflow Run Ownership
> A workflow run must be owned by exactly one executor at a time. Executor death must not orphan a run for more than 40 seconds.

### I-5: Workflow Resumption
> A resumed workflow must skip completed steps and re-execute only the in-progress or pending steps. No step may execute twice concurrently.

### I-6: Stateless Service Routing
> Loss of one stateless service instance must not black-hole requests for more than 15 seconds. Envoy must detect the failure and route to remaining instances.

### I-7: Endpoint Withdrawal
> A dead node's service endpoints must be removed from routing within 2 minutes (heartbeat timeout + xDS propagation).

### I-8: Node-Agent Absence
> If a node-agent is unreachable, the controller and doctor must correctly report the node as unhealthy. No workflow may assume a dead node-agent is reachable.

### I-9: Ingress Continuity
> Loss of the ingress gateway node must not make the cluster unreachable from outside for more than 5 seconds (VRRP failover).

### I-10: State Store Independence
> Application-layer HA must not depend on custom state stores. All leadership, ownership, and coordination uses etcd (existing) or ScyllaDB (existing).

---

## 5. Phase-by-Phase Implementation Plan

### HA-1 — Design Freeze

**Deliverable:** This document.

**Acceptance:** All service classifications, contracts, invariants, and mechanisms reviewed and approved.

---

### HA-2 — Controller Leader Election Hardening

**Goal:** Add fencing epoch and allow follower read RPCs.

**Work items:**
1. Add `/globular/clustercontroller/epoch` key, incremented on each leadership takeover
2. Add `leaderEpoch` field to server struct
3. `requireLeader()` checks both `srv.leader` flag AND epoch freshness
4. Wrap state-mutating etcd transactions with epoch guard
5. Allow read RPCs (`GetClusterInfo`, `GetNodeList`, `ListNodes`) on followers
6. Add `X-Leader-Node` response header so callers know who the leader is

**Tests:**
- Unit: epoch increment on leadership change
- Unit: stale-epoch write rejected
- Integration: kill leader → new leader elected → reconciliation resumes
- Invariants: I-1, I-2

---

### HA-3 — Doctor Leader Election

**Goal:** Prevent duplicate finding production across nodes.

**Work items:**
1. Add etcd-based leader election to cluster-doctor (same pattern as controller)
2. Election prefix: `/globular/cluster_doctor/leader`
3. Only leader runs `GetClusterReport` with `FRESHNESS_FRESH`
4. Followers serve `ExplainFinding` from cached findings
5. Freshness header includes `source: "leader"` or `source: "follower"`
6. Graceful resignation on shutdown

**Tests:**
- Unit: follower does not produce findings
- Integration: kill doctor leader → new leader starts producing within 20s
- Invariant: I-3

---

### HA-4 — Workflow Run Ownership and Resumption

**Goal:** Prevent orphaned runs and enable resume after executor crash.

**Work items:**
1. Add `executor_leases` ScyllaDB table
2. `ExecuteWorkflow` claims run via LWT before execution
3. Background heartbeat goroutine (10s interval)
4. Background orphan scanner (15s interval)
5. `ResumeWorkflow` RPC: loads run state, skips completed steps, re-executes
6. Orphan scanner calls `ResumeWorkflow` for stale-leased runs
7. Delete lease on run completion (success or failure)

**Tests:**
- Unit: claim via LWT succeeds for unclaimed run
- Unit: claim fails if another executor owns the run
- Unit: orphan detected after 30s heartbeat gap
- Integration: kill executor mid-run → run resumes on another instance
- Invariants: I-4, I-5

---

### HA-5 — Stateless Service Replication and Health-Aware Routing

**Goal:** Multiple instances of Class D services with Envoy health-driven routing.

Split into subgroups by operational risk — deploy and validate each before
moving to the next.

#### HA-5a — xDS infrastructure + health checking foundation

**Work items:**
1. Add `grpc.health.v1.Health` to the `globular_service` shared primitives (one implementation, all services inherit)
2. Update xDS server to generate EDS with all registered instances per service
3. Configure Envoy active health checking (gRPC, 5s interval, 3-failure threshold)
4. Controller marks instances unhealthy on heartbeat timeout
5. xDS removes unhealthy endpoints from Envoy clusters

**Validate with:** gateway only (lowest risk — stateless proxy, easy to test routing)

#### HA-5b — Core infrastructure services

**Services:** dns, event, discovery, resource, rbac, authentication

**Why first:** These are foundational — other services depend on them. If
health-aware routing works here, the rest follows.

**Risk:** Moderate — DNS failover must not cause resolution gaps. Event
failover must not lose published events (fire-and-forget is acceptable).

#### HA-5c — Data/storage services

**Services:** repository, persistence, file, search, monitoring, log

**Why second:** These are data-path services. Multi-instance means load
distribution, not just failover. Repository write path needs care (artifact
uploads must not corrupt on concurrent writes to different instances).

**Risk:** Low — all state is in Class A stores (MinIO, ScyllaDB).

#### HA-5d — Application services

**Services:** media, title, torrent, backup-manager

**Why third:** Least critical for cluster operations. Backup-manager has
its own etcd mutex for cluster-wide ops — multi-instance is safe because
the mutex prevents concurrent backups.

#### HA-5e — AI services

**Services:** mcp, ai-memory, ai-executor, ai-watcher, ai-router

**Why last:** These have the most complex interaction patterns (long-running
LLM calls, event subscriptions). Multi-instance ai-watcher needs care to
prevent duplicate event processing.

**Per-subgroup tests:**
- Stop one instance → Envoy routes to remaining within 15s
- Start new instance → Envoy adds within 5s
- Invariants: I-6, I-7

---

### HA-6 — Ingress HA (VIP)

**Goal:** Stable external entry point that survives gateway node loss.

**Work items:**
1. Deploy keepalived on all gateway-profile nodes
2. Configure VRRP with shared VIP
3. Health check: gateway process liveness + Envoy ready
4. Failover: 3 second detection, VIP moves to healthy node
5. DNS A record points to VIP (not individual node IPs)

**Tests:**
- Integration: kill gateway node → VIP moves → external requests succeed within 5s
- Invariant: I-9

---

## 6. Interaction With Existing Architecture

| Established rule | HA impact |
|-----------------|-----------|
| Centralized workflow execution | Preserved. ExecuteWorkflow still goes through WorkflowService. Run ownership adds durability, not a new execution path. |
| Workflow vs plan separation | Preserved. No change to workflow semantics. |
| Typed structured actions | Preserved. Actor callbacks are idempotent — safe for resume. |
| Canonical endpoint resolver | Preserved. All new dials use `ResolveDialTarget`. |
| Freshness contracts | Enhanced. Follower doctor declares `source: "follower"` — callers know it's not the authority. |
| Schema reference | Preserved. New etcd keys (epoch, executor leases) get pragmas. |
| Single definition source | Preserved. Workflow definitions stay in MinIO. |

HA work strengthens the architecture. No old ambiguities re-opened.

---

## 7. Execution Order Rationale

The phases are ordered by **blast radius and dependency**:

1. **HA-2 (controller fencing)** first — the controller is the most critical singleton. Adding fencing prevents the worst failure mode (split-brain writes) before anything else.

2. **HA-3 (doctor election)** next — simpler than workflow resumption, uses the same proven pattern as the controller. Prevents duplicate finding production.

3. **HA-4 (workflow resumption)** third — the most complex mechanism (LWT claims, heartbeats, orphan scanning, resume logic). Depends on HA-2 being stable since workflows dispatch to the controller.

4. **HA-5 (stateless replication)** fourth — the widest change (touches every Class D service) but the simplest per-service (just add health endpoint). Depends on xDS changes.

5. **HA-6 (ingress VIP)** last — infrastructure-level, lowest code risk, highest operational change. Independent of all other phases.

---

## 8. Summary

| What | Before | After |
|------|--------|-------|
| Controller leader loss | Reconciliation stops until restart | New leader in ~20s, fenced writes |
| Doctor authority | Single instance, no failover | Leader-elected, followers serve cached |
| Workflow executor crash | Run orphaned permanently | Resumed within ~40s by another executor |
| Service instance death | Traffic black-holed until manual restart | Envoy removes endpoint in ~15s |
| Ingress node loss | Cluster unreachable externally | VIP moves in ~3s |

**The target:**
> Loss of any single node does not break control-plane correctness.
