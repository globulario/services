package main

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/apply"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/planner"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/supervisor"
	"github.com/globulario/services/golang/security"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/plan/planpb"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (srv *NodeAgentServer) JoinCluster(ctx context.Context, req *node_agentpb.JoinClusterRequest) (*node_agentpb.JoinClusterResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	token := strings.TrimSpace(req.GetJoinToken())
	if token == "" {
		return nil, status.Error(codes.InvalidArgument, "join_token is required")
	}
	if srv.joinToken != "" && token != srv.joinToken {
		return nil, status.Error(codes.PermissionDenied, "join token mismatch")
	}
	controllerEndpoint := strings.TrimSpace(req.GetControllerEndpoint())
	if controllerEndpoint == "" {
		return nil, status.Error(codes.InvalidArgument, "controller_endpoint is required")
	}
	srv.controllerEndpoint = controllerEndpoint
	srv.state.ControllerEndpoint = controllerEndpoint
	if err := srv.saveState(); err != nil {
		log.Printf("warn: persist controller endpoint: %v", err)
	}

	if err := srv.ensureControllerClient(ctx); err != nil {
		return nil, status.Errorf(codes.Unavailable, "controller unavailable: %v", err)
	}

	resp, err := srv.controllerClient.RequestJoin(ctx, &cluster_controllerpb.RequestJoinRequest{
		JoinToken:    token,
		Identity:     buildNodeIdentity(),
		Labels:       parseNodeAgentLabels(),
		Capabilities: buildNodeCapabilities(),
	})
	if err != nil {
		return nil, err
	}
	srv.joinRequestID = resp.GetRequestId()
	srv.state.RequestID = srv.joinRequestID
	srv.state.NodeID = ""
	srv.nodeID = ""
	if err := srv.saveState(); err != nil {
		log.Printf("warn: persist join request: %v", err)
	}

	srv.startJoinApprovalWatcher(context.Background(), srv.joinRequestID)

	return &node_agentpb.JoinClusterResponse{
		RequestId: resp.GetRequestId(),
		Status:    resp.GetStatus(),
		Message:   resp.GetMessage(),
	}, nil
}

func (srv *NodeAgentServer) GetInventory(ctx context.Context, _ *node_agentpb.GetInventoryRequest) (*node_agentpb.GetInventoryResponse, error) {
	// Build installed components from local discovery + etcd installed_state.
	installed, _, _ := ComputeInstalledServices(ctx)
	components := make([]*node_agentpb.InstalledComponent, 0, len(installed))
	for _, info := range installed {
		if info.ServiceName == "" {
			continue
		}
		components = append(components, &node_agentpb.InstalledComponent{
			Name:      canonicalServiceName(info.ServiceName),
			Version:   info.Version,
			Installed: true,
		})
	}

	resp := &node_agentpb.GetInventoryResponse{
		Inventory: &node_agentpb.Inventory{
			Identity:   buildNodeIdentity(),
			UnixTime:   timestamppb.Now(),
			Components: components,
			Units:      detectUnits(ctx),
		},
	}
	return resp, nil
}

func (srv *NodeAgentServer) ApplyPlan(ctx context.Context, req *node_agentpb.ApplyPlanRequest) (*node_agentpb.ApplyPlanResponse, error) {
	if req == nil || req.GetPlan() == nil {
		return nil, status.Error(codes.InvalidArgument, "plan is required")
	}

	opID := strings.TrimSpace(req.GetOperationId())
	if opID == "" {
		opID = uuid.NewString()
	}
	op := srv.registerOperationWithID("apply plan", opID, req.GetPlan().GetProfiles())
	go srv.runPlan(ctx, op, req.GetPlan())
	return &node_agentpb.ApplyPlanResponse{OperationId: op.id}, nil
}

