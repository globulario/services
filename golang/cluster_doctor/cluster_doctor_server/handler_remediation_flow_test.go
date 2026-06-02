package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/security/approvaltest"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// mintApprovalForFinding returns a token bound to the same tuple the
// remediation handler expects (action class + entity ref + evidence digest
// + finding id). Tests that exercise the approval path use this to satisfy
// the security.ValidateApprovalToken contract introduced with
// docs/intent/remediation.token_contract.yaml.
//
// Also installs an in-memory stub for the etcd-backed replay store so
// the handler's ValidateApprovalToken call doesn't try to reach a real
// etcd from the test process.
func mintApprovalForFinding(t *testing.T, f rules.Finding, action *cluster_doctorpb.RemediationAction) string {
	t.Helper()
	approvaltest.Install(t, "", "")
	withStubbedEtcdReplay(t)
	target := f.EntityRef
	if strings.TrimSpace(target) == "" {
		target = f.FindingID
	}
	tok, err := security.MintApprovalToken(security.MintApprovalRequest{
		Actor:       "test-operator",
		ActionClass: action.GetActionType().String(),
		Target:      target,
		Generation:  digestFindingEvidence(f.Evidence),
		FindingID:   f.FindingID,
	})
	if err != nil {
		t.Fatalf("mint approval token: %v", err)
	}
	return tok
}

type fakeNodeAgentDialer struct{}

func (f *fakeNodeAgentDialer) SystemctlAction(_ context.Context, nodeID, unit, verb string) (string, error) {
	return "ok:" + verb + ":" + unit + ":" + nodeID, nil
}

func (f *fakeNodeAgentDialer) FileDelete(_ context.Context, nodeID, path string) error {
	_ = nodeID
	_ = path
	return nil
}

func (f *fakeNodeAgentDialer) DeleteCacheArtifact(_ context.Context, nodeID, publisherID, packageName string) (string, error) {
	return fmt.Sprintf("fake-deleted: publisher=%s package=%s on %s", publisherID, packageName, nodeID), nil
}

func TestExecuteRemediation_EscalationClearsOnApprovalThenReturnsToCooldown(t *testing.T) {
	withStubbedGatePersistence(t)

	findingID := "finding-remediation-flow"
	action := &cluster_doctorpb.RemediationAction{
		ActionType: cluster_doctorpb.ActionType_SYSTEMCTL_RESTART,
		Risk:       cluster_doctorpb.ActionRisk_RISK_LOW,
		Params: map[string]string{
			"unit":    "globular-node-agent.service",
			"node_id": "node-1",
		},
	}
	key := remediationGateKey(findingID, 0, action.GetActionType())
	autoRemediationCooldownByTarget.Delete(key)
	autoRemediationGateByTarget.Delete(key)

	srv := &ClusterDoctorServer{
		executor: &ActionExecutor{nodeAgentDialer: &fakeNodeAgentDialer{}},
		lastFindings: []rules.Finding{{
			FindingID:   findingID,
			InvariantID: "runtime.desired_enabled_not_alive",
			Summary:     "unit is not running",
			Evidence: []*cluster_doctorpb.Evidence{{
				SourceService: "cluster_controller",
				SourceRpc:     "GetClusterHealthV1",
				KeyValues:     map[string]string{"node": "node-1", "unit": "globular-node-agent.service"},
				Timestamp:     timestamppb.Now(),
			}},
			Remediation: []*cluster_doctorpb.RemediationStep{{
				Order:  1,
				Action: action,
			}},
		}},
	}
	srv.isAuthoritative.Store(true)

	ctx := context.Background()

	first, err := srv.ExecuteRemediation(ctx, &cluster_doctorpb.ExecuteRemediationRequest{FindingId: findingID, StepIndex: 0})
	if err != nil {
		t.Fatalf("first execute failed: %v", err)
	}
	if first.GetStatus() != "executed" {
		t.Fatalf("first execute status=%q, want executed", first.GetStatus())
	}

	for i := 0; i < autoRemediationEscalationThreshold; i++ {
		resp, err := srv.ExecuteRemediation(ctx, &cluster_doctorpb.ExecuteRemediationRequest{FindingId: findingID, StepIndex: 0})
		if err != nil {
			t.Fatalf("cooldown attempt %d failed: %v", i+1, err)
		}
		if i < autoRemediationEscalationThreshold-1 && !strings.Contains(resp.GetReason(), "cooldown active") {
			t.Fatalf("attempt %d expected cooldown reason, got %q", i+1, resp.GetReason())
		}
		if i == autoRemediationEscalationThreshold-1 && !strings.Contains(resp.GetReason(), "operator approval required") {
			t.Fatalf("attempt %d expected escalation reason, got %q", i+1, resp.GetReason())
		}
	}

	approvalToken := mintApprovalForFinding(t, srv.lastFindings[0], action)
	approved, err := srv.ExecuteRemediation(ctx, &cluster_doctorpb.ExecuteRemediationRequest{
		FindingId:     findingID,
		StepIndex:     0,
		ApprovalToken: approvalToken,
	})
	if err != nil {
		t.Fatalf("approved execute failed: %v", err)
	}
	if approved.GetStatus() != "executed" {
		t.Fatalf("approved execute status=%q, want executed", approved.GetStatus())
	}
	if gate, ok := remediationGateGet(key); ok && gate.Escalated {
		t.Fatalf("gate should be cleared after approved execution, got %+v", gate)
	}

	postClear, err := srv.ExecuteRemediation(ctx, &cluster_doctorpb.ExecuteRemediationRequest{FindingId: findingID, StepIndex: 0})
	if err != nil {
		t.Fatalf("post-clear execute failed: %v", err)
	}
	if !strings.Contains(postClear.GetReason(), "cooldown active") {
		t.Fatalf("post-clear expected cooldown reason, got %q", postClear.GetReason())
	}
	if strings.Contains(postClear.GetReason(), "operator approval required") {
		t.Fatalf("post-clear should not be escalated immediately, got %q", postClear.GetReason())
	}
}

