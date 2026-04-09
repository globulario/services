# Globular Distributed Compute Subsystem

## 1. Purpose

The Compute subsystem enables Globular to execute distributed workloads across the cluster in a **reproducible, observable, and workflow-driven manner**.

It allows Globular to:

- Define reproducible compute tasks
- Understand inputs, outputs, and execution requirements
- Partition work into parallel units (when applicable)
- Schedule execution across nodes
- Track execution via workflow telemetry
- Retry, resume, and verify results
- Aggregate outputs deterministically

> This is NOT a "single program running across multiple machines"
>
> It is a **workflow-driven distributed execution fabric**

---

## 2. Core Principle

Globular does not execute arbitrary commands as its source of truth.

Instead, all computation is defined through:

### Compute Definitions

A Compute Definition declares:

- What the task is
- How it can be partitioned
- Required resources
- Input and output contracts
- Verification strategy
- Merge/aggregation logic
- Retry safety and determinism

> Compute = **Declared Intent**, not ad hoc execution

---

## 3. Architecture Overview

### Subsystem Name

**Service:** `compute`

### Core Components

#### A. Compute Service (Control Plane)

Owns:

- Compute definitions
- Job submission and admission
- Partition planning
- Scheduling decisions
- Job lifecycle state
- Result aggregation metadata
- Telemetry publication

> The **brain**

#### B. Node-Agent Compute Runner

Runs on each node, embedded in `node-agent` or as a sidecar owned by it.

Responsible for:

- Input staging from MinIO or repository
- Artifact fetching
- Execution environment preparation
- Running compute units
- Streaming logs and metrics
- Uploading outputs
- Heartbeats and status reporting

> The **hand**

#### C. Compute Definition Store

Backed by repository artifacts.

Defines:

- Executable identity
- Runtime contract
- Input and output schemas
- Partitioning capability
- Merge and verification strategies
- Reproducibility constraints

> The **contract**

#### D. Workflow Engine

All orchestration is performed through workflows:

- Job submission
- Partitioning
- Scheduling
- Execution
- Retry
- Aggregation
- Verification
- Remediation

> Workflows remain the single source of orchestration truth

#### E. Shared Storage Layer

Uses existing Globular infrastructure:

- **MinIO** for inputs, outputs, and intermediate data
- **Repository** for compute artifacts and manifests
- **etcd** for desired state, runtime state, and leases
- **Prometheus** for metrics
- **EventService** for lifecycle events

---

## 4. State Model (4-Layer)

### Layer 1: Compute Artifact

- Executable bundle and manifest
- Immutable identity by version, build number, and checksum

### Layer 2: Desired Compute Job

- User or service intent
- Input references
- Execution policy and desired outcome

### Layer 3: Observed Execution Units

- Concrete partitions assigned to nodes
- Actual execution footprint in the cluster

### Layer 4: Runtime Health

Tracks:

- queued, admitted, staged, running, retrying, blocked, aggregating, verifying, completed, failed, degraded
- heartbeats
- throughput
- resource pressure
- skew and stragglers
- stuck partitions

---

## 5. Core Object Model

### ComputeDefinition

Declarative compute contract.

Key fields:

- `name`
- `version`
- `kind`
- `entrypoint`
- `runtime_type`
- `input_schema`
- `output_schema`
- `partition_strategy`
- `merge_strategy`
- `verify_strategy`
- `resource_profile`
- `retry_policy`
- `determinism_level`
- `idempotency_mode`
- `data_locality_hints`
- `security_policy`
- `allowed_node_profiles`
- `capabilities_required`

### ComputeJob

User-submitted execution.

Key fields:

- `job_id`
- `definition_ref`
- `definition_version`
- `input_refs`
- `parameters`
- `requested_output_location`
- `priority`
- `placement_policy`
- `deadline`
- `submitter`
- `workflow_run_id`
- `desired_parallelism`
- `reproducibility_mode`

### ComputePartitionPlan

Generated from definition plus input inspection.

Key fields:

- `plan_id`
- `job_id`
- `partitions[]`
- `estimated_cost`
- `estimated_bytes`
- `estimated_cpu`
- `estimated_memory`
- `aggregation_required`
- `merge_tree`
- `verification_plan`

### ComputeUnit

Schedulable execution unit.

Key fields:

- `unit_id`
- `job_id`
- `partition_id`
- `node_id`
- `state`
- `attempt`
- `input_refs`
- `output_ref`
- `lease_owner`
- `start_time`
- `end_time`
- `resource_reservation`
- `observed_progress`
- `checksum`
- `exit_status`
- `failure_reason`

