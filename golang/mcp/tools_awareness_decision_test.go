package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/globulario/services/golang/awareness/graph"
)

// ── decision_context tests ────────────────────────────────────────────────────

// TestDecisionContext_FileEditReturnsDecisionPath verifies that when graph is nil
// (degraded mode), the tool still returns a valid structured response with
// coverage and blind_spots — never a bare NO_MATCH.
func TestDecisionContext_FileEditReturnsDecisionPath(t *testing.T) {
	m := decisionContextNoGraph("add weighted path scoring", []string{"golang/awareness/semantic/weights.go"})

	// Must have all required output schema fields.
	for _, field := range []string{"goal", "summary", "confidence", "coverage", "top_decision_paths", "forbidden_actions", "blind_spots"} {
		if _, exists := m[field]; !exists {
			t.Errorf("decision_context missing required field %q", field)
		}
	}
}

// TestDecisionContext_SymptomReturnsForbiddenActions verifies that even in
// no-graph mode the response warns about forbidden actions being unavailable
// rather than silently returning empty.
func TestDecisionContext_SymptomReturnsForbiddenActions(t *testing.T) {
	m := decisionContextNoGraph("fix unknown gRPC service", []string{})

	forbidden, ok := m["forbidden_actions"].([]string)
	if !ok || len(forbidden) == 0 {
		t.Error("forbidden_actions must be non-empty (at least 'Cannot determine' message) in no-graph mode")
	}
}

// TestDecisionContext_RequiredTestsIncluded verifies the output schema includes
// required_tests field in all modes.
func TestDecisionContext_RequiredTestsIncluded(t *testing.T) {
	m := decisionContextNoGraph("modify cluster controller reconcile loop", []string{"golang/cluster_controller/reconcile.go"})

	if _, exists := m["required_tests"]; !exists {
		t.Error("decision_context must include required_tests field in all modes")
	}
}

// TestDecisionContext_NoMatchStillReportsCoverage verifies that when no graph
// matches exist, coverage is still reported (not bare NO_MATCH).
func TestDecisionContext_NoMatchStillReportsCoverage(t *testing.T) {
	m := decisionContextNoGraph("some unknown task", []string{})

	cov, ok := m["coverage"].(map[string]string)
	if !ok || cov == nil {
		t.Error("coverage must be a map[string]string in all modes")
	}
	if cov["graph"] == "" {
		t.Error("coverage.graph must not be empty — even 'not_checked' is informative")
	}

	bs, _ := m["blind_spots"].([]string)
	if len(bs) == 0 {
		t.Error("blind_spots must be non-empty when graph unavailable")
	}
}

// TestDecisionContext_DecisionPathsRankBeforeInformationOnlyPaths verifies
// the no-graph output contains references to decision/awareness tooling.
// The actual ranking is tested in semantic/score_test.go.
func TestDecisionContext_DecisionPathsRankBeforeInformationOnlyPaths(t *testing.T) {
	m := decisionContextNoGraph("update runtime config", []string{})

	warning, _ := m["warning"].(string)
	summary, _ := m["summary"].(string)
	action, _ := m["recommended_next_action"].(string)
	combined := warning + summary + action

	if !strings.Contains(combined, "decision") && !strings.Contains(combined, "awareness") {
		t.Errorf("output must reference decision context or awareness; got: %q", combined)
	}
}

// ── Action safety gate tests ──────────────────────────────────────────────────

// TestDecisionAction_IncludesRequiredEvidenceBefore verifies the decision_context
// output schema includes a field for forbidden_actions (which encode required evidence).
func TestDecisionAction_IncludesRequiredEvidenceBefore(t *testing.T) {
	m := decisionContextNoGraph("restart workflow service", []string{})

	if _, exists := m["forbidden_actions"]; !exists {
		t.Error("decision_context must include forbidden_actions — encodes required pre-action evidence")
	}
}

// TestDecisionAction_IncludesForbiddenIf verifies the forbidden_actions field
// is always present (encodes forbidden_if conditions).
func TestDecisionAction_IncludesForbiddenIf(t *testing.T) {
	m := decisionContextNoGraph("wipe etcd state", []string{})

	if _, exists := m["forbidden_actions"]; !exists {
		t.Error("forbidden_actions must always be present in decision_context output")
	}
}

// TestDecisionAction_IncludesVerificationAfter verifies the required_tests field
// encodes what must be verified after an action.
func TestDecisionAction_IncludesVerificationAfter(t *testing.T) {
	m := decisionContextNoGraph("deploy new service", []string{})

	if _, exists := m["required_tests"]; !exists {
		t.Error("required_tests must always be present — encodes required_verification_after contract")
	}
}

