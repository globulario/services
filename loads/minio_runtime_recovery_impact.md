# MinIO Runtime Recovery — Impact Report (Project E)

## Headline

**MinIO is actually running and healthy.** The `InfrastructureRelease`'s
`DEGRADED` phase is a STALE label from an earlier window where the unit
was inactive. The CRITICAL `objectstore.endpoint_unreachable` doctor
finding from the Project C inventory has cleared.

The remaining gap is **`objectstore_topology_mismatch` (primary) +
`desired_runtime_profile_mismatch` (secondary)**: the systemd unit and
the etcd `objectstore_contract` disagree about which disk path MinIO is
running on, and the cluster has only 1 storage node while the
reconciler requires 3 for distributed quorum.

## What is HEALTHY (do not touch)

| Probe | Result |
|---|---|
| `systemctl is-active globular-minio.service` | active (running) since 22:03:19 |
| Main PID | 121365 |
| TCP probe to `10.0.0.63:9000` | open |
| `curl https://10.0.0.63:9000/minio/health/live` | HTTP 200, 8.7ms |
| Certificate SAN includes 10.0.0.63 + globule-ryzen + *.globular.internal | yes |
| Repository MinIO mirror writes in last 5 min | none failed (silent — endpoint healthy) |
| `installed_state` SERVICE/INFRASTRUCTURE | `INFRASTRUCTURE` row at v1.2.70 with status=installed, no error metadata |

## What is BROKEN or STALE

### 1. Path mismatch (primary)

| Source | Path |
|---|---|
| Systemd unit `ExecStart` | `/mnt/40F43F08F43EFFA8/minio/data` |
| Objectstore contract `node_paths["10.0.0.63"]` | `/var/lib/globular/minio` |

The two sources of truth disagree. MinIO is happily running on the
mounted disk path, but the contract expects the default `/var/lib`
path. The reconciler currently can't act on this divergence because it
also hits **storage_nodes_below_quorum:1** and skips.

### 2. Historical `format.json` divergence on the mounted disk

Three different `format.json` files exist under
`/mnt/40F43F08F43EFFA8/minio/`:

| Path | Mode | Cluster identity | Set size | "this" UUID |
|---|---|---|---|---|
| `data/.minio.sys/format.json` | xl | A | 3 drives | `d2fa8438-…` |
| `data1/.minio.sys/format.json` | xl | B | 6 drives | `2a84ad30-…` |
| `data2/.minio.sys/format.json` | xl | B | 6 drives | `2bddb740-…` |

The `data` directory was a single-set 3-drive XL pool. `data1`+`data2`
were part of a separate 6-drive XL pool with a different cluster ID.
MinIO is currently running pointed only at `data`, so it is using
cluster A. `data1` and `data2` are unused leftovers from a previous
topology generation. **This is leftover state, not active corruption.**

### 3. Storage quorum below founding-cluster minimum

| Field | Value |
|---|---|
| `cluster_list_nodes` size | 1 (only globule-ryzen) |
| `cluster.founding_quorum` (architecture) | 3 (etcd + ScyllaDB + MinIO replication) |
| Objectstore reconciler outcome | `SKIP_NO_QUORUM` |
| Reason | `storage_nodes_below_quorum:1` |

This is the cluster's broader degraded state (1 of 5 nodes online),
not a MinIO defect. The contract was written as `mode=standalone` to
match the 1-node reality. The reconciler still refuses to alter the
topology until quorum recovers, which means even a contract bump (to
move from `/mnt/…` to `/var/lib/globular/minio` or vice versa) can't be
safely applied right now.

### 4. Stale `release.phase = DEGRADED`

The `InfrastructureRelease` for minio carries:

```
phase = DEGRADED
message = WAVE_BLOCKED max_parallel_nodes=1 total_nodes=1 total_waves=1 note=RUN_STATUS_SUCCEEDED
last_transition_unix_ms = 1780020200215  (2026-05-28 22:23:20 EDT)
nodes[0].phase = DEGRADED, error_message = (empty)
```

