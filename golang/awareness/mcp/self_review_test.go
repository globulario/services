package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// docsDir returns the absolute path to docs/awareness, relative to this package.
func selfReviewDocsDir(t *testing.T) string {
	t.Helper()
	// golang/awareness/mcp/ → up 3 dirs → repo root → docs/awareness
	abs, err := filepath.Abs("../../../docs/awareness")
	if err != nil {
		t.Fatalf("resolve docs dir: %v", err)
	}
	return abs
}

// newSelfReviewServer creates a test server with the real docs dir.
func newSelfReviewServer(t *testing.T) *Server {
	t.Helper()
	docsDir := selfReviewDocsDir(t)
	return NewWithGraph(Config{DocsDir: docsDir}, nil)
}

// callSelfReview is a convenience wrapper.
func callSelfReview(t *testing.T, s *Server, args map[string]interface{}) *selfReviewResult {
	t.Helper()
	raw, err := s.CallTool(context.Background(), "awareness.self_review", args)
	if err != nil {
		t.Fatalf("self_review tool error: %v", err)
	}
	result, ok := raw.(*selfReviewResult)
	if !ok {
		t.Fatalf("unexpected result type: %T", raw)
	}
	return result
}

// callRequirementFromCritique is a convenience wrapper.
func callRequirementFromCritique(t *testing.T, s *Server, args map[string]interface{}) *requirementFromCritiqueResult {
	t.Helper()
	raw, err := s.CallTool(context.Background(), "awareness.requirement_from_critique", args)
	if err != nil {
		t.Fatalf("requirement_from_critique tool error: %v", err)
	}
	result, ok := raw.(*requirementFromCritiqueResult)
	if !ok {
		t.Fatalf("unexpected result type: %T", raw)
	}
	return result
}

// ---------------------------------------------------------------------------
// Test 1: Single well-known criticism → P0 gap
// ---------------------------------------------------------------------------

func TestSelfReview_SingleKnownCriticism_ProducesP0Gap(t *testing.T) {
	s := newSelfReviewServer(t)

	result := callSelfReview(t, s, map[string]interface{}{
		"feedback": "Awareness does not automatically learn from verified fixes.",
	})

	// Should find the learn_from_verified_fix gap in closed_gaps (status: implemented).
	if len(result.ClosedGaps) == 0 && len(result.CapabilityGaps) == 0 {
		t.Fatal("expected at least one gap (open or closed), got none")
	}

	// Look for awareness.learn_from_verified_fix in either list.
	found := false
	for _, g := range result.ClosedGaps {
		if g.GapID == "awareness.learn_from_verified_fix" {
			found = true
			if g.Status != "implemented" {
				t.Errorf("expected status=implemented, got %q", g.Status)
			}
			if g.ClosureCondition == "" {
				t.Error("closed gap must have non-empty closure_condition")
			}
			if g.PreventsRepeatCriticism == "" {
				t.Error("closed gap must have non-empty prevents_repeat_criticism")
			}
		}
	}
	// Also accept it in open gaps (if playbook was changed to open).
	if !found {
		for _, g := range result.CapabilityGaps {
			if g.GapID == "awareness.learn_from_verified_fix" {
				found = true
				if g.Priority == "" {
					t.Error("gap must have non-empty priority")
				}
				if g.Requirement == "" {
					t.Error("gap must have non-empty requirement")
				}
				if len(g.ImplementationPlan) == 0 {
					t.Error("gap must have non-empty implementation_plan")
				}
				if len(g.TestsRequired) == 0 {
					t.Error("gap must have non-empty tests_required")
				}
				if g.ClosureCondition == "" {
					t.Error("gap must have non-empty closure_condition")
				}
			}
		}
	}
	if !found {
		t.Errorf("expected awareness.learn_from_verified_fix gap in output, got: closed=%v open=%v",
			gapIDs(result.ClosedGaps), openGapIDs(result.CapabilityGaps))
	}
}

// ---------------------------------------------------------------------------
// Test 2: Multiple criticisms → multiple gaps
// ---------------------------------------------------------------------------

