# Workflow Hardening — Implementation Plan

**Status:** Design approved. Ready for phased implementation.

Based on `docs/architecture/workflow-hardening.md` and the current codebase state.

---

## What This Adds

Today the engine knows: step started, step finished, step depends on X.

After hardening the engine also knows: is replay safe, should I verify first, what proof counts as "already done", do I need approval to resume.

This turns resume from "re-execute everything that wasn't marked done" into "inspect the world, skip what's already true, re-execute only what's needed."

---

## Implementation Phases

### WH-1 — Schema extension (types + parser)

**Files:**
- `golang/workflow/v1alpha1/types.go` — add `Execution`, `Verification`, `Compensation` structs to `WorkflowStepSpec`
- `golang/workflow/v1alpha1/loader.go` — parse new fields from YAML (already YAML-tagged)
- `golang/workflow/compiler/types.go` — add compiled equivalents to `CompiledStep`
- `golang/workflow/compiler/compiler.go` — pass through new fields during compilation

**New types:**
```go
type StepExecution struct {
    Idempotency     string `yaml:"idempotency" json:"idempotency"`         // safe_retry | verify_then_continue | manual_approval | compensatable
    ResumePolicy    string `yaml:"resume_policy" json:"resume_policy"`     // retry | verify_effect | rerun_if_no_receipt | pause_for_approval | fail
    ReceiptKey      string `yaml:"receipt_key" json:"receipt_key"`
    ReceiptRequired bool   `yaml:"receipt_required" json:"receipt_required"`
}

type StepVerification struct {
    Actor   ActorType      `yaml:"actor" json:"actor"`
    Action  string         `yaml:"action" json:"action"`
    With    map[string]any `yaml:"with" json:"with"`
    Success VerifySuccess  `yaml:"success" json:"success"`
}

type VerifySuccess struct {
    Expr string `yaml:"expr" json:"expr"`
}

type StepCompensation struct {
    Enabled bool           `yaml:"enabled" json:"enabled"`
    Actor   ActorType      `yaml:"actor" json:"actor"`
    Action  string         `yaml:"action" json:"action"`
    With    map[string]any `yaml:"with" json:"with"`
}
```

**Test:** Existing YAML loading tests still pass. New YAML with `execution:` fields parses correctly.

**Commit point:** `wh-1-schema-extension`

---

### WH-2 — Engine resume-policy dispatch

**Files:**
- `golang/workflow/engine/engine.go` — update `ExecuteCompiled` resume path to read `PreCompleted` + step metadata
- `golang/workflow/engine/resume.go` (new) — resume logic: `resolveResumeAction(step, preStatus)` returns skip/retry/verify/pause/fail
- `golang/workflow/workflow_server/executor_resume.go` — wire metadata into resume decision

**Logic (from the design):**

```
For each step that was in-progress when executor crashed:
  switch step.Execution.ResumePolicy:
    "retry":
      → re-execute unconditionally
    "verify_effect":
      → run step.Verification action first
      → if verification.success expr is true: mark step SUCCEEDED, skip
      → if verification fails: re-execute the step
    "rerun_if_no_receipt":
      → check receipt_key in step outputs/persistence
      → if receipt exists: skip
      → if no receipt: verify_effect, then execute if needed
    "pause_for_approval":
      → mark step BLOCKED, set run status BLOCKED
      → stop execution (operator or AI must approve)
    "fail":
      → mark step FAILED, fail the run
    default (no metadata):
      → fall back to current behavior: re-execute (backward compatible)
```

**Key invariant:** Steps without `execution:` metadata behave exactly as today. The hardening is opt-in per step.

**Test:**
- `TestResumeRetryPolicy` — retry re-executes unconditionally
- `TestResumeVerifyEffectSkips` — verification proves effect exists → skip
- `TestResumeVerifyEffectReexecutes` — verification says absent → re-execute
- `TestResumeFallbackForLegacySteps` — no metadata → same as today

**Commit point:** `wh-2-resume-policy-dispatch`

---

### WH-3 — Verification action dispatch

