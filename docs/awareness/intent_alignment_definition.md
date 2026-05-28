# Intent Alignment Contract for Globular Awareness

## Purpose

This definition gives Claude, Codex, and the Globular awareness system one meta-level rule above the existing intent graph.

The existing loop is:

```text
Intent -> Requirement -> Capability Resolution -> Reuse / Extend / Create / Block
```

The missing recursive rule is:

```text
Intent -> Requirement -> Child Intent -> Child Requirement -> Child Intent ...
```

But every child intent must remain aligned with the parent intent that indirectly created it.

## Core definition

**Intent alignment** means that a lower-level intent created from a requirement must preserve the direction, authority boundary, and purpose of the parent intent.

A child intent may specialize the parent intent. It may operationalize it. It may narrow it into a concrete service, workflow, schema, invariant, or remediation behavior.

It must not broaden, invert, bypass, duplicate, or replace the parent intent.

## Core law

```text
No requirement-born child intent may create implementation work until it proves alignment with its parent intent.
```

Or stronger:

```text
A child intent may only specialize its parent. It may not mutate the parent's meaning.
```

## Recursive model

```text
Parent Intent
  -> creates Requirement
    -> creates Child Intent
      -> creates Child Requirement
        -> creates Grandchild Intent
```

At every level:

```text
child_intent.direction must be contained inside parent_intent.direction
```

This does not mean the child is identical to the parent. It means the child serves the same architectural direction at a lower altitude.

## Why this matters

Without this rule, decomposition can drift.

A requirement can create a technically useful child intent that violates the parent.

Example:

```text
Parent intent:
  repository.metadata_is_authority

Requirement:
  durable_artifact_storage

Bad child intent:
  objectstore becomes source of artifact truth

Aligned child intent:
  objectstore provides durable blob storage while repository metadata remains authoritative
```

Both child intents respond to the same requirement. Only one keeps the chain angled correctly.

## Alignment verdicts

Every child intent must receive exactly one alignment verdict.

### aligned

The child intent specializes or operationalizes the parent without changing authority, ownership, or direction.

Allowed result: proceed to capability resolution.

### partially_aligned

The child intent appears useful but introduces risk, ambiguity, shared ownership, or unclear evidence.

Allowed result: no code generation. Produce a bounded impact report.

### conflicting

The child intent inverts, bypasses, replaces, or contradicts the parent intent.

Allowed result: block and escalate a design decision.

### duplicate

The child intent recreates an already-owned capability.

Allowed result: link to the existing capability owner or propose an extension.

### unknown_impact

Awareness cannot prove alignment with current graph evidence.

Allowed result: stop. Expand context through awareness preflight.

## Required graph edges

Use these edges in the awareness graph:

```yaml
parent_intent --creates_requirement--> requirement
requirement --creates_child_intent--> child_intent
child_intent --must_align_with--> parent_intent
child_intent --resolves_requirement--> requirement
requirement --resolved_as--> capability_resolution_verdict
child_intent --implemented_by--> service | workflow | schema | code | invariant
runtime_evidence --proves_or_disproves--> intent
```

## Required schema

```yaml
id: <intent_id>
kind: intent
parent_intent: <parent_intent_id>
created_from_requirement: <requirement_id>
statement: <what this child intent means>
direction: <the architectural direction it preserves>
authority_boundary: <what it owns and what it must not own>
alignment:
  verdict: aligned | partially_aligned | conflicting | duplicate | unknown_impact
  aligns_with: <parent_intent_id>
  proof: <short reason>
  forbidden_drift:
    - <drift pattern this child must avoid>
capability_resolution:
  verdict: satisfied_by_existing_capability | extends_existing_capability | creates_new_capability | blocked_by_conflict | unknown_impact
  owner: <service/workflow/contract/invariant id or null>
required_invariants:
  - <invariant_id>
required_tests:
  - <test or proof requirement>
```

## Forbidden drift patterns

A child intent is invalid when it does any of the following:

```text
- broadens the parent scope
- inverts the parent direction
- bypasses the parent authority
- replaces the source of truth
- duplicates an existing capability owner
- hides degraded, stale, missing, or unknown state
- turns runtime observation into desired-state mutation
- turns convenience behavior into authority
- skips audit, approval, or operator ceremony
- makes remediation unbounded
```

## Agent rule before code edits

Before creating or modifying code, Claude/Codex must output this block internally or in a preflight report:

```yaml
intent_alignment_preflight:
  parent_intent: <id>
  requirement: <id>
  child_intent: <id>
  alignment_verdict: aligned | partially_aligned | conflicting | duplicate | unknown_impact
  alignment_proof: <why the child preserves the parent direction>
  capability_resolution: reuse | extend | create | block | unknown
  existing_owner: <service/workflow/schema/invariant or null>
  forbidden_drift_checked:
    - source_of_truth_not_changed
    - no_duplicate_capability
    - no_authority_bypass
    - runtime_evidence_not_authority
    - audit_preserved
    - remediation_bounded
  decision: proceed | stop_for_impact_report | block_for_design_decision
```

## Hard stop conditions

The agent must stop before editing when any of these are true:

```text
- no parent intent is identified
- no requirement is identified
- child intent has no alignment verdict
- alignment verdict is conflicting
- alignment verdict is duplicate and no reuse/extension path is chosen
- alignment verdict is unknown_impact
- capability resolution is unknown_impact
- change creates or moves source of truth without explicit approved intent
```

## Integration points

Recommended file placement:

```text
docs/intent/meta/intent_requirement_child_alignment_contract.yaml
.awareness/rules/intent_alignment_contract.md
.awareness/invariants/child_intent_must_align_with_parent.yaml
```

Recommended awareness commands or checks:

```text
awareness preflight --changed <files> --require-intent-alignment
awareness explain-intent <intent_id> --show-parent-chain
awareness resolve-requirement <requirement_id> --capability-owners
awareness validate-graph --check-child-intent-alignment
```

## Minimal invariant

```yaml
id: awareness.child_intent_must_align_with_parent
kind: invariant
severity: critical
statement: >
  Every child intent created from a requirement must have a parent intent,
  an alignment verdict, and a capability-resolution verdict before implementation work is allowed.
violation_when:
  - child intent has no parent_intent
  - child intent has no created_from_requirement
  - child intent has no alignment.verdict
  - alignment.verdict in [conflicting, duplicate, unknown_impact]
  - capability_resolution.verdict == unknown_impact
required_response:
  - block generation or modification
  - produce bounded impact report
  - require operator/design approval if authority boundary changes
```

## Short formulation for Claude

Use this as the compact instruction:

> When an intent creates requirements, those requirements may create lower-level intents. Every lower-level intent must prove that it specializes and preserves the direction of its parent intent. If the child intent broadens, inverts, bypasses, duplicates, or replaces the parent intent, it is invalid. Unknown impact is a stop condition. Code may only be generated after parent intent, requirement, child intent alignment, and capability resolution are all explicit.
