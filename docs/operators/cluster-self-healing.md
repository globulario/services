# Cluster Self-Healing Reference

This page is written for operators who need to understand what Globular is doing **right now** to keep the cluster healthy — and what it cannot do, so you know when you need to act.

Every mechanism described here is running continuously. When you see an alert, an event, or a log line, this page tells you what triggered it, what the system is already doing about it, and what it cannot handle on its own.

---

## The Two Layers of Self-Healing

Globular has two independent self-healing layers that serve different roles:

| Layer | What it is | Requires | Runs when |
|-------|-----------|----------|-----------|
| **Reconcile loop** | Fast, in-process, runs under lock | etcd only | Every reconcile cycle (~30s) |
| **Invariant workflow** | Auditable, structured, workflow-tracked | etcd + Workflow Service | Triggered by reconcile loop |

The reconcile loop is the **last line of defense**. If the Workflow Service is down, MinIO is unavailable, or the cluster is severely degraded, the reconcile loop still runs and still enforces invariants. It logs what it finds but does not produce workflow audit records.

The invariant workflow runs on top of the reconcile loop when the cluster is healthy enough to support it. It produces structured reports, cluster events, and audit trails.

---

## What the System Monitors and Fixes Automatically

### 1. Workflow Definition Completeness

**What it checks**: All core workflow definitions (`cluster.reconcile`, `node.join`, `node.repair`, `release.apply.*`, etc.) must exist in etcd. Without them the cluster cannot reconcile, deploy, join new nodes, or heal itself.

**How it detects**: Reconcile loop reads each workflow key from etcd every 5 minutes. Missing keys are recorded immediately.

**What it does automatically**:
- Re-seeds missing definitions from `/var/lib/globular/workflows/` on the leader's local disk.
- Emits a `controller.workflows_repaired` cluster event listing what was restored.

**What it cannot do**: If the YAML files are missing from disk (e.g. the controller binary was replaced without the workflow package), it cannot restore them. You must reinstall the controller package.

**Operator signal**:
```
invariant: N core workflow definitions missing from etcd, repairing: [...]
```

---

### 2. Infrastructure Quorum

**What it checks**: Three hard requirements are always enforced.
- etcd runs on **all** nodes (every member must be reachable and participating in Raft).
- ScyllaDB has storage profile on **≥ 3** nodes.
- MinIO has storage profile on **≥ 3** nodes.

Dropping below these thresholds breaks write quorum for the respective system. The cluster continues running but cannot accept new writes, which blocks deployments, backups, and state changes.

**How it detects**: Reconcile loop counts nodes per profile on every cycle. The invariant workflow calls `validate_infra_quorum` with exact thresholds.

**What it does automatically**:
- Identifies non-storage nodes that could be promoted (have sufficient disk and RAM).
- Adds the `storage` and `control-plane` profiles to candidate nodes.
- This triggers the bootstrap state machine on the promoted node: it installs ScyllaDB and MinIO and joins the respective pools.

**What it cannot do**: It cannot create new machines. If you have fewer than 3 physical/virtual nodes total, it cannot restore quorum — you must add hardware.

**Operator signal**:
```
invariant: quorum_report violation: scylladb_quorum — need 3 storage nodes, have 2
```

---

### 3. Founding Node Profiles

**What it checks**: The first 3 nodes that joined the cluster must permanently carry `core`, `control-plane`, and `storage` profiles. These profiles cannot be removed even as the cluster grows. Losing them drops etcd, ScyllaDB, or MinIO below quorum.

**How it detects**: Invariant workflow evaluates founding node set on every enforcement cycle.

**What it does automatically**: Reports violations. Does **not** auto-remove profiles from nodes (profile removal requires explicit operator intent). Blocks profile-removal requests that would violate this invariant at the API level — `SetNodeProfiles` returns an error.

