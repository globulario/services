# Pending awareness-graph scars — 2026-08-19

Recorded as a file because the awareness graph was unavailable at the time
(Oxigraph store at 127.0.0.1:7878 held 6 triples and lacked the authority
marker `6edb72fd83d6a9b2d8df8a28766718dd6ace1a9b5b59d0dc0b261050d34d69fe`,
so `awareness-graph` failed closed and `sensei propose` could not run).

Feed these into `sensei propose` once `sensei rebuild` has restored the store.

---

## SCAR 1 — `topology apply --i-understand-data-reset` does not remove `format.json`

**Contract status:** found (violated) — the flag's documented purpose is the wipe.

### Contract
`globular objectstore topology apply --i-understand-data-reset` is documented as:

> `--i-understand-data-reset   Confirm destructive apply: wipes .minio.sys where topology changes`

The apply preview also states explicitly:

> `DESTRUCTIVE OPERATION: standalone → distributed: .minio.sys will be wiped on 3 nodes`

### What actually happened
On a 3-drive → 4-drive pool change (generation 1 → 2), the apply wiped the
**bucket directories** but left `.minio.sys/format.json` in place on all three
pre-existing drives. MinIO was then handed a 4-drive configuration pointing at
disks still stamped with a 3-drive erasure format.

MinIO cannot expand an erasure set in place. Result:

```
FATAL Unable to initialize backend: /mnt/data/data drive is already being used in
another erasure deployment. (Number of drives specified: 4 but the number of drives
found in the 3rd drive's format.json: 3)
```

The stale format encoded a 3-UUID set:

```json
"sets":[["e941a494-c611-47ff-904f-5c758dd41ba1",
         "fad92ba3-76b2-4f57-9a7d-63c95580aab5",
         "c300c0ef-8f4a-4081-9426-bbd38c9d01c9"]]
```

### Observed impact
- MinIO restart-looped on ryzen (10.0.0.63), hp-01 (10.0.0.9), lenovo (10.0.0.102).
- hp-01 hit the systemd start limit and entered a hard `failed` state
  (`Start request repeated too quickly`), starving the pool of quorum; the
  surviving node logged `Waiting for a minimum of 2 drives to come online`.
- Object storage was fully down until manual intervention.
- **Second-order effect:** because the buckets *were* wiped while the format
  was not, all five buckets (`globular`, `globular-config`, `globular-backups`,
  `globular-packages`, `globular-search-index`) vanished. `webroot-sync` then
  failed on every node with `list objects: The specified bucket does not exist`.
  Two distinct failures from one incomplete wipe.

### Manual repair that worked
1. Stop MinIO on all pool nodes via the sanctioned path
   (`globular node control --unit globular-minio.service --action stop`).
2. Remove `.minio.sys` and the bucket dirs from each admitted drive path.
3. Let the node-agent restart MinIO — it then formatted cleanly:
   `Formatting 1st pool, 1 set(s), 4 drives per set.`
4. Recreate the buckets (see SCAR 3).

Verified after repair: `4 drives online, 0 drives offline, EC:2`; an object
written via 10.0.0.8 read back identically from all four nodes.

### Required fix
`apply` must either (a) actually remove `format.json` on every drive whose
topology changes, or (b) refuse the apply with a clear error when it cannot,
rather than leaving the pool in a state MinIO can never initialize.

### Forbidden fix
Do NOT "fix" this by having the operator hand-delete `format.json` as the
documented procedure. The reset flag exists precisely so the workflow owns
this; pushing it to the operator makes every drive-count change a manual,
error-prone outage.

### Required test
A drive-count change (N → N+1) on a formatted pool must leave every admitted
drive with no stale `format.json`, and MinIO must reach `N+1 drives online`
without entering a restart loop.

### Proposed command
```bash
sensei propose --kind failure_mode \
  --title "objectstore topology apply leaves stale format.json, making drive-count changes unrecoverable" \
  --evidence "gen1→gen2 3→4 drive change: buckets wiped but .minio.sys retained; MinIO FATAL 'already being used in another erasure deployment'; hp-01 hit systemd start limit; object storage down until manual .minio.sys removal" \
  --source-file golang/cluster_controller/cluster_controller_server/workflow_objectstore.go \
  --required-test "drive-count change leaves no stale format.json and reaches N+1 drives online"
```

---

## Also observed 2026-08-19 — not yet written up

2. **Topology status reported `✓ CONVERGED` / `MinIO health: HEALTHY` while three
   of four MinIO units were dead or restart-looping.** The gate polls a single
   endpoint on ryzen rather than verifying pool membership, so a broken pool
   reads as converged. This is what made the failure in SCAR 1 hard to see.