**Files:**
- `golang/workflow/engine/engine.go` — add `runVerification(ctx, step, run)` method
- `golang/workflow/engine/engine.go` — evaluate `success.expr` against verification result

**How verification works:**
1. Engine looks up `step.Verification.Actor` + `step.Verification.Action` in the Router
2. Dispatches the verification action (same as a regular step but isolated)
3. Evaluates `success.expr` against the result output (using `DefaultEvalCond`)
4. Returns tri-state: `present` / `absent` / `inconclusive`

**Tri-state verification outcome:**

| Outcome | Meaning | Resume action |
|---------|---------|---------------|
| `present` | Effect exists, proof is clear | Mark step SUCCEEDED, skip |
| `absent` | Effect does not exist, safe to execute | Re-execute the step |
| `inconclusive` | Cannot determine — partial state, timeout, error | Depends on `idempotency`: `safe_retry` → re-execute; `verify_then_continue` → re-execute; `manual_approval` → pause for approval; `compensatable` → attempt compensation |

The engine MUST NOT treat inconclusive as present. Inconclusive means "the mirror is foggy" — the safe default is to be conservative based on the step's idempotency class.

**Important:** Verification actions are dispatched to the same actor endpoints as regular step actions. No new callback path needed — the existing `WorkflowActorService.ExecuteAction` handles them.

**Test:**
- `TestVerificationActionDispatched` — verification handler is called
- `TestVerificationExprEvaluation` — `result.ok == true` evaluated correctly
- `TestVerificationFailureTriggers Reexecution` — failed verification → step runs

**Commit point:** `wh-3-verification-dispatch`

---

### WH-4 — Verification action handlers

**New handlers to register (in `engine/actors*.go`):**

| Handler | Actor | Purpose | Implementation |
|---------|-------|---------|----------------|
| `node.verify_packages_installed` | node-agent | Check if packages are installed at expected version | Read installed_state from etcd, compare |
| `node.verify_installed_state_synced` | node-agent | Check if installed state matches reality | Compare etcd vs systemd |
| `node.verify_installed_package_state` | node-agent | Check specific package version/hash | Single package check |
| `controller.release.verify_status` | controller | Check release phase matches expected | Read release resource |
| `controller.release.verify_node_status` | controller | Check per-node release status | Read node entry in release |
| `controller.release.verify_terminal_status` | controller | Check release is in terminal state | Read release, check AVAILABLE/FAILED |
| `controller.bootstrap.verify_phase` | controller | Check node bootstrap phase | Read in-memory node state |

**Implementation pattern:** Each verification handler is a read-only check — no mutations. Returns `{status: "present"/"absent"/"inconclusive", ...detail}`.

**Dual-source verification for install/sync steps:**

Install and sync verification handlers MUST check both:
1. **Local reality** — package files present, systemd unit exists, service active/healthy
2. **Authoritative state** — etcd installed_state record matches expected version/hash

This prevents the exact failure class we care about: "side effect happened on node but sync to authoritative state didn't complete." Single-source verification (etcd only) would miss this gap.

Result matrix for install verification:

| Local reality | etcd state | Verdict |
|--------------|------------|---------|
| Package present + healthy | Synced at correct version | `present` |
| Package present + healthy | Not synced or wrong version | `inconclusive` (effect happened, sync pending) |
| Package absent | Any | `absent` |
| Package present + unhealthy | Any | `inconclusive` (partial install) |

**These handlers serve dual purpose:**
1. Resume verification during orphan recovery
2. Standalone verification steps in workflows (the `verify_installed` steps already exist)

**Test:** Each handler tested in isolation with mock state.

**Commit point:** `wh-4-verification-handlers`

---

### WH-5 — Apply metadata to node.join YAML

**File:** `golang/workflow/definitions/node.join.yaml`

Add `execution:` and `verification:` blocks to each step per the design doc example. No handler changes — the handlers from WH-4 provide the verification actions.

**Verify:** `TestEmbeddedWorkflowsHaveRegisteredActions` still passes (all verification actions are registered).

**Commit point:** `wh-5-node-join-hardened`

---

