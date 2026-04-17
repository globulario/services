# Computing on Globular

Globular includes a distributed compute service for running batch jobs, data processing pipelines, and parallelizable workloads across cluster nodes. Jobs are submitted as definitions with inputs, executed through the workflow engine, and produce verified outputs stored in MinIO.

This page covers the compute architecture, job types, submission and execution model, placement and scheduling, failure recovery, and output verification.

## Why a Compute Service

Many distributed systems need to run computational workloads beyond serving gRPC requests: video transcoding, data transformation, report generation, machine learning inference, bulk imports, and index rebuilding. These workloads share common requirements:

- **Distribution**: Split work across multiple nodes for parallel execution
- **Fault tolerance**: Retry failed units without restarting the entire job
- **Verification**: Confirm outputs are correct before marking the job complete
- **Observability**: Track progress, resource usage, and failure reasons
- **Scheduling**: Place work on nodes with appropriate resources

Globular's compute service provides all of this, integrated with the platform's workflow engine, MinIO storage, and etcd state management.

## Core Concepts

### Compute Definitions

A **compute definition** declares what to run. It's registered once and reused across job submissions:

```
ComputeDefinition {
  name: "ffmpeg_transcode"
  version: "1.0.0"
  artifact_uri: "s3://compute/ffmpeg_transcode_1.0.0.tar.gz"
  artifact_sha256: "abc123..."
  entrypoint: "/app/transcode.sh"
  kind: PARTITIONABLE_BATCH
  runtime_type: NATIVE_BINARY
  resource_profile: { min_cpu_millis: 2000, min_memory_bytes: 4GB }
  partition_strategy: { type: "per_input" }
  verify_strategy: { type: CHECKSUM }
  idempotency_mode: SAFE_RETRY
  determinism_level: DETERMINISTIC
}
```

**Key fields**:
- `artifact_uri` / `artifact_sha256`: Where to fetch the executable, with integrity verification
- `entrypoint`: The command to run (can be a binary, script, or any executable)
- `kind`: Job type (see below)
- `runtime_type`: Execution environment — `NATIVE_BINARY`, `SCRIPTED`, `WASM`, `OCI`, `VM`
- `resource_profile`: CPU, memory, and disk requirements for node placement
- `partition_strategy`: How to split inputs across parallel units
- `verify_strategy`: How to verify outputs (checksum, schema validation, or custom)
- `idempotency_mode`: Whether the job is safe to retry (`SAFE_RETRY`, `NO_AUTOMATIC_RETRY`, `APPEND_ONLY`)
- `determinism_level`: Whether re-execution produces identical output (`DETERMINISTIC`, `NON_DETERMINISTIC`)

### Job Types

| Kind | Description | Units | Example |
|------|-------------|-------|---------|
| `SINGLE_NODE` | Run entrypoint once on one node | 1 | Report generation, single-file processing |
| `PARTITIONABLE_BATCH` | Split inputs into N partitions, execute in parallel | N (one per partition) | Video transcoding (one unit per video file) |
| `MAP_REDUCE` | Map phase + reduce phase with aggregation | N mappers + aggregation | Log analysis, word count |
| `PIPELINE` | Multi-stage DAG with data flow between stages | Varies | ETL pipelines, multi-step transformations |
| `AGGREGATION` | Collect and merge results from other jobs | 1 | Final merge of distributed computation |
| `GPU_BATCH` | GPU-aware scheduling | N | ML inference, rendering |
| `STREAM_WINDOWED` | Reserved for streaming workloads | Varies | Real-time data processing |

### Jobs, Units, and Partitions

Every submission creates a **job**. The job is broken into one or more **units**:

```
Job (one per submission)
├── Unit 1 (partition 1) → executes on node-1
├── Unit 2 (partition 2) → executes on node-2
└── Unit 3 (partition 3) → executes on node-3
```

- **Single-unit jobs** (`SINGLE_NODE`): One job = one unit = one execution
- **Multi-unit jobs** (`PARTITIONABLE_BATCH`, `MAP_REDUCE`): One job = N units, each processing a partition of the input

### Content-Addressed Storage

All inputs and outputs use **content addressing** via ObjectRef:

```
ObjectRef {
  uri: "compute/inputs/video1.mp4"     # MinIO object key
  sha256: "a1b2c3d4..."                # Content hash
  size_bytes: 104857600                 # Size for quota tracking
  metadata: { "format": "mp4" }        # Arbitrary metadata
}
```

