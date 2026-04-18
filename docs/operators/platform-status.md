# Platform Status

> **Last updated**: April 2026 — reflects the v0.1.x codebase on the 3-node reference cluster.

This page is the honest inventory. It separates what works from what is partial, what is coming, and what is intentionally not supported. Read it before you plan a production deployment.

---

## Legend

| Symbol | Meaning |
|--------|---------|
| ✅ **Implemented** | Works in production on the reference cluster. Tested. |
| 🔶 **Partial** | Works for the common case. Known gaps or rough edges. |
| 🔲 **Planned** | On the roadmap but not yet written or deployed. |
| ❌ **Not supported** | Intentionally out of scope. No plans to add. |

---

## Core Infrastructure

| Component | Status | Notes |
|-----------|--------|-------|
| etcd (single node) | ✅ | Runs on every node. All config and state stored here. |
| etcd (HA, 3 nodes) | ✅ | Requires all 3 nodes to have the `core` profile. |
| ScyllaDB (3-node replication) | ✅ | RF=3. Requires 3 storage-profile nodes. |
| MinIO (distributed erasure coding) | ✅ | 3+ storage nodes. Requires initial pool setup. |
| MinIO (single-node / dev) | 🔶 | Works but has no redundancy. Data lost if node fails. |
| systemd service management | ✅ | All services run as native systemd units. |
| TLS/mTLS between services | ✅ | Cluster CA issues all service certs. Auto-rotated. |
| PKI bootstrap (self-signed cluster CA) | ✅ | Generated on first boot. |
| Let's Encrypt wildcard certificates | ✅ | Via DNS-01 challenge. Supported providers: Cloudflare. |
| Certificate auto-rotation | ✅ | Node agent handles renewal. |
| VIP failover (keepalived) | ✅ | VRRP between gateway nodes. Floats to healthy node. |

---

## Control Plane

| Component | Status | Notes |
|-----------|--------|-------|
| Cluster Controller — leader election | ✅ | etcd-based. One leader per cluster. |
| Cluster Controller — leader forwarding | ✅ | All 22 write RPCs forwarded to leader transparently. |
| Cluster Controller — desired state reconciliation | ✅ | Convergence-filtered; readiness-gated. |
| Node Agent — heartbeat | ✅ | Lean Phase 1+2 only (local markers + etcd). No repo calls. |
| Node Agent — installed state tracking | ✅ | Per-kind, per-node, build_id as sole identity. |
| Node Agent — partial_apply support | ✅ | Idempotent resume for multi-artifact apply. |
| Workflow Service — workflow execution | ✅ | YAML-defined, actor-dispatched, audit trail. |
| Workflow Service — retry with backoff | ✅ | Per-step maxAttempts and backoff configured in YAML. |
| Workflow Service — resume after restart | ✅ | Runs survive controller restart. |
| Workflow Service — centralized execution | ✅ | Controller delegates; workflow engine runs on its own port. |
| Repository — artifact publish pipeline | ✅ | STAGING → VERIFIED → PUBLISHED state machine. |
| Repository — build_id as sole identity | ✅ | UUIDv7. Immutable. Used for convergence, never version string. |
| Repository — GC | ✅ | Reachability-guarded GC. Protects desired-state pinned artifacts. |
| Repository — artifact law validation | ✅ | Invariant checks on publish. Prevents illegal state transitions. |
| DNS — authoritative for globular.internal | ✅ | ScyllaDB-backed, shared across all DNS instances. |
| DNS — split-horizon (internal vs external) | ❌ | Not supported. Use /etc/hosts on each node for hairpin NAT. |
| DNS — upstream forwarding | ✅ | Queries not in cluster zone forwarded upstream. |

---

## Node Lifecycle

