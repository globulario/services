package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/evidence"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TestFindingIncludesEvidenceTrustLevel — contract test for
// evidence.provenance_trust_levels. Findings expose a trust level computed
// from each evidence entry's source/writer/timestamp. The mapping must be
// stable: a fresh controller-snapshot evidence classifies AUTHORITATIVE,
// a 20-minute-old one classifies STALE.
func TestFindingIncludesEvidenceTrustLevel(t *testing.T) {
	now := time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC)

	fresh := rules.Finding{
		FindingID: "f-fresh",
		Evidence: []*cluster_doctorpb.Evidence{{
			SourceService: "cluster_controller",
			SourceRpc:     "GetClusterHealthV1",
			Timestamp:     timestamppb.New(now.Add(-30 * time.Second)),
		}},
	}
	if got := findingEvidenceTrust(fresh, now); got != evidence.TrustAuthoritative {
		t.Fatalf("fresh controller snapshot: got %s, want AUTHORITATIVE", got)
	}

	stale := rules.Finding{
		FindingID: "f-stale",
		Evidence: []*cluster_doctorpb.Evidence{{
			SourceService: "cluster_controller",
			SourceRpc:     "GetClusterHealthV1",
			// controller snapshot window = 2min, stale beyond 4min
			Timestamp: timestamppb.New(now.Add(-20 * time.Minute)),
		}},
	}
	if got := findingEvidenceTrust(stale, now); got != evidence.TrustStale {
		t.Fatalf("20min-old snapshot: got %s, want STALE", got)
	}

	// A finding with no evidence is untrusted — silence is not freshness.
	empty := rules.Finding{FindingID: "f-empty"}
	if got := findingEvidenceTrust(empty, now); got != evidence.TrustUntrusted {
		t.Fatalf("empty evidence: got %s, want UNTRUSTED", got)
	}

	// Worst across multiple entries wins.
	mixed := rules.Finding{
		FindingID: "f-mixed",
		Evidence: []*cluster_doctorpb.Evidence{
			{SourceService: "cluster_controller", Timestamp: timestamppb.New(now.Add(-30 * time.Second))},
			{SourceService: "cluster_controller", Timestamp: timestamppb.New(now.Add(-3 * time.Minute))}, // degraded
		},
	}
	if got := findingEvidenceTrust(mixed, now); got != evidence.TrustDegraded {
		t.Fatalf("mixed evidence: got %s, want DEGRADED (worst wins)", got)
	}
}

// TestRemediationBlocksOnStaleEvidence — contract test. The remediation
// handler must refuse to execute when the finding's evidence has degraded
// past the freshness windows defined in golang/evidence. Dry-run is
// exempt so operators can still inspect what a remediation would do.
func TestRemediationBlocksOnStaleEvidence(t *testing.T) {
	withStubbedGatePersistence(t)

	staleTimestamp := timestamppb.New(time.Now().Add(-30 * time.Minute))
	findingID := "finding-stale-evidence"
	action := &cluster_doctorpb.RemediationAction{
		ActionType: cluster_doctorpb.ActionType_SYSTEMCTL_RESTART,
		Risk:       cluster_doctorpb.ActionRisk_RISK_LOW,
		Params: map[string]string{
			"unit":    "globular-node-agent.service",
			"node_id": "node-1",
		},
	}
	srv := &ClusterDoctorServer{
		executor: &ActionExecutor{nodeAgentDialer: &fakeNodeAgentDialer{}},
		lastFindings: []rules.Finding{{
			FindingID:   findingID,
			InvariantID: "runtime.desired_enabled_not_alive",
			Summary:     "unit is not running",
			Evidence: []*cluster_doctorpb.Evidence{{
				SourceService: "cluster_controller",
				SourceRpc:     "GetClusterHealthV1",
				KeyValues:     map[string]string{"node": "node-1"},
				Timestamp:     staleTimestamp,
			}},
			Remediation: []*cluster_doctorpb.RemediationStep{{
				Order:  1,
				Action: action,
			}},
		}},
	}
	srv.isAuthoritative.Store(true)

	// Live execution must be rejected with a trust-aware reason.
	resp, err := srv.ExecuteRemediation(context.Background(), &cluster_doctorpb.ExecuteRemediationRequest{
		FindingId: findingID,
		StepIndex: 0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetStatus() != "rejected" {
		t.Fatalf("expected rejected, got status=%q reason=%q", resp.GetStatus(), resp.GetReason())
	}
	if !strings.Contains(resp.GetReason(), "evidence trust=") {
		t.Fatalf("rejection reason must surface evidence trust, got: %q", resp.GetReason())
	}

	// Dry-run with the same stale evidence is allowed through — operators
	// must still be able to inspect what would happen without proving
	// freshness first.
	dryResp, err := srv.ExecuteRemediation(context.Background(), &cluster_doctorpb.ExecuteRemediationRequest{
		FindingId: findingID,
		StepIndex: 0,
		DryRun:    true,
	})
	if err != nil {
		t.Fatalf("dry-run error: %v", err)
	}
	if dryResp.GetStatus() == "rejected" && strings.Contains(dryResp.GetReason(), "evidence trust") {
		t.Fatalf("dry-run must NOT be rejected by trust gate, got: %q", dryResp.GetReason())
	}
}
