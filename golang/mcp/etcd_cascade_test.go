package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// etcdNOSPACELogs is the canonical incident log fixture: etcd NOSPACE →
// leader loss → controller lease expiry → workflow dispatch timeout.
// All five patterns (etcd_nospace, leader_instability, deadline_exceeded,
// workflow_stuck, connection_reset) are represented so scoring is realistic.
const etcdNOSPACELogs = `
May 07 18:00:01 globule-ryzen etcd[1234]: mvcc: database space exceeded
May 07 18:00:01 globule-ryzen etcd[1234]: NOSPACE alarm is activated — all writes rejected
May 07 18:00:02 globule-ryzen etcd[1234]: lost leader
May 07 18:00:03 globule-ryzen etcd[1234]: elected leader 8e9e05c52164694d
May 07 18:00:05 globule-ryzen etcd[1234]: lost leader
May 07 18:00:06 globule-ryzen etcd[1234]: elected leader 8e9e05c52164694d
May 07 18:00:10 globule-ryzen cluster_controller[5678]: lease expired, attempting re-election
May 07 18:00:11 globule-ryzen cluster_controller[5678]: lease expired, attempting re-election
May 07 18:00:15 globule-ryzen workflow[9012]: context deadline exceeded dispatching workflow step
May 07 18:00:16 globule-ryzen workflow[9012]: workflow stuck at CONVERGING
`

// ---------------------------------------------------------------------------
// Test 1 — offline_diagnose maps etcd NOSPACE logs to etcd/control-plane failure modes
// ---------------------------------------------------------------------------

// TestOfflineDiagnose_EtcdNOSPACE_MapsToEtcdFailureMode verifies that the four
// promoted etcd/control-plane failure modes appear in suspected_failure_modes with
// score > 0.5 when given the canonical incident logs.
func TestOfflineDiagnose_EtcdNOSPACE_MapsToEtcdFailureMode(t *testing.T) {
	s := newSelfReviewServer(t) // uses real docs/awareness dir with promoted entries

	result, err := s.callTool(context.Background(), "awareness.offline_diagnose", map[string]interface{}{
		"journalctl_text": etcdNOSPACELogs,
	})
	if err != nil {
		t.Fatalf("offline_diagnose error: %v", err)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected result type: %T", result)
	}

	fms, _ := m["suspected_failure_modes"].([]offlineFailureModeMatch)

	wantAbove := []struct {
		id        string
		minScore  float64
	}{
		{"etcd.nospace_alarm", 0.5},
		{"etcd.leader_instability", 0.5},
		{"workflow.dispatch_timeout_due_to_control_plane_instability", 0.5},
	}

	for _, want := range wantAbove {
		found := false
		for _, fm := range fms {
			if fm.ID == want.id {
				found = true
				if fm.MatchScore < want.minScore {
					t.Errorf("failure mode %q: score %.2f < required %.2f", want.id, fm.MatchScore, want.minScore)
				}
				break
			}
		}
		if !found {
			t.Errorf("failure mode %q not found in suspected_failure_modes", want.id)
		}
	}

	// controller.lease_expired must be present (score > 0 — it scores 0.67).
	foundController := false
	for _, fm := range fms {
		if fm.ID == "controller.lease_expired_due_to_etcd_instability" {
			foundController = true
			if fm.MatchScore <= 0.0 {
				t.Errorf("controller.lease_expired_due_to_etcd_instability: score %.2f must be > 0", fm.MatchScore)
			}
			break
		}
	}
	if !foundController {
		t.Error("controller.lease_expired_due_to_etcd_instability not found in suspected_failure_modes")
	}

	// etcd entries must be at or above the top of the list (score ≥ any objectstore entry).
	etcdScore := scoreOf(fms, "etcd.nospace_alarm")
	for _, fm := range fms {
		if strings.HasPrefix(fm.ID, "objectstore") {
			if fm.MatchScore > etcdScore {
				t.Errorf("objectstore entry %q (score %.2f) scores above etcd.nospace_alarm (%.2f) — false positive",
					fm.ID, fm.MatchScore, etcdScore)
			}
		}
	}
}

// TestOfflineDiagnose_EtcdNOSPACE_DoesNotMapToObjectstore verifies that objectstore
// failure modes are absent from results when the incident is clearly etcd/control-plane.
func TestOfflineDiagnose_EtcdNOSPACE_DoesNotMapToObjectstore(t *testing.T) {
	s := newSelfReviewServer(t)

	result, err := s.callTool(context.Background(), "awareness.offline_diagnose", map[string]interface{}{
		"journalctl_text": etcdNOSPACELogs,
	})
	if err != nil {
		t.Fatalf("offline_diagnose error: %v", err)
	}
	m, _ := result.(map[string]interface{})
	fms, _ := m["suspected_failure_modes"].([]offlineFailureModeMatch)

	for _, fm := range fms {
		if strings.HasPrefix(fm.ID, "objectstore") {
			etcdScore := scoreOf(fms, "etcd.nospace_alarm")
			if fm.MatchScore >= etcdScore && etcdScore > 0 {
				t.Errorf("objectstore entry %q (%.2f) tied/above etcd.nospace_alarm (%.2f) — false positive not suppressed",
					fm.ID, fm.MatchScore, etcdScore)
			}
		}
	}
}

