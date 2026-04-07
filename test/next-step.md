## Subject: V1 Validation Phase — Prove Recovery Semantics Under Failure

V1 is complete from a capability standpoint.

The goal now is not to add features, but to **prove that the system behaves correctly under real failure conditions** and produces **clear, explainable recovery traces**.

---

## Objective

Validate that:

1. Workflow resume works end-to-end with real actors
2. HA mechanisms behave correctly under multi-node failure
3. Receipts + verification + resume_policy produce correct decisions
4. Blocked-run states are surfaced and actionable
5. The system produces a **clear operational narrative** for every failure

---

## Phase 1 — End-to-End Resume Validation (Critical)

### Scenario

Simulate a real workflow interruption:

* Start `node.join` or `release.apply.infrastructure`
* Kill the executor mid-step (during a non-trivial action like Scylla or MinIO install)
* Ensure:

  * Run is claimed by another executor
  * Actor endpoints are re-resolved
  * Resume logic executes correctly

### Validate

* Correct resume decision:

  * skip / re-execute / block
* Verification result is used correctly
* No duplicate side effects
* Workflow completes or blocks safely

### Output Required

Provide a **timeline trace**:

* step execution
* executor death
* claim event
* verification result
* resume decision (with reason)
* final outcome

---

## Phase 2 — Receipt-Driven Behavior (Targeted)

### Goal

Prove that receipts are not just stored, but **actively used in decision-making**

### Task

Identify 1–2 steps where:

* `rerun_if_no_receipt` is appropriate
* verification alone is insufficient or ambiguous

Modify workflow YAML accordingly.

### Validate

* Receipt exists → step is skipped safely
* No receipt → step re-executes
* Behavior matches policy exactly

---

## Phase 3 — HA Drill Series (HA-5b → HA-5e)

### Scenarios

Run controlled failure drills:

* Kill leader during active workflows
* Stop Scylla temporarily
* Break MinIO access
* Simulate node partition

### Validate

* No orphaned runs remain
* No conflicting execution
* Resume decisions are consistent with policy
* System converges without manual intervention (unless blocked by design)

### Output Required

For each drill:

* summary of failure
* timeline of events
* doctor findings
* recovery path taken
* final state

---

## Phase 4 — Blocked-Run UX + Doctor Integration

### Goal

Ensure blocked states are **clear and actionable**

### Validate

* Doctor surfaces blocked workflows with:

  * step ID
  * reason
  * recommended action
* Operator can:

  * approve
  * trigger remediation workflow
* Resume proceeds correctly after approval

---

## Phase 5 — System Hygiene

### Tasks

* Resolve or remove offline nodes (dell, nuc)
* Ensure doctor reports are clean (no known-noise)
* Confirm resolver consistency across all services

---

## Deliverables

1. At least **one full recovery trace** (clean, readable, end-to-end)
2. Results of HA drills (with observations)
3. Confirmation of receipt-driven step behavior
4. List of any inconsistencies, edge cases, or unexpected behavior

---

## Success Criteria

The system is considered validated when:

* Every failure produces a clear, explainable decision chain
* Resume behavior matches defined policies
* No unsafe re-execution occurs
* Blocked states are surfaced instead of hidden
* The system converges or pauses safely — never blindly retries

---

## Key Principle

We are not testing if the system works.

We are testing if:

> **the system behaves correctly when reality breaks assumptions**

---

Proceed with Phase 1.
