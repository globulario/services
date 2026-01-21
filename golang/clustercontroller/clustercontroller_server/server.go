package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"github.com/globulario/services/golang/plan/planpb"
	"github.com/globulario/services/golang/plan/store"
	"github.com/globulario/services/golang/repository/repository_client"
	"github.com/globulario/services/golang/repository/repositorypb"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	agentIdleTimeoutDefault  = 5 * time.Minute
	agentCleanupInterval     = 1 * time.Minute
	operationCleanupInterval = 1 * time.Minute
	operationTimeout         = 10 * time.Minute
	joinRequestRetention     = 72 * time.Hour
	pendingJoinRetention     = 7 * 24 * time.Hour
	statePersistInterval     = 5 * time.Second
	statusGracePeriod        = 2 * time.Minute
	planPollInterval         = 3 * time.Second
	upgradePlanTTL           = 10 * time.Minute
	defaultProbePort         = 80
	defaultBinaryPath        = "/usr/local/bin/globular"
	defaultTargetPublisher   = "globular"
	defaultTargetName        = "globular"
	upgradeDiskMinBytes      = 1 << 30
	repositoryAddressEnv     = "REPOSITORY_ADDRESS"

	// Health monitoring constants
	healthCheckInterval     = 30 * time.Second // How often to check node health
	unhealthyThreshold      = 2 * time.Minute  // Time without contact before marking unhealthy
	recoveryAttemptInterval = 5 * time.Minute  // How often to attempt recovery
	maxRecoveryAttempts     = 3                // Max recovery attempts before giving up
)

type server struct {
	clustercontrollerpb.UnimplementedClusterControllerServiceServer

	cfg              *clusterControllerConfig
	cfgPath          string
	statePath        string
	state            *controllerState
	mu               sync.Mutex
	muHeldSince      atomic.Int64
	muHeldBy         atomic.Value
	planStore        store.PlanStore
	agentMu          sync.Mutex
	agentClients     map[string]*agentClient
	agentInsecure    bool
	agentIdleTimeout time.Duration
	agentCAPath      string
	lastStateSave    time.Time
	agentServerName  string
	opMu             sync.Mutex
	operations       map[string]*operationState
	watchMu          sync.Mutex
	watchers         map[*operationWatcher]struct{}
	reconcileRunning atomic.Bool
}

var testHookBeforeReportNodeStatusApply func()

func newServer(cfg *clusterControllerConfig, cfgPath, statePath string, state *controllerState, planStore store.PlanStore) *server {
	if state == nil {
		state = newControllerState()
	}
	if statePath == "" {
		statePath = defaultClusterStatePath
	}
	agentCAPath := strings.TrimSpace(os.Getenv("CLUSTER_AGENT_CA"))
	serverName := strings.TrimSpace(os.Getenv("CLUSTER_AGENT_SERVER_NAME"))
	srv := &server{
		cfg:              cfg,
		cfgPath:          cfgPath,
		statePath:        statePath,
		state:            state,
		planStore:        planStore,
		agentClients:     make(map[string]*agentClient),
		agentInsecure:    strings.EqualFold(os.Getenv("CLUSTER_INSECURE_AGENT_GRPC"), "true"),
		agentIdleTimeout: agentIdleTimeoutDefault,
		agentCAPath:      agentCAPath,
		agentServerName:  serverName,
		operations:       make(map[string]*operationState),
		watchers:         make(map[*operationWatcher]struct{}),
	}

	safeGo("mu-watchdog", func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			since := srv.muHeldSince.Load()
			if since == 0 {
				continue
			}
			heldFor := time.Since(time.Unix(0, since))
			if heldFor < 3*time.Second {
				continue
			}
			tag, _ := srv.muHeldBy.Load().(string)
			log.Printf("[WARN] srv.mu held for %s by %q; dumping goroutines", heldFor, tag)
			buf := make([]byte, 1<<20)
			n := runtime.Stack(buf, true)
			log.Printf("[WARN] goroutine dump:\n%s", string(buf[:n]))
		}
	})

	return srv
}

func (srv *server) lock(tag string) {
	srv.mu.Lock()
	srv.muHeldBy.Store(tag)
	srv.muHeldSince.Store(time.Now().UnixNano())
}

func (srv *server) unlock() {
	srv.muHeldSince.Store(0)
	srv.muHeldBy.Store("")
	srv.mu.Unlock()
}

func safeGo(tag string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("panic in %s: %v\n%s", tag, r, debug.Stack())
			}
		}()
		fn()
	}()
}

func (srv *server) GetClusterInfo(ctx context.Context, req *timestamppb.Timestamp) (*clustercontrollerpb.ClusterInfo, error) {
	created := srv.state.CreatedAt
	if created.IsZero() {
		created = time.Now()
	}
	clusterID := srv.state.ClusterId
	if clusterID == "" {
		clusterID = srv.cfg.ClusterDomain
	}
	info := &clustercontrollerpb.ClusterInfo{
		ClusterDomain: srv.cfg.ClusterDomain,
		ClusterId:     clusterID,
		CreatedAt:     timestamppb.New(created),
	}
	return info, nil
}

func (srv *server) CreateJoinToken(ctx context.Context, req *clustercontrollerpb.CreateJoinTokenRequest) (*clustercontrollerpb.CreateJoinTokenResponse, error) {
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
	srv.state.JoinRequests[reqID] = &joinRequestRecord{
		RequestID:   reqID,
		Token:       token,
		Identity:    protoToStoredIdentity(req.GetIdentity()),
		Labels:      copyLabels(req.GetLabels()),
		RequestedAt: time.Now(),
		Status:      "pending",
	}
	if err := srv.persistStateLocked(true); err != nil {
		return nil, status.Errorf(codes.Internal, "persist join request: %v", err)
	}
	return &clustercontrollerpb.RequestJoinResponse{
		RequestId: reqID,
		Status:    "pending",
		Message:   "pending approval",
	}, nil
}

func (srv *server) ListJoinRequests(ctx context.Context, req *clustercontrollerpb.ListJoinRequestsRequest) (*clustercontrollerpb.ListJoinRequestsResponse, error) {
	srv.lock("unknown")
	defer srv.unlock()
	resp := &clustercontrollerpb.ListJoinRequestsResponse{}
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
		resp.Pending = append(resp.Pending, &clustercontrollerpb.JoinRequestRecord{
			RequestId: jr.RequestID,
			Identity:  storedIdentityToProto(jr.Identity),
			Status:    jr.Status,
			Profiles:  append([]string(nil), jr.Profiles...),
			Metadata:  copyLabels(jr.Labels),
		})
	}
	return resp, nil
}

func (srv *server) ApproveJoin(ctx context.Context, req *clustercontrollerpb.ApproveJoinRequest) (*clustercontrollerpb.ApproveJoinResponse, error) {
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
	profiles := req.GetProfiles()
	if len(profiles) == 0 {
		profiles = srv.cfg.DefaultProfiles
	}
	jr.Profiles = append([]string(nil), profiles...)
	nodeID := uuid.NewString()
	jr.AssignedNodeID = nodeID

	// Create new node with current network generation
	node := &nodeState{
		NodeID:                nodeID,
		Identity:              jr.Identity,
		Profiles:              append([]string(nil), profiles...),
		LastSeen:              time.Now(),
		Status:                "converging",
		Metadata:              copyLabels(jr.Labels),
		LastAppliedGeneration: 0, // New node hasn't applied any generation yet
	}
	srv.state.Nodes[nodeID] = node

	if err := srv.persistStateLocked(true); err != nil {
		srv.unlock()
		return nil, status.Errorf(codes.Internal, "persist node state: %v", err)
	}

	// Immediately dispatch initial plan with network config if node has endpoint
	// Note: New nodes won't have endpoint yet, so reconciliation loop will pick this up
	// when the node first reports status with its agent endpoint
	srv.unlock()

	return &clustercontrollerpb.ApproveJoinResponse{
		NodeId:  nodeID,
		Message: "approved; node will receive configuration on first heartbeat",
	}, nil
}