func TestSelfReview_MultipleCriticisms_ProducesMultipleGaps(t *testing.T) {
	s := newSelfReviewServer(t)

	// Use exact keyword phrases from agent_playbooks.yaml to ensure reliable matching.
	// "offline" matches offline_diagnose; "grep-based" matches ast_scan; "causal chain" matches causal_chain.
	feedback := `awareness offline diagnose is missing — when the cluster is down, gRPC is unavailable and there is no way to diagnose.
scan_violations is grep-based and misses indirect patterns hidden in variables.
there is no causal chain linking multiple symptoms to a root cause.`

	result := callSelfReview(t, s, map[string]interface{}{
		"feedback": feedback,
	})

	// All matched gaps — both open and closed (implemented is still a match).
	allGapIDs := append(gapIDs(result.ClosedGaps), openGapIDs(result.CapabilityGaps)...)

	wantPatterns := []struct {
		needle string
		desc   string
	}{
		{"offline_diagnose", "offline_diagnose (offline/grpc unavailable keywords)"},
		{"ast_scan", "ast_scan (grep-based keyword)"},
		{"causal_chain", "causal_chain (causal chain keyword)"},
	}

	for _, want := range wantPatterns {
		found := false
		for _, id := range allGapIDs {
			if contains(id, want.needle) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected gap matching %q (%s) in output, got closed=%v open=%v",
				want.needle, want.desc, gapIDs(result.ClosedGaps), openGapIDs(result.CapabilityGaps))
		}
	}
}

// ---------------------------------------------------------------------------
// Test 3: Vague criticism → incomplete, no fake gap invented
// ---------------------------------------------------------------------------

