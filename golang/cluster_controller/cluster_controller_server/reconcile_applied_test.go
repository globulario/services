package main

import (
	"context"
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_controller/resourcestore"
	"github.com/globulario/services/golang/plan/planpb"
	"google.golang.org/protobuf/proto"
)

func newTestServerWithNode(t *testing.T, kv *mapKV, ps *fakePlanStore) *server {
	t.Helper()
	return &server{
		cfg: &clusterControllerConfig{},
		state: &controllerState{Nodes: map[string]*nodeState{
			"n1": {
				NodeID:       "n1",
				Capabilities: &storedCapabilities{CanApplyPrivileged: true},
			},
		}},
		kv:              kv,
		planStore:        ps,
		resources:        resourcestore.NewMemStore(),
		planSignerState: testPlanSigner(t),
	}
}

func desiredNetworkForTests() *cluster_controllerpb.DesiredNetwork {
	return &cluster_controllerpb.DesiredNetwork{
		Domain:   "example.com",
		Protocol: "http",
		PortHttp: 80,
	}
}

func applyDesiredForTests(t *testing.T, srv *server, net *cluster_controllerpb.DesiredNetwork, services map[string]string) {
	t.Helper()
	ctx := context.Background()
	if net != nil {
		_, err := srv.resources.Apply(ctx, "ClusterNetwork", &cluster_controllerpb.ClusterNetwork{
			Meta: &cluster_controllerpb.ObjectMeta{Name: "default", Generation: 1},
			Spec: &cluster_controllerpb.ClusterNetworkSpec{
				ClusterDomain:    net.GetDomain(),
				Protocol:         net.GetProtocol(),
				PortHttp:         net.GetPortHttp(),
				PortHttps:        net.GetPortHttps(),
				AlternateDomains: append([]string(nil), net.GetAlternateDomains()...),
				AcmeEnabled:      net.GetAcmeEnabled(),
				AdminEmail:       net.GetAdminEmail(),
			},
		})
		if err != nil {
			t.Fatalf("apply network: %v", err)
		}
	}
	for svc, ver := range services {
		canon := canonicalServiceName(svc)
		_, err := srv.resources.Apply(ctx, "ServiceDesiredVersion", &cluster_controllerpb.ServiceDesiredVersion{
			Meta: &cluster_controllerpb.ObjectMeta{Name: canon, Generation: 1},
			Spec: &cluster_controllerpb.ServiceDesiredVersionSpec{
				ServiceName: canon,
				Version:     ver,
			},
		})
		if err != nil {
			t.Fatalf("apply service: %v", err)
		}
	}
}

func TestReconcileDoesNotMarkAppliedOnEmit(t *testing.T) {
	kv := newMapKV()
	ps := &fakePlanStore{}
	srv := newTestServerWithNode(t, kv, ps)
	applyDesiredForTests(t, srv, desiredNetworkForTests(), nil)
	srv.reconcileNodes(context.Background())
	if ps.count != 1 {
		t.Fatalf("expected 1 plan emission, got %d", ps.count)
	}
	applied, err := srv.getNodeAppliedHash(context.Background(), "n1")
	if err != nil {
		t.Fatalf("get applied hash: %v", err)
	}
	if applied != "" {
		t.Fatalf("expected no applied hash after emit, got %s", applied)
	}
}

func TestReconcileDoesNotReemitWhileRunning(t *testing.T) {
	kv := newMapKV()
	ps := &fakePlanStore{}
	srv := newTestServerWithNode(t, kv, ps)
	net := desiredNetworkForTests()
	applyDesiredForTests(t, srv, net, nil)
	srv.reconcileNodes(context.Background())
	firstPlan := proto.Clone(ps.lastPlan).(*planpb.NodePlan)
	meta := &planMeta{PlanId: firstPlan.GetPlanId(), Generation: firstPlan.GetGeneration(), DesiredHash: mustHash(t, net), LastEmit: time.Now().UnixMilli()}
	if err := srv.putNodePlanMeta(context.Background(), "n1", meta); err != nil {
		t.Fatalf("put meta: %v", err)
	}
	ps.PutStatus(context.Background(), "n1", &planpb.NodePlanStatus{
		PlanId:     firstPlan.GetPlanId(),
		NodeId:     "n1",
		Generation: firstPlan.GetGeneration(),
		State:      planpb.PlanState_PLAN_RUNNING,
	})
	srv.reconcileNodes(context.Background())
	if ps.count != 1 {
		t.Fatalf("expected no additional plan while running, got %d", ps.count)
	}
}

