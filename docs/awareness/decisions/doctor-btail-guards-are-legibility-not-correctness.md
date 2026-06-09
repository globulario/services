---
id: doctor_btail_guards_are_legibility_not_correctness
type: architecture_decision
status: accepted
summary: The ~55 doctor rules still flagged by doctor_rule_evaluate_must_consult_snap_errors are behaviorally covered by two layers (the registry source-unavailable findings and the TestNoRuleEmitsConfidentFailureOnErroredSnapshot ratchet). Adding per-rule snap.HadError guards to them is legibility/scanner-count only, not a correctness fix, and is intentionally deferred — several of these rules already implement carefully-tuned partial-snapshot handling that a blanket guard could disturb.
invariants:
  - doctor_rule_evaluate_must_consult_snap_errors
  - meta.harvest_and_yield_are_distinct_availability_dimensions
  - objectstore.partial_snapshot_unknown_not_down
related_services:
  - cluster-doctor
---

## Doctor B-Tail Guards Are Legibility, Not Correctness

`doctor_rule_evaluate_must_consult_snap_errors` flags ~55 rule `Evaluate`
methods that read a `Snapshot` field without consulting an error token
(`snap.HadError` / a dedicated `*LoadError`). This decision records why that
non-zero count is **by design** and why the remaining sites are intentionally
not individually guarded.

### The bug class is already covered by two layers

1. **FALSE_POSITIVE** (a rule emits a confident finding on errored/empty data):
   gated by `TestNoRuleEmitsConfidentFailureOnErroredSnapshot`, a registry-wide
   CI ratchet that proves no rule emits a confident `INVARIANT_FAIL` on a
   fully-errored snapshot. By construction, none of the flagged sites can
   produce a false confident finding.

2. **FALSE_NEGATIVE** (a rule whose only source errored emits nothing, masking
   an outage): covered structurally by the registry's
   `snapshotSourceUnavailableFindings`, which emits one `INVARIANT_UNKNOWN`
   finding per errored source regardless of which rule went silent.

So a per-rule `snap.HadError` guard on a flagged site changes **no behavior** —
it only makes the per-function scanner recognize the rule as conformant and
drop the drift count.

### Why blanket guards are not safe to grind

Several flagged rules already implement deliberate partial-snapshot handling.
The objectstore rules enforce the critical invariant
`objectstore.partial_snapshot_unknown_not_down` ("missing inventory must be
treated as unknown, not known-down") and gate their per-node logic on
`snap.DataIncomplete`. Adding a blanket top-level guard to these could suppress
findings the rule is carefully designed to still produce on a *partial* (not
fully-errored) snapshot. The risk/value ratio is bad: real risk of disturbing
tuned behavior, zero behavior gain.

### What WOULD warrant a guard

Add a `snap.HadError(service, rpc)` (or dedicated `*LoadError`) guard to a
specific rule only when:

- the rule's verdict depends on a **single** source, AND
- reading that source empty would otherwise produce a misleading finding that
  the registry/ratchet layers do not already neutralize.

That is a per-rule judgment, made when touching the rule for another reason —
not a mechanical sweep.

### Forbidden pattern

- Do not add blanket `snap.HadError` guards across the B-tail purely to drive
  the scanner count to zero; that trades real (if small) regression risk for a
  cosmetic number, and erodes the tuned partial-snapshot behavior the doctor
  rules already implement.
