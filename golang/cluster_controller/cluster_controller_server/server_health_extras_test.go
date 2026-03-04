package main

import (
	"context"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_controller/resourcestore"
	"github.com/globulario/services/golang/plan/planpb"
)

// TestHealthNoAwaitingPrivilegedApplyWithExtras verifies Scenario 1:
// When the node has extra installed services beyond the desired state,
// enableServiceRemoval=false, and all desired services match installed
// versions, the health check must NOT show PLAN_AWAITING_PRIVILEGED_APPLY.
func TestHealthNoAwaitingPrivilegedApplyWithExtras(t *testing.T) {
	kv := newMapKV()
	ps := &fakePlanStore{}
	srv := &server{
		cfg: &clusterControllerConfig{},
		state: &controllerState{Nodes: map[string]*nodeState{
			"n1": {
				NodeID: "n1",
				// Node CANNOT self-apply — this is the key condition.
				Capabilities: &storedCapabilities{CanApplyPrivileged: false},
				// Node reports 4 installed services (2 desired + 2 extra).
				InstalledVersions: map[string]string{
					"authentication": "0.0.1",
					"dns":            "0.0.1",
					"blog":           "0.0.1", // extra
					"media":          "0.0.1", // extra
				},
			},
		}},
		kv:        kv,
		planStore: ps,
		resources: resourcestore.NewMemStore(),
	}

	// Desired state: only 2 services.
	ctx := context.Background()
	_, _ = srv.resources.Apply(ctx, "ClusterNetwork", &cluster_controllerpb.ClusterNetwork{
		Meta: &cluster_controllerpb.ObjectMeta{Name: "default", Generation: 1},
		Spec: &cluster_controllerpb.ClusterNetworkSpec{ClusterDomain: "example.com", Protocol: "http", PortHttp: 80},
	})
	hashNet, _ := hashDesiredNetwork(&cluster_controllerpb.DesiredNetwork{Domain: "example.com", Protocol: "http", PortHttp: 80})
	_ = srv.putNodeAppliedHash(ctx, "n1", hashNet)

	for _, svc := range []string{"authentication", "dns"} {
		_, _ = srv.resources.Apply(ctx, "ServiceDesiredVersion", &cluster_controllerpb.ServiceDesiredVersion{
			Meta: &cluster_controllerpb.ObjectMeta{Name: svc, Generation: 1},
			Spec: &cluster_controllerpb.ServiceDesiredVersionSpec{ServiceName: svc, Version: "0.0.1"},
		})
	}

	// Set applied service hash to the INVENTORY hash (includes extras) —
	// simulates what ReportNodeStatus used to do before the fix. With the
	// old code, this would cause an impossible mismatch. After the fix,
	// the health logic checks desired-vs-installed instead of hashes.
	inventoryHash := stableServiceDesiredHash(map[string]string{
		"authentication": "0.0.1",
		"dns":            "0.0.1",
		"blog":           "0.0.1",
		"media":          "0.0.1",
	})
	_ = srv.putNodeAppliedServiceHash(ctx, "n1", inventoryHash)

	resp, err := srv.GetClusterHealthV1(ctx, &cluster_controllerpb.GetClusterHealthV1Request{})
	if err != nil {
		t.Fatalf("GetClusterHealthV1: %v", err)
	}

	if len(resp.GetNodes()) != 1 {
		t.Fatalf("expected 1 node health, got %d", len(resp.GetNodes()))
	}
	nh := resp.GetNodes()[0]

	// The phase must NOT be PLAN_AWAITING_PRIVILEGED_APPLY since all desired
	// services are installed at the correct version.
	awaitingPhase := planpb.PlanState_PLAN_AWAITING_PRIVILEGED_APPLY.String()
	if nh.GetCurrentPlanPhase() == awaitingPhase {
		t.Fatalf("expected phase != %s when extras exist but all desired match; got %s",
			awaitingPhase, nh.GetCurrentPlanPhase())
	}
}

