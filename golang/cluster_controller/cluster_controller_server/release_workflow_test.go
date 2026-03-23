package main

import (
	"context"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_controller/resourcestore"
	"github.com/globulario/services/golang/plan/planpb"
	"google.golang.org/protobuf/proto"
)

// multiNodePlanStore supports per-node plan statuses for workflow tests.
type multiNodePlanStore struct {
	plans    map[string]*planpb.NodePlan
	statuses map[string]*planpb.NodePlanStatus
}

func newMultiNodePlanStore() *multiNodePlanStore {
	return &multiNodePlanStore{
		plans:    make(map[string]*planpb.NodePlan),
		statuses: make(map[string]*planpb.NodePlanStatus),
	}
}

func (m *multiNodePlanStore) PutCurrentPlan(_ context.Context, nodeID string, plan *planpb.NodePlan) error {
	m.plans[nodeID] = proto.Clone(plan).(*planpb.NodePlan)
	return nil
}
func (m *multiNodePlanStore) GetCurrentPlan(_ context.Context, nodeID string) (*planpb.NodePlan, error) {
	return m.plans[nodeID], nil
}
func (m *multiNodePlanStore) PutStatus(_ context.Context, nodeID string, s *planpb.NodePlanStatus) error {
	m.statuses[nodeID] = s
	return nil
}
func (m *multiNodePlanStore) GetStatus(_ context.Context, nodeID string) (*planpb.NodePlanStatus, error) {
	return m.statuses[nodeID], nil
}
func (m *multiNodePlanStore) AppendHistory(context.Context, string, *planpb.NodePlan) error {
	return nil
}

// ── Phase 1: Removal workflow semantics ──────────────────────────────────────

func TestRemovalWorkflow_HappyPath(t *testing.T) {
	ps := &fakePlanStore{}
	srv := &server{
		cfg:   &clusterControllerConfig{},
		state: &controllerState{Nodes: map[string]*nodeState{"n1": {NodeID: "n1"}}},
		planStore:       ps,
		resources:       resourcestore.NewMemStore(),
		planSignerState: testPlanSigner(t),
	}
	srv.resources.Apply(context.Background(), "ServiceRelease", &cluster_controllerpb.ServiceRelease{
		Meta: &cluster_controllerpb.ObjectMeta{Name: "pub/svc", Generation: 1},
		Spec: &cluster_controllerpb.ServiceReleaseSpec{
			PublisherID: "pub", ServiceName: "svc", Version: "1.0.0", Removing: true,
		},
		Status: &cluster_controllerpb.ServiceReleaseStatus{
			Phase: cluster_controllerpb.ReleasePhaseAvailable, ObservedGeneration: 1,
			Nodes: []*cluster_controllerpb.NodeReleaseStatus{{NodeID: "n1", Phase: cluster_controllerpb.ReleasePhaseAvailable}},
		},
	})

	// First reconcile: dispatch uninstall plans → REMOVING.
	srv.reconcileRelease(context.Background(), "pub/svc")
	obj, _, _ := srv.resources.Get(context.Background(), "ServiceRelease", "pub/svc")
	rel := obj.(*cluster_controllerpb.ServiceRelease)
	if rel.Status.Phase != ReleasePhaseRemoving {
		t.Fatalf("expected REMOVING, got %s", rel.Status.Phase)
	}
	if rel.Status.WorkflowKind != "remove" {
		t.Fatalf("expected workflow_kind=remove, got %s", rel.Status.WorkflowKind)
	}
	if ps.lastPlan == nil || ps.lastPlan.GetReason() != "service_remove" {
		t.Fatalf("expected service_remove plan")
	}

	// Simulate plan success.
	ps.PutStatus(context.Background(), "n1", &planpb.NodePlanStatus{
		PlanId: ps.lastPlan.GetPlanId(), NodeId: "n1",
		Generation: ps.lastPlan.GetGeneration(),
		State:      planpb.PlanState_PLAN_SUCCEEDED,
	})

	// Second reconcile: all succeeded → REMOVED.
	srv.reconcileRelease(context.Background(), "pub/svc")
	obj, _, _ = srv.resources.Get(context.Background(), "ServiceRelease", "pub/svc")
	rel = obj.(*cluster_controllerpb.ServiceRelease)
	if rel.Status.Phase != ReleasePhaseRemoved {
		t.Fatalf("expected REMOVED, got %s", rel.Status.Phase)
	}
	if rel.Status.TransitionReason != "all_nodes_removed" {
		t.Fatalf("expected transition_reason=all_nodes_removed, got %s", rel.Status.TransitionReason)
	}

	// Third reconcile: garbage-collect.
	srv.reconcileRelease(context.Background(), "pub/svc")
	obj, _, _ = srv.resources.Get(context.Background(), "ServiceRelease", "pub/svc")
	if obj != nil {
		t.Fatalf("expected release to be garbage-collected")
	}
}