### ComputeResult

Final validated output.

Key fields:

- `job_id`
- `result_ref`
- `result_manifest`
- `verification_status`
- `produced_by_units`
- `checksums`
- `metadata`
- `completed_at`

---

## 6. Reproducibility Model

Every compute task must be reproducible and auditable.

### A. Immutable Artifact Identity

- Exact package version
- Build number
- SHA-256
- Manifest version

### B. Immutable Input References

- Content-addressed object refs
- Input checksums
- Partition boundaries
- Parameter hash

### C. Execution Environment Contract

- CPU architecture
- OS family and version
- Runtime dependencies
- GPU requirements
- Environment variables declared in the manifest

### Determinism Levels

- `DETERMINISTIC`
- `MOSTLY_DETERMINISTIC`
- `NON_DETERMINISTIC_BOUNDED`
- `NON_DETERMINISTIC`

### Idempotency Modes

- `SAFE_RETRY`
- `RETRY_WITH_CLEANUP`
- `NO_AUTOMATIC_RETRY`
- `APPEND_ONLY`

Retry behavior must be declared, never guessed.

---

## 7. Compute Definition Manifest

Globular should extend its artifact manifest with a compute contract.

Example:

```yaml
kind: COMPUTE
name: ffmpeg-segment-transcode
version: 1.2.0
build_number: 4
entrypoint: /app/bin/worker
runtime_type: NATIVE_BINARY

compute:
  task_kind: PARTITIONABLE_BATCH
  input_schema: media.segment.transcode.request.v1
  output_schema: media.segment.transcode.result.v1

  partition_strategy:
    type: RANGE
    unit: TIME_SEGMENT
    min_partition_size: 30s
    max_partition_size: 300s
    supports_adaptive_partitioning: true

  merge_strategy:
    type: CONCAT_MEDIA_MANIFEST

  verify_strategy:
    type: MEDIA_PROBE
    checks:
      - duration_matches_expected
      - codec_matches_target
      - no_missing_segments

  determinism_level: MOSTLY_DETERMINISTIC
  idempotency_mode: SAFE_RETRY

  resources:
    cpu: "2-8"
    memory_mb: 4096
    gpu: 0
    local_disk_mb: 20480
    network_intensity: HIGH

  placement:
    require_profiles: ["media-worker"]
    prefer_data_locality: true
    avoid_busy_nodes: true

  inputs:
    immutable: true
    content_addressed: true

  outputs:
    content_addressed: true
    upload_to: minio://cluster-compute-results/
```

This gives Globular enough semantic shape to reason about compute tasks.

---

## 8. Scheduling Model

### Principles

The scheduler is responsible for:

- admission
- placement
- leasing

The workflow controls lifecycle.  
The scheduler controls where eligible work runs.

### Placement Inputs

- Node capabilities
- Current node pressure
- Profile membership
- Affinity and anti-affinity
- Data locality
- Retry history
- Job priority
- Fairness policy
- Cluster-wide resource reservations

### Placement Policies

- `PACK`
- `SPREAD`
- `LOCALITY_FIRST`
- `LOWEST_LOAD`
- `GPU_AWARE`
- `LATENCY_SENSITIVE`
- `THROUGHPUT_MAX`

### Lease Model

Each compute unit is leased to a node:

- Lease has a TTL
- `node-agent` renews the lease through heartbeats
- Expired lease makes the unit eligible for requeue
- Output commit must be atomic to prevent duplicate finalization

---

## 9. Workflow Model

### Core Workflows

#### `compute.job.submit`

Steps:

1. validate_definition
2. validate_inputs
3. classify_job
4. estimate_cost
5. admit_or_reject
6. create_job_record
7. start_partition_workflow

#### `compute.job.partition`

Steps:

1. inspect_inputs
2. generate_partition_plan
3. verify_partition_plan
4. persist_units
5. enqueue_units
6. emit_plan_generated

#### `compute.unit.execute`

Steps:

1. reserve_unit
2. choose_node
3. stage_inputs
4. stage_runtime
5. execute
6. capture_logs_metrics
7. upload_output
8. verify_unit_output
9. mark_unit_succeeded_or_failed

#### `compute.job.aggregate`

Steps:

1. wait_for_required_units
2. assess_missing_or_failed
3. retry_or_degrade
4. run_merge
5. verify_aggregate
6. publish_result
7. finalize_job

