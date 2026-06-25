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
