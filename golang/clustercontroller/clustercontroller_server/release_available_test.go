package main

import (
	"context"
	"strings"
	"testing"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"github.com/globulario/services/golang/clustercontroller/resourcestore"
	"github.com/globulario/services/golang/plan/planpb"
)

func TestReleaseAvailableWhenMinReplicasMatch(t *testing.T) {
	ctx := context.Background()
	ps := &fakePlanStore{}
	state := newControllerState()
	state.ClusterId = "cluster-1"
	desiredHash := ComputeReleaseDesiredHash("pub", "gateway", "1.0.0", nil)
	unit := serviceUnitForCanonical("gateway")
	state.Nodes = map[string]*nodeState{
		"n1": {NodeID: "n1", Status: "ready", AppliedServicesHash: desiredHash, Units: []unitStatusRecord{{Name: unit, State: "active"}}},
		"n2": {NodeID: "n2", Status: "ready", AppliedServicesHash: desiredHash, Units: []unitStatusRecord{{Name: unit, State: "active"}}},
	}
	srv := &server{
		cfg:       &clusterControllerConfig{},
		state:     state,
		planStore: ps,
		resources: resourcestore.NewMemStore(),
	}
	rel := testRelease(desiredHash, 0)
	rel.Status.Nodes = []*clustercontrollerpb.NodeReleaseStatus{{NodeID: "n1"}, {NodeID: "n2"}}
	if _, err := srv.resources.Apply(ctx, "ServiceRelease", rel); err != nil {
		t.Fatalf("apply release: %v", err)
	}

	srv.reconcileReleaseAvailable(ctx, rel)

	obj, _, _ := srv.resources.Get(ctx, "ServiceRelease", rel.Meta.Name)
	updated := obj.(*clustercontrollerpb.ServiceRelease)
	if updated.Status.Phase != clustercontrollerpb.ReleasePhaseAvailable {
		t.Fatalf("expected phase AVAILABLE, got %s", updated.Status.Phase)
	}
	if ps.count != 0 {
		t.Fatalf("expected no new plans emitted, got %d", ps.count)
	}
}

func TestReleaseDegradedWhenMismatchedButMinSatisfied(t *testing.T) {
	ctx := context.Background()
	ps := &fakePlanStore{}
	state := newControllerState()
	state.ClusterId = "cluster-1"
	desiredHash := ComputeReleaseDesiredHash("pub", "gateway", "1.0.0", nil)
	unit := serviceUnitForCanonical("gateway")
	state.Nodes = map[string]*nodeState{
		"n1": {NodeID: "n1", Status: "ready", AppliedServicesHash: desiredHash, Units: []unitStatusRecord{{Name: unit, State: "active"}}},
		"n2": {NodeID: "n2", Status: "ready", AppliedServicesHash: "deadbeef", Units: []unitStatusRecord{{Name: unit, State: "active"}}},
	}
	srv := &server{
		cfg:       &clusterControllerConfig{},
		state:     state,
		planStore: ps,
		resources: resourcestore.NewMemStore(),
	}
	rel := testRelease(desiredHash, 1) // maxUnavailable=1 => minReplicas=1
	rel.Status.Nodes = []*clustercontrollerpb.NodeReleaseStatus{{NodeID: "n1"}, {NodeID: "n2"}}
	if _, err := srv.resources.Apply(ctx, "ServiceRelease", rel); err != nil {
		t.Fatalf("apply release: %v", err)
	}

	srv.reconcileReleaseAvailable(ctx, rel)

	obj, _, _ := srv.resources.Get(ctx, "ServiceRelease", rel.Meta.Name)
	updated := obj.(*clustercontrollerpb.ServiceRelease)
	if updated.Status.Phase != clustercontrollerpb.ReleasePhaseDegraded {
		t.Fatalf("expected phase DEGRADED, got %s", updated.Status.Phase)
	}
	if ps.count != 1 {
		t.Fatalf("expected drift to enqueue 1 plan, got %d", ps.count)
	}
}

