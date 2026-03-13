package main

import (
	"context"
	"strings"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_controller/resourcestore"
	"github.com/globulario/services/golang/plan/planpb"
)

// ── Conformance: update and rollback ──────────────────────────────────────────

func TestConformance_VersionChange_NewPlan(t *testing.T) {
	digestV1 := strings.Repeat("a", 64)
	digestV2 := strings.Repeat("b", 64)

	relV1 := conformanceRelease("authentication", "1.0.0", digestV1)
	relV2 := conformanceRelease("authentication", "2.0.0", digestV2)

	planV1, err := CompileReleasePlan("node-1", relV1, "", "cluster-1")
	if err != nil {
		t.Fatalf("compile v1: %v", err)
	}
	planV2, err := CompileReleasePlan("node-1", relV2, "1.0.0", "cluster-1")
	if err != nil {
		t.Fatalf("compile v2: %v", err)
	}

	// Plans should have different desired hashes.
	if planV1.GetDesiredHash() == planV2.GetDesiredHash() {
		t.Fatal("version change must produce different desired hashes")
	}

	// V2 plan should have rollback steps (upgrading from 1.0.0).
	if len(planV2.GetSpec().GetRollback()) == 0 {
		t.Fatal("upgrade plan should include rollback steps")
	}
}

func TestConformance_DriftDetection_DispatchesOnMismatch(t *testing.T) {
	srv := &server{
		cfg: &clusterControllerConfig{},
		state: &controllerState{Nodes: map[string]*nodeState{
			"n1": {
				NodeID:              "n1",
				AppliedServicesHash: "old-hash",
				Status:              "ready",
				Units:               []unitStatusRecord{{Name: serviceUnitForCanonical("authentication"), State: "active"}},
			},
		}},
		resources: resourcestore.NewMemStore(),
	}
	ps := &stubPlanStore{}
	srv.planStore = ps

	srv.resources.Apply(context.Background(), "ServiceRelease", &cluster_controllerpb.ServiceRelease{
		Meta: &cluster_controllerpb.ObjectMeta{Name: "auth-rel", Generation: 1},
		Spec: &cluster_controllerpb.ServiceReleaseSpec{
			PublisherID: "core@globular.io",
			ServiceName: "authentication",
		},
		Status: &cluster_controllerpb.ServiceReleaseStatus{
			Phase:              cluster_controllerpb.ReleasePhaseAvailable,
			DesiredHash:        "new-hash",
			ObservedGeneration: 1,
			Nodes: []*cluster_controllerpb.NodeReleaseStatus{
				{NodeID: "n1"},
			},
		},
	})

	dispatched := false
	srv.testHasActivePlanWithLock = func(context.Context, string, string) bool { return false }
	srv.testDispatchReleasePlan = func(ctx context.Context, rel *cluster_controllerpb.ServiceRelease, nodeID string) (*planpb.NodePlan, error) {
		dispatched = true
		return &planpb.NodePlan{PlanId: "plan-drift", NodeId: nodeID}, nil
	}

	obj, _, _ := srv.resources.Get(context.Background(), "ServiceRelease", "auth-rel")
	rel := obj.(*cluster_controllerpb.ServiceRelease)

	if err := srv.reconcileReleaseAvailable(context.Background(), rel); err != nil {
		t.Fatalf("reconcileReleaseAvailable: %v", err)
	}
	if !dispatched {
		t.Fatal("expected plan dispatch on hash mismatch (drift)")
	}
}

