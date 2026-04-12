# Compute Phase 2 Baseline

## What is implemented

The compute subsystem is a workflow-driven distributed execution engine deployed across 3 nodes. It handles the full lifecycle of compute jobs: definition → submission → partitioning → dispatch → execution → verification → aggregation → terminal state.

### Core components

| Component | File | Role |
|-----------|------|------|
| ComputeService | handlers.go | Job/definition CRUD, submission |
| ComputeRunnerService | runner.go | Staging, execution, heartbeat, cancel |
| WorkflowActorService | actor_service.go | Receives workflow engine callbacks |
| Partition planner | partition_planner.go | Splits jobs into units |
| Remote runner | remote_runner.go | gRPC client for cross-node dispatch |
| MinIO staging | minio_staging.go | Input fetch, output upload |
| Verifier | verifier.go | CHECKSUM and structural verification |
| Retry policy | retry_policy.go | Bounded retries with failure classification |
| Workflow dispatch | workflow_dispatch.go | Publishes definitions, calls workflow engine |

### Orchestration path

```
SubmitComputeJob
  → planPartitions (if partitionable)
  → persist units to etcd
  → executeViaWorkflow
    → WorkflowService.ExecuteWorkflow("compute.job.submit")
      → compute.load_job
      → compute.validate_job_definition
      → compute.admit_job
      → compute.create_single_unit (finds existing units)
      → compute.dispatch_all_units
        → for each unit: choose node → grant lease → stage → run
      → compute.await_all_units
        → poll unit states, evaluate retry policy on failures
      → compute.assess_unit_outcomes
      → compute.create_result (verification + aggregate manifest)
      → compute.finalize_job
```

One workflow path. No inline shortcuts. No hidden orchestration.

## What was validated live

All validation ran on the 3-node cluster (globule-dell, globule-nuc, globule-ryzen).

| Test | Definition | Units | Result |
|------|-----------|-------|--------|
| Single-unit success | e2e-test | 1 | JOB_COMPLETED, STRUCTURALLY_VERIFIED |
| Multi-unit success | multi-unit-test | 3 | 3x UNIT_SUCCEEDED, JOB_COMPLETED |
| Cross-node dispatch | multi-unit-test | 3 | Units on 10.0.0.20, 10.0.0.8, 10.0.0.63 |
| Retryable failure | retry-test (NON_DETERMINISTIC) | 1 | attempt 1→2→3, max reached |
| Non-retryable failure | deterministic-fail (DETERMINISTIC) | 1 | attempt 1, no retry |
| Real ffmpeg transcode | media-transcode | 1 | 81KB test-tone.mp3 in MinIO |
| Real backup snapshot | backup-snapshot | 1 | Config snapshot JSON in MinIO |
| Cancellation | CancelComputeJob | 1 | JOB_CANCELLED, process killed |

## Proven use cases

### media-transcode@1.0.0
- SINGLE_NODE, DETERMINISTIC
- Transcodes video to 720p MP4 or audio to 192k MP3 via ffmpeg
- Generates test tone when no input provided
- Output uploaded to MinIO with SHA-256 checksum

### backup-snapshot@1.0.0
- PARTITIONABLE_BATCH, NON_DETERMINISTIC, count:3
- Creates per-node config snapshots as tar.gz + metadata JSON
- Designed for parallel execution — one snapshot per node

## Out of scope for Phase 3

These are not planned next:
- GPU scheduling
- Speculative execution
- Complex balancing heuristics (LOCALITY_FIRST, etc.)
- VM/OCI/WASM runtimes
- Multi-stage pipelines
- Streaming/windowed compute

## Known limitations

1. **Aggregate result** uses last unit's output ref for single-unit jobs; multi-unit produces a manifest but does not merge data
2. **Profile-based filtering** reads node profiles from etcd but currently falls back to all nodes when no match
3. **Workflow definitions** published on startup via MinIO, not via the package pipeline
4. **No resource reservation** — leases track ownership but don't reserve CPU/memory
5. **Heartbeat progress** always reports 0.0 — entrypoints don't update progress
6. **etcdctl snapshot** blocked by user permissions in backup-snapshot (config snapshot works)