func TestReleaseFailedWhenMinNotSatisfied(t *testing.T) {
	ctx := context.Background()
	ps := &fakePlanStore{}
	state := newControllerState()
	state.ClusterId = "cluster-1"
	desiredHash := ComputeReleaseDesiredHash("pub", "gateway", "1.0.0", nil)
	unit := serviceUnitForCanonical("gateway")
	state.Nodes = map[string]*nodeState{
		"n1": {NodeID: "n1", Status: "ready", AppliedServicesHash: desiredHash, Units: []unitStatusRecord{{Name: unit, State: "active"}}},
		"n2": {NodeID: "n2", Status: "ready", AppliedServicesHash: "mismatch", Units: []unitStatusRecord{{Name: unit, State: "active"}}},
	}
	srv := &server{
		cfg:       &clusterControllerConfig{},
		state:     state,
		planStore: ps,
		resources: resourcestore.NewMemStore(),
	}
	rel := testRelease(desiredHash, 0) // maxUnavailable=0 => minReplicas=2
	rel.Status.Nodes = []*clustercontrollerpb.NodeReleaseStatus{{NodeID: "n1"}, {NodeID: "n2"}}
	if _, err := srv.resources.Apply(ctx, "ServiceRelease", rel); err != nil {
		t.Fatalf("apply release: %v", err)
	}

	srv.reconcileReleaseAvailable(ctx, rel)

	obj, _, _ := srv.resources.Get(ctx, "ServiceRelease", rel.Meta.Name)
	updated := obj.(*clustercontrollerpb.ServiceRelease)
	if updated.Status.Phase != clustercontrollerpb.ReleasePhaseFailed {
		t.Fatalf("expected phase FAILED, got %s", updated.Status.Phase)
	}
}

func TestDriftDoesNotDispatchWhenActivePlanLockHeld(t *testing.T) {
	ctx := context.Background()
	ps := &fakePlanStore{
		status: &planpb.NodePlanStatus{State: planpb.PlanState_PLAN_RUNNING},
		lastPlan: &planpb.NodePlan{
			Locks: []string{"service:gateway"},
		},
	}
	state := newControllerState()
	state.ClusterId = "cluster-1"
	desiredHash := ComputeReleaseDesiredHash("pub", "gateway", "1.0.0", nil)
	unit := serviceUnitForCanonical("gateway")
	state.Nodes = map[string]*nodeState{
		"n1": {NodeID: "n1", Status: "ready", AppliedServicesHash: "stale", Units: []unitStatusRecord{{Name: unit, State: "active"}}},
	}
	srv := &server{
		cfg:       &clusterControllerConfig{},
		state:     state,
		planStore: ps,
		resources: resourcestore.NewMemStore(),
	}
	rel := testRelease(desiredHash, 0)
	rel.Status.Nodes = []*clustercontrollerpb.NodeReleaseStatus{{NodeID: "n1"}}
	if _, err := srv.resources.Apply(ctx, "ServiceRelease", rel); err != nil {
		t.Fatalf("apply release: %v", err)
	}

	srv.reconcileReleaseAvailable(ctx, rel)

	if ps.count != 0 {
		t.Fatalf("expected no plan dispatch when lock held, got %d", ps.count)
	}
}

func testRelease(desiredHash string, maxUnavailable uint32) *clustercontrollerpb.ServiceRelease {
	return &clustercontrollerpb.ServiceRelease{
		Meta: &clustercontrollerpb.ObjectMeta{Name: "my-release", Generation: 1},
		Spec: &clustercontrollerpb.ServiceReleaseSpec{
			PublisherID:    "pub",
			ServiceName:    "gateway",
			MaxUnavailable: maxUnavailable,
			Version:        "1.0.0",
			Config:         map[string]string{},
		},
		Status: &clustercontrollerpb.ServiceReleaseStatus{
			Phase:                  clustercontrollerpb.ReleasePhaseAvailable,
			ResolvedVersion:        "1.0.0",
			ResolvedArtifactDigest: strings.Repeat("a", 64),
			DesiredHash:            desiredHash,
			ObservedGeneration:     1,
		},
	}
}
