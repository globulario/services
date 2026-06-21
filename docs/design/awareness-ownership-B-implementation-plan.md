# Path B Implementation Plan — Ownership-Aware Seed Freshness (awareness-graph)

> Status: **draft for review (B not yet chosen).** Targets the **awareness-graph**
> repo, not services. Companion to `awareness-seed-ownership-decision.md`.
> Prerequisite: awareness-graph PR #106 (authoredIn determinism) — independent,
> already up.

## Goal

Make `awg audit` embeddata-freshness classify **services-generated** triples as
services-owned / external-context (tolerated), so a services PR that adds
generated awareness content no longer forces a synchronized awareness-graph
seed regeneration — **without** weakening detection of genuine awareness-graph-
owned drift.

## Exact root cause (grounded in `cmd/awg/seedfreshness.go`)

`classifySeedDiff(committed, generated, agOnly)` buckets each differing triple:

```go
if agOwnershipKeys[ntOwnershipKey(l)] { owned = ... } else { external = ... }
```

- `agOnly` = seed regenerated from `agRepo/docs/awareness` **alone** (the owned corpus).
- `ntOwnershipKey(l)` = `subject + predicate + objectFamily`, where `objectFamily`
  collapses `<…#failureMode/X>` → `failureMode` (so a *value* change on an owned
  edge still counts as drift).

The bug is a **key collision across repos**:

1. The awareness-graph corpus **hand-authors facts that reference services source
   files** — e.g. `…/repair_plans/workflow.foreach_guard_before_collection.yaml`
   references `golang/workflow/engine/engine.go`;
   `…/generic/state_authority_invariants.yaml` references
   `release_runtime_convergence.go` and `netutil/node_identity.go`. So `agOnly`
   contains `sourceFile/engine.go <implements> failureMode/<ag-ref>`, and its
   ownership key `sourceFile/engine.go + implements + failureMode` is "owned."
2. Services #37 adds a **new generated edge** on the same file:
   `sourceFile/engine.go <implements> failureMode/workflow.reconcile_nil_collection…`.
   Its ownership key is **identical** (objectFamily collapses both to `failureMode`).
3. → the services-generated edge is classified **ag-owned drift** and fails the gate,
   even though awareness-graph never authored *that* edge.

Confirmed: the committed seed already embeds 724 services workflow `sourceFile`
triples + 20 `component.scripts` triples; #37 changes/adds edges on those same
ag-referenced subjects, so the collision is real and structural, not incidental.

## The fix: ownership must follow authorship, not key-collision

A diff triple is awareness-graph-owned **iff awareness-graph authored that triple**
— not merely if it shares a fuzzy key with an ag-authored edge on the same subject.

Services-generated triples are distinguishable after PR #106: their provenance is
`authoredIn "docs/awareness/generated/<f>"` (services-staged), whereas ag-authored
facts are `authoredIn` under hand-authored ag paths (`docs/awareness/generic/…`,
`…/architecture/…`, `…/repair_plans/…`). The fix uses that provenance.

### Mechanism (in `classifySeedDiff` / `seedfreshness.go`)

Build a **services-generated provenance set** and subtract it from `owned`:

1. From the freshly `generated` NT, collect the set of **subjects** whose
   `authoredIn` object is under `docs/awareness/generated/` **and** whose generated
   file is services-staged (the `filteredServicesGeneratedDir` output / `platform_*`,
   `echo_*`, `component.*` families — NOT the ag-own `awareness_graph_*`).
   Call this `svcGeneratedSubjects`.
2. In the owned/external split, a diff triple stays `owned` **only if** its key is
   in `agOwnershipKeys` **and** its subject is **not** in `svcGeneratedSubjects`.
   Otherwise it is `external` (tolerated).

This preserves the existing fuzzy-key behavior for genuinely ag-authored edges
(value drift on an ag-owned fact still fails) while excluding edges whose subject
is owned by a services-generated file.

