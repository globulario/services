# Case 04: Last-Known-Good Expansion

## Status In Code
- PARTIAL: shared LKG package exists and ingress is migrated; Envoy/MinIO/DNS consumers are not fully migrated.

## Target Invariant
- Critical consumers must preserve service continuity using validated LKG when authority is temporarily unavailable.

## Required Implementation
- Add shared LKG helper library:
  - atomic write (`tmp + fsync + rename`)
  - checksum/generation validation
  - monotonic generation guard
- Reuse in ingress and future consumers.

## Remaining To Reach DoD
- Migrate Envoy runtime config consumer to shared LKG helper.
- Migrate MinIO topology/config consumer to shared LKG helper.
- Migrate DNS persisted zone snapshot path to shared LKG helper.
- Add compatibility test matrix (no-LKG/valid-LKG/corrupt-LKG) for each migrated consumer.

## Tests
- Unit: corrupt LKG rejected, prior valid state retained.
- Unit: atomic write survives restart/crash simulation.
- Integration: etcd read outage still serves from LKG path.

## DoD
- LKG is durable, validated, and reusable, not ad-hoc.
