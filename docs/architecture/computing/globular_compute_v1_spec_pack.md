# Globular Compute v1 Specification Pack

This document defines:

1. Proto definitions and gRPC contracts
2. Exact workflow YAML definitions
3. Node-agent execution specification

It is written to be implementation-ready and to minimize architectural drift.

---

# 1. Scope and Design Guardrails

## 1.1 v1 Scope

Compute v1 includes:

- typed compute definitions
- typed compute jobs
- single-unit execution as the default path
- optional partition planning objects, but partition fan-out may remain disabled until phase 2
- workflow-driven orchestration
- repository-backed definition identity
- MinIO-backed staged inputs and outputs
- etcd-backed desired state, runtime state, and leases
- node-agent execution runner
- explicit verification contract
- explicit retry and determinism declarations

## 1.2 Architectural Guardrails

The implementation MUST follow these rules:

1. Workflows are the only orchestration source of truth.
2. The compute service performs admission, state mutation coordination, and scheduling decisions, but MUST NOT hide lifecycle orchestration outside workflows.
3. node-agent MUST NOT invent work locally.
4. Repository remains the canonical source of artifact and definition identity.
5. etcd stores desired state, runtime state, leases, and compact execution metadata only.
6. Large inputs, outputs, logs, and payload blobs MUST NOT be stored in etcd.
7. Success MUST NOT be based only on process exit code.
8. Retry behavior MUST follow the declared idempotency and determinism rules.
9. Output finalization MUST be atomic.
10. The implementation MUST preserve Globular PKI, mTLS, RBAC, provenance, and workflow telemetry conventions.

---

# 2. Proto Definitions and gRPC Contracts

## 2.1 Package Layout

Recommended package layout:

- `proto/globular/compute/compute.proto`
- `proto/globular/compute/runner.proto`

## 2.2 `compute.proto`