etcd stores only ObjectRef pointers (metadata). All binary data lives in MinIO. This separation ensures etcd stays small while supporting arbitrarily large inputs and outputs.

## Job Lifecycle

### State Machine

A job progresses through these states:

```
PENDING → ADMITTED → PARTITIONING → QUEUED → RUNNING →
    AGGREGATING → VERIFYING → COMPLETED / DEGRADED / FAILED / CANCELLED
```

**PENDING**: Job submitted, awaiting admission.
**ADMITTED**: Job accepted, resources validated.
**PARTITIONING**: Inputs being split into units (multi-unit jobs).
**QUEUED**: Units created, waiting for node placement.
**RUNNING**: At least one unit is executing.
**AGGREGATING**: All units completed, merging results.
**VERIFYING**: Output verification in progress.
**COMPLETED**: All units succeeded, output verified.
**DEGRADED**: Some units succeeded, some failed. Partial output available.
**FAILED**: All units failed or a critical failure occurred.
**CANCELLED**: Job cancelled by operator.

### Unit State Machine

Each unit independently progresses:

```
PENDING → RESERVED → ASSIGNED → STAGING → RUNNING →
    UPLOADING → VERIFYING → SUCCEEDED / FAILED / CANCELLED
```

**PENDING**: Unit created, awaiting node assignment.
**RESERVED**: Resources reserved on target node.
**ASSIGNED**: Node selected, execution lock acquired.
**STAGING**: Inputs being downloaded from MinIO to the node.
**RUNNING**: Entrypoint executing.
**UPLOADING**: Execution complete, output being uploaded to MinIO.
**VERIFYING**: Output verification in progress.
**SUCCEEDED**: Execution and verification passed.
**FAILED**: Execution failed or verification failed.

## Submitting Jobs

### Register a Definition

Before submitting jobs, register the compute definition:

```bash
# Register via gRPC (programmatic)
# ComputeService.RegisterComputeDefinition(...)

# Or via CLI
globular compute definition register \
  --name ffmpeg_transcode \
  --version 1.0.0 \
  --artifact s3://compute/ffmpeg_transcode_1.0.0.tar.gz \
  --entrypoint /app/transcode.sh \
  --kind PARTITIONABLE_BATCH \
  --cpu 2000 --memory 4GB
```

### Submit a Job

```bash
globular compute submit \
  --definition ffmpeg_transcode \
  --version 1.0.0 \
  --input "s3://data/video1.mp4,sha256:abc123..." \
  --input "s3://data/video2.mp4,sha256:def456..." \
  --parameters '{"bitrate": "5000k", "format": "h264"}' \
  --parallelism 2 \
  --priority high
```

What happens internally:

1. **Validation**: Compute service verifies the definition exists and matches the request
2. **Job creation**: A `ComputeJob` record is written to etcd at `/globular/compute/jobs/{job_id}` with state `JOB_PENDING`
3. **Admission**: Job transitions to `JOB_ADMITTED` after resource validation
4. **Partitioning**: For multi-unit jobs, a partition plan is created (one unit per input in this case)
5. **Workflow dispatch**: The `compute.job.submit` workflow is started via the workflow engine
6. **Unit dispatch**: Each unit gets its own `compute.unit.execute` workflow
7. **Execution**: Units run in parallel on assigned nodes
8. **Aggregation**: After all units complete, results are merged
9. **Finalization**: Job transitions to `COMPLETED`, `DEGRADED`, or `FAILED`

### Monitor Job Progress

```bash
# List jobs
globular compute jobs list
# JOB ID       DEFINITION          STATUS     UNITS    PROGRESS
# job-abc123   ffmpeg_transcode    RUNNING    2/2      65%

# Get job details
globular compute jobs get job-abc123
# Shows: state, units with individual progress, timing, errors

# Get job result (after completion)
globular compute result job-abc123
# Shows: output ObjectRef, trust level, checksums
```

### Cancel a Job

```bash
globular compute jobs cancel job-abc123
```

This sends a cancellation signal to all running units. The runner terminates the entrypoint process, and units transition to `CANCELLED`.

## Execution Model

### Workflow-Driven Orchestration

All compute execution goes through the Globular workflow engine. Three workflows orchestrate job execution:

