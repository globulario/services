# Behavioral Memory Runtime Awareness Backlog

> Status: **reconciled 2026-06-20 — backlog fully delivered.** Originally a post-PR-8
> design note that ranked the next governance increments without authorizing
> implementation. As of 2026-06-20 every proposed increment (PR-9 through PR-12)
> plus the later PR-13 has shipped; this document is now a delivery record, not a
> forward backlog.
> Scope: preserve the three-organ model, record which blind spots are now closed,
> and point at the evidence for each delivered increment.

## Implementation status (reconciled 2026-06-20)

Each backlog item below has landed. Evidence is in code and commits, not in this note.

| Item | Status | Primary evidence |
|------|--------|------------------|
| PR-9 — Governed Observation Ingestion | ✅ delivered | `golang/ai_memory/domains/cluster_operator/observation/{ingest.go,client.go}` (`FromDoctorFinding`, `FromInfraProbe`, `FromWatcherIncident`, `RecordBundle`); live emit paths in cluster_doctor `server.go:380`, ai_watcher `server.go:540`, node_agent `grpc_infra_probe.go` (×4), ai_executor `behavioral_feed.go`; `SignalKind` + `ObservationAuthorityLevel` enums in `proto/behavioral_memory.proto`; commits `d7bcbc52`, `e6a15108` |
| PR-10 — Outcome-to-Promotion Candidate Queue | ✅ delivered | `golang/ai_memory/behavioral/core/candidates.go` (`GeneratePromotionCandidate`, `ListPromotionCandidates`); `PromotionCandidate` message + `PromotionCandidateStatus` enum (QUEUED/REVIEWED/DISMISSED/MATERIALIZED); MCP `behavioral_generate_promotion_candidate`, `behavioral_list_promotion_candidates` |
| PR-11 — AWG ↔ Behavioral Reconciliation | ✅ delivered | `golang/ai_memory/behavioral/core/reconciliation.go` (`RUNTIME_CONTRADICTS_AWG`, `RUNTIME_REINFORCES_AWG`, `BEHAVIORAL_CANDIDATE_MISSING_AWG_MAPPING`, `AWG_RUNTIME_RELEVANT_WITHOUT_BEHAVIORAL_CANDIDATE`, `AWG_MAPPING_MISSING_TEST_CANDIDATE`); MCP `behavioral_generate_reconciliation_report`, `behavioral_list_reconciliation_reports` |
| PR-12 — Agent Self-Operation Governance | ✅ delivered | `golang/ai_executor/ai_executor_server/behavioral_self_operation.go` (`agent_tool_attempt`, `agent_blocked_action`, `agent_classifier_denial`, `agent_safety_hook_event`) |
| PR-13 — Verdict provenance + governance coverage (beyond original backlog) | ✅ delivered | `PromotionDecisionRecord` message; `GetGovernanceCoverage` RPC; commit `f7f92a18` |

The "What not to do" guardrails below were all honored: observation sources record
`Signals`/`Evidence` but never auto-promote; behavioral-memory still executes no
repairs; AWG and behavioral-memory remain distinct surfaces bridged only by explicit
reconciliation reports; flat `ai_memory` recall is still not treated as governance.

## Three-organ model

The design remains intentionally split across three organs with different authority and purpose:

- **AWG** governs code/change intent.
- **behavioral-memory** governs runtime/operator action.
- **flat ai_memory** remembers incidents, debugging history, and decisions, but it is not governance.

These roles must stay distinct. Runtime learning should not collapse into code-governance, and recall should not be misrepresented as policy.

## Ranked blind spots

> The five blind spots below were the state at the time of writing. Blind spots 1–5
> are now addressed by PR-9 through PR-12 (see status table above); they are retained
> here as the architectural rationale that drove each increment.

### 1. Observation is not yet governed input — RESOLVED (PR-9)

- `ai_executor` is only one afferent nerve.
- `cluster-doctor` findings, `infra_probe` truth-plane state, and `ai_watcher` event streams must become first-class `Signals` and `Evidence`.
- The Scylla `group0` quorum loss is the proof: it was found manually through `infra_probe`, not through governed behavioral input.

Architectural implication: behavioral-memory currently governs decisions after partial intake, but the observation boundary is too narrow. Governed runtime awareness is incomplete until non-agent observation sources enter the same evidence ladder with preserved provenance.

### 2. AWG and behavioral-memory do not yet reconcile — RESOLVED (PR-11)

- AWG invariants can describe runtime-adjacent facts.
- behavioral-memory outcomes can reveal runtime behavior that should become AWG invariants or failure modes.
- There is no bridge yet from confirmed runtime outcome to proposed AWG invariant, failure mode, or test, and there is no drift detector between the two governance surfaces.

Architectural implication: the system has two governance surfaces with overlapping truth about operational safety, but no formal reconciliation path. That leaves runtime discoveries stranded unless a human manually translates them.

### 3. Learning loop does not surface promotion candidates — RESOLVED (PR-10)

