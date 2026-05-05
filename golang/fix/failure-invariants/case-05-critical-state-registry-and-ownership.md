# Case 05: Critical State Registry And Ownership

## Status In Code
- PARTIAL: critical registry structs exist, but doctor is not fully wired to evaluate missing/invalid keys from registry entries.

## Target Invariant
- Every critical key has one authoritative owner, schema, delete policy, and healing policy.

## Required Implementation
- Create registry model (code + docs) for keys such as:
  - `/globular/ingress/v1/spec`
  - `/globular/objectstore/config`
  - `/globular/pki/ca`
  - release/BOM desired state
  - cluster membership desired state
- For each key define:
  - owner service
  - schema version
  - restore strategy
  - LKG consumer behavior
  - delete approval policy.

## Remaining To Reach DoD
- Wire registry into doctor collector iteration (source of truth for critical-key checks).
- Emit missing/stale/schema-mismatch findings automatically from registry definitions.
- Enforce owner/writer validation in controller writes for registry-managed keys.
- Add table-driven tests over registry entries to guarantee each key has complete policy fields.

## Files/Components
- New shared package under controller/health domain.
- Doctor collector uses registry for key-missing checks.

## Tests
- Unit: unknown writer detected and overwritten/rejected per policy.
- Integration: delete critical key => owner restores.

## DoD
- Critical key governance is declarative and enforceable.