func TestRemovalWorkflow_PartialFailure(t *testing.T) {
	ps := &fakePlanStore{}
	srv := &server{
		cfg:   &clusterControllerConfig{},
		state: &controllerState{Nodes: map[string]*nodeState{"n1": {NodeID: "n1"}}},
		planStore:       ps,
		resources:       resourcestore.NewMemStore(),
		planSignerState: testPlanSigner(t),
	}
	srv.resources.Apply(context.Background(), "ServiceRelease", &cluster_controllerpb.ServiceRelease{
		Meta: &cluster_controllerpb.ObjectMeta{Name: "pub/svc", Generation: 1},
		Spec: &cluster_controllerpb.ServiceReleaseSpec{
			PublisherID: "pub", ServiceName: "svc", Version: "1.0.0", Removing: true,
		},
		Status: &cluster_controllerpb.ServiceReleaseStatus{Phase: cluster_controllerpb.ReleasePhaseAvailable, ObservedGeneration: 1},
	})

	srv.reconcileRelease(context.Background(), "pub/svc")

	// Simulate plan failure.
	ps.PutStatus(context.Background(), "n1", &planpb.NodePlanStatus{
		PlanId: ps.lastPlan.GetPlanId(), NodeId: "n1",
		Generation: ps.lastPlan.GetGeneration(),
		State:      planpb.PlanState_PLAN_FAILED,
		ErrorMessage: "unit still active",
	})

	srv.reconcileRelease(context.Background(), "pub/svc")
	obj, _, _ := srv.resources.Get(context.Background(), "ServiceRelease", "pub/svc")
	rel := obj.(*cluster_controllerpb.ServiceRelease)
	if rel.Status.Phase != cluster_controllerpb.ReleasePhaseFailed {
		t.Fatalf("expected FAILED, got %s", rel.Status.Phase)
	}
	if rel.Status.TransitionReason != "removal_failed" {
		t.Fatalf("expected transition_reason=removal_failed, got %s", rel.Status.TransitionReason)
	}
}

// ── Phase 2: Rollback and degraded logic ─────────────────────────────────────

func TestRolledBack_AllNodes(t *testing.T) {
	ps := newMultiNodePlanStore()
	srv := &server{
		cfg:       &clusterControllerConfig{},
		state:     &controllerState{Nodes: map[string]*nodeState{"n1": {NodeID: "n1"}, "n2": {NodeID: "n2"}}},
		planStore: ps,
		resources: resourcestore.NewMemStore(),
	}
	nodes := []*cluster_controllerpb.NodeReleaseStatus{
		{NodeID: "n1", PlanID: "p1", Phase: cluster_controllerpb.ReleasePhaseApplying},
		{NodeID: "n2", PlanID: "p2", Phase: cluster_controllerpb.ReleasePhaseApplying},
	}
	ps.statuses = map[string]*planpb.NodePlanStatus{
		"n1": {PlanId: "p1", State: planpb.PlanState_PLAN_ROLLED_BACK, ErrorMessage: "check failed", ErrorStepId: "step-3"},
		"n2": {PlanId: "p2", State: planpb.PlanState_PLAN_ROLLED_BACK, ErrorMessage: "probe failed"},
	}

	updated, succeeded, failed, rolledBack, running := srv.checkNodePlanStatuses(context.Background(), nodes, "1.0.0")
	if succeeded != 0 || failed != 0 || rolledBack != 2 || running != 0 {
		t.Fatalf("expected 0/0/2/0, got s=%d f=%d rb=%d r=%d", succeeded, failed, rolledBack, running)
	}
	// Verify node-level phases.
	for _, u := range updated {
		if u.Phase != cluster_controllerpb.ReleasePhaseRolledBack {
			t.Fatalf("node %s: expected ROLLED_BACK, got %s", u.NodeID, u.Phase)
		}
	}
	// Verify FailedStepID propagation.
	if updated[0].FailedStepID != "step-3" {
		t.Fatalf("expected failed_step_id=step-3, got %s", updated[0].FailedStepID)
	}
}

