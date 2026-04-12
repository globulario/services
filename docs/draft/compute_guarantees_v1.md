# Compute Guarantees v1

Operator-facing guarantees for the Globular Compute subsystem as of Phase 2.

## Lease guarantees

- Every running unit has an etcd TTL lease (30 seconds)
- Lease is granted at unit assignment, renewed during execution
- If the runner process dies, the lease expires within 30 seconds
- The await loop detects lease expiry and transitions the unit to UNIT_LEASE_EXPIRED
- No two nodes can hold the same unit's lease simultaneously (etcd atomicity)
- Lease is explicitly revoked on normal completion or cancellation

## Heartbeat guarantees

- The runner sends heartbeat keys to etcd every 5 seconds while a unit executes
- Heartbeat keys have a 15-second TTL — auto-expire if the runner stops writing
- The await loop checks lease liveness every 10 seconds
- Heartbeat progress is currently always 0.0 (entrypoints do not report progress yet)

## Retry guarantees

- Maximum 3 attempts per unit (hard cap, not configurable yet)
- Retry decisions are based on: IdempotencyMode, DeterminismLevel, FailureClass
- DETERMINISTIC definitions with EXECUTION_NONZERO_EXIT are never retried
- DETERMINISTIC_REPEAT_FAILURE class is never retried regardless of policy
- NO_AUTOMATIC_RETRY mode blocks all retries regardless of failure class
- Transient failures (NODE_UNREACHABLE, LEASE_EXPIRED, RESOURCE_EXHAUSTED) are always retried
- Each retry picks a (possibly different) node via round-robin
- Retry tracking uses (unit_id, attempt) keys to prevent double-retry in the same cycle

## Verification guarantees

- Verification runs after output upload, before result creation
- CHECKSUM type: computes SHA-256 of all output files, compares against expected values in rule.checks[]
- SCHEMA_VALIDATE type: checks that output directory has at least one non-empty file
- No verify_strategy declared → trust level is UNVERIFIED (pass, backward compatible)
- Verification failure with declared strategy → JOB_FAILED even if exit code was 0
- Trust level mapping:
  - UNVERIFIED: no strategy or skipped
  - STRUCTURALLY_VERIFIED: output exists with content
  - CONTENT_VERIFIED: checksum matched expected values

## Output commit guarantees

- All unit outputs are uploaded to MinIO before the unit is marked SUCCEEDED
- Each output file gets a computed SHA-256 checksum
- ObjectRef (URI + checksum + size) is stored on the unit in etcd — never the blob
- For multi-unit jobs, an aggregate manifest.json is uploaded to MinIO containing all unit output refs
- etcd never stores blob data — only metadata pointers

## Workflow orchestration guarantees

- All job execution goes through one workflow path: compute.job.submit
- No inline dispatch, no runner-side job finalization, no hidden shortcuts
- The workflow engine dispatches each step as an actor callback to the compute service
- Step failures trigger the onFailure hook (compute.mark_job_failed)
- The workflow service records run status for observability
- Workflow definitions are published to MinIO on compute service startup (idempotent)

## Explicit non-guarantees

These are NOT guaranteed in Phase 2:

- **No resource reservation**: leases track ownership, not CPU/memory capacity
- **No exactly-once execution**: units may be re-dispatched on retry
- **No ordering**: multi-unit partitions execute in arbitrary order
- **No data locality**: placement is round-robin, not locality-aware
- **No partial success**: if any unit fails after max retries, the job fails
- **No output merge**: aggregate manifest lists outputs but does not merge data
- **No real-time progress**: heartbeat progress is always 0.0
- **No preemption**: running units cannot be preempted by higher-priority jobs
