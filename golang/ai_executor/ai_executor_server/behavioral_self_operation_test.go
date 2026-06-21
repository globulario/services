package main

import (
	"context"
	"testing"

	ai_executorpb "github.com/globulario/services/golang/ai_executor/ai_executorpb"
	bpb "github.com/globulario/services/golang/ai_memory/behavioral_memorypb"
)

func TestNewSelfOperationAttempt(t *testing.T) {
	ev := newSelfOperationAttempt(&ai_executorpb.Job{
		IncidentId:   "inc-1",
		ActionTarget: "restart_service",
		State:        ai_executorpb.JobState_JOB_EXECUTING,
		Tier:         1,
		Diagnosis: &ai_executorpb.Diagnosis{
			RootCause:    "service crash",
			Summary:      "service down",
			ActionReason: "restart may recover it",
		},
	})
	if ev.sourceKind != sourceKindAgentToolAttempt {
		t.Fatalf("sourceKind=%q", ev.sourceKind)
	}
	if ev.signalKind != bpb.SignalKind_SIGNAL_OBSERVED_RUNTIME_FACT {
		t.Fatalf("signalKind=%v", ev.signalKind)
	}
	if ev.outcomeStatus != "" {
		t.Fatalf("outcomeStatus=%q", ev.outcomeStatus)
	}
	if ev.theme != "agent.self_operation.attempt.restart_service" {
		t.Fatalf("theme=%q", ev.theme)
	}
}

func TestNewBlockedSelfOperation(t *testing.T) {
	ev := newBlockedSelfOperation(&ai_executorpb.Job{
		IncidentId:   "inc-2",
		ActionTarget: "clear_storage",
		State:        ai_executorpb.JobState_JOB_DENIED,
		Tier:         2,
		DeniedBy:     "operator",
		DeniedReason: "too risky",
		Diagnosis:    &ai_executorpb.Diagnosis{RootCause: "disk pressure"},
	})
	if ev.sourceKind != sourceKindAgentBlockedAction {
		t.Fatalf("sourceKind=%q", ev.sourceKind)
	}
	if ev.outcomeStatus != "blocked" {
		t.Fatalf("outcomeStatus=%q", ev.outcomeStatus)
	}
	if ev.theme != "agent.self_operation.blocked.clear_storage" {
		t.Fatalf("theme=%q", ev.theme)
	}
	if ev.metadata["denied_by"] != "operator" {
		t.Fatalf("denied_by=%q", ev.metadata["denied_by"])
	}
}

func TestNewClassifierAndSafetySelfOperation(t *testing.T) {
	req := &ai_executorpb.PeerProposalRequest{
		ProposedAction: ai_executorpb.ActionType_ACTION_RESTART_SERVICE,
		Target:         "etcd",
		Tier:           2,
		Diagnosis:      &ai_executorpb.Diagnosis{IncidentId: "inc-3", RootCause: "group0 loss"},
	}

	denial := newClassifierDenialSelfOperation(req, ai_executorpb.PeerVote_VOTE_REJECT, "unsafe")
	if denial.sourceKind != sourceKindAgentClassifierDenial {
		t.Fatalf("denial sourceKind=%q", denial.sourceKind)
	}
	if denial.outcomeStatus != "blocked" {
		t.Fatalf("denial outcomeStatus=%q", denial.outcomeStatus)
	}
	if denial.theme != "agent.self_operation.classifier_denial.restart_service" {
		t.Fatalf("denial theme=%q", denial.theme)
	}

	safety := newSafetyHookSelfOperation(req, "human approval required")
	if safety.sourceKind != sourceKindAgentSafetyHookEvent {
		t.Fatalf("safety sourceKind=%q", safety.sourceKind)
	}
	if safety.outcomeStatus != "blocked" {
		t.Fatalf("safety outcomeStatus=%q", safety.outcomeStatus)
	}
	if safety.theme != "agent.self_operation.safety_hook.restart_service" {
		t.Fatalf("safety theme=%q", safety.theme)
	}
}

func TestRecordBehavioralSelfOperationNonFatal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		recordBehavioralSelfOperation(ctx, behavioralSelfOperation{
			sourceKind: sourceKindAgentToolAttempt,
			signalKind: bpb.SignalKind_SIGNAL_OBSERVED_RUNTIME_FACT,
			theme:      "agent.self_operation.attempt.restart_service",
			incidentID: "inc-test",
			actionType: "restart_service",
			note:       "attempt",
		})
	}()
	<-done
}
