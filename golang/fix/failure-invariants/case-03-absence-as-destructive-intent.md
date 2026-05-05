# Case 03: Absence Interpreted As Destructive Intent

## Status In Code
- DONE: ingress and explicit-disable guard path now prevent destructive fallback on missing/invalid spec.

## Target Invariant
- Missing/timeout/invalid control-plane state must not imply “disable/stop/delete”.

## Implemented
- Add shared consumer policy:
  - `missing => HOLD_LAST_KNOWN_GOOD`
  - `invalid => HOLD_LAST_KNOWN_GOOD`
  - `timeout => HOLD_LAST_KNOWN_GOOD`
  - only explicit valid disable intent may stop runtime.
- Apply to ingress now; phase in objectstore and other critical runtime consumers.

## Remaining Gap
- Roll this policy into other critical consumers (objectstore/envoy/dns runtime adapters) as separate follow-up work.

## Doctor
- `ingress.keepalived_disabled_without_explicit_spec`
- equivalent invariants for additional consumers as added.

## Tests
- Unit: missing key does not stop managed service.
- Integration: delete key while running => runtime remains active and degraded state is surfaced.

## DoD
- No critical runtime shuts down solely because a key is absent.