#### `compute.job.repair`

Steps:

1. inspect_stuck_or_failed_units
2. classify_failure
3. reschedule_safe_units
4. quarantine_bad_node_if_needed
5. restart_aggregation_if_needed
6. verify_convergence

#### `compute.job.cancel`

Steps:

1. freeze_new_assignments
2. revoke_pending_leases
3. signal_running_units
4. collect_partial_outputs
5. cleanup_if_policy_allows
6. finalize_cancelled

---

## 10. Execution Environment

Supported runtime types:

- `NATIVE_BINARY`
- `SCRIPTED`
- `WASM`
- `OCI_OPTIONAL`
- `VM_OPTIONAL`

### Recommended rollout

Start with:

- `NATIVE_BINARY`
- `SCRIPTED`

Add later:

- `WASM`

WASM is attractive because it is:

- portable
- restricted
- reproducible
- easier to secure than arbitrary shell execution

---

## 11. Data Layout

### etcd Paths

```text
/globular/compute/jobs/{job_id}
/globular/compute/jobs/{job_id}/desired
/globular/compute/jobs/{job_id}/plan
/globular/compute/jobs/{job_id}/units/{unit_id}

/globular/compute/runtime/units/{unit_id}
/globular/compute/runtime/nodes/{node_id}/allocations/{unit_id}

/globular/compute/results/{job_id}
```

Repository remains the canonical source of definition metadata.  
etcd only stores active desired and runtime state.

---

## 12. Verification Model

Every `ComputeDefinition` must declare a verification strategy.

### Verification Types

- `CHECKSUM`
- `SCHEMA_VALIDATE`
- `MEDIA_PROBE`
- `NUMERIC_TOLERANCE`
- `ROW_COUNT`
- `CUSTOM_VERIFIER`
- `MULTI_STAGE`

### Result Trust Levels

- `UNVERIFIED`
- `STRUCTURALLY_VERIFIED`
- `CONTENT_VERIFIED`
- `FULLY_REPRODUCED`
- `DEGRADED_ACCEPTED`

Verification status must be explicit and operator-visible.

---

## 13. Balancing and Adaptive Execution

### Static balancing

Use declared estimates such as:

- CPU seconds
- memory
- I/O intensity
- expected output size
- preferred partition size

### Dynamic balancing

Observed telemetry can later drive:

- slow node detection
- straggler rebalancing
- adaptive partition resizing
- speculative duplicate execution for lagging units
- node quarantine on repeated corruption or failure

### Adaptive partitioning

For partitionable tasks:

- start coarse
- split more finely if cluster headroom exists
- merge pending work if overhead dominates
- bisect repeatedly failing segments if the definition permits it

---

## 14. Security Model

The compute subsystem must follow Globular security rules.

### Rules

- All job submissions must be authenticated and authorized
- Definition execution must be policy-controlled
- Runtime artifact identity must be verified by checksum or signature
- `node-agent` only executes admitted units
- Input and output refs must be explicitly authorized
- No arbitrary host filesystem access unless approved by policy
- Compute jobs run with scoped credentials
- Inter-service calls preserve identity through Globular PKI and mTLS

### Suggested permissions

- `compute.definition.read`
- `compute.definition.publish`
- `compute.job.submit`
- `compute.job.cancel`
- `compute.job.read`
- `compute.job.override_priority`
- `compute.job.read_logs`
- `compute.admin.repair`
- `compute.admin.quarantine_node`

---

## 15. Failure Model

### Unit failure classes

- `artifact_fetch_failed`
- `input_missing`
- `lease_expired`
- `node_unreachable`
- `execution_nonzero_exit`
- `output_upload_failed`
- `output_verification_failed`
- `resource_exhausted`
- `policy_blocked`
- `deterministic_repeat_failure`
- `aggregation_blocked`

### Job-level outcomes

- `RETRYING`
- `PARTIAL_DEGRADED`
- `FAILED_TERMINAL`
- `WAITING_APPROVAL`
- `COMPLETED_WITH_WARNINGS`

### Important rule

If a definition is marked deterministic and the same failure repeats across multiple healthy nodes, Globular should classify it as a definition or input problem instead of endlessly retrying it.

---

## 16. Observability

### Metrics

- queued jobs
- running jobs
- runnable units
- success and failure totals
- retry counts
- queue delay
- execution duration by definition
- node compute saturation
- verification failures
- lease expirations
- straggler count
- speculative re-execution count

### Events