**What it cannot do**: Restore profiles that were removed by bypassing the API (direct etcd writes). If you manually remove a founding profile via `etcdctl`, the next reconcile will detect the violation and report it, but will not add the profile back automatically.

**Operator signal**:
```
invariant: profile_report violation: node globule-dell missing profiles [storage]
```

**Recovery**:
```bash
globular cluster nodes profiles set globule-dell core control-plane storage
```

---

### 4. MinIO Distributed Storage Health

**What it checks**: MinIO is the artifact repository and backup store. The invariant checks:
- Root credentials exist in controller state.
- Pool has ≥ 3 nodes (required for distributed erasure coding).
- All storage-profile nodes appear in `MINIO_VOLUMES`.
- All pool nodes have `minio_join_phase == verified`.
- `globular-minio.service` is active on all pool nodes.
- Erasure set has ≥ 4 endpoints (pool_nodes × drives_per_node).

**How it detects**: Invariant workflow reads pool state, join phases, and unit status from controller state (populated by heartbeats).

**What it does automatically**:
- **Node not in pool but has storage profile**: resets `minio_join_phase` to `""` — the pool manager picks it up on the next reconcile and adds it.
- **Node in pool with `join_phase == failed`**: resets join phase for retry.
- **Node in pool but service not running**: clears the rendered config hash (forcing re-render on next reconcile) and restarts `globular-minio.service` via node-agent.
- **Leader failover**: re-publishes MinIO credentials to etcd immediately on leadership acquisition, so worker nodes always have current credentials.

**What it cannot do**: Recover lost erasure shards. If data was stored with RF=2 and 2 nodes are permanently lost, that data is unrecoverable without a backup.

**Operator signal**:
```
controller.invariant.disk_pressure_critical / minio_service_not_running / minio_pool_insufficient
```

---

### 5. PKI and Certificate Health

**What it checks**: Every node's TLS certificate must be:
- Present and readable.
- Not expired (`days_until_expiry > 0`).
- Not within 30 days of expiry (WARN) or 7 days (ERROR).
- Containing SANs for all advertised IPs (including the VIP if applicable).
- Validating against the cluster CA.

A missing or expired certificate silently breaks all inter-service gRPC on that node because mTLS handshakes fail.

**How it detects**: Invariant workflow calls `GetCertificateStatus` on each node-agent. This is a live RPC — it reads the actual cert from disk, not from etcd.

**What it does automatically**:
- Restarts `globular-node-agent.service` on affected nodes. The node-agent's `ExecStartPre` re-issues a certificate from the cluster CA on every start. This fixes:
  - Expired certificates.
  - Missing SANs (re-issued with current node IPs).
  - Invalid chains (re-signed by current CA).
  - Missing certificates (generated from scratch).

**What it cannot do**: Fix a missing CA certificate. If `/var/lib/globular/pki/ca.crt` is gone from a node, the re-issuance itself fails. This requires manual intervention.

**Operator signal**:
```
invariant: pki_cert_expiring_soon — node globule-nuc cert expires in 6 days
invariant: pki_san_missing — node globule-ryzen cert missing IP SANs [10.0.0.100]
```

**Manual recovery if CA is missing**:
```bash
# Copy CA from another healthy node
scp root@globule-ryzen:/var/lib/globular/pki/ca.crt root@globule-nuc:/var/lib/globular/pki/ca.crt
systemctl restart globular-node-agent
```

---

### 6. Disk Space Health

**What it checks**: Free disk space on every node, using `DiskFreeBytes` reported in each heartbeat. Two thresholds:
- **WARN**: free < 20% — the node is filling up.
- **CRITICAL**: free < 5% — write failures are imminent.

Low disk kills etcd (WAL writes fail), ScyllaDB (compaction stops), and the node-agent (cannot write rendered configs or package downloads).

**How it detects**:
- **Reconcile loop** (fast path): checks every reconcile cycle, logs WARN/CRITICAL immediately. No MinIO or Workflow Service needed.
- **Invariant workflow**: `validate_disk_health` step checks same data, emits structured cluster events per affected node.

