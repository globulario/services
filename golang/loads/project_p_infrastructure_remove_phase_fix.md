# Project P — INFRASTRUCTURE remove phase transition fix

Generated: 2026-05-29

## Root cause

`reconcileRemoving` patched the release directly to `REMOVED` (or to
`FAILED` via the "fail" SetFields path) without first transitioning the
release into `REMOVING`. When a release in `FAILED` phase had
`Spec.Removing=true` set (the canonical disable signal), the workflow
ran but the controller's post-workflow patch was rejected by
`advancePhase`:

```
release core@globular.io/scylla-manager: BLOCKED:
  invalid phase transition "FAILED" → "REMOVED"
```

The phase-transition table (release_phase.go) requires
`FAILED → REMOVING → REMOVED`. There is no direct `FAILED → REMOVED`
edge — by design — so terminal `REMOVED` only occurs after the
release first passes through `REMOVING`. The pre-fix path tried to
short-circuit that.

A secondary issue lived in the workflow YAML: its first step
`id: mark_removing` called `controller.release.mark_applying` (a
documented no-op for the apply workflow) and verified
`expected_status: APPLYING`. Semantically wrong for a removal flow,
but inert in practice because the controller owns all release phase
transitions; the YAML mark-step is a marker for verification only.

## Files changed

| File | Change |
|---|---|
| `golang/cluster_controller/cluster_controller_server/release_pipeline.go` | Add `transitionToRemoving` helper; call it at the top of `reconcileRemoving` |
| `golang/cluster_controller/cluster_controller_server/release_pipeline_remove_phase_test.go` | New regression tests (6) |
| `golang/cluster_controller/cluster_controller_server/workflow_release.go` | Populate `MarkReleaseRemoving` no-op in both `ReleaseControllerConfig` builders |
| `golang/workflow/definitions/release.remove.package.yaml` | First step now calls `controller.release.mark_removing`; verify expects `REMOVING` |
| `golang/workflow/engine/actors.go` | Add `MarkReleaseRemoving` field to `ReleaseControllerConfig`; register `controller.release.mark_removing` action |

Commit: `fa44aa57`.

## Exact transition before / after

### Pre-fix sequence (FAILED + Spec.Removing=true)

```
reconcileRemoving entry:
  h.Phase = FAILED
  Dispatch release.remove.package workflow
  Workflow first step: controller.release.mark_applying  ← no-op
  Workflow verify_status expects APPLYING                ← mismatch (no-op)
  Workflow per-node uninstall steps complete
  Workflow finalize_direct_apply runs
  Controller post-workflow patch: Phase=REMOVED          ← BLOCKED
  advancePhase(FAILED, REMOVED) → error
  Patch silently rejected; release stuck at FAILED
```

### Post-fix sequence (FAILED + Spec.Removing=true)

```
reconcileRemoving entry:
  h.Phase = FAILED
  transitionToRemoving(ctx, h)
    → Patch Phase=REMOVING, SetFields="phase", reason="removing_dispatch"
    → advancePhase(FAILED, REMOVING) → OK
  h.Phase = REMOVING
  Dispatch release.remove.package workflow
  Workflow first step: controller.release.mark_removing  ← new no-op
  Workflow verify_status expects REMOVING                ← matches
  Workflow per-node uninstall steps complete
  Workflow finalize_direct_apply runs
  Controller post-workflow patch: Phase=REMOVED
  advancePhase(REMOVING, REMOVED) → OK
  Release reaches REMOVED cleanly
```

Failure path (workflow returns error): `REMOVING → FAILED` (allowed).

## Tests added

`golang/cluster_controller/cluster_controller_server/release_pipeline_remove_phase_test.go`:

| Test | What it asserts |
|---|---|
| `TestTransitionToRemoving_FromFailed_EmitsRemovingPatch` | The literal bug repro — FAILED handle entering reconcileRemoving emits exactly one patch with Phase=REMOVING, SetFields="phase", TransitionReason="removing_dispatch". Explicitly checks the patch is not Phase=REMOVED, not Phase=APPLYING. |
| `TestTransitionToRemoving_AlreadyRemoving_NoOp` | Idempotency: handle already in REMOVING emits zero patches. |
| `TestTransitionToRemoving_FromAvailable_EmitsRemovingPatch` | Normal entry from AVAILABLE still produces the REMOVING patch. |
| `TestTransitionToRemoving_NilHandle_NoPanic` | Defensive: nil handle returns without panic. |
| `TestPhaseTransitionGuards_PreFixPathStillBlocked` | Pins the invariant: `FAILED → APPLYING` and `FAILED → REMOVED` stay blocked. `FAILED → REMOVING`, `REMOVING → REMOVED`, `REMOVING → FAILED` are all allowed. |
| `TestRemoveWorkflowYAML_UsesMarkRemovingNotMarkApplying` | YAML lint: the on-disk `release.remove.package.yaml` references `controller.release.mark_removing` and `expected_status: REMOVING`, NOT the pre-fix `controller.release.mark_applying` / `expected_status: APPLYING`. |

Plus existing automatic coverage:

- `workflow/engine/TestEmbeddedWorkflowsHaveRegisteredActions` —
  scans embedded workflow YAML for action references and verifies a
  handler is registered. The new `controller.release.mark_removing`
  reference is automatically validated.

## Test results

```
cluster_controller_server  PASS  (13 tests run from the focused set; 0.10s)
workflow/engine            PASS  (full package; 55.68s)
cluster_controller_server  PASS  (full package; 9.83s)
```

## Would this have avoided the scylla-manager disable fallback?

**Yes — directly.** The Project O follow-up disable attempted exactly
this path: set `Spec.Removing=true` on scylla-manager's
InfrastructureRelease while it was in `FAILED` phase, expecting the
controller to drive it to `REMOVED`. The transition blocked, the
removal workflow's per-node uninstall steps never ran, and we fell
back to a systemd drop-in override (`/etc/systemd/system/globular-scylla-manager.service.d/disable.conf`).

With this fix in place:

```
Spec.Removing=true
  → reconcileRemoving entry
  → transitionToRemoving emits FAILED → REMOVING patch
  → workflow runs end-to-end
  → REMOVING → REMOVED on success (or → FAILED on uninstall failure)
```

The package would be uninstalled cleanly. The systemd override would
not have been necessary.

The override is still in place from yesterday's incident; **per
operator directive, it is not being removed in this task**. Once the
fix ships to a running controller, the override can be retired and
the canonical Globular path used for any future INFRASTRUCTURE
disable.

## Out of scope

- Project Q (make `reconcileInfraRelease` honor `Spec.Paused`) is
  intentionally NOT implemented here.
- The systemd drop-in for scylla-manager remains in place.
- Package rebuilds for the 23 still-bare-WD units were NOT queued.
- scylla-manager root cause investigation is unchanged (see
  `loads/scylla_manager_null_healthcheck_tasks_root_cause.md`).
