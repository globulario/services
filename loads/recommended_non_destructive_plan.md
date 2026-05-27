# Recommendation

MANUAL_OBJECTSTORE_RECOVERY_REQUIRED

## Why this classification

- etcd objectstore contract (`/globular/objectstore/config`) currently declares a 4-node distributed set: `10.0.0.63, 10.0.0.20, 10.0.0.8, 10.0.0.9`.
- On-disk distributed MinIO `format.json` on reachable storage nodes (`10.0.0.8`, `10.0.0.20`, `10.0.0.9`) consistently declares a 5-member deployment (`deployment id 443db6b4-5dbe-4ead-9cb9-143c0c80cbed`).
- Runtime commands are split (3/4 endpoint launches), and `10.0.0.63` fails to start with fatal mismatch: configured 4 vs metadata 5.
- `10.0.0.102` is unreachable and remains a suspected required member of the active 5-member deployment.

This satisfies stop conditions:

- `etcd contract disagrees with format.json`
- `10.0.0.102 may be required by existing deployment but unreachable`
- `repair would require topology transition logic; manual env edits alone are unsafe`

## Non-destructive recovery sequence

1. Freeze repository import operations (`globular repository sync` remains blocked).
2. Capture full `format.json` from all five nodes, including `10.0.0.63` and `10.0.0.102` once reachable, to map member-ID to node-IP deterministically.
3. Determine authoritative universe:
   - If 5-member deployment is authoritative, restore `10.0.0.102` reachability first and relaunch all MinIO nodes on the same 5-endpoint set.
   - If 4-member target is intended, create and execute an explicit approved topology transition workflow from 5 -> 4; do not hand-edit envs.
4. Render MinIO env/drop-ins from the chosen authoritative contract on every storage node, then rolling restart MinIO.
5. Verify convergence criteria before repository retry:
   - identical endpoint count in runtime on every storage node
   - MinIO health initialized (no bootstrap mismatch, no `drive is already being used in another erasure deployment`)
   - repository no longer logs degraded mirror warnings

## Explicitly prohibited during this phase

- deleting or rewriting `format.json`
- MinIO reinitialize/wipe
- endpoint-count reduction by manual env edits
- retrying etcd import or broad repository sync