```proto
syntax = "proto3";

package globular.compute;

option go_package = "globular.io/globular/proto/compute;computepb";

import "google/protobuf/timestamp.proto";
import "google/protobuf/struct.proto";

enum ComputeDefinitionKind {
  COMPUTE_DEFINITION_KIND_UNSPECIFIED = 0;
  SINGLE_NODE = 1;
  PARTITIONABLE_BATCH = 2;
  MAP_REDUCE = 3;
  PIPELINE = 4;
  AGGREGATION = 5;
  GPU_BATCH = 6;
  STREAM_WINDOWED = 7;
}

enum RuntimeType {
  RUNTIME_TYPE_UNSPECIFIED = 0;
  NATIVE_BINARY = 1;
  SCRIPTED = 2;
  WASM = 3;
  OCI_OPTIONAL = 4;
  VM_OPTIONAL = 5;
}

enum DeterminismLevel {
  DETERMINISM_LEVEL_UNSPECIFIED = 0;
  DETERMINISTIC = 1;
  MOSTLY_DETERMINISTIC = 2;
  NON_DETERMINISTIC_BOUNDED = 3;
  NON_DETERMINISTIC = 4;
}

enum IdempotencyMode {
  IDEMPOTENCY_MODE_UNSPECIFIED = 0;
  SAFE_RETRY = 1;
  RETRY_WITH_CLEANUP = 2;
  NO_AUTOMATIC_RETRY = 3;
  APPEND_ONLY = 4;
}

enum VerificationType {
  VERIFICATION_TYPE_UNSPECIFIED = 0;
  CHECKSUM = 1;
  SCHEMA_VALIDATE = 2;
  MEDIA_PROBE = 3;
  NUMERIC_TOLERANCE = 4;
  ROW_COUNT = 5;
  CUSTOM_VERIFIER = 6;
  MULTI_STAGE = 7;
}

enum JobState {
  JOB_STATE_UNSPECIFIED = 0;
  JOB_PENDING = 1;
  JOB_ADMITTED = 2;
  JOB_PARTITIONING = 3;
  JOB_QUEUED = 4;
  JOB_RUNNING = 5;
  JOB_AGGREGATING = 6;
  JOB_VERIFYING = 7;
  JOB_COMPLETED = 8;
  JOB_DEGRADED = 9;
  JOB_FAILED = 10;
  JOB_CANCELLED = 11;
  JOB_WAITING_APPROVAL = 12;
  JOB_RETRYING = 13;
}

enum UnitState {
  UNIT_STATE_UNSPECIFIED = 0;
  UNIT_PENDING = 1;
  UNIT_RESERVED = 2;
  UNIT_ASSIGNED = 3;
  UNIT_STAGING = 4;
  UNIT_RUNNING = 5;
  UNIT_UPLOADING = 6;
  UNIT_VERIFYING = 7;
  UNIT_SUCCEEDED = 8;
  UNIT_FAILED = 9;
  UNIT_CANCELLED = 10;
  UNIT_LEASE_EXPIRED = 11;
}

enum ResultTrustLevel {
  RESULT_TRUST_LEVEL_UNSPECIFIED = 0;
  UNVERIFIED = 1;
  STRUCTURALLY_VERIFIED = 2;
  CONTENT_VERIFIED = 3;
  FULLY_REPRODUCED = 4;
  DEGRADED_ACCEPTED = 5;
}

enum PlacementPolicy {
  PLACEMENT_POLICY_UNSPECIFIED = 0;
  PACK = 1;
  SPREAD = 2;
  LOCALITY_FIRST = 3;
  LOWEST_LOAD = 4;
  GPU_AWARE = 5;
  LATENCY_SENSITIVE = 6;
  THROUGHPUT_MAX = 7;
}

enum FailureClass {
  FAILURE_CLASS_UNSPECIFIED = 0;
  ARTIFACT_FETCH_FAILED = 1;
  INPUT_MISSING = 2;
  LEASE_EXPIRED = 3;
  NODE_UNREACHABLE = 4;
  EXECUTION_NONZERO_EXIT = 5;
  OUTPUT_UPLOAD_FAILED = 6;
  OUTPUT_VERIFICATION_FAILED = 7;
  RESOURCE_EXHAUSTED = 8;
  POLICY_BLOCKED = 9;
  DETERMINISTIC_REPEAT_FAILURE = 10;
  AGGREGATION_BLOCKED = 11;
}

message ObjectRef {
  string uri = 1;
  string sha256 = 2;
  uint64 size_bytes = 3;
  map<string, string> metadata = 4;
}

message ResourceProfile {
  uint32 min_cpu_millis = 1;
  uint32 max_cpu_millis = 2;
  uint64 min_memory_bytes = 3;
  uint64 max_memory_bytes = 4;
  uint64 local_disk_bytes = 5;
  uint32 gpu_count = 6;
  string network_intensity = 7;
}

message PlacementRules {
  repeated string require_profiles = 1;
  repeated string prefer_profiles = 2;
  bool prefer_data_locality = 3;
  bool avoid_busy_nodes = 4;
  PlacementPolicy default_policy = 5;
}

message PartitionStrategy {
  string type = 1;
  string unit = 2;
  string min_partition_size = 3;
  string max_partition_size = 4;
  bool supports_adaptive_partitioning = 5;
}

message MergeStrategy {
  string type = 1;
  google.protobuf.Struct config = 2;
}

message VerificationRule {
  VerificationType type = 1;
  repeated string checks = 2;
  string custom_verifier_ref = 3;
  google.protobuf.Struct config = 4;
}

message SecurityPolicy {
  repeated string allowed_roots = 1;
  bool network_egress_allowed = 2;
  bool host_filesystem_access_allowed = 3;
  repeated string required_permissions = 4;
}

message ComputeDefinition {
  string name = 1;
  string version = 2;
  int64 build_number = 3;
  string artifact_uri = 4;
  string artifact_sha256 = 5;
  ComputeDefinitionKind kind = 6;
  string entrypoint = 7;
  RuntimeType runtime_type = 8;
  string input_schema = 9;
  string output_schema = 10;
  PartitionStrategy partition_strategy = 11;
  MergeStrategy merge_strategy = 12;
  VerificationRule verify_strategy = 13;
  ResourceProfile resource_profile = 14;
  DeterminismLevel determinism_level = 15;
  IdempotencyMode idempotency_mode = 16;
  PlacementRules placement = 17;
  SecurityPolicy security_policy = 18;
  repeated string capabilities_required = 19;
  repeated string allowed_node_profiles = 20;
  map<string, string> labels = 21;
}

message ComputeJobSpec {
  string definition_name = 1;
  string definition_version = 2;
  int64 definition_build_number = 3;
  repeated ObjectRef input_refs = 4;
  google.protobuf.Struct parameters = 5;
  ObjectRef requested_output_location = 6;
  uint32 desired_parallelism = 7;
  PlacementPolicy placement_policy = 8;
  string priority = 9;
  google.protobuf.Timestamp deadline = 10;
  string repro_mode = 11;
  repeated string tags = 12;
}

message ComputeJob {
  string job_id = 1;
  ComputeJobSpec spec = 2;
  JobState state = 3;
  string workflow_run_id = 4;
  string submitter = 5;
  google.protobuf.Timestamp created_at = 6;
  google.protobuf.Timestamp updated_at = 7;
  string failure_message = 8;
}

message Partition {
  string partition_id = 1;
  repeated ObjectRef input_refs = 2;
  google.protobuf.Struct parameters = 3;
  uint64 estimated_cpu_millis = 4;
  uint64 estimated_memory_bytes = 5;
  uint64 estimated_bytes = 6;
  map<string, string> locality_hints = 7;
}

message ComputePartitionPlan {
  string plan_id = 1;
  string job_id = 2;
  repeated Partition partitions = 3;
  bool aggregation_required = 4;
  string merge_tree = 5;
  google.protobuf.Struct verification_plan = 6;
  google.protobuf.Timestamp created_at = 7;
}

message ComputeUnit {
  string unit_id = 1;
  string job_id = 2;
  string partition_id = 3;
  string node_id = 4;
  UnitState state = 5;
  uint32 attempt = 6;
  repeated ObjectRef input_refs = 7;
  ObjectRef output_ref = 8;
  string lease_owner = 9;
  google.protobuf.Timestamp lease_expires_at = 10;
  google.protobuf.Timestamp start_time = 11;
  google.protobuf.Timestamp end_time = 12;
  ResourceProfile reservation = 13;
  double observed_progress = 14;
  string checksum = 15;
  int32 exit_status = 16;
  FailureClass failure_class = 17;
  string failure_reason = 18;
}

message ComputeResult {
  string job_id = 1;
  ObjectRef result_ref = 2;
  ResultTrustLevel trust_level = 3;
  repeated string checksums = 4;
  google.protobuf.Struct metadata = 5;
  google.protobuf.Timestamp completed_at = 6;
}

message RegisterComputeDefinitionRequest {
  ComputeDefinition definition = 1;
}

message RegisterComputeDefinitionResponse {
  ComputeDefinition definition = 1;
}

message GetComputeDefinitionRequest {
  string name = 1;
  string version = 2;
  int64 build_number = 3;
}

message GetComputeDefinitionResponse {
  ComputeDefinition definition = 1;
}

message ListComputeDefinitionsRequest {
  string name_prefix = 1;
  string page_token = 2;
  uint32 page_size = 3;
}

message ListComputeDefinitionsResponse {
  repeated ComputeDefinition definitions = 1;
  string next_page_token = 2;
}

message ValidateComputeDefinitionRequest {
  ComputeDefinition definition = 1;
}

message ValidateComputeDefinitionResponse {
  bool valid = 1;
  repeated string errors = 2;
  repeated string warnings = 3;
}

message SubmitComputeJobRequest {
  ComputeJobSpec spec = 1;
}

message SubmitComputeJobResponse {
  ComputeJob job = 1;
}

message GetComputeJobRequest {
  string job_id = 1;
}

message GetComputeJobResponse {
  ComputeJob job = 1;
}

message ListComputeJobsRequest {
  JobState state_filter = 1;
  string submitter = 2;
  string page_token = 3;
  uint32 page_size = 4;
}

message ListComputeJobsResponse {
  repeated ComputeJob jobs = 1;
  string next_page_token = 2;
}

message CancelComputeJobRequest {
  string job_id = 1;
  string reason = 2;
}

message CancelComputeJobResponse {
  ComputeJob job = 1;
}

message RetryComputeJobRequest {
  string job_id = 1;
  string reason = 2;
}

message RetryComputeJobResponse {
  ComputeJob job = 1;
}

message ApproveComputeJobRequest {
  string job_id = 1;
  string approval_note = 2;
}

message ApproveComputeJobResponse {
  ComputeJob job = 1;
}

message GetComputeResultRequest {
  string job_id = 1;
}

message GetComputeResultResponse {
  ComputeResult result = 1;
}

message ListComputeUnitsRequest {
  string job_id = 1;
  UnitState state_filter = 2;
  string page_token = 3;
  uint32 page_size = 4;
}

message ListComputeUnitsResponse {
  repeated ComputeUnit units = 1;
  string next_page_token = 2;
}

message GetComputeUnitRequest {
  string unit_id = 1;
}

message GetComputeUnitResponse {
  ComputeUnit unit = 1;
}

message StreamComputeUnitLogsRequest {
  string unit_id = 1;
  bool follow = 2;
  uint32 tail_lines = 3;
}

message ComputeLogChunk {
  string unit_id = 1;
  string node_id = 2;
  google.protobuf.Timestamp timestamp = 3;
  string stream = 4;
  bytes data = 5;
}

message QuarantineComputeNodeRequest {
  string node_id = 1;
  string reason = 2;
}

message QuarantineComputeNodeResponse {
  string node_id = 1;
  bool quarantined = 2;
}

message UnquarantineComputeNodeRequest {
  string node_id = 1;
}

message UnquarantineComputeNodeResponse {
  string node_id = 1;
  bool quarantined = 2;
}

message RebalanceComputeJobRequest {
  string job_id = 1;
  string reason = 2;
}

message RebalanceComputeJobResponse {
  ComputeJob job = 1;
}

message ExplainPlacementDecisionRequest {
  string job_id = 1;
  string unit_id = 2;
}

message ExplainPlacementDecisionResponse {
  repeated string reasons = 1;
  repeated string considered_nodes = 2;
  string selected_node = 3;
}

service ComputeService {
  rpc RegisterComputeDefinition(RegisterComputeDefinitionRequest) returns (RegisterComputeDefinitionResponse);
  rpc GetComputeDefinition(GetComputeDefinitionRequest) returns (GetComputeDefinitionResponse);
  rpc ListComputeDefinitions(ListComputeDefinitionsRequest) returns (ListComputeDefinitionsResponse);
  rpc ValidateComputeDefinition(ValidateComputeDefinitionRequest) returns (ValidateComputeDefinitionResponse);

  rpc SubmitComputeJob(SubmitComputeJobRequest) returns (SubmitComputeJobResponse);
  rpc GetComputeJob(GetComputeJobRequest) returns (GetComputeJobResponse);
  rpc ListComputeJobs(ListComputeJobsRequest) returns (ListComputeJobsResponse);
  rpc CancelComputeJob(CancelComputeJobRequest) returns (CancelComputeJobResponse);
  rpc RetryComputeJob(RetryComputeJobRequest) returns (RetryComputeJobResponse);
  rpc ApproveComputeJob(ApproveComputeJobRequest) returns (ApproveComputeJobResponse);
  rpc GetComputeResult(GetComputeResultRequest) returns (GetComputeResultResponse);

  rpc ListComputeUnits(ListComputeUnitsRequest) returns (ListComputeUnitsResponse);
  rpc GetComputeUnit(GetComputeUnitRequest) returns (GetComputeUnitResponse);
  rpc StreamComputeUnitLogs(StreamComputeUnitLogsRequest) returns (stream ComputeLogChunk);

  rpc QuarantineComputeNode(QuarantineComputeNodeRequest) returns (QuarantineComputeNodeResponse);
  rpc UnquarantineComputeNode(UnquarantineComputeNodeRequest) returns (UnquarantineComputeNodeResponse);
  rpc RebalanceComputeJob(RebalanceComputeJobRequest) returns (RebalanceComputeJobResponse);
  rpc ExplainPlacementDecision(ExplainPlacementDecisionRequest) returns (ExplainPlacementDecisionResponse);
}
```

