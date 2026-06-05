package main

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// ── Snapshot model ───────────────────────────────────────────────────────────

// TestRecoverySnapshotRequiresInstalledInventory verifies that buildReseedPlan
// returns an error when given an empty snapshot, ensuring we never attempt
// a reseed from a blank state.
func TestRecoverySnapshotRequiresInstalledInventory(t *testing.T) {
	_, err := buildReseedPlan(nil, false)
	if err == nil {
		t.Error("expected error for nil snapshot, got nil")
	}

	_, err = buildReseedPlan(&cluster_controllerpb.NodeRecoverySnapshot{}, false)
	if err == nil {
		t.Error("expected error for empty snapshot, got nil")
	}

	// Non-empty snapshot should succeed.
	snap := &cluster_controllerpb.NodeRecoverySnapshot{
		Artifacts: []cluster_controllerpb.SnapshotArtifact{
			{Name: "etcd", Kind: "INFRASTRUCTURE", Version: "3.5.0", BuildID: "bld-001"},
		},
	}
	plan, err := buildReseedPlan(snap, false)
	if err != nil {
		t.Fatalf("expected plan for non-empty snapshot, got error: %v", err)
	}
	if len(plan) != 1 {
		t.Fatalf("expected 1 planned artifact, got %d", len(plan))
	}
}

// ── Planner ordering ─────────────────────────────────────────────────────────

// TestRecoveryPlannerOrdersInfraBeforeServices verifies that foundation-class
// artifacts (etcd, scylladb) are scheduled before core-control-plane services,
// and control-plane before workload applications.
func TestRecoveryPlannerOrdersInfraBeforeServices(t *testing.T) {
	artifacts := []cluster_controllerpb.SnapshotArtifact{
		{Name: "my-app", Kind: "APPLICATION", Version: "1.0.0"},
		{Name: "log", Kind: "SERVICE", Version: "1.0.0"},
		{Name: "etcd", Kind: "INFRASTRUCTURE", Version: "3.5.0"},
		{Name: "scylladb", Kind: "INFRASTRUCTURE", Version: "5.0.0"},
		{Name: "authentication", Kind: "SERVICE", Version: "1.0.0"},
		{Name: "monitoring", Kind: "SERVICE", Version: "1.0.0"},
	}

	sorted := sortedReseedOrder(artifacts)

	if len(sorted) != len(artifacts) {
		t.Fatalf("sorted length mismatch: got %d, want %d", len(sorted), len(artifacts))
	}

	// etcd and scylladb must appear before authentication / log.
	pos := func(name string) int {
		for i, a := range sorted {
			if a.Name == name {
				return i
			}
		}
		return -1
	}

	if pos("etcd") >= pos("authentication") {
		t.Errorf("etcd should be before authentication: etcd=%d auth=%d", pos("etcd"), pos("authentication"))
	}
	if pos("scylladb") >= pos("log") {
		t.Errorf("scylladb should be before log: scylladb=%d log=%d", pos("scylladb"), pos("log"))
	}
	if pos("authentication") >= pos("my-app") {
		t.Errorf("authentication should be before my-app: auth=%d app=%d", pos("authentication"), pos("my-app"))
	}
	if pos("monitoring") >= pos("my-app") {
		t.Errorf("monitoring should be before my-app: monitoring=%d app=%d", pos("monitoring"), pos("my-app"))
	}
}

// TestRecoveryPlannerOrderIsStable verifies that two identical artifact lists
// produce the same sorted order (deterministic).
func TestRecoveryPlannerOrderIsStable(t *testing.T) {
	artifacts := []cluster_controllerpb.SnapshotArtifact{
		{Name: "workflow", Kind: "SERVICE", Version: "1.0.0"},
		{Name: "etcd", Kind: "INFRASTRUCTURE", Version: "3.5.0"},
		{Name: "rbac", Kind: "SERVICE", Version: "1.0.0"},
		{Name: "minio", Kind: "INFRASTRUCTURE", Version: "7.0.0"},
	}

	first := sortedReseedOrder(artifacts)
	second := sortedReseedOrder(artifacts)

	for i := range first {
		if first[i].Name != second[i].Name {
			t.Errorf("sort not stable: position %d differs (%s vs %s)", i, first[i].Name, second[i].Name)
		}
	}
}

// ── Exact-replay validation ──────────────────────────────────────────────────

// TestRecoveryPlannerFailsOnMissingExactBuildInStrictMode verifies that when
// exact_replay_required=true, any artifact without a build_id causes an error.
func TestRecoveryPlannerFailsOnMissingExactBuildInStrictMode(t *testing.T) {
	snap := &cluster_controllerpb.NodeRecoverySnapshot{
		Artifacts: []cluster_controllerpb.SnapshotArtifact{
			{Name: "etcd", Kind: "INFRASTRUCTURE", Version: "3.5.0", BuildID: "bld-001"},
			{Name: "log", Kind: "SERVICE", Version: "1.0.0"}, // no build_id
		},
	}

	_, err := buildReseedPlan(snap, true /* exactRequired */)
	if err == nil {
		t.Error("expected error when exactRequired=true and artifact missing build_id, got nil")
	}
}

