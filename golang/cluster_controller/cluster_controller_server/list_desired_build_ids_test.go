// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.list_desired_build_ids_test
// @awareness file_role=unit_test_for_list_desired_build_ids_typed_rpc
// @awareness enforces=globular.platform:invariant.four_layer.truth_read_via_owner_rpc_not_direct_storage
// @awareness enforces=globular.platform:invariant.repository.desired_build_id_is_hard_reachability_root
// @awareness risk=high
package main

import (
	"context"
	"sort"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_controller/resourcestore"
)

// TestListDesiredBuildIDs_UnionAcrossAllResourceKinds is the contract
// test for the v1.2.170 typed RPC that replaces direct etcd scans of
// /globular/resources/* in the repository and cluster_doctor.
//
// The RPC must include build_ids from every active desired-state
// record: SDV.Spec, ServiceRelease.{Spec,Status}, InfrastructureRelease.{Spec,Status},
// ApplicationRelease.{Spec,Status}. Empties are skipped, duplicates
// deduplicated.
func TestListDesiredBuildIDs_UnionAcrossAllResourceKinds(t *testing.T) {
	ctx := context.Background()
	store := resourcestore.NewMemStore()
	srv := &server{resources: store}

	mustApply := func(typ string, obj interface{}) {
		t.Helper()
		if _, err := store.Apply(ctx, typ, obj); err != nil {
			t.Fatalf("apply %s: %v", typ, err)
		}
	}

	// SDV with build_id "sdv-1".
	mustApply("ServiceDesiredVersion", &cluster_controllerpb.ServiceDesiredVersion{
		Meta: &cluster_controllerpb.ObjectMeta{Name: "echo"},
		Spec: &cluster_controllerpb.ServiceDesiredVersionSpec{
			ServiceName: "echo",
			Version:     "1.0.0",
			BuildID:     "sdv-1",
		},
	})

	// ServiceRelease: spec build_id "svc-1", status resolved "svc-resolved-1".
	mustApply("ServiceRelease", &cluster_controllerpb.ServiceRelease{
		Meta: &cluster_controllerpb.ObjectMeta{Name: "core@globular.io/echo"},
		Spec: &cluster_controllerpb.ServiceReleaseSpec{
			ServiceName: "echo",
			BuildID:     "svc-1",
		},
		Status: &cluster_controllerpb.ServiceReleaseStatus{
			ResolvedBuildID: "svc-resolved-1",
		},
	})

	// InfrastructureRelease: spec "infra-1", status "infra-resolved-1".
	mustApply("InfrastructureRelease", &cluster_controllerpb.InfrastructureRelease{
		Meta: &cluster_controllerpb.ObjectMeta{Name: "core@globular.io/etcd"},
		Spec: &cluster_controllerpb.InfrastructureReleaseSpec{
			Component: "etcd",
			BuildID:   "infra-1",
		},
		Status: &cluster_controllerpb.InfrastructureReleaseStatus{
			ResolvedBuildID: "infra-resolved-1",
		},
	})

	// ApplicationRelease: spec "app-1", status "app-resolved-1".
	mustApply("ApplicationRelease", &cluster_controllerpb.ApplicationRelease{
		Meta: &cluster_controllerpb.ObjectMeta{Name: "core@globular.io/console"},
		Spec: &cluster_controllerpb.ApplicationReleaseSpec{
			AppName: "console",
			BuildID: "app-1",
		},
		Status: &cluster_controllerpb.ApplicationReleaseStatus{
			ResolvedBuildID: "app-resolved-1",
		},
	})

	// Dedup case: a second SR with the same build_id should not produce a duplicate.
	mustApply("ServiceRelease", &cluster_controllerpb.ServiceRelease{
		Meta: &cluster_controllerpb.ObjectMeta{Name: "core@globular.io/echo-mirror"},
		Spec: &cluster_controllerpb.ServiceReleaseSpec{
			ServiceName: "echo-mirror",
			BuildID:     "svc-1", // duplicate
		},
		Status: &cluster_controllerpb.ServiceReleaseStatus{
			ResolvedBuildID: "svc-resolved-1", // duplicate
		},
	})

	// Empty build_ids must be skipped (no false-positive).
	mustApply("ServiceDesiredVersion", &cluster_controllerpb.ServiceDesiredVersion{
		Meta: &cluster_controllerpb.ObjectMeta{Name: "blank"},
		Spec: &cluster_controllerpb.ServiceDesiredVersionSpec{
			ServiceName: "blank",
			Version:     "0.0.0",
			BuildID:     "",
		},
	})

	resp, err := srv.ListDesiredBuildIDs(ctx, &cluster_controllerpb.ListDesiredBuildIDsRequest{})
	if err != nil {
		t.Fatalf("ListDesiredBuildIDs: %v", err)
	}

	got := append([]string(nil), resp.GetBuildIds()...)
	sort.Strings(got)
	want := []string{
		"app-1", "app-resolved-1",
		"infra-1", "infra-resolved-1",
		"sdv-1",
		"svc-1", "svc-resolved-1",
	}

	if len(got) != len(want) {
		t.Fatalf("build_ids count mismatch: got %d (%v) want %d (%v)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("build_ids[%d] = %q want %q (full got=%v)", i, got[i], want[i], got)
		}
	}
}

