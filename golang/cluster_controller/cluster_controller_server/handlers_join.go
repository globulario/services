package main

import (
	"context"
	"log"
	"sort"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/security"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (srv *server) CreateJoinToken(ctx context.Context, req *cluster_controllerpb.CreateJoinTokenRequest) (*cluster_controllerpb.CreateJoinTokenResponse, error) {
	if !srv.isLeader() {
		resp := &cluster_controllerpb.CreateJoinTokenResponse{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/CreateJoinToken", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}
	srv.lock("CreateJoinToken")
	defer srv.unlock()
	token := uuid.NewString()
	expiresAt := time.Now().Add(24 * time.Hour)
	if req != nil && req.ExpiresAt != nil {
		expiresAt = req.ExpiresAt.AsTime()
	}
	srv.state.JoinTokens[token] = &joinTokenRecord{
		Token:     token,
		ExpiresAt: expiresAt,
		MaxUses:   1,
	}
	if err := srv.persistStateLocked(true); err != nil {
		return nil, status.Errorf(codes.Internal, "persist token: %v", err)
	}
	return &cluster_controllerpb.CreateJoinTokenResponse{
		JoinToken: token,
		ExpiresAt: timestamppb.New(expiresAt),
	}, nil
}

func (srv *server) RequestJoin(ctx context.Context, req *cluster_controllerpb.RequestJoinRequest) (*cluster_controllerpb.RequestJoinResponse, error) {
	if !srv.isLeader() {
		resp := &cluster_controllerpb.RequestJoinResponse{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/RequestJoin", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}
	if req == nil || req.GetJoinToken() == "" {
		return nil, status.Error(codes.InvalidArgument, "join_token is required")
	}
	token := strings.TrimSpace(req.GetJoinToken())
	srv.lock("RequestJoin")
	jt := srv.state.JoinTokens[token]
	if jt == nil {
		srv.unlock()
		return nil, status.Error(codes.NotFound, "join token not found")
	}
	if time.Now().After(jt.ExpiresAt) {
		srv.unlock()
		return nil, status.Error(codes.PermissionDenied, "token expired")
	}
	if jt.Uses >= jt.MaxUses {
		srv.unlock()
		return nil, status.Error(codes.PermissionDenied, "token uses exhausted")
	}
	jt.Uses++
	reqID := uuid.NewString()
	caps := req.GetCapabilities()
	jr := &joinRequestRecord{
		RequestID:         reqID,
		Token:             token,
		Identity:          protoToStoredIdentity(req.GetIdentity()),
		Labels:            copyLabels(req.GetLabels()),
		RequestedAt:       time.Now(),
		Status:            "pending",
		Capabilities:      capsToStored(caps),
		SuggestedProfiles: deduceProfiles(caps, countNodesWithProfile(srv.state.Nodes, "storage")),
	}
	srv.state.JoinRequests[reqID] = jr

	// A valid token is proof the operator authorised this join. Auto-approve
	// immediately instead of queuing for manual review. The request stays
	// "pending" only if approveJoinRecordLocked somehow fails (shouldn't
	// happen), in which case the operator can approve manually.
	srv.approveJoinRecordLocked(jr, nil)

	if err := srv.persistStateLocked(true); err != nil {
		srv.unlock()
		return nil, status.Errorf(codes.Internal, "persist join request: %v", err)
	}
	srv.unlock()

	// Async side-effects (RBAC binding + bootstrap workflow trigger) must run
	// outside the lock because they re-acquire it internally.
	srv.postApproveJoinAsync(jr)

	return &cluster_controllerpb.RequestJoinResponse{
		RequestId: reqID,
		Status:    jr.Status,
		Message:   jr.statusMessage(),
	}, nil
}

func (srv *server) ListJoinRequests(ctx context.Context, req *cluster_controllerpb.ListJoinRequestsRequest) (*cluster_controllerpb.ListJoinRequestsResponse, error) {
	srv.lock("unknown")
	defer srv.unlock()
	resp := &cluster_controllerpb.ListJoinRequestsResponse{}
	pending := make([]*joinRequestRecord, 0, len(srv.state.JoinRequests))
	for _, jr := range srv.state.JoinRequests {
		if jr.Status != "pending" {
			continue
		}
		pending = append(pending, jr)
	}
	sort.Slice(pending, func(i, j int) bool {
		return pending[i].RequestedAt.Before(pending[j].RequestedAt)
	})
	for _, jr := range pending {
		resp.Pending = append(resp.Pending, &cluster_controllerpb.JoinRequestRecord{
			RequestId:         jr.RequestID,
			Identity:          storedIdentityToProto(jr.Identity),
			Status:            jr.Status,
			Profiles:          append([]string(nil), jr.Profiles...),
			Metadata:          copyLabels(jr.Labels),
			Capabilities:      storedToProtoCapabilities(jr.Capabilities),
			SuggestedProfiles: append([]string(nil), jr.SuggestedProfiles...),
		})
	}
	return resp, nil
}

func (srv *server) ApproveJoin(ctx context.Context, req *cluster_controllerpb.ApproveJoinRequest) (*cluster_controllerpb.ApproveJoinResponse, error) {
	if !srv.isLeader() {
		resp := &cluster_controllerpb.ApproveJoinResponse{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/ApproveJoin", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	reqID := strings.TrimSpace(req.GetRequestId())
	if reqID == "" {
		reqID = strings.TrimSpace(req.GetNodeId())
	}
	if reqID == "" {
		return nil, status.Error(codes.InvalidArgument, "request_id is required")
	}
	srv.lock("ApproveJoin")
	jr := srv.state.JoinRequests[reqID]
	if jr == nil {
		srv.unlock()
		return nil, status.Error(codes.NotFound, "join request not found")
	}
	if jr.Status != "pending" {
		srv.unlock()
		return nil, status.Error(codes.FailedPrecondition, "request not pending")
	}

	srv.approveJoinRecordLocked(jr, req.GetProfiles())

	if err := srv.persistStateLocked(true); err != nil {
		srv.unlock()
		return nil, status.Errorf(codes.Internal, "persist node state: %v", err)
	}
	srv.unlock()

	srv.postApproveJoinAsync(jr)

	return &cluster_controllerpb.ApproveJoinResponse{
		NodeId:        jr.AssignedNodeID,
		Message:       "approved; node will receive configuration on first heartbeat",
		NodeToken:     jr.NodeToken,
		NodePrincipal: jr.NodePrincipal,
	}, nil
}

// approveJoinRecordLocked performs the in-memory approval of a pending join
// request. It MUST be called with srv.lock held. It does NOT persist state or
// launch async tasks — callers are responsible for that.
//
// profiles may be nil/empty; in that case the suggested or default profiles are used.
func (srv *server) approveJoinRecordLocked(jr *joinRequestRecord, profiles []string) {
	if len(profiles) == 0 {
		profiles = jr.SuggestedProfiles
	}
	if len(profiles) == 0 {
		profiles = srv.cfg.DefaultProfiles
	}
	profiles = normalizeProfiles(profiles)

	// INVARIANT: The first 3 nodes MUST have foundational profiles
	// (core, control-plane, storage) to establish quorum for etcd,
	// ScyllaDB, and MinIO. Without 3 storage nodes, there is no
	// redundancy — MinIO becomes a single point of failure that
	// cascades into workflow execution and artifact publishing.
	storageCount := countNodesWithProfile(srv.state.Nodes, "storage")
	profiles = enforceFoundingProfiles(profiles, storageCount)
	jr.Profiles = profiles

	nodeID := deterministicNodeID(jr.Identity, jr.Labels)
	jr.AssignedNodeID = nodeID
	jr.Status = "approved"

	// Compute the node's advertised FQDN for DNS registration.
	// Format: <hostname>.<cluster-domain> (e.g. globule-dell.globular.internal)
	advertiseFqdn := ""
	if hostname := strings.TrimSpace(jr.Identity.Hostname); hostname != "" {
		domain := ""
		if srv.state.ClusterNetworkSpec != nil {
			domain = strings.TrimSuffix(strings.TrimSpace(srv.state.ClusterNetworkSpec.GetClusterDomain()), ".")
		}
		if domain != "" {
			advertiseFqdn = hostname + "." + domain
		}
	}

	node := &nodeState{
		NodeID:                nodeID,
		Identity:              jr.Identity,
		Profiles:              profiles,
		LastSeen:              time.Now(),
		Status:                "converging",
		Metadata:              copyLabels(jr.Labels),
		LastAppliedGeneration: 0,
		BootstrapPhase:        BootstrapAdmitted,
		BootstrapStartedAt:    time.Now(),
		AdvertiseFqdn:         advertiseFqdn,
	}
	srv.state.Nodes[nodeID] = node
	srv.removeStaleNodesLocked(nodeID, jr.Identity, "")

	nodePrincipal := "node_" + nodeID
	nodeToken, err := security.GenerateToken(
		365*24*60,    // 1 year TTL
		nodeID,       // audience = node ID
		nodePrincipal,
		"node-agent",
		"",
	)
	if err != nil {
		log.Printf("WARN: failed to generate node token for %s: %v", nodeID, err)
	} else {
		jr.NodeToken = nodeToken
		jr.NodePrincipal = nodePrincipal
	}
}

// postApproveJoinAsync launches the async side-effects after an approval:
// RBAC binding and bootstrap workflow trigger. Must be called WITHOUT the lock.
func (srv *server) postApproveJoinAsync(jr *joinRequestRecord) {
	nodeID := jr.AssignedNodeID
	nodePrincipal := jr.NodePrincipal

	if nodePrincipal != "" {
		go srv.ensureNodeExecutorBinding(nodePrincipal)
	}

	// Trigger the join workflow immediately if the node-agent is already
	// reachable. The "first heartbeat" trigger in ReportNodeStatus may miss
	// if the agent started heartbeating before approval (race condition).
	srv.lock("postApproveJoinAsync:triggerWorkflow")
	node := srv.state.Nodes[nodeID]
	if node != nil && !node.BootstrapWorkflowActive {
		node.BootstrapWorkflowActive = true
		agentEndpoint := node.AgentEndpoint
		log.Printf("ApproveJoin: triggering join workflow for %s at %s", nodeID, agentEndpoint)
		go srv.triggerJoinWorkflow(nodeID, agentEndpoint)
	}
	srv.unlock()
}

func (srv *server) RejectJoin(ctx context.Context, req *cluster_controllerpb.RejectJoinRequest) (*cluster_controllerpb.RejectJoinResponse, error) {
	if !srv.isLeader() {
		resp := &cluster_controllerpb.RejectJoinResponse{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/RejectJoin", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	reqID := strings.TrimSpace(req.GetRequestId())
	if reqID == "" {
		reqID = strings.TrimSpace(req.GetNodeId())
	}
	if reqID == "" {
		return nil, status.Error(codes.InvalidArgument, "request_id is required")
	}
	srv.lock("unknown")
	defer srv.unlock()
	jr := srv.state.JoinRequests[reqID]
	if jr == nil {
		return nil, status.Error(codes.NotFound, "join request not found")
	}
	if jr.Status != "pending" {
		return nil, status.Error(codes.FailedPrecondition, "request not pending")
	}
	jr.Status = "rejected"
	jr.Reason = req.GetReason()
	if err := srv.persistStateLocked(true); err != nil {
		return nil, status.Errorf(codes.Internal, "persist join request: %v", err)
	}
	return &cluster_controllerpb.RejectJoinResponse{
		NodeId:  jr.AssignedNodeID,
		Message: "rejected",
	}, nil
}

func (srv *server) cleanupJoinStateLocked(now time.Time) bool {
	dirty := false
	for token, jt := range srv.state.JoinTokens {
		if jt.MaxUses > 0 && jt.Uses >= jt.MaxUses {
			delete(srv.state.JoinTokens, token)
			dirty = true
			continue
		}
		if !jt.ExpiresAt.IsZero() && now.After(jt.ExpiresAt) {
			delete(srv.state.JoinTokens, token)
			dirty = true
		}
	}
	for reqID, jr := range srv.state.JoinRequests {
		if jr.Status == "pending" {
			if now.Sub(jr.RequestedAt) > pendingJoinRetention {
				delete(srv.state.JoinRequests, reqID)
				dirty = true
			}
			continue
		}
		if now.Sub(jr.RequestedAt) > joinRequestRetention {
			delete(srv.state.JoinRequests, reqID)
			dirty = true
		}
	}
	return dirty
}

func (srv *server) GetJoinRequestStatus(ctx context.Context, req *cluster_controllerpb.GetJoinRequestStatusRequest) (*cluster_controllerpb.GetJoinRequestStatusResponse, error) {
	if req == nil || req.GetRequestId() == "" {
		return nil, status.Error(codes.InvalidArgument, "request_id is required")
	}
	srv.lock("unknown")
	defer srv.unlock()
	jr := srv.state.JoinRequests[req.GetRequestId()]
	if jr == nil {
		return nil, status.Error(codes.NotFound, "join request not found")
	}
	return &cluster_controllerpb.GetJoinRequestStatusResponse{
		Status:        jr.Status,
		NodeId:        jr.AssignedNodeID,
		Message:       jr.Reason,
		Profiles:      append([]string(nil), jr.Profiles...),
		NodeToken:     jr.NodeToken,
		NodePrincipal: jr.NodePrincipal,
	}, nil
}
