# Project Q — honor Spec.Paused on InfrastructureRelease

## Status

**COMPLETE.** Code committed at `f10cb471`, `cluster-controller@1.2.128`
deployed and running, Project R/S state intact.

## Root cause

Pre-fix, the InfrastructureRelease reconciler did not honor any
operator-pause signal — the `Paused` field did not even exist on
`InfrastructureReleaseSpec`. The symmetric ServiceRelease reconciler had
the gate at `release_reconciler.go:226`:

```go
if rel.Spec.Paused {
    return
}
```

Without the INFRA-side equivalent, operators had no Globular-native
way to pause an INFRASTRUCTURE component:

- `Spec.Removing=true` was destructive (triggers the
  `release.remove.package` workflow → uninstall).
- The only non-destructive option was an ad-hoc systemd override (the
  `/bin/sleep infinity` ExecStart drop-in used during Project R's
  scylla-manager disable). That was always an emergency stabilization,
  not an operational primitive.

## Files changed

| File | Change |
|---|---|
| `golang/cluster_controller/cluster_controllerpb/resources_types.go` | Add `Paused bool` field to `InfrastructureReleaseSpec`, json-tagged `paused,omitempty`. Doc comment explains the semantics. |
| `golang/cluster_controller/cluster_controller_server/release_pipeline.go` | Populate `Paused: rel.Spec.Paused` in `infraReleaseHandle` so the handle carries it (mirrors the SERVICE handle builder). Add package-level helper `infraReconcilePauseGate(rel)` returning `(skip, reason)` — pulled out so the regression test can assert the gate without instantiating `*server`. |
| `golang/cluster_controller/cluster_controller_server/release_reconciler.go` | Call `infraReconcilePauseGate` at the top of `reconcileInfraRelease`, after the nil-checks but **before** the Removing dispatch. Paused takes precedence over Removing. One log line per reconcile cycle when skipped (matches the SERVICE side's log noise level). |
| `golang/cluster_controller/cluster_controller_server/release_reconciler_paused_test.go` | New, 9 tests. |

Commit: `f10cb471`. Deploy: `cluster-controller@1.2.128+1`, binary
sha256 `ad0de1632e39e2717a95dae24cc15f5f459e948fa9efecb16ca04b77a36656ab`.

## Exact paused semantics

When an operator sets `Spec.Paused=true` on an InfrastructureRelease:

| Behavior | Pre-fix | Post-fix |
|---|---|---|
| Reconciler iteration | runs normally — drift detection, workflow dispatch, phase mutation all proceed | `infraReconcilePauseGate` returns skip → reconciler returns immediately with one log line per cycle |
| Apply workflow dispatch | normal | **skipped** |
| Remove workflow dispatch | normal | **skipped** (paused takes precedence over removing) |
| Phase mutation | normal | **none** — release stays in whatever phase it was in (AVAILABLE / DEGRADED / etc.) |
| Status reporting | normal | **observable as-is** — convergence reports remain accurate, doctor invariants still see the unit's runtime state, no need for a new "paused" phase |
| Operator-visible reason | none | log line `infra-release <name>: skipping reconcile (Spec.Paused=true)` once per loop |
| Workflow callbacks (in-flight) | continue | continue — in-flight workflows are not aborted (Project Q does not interrupt mid-flight work; it only blocks new dispatch) |

To unpause, the operator flips `Spec.Paused` back to `false`. The next
reconciler iteration sees `skip=false` and resumes normal handling. No
manual reset of phase or other field is required. **Idempotency
proven by `TestInfraReconcilePauseGate_TogglePausedFalse_ResumesReconcile`.**

Paused is independent of Phase: a release can be Paused at any phase
(PENDING / RESOLVED / AVAILABLE / DEGRADED / FAILED / REMOVING / …)
and the gate fires consistently.

## Tests added

`release_reconciler_paused_test.go` — 9 tests, all PASS:

| Test | Assertion |
|---|---|
| `TestInfraReconcilePauseGate_Paused_BlocksReconcile` | `Paused=true` → `(skip=true, "Spec.Paused=true")` |
| `TestInfraReconcilePauseGate_Unpaused_AllowsReconcile` | `Paused=false` → `(skip=false, "")` |
| `TestInfraReconcilePauseGate_PausedTakesPrecedenceOverRemoving` | `Paused=true && Removing=true` → still skip (matches SERVICE-side semantics — auto-uninstall must not bypass operator pause) |
| `TestInfraReconcilePauseGate_Idempotent` | Re-checking the same struct 5× returns the same verdict (no hidden state) |
| `TestInfraReconcilePauseGate_TogglePausedFalse_ResumesReconcile` | Flipping the field at runtime resumes reconciliation |
| `TestInfraReconcilePauseGate_NilRelease_NoPanic` | Defensive: nil release → `(false, "")`, no panic |
| `TestInfraReconcilePauseGate_NilSpec_NoPanic` | Defensive: nil spec → `(false, "")`, no panic |
| `TestInfraReleaseHandle_PausedIsPopulated` | The handle builder still populates `h.Paused` from `rel.Spec.Paused` (regression guard if someone removes the wiring) |
| `TestServiceReleaseSpec_HasPausedField` | Regression guard on the SERVICE side: a zero-value `ServiceReleaseSpec.Paused` is `false`. Catches accidental removal of the field. |

## Test results

```
go test ./cluster_controller/cluster_controller_server/ -run "TestInfraReconcilePauseGate|TestInfraReleaseHandle_Paused|TestServiceReleaseSpec_HasPaused" -v -count=1
  9 tests PASS (0.100s)

go test ./cluster_controller/cluster_controller_server/ -count=1
  Full controller suite PASS (10.146s — 700+ tests, no regressions)
```

Project P removal-phase tests (`TestTransitionToRemoving_*`,
`TestPhaseTransitionGuards_PreFixPathStillBlocked`,
`TestRemoveWorkflowYAML_UsesMarkRemovingNotMarkApplying`) still green.

## Before / after behavior

### Pre-Project-Q

```
operator sets InfrastructureRelease.Spec.Paused = true
controller reconcile cycle:
  - reads InfrastructureRelease
  - builds handle (Paused field absent on spec)
  - runs full reconcile (drift detection, workflow dispatch, phase mutation)
  - operator pause has zero effect
operator's only options:
  - Spec.Removing=true → uninstall (destructive)
  - manual systemd override → unofficial, fragile (the Project R sleep-infinity workaround)
```

### Post-Project-Q

```
operator sets InfrastructureRelease.Spec.Paused = true
controller reconcile cycle:
  - reads InfrastructureRelease
  - infraReconcilePauseGate → (skip=true, "Spec.Paused=true")
  - log line: "infra-release <name>: skipping reconcile (Spec.Paused=true)"
  - return immediately
operator unpause:
  - Spec.Paused = false on next loop → resumes normally
package remains installed; running process continues unaffected
in-flight workflows continue (Project Q blocks NEW dispatch, not existing work)
```

## Would this have avoided the scylla-manager sleep override fallback?

**Yes.** During Project R, when the canonical `Spec.Removing=true`
mechanism failed (due to the `release.remove.package` workflow's then-
broken mark_applying step — the bug Project P later fixed), the
operator-visible options were:

1. Wait for Project P to ship → days
2. Use `Spec.Removing=true` again → would now succeed but is
   destructive
3. **Ad-hoc systemd `ExecStart=/bin/sleep infinity` override** ← chosen

With Project Q shipped, a fourth option would have existed:

4. `Spec.Paused=true` → controller stops reconciling, in-flight
   workflow drains, the unit stays in whatever state it was. Operator
   investigates and ships the fix. When ready, `Spec.Paused=false`
   resumes normal operation.

The sleep-infinity override (preserved at
`loads/scylla_manager_disable_override.conf`) can stay as a documented
emergency-only artifact; Project Q is the operational primitive going
forward.

## Project U status

**Project U remains queued (planning only).** Project U is the
HTTPS-hardening project for scylla-manager's manager API. It is
independent of Project Q. The recommendation from Project U's own
planning report — "run Project Q before Project U" — is now satisfied.

When the operator authorizes Project U, the four-step migration
sequence in `loads/project_u_scylla_manager_https_hardening_plan.md`
can run as designed. No revision to the Project U plan is required
based on Project Q's outcome.

## Live-cluster sanity check after deploy

| Check | State |
|---|---|
| `globular-cluster-controller.service` | active running, PID 735118, NRestarts=0, real v1.2.128 binary |
| `globular-scylla-manager.service` | active running (Project R/S state) |
| `/api/v1/clusters` | 1 cluster: `globular-internal` |
| Doctor scylla-manager findings | 0 |
| Project R backup tasks | both DONE, unchanged |

The +1 in total doctor finding count (24 → 25) is `cluster.services.drift`
on the just-installed cluster-controller — transient during convergence,
unrelated to Project Q semantics.

## What this does NOT change

- Phase transition table guards (Project P). `FAILED → APPLYING` and
  `FAILED → REMOVED` remain blocked.
- The ServiceRelease pause gate (unchanged).
- Removal workflow semantics (Project P). When Paused is false and
  Removing is true the existing removal path fires.
- Drift detection or workflow internals.
- No special-case for scylla-manager or any other specific component.

## Out-of-scope follow-ups

- **Project U** — scylla-manager HTTPS hardening (queued, planning
  report at `loads/project_u_scylla_manager_https_hardening_plan.md`).
- **Optional**: a `globular infrastructure pause <name>` CLI verb that
  sets `Spec.Paused=true` via the controller RPC instead of requiring
  direct etcd writes. Not part of Project Q's minimal-correct fix.