// TestListDesiredBuildIDs_NilResourceStore returns FailedPrecondition
// rather than panicking. Mirrors the listAllDesiredServices guard.
func TestListDesiredBuildIDs_NilResourceStore(t *testing.T) {
	srv := &server{}
	_, err := srv.ListDesiredBuildIDs(context.Background(), &cluster_controllerpb.ListDesiredBuildIDsRequest{})
	if err == nil {
		t.Fatalf("expected error when resources is nil, got nil")
	}
}

// TestGetDesiredState_PopulatesBuildId asserts the v1.2.172 proto
// extension: DesiredService.build_id MUST flow through GetDesiredState
// so globularcli (and other consumers) can read it via the typed RPC
// instead of scanning /globular/resources/ServiceDesiredVersion/* in
// etcd directly. Anchored by
// invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage.
func TestGetDesiredState_PopulatesBuildId(t *testing.T) {
	ctx := context.Background()
	store := resourcestore.NewMemStore()
	srv := &server{resources: store}

	mustApply := func(typ string, obj interface{}) {
		t.Helper()
		if _, err := store.Apply(ctx, typ, obj); err != nil {
			t.Fatalf("apply %s: %v", typ, err)
		}
	}

	mustApply("ServiceDesiredVersion", &cluster_controllerpb.ServiceDesiredVersion{
		Meta: &cluster_controllerpb.ObjectMeta{Name: "echo"},
		Spec: &cluster_controllerpb.ServiceDesiredVersionSpec{
			ServiceName: "echo",
			Version:     "1.2.3",
			BuildNumber: 7,
			BuildID:     "sdv-build-id-7",
		},
	})

	mustApply("InfrastructureRelease", &cluster_controllerpb.InfrastructureRelease{
		Meta: &cluster_controllerpb.ObjectMeta{Name: "core@globular.io/etcd"},
		Spec: &cluster_controllerpb.InfrastructureReleaseSpec{
			Component:   "etcd",
			Version:     "3.5.0",
			BuildNumber: 1,
			BuildID:     "infra-build-id-1",
		},
	})

	resp, err := srv.GetDesiredState(ctx, nil)
	if err != nil {
		t.Fatalf("GetDesiredState: %v", err)
	}

	byID := make(map[string]*cluster_controllerpb.DesiredService)
	for _, svc := range resp.GetServices() {
		byID[svc.GetServiceId()] = svc
	}

	if echo := byID["echo"]; echo == nil {
		t.Fatalf("echo missing from response: %+v", byID)
	} else if echo.GetBuildId() != "sdv-build-id-7" {
		t.Errorf("echo build_id = %q want %q", echo.GetBuildId(), "sdv-build-id-7")
	}
	if etcd := byID["etcd"]; etcd == nil {
		t.Fatalf("etcd missing from response: %+v", byID)
	} else if etcd.GetBuildId() != "infra-build-id-1" {
		t.Errorf("etcd build_id = %q want %q", etcd.GetBuildId(), "infra-build-id-1")
	}
}
