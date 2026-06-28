package main

import (
	"context"
	"errors"
	"testing"

	ai_executorpb "github.com/globulario/services/golang/ai_executor/ai_executorpb"
	"github.com/globulario/services/golang/ai_watcher/ai_watcherpb"
)

// These tests guard the SCAR recorded for the Tier-2 (REQUIRE_APPROVAL) approval
// path: approval must DISPATCH the approved action and resolve strictly on the
// execution outcome. Approval alone must transition to REMEDIATING, never
// directly to RESOLVED. Dispatch/execution failure must land in INCIDENT_FAILED
// with execution-error metadata — never a false RESOLVED.
// See ai_memory scar: ai-watcher Tier-2 approval false closure.

func awaitingApprovalServer() (*server, *ai_watcherpb.Incident) {
	inc := &ai_watcherpb.Incident{
		Id:             "inc-1",
		Status:         ai_watcherpb.IncidentStatus_INCIDENT_AWAITING_APPROVAL,
		ProposedAction: "restart_service:globular-foo.service",
		Metadata:       map[string]string{"rule_id": "service-crash"},
	}
	srv := &server{
		incidents: map[string]*ai_watcherpb.Incident{inc.Id: inc},
	}
	srv.stats.ApprovalsPending = 1
	return srv, inc
}

// withDispatch swaps the executor-dispatch seam for the duration of a test and
// returns a pointer to a call counter.
func withDispatch(t *testing.T, fn func(incidentID, approver string) (ai_executorpb.JobState, string, error)) *int {
	t.Helper()
	calls := 0
	orig := dispatchApprovedActionToExecutor
	dispatchApprovedActionToExecutor = func(incidentID, approver string) (ai_executorpb.JobState, string, error) {
		calls++
		return fn(incidentID, approver)
	}
	// Run the dispatch goroutine synchronously so the terminal state is
	// observable deterministically right after ApproveAction returns.
	origRun := runApprovalDispatch
	runApprovalDispatch = func(f func()) { f() }
	t.Cleanup(func() {
		dispatchApprovedActionToExecutor = orig
		runApprovalDispatch = origRun
	})
	return &calls
}

func TestApproveAction_SuccessResolvesOnlyAfterExecution(t *testing.T) {
	srv, inc := awaitingApprovalServer()
	calls := withDispatch(t, func(id, approver string) (ai_executorpb.JobState, string, error) {
		if id != "inc-1" || approver != "dave" {
			t.Fatalf("dispatch got id=%q approver=%q", id, approver)
		}
		return ai_executorpb.JobState_JOB_SUCCEEDED, "", nil
	})

	_, err := srv.ApproveAction(context.Background(), &ai_watcherpb.ApproveActionRqst{IncidentId: "inc-1", Approver: "dave"})
	if err != nil {
		t.Fatalf("ApproveAction: %v", err)
	}
	if *calls != 1 {
		t.Fatalf("dispatch called %d times, want exactly 1", *calls)
	}
	if inc.Status != ai_watcherpb.IncidentStatus_INCIDENT_RESOLVED {
		t.Fatalf("status=%s, want RESOLVED (only after successful execution)", inc.Status)
	}
	if inc.Metadata["approved_by"] != "dave" {
		t.Fatalf("approved_by=%q, want dave", inc.Metadata["approved_by"])
	}
	if inc.Metadata["approved_action"] != "restart_service:globular-foo.service" {
		t.Fatalf("approved_action=%q not recorded", inc.Metadata["approved_action"])
	}
	if _, bad := inc.Metadata["execution_error"]; bad {
		t.Fatalf("execution_error must not be set on success")
	}
}

func TestApproveAction_ExecutionFailureIsNotResolved(t *testing.T) {
	srv, inc := awaitingApprovalServer()
	calls := withDispatch(t, func(id, approver string) (ai_executorpb.JobState, string, error) {
		return ai_executorpb.JobState_JOB_FAILED, "unit refused to start", nil
	})

	if _, err := srv.ApproveAction(context.Background(), &ai_watcherpb.ApproveActionRqst{IncidentId: "inc-1", Approver: "dave"}); err != nil {
		t.Fatalf("ApproveAction: %v", err)
	}
	if *calls != 1 {
		t.Fatalf("dispatch called %d times, want 1", *calls)
	}
	if inc.Status == ai_watcherpb.IncidentStatus_INCIDENT_RESOLVED {
		t.Fatalf("FALSE CLOSURE: execution failed but status is RESOLVED")
	}
	if inc.Status != ai_watcherpb.IncidentStatus_INCIDENT_FAILED {
		t.Fatalf("status=%s, want FAILED", inc.Status)
	}
	if inc.Metadata["execution_error"] != "unit refused to start" {
		t.Fatalf("execution_error=%q not recorded", inc.Metadata["execution_error"])
	}
}

