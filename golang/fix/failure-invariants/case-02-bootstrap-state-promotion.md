# Case 02: Bootstrap State Promotion

## Status In Code
- PARTIAL: `authoritative`/source markers exist and consumers warn, but promotion workflow is not complete.

## Target Invariant
- Temporary/bootstrap values must never be promoted as authoritative desired state without ownership and validation.

## Required Implementation
- Add explicit bootstrap markers in desired-state records: `source=bootstrap`, `authoritative=false`.
- Require controller promotion pass to convert bootstrap -> authoritative with:
  - schema validation
  - checksum/generation rewrite
  - `writer_leader_id` stamp
- Consumers must treat bootstrap records as non-final.

## Remaining To Reach DoD
- Implement controller promotion reconciler:
  - scan bootstrap-marked critical desired-state records
  - validate schema and ownership
  - write authoritative replacement with new generation/checksum/writer
- Block non-authoritative records from being considered converged state in doctor.
- Add integration test: first boot with bootstrap records only must not claim convergence until promotion runs.

## Files/Components
- Controller desired-state publishers (ingress/objectstore/etc.).
- Shared model structs for desired-state metadata.
- Node-agent consumer validation path.

## Tests
- Unit: bootstrap record is ignored as final by consumers.
- Integration: first-boot cluster converges only after controller promotion.

## DoD
- Bootstrap defaults cannot accidentally become permanent cluster intent.
