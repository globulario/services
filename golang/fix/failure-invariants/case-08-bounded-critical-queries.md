# Case 08: Bounded Critical Queries

## Status In Code
- PARTIAL: bounded helper exists and key reconcile paths are covered; DNS reconciler and some secondary paths still need timeout audit.

## Target Invariant
- Critical control-plane queries never run unbounded and never monopolize reconcile loops.

## Required Implementation
- Apply explicit context deadlines (5-15s policy by operation class) to:
  - Scylla schema/projections queries
  - etcd reads/writes in reconcile loops
  - DNS reload data fetches
- Standardize timeout wrappers and error tagging for doctor.

## Remaining To Reach DoD
- Complete timeout audit for DNS reload/reconciler loops and watcher callbacks.
- Add static lint/check rule (or unit guard) that critical lane handlers must call bounded wrapper.
- Add integration timeout simulation for DNS backend stall and verify lane releases + doctor finding.

## Tests
- Unit: query timeout maps to degraded result category.
- Integration: induced slow backend triggers timeout findings and lane recovery.

## DoD
- No critical reconcile path can hang indefinitely.
