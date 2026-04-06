# Missing Capabilities — Analysis Against Current Implementation

Response to `missing-capabilities.md`. Maps each gap to what exists, what's
needed, and the minimal path forward.

---

## 1. Step Receipts (Durable Execution Memory)

### What already exists
- **WorkflowStep records in ScyllaDB** — every step's status, timing, error,
  and `details_json` (output) are persisted by the `executionRecorder` during
  centralized execution. This is already a partial receipt.
- **`receipt_key` field in step schema** (WH-1) — defined, parsed, compiled.
  Not yet written to storage.
- **`rerun_if_no_receipt` resume policy** (WH-2) — dispatch logic exists,
  checks `run.Outputs[receipt_key]`. Falls through to verification if absent.

### What's needed
- **WH-8: `step_receipts` ScyllaDB table** — already designed in the
  implementation plan. Schema:
  ```sql
  CREATE TABLE workflow.step_receipts (
      run_id text, step_id text, receipt_key text,
      result_json text, created_at bigint,
      PRIMARY KEY (run_id, step_id)
  )
  ```
- **Write receipt after step success** when `receipt_key` is configured.
  ~20 lines in `executor.go` after step completion.
- **Read receipt in `ResumeRun`** before running verification. ~10 lines
  in `executor_resume.go`.

### Answers to questions
- **Where stored?** ScyllaDB (same keyspace as workflow_runs/steps). Colocated.
- **Written by executor** — the `OnStepDone` callback already has access to
  step output. Just add a conditional write.
- **Integration with `rerun_if_no_receipt`** — already wired in WH-2's
  `resolveResumeAction`. Just needs the persistence backend.

### Impact: Small. Additive. One table + ~30 lines of code.

---

## 2. Verification Semantics (Stronger Truth Model)

### What already exists — DONE
- **Tri-state verification** is already implemented (WH-2):
  `VerifyPresent` / `VerifyAbsent` / `VerifyInconclusive`
- **`runVerification()`** dispatches the action, evaluates `success.expr`,
  returns the tri-state outcome.
- **Inconclusive handling** per idempotency class:
  `safe_retry` → re-execute, `manual_approval` → block.
- **Verification handlers** (WH-4) return structured `{status: "present"/"absent"/"inconclusive", ...detail}`.

### What's needed — Nothing
This is complete. The implementation matches the design exactly.

### Answer to questions
- **Structured result instead of `success.expr`?** The handlers already return
  structured maps. The `success.expr` evaluates against that structure. Both
  work together — expr for the engine, structured output for auditing.
- **How is inconclusive surfaced?** Step is marked FAILED with explicit
  error message: "verification inconclusive and step requires manual approval."
  Visible in `workflow_steps` table and `WatchRun` stream.

### Impact: Zero. Already done.

---

## 3. Operational Timeline / Narrative Layer

### What already exists
- **`workflow_runs` + `workflow_steps`** in ScyllaDB — every run's start,
  finish, status, every step's start, finish, status, error, `details_json`.
- **`workflow_events`** table — append-only event stream per run.
- **`WatchRun` / `WatchNodeRuns`** streaming RPCs — live workflow monitor.
- **`GetRun` / `GetRunEvents`** RPCs — read run + event history.
- **Auto-recording** in centralized executor — every step done fires
  `OnStepDone` which writes to ScyllaDB.

### What's needed
- **Resume-specific events** — when the orphan scanner claims a run and
  resumes it, emit events: `workflow.run.resumed`, `workflow.step.verification_skipped`,
  `workflow.step.verification_reexecuted`, `workflow.step.blocked_for_approval`.
  ~15 lines each in `resume.go` and `executor_resume.go`.
- **Decision annotations in step records** — add `resume_decision` field to
  `workflow_steps` (or use `details_json`): "skipped: verification=present",
  "re-executed: verification=absent", "blocked: inconclusive+manual_approval".
  Already natural to add in the `resolveResumeAction` path.

### What's NOT needed
- No new event model. The existing `workflow_events` + `workflow_steps` tables
  are sufficient.
- No new projection layer. `GetRun` + `GetRunEvents` already provides the
  timeline.
- The narrative is reconstructable from: run status transitions + step status
  transitions + step `details_json` + workflow events.

### Impact: Small. ~50 lines of event emission in resume paths.

---

## 4. Doctor ↔ Workflow Integration

### What already exists
- **Doctor findings** have structured `RemediationAction` with action type,
  risk, parameters.
- **`remediate.doctor.finding` workflow** executes the full pipeline:
  resolve → assess → approve → execute → verify convergence.
- **`StartRemediationWorkflow` RPC** delegates to centralized WorkflowService.
- **Doctor telemetry invariants** (`workflow_telemetry.go`) already monitor:
  step failure rates, drift stuck cycles, no-activity periods.
- **Workflow run history** is queryable via `ListRuns` / `GetRun`.

### What's needed
- **New doctor invariant: blocked/paused workflow runs** — scan `workflow_runs`
  for runs with status=BLOCKED (from `pause_for_approval`). Produce finding:
  "Workflow X paused: step Y requires approval after inconclusive verification."
  ~30 lines in a new rule in `rules/workflow_blocked.go`.
- **Finding-to-step mapping** — the finding already carries `finding_id`.
  The `StartRemediationWorkflow` response includes `run_id`. The chain is:
  finding → workflow run → steps. Already queryable.

### What's NOT needed
- Doctor does NOT need to understand step metadata directly. It reads
  workflow run state (status, error message) which already encodes the
  resume decision.
