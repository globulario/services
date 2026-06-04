package main

import (
	"context"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_controller/resourcestore"
)

// Regression guard for the etcd-bloat incident captured in
// docs/awareness/reports/etcd_bloat_investigation_2026-06-03.md.
// publishWaveState used to call resources.Apply unconditionally on every
// invocation, so workflow re-dispatches against the same release (e.g. the
// envoy restart-storm pattern) produced one MVCC revision per call. The
// envoy InfrastructureRelease accumulated ~99K versions from this writer
// alone before the equality guard landed.

func TestPublishWaveStateInfraSkipsApplyWhenMessageUnchanged(t *testing.T) {
	base := resourcestore.NewMemStore()
	srv := &server{}

	ctx := context.Background()

	rel := &cluster_controllerpb.InfrastructureRelease{
		Meta:   &cluster_controllerpb.ObjectMeta{Name: "core@globular.io/envoy"},
		Spec:   &cluster_controllerpb.InfrastructureReleaseSpec{Component: "envoy"},
		Status: &cluster_controllerpb.InfrastructureReleaseStatus{},
	}
	if _, err := base.Apply(ctx, "InfrastructureRelease", rel); err != nil {
		t.Fatalf("pre-populate: %v", err)
	}

	counting := &applyCountStore{Store: base}
	srv.resources = counting

	if err := srv.publishWaveState(ctx, rel.Meta.Name, "INFRASTRUCTURE", waveStateRunning, 2, 5, 3, ""); err != nil {
		t.Fatalf("first publish: %v", err)
	}
	if counting.count != 1 {
		t.Fatalf("first publish should write once, got %d Apply calls", counting.count)
	}

	if err := srv.publishWaveState(ctx, rel.Meta.Name, "INFRASTRUCTURE", waveStateRunning, 2, 5, 3, ""); err != nil {
		t.Fatalf("second publish: %v", err)
	}
	if counting.count != 1 {
		t.Fatalf("identical second publish must skip Apply (got %d, want 1)", counting.count)
	}

	if err := srv.publishWaveState(ctx, rel.Meta.Name, "INFRASTRUCTURE", waveStateCommitted, 2, 5, 3, ""); err != nil {
		t.Fatalf("third publish: %v", err)
	}
	if counting.count != 2 {
		t.Fatalf("changed wave state must trigger Apply (got %d, want 2)", counting.count)
	}
}

func TestPublishWaveStateServiceSkipsApplyWhenMessageAndReasonUnchanged(t *testing.T) {
	base := resourcestore.NewMemStore()
	srv := &server{}

	ctx := context.Background()

	rel := &cluster_controllerpb.ServiceRelease{
		Meta:   &cluster_controllerpb.ObjectMeta{Name: "core@globular.io/echo"},
		Spec:   &cluster_controllerpb.ServiceReleaseSpec{ServiceName: "echo"},
		Status: &cluster_controllerpb.ServiceReleaseStatus{},
	}
	if _, err := base.Apply(ctx, "ServiceRelease", rel); err != nil {
		t.Fatalf("pre-populate: %v", err)
	}

	counting := &applyCountStore{Store: base}
	srv.resources = counting

	if err := srv.publishWaveState(ctx, rel.Meta.Name, "SERVICE", waveStateRunning, 2, 5, 3, ""); err != nil {
		t.Fatalf("first publish: %v", err)
	}
	if counting.count != 1 {
		t.Fatalf("first publish should write once, got %d Apply calls", counting.count)
	}

	if err := srv.publishWaveState(ctx, rel.Meta.Name, "SERVICE", waveStateRunning, 2, 5, 3, ""); err != nil {
		t.Fatalf("second publish: %v", err)
	}
	if counting.count != 1 {
		t.Fatalf("identical second publish must skip Apply (got %d, want 1)", counting.count)
	}

	if err := srv.publishWaveState(ctx, rel.Meta.Name, "SERVICE", waveStateBlocked, 2, 5, 3, "scylla unavailable"); err != nil {
		t.Fatalf("third publish: %v", err)
	}
	if counting.count != 2 {
		t.Fatalf("changed wave state must trigger Apply (got %d, want 2)", counting.count)
	}
}