## 2.3 `runner.proto`

```proto
syntax = "proto3";

package globular.compute.runner;

option go_package = "globular.io/globular/proto/compute/runner;runnerpb";

import "google/protobuf/timestamp.proto";
import "google/protobuf/struct.proto";
import "globular/compute/compute.proto";

message StageComputeUnitRequest {
  string unit_id = 1;
  string job_id = 2;
  globular.compute.ComputeDefinition definition = 3;
  globular.compute.ComputeUnit unit = 4;
  globular.compute.ComputeJobSpec job_spec = 5;
  string execution_root = 6;
}

message StageComputeUnitResponse {
  bool staged = 1;
  string staging_path = 2;
  repeated string warnings = 3;
}

message RunComputeUnitRequest {
  string unit_id = 1;
  string job_id = 2;
  globular.compute.ComputeDefinition definition = 3;
  globular.compute.ComputeUnit unit = 4;
  globular.compute.ComputeJobSpec job_spec = 5;
  string staging_path = 6;
}

message RunComputeUnitResponse {
  bool accepted = 1;
  string execution_id = 2;
}

message CancelComputeUnitRequest {
  string unit_id = 1;
  string job_id = 2;
  string reason = 3;
}

message CancelComputeUnitResponse {
  bool accepted = 1;
}

message ReportComputeHeartbeatRequest {
  string execution_id = 1;
  string unit_id = 2;
  string job_id = 3;
  string node_id = 4;
  double progress = 5;
  uint64 stdout_bytes = 6;
  uint64 stderr_bytes = 7;
  uint64 output_bytes = 8;
  google.protobuf.Struct metrics = 9;
  google.protobuf.Timestamp observed_at = 10;
}

message ReportComputeHeartbeatResponse {
  bool ok = 1;
  bool should_cancel = 2;
}

message CommitComputeOutputRequest {
  string execution_id = 1;
  string unit_id = 2;
  string job_id = 3;
  globular.compute.ObjectRef output_ref = 4;
  int32 exit_status = 5;
  string checksum = 6;
  google.protobuf.Struct verifier_metadata = 7;
}

message CommitComputeOutputResponse {
  bool committed = 1;
  string result_state = 2;
}

service ComputeRunnerService {
  rpc StageComputeUnit(StageComputeUnitRequest) returns (StageComputeUnitResponse);
  rpc RunComputeUnit(RunComputeUnitRequest) returns (RunComputeUnitResponse);
  rpc CancelComputeUnit(CancelComputeUnitRequest) returns (CancelComputeUnitResponse);
  rpc ReportComputeHeartbeat(ReportComputeHeartbeatRequest) returns (ReportComputeHeartbeatResponse);
  rpc CommitComputeOutput(CommitComputeOutputRequest) returns (CommitComputeOutputResponse);
}
```