**compute.job.submit** — Main orchestration:
```
load_job → validate_definition → admit_job → create_units →
    dispatch_all_units → await_all_units → assess_units →
    create_result → finalize_job
```

**compute.unit.execute** — Per-unit execution:
```
choose_node → mark_unit_assigned → stage_unit → run_unit →
    await_unit_terminal
```

**compute.job.aggregate** — Result aggregation:
```
assess_units → create_result → finalize_job
```

Every workflow step is **idempotent** — if the workflow engine crashes and replays a step, the same result is produced without side effects. This is the same workflow engine used for service deployment, so compute jobs get the same failure handling, retry logic, and audit trail.

### Input Staging

When a unit is assigned to a node, the compute runner stages inputs:

1. Resolves the MinIO endpoint from etcd
2. Downloads each input ObjectRef from MinIO to the staging directory: `{staging_path}/inputs/`
3. Verifies each download's SHA256 checksum against the ObjectRef
4. Downloads the compute artifact (executable) from MinIO
5. Verifies the artifact's checksum
6. Extracts the artifact to `{staging_path}/artifact/` (if tarball)

If any download fails or checksum mismatches, the unit fails with `ARTIFACT_FETCH_FAILED` or `INPUT_MISSING`.

### Entrypoint Execution

The runner executes the entrypoint as a subprocess:

```bash
cd {staging_path}
COMPUTE_JOB_ID=job-abc123 \
COMPUTE_UNIT_ID=unit-def456 \
COMPUTE_STAGING_PATH=/var/lib/globular/compute/staging/unit-def456 \
COMPUTE_PARAMETERS_JSON='{"bitrate":"5000k","format":"h264"}' \
/app/transcode.sh
```

The entrypoint:
- Reads inputs from `{COMPUTE_STAGING_PATH}/inputs/`
- Reads parameters from `COMPUTE_PARAMETERS_JSON`
- Writes outputs to `{COMPUTE_STAGING_PATH}/outputs/`
- Optionally writes progress to `{COMPUTE_STAGING_PATH}/progress.json`:
  ```json
  {"progress": 0.65, "message": "Transcoding frame 15000/23000", "items_done": 15000, "items_total": 23000}
  ```
- Exits with status 0 (success) or non-zero (failure)

### Heartbeats

While a unit is running, the runner periodically reports heartbeats to the compute service:

```
ReportComputeHeartbeat {
  unit_id: "unit-def456"
  progress: 0.65
  message: "Transcoding frame 15000/23000"
  stdout_bytes: 4096
  stderr_bytes: 0
  custom_metrics: {"fps": 120, "bitrate_actual": "4800k"}
}
```

The compute service responds with `{ok: true, should_cancel: false}`. If the job has been cancelled, the response includes `should_cancel: true`, and the runner terminates the entrypoint.

### Output Commit

After the entrypoint exits:

1. Runner computes SHA256 of all output files
2. Uploads outputs to MinIO
3. Calls `CommitComputeOutput` with the output ObjectRef, exit status, and checksum
4. Compute service verifies the output (if verification strategy is configured)
5. Unit transitions to `SUCCEEDED` or `FAILED`

## Node Placement

### How Nodes Are Selected

The placement engine selects nodes in two phases:

**Phase 1: Hard Filters** — eliminate unsuitable nodes:
- Node must have required profiles (e.g., `compute`, `gpu`)
- Node must meet minimum resource requirements (CPU, memory, disk)
- Node must be reachable (healthy heartbeat)

**Phase 2: Scoring** — rank remaining nodes:
- Calculate current load (count of active units: PENDING/ASSIGNED/STAGING/RUNNING)
- Calculate available capacity (total resources minus reserved)
- Score = (available_capacity / node_capacity) − (load × priority_factor)
- Higher-priority jobs get a scoring boost (tolerate more loaded nodes)
- Deterministic tie-breaking via round-robin counter

### Placement Policies

| Policy | Behavior |
|--------|----------|
| `PACK` | Fill nodes densely before spreading |
| `SPREAD` | Distribute across nodes evenly |
| `LOCALITY_FIRST` | Co-locate with input data |
| `LOWEST_LOAD` | Choose the least-loaded node |
| `GPU_AWARE` | GPU-optimized scheduling |
| `LATENCY_SENSITIVE` | Prefer low-latency nodes |
| `THROUGHPUT_MAX` | Maximize aggregate throughput |