func (srv *server) RejectJoin(ctx context.Context, req *clustercontrollerpb.RejectJoinRequest) (*clustercontrollerpb.RejectJoinResponse, error) {
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
	return &clustercontrollerpb.RejectJoinResponse{
		NodeId:  jr.AssignedNodeID,
		Message: "rejected",
	}, nil
}

func (srv *server) ListNodes(ctx context.Context, req *clustercontrollerpb.ListNodesRequest) (*clustercontrollerpb.ListNodesResponse, error) {
	srv.lock("unknown")
	defer srv.unlock()
	resp := &clustercontrollerpb.ListNodesResponse{}
	nodes := make([]*nodeState, 0, len(srv.state.Nodes))
	for _, node := range srv.state.Nodes {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].NodeID < nodes[j].NodeID
	})
	for _, node := range nodes {
		meta := copyLabels(node.Metadata)
		if node.LastError != "" {
			if meta == nil {
				meta = make(map[string]string)
			}
			meta["last_error"] = node.LastError
		}
		resp.Nodes = append(resp.Nodes, &clustercontrollerpb.NodeRecord{
			NodeId:        node.NodeID,
			Identity:      storedIdentityToProto(node.Identity),
			LastSeen:      timestamppb.New(node.LastSeen),
			Status:        node.Status,
			Profiles:      append([]string(nil), node.Profiles...),
			Metadata:      meta,
			AgentEndpoint: node.AgentEndpoint,
		})
	}
	return resp, nil
}

func (srv *server) SetNodeProfiles(ctx context.Context, req *clustercontrollerpb.SetNodeProfilesRequest) (*clustercontrollerpb.SetNodeProfilesResponse, error) {
	if req == nil || req.GetNodeId() == "" || len(req.GetProfiles()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "--profile is required")
	}
	srv.lock("unknown")
	defer srv.unlock()
	node := srv.state.Nodes[req.GetNodeId()]
	if node == nil {
		return nil, status.Error(codes.NotFound, "node not found")
	}
	node.Profiles = append([]string(nil), req.GetProfiles()...)
	node.LastSeen = time.Now()
	if err := srv.persistStateLocked(true); err != nil {
		return nil, status.Errorf(codes.Internal, "persist node profiles: %v", err)
	}
	return &clustercontrollerpb.SetNodeProfilesResponse{
		OperationId: uuid.NewString(),
	}, nil
}

func (srv *server) RemoveNode(ctx context.Context, req *clustercontrollerpb.RemoveNodeRequest) (*clustercontrollerpb.RemoveNodeResponse, error) {
	if req == nil || req.GetNodeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}
	nodeID := strings.TrimSpace(req.GetNodeId())

	srv.lock("remove-node")
	node := srv.state.Nodes[nodeID]
	if node == nil {
		srv.unlock()
		return nil, status.Error(codes.NotFound, "node not found")
	}

	agentEndpoint := node.AgentEndpoint
	srv.unlock()

	opID := uuid.NewString()
	var drainErr error

	// If drain requested and node has an agent endpoint, try to stop services gracefully
	if req.GetDrain() && agentEndpoint != "" {
		drainErr = srv.drainNode(ctx, node, opID)
		if drainErr != nil && !req.GetForce() {
			return nil, status.Errorf(codes.FailedPrecondition, "drain failed (use force=true to override): %v", drainErr)
		}
	}

	// Remove from state
	srv.lock("remove-node")
	delete(srv.state.Nodes, nodeID)
	if err := srv.persistStateLocked(true); err != nil {
		srv.unlock()
		return nil, status.Errorf(codes.Internal, "persist node removal: %v", err)
	}
	srv.unlock()

	// Close agent client if we have one
	if agentEndpoint != "" {
		srv.closeAgentClient(agentEndpoint)
	}

	message := fmt.Sprintf("node %s removed from cluster", nodeID)
	if drainErr != nil {
		message = fmt.Sprintf("node %s removed (drain failed: %v)", nodeID, drainErr)
	}

	srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, clustercontrollerpb.OperationPhase_OP_SUCCEEDED, message, 100, true, ""))

	return &clustercontrollerpb.RemoveNodeResponse{
		OperationId: opID,
		Message:     message,
	}, nil
}

// drainNode sends stop commands to the node agent to gracefully stop all services.
func (srv *server) drainNode(ctx context.Context, node *nodeState, opID string) error {
	if node.AgentEndpoint == "" {
		return fmt.Errorf("node %s has no agent endpoint", node.NodeID)
	}

	srv.broadcastOperationEvent(srv.newOperationEvent(opID, node.NodeID, clustercontrollerpb.OperationPhase_OP_RUNNING, "draining node services", 10, false, ""))

	// Build a plan with stop actions for all services
	plan := &clustercontrollerpb.NodePlan{
		NodeId:   node.NodeID,
		Profiles: node.Profiles,
	}

	// Add stop actions for known service units based on profiles
	unitStops := []string{}
	for _, profile := range node.Profiles {
		switch profile {
		case "core":
			unitStops = append(unitStops, "globular-etcd.service", "globular-minio.service", "globular-xds.service", "globular-dns.service")
		case "compute":
			unitStops = append(unitStops, "globular-etcd.service", "globular-minio.service", "globular-xds.service")
		case "control-plane":
			unitStops = append(unitStops, "globular-etcd.service", "globular-xds.service")
		case "storage":
			unitStops = append(unitStops, "globular-minio.service")
		case "dns":
			unitStops = append(unitStops, "globular-dns.service")
		case "gateway":
			unitStops = append(unitStops, "globular-xds.service")
		}
	}

	// Dedupe and add stop actions
	seen := make(map[string]bool)
	for _, unit := range unitStops {
		if !seen[unit] {
			seen[unit] = true
			plan.UnitActions = append(plan.UnitActions, &clustercontrollerpb.UnitAction{
				UnitName: unit,
				Action:   "stop",
			})
		}
	}

	if len(plan.UnitActions) == 0 {
		return nil // Nothing to drain
	}

	client, err := srv.getAgentClient(ctx, node.AgentEndpoint)
	if err != nil {
		return fmt.Errorf("connect to agent: %w", err)
	}

	if err := client.ApplyPlan(ctx, plan, opID); err != nil {
		return fmt.Errorf("apply drain plan: %w", err)
	}

	srv.broadcastOperationEvent(srv.newOperationEvent(opID, node.NodeID, clustercontrollerpb.OperationPhase_OP_RUNNING, "drain plan sent", 50, false, ""))

	return nil
}