## 2.4 Contract Notes

### ComputeService responsibilities

- validate and register definitions
- admit jobs
- persist desired and runtime metadata
- create or reference workflow runs
- decide placement
- own lease issuance and lease validation
- own final output commit validation
- expose operator-facing read APIs

### ComputeRunnerService responsibilities

- execute only explicitly assigned units
- stage artifacts and inputs
- run under the declared runtime contract
- emit heartbeats
- upload outputs
- request output commit
- never mutate cluster desired state directly

### Explicit non-responsibilities

- ComputeService does not embed opaque orchestration outside workflows
- node-agent does not schedule itself
- runner does not claim work by scanning etcd and self-assigning
- etcd is not a blob store

---

# 3. etcd Ownership and Key Layout

The following keys are recommended.

```text
/globular/compute/definitions/cache/{name}/{version}/{build_number}
/globular/compute/jobs/{job_id}
/globular/compute/jobs/{job_id}/desired
/globular/compute/jobs/{job_id}/plan
/globular/compute/jobs/{job_id}/units/{unit_id}
/globular/compute/jobs/{job_id}/result

/globular/compute/runtime/leases/{unit_id}
/globular/compute/runtime/units/{unit_id}
/globular/compute/runtime/nodes/{node_id}/allocations/{unit_id}
/globular/compute/runtime/jobs/{job_id}/summary
```