**What it does automatically**:
- Emits `controller.invariant.disk_pressure_warn` or `controller.invariant.disk_pressure_critical` cluster events.
- Logs a CRITICAL line per affected node every reconcile cycle.
- Skips the node for new package deployments when CRITICAL (prevents making a full disk worse).

**What it cannot do**: Add disk capacity. Journal vacuum (`journalctl --vacuum-size`) is planned as a future node-agent action but is not yet automated.

**Operator action when CRITICAL**:
```bash
# On the affected node — identify what is consuming space
df -h /var/lib/globular /var/log /

# 1. Vacuum systemd journals (usually the largest fast win)
journalctl --vacuum-size=2G

# 2. Remove downloaded package archives for old versions
ls /var/lib/globular/packages/
# Keep only the version currently desired; remove the rest

# 3. If ScyllaDB is running, check its data dir
du -sh /var/lib/scylla/

# 4. After cleanup, verify node is healthy
globular cluster health
```

---

### 7. Node Reachability and Network Partition Detection

**What it checks**: Every node must send a heartbeat to the controller within `heartbeatStaleThreshold` (5 minutes). Persistent absence indicates a network partition, a crashed node, or a network misconfiguration.

Two escalation levels:
- **WARN** (stale > 5 min): node is unresponsive. Reconcile loop already blocks new deployments to this node.
- **CRITICAL** (stale > 15 min): likely network partition. Node is soft-fenced.

**How it detects**: Reconcile loop tracks `LastSeen` for every node on every cycle. Invariant workflow classifies staleness and flags `quorum_at_risk` when ≥ 2 nodes are critical.

**What it does automatically**:

*For a single unreachable node*:
- Blocks new package deployments to that node (reconcile `blockedReasons` + partition fence).
- Sets `Metadata["partition_fenced_since"]` on the node — release pipeline skips it.
- Emits `controller.invariant.node_partitioned` CRITICAL cluster event.
- **Auto-clears**: when the heartbeat resumes, removes the fence and emits `controller.invariant.node_partition_healed`.

*For ≥ 2 unreachable nodes (quorum at risk)*:
- Emits `controller.invariant.quorum_loss_imminent` CRITICAL cluster event.
- Writes `/globular/cluster/alerts/quorum_loss` to etcd — visible to Alertmanager and external monitoring even if the EventService is degraded.
- Includes the list of unreachable nodes and points to recovery documentation.

**What it cannot do**: Restart a physically unreachable node. It cannot cross a network partition to fix what it cannot reach.

**Operator signals**:
```
controller.invariant.node_partitioned        — single node gone > 15 min
controller.invariant.quorum_loss_imminent    — ≥ 2 nodes gone, cluster at risk
/globular/cluster/alerts/quorum_loss         — etcd key written as durable alert
```

---

## When the Cluster Cannot Recover Alone

Some failure modes are outside what any software can fix automatically. Knowing them in advance prevents you from waiting for an automation that will never come.

### Dual-node failure

If 2 of 3 nodes go down simultaneously or in rapid succession:

- **etcd loses Raft quorum** (1 of 3 members) — no new writes can be committed. The controller cannot advance workflow state, cannot change desired state, cannot schedule anything.
- **The invariant workflow cannot run** — it needs etcd writes to commit step results.
- **ScyllaDB and MinIO lose write quorum** — DNS records, backup data, and artifact metadata cannot be written.

The surviving node can still serve existing gRPC traffic for services that don't need etcd writes. It will emit the quorum loss alert before losing write access. After that, it is silent.