func TestRolledBack_Mixed(t *testing.T) {
	// Node A success, Node B rollback → DEGRADED
	ps := newMultiNodePlanStore()
	srv := &server{
		cfg:       &clusterControllerConfig{},
		state:     &controllerState{Nodes: map[string]*nodeState{"n1": {NodeID: "n1"}, "n2": {NodeID: "n2"}}},
		planStore: ps,
		resources: resourcestore.NewMemStore(),
	}
	nodes := []*cluster_controllerpb.NodeReleaseStatus{
		{NodeID: "n1", PlanID: "p1"},
		{NodeID: "n2", PlanID: "p2"},
	}
	ps.statuses = map[string]*planpb.NodePlanStatus{
		"n1": {PlanId: "p1", State: planpb.PlanState_PLAN_SUCCEEDED},
		"n2": {PlanId: "p2", State: planpb.PlanState_PLAN_ROLLED_BACK},
	}
	_, succeeded, failed, rolledBack, running := srv.checkNodePlanStatuses(context.Background(), nodes, "1.0.0")
	if succeeded != 1 || failed != 0 || rolledBack != 1 || running != 0 {
		t.Fatalf("expected 1/0/1/0, got s=%d f=%d rb=%d r=%d", succeeded, failed, rolledBack, running)
	}
}

func TestFailedStepPropagation(t *testing.T) {
	ps := newMultiNodePlanStore()
	srv := &server{
		cfg:       &clusterControllerConfig{},
		state:     &controllerState{Nodes: map[string]*nodeState{"n1": {NodeID: "n1"}}},
		planStore: ps,
		resources: resourcestore.NewMemStore(),
	}
	nodes := []*cluster_controllerpb.NodeReleaseStatus{
		{NodeID: "n1", PlanID: "p1"},
	}
	ps.statuses = map[string]*planpb.NodePlanStatus{
		"n1": {PlanId: "p1", State: planpb.PlanState_PLAN_FAILED, ErrorStepId: "artifact.fetch", ErrorMessage: "download timeout"},
	}
	updated, _, _, _, _ := srv.checkNodePlanStatuses(context.Background(), nodes, "1.0.0")
	if len(updated) != 1 {
		t.Fatalf("expected 1 node status")
	}
	if updated[0].FailedStepID != "artifact.fetch" {
		t.Fatalf("expected failed_step_id=artifact.fetch, got %s", updated[0].FailedStepID)
	}
	if updated[0].ErrorMessage != "download timeout" {
		t.Fatalf("expected error=download timeout, got %s", updated[0].ErrorMessage)
	}
}

// ── Phase 3: Transition enforcement ──────────────────────────────────────────

func TestAdvancePhase_InvalidTransition_ReturnsError(t *testing.T) {
	cases := []struct {
		from, to string
	}{
		{cluster_controllerpb.ReleasePhaseAvailable, cluster_controllerpb.ReleasePhaseApplying},
		{cluster_controllerpb.ReleasePhaseFailed, cluster_controllerpb.ReleasePhaseAvailable},
		{ReleasePhaseRemoved, cluster_controllerpb.ReleasePhasePending},
		{ReleasePhaseRemoved, ReleasePhaseRemoving},
		{cluster_controllerpb.ReleasePhaseRolledBack, cluster_controllerpb.ReleasePhaseAvailable},
	}
	for _, tc := range cases {
		if err := advancePhase(tc.from, tc.to); err == nil {
			t.Errorf("expected error for %s → %s", tc.from, tc.to)
		}
	}
}

func TestAdvancePhase_ValidTransition_NoError(t *testing.T) {
	cases := []struct {
		from, to string
	}{
		{"", cluster_controllerpb.ReleasePhasePending},
		{"", cluster_controllerpb.ReleasePhaseFailed},
		{cluster_controllerpb.ReleasePhasePending, cluster_controllerpb.ReleasePhaseResolved},
		{cluster_controllerpb.ReleasePhaseResolved, cluster_controllerpb.ReleasePhaseApplying},
		{cluster_controllerpb.ReleasePhaseApplying, cluster_controllerpb.ReleasePhaseAvailable},
		{cluster_controllerpb.ReleasePhaseApplying, cluster_controllerpb.ReleasePhaseRolledBack},
		{cluster_controllerpb.ReleasePhaseAvailable, cluster_controllerpb.ReleasePhasePending},
		{cluster_controllerpb.ReleasePhaseAvailable, cluster_controllerpb.ReleasePhaseDegraded},
		{cluster_controllerpb.ReleasePhaseAvailable, cluster_controllerpb.ReleasePhaseFailed},
		{cluster_controllerpb.ReleasePhaseFailed, cluster_controllerpb.ReleasePhasePending},
		{cluster_controllerpb.ReleasePhaseRolledBack, cluster_controllerpb.ReleasePhasePending},
		// Removal transitions
		{cluster_controllerpb.ReleasePhaseAvailable, ReleasePhaseRemoving},
		{cluster_controllerpb.ReleasePhaseFailed, ReleasePhaseRemoving},
		{ReleasePhaseRemoving, ReleasePhaseRemoved},
		{ReleasePhaseRemoving, cluster_controllerpb.ReleasePhaseFailed},
	}
	for _, tc := range cases {
		if err := advancePhase(tc.from, tc.to); err != nil {
			t.Errorf("unexpected error for %s → %s: %v", tc.from, tc.to, err)
		}
	}
}

