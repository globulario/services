# MinIO Topology Validation

This document explains how to prove that MinIO topology automation has converged correctly at each lifecycle stage. All checks are read-only — no mutations, no restarts.

---

## What convergence means

MinIO topology convergence is proven by four independent checks:

1. **Generation applied** — `applied_generation` in etcd equals `desired_generation`
2. **Fingerprint match** — every pool node rendered the exact topology described by desired state (not just the right generation number)
3. **Service active** — `globular-minio.service` is running on all pool nodes
4. **Health endpoint** — MinIO responds HTTP 200 at `/minio/health/live`

The fingerprint is a SHA-256 of `generation|mode|sorted_nodes|drives_per_node|volumes_hash`. Two nodes that rendered identical topology have identical fingerprints — this proves they didn't render stale standalone configs or old pool membership.

---

## Quickstart

```bash
# Full convergence proof (requires etcdctl, jq, curl, sha256sum)
./scripts/validate-minio-topology.sh

# Skip SSH checks (read-only from control plane only)
NO_SSH=1 ./scripts/validate-minio-topology.sh

# CLI (requires globular CLI + etcd access)
globular objectstore topology status

# JSON output for scripting
globular objectstore topology status --json | jq .converged
```

Exit code 0 = converged. Exit code 1 = not converged (exact cause printed).

---

## Key etcd paths

| Path | Content |
|------|---------|
| `/globular/objectstore/config` | Desired state: mode, nodes, drives, endpoint, volumes_hash, generation |
| `/globular/objectstore/applied_generation` | Last generation fully applied by the workflow |
| `/globular/objectstore/restart_in_progress` | Set while workflow is running; cleared on success or failure |
| `/globular/objectstore/last_restart_result` | JSON: `{status, applied_at/failed_at, reason}` |
| `/globular/locks/objectstore/minio/topology-restart` | Workflow mutex; lease-backed (30-min TTL) |
| `/globular/nodes/{id}/objectstore/rendered_generation` | Per-node: last generation the node agent rendered |
| `/globular/nodes/{id}/objectstore/rendered_state_fingerprint` | Per-node: fingerprint of the topology the node agent rendered |

---

## Scenario A: Day-0 single node (standalone)

**Context**: First node joined, MinIO running in standalone mode. No pool yet.

**Expected etcd state**:
```
/globular/objectstore/config        → mode=standalone, nodes=["10.0.0.63"], generation=1
/globular/objectstore/applied_generation → 1
restart_in_progress                  → (absent)
topology lock                        → (absent)
```

**Expected node state**:
```
rendered_generation  = 1
rendered_fingerprint = SHA256("1|standalone|10.0.0.63|1|<volumes_hash>")
globular-minio.service = active
```

**Validation output** (healthy):
```
Desired generation:   1
Mode:                 standalone
Pool nodes:           10.0.0.63
Applied generation:   1
Pending:              no
Restart in progress:  no
Topology lock:        not held
Overall:              CONVERGED
```

**What to look for**:
- `distributed.conf` must NOT exist on the node (standalone needs no distributed override)
- `MINIO_VOLUMES` in `minio.env` should be a local path, not `http://`

---

## Scenario B: Day-1 join — second or third node joins, topology becomes distributed

**Context**: A second (or third) node joins the cluster. The controller publishes a new desired state with `mode=distributed`, bumps the generation, and fires `objectstore.minio.apply_topology_generation`.

**Workflow sequence**:
1. Controller writes `desired_generation=3`, `mode=distributed`, `nodes=[...]` to etcd
2. Workflow acquires the topology lock (etcd lease, 30-min TTL)
3. Each pool node's node agent reads new desired state, renders `distributed.conf` and `minio.env`, writes `rendered_generation=3` and `rendered_state_fingerprint=<fp>` to etcd
4. Workflow step `check_all_nodes_rendered` polls until all pool nodes have the correct fingerprint (not just generation number)
5. Workflow step `start_minio` restarts `globular-minio.service` on each pool node
6. Workflow step `verify_minio_cluster_healthy` polls MinIO `/minio/health/live` until HTTP 200
7. Workflow step `record_applied_generation` writes `applied_generation=3`
8. Lock released, `restart_in_progress` cleared, `last_restart_result={status:succeeded}` written

**Validation output** (mid-workflow, normal):
```
Desired generation:   3
Applied generation:   2
Pending:              YES
Restart in progress:  YES (flag set)
Topology lock:        HELD since 2026-04-25T10:31:00Z
Overall:              NOT CONVERGED: applied_generation=2 < desired=3; restart_in_progress flag set
```

