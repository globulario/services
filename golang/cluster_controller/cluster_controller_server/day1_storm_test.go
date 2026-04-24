package main

import (
	"context"
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_controller/resourcestore"
)

// applyCountStore wraps a Store and counts how many times Apply is called.
// Used to verify that no-op patches do not emit spurious MODIFIED events.
type applyCountStore struct {
	resourcestore.Store
	count int
}

func (s *applyCountStore) Apply(ctx context.Context, typ string, obj interface{}) (interface{}, error) {
	s.count++
	return s.Store.Apply(ctx, typ, obj)
}

// ── Phase 3 tests: retry patch semantics + equality guard ────────────────────

// TestRetryPatchMutatesRetryFields verifies the "retry" SetFields case:
//   - RetryCount is incremented.
//   - LastRetryUnixMs and NextRetryUnixMs are set.
//   - LastTransientError and BlockedReason are populated.
//   - TransitionReason is updated.
//   - Phase is NOT changed.
//   - LastTransitionUnixMs is NOT touched (it belongs to the phase transition, not retry bookkeeping).
func TestRetryPatchMutatesRetryFields(t *testing.T) {
	before := time.Now()
	s := &cluster_controllerpb.ServiceReleaseStatus{
		Phase:                cluster_controllerpb.ReleasePhaseResolved,
		TransitionReason:     "resolved",
		Message:              "",
		LastTransitionUnixMs: 1000,
		RetryCount:           0,
	}

	applyPatchToSvcStatus(s, statusPatch{
		Message:          "workflow circuit breaker open: 5 failures in 5m0s",
		TransitionReason: "workflow_transient_error",
		BlockedReason:    "workflow_circuit_open",
		SetFields:        "retry",
	})

	if s.Phase != cluster_controllerpb.ReleasePhaseResolved {
		t.Errorf("Phase must not change on retry patch, got %q", s.Phase)
	}
	if s.RetryCount != 1 {
		t.Errorf("RetryCount should be 1 after first retry, got %d", s.RetryCount)
	}
	if s.LastRetryUnixMs < before.UnixMilli() {
		t.Errorf("LastRetryUnixMs should be >= now, got %d", s.LastRetryUnixMs)
	}
	if s.NextRetryUnixMs <= s.LastRetryUnixMs {
		t.Errorf("NextRetryUnixMs should be > LastRetryUnixMs, got next=%d last=%d", s.NextRetryUnixMs, s.LastRetryUnixMs)
	}
	if s.LastTransientError == "" {
		t.Error("LastTransientError must be set by retry patch")
	}
	if s.BlockedReason != "workflow_circuit_open" {
		t.Errorf("BlockedReason not set: %q", s.BlockedReason)
	}
	if s.TransitionReason != "workflow_transient_error" {
		t.Errorf("TransitionReason not updated: %q", s.TransitionReason)
	}
	// LastTransitionUnixMs must be untouched — it belongs to the RESOLVED transition.
	if s.LastTransitionUnixMs != 1000 {
		t.Errorf("LastTransitionUnixMs must not be changed by retry patch, got %d", s.LastTransitionUnixMs)
	}
}

// TestUnknownPatchFieldDoesNotApply verifies that an unrecognized SetFields value
// is a no-op: no status fields are mutated.
func TestUnknownPatchFieldDoesNotApply(t *testing.T) {
	s := &cluster_controllerpb.ServiceReleaseStatus{
		Phase:   cluster_controllerpb.ReleasePhaseResolved,
		Message: "original",
	}
	applyPatchToSvcStatus(s, statusPatch{
		Phase:     cluster_controllerpb.ReleasePhaseFailed,
		Message:   "injected",
		SetFields: "unknown_field_that_does_not_exist",
	})
	if s.Phase != cluster_controllerpb.ReleasePhaseResolved {
		t.Errorf("Phase must not change for unknown SetFields, got %q", s.Phase)
	}
	if s.Message != "original" {
		t.Errorf("Message must not change for unknown SetFields, got %q", s.Message)
	}
}

// TestRetryPatchAlwaysAdvancesState verifies that every "retry" patch call
// increments RetryCount and advances NextRetryUnixMs — meaning patchReleaseStatus
// always calls Apply for retry patches.
//
// The MODIFIED storm is prevented not by suppressing Apply, but by the
// reconciler's "if NextRetryUnixMs > now → return early" guard, which prevents
// re-dispatch until the backoff window expires.
func TestRetryPatchAlwaysAdvancesState(t *testing.T) {
	base := resourcestore.NewMemStore()
	counting := &applyCountStore{Store: base}
	srv := &server{resources: counting}

	ctx := context.Background()

	rel := &cluster_controllerpb.ServiceRelease{
		Meta: &cluster_controllerpb.ObjectMeta{Name: "test-svc"},
		Spec: &cluster_controllerpb.ServiceReleaseSpec{ServiceName: "cluster-controller"},
		Status: &cluster_controllerpb.ServiceReleaseStatus{
			Phase:            cluster_controllerpb.ReleasePhaseResolved,
			TransitionReason: "resolved",
		},
	}
	if _, err := counting.Apply(ctx, "ServiceRelease", rel); err != nil {
		t.Fatalf("pre-populate: %v", err)
	}
	setupCount := counting.count // 1

	retryPatch := func() error {
		return srv.patchReleaseStatus(ctx, "test-svc", func(s *cluster_controllerpb.ServiceReleaseStatus) {
			applyPatchToSvcStatus(s, statusPatch{
				Message:          "workflow circuit breaker open",
				TransitionReason: "workflow_transient_error",
				BlockedReason:    "workflow_circuit_open",
				SetFields:        "retry",
			})
		})
	}

	// First retry: RetryCount 0→1, Apply must be called.
	if err := retryPatch(); err != nil {
		t.Fatalf("first retry patch: %v", err)
	}
	if counting.count != setupCount+1 {
		t.Errorf("first retry patch must call Apply (count %d → %d)", setupCount, counting.count)
	}

	// Second retry: RetryCount 1→2, NextRetryUnixMs advances — Apply again.
	if err := retryPatch(); err != nil {
		t.Fatalf("second retry patch: %v", err)
	}
	if counting.count != setupCount+2 {
		t.Errorf("second retry patch must call Apply (count %d → %d)", setupCount+1, counting.count)
	}

	// Verify RetryCount is 2.
	obj, _, _ := base.Get(ctx, "ServiceRelease", "test-svc")
	if got, _ := obj.(*cluster_controllerpb.ServiceRelease); got != nil {
		if got.Status.RetryCount != 2 {
			t.Errorf("RetryCount should be 2 after two retry patches, got %d", got.Status.RetryCount)
		}
		if got.Status.NextRetryUnixMs <= 0 {
			t.Error("NextRetryUnixMs must be set after retry patches")
		}
	}
}

