# Demo: Backup Snapshot

## Prerequisites

- Compute service running on all 3 nodes
- Script `/usr/local/bin/compute-backup-snapshot.sh` deployed to all compute nodes
- `/var/lib/globular/services/` readable by `globular` user

## Register the definition

```bash
grpc_call compute.ComputeService RegisterComputeDefinition '{
  "definition": {
    "name": "backup-snapshot",
    "version": "1.0.0",
    "entrypoint": "/usr/local/bin/compute-backup-snapshot.sh",
    "runtime_type": 1,
    "kind": 2,
    "partition_strategy": {"type": "count", "unit": "3"},
    "determinism_level": 4,
    "idempotency_mode": 1,
    "verify_strategy": {"type": 2}
  }
}'
```

## Submit a partitioned job (3 nodes)

```bash
grpc_call compute.ComputeService SubmitComputeJob '{
  "spec": {
    "definition_name": "backup-snapshot",
    "definition_version": "1.0.0",
    "desired_parallelism": 3,
    "tags": ["backup"]
  }
}'
# Returns: job_id
```

## Watch progress

```bash
# Job state
grpc_call compute.ComputeService GetComputeJob '{"job_id": "<JOB_ID>"}'

# All units (should show 3 with partition IDs)
grpc_call compute.ComputeService ListComputeUnits '{"job_id": "<JOB_ID>"}'
```

### Expected unit progression

```
UNIT_PENDING → UNIT_ASSIGNED → UNIT_STAGING → UNIT_RUNNING → UNIT_SUCCEEDED
```

Each unit shows:
- `partitionId`: part-0, part-1, part-2
- `nodeId`: service instance ID (distinct per node)
- `leaseOwner`: node + lease ID
- `outputRef`: MinIO URI + checksum

## Inspect result

```bash
grpc_call compute.ComputeService GetComputeResult '{"job_id": "<JOB_ID>"}'
```

### Expected output (3-unit success)

```json
{
  "resultRef": {
    "uri": "minio://globular/.../aggregate/output/manifest.json",
    "sha256": "<hex>",
    "sizeBytes": "<varies>"
  },
  "checksums": ["<unit0-sha256>", "<unit1-sha256>", "<unit2-sha256>"],
  "trustLevel": "STRUCTURALLY_VERIFIED"
}
```

The aggregate manifest.json contains:

```json
{
  "job_id": "<JOB_ID>",
  "completed_at": "2026-04-12T...",
  "unit_count": 3,
  "units": [
    {
      "unit_id": "...",
      "partition_id": "part-0",
      "node_id": "...",
      "state": "UNIT_SUCCEEDED",
      "output_uri": "minio://globular/.../snapshot-...-globule-dell.json",
      "checksum": "<hex>",
      "size_bytes": 247
    },
    ...
  ]
}
```

## What each unit produces

Per node:
- `snapshot-<timestamp>-<hostname>-configs.tar.gz` — service config files
- `snapshot-<timestamp>-<hostname>.json` — metadata with node, timestamp, file count

## Failure signals

| Signal | Meaning |
|--------|---------|
| Only 1 unit created | Gateway routed to node with old binary — redeploy |
| `UNIT_RUNNING` for >30s | Script may be hanging (check service logs) |
| `JOB_FAILED` + "one or more units failed" | Check individual unit failure_reason |
| All units on same nodeId | Endpoint resolution collapsed — check resolveComputeNodes |

## Single-unit fallback

If `desired_parallelism` is omitted or set to 1, the job runs as a single unit on one node (no partitioning). The backup captures that one node's config only.
