package main

// recovery_rpc.go — server implementation of NodeRecoveryServiceServer.
//
// The server struct implements NodeRecoveryServiceServer by embedding
// cluster_controllerpb.UnimplementedNodeRecoveryServiceServer and overriding
// the four RPC methods below.

import (
	"context"
	"fmt"
	"log"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/workflow/engine"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Ensure server implements NodeRecoveryServiceServer at compile time.
var _ cluster_controllerpb.NodeRecoveryServiceServer = (*server)(nil)

// StartNodeFullReseedRecovery validates preconditions, optionally captures a
// snapshot, and dispatches the node.recover.full_reseed workflow.
func (srv *server) StartNodeFullReseedRecovery(ctx context.Context, req *cluster_controllerpb.StartNodeFullReseedRecoveryRequest) (*cluster_controllerpb.StartNodeFullReseedRecoveryResponse, error) {
	if err := srv.requireLeader(ctx); err != nil {
		return nil, err
	}
	if req.NodeID == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}
	if req.Reason == "" {
		return nil, status.Error(codes.InvalidArgument, "reason is required")
	}

	// Validate preconditions.
	if err := srv.recoveryValidateRequest(ctx, req.NodeID, req.Reason, req.ExactReplayRequired, req.Force, req.DryRun, req.SnapshotID); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "precondition failed: %v", err)
	}

	// Check cluster safety.
	warnings, err := srv.recoveryCheckClusterSafety(ctx, req.NodeID, req.Force)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "cluster safety: %v", err)
	}

	// Build the dry-run / planning response.
	snapshotID := req.SnapshotID
	plan, err := srv.recoveryPlanReseed(ctx, req.NodeID, req.ExactReplayRequired, snapshotID)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "plan reseed: %v", err)
	}

	resp := &cluster_controllerpb.StartNodeFullReseedRecoveryResponse{
		State:            "PLANNED",
		Warnings:         warnings,
		PlannedArtifacts: plan,
	}

	if req.DryRun {
		resp.State = "DRY_RUN"
		log.Printf("recovery RPC: dry-run for node %s — %d artifacts planned", req.NodeID, len(plan))
		return resp, nil
	}

	// Capture a fresh snapshot if not supplying one.
	if snapshotID == "" {
		snap, err := srv.captureNodeInventorySnapshot(ctx, req.NodeID, req.Reason, "operator")
		if err != nil {
			return nil, status.Errorf(codes.Internal, "capture snapshot: %v", err)
		}
		snapshotID = snap.SnapshotID
		resp.SnapshotID = snapshotID
	}

	// Dispatch the workflow.
	mode := cluster_controllerpb.NodeRecoveryModeExactReplayRequired
	if !req.ExactReplayRequired {
		mode = cluster_controllerpb.NodeRecoveryModeAllowResolutionFallback
	}

	workflowInputs := map[string]any{
		"cluster_id":            srv.getClusterID(),
		"node_id":               req.NodeID,
		"reason":                req.Reason,
		"exact_replay_required": req.ExactReplayRequired,
		"force":                 req.Force,
		"dry_run":               false,
		"snapshot_id":           snapshotID,
		"note":                  req.Note,
	}

	runID, err := srv.dispatchWorkflow(ctx, "node.recover.full_reseed", workflowInputs)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "dispatch workflow: %v", err)
	}

	// Record the initial recovery state.
	st := &cluster_controllerpb.NodeRecoveryState{
		NodeID:     req.NodeID,
		WorkflowID: runID,
		SnapshotID: snapshotID,
		Phase:      cluster_controllerpb.NodeRecoveryPhasePrecheck,
		Mode:       mode,
		StartedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
		Reason:     req.Reason,
	}
	if err := srv.putNodeRecoveryState(ctx, st); err != nil {
		log.Printf("recovery RPC: could not persist initial state for %s: %v (non-fatal)", req.NodeID, err)
	}

	resp.WorkflowID = runID
	resp.SnapshotID = snapshotID
	resp.State = "DISPATCHED"

	log.Printf("recovery RPC: dispatched node.recover.full_reseed for %s — workflow=%s snapshot=%s mode=%s",
		req.NodeID, runID, snapshotID, mode)
	return resp, nil
}

// GetNodeRecoveryStatus returns the current recovery state, snapshot, and
// per-artifact results for a node.
func (srv *server) GetNodeRecoveryStatus(ctx context.Context, req *cluster_controllerpb.GetNodeRecoveryStatusRequest) (*cluster_controllerpb.GetNodeRecoveryStatusResponse, error) {
	if req.NodeID == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}

	st, err := srv.getNodeRecoveryState(ctx, req.NodeID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load recovery state: %v", err)
	}

	resp := &cluster_controllerpb.GetNodeRecoveryStatusResponse{
		Recovery: st,
	}

	if st != nil && st.SnapshotID != "" {
		snap, _ := srv.getNodeRecoverySnapshot(ctx, req.NodeID, st.SnapshotID)
		resp.Snapshot = snap
	}

	results, _ := srv.listArtifactResults(ctx, req.NodeID)
	resp.Results = results

	return resp, nil
}