Ownership rules:

- Repository is canonical for artifact identity.
- Compute service owns all `/globular/compute/jobs/*` and `/globular/compute/runtime/*` state.
- node-agent may write only via ComputeService RPCs or explicitly authorized status update channels if introduced later.
- Workflow service owns workflow run state in its own namespace and remains the orchestration source of truth.

---

# 4. Exact Workflow YAML Definitions

These YAML definitions are written to match the architectural model. They are intentionally explicit and should not be replaced by a hidden planner.

## 4.1 `compute.job.submit.yaml`

```yaml
apiVersion: workflows.globular.io/v1
kind: WorkflowDefinition
metadata:
  name: compute.job.submit
spec:
  description: Admit a compute job and start partitioning or direct execution.
  input:
    type: object
    required: [job_id]
    properties:
      job_id:
        type: string

  steps:
    - id: load_job
      action: compute.LoadJob
      with:
        job_id: ${input.job_id}

    - id: validate_definition
      action: compute.ValidateJobDefinition
      with:
        job_id: ${input.job_id}

    - id: validate_inputs
      action: compute.ValidateJobInputs
      with:
        job_id: ${input.job_id}

    - id: classify_job
      action: compute.ClassifyJob
      with:
        job_id: ${input.job_id}

    - id: estimate_cost
      action: compute.EstimateJobCost
      with:
        job_id: ${input.job_id}

    - id: admission_check
      action: compute.AdmitJob
      with:
        job_id: ${input.job_id}

    - id: mark_admitted
      action: compute.MarkJobState
      with:
        job_id: ${input.job_id}
        state: JOB_ADMITTED

    - id: start_next
      switch:
        - when: ${steps.classify_job.outputs.partitionable == true}
          action: workflow.Start
          with:
            name: compute.job.partition
            input:
              job_id: ${input.job_id}
        - when: ${steps.classify_job.outputs.partitionable == false}
          action: workflow.Start
          with:
            name: compute.unit.execute
            input:
              job_id: ${input.job_id}
              unit_id: ${steps.classify_job.outputs.single_unit_id}

  onFailure:
    - action: compute.MarkJobFailed
      with:
        job_id: ${input.job_id}
        reason: ${workflow.error}
```

## 4.2 `compute.job.partition.yaml`

```yaml
apiVersion: workflows.globular.io/v1
kind: WorkflowDefinition
metadata:
  name: compute.job.partition
spec:
  description: Build a partition plan and enqueue execution units.
  input:
    type: object
    required: [job_id]
    properties:
      job_id:
        type: string

  steps:
    - id: mark_partitioning
      action: compute.MarkJobState
      with:
        job_id: ${input.job_id}
        state: JOB_PARTITIONING

    - id: inspect_inputs
      action: compute.InspectInputs
      with:
        job_id: ${input.job_id}

    - id: generate_partition_plan
      action: compute.GeneratePartitionPlan
      with:
        job_id: ${input.job_id}

    - id: verify_partition_plan
      action: compute.VerifyPartitionPlan
      with:
        job_id: ${input.job_id}
        plan_id: ${steps.generate_partition_plan.outputs.plan_id}

    - id: persist_units
      action: compute.PersistUnitsFromPlan
      with:
        job_id: ${input.job_id}
        plan_id: ${steps.generate_partition_plan.outputs.plan_id}

    - id: enqueue_units
      action: compute.EnqueueUnits
      with:
        job_id: ${input.job_id}

    - id: emit_event
      action: events.Publish
      with:
        name: compute_job_partitioned
        payload:
          job_id: ${input.job_id}
          plan_id: ${steps.generate_partition_plan.outputs.plan_id}

    - id: start_units
      foreach:
        items: ${steps.persist_units.outputs.unit_ids}
        maxParallel: 32
        do:
          action: workflow.Start
          with:
            name: compute.unit.execute
            input:
              job_id: ${input.job_id}
              unit_id: ${item}

    - id: start_aggregate
      action: workflow.Start
      with:
        name: compute.job.aggregate
        input:
          job_id: ${input.job_id}

  onFailure:
    - action: compute.MarkJobFailed
      with:
        job_id: ${input.job_id}
        reason: ${workflow.error}
```

## 4.3 `compute.unit.execute.yaml`