// TestRecoveryPlannerAllowsFallbackWhenConfigured verifies that when
// exact_replay_required=false, artifacts without a build_id are allowed and
// receive source=REPOSITORY_RESOLVED.
func TestRecoveryPlannerAllowsFallbackWhenConfigured(t *testing.T) {
	snap := &cluster_controllerpb.NodeRecoverySnapshot{
		Artifacts: []cluster_controllerpb.SnapshotArtifact{
			{Name: "etcd", Kind: "INFRASTRUCTURE", Version: "3.5.0", BuildID: "bld-001"},
			{Name: "log", Kind: "SERVICE", Version: "1.0.0"}, // no build_id
		},
	}

	plan, err := buildReseedPlan(snap, false /* exactRequired */)
	if err != nil {
		t.Fatalf("expected success with exactRequired=false, got: %v", err)
	}
	if len(plan) != 2 {
		t.Fatalf("expected 2 planned artifacts, got %d", len(plan))
	}

	// Find log artifact.
	for _, p := range plan {
		if p.Name == "log" {
			if p.Source != "REPOSITORY_RESOLVED" {
				t.Errorf("expected log source=REPOSITORY_RESOLVED, got %q", p.Source)
			}
			return
		}
	}
	t.Error("log artifact not found in plan")
}

// TestRecoveryPlannerExactBuildArtifactSource verifies that artifacts with a
// build_id get source=SNAPSHOT_EXACT.
func TestRecoveryPlannerExactBuildArtifactSource(t *testing.T) {
	snap := &cluster_controllerpb.NodeRecoverySnapshot{
		Artifacts: []cluster_controllerpb.SnapshotArtifact{
			{Name: "etcd", Kind: "INFRASTRUCTURE", Version: "3.5.0", BuildID: "bld-exact-001"},
		},
	}

	plan, err := buildReseedPlan(snap, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan[0].Source != "SNAPSHOT_EXACT" {
		t.Errorf("expected source=SNAPSHOT_EXACT, got %q", plan[0].Source)
	}
	if plan[0].BuildID != "bld-exact-001" {
		t.Errorf("expected build_id=bld-exact-001, got %q", plan[0].BuildID)
	}
}

// ── Cycle detection ──────────────────────────────────────────────────────────

// TestValidateNoReseedCycle_DetectsCycle verifies that a cycle in the
// requires/provides graph is detected and reported.
func TestValidateNoReseedCycle_DetectsCycle(t *testing.T) {
	artifacts := []cluster_controllerpb.SnapshotArtifact{
		{Name: "a", Kind: "SERVICE", Requires: []string{"b"}},
		{Name: "b", Kind: "SERVICE", Requires: []string{"c"}},
		{Name: "c", Kind: "SERVICE", Requires: []string{"a"}}, // cycle: a→b→c→a
	}

	err := validateNoReseedCycle(artifacts)
	if err == nil {
		t.Error("expected cycle detection error, got nil")
	}
}

// TestValidateNoReseedCycle_AcceptsDAG verifies that a valid DAG (no cycles)
// passes cycle validation.
func TestValidateNoReseedCycle_AcceptsDAG(t *testing.T) {
	artifacts := []cluster_controllerpb.SnapshotArtifact{
		{Name: "etcd", Kind: "INFRASTRUCTURE"},
		{Name: "log", Kind: "SERVICE", Requires: []string{"etcd"}},
		{Name: "workflow", Kind: "SERVICE", Requires: []string{"log", "etcd"}},
		{Name: "my-app", Kind: "APPLICATION", Requires: []string{"workflow"}},
	}

	if err := validateNoReseedCycle(artifacts); err != nil {
		t.Errorf("expected no cycle, got error: %v", err)
	}
}

// TestValidateNoReseedCycle_IgnoresExternalDeps verifies that requires entries
// naming artifacts not in the snapshot (external dependencies) do not cause
// false cycle detection errors.
func TestValidateNoReseedCycle_IgnoresExternalDeps(t *testing.T) {
	artifacts := []cluster_controllerpb.SnapshotArtifact{
		{Name: "my-app", Kind: "APPLICATION", Requires: []string{"external-service"}},
	}

	if err := validateNoReseedCycle(artifacts); err != nil {
		t.Errorf("external dep should be ignored, got error: %v", err)
	}
}

// ── Plan order field ─────────────────────────────────────────────────────────

// TestRecoveryPlannerOrderField verifies that PlannedRecoveryArtifact.Order
// reflects insertion position in the install sequence.
func TestRecoveryPlannerOrderField(t *testing.T) {
	snap := &cluster_controllerpb.NodeRecoverySnapshot{
		Artifacts: []cluster_controllerpb.SnapshotArtifact{
			{Name: "my-app", Kind: "APPLICATION", Version: "1.0.0"},
			{Name: "etcd", Kind: "INFRASTRUCTURE", Version: "3.5.0", BuildID: "bld-001"},
		},
	}

	plan, err := buildReseedPlan(snap, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i, p := range plan {
		if int(p.Order) != i {
			t.Errorf("artifact %s: expected order=%d, got %d", p.Name, i, p.Order)
		}
	}

	// etcd (INFRASTRUCTURE/FOUNDATION) must have lower order than my-app.
	etcdOrder, appOrder := -1, -1
	for _, p := range plan {
		if p.Name == "etcd" {
			etcdOrder = int(p.Order)
		}
		if p.Name == "my-app" {
			appOrder = int(p.Order)
		}
	}
	if etcdOrder >= appOrder {
		t.Errorf("etcd should have lower order than my-app: etcd=%d app=%d", etcdOrder, appOrder)
	}
}

// ── Reconciler fencing ───────────────────────────────────────────────────────

// TestReconcilerSkipsNodeWhenRecoveryPaused verifies the isNodeUnderRecovery
// predicate:
//   - Returns false when state is nil
//   - Returns false when ReconciliationPaused=false
//   - Returns false when phase is terminal (COMPLETE / FAILED)
//   - Returns true only when non-terminal + ReconciliationPaused=true
func TestReconcilerSkipsNodeWhenRecoveryPaused(t *testing.T) {
	// nil state → not fenced
	if isNodeUnderRecoveryState(nil) {
		t.Error("nil state should not be fenced")
	}

	// not paused → not fenced
	notPaused := &cluster_controllerpb.NodeRecoveryState{
		Phase:                cluster_controllerpb.NodeRecoveryPhaseReseedArtifacts,
		ReconciliationPaused: false,
	}
	if isNodeUnderRecoveryState(notPaused) {
		t.Error("ReconciliationPaused=false should not be fenced")
	}

	// COMPLETE terminal → not fenced
	complete := &cluster_controllerpb.NodeRecoveryState{
		Phase:                cluster_controllerpb.NodeRecoveryPhaseComplete,
		ReconciliationPaused: true,
	}
	if isNodeUnderRecoveryState(complete) {
		t.Error("COMPLETE phase should not be fenced even if ReconciliationPaused=true")
	}

	// FAILED terminal → not fenced
	failed := &cluster_controllerpb.NodeRecoveryState{
		Phase:                cluster_controllerpb.NodeRecoveryPhaseFailed,
		ReconciliationPaused: true,
	}
	if isNodeUnderRecoveryState(failed) {
		t.Error("FAILED phase should not be fenced even if ReconciliationPaused=true")
	}

	// Active phase + paused → fenced
	active := &cluster_controllerpb.NodeRecoveryState{
		Phase:                cluster_controllerpb.NodeRecoveryPhaseReseedArtifacts,
		ReconciliationPaused: true,
	}
	if !isNodeUnderRecoveryState(active) {
		t.Error("active phase + ReconciliationPaused=true should be fenced")
	}
}

// isNodeUnderRecoveryState lives in recovery_state_store.go — the
// production isNodeUnderRecovery shares it so this test exercises the
// same decision logic the reconciler uses.

// TestIsNodeUnderRecoveryState_FailClosedSemantics pins the decision
// table behind the fence so a future refactor can't accidentally
// drop the "terminal phase = unfenced" rule. The companion
// isNodeUnderRecovery (etcd-backed) wraps this and adds an `observable`
// return so transient etcd outages are also treated as fenced — the
// regression that test coverage in reconcile_nodes.go pins (see
// TestReconcilerFencesNodeWhenRecoveryUnobservable, when added).
func TestIsNodeUnderRecoveryState_FailClosedSemantics(t *testing.T) {
	cases := []struct {
		name     string
		st       *cluster_controllerpb.NodeRecoveryState
		expected bool
	}{
		{"nil state", nil, false},
		{"phase active, paused=false", &cluster_controllerpb.NodeRecoveryState{
			Phase:                cluster_controllerpb.NodeRecoveryPhaseReseedArtifacts,
			ReconciliationPaused: false,
		}, false},
		{"phase terminal complete, paused=true", &cluster_controllerpb.NodeRecoveryState{
			Phase:                cluster_controllerpb.NodeRecoveryPhaseComplete,
			ReconciliationPaused: true,
		}, false},
		{"phase terminal failed, paused=true", &cluster_controllerpb.NodeRecoveryState{
			Phase:                cluster_controllerpb.NodeRecoveryPhaseFailed,
			ReconciliationPaused: true,
		}, false},
		{"phase active, paused=true", &cluster_controllerpb.NodeRecoveryState{
			Phase:                cluster_controllerpb.NodeRecoveryPhaseReseedArtifacts,
			ReconciliationPaused: true,
		}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isNodeUnderRecoveryState(tc.st); got != tc.expected {
				t.Errorf("isNodeUnderRecoveryState(%s) = %v; want %v", tc.name, got, tc.expected)
			}
		})
	}
}
