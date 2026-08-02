package main

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_controller/resourcestore"
)

// TestCollectDesiredVersions_CarriesOverridePublisher guards the defect found on
// 2026-08-01.
//
// ServiceDesiredVersionSpec.PublisherID exists precisely so a local override can
// be resolved under its own identity lane — its doc comment says so. But
// collectDesiredVersions dropped the field and the drift-reconciler hardcoded
// defaultPublisherID(), so the resolver asked the repository for
//
//	core@globular.io/<svc>@<local-version>
//
// and got "no published artifact found". `globular pkg override` — the mechanism
// that exists to run a locally-built package — registered the override, reported
// drift against it on every node, and could never converge. Nothing errored
// loudly; the reconciler just logged a skip each cycle, forever.
//
// The publisher must survive the trip from desired state to resolution.
func TestCollectDesiredVersions_CarriesOverridePublisher(t *testing.T) {
	ctx := context.Background()
	store := resourcestore.NewMemStore()
	srv := &server{resources: store}

	const (
		svc       = "cluster-doctor"
		localPub  = "local@globular.internal"
		localVers = "1.2.290+local.milestone3.1"
	)

	if _, err := store.Apply(ctx, "ServiceDesiredVersion", &cluster_controllerpb.ServiceDesiredVersion{
		Meta: &cluster_controllerpb.ObjectMeta{Name: svc},
		Spec: &cluster_controllerpb.ServiceDesiredVersionSpec{
			ServiceName: svc,
			Version:     localVers,
			PublisherID: localPub,
		},
	}); err != nil {
		t.Fatalf("seed desired state: %v", err)
	}

	desired := srv.collectDesiredVersions(ctx)

	dv, ok := desired["SERVICE/"+svc]
	if !ok {
		t.Fatalf("desired state for %q not collected; got keys %v", svc, keysOfDesired(desired))
	}
	if dv.publisherID != localPub {
		t.Errorf("publisherID = %q, want %q.\n"+
			"Dropping it makes the drift-reconciler resolve the local artifact under the\n"+
			"official publisher, where it does not exist — the override never converges\n"+
			"and only logs a skip each cycle.", dv.publisherID, localPub)
	}
	if dv.version != localVers {
		t.Errorf("version = %q, want %q", dv.version, localVers)
	}
}

// TestCollectDesiredVersions_OfficialDesiredStateHasEmptyPublisher verifies the
// normal case is untouched.
//
// PublisherID is empty for every ordinary desired-state record, and empty must
// keep meaning "the official publisher", resolved at use time. If collection
// started substituting a concrete value here, the fallback would become
// unreachable and a later change to defaultPublisherID() would silently stop
// applying to desired state.
func TestCollectDesiredVersions_OfficialDesiredStateHasEmptyPublisher(t *testing.T) {
	ctx := context.Background()
	store := resourcestore.NewMemStore()
	srv := &server{resources: store}

	const svc = "cluster-doctor"
	if _, err := store.Apply(ctx, "ServiceDesiredVersion", &cluster_controllerpb.ServiceDesiredVersion{
		Meta: &cluster_controllerpb.ObjectMeta{Name: svc},
		Spec: &cluster_controllerpb.ServiceDesiredVersionSpec{
			ServiceName: svc,
			Version:     "1.2.290",
			BuildNumber: 1,
		},
	}); err != nil {
		t.Fatalf("seed desired state: %v", err)
	}

	dv, ok := srv.collectDesiredVersions(ctx)["SERVICE/"+svc]
	if !ok {
		t.Fatal("desired state not collected")
	}
	if dv.publisherID != "" {
		t.Errorf("publisherID = %q, want empty — an ordinary desired record names no "+
			"identity lane and must fall back to the official publisher at resolution time",
			dv.publisherID)
	}
}

func keysOfDesired(m map[string]desiredVersionInfo) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
