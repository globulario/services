package main

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// Plan-based tests removed — workflow runs are now authoritative.
// Deleted: TestRemovalWorkflow_*, TestRolledBack_*, TestFailedStepPropagation,
//   TestApplyingPhase_*, TestHasAnyActivePlan_*, TestCheckNodePlanStatuses_*,
//   TestDifferentLocksMustNotOverwriteBusyNodeSlot, TestMixedNodeRollout_*,
//   TestReconcileApplying_AllLostPlans_*, TestReconcile_ControllerRestart_*,
//   TestHasUnservedNodes_*

// ── Phase transition enforcement (pure logic, no plan dependency) ────────────

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
		{cluster_controllerpb.ReleasePhaseResolved, cluster_controllerpb.ReleasePhaseAvailable},
		{cluster_controllerpb.ReleasePhaseApplying, cluster_controllerpb.ReleasePhaseAvailable},
		{cluster_controllerpb.ReleasePhaseApplying, cluster_controllerpb.ReleasePhaseResolved},
		{cluster_controllerpb.ReleasePhaseApplying, cluster_controllerpb.ReleasePhaseRolledBack},
		{cluster_controllerpb.ReleasePhaseAvailable, cluster_controllerpb.ReleasePhasePending},
		{cluster_controllerpb.ReleasePhaseAvailable, cluster_controllerpb.ReleasePhaseDegraded},
		{cluster_controllerpb.ReleasePhaseAvailable, cluster_controllerpb.ReleasePhaseFailed},
		{cluster_controllerpb.ReleasePhaseFailed, cluster_controllerpb.ReleasePhasePending},
		{cluster_controllerpb.ReleasePhaseRolledBack, cluster_controllerpb.ReleasePhasePending},
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
	applyPatchToSvcStatus(s, statusPatch{
		Phase:            cluster_controllerpb.ReleasePhaseApplying,
		TransitionReason: "plans_dispatched",
		SetFields:        "phase",
	})
	if s.TransitionReason != "plans_dispatched" {
		t.Fatalf("expected TransitionReason=plans_dispatched, got %s", s.TransitionReason)
	}
}

func TestPlanned_NotInTransitionMap(t *testing.T) {
	if _, ok := validPhaseTransitions[cluster_controllerpb.ReleasePhasePlanned]; ok {
		t.Fatalf("PLANNED should not be in the transition map (reserved for future use)")
	}
	for from, targets := range validPhaseTransitions {
		if targets[cluster_controllerpb.ReleasePhasePlanned] {
			t.Fatalf("phase %q allows transition to PLANNED, which should not be reachable", from)
		}
	}
}

func TestTransitionMap_AllPhasesPresent(t *testing.T) {
	// Verify all non-empty phases appear in the map.
	expected := []string{
		cluster_controllerpb.ReleasePhasePending,
		cluster_controllerpb.ReleasePhaseResolved,
		cluster_controllerpb.ReleasePhaseApplying,
		cluster_controllerpb.ReleasePhaseAvailable,
		cluster_controllerpb.ReleasePhaseFailed,
		cluster_controllerpb.ReleasePhaseDegraded,
		cluster_controllerpb.ReleasePhaseRolledBack,
		ReleasePhaseRemoving,
		ReleasePhaseRemoved,
	}
	for _, p := range expected {
		if _, ok := validPhaseTransitions[p]; !ok {
			t.Errorf("phase %q missing from transition map", p)
		}
	}
}

func TestTransitionMap_RemovingReachableFromAllNonTerminal(t *testing.T) {
	// REMOVING should be reachable from AVAILABLE and FAILED.
	for _, src := range []string{cluster_controllerpb.ReleasePhaseAvailable, cluster_controllerpb.ReleasePhaseFailed} {
		if !validPhaseTransitions[src][ReleasePhaseRemoving] {
			t.Errorf("%s should allow transition to REMOVING", src)
		}
	}
}