// CreateNodeRecoverySnapshot takes a standalone snapshot of a node's installed
// artifact set without starting a recovery workflow. Useful for pre-maintenance
// captures or to supply to a subsequent StartNodeFullReseedRecovery call.
func (srv *server) CreateNodeRecoverySnapshot(ctx context.Context, req *cluster_controllerpb.CreateNodeRecoverySnapshotRequest) (*cluster_controllerpb.CreateNodeRecoverySnapshotResponse, error) {
	if err := srv.requireLeader(ctx); err != nil {
		return nil, err
	}
	if req.NodeID == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}

	reason := req.Reason
	if reason == "" {
		reason = "operator-initiated standalone snapshot"
	}

	snap, err := srv.captureNodeInventorySnapshot(ctx, req.NodeID, reason, "operator")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "capture snapshot: %v", err)
	}

	return &cluster_controllerpb.CreateNodeRecoverySnapshotResponse{
		SnapshotID: snap.SnapshotID,
		Snapshot:   snap,
	}, nil
}

// AckNodeReprovisioned is called by the operator (or installer callback) to
// signal that the machine wipe and OS reinstall are complete. The workflow's
// AWAIT_REPROVISION step polls for this flag.
func (srv *server) AckNodeReprovisioned(ctx context.Context, req *cluster_controllerpb.AckNodeReprovisionedRequest) (*emptypb.Empty, error) {
	if err := srv.requireLeader(ctx); err != nil {
		return nil, err
	}
	if req.NodeID == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}

	st, err := srv.getNodeRecoveryState(ctx, req.NodeID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load recovery state: %v", err)
	}
	if st == nil {
		return nil, status.Errorf(codes.NotFound, "no active recovery for node %s", req.NodeID)
	}
	if st.Phase != cluster_controllerpb.NodeRecoveryPhaseAwaitReprovision {
		return nil, status.Errorf(codes.FailedPrecondition,
			"node %s is in phase %s, not AWAIT_REPROVISION — ACK not applicable", req.NodeID, st.Phase)
	}

	st.ReprovisionAcked = true
	if req.WorkflowID != "" && st.WorkflowID != req.WorkflowID {
		return nil, status.Errorf(codes.InvalidArgument,
			"workflow_id mismatch: expected %s, got %s", st.WorkflowID, req.WorkflowID)
	}

	if err := srv.putNodeRecoveryState(ctx, st); err != nil {
		return nil, status.Errorf(codes.Internal, "persist ack: %v", err)
	}

	log.Printf("recovery: operator ACK received for node %s reprovision (workflow=%s note=%q)",
		req.NodeID, st.WorkflowID, req.Note)

	srv.emitClusterEvent("node.recovery.reprovision_acked", map[string]interface{}{
		"severity":    "INFO",
		"node_id":     req.NodeID,
		"workflow_id": st.WorkflowID,
		"note":        req.Note,
		"message":     fmt.Sprintf("Node %s reprovision acknowledged by operator", req.NodeID),
	})

	return &emptypb.Empty{}, nil
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// getClusterID returns the cluster_id from etcd / config.
func (srv *server) getClusterID() string {
	srv.lock("getClusterID")
	defer srv.unlock()
	// The cluster ID is typically the cluster domain. Fall back to a constant.
	if cfg := srv.cfg; cfg != nil && cfg.ClusterDomain != "" {
		return cfg.ClusterDomain
	}
	return "globular.internal"
}

// dispatchWorkflow submits a workflow run via the centralized WorkflowService
// and returns the run ID. It builds a per-run Router wired to the recovery
// controller callbacks so actor dispatch works during execution.
func (srv *server) dispatchWorkflow(ctx context.Context, workflowName string, inputs map[string]any) (string, error) {
	router := engine.NewRouter()
	engine.RegisterNodeRecoveryControllerActions(router, srv.buildNodeRecoveryControllerConfig())

	nodeID := fmt.Sprint(inputs["node_id"])
	correlationID := fmt.Sprintf("recovery:%s:%d", nodeID, time.Now().UnixNano())

	resp, err := srv.executeWorkflowCentralized(ctx, workflowName, correlationID, inputs, router)
	if err != nil {
		return "", fmt.Errorf("execute workflow %q: %w", workflowName, err)
	}
	return resp.RunId, nil
}
