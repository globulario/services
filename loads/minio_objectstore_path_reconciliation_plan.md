# MinIO Objectstore Path Reconciliation — Plan (Project E2)

## Headline correction to the Project E inventory

Project E's matrix and report concluded that the systemd unit
`ExecStart` path (`/mnt/40F43F08F43EFFA8/minio/data`) and the
objectstore contract `node_paths` value (`/var/lib/globular/minio`)
disagreed, and classified the gap as
`objectstore_topology_mismatch` (medium risk).

**Project E2's deeper probe found the opposite.** The systemd unit
also sets:

```
EnvironmentFile=-/var/lib/globular/minio/minio.env
```

which contains:

```
MINIO_VOLUMES=/var/lib/globular/minio/data
```

The `MINIO_VOLUMES` environment variable **overrides** the positional
`ExecStart` argument. Live write test confirms it: a freshly-piped
object (`test-token-1780022126.txt`) landed at
`/var/lib/globular/minio/data/globular/...`, not at `/mnt/...`. The
`/mnt/40F43F08F43EFFA8/minio/data` argument in the ExecStart command
line is functionally dead — MinIO ignores it.

So the runtime path AND the contract path agree. There is no
data-orphaning risk from a future topology reconcile pointing in the
wrong direction. The `/mnt/...` data is stale leftover from an
earlier deployment generation.

## Revised classification

**`contract_path_active_runtime_path_stale`**

The contract path is the active path. The `/mnt/...` "runtime path"
identified by reading the ExecStart text is functionally not the
runtime — it is stale leftover data with a different cluster identity
that MinIO has not touched since 2026-04-29.

## Evidence (full matrix in `loads/minio_objectstore_path_matrix.tsv`)

### What MinIO ACTUALLY writes to (proven, not inferred)

| Test | Result |
|---|---|
| `MINIO_VOLUMES` env | `/var/lib/globular/minio/data` |
| Recent file count last 60 min at `/var/lib/globular/minio/data` | 29 (active) |
| Recent file count last 60 min at `/mnt/40F43F08F43EFFA8/minio/data` | 0 (idle) |
| Write probe via `mc pipe wrtest/globular/test-token-<ts>.txt` | landed at `/var/lib/globular/minio/data/globular/test-token-…txt` ✓ |
| Repository init log | `repository storage initialized local=/var/lib/globular/repository mirror_available=true` (mirror to MinIO succeeded) |

### What `/mnt/40F43F08F43EFFA8/minio/data` actually contains

| Field | Value |
|---|---|
| format.json cluster ID | `1b3577da-…` (xl mode, 3-drive set) |
| Last modified | 2026-04-29 |
| Buckets present | globular, globular-search-index |
| Total size | 378M |
| MinIO writes during this session | 0 |

### What `/var/lib/globular/minio/data` actually contains

| Field | Value |
|---|---|
| format.json cluster ID | `7e935e5e-…` (xl-single mode, single-drive set) |
| Last modified | 2026-05-27 |
| Buckets present | globular, globular-config, globular-search-index |
| Total size | 320M (different content, more recent) |
| MinIO writes during this session | YES (provenance.json for v1.2.120/v1.2.122 deploys, .usage-cache.bin updates) |
| MinIO writes during write probe | YES (test-token landed here) |

The two paths are completely different MinIO clusters by identity.
The `/mnt/...` one was decommissioned at some prior point; the
`/var/lib/globular/minio/data` one is what's running.

## Risk

Low.

The data-orphaning concern from Project E was based on an incorrect
read of which path is "runtime". With the env override discovery, the
contract and the actual data path already agree.

Residual concerns:
1. **Cosmetic:** the ExecStart positional arg `/mnt/...` is misleading
   to a human reader. A future operator might believe MinIO is using
   `/mnt/...` and act on that belief. A small unit-file cleanup would
   eliminate the source of confusion.
2. **Archival:** the `/mnt/.../minio/data` directory holds 378M of
   stale leftover data from a previous 3-drive XL cluster. Cluster ID
   `1b3577da-…` is dead. Keeping it indefinitely wastes disk; deleting
   it requires explicit operator approval per the handoff's forbidden
   list.