```yaml
apiVersion: workflows.globular.io/v1
kind: WorkflowDefinition
metadata:
  name: compute.unit.execute
spec:
  description: Assign, stage, run, verify, and finalize one compute unit.
  input:
    type: object
    required: [job_id, unit_id]
    properties:
      job_id:
        type: string
      unit_id:
        type: string

  steps:
    - id: reserve_unit
      action: compute.ReserveUnit
      with:
        job_id: ${input.job_id}
        unit_id: ${input.unit_id}

    - id: choose_node
      action: compute.ChooseNode
      with:
        job_id: ${input.job_id}
        unit_id: ${input.unit_id}

    - id: issue_lease
      action: compute.IssueUnitLease
      with:
        job_id: ${input.job_id}
        unit_id: ${input.unit_id}
        node_id: ${steps.choose_node.outputs.node_id}
        ttl_seconds: 60

    - id: mark_assigned
      action: compute.MarkUnitAssigned
      with:
        job_id: ${input.job_id}
        unit_id: ${input.unit_id}
        node_id: ${steps.choose_node.outputs.node_id}

    - id: stage_unit
      action: compute.DispatchStage
      with:
        job_id: ${input.job_id}
        unit_id: ${input.unit_id}
        node_id: ${steps.choose_node.outputs.node_id}

    - id: run_unit
      action: compute.DispatchRun
      with:
        job_id: ${input.job_id}
        unit_id: ${input.unit_id}
        node_id: ${steps.choose_node.outputs.node_id}

    - id: await_completion
      action: compute.WaitForUnitTerminalState
      with:
        job_id: ${input.job_id}
        unit_id: ${input.unit_id}
        timeout_seconds: 86400

    - id: verify_unit_output
      switch:
        - when: ${steps.await_completion.outputs.unit_state == "UNIT_SUCCEEDED"}
          action: compute.VerifyUnitOutput
          with:
            job_id: ${input.job_id}
            unit_id: ${input.unit_id}
        - when: ${steps.await_completion.outputs.unit_state != "UNIT_SUCCEEDED"}
          action: compute.Noop
          with: {}

    - id: finalize_success
      switch:
        - when: ${steps.await_completion.outputs.unit_state == "UNIT_SUCCEEDED"}
          action: compute.MarkUnitSucceeded
          with:
            job_id: ${input.job_id}
            unit_id: ${input.unit_id}
        - when: ${steps.await_completion.outputs.unit_state != "UNIT_SUCCEEDED"}
          action: compute.MarkUnitFailed
          with:
            job_id: ${input.job_id}
            unit_id: ${input.unit_id}
            failure_class: ${steps.await_completion.outputs.failure_class}
            reason: ${steps.await_completion.outputs.failure_reason}

  onFailure:
    - action: compute.MarkUnitFailed
      with:
        job_id: ${input.job_id}
        unit_id: ${input.unit_id}
        failure_class: EXECUTION_NONZERO_EXIT
        reason: ${workflow.error}
```

## 4.4 `compute.job.aggregate.yaml`

```yaml
apiVersion: workflows.globular.io/v1
kind: WorkflowDefinition
metadata:
  name: compute.job.aggregate
spec:
  description: Wait for required units, aggregate outputs when needed, verify result, and finalize the job.
  input:
    type: object
    required: [job_id]
    properties:
      job_id:
        type: string

  steps:
    - id: wait_for_units
      action: compute.WaitForJobUnitsReadyForAggregation
      with:
        job_id: ${input.job_id}

    - id: assess_failures
      action: compute.AssessJobUnitOutcomes
      with:
        job_id: ${input.job_id}

    - id: maybe_repair
      switch:
        - when: ${steps.assess_failures.outputs.requires_repair == true}
          action: workflow.Start
          with:
            name: compute.job.repair
            input:
              job_id: ${input.job_id}
        - when: ${steps.assess_failures.outputs.requires_repair == false}
          action: compute.Noop
          with: {}

    - id: reload_after_repair
      action: compute.AssessJobUnitOutcomes
      with:
        job_id: ${input.job_id}

    - id: gate_aggregation
      action: compute.RequireAggregationEligible
      with:
        job_id: ${input.job_id}

    - id: mark_aggregating
      action: compute.MarkJobState
      with:
        job_id: ${input.job_id}
        state: JOB_AGGREGATING

    - id: run_merge
      switch:
        - when: ${steps.gate_aggregation.outputs.aggregation_required == true}
          action: compute.RunMerge
          with:
            job_id: ${input.job_id}
        - when: ${steps.gate_aggregation.outputs.aggregation_required == false}
          action: compute.SelectSingleUnitOutputAsResult
          with:
            job_id: ${input.job_id}

    - id: mark_verifying
      action: compute.MarkJobState
      with:
        job_id: ${input.job_id}
        state: JOB_VERIFYING

    - id: verify_result
      action: compute.VerifyJobResult
      with:
        job_id: ${input.job_id}

    - id: publish_result
      action: compute.PublishJobResult
      with:
        job_id: ${input.job_id}

    - id: finalize_job
      action: compute.FinalizeJob
      with:
        job_id: ${input.job_id}

  onFailure:
    - action: compute.MarkJobFailed
      with:
        job_id: ${input.job_id}
        reason: ${workflow.error}
```

## 4.5 `compute.job.repair.yaml`

