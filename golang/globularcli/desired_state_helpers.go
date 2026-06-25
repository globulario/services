package main

// desired_state_helpers.go — direct-etcd helpers for desired-state writes.
//
// platform-upgrade used to call upsertServiceDesiredVersion in a loop
// over every BOM package — that was the v1.2.159 incident. As of
// v1.2.160 platform-upgrade dispatches the platform.upgrade workflow
// instead, which writes ServiceDesiredVersion only after gated
// per-(node, package) decisions.
//
// These helpers remain because they're still used for single-package,
// operator-driven flows:
//   - upsertServiceDesiredVersion: pkg override apply/restore
//     (golang/globularcli/pkg_override_cmds.go) — explicit, one
//     package at a time, with an operator-supplied build_id.
//   - updateInfraReleaseVersion: `globular release set-version`
//     (golang/globularcli/release_cmds.go) — single-infra-package
//     equivalent of platform-upgrade, kept because the infra side
//     still has no per-(node, package) workflow yet.

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// desiredServiceClient is the controller RPC seam upsertServiceDesiredVersion uses
// to set desired state through the owner path; overridable in tests.
type desiredServiceClient interface {
	UpsertDesiredService(ctx context.Context, req *cluster_controllerpb.UpsertDesiredServiceRequest, opts ...grpc.CallOption) (*cluster_controllerpb.DesiredState, error)
}

var desiredServiceClientFactory = func(conn grpc.ClientConnInterface) desiredServiceClient {
	return cluster_controllerpb.NewClusterControllerServiceClient(conn)
}

// upsertServiceDesiredVersion sets a ServiceDesiredVersion through the controller's
// typed UpsertDesiredService RPC (the owner of /globular/resources), not a raw etcd
// write (RT-2). Used for single-package overrides where the operator has explicitly
// chosen a (name, version, build_id) tuple — build_id is the precise,
// repository-allocated identity. Publisher is tracked in the LocalOverride record;
// the governed DesiredService spec carries no publisher_id field. The controller
// bumps generation, validates the artifact against the repository, triggers
// reconcile, and audits the write server-side.
func upsertServiceDesiredVersion(serviceName, version string, buildNumber int64, buildID string) error {
	conn, err := controllerConnFactory()
	if err != nil {
		return fmt.Errorf("connect to controller: %w", err)
	}
	if c, ok := conn.(*grpc.ClientConn); ok && c != nil {
		defer func() { _ = c.Close() }()
	}
	client := desiredServiceClientFactory(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = client.UpsertDesiredService(ctx, &cluster_controllerpb.UpsertDesiredServiceRequest{
		Service: &cluster_controllerpb.DesiredService{
			ServiceId:   serviceName,
			Version:     version,
			BuildNumber: buildNumber,
			BuildId:     buildID,
		},
	}, jsonCallOption())
	if err != nil {
		return fmt.Errorf("upsert desired service %s: %w", serviceName, err)
	}
	return nil
}

// updateInfraReleaseVersion updates the spec.version of an
// InfrastructureRelease record in etcd. Used by `globular release
// set-version` for single-infra-package version pinning.
func updateInfraReleaseVersion(publisher, component, version string) error {
	conn, err := controllerConnFactory()
	if err != nil {
		return fmt.Errorf("connect to controller: %w", err)
	}
	if c, ok := conn.(*grpc.ClientConn); ok && c != nil {
		defer func() { _ = c.Close() }()
	}
	client := resourcesClientFactory(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	name := publisher + "/" + component

	// GET-modify-Apply through the owner's typed RPC. The previous implementation
	// raw-wrote /globular/resources/InfrastructureRelease/... directly to etcd,
	// bypassing the cluster-controller (the registered owner) and its desired-state
	// ownership guard. Route through ApplyInfrastructureRelease so the write is
	// governed (generation bump, ownership check, reconcile) instead of a raw put.
	// Read the current release first to preserve sibling spec fields (build_number,
	// channel, …) and change only the version — matching the prior read-modify-write.
	obj, err := client.GetInfrastructureRelease(ctx,
		&cluster_controllerpb.GetInfrastructureReleaseRequest{Name: name}, jsonCallOption())
	if err != nil || obj == nil {
		// Not present yet (or unreadable): create a fresh release.
		obj = &cluster_controllerpb.InfrastructureRelease{}
	}
	if obj.Meta == nil {
		obj.Meta = &cluster_controllerpb.ObjectMeta{}
	}
	obj.Meta.Name = name
	if obj.Spec == nil {
		obj.Spec = &cluster_controllerpb.InfrastructureReleaseSpec{}
	}
	obj.Spec.PublisherID = publisher
	obj.Spec.Component = component
	obj.Spec.Version = version

	if _, err := client.ApplyInfrastructureRelease(ctx,
		&cluster_controllerpb.ApplyInfrastructureReleaseRequest{Object: obj}, jsonCallOption()); err != nil {
		return fmt.Errorf("apply infrastructure release %s: %w", name, err)
	}
	return nil
}
