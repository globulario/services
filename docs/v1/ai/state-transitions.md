# State Transitions

Strict state machines for key system objects.

## Package Publish States

```
STAGING → VERIFIED → PUBLISHED
VERIFIED → FAILED
PUBLISHED → DEPRECATED → YANKED
PUBLISHED → QUARANTINED (admin only)
QUARANTINED → PUBLISHED (admin un-quarantine)
PUBLISHED → REVOKED (admin or owner)
```

## Node Lifecycle

```
discovered → admitted → profile_resolved → converging → workload_ready
converging → degraded (if packages fail)
workload_ready → ready
ready → degraded (if health fails)
degraded → ready (after repair)
```

## Service Status Vocabulary

| Status | Meaning |
|--------|---------|
| Installed | desired == installed, converged |
| Planned | desired set, not yet installed |
| Available | in repo, no desired release |
| Drifted | installed version differs from desired |
| Unmanaged | installed without a desired-state entry |
| Missing in repo | desired/installed but artifact not in repository |
| Orphaned | in repo, not desired, not installed |

## Compute Job States

```
JOB_PENDING → JOB_ADMITTED → JOB_RUNNING → JOB_COMPLETED
JOB_RUNNING → JOB_FAILED
JOB_RUNNING → JOB_CANCELLED
JOB_ADMITTED → JOB_FAILED (placement failed, deadline passed)
```

## Compute Unit States

```
UNIT_PENDING → UNIT_ASSIGNED → UNIT_STAGING → UNIT_RUNNING → UNIT_SUCCEEDED
UNIT_RUNNING → UNIT_FAILED → [retry] → UNIT_PENDING (if retryable, max 3)
UNIT_RUNNING → UNIT_CANCELLED (deadline or explicit cancel)
UNIT_RUNNING → UNIT_LEASE_EXPIRED (runner died)
UNIT_FAILED (max attempts) → terminal
```

## Workflow Run States

```
RUN_STATUS_PENDING → RUN_STATUS_RUNNING → RUN_STATUS_SUCCEEDED
RUN_STATUS_RUNNING → RUN_STATUS_FAILED
```

## Verification Trust Levels

```
UNVERIFIED → STRUCTURALLY_VERIFIED → CONTENT_VERIFIED → FULLY_REPRODUCED
```

| Level | Meaning |
|-------|---------|
| UNVERIFIED | No verify_strategy declared or skipped |
| STRUCTURALLY_VERIFIED | Output exists with non-empty files |
| CONTENT_VERIFIED | Checksum matched expected values |
| FULLY_REPRODUCED | Bitwise-identical reproduction (not yet implemented) |

## Lease Lifecycle

```
grant(jobID, unitID, nodeID, 30s TTL)
  → key: /globular/compute/leases/{jobID}/{unitID}
  → KeepAlive renewal during execution
  → revoke on completion or cancellation
  → auto-expire on runner death (30s TTL)
```
