# Globular — Governance-Loop Roadmap (Tier A–G)

> **Map correction (2026-06-25, second pass — read this).** A first rewrite today
> declared GC-2 the "unstarted keystone." **That was wrong.** It was derived from
> the `services` repo git log alone. The Tier A/B/C governance loop
> (`GC / WB / CG`) does **not** live in `services` — it lives in the separate
> **`awareness-graph` repo**, where GC-1/2/3, WB-1/2/3, and CG-5/6 are all
> **merged on master**. This file now matches reality across **both** repos.
>
> **THE CROSS-REPO RULE (the trap that bit us):** the Tier A–G work is split
> across two repos. Never score a tier from one repo's log.
> - **`services`** (this repo) → **RT, OT, EX, BH** (Go services, controller, node-agent, doctor, govops).
> - **`awareness-graph`** (`~/Documents/github.com/globulario/awareness-graph`) → **GC, WB, CG** (the awg corpus, seed pipeline, coherence gates).
>
> The map had an old bridge drawn where the river had moved. It's fixed now.

---

## The core correction (revised, verified 2026-06-25)

The **runtime (RT)**, **operator-truth (OT)**, **execution-surface (EX)** tiers
in `services` AND the **awareness-loop tiers (GC/WB/CG)** in `awareness-graph`
are **materially built and merged.** The conveyor belt the first rewrite said
was missing — automated authored-YAML → seed → embeddata rebuild on merge —
**exists, runs, and is healthy** (`awareness-graph` `seed-rebuild.yml`, bot
commits `fafdda9`, `00b3769`).

**GC-2 is GREEN — there is no GC-2 build work remaining.** An earlier recurring
stamp-staleness wound (the auto-rebuild refreshed `awareness.nt` but left
`awareness.transaction.tsv` stale, and `[skip ci]` hid the resulting test
break — manual "restore master" commits #132, #135) was **systemically fixed by
#136** (`3699ff3`): the bot now commits the stamp *with* the seed (gated on the
seed changing so the non-reproducible binary SHA doesn't churn), runs an
**in-workflow integrity gate** (`go build ./...` + embedded-seed integrity
tests) before the push, and dropped `[skip ci]`. **Verified this session:** the
two integrity tests (`TestMetadata_ExposesEmbeddedSeedMarkerState`,
`TestGraphAuthorityCarriesEmbeddedTransactionCertification`) **pass on
origin/master** — stamp certifies the current seed.

> ⚠️ **Stale branch:** `fix/regenerate-stale-transaction-stamp` (`31b4c2c`) is a
> **pre-#136 manual restore** — its own commit body proposes, as "Follow-up (not
> here)", exactly what #136 then implemented. It is **superseded; do not land
> it.** (It carries unrelated uncommitted working-tree changes — leave those to
> the owner.)

**The remaining frontier is the coverage grind, not the loop.** With the
conveyor belt healthy, CG-2 promotions and BH enforcement coverage are now
genuinely *downhill*. That is where effort should go next.

### "Same letter, different animal" — the EX label trap (still true)