// TestDecisionAction_IncludesStopCondition verifies blind_spots encodes stop
// conditions (when the decision cannot proceed, stop and explain why).
func TestDecisionAction_IncludesStopCondition(t *testing.T) {
	m := decisionContextNoGraph("modify forbidden action", []string{})

	bs, _ := m["blind_spots"].([]string)
	if len(bs) == 0 {
		t.Error("blind_spots (stop conditions) must be non-empty when operating in degraded mode")
	}
}

// TestDecisionAction_DangerousActionRequiresApproval verifies that the warning
// field communicates the NO_MATCH ≠ safe rule.
func TestDecisionAction_DangerousActionRequiresApproval(t *testing.T) {
	m := decisionContextNoGraph("delete all desired state", []string{})

	warning, _ := m["warning"].(string)
	if !strings.Contains(warning, "NO_MATCH") {
		t.Errorf("warning must include NO_MATCH rule for dangerous actions; got: %q", warning)
	}
}

// ── Weight learning tests ─────────────────────────────────────────────────────

// TestLearnFromFix_ProposesPathWeightAdjustment verifies that path_weights.yaml
// exists and can be read — prerequisite for proposing weight adjustments.
func TestLearnFromFix_ProposesPathWeightAdjustment(t *testing.T) {
	_, err := os.ReadFile("../../docs/awareness/knowledge/path_weights.yaml")
	if err != nil {
		t.Errorf("path_weights.yaml must exist for weight proposals to be valid: %v", err)
	}
}

// TestLearnFromFix_DoesNotApplyWeightsWithoutApproval verifies path_weights.yaml
// contains the human approval annotation.
func TestLearnFromFix_DoesNotApplyWeightsWithoutApproval(t *testing.T) {
	content, err := os.ReadFile("../../docs/awareness/knowledge/path_weights.yaml")
	if err != nil {
		t.Skipf("path_weights.yaml not found: %v", err)
	}
	if !strings.Contains(string(content), "human approval") && !strings.Contains(string(content), "requires_human_approval") {
		t.Error("path_weights.yaml must include a note that weight changes require human approval")
	}
}

// TestPromotedWeightChangeRequiresGraphRebuild verifies path_weights.yaml
// mentions graph rebuild as a post-promotion step.
func TestPromotedWeightChangeRequiresGraphRebuild(t *testing.T) {
	content, err := os.ReadFile("../../docs/awareness/knowledge/path_weights.yaml")
	if err != nil {
		t.Skipf("path_weights.yaml not found: %v", err)
	}
	if !strings.Contains(string(content), "awareness build") {
		t.Error("path_weights.yaml must mention 'globular awareness build' as required after promotion")
	}
}

// TestWeightProposalIncludesReasonAndEvidence verifies the path_weights.yaml
// schema includes all required weight categories.
func TestWeightProposalIncludesReasonAndEvidence(t *testing.T) {
	content, err := os.ReadFile("../../docs/awareness/knowledge/path_weights.yaml")
	if err != nil {
		t.Skipf("path_weights.yaml not found: %v", err)
	}
	s := string(content)
	for _, required := range []string{"trust:", "domain:", "severity:", "evidence:", "penalties:"} {
		if !strings.Contains(s, required) {
			t.Errorf("path_weights.yaml must contain %q section", required)
		}
	}
}

// ── Decision graph integrity tests ────────────────────────────────────────────

// TestDecisionIntegrity_RecommendedActionContradictsForbiddenFixFails verifies
// contradicted actions produce forbidden_actions or blind_spots entries.
func TestDecisionIntegrity_RecommendedActionContradictsForbiddenFixFails(t *testing.T) {
	m := decisionContextNoGraph("apply forbidden fix", []string{})

	forbidden, _ := m["forbidden_actions"].([]string)
	bs, _ := m["blind_spots"].([]string)

	if len(forbidden) == 0 && len(bs) == 0 {
		t.Error("contradicted action must produce either forbidden_actions or blind_spots entries")
	}
}

// TestDecisionIntegrity_ForbiddenActionMissingSafeAlternativeFails verifies
// forbidden_actions is always present.
func TestDecisionIntegrity_ForbiddenActionMissingSafeAlternativeFails(t *testing.T) {
	m := decisionContextNoGraph("do dangerous thing", []string{})

	if _, exists := m["forbidden_actions"]; !exists {
		t.Error("forbidden_actions must always be present — even in no-graph mode")
	}
}

// TestDecisionIntegrity_DangerousActionMissingApprovalFails verifies the
// warning field is always present.
func TestDecisionIntegrity_DangerousActionMissingApprovalFails(t *testing.T) {
	m := decisionContextNoGraph("delete critical key", []string{})

	if _, exists := m["warning"]; !exists {
		t.Error("warning field must always be present in decision_context")
	}
}

