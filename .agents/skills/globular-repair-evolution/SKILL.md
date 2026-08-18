---
name: globular-repair-evolution
description: Use for Globular incident repair, simulation-discovered defects, feature implementation, architecture evolution, scenario creation, adversarial cluster testing, autonomous engineering, and behavioral learning from execution. Enforces the simulation-first governed repair/evolution lifecycle.
---

# Globular Repair and Evolution

Canonical strategy: `docs/design/autonomous-repair-evolution-strategy.md`.

Core law:

> Novel repair and evolution must be proven against a reproducible model of the system before they may alter the authoritative system.

Route work through:

```text
CLASSIFY
  -> GROUND CONTRACT + AUTHORITY
  -> CHANGE ENVELOPE + PROOF PLAN
  -> BIND EXACT CANDIDATE + FREEZE PLAN DIGEST
  -> ISOLATED CANDIDATE
  -> DECLARED EXACT-COMMAND LOCAL TESTS
  -> BOUND QUICKSTART SCENARIOS
  -> PROVEN
  -> SENSEI ADMISSION
  -> IMMUTABLE RELEASE
  -> PRODUCTION VERIFICATION
  -> BEHAVIORAL LEARNING
```

Author governing contracts, invariants, forbidden repairs, required tests, and required scenarios **before** binding the candidate. Candidate binding freezes those obligations into `plan_digest`. Changing the proof plan afterward invalidates the candidate envelope; never delete or weaken a failing obligation to recover green.

Executable pre-admission tools:

```text
evolution-change init
  mint a DRAFT ChangeEnvelope

evolution-change bind-candidate
  bind exact repository + git revision and freeze plan_digest; rebinding clears stale evidence

evolution-run-test
  execute one exact command frozen in the envelope after verifying checkout HEAD

evolution-run-scenario
  execute one exact-candidate + exact-plan quickstart proof scenario

evolution-prove
  rerun all required local tests first, then required scenarios, stopping on failure

evolution-change status
  show candidate, plan_digest, missing/failed proof, and next authority boundary

evolution-change validate
  validate envelope and print identity digest

simulation-learning-ingest
  ingest quickstart evidence into Behavioral Memory without promotion capability
```

`evolution-prove` may advance only to **PROVEN**. Do not add a convenience command that self-asserts Sensei `ACCEPT`, release success, production verification, or Behavioral Memory promotion.

Required local tests freeze their exact command. A PASS record from a substituted command is invalid. Candidate checkout `git HEAD` must equal the envelope candidate revision. Test evidence must carry the same `plan_digest`. Failed reruns may downgrade PROVEN back to CANDIDATE.

Cluster simulation begins only after required local tests are green. Governed quickstart uses:

```text
GLOBULAR_CHANGE_ID
GLOBULAR_CHANGE_ENVELOPE_REF
GLOBULAR_CANDIDATE_REPOSITORY
GLOBULAR_CANDIDATE_REVISION
GLOBULAR_CHANGE_PLAN_DIGEST
GLOBULAR_REQUIRE_CHANGE_BINDING=1
```

Quickstart proof is bound to the change id, candidate repository/revision, frozen plan digest, quickstart simulation revision, and the identity of the single invocation that produced it. Unsupported required actions/probes are not PASS.

Services mints that invocation identity, creates the run directory for it, passes both to the harness as `GLOBULAR_PROOF_INVOCATION_ID`/`GLOBULAR_PROOF_RUN_DIR`, and reads the proof back only from there. The `reports/latest` symlink is never a proof source: it names whichever run rotated it last, so a rerun that dies early, or a concurrent run for the same change, could otherwise hand back an earlier PASS. A run that produces no valid proof of its own withdraws the standing claim instead of leaving it at PROVEN.

Current semantic chaos includes `chaos.pause_service`/`chaos.resume_service` using SIGSTOP/SIGCONT. Do not claim the zombie-controller invariant proven until a real owner-path generation/fencing mutation attempt can independently prove the stale controller was rejected.

Use `sensei-architect` to discover/challenge contracts, ownership, invariants, failure modes, and proof. Use `sensei-admission` for the exact mutation boundary, and `sensei-closure` when admission cannot be established. Sensei acceptance must bind both the candidate revision and frozen plan digest.

Known governed operational remedies may run through their typed production workflows. Novel code/schema/workflow/architecture repair must happen in an isolated engineering environment. Never create an AI hot-patch path into production.

Simulation may write observations/evidence/outcomes and candidate behavioral knowledge autonomously. Change-bound learning must carry the same candidate revision, plan digest, and simulation revision as the proof. It must remain `production_authoritative=false`, `promotion_required=true`, and `may_promote=false`. The simulation ingestion interface intentionally has no promotion method.

When production and simulation materially disagree, record a fidelity defect instead of hiding the discrepancy.
