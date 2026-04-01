package main

import (
	"context"
	"fmt"
	"strings"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ── RPCs ────────────────────────────────────────────────────────────────

func (srv *server) UpdateClusterNetwork(ctx context.Context, req *cluster_controllerpb.UpdateClusterNetworkRequest) (*cluster_controllerpb.UpdateClusterNetworkResponse, error) {
	if err := srv.requireLeader(ctx); err != nil {
		return nil, err
	}
	if req == nil || req.GetSpec() == nil {
		return nil, status.Error(codes.InvalidArgument, "spec is required")
	}
	spec := req.GetSpec()
	domain := strings.TrimSpace(spec.GetClusterDomain())
	if domain == "" {
		return nil, status.Error(codes.InvalidArgument, "cluster_domain is required")
	}
	spec.ClusterDomain = domain

	protocol := strings.ToLower(strings.TrimSpace(spec.GetProtocol()))
	if protocol == "" {
		protocol = "http"
	}
	if protocol != "http" && protocol != "https" {
		return nil, status.Error(codes.InvalidArgument, "protocol must be http or https")
	}
	spec.Protocol = protocol

	if protocol == "http" && spec.GetPortHttp() == 0 {
		spec.PortHttp = 80
	}
	if protocol == "https" && spec.GetPortHttps() == 0 {
		spec.PortHttps = 443
	}

	if spec.GetAcmeEnabled() && strings.TrimSpace(spec.GetAdminEmail()) == "" {
		return nil, status.Error(codes.InvalidArgument, "admin_email is required when acme_enabled is true")
	}

	spec.AdminEmail = strings.TrimSpace(spec.GetAdminEmail())
	spec.AlternateDomains = normalizeDomains(spec.GetAlternateDomains())

	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	applied, err := srv.resources.Apply(ctx, "ClusterNetwork", &cluster_controllerpb.ClusterNetwork{
		Meta: &cluster_controllerpb.ObjectMeta{Name: "default"},
		Spec: spec,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "apply desired network: %v", err)
	}
	gen := uint64(0)
	if cn, ok := applied.(*cluster_controllerpb.ClusterNetwork); ok && cn.Meta != nil {
		gen = uint64(cn.Meta.Generation)
	}
	return &cluster_controllerpb.UpdateClusterNetworkResponse{
		Generation: gen,
	}, nil
}

func (srv *server) CompleteOperation(ctx context.Context, req *cluster_controllerpb.CompleteOperationRequest) (*cluster_controllerpb.CompleteOperationResponse, error) {
	if err := srv.requireLeader(ctx); err != nil {
		return nil, err
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	opID := strings.TrimSpace(req.GetOperationId())
	if opID == "" {
		return nil, status.Error(codes.InvalidArgument, "operation_id is required")
	}
	nodeID := strings.TrimSpace(req.GetNodeId())
	if nodeID == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}
	phase := cluster_controllerpb.OperationPhase_OP_SUCCEEDED
	if !req.GetSuccess() {
		phase = cluster_controllerpb.OperationPhase_OP_FAILED
	}
	message := strings.TrimSpace(req.GetMessage())
	if message == "" {
		if phase == cluster_controllerpb.OperationPhase_OP_SUCCEEDED {
			message = "operation completed"
		} else {
			message = "operation failed"
		}
	}
	percent := req.GetPercent()
	if percent == 0 && phase == cluster_controllerpb.OperationPhase_OP_SUCCEEDED {
		percent = 100
	}
	errMsg := strings.TrimSpace(req.GetError())
	evt := srv.newOperationEvent(opID, nodeID, phase, message, percent, true, errMsg)
	srv.broadcastOperationEvent(evt)
	return &cluster_controllerpb.CompleteOperationResponse{
		Message: fmt.Sprintf("operation %s completion recorded", opID),
	}, nil
}