### Priority Classes

| Priority | Value | Behavior |
|----------|-------|----------|
| Low | 1 | Scheduled after higher-priority jobs |
| Normal | 5 | Default priority |
| High | 8 | Scheduling boost, tolerate loaded nodes |
| Critical | 10 | Maximum scheduling priority |

## Failure Handling

### Retry Policy

The compute service decides whether to retry a failed unit based on three factors:

**1. Idempotency mode** (from definition):
- `SAFE_RETRY` (default): Retry if failure is transient
- `NO_AUTOMATIC_RETRY`: Never retry — operator must intervene
- `RETRY_WITH_CLEANUP`: Retry with cleanup of previous state
- `APPEND_ONLY`: Safe to retry (idempotent append operations)

**2. Failure classification**:

| Failure Class | Retryable | Description |
|--------------|-----------|-------------|
| `NODE_UNREACHABLE` | Yes | Network failure to runner |
| `LEASE_EXPIRED` | Yes | Unit lost exclusive lock |
| `RESOURCE_EXHAUSTED` | Yes | OOM or disk full |
| `INPUT_MISSING` | Yes | Input not found (may be transient) |
| `ARTIFACT_FETCH_FAILED` | Yes | Artifact download failed |
| `OUTPUT_UPLOAD_FAILED` | Yes | MinIO upload failed |
| `EXECUTION_NONZERO_EXIT` | Depends | If DETERMINISTIC → no; if NON_DETERMINISTIC → yes |
| `OUTPUT_VERIFICATION_FAILED` | Depends | If DETERMINISTIC → no; if NON_DETERMINISTIC → yes |
| `DETERMINISTIC_REPEAT_FAILURE` | No | Same failure will recur |
| `POLICY_BLOCKED` | No | Policy violation |

**3. Attempt count**: Maximum 3 attempts per unit (configurable). After max attempts, the unit enters FAILED state permanently.

**Key insight**: For deterministic jobs, a non-zero exit code will produce the same failure on retry — so the system doesn't retry. For non-deterministic jobs (e.g., jobs depending on external state), retry may succeed.

### Job-Level Failure Assessment

After all units reach terminal state, the compute service assesses the outcome:

- **All succeeded** → `JOB_COMPLETED`
- **Some succeeded, some failed** → `JOB_DEGRADED` (partial output available)
- **All failed** → `JOB_FAILED`

Degraded jobs produce partial results. The result record indicates which partitions succeeded and which failed, allowing the operator to resubmit only the failed partitions.

## Output Verification

### Verification Strategies

| Strategy | How It Works | Trust Level |
|----------|-------------|-------------|
| None | No verification | `UNVERIFIED` |
| `CHECKSUM` | Compare output SHA256 against expected values | `CONTENT_VERIFIED` |
| `SCHEMA_VALIDATE` | Check output files exist and are non-empty | `STRUCTURALLY_VERIFIED` |
| `CUSTOM_VERIFIER` | Run a custom verification script | Depends on result |

### Trust Levels

| Level | Meaning |
|-------|---------|
| `UNVERIFIED` | No verification was performed |
| `STRUCTURALLY_VERIFIED` | Output exists and has expected structure |
| `CONTENT_VERIFIED` | Checksum or schema validation passed |
| `FULLY_REPRODUCED` | Bitwise-identical output from independent re-execution |
| `DEGRADED_ACCEPTED` | Verification failed but output accepted anyway |

## Execution Guarantees

**Idempotency**: All workflow steps are idempotent. Replaying a step after a crash produces the same result. Partition plans are deterministic. Placement uses round-robin tie-breaking for reproducibility.

**Ordering**: Job → Units → Execution follows a strict DAG. Multi-unit dispatch is parallel. Aggregation only starts after all units reach terminal state.

**Atomicity**: Unit success/failure is atomic via `CommitComputeOutput`. Job state transitions are atomic via etcd writes. Output uploads to MinIO complete before the result is committed.

**Durability**: All state is stored in etcd (replicated, persisted). No in-memory-only state. Jobs can be inspected and resumed after service restart.

**Exclusivity**: Each unit has an etcd lease for exclusive ownership. If the runner crashes, the lease expires and the unit can be retried on another node. This prevents double-execution.

## Practical Scenarios