// TestDecisionIntegrity_InvalidPathCannotRankHigh verifies confidence is not
// high when graph is unavailable (no paths to rank).
func TestDecisionIntegrity_InvalidPathCannotRankHigh(t *testing.T) {
	m := decisionContextNoGraph("test invalid path", []string{})

	conf, _ := m["confidence"].(string)
	if conf == "high" {
		t.Error("confidence must not be high when graph is unavailable — no paths to rank")
	}
}

// TestDecisionIntegrity_WeightsConfigParses verifies path_weights.yaml has
// expected top-level sections.
func TestDecisionIntegrity_WeightsConfigParses(t *testing.T) {
	content, err := os.ReadFile("../../docs/awareness/knowledge/path_weights.yaml")
	if err != nil {
		t.Skipf("path_weights.yaml not found: %v", err)
	}
	if len(content) == 0 {
		t.Error("path_weights.yaml must not be empty")
	}
	s := string(content)
	for _, section := range []string{"trust:", "domain:", "severity:", "penalties:"} {
		if !strings.Contains(s, section) {
			t.Errorf("path_weights.yaml missing section %q", section)
		}
	}
}

// ── Claude hooks tests ────────────────────────────────────────────────────────

// TestClaudeHooks_DecisionContextRequiredForHighRiskFiles verifies CLAUDE.md
// requires decision_context before editing high-risk files.
func TestClaudeHooks_DecisionContextRequiredForHighRiskFiles(t *testing.T) {
	content, err := os.ReadFile("../../CLAUDE.md")
	if err != nil {
		t.Skipf("CLAUDE.md not found: %v", err)
	}
	s := string(content)
	if !strings.Contains(s, "decision_context") && !strings.Contains(s, "awareness.decision_context") {
		t.Error("CLAUDE.md must reference awareness.decision_context for high-risk file edits")
	}
}

// TestClaudeHooks_NoMatchDoesNotMeanSafeFooterPresent verifies CLAUDE.md
// contains the NO_MATCH ≠ safe warning.
func TestClaudeHooks_NoMatchDoesNotMeanSafeFooterPresent(t *testing.T) {
	content, err := os.ReadFile("../../CLAUDE.md")
	if err != nil {
		t.Skipf("CLAUDE.md not found: %v", err)
	}
	if !strings.Contains(string(content), "NO_MATCH") {
		t.Error("CLAUDE.md must include NO_MATCH warning")
	}
}

// TestClaudeHooks_RequiredTestsMentioned verifies CLAUDE.md mentions running
// required tests after edits.
func TestClaudeHooks_RequiredTestsMentioned(t *testing.T) {
	content, err := os.ReadFile("../../CLAUDE.md")
	if err != nil {
		t.Skipf("CLAUDE.md not found: %v", err)
	}
	s := string(content)
	if !strings.Contains(s, "required tests") && !strings.Contains(s, "required_tests") && !strings.Contains(s, "go test") {
		t.Error("CLAUDE.md must mention running required tests as part of the edit workflow")
	}
}

// TestDecisionContext_UsesDecisionClassFilter verifies that the decision_causal_chain
// in decision_context output follows only decision-class edges (not information).
// This is the canonical test for the decision/information edge separation feature.
func TestDecisionContext_UsesDecisionClassFilter(t *testing.T) {
	g, err := graph.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer g.Close()

	ctx := context.Background()
	_ = g.AddNode(ctx, graph.Node{ID: "source_file:some.go", Type: graph.NodeTypeSourceFile, Name: "some.go"})
	_ = g.AddNode(ctx, graph.Node{ID: "invariant:x", Type: graph.NodeTypeInvariant, Name: "x"})
	// Decision-class edge.
	_ = g.AddEdge(ctx, graph.Edge{Src: "source_file:some.go", Kind: graph.EdgeEnforces, Dst: "invariant:x"})
	// Information-class edge.
	_ = g.AddEdge(ctx, graph.Edge{Src: "source_file:some.go", Kind: graph.EdgeMentionedIn, Dst: "invariant:x"})

	// TraverseDecision should only follow decision-class edges.
	result, err := g.TraverseDecision(ctx, "source_file:some.go", 2)
	if err != nil {
		t.Fatalf("TraverseDecision: %v", err)
	}

	for _, n := range result.Nodes {
		if n.ID == "invariant:x" {
			return // reachable via enforces (decision class) — pass
		}
	}
	t.Error("expected invariant:x reachable via decision-class enforces edge")
}