func (srv *server) GetClusterHealth(ctx context.Context, req *clustercontrollerpb.GetClusterHealthRequest) (*clustercontrollerpb.GetClusterHealthResponse, error) {
	srv.lock("cluster-health")
	defer srv.unlock()

	resp := &clustercontrollerpb.GetClusterHealthResponse{
		TotalNodes: int32(len(srv.state.Nodes)),
	}

	now := time.Now()
	healthyThreshold := 2 * time.Minute // Node is healthy if seen within this time

	for _, node := range srv.state.Nodes {
		nodeHealth := &clustercontrollerpb.NodeHealthStatus{
			NodeId:    node.NodeID,
			Hostname:  node.Identity.Hostname,
			LastError: node.LastError,
			LastSeen:  timestamppb.New(node.LastSeen),
		}

		// Determine node health status
		timeSinceSeen := now.Sub(node.LastSeen)
		switch {
		case node.Status == "healthy" && timeSinceSeen < healthyThreshold:
			nodeHealth.Status = "healthy"
			resp.HealthyNodes++
		case node.Status == "unhealthy" || node.LastError != "":
			nodeHealth.Status = "unhealthy"
			nodeHealth.FailedChecks = 1
			if node.LastError != "" {
				nodeHealth.LastError = node.LastError
			}
			resp.UnhealthyNodes++
		case timeSinceSeen >= healthyThreshold:
			nodeHealth.Status = "unknown"
			nodeHealth.LastError = fmt.Sprintf("not seen for %v", timeSinceSeen.Round(time.Second))
			resp.UnknownNodes++
		default:
			nodeHealth.Status = "unknown"
			resp.UnknownNodes++
		}

		resp.NodeHealth = append(resp.NodeHealth, nodeHealth)
	}

	// Determine overall cluster status
	switch {
	case resp.TotalNodes == 0:
		resp.Status = "unhealthy"
	case resp.UnhealthyNodes == 0 && resp.UnknownNodes == 0:
		resp.Status = "healthy"
	case resp.HealthyNodes > 0:
		resp.Status = "degraded"
	default:
		resp.Status = "unhealthy"
	}

	return resp, nil
}

func (srv *server) GetNodePlan(ctx context.Context, req *clustercontrollerpb.GetNodePlanRequest) (*clustercontrollerpb.GetNodePlanResponse, error) {
	if req == nil || req.GetNodeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}
	srv.lock("unknown")
	defer srv.unlock()
	node := srv.state.Nodes[req.GetNodeId()]
	if node == nil {
		return nil, status.Error(codes.NotFound, "node not found")
	}
	plan := srv.computeNodePlan(node)
	return &clustercontrollerpb.GetNodePlanResponse{
		Plan: plan,
	}, nil
}

func (srv *server) UpdateClusterNetwork(ctx context.Context, req *clustercontrollerpb.UpdateClusterNetworkRequest) (*clustercontrollerpb.UpdateClusterNetworkResponse, error) {
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

	srv.lock("unknown")
	changed := !proto.Equal(srv.state.ClusterNetworkSpec, spec)
	gen := computeNetworkGeneration(spec)
	if gen == 0 {
		srv.unlock()
		return nil, status.Error(codes.Internal, "failed to compute network generation")
	}
	if changed || srv.state.NetworkingGeneration != gen {
		srv.state.ClusterNetworkSpec = proto.Clone(spec).(*clustercontrollerpb.ClusterNetworkSpec)
		srv.state.NetworkingGeneration = gen
		if err := srv.persistStateLocked(true); err != nil {
			srv.unlock()
			return nil, status.Errorf(codes.Internal, "persist network spec: %v", err)
		}
	}
	generation := srv.state.NetworkingGeneration
	srv.unlock()

	return &clustercontrollerpb.UpdateClusterNetworkResponse{
		Generation: generation,
	}, nil
}

func (srv *server) ApplyNodePlan(ctx context.Context, req *clustercontrollerpb.ApplyNodePlanRequest) (*clustercontrollerpb.ApplyNodePlanResponse, error) {
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
	plan := srv.computeNodePlan(node)
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
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, clustercontrollerpb.OperationPhase_OP_QUEUED, "plan queued", 0, false, ""))
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, clustercontrollerpb.OperationPhase_OP_RUNNING, "plan running", 5, false, ""))
	if err := srv.dispatchPlan(ctx, node, plan, opID); err != nil {
		log.Printf("node %s apply dispatch failed: %v", nodeID, err)
		srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, clustercontrollerpb.OperationPhase_OP_FAILED, "plan failed", 0, true, err.Error()))
		return nil, status.Errorf(codes.Internal, "dispatch plan: %v", err)
	}
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, clustercontrollerpb.OperationPhase_OP_RUNNING, "plan dispatched to node-agent", 25, false, ""))
	if srv.recordPlanSent(nodeID, hash) {
		srv.lock("unknown")
		if err := srv.persistStateLocked(true); err != nil {
			log.Printf("persist state after ApplyNodePlan: %v", err)
		}
		srv.unlock()
	}

	return &clustercontrollerpb.ApplyNodePlanResponse{
		OperationId: opID,
	}, nil
}

func (srv *server) CompleteOperation(ctx context.Context, req *clustercontrollerpb.CompleteOperationRequest) (*clustercontrollerpb.CompleteOperationResponse, error) {
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
	phase := clustercontrollerpb.OperationPhase_OP_SUCCEEDED
	if !req.GetSuccess() {
		phase = clustercontrollerpb.OperationPhase_OP_FAILED
	}
	message := strings.TrimSpace(req.GetMessage())
	if message == "" {
		if phase == clustercontrollerpb.OperationPhase_OP_SUCCEEDED {
			message = "plan applied"
		} else {
			message = "plan failed"
		}
	}
	percent := req.GetPercent()
	if percent == 0 && phase == clustercontrollerpb.OperationPhase_OP_SUCCEEDED {
		percent = 100
	}
	errMsg := strings.TrimSpace(req.GetError())
	evt := srv.newOperationEvent(opID, nodeID, phase, message, percent, true, errMsg)
	srv.broadcastOperationEvent(evt)
	return &clustercontrollerpb.CompleteOperationResponse{
		Message: fmt.Sprintf("operation %s completion recorded", opID),
	}, nil
}

func (srv *server) UpgradeGlobular(ctx context.Context, req *clustercontrollerpb.UpgradeGlobularRequest) (*clustercontrollerpb.UpgradeGlobularResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if len(req.GetArtifact()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "artifact is required")
	}
	platform := strings.TrimSpace(req.GetPlatform())
	if platform == "" {
		return nil, status.Error(codes.InvalidArgument, "platform is required")
	}
	nodeID := strings.TrimSpace(req.GetNodeId())
	if nodeID == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}
	srv.lock("unknown")
	node := srv.state.Nodes[nodeID]
	srv.unlock()
	if node == nil {
		return nil, status.Error(codes.NotFound, "node not found")
	}
	if srv.planStore == nil {
		return nil, status.Error(codes.FailedPrecondition, "plan store unavailable")
	}

	sha := strings.TrimSpace(req.GetSha256())
	if sha == "" {
		hash := sha256.Sum256(req.GetArtifact())
		sha = hex.EncodeToString(hash[:])
	} else {
		sha = strings.ToLower(sha)
	}

	planID := uuid.NewString()
	ref := &repositorypb.ArtifactRef{
		PublisherId: defaultTargetPublisher,
		Name:        defaultTargetName,
		Version:     planID,
		Platform:    platform,
		Kind:        repositorypb.ArtifactKind_SUBSYSTEM,
	}
	if err := uploadArtifact(ctx, ref, req.GetArtifact()); err != nil {
		return nil, status.Errorf(codes.Internal, "stage artifact: %v", err)
	}

	targetPath := strings.TrimSpace(req.GetTargetPath())
	if targetPath == "" {
		targetPath = os.Getenv("GLOBULAR_BINARY_PATH")
	}
	if targetPath == "" {
		targetPath = defaultBinaryPath
	}
	if targetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "target_path unavailable")
	}

	minPath := filepath.Dir(targetPath)
	fetchDest := filepath.Join(os.TempDir(), "globular-upgrade", planID, filepath.Base(targetPath))

	generation := srv.nextPlanGeneration(ctx, nodeID)
	expires := time.Now().Add(upgradePlanTTL)
	plan := buildUpgradePlan(planID, nodeID, srv.state.ClusterId, generation, expires, targetPath, fetchDest, ref, sha, req.GetProbePort(), minPath)

	if err := srv.planStore.PutCurrentPlan(ctx, nodeID, plan); err != nil {
		return nil, status.Errorf(codes.Internal, "persist plan: %v", err)
	}
	if appendable, ok := srv.planStore.(interface {
		AppendHistory(ctx context.Context, nodeID string, plan *planpb.NodePlan) error
	}); ok {
		_ = appendable.AppendHistory(ctx, nodeID, plan)
	}

	opID := uuid.NewString()
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, clustercontrollerpb.OperationPhase_OP_QUEUED, "upgrade queued", 0, false, ""))
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, clustercontrollerpb.OperationPhase_OP_RUNNING, "plan dispatched", 10, false, ""))

	status, err := srv.waitForPlanStatus(ctx, nodeID, planID, expires)
	if err != nil {
		srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, clustercontrollerpb.OperationPhase_OP_FAILED, "plan failed", 100, true, err.Error()))
		return nil, err
	}

	phase := clustercontrollerpb.OperationPhase_OP_SUCCEEDED
	msg := "plan succeeded"
	done := true
	errMsg := ""
	if status.GetState() != planpb.PlanState_PLAN_SUCCEEDED {
		phase = clustercontrollerpb.OperationPhase_OP_FAILED
		msg = "plan completed with error"
		errMsg = status.GetErrorMessage()
	}
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, phase, msg, 100, done, errMsg))

	return &clustercontrollerpb.UpgradeGlobularResponse{
		PlanId:        planID,
		Generation:    generation,
		TerminalState: planStateName(status.GetState()),
		ErrorStepId:   status.GetErrorStepId(),
		ErrorMessage:  status.GetErrorMessage(),
	}, nil
}