func TestReconcileMarksAppliedOnSuccess(t *testing.T) {
	kv := newMapKV()
	ps := &fakePlanStore{}
	srv := newTestServerWithNode(t, kv, ps)
	net := desiredNetworkForTests()
	applyDesiredForTests(t, srv, net, nil)
	srv.reconcileNodes(context.Background())
	plan := ps.lastPlan
	hash := mustHash(t, net)
	meta := &planMeta{PlanId: plan.GetPlanId(), Generation: plan.GetGeneration(), DesiredHash: hash, LastEmit: time.Now().UnixMilli()}
	if err := srv.putNodePlanMeta(context.Background(), "n1", meta); err != nil {
		t.Fatalf("put meta: %v", err)
	}
	ps.PutStatus(context.Background(), "n1", &planpb.NodePlanStatus{
		PlanId:     plan.GetPlanId(),
		NodeId:     "n1",
		Generation: plan.GetGeneration(),
		State:      planpb.PlanState_PLAN_SUCCEEDED,
	})
	srv.reconcileNodes(context.Background())
	applied, err := srv.getNodeAppliedHash(context.Background(), "n1")
	if err != nil {
		t.Fatalf("get applied: %v", err)
	}
	if applied != hash {
		t.Fatalf("expected applied hash %s, got %s", hash, applied)
	}
}

func TestReconcileReemitsAfterFailure(t *testing.T) {
	kv := newMapKV()
	ps := &fakePlanStore{}
	srv := newTestServerWithNode(t, kv, ps)
	net := desiredNetworkForTests()
	applyDesiredForTests(t, srv, net, nil)
	srv.reconcileNodes(context.Background())
	firstPlan := ps.lastPlan
	hash := mustHash(t, net)
	meta := &planMeta{PlanId: firstPlan.GetPlanId(), Generation: firstPlan.GetGeneration(), DesiredHash: hash, LastEmit: time.Now().Add(-time.Minute).UnixMilli()}
	if err := srv.putNodePlanMeta(context.Background(), "n1", meta); err != nil {
		t.Fatalf("put meta: %v", err)
	}
	ps.PutStatus(context.Background(), "n1", &planpb.NodePlanStatus{
		PlanId:     firstPlan.GetPlanId(),
		NodeId:     "n1",
		Generation: firstPlan.GetGeneration(),
		State:      planpb.PlanState_PLAN_FAILED,
	})
	srv.reconcileNodes(context.Background())
	if ps.count < 2 {
		t.Fatalf("expected re-emit after failure, got %d", ps.count)
	}
}

func mustHash(t *testing.T, net *cluster_controllerpb.DesiredNetwork) string {
	t.Helper()
	h, err := hashDesiredNetwork(net)
	if err != nil {
		t.Fatalf("hashDesiredNetwork: %v", err)
	}
	return h
}

// seedCoreDesired pre-populates desired-state entries for all core-profile
// components so that infra materialization doesn't change the desired set.
// In production, seed/desired-set would do this; in tests we must do it explicitly.
func seedCoreDesired(t *testing.T, srv *server) {
	t.Helper()
	ctx := context.Background()
	for _, comp := range ComponentsForProfile("core") {
		if comp.Kind == KindInfrastructure {
			relName := defaultPublisherID() + "/" + comp.Name
			srv.resources.Apply(ctx, "InfrastructureRelease", &cluster_controllerpb.InfrastructureRelease{
				Meta: &cluster_controllerpb.ObjectMeta{Name: relName},
				Spec: &cluster_controllerpb.InfrastructureReleaseSpec{
					PublisherID: defaultPublisherID(),
					Component:   comp.Name,
					Version:     "0.0.1",
				},
				Status: &cluster_controllerpb.InfrastructureReleaseStatus{},
			})
		} else {
			srv.resources.Apply(ctx, "ServiceDesiredVersion", &cluster_controllerpb.ServiceDesiredVersion{
				Meta: &cluster_controllerpb.ObjectMeta{Name: comp.Name, Generation: 1},
				Spec: &cluster_controllerpb.ServiceDesiredVersionSpec{
					ServiceName: comp.Name,
					Version:     "0.0.1",
				},
			})
		}
	}
}

