package main

import (
	"context"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_controller/resourcestore"
)

// TestServiceRemovalViaReleasePipeline verifies that setting Removing=true on
// a ServiceRelease triggers the release pipeline removal workflow (REMOVING phase).
func TestServiceRemovalViaReleasePipeline(t *testing.T) {
	ps := &fakePlanStore{}
	srv := &server{
		cfg: &clusterControllerConfig{},
		state: &controllerState{Nodes: map[string]*nodeState{
			"n1": {
				NodeID:       "n1",
				Units:        []unitStatusRecord{{Name: serviceUnitForCanonical("gateway")}},
				Capabilities: &storedCapabilities{CanApplyPrivileged: true},
			},
		}},
		planStore:            ps,
		resources:            resourcestore.NewMemStore(),
		enableServiceRemoval: true,
		planSignerState:      testPlanSigner(t),
	}

	// Create a ServiceRelease with Removing=true.
	_, _ = srv.resources.Apply(context.Background(), "ServiceRelease", &cluster_controllerpb.ServiceRelease{
		Meta: &cluster_controllerpb.ObjectMeta{Name: "core@globular.io/gateway", Generation: 1},
		Spec: &cluster_controllerpb.ServiceReleaseSpec{
			PublisherID: "core@globular.io",
			ServiceName: "gateway",
			Version:     "0.1.0",
			Removing:    true,
		},
		Status: &cluster_controllerpb.ServiceReleaseStatus{
			Phase:              cluster_controllerpb.ReleasePhaseAvailable,
			ObservedGeneration: 1,
			ResolvedVersion:    "0.1.0",
			Nodes: []*cluster_controllerpb.NodeReleaseStatus{
				{NodeID: "n1", Phase: cluster_controllerpb.ReleasePhaseAvailable},
			},
		},
	})

	// Reconcile should transition to REMOVING and dispatch uninstall plans.
	srv.reconcileRelease(context.Background(), "core@globular.io/gateway")

	// Verify the release moved to REMOVING.
	obj, _, err := srv.resources.Get(context.Background(), "ServiceRelease", "core@globular.io/gateway")
	if err != nil {
		t.Fatalf("get release: %v", err)
	}
	rel := obj.(*cluster_controllerpb.ServiceRelease)
	if rel.Status.Phase != ReleasePhaseRemoving {
		t.Fatalf("expected phase REMOVING, got %s", rel.Status.Phase)
	}

	// Verify uninstall plan was dispatched.
	if ps.lastPlan == nil {
		t.Fatalf("expected uninstall plan emitted")
	}
	if ps.lastPlan.GetReason() != "service_remove" {
		t.Fatalf("expected service_remove reason, got %s", ps.lastPlan.GetReason())
	}
}