func TestAdvancePhase_NoOp_Allowed(t *testing.T) {
	phases := []string{
		"", cluster_controllerpb.ReleasePhasePending,
		cluster_controllerpb.ReleasePhaseApplying,
		cluster_controllerpb.ReleasePhaseAvailable,
		ReleasePhaseRemoving, ReleasePhaseRemoved,
	}
	for _, p := range phases {
		if err := advancePhase(p, p); err != nil {
			t.Errorf("no-op %s → %s should be allowed: %v", p, p, err)
		}
	}
}

func TestTerminalPhases_NoOutgoing(t *testing.T) {
	// REMOVED is terminal — nothing should be allowed.
	targets := []string{
		cluster_controllerpb.ReleasePhasePending,
		cluster_controllerpb.ReleasePhaseResolved,
		cluster_controllerpb.ReleasePhaseApplying,
		cluster_controllerpb.ReleasePhaseAvailable,
		ReleasePhaseRemoving,
	}
	for _, target := range targets {
		if err := advancePhase(ReleasePhaseRemoved, target); err == nil {
			t.Errorf("REMOVED → %s should be blocked", target)
		}
	}
}

// ── Phase 4: Workflow status integrity ───────────────────────────────────────

func TestWorkflowKindInstall(t *testing.T) {
	h := &releaseHandle{Removing: false, Nodes: nil}
	if k := computeWorkflowKind(h); k != "install" {
		t.Fatalf("expected install, got %s", k)
	}
}

func TestWorkflowKindUpgrade(t *testing.T) {
	h := &releaseHandle{
		Removing: false,
		Nodes:    []*cluster_controllerpb.NodeReleaseStatus{{NodeID: "n1", InstalledVersion: "0.9.0"}},
	}
	if k := computeWorkflowKind(h); k != "upgrade" {
		t.Fatalf("expected upgrade, got %s", k)
	}
}

func TestWorkflowKindRemove(t *testing.T) {
	h := &releaseHandle{Removing: true}
	if k := computeWorkflowKind(h); k != "remove" {
		t.Fatalf("expected remove, got %s", k)
	}
}

func TestStartedAtUnixMs_SetOnce(t *testing.T) {
	s := &cluster_controllerpb.ServiceReleaseStatus{StartedAtUnixMs: 1000}
	applyPatchToSvcStatus(s, statusPatch{
		Phase:           cluster_controllerpb.ReleasePhaseApplying,
		StartedAtUnixMs: 2000,
		SetFields:       "phase",
	})
	if s.StartedAtUnixMs != 1000 {
		t.Fatalf("StartedAtUnixMs should not be overwritten, got %d", s.StartedAtUnixMs)
	}
}

func TestStartedAtUnixMs_SetWhenZero(t *testing.T) {
	s := &cluster_controllerpb.ServiceReleaseStatus{}
	applyPatchToSvcStatus(s, statusPatch{
		Phase:           cluster_controllerpb.ReleasePhaseResolved,
		StartedAtUnixMs: 5000,
		SetFields:       "phase",
	})
	if s.StartedAtUnixMs != 5000 {
		t.Fatalf("StartedAtUnixMs should be set when zero, got %d", s.StartedAtUnixMs)
	}
}

func TestTransitionReason_AlwaysSet(t *testing.T) {
	s := &cluster_controllerpb.ServiceReleaseStatus{}
	applyPatchToSvcStatus(s, statusPatch{
		Phase:            cluster_controllerpb.ReleasePhaseResolved,
		TransitionReason: "resolved",
		SetFields:        "phase",
	})
	if s.TransitionReason != "resolved" {
		t.Fatalf("expected TransitionReason=resolved, got %s", s.TransitionReason)
	}
	// Update reason on next transition.
	applyPatchToSvcStatus(s, statusPatch{
		Phase:            cluster_controllerpb.ReleasePhaseApplying,
		TransitionReason: "plans_dispatched",
		SetFields:        "phase",
	})
	if s.TransitionReason != "plans_dispatched" {
		t.Fatalf("expected TransitionReason=plans_dispatched, got %s", s.TransitionReason)
	}
}

