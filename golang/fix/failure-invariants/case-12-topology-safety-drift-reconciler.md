# Case 12: Topology Safety In Drift Reconciler

## Status In Code
- PARTIAL: topology preflight exists for remove-node RPC, but drift reconciler does not consistently call safety preflight and objectstore topology checks are incomplete.

## Target Invariant
- Drift reconcile must never apply topology/runtime changes that violate known safety constraints.

## Required Implementation
- Add topology preflight checks before applying drift plans:
  - storage quorum viability
  - ingress participant validity
  - objectstore topology consistency with desired contract
  - controller profile/role placement sanity
- If unsafe:
  - block only unsafe action
  - emit lane `DEGRADED` + specific doctor invariant
  - continue other safe reconciles.

## Remaining To Reach DoD
- Call topology preflight in drift reconciler action planner (not only direct RPC path).
- Add objectstore topology contract checks (participant existence, expected quorum layout, stale-node detection).
- Add lane-level doctor finding with remediation when drift action is denied by safety gate.

## Doctor
- Add/extend invariants for topology safety denial with remediation hints.

## Tests
- Integration: stale/minority topology proposal is rejected while unrelated reconcile actions continue.
- Unit: per-action safety gate decisions are deterministic.

## DoD
- Drift reconciler becomes safety-aware, not purely convergent.
