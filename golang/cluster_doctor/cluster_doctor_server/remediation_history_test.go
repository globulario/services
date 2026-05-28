package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestSummarizeHistoricalSuccessfulActions_FiltersAndRanks(t *testing.T) {
	orig := listRemediationAuditsFn
	listRemediationAuditsFn = func(context.Context, int) ([]RemediationAudit, error) {
		return []RemediationAudit{
			{InvariantID: "inv-1", EvidenceDigest: "sha256:x", ActionType: "SYSTEMCTL_RESTART", Executed: true, Timestamp: 10},
			{InvariantID: "inv-1", EvidenceDigest: "sha256:x", ActionType: "SYSTEMCTL_RESTART", Executed: true, Timestamp: 12},
			{InvariantID: "inv-1", EvidenceDigest: "sha256:x", ActionType: "FILE_DELETE", Executed: true, Timestamp: 11},
			{InvariantID: "inv-1", EvidenceDigest: "sha256:x", ActionType: "FILE_DELETE", Executed: false, Timestamp: 14},
			{InvariantID: "inv-1", EvidenceDigest: "sha256:x", ActionType: "SYSTEMCTL_RESTART", Executed: false, Rejected: true, Timestamp: 15},
			{InvariantID: "inv-2", EvidenceDigest: "sha256:x", ActionType: "SYSTEMCTL_RESTART", Executed: true, Timestamp: 16},
		}, nil
	}
	defer func() { listRemediationAuditsFn = orig }()

	stats := summarizeHistoricalSuccessfulActions(context.Background(), "inv-1", "sha256:x", 200)
	if len(stats) != 2 {
		t.Fatalf("expected 2 stats, got %d", len(stats))
	}
	if stats[0].ActionType != "SYSTEMCTL_RESTART" || stats[0].Successes != 2 {
		t.Fatalf("unexpected top stat: %+v", stats[0])
	}
}

func TestExplainFinding_AppendsHistoricalHint(t *testing.T) {
	orig := listRemediationAuditsFn
	listRemediationAuditsFn = func(context.Context, int) ([]RemediationAudit, error) {
		return []RemediationAudit{{
			InvariantID:    "runtime.desired_enabled_not_alive",
			EvidenceDigest: "sha256:hist",
			ActionType:     "SYSTEMCTL_RESTART",
			Executed:       true,
			Timestamp:      time.Now().Unix(),
		}}, nil
	}
	defer func() { listRemediationAuditsFn = orig }()

	ev := []*cluster_doctorpb.Evidence{{
		SourceService: "cluster_controller",
		SourceRpc:     "GetClusterHealthV1",
		KeyValues:     map[string]string{"node": "n1"},
		Timestamp:     timestamppb.Now(),
	}}
	// force deterministic digest match with stubbed audits
	d := digestFindingEvidence(ev)
	listRemediationAuditsFn = func(context.Context, int) ([]RemediationAudit, error) {
		return []RemediationAudit{{
			InvariantID:    "runtime.desired_enabled_not_alive",
			EvidenceDigest: d,
			ActionType:     "SYSTEMCTL_RESTART",
			Executed:       true,
			Timestamp:      time.Now().Unix(),
		}}, nil
	}

	srv := &ClusterDoctorServer{lastFindings: []rules.Finding{{
		FindingID:   "f1",
		InvariantID: "runtime.desired_enabled_not_alive",
		Summary:     "unit stopped",
		Evidence:    ev,
	}}}

	resp, err := srv.ExplainFinding(context.Background(), &cluster_doctorpb.ExplainFindingRequest{FindingId: "f1"})
	if err != nil {
		t.Fatalf("ExplainFinding failed: %v", err)
	}
	if !strings.Contains(resp.GetWhyFailed(), "Historical successful actions") {
		t.Fatalf("expected historical hint in why_failed, got %q", resp.GetWhyFailed())
	}
	if len(resp.GetPlanDiff()) == 0 || resp.GetPlanDiff()[0] != "historical_success_actions_present" {
		t.Fatalf("expected plan_diff hint, got %v", resp.GetPlanDiff())
	}
}