// ── Phase 5: PLANNED documentation ──────────────────────────────────────────

func TestPlanned_NotInTransitionMap(t *testing.T) {
	// PLANNED should not appear as a source phase in the transition map.
	if _, ok := validPhaseTransitions[cluster_controllerpb.ReleasePhasePlanned]; ok {
		t.Fatalf("PLANNED should not be in the transition map (reserved for future use)")
	}
	// No phase should be able to transition TO PLANNED either.
	for from, targets := range validPhaseTransitions {
		if targets[cluster_controllerpb.ReleasePhasePlanned] {
			t.Fatalf("phase %q allows transition to PLANNED, which should not be reachable", from)
		}
	}
}

// ── Phase 6: End-to-end lifecycle (pipeline-level) ───────────────────────────

func TestApplyingPhase_AllSucceeded_Available(t *testing.T) {
	ps := &multiNodePlanStore{plans: map[string]*planpb.NodePlan{}, statuses: map[string]*planpb.NodePlanStatus{
		"n1": {PlanId: "p1", State: planpb.PlanState_PLAN_SUCCEEDED},
	}}
	srv := &server{cfg: &clusterControllerConfig{}, state: &controllerState{}, planStore: ps, resources: resourcestore.NewMemStore()}
	h := &releaseHandle{
		Name: "test", ResourceType: "ServiceRelease", Phase: cluster_controllerpb.ReleasePhaseApplying,
		ResolvedVersion: "1.0.0",
		Nodes:           []*cluster_controllerpb.NodeReleaseStatus{{NodeID: "n1", PlanID: "p1"}},
		PatchStatus: func(ctx context.Context, p statusPatch) error {
			if p.Phase != cluster_controllerpb.ReleasePhaseAvailable {
				t.Fatalf("expected AVAILABLE, got %s", p.Phase)
			}
			if p.TransitionReason != "all_nodes_succeeded" {
				t.Fatalf("expected reason=all_nodes_succeeded, got %s", p.TransitionReason)
			}
			return nil
		},
	}
	srv.reconcileApplying(context.Background(), h)
}

func TestApplyingPhase_AllFailed_Failed(t *testing.T) {
	ps := &multiNodePlanStore{plans: map[string]*planpb.NodePlan{}, statuses: map[string]*planpb.NodePlanStatus{
		"n1": {PlanId: "p1", State: planpb.PlanState_PLAN_FAILED, ErrorStepId: "artifact.verify"},
	}}
	srv := &server{cfg: &clusterControllerConfig{}, state: &controllerState{}, planStore: ps, resources: resourcestore.NewMemStore()}
	h := &releaseHandle{
		Name: "test", ResourceType: "ServiceRelease", Phase: cluster_controllerpb.ReleasePhaseApplying,
		Nodes: []*cluster_controllerpb.NodeReleaseStatus{{NodeID: "n1", PlanID: "p1"}},
		PatchStatus: func(ctx context.Context, p statusPatch) error {
			if p.Phase != cluster_controllerpb.ReleasePhaseFailed {
				t.Fatalf("expected FAILED, got %s", p.Phase)
			}
			if p.TransitionReason != "all_nodes_failed" {
				t.Fatalf("expected reason=all_nodes_failed, got %s", p.TransitionReason)
			}
			// Verify FailedStepID propagated to node status.
			for _, n := range p.Nodes {
				if n.FailedStepID != "artifact.verify" {
					t.Fatalf("expected failed_step=artifact.verify, got %s", n.FailedStepID)
				}
			}
			return nil
		},
	}
	srv.reconcileApplying(context.Background(), h)
}