```yaml
apiVersion: workflows.globular.io/v1
kind: WorkflowDefinition
metadata:
  name: compute.job.repair
spec:
  description: Repair stuck or failed units according to retry, determinism, and node-health policies.
  input:
    type: object
    required: [job_id]
    properties:
      job_id:
        type: string

  steps:
    - id: inspect_failures
      action: compute.InspectJobFailures
      with:
        job_id: ${input.job_id}

    - id: classify_failures
      action: compute.ClassifyFailures
      with:
        job_id: ${input.job_id}

    - id: quarantine_nodes
      foreach:
        items: ${steps.classify_failures.outputs.nodes_to_quarantine}
        maxParallel: 8
        do:
          action: compute.QuarantineNode
          with:
            node_id: ${item}
            reason: repeated_compute_corruption_or_failure

    - id: reschedule_units
      foreach:
        items: ${steps.classify_failures.outputs.retryable_unit_ids}
        maxParallel: 16
        do:
          action: workflow.Start
          with:
            name: compute.unit.execute
            input:
              job_id: ${input.job_id}
              unit_id: ${item}

    - id: fail_non_retryable
      foreach:
        items: ${steps.classify_failures.outputs.non_retryable_unit_ids}
        maxParallel: 16
        do:
          action: compute.MarkUnitFailed
          with:
            job_id: ${input.job_id}
            unit_id: ${item}
            failure_class: DETERMINISTIC_REPEAT_FAILURE
            reason: repeated_failure_not_retryable

    - id: verify_convergence
      action: compute.VerifyRepairConvergence
      with:
        job_id: ${input.job_id}

  onFailure:
    - action: events.Publish
      with:
        name: compute_job_failed
        payload:
          job_id: ${input.job_id}
          reason: ${workflow.error}
```

## 4.6 `compute.job.cancel.yaml`

```yaml
apiVersion: workflows.globular.io/v1
kind: WorkflowDefinition
metadata:
  name: compute.job.cancel
spec:
  description: Cancel a job, revoke pending leases, signal active units, and finalize cancellation.
  input:
    type: object
    required: [job_id, reason]
    properties:
      job_id:
        type: string
      reason:
        type: string

  steps:
    - id: freeze_assignments
      action: compute.FreezeJobAssignments
      with:
        job_id: ${input.job_id}

    - id: revoke_leases
      action: compute.RevokePendingLeases
      with:
        job_id: ${input.job_id}

    - id: signal_running_units
      action: compute.SignalRunningUnitsCancel
      with:
        job_id: ${input.job_id}
        reason: ${input.reason}

    - id: collect_partial_outputs
      action: compute.CollectPartialOutputs
      with:
        job_id: ${input.job_id}

    - id: finalize_cancelled
      action: compute.FinalizeCancelledJob
      with:
        job_id: ${input.job_id}
        reason: ${input.reason}

  onFailure:
    - action: compute.MarkJobFailed
      with:
        job_id: ${input.job_id}
        reason: ${workflow.error}
```

---

# 5. Node-Agent Execution Specification

This section is normative.

## 5.1 Purpose

The node-agent compute runner executes assigned compute units under a declared runtime contract and reports progress back to the compute service.

It does not schedule work, mutate desired state, or invent orchestration.

## 5.2 Responsibilities

The runner MUST:

1. accept only explicitly assigned work
2. validate definition identity and artifact checksum before execution
3. create a per-execution working directory
4. stage declared inputs from MinIO or repository
5. materialize a deterministic execution environment
6. execute the declared entrypoint only
7. stream logs and periodic heartbeats
8. upload outputs to the declared object location
9. request final output commit from the compute service
10. clean local temp state according to policy

## 5.3 Non-Responsibilities

The runner MUST NOT:

- scan etcd for work
- self-assign units
- mutate job desired state
- decide retry eligibility
- publish final result objects without commit approval
- reinterpret the definition contract
- execute undeclared host binaries outside the allowed runtime path

## 5.4 Execution Root Layout

Recommended local execution layout:

```text
/var/lib/globular/compute/
  executions/
    {execution_id}/
      definition.json
      job_spec.json
      unit.json
      inputs/
      outputs/
      work/
      logs/
        stdout.log
        stderr.log
      state.json
```

## 5.5 Staging Requirements

During `StageComputeUnit`, the runner MUST:

1. validate unit_id, job_id, definition identity, and checksum inputs
2. create execution directory
3. fetch artifact bundle
4. fetch declared input objects
5. verify fetched object checksums
6. write normalized local metadata files
7. prepare runtime command and environment manifest
8. return the staging path

If any fetch or verification step fails, the runner MUST fail staging and MUST NOT begin execution.

## 5.6 Runtime Contract

### NATIVE_BINARY

The runner executes the declared entrypoint from the staged artifact root.

Allowed behavior:

- binary path resolved only within the staged artifact
- working directory set to execution work directory
- stdout and stderr captured
- environment limited to declared variables plus Globular-required execution metadata

### SCRIPTED

The runner executes a declared script entrypoint through an approved interpreter path defined by policy.

The interpreter MUST be allowlisted.  
The script MUST originate from the staged artifact, not arbitrary host paths.

### WASM

Reserved for later phase.  
Do not partially implement in v1.

