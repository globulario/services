---
id: doctor_remediation_requires_conclusive_finding_evidence
type: architecture_decision
status: accepted
summary: Cluster Doctor remediation authority is finding-scoped, not snapshot-global. Unrelated collector failures remain visible but do not veto a conclusive target finding. Conversely, no mutation may execute unless the finding verdict is INVARIANT_FAIL, CheckError is empty, and the finding evidence passes the existing provenance and freshness gate. PASS, PENDING, and UNKNOWN remain diagnostic truth only, including on a full-harvest snapshot.
invariants:
  - meta.harvest_and_yield_are_distinct_availability_dimensions
  - meta.fallback_must_degrade_semantics
  - evidence.provenance_trust_levels
related_services:
  - cluster-doctor
---

## Doctor Remediation Requires Conclusive Finding Evidence

A cluster snapshot is an observation envelope, not a single indivisible proof.
`DataIncomplete` means at least one collector failed. It must remain visible to
operators, but it cannot become a universal veto over independently supported
findings.

The authority boundary is the individual finding:

- when a finding's own evidence source failed, registry harvest policy downgrades
  its verdict to `INVARIANT_UNKNOWN` and records `CheckError`;
- when only unrelated collectors failed, the finding may retain a conclusive
  `INVARIANT_FAIL` verdict and continue through the normal execution gates;
- an instance-qualified source such as `node_agent@node-1/GetInventory` is
  distinct from the same RPC failing on another node;
- evidence freshness is necessary but not sufficient. A fresh `UNKNOWN`,
  `PENDING`, or `PASS` finding is not mutation authority.

### Execution contract

Every non-dry-run remediation path, whether initiated by the periodic healer or
an operator RPC, must enforce the same minimum closure:

1. `InvariantStatus == INVARIANT_FAIL`.
2. `CheckError == ""`.
3. Evidence is present and passes provenance/freshness classification.
4. Existing leader, behavioral-governance, blocklist, approval, cooldown,
   failure-rate, workflow, and audit gates still pass.

Dry-run remains available for rehearsal and audit even when verdict closure is
not satisfied, because it produces no mutation.

### Scar captured

The prior global `DataIncomplete -> observe` downgrade was safe but overly broad:
an unrelated Envoy, workflow, package-integrity, or other-node collection failure
could indefinitely suppress a target restart whose own inventory evidence was
fresh and conclusive.

Removing that global veto without a finding-level fail-closed check would create
the opposite defect. It could allow an indeterminate finding to mutate state.
The repair therefore couples scoped harvest eligibility with a universal
conclusive-failure verdict closure.

### Forbidden patterns

- Do not restore cluster-wide `DataIncomplete` as the sole remediation veto.
- Do not bypass verdict closure because individual evidence rows are fresh.
- Do not treat `UNKNOWN`, `PENDING`, or `PASS` as executable findings.
- Do not parse collector error strings to infer target scope; use structured
  instance-qualified service/RPC identity.
- Do not create a second execution path with weaker closure than
  `ExecuteRemediation`.