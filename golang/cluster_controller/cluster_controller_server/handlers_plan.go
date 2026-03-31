package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/plan/planpb"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

func (srv *server) GetNodePlan(ctx context.Context, req *cluster_controllerpb.GetNodePlanRequest) (*cluster_controllerpb.GetNodePlanResponse, error) {
	if req == nil || req.GetNodeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}
	srv.lock("unknown")
	defer srv.unlock()
	node := srv.state.Nodes[req.GetNodeId()]
	if node == nil {
		return nil, status.Error(codes.NotFound, "node not found")
	}
	plan, err := srv.computeNodePlan(node)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "compute plan: %v", err)
	}
	return &cluster_controllerpb.GetNodePlanResponse{
		Plan: plan,
	}, nil
}

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

func (srv *server) ApplyNodePlan(ctx context.Context, req *cluster_controllerpb.ApplyNodePlanRequest) (*cluster_controllerpb.ApplyNodePlanResponse, error) {
	if err := srv.requireLeader(ctx); err != nil {
		return nil, err
	}
	if req == nil || strings.TrimSpace(req.GetNodeId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}
	nodeID := strings.TrimSpace(req.GetNodeId())
	srv.lock("unknown")
	node := srv.state.Nodes[nodeID]
	srv.unlock()
	if node == nil {
		return nil, status.Error(codes.NotFound, "node not found")
	}
	if node.AgentEndpoint == "" {
		return nil, status.Error(codes.FailedPrecondition, "agent endpoint unknown")
	}
	plan, planErr := srv.computeNodePlan(node)
	if planErr != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "compute plan: %v", planErr)
	}
	if plan == nil {
		return nil, status.Error(codes.FailedPrecondition, "plan is empty")
	}
	if len(plan.GetUnitActions()) == 0 && len(plan.GetRenderedConfig()) == 0 {
		return nil, status.Error(codes.FailedPrecondition, "plan has no changes")
	}
	hash := planHash(plan)
	if hash == "" {
		return nil, status.Error(codes.FailedPrecondition, "plan has no changes")
	}

	opID := uuid.NewString()
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, cluster_controllerpb.OperationPhase_OP_QUEUED, "plan queued", 0, false, ""))
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, cluster_controllerpb.OperationPhase_OP_RUNNING, "plan running", 5, false, ""))
	if err := srv.dispatchPlan(ctx, node, plan, opID); err != nil {
		log.Printf("node %s apply dispatch failed: %v", nodeID, err)
		srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, cluster_controllerpb.OperationPhase_OP_FAILED, "plan failed", 0, true, err.Error()))
		return nil, status.Errorf(codes.Internal, "dispatch plan: %v", err)
	}
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, cluster_controllerpb.OperationPhase_OP_RUNNING, "plan dispatched to node-agent", 25, false, ""))
	// Phase 4b: store pending rendered config hashes on dispatch.
	// These will be promoted to RenderedConfigHashes only when the agent reports apply success.
	if len(plan.GetRenderedConfig()) > 0 {
		srv.lock("rendered-config-hashes")
		if n := srv.state.Nodes[nodeID]; n != nil {
			n.PendingRenderedConfigHashes = HashRenderedConfigs(plan.GetRenderedConfig())
		}
		srv.unlock()
	}
	if srv.recordPlanSent(nodeID, hash) {
		srv.lock("apply-node-plan:persist")
		func() {
			defer srv.unlock()
			if err := srv.persistStateLocked(true); err != nil {
				log.Printf("persist state after ApplyNodePlan: %v", err)
			}
		}()
	}

	return &cluster_controllerpb.ApplyNodePlanResponse{
		OperationId: opID,
	}, nil
}