func TestExecuteRemediation_RejectsWhenFindingHasNoInvariantOrEvidence(t *testing.T) {
	withStubbedGatePersistence(t)
	findingID := "finding-missing-policy-context"
	srv := &ClusterDoctorServer{
		executor: &ActionExecutor{nodeAgentDialer: &fakeNodeAgentDialer{}},
		lastFindings: []rules.Finding{{
			FindingID: findingID,
			Remediation: []*cluster_doctorpb.RemediationStep{{
				Order: 1,
				Action: &cluster_doctorpb.RemediationAction{
					ActionType: cluster_doctorpb.ActionType_SYSTEMCTL_RESTART,
					Risk:       cluster_doctorpb.ActionRisk_RISK_LOW,
					Params:     map[string]string{"unit": "globular-node-agent.service", "node_id": "node-1"},
				},
			}},
		}},
	}
	srv.isAuthoritative.Store(true)

	resp, err := srv.ExecuteRemediation(context.Background(), &cluster_doctorpb.ExecuteRemediationRequest{FindingId: findingID, StepIndex: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetStatus() != "rejected" || !strings.Contains(resp.GetReason(), "policy guardrail") {
		t.Fatalf("expected policy guardrail rejection, got status=%q reason=%q", resp.GetStatus(), resp.GetReason())
	}
}

func TestExecuteRemediation_RequiresApprovalAfterRepeatedFailures(t *testing.T) {
	withStubbedGatePersistence(t)
	origAudits := listRemediationAuditsFn
	listRemediationAuditsFn = func(context.Context, int) ([]RemediationAudit, error) {
		now := time.Now().Unix()
		return []RemediationAudit{
			{InvariantID: "runtime.desired_enabled_not_alive", EvidenceDigest: "sha256:fail", ActionType: "SYSTEMCTL_RESTART", Executed: false, Reason: "failed", Timestamp: now - 30},
			{InvariantID: "runtime.desired_enabled_not_alive", EvidenceDigest: "sha256:fail", ActionType: "SYSTEMCTL_RESTART", Executed: false, Reason: "failed", Timestamp: now - 60},
			{InvariantID: "runtime.desired_enabled_not_alive", EvidenceDigest: "sha256:fail", ActionType: "SYSTEMCTL_RESTART", Executed: false, Reason: "failed", Timestamp: now - 90},
		}, nil
	}
	defer func() { listRemediationAuditsFn = origAudits }()

	ev := []*cluster_doctorpb.Evidence{{
		SourceService: "cluster_controller",
		SourceRpc:     "GetClusterHealthV1",
		KeyValues:     map[string]string{"node": "node-1", "unit": "globular-node-agent.service"},
		Timestamp:     timestamppb.Now(),
	}}
	digest := digestFindingEvidence(ev)
	// SYSTEMCTL_RESTART trips at threshold 5 under the cluster-wide policy
	// (see golang/remediation/policy.go DefaultFailureRatePolicy). Stub
	// five failed attempts — the next live call must escalate.
	listRemediationAuditsFn = func(context.Context, int) ([]RemediationAudit, error) {
		now := time.Now().Unix()
		return []RemediationAudit{
			{InvariantID: "runtime.desired_enabled_not_alive", EvidenceDigest: digest, ActionType: "SYSTEMCTL_RESTART", Executed: false, Reason: "failed", Timestamp: now - 30},
			{InvariantID: "runtime.desired_enabled_not_alive", EvidenceDigest: digest, ActionType: "SYSTEMCTL_RESTART", Executed: false, Reason: "failed", Timestamp: now - 60},
			{InvariantID: "runtime.desired_enabled_not_alive", EvidenceDigest: digest, ActionType: "SYSTEMCTL_RESTART", Executed: false, Reason: "failed", Timestamp: now - 90},
			{InvariantID: "runtime.desired_enabled_not_alive", EvidenceDigest: digest, ActionType: "SYSTEMCTL_RESTART", Executed: false, Reason: "failed", Timestamp: now - 120},
			{InvariantID: "runtime.desired_enabled_not_alive", EvidenceDigest: digest, ActionType: "SYSTEMCTL_RESTART", Executed: false, Reason: "failed", Timestamp: now - 150},
		}, nil
	}

	srv := &ClusterDoctorServer{
		executor: &ActionExecutor{nodeAgentDialer: &fakeNodeAgentDialer{}},
		lastFindings: []rules.Finding{{
			FindingID:   "finding-failure-escalation",
			InvariantID: "runtime.desired_enabled_not_alive",
			Summary:     "unit is not running",
			Evidence:    ev,
			Remediation: []*cluster_doctorpb.RemediationStep{{
				Order: 1,
				Action: &cluster_doctorpb.RemediationAction{
					ActionType: cluster_doctorpb.ActionType_SYSTEMCTL_RESTART,
					Risk:       cluster_doctorpb.ActionRisk_RISK_LOW,
					Params:     map[string]string{"unit": "globular-node-agent.service", "node_id": "node-1"},
				},
			}},
		}},
	}
	srv.isAuthoritative.Store(true)

	resp, err := srv.ExecuteRemediation(context.Background(), &cluster_doctorpb.ExecuteRemediationRequest{FindingId: "finding-failure-escalation", StepIndex: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetStatus() != "rejected" || !strings.Contains(resp.GetReason(), "failure-rate breaker open") {
		t.Fatalf("expected failure-rate breaker rejection, got status=%q reason=%q", resp.GetStatus(), resp.GetReason())
	}

	approvalTok := mintApprovalForFinding(t, srv.lastFindings[0], srv.lastFindings[0].Remediation[0].GetAction())
	approved, err := srv.ExecuteRemediation(context.Background(), &cluster_doctorpb.ExecuteRemediationRequest{FindingId: "finding-failure-escalation", StepIndex: 0, ApprovalToken: approvalTok})
	if err != nil {
		t.Fatalf("approved call error: %v", err)
	}
	if approved.GetStatus() != "executed" {
		t.Fatalf("approved call should execute, got status=%q", approved.GetStatus())
	}
}
