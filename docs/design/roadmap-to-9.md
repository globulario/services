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
| **A — GC** | awareness-graph | Make awareness changes cheap & safe | ✅ **Built + merged + healthy (GC-2 green, verified); GC-3 leg not re-verified** |
| **B — WB** | awareness-graph | Close the write-back loop | ✅ Merged (WB-1/2/3); both ends still have manual seams |
| **C — CG** | awareness-graph | Coverage grind | 🟡 Gates merged (CG-5/6); CG-2 promotion grind ongoing (now cheap) |
| **D — RT** | services | Universalize runtime governance | ✅ Complete |
| **E — BH** | services | Behavioral liveness | 🟡 Mostly built; bounded enforcement remaining |
| **F — OT** | services | Operator truth classified | ✅ Complete |
| **G — EX** | (both) | Make extension boring | ⚪ Untouched — correctly last |

---

## Tier A — Make awareness changes cheap & safe (GC) — `awareness-graph` repo

- **GC-1 — coherence pre-merge gate.** ✅ Merged (`aa3b8bb` "coherence gate
  (GC-1/2/3)"). Catches duplicate-id / dangling-ref / orphan. *(Runtime behavior
  of the orphan store-vs-YAML leg not independently re-verified this session.)*
- **GC-2 — automated seed rebuild on merge.** ✅ **Built + merged + healthy.**
  `seed-rebuild.yml` fires `on: push → master`, runs
  `scripts/build-awareness-graph.sh`, auto-commits refreshed `awareness.nt`.
  Bot commits exist (`fafdda9`, `00b3769`). `203c5f7` adds `awg audit
  --warn-stale`. The earlier stamp-staleness recurrence was **systemically
  fixed by #136** (`3699ff3`): stamp committed *with* the seed (gated on seed
  change) + in-workflow integrity gate (`go build` + embedded-seed tests) before
  push + no `[skip ci]`. **Verified this session: integrity tests pass on
  origin/master.** No residual.
- **GC-3 — live-store ↔ authored-YAML reconciliation.** ✅ Marked landed by
  awareness-graph history (part of `aa3b8bb` "GC-1/2/3"). ⚠️ **Runtime
  reconciliation behavior still deserves one direct verification pass before
  relying on it for release claims** — named here so the uncertainty doesn't
  block the CG-2 grind, but isn't forgotten.

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
- **CG-2 — promote evidence-backed invariants at scale.** 🟡 Ongoing grind
  (#95/#96 in services corpus + continuous). Now cheap because GC-2 exists —
  *once its stamp reliability is fixed.*
- **CG-1 / CG-3 / CG-4** — evidence audit, missing guard+test long tail, confirm
  impact-ci fires end-to-end. 🟡 Folded into the ongoing grind.

## Tier D — Universalize runtime governance (RT) — `services` repo — ✅ COMPLETE

- RT-1 surface audit (#101); RT-2 route/guard writes onto owner RPCs (#104–#115);
  RT-3 owner-guard chokepoint + funnel capstone + registry consolidation
  (#112, #117–#122); RT-4 raw-write scanner, Go + shell-aware (#102, #103, #116).

## Tier E — Behavioral liveness (BH) — `services` repo — 🟡 MOSTLY BUILT

- BH-1 deterministic hard-refusal of raw owner-state writes via govops (#100). ✅
- Remaining: bounded enforcement coverage named by the #138 verify-first audit.

## Tier F — Operator truth classified (OT) — `services` repo — ✅ COMPLETE

- OT-1 observe-truth audit (#124); OT-2 evidence collection-time + downgrade on
  errored evidence (#125, #130, #131); OT-3 atomic desired+runtime write,
  cache-freshness signal, stale-mirror rule, RBAC cache flush (#127–#129, #132);
  OT-4 evidence-time ratchet (#126).

## Tier G — Make extension boring — ⚪ UNTOUCHED, CORRECTLY LAST

Not the execution-surface `EX-*` commits (done). Harvests proven patterns into
templates — promote-invariant scaffold, owner-path dispatcher template,
new-service onboarding template, "Adding X is boring" runbooks. Do it last.

---

## The ordered roadmap (corrected)

```
NOW →  CG-2 promotion grind at scale (now genuinely DOWNHILL — the GC-2
       conveyor belt is healthy). + BH bounded enforcement from the #138 audit.
SOON:  Verify the GC-3 store↔YAML reconciliation leg actually runs (claimed in
       aa3b8bb's "GC-1/2/3" but not re-verified); wire WB-2 incident→candidate
       into a standing review queue (tooling exists, seam is manual).
LAST:  Tier G template harvest (promote-invariant scaffold, dispatcher template,
       onboarding template, runbooks) — patterns are now proven stable.
DONE:  Tier A/B/C built+merged+healthy on awareness-graph master (GC-1/2/3 incl.
       GC-2 verified, WB-1/2/3, CG-5/6); Tier D (RT-1..4), Tier F (OT-1..4),
       BH-1, execution-surface EX. Stale branch fix/regenerate-stale-transaction-
       stamp is superseded by #136 — do not land.
```

---

## Appendix: superseded Phase A–G plan (historical)

The original 7.5→9 plan (CLI `AllocateUpload`, deploy-validate P3–7, automated
invariant tests, service-health cleanup, semver, test-cluster, day-0 hardening)
is retained in git history. It is **not** the active roadmap.
</content>