**Validation output** (after convergence):
```
Desired generation:   3
Mode:                 distributed
Pool nodes:           10.0.0.63, 10.0.0.8, 10.0.0.20
Applied generation:   3
Pending:              no
Restart in progress:  no
Topology lock:        not held
Last result:          succeeded at 2026-04-25T10:32:47Z

NODE       IP          RENDERED_GEN  FINGERPRINT_MATCH  SERVICE
node-1     10.0.0.63   3             CONVERGED
node-2     10.0.0.8    3             CONVERGED
node-3     10.0.0.20   3             CONVERGED

MinIO health:  HEALTHY (HTTP 200) at http://10.0.0.63:9000/minio/health/live
Overall:       CONVERGED
```

**What to look for**:
- All three nodes must show `rendered_generation == desired_generation`
- All fingerprints must match — a mismatch means a node rendered a different topology (stale standalone config, wrong pool membership)
- `MINIO_VOLUMES` in `minio.env` must start with `http://` on all pool nodes
- `distributed.conf` must exist under `/etc/systemd/system/globular-minio.service.d/`

---

## Scenario C: Rejoin or wipe — a pool node is wiped and rejoins

**Context**: `node-2` (10.0.0.8) is reinstalled. Its local MinIO data and config are gone. The node agent reads desired state from etcd and re-renders.

**What must happen automatically** (no operator intervention):
1. Node agent starts, reads `/globular/objectstore/config` from etcd
2. Renders `minio.env` with distributed `MINIO_VOLUMES` (not standalone)
3. Renders `distributed.conf`
4. Writes `rendered_generation=3` and the correct fingerprint to etcd
5. A topology workflow may or may not fire (depends on whether generation changed)

**Proof of correct re-render** — check per-node keys directly:
```bash
etcdctl get /globular/nodes/node-2/objectstore/rendered_state_fingerprint
# Must equal the expected fingerprint computed from desired state

etcdctl get /globular/nodes/node-2/objectstore/rendered_generation
# Must equal desired_generation (3)
```

**Common failure mode** — node renders standalone after wipe:
```
NOT CONVERGED: node node-2(10.0.0.8): fingerprint mismatch (rendered=a1b2c3d4)
```

The fingerprint mismatch means the rendered fingerprint doesn't match the expected value computed from the current desired state. The node rendered a different topology — likely standalone. Fix:
1. Check `minio.env` on the node: `MINIO_VOLUMES` must be `http://` not a local path
2. Check node agent logs for why it rendered standalone
3. If desired state was not yet read: wait for node agent to sync, or force a desired-state re-read

---

## Scenario D: Workflow failure — lock stuck, restart_in_progress set

**Context**: The topology workflow fails mid-run (controller crash, network partition, MinIO health check timeout).

### What the `onFailure` handler does

When the workflow fails, `controller.objectstore.failure_cleanup` runs atomically:
1. Deletes the topology lock (releasing the etcd lease)
2. Deletes `restart_in_progress`
3. Writes `last_restart_result = {status: "failed", failed_at: <timestamp>, reason: "workflow_failed"}`

### Detecting a clean failure

```
Pending:              YES
Restart in progress:  no
Topology lock:        not held
Last result:          failed at 2026-04-25T10:35:12Z
Overall:              NOT CONVERGED: applied_generation=2 < desired=3
```

This is the correct state after a clean failure — the lock is gone, `restart_in_progress` is cleared, and `last_restart_result` records the failure. The workflow can be safely retried.

### Detecting a crash without cleanup (stale lock)

If the controller crashed before `onFailure` ran, you may see:
```
Restart in progress:  YES (flag set)
Topology lock:        HELD since 2026-04-25T09:00:00Z
```

The lock uses a 30-minute etcd lease TTL. After 30 minutes, the lease expires automatically and the lock is released. The `restart_in_progress` flag is then the only remaining artifact.

Manual recovery (only if the lock has been held for > 30 min):
```bash
# Verify lock TTL is expired
etcdctl get /globular/locks/objectstore/minio/topology-restart

# Clear stale restart_in_progress
etcdctl del /globular/objectstore/restart_in_progress

# Clear stale lock if still present
etcdctl del /globular/locks/objectstore/minio/topology-restart

# Write explicit failed result
etcdctl put /globular/objectstore/last_restart_result \
  '{"status":"failed","failed_at":"'$(date -u +%Y-%m-%dT%H:%M:%SZ)'","reason":"manual_recovery"}'
```

Then retry the workflow:
```bash
globular workflow start objectstore.minio.apply_topology_generation
```

### Detecting health-check failure (applied_generation not advanced)

If MinIO fails to become healthy after restart, `verify_minio_cluster_healthy` times out and the workflow fails. In this state:

```
Applied generation:   2          ← not advanced (health check failed before record step)
Desired generation:   3
Last result:          failed at 2026-04-25T10:35:12Z
MinIO health:         UNHEALTHY: connection refused
```