func TestServiceReconcileMarksAppliedOnSuccess(t *testing.T) {
	kv := newMapKV()
	ps := &fakePlanStore{}
	srv := newTestServerWithNode(t, kv, ps)
	srv.state.Nodes["n1"].Units = []unitStatusRecord{
		{Name: serviceUnitForCanonical("ldap")},
		{Name: "globular-event.service", State: "active"}, // runtime dep of ldap
	}
	// Seed all core-profile services so materialization doesn't change the desired set.
	seedCoreDesired(t, srv)
	applyDesiredForTests(t, srv, desiredNetworkForTests(), map[string]string{"globular-ldap.service": "1.2.3"})
	// Mark network converged so service reconcile can proceed.
	if err := srv.putNodeAppliedHash(context.Background(), "n1", mustHash(t, desiredNetworkForTests())); err != nil {
		t.Fatalf("putNodeAppliedHash: %v", err)
	}
	srv.reconcileNodes(context.Background())
	plan := ps.lastPlan
	if plan == nil {
		t.Fatalf("expected service plan emitted")
	}
	// Use the plan's desired hash (includes all seeded + desired services).
	svcHash := plan.GetDesiredHash()
	if svcHash == "" {
		t.Fatalf("plan desired_hash is empty")
	}
	ps.PutStatus(context.Background(), "n1", &planpb.NodePlanStatus{
		PlanId:     plan.GetPlanId(),
		NodeId:     "n1",
		Generation: plan.GetGeneration(),
		State:      planpb.PlanState_PLAN_SUCCEEDED,
	})
	// Simulate what the node agent does after a successful install:
	// report the installed version so the reconciler knows it converged.
	// Must include ALL desired services at the correct versions for the hash to match.
	installed := make(map[string]string)
	desiredCanon, _, _ := srv.loadDesiredServices(context.Background())
	for svc, ver := range desiredCanon {
		installed[svc] = ver
	}
	srv.state.Nodes["n1"].InstalledVersions = installed
	srv.reconcileNodes(context.Background())
	appliedSvc, err := srv.getNodeAppliedServiceHash(context.Background(), "n1")
	if err != nil {
		t.Fatalf("get applied svc hash: %v", err)
	}
	if appliedSvc != svcHash {
		t.Fatalf("expected applied service hash %s, got %s", svcHash, appliedSvc)
	}
}

func TestServiceReconcileDoesNotReemitWhileRunning(t *testing.T) {
	kv := newMapKV()
	ps := &fakePlanStore{}
	srv := newTestServerWithNode(t, kv, ps)
	srv.state.Nodes["n1"].Units = []unitStatusRecord{
		{Name: serviceUnitForCanonical("ldap")},
		{Name: "globular-event.service", State: "active"},
	}
	seedCoreDesired(t, srv)
	net := desiredNetworkForTests()
	applyDesiredForTests(t, srv, net, map[string]string{
		"globular-ldap.service": "1.2.3",
	})
	if err := srv.putNodeAppliedHash(context.Background(), "n1", mustHash(t, net)); err != nil {
		t.Fatalf("putNodeAppliedHash: %v", err)
	}
	srv.reconcileNodes(context.Background())
	firstPlan := ps.lastPlan
	if firstPlan == nil {
		t.Fatalf("expected first plan emitted")
	}
	svcHash := firstPlan.GetDesiredHash()
	ps.PutStatus(context.Background(), "n1", &planpb.NodePlanStatus{
		PlanId:     firstPlan.GetPlanId(),
		NodeId:     "n1",
		Generation: firstPlan.GetGeneration(),
		State:      planpb.PlanState_PLAN_RUNNING,
	})
	srv.reconcileNodes(context.Background())
	if ps.count != 1 {
		t.Fatalf("expected no re-emit while running, got %d", ps.count)
	}
	if svcHash == "" {
		t.Fatalf("expected desired hash set on plan")
	}
}

// ---------------------------------------------------------------------------
// Day 1 infra materialization tests
// ---------------------------------------------------------------------------

