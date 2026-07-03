// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.remove_desired_service_gc_test
// @awareness file_role=unit_test_for_remove_desired_service_release_garbage_collection
// @awareness enforces=globular.platform:invariant.cluster.desired_state_authority_over_installed_state
// @awareness risk=high
package main

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_controller/resourcestore"
)

// seedDesiredAndRelease seeds a ServiceDesiredVersion plus its ServiceRelease
// (with the given phase and per-node rollout status) for a workload service.
func seedDesiredAndRelease(t *testing.T, store resourcestore.Store, canon, phase string, nodes []*cluster_controllerpb.NodeReleaseStatus) {
	t.Helper()
	ctx := context.Background()
	sdv := &cluster_controllerpb.ServiceDesiredVersion{
		Meta: &cluster_controllerpb.ObjectMeta{Name: canon},
		Spec: &cluster_controllerpb.ServiceDesiredVersionSpec{Version: "1.2.267"},
	}
	if _, err := store.Apply(ctx, "ServiceDesiredVersion", sdv); err != nil {
		t.Fatalf("seed ServiceDesiredVersion %q: %v", canon, err)
	}
	rel := &cluster_controllerpb.ServiceRelease{
		Meta: &cluster_controllerpb.ObjectMeta{Name: defaultPublisherID() + "/" + canon},
		Spec: &cluster_controllerpb.ServiceReleaseSpec{ServiceName: canon, Version: "1.2.267"},
		Status: &cluster_controllerpb.ServiceReleaseStatus{
			Phase: phase,
			Nodes: nodes,
		},
	}
	if _, err := store.Apply(ctx, "ServiceRelease", rel); err != nil {
		t.Fatalf("seed ServiceRelease %q: %v", canon, err)
	}
}

// A workload service that only ever RESOLVED (never dispatched to any node: no
// per-node status) has nothing to uninstall. RemoveDesiredService must garbage-
// collect its ServiceRelease SYNCHRONOUSLY so it disappears from the desired/
// install list immediately — without waiting on the async removal reconcile or
// the workflow engine. This decision is made from the release's OWN lifecycle
// status (phase + Status.Nodes), never from a node installed-state probe, so it
// stays clear of cluster.desired_state_authority_over_installed_state.
func TestRemoveDesiredService_NeverDispatched_GCsReleaseSynchronously(t *testing.T) {
	ctx := context.Background()
	store := resourcestore.NewMemStore()
	srv := &server{resources: store}
	srv.leader.Store(true) // reach the removal path (skip leader-forward)

	seedDesiredAndRelease(t, store, "media", cluster_controllerpb.ReleasePhaseResolved, nil)

	if _, err := srv.RemoveDesiredService(ctx, &cluster_controllerpb.RemoveDesiredServiceRequest{ServiceId: "media"}); err != nil {
		t.Fatalf("RemoveDesiredService(media): %v", err)
	}

	if obj, _, _ := store.Get(ctx, "ServiceDesiredVersion", "media"); obj != nil {
		t.Error("ServiceDesiredVersion(media) must be deleted by removal")
	}
	if obj, _, _ := store.Get(ctx, "ServiceRelease", defaultPublisherID()+"/media"); obj != nil {
		t.Error("ServiceRelease(media) was never dispatched — it must be garbage-collected synchronously, not left lingering as removing=true")
	}
}

// A workload service that DID roll out to a node must NOT be dropped
// synchronously — its ServiceRelease is flagged spec.removing so the async,
// lifecycle-tracked removal workflow actually uninstalls it from that node
// first (HARD RULE 4). Synchronously deleting it would orphan the installed
// bits on the node with no uninstall.
func TestRemoveDesiredService_RolledOut_FlagsRemovingKeepsRelease(t *testing.T) {
	ctx := context.Background()
	store := resourcestore.NewMemStore()
	srv := &server{resources: store}
	srv.leader.Store(true)

	seedDesiredAndRelease(t, store, "media", cluster_controllerpb.ReleasePhaseAvailable,
		[]*cluster_controllerpb.NodeReleaseStatus{{NodeID: "node-1", Phase: cluster_controllerpb.ReleasePhaseAvailable, InstalledVersion: "1.2.267"}})

	if _, err := srv.RemoveDesiredService(ctx, &cluster_controllerpb.RemoveDesiredServiceRequest{ServiceId: "media"}); err != nil {
		t.Fatalf("RemoveDesiredService(media): %v", err)
	}

	if obj, _, _ := store.Get(ctx, "ServiceDesiredVersion", "media"); obj != nil {
		t.Error("ServiceDesiredVersion(media) must be deleted by removal")
	}
	obj, _, _ := store.Get(ctx, "ServiceRelease", defaultPublisherID()+"/media")
	if obj == nil {
		t.Fatal("ServiceRelease(media) rolled out to a node — it must be kept for the async uninstall workflow, not synchronously deleted")
	}
	rel, ok := obj.(*cluster_controllerpb.ServiceRelease)
	if !ok || rel.Spec == nil || !rel.Spec.Removing {
		t.Errorf("ServiceRelease(media) must be flagged spec.removing=true for the removal workflow, got %+v", obj)
	}
}