| Label | Correct framing | Status |
|-------|-----------------|--------|
| `EX-*` commits in `services` (#133–#136) | **Execution-surface** governance (supervisor routing, remediation audit ring) | **Done** |
| Roadmap **Tier G** | **Extension** / onboarding / scaffold / template harvest | **Untouched — and correctly last** |

---

## Status at a glance

| Tier | Repo | Theme | Status |
|------|------|-------|--------|
| **A — GC** | awareness-graph | Make awareness changes cheap & safe | ✅ **Complete + verified — GC-1/2/3 all confirmed at runtime this session** |
| **B — WB** | awareness-graph | Close the write-back loop | ✅ Merged (WB-1/2/3); both ends still have manual seams |
| **C — CG** | awareness-graph + services | Coverage grind | ✅ **CG-2 exhausted (services corpus fully harvested); CG-3 in progress (2 done, 3 deferred as spine changes)** |
| **D — RT** | services | Universalize runtime governance | ✅ Complete |
| **E — BH** | services + awareness-graph | Behavioral liveness | ✅ **Complete by honest disposition (3 behavioral, 3 intentionally planned)** |
| **F — OT** | services | Operator truth classified | ✅ Complete |
| **G — EX** | (both) | Make extension boring | ⚪ Untouched — correctly last |

---

## Tier A — Make awareness changes cheap & safe (GC) — `awareness-graph` repo

- **GC-1 — coherence pre-merge gate.** ✅ Merged (`aa3b8bb` "coherence gate
  (GC-1/2/3)"). Catches duplicate-id / dangling-ref / seed-orphan.
- **GC-2 — automated seed rebuild on merge.** ✅ **Built + merged + healthy.**
  `seed-rebuild.yml` fires `on: push → master`, runs
  `scripts/build-awareness-graph.sh`, auto-commits refreshed `awareness.nt`.
  Bot commits exist (`fafdda9`, `00b3769`). `203c5f7` adds `awg audit
  --warn-stale`. The earlier stamp-staleness recurrence was **systemically
  fixed by #136** (`3699ff3`): stamp committed *with* the seed (gated on seed
  change) + in-workflow integrity gate (`go build` + embedded-seed tests) before
  push + no `[skip ci]`. **Verified this session: integrity tests pass on
  origin/master.** No residual.
- **GC-3 — live-store ↔ authored-YAML reconciliation.** ✅ **Verified at runtime
  this session.** `awg reconcile` (cmd/awg/cmd_reconcile.go) diffs the live
  Oxigraph store against an authored baseline (`-baseline yaml` = true-orphan
  detector, `-baseline seed` = deployed-runtime detector); `-require-clean`
  gates. Unit tests pass; live runs against Oxigraph (localhost:7878) produced
  correct store-only-orphan + lagging reports with the documented exit codes.
  Strongest signal: it correctly flagged this session's own CG-2/CG-3 authored
  edits (renamed/removed symbols like `AuthorizeRelease`, `build-services.sh`) as
  store-only orphans, because the live store hasn't been reloaded from the
  post-merge seed — exactly the store↔YAML drift the job exists to surface.
  (ai-memory `3dc511ee`.)

## Tier B — Close the write-back loop (WB) — `awareness-graph` repo

- **WB-1 — promotion → rebuild → checks fires automatically.** ✅ `d561c92`
  ("promote fires the coherence gate after rebuild").
- **WB-2 — incident → candidate generator / review queue.** ✅ `2b6d983`
  ("draft-candidate — incident → review-queue candidate bridge"). MCP tool
  `behavioral_generate_promotion_candidate` is the runtime face.
- **WB-3 — end-to-end loop CI.** ✅ `05d86f8` ("end-to-end write-back loop
  demonstration"). Residual: both ends still have manual seams
  (incident→candidate is agent labor; promotion→rebuild relies on GC-2's
  reliability above).

## Tier C — Coverage grind (CG) — `awareness-graph` repo

- **CG-5 — impact-gate (changed-files → required_tests, fail-closed).** ✅
  `e801e4b`.
- **CG-6 — severity vocabulary enforced in `awg validate`.** ✅ `3a26913`
  (+ `services` #99 corpus alignment).
- **CG-2 — promote evidence-backed invariants at scale.** ✅ **Exhausted for the
  services corpus.** `package_identity_invariants.yaml` was the only awareness
  YAML with proposed invariants; **11 promoted across 3 batches** (#140 ×8,
  #141 ×2, #142 ×1), every one evidence + test + gate-backed. The verify-against-
  real-code discipline caught 8+ wrong/non-existent symbols the triage agent
  listed — none shipped.
- **CG-3 — earn new truth (guard/test first, promote second).** 🟡 In progress.
  Done: `release.version_single_authority` (#143, proof test),
  `publish.release_artifact_must_be_stripped` strip-half (#144, new ELF strip
  gate), `convergence.identity_is_build_id` (Path C landed in
  `repair_node_workflow` with focused proof tests), and
  `package.release_vs_dev_channel_boundary` (repo dev-lane coercion +
  controller desired-state gate + deploy DEV skip path, all proof-tested).
  Deferred as spine changes, each with a recorded finding:
  `publish...stripped` size-envelope half (`7b326026`),
  `staging.content_addressed` (largest, unscoped).
- **CG-1 / CG-4** — evidence audit, confirm impact-ci fires end-to-end. 🟡 Folded
  into the grind.

**Tally:** 15 invariants active in `package_identity_invariants.yaml`, 3 proposed
(the deferred spine changes above).

## Tier D — Universalize runtime governance (RT) — `services` repo — ✅ COMPLETE

- RT-1 surface audit (#101); RT-2 route/guard writes onto owner RPCs (#104–#115);
  RT-3 owner-guard chokepoint + funnel capstone + registry consolidation
  (#112, #117–#122); RT-4 raw-write scanner, Go + shell-aware (#102, #103, #116).

## Tier E — Behavioral liveness (BH) — `services` + `awareness-graph` — ✅ COMPLETE

**Complete by honest disposition** (2026-06-26). The behavioral organ (BH-1 +
PR-9..13) was already delivered; the #138 verify-first audit reframed the rest as
a bounded meta-principle mechanization sweep, not a new structural tier. Worked
the 6 `planned` meta-principles to a truthful end:

- BH-1 deterministic hard-refusal of raw owner-state writes via govops (#100). ✅
- **3 principles behavioral:**
  - `meta.discovery_produces_candidates_not_facts` — already behavioral on the
    canonical (AG #133); the stale services mirror had hidden this.
  - `meta.ui.interactive_element_must_have_stable_identity` — already behavioral
    (element-identity ratchet, frozen count).
  - `meta.exception_must_have_reason_owner_and_expiry` — **built this arc** (AG
    #140): canonical exception **ledger** (`docs/awareness-control/exceptions.yaml`,
    9 governed exceptions) + a non-polluting validator
    (`cmd/principle-check/exception_ledger_test.go`, 10 tests) that lints ONLY the
    ledger and fails on missing owner/expiry/removal_condition, expired entries,
    or malformed dates; `planned → behavioral`.
- **3 principles intentionally `planned`** — a hard gate for each would *pollute*
  (cry wolf), the opposite of robustness, so deliberately NOT built:
  - `meta.runtime_change_requires_observability_path` — general diff-scanner
    false-positives; needs semantic detection (its own text frames the
    un-observed case as a *candidate for review*, i.e. advisory).
  - `meta.change_must_be_split_into_reviewable_slices` — diff-size/path-mixing
    heuristic; inherently noisy.
  - `meta.architectural_intent_must_change_before_structural_drift` — mechanizable
    in theory but not safe as a cheap scanner; stays `planned` (NOT `review_only` —
    the mechanism is named, ai-memory `ad5f1a67`).
- **0 hidden remaining build candidates.** Re-derive Tier E status from the
  awareness-graph CANONICAL coverage registry, never the services mirror — a stale
  mirror bred phantom "planned" work that #138 partly inherited (ai-memory
  `0545ad95`).
- **Self-governance:** the coverage-mirror-lag that caused that phantom work is now
  itself a ledgered, dated exception (`exception.coverage_mirror_lag_tolerance`),
  and the mirror was resynced to clear it (#147/#148).

Companion OT-4 carry-forward: `meta.binding_outlives_evidence_until_invalidated`
`candidate → active` (AG #139, already enforced by a ruleguard).

## Tier F — Operator truth classified (OT) — `services` repo — ✅ COMPLETE

- OT-1 observe-truth audit (#124); OT-2 evidence collection-time + downgrade on
  errored evidence (#125, #130, #131); OT-3 atomic desired+runtime write,
  cache-freshness signal, stale-mirror rule, RBAC cache flush (#127–#129, #132);
  OT-4 evidence-time ratchet (#126).

## Tier G — Make extension boring — ⚪ UNTOUCHED, CORRECTLY LAST

Not the execution-surface `EX-*` commits (done). Harvests proven patterns into
templates — promote-invariant scaffold, owner-path dispatcher template,
new-service onboarding template, "Adding X is boring" runbooks. The `services`
half is now frozen in `docs/design/tier-g-extension-harvest.md`; the invariant-
promotion scaffold remains cross-repo in `awareness-graph`. Do it last.

---

## The ordered roadmap (corrected)

```
NOW →  The easy coverage is exhausted AND Tier E is closed. What remains is
       dedicated, optional engineering — one careful slice at a time, none urgent:
         • CG-3 spine changes (own design session each): the
           `publish...stripped` size-envelope half and `staging.content_addressed`.
           `convergence.identity_is_build_id` and
           `package.release_vs_dev_channel_boundary` are already landed in `services`.
         • Tier G template harvest (promote-invariant scaffold, dispatcher
           template, onboarding template, runbooks) — patterns now proven stable.
       Optional small seam: wire WB-2 incident→candidate into a standing review
       queue (tooling exists).
       Do NOT build the 3 intentionally-`planned` Tier E meta-principles — a hard
       gate for each would pollute (cry wolf). Leaving them planned is the correct
       finish, not a gap.
DONE:  Tier A complete + VERIFIED at runtime (GC-1/2/3 — GC-2 seed-rebuild via
       #136 integrity gate, GC-3 via awg reconcile). Tier B (WB-1/2/3). Tier C:
       CG-5/6 gates + CG-2 fully harvested (11 promoted) + 2 CG-3 earned
       (#143/#144). Tier D (RT-1..4). Tier E COMPLETE (3 behavioral incl. the
       exception-ledger gate AG #140; 3 intentionally planned; OT-4 binding_outlives
       AG #139). Tier F (OT-1..4). execution-surface EX. Stale branch
       fix/regenerate-stale-transaction-stamp superseded by #136 — do not land.
```

---

## Appendix: superseded Phase A–G plan (historical)

The original 7.5→9 plan (CLI `AllocateUpload`, deploy-validate P3–7, automated
invariant tests, service-health cleanup, semver, test-cluster, day-0 hardening)
is retained in git history. It is **not** the active roadmap.
</content>
