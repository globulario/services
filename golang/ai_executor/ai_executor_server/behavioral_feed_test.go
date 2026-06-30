package main

import (
	"context"
	"testing"
	"time"

	ai_executorpb "github.com/globulario/services/golang/ai_executor/ai_executorpb"
)

func TestBehavioralActionVerb(t *testing.T) {
	cases := map[ai_executorpb.ActionType]string{
		ai_executorpb.ActionType_ACTION_RESTART_SERVICE: "restart_service",
		ai_executorpb.ActionType_ACTION_CLEAR_STORAGE:   "clear_storage",
		ai_executorpb.ActionType_ACTION_RENEW_CERT:      "renew_cert",
		ai_executorpb.ActionType_ACTION_BLOCK_IP:        "block_ip",
		ai_executorpb.ActionType_ACTION_DRAIN_ENDPOINT:  "drain_endpoint",
		ai_executorpb.ActionType_ACTION_NOTIFY_ADMIN:    "notify_admin",
		ai_executorpb.ActionType_ACTION_NONE:            "observe",
	}
	for in, want := range cases {
		if got := behavioralActionVerb(in); got != want {
			t.Errorf("behavioralActionVerb(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestBehavioralOutcomeStatus(t *testing.T) {
	// Only terminal taken-action statuses map to an outcome; everything else is
	// "" (no outcome recorded yet) so observe-only / pending never fabricate one.
	cases := map[ai_executorpb.ActionStatus]string{
		ai_executorpb.ActionStatus_ACTION_SUCCEEDED: "success",
		ai_executorpb.ActionStatus_ACTION_FAILED:    "failure",
		ai_executorpb.ActionStatus_ACTION_SKIPPED:   "",
		ai_executorpb.ActionStatus_ACTION_PENDING:   "",
		ai_executorpb.ActionStatus_ACTION_EXECUTING: "",
	}
	for in, want := range cases {
		if got := behavioralOutcomeStatus(in); got != want {
			t.Errorf("behavioralOutcomeStatus(%v) = %q, want %q", in, got, want)
		}
	}
}

// The afferent feed must be strictly non-fatal: even with no reachable
// behavioral-memory (and a cancelled context), it returns without panicking — the
// executor's remediation path must never be taken down by a feed problem
// (intent.ai.supplementary_not_required).
func TestRecordBehavioralExperienceNonFatal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // force any RPC to fail fast

	diagnosis := &ai_executorpb.Diagnosis{
		IncidentId:     "inc-test",
		RootCause:      "etcd.nospace",
		Summary:        "test incident",
		ProposedAction: "restart_service",
	}
	action := &ai_executorpb.RemediationAction{
		Id:         "act-test",
		IncidentId: "inc-test",
		Type:       ai_executorpb.ActionType_ACTION_RESTART_SERVICE,
		Status:     ai_executorpb.ActionStatus_ACTION_SUCCEEDED,
		Target:     "etcd",
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		recordBehavioralExperience(ctx, diagnosis, action) // must not panic
	}()
	<-done // if it panicked, the goroutine crash would fail the test process
}

func TestNewBehavioralTraceAction(t *testing.T) {
	diag := &ai_executorpb.Diagnosis{
		IncidentId:     "inc-trace",
		ProposedAction: "restart_service",
		ActionReason:   "diagnostic trace",
	}

	trace := newBehavioralTraceAction(diag, ai_executorpb.ActionStatus_ACTION_SKIPPED)
	if trace.GetIncidentId() != "inc-trace" {
		t.Fatalf("incident id = %q, want inc-trace", trace.GetIncidentId())
	}
	if trace.GetType() != ai_executorpb.ActionType_ACTION_RESTART_SERVICE {
		t.Fatalf("type = %v, want ACTION_RESTART_SERVICE", trace.GetType())
	}
	if trace.GetStatus() != ai_executorpb.ActionStatus_ACTION_SKIPPED {
		t.Fatalf("status = %v, want ACTION_SKIPPED", trace.GetStatus())
	}
	if trace.GetTarget() != "restart_service" {
		t.Fatalf("target = %q, want restart_service", trace.GetTarget())
	}
}

func TestObserveAndPendingEmitBehavioralExperience(t *testing.T) {
	orig := emitBehavioralExperience
	defer func() { emitBehavioralExperience = orig }()

	type seen struct {
		status ai_executorpb.ActionStatus
	}
	seenCh := make(chan seen, 2)
	emitBehavioralExperience = func(_ context.Context, _ *ai_executorpb.Diagnosis, action *ai_executorpb.RemediationAction) {
		seenCh <- seen{status: action.GetStatus()}
	}

	diag := &ai_executorpb.Diagnosis{
		IncidentId:     "inc-emit",
		ProposedAction: "observe_and_record",
		ActionReason:   "test",
	}

	emitTierBehavioralExperience(context.Background(), diag, 0)
	emitTierBehavioralExperience(context.Background(), diag, 1)
	emitTierBehavioralExperience(context.Background(), diag, 2)

	var got []ai_executorpb.ActionStatus
	timeout := time.After(2 * time.Second)
	for len(got) < 2 {
		select {
		case s := <-seenCh:
			got = append(got, s.status)
		case <-timeout:
			t.Fatalf("timed out waiting for behavioral emits; got %v", got)
		}
	}

	want := map[ai_executorpb.ActionStatus]bool{
		ai_executorpb.ActionStatus_ACTION_SKIPPED: true,
		ai_executorpb.ActionStatus_ACTION_PENDING: true,
	}
	for _, status := range got {
		if !want[status] {
			t.Fatalf("unexpected emitted status %v", status)
		}
		delete(want, status)
	}
	if len(want) != 0 {
		t.Fatalf("missing emitted statuses: %v", want)
	}
}
