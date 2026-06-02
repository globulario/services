package main

import (
	"context"
	"strings"
	"testing"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TestExecuteRemediation_HardBlocksETCDPut_Always enforces, at the handler
// boundary, that ETCD_PUT can NEVER be auto-executed regardless of the rule
// author's risk tag, regardless of whether evidence is present, and
// regardless of whether the caller supplies an approval token. The
// projection-clauses.md Clause 8 invariant is implemented by executor.go's
// hardBlocked(); this test pins the contract at the ExecuteRemediation RPC
// so a future refactor cannot accidentally route around the executor
// without also failing here.
//
// Test matrix walks every (risk × approval_token) cell and asserts the
// response is REJECTED with the canonical hardBlocked reason. The
// underlying ActionExecutor uses a fake dialer so that a regression which
// inadvertently passed the hard-block layer would surface here as a
// successful Executed=true — making the failure obvious.
func TestExecuteRemediation_HardBlocksETCDPut_Always(t *testing.T) {
	withStubbedGatePersistence(t)

	cases := []struct {
		name  string
		risk  cluster_doctorpb.ActionRisk
		token bool // include a syntactically-present approval token
	}{
		{"risk_low_no_token", cluster_doctorpb.ActionRisk_RISK_LOW, false},
		{"risk_low_with_token", cluster_doctorpb.ActionRisk_RISK_LOW, true},
		{"risk_medium_no_token", cluster_doctorpb.ActionRisk_RISK_MEDIUM, false},
		{"risk_high_no_token", cluster_doctorpb.ActionRisk_RISK_HIGH, false},
		{"risk_high_with_token", cluster_doctorpb.ActionRisk_RISK_HIGH, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findingID := "finding-hardblock-" + tc.name
			action := &cluster_doctorpb.RemediationAction{
				ActionType: cluster_doctorpb.ActionType_ETCD_PUT,
				Risk:       tc.risk,
				Params: map[string]string{
					"key":   "/globular/resources/ServiceRelease/event",
					"value": `{"status":{"phase":"AVAILABLE"}}`,
				},
			}
			// Clear any leftover gate/cooldown state for this key.
			gateKey := remediationGateKey(findingID, 0, action.GetActionType())
			autoRemediationCooldownByTarget.Delete(gateKey)
			autoRemediationGateByTarget.Delete(gateKey)

			srv := &ClusterDoctorServer{
				executor: &ActionExecutor{nodeAgentDialer: &fakeNodeAgentDialer{}},
				lastFindings: []rules.Finding{{
					FindingID:   findingID,
					InvariantID: "release.stuck_resolved",
					Summary:     "ServiceRelease stuck at RESOLVED",
					Evidence: []*cluster_doctorpb.Evidence{{
						SourceService: "cluster_controller",
						SourceRpc:     "ListReleases",
						KeyValues:     map[string]string{"release_name": "core@globular.io/event"},
						Timestamp:     timestamppb.Now(),
					}},
					Remediation: []*cluster_doctorpb.RemediationStep{{
						Order:  1,
						Action: action,
					}},
				}},
			}
			srv.isAuthoritative.Store(true)

			req := &cluster_doctorpb.ExecuteRemediationRequest{
				FindingId: findingID,
				StepIndex: 0,
			}
			if tc.token {
				// A non-empty token MUST NOT change the hard-block verdict.
				// The handler must reject ETCD_PUT before token validation
				// or any approval gate runs.
				req.ApprovalToken = "non-empty-token-should-not-matter"
			}

			resp, err := srv.ExecuteRemediation(context.Background(), req)
			if err != nil {
				t.Fatalf("ExecuteRemediation returned err=%v (expected non-error rejection)", err)
			}
			if resp.GetExecuted() {
				t.Fatalf("ETCD_PUT was executed — hard-block bypassed! status=%q reason=%q",
					resp.GetStatus(), resp.GetReason())
			}
			if resp.GetStatus() != "rejected" {
				t.Fatalf("status = %q, want %q (reason: %q)",
					resp.GetStatus(), "rejected", resp.GetReason())
			}
			if !strings.Contains(resp.GetReason(), "ETCD_PUT") {
				t.Fatalf("reason should cite ETCD_PUT hard-block, got %q", resp.GetReason())
			}
			if resp.GetAuditId() == "" {
				t.Fatalf("rejection must still produce an audit_id; got empty")
			}
		})
	}
}
