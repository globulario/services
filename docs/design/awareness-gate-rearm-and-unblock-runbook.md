# Runbook — Re-arm the awareness gate & unblock PR #37

> Pre-staged 2026-06-21. **Nothing here is executed yet.** This is the ordered
> sequence to run **once awareness-graph ownership-model B lands on `master`**.
> Companion to `awareness-seed-ownership-decision.md` (Track B diagnosis) and
> `awareness-ownership-B-implementation-plan.md` (the maintainer's fix).
>
> Current state: PR #37 is **mechanically green, architecturally conditional** —
> the `awg audit` embeddata-freshness step is **advisory** (commit `cf9ef0e9`).
> Merge/deploy/PR-20/behavioral-memory are **parked** by explicit decision
> ("fix the gate first").

## Gate (do not start until this is true)

- [ ] awareness-graph **ownership-model B** is merged to **`master`** (not just on
      `feat/contract-centered-repair-spine`). Confirm services-generated families
      (`component.*`, generated `sourceFile→failureMode implements`, generated
      `test` triples, `contract.workflow.foreach_guard_order violatedBy`) are
      classified **external/tolerated**, with the negative-control test
      (genuine AWG-owned drift still FAILs) present and passing.

Verify locally before touching services CI (both must hold against **master**):

```bash
# from an awareness-graph master checkout, services worktree on each ref:
GOFLAGS=-buildvcs=false go run ./cmd/awg audit -check \
  -services-repo <services_#37_worktree> -ag-repo .     # expect: embeddata-freshness PASS
GOFLAGS=-buildvcs=false go run ./cmd/awg audit -check \
  -services-repo <services_master_worktree> -ag-repo .  # expect: PASS (control, no regression)
```

Both must PASS **without** embedding #37-era services content in the seed. If
`-services-repo master` still FAILs with owned drift on the families above, B is
**not** complete — do not proceed; ping the maintainer.

## Step 1 — Re-arm the hard gate (services, commit to #37)

In `.github/workflows/ci.yml`, revert the advisory step to a hard gate: remove
the `TEMPORARY` comment block and `continue-on-error: true`, restore the name.

```diff
-      # TEMPORARY (advisory, not a hard gate): ... (whole comment block)
-      - name: Awareness audit (awg audit -check, advisory until ag determinism+B land)
+      - name: Enforce awareness audit (awg audit -check, hard gate)
         if: steps.checkout_awareness_graph.outcome == 'success'
-        continue-on-error: true
         working-directory: ${{ github.workspace }}/awareness-graph
         run: go run ./cmd/awg audit -check -services-repo ${{ github.workspace }}/services -ag-repo .
```

Push, watch `build-test`. It must be **green with the hard gate live** — this is
the first **real** (not advisory) green. If it fails, B isn't actually on master
or doesn't cover #37's content; stop and re-diagnose.

## Step 2 — Merge #37 (explicit decision)

Only now is #37 architecturally final. Merging is still a deliberate act, not a
reflex on the checkmark — but the conditionality is gone. Squash/merge per repo
convention.

## Step 3 — Local Day-0 regression on ryzen (before trusting the release)

Hard rule: test Day-0 locally before any release. One eye on the **digest-
comparison path** (`golang/digest` consolidation is the only install-path change
this branch made; verify_integrity / heartbeat / blob_integrity).

```bash
scripts/build-local-release.sh --version <next>   # full local bundle, mirrors CI
sudo bash install-day0.sh                          # on ryzen; expect EXIT=0 + all milestones
```

## Step 4 — Publish + governed platform-upgrade

CI publishes platform artifacts on merge to master. Then the governed upgrade
(your call — irreversible-ish cluster mutation):

```
repo sync --tag v<X.Y.Z>
platform-upgrade v<X.Y.Z>
```

This rolls the **deployed** cluster-doctor (and the release-boundary diagnostic).

## Step 5 — PR-20 live validation (deployed doctor)

The release-boundary doctor must be observed in a **deployed** binary. Confirm:

```bash
globular release verify-boundary event          # expect: PROVEN (event@globule-ryzen)
```

And the deployed doctor report must show `release.boundary_unproven` **silent**
for `event@globule-ryzen` (no false-unproven). Use
`mcp__globular__cluster_get_doctor_report` / `cluster_explain_finding` to confirm
the rule fires correctly on real runtime evidence (not unit-test shadows).

## Step 6 — behavioral-memory DIAGNOSTIC_CLAIM emission

Only after Step 5 passes on the **deployed** doctor: enable the doctor →
behavioral-memory `DIAGNOSTIC_CLAIM` emission increment. This is the gated step —
no ghost memories from unit-test shadows.

## Order is strict

`B-on-master → re-arm gate → real green → merge → local Day-0 → publish →
platform-upgrade → PR-20 deployed validation → behavioral-memory`. Do not skip
the control check (Gate) or the local Day-0 (Step 3).
