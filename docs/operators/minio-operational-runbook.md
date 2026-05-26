# MinIO Operational Runbook

This runbook covers the failure scenarios encountered during live cluster operations.
For topology convergence validation, see [minio-topology-validation.md](minio-topology-validation.md).

---

## Quick reference: etcd paths

| Path | Purpose |
|------|---------|
| `/globular/cluster/scylla/hosts` | ScyllaDB seed IPs read by workflow, ai_memory, resource services at startup |
| `/globular/objectstore/config` | Current objectstore config (mode, nodes, drives, credentials) |
| `/globular/objectstore/applied_generation` | Written when topology workflow completes |
| `/globular/objectstore/reconcile/last` | Outcome of the last objectstore reconciler tick |
| `/globular/objectstore/topology/transition/{gen}` | Approved destructive transition record — node-agents check this before wiping `.minio.sys` |
| `/globular/nodes/{node_id}/suspended/minio` | Crash-loop suppressor marker (TTL=1800s); minio will not restart while this key exists |

---

## Incident 1: Workflow service crash-loops at startup ("unable to dial control conn")

### Symptom

```
globular-workflow.service crashes immediately with:
gocql: unable to dial control conn 10.0.0.1:9042: dial tcp 10.0.0.1:9042: connect: connection refused
```

The IP `10.0.0.1` (or any non-cluster IP) appears in the ScyllaDB seed list.

### Root cause

`/globular/cluster/scylla/hosts` in etcd contains a router/gateway address instead of
real cluster node IPs. This poisons every service that calls `config.GetScyllaHosts()`
on startup: workflow, ai_memory, resource.

This happens when a node transiently has the gateway IP bound to one of its interfaces
(keepalived misconfiguration, PPPoE, bridge), `gatherIPs()` captures it, it sorts before
the real IP (e.g. `10.0.0.1 < 10.0.0.63`), and `publishScyllaHostsIfNeeded()` writes it.

**Fixed in v1.2.106**: `gatherIPs()` now excludes default-gateway IPs by reading
`/proc/net/route`, and `publishScyllaHostsIfNeeded()` TCP-probes port 9042 before writing.

### Diagnosis

```bash
# Check what is currently in the key
etcdctl --endpoints=https://<node>:2379 \
  --cacert=/var/lib/globular/pki/ca.crt \
  --cert=/var/lib/globular/pki/issued/services/service.crt \
  --key=/var/lib/globular/pki/issued/services/service.key \
  get /globular/cluster/scylla/hosts
# Correct value: ["10.0.0.63","10.0.0.8","10.0.0.20","10.0.0.9","10.0.0.102"]
# Bad value:     ["10.0.0.1"]
```

### Fix

```bash
# Replace with the real cluster node IPs (all nodes with core or storage profile)
etcdctl ... put /globular/cluster/scylla/hosts \
  '["10.0.0.63","10.0.0.8","10.0.0.20","10.0.0.9","10.0.0.102"]'

# Reset failed state and start the service
sudo systemctl reset-failed globular-workflow.service
sudo systemctl start globular-workflow.service

# Verify
sudo journalctl -u globular-workflow.service -n 5 --no-pager | grep -i "scylla\|connected\|failed"
```

**Expected recovery log**: `ScyllaDB connected hosts=[10.0.0.63 10.0.0.8 ...]`

---

## Incident 2: MinIO crash-loops after standalone→distributed transition ("format.json: 1 drive, specified: 5")

### Symptom

```
globular-minio.service on one or more nodes fails immediately with:
FATAL Unable to initialize backend: /mnt/data/data drive is already being used in
another erasure deployment. (Number of drives specified: 5 but the number of drives
found in the 1st drive's format.json: 1)
```

After 5 crashes in 60 seconds, the crash-loop suppressor fires:
```
crash-loop-suppressor: globular-minio.service crashed 5 times in 1m0s — disabling unit
crash-loop-suppressor: wrote suspended marker /globular/nodes/{node_id}/suspended/minio (TTL=1800s)
```

### Root cause

During a standalone→distributed transition, the node-agent renders the new distributed
config (5-node MINIO_VOLUMES) and starts MinIO. But MinIO finds the old `.minio.sys/format.json`
from the previous standalone deployment (which records `drives=1`). MinIO refuses to start
with a mismatched drive count to protect data integrity.

The node-agent logs this as: `standalone→distributed mode transition detected (gen=1) — wipe
governed by transition record` — it defers the wipe to be authorized by the transition record,
but the wipe itself is not guaranteed before the first MinIO start attempt.

### Data path note

