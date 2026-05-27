package main

import (
	"context"
	"testing"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

func withStubbedGatePersistence(t *testing.T) {
	t.Helper()
	persisted := map[string]remediationGateState{}
	remediationGatePersistFn = func(_ context.Context, key string, state remediationGateState) {
		persisted[key] = state
	}
	remediationGateLoadFn = func(_ context.Context, key string) (remediationGateState, bool) {
		state, ok := persisted[key]
		return state, ok
	}
	remediationGateDeleteFn = func(_ context.Context, key string) {
		delete(persisted, key)
	}
	t.Cleanup(func() {
		remediationGatePersistFn = remediationGatePersist
		remediationGateLoadFn = remediationGateLoad
		remediationGateDeleteFn = remediationGateDelete
	})
}

func TestRemediationGateEscalatesAfterRepeatedCooldownRejections(t *testing.T) {
	withStubbedGatePersistence(t)
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

func TestRemediationGateGetLoadsPersistedStateAfterMemoryClear(t *testing.T) {
	withStubbedGatePersistence(t)
	key := remediationGateKey("finding-load", 0, cluster_doctorpb.ActionType_SYSTEMCTL_RESTART)
	autoRemediationGateByTarget.Delete(key)

	state := remediationGateRecordCooldownRejection(key, time.Now())
	if state.CooldownRejections != 1 {
		t.Fatalf("expected first rejection count=1, got %d", state.CooldownRejections)
	}

	autoRemediationGateByTarget.Delete(key) // simulate process restart / in-memory loss
	loaded, ok := remediationGateGet(key)
	if !ok {
		t.Fatal("expected persisted gate state to be loaded")
	}
	if loaded.CooldownRejections != 1 {
		t.Fatalf("expected persisted rejection count=1, got %d", loaded.CooldownRejections)
	}
}

func TestRemediationGateClearResetsState(t *testing.T) {
	withStubbedGatePersistence(t)
	key := remediationGateKey("finding-clear", 1, cluster_doctorpb.ActionType_FILE_DELETE)
	autoRemediationGateByTarget.Delete(key)

	remediationGateRecordCooldownRejection(key, time.Now())
	remediationGateClear(key)
	if _, ok := remediationGateGet(key); ok {
		t.Fatal("expected gate state to be cleared")
	}
}

func TestAppendRemediationGateEvidence_AnnotatesFindingAndSummarizes(t *testing.T) {
	withStubbedGatePersistence(t)
	findingID := "finding-evidence"
	key := remediationGateKey(findingID, 0, cluster_doctorpb.ActionType_SYSTEMCTL_RESTART)
	autoRemediationGateByTarget.Delete(key)

	now := time.Now()
	for i := 0; i < autoRemediationEscalationThreshold; i++ {
		remediationGateRecordCooldownRejection(key, now.Add(time.Duration(i)*time.Second))
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

	summary := appendRemediationGateEvidence(findings)
	if len(findings[0].Evidence) == 0 {
		t.Fatal("expected remediation gate evidence to be attached")
	}
	if findings[0].Evidence[0].GetKeyValues()["remediation_gate"] != "escalated" {
		t.Fatal("expected escalated remediation_gate evidence")
	}
	if summary.Escalated != 1 || summary.Cooldown != 0 {
		t.Fatalf("unexpected summary: escalated=%d cooldown=%d", summary.Escalated, summary.Cooldown)
	}
}
