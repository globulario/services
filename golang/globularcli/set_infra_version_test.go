package main

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"google.golang.org/grpc"
)

// TestUpdateInfraReleaseVersion_RoutesThroughOwnerRPC proves the RT-2 migration:
// `release set-infra-version` no longer raw-writes etcd — it reads and applies the
// InfrastructureRelease through the cluster-controller's typed RPC (which passes
// through the desired-state ownership guard), preserving sibling spec fields and
// changing only the version.
func TestUpdateInfraReleaseVersion_RoutesThroughOwnerRPC(t *testing.T) {
	oldConn := controllerConnFactory
	oldFactory := resourcesClientFactory
	defer func() { controllerConnFactory = oldConn; resourcesClientFactory = oldFactory }()
	controllerConnFactory = func() (grpc.ClientConnInterface, error) { return nil, nil }

	t.Run("preserves sibling spec fields, updates only version", func(t *testing.T) {
		fc := &fakeResourcesClient{
			infraGet: &cluster_controllerpb.InfrastructureRelease{
				Meta: &cluster_controllerpb.ObjectMeta{Name: "core@globular.io/scylla"},
				Spec: &cluster_controllerpb.InfrastructureReleaseSpec{
					PublisherID: "core@globular.io", Component: "scylla",
					Version: "5.4.0", BuildNumber: 42, Channel: "stable",
				},
			},
		}
		resourcesClientFactory = func(grpc.ClientConnInterface) releaseResourcesClient { return fc }

		if err := updateInfraReleaseVersion("core@globular.io", "scylla", "5.4.1"); err != nil {
			t.Fatalf("updateInfraReleaseVersion: %v", err)
		}
		if fc.lastInfraApply == nil {
			t.Fatal("expected ApplyInfrastructureRelease (owner RPC) to be called — no raw etcd write")
		}
		spec := fc.lastInfraApply.Object.Spec
		if spec.Version != "5.4.1" {
			t.Errorf("version = %q, want 5.4.1", spec.Version)
		}
		if spec.BuildNumber != 42 || spec.Channel != "stable" {
			t.Errorf("sibling fields not preserved: build=%d channel=%q (want 42/stable)",
				spec.BuildNumber, spec.Channel)
		}
		if spec.PublisherID != "core@globular.io" || spec.Component != "scylla" {
			t.Errorf("identity fields wrong: %q/%q", spec.PublisherID, spec.Component)
		}
	})

	t.Run("creates a fresh release when not present", func(t *testing.T) {
		fc := &fakeResourcesClient{infraGet: nil} // GET returns nil → treated as not present
		resourcesClientFactory = func(grpc.ClientConnInterface) releaseResourcesClient { return fc }

		if err := updateInfraReleaseVersion("core@globular.io", "etcd", "3.5.14"); err != nil {
			t.Fatalf("updateInfraReleaseVersion: %v", err)
		}
		if fc.lastInfraApply == nil {
			t.Fatal("expected ApplyInfrastructureRelease to be called")
		}
		obj := fc.lastInfraApply.Object
		if v := obj.Spec.Version; v != "3.5.14" {
			t.Errorf("version = %q, want 3.5.14", v)
		}
		if obj.Spec.PublisherID != "core@globular.io" || obj.Spec.Component != "etcd" {
			t.Errorf("identity fields wrong: %q/%q", obj.Spec.PublisherID, obj.Spec.Component)
		}
		if name := obj.Meta.Name; name != "core@globular.io/etcd" {
			t.Errorf("meta.name = %q, want core@globular.io/etcd", name)
		}
	})
}
