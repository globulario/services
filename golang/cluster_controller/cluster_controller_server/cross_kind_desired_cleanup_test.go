package main

// cross_kind_desired_cleanup_test.go — the audited cleanup path for legacy
// pre-guard cross-kind pollution. The cross-kind guard (desired.keyed_by_kind_and_name)
// prevents CREATING a ServiceDesiredVersion for an infrastructure-owned name, but
// offers no cleanup for entries written before it existed — the xds 1.2.235+492
// ghost that poisoned the doctor drift check after the infra-release was already
// healed to 1.2.237. pruneCrossKindServiceDesired is that cleanup.

import (
	"context"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_controller/resourcestore"
)

// seedInfraReleaseWithComponent seeds an InfrastructureRelease whose Spec.Component
// is set (the production shape — the cleanup classifies ownership by Component).
func seedInfraReleaseWithComponent(t *testing.T, store resourcestore.Store, component, version string) {
	t.Helper()
	obj := &cluster_controllerpb.InfrastructureRelease{
		Meta: &cluster_controllerpb.ObjectMeta{Name: defaultPublisherID() + "/" + component},
		Spec: &cluster_controllerpb.InfrastructureReleaseSpec{Component: component, Version: version},
	}
	if _, err := store.Apply(context.Background(), "InfrastructureRelease", obj); err != nil {
		t.Fatalf("seed InfrastructureRelease %q: %v", component, err)
	}
}

// TestPruneCrossKindServiceDesired removes the legacy cross-kind ServiceDesiredVersion
// for an infrastructure-owned name (xds) while preserving valid service-desired (sql).
func TestPruneCrossKindServiceDesired(t *testing.T) {
	store := resourcestore.NewMemStore()
	ctx := context.Background()

	seedInfraReleaseWithComponent(t, store, "xds", "1.2.237") // xds is owned by InfrastructureRelease
	seedServiceDesired(t, store, "xds", "1.2.235")            // legacy cross-kind pollution
	seedServiceDesired(t, store, "sql", "1.2.235")            // valid service-desired — must survive

	removed, err := pruneCrossKindServiceDesired(ctx, store)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if len(removed) != 1 || removed[0] != "xds" {
		t.Fatalf("expected exactly [xds] removed, got %v", removed)
	}
	if obj, _, _ := store.Get(ctx, "ServiceDesiredVersion", "xds"); obj != nil {
		t.Error("cross-kind ServiceDesiredVersion(xds) must be removed")
	}
	if obj, _, _ := store.Get(ctx, "ServiceDesiredVersion", "sql"); obj == nil {
		t.Error("valid ServiceDesiredVersion(sql) must be preserved")
	}
	// The InfrastructureRelease authority is untouched (no removal workflow).
	if obj, _, _ := store.Get(ctx, "InfrastructureRelease", defaultPublisherID()+"/xds"); obj == nil {
		t.Error("InfrastructureRelease(xds) must remain — cleanup removes only the invalid service-desired record")
	}
}

// TestPruneCrossKindServiceDesired_NoInfraOwnerDeletesNothing is the fail-safe:
// if no InfrastructureRelease exists, ownership cannot be classified, so the
// cleanup must delete nothing rather than guess.
func TestPruneCrossKindServiceDesired_NoInfraOwnerDeletesNothing(t *testing.T) {
	store := resourcestore.NewMemStore()
	ctx := context.Background()

	seedServiceDesired(t, store, "sql", "1.2.235")

	removed, err := pruneCrossKindServiceDesired(ctx, store)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if len(removed) != 0 {
		t.Fatalf("expected nothing removed when no InfrastructureRelease owner exists, got %v", removed)
	}
	if obj, _, _ := store.Get(ctx, "ServiceDesiredVersion", "sql"); obj == nil {
		t.Error("sql must be preserved (fail-safe: ownership cannot be classified)")
	}
}
