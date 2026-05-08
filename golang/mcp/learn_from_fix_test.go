package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/awareness/learning"
)

// setupLearnFromFixServer builds a minimal server with a writable docs dir.
func setupLearnFromFixServer(t *testing.T) (*server, string) {
	t.Helper()
	docsDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(docsDir, "proposals"), 0o755)
	_ = os.MkdirAll(filepath.Join(docsDir, "knowledge"), 0o755)
	s := NewWithGraph(Config{DocsDir: docsDir}, nil)
	t.Cleanup(func() { s.Close() })
	return s, docsDir
}

// TestLearnFromFix_Registered verifies the tool is available.
func TestLearnFromFix_Registered(t *testing.T) {
	s := NewWithGraph(Config{}, nil)
	if !s.HasTool("awareness.learn_from_fix") {
		t.Error("awareness.learn_from_fix must be registered")
	}
}

// TestLearnFromFix_NewFailureMode verifies a failure mode proposal is generated.
func TestLearnFromFix_NewFailureMode(t *testing.T) {
	s, docsDir := setupLearnFromFixServer(t)

	result, err := s.CallTool(context.Background(), "awareness.learn_from_fix", map[string]interface{}{
		"symptom_text": "connection refused to 127.0.0.1:12000",
		"root_cause":   "hardcoded loopback address bypasses etcd service discovery",
		"fix_summary":  "replaced 127.0.0.1 with address resolved from etcd",
	})
	if err != nil {
		t.Fatalf("learn_from_fix error: %v", err)
	}

	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}

	proposals, ok := m["proposals"].([]learnFromFixProposal)
	if !ok || len(proposals) == 0 {
		t.Fatalf("expected non-empty proposals, got %v", m["proposals"])
	}

	// At least one proposal should target failure_modes.yaml
	found := false
	for _, p := range proposals {
		if p.TargetFile == "failure_modes.yaml" {
			found = true
			if p.EntryID == "" {
				t.Error("failure mode proposal must have non-empty EntryID")
			}
			if p.Operation == "" {
				t.Error("failure mode proposal must have non-empty Operation")
			}
		}
	}
	if !found {
		t.Error("expected a failure_modes.yaml proposal")
	}

	// Verify the proposal file was written to proposals dir.
	proposalPath, _ := m["proposal_path"].(string)
	if proposalPath == "" {
		t.Error("expected proposal_path in result")
	}
	if _, err := os.Stat(proposalPath); err != nil {
		t.Errorf("proposal file not written to disk: %v", err)
	}
	// Verify file is within docsDir/proposals
	proposalsDir := filepath.Join(docsDir, "proposals")
	rel, err := filepath.Rel(proposalsDir, proposalPath)
	if err != nil || filepath.IsAbs(rel) || len(rel) == 0 {
		t.Errorf("proposal file %q is outside proposals dir %q", proposalPath, proposalsDir)
	}
}

// TestLearnFromFix_ForbiddenFixProposal verifies forbidden fix proposal when known_bad_fix is provided.
func TestLearnFromFix_ForbiddenFixProposal(t *testing.T) {
	s, _ := setupLearnFromFixServer(t)

	result, err := s.CallTool(context.Background(), "awareness.learn_from_fix", map[string]interface{}{
		"symptom_text":  "workflow stuck after controller restart",
		"root_cause":    "stale etcd leader lease not cleared",
		"fix_summary":   "added lease expiry guard in controller startup",
		"known_bad_fix": "manually deleting etcd keys to force leader election",
	})
	if err != nil {
		t.Fatalf("learn_from_fix error: %v", err)
	}

	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}

	proposals, _ := m["proposals"].([]learnFromFixProposal)
	found := false
	for _, p := range proposals {
		if p.TargetFile == "forbidden_fixes.yaml" {
			found = true
			if !p.RequiresHumanApproval {
				t.Error("forbidden fix proposal must require human approval")
			}
			if p.Confidence != "high" {
				t.Errorf("forbidden fix confidence should be 'high', got %q", p.Confidence)
			}
			if !containsSubstr(p.YAMLPatch, "deleting etcd") && !containsSubstr(p.YAMLPatch, "manually") {
				t.Logf("yaml_patch: %s", p.YAMLPatch)
			}
		}
	}
	if !found {
		t.Error("expected a forbidden_fixes.yaml proposal when known_bad_fix is provided")
	}
}

// TestLearnFromFix_ScanRuleProposal verifies scan rule proposal when Go files changed and bad pattern in symptom.
func TestLearnFromFix_ScanRuleProposal(t *testing.T) {
	s, _ := setupLearnFromFixServer(t)

	result, err := s.CallTool(context.Background(), "awareness.learn_from_fix", map[string]interface{}{
		"symptom_text":  "grpc.Dial used with 127.0.0.1:10004 causing single-node limitation",
		"root_cause":    "hardcoded localhost address prevents multi-node operation",
		"fix_summary":   "resolve address from etcd service discovery",
		"changed_files": []interface{}{"golang/cluster_controller/server.go", "golang/workflow/client.go"},
	})
	if err != nil {
		t.Fatalf("learn_from_fix error: %v", err)
	}

	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}

	proposals, _ := m["proposals"].([]learnFromFixProposal)
	found := false
	for _, p := range proposals {
		if p.TargetFile == "knowledge/scan_rules.yaml" {
			found = true
			if p.YAMLPatch == "" {
				t.Error("scan rule proposal must have yaml_patch")
			}
		}
	}
	if !found {
		t.Logf("proposals: %+v", proposals)
		t.Error("expected a scan_rules.yaml proposal when Go files changed and loopback in symptom")
	}
}