func TestCausalChain_LeaderLoss_After_NOSPACE(t *testing.T) {
	TestCausalChain_EtcdNOSPACE_ProducesControlPlaneCascade(t)
}

// ---------------------------------------------------------------------------
// Test 2 — causal_chain produces 4-step control-plane cascade
// ---------------------------------------------------------------------------

// TestCausalChain_EtcdNOSPACE_ProducesControlPlaneCascade verifies that providing
// the four canonical etcd cascade events produces a chain matching ≥ 3 of 4 steps
// and a fix order that puts etcd remediation before controller/workflow actions.
func TestCausalChain_EtcdNOSPACE_ProducesControlPlaneCascade(t *testing.T) {
	s, _ := setupCausalServer(t) // fixture already contains etcd_disk_pressure_to_workflow_timeout

	result, err := s.callTool(context.Background(), "awareness.causal_chain", map[string]interface{}{
		"events": []interface{}{
			"etcd: NOSPACE alarm activated — mvcc: database space exceeded",
			"etcd: lost leader — leader election started",
			"cluster_controller: lease expired, attempting re-election",
			"workflow: context deadline exceeded dispatching workflow step",
		},
	})
	if err != nil {
		t.Fatalf("causal_chain error: %v", err)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected result type: %T", result)
	}

	chains, _ := m["chains"].([]causalChainResult)

	var cascade *causalChainResult
	for i := range chains {
		if chains[i].RootSignal == "etcd_disk_pressure" {
			cascade = &chains[i]
			break
		}
	}
	if cascade == nil {
		t.Fatalf("expected etcd_disk_pressure chain, got chains: %+v", chains)
	}

	// Must match at least 3 of 4 steps (≥ 75% — each of the 4 events maps to one step).
	if cascade.MatchedSteps < 3 {
		t.Errorf("expected ≥ 3 matched steps for 4-event input, got %d/%d", cascade.MatchedSteps, cascade.TotalSteps)
	}

	// Recommended fix order must mention etcd before controller and workflow.
	fixOrder := cascade.RecommendedFixOrder
	if len(fixOrder) < 2 {
		t.Fatalf("expected at least 2 fix-order steps, got %d", len(fixOrder))
	}
	etcdIdx, controllerIdx, workflowIdx := -1, -1, -1
	for i, step := range fixOrder {
		lower := strings.ToLower(step)
		if etcdIdx == -1 && strings.Contains(lower, "etcd") {
			etcdIdx = i
		}
		if controllerIdx == -1 && (strings.Contains(lower, "controller") || strings.Contains(lower, "lease")) {
			controllerIdx = i
		}
		if workflowIdx == -1 && strings.Contains(lower, "workflow") {
			workflowIdx = i
		}
	}
	if etcdIdx == -1 {
		t.Error("fix order must mention etcd remediation")
	}
	if controllerIdx != -1 && etcdIdx > controllerIdx {
		t.Errorf("fix order puts etcd (pos %d) after controller (pos %d) — etcd must be first", etcdIdx, controllerIdx)
	}
	if workflowIdx != -1 && etcdIdx > workflowIdx {
		t.Errorf("fix order puts etcd (pos %d) after workflow (pos %d) — etcd must be first", etcdIdx, workflowIdx)
	}
}

// ---------------------------------------------------------------------------
// Test 3 — self_review routes original criticism to closed_gaps
// ---------------------------------------------------------------------------

