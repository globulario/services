package main

import (
	"context"
	"errors"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// Regression guard for the fast-path added to reconcileRemoving:
// when a release is marked Removing=true and no node reports installed_state
// for the package, the controller skips the no-op uninstall workflow and
// transitions the release directly to REMOVED so the existing GC sweep can
// delete the ServiceRelease record.
//
// Without this fast-path, a release.remove.package workflow is dispatched
// against every node even when there is nothing to remove. On a single-node
// cluster, that produced 7 stuck-at-RESOLVED releases on 2026-06-04 after
// 7 optional/test services were removed from desired state via Part A.

func newRemovingTestServer(t *testing.T) *server {
	t.Helper()
	srv := &server{}
	srv.leader.Store(true)
	srv.state = &controllerState{
		Nodes: map[string]*nodeState{
			"node-a": {Identity: storedIdentity{Hostname: "node-a"}},
		},
	}
	srv.inflightWorkflows = map[string]context.CancelFunc{}
	return srv
}

func newRemovingHandle(patches *[]statusPatch) *releaseHandle {
	return &releaseHandle{
		Name:               "core@globular.io/echo",
		ResourceType:       "ServiceRelease",
		Phase:              cluster_controllerpb.ReleasePhaseResolved,
		Removing:           true,
		InstalledStateKind: "SERVICE",
		InstalledStateName: "echo",
		PatchStatus: func(ctx context.Context, p statusPatch) error {
			*patches = append(*patches, p)
			return nil
		},
	}
}

func TestReconcileRemoving_FastPath_NoInstalledStateAnywhere_MarksRemoved(t *testing.T) {
	srv := newRemovingTestServer(t)
	srv.testListInstalledPackages = func(_ context.Context, _, _ string) ([]*node_agentpb.InstalledPackage, error) {
		return nil, nil // no installs anywhere
	}

	var patches []statusPatch
	h := newRemovingHandle(&patches)

	srv.reconcileRemoving(context.Background(), h)

	if len(patches) != 1 {
		t.Fatalf("expected exactly one PatchStatus call (fast-path REMOVED), got %d", len(patches))
	}
	got := patches[0]
	if got.Phase != ReleasePhaseRemoved {
		t.Errorf("Phase = %q, want %q", got.Phase, ReleasePhaseRemoved)
	}
	if got.TransitionReason != "no_installed_state" {
		t.Errorf("TransitionReason = %q, want %q", got.TransitionReason, "no_installed_state")
	}
	if got.SetFields != "phase" {
		t.Errorf("SetFields = %q, want %q (matches set_fields_routing_must_match_release_kind invariant)", got.SetFields, "phase")
	}
}

func TestReconcileRemoving_FastPath_InstalledStateOnOneNode_DoesNotShortCircuit(t *testing.T) {
	srv := newRemovingTestServer(t)
	srv.testListInstalledPackages = func(_ context.Context, nodeID, _ string) ([]*node_agentpb.InstalledPackage, error) {
		return []*node_agentpb.InstalledPackage{{Name: "echo"}}, nil
	}

	var patches []statusPatch
	h := newRemovingHandle(&patches)

	// The workflow dispatch path will attempt to reach a workflow service that
	// the test server does not configure; recover from the resulting panic so
	// the assertion below still runs.
	defer func() {
		_ = recover()
		for _, p := range patches {
			if p.Phase == ReleasePhaseRemoved && p.TransitionReason == "no_installed_state" {
				t.Errorf("fast-path fired when installed_state present (patch=%+v)", p)
			}
		}
	}()
	srv.reconcileRemoving(context.Background(), h)
}

func TestReconcileRemoving_FastPath_LookupError_DoesNotMarkRemoved(t *testing.T) {
	srv := newRemovingTestServer(t)
	srv.testListInstalledPackages = func(_ context.Context, _, _ string) ([]*node_agentpb.InstalledPackage, error) {
		return nil, errors.New("simulated etcd outage")
	}

	var patches []statusPatch
	h := newRemovingHandle(&patches)

	defer func() {
		_ = recover()
		for _, p := range patches {
			if p.Phase == ReleasePhaseRemoved && p.TransitionReason == "no_installed_state" {
				t.Errorf("fast-path fired on lookup error — must fail closed (patch=%+v)", p)
			}
		}
	}()
	srv.reconcileRemoving(context.Background(), h)
}

func TestReconcileRemoving_FastPath_ActiveWorkflowDeferredNoFastPath(t *testing.T) {
	srv := newRemovingTestServer(t)
	srv.testListInstalledPackages = func(_ context.Context, _, _ string) ([]*node_agentpb.InstalledPackage, error) {
		return nil, nil // would fast-path if reached
	}
	// Simulate an in-flight workflow for this release.
	releaseID := "ServiceRelease/core@globular.io/echo"
	_, cancel := context.WithCancel(context.Background())
	srv.inflightWorkflows[releaseID] = cancel
	defer cancel()

	var patches []statusPatch
	h := newRemovingHandle(&patches)

	srv.reconcileRemoving(context.Background(), h)

	if len(patches) != 0 {
		t.Fatalf("expected zero PatchStatus calls while a workflow is in-flight, got %d (%+v)", len(patches), patches)
	}
}

func TestReconcileRemoving_FastPath_NoNodes_StillTakesNoNodesPath(t *testing.T) {
	// Regression guard: the pre-existing len(nodeIDs)==0 fast-path
	// (reason="no_nodes") must remain intact and take precedence over the
	// new no_installed_state fast-path so no installed-state lookup is even
	// attempted on a 0-node cluster.
	srv := &server{}
	srv.leader.Store(true)
	srv.state = &controllerState{Nodes: map[string]*nodeState{}}
	srv.testListInstalledPackages = func(_ context.Context, _, _ string) ([]*node_agentpb.InstalledPackage, error) {
		t.Fatal("installed-state lookup must NOT run on a 0-node cluster")
		return nil, nil
	}

	var patches []statusPatch
	h := newRemovingHandle(&patches)

	srv.reconcileRemoving(context.Background(), h)

	if len(patches) != 1 || patches[0].TransitionReason != "no_nodes" {
		t.Fatalf("expected single PatchStatus with reason=no_nodes, got %+v", patches)
	}
}