The `applied_generation` was intentionally NOT advanced because MinIO never confirmed healthy. The previous generation's applied state is preserved — nothing was silently committed.

---

## Doctor invariants

The cluster doctor continuously checks topology convergence. Run `globular doctor report` to see current findings.

| Invariant ID | Severity | Fires when |
|-------------|---------|-----------|
| `objectstore.minio.topology_consistency` | WARN | `applied_generation < desired_generation` |
| `objectstore.minio.topology_consistency` | CRITICAL | Desired mode is distributed but `applied_generation == 0` (workflow never ran) |
| `objectstore.minio.fingerprint_divergence` | CRITICAL | Any pool node rendered a different topology fingerprint than expected |
| `objectstore.minio.post_apply_health` | CRITICAL | `applied_generation >= desired_generation` but a pool node's `globular-minio.service` is not active |

The `fingerprint_divergence` invariant is the strongest proof — it fires even when generation numbers match but the actual rendered config diverges (e.g., node rendered standalone after a wipe).

---

## Troubleshooting reference

| Symptom | Likely cause | Check |
|---------|-------------|-------|
| `pending: YES` after join | Workflow hasn't run or is in progress | `globular workflow status objectstore.minio.apply_topology_generation` |
| `fingerprint: mismatch` on a node | Node rendered different topology | Check `minio.env` MINIO_VOLUMES on the node; check node agent logs |
| `fingerprint: missing` on a node | Node hasn't rendered yet | Node agent may not have synced desired state; check node agent health |
| `restart_in_progress: YES` with no lock | Crash before `onFailure`; lock auto-expired | Clear flag manually, retry workflow |
| Lock held > 30 min | Controller crash before onFailure + lease not granted | Lock should auto-expire; if not, delete manually |
| MinIO health unhealthy post-apply | MinIO failed to start in distributed mode | Check `journalctl -u globular-minio.service` on pool nodes |
| `applied_generation` not advancing | `verify_minio_cluster_healthy` timed out | MinIO health check failed; see above |

---

## Live validation record — 2026-04-26

**Tag**: `v1.0.81-minio-gate-validated`
**Cluster**: globule-ryzen (10.0.0.63), globule-nuc (10.0.0.8), globule-dell (10.0.0.20)
**Desired state**: standalone, generation=1, pool=`[10.0.0.63]`

### Gate enforcement confirmed

| Node | Pool member | Result |
|------|-------------|--------|
| globule-ryzen (10.0.0.63) | YES | MinIO active, 200 live/ready throughout |
| globule-nuc (10.0.0.8) | NO | `enforceMinioHeld` stopped MinIO at 05:18:31 on first reconcile after deploy |
| globule-dell (10.0.0.20) | NO | MinIO was not running (already held from prior reconcile) |

**Nuc journal evidence**:
```
2026-04-26T05:18:31 globule-nuc minio[3470147]: INFO: Exiting on signal: TERMINATED
2026-04-26T05:18:31 globule-nuc systemd[1]: Stopping globular-minio.service...
2026-04-26T05:18:31 globule-nuc systemd[1]: globular-minio.service: Deactivated successfully.
```

MinIO on nuc had been running since 03:43:51 (started by the apply_topology_generation workflow
coordinated restart — this is expected: the workflow restarts all pool nodes simultaneously,
and nuc was briefly a restart target). The new node-agent (v1.0.81) deployed at ~05:18 fired
`reconcileMinioSystemdConfig` on startup, detected nuc is not in `ObjectStoreDesiredState.Nodes`,
and stopped it within seconds.

### Paths validated

| Path | Mechanism | Status |
|------|-----------|--------|
| A — ControlService RPC | `nodeIPInPool()` rejects `start`/`restart` | Validated by unit tests (12 cases) |
| B — ApplyPackageRelease | Returns `installed_held` for non-members | Validated by unit tests |
| C — workflow restart action | Only targets `$.pool_nodes` (derived from desired state) | Design-safe by construction |
| reconcile — `enforceMinioHeld` | Stops active MinIO on non-member at every sync | **Live-validated** on nuc at 05:18:31 |

### Root cause of earlier MinIO outage (same session)

MinIO on ryzen was stopped manually by `unix-user:dave` at 04:40:14 (confirmed via polkit log),
not by the apply_topology_generation workflow (which had already completed successfully at 03:43
with `RUN_STATUS_SUCCEEDED`). The deadlock that followed (repo needs MinIO, node-agent needs repo
to fetch MinIO) was broken by manually restarting MinIO on ryzen after confirming pre-flight:
desired state present, ryzen in pool, no pending destructive transition, no active lock.