3. **`scripts/ensure-minio-buckets.sh` creates only the main `globular` bucket.**
   The other four required by `golang/config/minio_config.go`
   (`ClusterConfigBucket`), `backup_manager` (`globular-backups`), and
   `shared_index` (`globular-search-index`) are created only by the node-agent
   Day-0 workflow (`workflow_day0.go:240`). Any reformat therefore silently
   breaks `webroot-sync` and backups with no bucket-level recreation path.

4. **`scripts/release/install-day0.sh:90` — unenforced probe timeout.** The loop
   advertises a 120s bound (`60` iterations × `sleep 2`) but `cqlsh` is invoked
   with no client-side timeout, so a single attempt can block forever and the
   loop never reaches iteration 2. Observed as an indefinite Day-0 hang at
   "Waiting for ScyllaDB write readiness (DDL probe)" against a stale 5-node
   Raft group0 with no quorum. A `timeout 10 cqlsh …` per attempt would turn a
   silent hang into the intended warning.

5. **Scylla seed renderer gap.** The Day-0 node's rendered `scylla.yaml` keeps
   its self-only seed after peers join, even though etcd's
   `/globular/cluster/scylla/hosts` correctly lists all five hosts. Harmless at
   runtime while the node is a NORMAL ring member (severity INFO), but a wiped
   restart would re-bootstrap an isolated ring. Note
   `infra_truth/attestation.go:14` marks hand-editing `scylla.yaml` a forbidden
   fix — the owner is the post-install renderer plus controller desired state.

