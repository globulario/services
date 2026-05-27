# Manual Objectstore Recovery Plan

## Recommendation
RESTORE_UNREACHABLE_NODE_FIRST

## Objective
Restore one coherent 5-member MinIO topology first, then reconcile etcd contract to that reality.

## Guardrails
- Do not wipe MinIO data.
- Do not delete/rewrite `format.json`.
- Do not run repository import.
- Do not force 5->4 by env edits only.

## Plan

1. Restore node `10.0.0.102` network/host reachability.
- recover L2/L3 path, host power/state, SSH, node-agent (11000), MinIO (9000)
- confirm `/mnt/data/data/.minio.sys/format.json` exists and matches deployment id `443db6b4-...`

2. Render uniform 5-endpoint MinIO configuration on all 5 storage nodes.
- endpoint set: `10.0.0.63, 10.0.0.102, 10.0.0.20, 10.0.0.8, 10.0.0.9`
- ensure service drop-ins/env are identical in endpoint count/order policy

3. Rolling restart MinIO across all 5 nodes.
- validate each node reports same endpoint count and no bootstrap mismatch
- ensure no `Number of drives specified: 4 ... format.json: 5` errors

4. Verify backend health.
- MinIO health initialized on all intended nodes
- repository stops reporting mirror unavailable/degraded write-unsafe mode

5. Reconcile etcd objectstore contract to match healthy runtime metadata.
- update `/globular/objectstore/config` to the recovered 5-member truth if still drifted
- record/review transition/audit metadata

6. Only after above: reopen repository import chain.

## Branch if 10.0.0.102 cannot be restored
If node remains unavailable after infra recovery attempts, stop and create an explicit approved 5->4 topology transition workflow (no manual metadata edits), then execute transition and re-verify.