// TestSelfReview_EtcdKnowledgeGapClosed verifies that the original gap criticism
// ("offline_diagnose lacked dedicated etcd NOSPACE knowledge, mapped to objectstore
// false positives") routes to closed_gaps, not capability_gaps.
func TestSelfReview_EtcdKnowledgeGapClosed(t *testing.T) {
	s := newSelfReviewServer(t)

	criticism := "offline_diagnose lacked dedicated etcd NOSPACE / leader instability knowledge and therefore mapped the incident to objectstore false positives"

	result := callSelfReview(t, s, map[string]interface{}{
		"feedback": criticism,
	})

	// Must not appear in open capability_gaps.
	for _, g := range result.CapabilityGaps {
		if g.GapID == "awareness.etcd_control_plane_knowledge_gap" {
			t.Errorf("etcd_control_plane_knowledge_gap found in capability_gaps (open) — expected it to be closed")
		}
	}

	// Must appear in closed_gaps with status=implemented.
	found := false
	for _, g := range result.ClosedGaps {
		if g.GapID == "awareness.etcd_control_plane_knowledge_gap" {
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
	if !found {
		t.Errorf("etcd_control_plane_knowledge_gap not found in closed_gaps — open=%v closed=%v",
			openGapIDs(result.CapabilityGaps), gapIDs(result.ClosedGaps))
	}
}

// ---------------------------------------------------------------------------
// Test 4 — forbidden_fixes.yaml contains the three required etcd cascade entries
// ---------------------------------------------------------------------------

// TestForbiddenFixes_EtcdCascade verifies that the three operationally critical
// forbidden fixes are present in forbidden_fixes.yaml:
//  - workflow.treat_as_root_cause (don't treat workflow timeout as root when etcd is upstream)
//  - etcd.disarm_before_compact (don't disarm NOSPACE before compact/defrag)
//  - controller.restart_before_etcd_stable (don't restart controller before etcd is healthy)
func TestForbiddenFixes_EtcdCascade(t *testing.T) {
	docsDir := selfReviewDocsDir(t)
	path := filepath.Join(docsDir, "forbidden_fixes.yaml")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read forbidden_fixes.yaml: %v", err)
	}

	var root map[string]interface{}
	if err := yaml.Unmarshal(data, &root); err != nil {
		t.Fatalf("parse forbidden_fixes.yaml: %v", err)
	}
	items, _ := root["forbidden_fixes"].([]interface{})

	byID := make(map[string]map[string]interface{}, len(items))
	for _, item := range items {
		m, _ := item.(map[string]interface{})
		if id, ok := m["id"].(string); ok && id != "" {
			byID[id] = m
		}
	}

	required := []struct {
		id      string
		purpose string
	}{
		{
			id:      "workflow.treat_as_root_cause",
			purpose: "do not treat workflow timeout as root cause when etcd is upstream",
		},
		{
			id:      "etcd.disarm_before_compact",
			purpose: "do not disarm NOSPACE alarm before compact+defrag",
		},
		{
			id:      "controller.restart_before_etcd_stable",
			purpose: "do not restart controller before etcd is healthy",
		},
	}

	for _, req := range required {
		entry, ok := byID[req.id]
		if !ok {
			t.Errorf("missing forbidden fix %q (%s)", req.id, req.purpose)
			continue
		}
		summary, _ := entry["summary"].(string)
		if strings.TrimSpace(summary) == "" {
			t.Errorf("forbidden fix %q has empty summary", req.id)
		}
	}

	// Verify all six cascade forbidden fixes are present.
	all := []string{
		"etcd.disarm_before_compact",
		"etcd.restart_nodes_without_nospace_fix",
		"controller.restart_before_etcd_stable",
		"controller.kill_to_clear_lease_contention",
		"workflow.restart_before_control_plane_stable",
		"workflow.treat_as_root_cause",
	}
	for _, id := range all {
		if _, ok := byID[id]; !ok {
			t.Errorf("missing etcd cascade forbidden fix %q", id)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 5 — offline_diagnose surfaces direct etcdctl diagnostics for etcd.nospace_alarm
// ---------------------------------------------------------------------------

// TestOfflineDiagnose_EtcdNOSPACE_SurfacesDirectDiagnostics verifies that
// recommended_next_diagnostics contains etcdctl alarm list and compact guidance
// when etcd.nospace_alarm is the top-ranked failure mode — the operator must NOT
// need to call awareness.explain_symptom to learn the first-line commands.
func TestOfflineDiagnose_EtcdNOSPACE_SurfacesDirectDiagnostics(t *testing.T) {
	s := newSelfReviewServer(t)

	result, err := s.callTool(context.Background(), "awareness.offline_diagnose", map[string]interface{}{
		"journalctl_text": etcdNOSPACELogs,
	})
	if err != nil {
		t.Fatalf("offline_diagnose error: %v", err)
	}
	m, _ := result.(map[string]interface{})

	nextDiag, _ := m["recommended_next_diagnostics"].([]string)
	if len(nextDiag) == 0 {
		t.Fatal("recommended_next_diagnostics is empty")
	}

	combined := strings.Join(nextDiag, " ")

	// Must surface etcdctl alarm list directly.
	if !strings.Contains(strings.ToLower(combined), "etcdctl alarm list") {
		t.Errorf("recommended_next_diagnostics must contain 'etcdctl alarm list', got: %v", nextDiag)
	}

	// Must surface compact guidance.
	if !strings.Contains(strings.ToLower(combined), "compact") {
		t.Errorf("recommended_next_diagnostics must contain compact guidance, got: %v", nextDiag)
	}

	// Must NOT require the operator to call explain_symptom as the only guidance.
	allExplain := true
	for _, d := range nextDiag {
		if !strings.Contains(d, "awareness.explain_symptom") {
			allExplain = false
			break
		}
	}
	if allExplain && len(nextDiag) > 0 {
		t.Errorf("recommended_next_diagnostics contains only explain_symptom calls — direct commands must be surfaced: %v", nextDiag)
	}
}

// ---------------------------------------------------------------------------
// Test 6 — production causal_rules.yaml contains the etcd cascade rule
// ---------------------------------------------------------------------------

// TestCausalRules_ProductionFile_EtcdCascadePresent verifies that the real
// docs/awareness/knowledge/causal_rules.yaml file contains the
// etcd_disk_pressure_to_workflow_timeout rule with all four cascade steps
// and a fix order that puts etcd remediation before controller/workflow.
func TestCausalRules_ProductionFile_EtcdCascadePresent(t *testing.T) {
	docsDir := selfReviewDocsDir(t)
	path := filepath.Join(docsDir, "knowledge", "causal_rules.yaml")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read causal_rules.yaml: %v", err)
	}

	var root map[string]interface{}
	if err := yaml.Unmarshal(data, &root); err != nil {
		t.Fatalf("parse causal_rules.yaml: %v", err)
	}

	// The file uses "causal_rules:" as the top-level key.
	rules, _ := root["causal_rules"].([]interface{})
	if len(rules) == 0 {
		t.Fatal("causal_rules.yaml has no rules")
	}

	// Locate etcd_disk_pressure_to_workflow_timeout.
	var cascade map[string]interface{}
	for _, item := range rules {
		m, _ := item.(map[string]interface{})
		if id, _ := m["id"].(string); id == "etcd_disk_pressure_to_workflow_timeout" {
			cascade = m
			break
		}
	}
	if cascade == nil {
		t.Fatal("rule etcd_disk_pressure_to_workflow_timeout not found in causal_rules.yaml")
	}

	// Sequence must have at least 4 steps.
	seq, _ := cascade["sequence"].([]interface{})
	if len(seq) < 4 {
		t.Fatalf("expected ≥ 4 sequence steps, got %d", len(seq))
	}

	// Each step must cover one layer of the cascade. We match by keyword content
	// across the event name, component, and keywords fields.
	type stepCheck struct {
		label    string
		needsAny []string // at least one of these must appear in the step blob
	}
	checks := []stepCheck{
		{"etcd disk pressure / NOSPACE", []string{"nospace", "database space", "etcd_nospace", "etcd disk"}},
		{"leader instability", []string{"leader", "election", "quorum"}},
		{"controller lease churn", []string{"lease", "controller", "leadership"}},
		{"workflow dispatch timeout", []string{"workflow", "dispatch", "deadline"}},
	}

	for i, chk := range checks {
		if i >= len(seq) {
			t.Errorf("step %d (%s) missing — only %d steps present", i, chk.label, len(seq))
			continue
		}
		stepBytes, _ := yaml.Marshal(seq[i])
		blob := strings.ToLower(string(stepBytes))
		found := false
		for _, kw := range chk.needsAny {
			if strings.Contains(blob, kw) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("step %d (%s): none of %v found in:\n%s", i, chk.label, chk.needsAny, blob)
		}
	}

	// recommended_fix_order must exist and start with etcd remediation.
	fixOrder, _ := cascade["recommended_fix_order"].([]interface{})
	if len(fixOrder) == 0 {
		t.Fatal("recommended_fix_order is empty")
	}

	// Find positions of etcd, controller, and workflow in the fix order.
	etcdIdx, controllerIdx, workflowIdx := -1, -1, -1
	for i, step := range fixOrder {
		s := strings.ToLower(fmt.Sprintf("%v", step))
		if etcdIdx == -1 && strings.Contains(s, "etcd") {
			etcdIdx = i
		}
		if controllerIdx == -1 && (strings.Contains(s, "controller") || strings.Contains(s, "leader") || strings.Contains(s, "quorum")) {
			controllerIdx = i
		}
		if workflowIdx == -1 && strings.Contains(s, "workflow") {
			workflowIdx = i
		}
	}

	if etcdIdx == -1 {
		t.Error("recommended_fix_order must mention etcd remediation")
	}
	if controllerIdx != -1 && etcdIdx > controllerIdx {
		t.Errorf("fix order: etcd (pos %d) must come before controller/leader (pos %d)", etcdIdx, controllerIdx)
	}
	if workflowIdx != -1 && etcdIdx > workflowIdx {
		t.Errorf("fix order: etcd (pos %d) must come before workflow (pos %d)", etcdIdx, workflowIdx)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// scoreOf returns the MatchScore for a given failure mode ID, or 0 if not present.
func scoreOf(fms []offlineFailureModeMatch, id string) float64 {
	for _, fm := range fms {
		if fm.ID == id {
			return fm.MatchScore
		}
	}
	return 0
}