// The sibling failure mode: a dispatch/connection error must NOT be absorbed
// into a success state (failure_mode ai_watcher.connection_error_marked_resolved).
func TestApproveAction_DispatchErrorIsNotResolved(t *testing.T) {
	srv, inc := awaitingApprovalServer()
	withDispatch(t, func(id, approver string) (ai_executorpb.JobState, string, error) {
		return 0, "", errors.New("executor connection failed: no route")
	})

	if _, err := srv.ApproveAction(context.Background(), &ai_watcherpb.ApproveActionRqst{IncidentId: "inc-1", Approver: "dave"}); err != nil {
		t.Fatalf("ApproveAction: %v", err)
	}
	if inc.Status == ai_watcherpb.IncidentStatus_INCIDENT_RESOLVED {
		t.Fatalf("FALSE CLOSURE: dispatch error but status is RESOLVED")
	}
	if inc.Status != ai_watcherpb.IncidentStatus_INCIDENT_FAILED {
		t.Fatalf("status=%s, want FAILED", inc.Status)
	}
	if inc.Metadata["execution_error"] == "" {
		t.Fatalf("execution_error must record the dispatch failure")
	}
}

// #4: an in-flight (non-terminal) executor job state must keep the incident
// REMEDIATING — never collapse to FAILED, never claim RESOLVED.
func TestApproveAction_InFlightJobStaysRemediating(t *testing.T) {
	srv, inc := awaitingApprovalServer()
	withDispatch(t, func(id, approver string) (ai_executorpb.JobState, string, error) {
		return ai_executorpb.JobState_JOB_APPROVED, "", nil // approved but not yet executed
	})

	if _, err := srv.ApproveAction(context.Background(), &ai_watcherpb.ApproveActionRqst{IncidentId: "inc-1", Approver: "dave"}); err != nil {
		t.Fatalf("ApproveAction: %v", err)
	}
	if inc.Status == ai_watcherpb.IncidentStatus_INCIDENT_RESOLVED {
		t.Fatal("FALSE CLOSURE: in-flight job marked RESOLVED")
	}
	if inc.Status == ai_watcherpb.IncidentStatus_INCIDENT_FAILED {
		t.Fatal("in-flight job wrongly collapsed to FAILED")
	}
	if inc.Status != ai_watcherpb.IncidentStatus_INCIDENT_REMEDIATING {
		t.Fatalf("status=%s, want REMEDIATING", inc.Status)
	}
	if inc.Metadata["last_job_status"] != "JOB_APPROVED" {
		t.Fatalf("last_job_status=%q, want JOB_APPROVED", inc.Metadata["last_job_status"])
	}
}

func TestClassifyApprovalOutcome(t *testing.T) {
	cases := []struct {
		s    ai_executorpb.JobState
		want approvalOutcome
	}{
		{ai_executorpb.JobState_JOB_SUCCEEDED, approvalResolved},
		{ai_executorpb.JobState_JOB_FAILED, approvalFailed},
		{ai_executorpb.JobState_JOB_EXPIRED, approvalFailed},
		{ai_executorpb.JobState_JOB_DENIED, approvalFailed},
		{ai_executorpb.JobState_JOB_CLOSED, approvalFailed},
		{ai_executorpb.JobState_JOB_APPROVED, approvalPending},
		{ai_executorpb.JobState_JOB_EXECUTING, approvalPending},
		{ai_executorpb.JobState_JOB_AWAITING_APPROVAL, approvalPending},
		{ai_executorpb.JobState(9999), approvalPending}, // unknown → pending, never resolved
	}
	for _, c := range cases {
		if got := classifyApprovalOutcome(c.s); got != c.want {
			t.Errorf("classifyApprovalOutcome(%s)=%d, want %d", c.s, got, c.want)
		}
	}
}

func TestApproveAction_RejectsNonAwaitingIncidentWithoutDispatch(t *testing.T) {
	srv, inc := awaitingApprovalServer()
	inc.Status = ai_watcherpb.IncidentStatus_INCIDENT_DETECTED
	calls := withDispatch(t, func(id, approver string) (ai_executorpb.JobState, string, error) {
		return ai_executorpb.JobState_JOB_SUCCEEDED, "", nil
	})

	if _, err := srv.ApproveAction(context.Background(), &ai_watcherpb.ApproveActionRqst{IncidentId: "inc-1", Approver: "dave"}); err == nil {
		t.Fatalf("expected error approving a non-awaiting incident")
	}
	if *calls != 0 {
		t.Fatalf("dispatch must not run for a non-awaiting incident (ran %d times)", *calls)
	}
	if inc.Status != ai_watcherpb.IncidentStatus_INCIDENT_DETECTED {
		t.Fatalf("status mutated to %s; precondition reject must leave it untouched", inc.Status)
	}
}