func TestApplyingPhase_MixedSuccessFailure_Degraded(t *testing.T) {
	ps := &multiNodePlanStore{plans: map[string]*planpb.NodePlan{}, statuses: map[string]*planpb.NodePlanStatus{
		"n1": {PlanId: "p1", State: planpb.PlanState_PLAN_SUCCEEDED},
		"n2": {PlanId: "p2", State: planpb.PlanState_PLAN_FAILED},
	}}
	srv := &server{cfg: &clusterControllerConfig{}, state: &controllerState{}, planStore: ps, resources: resourcestore.NewMemStore()}
	h := &releaseHandle{
		Name: "test", ResourceType: "ServiceRelease", Phase: cluster_controllerpb.ReleasePhaseApplying,
		Nodes: []*cluster_controllerpb.NodeReleaseStatus{
			{NodeID: "n1", PlanID: "p1"},
			{NodeID: "n2", PlanID: "p2"},
		},
		PatchStatus: func(ctx context.Context, p statusPatch) error {
			if p.Phase != cluster_controllerpb.ReleasePhaseDegraded {
				t.Fatalf("expected DEGRADED, got %s", p.Phase)
			}
			return nil
		},
	}
	srv.reconcileApplying(context.Background(), h)
}

func TestApplyingPhase_MixedSuccessRollback_Degraded(t *testing.T) {
	ps := &multiNodePlanStore{plans: map[string]*planpb.NodePlan{}, statuses: map[string]*planpb.NodePlanStatus{
		"n1": {PlanId: "p1", State: planpb.PlanState_PLAN_SUCCEEDED},
		"n2": {PlanId: "p2", State: planpb.PlanState_PLAN_ROLLED_BACK},
	}}
	srv := &server{cfg: &clusterControllerConfig{}, state: &controllerState{}, planStore: ps, resources: resourcestore.NewMemStore()}
	h := &releaseHandle{
		Name: "test", ResourceType: "ServiceRelease", Phase: cluster_controllerpb.ReleasePhaseApplying,
		Nodes: []*cluster_controllerpb.NodeReleaseStatus{
			{NodeID: "n1", PlanID: "p1"},
			{NodeID: "n2", PlanID: "p2"},
		},
		PatchStatus: func(ctx context.Context, p statusPatch) error {
			if p.Phase != cluster_controllerpb.ReleasePhaseDegraded {
				t.Fatalf("expected DEGRADED for mixed success+rollback, got %s", p.Phase)
			}
			if p.TransitionReason != "partial_rollback" {
				t.Fatalf("expected reason=partial_rollback, got %s", p.TransitionReason)
			}
			return nil
		},
	}
	srv.reconcileApplying(context.Background(), h)
}

func TestApplyingPhase_AllRolledBack_RolledBack(t *testing.T) {
	ps := &multiNodePlanStore{plans: map[string]*planpb.NodePlan{}, statuses: map[string]*planpb.NodePlanStatus{
		"n1": {PlanId: "p1", State: planpb.PlanState_PLAN_ROLLED_BACK},
		"n2": {PlanId: "p2", State: planpb.PlanState_PLAN_ROLLED_BACK},
	}}
	srv := &server{cfg: &clusterControllerConfig{}, state: &controllerState{}, planStore: ps, resources: resourcestore.NewMemStore()}
	h := &releaseHandle{
		Name: "test", ResourceType: "ServiceRelease", Phase: cluster_controllerpb.ReleasePhaseApplying,
		Nodes: []*cluster_controllerpb.NodeReleaseStatus{
			{NodeID: "n1", PlanID: "p1"},
			{NodeID: "n2", PlanID: "p2"},
		},
		PatchStatus: func(ctx context.Context, p statusPatch) error {
			if p.Phase != cluster_controllerpb.ReleasePhaseRolledBack {
				t.Fatalf("expected ROLLED_BACK, got %s", p.Phase)
			}
			if p.TransitionReason != "all_nodes_rolled_back" {
				t.Fatalf("expected reason=all_nodes_rolled_back, got %s", p.TransitionReason)
			}
			return nil
		},
	}
	srv.reconcileApplying(context.Background(), h)
}

// ── Phase 7: Transition map coherence ────────────────────────────────────────

func TestTransitionMap_AllPhasesPresent(t *testing.T) {
	expectedPhases := []string{
		"",
		cluster_controllerpb.ReleasePhasePending,
		cluster_controllerpb.ReleasePhaseResolved,
		cluster_controllerpb.ReleasePhaseApplying,
		cluster_controllerpb.ReleasePhaseAvailable,
		cluster_controllerpb.ReleasePhaseDegraded,
		cluster_controllerpb.ReleasePhaseFailed,
		cluster_controllerpb.ReleasePhaseRolledBack,
		ReleasePhaseRemoving,
		ReleasePhaseRemoved,
	}
	for _, p := range expectedPhases {
		if _, ok := validPhaseTransitions[p]; !ok {
			t.Errorf("phase %q missing from transition map", p)
		}
	}
}