func (srv *server) ReportNodeStatus(ctx context.Context, req *clustercontrollerpb.ReportNodeStatusRequest) (*clustercontrollerpb.ReportNodeStatusResponse, error) {
	if req == nil || req.GetStatus() == nil || req.GetStatus().GetNodeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "status.node_id is required")
	}
	nodeStatus := req.GetStatus()
	ns := nodeStatus
	nodeID := strings.TrimSpace(ns.GetNodeId())
	newIdentity := protoToStoredIdentity(ns.GetIdentity())
	newEndpoint := strings.TrimSpace(ns.GetAgentEndpoint())
	reportedAt := time.Now()
	if ts := ns.GetReportedAt(); ts != nil {
		reportedAt = ts.AsTime()
	}
	rawUnits := protoUnitsToStored(ns.GetUnits())
	units := normalizedUnits(rawUnits)
	lastError := ns.GetLastError()

	// Snapshot existing node for evaluation without holding the lock during compute.
	srv.lock("ReportNodeStatus:snapshot")
	node := srv.state.Nodes[nodeID]
	if node == nil {
		srv.unlock()
		return nil, status.Error(codes.NotFound, "node not found")
	}
	nodeSnapshot := *node
	srv.unlock()

	healthStatus, reason := srv.evaluateNodeStatus(&nodeSnapshot, units)
	if lastError == "" && reason != "" && healthStatus != "ready" {
		lastError = reason
	}

	if testHookBeforeReportNodeStatusApply != nil {
		testHookBeforeReportNodeStatusApply()
	}

	srv.lock("ReportNodeStatus:commit")
	defer srv.unlock()
	node = srv.state.Nodes[nodeID]
	if node == nil {
		return nil, status.Error(codes.NotFound, "node not found")
	}
	changed := false

	if !identitiesEqual(node.Identity, newIdentity) {
		changed = true
	}
	node.Identity = newIdentity

	oldEndpoint := node.AgentEndpoint
	node.AgentEndpoint = newEndpoint
	node.ReportedAt = reportedAt
	node.LastSeen = reportedAt

	if !unitsEqual(node.Units, units) {
		node.Units = units
		changed = true
	}
	if node.Status != healthStatus {
		node.Status = healthStatus
		changed = true
	}
	if node.LastError != lastError {
		node.LastError = lastError
		changed = true
	}
	if oldEndpoint != newEndpoint {
		changed = true
	}
	endpointToClose := ""
	if oldEndpoint != "" && oldEndpoint != newEndpoint {
		endpointToClose = oldEndpoint
	}
	if changed {
		if err := srv.persistStateLocked(false); err != nil {
			return nil, status.Errorf(codes.Internal, "persist node status: %v", err)
		}
	}

	if endpointToClose != "" {
		srv.closeAgentClient(endpointToClose)
	}
	return &clustercontrollerpb.ReportNodeStatusResponse{
		Message: "status recorded",
	}, nil
}

func (srv *server) GetJoinRequestStatus(ctx context.Context, req *clustercontrollerpb.GetJoinRequestStatusRequest) (*clustercontrollerpb.GetJoinRequestStatusResponse, error) {
	if req == nil || req.GetRequestId() == "" {
		return nil, status.Error(codes.InvalidArgument, "request_id is required")
	}
	srv.lock("unknown")
	defer srv.unlock()
	jr := srv.state.JoinRequests[req.GetRequestId()]
	if jr == nil {
		return nil, status.Error(codes.NotFound, "join request not found")
	}
	return &clustercontrollerpb.GetJoinRequestStatusResponse{
		Status:   jr.Status,
		NodeId:   jr.AssignedNodeID,
		Message:  jr.Reason,
		Profiles: append([]string(nil), jr.Profiles...),
	}, nil
}

func (srv *server) startReconcileLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	safeGo("reconcile-loop", func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				srv.reconcileNodes(ctx)
			}
		}
	})
}

func (srv *server) startAgentCleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(agentCleanupInterval)
	safeGo("agent-cleanup", func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				srv.cleanupAgentClients()
			}
		}
	})
}

func (srv *server) startOperationCleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(operationCleanupInterval)
	safeGo("operation-cleanup", func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				srv.cleanupTimedOutOperations()
			}
		}
	})
}

// startHealthMonitorLoop runs periodic health checks and attempts recovery for unhealthy nodes.
func (srv *server) startHealthMonitorLoop(ctx context.Context) {
	ticker := time.NewTicker(healthCheckInterval)
	safeGo("health-monitor", func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				srv.monitorNodeHealth(ctx)
			}
		}
	})
}