The MinIO data subdirectory is one level below the approved disk path:
- Approved disk: `/mnt/data`
- MinIO data dir: `/mnt/data/data`
- Format file:    `/mnt/data/data/.minio.sys/format.json`  ← this is what needs to be wiped

### Diagnosis

```bash
# Confirm the crash reason
sudo journalctl -u globular-minio.service -n 30 --no-pager | grep -i "fatal\|format"

# Check if the crash-loop suppressor has fired
etcdctl ... get /globular/nodes/{node_id}/suspended/minio

# Check .minio.sys exists with old format
sudo ls -la /mnt/data/data/.minio.sys/
```

### Fix

**Step 1**: Wipe the stale `.minio.sys` on every affected node.

```bash
# For each node showing the crash (ryzen, lenovo, etc.)
sudo rm -rf /mnt/data/data/.minio.sys
```

**Step 2**: Clear the crash-loop suspension marker from etcd.

```bash
# Find the node_id (from etcd or globular cluster nodes)
etcdctl ... del /globular/nodes/{node_id}/suspended/minio
```

**Step 3**: Reset failed systemd state and start MinIO.

```bash
sudo systemctl reset-failed globular-minio.service
sudo systemctl start globular-minio.service
```

**Step 4**: Verify MinIO forms the distributed cluster.

```bash
sudo journalctl -u globular-minio.service -n 20 --no-pager | grep -E "Formatting|Pool|API:"
# Expected: "Formatting 1st pool, 1 set(s), 5 drives per set."
# Then:     "API: https://10.0.0.63:9000"
```

**Step 5**: Confirm all nodes are active.

```bash
for node in globule-ryzen globule-nuc globule-dell globule-hp-01 globule-lenovo; do
  echo "$node: $(ssh $node systemctl is-active globular-minio.service)"
done
```

---

## Incident 3: Topology workflow stuck — "already owned by another executor"

### Symptom

The controller repeatedly logs:
```
workflow objectstore.minio.apply_topology_generation (correlation=objectstore.topology:1):
RPC failed: rpc error: code = Unknown desc = run objectstore.topology:1 already owned
by another executor
```

The objectstore reconciler records `FAILED_TRANSIENT` and enters exponential backoff.

### Root cause

The workflow service crashed mid-execution while the topology run was in-flight. The executor
lease in ScyllaDB (`workflow.executor_leases`) still shows the old (dead) executor as the owner.
When the controller re-dispatches, the new workflow service rejects the run.

### What happens automatically

The workflow service runs an **orphan scanner** every 15 seconds. The scanner:
1. Reads all rows from `workflow.executor_leases`
2. Finds rows where `heartbeat_at` is older than 30 seconds
3. Claims stale runs via LWT and calls `resumeOrphanedRun()`

**Do not intervene immediately.** Within ~30–60 seconds after the workflow service starts,
the orphan scanner claims the stale lease and resumes the run. The controller's repeated
"already owned" errors stop once the scanner has taken ownership.

### How to confirm the scanner has taken over

```bash
sudo journalctl -u globular-workflow.service --since "5 minutes ago" --no-pager \
  | grep -i "objectstore.topology\|check_all_rendered\|orphan"
```

Expected after recovery:
```
executor: starting workflow  run_id=objectstore.topology:1
executor: dispatching action  step_id=check_all_rendered
```

### If the run is genuinely stuck (orphan scanner not firing)

Check if the workflow service is actually running:
```bash
systemctl is-active globular-workflow.service
sudo journalctl -u globular-workflow.service -n 20 --no-pager | grep -E "connected|failed|ScyllaDB"
```

If the workflow service is crash-looping, fix that first (see Incident 1 above).

---

## Incident 4: Objectstore reconciler in permanent BACKOFF

### Symptom

```bash
etcdctl ... get /globular/objectstore/reconcile/last
# {"outcome":"BACKOFF","reason":"retry_in_11m0s","desired_generation":1,"storage_nodes":5}
```

The reconciler is not dispatching the topology workflow, even though `applied_generation`
does not match `desired_generation`.

### How backoff works

The reconciler uses exponential backoff after repeated failures:
- First failure: `FAILED_TRANSIENT`, retries on next tick (30s)
- After 2+ failures: `BACKOFF` with exponential delay (3m → 6m → 11m → up to 30m)

Backoff is intentional — if the workflow service is unavailable, hammering it with retries
does not help. Wait for the backoff window to expire. The reconciler fires again automatically.

### Check if backoff is still active