func TestMaterializeInfra_WorkloadRequiresMissingInfra(t *testing.T) {
	// Scenario: ai-memory is desired, but scylladb has no desired-state entry.
	// The controller should auto-create the scylladb InfrastructureRelease.
	kv := newMapKV()
	ps := &fakePlanStore{}
	srv := &server{
		cfg: &clusterControllerConfig{},
		state: &controllerState{Nodes: map[string]*nodeState{
			"n1": {
				NodeID:         "n1",
				Profiles:       []string{"database"},
				BootstrapPhase: BootstrapWorkloadReady,
				Capabilities:   &storedCapabilities{CanApplyPrivileged: true},
				Units: []unitStatusRecord{
					{Name: "scylla-server.service", State: "active"},
					{Name: "globular-event.service", State: "active"},
				},
			},
		}},
		kv:              kv,
		planStore:       ps,
		resources:       resourcestore.NewMemStore(),
		planSignerState: testPlanSigner(t),
	}

	ctx := context.Background()
	applyDesiredForTests(t, srv, desiredNetworkForTests(), map[string]string{
		"globular-ai-memory.service": "0.0.1",
	})
	if err := srv.putNodeAppliedHash(ctx, "n1", mustHash(t, desiredNetworkForTests())); err != nil {
		t.Fatalf("putNodeAppliedHash: %v", err)
	}

	// Verify scylladb not in desired state before reconcile.
	obj, _, _ := srv.resources.Get(ctx, "InfrastructureRelease", defaultPublisherID()+"/scylladb")
	if obj != nil {
		t.Fatal("scylladb InfrastructureRelease should not exist before reconcile")
	}

	srv.reconcileNodes(ctx)

	// After reconcile, scylladb should be auto-materialized.
	obj, _, _ = srv.resources.Get(ctx, "InfrastructureRelease", defaultPublisherID()+"/scylladb")
	if obj == nil {
		t.Fatal("expected scylladb InfrastructureRelease to be auto-created")
	}
	rel := obj.(*cluster_controllerpb.InfrastructureRelease)
	if rel.Spec.Component != "scylladb" {
		t.Errorf("expected component=scylladb, got %q", rel.Spec.Component)
	}

	// Check that the intent records the materialization.
	node := srv.state.Nodes["n1"]
	if node.ResolvedIntent == nil {
		t.Fatal("expected resolved intent")
	}
	if len(node.ResolvedIntent.MaterializedDesired) == 0 {
		t.Fatal("expected materialized desired entries")
	}
}

func TestMaterializeInfra_Idempotent(t *testing.T) {
	kv := newMapKV()
	ps := &fakePlanStore{}
	srv := &server{
		cfg: &clusterControllerConfig{},
		state: &controllerState{Nodes: map[string]*nodeState{
			"n1": {
				NodeID:         "n1",
				Profiles:       []string{"database"},
				BootstrapPhase: BootstrapWorkloadReady,
				Capabilities:   &storedCapabilities{CanApplyPrivileged: true},
				Units: []unitStatusRecord{
					{Name: "scylla-server.service", State: "active"},
					{Name: "globular-event.service", State: "active"},
				},
			},
		}},
		kv:              kv,
		planStore:       ps,
		resources:       resourcestore.NewMemStore(),
		planSignerState: testPlanSigner(t),
	}

	ctx := context.Background()
	applyDesiredForTests(t, srv, desiredNetworkForTests(), map[string]string{
		"globular-ai-memory.service": "0.0.1",
	})
	if err := srv.putNodeAppliedHash(ctx, "n1", mustHash(t, desiredNetworkForTests())); err != nil {
		t.Fatalf("putNodeAppliedHash: %v", err)
	}

	// Run reconcile twice — second run should NOT create duplicates.
	srv.reconcileNodes(ctx)
	srv.reconcileNodes(ctx)

	items, _, _ := srv.resources.List(ctx, "InfrastructureRelease", "")
	scyllaCount := 0
	for _, obj := range items {
		if rel, ok := obj.(*cluster_controllerpb.InfrastructureRelease); ok && rel.Spec != nil && rel.Spec.Component == "scylladb" {
			scyllaCount++
		}
	}
	if scyllaCount != 1 {
		t.Errorf("expected exactly 1 scylladb InfrastructureRelease, got %d", scyllaCount)
	}
}

func TestMaterializeInfra_UsesInstalledVersion(t *testing.T) {
	kv := newMapKV()
	ps := &fakePlanStore{}
	srv := &server{
		cfg: &clusterControllerConfig{},
		state: &controllerState{Nodes: map[string]*nodeState{
			"n1": {
				NodeID:         "n1",
				Profiles:       []string{"database"},
				BootstrapPhase: BootstrapWorkloadReady,
				Capabilities:   &storedCapabilities{CanApplyPrivileged: true},
				Units: []unitStatusRecord{
					{Name: "scylla-server.service", State: "active"},
					{Name: "globular-event.service", State: "active"},
				},
				InstalledVersions: map[string]string{"scylladb": "5.4.8"},
			},
		}},
		kv:              kv,
		planStore:       ps,
		resources:       resourcestore.NewMemStore(),
		planSignerState: testPlanSigner(t),
	}

	ctx := context.Background()
	applyDesiredForTests(t, srv, desiredNetworkForTests(), map[string]string{
		"globular-ai-memory.service": "0.0.1",
	})
	if err := srv.putNodeAppliedHash(ctx, "n1", mustHash(t, desiredNetworkForTests())); err != nil {
		t.Fatalf("putNodeAppliedHash: %v", err)
	}

	srv.reconcileNodes(ctx)

	obj, _, _ := srv.resources.Get(ctx, "InfrastructureRelease", defaultPublisherID()+"/scylladb")
	if obj == nil {
		t.Fatal("expected scylladb to be materialized")
	}
	rel := obj.(*cluster_controllerpb.InfrastructureRelease)
	if rel.Spec.Version != "5.4.8" {
		t.Errorf("expected version 5.4.8 from installed, got %q", rel.Spec.Version)
	}
}