func TestTransitionMap_RemovingReachableFromAllNonTerminal(t *testing.T) {
	// Every non-terminal, non-removing phase should allow → REMOVING.
	nonTerminal := []string{
		"",
		cluster_controllerpb.ReleasePhasePending,
		cluster_controllerpb.ReleasePhaseResolved,
		cluster_controllerpb.ReleasePhaseApplying,
		cluster_controllerpb.ReleasePhaseAvailable,
		cluster_controllerpb.ReleasePhaseDegraded,
		cluster_controllerpb.ReleasePhaseFailed,
		cluster_controllerpb.ReleasePhaseRolledBack,
	}
	for _, p := range nonTerminal {
		targets := validPhaseTransitions[p]
		if !targets[ReleasePhaseRemoving] {
			t.Errorf("phase %q should allow → REMOVING", p)
		}
	}
}

// ── Phase 8: Distributed stability ───────────────────────────────────────────

func TestReconcile_ControllerRestart_ResumesFromStoredPhase(t *testing.T) {
	// Simulate controller restart: create server, store a release in APPLYING,
	// then reconcile — should poll plan statuses, not re-dispatch.
	ps := &multiNodePlanStore{plans: map[string]*planpb.NodePlan{}, statuses: map[string]*planpb.NodePlanStatus{
		"n1": {PlanId: "p1", State: planpb.PlanState_PLAN_SUCCEEDED},
	}}
	srv := &server{
		cfg:       &clusterControllerConfig{},
		state:     &controllerState{Nodes: map[string]*nodeState{"n1": {NodeID: "n1"}}},
		planStore: ps,
		resources: resourcestore.NewMemStore(),
	}
	srv.resources.Apply(context.Background(), "ServiceRelease", &cluster_controllerpb.ServiceRelease{
		Meta: &cluster_controllerpb.ObjectMeta{Name: "pub/svc", Generation: 1},
		Spec: &cluster_controllerpb.ServiceReleaseSpec{PublisherID: "pub", ServiceName: "svc", Version: "1.0.0"},
		Status: &cluster_controllerpb.ServiceReleaseStatus{
			Phase: cluster_controllerpb.ReleasePhaseApplying, ObservedGeneration: 1,
			ResolvedVersion: "1.0.0",
			Nodes: []*cluster_controllerpb.NodeReleaseStatus{{NodeID: "n1", PlanID: "p1", Phase: cluster_controllerpb.ReleasePhaseApplying}},
		},
	})

	srv.reconcileRelease(context.Background(), "pub/svc")
	obj, _, _ := srv.resources.Get(context.Background(), "ServiceRelease", "pub/svc")
	rel := obj.(*cluster_controllerpb.ServiceRelease)
	if rel.Status.Phase != cluster_controllerpb.ReleasePhaseAvailable {
		t.Fatalf("expected AVAILABLE after restart+success, got %s", rel.Status.Phase)
	}
}

// ── Phase 9: Plan slot serialization (single-slot model) ─────────────────────

func TestHasAnyActivePlan_RunningBlocksDispatch(t *testing.T) {
	ps := newMultiNodePlanStore()
	ps.statuses["n1"] = &planpb.NodePlanStatus{
		NodeId: "n1", PlanId: "p1", State: planpb.PlanState_PLAN_RUNNING,
	}
	srv := &server{planStore: ps}

	if !srv.hasAnyActivePlan(context.Background(), "n1") {
		t.Fatal("expected hasAnyActivePlan=true for RUNNING plan")
	}
}

func TestHasAnyActivePlan_PendingBlocksDispatch(t *testing.T) {
	ps := newMultiNodePlanStore()
	ps.statuses["n1"] = &planpb.NodePlanStatus{
		NodeId: "n1", PlanId: "p1", State: planpb.PlanState_PLAN_PENDING,
	}
	srv := &server{planStore: ps}

	if !srv.hasAnyActivePlan(context.Background(), "n1") {
		t.Fatal("expected hasAnyActivePlan=true for PENDING plan")
	}
}

func TestHasAnyActivePlan_TerminalDoesNotBlock(t *testing.T) {
	for _, state := range []planpb.PlanState{
		planpb.PlanState_PLAN_SUCCEEDED,
		planpb.PlanState_PLAN_FAILED,
		planpb.PlanState_PLAN_ROLLED_BACK,
	} {
		ps := newMultiNodePlanStore()
		ps.statuses["n1"] = &planpb.NodePlanStatus{
			NodeId: "n1", PlanId: "p1", State: state,
		}
		srv := &server{planStore: ps}
		if srv.hasAnyActivePlan(context.Background(), "n1") {
			t.Fatalf("expected hasAnyActivePlan=false for terminal state %s", state)
		}
	}
}

