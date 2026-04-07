## Subject: Workflow Hardening — Missing Capabilities & Integration Plan

We’ve implemented (or are implementing) workflow hardening with:

* step-level `execution` metadata (idempotency, resume_policy)
* verification actions
* resume dispatch logic
* application to critical workflows (node.join, release.apply.infrastructure)

This gives us **intent-aware execution and safer resume semantics**.

---

## Goal

Identify **what is still missing to complete the system** so that:

1. Resume is fully reliable under partial failure
2. Failures are **explicit, explainable, and auditable**
3. The system can **reason about past actions**, not just current state
4. Operator and AI can **understand and safely intervene**

---

## Missing Capabilities (Design-Level)

### 1. Step Receipts (Durable Execution Memory)

**Problem:**
Verification alone is not always sufficient to determine if a step already executed successfully, especially under:

* executor crash mid-step
* partial side effects
* inconsistent external state

**Needed:**
Introduce **step receipts**:

* durable record written after step completion
* keyed by `(run_id, step_id, receipt_key)`
* contains:

  * action inputs
  * observed result
  * timestamp
  * optional verification snapshot

**Questions:**

* Where should receipts be stored? (ScyllaDB table proposed)
* Should they be written by executor or via callback?
* How do receipts integrate with `rerun_if_no_receipt`?

**Impacted services:**

* workflow executor
* workflow persistence (ScyllaDB)
* possibly node-agent (if local receipts are useful)

---

### 2. Verification Semantics (Stronger Truth Model)

**Problem:**
Current verification returns boolean-like success, but real-world states include:

* present
* absent
* inconsistent / inconclusive

**Needed:**
Extend verification result model:

* `{ status: present | absent | inconclusive, details: ... }`

Update resume behavior:

* present → mark complete
* absent → re-execute
* inconclusive → pause or escalate (based on idempotency)

**Questions:**

* Should verification return a structured result instead of relying only on `success.expr`?
* How should inconclusive be surfaced in workflow state?

**Impacted services:**

* workflow engine (evaluation logic)
* actor handlers (verification responses)
* doctor (interpretation)

---

### 3. Operational Timeline / Narrative Layer

**Problem:**
Current logs/events are fragmented. We lack a **coherent execution story**.

**Needed:**
A structured timeline per run:

* step start / end
* resume events (executor crash, claim)
* verification decisions
* resume decisions (retry / skip / pause)
* final outcomes

**Goal:**
Make failures explainable as a **sequence of decisions**, not raw logs.

**Questions:**

* Should this be derived from WorkflowRun + WorkflowStep records?
* Do we need a new “event” model or projection layer?

**Impacted services:**

* workflow service (event emission)
* event service (if reused)
* globular-admin UI

---

### 4. Doctor ↔ Workflow Integration

**Problem:**
Doctor detects issues, but does not fully leverage workflow semantics.

**Needed:**
Doctor should:

* understand step metadata (idempotency, resume_policy)
* inspect failed or blocked runs
* produce findings like:

  * “Step X paused due to inconclusive verification”
  * “Safe action: run repair workflow”
* optionally trigger remediation workflows

**Questions:**

* Should doctor read WorkflowRun state directly or via projection?
* How do we map findings to specific workflow steps?

**Impacted services:**

* cluster-doctor
* workflow service
* possibly AI executor

---

### 5. Source-of-Truth Clarification (Verification Contract)

**Problem:**
Verification depends on consistent authoritative sources.

**Needed:**
Define clearly per domain:

* installed state → etcd / node-agent
* runtime health → probes
* workflow state → ScyllaDB
* artifact truth → repository / MinIO

Enforce:

* verification must read from authoritative source(s)
* avoid relying on a single potentially stale source

**Questions:**

* Do we need a formal “verification contract” per action?
* Should verification handlers aggregate multiple signals?

**Impacted services:**

* node-agent
* cluster-controller
* repository
* monitoring / probes

---

### 6. Safety Rails / Policy Layer

**Problem:**
System may take unsafe automatic actions in ambiguous cases.

**Needed:**
Policy constraints:

* block destructive actions without approval
* limit retries or loops
* enforce cluster-wide safety boundaries

**Questions:**

* Should this live in workflow engine or as a separate policy service?
* How is “manual approval” surfaced and resolved?

**Impacted services:**

* workflow engine
* cluster-doctor (for escalation)
* UI / admin interface

---

### 7. Failure Injection / Validation Strategy

**Problem:**
We need proof that the system behaves correctly under failure.

**Needed:**
Test scenarios:

* executor crash mid-step
* Scylla partial failure
* MinIO unavailable
* network partition

**Goal:**
Validate:

* resume behavior matches `resume_policy`
* verification drives correct decisions

**Questions:**

* Should we build a dedicated chaos workflow?
* Or integrate failure injection into existing test harness?

**Impacted services:**

* workflow engine
* test infrastructure
* node-agent (for fault injection hooks)

---

## What Already Exists (Validate Before Building)

Before implementing new components, confirm whether we already have:

* partial receipt-like data in WorkflowRun / Step outputs
* sufficient event history for timeline reconstruction
* doctor access to workflow state
* verification handlers that already return rich data

---

## Deliverables Requested

Please:

1. Map each missing capability to:

   * existing components (if partially implemented)
   * required changes
   * new components (if needed)

2. Identify:

   * which services are impacted
   * whether changes are additive or require refactoring

3. Propose:

   * minimal implementation path (phased, like WH-1 → WH-8)
   * risks and edge cases

4. Highlight:

   * anything in current design that conflicts with these additions

---

## Key Principle

We are not adding features for completeness.

We are ensuring that:

> **every workflow step can be resumed safely, explained clearly, and verified against reality**

---

## End Goal

A system where:

* failures are not ambiguous
* recovery is not guesswork
* the system can explain **what happened and why it chose its next action**

---

Please analyze this against the current implementation and propose how to integrate cleanly.
