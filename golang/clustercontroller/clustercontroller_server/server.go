package main

import (
	"context"
	"strings"
	"sync"
	"time"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type server struct {
	clustercontrollerpb.UnimplementedClusterControllerServiceServer

	cfg       *clusterControllerConfig
	cfgPath   string
	statePath string
	state     *controllerState
	mu        sync.Mutex
}

func newServer(cfg *clusterControllerConfig, cfgPath, statePath string, state *controllerState) *server {
	if state == nil {
		state = newControllerState()
	}
	if statePath == "" {
		statePath = defaultClusterStatePath
	}
	return &server{
		cfg:       cfg,
		cfgPath:   cfgPath,
		statePath: statePath,
		state:     state,
	}
}

func (srv *server) GetClusterInfo(ctx context.Context, req *timestamppb.Timestamp) (*clustercontrollerpb.ClusterInfo, error) {
	info := &clustercontrollerpb.ClusterInfo{
		ClusterDomain: srv.cfg.ClusterDomain,
		ClusterId:     srv.cfg.ClusterDomain,
		CreatedAt:     timestamppb.Now(),
	}
	return info, nil
}

func (srv *server) CreateJoinToken(ctx context.Context, req *clustercontrollerpb.CreateJoinTokenRequest) (*clustercontrollerpb.CreateJoinTokenResponse, error) {
	srv.mu.Lock()
	defer srv.mu.Unlock()
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
	if err := srv.state.save(srv.statePath); err != nil {
		return nil, status.Errorf(codes.Internal, "persist token: %v", err)
	}
	return &clustercontrollerpb.CreateJoinTokenResponse{
		JoinToken: token,
		ExpiresAt: timestamppb.New(expiresAt),
	}, nil
}

func (srv *server) RequestJoin(ctx context.Context, req *clustercontrollerpb.RequestJoinRequest) (*clustercontrollerpb.RequestJoinResponse, error) {
	if req == nil || req.GetJoinToken() == "" {
		return nil, status.Error(codes.InvalidArgument, "join_token is required")
	}
	token := strings.TrimSpace(req.GetJoinToken())
	srv.mu.Lock()
	defer srv.mu.Unlock()
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
	srv.state.JoinRequests[reqID] = &joinRequestRecord{
		RequestID:   reqID,
		Token:       token,
		Identity:    protoToStoredIdentity(req.GetIdentity()),
		Labels:      copyLabels(req.GetLabels()),
		RequestedAt: time.Now(),
		Status:      "pending",
	}
	if err := srv.state.save(srv.statePath); err != nil {
		return nil, status.Errorf(codes.Internal, "persist join request: %v", err)
	}
	return &clustercontrollerpb.RequestJoinResponse{
		NodeId:  reqID,
		Status:  "pending",
		Message: "pending approval",
	}, nil
}

func (srv *server) ListJoinRequests(ctx context.Context, req *clustercontrollerpb.ListJoinRequestsRequest) (*clustercontrollerpb.ListJoinRequestsResponse, error) {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	resp := &clustercontrollerpb.ListJoinRequestsResponse{}
	for _, jr := range srv.state.JoinRequests {
		if jr.Status != "pending" {
			continue
		}
		resp.Pending = append(resp.Pending, &clustercontrollerpb.NodeRecord{
			NodeId:   jr.RequestID,
			Identity: storedIdentityToProto(jr.Identity),
			Status:   jr.Status,
			Metadata: jr.Labels,
		})
	}
	return resp, nil
}

func (srv *server) ApproveJoin(ctx context.Context, req *clustercontrollerpb.ApproveJoinRequest) (*clustercontrollerpb.ApproveJoinResponse, error) {
	if req == nil || req.GetNodeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id (request id) is required")
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	jr := srv.state.JoinRequests[req.GetNodeId()]
	if jr == nil {
		return nil, status.Error(codes.NotFound, "join request not found")
	}
	if jr.Status != "pending" {
		return nil, status.Error(codes.FailedPrecondition, "request not pending")
	}
	jr.Status = "approved"
	profiles := req.GetProfiles()
	if len(profiles) == 0 {
		profiles = srv.cfg.DefaultProfiles
	}
	nodeID := uuid.NewString()
	srv.state.Nodes[nodeID] = &nodeState{
		NodeID:   nodeID,
		Identity: jr.Identity,
		Profiles: append([]string(nil), profiles...),
		LastSeen: time.Now(),
		Status:   "ready",
		Metadata: copyLabels(jr.Labels),
	}
	if err := srv.state.save(srv.statePath); err != nil {
		return nil, status.Errorf(codes.Internal, "persist node state: %v", err)
	}
	return &clustercontrollerpb.ApproveJoinResponse{
		NodeId:  nodeID,
		Message: "approved",
	}, nil
}

func (srv *server) RejectJoin(ctx context.Context, req *clustercontrollerpb.RejectJoinRequest) (*clustercontrollerpb.RejectJoinResponse, error) {
	if req == nil || req.GetNodeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id (request id) is required")
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	jr := srv.state.JoinRequests[req.GetNodeId()]
	if jr == nil {
		return nil, status.Error(codes.NotFound, "join request not found")
	}
	if jr.Status != "pending" {
		return nil, status.Error(codes.FailedPrecondition, "request not pending")
	}
	jr.Status = "rejected"
	jr.Reason = req.GetReason()
	if err := srv.state.save(srv.statePath); err != nil {
		return nil, status.Errorf(codes.Internal, "persist join request: %v", err)
	}
	return &clustercontrollerpb.RejectJoinResponse{
		NodeId:  req.GetNodeId(),
		Message: "rejected",
	}, nil
}

func (srv *server) ListNodes(ctx context.Context, req *clustercontrollerpb.ListNodesRequest) (*clustercontrollerpb.ListNodesResponse, error) {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	resp := &clustercontrollerpb.ListNodesResponse{}
	for _, node := range srv.state.Nodes {
		resp.Nodes = append(resp.Nodes, &clustercontrollerpb.NodeRecord{
			NodeId:   node.NodeID,
			Identity: storedIdentityToProto(node.Identity),
			LastSeen: timestamppb.New(node.LastSeen),
			Status:   node.Status,
			Profiles: append([]string(nil), node.Profiles...),
			Metadata: node.Metadata,
		})
	}
	return resp, nil
}

func (srv *server) SetNodeProfiles(ctx context.Context, req *clustercontrollerpb.SetNodeProfilesRequest) (*clustercontrollerpb.SetNodeProfilesResponse, error) {
	if req == nil || req.GetNodeId() == "" || len(req.GetProfiles()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "--profile is required")
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	node := srv.state.Nodes[req.GetNodeId()]
	if node == nil {
		return nil, status.Error(codes.NotFound, "node not found")
	}
	node.Profiles = append([]string(nil), req.GetProfiles()...)
	node.LastSeen = time.Now()
	if err := srv.state.save(srv.statePath); err != nil {
		return nil, status.Errorf(codes.Internal, "persist node profiles: %v", err)
	}
	return &clustercontrollerpb.SetNodeProfilesResponse{
		OperationId: uuid.NewString(),
	}, nil
}

func (srv *server) GetNodePlan(ctx context.Context, req *clustercontrollerpb.GetNodePlanRequest) (*clustercontrollerpb.GetNodePlanResponse, error) {
	if req == nil || req.GetNodeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	node := srv.state.Nodes[req.GetNodeId()]
	if node == nil {
		return nil, status.Error(codes.NotFound, "node not found")
	}
	return &clustercontrollerpb.GetNodePlanResponse{
		Plan: &clustercontrollerpb.NodePlan{
			NodeId:   node.NodeID,
			Profiles: append([]string(nil), node.Profiles...),
		},
	}, nil
}

func (srv *server) WatchOperations(req *clustercontrollerpb.WatchOperationsRequest, stream clustercontrollerpb.ClusterControllerService_WatchOperationsServer) error {
	return status.Errorf(codes.Unimplemented, "ClusterController.WatchOperations is not implemented yet")
}

func protoToStoredIdentity(pi *clustercontrollerpb.NodeIdentity) storedIdentity {
	if pi == nil {
		return storedIdentity{}
	}
	return storedIdentity{
		Hostname:     pi.GetHostname(),
		Domain:       pi.GetDomain(),
		Ips:          append([]string(nil), pi.GetIps()...),
		Os:           pi.GetOs(),
		Arch:         pi.GetArch(),
		AgentVersion: pi.GetAgentVersion(),
	}
}

func storedIdentityToProto(si storedIdentity) *clustercontrollerpb.NodeIdentity {
	return &clustercontrollerpb.NodeIdentity{
		Hostname:     si.Hostname,
		Domain:       si.Domain,
		Ips:          append([]string(nil), si.Ips...),
		Os:           si.Os,
		Arch:         si.Arch,
		AgentVersion: si.AgentVersion,
	}
}

func copyLabels(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
