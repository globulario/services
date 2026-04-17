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
	srv.lock("unknown")
	defer srv.unlock()
	jt := srv.state.JoinTokens[token]
	if jt == nil {
		return nil, status.Error(codes.NotFound, "join token not found")
	}
	if time.Now().After(jt.ExpiresAt) {
		return nil, status.Error(codes.PermissionDenied, "token expired")
	}
	if jt.Uses >= jt.MaxUses {
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
		SuggestedProfiles: deduceProfiles(caps),
	}
	srv.state.JoinRequests[reqID] = jr
	if err := srv.persistStateLocked(true); err != nil {
		return nil, status.Errorf(codes.Internal, "persist join request: %v", err)
	}
	return &cluster_controllerpb.RequestJoinResponse{
		RequestId: reqID,
		Status:    "pending",
		Message:   "pending approval",
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
	jr.Status = "approved"
	rawProfiles := req.GetProfiles()
	if len(rawProfiles) == 0 {
		rawProfiles = srv.cfg.DefaultProfiles
	}
	profiles := normalizeProfiles(rawProfiles)
	jr.Profiles = profiles
	nodeID := deterministicNodeID(jr.Identity, jr.Labels)
	jr.AssignedNodeID = nodeID

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

	// Create new node with current network generation
	node := &nodeState{
		NodeID:                nodeID,
		Identity:              jr.Identity,
		Profiles:              profiles,
		LastSeen:              time.Now(),
		Status:                "converging",
		Metadata:              copyLabels(jr.Labels),
		LastAppliedGeneration: 0, // New node hasn't applied any generation yet
		BootstrapPhase:        BootstrapAdmitted,
		BootstrapStartedAt:    time.Now(),
		AdvertiseFqdn:         advertiseFqdn,
	}
	srv.state.Nodes[nodeID] = node

	// Clean up stale nodes with the same hostname or IP.
	srv.removeStaleNodesLocked(nodeID, jr.Identity, "")

	// Generate node-scoped identity token
	nodePrincipal := "node_" + nodeID
	nodeToken, err := security.GenerateToken(
		365*24*60,                   // 1 year TTL
		nodeID,                      // audience = node ID
		nodePrincipal,               // principal_id = node_<uuid>
		"node-agent",                // display name
		"",                          // email (not applicable)
	)
	if err != nil {
		log.Printf("WARN: failed to generate node token for %s: %v", nodeID, err)
		// Non-fatal: node falls back to existing auth
	} else {
		jr.NodeToken = nodeToken
		jr.NodePrincipal = nodePrincipal
	}

	if err := srv.persistStateLocked(true); err != nil {
		srv.unlock()
		return nil, status.Errorf(codes.Internal, "persist node state: %v", err)
	}

	// Immediately dispatch initial plan with network config if node has endpoint
	// Note: New nodes won't have endpoint yet, so reconciliation loop will pick this up
	// when the node first reports status with its agent endpoint
	srv.unlock()
	// NOTE: lock is released here; remaining code below does not need the lock.

	// Create RBAC binding for node-executor role (best-effort, async)
	if nodePrincipal != "" {
		go srv.ensureNodeExecutorBinding(nodePrincipal)
	}

	// Trigger the join workflow immediately if the node-agent is already
	// reachable. The "first heartbeat" trigger in ReportNodeStatus may miss
	// if the agent started heartbeating before approval (race condition).
	// Agent endpoint comes from the node's heartbeat report (source of truth).
	// Do not construct it from IPs with a hardcoded port.
	agentEndpoint := ""
	srv.lock("ApproveJoin:triggerWorkflow")
	node = srv.state.Nodes[nodeID]
	if node != nil && !node.BootstrapWorkflowActive {
		if node.AgentEndpoint != "" {
			agentEndpoint = node.AgentEndpoint
		}
		node.BootstrapWorkflowActive = true
		log.Printf("ApproveJoin: triggering join workflow for %s at %s", nodeID, agentEndpoint)
		go srv.triggerJoinWorkflow(nodeID, agentEndpoint)
	}
	srv.unlock()

	return &cluster_controllerpb.ApproveJoinResponse{
		NodeId:        nodeID,
		Message:       "approved; node will receive configuration on first heartbeat",
		NodeToken:     jr.NodeToken,
		NodePrincipal: jr.NodePrincipal,
	}, nil
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
