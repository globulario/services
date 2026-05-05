# Case 06: Intent Markers And Tombstones

## Status In Code
- PARTIAL: ingress delete-approval check exists, but approve/deny tests and cross-domain adoption are incomplete.

## Target Invariant
- Destructive desired-state deletion requires explicit audited intent, never accidental key loss.

## Required Implementation
- Add delete approval marker flow:
  - `/globular/<domain>/delete_approval/<generation>`
- Controller delete path requires valid approval marker and leader identity.
- Otherwise controller restores missing key on next cycle.

## Remaining To Reach DoD
- Add unit tests for ingress approve/deny delete flow (with generation mismatch and stale approval cases).
- Add integration test for unauthorized delete auto-restore.
- Extract tombstone/delete-approval helper and apply to at least one additional critical domain (objectstore or PKI metadata).

## Tests
- Unit: delete without approval => restore.
- Unit: delete with valid approval => accepted and reflected.
- Integration: ingress key delete without approval keeps VIP behavior stable.

## DoD
- Key deletion is intentional, auditable, and reversible by default.
