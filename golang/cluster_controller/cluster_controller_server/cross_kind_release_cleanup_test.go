package main

// cross_kind_release_cleanup_test.go — regression for the xds cache_digest_mismatch
// loop whose true faucet was a legacy cross-kind *ServiceRelease* (not a
// ServiceDesiredVersion). PR #154 closed the ServiceDesiredVersion faucet, but
// ensureServiceReleasesFromDesired "only creates releases, does not clean up", so
// a SERVICE-kind ServiceRelease for an infrastructure-owned name (xds@1.2.235)
// survived and kept dispatching SERVICE-kind installs from a stale pinned tarball,
// repeatedly clobbering the canonical INFRASTRUCTURE/xds state. These tests pin
// the release-side cleanup (seam 1) and the create-path reconcile-delete (seam 2).
//
// Disk-truth cleanup on the node-agent (cleanupStaleKindsByDiskTruth) is NOT the
// bug and is deliberately untouched — it was fed poisoned evidence by this faucet.

import (
	"context"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_controller/resourcestore"
)

// seedServiceReleaseSpec seeds a ServiceRelease keyed "<publisher>/<canon>" with
// Spec.ServiceName set (the production shape the cleanup classifies by).
func seedServiceReleaseSpec(t *testing.T, store resourcestore.Store, canon, version string) {
	t.Helper()
	obj := &cluster_controllerpb.ServiceRelease{
		Meta: &cluster_controllerpb.ObjectMeta{Name: defaultPublisherID() + "/" + canon},
		Spec: &cluster_controllerpb.ServiceReleaseSpec{ServiceName: canon, Version: version},
	}
	if _, err := store.Apply(context.Background(), "ServiceRelease", obj); err != nil {
		t.Fatalf("seed ServiceRelease %q: %v", canon, err)
	}
}

// TestPruneCrossKindServiceReleases (seam 1) removes the legacy cross-kind
// ServiceRelease for an infrastructure-owned name (xds) while preserving a valid
// service release (sql) and the InfrastructureRelease authority itself.
func TestPruneCrossKindServiceReleases(t *testing.T) {
	store := resourcestore.NewMemStore()
	ctx := context.Background()

	seedInfraReleaseWithComponent(t, store, "xds", "1.2.237") // xds owned by InfrastructureRelease
	seedServiceReleaseSpec(t, store, "xds", "1.2.235")        // the cross-kind faucet
	seedServiceReleaseSpec(t, store, "sql", "1.2.235")        // valid service release — must survive

	removed, err := pruneCrossKindServiceReleases(ctx, store)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if len(removed) != 1 || removed[0] != "xds" {
		t.Fatalf("expected exactly [xds] removed, got %v", removed)
	}
	if obj, _, _ := store.Get(ctx, "ServiceRelease", defaultPublisherID()+"/xds"); obj != nil {
		t.Error("cross-kind ServiceRelease(xds) must be removed — it is the install faucet")
	}
	if obj, _, _ := store.Get(ctx, "ServiceRelease", defaultPublisherID()+"/sql"); obj == nil {
		t.Error("valid ServiceRelease(sql) must be preserved")
	}
	// The InfrastructureRelease authority is untouched (no removal workflow).
	if obj, _, _ := store.Get(ctx, "InfrastructureRelease", defaultPublisherID()+"/xds"); obj == nil {
		t.Error("InfrastructureRelease(xds) must remain — cleanup removes only the invalid service release")
	}
}

// TestPruneCrossKindServiceReleases_CatalogIsOracleWithoutInfraRelease is the
// model regression for #160: kind is classified by the component catalog (the
// SAME oracle as the write guard), NOT by "does an InfrastructureRelease object
// exist". The catalog knows xds is INFRASTRUCTURE whether or not an
// InfrastructureRelease has been created yet, so the cross-kind faucet is closed
// even in the bootstrap / join / backup-restore window where the old
// InfrastructureRelease-existence proxy went inert and let the ghost survive.
func TestPruneCrossKindServiceReleases_CatalogIsOracleWithoutInfraRelease(t *testing.T) {
	store := resourcestore.NewMemStore()
	ctx := context.Background()

	// NO InfrastructureRelease seeded — the old proxy would have deleted nothing.
	seedServiceReleaseSpec(t, store, "xds", "1.2.235")

	removed, err := pruneCrossKindServiceReleases(ctx, store)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if len(removed) != 1 || removed[0] != "xds" {
		t.Fatalf("expected [xds] pruned by catalog oracle without an InfrastructureRelease, got %v", removed)
	}
	if obj, _, _ := store.Get(ctx, "ServiceRelease", defaultPublisherID()+"/xds"); obj != nil {
		t.Error("xds ServiceRelease must be pruned — catalog says INFRASTRUCTURE regardless of InfrastructureRelease existence")
	}
}

// TestPruneCrossKindServiceReleases_ThirdPartyNotInCatalogKept is the fail-open:
// a name absent from the component catalog is treated as a service and must never
// be pruned, so third-party services are unaffected.
func TestPruneCrossKindServiceReleases_ThirdPartyNotInCatalogKept(t *testing.T) {
	store := resourcestore.NewMemStore()
	ctx := context.Background()

	const thirdParty = "acme-thirdparty-service"
	if CatalogByName(thirdParty) != nil {
		t.Fatalf("test premise broken: %q unexpectedly present in catalog", thirdParty)
	}
	seedServiceReleaseSpec(t, store, thirdParty, "1.0.0")

	removed, err := pruneCrossKindServiceReleases(ctx, store)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if len(removed) != 0 {
		t.Fatalf("expected nothing pruned for a non-catalog third-party name, got %v", removed)
	}
	if obj, _, _ := store.Get(ctx, "ServiceRelease", defaultPublisherID()+"/"+thirdParty); obj == nil {
		t.Error("third-party ServiceRelease (not in catalog) must be preserved (fail-open)")
	}
}

// TestDeleteCrossKindServiceRelease (seam 2) covers the create-path reconcile:
// an infra-managed name must not retain a ServiceRelease — skipping create is not
// enough, the pre-existing one is actively removed. Absent release is a no-op.
func TestDeleteCrossKindServiceRelease(t *testing.T) {
	store := resourcestore.NewMemStore()
	ctx := context.Background()
	srv := &server{resources: store}

	seedServiceReleaseSpec(t, store, "xds", "1.2.235")

	srv.deleteCrossKindServiceRelease(ctx, "xds")
	if obj, _, _ := store.Get(ctx, "ServiceRelease", defaultPublisherID()+"/xds"); obj != nil {
		t.Error("infra-managed name must not retain a ServiceRelease after create-path reconcile")
	}

	// No-op when the release is absent — must not error or fabricate state.
	srv.deleteCrossKindServiceRelease(ctx, "xds")
	if obj, _, _ := store.Get(ctx, "ServiceRelease", defaultPublisherID()+"/xds"); obj != nil {
		t.Error("delete of an absent ServiceRelease must remain a no-op")
	}
}
