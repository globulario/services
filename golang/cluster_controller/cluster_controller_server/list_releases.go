// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.list_releases
// @awareness file_role=typed_grpc_handlers_for_full_release_objects_consumed_by_cluster_doctor_verification
// @awareness enforces=globular.platform:invariant.four_layer.truth_read_via_owner_rpc_not_direct_storage
// @awareness risk=high
package main

import (
	"context"
	"encoding/json"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ListServiceReleases returns every ServiceRelease object in the
// controller's resource store, JSON-encoded. cluster_doctor's
// verification step + future audit tools consume this; they need
// the FULL ServiceRelease (Spec, Status.Nodes, ResolvedBuildID,
// RequiredNodes, …) which the narrow GetDesiredState projection
// drops.
//
// Anchored by invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage:
// the cluster_controller owns /globular/resources/ServiceRelease/* —
// consumers MUST call this RPC instead of scanning etcd directly.
//
// Read-only; no leader-forwarding. The resource store is
// eventually consistent across followers via etcd watches.
func (srv *server) ListServiceReleasesJson(ctx context.Context, _ *cluster_controllerpb.ListServiceReleasesJsonRequest) (*cluster_controllerpb.ListServiceReleasesJsonResponse, error) {
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	items, _, err := srv.resources.List(ctx, "ServiceRelease", "")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list ServiceRelease: %v", err)
	}
	resp := &cluster_controllerpb.ListServiceReleasesJsonResponse{
		ReleasesJson: make([]string, 0, len(items)),
	}
	for _, obj := range items {
		rel, ok := obj.(*cluster_controllerpb.ServiceRelease)
		if !ok || rel == nil {
			continue
		}
		data, jErr := json.Marshal(rel)
		if jErr != nil {
			// Skip individual broken entries rather than failing
			// the whole RPC — matches the consumer's prior tolerance
			// for per-entry unmarshal failures.
			continue
		}
		resp.ReleasesJson = append(resp.ReleasesJson, string(data))
	}
	return resp, nil
}

// ListInfrastructureReleases is the parallel handler for
// /globular/resources/InfrastructureRelease/*.
func (srv *server) ListInfrastructureReleasesJson(ctx context.Context, _ *cluster_controllerpb.ListInfrastructureReleasesJsonRequest) (*cluster_controllerpb.ListInfrastructureReleasesJsonResponse, error) {
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	items, _, err := srv.resources.List(ctx, "InfrastructureRelease", "")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list InfrastructureRelease: %v", err)
	}
	resp := &cluster_controllerpb.ListInfrastructureReleasesJsonResponse{
		ReleasesJson: make([]string, 0, len(items)),
	}
	for _, obj := range items {
		rel, ok := obj.(*cluster_controllerpb.InfrastructureRelease)
		if !ok || rel == nil {
			continue
		}
		data, jErr := json.Marshal(rel)
		if jErr != nil {
			continue
		}
		resp.ReleasesJson = append(resp.ReleasesJson, string(data))
	}
	return resp, nil
}