> Note: distinguishing services-generated (`platform_*`, `echo_*`, `component.*`)
> from ag-own generated (`awareness_graph_*`) is essential — both live under a
> `generated/` dir, but only the services-staged ones should be external. The
> staging boundary already exists in `servicesGeneratedDir` /
> `filteredServicesGeneratedDir`; reuse it as the source of truth for the set.

### Alternative considered (rejected as primary)

- *Drop the objectFamily collapse for generated `implements`/`anchoredIn` edges* —
  would also break the collision, but it weakens ag-owned value-drift detection on
  those predicates. The provenance approach keeps detection intact. Keep this only
  as a fallback if provenance threading proves impractical.

## The load-bearing guardrail (the test that makes B safe)

B is only acceptable if **a genuine awareness-graph-owned drift still fails.** Add
tests in `cmd/awg/seedfreshness_test.go`:

1. **Services-generated edge on an ag-referenced subject → EXTERNAL (tolerated).**
   Seed a diff where `sourceFile/engine.go implements failureMode/<new>` is
   services-generated (authoredIn `docs/awareness/generated/…`) while agOnly owns
   `sourceFile/engine.go implements failureMode/<ag-ref>`. Assert it lands in
   `external`, not `owned`.
2. **Genuine ag-owned drift STILL fails (negative control).** Mutate an ag-authored
   triple (authoredIn `docs/awareness/generic/…` or `…/architecture/…`) — e.g.
   change a `severity` literal or an `affects` edge — and assert it lands in `owned`.
   This is the line that prevents B from becoming "tolerate everything."
3. **ag-own generated drift STILL fails.** A change to `awareness_graph_*`
   (ag's own generated code symbols) must remain `owned` — proving the
   services-vs-ag generated distinction holds.
4. **Determinism unchanged.** Repeated classification on the same inputs is stable
   (locks in PR #106's gain).

CI hard-gate already exists; these unit tests are the real safety net.

## Files to change (awareness-graph repo)

```
cmd/awg/seedfreshness.go        # svcGeneratedSubjects set + owned/external refinement
cmd/awg/seedfreshness_test.go   # the 4 guardrail tests above
docs/awareness/failure_modes.yaml  # (optional) record the misclassification failure mode,
                                   #   with required_test link — ownership-aware seed append, not full regen
```

No services-repo changes. No seed regeneration required for the code fix itself
(verify: after the fix, `awg audit` vs services #37 should PASS without embedding
#37's content — that's the whole point of B).

## Validation matrix

Run `go run ./cmd/awg audit -check -verbose -services-repo <svc> -ag-repo .`:

| ag ref | services ref | expected |
|---|---|---|
| fix branch | services **master** | PASS (no regression) |
| fix branch | services **#37** | **PASS** (services-generated now external/tolerated) — the unlock |
| fix branch | #37, **3 repeated runs** | identical (deterministic) |
| fix branch + a deliberately mutated **ag-authored** triple | any | **FAIL** (guardrail holds) |

Plus `go test ./cmd/awg` green.

## Governance / guardrails

- Do **not** weaken or disable embeddata-freshness; B *narrows ownership*, it does
  not blanket-tolerate. The negative-control test is mandatory and must be reviewed.
- Run AWG briefing on `cmd/awg/seedfreshness.go` (+ `failure_modes.yaml` if touched)
  before editing; obey the no-full-regenerate seed warning (ownership-aware append
  only if the failure-mode record is added).
- awareness-graph **maintainer review required** — this changes the audit's
  ownership model, the corpus's core contract.

## Sequencing

1. PR #106 (determinism) merges first — it's the prerequisite that makes
   `authoredIn = docs/awareness/generated/<f>` a reliable provenance signal.
2. This B PR lands on awareness-graph master (own PR, maintainer-reviewed).
3. Services #37 CI re-checks: `build-test` → `awg audit` should now go green
   **without** a coordinated seed blob — then the rest of the services list
   (lint → merge → publish → platform-upgrade → PR-20 → behavioral-memory) resumes
   in order.

## What B explicitly does NOT do

- Does not embed services content into the awareness-graph seed.
- Does not pin services CI.
- Does not change services awareness YAML.
- Does not relax detection of awareness-graph-owned drift (guarded by test #2/#3).
