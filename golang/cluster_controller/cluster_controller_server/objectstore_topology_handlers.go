// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.objectstore_topology_handlers
// @awareness file_role=typed_grpc_mutation_gate_for_objectstore_topology_apply
// @awareness implements=globular.platform:intent.controller.leader_election_gates_all_writes
// @awareness risk=high
package main

// objectstore_topology_handlers.go — typed gRPC handler for
// ApplyObjectStoreTopology.
//
// Applying an objectstore topology proposal used to be driven by the CLI writing
// an apply_request blob to etcd and polling an apply_result key the controller's
// background watcher wrote back (an etcd-mediated request/result handshake). That
// made the CLI a raw writer of controller-owned objectstore state (RT-2
// direct-write surface) and split the apply contract across an async channel.
//
// This handler routes the same operation through the owner's typed RPC: it is
// leader-gated (controller.leader_election_gates_all_writes,
// meta.competing_writers_must_converge_or_be_fenced — the apply watcher is
// likewise leader-only), loads the proposal by id authoritatively, and runs the
// exact same applyObjectStoreTopologyRequest contract the watcher invokes
// (transition pre-write, admission/validation, rollback on failure). The outcome
// is returned synchronously, so the nonce-matching poll loop disappears. The
// background apply_request watcher is retained as a compatibility drain for any
// older CLI that still writes the request key.

