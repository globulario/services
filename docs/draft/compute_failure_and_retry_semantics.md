# Compute Failure and Retry Semantics

## Retry policy

- Maximum 3 attempts per unit (hard cap)
- Retry evaluated in `computeAwaitAllUnits` when a unit reaches UNIT_FAILED
- Each (unit_id, attempt) pair evaluated only once per poll cycle
- Retried units get attempt incremented, state reset, re-dispatched to fresh placement

## Decision matrix

| IdempotencyMode | DeterminismLevel | FailureClass | Retry? |
|----------------|-----------------|--------------|--------|
| NO_AUTOMATIC_RETRY | any | any | Never |
| SAFE_RETRY | DETERMINISTIC | EXECUTION_NONZERO_EXIT | Never |
| SAFE_RETRY | DETERMINISTIC | OUTPUT_VERIFICATION_FAILED | Never |
| SAFE_RETRY | NON_DETERMINISTIC | EXECUTION_NONZERO_EXIT | Yes |
| SAFE_RETRY | NON_DETERMINISTIC | OUTPUT_VERIFICATION_FAILED | Yes |
| any | any | NODE_UNREACHABLE | Always |
| any | any | LEASE_EXPIRED | Always |
| any | any | RESOURCE_EXHAUSTED | Always |
| any | any | ARTIFACT_FETCH_FAILED | Yes |
| any | any | INPUT_MISSING | Yes |
| any | any | OUTPUT_UPLOAD_FAILED | Yes |
| any | any | DETERMINISTIC_REPEAT_FAILURE | Never |
| any | any | POLICY_BLOCKED | Never |
| any | any | AGGREGATION_BLOCKED | Never |

## Failure classes

| Class | Meaning | Source |
|-------|---------|--------|
| ARTIFACT_FETCH_FAILED | MinIO input download failed | StageComputeUnit |
| INPUT_MISSING | Input key not found in MinIO | StageComputeUnit |
| LEASE_EXPIRED | etcd lease TTL exceeded | computeAwaitUnitTerminal |
| NODE_UNREACHABLE | gRPC dial to runner failed | computeDispatchAllUnits |
| EXECUTION_NONZERO_EXIT | Process exited non-zero | executeUnit |
| OUTPUT_UPLOAD_FAILED | MinIO output upload failed | handleExecutionComplete |
| OUTPUT_VERIFICATION_FAILED | Checksum mismatch or structural check failed | verifyOutput |
| RESOURCE_EXHAUSTED | Node capacity insufficient | placeUnit |
| POLICY_BLOCKED | RBAC/security policy violation | validatePackageAccess |
| DETERMINISTIC_REPEAT_FAILURE | Same failure on deterministic definition | shouldRetryUnit |
| AGGREGATION_BLOCKED | Aggregate step cannot proceed | computeCreateResult |

## Classification helpers

- `classifyStageFailure(err)`: maps staging errors to ARTIFACT_FETCH_FAILED, INPUT_MISSING, or NODE_UNREACHABLE
- `classifyRunFailure(err)`: maps dispatch errors to NODE_UNREACHABLE or EXECUTION_NONZERO_EXIT

## Terminal state transitions

### Successful path
```
UNIT_PENDING → UNIT_ASSIGNED → UNIT_STAGING → UNIT_RUNNING → UNIT_SUCCEEDED
```

### Failed path (retryable)
```
UNIT_RUNNING → UNIT_FAILED → [retry] → UNIT_PENDING → UNIT_ASSIGNED → ...
```

### Failed path (terminal)
```
UNIT_RUNNING → UNIT_FAILED (max attempts or non-retryable)
```

### Timeout path
```
UNIT_RUNNING → UNIT_CANCELLED (deadline exceeded)
```

### Lease expiry
```
UNIT_RUNNING → UNIT_LEASE_EXPIRED (runner died)
```

## Job terminal states

| State | Condition |
|-------|-----------|
| JOB_COMPLETED | All units SUCCEEDED + verification passed |
| JOB_FAILED | Any unit FAILED after max retries |
| JOB_FAILED | Verification failed (even if execution succeeded) |
| JOB_FAILED | Deadline exceeded |
| JOB_FAILED | Placement failed (no eligible nodes) |
| JOB_CANCELLED | Explicit CancelComputeJob call |

## Non-guarantees

- No exactly-once execution (retries may re-run)
- No ordering of retried units
- No partial success (any terminal failure → JOB_FAILED)
- No automatic cleanup of staging directories
