# Behavioral Memory Runtime Awareness Backlog

> Status: post-PR-8 design note and next-PR backlog.
> Scope: preserve the three-organ model, capture the architectural blind spots, and rank the next governance increments without starting implementation.

## Three-organ model

The design remains intentionally split across three organs with different authority and purpose:

- **AWG** governs code/change intent.
- **behavioral-memory** governs runtime/operator action.
- **flat ai_memory** remembers incidents, debugging history, and decisions, but it is not governance.

These roles must stay distinct. Runtime learning should not collapse into code-governance, and recall should not be misrepresented as policy.

## Ranked blind spots

### 1. Observation is not yet governed input

- `ai_executor` is only one afferent nerve.
- `cluster-doctor` findings, `infra_probe` truth-plane state, and `ai_watcher` event streams must become first-class `Signals` and `Evidence`.
- The Scylla `group0` quorum loss is the proof: it was found manually through `infra_probe`, not through governed behavioral input.

Architectural implication: behavioral-memory currently governs decisions after partial intake, but the observation boundary is too narrow. Governed runtime awareness is incomplete until non-agent observation sources enter the same evidence ladder with preserved provenance.

### 2. AWG and behavioral-memory do not yet reconcile

- AWG invariants can describe runtime-adjacent facts.
- behavioral-memory outcomes can reveal runtime behavior that should become AWG invariants or failure modes.
- There is no bridge yet from confirmed runtime outcome to proposed AWG invariant, failure mode, or test, and there is no drift detector between the two governance surfaces.

Architectural implication: the system has two governance surfaces with overlapping truth about operational safety, but no formal reconciliation path. That leaves runtime discoveries stranded unless a human manually translates them.

### 3. Learning loop does not surface promotion candidates

- Outcomes accumulate, but no candidate generator says "this repeated pattern is ready for review."
- Human-gated promotion remains correct, but the system needs a safe review queue.

Architectural implication: the current loop records outcomes but does not convert repetition, consistency, and explicit evidence into reviewable proposals. Learning exists as storage, not as a governed promotion pipeline.

### 4. Tonight's lessons are still manual

- `policy-embed` / `detect-changes` / stale `cluster_controller` grants should become an AWG failure mode, invariant, or test candidate.
- `group0` quorum or schema mutation failure should become a behavioral-memory signal or outcome candidate and possibly an AWG runtime-adjacent invariant.
- Boundary bugs should not live only in comments.

Architectural implication: incident knowledge is still being preserved informally. That is useful for recall, but insufficient for durable governance or repeatable review.

### 5. Agent self-operation is governed by a third layer

- Claude Code hooks and classifier policy governed unsafe bypass behavior tonight.
- AWG did not cover it because it is not code architecture.
- behavioral-memory did not cover it because self-actions are not yet `Signals` or `CheckActions`.
- Add a future track for agent self-operation signals, actions, and outcomes.

Architectural implication: the agent's own operating boundary is already governed in practice, but outside both current memory-governance systems. That leaves an important behavioral surface unmodeled.

## Backlog proposal

### PR-9 — Governed Observation Ingestion

- Feed `cluster-doctor` findings into behavioral-memory as `Signals` and `Evidence`.
- Feed `infra_probe` truth-plane results as `Signals` and `Evidence`.
- Feed `ai_watcher` events as `Signals`.
- Preserve `source`, `authority`, `condition`, `severity`, `entity_ref`, `cluster_id`, and timestamps.
- No auto-promotion.
- No live repair execution.

Intent: widen the governed afferent path without changing promotion policy or allowing observation components to act.

### PR-10 — Outcome-to-Promotion Candidate Queue

- Detect repeated outcomes and themes.
- Generate `PROPOSED_PRINCIPLE` candidates only when evidence, authority, and conditions are explicit.
- Human review remains required.
- No auto-promotion.

Intent: add a safe review queue that converts repeated governed outcomes into candidate principles without weakening human approval gates.

### PR-11 — AWG ↔ Behavioral-Memory Reconciliation

- Runtime outcome can propose AWG failure mode, invariant, or test candidates.
- AWG invariant can seed behavioral principle candidates when runtime-relevant.
- Add drift checks so runtime outcomes contradicting or reinforcing AWG invariants are surfaced.

Intent: connect the two governance surfaces without merging them, so code intent and runtime behavior can inform one another through explicit proposals and drift reports.

### PR-12 — Agent Self-Operation Governance

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

This note is complete when it is used as the architectural reference for post-PR-8 planning only. It does not authorize PR-9 implementation, schema changes, runtime ingestion wiring, or governance-surface unification.