import (
	"context"
	"net"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	configpkg "github.com/globulario/services/golang/config"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// applyObjectStoreClock is overridable in tests so the request timestamp is
// deterministic; production uses the real wall clock via timestamppb.Now.
var applyObjectStoreClock = func() *timestamppb.Timestamp { return timestamppb.Now() }

// ApplyObjectStoreTopology applies a planned topology proposal through the owner
// path. Leader-gated; the proposal is re-loaded and re-validated by the
// controller (the CLI's pre-flight is advisory). Returns the apply outcome
// synchronously — status "accepted" with the new generation, or "failed" with
// the reason — so the caller no longer round-trips through etcd apply keys.
func (srv *server) ApplyObjectStoreTopology(ctx context.Context, req *cluster_controllerpb.ApplyObjectStoreTopologyRequest) (*cluster_controllerpb.ApplyObjectStoreTopologyResponse, error) {
	proposalID := req.GetProposalId()
	if proposalID == "" {
		return nil, status.Error(codes.InvalidArgument, "proposal_id is required")
	}
	if !srv.isLeader() {
		resp := &cluster_controllerpb.ApplyObjectStoreTopologyResponse{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/ApplyObjectStoreTopology", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}

	// Re-load the proposal authoritatively. A missing proposal is an explicit
	// NotFound (absence_scope_must_be_explicit), not a silent empty apply.
	proposal, err := configpkg.LoadTopologyProposal(ctx, proposalID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "load proposal %s: %v", proposalID, err)
	}

	applyReq := &configpkg.ObjectStoreApplyRequest{
		ProposalID:       proposalID,
		Proposal:         proposal,
		ForceDestructive: req.GetForceDestructive(),
		RequestedAt:      applyObjectStoreClock().AsTime(),
	}

	// Same contract the watcher runs: transition pre-write, admission/validation,
	// rollback on failure. The function never returns nil.
	result := srv.applyObjectStoreTopologyRequest(ctx, applyReq)

	return &cluster_controllerpb.ApplyObjectStoreTopologyResponse{
		Status:     result.Status,
		Generation: result.Generation,
		Error:      result.Error,
		ProposalId: proposalID,
	}, nil
}

// SanitizeObjectStorePool removes stale MinIO pool peers from the controller's
// authoritative state. The controller is the sole owner of
// /globular/clustercontroller/state: it read-modify-writes its FULL controllerState
// (no field loss) under the state lock and persists through persistStateLocked,
// which republishes objectstore desired state canonically. This replaces the CLI
// path that unmarshalled the state blob into a 4-field projection and raw-Put it
// back — silently clobbering every other field.
//
// Leader-gated (meta.competing_writers_must_converge_or_be_fenced). dry_run (or a
// no-op) returns the computed before/after/removed without mutating; applied is
// false. The result fields are always explicit (meta.silence_is_not_valid_for_unexpected).
func (srv *server) SanitizeObjectStorePool(ctx context.Context, req *cluster_controllerpb.SanitizeObjectStorePoolRequest) (*cluster_controllerpb.SanitizeObjectStorePoolResponse, error) {
	if !srv.isLeader() {
		resp := &cluster_controllerpb.SanitizeObjectStorePoolResponse{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/SanitizeObjectStorePool", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}

	srv.mu.Lock()
	defer srv.mu.Unlock()

	if srv.state == nil {
		return nil, status.Error(codes.FailedPrecondition, "controller state not initialized")
	}

	before := append([]string(nil), srv.state.MinioPoolNodes...)
	after, removed := sanitizeMinioPoolNodes(before, srv.state.Nodes)

	resp := &cluster_controllerpb.SanitizeObjectStorePoolResponse{
		Before:     before,
		After:      after,
		Removed:    removed,
		Generation: srv.state.ObjectStoreGeneration,
		Applied:    false,
	}

	// Nothing stale, or a preview: report the computed change without mutating.
	if len(removed) == 0 || req.GetDryRun() {
		return resp, nil
	}

	// Apply: mutate the full authoritative state and persist. persistStateLocked is
	// the durable-commit gate (etcd is authoritative) and republishes objectstore
	// desired state — so no separate hand-rolled recompute is needed.
	srv.state.MinioPoolNodes = after
	srv.state.ObjectStoreGeneration++
	if err := srv.persistStateLocked(true); err != nil {
		return nil, status.Errorf(codes.Internal, "persist sanitized state: %v", err)
	}
	resp.Generation = srv.state.ObjectStoreGeneration
	resp.Applied = true
	return resp, nil
}

// sanitizeMinioPoolNodes returns the MinIO pool with stale peers removed — pool
// IPs that no longer belong to an eligible (non removed/unreachable/blocked)
// cluster node — preserving order and de-duplicating. removed is the set of
// deduped pool entries dropped.
//
// If the node set is empty the pool is NOT pruned (only de-duplicated): an empty
// node map is "unknown", not "every peer is stale" — pruning on absent node
// information would wipe a live pool (meta.absence_scope_must_be_explicit).
func sanitizeMinioPoolNodes(pool []string, nodes map[string]*nodeState) (after, removed []string) {
	if len(pool) == 0 {
		return nil, nil
	}

	dedup := func(keep func(string) bool) (kept, dropped []string) {
		seen := make(map[string]struct{}, len(pool))
		for _, ip := range pool {
			if _, dup := seen[ip]; dup {
				continue
			}
			seen[ip] = struct{}{}
			if keep(ip) {
				kept = append(kept, ip)
			} else {
				dropped = append(dropped, ip)
			}
		}
		return kept, dropped
	}

	if len(nodes) == 0 {
		// Unknown node set: keep every syntactically valid, non-loopback IP.
		after, removed = dedup(func(ip string) bool {
			return net.ParseIP(ip) != nil && !configpkg.IsLoopbackEndpoint(ip)
		})
		return after, removed
	}

	allowed := make(map[string]struct{})
	for _, n := range nodes {
		if n == nil {
			continue
		}
		switch n.Status {
		case "removed", "unreachable", "blocked":
			continue
		}
		for _, ip := range n.Identity.Ips {
			if ip == "" || configpkg.IsLoopbackEndpoint(ip) {
				continue
			}
			allowed[ip] = struct{}{}
		}
	}
	after, removed = dedup(func(ip string) bool {
		_, ok := allowed[ip]
		return ok
	})
	return after, removed
}
