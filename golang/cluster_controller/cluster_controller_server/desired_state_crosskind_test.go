// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.desired_state_crosskind_test
// @awareness file_role=unit_test_for_cross_kind_desired_write_guard
// @awareness enforces=globular.platform:invariant.desired.keyed_by_kind_and_name
// @awareness risk=high
package main

import (
	"context"
	"strings"
	"testing"

	"github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_controller/resourcestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Invariant desired.keyed_by_kind_and_name: a ServiceDesiredVersion is a
// SERVICE-kind record. Writing one for an infrastructure package (the
// `services desired set xds --force` cross-kind scar, incident a399ebea) must be
// refused at the operator RPC boundary, before any leader-forward or store write.
func TestUpsertRejectsCrossKindDesiredRecord(t *testing.T) {
	ctx := context.Background()
	srv := &server{resources: resourcestore.NewMemStore()}

	_, err := srv.UpsertDesiredService(ctx, &cluster_controllerpb.UpsertDesiredServiceRequest{
		Service: &cluster_controllerpb.DesiredService{ServiceId: "xds", Version: "1.2.235"},
	})
	if err == nil {
		t.Fatal("cross-kind desired write for infrastructure package 'xds' must be refused")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("want InvalidArgument, got %v (%v)", status.Code(err), err)
	}
	if !strings.Contains(err.Error(), "INFRASTRUCTURE") {
		t.Errorf("error should explain the cross-kind rejection, got: %v", err)
	}
}

// RemoveDesiredService is also a SERVICE-kind operation, so removing an
// infrastructure package through it is refused at the operator boundary too
// (infrastructure removal goes through InfrastructureRelease with spec.removing).
func TestRemoveDesiredServiceRejectsCrossKind(t *testing.T) {
	ctx := context.Background()
	srv := &server{resources: resourcestore.NewMemStore()}

	_, err := srv.RemoveDesiredService(ctx, &cluster_controllerpb.RemoveDesiredServiceRequest{ServiceId: "xds"})
	if err == nil {
		t.Fatal("removing infrastructure package 'xds' via RemoveDesiredService must be refused")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("want InvalidArgument, got %v (%v)", status.Code(err), err)
	}
}

// The guard is keyed by canonical kind from the component catalog: infrastructure
// and command packages are refused; workload services and packages absent from the
// catalog (third-party services) pass through (fail-open).
func TestRejectCrossKindDesiredWrite_ByCatalogKind(t *testing.T) {
	cases := []struct {
		name      string
		service   string
		wantError bool
	}{
		{"infrastructure xds refused", "xds", true},
		{"infrastructure etcd refused", "etcd", true},
		{"workload/absent echo passes", "echo", false},
		{"unknown name passes (fail-open)", "totally-not-a-package", false},
		{"empty name passes (reported later)", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := rejectCrossKindDesiredWrite(tc.service)
			if tc.wantError {
				if err == nil {
					t.Fatalf("expected %q to be refused as cross-kind", tc.service)
				}
				if status.Code(err) != codes.InvalidArgument {
					t.Errorf("want InvalidArgument, got %v", status.Code(err))
				}
			} else if err != nil {
				t.Fatalf("expected %q to pass the cross-kind guard, got: %v", tc.service, err)
			}
		})
	}
}
