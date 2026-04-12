# Compute Phase 2 Baseline

## Status: Validated and Deployed

Distributed compute with partitioned multi-unit jobs, cross-node dispatch,
bounded retries, MinIO staging, verification, and aggregation.

## Capabilities

- Workflow-driven orchestration (single path, no shortcuts)
- Partition planner: `per_input`, `count` strategies
- Cross-node dispatch via round-robin on direct service ports
- etcd TTL leases (30s) with heartbeat renewal (5s)
- MinIO input fetch + output upload with SHA-256 checksums
- Verification: CHECKSUM (content match) and SCHEMA_VALIDATE (structural)
- Bounded retries (max 3) with failure classification
- Aggregate result with per-unit checksums collected
- Real cancellation (kills process, revokes lease)
- Deployed on 3 nodes via normal package pipeline

## Validated Workloads

| Definition | Kind | What it does |
|-----------|------|-------------|
| `media-transcode@1.0.0` | SINGLE_NODE | ffmpeg transcode (video→720p MP4, audio→MP3) |
| `backup-snapshot@1.0.0` | PARTITIONABLE_BATCH | Per-node config snapshots to MinIO |

## Guarantees

- No infinite retry loops (max 3 attempts, hard cap)
- Deterministic definitions never retry on execution failure
- Job terminal state is truthful (COMPLETED, FAILED, CANCELLED)
- etcd stores only metadata — blobs go to MinIO
- One workflow path — no hidden inline shortcuts

## Limitations

- No adaptive scheduling / load-aware placement
- No speculative execution
- No advanced merge (aggregate uses last output ref)
- Profile-based placement not yet enforced
- Workflow definitions published on startup, not via package pipeline

## Demo: ffmpeg transcode

```bash
# Register
grpc compute.ComputeService RegisterComputeDefinition \
  definition.name=media-transcode definition.version=1.0.0 \
  definition.entrypoint=/usr/local/bin/compute-transcode.sh \
  definition.runtime_type=NATIVE_BINARY definition.kind=SINGLE_NODE

# Submit (no input = generates test tone)
grpc compute.ComputeService SubmitComputeJob \
  spec.definition_name=media-transcode spec.definition_version=1.0.0

# Check result
grpc compute.ComputeService GetComputeResult job_id=<job_id>
# → resultRef.uri = minio://globular/.../test-tone.mp3
# → trustLevel = STRUCTURALLY_VERIFIED
```

## Demo: backup snapshot (partitioned)

```bash
# Register
grpc compute.ComputeService RegisterComputeDefinition \
  definition.name=backup-snapshot definition.version=1.0.0 \
  definition.entrypoint=/usr/local/bin/compute-backup-snapshot.sh \
  definition.runtime_type=NATIVE_BINARY definition.kind=PARTITIONABLE_BATCH \
  definition.partition_strategy.type=count definition.partition_strategy.unit=3

# Submit with parallelism=3 (one unit per node)
grpc compute.ComputeService SubmitComputeJob \
  spec.definition_name=backup-snapshot spec.definition_version=1.0.0 \
  spec.desired_parallelism=3

# Check units (should show 3 partitions across nodes)
grpc compute.ComputeService ListComputeUnits job_id=<job_id>

# Check aggregate result (checksums from all units)
grpc compute.ComputeService GetComputeResult job_id=<job_id>
```