func (srv *NodeAgentServer) ApplyPlanV1(ctx context.Context, req *node_agentpb.ApplyPlanV1Request) (*node_agentpb.ApplyPlanV1Response, error) {
	if req == nil || req.GetPlan() == nil {
		return nil, status.Error(codes.InvalidArgument, "plan is required")
	}
	if srv.planStore == nil {
		return nil, status.Error(codes.FailedPrecondition, "plan store unavailable")
	}
	plan := proto.Clone(req.GetPlan()).(*planpb.NodePlan)
	if plan.GetNodeId() == "" {
		plan.NodeId = srv.nodeID
	}
	if srv.nodeID != "" && plan.GetNodeId() != "" && plan.GetNodeId() != srv.nodeID {
		return nil, status.Error(codes.InvalidArgument, "plan node_id does not match this agent")
	}
	// Validate cluster_id: reject plans targeting a different cluster.
	// This prevents cross-cluster plan injection where a controller from
	// cluster A could instruct a node in cluster B.
	if planClusterID := strings.TrimSpace(plan.GetClusterId()); planClusterID != "" {
		localClusterID, err := security.GetLocalClusterID()
		if err == nil && localClusterID != "" && planClusterID != localClusterID {
			return nil, status.Errorf(codes.InvalidArgument,
				"plan cluster_id %q does not match local cluster %q", planClusterID, localClusterID)
		}
	}
	// IssuedBy records the RBAC principal that dispatched this plan.
	// Authorization for the ApplyPlan RPC itself is enforced upstream by the
	// gRPC interceptor; IssuedBy is preserved here for audit trail purposes.
	planID := strings.TrimSpace(plan.GetPlanId())
	if planID == "" {
		planID = uuid.NewString()
		plan.PlanId = planID
	}
	opID := strings.TrimSpace(req.GetOperationId())
	if opID == "" {
		opID = planID
	}
	generation := plan.GetGeneration()
	if generation == 0 {
		generation = srv.lastPlanGeneration + 1
		if statusValue, err := srv.planStore.GetStatus(ctx, srv.nodeID); err == nil && statusValue != nil && statusValue.GetGeneration() >= generation {
			generation = statusValue.GetGeneration() + 1
		}
		if currentPlan, err := srv.planStore.GetCurrentPlan(ctx, srv.nodeID); err == nil && currentPlan != nil && currentPlan.GetGeneration() >= generation {
			generation = currentPlan.GetGeneration() + 1
		}
		plan.Generation = generation
	}
	if plan.GetCreatedUnixMs() == 0 {
		plan.CreatedUnixMs = uint64(time.Now().UnixMilli())
	}
	if err := srv.planStore.PutCurrentPlan(ctx, srv.nodeID, plan); err != nil {
		return nil, status.Errorf(codes.Internal, "persist plan: %v", err)
	}
	log.Printf("nodeagent: ApplyPlanV1 stored plan node=%s plan_id=%s gen=%d op_id=%s", plan.GetNodeId(), planID, plan.GetGeneration(), opID)
	return &node_agentpb.ApplyPlanV1Response{
		OperationId:    opID,
		PlanId:         planID,
		PlanGeneration: plan.GetGeneration(),
	}, nil
}

func (srv *NodeAgentServer) GetPlanStatusV1(ctx context.Context, req *node_agentpb.GetPlanStatusV1Request) (*node_agentpb.GetPlanStatusV1Response, error) {
	if srv.planStore == nil {
		return nil, status.Error(codes.FailedPrecondition, "plan store unavailable")
	}
	targetNode := srv.nodeID
	if req != nil && strings.TrimSpace(req.GetNodeId()) != "" {
		if srv.nodeID != "" && req.GetNodeId() != srv.nodeID {
			return nil, status.Error(codes.InvalidArgument, "node_id does not match this agent")
		}
		targetNode = strings.TrimSpace(req.GetNodeId())
	}
	statusMsg, err := srv.planStore.GetStatus(ctx, targetNode)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "fetch plan status: %v", err)
	}
	// When an operation_id is requested, verify it matches the current plan.
	// The operation_id equals plan_id (set in ApplyPlanV1).
	if reqOpID := strings.TrimSpace(req.GetOperationId()); reqOpID != "" {
		if statusMsg == nil || statusMsg.GetPlanId() != reqOpID {
			return nil, status.Errorf(codes.NotFound, "no active plan for operation_id %q", reqOpID)
		}
	}
	return &node_agentpb.GetPlanStatusV1Response{Status: statusMsg}, nil
}