func TestMaterializeInfra_WorkloadBlockedUntilInfraHealthy(t *testing.T) {
	kv := newMapKV()
	ps := &fakePlanStore{}
	srv := &server{
		cfg: &clusterControllerConfig{},
		state: &controllerState{Nodes: map[string]*nodeState{
			"n1": {
				NodeID:         "n1",
				Profiles:       []string{"database"},
				BootstrapPhase: BootstrapWorkloadReady,
				Capabilities:   &storedCapabilities{CanApplyPrivileged: true},
				Units: []unitStatusRecord{
					{Name: "globular-event.service", State: "active"},
				},
			},
		}},
		kv:              kv,
		planStore:       ps,
		resources:       resourcestore.NewMemStore(),
		planSignerState: testPlanSigner(t),
	}

	ctx := context.Background()
	applyDesiredForTests(t, srv, desiredNetworkForTests(), map[string]string{
		"globular-ai-memory.service": "0.0.1",
	})
	if err := srv.putNodeAppliedHash(ctx, "n1", mustHash(t, desiredNetworkForTests())); err != nil {
		t.Fatalf("putNodeAppliedHash: %v", err)
	}

	srv.reconcileNodes(ctx)

	node := srv.state.Nodes["n1"]
	if node.Day1Phase != Day1WorkloadBlocked && node.Day1Phase != Day1InfraPlanned && node.Day1Phase != Day1InfraInstalled {
		t.Errorf("expected workload_blocked/infra_planned/infra_installed, got %q (%s)", node.Day1Phase, node.Day1PhaseReason)
	}
}

func TestMaterializeInfra_ProfileImpliesInfra(t *testing.T) {
	kv := newMapKV()
	ps := &fakePlanStore{}
	srv := &server{
		cfg: &clusterControllerConfig{},
		state: &controllerState{Nodes: map[string]*nodeState{
			"n1": {
				NodeID:         "n1",
				Profiles:       []string{"storage"},
				BootstrapPhase: BootstrapWorkloadReady,
				Capabilities:   &storedCapabilities{CanApplyPrivileged: true},
			},
		}},
		kv:              kv,
		planStore:       ps,
		resources:       resourcestore.NewMemStore(),
		planSignerState: testPlanSigner(t),
	}

	ctx := context.Background()
	applyDesiredForTests(t, srv, desiredNetworkForTests(), map[string]string{})
	if err := srv.putNodeAppliedHash(ctx, "n1", mustHash(t, desiredNetworkForTests())); err != nil {
		t.Fatalf("putNodeAppliedHash: %v", err)
	}

	srv.reconcileNodes(ctx)

	obj, _, _ := srv.resources.Get(ctx, "InfrastructureRelease", defaultPublisherID()+"/minio")
	if obj == nil {
		t.Fatal("expected minio InfrastructureRelease to be auto-created for storage profile")
	}
}

func TestServiceReconcileReemitsAfterFailure(t *testing.T) {
	kv := newMapKV()
	ps := &fakePlanStore{}
	srv := newTestServerWithNode(t, kv, ps)
	srv.state.Nodes["n1"].Units = []unitStatusRecord{
		{Name: serviceUnitForCanonical("ldap")},
		{Name: "globular-event.service", State: "active"},
	}
	net := desiredNetworkForTests()
	applyDesiredForTests(t, srv, net, map[string]string{
		"globular-ldap.service": "1.2.3",
	})
	if err := srv.putNodeAppliedHash(context.Background(), "n1", mustHash(t, net)); err != nil {
		t.Fatalf("putNodeAppliedHash: %v", err)
	}
	srv.reconcileNodes(context.Background())
	firstPlan := ps.lastPlan
	ps.PutStatus(context.Background(), "n1", &planpb.NodePlanStatus{
		PlanId:     firstPlan.GetPlanId(),
		NodeId:     "n1",
		Generation: firstPlan.GetGeneration(),
		State:      planpb.PlanState_PLAN_FAILED,
	})
	srv.reconcileNodes(context.Background())
	if ps.count < 2 {
		t.Fatalf("expected re-emit after failure, got %d", ps.count)
	}
}