6. **`objectstore.write_quorum_lost` can emit CRITICAL from absent data.**
   The rule lives in `rules/objectstore_physical_overlap.go` (cluster-doctor rules package). It never
   queries MinIO — it infers active drive count from node-agent inventory
   (`globular-minio.service` state per node). Its suppression guard requires an
   *explicitly recorded* collector error (`HadError` on ListNodes /
   GetInventory) AND zero confirmed-down nodes:

   ```go
   if inventoryGap && len(knownDownNodes) == 0 { return nil }
   ```

   But nodes land in `noInventoryNodes` via paths that record no such error:
   `nodeID == ""` (no NodeRecord for the pool IP), `inv == nil` (inventory
   absent from the snapshot map), and the staleness branch. In those cases a
   partial harvest emits **CRITICAL with zero confirmed-down nodes** — a
   quorum-loss verdict resting on missing data rather than observed state.

   This contradicts the project's own principle
   `ops.always.doctor.reduced-harvest-honesty` ("doctor reduced-harvest
   findings are UNKNOWN/WARN, not FAIL/ERROR").

   **Proposed fix:** do not emit CRITICAL when `len(knownDownNodes) == 0`. If
   no node is *confirmed* down, every "down" node is merely unobserved, which
   is indeterminate, not proven quorum loss. The genuine total-blackout case
   (all node-agents down) is already covered by the `node.reachable` CRITICAL,
   so detection is not lost.

   **Caveat — NOT reproduced.** During 25 `--fresh` samples over ~4 min on
   2026-08-19 (16:39–16:45) this finding never fired; the pool was healthy
   throughout (`4 drives online, 0 offline, EC:2`, cross-node read-back OK).
   The CRITICAL observed earlier that day was *correct at the time* — MinIO was
   genuinely stopped on 3 of 4 nodes during the SCAR-1 repair. The above is a
   code-read finding (latent false-positive path), not a reproduced failure.
   Reproduce against a forced partial harvest before changing the rule.

---

## SCAR 7 — cluster-scoped rules evaluated against a node-filtered snapshot (ROOT CAUSE, REPRODUCED)

**Supersedes the "not reproduced" caveat in item 6.** This IS reproducible and is
what globular-admin displays.

### Symptom
`globular doctor report cluster` → clean (1 INFO).
`globular doctor report node <any-node>` → **CRITICAL objectstore.write_quorum_lost**,
on both leader and followers. globular-admin renders per-node findings, so every
node shows "Worst Severity: CRITICAL" while the pool is completely healthy
(`4 drives online, 0 drives offline, EC:2`, cross-node object read/write verified).

### Evidence (from `doctor report node <nuc> --fresh --json`)
```
active_drives   = 1
active_nodes    = 10.0.0.8
known_down_nodes=                      <-- EMPTY: nothing confirmed down
no_inventory_nodes = 10.0.0.63,10.0.0.102,10.0.0.9
minio_state_by_pool_ip = 10.0.0.63=unknown(no_node_record),
                         10.0.0.102=unknown(no_node_record),
                         10.0.0.9=unknown(no_node_record), 10.0.0.8=active
snapshot_node_count = 1                <-- cluster rule ran on a 1-node snapshot
data_incomplete = false
```

### Root cause — `golang/cluster_doctor/cluster_doctor_server/rules/registry.go`, `EvaluateForNode`
```go
// Filter Nodes to just the requested one.
for _, n := range snap.Nodes {
    if n.GetNodeId() == nodeID { nodesnap.Nodes = append(nodesnap.Nodes, n); break }
}

for _, inv := range r.invariants {
    if inv.Scope() == "node" || inv.Scope() == "cluster" {
        all = append(all, inv.Evaluate(nodesnap, r.cfg)...)   // <-- cluster rules get the 1-node snapshot
    }
}
```
The snapshot is deliberately narrowed to one node, then **cluster-scoped** rules
are evaluated against it. Any cluster rule that resolves membership (pool IP ->
NodeRecord) sees every other node as `no_node_record` and counts it down.
`known_down_nodes` stays empty because nothing was actually observed down — the
verdict rests entirely on absent data, violating
`ops.always.doctor.reduced-harvest-honesty`.

### Fix (exact patch, blocked on awareness briefing to apply)
```go
	var all []Finding
	for _, inv := range r.invariants {
		switch inv.Scope() {
		case "node":
			// Node-scoped rules reason about THIS node.
			all = append(all, inv.Evaluate(nodesnap, r.cfg)...)
		case "cluster":
			// Cluster-scoped rules must reason over the FULL node set.
			all = append(all, inv.Evaluate(snap, r.cfg)...)
		}
	}
```

### Required test
`EvaluateForNode` on a healthy N-node pool must not emit
`objectstore.write_quorum_lost`; a cluster-scoped rule must receive the same
node set it would receive from `EvaluateCluster`.

### Forbidden fix
Do NOT silence this by dropping cluster-scoped rules from the node report — the
admin legitimately surfaces cluster criticals on node pages. And do NOT special-case
`write_quorum_lost`; every cluster-scoped rule sharing this path is affected.

---

## SCAR 8 — node heartbeat interval sits too close to the unreachable threshold

**Symptom:** `node.reachable` CRITICAL fires intermittently on healthy nodes,
landing on a different node each time. Combined with SCAR 7 this made every node
in globular-admin show "Worst Severity: CRITICAL" on a fully healthy cluster.

**Measured 2026-08-19, 8 rounds x 5 nodes (heartbeat_age_seconds, threshold 120s):**
```
18:59  ryzen=34 nuc=113 hp01=65 lenovo=65  dell=67
19:00  ryzen=35 nuc=73  hp01=55 lenovo=79  dell=44
19:01  ryzen=64 nuc=49  hp01=37 lenovo=126 dell=94    <-- BREACH
19:03  ryzen=62 nuc=51  hp01=58 lenovo=76  dell=112
19:05  ryzen=45 nuc=79  hp01=49 lenovo=36  dell=63
19:07  ryzen=34 nuc=48  hp01=64 lenovo=55  dell=46
19:10  ryzen=80 nuc=75  hp01=47 lenovo=47  dell=48
19:12  ryzen=34 nuc=45  hp01=39 lenovo=123 dell=65    <-- BREACH
```
Range 34-126s. 2/40 readings exceed 120s; nuc (113) and dell (112) came within
10s. Max observed 126s > threshold 120s.

**Not a node fault.** Every node verified healthy at the time of the breaches:
ping OK, agent port 11000 open, `globular-node-agent` active, `NRestarts=0`.
Re-querying lenovo seconds after its CRITICAL returned `reachable: true`,
`heartbeat_age: 72s`.

**Root cause:** the heartbeat publish interval leaves too little margin under the
120s staleness threshold, so ordinary scheduling jitter crosses it.

**Fix options (tuning decision — NOT applied):**
- raise the unreachable threshold above the observed p100 (>=180s), or
- shorten the heartbeat publish interval so p100 age stays well under 120s.

**Required test:** over a sustained sample on an idle healthy cluster, max
observed `heartbeat_age_seconds` must stay below the unreachable threshold with
margin (suggest p100 <= 0.6 * threshold).

**Forbidden fix:** do NOT suppress `node.reachable` findings in the UI — an
unreachable node is a real condition that must stay visible. Fix the timing
relationship, not the reporting.