- Outcomes accumulate, but no candidate generator says "this repeated pattern is ready for review."
- Human-gated promotion remains correct, but the system needs a safe review queue.

Architectural implication: the current loop records outcomes but does not convert repetition, consistency, and explicit evidence into reviewable proposals. Learning exists as storage, not as a governed promotion pipeline.

### 4. Tonight's lessons are still manual — PARTIALLY RESOLVED (PR-9 ingestion path exists)

> The afferent path now exists (doctor findings, infra_probe truth-plane state, and
> watcher events enter as governed `Signals`/`Evidence`), so future `group0`/schema
> incidents arrive as governed input rather than manual recall. Translating a specific
> past incident into a concrete AWG failure mode/invariant/test is still a human,
> per-incident act — that is by design (no auto-promotion).


- `policy-embed` / `detect-changes` / stale `cluster_controller` grants should become an AWG failure mode, invariant, or test candidate.
- `group0` quorum or schema mutation failure should become a behavioral-memory signal or outcome candidate and possibly an AWG runtime-adjacent invariant.
- Boundary bugs should not live only in comments.

Architectural implication: incident knowledge is still being preserved informally. That is useful for recall, but insufficient for durable governance or repeatable review.

### 5. Agent self-operation is governed by a third layer — RESOLVED (PR-12)

- Claude Code hooks and classifier policy governed unsafe bypass behavior tonight.
- AWG did not cover it because it is not code architecture.
- behavioral-memory did not cover it because self-actions are not yet `Signals` or `CheckActions`.
- Add a future track for agent self-operation signals, actions, and outcomes.

Architectural implication: the agent's own operating boundary is already governed in practice, but outside both current memory-governance systems. That leaves an important behavioral surface unmodeled.

## Backlog proposal (all delivered — see status table)

> The four increments below are kept verbatim as the original specification each
> PR was built against. Every one has shipped; the per-PR `Intent` lines describe
> what the delivered code does.

### PR-9 — Governed Observation Ingestion — ✅ DELIVERED

- Feed `cluster-doctor` findings into behavioral-memory as `Signals` and `Evidence`.
- Feed `infra_probe` truth-plane results as `Signals` and `Evidence`.
- Feed `ai_watcher` events as `Signals`.
- Preserve `source`, `authority`, `condition`, `severity`, `entity_ref`, `cluster_id`, and timestamps.
- PR-9 must preserve source kind and authority level. `infra_probe` truth-plane observations, `cluster-doctor` diagnostic findings, `ai_watcher` events, and agent interpretations must enter with distinct signal kinds or evidence kinds. Diagnostic findings are claims or evidence, not automatic authority.
- No auto-promotion.
- No live repair execution.

Intent: widen the governed afferent path without changing promotion policy or allowing observation components to act.

### PR-10 — Outcome-to-Promotion Candidate Queue — ✅ DELIVERED

- Detect repeated outcomes and themes.
- Generate `PROPOSED_PRINCIPLE` candidates only when evidence, authority, and conditions are explicit.
- Human review remains required.
- No auto-promotion.

Intent: add a safe review queue that converts repeated governed outcomes into candidate principles without weakening human approval gates.

### PR-11 — AWG ↔ Behavioral-Memory Reconciliation — ✅ DELIVERED

- Runtime outcome can propose AWG failure mode, invariant, or test candidates.
- AWG invariant can seed behavioral principle candidates when runtime-relevant.
- Add drift checks so runtime outcomes contradicting or reinforcing AWG invariants are surfaced.

Intent: connect the two governance surfaces without merging them, so code intent and runtime behavior can inform one another through explicit proposals and drift reports.

### PR-12 — Agent Self-Operation Governance — ✅ DELIVERED

- Treat agent tool attempts, blocked actions, classifier denials, and safety-hook events as behavioral `Signals` and `Outcomes`.
- Do not bypass existing hooks.
- Use this only for learning and review, not for weakening safety gates.

Intent: model the agent's own operational behavior as a governed runtime domain while preserving existing external safety controls.

## What not to do

- do not let doctor, probe, or watcher auto-promote principles
- do not make behavioral-memory execute repairs
- do not bypass AWG hooks to write the backlog
- do not merge AWG and behavioral-memory into one blob
- do not treat flat `ai_memory` recall as governance

## Exit condition for this note

> **Met as of 2026-06-20.** The original exit condition ("used as the architectural
> reference for post-PR-8 planning only … does not authorize PR-9 implementation") is
> obsolete: PR-9 through PR-12 plus PR-13 are implemented, tested, and wired into the
> live afferent paths. This note has transitioned from a forward backlog to a delivery
> record.

This note's original intent was to be the architectural reference for post-PR-8 planning. That planning is complete and the increments are delivered. Future governance work (e.g. broadening reconciliation drift detection, or per-incident AWG failure-mode extraction from governed observations) should be tracked in a new design note rather than appended here, so this document stays an accurate record of what the PR-9..PR-13 arc delivered.