| Operation | Status | Notes |
|-----------|--------|-------|
| Node bootstrap (Day-0) | ✅ | `node.bootstrap` workflow with full phase progression. |
| Node join approval | ✅ | Join request + token + profile assignment. |
| Founding quorum enforcement | ✅ | First 3 nodes must have core+control-plane+storage. Enforced at join and profile change. |
| Node repair (targeted, no wipe) | ✅ | `node.repair` workflow. Three modes: from_repository, from_reference, full local reseed. |
| Node full-reseed recovery (wipe + rebuild) | ✅ | `node.recover.full_reseed` workflow. Snapshot → fence → human reprovision → reseed → verify. |
| Node removal | ✅ | Removes from registry, etcd membership, MinIO pool. |
| Downgrade guard | ✅ | Unconditional. Rollback requires explicit Force=true. No automatic rollback ever. |
| Process fingerprinting (entrypoint checksum) | ✅ | Repository RPC + node agent integration. Detects binary drift. |
| CORRUPTED artifact state | ✅ | Set when checksum mismatch detected at runtime. |

---

## Cluster Invariants and Doctor

| Invariant | Status | Notes |
|-----------|--------|-------|
| Founding quorum (etcd/ScyllaDB/MinIO on 3 nodes) | ✅ | Checked at join, profile change, and reconciliation. |
| Storage node count (≥3 for distributed MinIO/Scylla) | ✅ | Enforced by invariant workflow. |
| Desired-state convergence (drift detection) | ✅ | Convergence hash comparison. |
| Version gate (no downgrades) | ✅ | Per-artifact, per-node. |
| TLS cert validity (SAN completeness, expiry) | ✅ | Doctor invariant. Checked per node. |
| MinIO pool health | ✅ | Checked by invariant workflow. Repair triggers pool re-enrollment. |
| etcd membership health | ✅ | Quorum check. Alert if member count drops. |
| ScyllaDB schema migration integrity | ✅ | 3-phase migration with validation. |
| Cluster Doctor — observe mode | ✅ | Continuous invariant checking, no auto-action. |
| Cluster Doctor — enforce mode | ✅ | Auto-heal for Tier 1 actions (restart, clear cache). |
| Cluster Doctor — repair mode | ✅ | Dispatches repair workflows for failed artifacts. |
| Workflow completeness invariant | ✅ | Detects and retries workflows stuck in FAILED without successor. |

---

## Security

| Feature | Status | Notes |
|---------|--------|-------|
| JWT authentication (Ed25519) | ✅ | All RPCs require signed tokens. |
| mTLS between services | ✅ | Cluster CA issues all service certs. |
| RBAC — role binding | ✅ | Per-resource, per-account, per-group. |
| RBAC — cluster roles | ✅ | admin, operator, reader, developer, auditor, service-account. |
| RBAC — proto annotations | ✅ | Every RPC has `(globular.auth.authz)` annotation. |
| Token on-demand generation | ✅ | No tokens stored on disk. Cached in memory with 60s expiry margin. |
| Bootstrap security window | 🔶 | 30-minute window enforced; stale flag file not auto-cleaned. |
| Audit log | ✅ | Every RBAC decision logged with caller context. |
| Secrets in etcd | ❌ | No token or credential storage in etcd values. Intentional. |

---

## AI Layer

| Feature | Status | Notes |
|---------|--------|-------|
| AI Memory (ScyllaDB-backed) | ✅ | Persistent knowledge store. MCP tools available. |
| AI Executor (diagnosis + remediation) | ✅ | Observe → Diagnose → Recommend → Approve → Execute → Verify. |
| AI Watcher (event monitoring) | ✅ | Event stream subscriber. Feeds executor on incidents. |
| AI Router (dynamic routing) | ✅ | Routes queries to appropriate executor instances. |
| MCP server (129+ tools) | ✅ | Full cluster introspection and action tools via MCP protocol. |
| Tier 0 (observe) | ✅ | Read-only diagnosis. Always safe. |
| Tier 1 (auto-remediate) | ✅ | Pre-approved actions. Restart, clear cache. |
| Tier 2 (require approval) | ✅ | Human must approve before execution. |
| AI watcher CLI commands | 🔲 | No CLI wrappers yet. Use MCP tools or direct gRPC. |
| AI router CLI commands | 🔲 | No CLI wrappers yet. Use MCP tools or direct gRPC. |
| AI memory CLI commands | 🔲 | No CLI wrappers yet. Use MCP tools. |

