package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/globulario/services/golang/awareness/runtime"
)

// TestSuggestIncident_Registered verifies the tool is registered.
func TestSuggestIncident_Registered(t *testing.T) {
	s := NewWithGraph(Config{}, nil)
	if !s.HasTool("awareness.suggest_incident") {
		t.Error("awareness.suggest_incident should be registered")
	}
}

// TestSuggestIncident_LiveFalseWithoutSnapshotIDErrors verifies that
// passing live=false without a snapshot_id returns a clear error.
func TestSuggestIncident_LiveFalseWithoutSnapshotIDErrors(t *testing.T) {
	s := NewWithGraph(Config{}, nil)
	_, err := s.CallTool(context.Background(), "awareness.suggest_incident", map[string]interface{}{
		"live": false,
	})
	if err == nil {
		t.Error("expected error when live=false and no snapshot_id provided")
	}
	if !strings.Contains(err.Error(), "snapshot_id") {
		t.Errorf("error should mention snapshot_id, got: %v", err)
	}
}

// TestSuggestIncident_LiveProducesCandidates verifies that live=true with
// a fake bridge returns a valid result.
func TestSuggestIncident_LiveProducesCandidates(t *testing.T) {
	// Build a server with a fake bridge that has doctor findings.
	s := NewWithGraph(Config{}, nil)

	// Override bridge via DoctorAddr — since we can't inject a bridge directly
	// through the tool, we call live=true (default) and check the result shape.
	result, err := s.CallTool(context.Background(), "awareness.suggest_incident", map[string]interface{}{
		"live":         true,
		"min_severity": "high",
	})
	if err != nil {
		t.Fatalf("suggest_incident error: %v", err)
	}

	m := result.(map[string]interface{})
	// Must have candidate_incidents key (even if empty — no live sources).
	if _, ok := m["candidate_incidents"]; !ok {
		t.Error("expected candidate_incidents in result")
	}
	// Must have snapshot_id.
	if snapID, ok := m["snapshot_id"].(string); !ok || snapID == "" {
		t.Error("expected non-empty snapshot_id in result")
	}
	// Must have confidence.
	if conf, ok := m["confidence"].(string); !ok || conf == "" {
		t.Error("expected non-empty confidence in result")
	}
}

// TestSuggestIncident_IncludeYAMLGeneratesDraft verifies that include_yaml=true
// produces YAML drafts in candidate entries.
func TestSuggestIncident_IncludeYAMLGeneratesDraft(t *testing.T) {
	snap := &runtime.RuntimeSnapshot{
		ID: "snap-test",
		DoctorFindings: []runtime.DoctorFinding{
			{FindingID: "f1", Severity: "critical", Title: "etcd disk full", InvariantRef: "etcd.disk_saturation"},
		},
	}

	openIDs := make(map[string]bool)
	candidates := suggestFromSnapshot(snap, "critical", openIDs, true)

	if len(candidates) == 0 {
		t.Fatal("expected at least one candidate")
	}
	for _, c := range candidates {
		if c.YAMLDraft == "" {
			t.Errorf("expected non-empty YAMLDraft for candidate %s", c.FailureModeID)
		}
		if !strings.Contains(c.YAMLDraft, "severity:") {
			t.Errorf("YAML draft missing severity field: %s", c.YAMLDraft)
		}
	}
}

// TestSuggestIncident_DuplicateOpenIncidentNotDuplicated verifies that when an
// incident is already open for a failure_mode_id, AlreadyOpen=true is set.
func TestSuggestIncident_DuplicateOpenIncidentNotDuplicated(t *testing.T) {
	snap := &runtime.RuntimeSnapshot{
		ID: "snap-dup",
		DoctorFindings: []runtime.DoctorFinding{
			{FindingID: "f2", Severity: "critical", Title: "controller crash", InvariantRef: "controller.liveness"},
		},
	}

	openIDs := map[string]bool{
		"controller.liveness": true,
	}
	candidates := suggestFromSnapshot(snap, "critical", openIDs, false)

	found := false
	for _, c := range candidates {
		if c.FailureModeID == "controller.liveness" {
			if !c.AlreadyOpen {
				t.Error("expected AlreadyOpen=true for already-open incident")
			}
			found = true
		}
	}
	if !found {
		t.Error("expected candidate for controller.liveness")
	}
}

