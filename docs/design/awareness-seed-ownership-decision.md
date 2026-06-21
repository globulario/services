# Decision Note — Cross-Repo Awareness Seed/Ownership for Large Services PRs

> Status: **decision-required.** Written 2026-06-21 after services PR #37
> (`infra-truth-plane-etcd-minio-envoy`) hit a structural CI wall on the
> awareness-graph `awg audit` embeddata-freshness gate. This note frames the
> three options (A/B/C) so the seed/ownership model is decided **before** anyone
> regenerates a seed or changes ownership rules. It does not implement anything.
>
> **UPDATE 2026-06-21 (empirical) — Path A is ruled out.** Path A was *executed*
> (ag master seed rebuilt against services #37, on top of the #106 determinism
> branch) and **failed on two counts**:
> 1. **It did not green #37** — `embeddata-freshness` still FAILED with a stable 4
>    owned drift. Those 4 are `component.scripts authoredIn` triples where the
>    audit's generation still mints a per-run `/tmp/tmp.XXXXX/` path — a **second
>    determinism bug** that PR #106 did not cover. No seed regen can converge
>    while that path is non-deterministic.
> 2. **It broke the shared control** — auditing the #37-built seed against
>    services *master* then FAILED with 21 owned drift. Embedding one branch's
>    services content into awareness-graph master's shared seed breaks every other
>    services PR's CI against that master until #37 merges.
>
> Conclusion: A is not a viable shared-gate strategy. The proven path is
> **[complete the determinism fix] → [Path B]**. See
> `awareness-ownership-B-implementation-plan.md`.

## The situation in one paragraph

A services PR that adds awareness YAML causes the awareness-graph repo's embedded
seed (`golang/server/embeddata/awareness.nt`) to drift, because the services-
generated triples it produces are classified **awareness-graph-owned** and are
therefore required to be physically embedded in awareness-graph `master`'s seed.
For a small PR this is invisible; for a 98-commit infra branch it is a ~28k-line
seed addition coupled to the unmerged branch. The services CI checks out
awareness-graph `master` (unpinned) and runs `awg audit -check -services-repo
<svc> -ag-repo .`, so the gate fails until `master`'s seed reflects the branch's
content.

## What is already settled (not part of this decision)

- **Determinism bug** — `filteredServicesGeneratedDir` baked a random `/tmp`
  path into `authoredIn`, so the audit drift count fluctuated run-to-run. Fixed
  by awareness-graph **PR #106** (code-only). This is a **prerequisite**; it does
  **not** green any services branch by itself. Proven by controlled test:
  determinism fix → drift stable (21/21/21) but still non-zero.
- **The release-boundary work** (services PR-16→19) is complete, tested, and
  live-proven (`event@globule-ryzen` PROVEN). It is **not** the blocker.

The only open question is the **seed/ownership publishing model** below.

## The fork

| | A — Coordinated seed regen | B — Ownership-model change | C — Branch split |
|---|---|---|---|
| **What it does** | Land the services awareness YAML and a matching awareness-graph seed regeneration **together**, reviewing the large generated-seed diff as the branch's real awareness footprint. | Change `awg audit` so services-generated triples (`docs/awareness/generated/*`, services-authored YAML) are **services-owned / external-context (tolerated)**, not awareness-graph-owned. | Split the infra branch so awareness YAML + seed updates land in **small controlled chunks**, each a reviewable seed diff. |
| **What becomes true** | awareness-graph `master`'s seed embeds every services-generated fact. Cross-repo lockstep is **explicit and mandatory** for every awareness-touching services PR. | awareness-graph `master` no longer embeds services-generated facts; **services validates its own generated awareness**, AWG only enforces AWG-owned freshness. The cross-repo gate stops failing on services content. | Same model as today, but each landing's seed blast radius is small enough to review. Lockstep remains, just chunked. |
| **Blast radius** | Large (one ~28k-line seed diff per big branch). | Medium one-time (audit classifier + tests), then **small forever**. | Small per chunk, but **many** chunks + sequencing cost. |
| **Time to ship #37** | Fastest if #37 must land as one megabranch. | Slower up front (real change to the audit model), fastest thereafter. | Slowest (re-cutting a 98-commit branch). |
| **Risk** | Recurs on every future large awareness PR; reviewers rubber-stamp giant seed diffs (→ the diff stops being meaningfully reviewed). | Must get the ownership boundary exactly right; a too-broad "external" bucket could let real AWG-owned drift slip through. Needs awareness-graph maintainer sign-off. | Low technical risk; high coordination/time risk; doesn't fix the underlying model. |
| **Reversibility** | Easy per-PR; doesn't change the model. | A real architectural commitment (but itself revertable code). | Easy; purely process. |

## What each path asserts about the architecture

- **A** asserts: *the awareness-graph seed is the single embedded source of truth
  for all facts, including services-generated ones; services PRs own keeping it
  fresh in lockstep.*
- **B** asserts: *services-generated awareness is owned and validated by the
  services repo; the awareness-graph seed is authoritative only for
  awareness-graph-authored facts; cross-repo facts are tolerated context.*
- **C** asserts nothing new about the model — it only makes A's blast radius
  tolerable by chunking.

## Recommendation

- **Strategic:** **B.** If services-generated awareness is meant to be owned by
  services (it is — those triples are inferred from services code/YAML), then the
  ownership model should say so, and the seed should not have to absorb services
  content on every PR. B removes the recurring cross-repo failure class instead of
  paying it each time. It needs the awareness-graph maintainers' judgment on the
  exact owned/external boundary, plus tests that prove real AWG-owned drift is
  still caught.
- **Pragmatic ship-it:** ~~**A**~~ **— withdrawn.** A was executed and is
  non-viable (see the UPDATE callout at the top): it does not converge (second
  determinism bug) and it breaks the shared services-master control. Do not
  attempt coordinated seed regeneration for #37 again.
- **C** only if reviewers need the branch carved for review reasons; it does not
  fix the underlying model and does not unblock the gate by itself.

Proven sequence: **complete the determinism fix** (kill the second
`/tmp/tmp.XXXXX/` `component.scripts` authoredIn source, extending #106), **then
ship B** as the durable fix so services-generated awareness is tolerated as
external and master's seed never has to embed branch-specific services content.

## Guardrails for whoever executes the decision

- Do **not** weaken or disable `awg audit` / embeddata-freshness as a shortcut.
- Do **not** pin services CI to an awareness-graph feature branch.
- For **A**: regenerate the seed the ownership-aware way; review the diff as
  generated output; do not hand-edit `awareness.nt`.
- For **B**: change the classifier + add tests that a genuine AWG-owned drift
  still fails; get awareness-graph maintainer review.
- For **C**: keep each chunk's seed diff small and self-consistent.

## Pointers

- awareness-graph PR #106 — determinism prerequisite (code-only).
- services PR #37 — blocked at `build-test` → `awg audit` embeddata-freshness.
- Reproduce: `go run ./cmd/awg audit -check -verbose -services-repo <svc> -ag-repo .`
  (control: services master → PASS; services #37 → FAIL, 21 owned drift).