## 5.7 Environment Variables

The runner MAY inject a minimal stable environment contract.

Recommended execution variables:

- `GLOBULAR_JOB_ID`
- `GLOBULAR_UNIT_ID`
- `GLOBULAR_EXECUTION_ID`
- `GLOBULAR_INPUT_DIR`
- `GLOBULAR_OUTPUT_DIR`
- `GLOBULAR_WORK_DIR`
- `GLOBULAR_DEFINITION_PATH`
- `GLOBULAR_JOB_SPEC_PATH`
- `GLOBULAR_UNIT_PATH`

The runner MUST NOT inject arbitrary ad hoc environment variables as a substitute for repository or etcd-backed config.

## 5.8 Heartbeats

While a unit is running, the runner MUST send periodic heartbeats.

Recommended interval:

- every 5 to 10 seconds

Heartbeat payload includes:

- execution_id
- progress
- stdout/stderr byte counters
- output byte counters
- selected resource metrics
- observed timestamp

If the service indicates `should_cancel = true`, the runner MUST initiate cooperative termination.

## 5.9 Cancellation

On cancel:

1. send termination signal to the process
2. wait for graceful exit for a bounded timeout
3. force kill if necessary
4. update local state
5. stop heartbeats
6. report final cancellation status

The runner MUST distinguish between:

- user cancellation
- lease expiry cancellation
- local execution failure

## 5.10 Output Handling

The runner MUST:

1. write outputs only into the execution output directory
2. upload declared outputs to the configured object destination
3. compute output checksum
4. call `CommitComputeOutput`
5. treat output as non-final until commit succeeds

If upload succeeds but commit fails, the runner MUST leave the uploaded object in a non-final state that can be safely reconciled or garbage-collected later.

## 5.11 Security Constraints

The runner MUST enforce:

- no arbitrary host filesystem access unless explicitly allowed
- no artifact execution if checksum mismatches
- no undeclared network egress if policy forbids it
- no cross-job shared mutable state
- no secret material persisted into logs

## 5.12 Logging Rules

The runner MUST:

- write stdout and stderr to separate files
- make logs streamable
- correlate every line/chunk with execution metadata
- cap local retention by size and age
- never write sensitive credentials into logs intentionally

## 5.13 Local State Machine

Recommended local execution state machine:

- RECEIVED
- STAGING
- STAGED
- STARTING
- RUNNING
- CANCELLING
- UPLOADING
- COMMITTING
- SUCCEEDED
- FAILED
- CANCELLED

This local state machine is implementation detail, but its terminal outcomes must map cleanly to cluster-level `UnitState`.

## 5.14 Drift-Proof Rules

These rules are mandatory:

1. The runner only runs what the service assigned.
2. Assignment exists only when there is a valid lease.
3. The runner cannot silently replace the declared entrypoint.
4. The runner cannot reinterpret verification success from exit code alone.
5. The runner cannot finalize outputs by itself.
6. The runner cannot keep retrying locally outside declared policy.
7. The runner cannot substitute environment variables for source-of-truth config.
8. The runner cannot promote local observations into cluster desired state.

---

# 6. Implementation Sequence

## Phase 1: Compute v1 Core

Implement first:

- `ComputeDefinition`
- `ComputeJob`
- `ComputeUnit`
- `ComputeService`
- `ComputeRunnerService`
- `compute.job.submit`
- `compute.unit.execute`
- `compute.job.aggregate`
- node-agent staging, execution, heartbeat, output commit
- checksum verification
- single-unit jobs as the default path

## Phase 2: Partitionable Jobs

Implement next:

- partition planning
- `compute.job.partition`
- multiple units per job
- basic aggregation
- repair workflow
- static balancing

## Phase 3: Advanced Scheduling

Later:

- adaptive partitioning
- straggler mitigation
- speculative duplicate execution
- richer locality optimization
- automatic quarantine heuristics

---

# 7. Acceptance Criteria

## 7.1 Compute v1 Core

A v1 implementation is acceptable when:

1. a compute definition can be registered and validated
2. a compute job can be submitted by definition reference
3. the job starts via workflow, not hidden service logic
4. one unit is assigned to one node with a lease
5. node-agent stages inputs and artifact successfully
6. node-agent executes the declared entrypoint
7. heartbeats are observed during execution
8. outputs are uploaded and committed atomically
9. verification result is explicit
10. job reaches a truthful terminal state
11. logs are accessible by unit
12. cancellation works
13. deterministic repeated failures are not retried forever

## 7.2 No-Drift Acceptance

The implementation fails review if any of the following occur:

- orchestration hidden outside workflows
- node-agent self-scheduling
- etcd used for large payloads
- success defined only by exit code
- local retries ignore idempotency policy
- local environment variables replace canonical configuration paths
- output finalization bypasses service commit

---

# 8. Final Architecture Statement

Globular Compute v1 is a workflow-driven distributed execution subsystem in which typed compute definitions produce reproducible execution units, scheduled through explicit placement and lease control, executed by node-agent under a strict runtime contract, and finalized only after declared verification and atomic output commit.