// monitorNodeHealth checks all nodes and attempts recovery for unhealthy ones.
func (srv *server) monitorNodeHealth(ctx context.Context) {
	now := time.Now()

	srv.lock("health-monitor:snapshot")
	nodes := make([]*nodeState, 0, len(srv.state.Nodes))
	for _, node := range srv.state.Nodes {
		nodes = append(nodes, node)
	}
	srv.unlock()

	var stateDirty bool

	for _, node := range nodes {
		timeSinceSeen := now.Sub(node.LastSeen)

		srv.lock("health-monitor:check")
		currentNode := srv.state.Nodes[node.NodeID]
		if currentNode == nil {
			srv.unlock()
			continue
		}

		previousStatus := currentNode.Status

		// Check if node is unhealthy
		if timeSinceSeen > unhealthyThreshold {
			currentNode.FailedHealthChecks++

			if currentNode.Status != "unhealthy" {
				currentNode.Status = "unhealthy"
				currentNode.MarkedUnhealthySince = now
				currentNode.LastError = fmt.Sprintf("no contact for %v", timeSinceSeen.Round(time.Second))
				log.Printf("node %s marked unhealthy: %s", node.NodeID, currentNode.LastError)
				stateDirty = true
			}

			// Attempt recovery if needed
			shouldRecover := currentNode.RecoveryAttempts < maxRecoveryAttempts &&
				(currentNode.LastRecoveryAttempt.IsZero() || now.Sub(currentNode.LastRecoveryAttempt) > recoveryAttemptInterval)

			if shouldRecover && node.AgentEndpoint != "" {
				currentNode.LastRecoveryAttempt = now
				currentNode.RecoveryAttempts++
				stateDirty = true
				log.Printf("attempting recovery for node %s (attempt %d/%d)", node.NodeID, currentNode.RecoveryAttempts, maxRecoveryAttempts)
				srv.unlock()

				// Attempt to reconnect and redispatch plan
				if err := srv.attemptNodeRecovery(ctx, node); err != nil {
					log.Printf("recovery attempt for node %s failed: %v", node.NodeID, err)
					srv.lock("health-monitor:recovery-failed")
					if n := srv.state.Nodes[node.NodeID]; n != nil {
						n.LastError = fmt.Sprintf("recovery failed: %v", err)
					}
					srv.unlock()
				} else {
					log.Printf("recovery attempt for node %s initiated successfully", node.NodeID)
				}
				continue
			}
		} else if currentNode.Status == "unhealthy" && previousStatus == "unhealthy" {
			// Node came back online - reset recovery counters
			currentNode.Status = "healthy"
			currentNode.FailedHealthChecks = 0
			currentNode.RecoveryAttempts = 0
			currentNode.MarkedUnhealthySince = time.Time{}
			currentNode.LastError = ""
			log.Printf("node %s recovered and marked healthy", node.NodeID)
			stateDirty = true
		}
		srv.unlock()
	}

	if stateDirty {
		srv.lock("health-monitor:persist")
		if err := srv.persistStateLocked(true); err != nil {
			log.Printf("health monitor: persist state: %v", err)
		}
		srv.unlock()
	}
}

// attemptNodeRecovery tries to reconnect to a node and redispatch its plan.
func (srv *server) attemptNodeRecovery(ctx context.Context, node *nodeState) error {
	if node.AgentEndpoint == "" {
		return fmt.Errorf("no agent endpoint for node %s", node.NodeID)
	}

	// Close any existing connection to force reconnection
	srv.closeAgentClient(node.AgentEndpoint)

	// Get fresh agent client
	client, err := srv.getAgentClient(ctx, node.AgentEndpoint)
	if err != nil {
		return fmt.Errorf("connect to agent: %w", err)
	}

	// Try to get inventory to verify connectivity
	_, err = client.GetInventory(ctx)
	if err != nil {
		return fmt.Errorf("get inventory: %w", err)
	}

	// If we can connect, dispatch the current plan
	plan := srv.computeNodePlan(node)
	if plan == nil || (len(plan.GetUnitActions()) == 0 && len(plan.GetRenderedConfig()) == 0) {
		// No plan needed, just mark as recovered
		return nil
	}

	opID := uuid.NewString()
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, node.NodeID, clustercontrollerpb.OperationPhase_OP_QUEUED, "recovery: plan queued", 0, false, ""))

	if err := srv.dispatchPlan(ctx, node, plan, opID); err != nil {
		srv.broadcastOperationEvent(srv.newOperationEvent(opID, node.NodeID, clustercontrollerpb.OperationPhase_OP_FAILED, "recovery: plan failed", 0, true, err.Error()))
		return fmt.Errorf("dispatch plan: %w", err)
	}

	srv.broadcastOperationEvent(srv.newOperationEvent(opID, node.NodeID, clustercontrollerpb.OperationPhase_OP_RUNNING, "recovery: plan dispatched", 25, false, ""))
	return nil
}

func (srv *server) reconcileNodes(ctx context.Context) {
	if !srv.reconcileRunning.CompareAndSwap(false, true) {
		return
	}
	defer srv.reconcileRunning.Store(false)
	now := time.Now()
	srv.lock("reconcile:snapshot")
	nodes := make([]*nodeState, 0, len(srv.state.Nodes))
	for _, node := range srv.state.Nodes {
		nodes = append(nodes, node)
	}
	stateDirty := srv.cleanupJoinStateLocked(now)
	srv.unlock()

	for _, node := range nodes {
		if node.AgentEndpoint == "" {
			continue
		}
		plan := srv.computeNodePlan(node)
		if plan == nil || (len(plan.GetUnitActions()) == 0 && len(plan.GetRenderedConfig()) == 0) {
			continue
		}
		hash := planHash(plan)
		if hash == "" {
			continue
		}
		if !srv.shouldDispatch(node, hash) {
			continue
		}
		opID := uuid.NewString()
		srv.broadcastOperationEvent(srv.newOperationEvent(opID, node.NodeID, clustercontrollerpb.OperationPhase_OP_QUEUED, "plan queued", 0, false, ""))
		srv.broadcastOperationEvent(srv.newOperationEvent(opID, node.NodeID, clustercontrollerpb.OperationPhase_OP_RUNNING, "plan running", 5, false, ""))
		if err := srv.dispatchPlan(ctx, node, plan, opID); err != nil {
			log.Printf("plan dispatch for node %s failed: %v", node.NodeID, err)
			srv.broadcastOperationEvent(srv.newOperationEvent(opID, node.NodeID, clustercontrollerpb.OperationPhase_OP_FAILED, "plan failed", 0, true, err.Error()))
			if srv.recordPlanError(node.NodeID, err.Error()) {
				stateDirty = true
			}
			if srv.updateNodeState(node.NodeID, "degraded", err.Error()) {
				stateDirty = true
			}
			continue
		}
		srv.broadcastOperationEvent(srv.newOperationEvent(opID, node.NodeID, clustercontrollerpb.OperationPhase_OP_RUNNING, "plan dispatched to node-agent", 25, false, ""))
		if srv.recordPlanSent(node.NodeID, hash) {
			stateDirty = true
		}
	}

	if stateDirty {
		srv.lock("reconcile:persist")
		if err := srv.persistStateLocked(true); err != nil {
			log.Printf("persist state: %v", err)
		}
		srv.unlock()
	}
}

func (srv *server) computeNodePlan(node *nodeState) *clustercontrollerpb.NodePlan {
	if node == nil {
		return nil
	}
	actionList := buildPlanActions(node.Profiles)
	plan := &clustercontrollerpb.NodePlan{
		NodeId:   node.NodeID,
		Profiles: append([]string(nil), node.Profiles...),
	}
	if len(actionList) > 0 {
		plan.UnitActions = actionList
	}
	if rendered := srv.renderedConfigForNode(node); len(rendered) > 0 {
		plan.RenderedConfig = rendered
	}
	return plan
}