The note `RUN_STATUS_SUCCEEDED` means the workflow succeeded. The
`DEGRADED` label was written by the convergence-committer when the
unit was inactive (pre-22:03:19, see Project C's CRITICAL finding).
Once the unit came up, the release record was not re-evaluated. The
controller's reconcileAvailable / convergence-committer would normally
roll DEGRADED forward to AVAILABLE when health improves, but the
**doctor's MinIO inactive finding was the trigger and no fresh
ConvergenceResultV1 has flowed back to advance the release**.

## Classification

| Classification | Match | Rationale |
|---|---|---|
| `systemd_unit_inactive` | NO | unit is active (running) |
| `objectstore_contract_missing` | NO | `/globular/objectstore/config` is populated and `credentials_ready=true`, `endpoint_ready=true` |
| `objectstore_topology_mismatch` | **YES (primary)** | unit `ExecStart` path ≠ contract `node_paths` value |
| `minio_format_mismatch` | partial | three format.json files with different cluster IDs at data/data1/data2; MinIO running with `data` only, others are stale leftovers |
| `certificate_or_dns_san_failure` | NO | cert SANs include 10.0.0.63 + cluster names |
| `disk_path_missing_or_permission_denied` | NO | unit started cleanly, MainPID alive |
| `package_install_incomplete` | NO | installed_state.status=installed at desired version |
| `desired_runtime_profile_mismatch` | **YES (secondary)** | cluster has 1 storage node; founding-quorum minimum is 3; reconciler SKIP_NO_QUORUM |
| `dependency_not_ready` | NO | endpoint healthy, mirror writes silent |
| `unknown_impact` | NO | every column has bounded evidence |

**Primary classification: `objectstore_topology_mismatch`.**
**Secondary classification: `desired_runtime_profile_mismatch`.**

## Risk

Medium.

- **Now**: MinIO is functional. Repository mirror works. No data loss
  in progress.
- **If quorum recovers** (other 4 nodes rejoin): the reconciler may
  attempt to align the contract with the unit. If the path divergence
  is resolved by changing the unit to point at
  `/var/lib/globular/minio`, MinIO would start with a fresh empty
  format.json there and the existing `globular` + `globular-search-index`
  buckets under `/mnt/…/data` would be invisible. **This is the
  forbidden-fix territory: do not regenerate format.json or change
  unit paths without explicit topology approval.**
- **If quorum stays at 1**: MinIO continues running, but the DEGRADED
  release label remains and the doctor's broader hash_drift findings
  continue.

## Recommended next steps (NOT Project E scope — inventory only)

Per the handoff's "forbidden fixes":

- Do NOT wipe MinIO data.
- Do NOT delete format.json.
- Do NOT force a 4-node/5-node topology change.
- Do NOT regenerate objectstore contract without explicit topology approval.
- Do NOT mark MinIO AVAILABLE manually.

What COULD safely happen next (would need its own handoff/Project F):

1. **Reconcile the release label.** The cluster-controller's
   `reconcileAvailable` should re-evaluate DEGRADED→AVAILABLE for
   InfrastructureReleases whose underlying unit and endpoint are
   healthy. Today it appears stale-state-only. A small enhancement
   would make the release follow the runtime evidence.

2. **Align the unit ExecStart with the contract.** Either:
   - (a) update the contract `node_paths` to `/mnt/40F43F08F43EFFA8/minio/data`
     so the contract reflects what's running (forward-compatible: when
     other nodes join, they can replicate the same path), or
   - (b) plan a migration from `/mnt/…` to `/var/lib/globular/minio`
     via `mc mirror` (backup), then a controlled stop / format / start
     cycle, then `mc mirror` restore. **High risk** — needs operator
     planning.

   Option (a) is safer and matches reality.

3. **Restore storage quorum.** When the other 4 nodes (nuc, dell,
   hp-01, lenovo) come back online, the reconciler will transition
   from `mode=standalone` toward a distributed pool. At that point the
   path mismatch must be resolved.

## Out-of-scope follow-ups noted

- `data1` and `data2` under `/mnt/40F43F08F43EFFA8/minio/` are
  leftover from a previous 6-drive XL configuration. They are not
  referenced by the current MinIO process. Cleanup is a separate
  housekeeping item.
- The DEGRADED InfrastructureRelease phase is not currently driving
  any cascading failure (resolver works, repository mirror works,
  endpoint is healthy). It is operator-visible noise. A controller
  enhancement to re-evaluate from runtime evidence would clear it.
- The broader cluster hash_drift findings for other services remain
  outside Project E's scope.

## Awareness records (drafted, NOT yet committed)

```yaml
# failure_modes.yaml
- id: minio.release_label_stale_when_runtime_recovered
  summary: |
    InfrastructureRelease for minio stays at phase=DEGRADED with
    note=RUN_STATUS_SUCCEEDED after the underlying unit recovered.
    The convergence-committer wrote DEGRADED when the unit was
    inactive; once the unit came up no fresh ConvergenceResultV1
    flows back to advance the release to AVAILABLE.

- id: minio.unit_path_disagrees_with_objectstore_contract
  summary: |
    Systemd ExecStart and /globular/objectstore/config node_paths can
    disagree about which disk path MinIO is running on. The current
    MinIO is functional but the contract reflects a different target.
    Reconciler SKIP_NO_QUORUM may mask this until storage quorum
    recovers; a topology change after quorum recovery would silently
    migrate to the wrong path.
```

```yaml
# invariants.yaml
- id: minio_runtime_path_must_match_objectstore_contract
  severity: critical
  statement: |
    The ExecStart disk path of globular-minio.service MUST equal
    /globular/objectstore/config.node_paths[<this_node_ip>]. A divergence
    is a topology hazard: a future reconciler-driven contract apply
    will move MinIO data to the contract path silently, orphaning any
    buckets stored at the unit path.
```

## Status

Inventory complete. No state mutation. No code changes. Awaiting
follow-up handoff for Project F (release label staleness) or contract
alignment.