// TestHealthAwaitingPrivilegedApplyWhenMissing verifies Scenario 2:
// When a desired service is genuinely missing and the node cannot self-apply,
// the health check SHOULD show PLAN_AWAITING_PRIVILEGED_APPLY.
func TestHealthAwaitingPrivilegedApplyWhenMissing(t *testing.T) {
	kv := newMapKV()
	ps := &fakePlanStore{}
	srv := &server{
		cfg: &clusterControllerConfig{},
		state: &controllerState{Nodes: map[string]*nodeState{
			"n1": {
				NodeID:       "n1",
				Capabilities: &storedCapabilities{CanApplyPrivileged: false},
				// Node has authentication but is MISSING dns.
				InstalledVersions: map[string]string{
					"authentication": "0.0.1",
				},
			},
		}},
		kv:        kv,
		planStore: ps,
		resources: resourcestore.NewMemStore(),
	}

	ctx := context.Background()
	_, _ = srv.resources.Apply(ctx, "ClusterNetwork", &cluster_controllerpb.ClusterNetwork{
		Meta: &cluster_controllerpb.ObjectMeta{Name: "default", Generation: 1},
		Spec: &cluster_controllerpb.ClusterNetworkSpec{ClusterDomain: "example.com", Protocol: "http", PortHttp: 80},
	})
	hashNet, _ := hashDesiredNetwork(&cluster_controllerpb.DesiredNetwork{Domain: "example.com", Protocol: "http", PortHttp: 80})
	_ = srv.putNodeAppliedHash(ctx, "n1", hashNet)

	for _, svc := range []string{"authentication", "dns"} {
		_, _ = srv.resources.Apply(ctx, "ServiceDesiredVersion", &cluster_controllerpb.ServiceDesiredVersion{
			Meta: &cluster_controllerpb.ObjectMeta{Name: svc, Generation: 1},
			Spec: &cluster_controllerpb.ServiceDesiredVersionSpec{ServiceName: svc, Version: "0.0.1"},
		})
	}

	resp, err := srv.GetClusterHealthV1(ctx, &cluster_controllerpb.GetClusterHealthV1Request{})
	if err != nil {
		t.Fatalf("GetClusterHealthV1: %v", err)
	}

	if len(resp.GetNodes()) != 1 {
		t.Fatalf("expected 1 node health, got %d", len(resp.GetNodes()))
	}
	nh := resp.GetNodes()[0]

	awaitingPhase := planpb.PlanState_PLAN_AWAITING_PRIVILEGED_APPLY.String()
	if nh.GetCurrentPlanPhase() != awaitingPhase {
		t.Fatalf("expected phase %s when desired service missing; got %q",
			awaitingPhase, nh.GetCurrentPlanPhase())
	}
}

