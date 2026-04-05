## Globular Convergence Model — AI Operational Discipline

### 1. Core Principle

Globular is not a feature platform.
Globular is a **convergence system**.

Its purpose is:

> Maintain the cluster in a state where **actual state matches intended state**, continuously.

All components — services, workflows, AI — exist to serve that goal.

---

### 2. The Evolution Problem

Initially, AI interacted with the cluster in **compensation mode**:

* Missing structure → infer context
* Missing state → reconstruct from logs
* Missing workflows → improvise fixes

This led to:

* High token usage
* Repeated reasoning
* Fragile fixes
* Inconsistent behavior

AI was *surviving the system*, not operating it.

---

### 3. The Architectural Shift

We are transitioning to **operational mode**:

AI must no longer compensate for missing structure.

Instead, the system provides:

* Structured projections (NodeIdentity, pkg_info, etc.)
* Clear sources of truth
* Explicit workflows
* Typed remediation paths

AI becomes:

> A **system operator**, not a guesser.

---

### 4. The Workflow vs Plan Boundary (CRITICAL)

This is the most important invariant in the system.

#### Workflow

A workflow defines:

* Intent
* Multi-step orchestration
* Dependencies
* Ordering
* Validation
* Risk and approval requirements

Workflows are:

* The **source of truth**
* Long-lived
* Observable
* Extensible

---

#### Plan

A plan is:

* A **compiled execution unit**
* Targeting a single node or actor
* Immediate and atomic
* Stateless and disposable

A plan contains:

* Concrete actions only (e.g., write file, restart service)

---

### 5. Non-Negotiable Rule

> **Plans must never contain workflow semantics.**

If a “plan” includes:

* Multiple phases
* Cross-node coordination
* Conditional branching
* Waiting logic
* Retry strategies
* Approval flows

Then it is **not a plan**.
It is a **workflow disguised as a plan**, and must be rejected.

---

### 6. AI Role and Constraints

AI is allowed to:

* Resolve cluster context (NodeIdentity, pkg_info)
* Detect invariant violations
* Diagnose causes
* Propose workflows
* Validate existing plans

AI is NOT allowed to:

* Execute arbitrary actions directly
* Bypass workflows
* Embed orchestration logic inside plans
* Invent implicit behavior not represented in the system

---

### 7. Execution Flow (Canonical Model)

```
AI / Operator
    ↓
Workflow (intent + orchestration)
    ↓
Workflow compiler
    ↓
Plans (per-node execution units)
    ↓
Node-agent executes
    ↓
State updated
    ↓
Reconciliation verifies convergence
```

---

### 8. Extensibility Rule

When a pattern repeats:

> It must become a workflow.

Not:

* a bigger plan
* a smarter plan
* a special-case patch

This ensures the system evolves structurally, not chaotically.

---

### 9. Anti-Pattern to Reject

```
"Plan":
  - check all nodes
  - fix credentials
  - wait for quorum
  - restart services
```

This is NOT a plan.

This is a workflow and must be defined as such.

---

### 10. Desired Outcome

The system must evolve toward:

* Deterministic reasoning
* Minimal token usage
* Explicit architecture
* Reusable workflows
* Observable execution

AI should operate like:

> A deterministic control-plane agent enforcing convergence

—not—

> A reactive debugger improvising fixes

---

### 11. Final Rule

> If the task requires thinking about sequence, coordination, or conditions, it is a workflow.

> If the task is a direct action on a single node, it is a plan.

Maintain this boundary at all times.

Violation of this rule will reintroduce:

* hidden complexity
* non-determinism
* system instability

---

End of discipline.
