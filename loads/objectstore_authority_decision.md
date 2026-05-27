# Objectstore Authority Decision

## Competing sources of truth

1. etcd contract `/globular/objectstore/config`
- currently 4 nodes (`10.0.0.63, 10.0.0.20, 10.0.0.8, 10.0.0.9`)

2. MinIO distributed metadata (`format.json`) + etcd format backups
- consistently 5-member deployment id `443db6b4-5dbe-4ead-9cb9-143c0c80cbed`
- includes node mapped to `10.0.0.102`

## Transition evidence

Found approved topology transition metadata:
- `/globular/objectstore/topology/transition/1` (`approved: true`)
- affected nodes include all 5 members including `10.0.0.102`
- destructive standalone->distributed transition record exists

Found no explicit approved record proving a later **5 -> 4** shrink/decommission.

## Decision
Current authoritative runtime universe should be treated as the **5-member MinIO metadata universe** until a verified, approved 5->4 transition exists and is completed.

Rationale:
- MinIO startup enforces on-disk metadata and fails hard when configured for 4.
- Existing distributed metadata and backups are internally consistent for 5.
- etcd 4-node contract appears drifted from active MinIO deployment metadata.

## Required recommendation (single)
**RESTORE_UNREACHABLE_NODE_FIRST**