func planHash(plan *clustercontrollerpb.NodePlan) string {
	if plan == nil {
		return ""
	}
	actions := plan.GetUnitActions()
	rendered := plan.GetRenderedConfig()
	if len(actions) == 0 && len(rendered) == 0 {
		return ""
	}
	h := sha256.New()
	sortedActions := append([]*clustercontrollerpb.UnitAction(nil), actions...)
	sort.Slice(sortedActions, func(i, j int) bool {
		a := sortedActions[i]
		b := sortedActions[j]
		if a == nil && b == nil {
			return false
		}
		if a == nil {
			return true
		}
		if b == nil {
			return false
		}
		if a.GetUnitName() != b.GetUnitName() {
			return a.GetUnitName() < b.GetUnitName()
		}
		return a.GetAction() < b.GetAction()
	})
	for _, action := range sortedActions {
		if action == nil {
			continue
		}
		h.Write([]byte(action.GetUnitName()))
		h.Write([]byte{0})
		h.Write([]byte(action.GetAction()))
		h.Write([]byte{0})
	}
	if len(rendered) > 0 {
		keys := make([]string, 0, len(rendered))
		for key := range rendered {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			h.Write([]byte(key))
			h.Write([]byte{0})
			h.Write([]byte(rendered[key]))
			h.Write([]byte{0})
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (srv *server) clusterNetworkSpec() *clustercontrollerpb.ClusterNetworkSpec {
	srv.lock("unknown")
	spec := srv.state.ClusterNetworkSpec
	srv.unlock()
	if spec == nil {
		return nil
	}
	if clone, ok := proto.Clone(spec).(*clustercontrollerpb.ClusterNetworkSpec); ok {
		return clone
	}
	return nil
}

func (srv *server) renderedConfigForSpec() map[string]string {
	spec := srv.clusterNetworkSpec()
	if spec == nil {
		return nil
	}
	out := make(map[string]string, 4)
	if specJSON, err := protojson.Marshal(spec); err == nil {
		out["cluster.network.spec.json"] = string(specJSON)
	}
	configPayload := map[string]interface{}{
		"Domain":           spec.GetClusterDomain(),
		"Protocol":         spec.GetProtocol(),
		"PortHTTP":         spec.GetPortHttp(),
		"PortHTTPS":        spec.GetPortHttps(),
		"AlternateDomains": spec.GetAlternateDomains(),
		"ACMEEnabled":      spec.GetAcmeEnabled(),
		"AdminEmail":       spec.GetAdminEmail(),
		"ACMEChallenge":    "dns-01",
		"ACMEDNSPreflight": true,
	}
	if cfgJSON, err := json.MarshalIndent(configPayload, "", "  "); err == nil {
		out["/var/lib/globular/network.json"] = string(cfgJSON)
	}
	if gen := srv.networkingGeneration(); gen > 0 {
		out["cluster.network.generation"] = fmt.Sprintf("%d", gen)
	}
	if units := restartUnitsForSpec(spec); len(units) > 0 {
		if b, err := json.Marshal(units); err == nil {
			out["reconcile.restart_units"] = string(b)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// snapshotClusterMembership creates a snapshot of all cluster nodes for config rendering.
// This captures the current state without holding the lock during config generation.
func (srv *server) snapshotClusterMembership() *clusterMembership {
	srv.lock("snapshot-membership")
	defer srv.unlock()

	membership := &clusterMembership{
		ClusterID: srv.state.ClusterId,
		Nodes:     make([]memberNode, 0, len(srv.state.Nodes)),
	}

	for _, node := range srv.state.Nodes {
		if node == nil {
			continue
		}
		// Use the first IP address if available
		var ip string
		if len(node.Identity.Ips) > 0 {
			ip = node.Identity.Ips[0]
		}
		membership.Nodes = append(membership.Nodes, memberNode{
			NodeID:   node.NodeID,
			Hostname: node.Identity.Hostname,
			IP:       ip,
			Profiles: append([]string(nil), node.Profiles...),
		})
	}

	// Sort nodes by ID for deterministic output
	sort.Slice(membership.Nodes, func(i, j int) bool {
		return membership.Nodes[i].NodeID < membership.Nodes[j].NodeID
	})

	return membership
}

// renderedConfigForNode combines network config with service-specific configs for a node.
func (srv *server) renderedConfigForNode(node *nodeState) map[string]string {
	// Start with the network config
	out := srv.renderedConfigForSpec()
	if out == nil {
		out = make(map[string]string)
	}

	// Get cluster membership snapshot for service config rendering
	membership := srv.snapshotClusterMembership()

	// Find the current node in the membership
	var currentMember *memberNode
	for i := range membership.Nodes {
		if membership.Nodes[i].NodeID == node.NodeID {
			currentMember = &membership.Nodes[i]
			break
		}
	}

	// If node not found in membership (shouldn't happen), create a temporary entry
	if currentMember == nil {
		var ip string
		if len(node.Identity.Ips) > 0 {
			ip = node.Identity.Ips[0]
		}
		currentMember = &memberNode{
			NodeID:   node.NodeID,
			Hostname: node.Identity.Hostname,
			IP:       ip,
			Profiles: node.Profiles,
		}
	}

	// Get the cluster domain from network spec
	domain := ""
	if spec := srv.clusterNetworkSpec(); spec != nil {
		domain = spec.GetClusterDomain()
	}

	// Create config context
	ctx := &serviceConfigContext{
		Membership:  membership,
		CurrentNode: currentMember,
		ClusterID:   membership.ClusterID,
		Domain:      domain,
	}

	// Render service-specific configs
	serviceConfigs := renderServiceConfigs(ctx)
	for path, content := range serviceConfigs {
		out[path] = content
	}

	if len(out) == 0 {
		return nil
	}
	return out
}

func (srv *server) networkingGeneration() uint64 {
	srv.lock("state:network-gen")
	gen := srv.state.NetworkingGeneration
	srv.unlock()
	return gen
}

func restartUnitsForSpec(spec *clustercontrollerpb.ClusterNetworkSpec) []string {
	if spec == nil {
		return nil
	}
	units := []string{
		"globular-etcd.service",
		"globular-dns.service",
		"globular-discovery.service",
		"globular-xds.service",
		"globular-envoy.service",
		"globular-gateway.service",
		"globular-minio.service",
		"scylladb.service",
	}
	if spec.GetProtocol() == "https" {
		units = append(units, "globular-storage.service")
	}
	return units
}

func computeNetworkGeneration(spec *clustercontrollerpb.ClusterNetworkSpec) uint64 {
	if spec == nil {
		return 0
	}
	domain := strings.ToLower(strings.TrimSpace(spec.GetClusterDomain()))
	protoStr := strings.ToLower(strings.TrimSpace(spec.GetProtocol()))
	alts := normalizeDomains(spec.GetAlternateDomains())
	sort.Strings(alts)
	builder := strings.Builder{}
	builder.WriteString(domain)
	builder.WriteString("|")
	builder.WriteString(protoStr)
	builder.WriteString("|")
	builder.WriteString(fmt.Sprintf("%d|%d|", spec.GetPortHttp(), spec.GetPortHttps()))
	builder.WriteString(fmt.Sprintf("%t|", spec.GetAcmeEnabled()))
	builder.WriteString(strings.ToLower(strings.TrimSpace(spec.GetAdminEmail())))
	builder.WriteString("|")
	for _, a := range alts {
		builder.WriteString(a)
		builder.WriteString(",")
	}
	sum := sha256.Sum256([]byte(builder.String()))
	var gen uint64
	for i := 0; i < 8; i++ {
		gen = (gen << 8) | uint64(sum[i])
	}
	if gen == 0 {
		gen = 1
	}
	return gen
}

func normalizeDomains(domains []string) []string {
	if len(domains) == 0 {
		return nil
	}
	seen := make(map[string]struct{})
	out := make([]string, 0, len(domains))
	for _, v := range domains {
		if v == "" {
			continue
		}
		trimmed := strings.TrimSpace(strings.ToLower(v))
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func (srv *server) shouldDispatch(node *nodeState, hash string) bool {
	if node == nil {
		return false
	}
	if node.AgentEndpoint == "" {
		return false
	}
	if hash == "" {
		return false
	}
	if node.LastPlanHash != hash {
		return true
	}
	if node.Status != "ready" {
		return true
	}
	if node.LastPlanError != "" {
		return true
	}
	return false
}

func (srv *server) dispatchPlan(ctx context.Context, node *nodeState, plan *clustercontrollerpb.NodePlan, operationID string) error {
	if plan == nil {
		return fmt.Errorf("node %s plan is empty", node.NodeID)
	}
	client, err := srv.getAgentClient(ctx, node.AgentEndpoint)
	if err != nil {
		return fmt.Errorf("node %s: %w", node.NodeID, err)
	}
	if err := client.ApplyPlan(ctx, plan, operationID); err != nil {
		return fmt.Errorf("node %s apply plan: %w", node.NodeID, err)
	}
	return nil
}

func (srv *server) getAgentClient(ctx context.Context, endpoint string) (*agentClient, error) {
	srv.agentMu.Lock()
	client := srv.agentClients[endpoint]
	srv.agentMu.Unlock()
	if client != nil {
		return client, nil
	}
	newClient, err := newAgentClient(ctx, endpoint, srv.agentInsecure, srv.agentCAPath, srv.agentServerName)
	if err != nil {
		return nil, err
	}
	srv.agentMu.Lock()
	srv.agentClients[endpoint] = newClient
	srv.agentMu.Unlock()
	return newClient, nil
}

func (srv *server) WatchOperations(req *clustercontrollerpb.WatchOperationsRequest, stream clustercontrollerpb.ClusterControllerService_WatchOperationsServer) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	ctx := stream.Context()
	w := &operationWatcher{
		nodeID: req.GetNodeId(),
		opID:   req.GetOperationId(),
		ch:     make(chan *clustercontrollerpb.OperationEvent, 8),
	}
	srv.addWatcher(w)
	defer func() {
		srv.removeWatcher(w)
		close(w.ch)
	}()
	srv.opMu.Lock()
	for _, op := range srv.operations {
		op.mu.Lock()
		last := op.last
		op.mu.Unlock()
		if last != nil && w.matches(last) {
			select {
			case w.ch <- last:
			default:
			}
		}
	}
	srv.opMu.Unlock()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case evt := <-w.ch:
			if evt == nil {
				continue
			}
			if err := stream.Send(evt); err != nil {
				return err
			}
			if evt.GetDone() {
				return nil
			}
		}
	}
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

func (srv *server) updateNodeState(nodeID, status, lastError string) bool {
	srv.lock("unknown")
	defer srv.unlock()
	node := srv.state.Nodes[nodeID]
	if node == nil {
		return false
	}
	changed := false
	if node.Status != status {
		node.Status = status
		changed = true
	}
	if node.LastError != lastError {
		node.LastError = lastError
		changed = true
	}
	if changed {
		node.LastSeen = time.Now()
	}
	return changed
}

func (srv *server) recordPlanSent(nodeID, planHash string) bool {
	srv.lock("plan:record-sent")
	defer srv.unlock()
	node := srv.state.Nodes[nodeID]
	if node == nil {
		return false
	}
	node.LastPlanSentAt = time.Now()
	if node.LastPlanError != "" {
		node.LastPlanError = ""
	}
	if planHash != "" {
		node.LastPlanHash = planHash
	}
	node.LastAppliedGeneration = srv.state.NetworkingGeneration
	return true
}

func (srv *server) recordPlanError(nodeID, errMsg string) bool {
	srv.lock("plan:record-error")
	defer srv.unlock()
	node := srv.state.Nodes[nodeID]
	if node == nil {
		return false
	}
	if node.LastPlanError == errMsg {
		return false
	}
	node.LastPlanError = errMsg
	return true
}

func (srv *server) closeAgentClient(endpoint string) {
	if endpoint == "" {
		return
	}
	srv.agentMu.Lock()
	defer srv.agentMu.Unlock()
	if client, ok := srv.agentClients[endpoint]; ok {
		client.Close()
		delete(srv.agentClients, endpoint)
	}
}

func (srv *server) cleanupAgentClients() {
	srv.agentMu.Lock()
	defer srv.agentMu.Unlock()
	for endpoint, client := range srv.agentClients {
		if client.idleDuration() > srv.agentIdleTimeout {
			client.Close()
			delete(srv.agentClients, endpoint)
			log.Printf("closed idle agent client %s", endpoint)
		}
	}
}

func (srv *server) cleanupTimedOutOperations() {
	now := time.Now()
	var expired []struct {
		id     string
		nodeID string
	}
	srv.opMu.Lock()
	for id, op := range srv.operations {
		op.mu.Lock()
		done := op.done
		created := op.created
		nodeID := op.nodeID
		op.mu.Unlock()
		if done || created.IsZero() || nodeID == "" {
			continue
		}
		if now.Sub(created) > operationTimeout {
			expired = append(expired, struct {
				id     string
				nodeID string
			}{id: id, nodeID: nodeID})
		}
	}
	srv.opMu.Unlock()
	for _, entry := range expired {
		evt := srv.newOperationEvent(entry.id, entry.nodeID, clustercontrollerpb.OperationPhase_OP_FAILED, "operation timed out", 0, true, "operation timed out")
		srv.broadcastOperationEvent(evt)
	}
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

func identitiesEqual(a, b storedIdentity) bool {
	if a.Hostname != b.Hostname || a.Domain != b.Domain || a.Os != b.Os || a.Arch != b.Arch || a.AgentVersion != b.AgentVersion {
		return false
	}
	if len(a.Ips) != len(b.Ips) {
		return false
	}
	for i := range a.Ips {
		if a.Ips[i] != b.Ips[i] {
			return false
		}
	}
	return true
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

func protoUnitsToStored(in []*clustercontrollerpb.NodeUnitStatus) []unitStatusRecord {
	if len(in) == 0 {
		return nil
	}
	out := make([]unitStatusRecord, 0, len(in))
	for _, u := range in {
		if u == nil {
			continue
		}
		out = append(out, unitStatusRecord{
			Name:    u.GetName(),
			State:   u.GetState(),
			Details: u.GetDetails(),
		})
	}
	return out
}

func storedUnitsToProto(in []unitStatusRecord) []*clustercontrollerpb.NodeUnitStatus {
	if len(in) == 0 {
		return nil
	}
	out := make([]*clustercontrollerpb.NodeUnitStatus, 0, len(in))
	for _, u := range in {
		out = append(out, &clustercontrollerpb.NodeUnitStatus{
			Name:    u.Name,
			State:   u.State,
			Details: u.Details,
		})
	}
	return out
}

func normalizedUnits(units []unitStatusRecord) []unitStatusRecord {
	if len(units) == 0 {
		return nil
	}
	out := append([]unitStatusRecord(nil), units...)
	sort.Slice(out, func(i, j int) bool {
		ni := strings.ToLower(out[i].Name)
		nj := strings.ToLower(out[j].Name)
		if ni != nj {
			return ni < nj
		}
		si := strings.ToLower(out[i].State)
		sj := strings.ToLower(out[j].State)
		if si != sj {
			return si < sj
		}
		return strings.ToLower(out[i].Details) < strings.ToLower(out[j].Details)
	})
	return out
}

type operationState struct {
	mu      sync.Mutex
	last    *clustercontrollerpb.OperationEvent
	created time.Time
	done    bool
	nodeID  string
}

type operationWatcher struct {
	nodeID string
	opID   string
	ch     chan *clustercontrollerpb.OperationEvent
}

func (w *operationWatcher) matches(evt *clustercontrollerpb.OperationEvent) bool {
	if w == nil || evt == nil {
		return false
	}
	if w.nodeID != "" && w.nodeID != evt.GetNodeId() {
		return false
	}
	if w.opID != "" && w.opID != evt.GetOperationId() {
		return false
	}
	return true
}

func (srv *server) getOperationState(id string) *operationState {
	srv.opMu.Lock()
	defer srv.opMu.Unlock()
	op, ok := srv.operations[id]
	if !ok {
		op = &operationState{}
		srv.operations[id] = op
	}
	return op
}

func (srv *server) broadcastOperationEvent(evt *clustercontrollerpb.OperationEvent) {
	if evt == nil {
		return
	}
	op := srv.getOperationState(evt.GetOperationId())
	op.mu.Lock()
	if op.created.IsZero() {
		op.created = time.Now()
	}
	if op.nodeID == "" && evt.GetNodeId() != "" {
		op.nodeID = evt.GetNodeId()
	}
	op.last = evt
	if evt.GetDone() {
		op.done = true
	}
	op.mu.Unlock()
	srv.watchMu.Lock()
	for w := range srv.watchers {
		if w.matches(evt) {
			select {
			case w.ch <- evt:
			default:
			}
		}
	}
	srv.watchMu.Unlock()
}

func (srv *server) newOperationEvent(opID, nodeID string, phase clustercontrollerpb.OperationPhase, message string, percent int32, done bool, errMsg string) *clustercontrollerpb.OperationEvent {
	return &clustercontrollerpb.OperationEvent{
		OperationId: opID,
		NodeId:      nodeID,
		Phase:       phase,
		Message:     message,
		Percent:     percent,
		Done:        done,
		Error:       errMsg,
		Ts:          timestamppb.Now(),
	}
}

func (srv *server) addWatcher(w *operationWatcher) {
	srv.watchMu.Lock()
	srv.watchers[w] = struct{}{}
	srv.watchMu.Unlock()
}

func (srv *server) removeWatcher(w *operationWatcher) {
	srv.watchMu.Lock()
	if _, ok := srv.watchers[w]; ok {
		delete(srv.watchers, w)
	}
	srv.watchMu.Unlock()
}
func (srv *server) evaluateNodeStatus(node *nodeState, units []unitStatusRecord) (string, string) {
	if node == nil {
		return "degraded", "missing node record"
	}
	plan := srv.computeNodePlan(node)
	required := requiredUnitsFromPlan(plan)
	if len(required) == 0 {
		return "ready", ""
	}
	unitStates := make(map[string]string, len(units))
	for _, u := range units {
		if u.Name == "" {
			continue
		}
		unitStates[strings.ToLower(u.Name)] = strings.ToLower(u.State)
	}
	var missing []string
	var notActive []string
	for unit := range required {
		state, ok := unitStates[strings.ToLower(unit)]
		if !ok {
			missing = append(missing, fmt.Sprintf("%s missing", unit))
			continue
		}
		if state != "active" {
			if state == "" {
				state = "unknown"
			}
			notActive = append(notActive, fmt.Sprintf("%s is %s", unit, state))
		}
	}
	if len(missing) > 0 || len(notActive) > 0 {
		reason := strings.Join(append(missing, notActive...), "; ")
		if node.ReportedAt.IsZero() || time.Since(node.ReportedAt) < statusGracePeriod {
			return "converging", reason
		}
		return "degraded", reason
	}
	return "ready", ""
}

func requiredUnitsFromPlan(plan *clustercontrollerpb.NodePlan) map[string]struct{} {
	req := make(map[string]struct{})
	if plan == nil {
		return req
	}
	for _, action := range plan.GetUnitActions() {
		if action == nil {
			continue
		}
		switch strings.ToLower(action.GetAction()) {
		case "start", "restart":
			req[action.GetUnitName()] = struct{}{}
		}
	}
	return req
}

func unitsEqual(a, b []unitStatusRecord) bool {
	an := normalizedUnits(a)
	bn := normalizedUnits(b)
	if len(an) != len(bn) {
		return false
	}
	for i := range an {
		if an[i].Name != bn[i].Name || an[i].State != bn[i].State || an[i].Details != bn[i].Details {
			return false
		}
	}
	return true
}

func buildUpgradePlan(planID, nodeID, clusterID string, generation uint64, expires time.Time, targetPath, fetchDest string, ref *repositorypb.ArtifactRef, sha string, probePort uint32, diskPath string) *planpb.NodePlan {
	if probePort == 0 {
		probePort = defaultProbePort
	}
	steps := []*planpb.PlanStep{
		planStep("check.disk_free", map[string]interface{}{
			"path":      diskPath,
			"min_bytes": float64(upgradeDiskMinBytes),
		}),
		planStep("artifact.fetch", map[string]interface{}{
			"publisher": ref.GetPublisherId(),
			"name":      ref.GetName(),
			"version":   ref.GetVersion(),
			"platform":  ref.GetPlatform(),
			"dest":      fetchDest,
		}),
		planStep("artifact.verify", map[string]interface{}{
			"path":   fetchDest,
			"sha256": sha,
		}),
		planStep("service.stop", map[string]interface{}{
			"unit": "globular.service",
		}),
		planStep("file.backup", map[string]interface{}{
			"path": targetPath,
		}),
		planStep("file.write_atomic", map[string]interface{}{
			"path": targetPath,
			"src":  fetchDest,
		}),
		planStep("service.start", map[string]interface{}{
			"unit": "globular.service",
		}),
		planStep("probe.http", map[string]interface{}{
			"url": fmt.Sprintf("http://127.0.0.1:%d/checksum", probePort),
		}),
	}
	rollback := []*planpb.PlanStep{
		planStep("file.restore_backup", map[string]interface{}{
			"path": targetPath,
		}),
		planStep("service.start", map[string]interface{}{
			"unit": "globular.service",
		}),
	}
	policy := &planpb.PlanPolicy{
		MaxRetries:       3,
		RetryBackoffMs:   2000,
		FailureMode:      planpb.FailureMode_FAILURE_MODE_ROLLBACK,
		DryRun:           false,
		MaxParallelSteps: 1,
	}
	return &planpb.NodePlan{
		ApiVersion:    "globular.io/plan/v1",
		Kind:          "NodePlan",
		ClusterId:     clusterID,
		NodeId:        nodeID,
		PlanId:        planID,
		Generation:    generation,
		CreatedUnixMs: uint64(time.Now().UnixMilli()),
		ExpiresUnixMs: uint64(expires.UnixMilli()),
		IssuedBy:      "cluster-controller",
		Reason:        "update_globular",
		Locks:         []string{"node-upgrade", "service:Globular"},
		Policy:        policy,
		Spec: &planpb.PlanSpec{
			Steps:    steps,
			Rollback: rollback,
		},
	}
}

func planStep(action string, args map[string]interface{}) *planpb.PlanStep {
	return &planpb.PlanStep{
		Id:     fmt.Sprintf("step-%s", strings.ReplaceAll(action, ".", "-")),
		Action: action,
		Args:   structFromMap(args),
	}
}

func structFromMap(fields map[string]interface{}) *structpb.Struct {
	if len(fields) == 0 {
		return nil
	}
	s, _ := structpb.NewStruct(fields)
	return s
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

func uploadArtifact(ctx context.Context, ref *repositorypb.ArtifactRef, data []byte) error {
	addr := strings.TrimSpace(os.Getenv(repositoryAddressEnv))
	if addr == "" {
		addr = "localhost:10101"
	}
	client, err := repository_client.NewRepositoryService_Client(addr, "repository.PackageRepository")
	if err != nil {
		return err
	}
	defer client.Close()
	return client.UploadArtifact(ref, data)
}

func (srv *server) persistStateLocked(force bool) error {
	if !force && time.Since(srv.lastStateSave) < statePersistInterval {
		return nil
	}
	if err := srv.state.save(srv.statePath); err != nil {
		return err
	}
	srv.lastStateSave = time.Now()
	return nil
}