func TestHasAnyActivePlan_NoStatusDoesNotBlock(t *testing.T) {
	ps := newMultiNodePlanStore()
	srv := &server{planStore: ps}
	if srv.hasAnyActivePlan(context.Background(), "n1") {
		t.Fatal("expected hasAnyActivePlan=false when no status exists")
	}
}

func TestDifferentLocksMustNotOverwriteBusyNodeSlot(t *testing.T) {
	ps := newMultiNodePlanStore()
	ps.statuses["n1"] = &planpb.NodePlanStatus{
		NodeId: "n1", PlanId: "p1", State: planpb.PlanState_PLAN_RUNNING,
	}
	ps.plans["n1"] = &planpb.NodePlan{
		NodeId: "n1", PlanId: "p1", Locks: []string{"infrastructure:minio"},
	}
	srv := &server{planStore: ps}

	// hasActivePlanWithLock returns false for a different lock
	if srv.hasActivePlanWithLock(context.Background(), "n1", "infrastructure:gateway") {
		t.Fatal("lock-specific check should NOT match different lock key")
	}
	// But hasAnyActivePlan still blocks — this is the fix
	if !srv.hasAnyActivePlan(context.Background(), "n1") {
		t.Fatal("node slot is busy, must block regardless of lock key")
	}
}

func TestHasUnservedNodes_FailedNodeIsUnserved(t *testing.T) {
	srv := &server{
		state: &controllerState{Nodes: map[string]*nodeState{
			"n1": {NodeID: "n1"},
			"n2": {NodeID: "n2"},
		}},
	}
	h := &releaseHandle{
		ResourceType: "InfrastructureRelease",
		Nodes: []*cluster_controllerpb.NodeReleaseStatus{
			{NodeID: "n1", Phase: cluster_controllerpb.ReleasePhaseAvailable},
			{NodeID: "n2", Phase: cluster_controllerpb.ReleasePhaseFailed},
		},
	}
	if !srv.hasUnservedNodes(h) {
		t.Fatal("expected hasUnservedNodes=true: n2 is FAILED")
	}
}

func TestHasUnservedNodes_RolledBackIsUnserved(t *testing.T) {
	srv := &server{
		state: &controllerState{Nodes: map[string]*nodeState{
			"n1": {NodeID: "n1"},
		}},
	}
	h := &releaseHandle{
		ResourceType: "InfrastructureRelease",
		Nodes: []*cluster_controllerpb.NodeReleaseStatus{
			{NodeID: "n1", Phase: cluster_controllerpb.ReleasePhaseRolledBack},
		},
	}
	if !srv.hasUnservedNodes(h) {
		t.Fatal("expected hasUnservedNodes=true: n1 is ROLLED_BACK")
	}
}

func TestHasUnservedNodes_AllAvailableIsServed(t *testing.T) {
	srv := &server{
		state: &controllerState{Nodes: map[string]*nodeState{
			"n1": {NodeID: "n1"},
		}},
	}
	h := &releaseHandle{
		ResourceType: "InfrastructureRelease",
		Nodes: []*cluster_controllerpb.NodeReleaseStatus{
			{NodeID: "n1", Phase: cluster_controllerpb.ReleasePhaseAvailable},
		},
	}
	if srv.hasUnservedNodes(h) {
		t.Fatal("expected hasUnservedNodes=false: all nodes AVAILABLE")
	}
}

func TestMixedNodeRollout_OnlyFailedNodeRetried(t *testing.T) {
	ps := newMultiNodePlanStore()
	// n1 succeeded, n2 failed (terminal) — slot is free for redispatch
	ps.statuses["n1"] = &planpb.NodePlanStatus{
		NodeId: "n1", PlanId: "p1", State: planpb.PlanState_PLAN_SUCCEEDED,
	}
	ps.statuses["n2"] = &planpb.NodePlanStatus{
		NodeId: "n2", PlanId: "p2", State: planpb.PlanState_PLAN_FAILED,
	}
	srv := &server{planStore: ps}

	// n1 is terminal → not active
	if srv.hasAnyActivePlan(context.Background(), "n1") {
		t.Fatal("n1 succeeded, should not block")
	}
	// n2 is terminal → not active, can be redispatched
	if srv.hasAnyActivePlan(context.Background(), "n2") {
		t.Fatal("n2 failed, should not block redispatch")
	}
}