func (srv *NodeAgentServer) WatchPlanStatusV1(req *node_agentpb.WatchPlanStatusV1Request, stream node_agentpb.NodeAgentService_WatchPlanStatusV1Server) error {
	if srv.planStore == nil {
		return status.Error(codes.FailedPrecondition, "plan store unavailable")
	}
	targetNode := srv.nodeID
	if req != nil && strings.TrimSpace(req.GetNodeId()) != "" {
		if srv.nodeID != "" && req.GetNodeId() != srv.nodeID {
			return status.Error(codes.InvalidArgument, "node_id does not match this agent")
		}
		targetNode = strings.TrimSpace(req.GetNodeId())
	}
	var lastPayload []byte
	sendStatus := func(ctx context.Context) error {
		statusMsg, err := srv.planStore.GetStatus(ctx, targetNode)
		if err != nil {
			return status.Errorf(codes.Internal, "fetch plan status: %v", err)
		}
		if statusMsg == nil {
			return nil
		}
		current, err := proto.Marshal(statusMsg)
		if err != nil {
			return status.Errorf(codes.Internal, "encode status: %v", err)
		}
		if string(current) == string(lastPayload) {
			return nil
		}
		lastPayload = current
		return stream.Send(statusMsg)
	}

	if err := sendStatus(stream.Context()); err != nil {
		return err
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case <-ticker.C:
			if err := sendStatus(stream.Context()); err != nil {
				return err
			}
		}
	}
}

func (srv *NodeAgentServer) runPlan(ctx context.Context, op *operation, plan *cluster_controllerpb.NodePlan) {
	op.broadcast(op.newEvent(cluster_controllerpb.OperationPhase_OP_QUEUED, "plan queued", 0, false, ""))
	op.broadcast(op.newEvent(cluster_controllerpb.OperationPhase_OP_RUNNING, "plan running", 5, false, ""))

	netChanged, err := srv.applyRenderedConfig(plan)
	if err != nil {
		msg := err.Error()
		op.broadcast(op.newEvent(cluster_controllerpb.OperationPhase_OP_FAILED, msg, 10, true, msg))
		srv.notifyControllerOperationResult(op.id, false, msg, err)
		return
	}
	networkGen := networkGenerationFromPlan(plan)
	if networkGen == 0 {
		msg := "network generation missing from plan"
		op.broadcast(op.newEvent(cluster_controllerpb.OperationPhase_OP_FAILED, msg, 15, true, msg))
		srv.notifyControllerOperationResult(op.id, false, msg, errors.New(msg))
		return
	}
	log.Printf("nodeagent: network generation desired=%d current=%d", networkGen, srv.lastNetworkGeneration)
	spec, specErr := specFromPlan(plan)
	if specErr != nil {
		msg := fmt.Sprintf("parse desired spec: %v", specErr)
		op.broadcast(op.newEvent(cluster_controllerpb.OperationPhase_OP_FAILED, msg, 18, true, msg))
		srv.notifyControllerOperationResult(op.id, false, msg, specErr)
		return
	}
	var desiredDomain string
	if spec != nil {
		desiredDomain = strings.TrimSpace(spec.GetClusterDomain())
	}
	if desiredDomain == "" {
		hasSpec := plan != nil && plan.GetRenderedConfig()["cluster.network.spec.json"] != ""
		msg := fmt.Sprintf("objectstore layout enforcement requires cluster domain, but none was found (spec_present=%t)", hasSpec)
		op.broadcast(op.newEvent(cluster_controllerpb.OperationPhase_OP_FAILED, msg, 19, true, msg))
		srv.notifyControllerOperationResult(op.id, false, msg, errors.New(msg))
		return
	}
	netChanged = netChanged || (networkGen > 0 && networkGen != srv.lastNetworkGeneration)
	if netChanged {
		if err := srv.reconcileNetwork(ctx, plan, op, networkGen, desiredDomain, netChanged); err != nil {
			msg := err.Error()
			op.broadcast(op.newEvent(cluster_controllerpb.OperationPhase_OP_FAILED, msg, 20, true, msg))
			srv.notifyControllerOperationResult(op.id, false, msg, err)
			return
		}
	}

	actions := planner.ComputeActions(plan)
	total := len(actions)
	current := 0
	var lastPercent int32
	err = apply.ApplyActions(ctx, actions, func(action planner.Action) {
		lastPercent = percentForStep(current, total)
		op.broadcast(op.newEvent(cluster_controllerpb.OperationPhase_OP_RUNNING, fmt.Sprintf("%s %s", action.Op, action.Unit), lastPercent, false, ""))
		current++
	})
	if err != nil {
		msg := err.Error()
		op.broadcast(op.newEvent(cluster_controllerpb.OperationPhase_OP_FAILED, msg, lastPercent, true, msg))
		srv.notifyControllerOperationResult(op.id, false, msg, err)
		return
	}

	// Ensure objectstore layout (bucket + sentinels) is present; must succeed.
	layoutCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	log.Printf("Invoking ensure_objectstore_layout (domain=%s)", desiredDomain)
	if err := srv.ensureObjectstoreLayout(layoutCtx, desiredDomain); err != nil {
		msg := fmt.Sprintf("ensure objectstore layout: %v", err)
		op.broadcast(op.newEvent(cluster_controllerpb.OperationPhase_OP_FAILED, msg, lastPercent, true, msg))
		srv.notifyControllerOperationResult(op.id, false, msg, err)
		return
	}

	op.broadcast(op.newEvent(cluster_controllerpb.OperationPhase_OP_SUCCEEDED, "plan applied", 100, true, ""))
	srv.lastNetworkGeneration = networkGen
	srv.state.NetworkGeneration = networkGen
	_ = srv.saveState()
	srv.notifyControllerOperationResult(op.id, true, "plan applied", nil)
}