- No new projection. Doctor reads from ScyllaDB `workflow_runs` via the
  existing collector's workflow client.

### Impact: Small. One new invariant rule (~30 lines).

---

## 5. Source-of-Truth Clarification (Verification Contract)

### What already exists — DONE
- **Schema reference** (Phase 6) documents every etcd key's writer and readers.
  14 entries with ownership, invariants, and source file references.
- **Dual-source verification** is defined in the hardening plan: install/sync
  handlers MUST check both local reality AND authoritative state.
- **`NodeVerificationConfig`** has `VerifyPackagesInstalled`,
  `VerifyInstalledStateSynced`, `VerifyPackageState` — all designed for
  dual-source checking.
- **Freshness contracts** (Phase 3) ensure every read surface declares its
  source and age.

### What's needed
- **Formal per-action verification contract** — document in each verification
  handler's comment: "checks: [local_files, systemd_unit, etcd_installed_state]".
  This is documentation, not code.
- **Nothing structural changes.** The design already mandates dual-source
  verification for install/sync steps.

### Impact: Documentation only. Zero code changes.

---

## 6. Safety Rails / Policy Layer

### What already exists
- **Hard blocklist** in doctor's `ActionExecutor` — ETCD_PUT, ETCD_DELETE,
  NODE_REMOVE are never auto-executable.
- **Risk gating** — RISK_HIGH and RISK_MEDIUM require approval tokens.
- **`manual_approval` idempotency class** + `pause_for_approval` resume
  policy — engine blocks and fails the step.
- **Workflow semaphore** — controller limits concurrent release workflows to 3.
- **Structured actions only** — no free-form shell. Every action is typed,
  audited, risk-gated.

### What's needed
- **Blocked-run surfacing** — when a run hits `pause_for_approval`, the run
  should be queryable as BLOCKED (not just FAILED). This requires:
  - Add `RUN_STATUS_BLOCKED` to the run status enum (already defined in
    proto: `RUN_STATUS_BLOCKED = 6`).
  - Update resume dispatch to set run status to BLOCKED instead of failing
    the step.
  - Add `ApproveBlockedRun` RPC or CLI command to unblock.
  ~50 lines in engine + ~20 lines in workflow service.

- **Retry/loop limits** — already handled by step `retry.maxAttempts` and
  `timeout`. No additional mechanism needed.

### Impact: Medium. Blocked-run surfacing is the main work.

---

## 7. Failure Injection / Validation Strategy

### What already exists
- **Test strategy** (`test/strategy.md`) defines L1-L4 test layers and
  convergence drills (7 drills defined).
- **Track A-H test tracks** cover workflow centralization, endpoint
  resolution, freshness, identity, pkg_info, schema, state alignment,
  remediation.
- **149 unit/integration tests** passing across all tracks.
- **HA-5a live validation** — DNS service stop/restart drill completed
  with measured results.

### What's needed
- **Drill execution** — run the 7 convergence drills from the test strategy
  on the live cluster. This is operational work, not code.
- **Chaos workflow** — optional but valuable. A workflow that:
  1. Stops a service
  2. Waits for doctor to detect it
  3. Verifies remediation workflow fires
  4. Checks convergence
  ~100 lines of YAML + test harness.

### Impact: Operational (drills) + optional small code (chaos workflow).

---

## Summary: What's Actually Missing vs Already Done

| Capability | Status | Effort |
|-----------|--------|--------|
| 1. Step receipts | **Designed, not implemented** (WH-8) | Small (~30 lines + table) |
| 2. Tri-state verification | **Done** (WH-2) | Zero |
| 3. Operational timeline | **Mostly done** (ScyllaDB events). Resume events needed. | Small (~50 lines) |
| 4. Doctor ↔ workflow | **Mostly done**. Blocked-run invariant needed. | Small (~30 lines) |
| 5. Source-of-truth | **Done** (schema reference + dual-source design) | Documentation only |
| 6. Safety rails | **Mostly done**. Blocked-run surfacing needed. | Medium (~70 lines) |
| 7. Failure injection | **Strategy exists**. Drills need execution. | Operational |

---

## Proposed Integration Path

| Phase | What | Depends on |
|-------|------|------------|
| MC-1 | Step receipts (WH-8) | Nothing — additive |
| MC-2 | Resume event emission | MC-1 (receipts provide richer events) |
| MC-3 | Blocked-run status + approval flow | MC-2 (events needed for observability) |
| MC-4 | Doctor blocked-run invariant | MC-3 (needs BLOCKED status) |
| MC-5 | Convergence drills | MC-1–4 stable |
| MC-6 | Chaos workflow (optional) | MC-5 |

Each phase is a clean commit point. Total new code: ~200 lines.

---

## Conflicts With Current Design

**None.** Every capability listed is additive:
- Receipts add a table and a write path
- Resume events add event emission calls
- Blocked-run status uses an existing proto enum
- Doctor invariant follows existing rule pattern
- Drills use existing test infrastructure

The current architecture was designed to support these extensions. Nothing
needs to be redesigned or re-opened.

---

## Key Principle Validated

> Every workflow step can be resumed safely, explained clearly, and verified
> against reality.

**Resume safely** — done (WH-1 through WH-7).
**Explained clearly** — mostly done (ScyllaDB records), needs resume event annotations.
**Verified against reality** — done (tri-state verification + dual-source handlers).

The remaining work is polish and operational proving, not architectural change.
