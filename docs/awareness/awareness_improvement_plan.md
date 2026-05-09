# Globular Awareness Improvement Plan

## Scope

Implement five improvements in awareness to reduce low-risk friction while preserving high-risk safety:

1. Low-risk fast-path gating.
2. Rule freshness and graph drift enforcement.
3. Confidence + known-unknown signaling.
4. Degraded-mode playbook.
5. False-positive feedback loop with audited promotion.

## Non-negotiable invariants

- `awareness.preflight.required_before_edit`
- `awareness.mcp.decision_context_no_bare_no_match`
- `awareness.no_false_silence_for_sensitive_tasks`
- `awareness.knowledge.graph_rebuild_after_yaml_edit`
- `awareness.graph.domain_classification_required`
- `awareness.graph_edges_need_provenance`
- `awareness.weight_learning.human_approval_required`
- `awareness.semantic_paths_must_explain_why`

## Forbidden fixes to preserve

- Do not skip preflight before edits.
- Do not treat graph/query silence as safe.
- Do not return bare `no_match` for sensitive tasks.
- Do not apply weight changes without approval and rebuild.
- Do not add edges without provenance/extractor evidence.
- Do not lower trust penalties to hide invalid decision paths.

## Epic A: Low-risk fast path

### Requirement

Add deterministic risk tiers (`low`, `medium`, `high`) so low-risk edits receive a narrowed but still safe decision context.

### Ownership and files

- CLI and preflight orchestration:
  - `golang/globularcli/awareness_preflight_cmd.go`
  - `golang/awareness/enforce/contracts.go`
- High-risk classification sources:
  - `docs/awareness/high_risk_files.yaml`
  - `docs/awareness/guardrails.yaml`

### Acceptance criteria

- Low-risk tasks return reduced invariant/failure-mode set.
- High-risk tasks never use fast-path.
- Fast-path is disabled when graph is stale or confidence is below threshold.

### Required tests

- `TestSmokePassesOnKnownInvariants`
- `TestNodeContextSmokePasses`
- `TestDecisionPathType_PreEdit`
- New: `TestPreflightFastPath_EnabledForLowRisk`
- New: `TestPreflightFastPath_DisabledForHighRisk`
- New: `TestPreflightFastPath_DisabledWhenGraphStale`

## Epic B: Freshness and drift enforcement

### Requirement

Track freshness metadata for knowledge sources and hard-gate confidence when YAML changes are newer than graph build artifacts.

### Ownership and files

- Drift and build checks:
  - `golang/awareness/enforce/drift.go`
  - `golang/awareness/enforce/drift_test.go`
  - `golang/globularcli/awareness_cmds.go`
  - `golang/globularcli/awareness_ci_check_cmd.go`

### Acceptance criteria

- Stale graph is always surfaced in agent output.
- Sensitive tasks in stale-graph state cannot return high-confidence decisions.
- Output includes explicit remediation command path.

### Required tests

- `TestGraphDriftNoStaleRefs`
- `TestAwarenessBuildCleanRemovesOldDB`
- New: `TestPreflightBlocksHighConfidenceWhenGraphStale`
- New: `TestPreflightStaleGraphIncludesRemediation`

## Epic C: Confidence and known-unknowns

### Requirement

Return explicit confidence and explainability factors (`coverage`, `provenance`, `freshness`, `path-quality`) with a hard `UNKNOWN_NOT_SAFE` status for sensitive gaps.

### Ownership and files

- Decision context and formatting:
  - `golang/awareness/enforce/coverage.go`
  - `golang/awareness/enforce/format.go`
  - `golang/awareness/enforce/summary.go`
  - `golang/globularcli/awareness_cmds.go`

### Acceptance criteria

- No bare `no_match` responses for sensitive tasks.
- Coverage gaps and blind spots are explicit in output.
- Decision path includes `why` explanation.

### Required tests

- `TestDecisionContext_NoMatchStillReportsCoverage`
- `TestDecisionContext_NoBareNoMatchWhenLowerTrustPathsExist`
- `TestSemanticPathIncludesExplanation`
- `TestDecisionIntegrity_InvalidPathCannotRankHigh`
- New: `TestDecisionContext_UnknownNotSafeForSensitiveTask`