## What I am NOT proposing in this plan

Per the handoff's "forbidden fixes" list:

- No deletion of `format.json` (neither runtime nor contract).
- No wipe of MinIO data anywhere.
- No 4-node/5-node or 1-node topology change.
- No regeneration of objectstore contract.
- No change to systemd path without an operator-approved data plan.
- No `mc mirror` or similar bulk copy.
- No manual marking of MinIO AVAILABLE.
- No deletion of stale `/mnt/.../data1` or `data2` directories without
  proof of obsolescence.
- No topology workflow run while storage_nodes_below_quorum:1.

The plan is inventory-only. Decisions remain with the operator.

## Operator decisions this plan surfaces

### Decision 1 — Unit file cosmetic cleanup

Option A — leave the unit file alone. The cosmetic gap stays as
documentation of historical intent. The env file is the source of
truth and the proof is in this report.

Option B — edit the unit file to remove the misleading positional arg
or update it to match `$MINIO_VOLUMES`. This requires:

- A node-agent-driven unit-file regeneration (the systemd unit on
  disk is owned by the install pipeline, not by ad-hoc editing).
- The next package install would regenerate the unit anyway, so a
  one-off edit would be lost — the upstream package template needs
  to be the change site, not the live unit file.
- Coordination with the install-time MinIO unit template, which is
  generated by the MinIO package spec.

Option B is the correct long-term path but requires touching the
MinIO package's unit template — outside Project E2 scope.

### Decision 2 — Stale `/mnt/...` directory archival

The `/mnt/40F43F08F43EFFA8/minio` directory contains:

```
data    — 378M, 3-drive XL cluster 1b3577da-… (stale)
data1   — additional drive of a 6-drive XL cluster (stale leftover)
data2   — additional drive of the same 6-drive XL cluster (stale leftover)
```

None are referenced by the running MinIO. Disk reclaim options:

- Leave as-is (operator preference; recoverable evidence of prior state).
- `mc mirror` the runtime path to a backup target, then archive
  `/mnt/...` with `tar` to a separate location, then operator-approved
  deletion. Requires operator approval per handoff.

### Decision 3 — Hold until storage quorum recovers

The reconciler currently reports `SKIP_NO_QUORUM` due to
`storage_nodes_below_quorum:1`. No topology change should be planned
under quorum-below-minimum. When the other 4 nodes rejoin, the
reconciler will be unblocked.

At that point, the current standalone contract may need to migrate to
distributed mode. This is a much larger plan and is **explicitly out
of Project E2 scope.**

## Awareness records (drafted, NOT yet committed)

```yaml
# failure_modes.yaml
- id: minio.unit_execstart_arg_misleads_when_env_volumes_overrides
  summary: |
    The systemd unit positional ExecStart argument for MinIO appears to
    set the data directory but is functionally overridden by the
    MINIO_VOLUMES environment variable. A human reader of the unit
    file alone may believe MinIO is using the positional path; the
    env-file path is what MinIO actually reads. Diagnose by checking
    /var/lib/globular/minio/minio.env or by probing where a fresh
    write lands.
```

```yaml
# invariants.yaml
- id: minio_runtime_path_must_match_objectstore_contract
  severity: critical
  statement: |
    A MinIO node's actual data path (determined by MINIO_VOLUMES env
    if set, else the ExecStart positional argument, in that order)
    MUST equal /globular/objectstore/config.node_paths[<this_node_ip>].
    Diagnosing the runtime path requires checking BOTH the env file
    AND the positional argument — relying on the ExecStart text alone
    misleads when MINIO_VOLUMES is also set.
```

## Status

Inventory complete. No changes made. Awaiting operator decision on:
- Decision 1 (unit-file cosmetic)
- Decision 2 (stale `/mnt/...` archival)
- Decision 3 (quorum recovery and downstream topology plan)

Project F (controller stale-DEGRADED-label refresh) is now unblocked
by this corrected inventory — the topology path is not in actual
critical mismatch, so reconcileAvailable can safely promote MinIO from
DEGRADED to AVAILABLE on runtime health evidence alone (subject to
Project F's own design constraints).
