# Case 01: Critical State Under-Replicated

## Status In Code
- DONE: schema guard enforces RF policy for critical keyspaces, publishes status under `/globular/scylla/schema_guard/*`, raises violations, and marks repair-required state.

## Target Invariant
- In multi-node clusters, critical Scylla keyspaces must never remain below required RF.

## Implemented
- Keep `desiredRFForCluster(storageNodes)` policy:
  - 1 node => RF=1
  - 2 nodes => RF=2
  - 3+ nodes => RF=3
- Preserve rule: never auto-lower RF.
- When RF < required:
  - emit doctor `scylla.keyspace.rf_policy_violation`
  - publish actionable status including `repair_required=true`
  - enqueue/mark repair action (operator-triggerable and controller-observable).

## Residual Hardening (Optional)
- Expand critical keyspace catalog coverage tests as new control-plane keyspaces are introduced.

## Files/Components
- `golang/clustercontroller/...` schema guard implementation.
- `golang/clustercontroller/...` keyspace inventory source.
- `golang/health/doctor/...` scylla invariants.
- CLI: `golang/globularcli/resilience_cmds.go` (`scylla schema status` output enrichment).

## Metrics
- `globular_scylla_keyspace_rf{keyspace}`
- `globular_scylla_keyspace_rf_required{keyspace}`
- `globular_scylla_schema_guard_violation{keyspace}`
- `globular_scylla_schema_guard_last_success_timestamp`

## Tests
- Unit: existing keyspace RF=1 on 5-node intent => ALTER to RF=3 path triggered.
- Unit: 1-node intent allows RF=1 without violation.
- Unit: ALTER failure still returns bounded error and publishes degraded status.
- Integration (`testcluster`): force RF drift then verify status key + doctor finding + non-crash controller.

## DoD
- Any critical keyspace below policy is detected, surfaced, and remediable without silent acceptance.