// TestNoopPatchEquality verifies that patchReleaseStatus skips Apply when
// a "phase" patch changes nothing (same phase, same message, same reason, same NextRetryUnixMs).
func TestNoopPatchEquality(t *testing.T) {
	base := resourcestore.NewMemStore()
	counting := &applyCountStore{Store: base}
	srv := &server{resources: counting}

	ctx := context.Background()

	rel := &cluster_controllerpb.ServiceRelease{
		Meta: &cluster_controllerpb.ObjectMeta{Name: "test-svc"},
		Spec: &cluster_controllerpb.ServiceReleaseSpec{ServiceName: "cluster-controller"},
		Status: &cluster_controllerpb.ServiceReleaseStatus{
			Phase:            cluster_controllerpb.ReleasePhaseResolved,
			Message:          "waiting for artifact",
			TransitionReason: "artifact_not_found",
		},
	}
	if _, err := counting.Apply(ctx, "ServiceRelease", rel); err != nil {
		t.Fatalf("pre-populate: %v", err)
	}
	setupCount := counting.count

	// Apply a "phase" patch that sets the same phase, message, reason.
	err := srv.patchReleaseStatus(ctx, "test-svc", func(s *cluster_controllerpb.ServiceReleaseStatus) {
		applyPatchToSvcStatus(s, statusPatch{
			Phase:            cluster_controllerpb.ReleasePhaseResolved,
			TransitionReason: "artifact_not_found",
			SetFields:        "phase",
		})
	})
	if err != nil {
		t.Fatalf("no-op phase patch: %v", err)
	}
	// Phase changed (RESOLVED→RESOLVED) is handled by applyPatchToSvcStatus which
	// returns true, but equality guard in patchReleaseStatus should catch the
	// phase+message+reason+nextRetryUnixMs equality and skip Apply.
	// NOTE: "phase" SetFields sets s.Phase = p.Phase (same value) and
	// s.TransitionReason = p.TransitionReason (same value) — nothing changes.
	if counting.count != setupCount {
		t.Errorf("Apply should not be called when nothing changes (count %d → %d)", setupCount, counting.count)
	}
}

// TestTransientWorkflowErrorBacksOff verifies that reconcileRelease does NOT
// call reconcileResolved (dispatch) when the release is in RESOLVED phase with
// TransitionReason="workflow_transient_error" and a recent LastTransitionUnixMs.
//
// This prevents the controller from re-dispatching a workflow to an unhealthy
// workflow service on every reconcile tick during a circuit-breaker-open period.
func TestTransientWorkflowErrorBacksOff(t *testing.T) {
	store := resourcestore.NewMemStore()
	srv := &server{resources: store}
	srv.leader.Store(true) // must be leader to pass mustBeLeader()
	srv.state = &controllerState{Nodes: map[string]*nodeState{}}

	ctx := context.Background()

	// A release stuck in RESOLVED with NextRetryUnixMs set to 30s in the future
	// (simulates a retry patch that was just written by the previous reconcile).
	rel := &cluster_controllerpb.ServiceRelease{
		Meta: &cluster_controllerpb.ObjectMeta{Name: "cluster-controller"},
		Spec: &cluster_controllerpb.ServiceReleaseSpec{ServiceName: "cluster-controller"},
		Status: &cluster_controllerpb.ServiceReleaseStatus{
			Phase:            cluster_controllerpb.ReleasePhaseResolved,
			TransitionReason: "workflow_transient_error",
			RetryCount:       1,
			NextRetryUnixMs:  time.Now().Add(30 * time.Second).UnixMilli(), // well in the future
		},
	}
	if _, err := store.Apply(ctx, "ServiceRelease", rel); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Wrap store with a counter AFTER pre-populate so baseline count is 0.
	counting := &applyCountStore{Store: store}
	srv.resources = counting

	srv.reconcileRelease(ctx, "cluster-controller")

	// Backoff must have suppressed all status writes.
	if counting.count != 0 {
		t.Errorf("reconcileRelease must not write status during transient backoff window (Apply count=%d)", counting.count)
	}

	// Phase must still be RESOLVED (no spurious transition to APPLYING or FAILED).
	obj, _, err := store.Get(ctx, "ServiceRelease", "cluster-controller")
	if err != nil {
		t.Fatalf("get after reconcile: %v", err)
	}
	got, _ := obj.(*cluster_controllerpb.ServiceRelease)
	if got == nil || got.Status == nil {
		t.Fatal("release disappeared from store after reconcile")
	}
	if got.Status.Phase != cluster_controllerpb.ReleasePhaseResolved {
		t.Errorf("Phase must remain RESOLVED after backoff suppression, got %q", got.Status.Phase)
	}
}