### Scenario 1: Video Transcoding Pipeline

Transcode 100 video files across a 5-node cluster:

```bash
# 1. Register the definition
globular compute definition register \
  --name ffmpeg_transcode --version 1.0.0 \
  --artifact s3://compute/ffmpeg_1.0.0.tar.gz \
  --entrypoint /app/transcode.sh \
  --kind PARTITIONABLE_BATCH \
  --partition-strategy per_input \
  --cpu 4000 --memory 8GB

# 2. Upload input files to MinIO
# (100 video files, each as an ObjectRef)

# 3. Submit the job
globular compute submit \
  --definition ffmpeg_transcode --version 1.0.0 \
  --inputs-from manifest.json \
  --parameters '{"codec": "h265", "bitrate": "3000k"}' \
  --parallelism 20 \
  --priority high

# 4. Monitor
globular compute jobs list
# Shows: 100 units, progress across nodes

# 5. Get results
globular compute result <job-id>
# Shows: 100 output ObjectRefs with checksums
```

The compute service creates 100 units (one per video), places them across 5 nodes (up to 20 concurrent based on parallelism), and produces 100 transcoded outputs.

### Scenario 2: Handling Partial Failure

3 of 100 units fail due to corrupt input files:

```bash
globular compute jobs get <job-id>
# Status: DEGRADED
# Units: 97 succeeded, 3 failed
# Failed units:
#   unit-001: EXECUTION_NONZERO_EXIT (corrupt input: video47.mp4)
#   unit-002: EXECUTION_NONZERO_EXIT (corrupt input: video82.mp4)
#   unit-003: RESOURCE_EXHAUSTED (OOM on node-3)

# Unit-003 was retried (RESOURCE_EXHAUSTED is retryable)
# but failed again after 3 attempts

# Get the partial result (97 successful outputs)
globular compute result <job-id>
# trust_level: DEGRADED_ACCEPTED
# 97 output ObjectRefs available
```

The operator can fix the corrupt inputs and resubmit only the failed partitions.

### Scenario 3: GPU-Accelerated ML Inference

```bash
# Register a GPU-aware definition
globular compute definition register \
  --name ml_inference --version 2.0.0 \
  --artifact s3://compute/ml_inference_2.0.0.tar.gz \
  --entrypoint /app/infer.py \
  --kind GPU_BATCH \
  --placement GPU_AWARE \
  --gpu 1 --memory 16GB

# Submit
globular compute submit \
  --definition ml_inference --version 2.0.0 \
  --input "s3://data/images.tar.gz,sha256:..." \
  --parameters '{"model": "resnet50", "batch_size": 32}' \
  --priority critical
```

The placement engine selects only nodes with GPU profiles, scoring by GPU availability.

## Failure Scenarios

### Artifact fetch failure

**Symptom**: Unit stuck in STAGING, then fails with `ARTIFACT_FETCH_FAILED`.
**Cause**: MinIO unreachable, artifact deleted, or checksum mismatch.
**Fix**: Check MinIO health. Verify the artifact exists: `globular pkg info <artifact>`. Re-upload if corrupt.

### Runner node goes down during execution

**Symptom**: Unit stays in RUNNING with no heartbeats. Eventually lease expires.
**Cause**: Node crash, network partition, or runner process killed.
**Fix**: Automatic — the lease expires, and the unit is retried on another node (if retries remain and the idempotency mode allows).

### Job stuck in AGGREGATING

**Symptom**: All units completed but job doesn't transition to COMPLETED.
**Cause**: Aggregation workflow step failed (MinIO upload of aggregate manifest failed).
**Fix**: Check workflow: `globular workflow list --correlation "compute/job/<job-id>"`. Check MinIO health.

### Output verification fails

**Symptom**: Unit execution succeeds (exit 0) but unit marked FAILED with `OUTPUT_VERIFICATION_FAILED`.
**Cause**: Output checksum doesn't match expected value in the definition.
**Fix**: If the definition is wrong, update it. If the output is genuinely bad, the entrypoint has a bug. For non-deterministic jobs, the system auto-retries.

## What's Next

- [Workflows](operators/workflows.md): How the workflow engine orchestrates compute jobs
- [Services and Packages](operators/services-and-packages.md): Package the compute runner for deployment
- [Observability](operators/observability.md): Monitor compute job metrics and logs
