package main

import (
	"context"
	"testing"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"github.com/globulario/services/golang/clustercontroller/resourcestore"
	"github.com/globulario/services/golang/plan/planpb"
)

// driftTestServer builds a server with one node and injectable hooks for plan dispatch and lock guard.
func driftTestServer() *server {
	return &server{
		cfg: &clusterControllerConfig{},
		state: &controllerState{Nodes: map[string]*nodeState{
			"n1": {NodeID: "n1", AppliedServicesHash: "applied-old", Status: "ready", Units: []unitStatusRecord{{Name: serviceUnitForCanonical("gateway"), State: "active"}}},
		}},
		resources: resourcestore.NewMemStore(),
	}
}

type stubPlanStore struct {
	lastPlan *planpb.NodePlan
}

func (s *stubPlanStore) PutCurrentPlan(ctx context.Context, nodeID string, plan *planpb.NodePlan) error {
	s.lastPlan = plan
	return nil
}
func (s *stubPlanStore) GetCurrentPlan(context.Context, string) (*planpb.NodePlan, error) {
	return nil, nil
}
func (s *stubPlanStore) PutStatus(context.Context, string, *planpb.NodePlanStatus) error { return nil }
func (s *stubPlanStore) GetStatus(context.Context, string) (*planpb.NodePlanStatus, error) {
	return nil, nil
}
func (s *stubPlanStore) AppendHistory(context.Context, string, *planpb.NodePlan) error { return nil }

func TestDriftDispatchesPlanOnMismatch(t *testing.T) {
	srv := driftTestServer()
	ps := &stubPlanStore{}
	srv.planStore = ps
	srv.resources.Apply(context.Background(), "ServiceRelease", &clustercontrollerpb.ServiceRelease{
		Meta: &clustercontrollerpb.ObjectMeta{Name: "rel1", Generation: 1},
		Spec: &clustercontrollerpb.ServiceReleaseSpec{
			PublisherID: "pub",
			ServiceName: "gateway",
		},
		Status: &clustercontrollerpb.ServiceReleaseStatus{
			Phase:              clustercontrollerpb.ReleasePhaseAvailable,
			DesiredHash:        "desired-new",
			ObservedGeneration: 1,
			Nodes: []*clustercontrollerpb.NodeReleaseStatus{
				{NodeID: "n1"},
			},
		},
	})

	// Override hasActivePlanWithLock to false and dispatchReleasePlan to stub
	dispatched := false
	srv.testHasActivePlanWithLock = func(context.Context, string, string) bool { return false }
	srv.testDispatchReleasePlan = func(ctx context.Context, rel *clustercontrollerpb.ServiceRelease, nodeID string) (*planpb.NodePlan, error) {
		dispatched = true
		plan := &planpb.NodePlan{PlanId: "plan1", NodeId: nodeID}
		ps.lastPlan = plan
		return plan, nil
	}

	obj, _, _ := srv.resources.Get(context.Background(), "ServiceRelease", "rel1")
	rel := obj.(*clustercontrollerpb.ServiceRelease)

	if err := srv.reconcileReleaseAvailable(context.Background(), rel); err != nil {
		t.Fatalf("reconcileReleaseAvailable error: %v", err)
	}
	if !dispatched {
		t.Fatalf("expected dispatch on drift")
	}
}

func TestDriftSkipsWhenLockHeld(t *testing.T) {
	srv := driftTestServer()
	ps := &stubPlanStore{}
	srv.planStore = ps
	srv.resources.Apply(context.Background(), "ServiceRelease", &clustercontrollerpb.ServiceRelease{
		Meta: &clustercontrollerpb.ObjectMeta{Name: "rel1", Generation: 1},
		Spec: &clustercontrollerpb.ServiceReleaseSpec{
			PublisherID: "pub",
			ServiceName: "gateway",
		},
		Status: &clustercontrollerpb.ServiceReleaseStatus{
			Phase:              clustercontrollerpb.ReleasePhaseAvailable,
			DesiredHash:        "desired-new",
			ObservedGeneration: 1,
			Nodes: []*clustercontrollerpb.NodeReleaseStatus{
				{NodeID: "n1"},
			},
		},
	})

	srv.testHasActivePlanWithLock = func(context.Context, string, string) bool { return true }
	srv.testDispatchReleasePlan = func(ctx context.Context, rel *clustercontrollerpb.ServiceRelease, nodeID string) (*planpb.NodePlan, error) {
		t.Fatalf("dispatch should not be called when lock held")
		return nil, nil
	}

	obj, _, _ := srv.resources.Get(context.Background(), "ServiceRelease", "rel1")
	rel := obj.(*clustercontrollerpb.ServiceRelease)

	if err := srv.reconcileReleaseAvailable(context.Background(), rel); err != nil {
		t.Fatalf("reconcileReleaseAvailable error: %v", err)
	}
}

func TestNoDispatchWhenHashesMatch(t *testing.T) {
	srv := driftTestServer()
	ps := &stubPlanStore{}
	srv.planStore = ps
	srv.state.Nodes["n1"].AppliedServicesHash = "desired-new"

	srv.resources.Apply(context.Background(), "ServiceRelease", &clustercontrollerpb.ServiceRelease{
		Meta: &clustercontrollerpb.ObjectMeta{Name: "rel1", Generation: 1},
		Spec: &clustercontrollerpb.ServiceReleaseSpec{
			PublisherID: "pub",
			ServiceName: "gateway",
		},
		Status: &clustercontrollerpb.ServiceReleaseStatus{
			Phase:              clustercontrollerpb.ReleasePhaseAvailable,
			DesiredHash:        "desired-new",
			ObservedGeneration: 1,
			Nodes: []*clustercontrollerpb.NodeReleaseStatus{
				{NodeID: "n1"},
			},
		},
	})

	srv.testHasActivePlanWithLock = func(context.Context, string, string) bool { return false }
	srv.testDispatchReleasePlan = func(ctx context.Context, rel *clustercontrollerpb.ServiceRelease, nodeID string) (*planpb.NodePlan, error) {
		t.Fatalf("dispatch should not run when hashes match")
		return nil, nil
	}

	obj, _, _ := srv.resources.Get(context.Background(), "ServiceRelease", "rel1")
	rel := obj.(*clustercontrollerpb.ServiceRelease)

	if err := srv.reconcileReleaseAvailable(context.Background(), rel); err != nil {
		t.Fatalf("reconcileReleaseAvailable error: %v", err)
	}
}
