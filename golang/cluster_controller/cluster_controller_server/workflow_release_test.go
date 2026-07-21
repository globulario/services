package main

import (
	"context"
	"errors"
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// TestIsSyntheticReleaseName locks in the contract the patch helpers
// depend on: any release name emitted by cluster.reconcile's drift
// dispatch (reconcile_actions.go:371-372) is treated as synthetic and
// its status patches become no-ops. The prefix "reconcile-" is the
// unique marker — real releases are named after their package.
//
// If this predicate drifts (e.g. a new reconcile path uses a different
// prefix, or a real release accidentally gets a "reconcile-" name),
// this test fails loudly instead of the status patches silently
// succeeding on a release that actually existed.
func TestIsSyntheticReleaseName(t *testing.T) {
	cases := map[string]bool{
		// Synthetic — dispatched by cluster.reconcile drift loop.
		"reconcile-cluster-controller": true,
		"reconcile-etcd":               true,
		"reconcile-scylladb":           true,
		// Real releases — package-named, persisted in etcd.
		"cluster-controller": false,
		"etcd":               false,
		"scylladb":           false,
		"":                   false,
		// Edge: name that merely contains "reconcile" mid-string.
		"pkg-reconcile":        false,
		"reconcilable-service": false,
	}
	for in, want := range cases {
		if got := isSyntheticReleaseName(in); got != want {
			t.Errorf("isSyntheticReleaseName(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestSelectReleaseTargets_AbsentServiceOnEligibleNodeIsTarget(t *testing.T) {
	restore := stubInstalledPackageForReleaseTarget(t, nil, nil)
	defer restore()

	srv := releaseTargetTestServer("n1", BootstrapWorkloadReady, []unitStatusRecord{})
	selection, err := srv.selectReleaseTargets(context.Background(), []any{"n1"}, "test-svc", "SERVICE", "desired-hash", "build-1")
	if err != nil {
		t.Fatalf("selectReleaseTargets: %v", err)
	}
	if len(selection.Targets) != 1 {
		t.Fatalf("targets len = %d, want 1 (%+v)", len(selection.Targets), selection)
	}
	if selection.FinalizeStatus != cluster_controllerpb.ReleasePhaseAvailable {
		t.Fatalf("FinalizeStatus = %q, want AVAILABLE", selection.FinalizeStatus)
	}
}

func TestSelectReleaseTargets_AllConvergedZeroTargetsAvailable(t *testing.T) {
	installed := &node_agentpb.InstalledPackage{
		Name:     "test-svc",
		Kind:     "SERVICE",
		Checksum: "desired-hash",
		BuildId:  "build-1",
		Status:   "installed",
	}
	restore := stubInstalledPackageForReleaseTarget(t, installed, nil)
	defer restore()

	srv := releaseTargetTestServer("n1", BootstrapWorkloadReady, []unitStatusRecord{
		{Name: "globular-test-svc.service", State: "active"},
	})
	selection, err := srv.selectReleaseTargets(context.Background(), []any{"n1"}, "test-svc", "SERVICE", "desired-hash", "build-1")
	if err != nil {
		t.Fatalf("selectReleaseTargets: %v", err)
	}
	if len(selection.Targets) != 0 {
		t.Fatalf("targets len = %d, want 0 (%+v)", len(selection.Targets), selection.Targets)
	}
	if selection.FinalizeStatus != cluster_controllerpb.ReleasePhaseAvailable {
		t.Fatalf("FinalizeStatus = %q, want AVAILABLE", selection.FinalizeStatus)
	}
}

func TestSelectReleaseTargets_InstalledStateUnknownZeroTargetsDeferred(t *testing.T) {
	restore := stubInstalledPackageForReleaseTarget(t, nil, errors.New("etcd deadline exceeded"))
	defer restore()

	srv := releaseTargetTestServer("n1", BootstrapWorkloadReady, []unitStatusRecord{})
	selection, err := srv.selectReleaseTargets(context.Background(), []any{"n1"}, "test-svc", "SERVICE", "desired-hash", "build-1")
	if err != nil {
		t.Fatalf("selectReleaseTargets: %v", err)
	}
	if len(selection.Targets) != 0 {
		t.Fatalf("targets len = %d, want 0 (%+v)", len(selection.Targets), selection.Targets)
	}
	if selection.FinalizeStatus != cluster_controllerpb.ReleasePhaseDeferred {
		t.Fatalf("FinalizeStatus = %q, want DEFERRED", selection.FinalizeStatus)
	}
	if selection.Reason != "installed_state_unknown" {
		t.Fatalf("Reason = %q, want installed_state_unknown", selection.Reason)
	}
}

func TestSelectReleaseTargets_BootstrapNotReadyZeroTargetsDeferred(t *testing.T) {
	restore := stubInstalledPackageForReleaseTarget(t, nil, nil)
	defer restore()

	srv := releaseTargetTestServer("n1", BootstrapAdmitted, []unitStatusRecord{})
	selection, err := srv.selectReleaseTargets(context.Background(), []any{"n1"}, "test-svc", "SERVICE", "desired-hash", "build-1")
	if err != nil {
		t.Fatalf("selectReleaseTargets: %v", err)
	}
	if len(selection.Targets) != 0 {
		t.Fatalf("targets len = %d, want 0 (%+v)", len(selection.Targets), selection.Targets)
	}
	if selection.FinalizeStatus != cluster_controllerpb.ReleasePhaseDeferred {
		t.Fatalf("FinalizeStatus = %q, want DEFERRED", selection.FinalizeStatus)
	}
	if selection.Reason != "bootstrap_not_ready" {
		t.Fatalf("Reason = %q, want bootstrap_not_ready", selection.Reason)
	}
}

func releaseTargetTestServer(nodeID string, phase BootstrapPhase, units []unitStatusRecord) *server {
	return &server{
		state: &controllerState{
			Nodes: map[string]*nodeState{
				nodeID: {
					NodeID:         nodeID,
					BootstrapPhase: phase,
					LastSeen:       time.Now(),
					AgentEndpoint:  "10.0.0.10:12001",
					Units:          units,
				},
			},
		},
	}
}

func stubInstalledPackageForReleaseTarget(t *testing.T, pkg *node_agentpb.InstalledPackage, err error) func() {
	t.Helper()
	old := getInstalledPackageForReleaseTarget
	getInstalledPackageForReleaseTarget = func(context.Context, string, string, string) (*node_agentpb.InstalledPackage, error) {
		return pkg, err
	}
	return func() {
		getInstalledPackageForReleaseTarget = old
	}
}
