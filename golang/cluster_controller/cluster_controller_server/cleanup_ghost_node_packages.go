// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.cleanup_ghost_node_packages
// @awareness file_role=typed_grpc_handler_for_ghost_node_package_cleanup
// @awareness enforces=globular.platform:invariant.four_layer.truth_read_via_owner_rpc_not_direct_storage
// @awareness enforces=globular.platform:invariant.destructive_actions.require_explicit_guard
// @awareness risk=critical
package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CleanupGhostNodePackages removes installed-package records under
// /globular/nodes/{node_id}/packages/ for a node that is no longer a
// member of the cluster. Replaces the prior globularcli
// cleanupGhostNodes path that issued cli.Delete directly against a
// non-owner prefix.
//
// Validation: the handler refuses to clean up a node_id that is
// currently listed in the controller's node registry. This guards
// against an operator typo wiping an active node's installed-state
// records. The refusal is surfaced as refused_active_node=true with
// deleted=0; callers can present a clearer error than a generic
// permission-denied.
//
// Leader-forwarded so destructive deletes happen on a single actor
// (matches the broader controller convention for state-mutating
// RPCs).
//
// Anchored by:
//
//	invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage
//	invariant:destructive_actions.require_explicit_guard
func (srv *server) CleanupGhostNodePackages(ctx context.Context, req *cluster_controllerpb.CleanupGhostNodePackagesRequest) (*cluster_controllerpb.CleanupGhostNodePackagesResponse, error) {
	if !srv.isLeader() {
		resp := &cluster_controllerpb.CleanupGhostNodePackagesResponse{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/CleanupGhostNodePackages", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}

	nodeID := strings.TrimSpace(req.GetNodeId())
	if nodeID == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}
	if srv.etcdClient == nil {
		return nil, status.Error(codes.FailedPrecondition, "etcd client unavailable")
	}

	// Validate that nodeID is NOT a current member. ListNodes is
	// the owner's typed read of the node registry; reusing it here
	// keeps the cleanup decision aligned with the same authority
	// the rest of the cluster sees.
	listCtx, listCancel := context.WithTimeout(ctx, 10*time.Second)
	nodesResp, err := srv.ListNodes(listCtx, &cluster_controllerpb.ListNodesRequest{})
	listCancel()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list nodes: %v", err)
	}
	registeredIDs := make([]string, 0, len(nodesResp.GetNodes()))
	for _, n := range nodesResp.GetNodes() {
		registeredIDs = append(registeredIDs, n.GetNodeId())
	}
	switch classifyGhostCleanup(registeredIDs, nodeID) {
	case ghostCleanupRefuseEmptyRegistry:
		// An empty-but-successful registry read is UNKNOWN authority, not proof
		// the node is a ghost. Deleting on it would wipe a live node's Layer-3
		// records during a registry blip. Absence is not permission to erase.
		log.Printf("cleanup-ghost: REFUSED — node registry returned zero nodes; cannot classify ghost packages from an empty authority set (node %s)", short8(nodeID))
		return nil, status.Error(codes.FailedPrecondition,
			"ghost cleanup refused: node registry returned zero nodes; refusing destructive cleanup on empty authority")
	case ghostCleanupRefuseActiveMember:
		log.Printf("cleanup-ghost: refusing to clean active node %s", short8(nodeID))
		return &cluster_controllerpb.CleanupGhostNodePackagesResponse{
			Deleted:           0,
			RefusedActiveNode: true,
		}, nil
	}

	prefix := fmt.Sprintf("/globular/nodes/%s/packages/", nodeID)
	delCtx, delCancel := context.WithTimeout(ctx, 10*time.Second)
	defer delCancel()
	delResp, err := srv.etcdClient.Delete(delCtx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "delete ghost packages: %v", err)
	}
	log.Printf("cleanup-ghost: deleted %d package records for ghost node %s",
		delResp.Deleted, short8(nodeID))
	return &cluster_controllerpb.CleanupGhostNodePackagesResponse{
		Deleted: int32(delResp.Deleted),
	}, nil
}

// short8 returns the first 8 chars of s for log lines. Avoids
// the [:8] panic when callers pass shorter ids.
func short8(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:8]
}

// ghostCleanupDecision is the classification of a ghost-package cleanup request
// against the current node registry.
type ghostCleanupDecision int

const (
	ghostCleanupAllow ghostCleanupDecision = iota
	// ghostCleanupRefuseEmptyRegistry: the registry read returned zero nodes —
	// UNKNOWN authority, not proof the target is a ghost. Refuse (fail closed).
	ghostCleanupRefuseEmptyRegistry
	// ghostCleanupRefuseActiveMember: the target is a current registry member —
	// not a ghost. Refuse.
	ghostCleanupRefuseActiveMember
)

// classifyGhostCleanup decides whether ghost-package cleanup for targetNodeID is
// allowed. An empty node registry is uncertainty, not permission to erase, so it
// refuses; a target that is still a listed member refuses; otherwise allow.
func classifyGhostCleanup(registeredNodeIDs []string, targetNodeID string) ghostCleanupDecision {
	if len(registeredNodeIDs) == 0 {
		return ghostCleanupRefuseEmptyRegistry
	}
	for _, id := range registeredNodeIDs {
		if strings.TrimSpace(id) == targetNodeID {
			return ghostCleanupRefuseActiveMember
		}
	}
	return ghostCleanupAllow
}