```bash
etcdctl ... get /globular/objectstore/reconcile/last | python3 -c "
import json,sys,time
d=json.load(sys.stdin)
ts=d.get('timestamp_unix',0)
age=int(time.time())-ts
print(f'Last reconcile: {age}s ago, outcome: {d[\"outcome\"]}, reason: {d.get(\"reason\",\"\")}')
"
```

### If backoff is too long and you need it to fire sooner

Delete the last reconcile record to force an immediate retry:
```bash
etcdctl ... del /globular/objectstore/reconcile/last
```

The reconciler detects the missing key as "no record" and fires on the next tick (30s).

---

## Incident 5: Doctor reports stale objectstore incidents after MinIO converges

### Symptom

Doctor reports:
- `objectstore.endpoint_unreachable` — MinIO endpoint not reachable
- `objectstore.minio.topology_consistency` — applied=0, desired=1
- `objectstore.minio.destructive_guard` — no transition record at `.../transition/1`

...even though MinIO is actually running and `applied_generation=1` is in etcd.

### Root cause

The doctor sweep runs on a cycle. Incidents raised in a prior sweep are not auto-resolved
until the next sweep sees the corrected state. The doctor may also have a stale snapshot
from before the topology workflow completed.

### Fix

Wait for the next doctor sweep (typically 60–120s). The incidents auto-resolve when the
doctor re-reads the current etcd state.

If incidents persist past 5 minutes:
```bash
# Confirm MinIO is healthy
curl -sk https://10.0.0.63:9000/minio/health/live && echo "OK"

# Confirm applied_generation matches desired
etcdctl ... get /globular/objectstore/applied_generation
etcdctl ... get /globular/objectstore/config | python3 -c "import json,sys; d=json.load(sys.stdin); print('desired gen:', d['generation'])"
```

If both are equal and MinIO is healthy, the incidents are stale. Force a doctor re-sweep:
```bash
globular doctor run
```

---

## Reference: MinIO distributed mode data layout

```
/mnt/data/                   ← approved disk path (node-agent manages ownership)
└── data/                    ← MinIO data subdirectory (MinIO creates this)
    ├── .minio.sys/          ← MinIO metadata; wipe this to reset drive membership
    │   └── format.json      ← encodes number of drives and pool layout
    ├── globular/            ← main object storage bucket
    ├── globular-backups/    ← backup bucket
    ├── globular-config/     ← config bucket
    └── globular-search-index/ ← search index bucket
```

**MINIO_VOLUMES pattern** (distributed mode, 5 nodes):
```
https://10.0.0.63:9000/mnt/data/data https://10.0.0.102:9000/mnt/data/data ...
```

**MINIO_VOLUMES pattern** (standalone mode):
```
/mnt/data/data
```

When `MINIO_VOLUMES` changes from standalone to distributed (or drive count changes),
MinIO detects the `format.json` mismatch and refuses to start. Always wipe
`/mnt/data/data/.minio.sys` before starting MinIO with a new pool configuration.

---

## Reference: Crash-loop suppressor

The node-agent monitors service crash frequency. If a service crashes 5+ times in 60 seconds,
it writes a suspended marker to etcd with TTL=1800s (30 min):

```
/globular/nodes/{node_id}/suspended/{service_name}
```

While this key exists, the node-agent will not start the service. The key auto-expires after
30 minutes, or can be cleared manually:

```bash
# Find the node_id
globular cluster nodes

# Clear the suspension
etcdctl ... del /globular/nodes/{node_id}/suspended/minio

# Then reset and start
sudo systemctl reset-failed globular-minio.service
sudo systemctl start globular-minio.service
```

The suppressor prevents crash storms from filling journals, but it can delay recovery.
Always clear it manually when you know the underlying cause is fixed.

---

## Topology apply checklist (standalone → distributed)

Before running `globular objectstore topology apply`:

- [ ] All target nodes have an admitted disk (`globular objectstore disk scan`)
- [ ] Workflow service is healthy on the controller node: `systemctl is-active globular-workflow.service`
- [ ] `/globular/cluster/scylla/hosts` contains real cluster IPs (not gateway): `etcdctl get /globular/cluster/scylla/hosts`
- [ ] No stale executor lease for `objectstore.topology:N`: wait for orphan scanner if workflow service recently restarted

After apply:

- [ ] `objectstore.topology:N` workflow shows SUCCEEDED in `globular workflow list`
- [ ] `/globular/objectstore/applied_generation` equals desired generation
- [ ] All 5 nodes show `globular-minio.service: active`
- [ ] MinIO health: `curl -sk https://{node_ip}:9000/minio/health/live` returns HTTP 200
- [ ] No suspended/minio markers in etcd
