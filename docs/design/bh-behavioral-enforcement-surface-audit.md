# BH — Behavioral-Enforcement Surface Audit (Tier E scoping spike)

The verify-first scope for Tier E, opened the same way as RT-1/OT-1/EX-1: establish
the real current state before proposing any code. The headline is a reframing big
enough to change the plan.

> **Tier E/behavioral is materially complete. "BH-2/BH-3" are phantom labels — no
> such items are defined anywhere. The genuine remaining behavioral-enforcement work
> is 6 `planned` meta-principles awaiting their gate, and it lives mostly in the
> awareness-graph repo.**

## 1. What "BH" actually is (verified)

BH = the **behavioral-memory / behavioral-enforcement** organ — the third governance
surface alongside AWG (code/change intent) and flat ai-memory (recall). It governs
*runtime/operator action* through enforced `meta.*` principles + `principle-check`
scanners + behavioral ratchet tests that CI hard-gates.

## 2. Current state — delivered and spot-checked

| Item | Status | Verified evidence |
|---|---|---|
| **BH-1** — deterministic behavioral hard-refusal of raw owner-state writes | ✅ shipped | commit #100; referenced throughout `rt1-direct-write-surface-audit.md` (RT-2/RT-3 are what make BH-1's refusal *bite*) |
| **PR-9** — Governed Observation Ingestion | ✅ delivered | `ai_memory/domains/cluster_operator/observation/ingest.go` (368 lines, exists) |
| **PR-10** — Outcome→Promotion Candidate Queue | ✅ delivered | `ai_memory/behavioral/core/candidates.go` (210 lines, exists) |
| **PR-11** — AWG ↔ Behavioral Reconciliation | ✅ delivered | `ai_memory/behavioral/core/reconciliation.go` (175 lines, exists) |
| **PR-12** — Agent Self-Operation Governance | ✅ delivered | `ai_executor/.../behavioral_self_operation.go` (244 lines, exists) |
| **PR-13** — Verdict provenance + governance coverage | ✅ delivered | `PromotionDecisionRecord`, `GetGovernanceCoverage` |

`behavioral-memory-runtime-awareness-backlog.md` records all five ranked blind spots
**RESOLVED** and its **exit condition met 2026-06-20**. Spot-checking the claimed
artifacts (not just trusting the self-report) confirms they exist with substance.

**There is no `BH-2` or `BH-3` anywhere** — not in docs, git history, the coverage
YAMLs, or the backlog. The label came from a roadmap/close-out shorthand for "the next
behavioral work," never a defined item. This is the same "claimed-remaining items
evaporate under verification" pattern that recurred all arc (RT-4 globularcli, the
HIDDEN_WORKFLOW lifts, OT-3 #3, EX day-0).

## 3. The genuine gap — the enforcement-tier coverage map

`docs/awareness/meta_principle_coverage.yaml` (a **generated mirror**; canonical is
`awareness-graph/docs/awareness-control/meta_principle_coverage.yaml`) classifies every
non-`code_scanner` `meta.*` principle by enforcement tier. Of 73 entries:

| Tier | Count | Meaning |
|---|---|---|
| `review_only` | 43 | **Legitimate terminal tier** — irreducibly human design philosophy; *no artifact can prove it*. Surfaces via `awareness.briefing` at design time. **NOT a gap.** |
| `behavioral` | 23 | Enforced by a runtime/integration/script gate (cited). |
| `declaration` | 7 | Enforced by a declaration-completeness gate. |
| `planned` | 6 | **A mechanizable tier is identified but the gate is not yet built.** Names `intended_tier`. **This is the tracked gap.** |

*Verify-first on my own first pass:* an initial "57 review_only vs 3 mechanized"
framing was itself overstated — `review_only` is correct-by-design, not unfinished.
The real, bounded gap is the **6 `planned`** principles, every one of which already
names `intended_tier: behavioral`:

1. `meta.ui.interactive_element_must_have_stable_identity`
2. `meta.change_must_be_split_into_reviewable_slices`
3. `meta.discovery_produces_candidates_not_facts`
4. `meta.runtime_change_requires_observability_path`
5. `meta.exception_must_have_reason_owner_and_expiry`
6. `meta.architectural_intent_must_change_before_structural_drift`

Each is "build a behavioral gate (runtime/integration/script) that proves the
principle, then flip the canonical coverage entry `planned → behavioral` citing the
gate." The coverage map's own ratchet (`TestMetaPrincipleCoverageMirrorCoherence`)
keeps the mirror honest.

## 4. Cross-repo reality

The coverage registry is **AWG-owned** — canonical in the **awareness-graph repo**, of
which this file is a generated mirror. So mechanizing the 6 `planned` principles is
mostly awareness-graph work (the gate artifact may be a behavioral test in either repo,
but the `planned → behavioral` reclassification edits the canonical AG copy). This is
the **same surface as the carry-forward OT-4 promotion** (promote
`meta.binding_outlives_evidence_until_invalidated` + `health.requires_fresh_evidence`
candidate→active, also AG-repo + embeddata rebuild).

## 5. Conclusion and recommendation

**Tier E is not a tier of remaining work — it is ~95% built.** The behavioral organ
(BH-1 + PR-9..13) is delivered and verified. What's left is a **bounded mechanization
sweep**, not a structural tier:

- **6 `planned` meta-principles** → build their `intended_tier: behavioral` gates,
  flip `planned → behavioral` (mostly awareness-graph repo).
- **OT-4 promotion** (2 principles candidate→active) — same AG-mechanization surface;
  fold into the same sweep.

Recommended framing: retire "Tier E/BH-2/BH-3" as a label. Track the real remaining
items as a single **AG-repo mechanization sweep** (6 planned-principle gates + OT-4
promotion + the embeddata-rebuild weak rung), scoped and executed in the
awareness-graph repo where the authority lives.

The governance arc's honest end-state: **the platform's authority skeleton (RT/OT/EX)
is closed and ratcheted; the behavioral organ is built; the only mechanically-unfinished
edge is 6 named, tracked principle gates — a curation sweep, not a new tier.**