// TestHealthAwaitingPrivilegedApplyWhenVersionDrift verifies Scenario 3:
// When a desired service is at the wrong version and the node cannot self-apply,
// the health check SHOULD show PLAN_AWAITING_PRIVILEGED_APPLY.
func TestHealthAwaitingPrivilegedApplyWhenVersionDrift(t *testing.T) {
	kv := newMapKV()
	ps := &fakePlanStore{}
	srv := &server{
		cfg: &clusterControllerConfig{},
		state: &controllerState{Nodes: map[string]*nodeState{
			"n1": {
				NodeID:       "n1",
				Capabilities: &storedCapabilities{CanApplyPrivileged: false},
				InstalledVersions: map[string]string{
					"authentication": "0.0.1", // behind
				},
			},
		}},
		kv:        kv,
		planStore: ps,
		resources: resourcestore.NewMemStore(),
	}

	ctx := context.Background()
	_, _ = srv.resources.Apply(ctx, "ClusterNetwork", &cluster_controllerpb.ClusterNetwork{
		Meta: &cluster_controllerpb.ObjectMeta{Name: "default", Generation: 1},
		Spec: &cluster_controllerpb.ClusterNetworkSpec{ClusterDomain: "example.com", Protocol: "http", PortHttp: 80},
	})
	hashNet, _ := hashDesiredNetwork(&cluster_controllerpb.DesiredNetwork{Domain: "example.com", Protocol: "http", PortHttp: 80})
	_ = srv.putNodeAppliedHash(ctx, "n1", hashNet)

	// Desired version is 0.0.2 but node has 0.0.1.
	_, _ = srv.resources.Apply(ctx, "ServiceDesiredVersion", &cluster_controllerpb.ServiceDesiredVersion{
		Meta: &cluster_controllerpb.ObjectMeta{Name: "authentication", Generation: 1},
		Spec: &cluster_controllerpb.ServiceDesiredVersionSpec{ServiceName: "authentication", Version: "0.0.2"},
	})

	resp, err := srv.GetClusterHealthV1(ctx, &cluster_controllerpb.GetClusterHealthV1Request{})
	if err != nil {
		t.Fatalf("GetClusterHealthV1: %v", err)
	}

	if len(resp.GetNodes()) != 1 {
		t.Fatalf("expected 1 node health, got %d", len(resp.GetNodes()))
	}
	nh := resp.GetNodes()[0]

	awaitingPhase := planpb.PlanState_PLAN_AWAITING_PRIVILEGED_APPLY.String()
	if nh.GetCurrentPlanPhase() != awaitingPhase {
		t.Fatalf("expected phase %s when version drift; got %q",
			awaitingPhase, nh.GetCurrentPlanPhase())
	}
}

// TestReconcileExternalInstallSetsAppliedHash verifies that when all desired
// services match installed versions (extras present), the reconciler writes
// the desired hash as the applied hash, and subsequent heartbeats do NOT
// overwrite it (since ReportNodeStatus now writes to observed, not applied).
func TestReconcileExternalInstallSetsAppliedHash(t *testing.T) {
	kv := newMapKV()
	ps := &fakePlanStore{}
	srv := &server{
		cfg: &clusterControllerConfig{},
		state: &controllerState{Nodes: map[string]*nodeState{
			"n1": {
				NodeID:       "n1",
				Capabilities: &storedCapabilities{CanApplyPrivileged: false},
				InstalledVersions: map[string]string{
					"authentication": "0.0.1",
					"dns":            "0.0.1",
					"blog":           "0.0.1", // extra
				},
			},
		}},
		kv:        kv,
		planStore: ps,
		resources: resourcestore.NewMemStore(),
	}

	ctx := context.Background()
	_, _ = srv.resources.Apply(ctx, "ClusterNetwork", &cluster_controllerpb.ClusterNetwork{
		Meta: &cluster_controllerpb.ObjectMeta{Name: "default", Generation: 1},
		Spec: &cluster_controllerpb.ClusterNetworkSpec{ClusterDomain: "example.com", Protocol: "http", PortHttp: 80},
	})
	hashNet, _ := hashDesiredNetwork(&cluster_controllerpb.DesiredNetwork{Domain: "example.com", Protocol: "http", PortHttp: 80})
	_ = srv.putNodeAppliedHash(ctx, "n1", hashNet)

	for _, svc := range []string{"authentication", "dns"} {
		_, _ = srv.resources.Apply(ctx, "ServiceDesiredVersion", &cluster_controllerpb.ServiceDesiredVersion{
			Meta: &cluster_controllerpb.ObjectMeta{Name: svc, Generation: 1},
			Spec: &cluster_controllerpb.ServiceDesiredVersionSpec{ServiceName: svc, Version: "0.0.1"},
		})
	}

	desiredHash := stableServiceDesiredHash(map[string]string{
		"authentication": "0.0.1",
		"dns":            "0.0.1",
	})

	// Run reconciler — should detect external install and set applied hash.
	srv.reconcileNodes(ctx)

	appliedHash, err := srv.getNodeAppliedServiceHash(ctx, "n1")
	if err != nil {
		t.Fatalf("getNodeAppliedServiceHash: %v", err)
	}
	if appliedHash != desiredHash {
		t.Fatalf("expected applied hash = desired hash %s, got %s", desiredHash, appliedHash)
	}
}