## Epic D: Degraded-mode playbook

### Requirement

When graph or query subsystems are degraded, emit constrained, deterministic guidance from raw knowledge fallback with explicit blocked actions and stop conditions.

### Ownership and files

- Enforcement and report logic:
  - `golang/awareness/enforce/report.go`
  - `golang/awareness/enforce/strict.go`
  - `docs/awareness/preflight_audit.md`
  - `docs/awareness/operational_handoff.md`

### Acceptance criteria

- Degraded mode is visible in output and audit records.
- Guidance distinguishes `allowed next steps` vs `blocked actions`.
- Sensitive tasks default to safe blocking behavior in degraded mode.

### Required tests

- `TestSelfCheckReportsNoisySections`
- `TestDecisionAction_DangerousActionRequiresApproval`
- `TestDecisionIntegrity_DangerousActionMissingApprovalFails`
- New: `TestPreflightDegradedMode_EmitsBlockedActions`
- New: `TestPreflightDegradedMode_UsesRawKnowledgeFallback`

## Epic E: False-positive feedback and promotion safety

### Requirement

Add structured false-positive capture and proposal workflow that only promotes after human approval and graph rebuild.

### Ownership and files

- Learning workflow:
  - `golang/awareness/learning/proposal.go`
  - `golang/awareness/learning/promote.go`
  - `golang/awareness/learning/validate.go`
  - `golang/awareness/learning/incident_bundle.go`
- Knowledge/proposal docs:
  - `docs/awareness/proposals/`
  - `docs/awareness/learning_rules.yaml`

### Acceptance criteria

- Every promoted rule/weight has evidence + reviewer approval.
- No direct promotion from MCP pathways.
- Promotion requires graph rebuild and clean validation.

### Required tests

- `TestLearnFromFix_DoesNotApplyWeightsWithoutApproval`
- `TestPromotedWeightChangeRequiresGraphRebuild`
- `TestWeightProposalIncludesReasonAndEvidence`
- `TestApproveProposalDoesNotPromote`
- `TestMCPCheckFailsWhenPromoteProposalExposed`
- New: `TestFalsePositiveFeedback_RecordHasRequiredFields`

## Cross-epic metrics

- Reduce false-positive rate on low-risk edits by at least 30%.
- Zero bare `no_match` on sensitive tasks.
- 100% of promotions linked to approval and evidence bundle.
- 100% stale-graph detections include remediation text.

## Delivery order

1. Epic B (freshness/drift) and Epic C (confidence) together.
2. Epic D (degraded mode).
3. Epic A (fast path) after B/C guardrails are enforced.
4. Epic E (feedback loop and promotion hardening).

## Definition of done

- All required tests pass.
- New tests for each epic pass.
- Awareness preflight output includes: impacted invariants, known failure modes, forbidden fixes, did-we-fix status, required tests.
- Documentation in this file and runbook docs reflects final behavior.

## Implementation status (2026-05-08)

- Epic A: implemented in `golang/awareness/preflight/*` with `risk_tier` and `fast_path_applied`.
- Epic B: implemented in `golang/awareness/preflight/*` with stale-graph-aware confidence gating.
- Epic C: implemented in `golang/awareness/preflight/*` with `confidence_factors` and `safety_status=UNKNOWN_NOT_SAFE`.
- Epic D: implemented in `golang/awareness/preflight/*` with `degraded_mode` playbook output.
- Epic E: implemented in `golang/awareness/learning/*` with structured false-positive feedback and promotion reviewer gating.

### Verification evidence

- Preflight tests:
  - `TestDecisionContext_UnknownNotSafeForSensitiveTask`
  - `TestPreflightConfidenceFactorsArePopulated`
  - `TestPreflightDegradedMode_EmitsBlockedActions`
  - `TestPreflightDegradedMode_UsesRawKnowledgeFallback`
  - `TestPreflightFastPath_EnabledForLowRisk`
  - `TestPreflightFastPath_DisabledForHighRisk`
- Learning tests:
  - `TestFalsePositiveFeedback_RecordHasRequiredFields`
  - `TestFalsePositiveFeedback_MissingRequiredFieldsBlocks`
  - `TestPromoteProposalRejectsApprovedWithoutReviewer`
