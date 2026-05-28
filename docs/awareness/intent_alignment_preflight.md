# Claude Integration Instruction: Intent Alignment Preflight

Before making any architecture, service, workflow, schema, remediation, or code change in Globular, run this reasoning protocol.

## Protocol

1. Identify the parent intent.
2. Identify the requirement created by that parent intent.
3. Identify the child intent created by the requirement.
4. Prove that the child intent preserves the parent intent direction.
5. Resolve the child intent requirements to existing capability owners.
6. Choose exactly one: reuse, extend, create, block, unknown impact.
7. Stop if alignment or capability resolution is unknown.

## Required output

```yaml
intent_alignment_preflight:
  parent_intent:
    id: <id>
    direction: <direction>
  requirement:
    id: <id>
    created_by: <parent_intent_id>
  child_intent:
    id: <id>
    created_by_requirement: <requirement_id>
    direction: <direction>
  alignment:
    verdict: aligned | partially_aligned | conflicting | duplicate | unknown_impact
    proof: <proof>
  capability_resolution:
    verdict: reuse | extend | create | block | unknown_impact
    owner: <existing owner or proposed owner>
  forbidden_drift_checked:
    source_of_truth_changed: false
    authority_bypassed: false
    duplicate_capability_created: false
    audit_removed: false
    remediation_unbounded: false
  decision: proceed | stop | block
```

## Non-negotiable rule

Do not modify or generate code when the verdict is `unknown_impact`, `conflicting`, or unresolved `duplicate`.