func TestConformance_NoDrift_NoDispatch(t *testing.T) {
	desiredHash := "converged-hash"
	srv := &server{
		cfg: &clusterControllerConfig{},
		state: &controllerState{Nodes: map[string]*nodeState{
			"n1": {
				NodeID:              "n1",
				AppliedServicesHash: desiredHash,
				Status:              "ready",
				Units:               []unitStatusRecord{{Name: serviceUnitForCanonical("authentication"), State: "active"}},
			},
		}},
		resources: resourcestore.NewMemStore(),
	}
	ps := &stubPlanStore{}
	srv.planStore = ps

	srv.resources.Apply(context.Background(), "ServiceRelease", &cluster_controllerpb.ServiceRelease{
		Meta: &cluster_controllerpb.ObjectMeta{Name: "auth-rel", Generation: 1},
		Spec: &cluster_controllerpb.ServiceReleaseSpec{
			PublisherID: "core@globular.io",
			ServiceName: "authentication",
		},
		Status: &cluster_controllerpb.ServiceReleaseStatus{
			Phase:              cluster_controllerpb.ReleasePhaseAvailable,
			DesiredHash:        desiredHash,
			ObservedGeneration: 1,
			Nodes: []*cluster_controllerpb.NodeReleaseStatus{
				{NodeID: "n1"},
			},
		},
	})

	srv.testHasActivePlanWithLock = func(context.Context, string, string) bool { return false }
	srv.testDispatchReleasePlan = func(ctx context.Context, rel *cluster_controllerpb.ServiceRelease, nodeID string) (*planpb.NodePlan, error) {
		t.Fatal("dispatch should not occur when hashes match")
		return nil, nil
	}

	obj, _, _ := srv.resources.Get(context.Background(), "ServiceRelease", "auth-rel")
	rel := obj.(*cluster_controllerpb.ServiceRelease)

	if err := srv.reconcileReleaseAvailable(context.Background(), rel); err != nil {
		t.Fatalf("reconcileReleaseAvailable: %v", err)
	}
}

func TestConformance_RollbackRestoresKnownGoodVersion(t *testing.T) {
	digest := strings.Repeat("c", 64)
	rel := conformanceRelease("rbac", "2.0.0", digest)

	// Upgrading from 1.5.0 → 2.0.0.
	plan, err := CompileReleasePlan("node-1", rel, "1.5.0", "cluster-1")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	rollbackSteps := plan.GetSpec().GetRollback()
	if len(rollbackSteps) == 0 {
		t.Fatal("expected rollback steps")
	}

	// Rollback must restore the prior version (1.5.0).
	var rollbackVersion string
	for _, step := range rollbackSteps {
		if step.GetAction() == "service.write_version_marker" {
			rollbackVersion = step.GetArgs().GetFields()["version"].GetStringValue()
			break
		}
	}
	if rollbackVersion != "1.5.0" {
		t.Errorf("rollback should restore version 1.5.0, got %q", rollbackVersion)
	}

	// Rollback must include artifact.fetch for the prior version.
	var rollbackFetchVersion string
	for _, step := range rollbackSteps {
		if step.GetAction() == "artifact.fetch" {
			rollbackFetchVersion = step.GetArgs().GetFields()["version"].GetStringValue()
			break
		}
	}
	if rollbackFetchVersion != "1.5.0" {
		t.Errorf("rollback artifact.fetch should use version 1.5.0, got %q", rollbackFetchVersion)
	}
}

func TestConformance_DesiredHashDeterministic(t *testing.T) {
	// Same inputs → same hash every time.
	h1 := ComputeReleaseDesiredHash("pub", "svc", "1.0.0", nil)
	h2 := ComputeReleaseDesiredHash("pub", "svc", "1.0.0", nil)
	if h1 != h2 {
		t.Fatalf("DesiredHash not deterministic: %q vs %q", h1, h2)
	}

	// Different version → different hash.
	h3 := ComputeReleaseDesiredHash("pub", "svc", "2.0.0", nil)
	if h1 == h3 {
		t.Fatal("different versions must produce different hashes")
	}

	// Different publisher → different hash.
	h4 := ComputeReleaseDesiredHash("other-pub", "svc", "1.0.0", nil)
	if h1 == h4 {
		t.Fatal("different publishers must produce different hashes")
	}
}