// TestLearnFromFix_BlindSpotWhenVerificationMissing verifies blind spot is set when verification is absent.
func TestLearnFromFix_BlindSpotWhenVerificationMissing(t *testing.T) {
	s, _ := setupLearnFromFixServer(t)

	result, err := s.CallTool(context.Background(), "awareness.learn_from_fix", map[string]interface{}{
		"symptom_text": "minio offline disk after power outage",
		"root_cause":   "disk not re-mounted after reboot",
		"fix_summary":  "added mount check to startup script",
		// no verification
	})
	if err != nil {
		t.Fatalf("learn_from_fix error: %v", err)
	}

	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}

	blindSpots, _ := m["blind_spots"].([]string)
	found := false
	for _, bs := range blindSpots {
		if containsSubstr(bs, "verification") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'verification' blind spot, got: %v", blindSpots)
	}
}

// TestLearnFromFix_HumanApprovalRequiredByDefault verifies all proposals have requires_human_approval=true.
func TestLearnFromFix_HumanApprovalRequiredByDefault(t *testing.T) {
	s, _ := setupLearnFromFixServer(t)

	result, err := s.CallTool(context.Background(), "awareness.learn_from_fix", map[string]interface{}{
		"symptom_text":  "address already in use on port 12000",
		"root_cause":    "orphaned controller process holds port",
		"fix_summary":   "added ExecStartPre pkill guard to unit file",
		"known_bad_fix": "just rebooting the node",
		"changed_files": []interface{}{"packages/cluster_controller/cluster_controller.spec.yaml"},
	})
	if err != nil {
		t.Fatalf("learn_from_fix error: %v", err)
	}

	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}

	proposals, _ := m["proposals"].([]learnFromFixProposal)
	if len(proposals) == 0 {
		t.Fatal("expected at least one proposal")
	}
	for _, p := range proposals {
		if !p.RequiresHumanApproval {
			t.Errorf("proposal %q must have requires_human_approval=true", p.ProposalID)
		}
	}
}

// TestLearnFromFix_NoDirectYAMLMutation verifies that knowledge YAML files are not modified.
func TestLearnFromFix_NoDirectYAMLMutation(t *testing.T) {
	docsDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(docsDir, "proposals"), 0o755)

	// Write baseline knowledge files.
	fmPath := filepath.Join(docsDir, "failure_modes.yaml")
	ffPath := filepath.Join(docsDir, "forbidden_fixes.yaml")
	invPath := filepath.Join(docsDir, "invariants.yaml")
	baseline := "failure_modes: []\n"
	_ = os.WriteFile(fmPath, []byte(baseline), 0o644)
	_ = os.WriteFile(ffPath, []byte("forbidden_fixes: []\n"), 0o644)
	_ = os.WriteFile(invPath, []byte("invariants: []\n"), 0o644)

	s := NewWithGraph(Config{DocsDir: docsDir}, nil)
	defer s.Close()

	_, err := s.CallTool(context.Background(), "awareness.learn_from_fix", map[string]interface{}{
		"symptom_text":  "etcd disk full causes workflow timeout",
		"root_cause":    "no etcd compaction scheduled",
		"fix_summary":   "added compaction job",
		"known_bad_fix": "manually deleting etcd keys",
	})
	if err != nil {
		t.Fatalf("learn_from_fix error: %v", err)
	}

	// Knowledge YAML files must remain unchanged.
	got, _ := os.ReadFile(fmPath)
	if string(got) != baseline {
		t.Errorf("failure_modes.yaml was modified directly (must go through proposals/)")
	}
	gotFF, _ := os.ReadFile(ffPath)
	if string(gotFF) != "forbidden_fixes: []\n" {
		t.Errorf("forbidden_fixes.yaml was modified directly")
	}
}

// TestLearnFromFix_ProposalIsLoadable verifies the saved proposal can be loaded by learning.LoadProposalFromFile.
func TestLearnFromFix_ProposalIsLoadable(t *testing.T) {
	s, docsDir := setupLearnFromFixServer(t)

	result, err := s.CallTool(context.Background(), "awareness.learn_from_fix", map[string]interface{}{
		"incident_id":  "INC-test-001",
		"symptom_text": "scylla connection refused at startup",
		"root_cause":   "service started before scylladb was ready",
		"fix_summary":  "added health check wait loop with deadline",
		"tests_added":  []interface{}{"TestScyllaReadyBeforeService"},
	})
	if err != nil {
		t.Fatalf("learn_from_fix error: %v", err)
	}
	m, _ := result.(map[string]interface{})
	proposalPath, _ := m["proposal_path"].(string)
	if proposalPath == "" {
		t.Fatal("no proposal_path returned")
	}

	spec, err := learning.LoadProposalFromFile(proposalPath)
	if err != nil {
		t.Fatalf("LoadProposalFromFile(%q): %v", proposalPath, err)
	}
	if spec.Proposal.Status != learning.StatusDraft {
		t.Errorf("expected DRAFT, got %q", spec.Proposal.Status)
	}
	if spec.LearnSource != "learn_from_fix" {
		t.Errorf("expected learn_source=learn_from_fix, got %q", spec.LearnSource)
	}
	if len(spec.FailureModes) == 0 {
		t.Error("expected at least one failure mode in proposal")
	}

	_ = docsDir // used implicitly via proposalPath
}

func containsSubstr(s, sub string) bool {
	return len(s) > 0 && len(sub) > 0 && (s == sub || len(s) >= len(sub) && stringContains(s, sub))
}

func stringContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