- `compute_job_submitted`
- `compute_job_admitted`
- `compute_job_partitioned`
- `compute_unit_assigned`
- `compute_unit_started`
- `compute_unit_progress`
- `compute_unit_succeeded`
- `compute_unit_failed`
- `compute_job_aggregating`
- `compute_job_verified`
- `compute_job_completed`
- `compute_job_degraded`
- `compute_job_failed`
- `compute_node_quarantined`

### Logs

All logs should correlate to:

- `job_id`
- `unit_id`
- `workflow_run_id`
- `node_id`
- `definition`
- `attempt`

---

## 17. API Surface

### Definitions

- `RegisterComputeDefinition`
- `GetComputeDefinition`
- `ListComputeDefinitions`
- `ValidateComputeDefinition`

### Jobs

- `SubmitComputeJob`
- `GetComputeJob`
- `ListComputeJobs`
- `CancelComputeJob`
- `RetryComputeJob`
- `ApproveComputeJob`
- `GetComputeResult`

### Units

- `ListComputeUnits`
- `GetComputeUnit`
- `StreamComputeUnitLogs`

### Scheduling and admin

- `QuarantineComputeNode`
- `UnquarantineComputeNode`
- `RebalanceComputeJob`
- `ExplainPlacementDecision`

### Node-agent runner

- `StageComputeUnit`
- `RunComputeUnit`
- `CancelComputeUnit`
- `ReportComputeHeartbeat`
- `CommitComputeOutput`

---

## 18. What “Understood Task” Means

Globular understands compute tasks through a typed compute contract.

That means every task definition must describe:

- the executable to run
- exact accepted inputs
- whether it can be partitioned
- how partitions are formed
- required resources
- what success means
- retry safety
- how outputs are merged
- how outputs are verified

This turns a task into an **operationally intelligible object**.

Globular is not just dispatching commands.  
It is executing **declared computational intent**.

---

## 19. Example: FFmpeg on Globular

### Definition type

`PARTITIONABLE_BATCH`

### Inputs

- source media object ref
- output codec configuration
- segment duration policy

### Partition strategy

- inspect media duration
- split into time segments
- optionally align to keyframes if pre-analysis exists

### Units

- each segment transcode becomes one compute unit

### Aggregation

- collect completed segment manifest
- concatenate or remux
- verify final media result

### Balancing

- large segments on strong nodes
- smaller segments on weaker nodes or as fill work
- prefer nodes with local cached input

### Failure handling

- retry segment on another node
- classify repeated identical failure as content or definition issue
- quarantine nodes that repeatedly produce corrupted outputs

---

## 20. Guardrails

### Do not

- Use etcd as transport for logs or payload blobs
- Make random shell commands the canonical task model
- Hide orchestration inside the scheduler
- Let `node-agent` invent work locally
- Rely on mutable node-local state without content-addressed refs
- Use opaque retry semantics
- Treat exit code alone as success
- Aggregate results without declared verification

---

## 21. Rollout Phases

### Phase 1: Compute v1 Core

- `ComputeDefinition`
- `ComputeJob`
- `ComputeUnit`
- node-agent execution
- MinIO staging
- workflow integration
- verification and result publishing
- no adaptive partitioning yet

### Phase 2: Partitionable Jobs

- partition planner
- aggregation workflow
- retries
- basic balancing
- node pressure awareness

### Phase 3: Advanced Balancing

- speculative execution
- adaptive partition sizing
- straggler mitigation
- locality-aware placement
- node quarantine heuristics

### Phase 4: Safer Runtimes

- WASM runtime
- stronger sandboxing
- richer resource isolation
- GPU-aware execution

---

## 22. Final Design Summary

The subsystem must be:

### Workflow-first
All orchestration goes through workflows.

### Definition-driven
Tasks are typed, reproducible, and operationally understood.

### Lease-based and HA-safe
Units are assigned through expiring leases and explicit commits.

### Content-addressed
Inputs and outputs are immutable and verifiable.

### Verifiable
Correctness is declared and checked.

### Adaptive
Scheduler can evolve to use declared intent plus observed telemetry.

### Globular-native
No hidden plan engine revival. No Kubernetes-shaped cargo cult.

---

## 23. Architecture Statement

> Globular Compute is a workflow-driven distributed execution subsystem where immutable compute definitions generate reproducible execution units, scheduled across nodes and verified through explicit correctness policies.

---

## 24. Key Insight

Globular does not merely run programs.

Globular executes **computational intent**.
