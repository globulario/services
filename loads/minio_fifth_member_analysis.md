# MinIO Fifth Member Analysis

## Question
Which node is the 5th member in the active distributed MinIO metadata universe?

## Evidence

1. Distributed `format.json` on reachable nodes (`10.0.0.8`, `10.0.0.20`, `10.0.0.9`) shares the same deployment id:
- `443db6b4-5dbe-4ead-9cb9-143c0c80cbed`
- set size: `5`

2. etcd `format_backup` records include a backup for node id `ffc29469-52d7-5929-844a-3a8d3a627d7c`:
- key: `/globular/objectstore/format_backup/ffc29469-52d7-5929-844a-3a8d3a627d7c/data`
- includes same deployment id `443db6b4-5dbe-4ead-9cb9-143c0c80cbed`

3. etcd disk admission record maps this node id to IP `10.0.0.102`:
- key: `/globular/objectstore/disk/admitted/ffc29469-52d7-5929-844a-3a8d3a627d7c/...`
- `node_ip: 10.0.0.102`

4. etcd topology proposal/transition explicitly lists 5 nodes including `10.0.0.102`.

## Conclusion
The fifth MinIO member is **10.0.0.102 (globule-lenovo)**.

This is not a stale anonymous disk ID; it is explicitly represented in objectstore admission, topology, and format backup records.