**Recovery path**:
```
1. Restore the two lost nodes (hardware fix, VM restart, network repair).
   → If nodes come back with intact data: etcd re-forms quorum automatically.
   → etcd member re-joins gossip within ~30 seconds of network restore.

2. If nodes are permanently lost:
   → Restore from the most recent backup.
   → See docs/operators/backup-and-restore.md → Disaster Recovery section.
   → globular backup restore <id> --full

3. After quorum is restored, clear the alert key:
   etcdctl del /globular/cluster/alerts/quorum_loss
```

### Physical disk full (no free space)

If a disk reaches 100% full:

- etcd stops committing (WAL write fails → leader steps down).
- ScyllaDB compaction stalls, then writes fail.
- The node-agent cannot write rendered configs — reconcile fails silently on that node.
- The invariant enforcement itself cannot write its checkpoint to etcd.

The system will have emitted WARN and CRITICAL log lines and events as the disk filled. Once at 0%, it cannot emit anything more.

**Recovery path**:
```bash
# Boot into recovery mode or use SSH (if accessible)
# Find the largest consumers
du -sh /var/lib/globular/* /var/lib/scylla /var/log /var/lib/etcd

# Emergency free-up (in order of safety):
journalctl --vacuum-size=1G          # safest — journal is always reclaimable
rm /var/lib/globular/packages/<old>  # remove superseded package archives
# Do NOT remove /var/lib/etcd or /var/lib/scylla — data loss

# After freeing space, restart affected services
systemctl restart etcd scylla-server globular-minio globular-node-agent
```

### Missing cluster CA

If `/var/lib/globular/pki/ca.crt` or `/var/lib/globular/pki/ca.key` is lost:

- Certificate re-issuance fails on all nodes.
- New nodes cannot join (cannot issue a valid service cert).
- Existing certs continue working until they expire.

There is no automatic recovery — the CA private key is the root of trust and is not replicated anywhere by design.

**Recovery**: Restore from backup. The CA key is included in the etcd snapshot backup.

---

## Reading the Signals

### Cluster events

Every automatic action emits a cluster event. Query recent events:

```bash
# Via MCP
mcp__globular__cluster_get_health

# Via CLI
globular cluster health --verbose

# Direct gRPC — list recent invariant events
grpcurl -H "authorization: bearer $TOKEN" \
  globular.internal:443 cluster_controller.ClusterControllerService/GetClusterHealth
```

Key event names and what they mean:

| Event name | Meaning |
|-----------|---------|
| `controller.workflows_repaired` | Workflow definitions were missing from etcd and re-seeded from disk |
| `controller.invariant_enforcement_report` | Full invariant cycle completed — check payload for violations |
| `controller.invariant.node_partitioned` | A node has been unreachable for >15 min; deployments paused to it |
| `controller.invariant.node_partition_healed` | Partitioned node sent a heartbeat; fence cleared automatically |
| `controller.invariant.quorum_loss_imminent` | ≥2 nodes unreachable; trigger emergency backup immediately |
| `controller.invariant.disk_pressure_warn` | Node disk < 20% free; plan cleanup |
| `controller.invariant.disk_pressure_critical` | Node disk < 5% free; act now |
| `controller.invariant_enforcement_failed` | Invariant workflow itself failed; check workflow service health |
| `controller.invariant_enforcement_completed` | All invariants passed or were remediated |

### etcd alert keys

These keys are written as durable signals when the cluster cannot reliably emit events:

| etcd key | Written when |
|---------|-------------|
| `/globular/cluster/alerts/quorum_loss` | ≥2 nodes critically unreachable |
| `/globular/dns/v1/zones` | DNS zone list mirror (for restart recovery) |

Check them directly:
```bash
etcdctl get /globular/cluster/alerts/quorum_loss
etcdctl get /globular/dns/v1/zones
```

Clear after recovery:
```bash
etcdctl del /globular/cluster/alerts/quorum_loss
```

### Logs

Every invariant action logs a structured line. On the leader node:

```bash
journalctl -u globular-cluster-controller --no-pager -f | grep -E "invariant|disk|partition|quorum"
```

Key log patterns:

| Pattern | Meaning |
|--------|---------|
| `invariant[disk] CRITICAL` | Disk < 5% free on a node — act immediately |
| `invariant[disk] WARN` | Disk < 20% free — plan cleanup soon |
| `invariant-reachability: fencing` | Node soft-fenced after 15 min absence |
| `invariant-reachability: unfencing` | Fenced node came back; auto-cleared |
| `invariant-reachability: CRITICAL — N nodes unreachable` | Quorum at risk — trigger backup |
| `invariant-repair: minio restarted on` | MinIO auto-restarted by invariant enforcement |
| `invariant-repair: node-agent restarted on` | Node-agent restarted for cert re-issuance |
| `invariant: N core workflow definitions missing` | Workflow definitions being re-seeded |

---

## What Requires Operator Action

The table below summarizes what the system handles vs. what you must do:

| Condition | Auto-remediated? | Operator action required |
|-----------|-----------------|--------------------------|
| Missing workflow definitions | ✓ Auto re-seeded from disk | Only if YAML files also missing from disk |
| Quorum node count low | ✓ Auto-promotes candidate nodes | Only if no candidates exist (add hardware) |
| MinIO node not in pool | ✓ Resets join phase, pool manager re-adds | Check logs if repeated failures |
| MinIO service not running | ✓ Clears config hash, restarts service | Only if service keeps crashing |
| Certificate expiring (<30d) | ✓ Restarts node-agent, cert re-issued | Only if CA is missing |
| Certificate SAN missing | ✓ Restarts node-agent, cert re-issued | Only if CA is missing |
| Disk WARN (< 20% free) | Alert emitted, deployments not blocked | Clean up before it becomes critical |
| Disk CRITICAL (< 5% free) | Alert emitted, deployments blocked | **Act now** — journal vacuum, remove old packages |
| Single node unreachable | Soft-fenced, deployments paused | Check network, SSH access; hardware if needed |
| ≥2 nodes unreachable | Critical alert + etcd key written | **Trigger emergency backup, restore hardware** |
| Missing CA certificate | Not recoverable automatically | Restore from backup |
| Physical disk 100% full | Cannot act (etcd write fails) | Emergency SSH cleanup |
| Founding profile removed via etcd bypass | Reported, not auto-corrected | `globular cluster nodes profiles set` |

---

## Invariant Enforcement Schedule

The invariant workflow runs on the **leader node** on every reconcile loop trigger (approximately every 30 seconds). The reconcile loop fast-path checks (quorum, disk, workflow completeness) run on every cycle. The full workflow — which makes RPC calls to node-agents and may restart services — is rate-limited to avoid hammering the cluster:

| Check | Frequency | Requires |
|-------|-----------|---------|
| Workflow completeness (reconcile loop) | Every 5 minutes | etcd only |
| Storage quorum enforcement | Every reconcile cycle | etcd only |
| Disk health logging | Every reconcile cycle | etcd only |
| Full invariant workflow | Every reconcile cycle trigger | etcd + Workflow Service |
| PKI health (node-agent RPCs) | Every invariant workflow run | etcd + node-agent reachability |
| Disk health (structured events) | Every invariant workflow run | etcd + Workflow Service |
| Node reachability classification | Every invariant workflow run | etcd + Workflow Service |

---

## Related Pages

- [Backup and Restore](backup-and-restore.md) — Emergency backup procedures, restore from snapshot
- [Failure Scenarios](failure-scenarios.md) — Infrastructure, service, and node failure catalog with step-by-step recovery
- [Cluster Doctor](cluster-doctor.md) — The Cluster Doctor service: invariant rules, findings, auto-heal
- [High Availability](high-availability.md) — Quorum, leader election, VIP failover
- [Node Recovery](node-recovery.md) — Full node wipe-and-rebuild procedure
- [Certificate Lifecycle](certificate-lifecycle.md) — PKI provisioning, rotation, troubleshooting
