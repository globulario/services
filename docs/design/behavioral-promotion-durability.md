# Behavioral-promotion durability & reconciliation gate (PROPOSED)

Status: **PROPOSED — awaiting operator approval before implementation.**
Origin: ai-memory incident `aab023cb-7bc8-11f1-bcbf-001fc69cd334` (2026-07-09).
Governing invariant: `behavioral.promotion_must_survive_rebuild_or_be_flagged`.
Failure mode: `behavioral.runtime_promotion_evaporates_on_store_reset`.

## Problem

Behavioral-memory **promotion** is authority state, but it is stored as **runtime-written
rows** (promoted principles + `promotion_decisions`) that are **not part of the immutable
behavioral seed**. The seed (`golang/ai_memory/behavioral`) ships:

- authorities (e.g. `authority.cluster.release_pipeline.deployed_identity`), and
- the conditions catalog (e.g. `condition.cluster.service.binary_update_intended`),

but it does **not** ship runtime promotions. Consequently a behavioral-store (ScyllaDB)
rebuild/reset restores the seeded catalog while **dropping every promotion**. Nothing
re-promotes them, and nothing reconciles the loss against the corpus/docs that still claim
the rule is enforced.

Observed instance (2026-07-09): principle `8a1cdef8-…854e` ("service binaries change only
through the deploy pipeline") was documented PROMOTED, but live:

| Probe | Result |
| --- | --- |
| `behavioral_explain_principle(8a1cdef8)` | not found |
| `behavioral_check_action(cp_binary_into_usr_lib_globular_bin)` | `allowed=true, governed=false` (audit `6ba12377`) |
| `behavioral_list_promotion_candidates` | 0 |
| `behavioral_generate_reconciliation_report` | `AWG_RUNTIME_RELEVANT_WITHOUT_BEHAVIORAL_CANDIDATE` (report `7b0d34da`) |

Defense-in-depth silently degraded from **two** enforcement surfaces (runtime PreToolUse
hook + behavioral kernel) to **one** — with no signal. This is the **success-by-assertion**
defect class: docs asserted a live surface the live surface did not provide.

> Note: the exact promoted-principle UUID is **unrecoverable** after a reset — re-promotion
> mints a new id (`8a1cdef8` → `34db74ee-…`). Identity loss is part of the cost.

## Requirement

Satisfy **at least one** of the following (the invariant is an OR):

### Option A — Durable promotion state (persist)
Promotion decisions survive a store rebuild/reset. Either:
- promotions are written to a durable, rebuild-surviving store (not wiped by the seed
  reload), or
- the seed-reload path **replays** a durable promotion ledger after restoring the seeded
  catalog, so `explain_principle` / `check_action` return the same verdicts before and
  after a rebuild.

### Option B — Reconciliation gate (detect + flag)
A **blocking** boot/CI check runs `behavioral_generate_reconciliation_report` across every
runtime-relevant AWG failure mode and every corpus/doc claim of a PROMOTED principle, and
**fails loudly** on `AWG_RUNTIME_RELEVANT_WITHOUT_BEHAVIORAL_CANDIDATE` (or on any
doc-claimed PROMOTED principle whose `explain_principle` is `not found`). Silent evaporation
becomes a tracked finding instead of a governance hole.

Option B is cheaper and the detection primitive **already exists** (the reconciliation RPC
returned the right finding for this case); Option A is the stronger guarantee. They compose.

## Proposed test (to be created on approval)

Path: `golang/ai_memory/behavioral/promotion_durability_test.go` (package-local; exact path
to be confirmed against the behavioral service layout).

```go
// TestPromotionSurvivesStoreRebuild_or_ReconciliationFlags asserts the durability invariant
// behavioral.promotion_must_survive_rebuild_or_be_flagged:
//
//   1. Promote a principle P bound to a seeded condition through the governed surface.
//   2. Assert CheckAction(P.forbidden_move) => blocked, governed=true.
//   3. Simulate a store rebuild/reset (reload seed into a fresh store).
//   4a. Option A path: assert CheckAction(P.forbidden_move) STILL => blocked, governed=true
//       (promotion survived), OR
//   4b. Option B path: assert GenerateReconciliationReport surfaces
//       AWG_RUNTIME_RELEVANT_WITHOUT_BEHAVIORAL_CANDIDATE for P's AWG failure mode
//       (evaporation is detected, not silent).
//
// A green run where step 4a shows governed=false AND step 4b surfaces no finding is the
// exact regression this guards against — success-by-assertion evaporation.
```

Plus a CI/boot gate (Option B) — e.g. an `awg`/behavioral-doctor subcommand run in CI that
enumerates runtime-relevant AWG failure modes with a doc-claimed PROMOTED principle and exits
non-zero on any missing live promotion.

## Non-goals / constraints

- The behavioral seed remains **generated** — author rule content in source YAML, never by
  hand-editing `embeddata/awareness.nt` (`awareness.seed_is_generated_author_in_yaml`).
- Re-promotion must always go through the governed surface (propose + record_evidence +
  RunContradictionCheck + PromotePrinciple gate) — never a back-channel write.
- This document is PROPOSED; no implementation lands without operator approval.