func (srv *NodeAgentServer) GetServiceLogs(ctx context.Context, req *node_agentpb.GetServiceLogsRequest) (*node_agentpb.GetServiceLogsResponse, error) {
	unit := strings.TrimSpace(req.GetUnit())
	if unit == "" {
		return nil, status.Error(codes.InvalidArgument, "unit is required")
	}
	if !strings.HasPrefix(unit, "globular-") {
		return nil, status.Error(codes.InvalidArgument, "unit must start with 'globular-'")
	}

	lines := int(req.GetLines())
	if lines <= 0 {
		lines = 50
	}
	if lines > 200 {
		lines = 200
	}

	priority := strings.TrimSpace(req.GetPriority())
	output, err := supervisor.ReadJournalctl(ctx, unit, lines, priority)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "journalctl: %v", err)
	}

	logLines := strings.Split(output, "\n")
	if len(logLines) == 1 && logLines[0] == "" {
		logLines = nil
	}

	return &node_agentpb.GetServiceLogsResponse{
		Unit:      unit,
		LineCount: int32(len(logLines)),
		Lines:     logLines,
	}, nil
}

func (srv *NodeAgentServer) GetCertificateStatus(ctx context.Context, _ *node_agentpb.GetCertificateStatusRequest) (*node_agentpb.GetCertificateStatusResponse, error) {
	resp := &node_agentpb.GetCertificateStatusResponse{}

	serverCertPath := config.GetLocalServerCertificatePath()
	if serverCertPath != "" {
		resp.ServerCert = parseCertInfo(serverCertPath)
	}

	caPath := config.GetLocalCACertificate()
	if caPath != "" {
		resp.CaCert = parseCertInfo(caPath)
	}

	return resp, nil
}

func parseCertInfo(certPath string) *node_agentpb.CertificateInfo {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return &node_agentpb.CertificateInfo{
			Subject: fmt.Sprintf("error reading cert: %v", err),
		}
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return &node_agentpb.CertificateInfo{
			Subject: "error: no PEM block found",
		}
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return &node_agentpb.CertificateInfo{
			Subject: fmt.Sprintf("error parsing cert: %v", err),
		}
	}

	sans := make([]string, 0, len(cert.DNSNames)+len(cert.IPAddresses))
	sans = append(sans, cert.DNSNames...)
	for _, ip := range cert.IPAddresses {
		sans = append(sans, ip.String())
	}

	daysUntilExpiry := int32(time.Until(cert.NotAfter).Hours() / 24)

	fingerprint := sha256.Sum256(cert.Raw)
	fpHex := fmt.Sprintf("%x", fingerprint)

	return &node_agentpb.CertificateInfo{
		Subject:         cert.Subject.CommonName,
		Issuer:          cert.Issuer.CommonName,
		Sans:            sans,
		NotBefore:       cert.NotBefore.UTC().Format(time.RFC3339),
		NotAfter:        cert.NotAfter.UTC().Format(time.RFC3339),
		DaysUntilExpiry: daysUntilExpiry,
		ChainValid:      time.Now().Before(cert.NotAfter) && time.Now().After(cert.NotBefore),
		Fingerprint:     fpHex,
	}
}