---

## Observability

| Feature | Status | Notes |
|---------|--------|-------|
| Prometheus metrics (all services) | ✅ | Services expose /metrics. Prometheus scrapes them. |
| Alertmanager | ✅ | Day-0 package. Alert routing configured. |
| Log aggregation (ring buffer) | ✅ | `query_log_ring` MCP tool. |
| Workflow history and audit trail | ✅ | Every run stored in etcd with step-level detail. |
| MCP observability tools | ✅ | metrics_query, metrics_targets, metrics_alerts, query_log_ring. |
| Grafana | 🔲 | Not packaged. Planned for Phase 3. |
| `globular metrics *` CLI commands | 🔲 | Use MCP tools instead. |

---

## CLI Completeness

The CLI covers the most common operator workflows. Some areas have gaps where the MCP server or direct gRPC is the current path.

| Area | Status | Gap |
|------|--------|-----|
| Cluster management | ✅ | Full coverage |
| Node management | ✅ | Full coverage including repair and recovery |
| Service desired state | ✅ | Full coverage |
| Workflow inspection | ✅ | list, get, diagnose |
| Package / repository | ✅ | publish, info, list, search, cleanup |
| Deploy pipeline | ✅ | `globular deploy` with `--bump` |
| DNS management | ✅ | zones, records, A/AAAA/TXT |
| Auth (login, tokens) | ✅ | login, logout, root-passwd |
| RBAC (roles, bindings) | ✅ | policy, rbac subcommands |
| Backup | 🔶 | `globular backup create/list` exist; schedule, retention, restore-plan missing |
| Monitoring | 🔲 | Use MCP: metrics_query, metrics_targets, metrics_alerts |
| AI management | 🔶 | Jobs and executor covered; watcher/router/memory missing |
| Compute | 🔲 | Not built. Phase 2+ feature. |
| Schema | ✅ | schema list, describe |

---

## Known Not Working / Not Supported

These are not gaps — they are intentional boundaries:

| Item | Why |
|------|-----|
| Containers / Docker / Kubernetes | Globular is explicitly not a container platform. Native binaries + systemd only. |
| Automatic rollback | Forbidden by design. Rollback requires explicit Force=true and operator intent. |
| Environment variables for configuration | etcd is the only config source. No env vars for service config. |
| Hardcoded addresses or ports in services | All gRPC ports come from etcd at runtime. No compile-time port constants. |
| Tokens stored on disk | Tokens are ephemeral, generated on demand, cached in memory. Never written to disk. |
| Localhost/127.0.0.1 for inter-service calls | All inter-service gRPC resolves from etcd. |
| Split-horizon DNS | Not supported. Use /etc/hosts for hairpin NAT. |
| Cross-node snapshot reuse in recovery | Snapshots are node-specific. Identity is not transferable. |

---

## Roadmap (next milestones)

| Milestone | Target | Key deliverables |
|-----------|--------|-----------------|
| v0.1.0 | Near-term | First tagged release; automated CI; containerized test cluster |
| v0.2.0 | Phase 3 | Grafana packaging; backup CLI completeness; AI watcher/router CLI |
| v1.0.0 | When criteria met | 7 days zero-anomaly; all INV-1–10 tested in CI; external operator validates course |

See [Versioning](../developers/versioning.md) for the full v1.0.0 criteria.

---

## How to verify current state yourself

Everything on this page can be confirmed against the live cluster:

```bash
# What is actually running
sudo systemctl list-units 'globular-*' --state=active

# What artifacts are published in the repository
globular repository list-artifacts

# What is installed on each node
globular cluster get-node-full-status --node-id <node-id>

# What invariants are passing
globular cluster get-doctor-report

# What the convergence model sees
globular cluster get-convergence-detail
```

If something listed as ✅ is not working on your cluster, check [Known Issues](known-issues.md) first — some features require specific configuration or were fixed in a later commit.