func (srv *server) ApplyNodePlanV1(ctx context.Context, req *cluster_controllerpb.ApplyNodePlanV1Request) (*cluster_controllerpb.ApplyNodePlanV1Response, error) {
	if err := srv.requireLeader(ctx); err != nil {
		return nil, err
	}

	// Validation
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	nodeID := strings.TrimSpace(req.GetNodeId())
	if nodeID == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}
	if req.GetPlan() == nil {
		return nil, status.Error(codes.InvalidArgument, "plan is required")
	}
	plan := req.GetPlan()

	// Validate plan node_id matches request node_id
	planNodeID := strings.TrimSpace(plan.GetNodeId())
	if planNodeID != "" && planNodeID != nodeID {
		return nil, status.Errorf(codes.InvalidArgument, "plan.node_id %q does not match request node_id %q", planNodeID, nodeID)
	}
	// If plan.node_id is empty, set it to request node_id
	if planNodeID == "" {
		plan.NodeId = nodeID
	}

	// Validate steps exist
	if plan.GetSpec() == nil || len(plan.GetSpec().GetSteps()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "plan must have at least one step")
	}

	// Verify node exists and has agent endpoint
	srv.lock("apply-plan-v1")
	node := srv.state.Nodes[nodeID]
	srv.unlock()
	if node == nil {
		return nil, status.Errorf(codes.NotFound, "node %q not found", nodeID)
	}
	if node.AgentEndpoint == "" {
		return nil, status.Errorf(codes.FailedPrecondition, "node %q has no agent endpoint", nodeID)
	}

	// Create operation ID
	opID := uuid.NewString()

	// Persist plan to disk (optional but recommended)
	if err := srv.persistPlanV1(nodeID, opID, plan); err != nil {
		log.Printf("warning: failed to persist plan for node %s operation %s: %v", nodeID, opID, err)
	}

	// Broadcast initial operation events
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, cluster_controllerpb.OperationPhase_OP_QUEUED, "plan received and validated", 0, false, ""))
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, cluster_controllerpb.OperationPhase_OP_RUNNING, "dispatching plan to node-agent", 5, false, ""))

	// Dispatch plan to node agent
	client, err := srv.getAgentClient(ctx, node.AgentEndpoint)
	if err != nil {
		log.Printf("node %s: failed to get agent client: %v", nodeID, err)
		srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, cluster_controllerpb.OperationPhase_OP_FAILED, "failed to connect to node-agent", 0, true, err.Error()))
		return nil, status.Errorf(codes.Internal, "get agent client: %v", err)
	}

	if err := client.ApplyPlanV1(ctx, plan, opID); err != nil {
		log.Printf("node %s: apply plan v1 dispatch failed: %v", nodeID, err)
		srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, cluster_controllerpb.OperationPhase_OP_FAILED, "plan dispatch failed", 0, true, err.Error()))
		return nil, status.Errorf(codes.Internal, "dispatch plan: %v", err)
	}

	srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, cluster_controllerpb.OperationPhase_OP_RUNNING, "plan dispatched to node-agent", 25, false, ""))

	return &cluster_controllerpb.ApplyNodePlanV1Response{
		OperationId: opID,
	}, nil
}

func (srv *server) persistPlanV1(nodeID, operationID string, plan *planpb.NodePlan) error {
	plansRoot := "/var/lib/globular/plans"
	nodeDir := filepath.Join(plansRoot, nodeID)

	// Create node directory with 0700 permissions
	if err := os.MkdirAll(nodeDir, 0700); err != nil {
		return fmt.Errorf("create plans directory: %w", err)
	}

	// Marshal plan to JSON
	planJSON, err := protojson.Marshal(plan)
	if err != nil {
		return fmt.Errorf("marshal plan: %w", err)
	}

	// Write to temp file and rename (atomic write)
	planFile := filepath.Join(nodeDir, operationID+".json")
	tempFile := planFile + ".tmp"

	if err := os.WriteFile(tempFile, planJSON, 0600); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := os.Rename(tempFile, planFile); err != nil {
		os.Remove(tempFile) // Clean up temp file on error
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
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
			message = "plan applied"
		} else {
			message = "plan failed"
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

func (srv *server) nextPlanGeneration(ctx context.Context, nodeID string) uint64 {
	var last uint64
	if plan, err := srv.planStore.GetCurrentPlan(ctx, nodeID); err == nil && plan != nil {
		last = plan.GetGeneration()
	}
	if status, err := srv.planStore.GetStatus(ctx, nodeID); err == nil && status != nil {
		if status.GetGeneration() > last {
			last = status.GetGeneration()
		}
	}
	return last + 1
}

func (srv *server) waitForPlanStatus(ctx context.Context, nodeID, planID string, expires time.Time) (*planpb.NodePlanStatus, error) {
	ticker := time.NewTicker(planPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, status.Errorf(codes.Canceled, "context canceled")
		case <-ticker.C:
			statusValue, err := srv.planStore.GetStatus(ctx, nodeID)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "fetch plan status: %v", err)
			}
			if statusValue == nil {
				if !expires.IsZero() && time.Now().After(expires) {
					return nil, status.Error(codes.DeadlineExceeded, "plan expired before execution")
				}
				continue
			}
			if statusValue.GetPlanId() != planID {
				continue
			}
			if isTerminalPlanState(statusValue.GetState()) {
				return statusValue, nil
			}
			if !expires.IsZero() && time.Now().After(expires) {
				return nil, status.Error(codes.DeadlineExceeded, "plan expired before completion")
			}
		}
	}
}

func planStateName(state planpb.PlanState) string {
	if name, ok := planpb.PlanState_name[int32(state)]; ok {
		return name
	}
	return fmt.Sprintf("PLAN_STATE_%d", state)
}

func isTerminalPlanState(state planpb.PlanState) bool {
	switch state {
	case planpb.PlanState_PLAN_SUCCEEDED, planpb.PlanState_PLAN_FAILED, planpb.PlanState_PLAN_ROLLED_BACK, planpb.PlanState_PLAN_EXPIRED:
		return true
	default:
		return false
	}
}
