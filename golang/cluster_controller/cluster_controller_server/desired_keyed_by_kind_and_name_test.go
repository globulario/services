package main

// desired_keyed_by_kind_and_name_test.go — D1: enforce invariant
// desired.keyed_by_kind_and_name at the controller desired-write gate.
//
// Kind is the resource type: ServiceDesiredVersion (SERVICE) vs
// InfrastructureRelease (INFRASTRUCTURE), each keyed by name. A SERVICE desired
// write for a name already managed as INFRASTRUCTURE must NOT create a cross-kind
// ServiceDesiredVersion ghost — the collision that fired the xds incident. It is
// routed to the InfrastructureRelease instead (the caller-side hard reject is D2).

import (
	"context"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_controller/resourcestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func seedInfraRelease(t *testing.T, store resourcestore.Store, name, version string) {
	t.Helper()
	obj := &cluster_controllerpb.InfrastructureRelease{
		Meta: &cluster_controllerpb.ObjectMeta{Name: defaultPublisherID() + "/" + name},
		Spec: &cluster_controllerpb.InfrastructureReleaseSpec{Version: version},
	}
	if _, err := store.Apply(context.Background(), "InfrastructureRelease", obj); err != nil {
		t.Fatalf("seed InfrastructureRelease %q: %v", name, err)
	}
}

// A SERVICE desired write for xds (managed as INFRASTRUCTURE) is routed to the
// InfrastructureRelease and creates NO cross-kind ServiceDesiredVersion ghost.
func TestDesiredKeyedByKindAndName_InfraNameRoutedNoServiceGhost(t *testing.T) {
	store := resourcestore.NewMemStore()
	ctx := context.Background()
	seedInfraRelease(t, store, "xds", "1.0.0")

	handled, err := routeInfrastructureDesired(ctx, store, "xds", "1.0.1", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatal("INFRASTRUCTURE-managed name must be handled (routed), not fall through to a SERVICE write")
	}
	// No cross-kind ghost.
	if obj, _, _ := store.Get(ctx, "ServiceDesiredVersion", "xds"); obj != nil {
		t.Fatal("cross-kind ghost: a ServiceDesiredVersion was created for an INFRASTRUCTURE name (desired.keyed_by_kind_and_name)")
	}
	// Routed: the InfrastructureRelease was bumped to the requested version.
	got, _, gerr := store.Get(ctx, "InfrastructureRelease", defaultPublisherID()+"/xds")
	if gerr != nil || got == nil {
		t.Fatalf("InfrastructureRelease missing after route: %v", gerr)
	}
	ir, ok := got.(*cluster_controllerpb.InfrastructureRelease)
	if !ok || ir.Spec == nil || ir.Spec.Version != "1.0.1" {
		t.Fatalf("InfrastructureRelease must be bumped to 1.0.1; got %+v", got)
	}
}

// A same-kind SERVICE write (no InfrastructureRelease for the name) is NOT
// handled by the infra router — the caller proceeds to write the
// ServiceDesiredVersion. Preserves valid same-kind writes.
func TestDesiredKeyedByKindAndName_ServiceNamePassesThrough(t *testing.T) {
	store := resourcestore.NewMemStore()
	handled, err := routeInfrastructureDesired(context.Background(), store, "echo", "1.2.0", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handled {
		t.Fatal("a name with no InfrastructureRelease must not be handled by the infra router — the same-kind SERVICE write must proceed")
	}
}

// An INFRASTRUCTURE record that exists but is unreadable (nil Spec) is refused
// with a typed FailedPrecondition, not ghosted.
func TestDesiredKeyedByKindAndName_UnreadableInfraRefused(t *testing.T) {
	store := resourcestore.NewMemStore()
	ctx := context.Background()
	bad := &cluster_controllerpb.InfrastructureRelease{
		Meta: &cluster_controllerpb.ObjectMeta{Name: defaultPublisherID() + "/broken"},
		// Spec intentionally nil — unreadable.
	}
	if _, err := store.Apply(ctx, "InfrastructureRelease", bad); err != nil {
		t.Fatalf("seed: %v", err)
	}
	handled, err := routeInfrastructureDesired(ctx, store, "broken", "1.0.0", 1)
	if !handled {
		t.Fatal("unreadable INFRASTRUCTURE record must be handled (refused), not fall through")
	}
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("want FailedPrecondition for unreadable infra, got %v", err)
	}
}
