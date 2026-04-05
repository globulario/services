## Directive: Implement the Remediation Workflow (Convergence Engine)

### Objective

You must implement a **first-class remediation workflow** that restores cluster convergence from structured state, without relying on ad-hoc reasoning or implicit behavior.

This workflow becomes the **primary operational path** for AI-driven actions.

---

### 1. Core Model

All operations must follow this pipeline:

```
resolve → project → diagnose → remediate → compile → execute → verify
```

Definitions:

* **Resolve**: Identify node/context (NodeIdentity)
* **Project**: Load structured state (pkg_info, services, infra)
* **Diagnose**: Detect invariant violations
* **Remediate**: Select a valid remediation strategy
* **Compile**: Convert remediation → workflow → plans
* **Execute**: Node-agents apply plans
* **Verify**: Ensure convergence is restored

---

### 2. Non-Negotiable Constraints

#### 2.1 Workflow vs Plan Boundary

* Workflows define orchestration, sequencing, dependencies
* Plans define **single-node, immediate actions only**

You MUST NOT:

* embed orchestration logic inside plans
* generate multi-phase or cross-node plans
* simulate workflows using plan structures

If logic requires:

* sequencing
* branching
* coordination
* waiting
* retries

→ it MUST be implemented as a workflow

---

#### 2.2 AI Behavior Rules

You MUST:

* operate only through structured projections
* detect drift via state mismatch (not logs alone)
* propose remediation as workflows
* validate risk before execution

You MUST NOT:

* execute shell-like or direct actions
* bypass workflow system
* invent hidden or implicit steps
* compensate for missing structure with guesswork

---

### 3. Workflow Definition (Initial Version)

Create a workflow:

```
name: remediate-drift
```

With steps:

1. resolve_context
2. load_projections
3. evaluate_invariants
4. detect_violations
5. diagnose_cause
6. select_remediation
7. assess_risk
8. require_approval (if risk ≥ threshold)
9. compile_to_plans
10. execute_plans
11. verify_convergence

Each step must be:

* observable
* logged
* attributable to a workflow run

---

### 4. Remediation Model

Define remediation as structured data:

* target (service / node / resource)
* cause (classification)
* actions (high-level, not plan-level)
* risk level (LOW / MEDIUM / HIGH)
* required approvals

Remediation must NOT directly contain execution commands.

---

### 5. Plan Compilation

Plans must be generated **only from workflow steps**.

Each plan must:

* target a single node
* contain only atomic actions
* be idempotent
* be disposable after execution

---

### 6. Extensibility Rule

When a remediation pattern repeats:

* promote it to a reusable workflow or sub-workflow

You MUST NOT:

* expand plans to handle new complexity
* introduce special-case execution paths

---

### 7. Success Criteria

This implementation is successful when:

* AI can restore a broken service using only this workflow
* no direct or ad-hoc execution paths are used
* plans remain simple and consistent
* workflows are fully observable end-to-end
* repeated issues produce reusable remediation patterns

---

### 8. Immediate Action

Begin implementation with:

1. Define workflow structure in code (proto + service)
2. Implement `remediate-drift` workflow skeleton
3. Integrate NodeIdentity + pkg_info projections
4. Add invariant evaluation (basic: service running / desired vs actual)
5. Implement simple remediation case (e.g., restart service)
6. Compile to plan and execute through node-agent
7. Verify convergence

Do not expand scope beyond this initial loop.

---

### Final Rule

> If you are about to generate a plan that requires reasoning about sequence or coordination, stop.
> You are implementing a workflow, not a plan.

Proceed with discipline.