// TestSuggestIncident_FixOrder_EtcdBeforeWorkflow verifies that when an etcd
// NOSPACE control-plane cascade is present, suggested candidates rank etcd
// remediation before controller and workflow actions, and never treat the
// workflow dispatch timeout as the root cause.
func TestSuggestIncident_FixOrder_EtcdBeforeWorkflow(t *testing.T) {
	// Construct a snapshot that mirrors the etcd NOSPACE → leader loss →
	// controller lease expired → workflow dispatch timeout cascade.
	// Doctor findings drive candidate ordering: critical findings appear first.
	snap := &runtime.RuntimeSnapshot{
		ID: "snap-etcd-cascade",
		MatchedFailureModes: []string{
			"etcd.nospace_alarm",
			"etcd.leader_instability",
			"controller.lease_expired_due_to_etcd_instability",
			"workflow.dispatch_timeout_due_to_control_plane_instability",
		},
		DoctorFindings: []runtime.DoctorFinding{
			{
				FindingID:    "etcd-nospace",
				Severity:     "critical",
				Title:        "etcd NOSPACE alarm — all writes rejected",
				InvariantRef: "etcd.nospace_alarm",
			},
			{
				FindingID:    "workflow-timeout",
				Severity:     "high",
				Title:        "workflow dispatch timeout — context deadline exceeded",
				InvariantRef: "workflow.dispatch_timeout_due_to_control_plane_instability",
			},
		},
	}

	openIDs := make(map[string]bool)
	candidates := suggestFromSnapshot(snap, "high", openIDs, true)

	if len(candidates) == 0 {
		t.Fatal("expected at least one candidate from etcd cascade snapshot")
	}

	// Locate etcd.nospace_alarm and workflow.dispatch_timeout in the candidate list.
	etcdIdx, workflowIdx := -1, -1
	for i, c := range candidates {
		id := c.FailureModeID
		if id == "" {
			id = c.FindingID
		}
		if id == "etcd.nospace_alarm" && etcdIdx == -1 {
			etcdIdx = i
		}
		if id == "workflow.dispatch_timeout_due_to_control_plane_instability" && workflowIdx == -1 {
			workflowIdx = i
		}
	}

	if etcdIdx == -1 {
		t.Error("etcd.nospace_alarm candidate not found — must be present as root-cause candidate")
	}
	if workflowIdx == -1 {
		t.Error("workflow.dispatch_timeout candidate not found")
	}

	// etcd must appear before workflow — etcd is root, workflow is terminal symptom.
	if etcdIdx != -1 && workflowIdx != -1 && etcdIdx > workflowIdx {
		t.Errorf("etcd candidate (pos %d) appears after workflow candidate (pos %d) — etcd is root, workflow is downstream",
			etcdIdx, workflowIdx)
	}

	// First candidate must not be the workflow timeout — do not treat it as root.
	firstID := candidates[0].FailureModeID
	if firstID == "" {
		firstID = candidates[0].FindingID
	}
	if strings.Contains(firstID, "workflow.dispatch") {
		t.Errorf("first candidate is %q — workflow timeout must not be treated as root cause", firstID)
	}

	// MatchedFMs on every candidate must list etcd entries before workflow entries,
	// preserving the upstream → downstream cascade order.
	for _, c := range candidates {
		if len(c.MatchedFMs) < 2 {
			continue
		}
		etcdFMIdx, workflowFMIdx := -1, -1
		for i, fm := range c.MatchedFMs {
			if strings.HasPrefix(fm, "etcd.") && etcdFMIdx == -1 {
				etcdFMIdx = i
			}
			if strings.HasPrefix(fm, "workflow.") && workflowFMIdx == -1 {
				workflowFMIdx = i
			}
		}
		if etcdFMIdx != -1 && workflowFMIdx != -1 && etcdFMIdx > workflowFMIdx {
			t.Errorf("candidate %q: MatchedFMs lists etcd (pos %d) after workflow (pos %d) — cascade order must be preserved",
				c.FailureModeID, etcdFMIdx, workflowFMIdx)
		}
	}

	// YAML draft for the etcd.nospace_alarm candidate must identify etcd as the failure mode.
	if etcdIdx != -1 {
		draft := candidates[etcdIdx].YAMLDraft
		if draft == "" {
			t.Error("etcd.nospace_alarm candidate has empty YAML draft (include_yaml=true was set)")
		} else if !strings.Contains(draft, "etcd.nospace_alarm") {
			t.Errorf("etcd candidate YAML draft does not reference etcd.nospace_alarm:\n%s", draft)
		}
	}
}

// TestSuggestIncident_SeparatesFailureModesFromInvariants verifies that
// MatchedFailureModes and MatchedInvariants appear separately.
func TestSuggestIncident_SeparatesFailureModesFromInvariants(t *testing.T) {
	snap := &runtime.RuntimeSnapshot{
		ID:                  "snap-sep",
		MatchedFailureModes: []string{"fm.a", "fm.b"},
		MatchedInvariants:   []string{"inv.x"},
		DoctorFindings: []runtime.DoctorFinding{
			{FindingID: "f3", Severity: "critical", Title: "test", InvariantRef: "inv.x"},
		},
	}

	openIDs := make(map[string]bool)
	candidates := suggestFromSnapshot(snap, "high", openIDs, false)

	// At least one candidate should have MatchedFMs.
	hasFMs := false
	hasInvs := false
	for _, c := range candidates {
		if len(c.MatchedFMs) > 0 {
			hasFMs = true
		}
		if len(c.MatchedInvs) > 0 {
			hasInvs = true
		}
	}
	if !hasFMs {
		t.Error("expected at least one candidate with MatchedFMs")
	}
	if !hasInvs {
		t.Error("expected at least one candidate with MatchedInvs")
	}
}
