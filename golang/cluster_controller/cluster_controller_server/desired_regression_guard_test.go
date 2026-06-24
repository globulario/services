package main

// desired_regression_guard_test.go — D4: invariant desired.no_regression_all_paths.
//
// A desired-version write must not move below the floor = max(current desired,
// installed high) on ANY write path. The previous behaviour silently
// auto-corrected a too-low version upward; D4 replaces that with reject-by-default
// plus an explicit, audited allow_regression override — the same policy and helper
// on both the SERVICE path (enforceServiceDesiredFloor) and the infrastructure
// path (routeInfrastructureDesired). Governance with a keyhole, not a brick wall.

import (
	"context"
	"strings"
	"testing"

	"github.com/globulario/services/golang/audittrail"
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_controller/resourcestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func seedServiceDesired(t *testing.T, store resourcestore.Store, canon, version string) {
	t.Helper()
	obj := &cluster_controllerpb.ServiceDesiredVersion{
		Meta: &cluster_controllerpb.ObjectMeta{Name: canon},
		Spec: &cluster_controllerpb.ServiceDesiredVersionSpec{ServiceName: canon, Version: version},
	}
	if _, err := store.Apply(context.Background(), "ServiceDesiredVersion", obj); err != nil {
		t.Fatalf("seed ServiceDesiredVersion %q: %v", canon, err)
	}
}

// captureDesiredAudit swaps the writeDesiredAudit seam so a test can observe the
// dedicated regression-override action without an etcd backend.
func captureDesiredAudit(t *testing.T) *[]audittrail.DesiredWriteRecord {
	t.Helper()
	var recs []audittrail.DesiredWriteRecord
	orig := writeDesiredAudit
	writeDesiredAudit = func(_ context.Context, rec audittrail.DesiredWriteRecord) error {
		recs = append(recs, rec)
		return nil
	}
	t.Cleanup(func() { writeDesiredAudit = orig })
	return &recs
}

func assertOverrideAudited(t *testing.T, recs *[]audittrail.DesiredWriteRecord, service string) {
	t.Helper()
	for _, r := range *recs {
		if r.Action == "desired_regression_override" && r.Service == service {
			return
		}
	}
	t.Fatalf("expected a desired_regression_override audit record for %q; got %+v", service, *recs)
}

// ── Pure floor helpers ───────────────────────────────────────────────────────

func TestDesiredVersionFloor(t *testing.T) {
	cases := []struct{ cur, high, want string }{
		{"1.2.5", "1.2.3", "1.2.5"}, // current desired dominates
		{"1.2.3", "1.2.5", "1.2.5"}, // installed-high dominates
		{"", "1.0.0", "1.0.0"},      // no current desired
		{"1.0.0", "", "1.0.0"},      // no installed-high
		{"", "", ""},                // no floor at all
	}
	for _, c := range cases {
		if got := desiredVersionFloor(c.cur, c.high); got != c.want {
			t.Errorf("desiredVersionFloor(%q,%q)=%q want %q", c.cur, c.high, got, c.want)
		}
	}
}

func TestRegressesBelowFloor(t *testing.T) {
	if regressesBelowFloor("1.2.3", "") {
		t.Error("empty floor must never regress")
	}
	if !regressesBelowFloor("1.2.3", "1.2.5") {
		t.Error("1.2.3 is below floor 1.2.5")
	}
	if regressesBelowFloor("1.2.5", "1.2.5") {
		t.Error("at-floor is not a regression")
	}
	if regressesBelowFloor("1.2.6", "1.2.5") {
		t.Error("above-floor is not a regression")
	}
}

// ── Test 1: below-floor SERVICE write rejected by default ────────────────────

func TestDesiredRegression_ServiceBelowFloorRejected(t *testing.T) {
	store := resourcestore.NewMemStore()
	ctx := context.Background()
	seedServiceDesired(t, store, "echo", "1.2.5")
	srv := &server{resources: store}

	err := srv.enforceServiceDesiredFloor(ctx, "echo", "1.2.3", "", false)
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("below-floor SERVICE write must be FailedPrecondition; got %v", err)
	}
	// The error must name current floor, attempted version, and --allow-regression.
	for _, want := range []string{"1.2.5", "1.2.3", "allow-regression"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error must mention %q; got %v", want, err)
		}
	}
}

