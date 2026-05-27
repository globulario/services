package main

import (
	"testing"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

func TestRemediationGateEscalatesAfterRepeatedCooldownRejections(t *testing.T) {
	key := remediationGateKey("finding-esc", 0, cluster_doctorpb.ActionType_SYSTEMCTL_RESTART)
	autoRemediationGateByTarget.Delete(key)

	now := time.Now()
	for i := 0; i < autoRemediationEscalationThreshold-1; i++ {
		state := remediationGateRecordCooldownRejection(key, now.Add(time.Duration(i)*time.Second))
		if state.Escalated {
			t.Fatalf("expected non-escalated before threshold, got escalated at attempt %d", i+1)
		}
	}
	state := remediationGateRecordCooldownRejection(key, now.Add(10*time.Second))
	if !state.Escalated {
		t.Fatal("expected escalation at threshold")
	}
}

func TestRemediationGateClearResetsState(t *testing.T) {
	key := remediationGateKey("finding-clear", 1, cluster_doctorpb.ActionType_FILE_DELETE)
	autoRemediationGateByTarget.Delete(key)

	remediationGateRecordCooldownRejection(key, time.Now())
	remediationGateClear(key)
	if _, ok := remediationGateGet(key); ok {
		t.Fatal("expected gate state to be cleared")
	}
}

func TestAppendRemediationGateEvidence_AnnotatesFinding(t *testing.T) {
	findingID := "finding-evidence"
	key := remediationGateKey(findingID, 0, cluster_doctorpb.ActionType_SYSTEMCTL_RESTART)
	autoRemediationGateByTarget.Delete(key)

	state := remediationGateRecordCooldownRejection(key, time.Now())
	if state.CooldownRejections == 0 {
		t.Fatal("expected gate state to record rejection")
	}

	findings := []rules.Finding{{
		FindingID: findingID,
		Summary:   "service unhealthy",
		Remediation: []*cluster_doctorpb.RemediationStep{{
			Order: 1,
			Action: &cluster_doctorpb.RemediationAction{
				ActionType: cluster_doctorpb.ActionType_SYSTEMCTL_RESTART,
			},
		}},
	}}

	appendRemediationGateEvidence(findings)
	if len(findings[0].Evidence) == 0 {
		t.Fatal("expected remediation gate evidence to be attached")
	}
	if findings[0].Evidence[0].GetKeyValues()["remediation_gate"] == "" {
		t.Fatal("expected remediation_gate key to be present")
	}
}