func TestSelfReview_VagueCriticism_MarkedIncomplete(t *testing.T) {
	s := newSelfReviewServer(t)

	result := callSelfReview(t, s, map[string]interface{}{
		"feedback": "Awareness is bad.",
	})

	// Must not invent a gap.
	if len(result.CapabilityGaps) > 0 {
		t.Errorf("vague feedback must not produce open capability gaps, got: %v", openGapIDs(result.CapabilityGaps))
	}

	// Must produce an incomplete entry or show low confidence.
	if len(result.IncompleteCriticisms) == 0 && result.Confidence != "low" {
		t.Error("vague feedback must produce incomplete_criticisms or low confidence")
	}

	for _, ic := range result.IncompleteCriticisms {
		if ic.MissingEvidence == "" {
			t.Error("incomplete_criticism must have non-empty missing_evidence")
		}
		if ic.Status != "incomplete" {
			t.Errorf("expected status=incomplete, got %q", ic.Status)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 4: All open gaps have closure_condition
// ---------------------------------------------------------------------------

func TestSelfReview_AllGapsHaveClosureCondition(t *testing.T) {
	s := newSelfReviewServer(t)

	feedback := `runtime sources are all noops, no real grpc data.
metric thresholds are hardcoded, yaml not used.
scan is only grep-based.
causal chains not supported.
coverage is inferred not explicit.
learn from fix is missing.`

	result := callSelfReview(t, s, map[string]interface{}{
		"feedback": feedback,
	})

	for _, g := range result.CapabilityGaps {
		if g.ClosureCondition == "" {
			t.Errorf("gap %q has empty closure_condition", g.GapID)
		}
	}
	for _, g := range result.ClosedGaps {
		if g.ClosureCondition == "" {
			t.Errorf("closed gap %q has empty closure_condition", g.GapID)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 5: All open gaps have tests_required
// ---------------------------------------------------------------------------

func TestSelfReview_AllGapsHaveTestsRequired(t *testing.T) {
	s := newSelfReviewServer(t)

	feedback := "runtime sources are all noops. scan is grep-based and cannot detect indirect patterns. no causal chain. thresholds are hardcoded."

	result := callSelfReview(t, s, map[string]interface{}{
		"feedback": feedback,
	})

	for _, g := range result.CapabilityGaps {
		if len(g.TestsRequired) == 0 {
			t.Errorf("gap %q has empty tests_required", g.GapID)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 6: All gaps have prevents_repeat_criticism
// ---------------------------------------------------------------------------

func TestSelfReview_AllGapsHavePreventsRepeatCriticism(t *testing.T) {
	s := newSelfReviewServer(t)

	feedback := "runtime sources are all noops. scan is grep-based. no causal chain. thresholds are hardcoded. coverage is inferred."

	result := callSelfReview(t, s, map[string]interface{}{
		"feedback": feedback,
	})

	for _, g := range result.CapabilityGaps {
		if g.PreventsRepeatCriticism == "" {
			t.Errorf("gap %q has empty prevents_repeat_criticism", g.GapID)
		}
	}
	for _, g := range result.ClosedGaps {
		if g.PreventsRepeatCriticism == "" {
			t.Errorf("closed gap %q has empty prevents_repeat_criticism", g.GapID)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 7: Already-implemented gap detected → appears in closed_gaps
// ---------------------------------------------------------------------------

func TestSelfReview_AlreadyImplementedGap_InClosedGaps(t *testing.T) {
	s := newSelfReviewServer(t)

	// The AST scanner is implemented (status: implemented in playbooks).
	result := callSelfReview(t, s, map[string]interface{}{
		"feedback": "There is no AST scanner for violations. The scan is only grep-based.",
	})

	found := false
	for _, g := range result.ClosedGaps {
		if contains(g.GapID, "ast_scan") {
			found = true
			if g.Status != "implemented" {
				t.Errorf("expected status=implemented for %q, got %q", g.GapID, g.Status)
			}
		}
	}
	if !found {
		t.Errorf("expected ast_scan gap in closed_gaps (status: implemented), got closed=%v open=%v",
			gapIDs(result.ClosedGaps), openGapIDs(result.CapabilityGaps))
	}

	// Must NOT appear in open gaps.
	for _, g := range result.CapabilityGaps {
		if contains(g.GapID, "ast_scan") {
			t.Errorf("ast_scan gap should be in closed_gaps (implemented), but found in open capability_gaps")
		}
	}
}

// ---------------------------------------------------------------------------
// Test 8: requirement_from_critique — single criticism → single requirement
// ---------------------------------------------------------------------------

func TestRequirementFromCritique_SingleCriticism(t *testing.T) {
	s := newSelfReviewServer(t)

	result := callRequirementFromCritique(t, s, map[string]interface{}{
		"criticism": "scan_violations is grep-based, not AST-based",
		"scope":     "scan_violations",
	})

	if result.GapID == "" {
		t.Error("gap_id must be non-empty")
	}
	if result.Criticism == "" {
		t.Error("criticism must be non-empty")
	}
	if result.Requirement == "" {
		t.Error("requirement must be non-empty")
	}
	if result.ClosureCondition == "" {
		t.Error("closure_condition must be non-empty")
	}
	if result.Confidence == "" {
		t.Error("confidence must be non-empty")
	}
	// Should match ast_scan pattern.
	if !contains(result.GapID, "ast_scan") {
		t.Errorf("expected gap_id to reference ast_scan, got %q", result.GapID)
	}
}

// ---------------------------------------------------------------------------
// Test 9: Graceful degradation — missing docs dir
// ---------------------------------------------------------------------------

func TestSelfReview_MissingDocsDir_GracefulDegradation(t *testing.T) {
	tmpDir := t.TempDir()
	// Use a non-existent docs dir inside the temp dir.
	s := NewWithGraph(Config{DocsDir: filepath.Join(tmpDir, "nonexistent")}, nil)

	result := callSelfReview(t, s, map[string]interface{}{
		"feedback": "awareness is missing features",
	})

	// Must not crash — must return a degraded result.
	if result == nil {
		t.Fatal("expected non-nil result on missing docs dir")
	}
	// Confidence should be low.
	if result.Confidence != "low" {
		t.Errorf("expected low confidence when docs dir missing, got %q", result.Confidence)
	}
}

// ---------------------------------------------------------------------------
// Test 10: parseFeedbackSegments handles various delimiters
// ---------------------------------------------------------------------------

func TestParseFeedbackSegments(t *testing.T) {
	feedback := `First criticism about runtime.

Second criticism about scan.
- Third bullet point.
* Fourth bullet.

Final paragraph.`

	segments := parseFeedbackSegments(feedback)

	if len(segments) < 4 {
		t.Errorf("expected at least 4 segments, got %d: %v", len(segments), segments)
	}

	// The full text should appear as the last segment.
	found := false
	for _, s := range segments {
		if len(s) > 50 { // longer than any individual segment
			found = true
			break
		}
	}
	if !found {
		t.Error("expected full feedback text as one segment for cross-sentence matching")
	}
}

// ---------------------------------------------------------------------------
// Test 11: Tools are registered
// ---------------------------------------------------------------------------

func TestSelfReviewToolsRegistered(t *testing.T) {
	s := NewWithGraph(Config{}, nil)

	tools := []string{
		"awareness.self_review",
		"awareness.requirement_from_critique",
	}
	for _, name := range tools {
		if !s.HasTool(name) {
			t.Errorf("tool %q not registered", name)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 12: Open gap (snapshot_storage) surfaces as open, not closed
// ---------------------------------------------------------------------------

func TestSelfReview_OpenGap_NotInClosedGaps(t *testing.T) {
	s := newSelfReviewServer(t)

	// snapshot_storage is marked status: open in agent_playbooks.yaml.
	result := callSelfReview(t, s, map[string]interface{}{
		"feedback": "runtime snapshots are not stored, suggest_incident cannot compare to baseline snapshot",
	})

	// snapshot_storage must NOT appear in closed_gaps.
	for _, g := range result.ClosedGaps {
		if contains(g.GapID, "snapshot_storage") {
			t.Errorf("snapshot_storage is open but appeared in closed_gaps: %q", g.GapID)
		}
	}

	// snapshot_storage SHOULD appear in open capability_gaps if matched.
	found := false
	for _, g := range result.CapabilityGaps {
		if contains(g.GapID, "snapshot_storage") {
			found = true
			if g.Status != "open" {
				t.Errorf("expected status=open for snapshot_storage, got %q", g.Status)
			}
		}
	}
	if !found {
		t.Logf("snapshot_storage gap not found in open gaps (may not have matched keywords) — open=%v", openGapIDs(result.CapabilityGaps))
	}
}

// ---------------------------------------------------------------------------
// Test 13: Duplicate proposal detection
// ---------------------------------------------------------------------------

func TestSelfReview_DuplicateProposalDetected(t *testing.T) {
	// Create a temp docs dir with a fake proposal that overlaps a gap.
	tmpDir := t.TempDir()
	knowledgeDir := filepath.Join(tmpDir, "knowledge")
	proposalsDir := filepath.Join(tmpDir, "proposals")
	if err := os.MkdirAll(knowledgeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(proposalsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Copy agent_playbooks.yaml from real docs dir.
	realDocsDir := selfReviewDocsDir(t)
	playbooksData, err := os.ReadFile(filepath.Join(realDocsDir, "knowledge", "agent_playbooks.yaml"))
	if err != nil {
		t.Fatalf("read agent_playbooks.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(knowledgeDir, "agent_playbooks.yaml"), playbooksData, 0o644); err != nil {
		t.Fatal(err)
	}

	// Write a fake proposal that covers snapshot_storage.
	fakeProposal := `proposal:
  id: proposal.awareness.snapshot_storage
  source_incident: TEST-001
  status: DRAFT
  created_at: "2026-01-01T00:00:00Z"
failure_modes:
  - id: failure_mode.snapshot_storage_missing
    title: Snapshot storage missing
evidence:
  source_incident: TEST-001
`
	if err := os.WriteFile(filepath.Join(proposalsDir, "test-snapshot.yaml"), []byte(fakeProposal), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewWithGraph(Config{DocsDir: tmpDir}, nil)

	result := callSelfReview(t, s, map[string]interface{}{
		"feedback": "runtime snapshots are not stored, suggest_incident cannot compare to baseline snapshot",
	})

	// If snapshot_storage matched and a proposal exists, it should be marked already_proposed.
	for _, g := range result.CapabilityGaps {
		if contains(g.GapID, "snapshot_storage") {
			if !g.AlreadyProposed {
				t.Errorf("gap %q should be marked already_proposed=true (fake proposal exists)", g.GapID)
			}
			if g.DuplicateOf == "" {
				t.Errorf("gap %q should have non-empty duplicate_of when already_proposed=true", g.GapID)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func gapIDs(gaps []closedGapResult) []string {
	out := make([]string, len(gaps))
	for i, g := range gaps {
		out[i] = g.GapID
	}
	return out
}

func openGapIDs(gaps []capabilityGapResult) []string {
	out := make([]string, len(gaps))
	for i, g := range gaps {
		out[i] = g.GapID
	}
	return out
}

func contains(s, sub string) bool {
	return len(sub) > 0 && (s == sub || len(s) >= len(sub) && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