// ── Test 2: below-floor infra-route write rejected by default ────────────────

func TestDesiredRegression_InfraBelowFloorRejected(t *testing.T) {
	store := resourcestore.NewMemStore()
	ctx := context.Background()
	seedInfraRelease(t, store, "xds", "1.2.5")

	handled, err := routeInfrastructureDesired(ctx, store, "xds", "1.2.3", 0, false, "")
	if !handled {
		t.Fatal("INFRASTRUCTURE-managed name must be handled")
	}
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("below-floor infra write must be FailedPrecondition; got %v", err)
	}
	if !strings.Contains(err.Error(), "infrastructure") {
		t.Fatalf("infra refusal should say infrastructure; got %v", err)
	}
	// The infra record must NOT have regressed.
	got, _, _ := store.Get(ctx, "InfrastructureRelease", defaultPublisherID()+"/xds")
	if ir, ok := got.(*cluster_controllerpb.InfrastructureRelease); !ok || ir.Spec == nil || ir.Spec.Version != "1.2.5" {
		t.Fatalf("infra version must be unchanged at 1.2.5; got %+v", got)
	}
}

// ── Test 3: allow_regression permits below-floor SERVICE write and audits ────

func TestDesiredRegression_ServiceOverridePermitsAndAudits(t *testing.T) {
	store := resourcestore.NewMemStore()
	ctx := context.Background()
	seedServiceDesired(t, store, "echo", "1.2.5")
	srv := &server{resources: store}
	captured := captureDesiredAudit(t)

	if err := srv.enforceServiceDesiredFloor(ctx, "echo", "1.2.3", "", true); err != nil {
		t.Fatalf("allow_regression must permit a below-floor SERVICE write; got %v", err)
	}
	assertOverrideAudited(t, captured, "echo")
}

// ── Test 4: allow_regression permits below-floor infra-route write and audits ─

func TestDesiredRegression_InfraOverridePermitsAndAudits(t *testing.T) {
	store := resourcestore.NewMemStore()
	ctx := context.Background()
	seedInfraRelease(t, store, "xds", "1.2.5")
	captured := captureDesiredAudit(t)

	handled, err := routeInfrastructureDesired(ctx, store, "xds", "1.2.3", 7, true, "")
	if !handled || err != nil {
		t.Fatalf("allow_regression must permit a below-floor infra write; handled=%v err=%v", handled, err)
	}
	assertOverrideAudited(t, captured, "xds")
	// The override must have actually applied the lower version.
	got, _, _ := store.Get(ctx, "InfrastructureRelease", defaultPublisherID()+"/xds")
	if ir, ok := got.(*cluster_controllerpb.InfrastructureRelease); !ok || ir.Spec == nil || ir.Spec.Version != "1.2.3" {
		t.Fatalf("override should have applied 1.2.3; got %+v", got)
	}
}

// ── Test 5: same/above-floor writes still succeed ────────────────────────────

func TestDesiredRegression_AtOrAboveFloorSucceeds(t *testing.T) {
	store := resourcestore.NewMemStore()
	ctx := context.Background()
	seedServiceDesired(t, store, "echo", "1.2.5")
	srv := &server{resources: store}

	for _, v := range []string{"1.2.5", "1.2.6", "2.0.0"} {
		if err := srv.enforceServiceDesiredFloor(ctx, "echo", v, "", false); err != nil {
			t.Fatalf("at/above-floor %s must succeed; got %v", v, err)
		}
	}
	// installed-high also raises the floor: current desired 1.2.5 but a node runs
	// 1.4.0 — a write of 1.3.0 still regresses below the installed high-water mark.
	if err := srv.enforceServiceDesiredFloor(ctx, "echo", "1.3.0", "1.4.0", false); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("installed-high 1.4.0 must raise the floor above 1.3.0; got %v", err)
	}
}
