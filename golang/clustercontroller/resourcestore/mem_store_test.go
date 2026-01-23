package resourcestore

import (
	"context"
	"testing"
	"time"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
)

func TestApplyGenerationStable(t *testing.T) {
	store := NewMemStore()
	net := &clustercontrollerpb.ClusterNetwork{
		Meta: &clustercontrollerpb.ObjectMeta{Name: "default"},
		Spec: &clustercontrollerpb.ClusterNetworkSpec{ClusterDomain: "example.com", Protocol: "http"},
	}
	obj, err := store.Apply(context.Background(), "ClusterNetwork", net)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	first := obj.(*clustercontrollerpb.ClusterNetwork)
	if first.Meta.Generation != 1 {
		t.Fatalf("expected generation 1, got %d", first.Meta.Generation)
	}
	// Apply identical spec, generation should stay the same.
	obj, err = store.Apply(context.Background(), "ClusterNetwork", net)
	if err != nil {
		t.Fatalf("apply identical: %v", err)
	}
	second := obj.(*clustercontrollerpb.ClusterNetwork)
	if second.Meta.Generation != 1 {
		t.Fatalf("expected generation unchanged, got %d", second.Meta.Generation)
	}
	// Modify spec, generation increments.
	net.Spec.Protocol = "https"
	obj, err = store.Apply(context.Background(), "ClusterNetwork", net)
	if err != nil {
		t.Fatalf("apply modified: %v", err)
	}
	third := obj.(*clustercontrollerpb.ClusterNetwork)
	if third.Meta.Generation != 2 {
		t.Fatalf("expected generation 2, got %d", third.Meta.Generation)
	}
}

func TestWatchEmitsEvents(t *testing.T) {
	store := NewMemStore()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := store.Watch(ctx, "ServiceDesiredVersion", "", "")
	if err != nil {
		t.Fatalf("watch: %v", err)
	}
	svc := &clustercontrollerpb.ServiceDesiredVersion{
		Meta: &clustercontrollerpb.ObjectMeta{Name: "gateway"},
		Spec: &clustercontrollerpb.ServiceDesiredVersionSpec{ServiceName: "gateway", Version: "1.0.0"},
	}
	if _, err := store.Apply(context.Background(), "ServiceDesiredVersion", svc); err != nil {
		t.Fatalf("apply: %v", err)
	}
	select {
	case evt := <-ch:
		if evt.Type != EventAdded {
			t.Fatalf("expected ADDED, got %s", evt.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for watch event")
	}
	// Update status and expect MODIFIED.
	if _, err := store.UpdateStatus(context.Background(), "ServiceDesiredVersion", "gateway", &clustercontrollerpb.ObjectStatus{ObservedGeneration: 1}); err != nil {
		t.Fatalf("update status: %v", err)
	}
	select {
	case evt := <-ch:
		if evt.Type != EventModified {
			t.Fatalf("expected MODIFIED, got %s", evt.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for modified event")
	}
	// Delete and expect DELETED.
	if err := store.Delete(context.Background(), "ServiceDesiredVersion", "gateway"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	select {
	case evt := <-ch:
		if evt.Type != EventDeleted {
			t.Fatalf("expected DELETED, got %s", evt.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for deleted event")
	}
}

func TestWatchFromRVReplays(t *testing.T) {
	store := NewMemStore()
	svc := &clustercontrollerpb.ServiceDesiredVersion{
		Meta: &clustercontrollerpb.ObjectMeta{Name: "gateway"},
		Spec: &clustercontrollerpb.ServiceDesiredVersionSpec{ServiceName: "gateway", Version: "1.0.0"},
	}
	obj, err := store.Apply(context.Background(), "ServiceDesiredVersion", svc)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	applied := obj.(*clustercontrollerpb.ServiceDesiredVersion)
	from := applied.Meta.ResourceVersion

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := store.Watch(ctx, "ServiceDesiredVersion", "", from)
	if err != nil {
		t.Fatalf("watch: %v", err)
	}
	// Update status to produce a new event; watcher should see it because fromRV was previous RV.
	if _, err := store.UpdateStatus(context.Background(), "ServiceDesiredVersion", "gateway", &clustercontrollerpb.ObjectStatus{ObservedGeneration: applied.Meta.Generation}); err != nil {
		t.Fatalf("update status: %v", err)
	}
	select {
	case evt := <-ch:
		if evt.Type != EventModified {
			t.Fatalf("expected MODIFIED replay, got %s", evt.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for replay event")
	}
}
