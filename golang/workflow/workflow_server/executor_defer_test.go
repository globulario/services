package main

import (
	"testing"
	"time"

	"github.com/globulario/services/golang/workflow/workflowpb"
)

// TestSchedulerSkipsDeferredBeforeCooldown is the workflow_server-level
// proof that B2 wires up: a deferred run whose backoff_until_ms is in
// the future causes shouldSkipForDeferral to return true (= dispatch
// is suppressed). Tested in isolation to avoid pulling in scylla.
func TestSchedulerSkipsDeferredBeforeCooldown(t *testing.T) {
	now := time.Now()
	deferUntil := now.Add(60 * time.Second).UnixMilli()
	deferred := &workflowpb.WorkflowRun{
		Id:             "run-1",
		Status:         workflowpb.RunStatus_RUN_STATUS_DEFERRED,
		BackoffUntilMs: deferUntil,
		RetryAttempt:   1,
	}
	if !shouldSkipForDeferral(deferred, now) {
		t.Fatalf("expected skip=true at now (60s before cooldown), got false")
	}
	if !shouldSkipForDeferral(deferred, now.Add(59*time.Second)) {
		t.Fatalf("expected skip=true 1s before cooldown, got false")
	}
}

// TestSchedulerResumesAfterCooldown is the symmetric proof: once
// backoff_until_ms has elapsed, the scheduler must NOT skip — the
// caller is free to re-dispatch and the engine will retry the
// deferred step from scratch.
func TestSchedulerResumesAfterCooldown(t *testing.T) {
	now := time.Now()
	deferUntil := now.Add(-1 * time.Second).UnixMilli() // already elapsed
	deferred := &workflowpb.WorkflowRun{
		Id:             "run-1",
		Status:         workflowpb.RunStatus_RUN_STATUS_DEFERRED,
		BackoffUntilMs: deferUntil,
		RetryAttempt:   2,
	}
	if shouldSkipForDeferral(deferred, now) {
		t.Fatalf("expected skip=false after cooldown elapsed, got true")
	}
	// Boundary: at exactly defer_until the run is eligible (>, not >=).
	atBoundary := time.UnixMilli(deferUntil)
	if shouldSkipForDeferral(deferred, atBoundary) {
		t.Fatalf("expected skip=false at exactly defer_until, got true")
	}
}

// TestSchedulerIgnoresNonDeferredStatuses confirms the guard does
// nothing to runs in other statuses — pending, executing, succeeded,
// failed, blocked all flow through normal dispatch.
func TestSchedulerIgnoresNonDeferredStatuses(t *testing.T) {
	now := time.Now()
	future := now.Add(60 * time.Second).UnixMilli()
	for _, st := range []workflowpb.RunStatus{
		workflowpb.RunStatus_RUN_STATUS_PENDING,
		workflowpb.RunStatus_RUN_STATUS_EXECUTING,
		workflowpb.RunStatus_RUN_STATUS_BLOCKED,
		workflowpb.RunStatus_RUN_STATUS_RETRYING,
		workflowpb.RunStatus_RUN_STATUS_SUCCEEDED,
		workflowpb.RunStatus_RUN_STATUS_FAILED,
	} {
		r := &workflowpb.WorkflowRun{Id: "r", Status: st, BackoffUntilMs: future}
		if shouldSkipForDeferral(r, now) {
			t.Errorf("status %s with future backoff_until_ms must not trigger skip", st)
		}
	}
}

// TestSchedulerHandlesNilLatestRun confirms that the no-prior-run case
// (a fresh correlation_id with nothing recorded) is a pass-through.
func TestSchedulerHandlesNilLatestRun(t *testing.T) {
	if shouldSkipForDeferral(nil, time.Now()) {
		t.Fatalf("nil latest run must not trigger skip")
	}
}
