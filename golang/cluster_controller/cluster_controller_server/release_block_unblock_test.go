package main

import (
	"context"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_controller/resourcestore"
)

func TestReconcileRelease_DeterministicBlocked_ParkedUntilUnblockSignal(t *testing.T) {
	ctx := context.Background()
	store := resourcestore.NewMemStore()

	srv := &server{resources: store}
	srv.leader.Store(true)
	srv.state = &controllerState{Nodes: map[string]*nodeState{}}

	rel := &cluster_controllerpb.ServiceRelease{
		Meta: &cluster_controllerpb.ObjectMeta{
			Name:        "sql",
			Generation:  1,
			Annotations: map[string]string{},
		},
		Spec: &cluster_controllerpb.ServiceReleaseSpec{
			ServiceName: "sql",
			Config:      map[string]string{"native_dependency_policy": "manual"},
		},
		Status: &cluster_controllerpb.ServiceReleaseStatus{
			Phase:              cluster_controllerpb.ReleasePhaseFailed,
			ObservedGeneration: 1,
			BlockedReason:      blockedReasonNativeDependencyMissing,
		},
	}
	if _, err := store.Apply(ctx, "ServiceRelease", rel); err != nil {
		t.Fatalf("setup apply: %v", err)
	}

	// No signal: must remain FAILED (parked).
	srv.reconcileRelease(ctx, "sql")
	obj, _, err := store.Get(ctx, "ServiceRelease", "sql")
	if err != nil {
		t.Fatalf("get after parked reconcile: %v", err)
	}
	got := obj.(*cluster_controllerpb.ServiceRelease)
	if got.Status.Phase != cluster_controllerpb.ReleasePhaseFailed {
		t.Fatalf("phase=%s, want FAILED when no unblock signal", got.Status.Phase)
	}

	// Set operator-resume unblock signal.
	if got.Meta.Annotations == nil {
		got.Meta.Annotations = make(map[string]string)
	}
	got.Meta.Annotations[annotationUnblockResume] = "true"
	if _, err := store.Apply(ctx, "ServiceRelease", got); err != nil {
		t.Fatalf("apply unblock annotation: %v", err)
	}

	srv.reconcileRelease(ctx, "sql")
	obj, _, err = store.Get(ctx, "ServiceRelease", "sql")
	if err != nil {
		t.Fatalf("get after unblocked reconcile: %v", err)
	}
	got = obj.(*cluster_controllerpb.ServiceRelease)
	if got.Status.Phase != cluster_controllerpb.ReleasePhasePending {
		t.Fatalf("phase=%s, want PENDING after unblock signal", got.Status.Phase)
	}
	if got.Status.BlockedReason != "" {
		t.Fatalf("blocked_reason=%q, want cleared", got.Status.BlockedReason)
	}
}