### WH-6 — Apply metadata to release.apply.infrastructure YAML

**File:** `golang/workflow/definitions/release.apply.infrastructure.yaml`

Same pattern as WH-5.

**Commit point:** `wh-6-release-infra-hardened`

---

### WH-7 — Apply metadata to remaining workflows

**Files:**
- `release.apply.package.yaml`
- `release.remove.package.yaml`
- `node.bootstrap.yaml`
- `node.repair.yaml`
- `cluster.reconcile.yaml`
- `remediate.doctor.finding.yaml`
- `day0.bootstrap.yaml`

Each gets appropriate `execution:` metadata. Read-only steps get `safe_retry`. Install/sync steps get `verify_then_continue`. State mutations get `verify_effect`.

**Commit point:** `wh-7-all-workflows-hardened`

---

### WH-8 — Receipt persistence (optional, high value)

**Files:**
- `golang/workflow/workflow_server/executor.go` — after step completion, write receipt if `receipt_key` is configured
- `golang/workflow/workflow_server/schema.go` — add `workflow_step_receipts` ScyllaDB table
- `golang/workflow/workflow_server/executor_resume.go` — check receipt before verification

**Receipt table:**
```sql
CREATE TABLE IF NOT EXISTS workflow.step_receipts (
    run_id      text,
    step_id     text,
    receipt_key text,
    result_json text,
    created_at  bigint,
    PRIMARY KEY (run_id, step_id)
)
```

**This is optional but high-value:** receipts provide a durable breadcrumb that survives executor crash. Without them, verification must re-query the world. With them, the engine can short-circuit to "already done" immediately.

**Commit point:** `wh-8-receipt-persistence`

---

## Execution Order Rationale

1. **WH-1 (schema)** first — types must exist before anything can use them
2. **WH-2 (resume dispatch)** — the resume brain, depends on WH-1
3. **WH-3 (verification dispatch)** — verification mechanism, depends on WH-2
4. **WH-4 (verification handlers)** — implements the checks WH-3 dispatches
5. **WH-5/6 (critical YAMLs)** — node.join and infra release first (highest risk)
6. **WH-7 (remaining YAMLs)** — broaden coverage
7. **WH-8 (receipts)** — optional but valuable durability upgrade

Each phase has its own commit point. No giant batches.

---

## Step Classification Matrix (for workflow authors)

| Step type | Idempotency | Resume policy | Verification |
|-----------|-------------|---------------|--------------|
| Read-only probe | `safe_retry` | `retry` | Health/status check |
| Package install | `verify_then_continue` | `verify_effect` | Local package + runtime + sync state (dual-source) |
| Status mark/update | `safe_retry` | `verify_effect` | Authoritative phase/status |
| State sync | `verify_then_continue` | `verify_effect` | Local reality + authoritative (dual-source) |
| Service restart | `verify_then_continue` | `verify_effect` | Runtime health probe |
| Destructive removal | `manual_approval` | `pause_for_approval` | Confirm target removed |
| External publish/bootstrap | `verify_then_continue` | `verify_effect` | Repo/object/state proof |
| Aggregation/classification | `safe_retry` | `retry` | None needed |

This prevents every step from getting the same metadata pasted on it.
Use this table as the authority when annotating workflow YAMLs.

---

## What Doesn't Change

- Normal execution path — steps run exactly as today
- Action handler contracts — no changes to existing handlers
- Workflow centralization — ExecuteWorkflow still drives everything
- Actor callback model — verification uses the same dispatch path
- Existing tests — all pass unchanged (new fields are optional)

---

## Interaction With HA-4 (Run Ownership)

The orphan scanner claims stale runs and calls `ResumeRun`. Today, `ResumeRun` uses `PreCompleted` to skip completed steps and re-executes RUNNING steps blindly.

After WH-2/WH-3, `ResumeRun` reads the step's `resume_policy` and dispatches accordingly:
- `retry` → same as today (re-execute)
- `verify_effect` → run verification first, skip if effect exists
- `pause_for_approval` → block the run, emit finding for operator

This makes orphan-run resume **fact-based instead of mood-based**.
