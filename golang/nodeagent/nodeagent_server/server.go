package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/apply"
	"github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/planner"
	"github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/supervisor"
	"github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/units"
	nodeagentpb "github.com/globulario/services/golang/nodeagent/nodeagentpb"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var defaultPort = "11000"

// NodeAgentServer implements the simplified node executor API.
type NodeAgentServer struct {
	nodeagentpb.UnimplementedNodeAgentServiceServer

	mu                 sync.Mutex
	stateMu            sync.Mutex
	controllerConnMu   sync.Mutex
	operations         map[string]*operation
	joinToken          string
	bootstrapToken     string
	controllerEndpoint string
	agentVersion       string
	bootstrapPlan      []string
	nodeID             string
	controllerConn     *grpc.ClientConn
	controllerClient   clustercontrollerpb.ClusterControllerServiceClient
	statePath          string
	state              *nodeAgentState
	joinRequestID      string
	advertisedAddr     string
	useInsecure        bool
	joinPollCancel     context.CancelFunc
	joinPollMu         sync.Mutex
}

func NewNodeAgentServer(statePath string, state *nodeAgentState) *NodeAgentServer {
	if state == nil {
		state = newNodeAgentState()
	}
	port := getEnv("NODE_AGENT_PORT", defaultPort)
	advertised := strings.TrimSpace(os.Getenv("NODE_AGENT_ADVERTISE_ADDR"))
	if advertised == "" {
		if ips := gatherIPs(); len(ips) > 0 {
			advertised = fmt.Sprintf("%s:%s", ips[0], port)
		} else {
			advertised = fmt.Sprintf("localhost:%s", port)
		}
	}
	useInsecure := strings.EqualFold(getEnv("NODE_AGENT_INSECURE", "false"), "true")
	controllerEndpoint := strings.TrimSpace(os.Getenv("NODE_AGENT_CONTROLLER_ENDPOINT"))
	if controllerEndpoint == "" {
		controllerEndpoint = state.ControllerEndpoint
	} else {
		state.ControllerEndpoint = controllerEndpoint
	}
	nodeID := state.NodeID
	if nodeID == "" {
		nodeID = strings.TrimSpace(os.Getenv("NODE_AGENT_NODE_ID"))
		state.NodeID = nodeID
	}
	return &NodeAgentServer{
		operations:         make(map[string]*operation),
		joinToken:          strings.TrimSpace(os.Getenv("NODE_AGENT_JOIN_TOKEN")),
		bootstrapToken:     strings.TrimSpace(os.Getenv("NODE_AGENT_BOOTSTRAP_TOKEN")),
		controllerEndpoint: controllerEndpoint,
		agentVersion:       getEnv("NODE_AGENT_VERSION", "v0.1.0"),
		bootstrapPlan:      nil,
		nodeID:             nodeID,
		statePath:          statePath,
		state:              state,
		joinRequestID:      state.RequestID,
		advertisedAddr:     advertised,
		useInsecure:        useInsecure,
	}
}

func (srv *NodeAgentServer) SetBootstrapPlan(plan []string) {
	srv.bootstrapPlan = append([]string(nil), plan...)
}

func (srv *NodeAgentServer) BootstrapIfNeeded(ctx context.Context) error {
	unit := units.UnitForService("etcd")
	if unit == "" {
		unit = "globular-etcd.service"
	}
	if err := supervisor.EnableNow(ctx, unit); err != nil {
		return err
	}
	if err := supervisor.WaitActive(ctx, unit, 30*time.Second); err != nil {
		return err
	}
	if len(srv.bootstrapPlan) == 0 {
		return nil
	}
	var reachable bool
	if srv.controllerEndpoint != "" {
		timeoutCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if err := srv.ensureControllerClient(timeoutCtx); err == nil {
			reachable = true
		}
	}
	if reachable {
		return nil
	}
	plan := buildBootstrapPlan(srv.bootstrapPlan)
	if len(plan.GetUnitActions()) == 0 {
		return nil
	}
	op := srv.registerOperation("bootstrap plan", srv.bootstrapPlan)
	go srv.runPlan(ctx, op, plan)
	return nil
}

func (srv *NodeAgentServer) StartHeartbeat(ctx context.Context) {
	go srv.heartbeatLoop(ctx)
}

func (srv *NodeAgentServer) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		if err := srv.reportStatus(ctx); err != nil {
			log.Printf("node heartbeat failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (srv *NodeAgentServer) reportStatus(ctx context.Context) error {
	if srv.controllerEndpoint == "" {
		return nil
	}
	if srv.controllerClient == nil {
		if err := srv.ensureControllerClient(ctx); err != nil {
			return err
		}
	}
	if srv.nodeID == "" {
		return nil
	}
	identity := buildNodeIdentity()
	status := &clustercontrollerpb.NodeStatus{
		NodeId:        srv.nodeID,
		Identity:      identity,
		Ips:           append([]string(nil), identity.GetIps()...),
		Units:         convertNodeAgentUnits(detectUnits()),
		LastError:     "",
		ReportedAt:    timestamppb.Now(),
		AgentEndpoint: srv.advertisedAddr,
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := srv.controllerClient.ReportNodeStatus(ctx, &clustercontrollerpb.ReportNodeStatusRequest{
		Status: status,
	})
	return err
}

func (srv *NodeAgentServer) ensureControllerClient(ctx context.Context) error {
	if srv.controllerEndpoint == "" {
		return errors.New("controller endpoint is not configured")
	}
	opts, err := srv.controllerDialOptions()
	if err != nil {
		return err
	}
	srv.controllerConnMu.Lock()
	defer srv.controllerConnMu.Unlock()
	if srv.controllerClient != nil {
		return nil
	}
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(dialCtx, srv.controllerEndpoint, opts...)
	if err != nil {
		return err
	}
	srv.controllerConn = conn
	srv.controllerClient = clustercontrollerpb.NewClusterControllerServiceClient(conn)
	return nil
}

func (srv *NodeAgentServer) controllerDialOptions() ([]grpc.DialOption, error) {
	if srv.controllerEndpoint == "" {
		return nil, errors.New("controller endpoint is not configured")
	}
	opts := []grpc.DialOption{grpc.WithBlock()}
	if srv.useInsecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		return opts, nil
	}
	serverName := srv.controllerEndpoint
	if host, _, err := net.SplitHostPort(srv.controllerEndpoint); err == nil {
		serverName = host
	}
	opts = append(opts, grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(nil, serverName)))
	return opts, nil
}

func (srv *NodeAgentServer) JoinCluster(ctx context.Context, req *nodeagentpb.JoinClusterRequest) (*nodeagentpb.JoinClusterResponse, error) {
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

	resp, err := srv.controllerClient.RequestJoin(ctx, &clustercontrollerpb.RequestJoinRequest{
		JoinToken: token,
		Identity:  buildNodeIdentity(),
		Labels:    parseNodeAgentLabels(),
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

	return &nodeagentpb.JoinClusterResponse{
		RequestId: resp.GetRequestId(),
		Status:    resp.GetStatus(),
		Message:   resp.GetMessage(),
	}, nil
}

func (srv *NodeAgentServer) GetInventory(ctx context.Context, _ *nodeagentpb.GetInventoryRequest) (*nodeagentpb.GetInventoryResponse, error) {
	resp := &nodeagentpb.GetInventoryResponse{
		Inventory: &nodeagentpb.Inventory{
			Identity:   buildNodeIdentity(),
			UnixTime:   timestamppb.Now(),
			Components: detectComponents([]string{"envoy", "etcd", "minio", "scylla", "globular"}),
			Units:      detectUnits(),
		},
	}
	return resp, nil
}

func (srv *NodeAgentServer) ApplyPlan(ctx context.Context, req *nodeagentpb.ApplyPlanRequest) (*nodeagentpb.ApplyPlanResponse, error) {
	if req == nil || req.GetPlan() == nil {
		return nil, status.Error(codes.InvalidArgument, "plan is required")
	}

	op := srv.registerOperation("apply plan", req.GetPlan().GetProfiles())
	go srv.runPlan(context.Background(), op, req.GetPlan())
	return &nodeagentpb.ApplyPlanResponse{OperationId: op.id}, nil
}

func (srv *NodeAgentServer) runPlan(ctx context.Context, op *operation, plan *clustercontrollerpb.NodePlan) {
	op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_QUEUED, "plan queued", 0, false, ""))
	op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_RUNNING, "plan running", 5, false, ""))

	actions := planner.ComputeActions(plan)
	total := len(actions)
	current := 0
	var lastPercent int32
	err := apply.ApplyActions(ctx, actions, func(action planner.Action) {
		lastPercent = percentForStep(current, total)
		op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_RUNNING, fmt.Sprintf("%s %s", action.Op, action.Unit), lastPercent, false, ""))
		current++
	})
	if err != nil {
		msg := err.Error()
		op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_FAILED, msg, lastPercent, true, msg))
		return
	}

	op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_SUCCEEDED, "plan applied", 100, true, ""))
}

func (srv *NodeAgentServer) WatchOperation(req *nodeagentpb.WatchOperationRequest, stream nodeagentpb.NodeAgentService_WatchOperationServer) error {
	if req == nil || strings.TrimSpace(req.GetOperationId()) == "" {
		return status.Error(codes.InvalidArgument, "operation_id is required")
	}
	op := srv.getOperation(req.GetOperationId())
	if op == nil {
		return status.Error(codes.NotFound, "operation not found")
	}

	ch, last := op.subscribe()
	defer op.unsubscribe(ch)

	if last != nil && last.Done {
		if err := stream.Send(last); err != nil {
			return err
		}
		return nil
	}

	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case evt := <-ch:
			if evt == nil {
				continue
			}
			if err := stream.Send(evt); err != nil {
				return err
			}
			if evt.Done {
				return nil
			}
		}
	}
}

func (srv *NodeAgentServer) BootstrapFirstNode(ctx context.Context, req *nodeagentpb.BootstrapFirstNodeRequest) (*nodeagentpb.BootstrapFirstNodeResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	profiles := append([]string(nil), req.GetProfiles()...)
	if len(profiles) == 0 {
		profiles = []string{"control-plane", "gateway"}
	}

	op := srv.registerOperation("bootstrap node", profiles)
	srv.startOperation(op, "bootstrapping first node")

	return &nodeagentpb.BootstrapFirstNodeResponse{
		OperationId: op.id,
		JoinToken:   srv.joinToken,
		Message:     "bootstrap initiated",
	}, nil
}

func (srv *NodeAgentServer) registerOperation(kind string, profiles []string) *operation {
	op := &operation{
		id:          uuid.NewString(),
		kind:        kind,
		profiles:    append([]string(nil), profiles...),
		subscribers: make(map[chan *nodeagentpb.OperationEvent]struct{}),
	}
	srv.mu.Lock()
	srv.operations[op.id] = op
	srv.mu.Unlock()
	return op
}

func (srv *NodeAgentServer) getOperation(id string) *operation {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	return srv.operations[id]
}

func (srv *NodeAgentServer) startOperation(op *operation, message string) {
	go func() {
		op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_QUEUED, fmt.Sprintf("%s queued", message), 0, false, ""))
		time.Sleep(100 * time.Millisecond)
		op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_RUNNING, fmt.Sprintf("%s started", message), 5, false, ""))

		total := len(op.profiles)
		for idx, profile := range op.profiles {
			time.Sleep(250 * time.Millisecond)
			percent := percentForStep(idx, total)
			op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_RUNNING, fmt.Sprintf("profile %s applied", profile), percent, false, ""))
		}

		time.Sleep(200 * time.Millisecond)
		op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_SUCCEEDED, fmt.Sprintf("%s complete", message), 100, true, ""))
	}()
}

func percentForStep(idx, total int) int32 {
	if total <= 0 {
		return 50
	}
	base := int32(20)
	step := int32(60 / total)
	res := base + step*int32(idx+1)
	if res > 95 {
		return 95
	}
	return res
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func gatherIPs() []string {
	var ips []string
	seen := make(map[string]struct{})
	ifaces, err := net.Interfaces()
	if err != nil {
		return ips
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue
			}
			text := ip.String()
			if _, ok := seen[text]; ok {
				continue
			}
			seen[text] = struct{}{}
			ips = append(ips, text)
		}
	}
	return ips
}

func buildNodeIdentity() *clustercontrollerpb.NodeIdentity {
	hostname, _ := os.Hostname()
	return &clustercontrollerpb.NodeIdentity{
		Hostname:     hostname,
		Domain:       os.Getenv("NODE_AGENT_DOMAIN"),
		Ips:          gatherIPs(),
		Os:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		AgentVersion: getEnv("NODE_AGENT_VERSION", "v0.1.0"),
	}
}

func detectComponents(names []string) []*nodeagentpb.InstalledComponent {
	components := make([]*nodeagentpb.InstalledComponent, 0, len(names))
	for _, name := range names {
		_, err := exec.LookPath(name)
		components = append(components, &nodeagentpb.InstalledComponent{
			Name:      name,
			Version:   "",
			Installed: err == nil,
		})
	}
	return components
}

func detectUnits() []*nodeagentpb.UnitStatus {
	known := []string{
		"globular-etcd.service",
		"globular-dns.service",
		"globular-discovery.service",
		"globular-event.service",
		"globular-rbac.service",
		"globular-file.service",
		"globular-minio.service",
		"globular-gateway.service",
		"globular-xds.service",
		"envoy.service",
	}
	ctx := context.Background()
	statuses := make([]*nodeagentpb.UnitStatus, 0, len(known))
	for _, unit := range known {
		state := "unknown"
		details := ""
		active, err := supervisor.IsActive(ctx, unit)
		if err != nil {
			details = err.Error()
		} else {
			if active {
				state = "active"
			} else {
				state = "inactive"
			}
			if out, err := supervisor.Status(ctx, unit); err == nil {
				details = out
			} else if details == "" {
				details = err.Error()
			}
		}
		statuses = append(statuses, &nodeagentpb.UnitStatus{
			Name:    unit,
			State:   state,
			Details: details,
		})
	}
	return statuses
}

func buildBootstrapPlan(services []string) *clustercontrollerpb.NodePlan {
	actions := make([]*clustercontrollerpb.UnitAction, 0, len(services))
	for _, svc := range services {
		unit := units.UnitForService(svc)
		if unit == "" {
			continue
		}
		actions = append(actions, &clustercontrollerpb.UnitAction{
			UnitName: unit,
			Action:   "start",
		})
	}
	if len(actions) == 0 {
		return &clustercontrollerpb.NodePlan{
			Profiles: []string{"bootstrap"},
		}
	}
	return &clustercontrollerpb.NodePlan{
		Profiles:    []string{"bootstrap"},
		UnitActions: actions,
	}
}

func (srv *NodeAgentServer) saveState() error {
	if srv.statePath == "" {
		return nil
	}
	srv.stateMu.Lock()
	defer srv.stateMu.Unlock()
	if srv.state == nil {
		srv.state = newNodeAgentState()
	}
	srv.state.ControllerEndpoint = srv.controllerEndpoint
	srv.state.RequestID = srv.joinRequestID
	srv.state.NodeID = srv.nodeID
	return srv.state.save(srv.statePath)
}

func (srv *NodeAgentServer) startJoinApprovalWatcher(ctx context.Context, requestID string) {
	if requestID == "" {
		return
	}
	srv.joinPollMu.Lock()
	if srv.joinPollCancel != nil {
		srv.joinPollCancel()
	}
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	srv.joinPollCancel = cancel
	srv.joinPollMu.Unlock()
	go srv.watchJoinStatus(ctx, requestID)
}

func (srv *NodeAgentServer) watchJoinStatus(ctx context.Context, requestID string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		if err := srv.ensureControllerClient(ctx); err != nil {
			log.Printf("join status: controller unreachable: %v", err)
			if !waitOrDone(ctx, ticker) {
				return
			}
			continue
		}
		resp, err := srv.controllerClient.GetJoinRequestStatus(ctx, &clustercontrollerpb.GetJoinRequestStatusRequest{
			RequestId: requestID,
		})
		if err != nil {
			log.Printf("join status poll error: %v", err)
			if !waitOrDone(ctx, ticker) {
				return
			}
			continue
		}
		switch strings.ToLower(resp.GetStatus()) {
		case "approved":
			if nodeID := resp.GetNodeId(); nodeID != "" {
				srv.applyApprovedNodeID(nodeID)
				log.Printf("join request %s approved (node %s)", requestID, nodeID)
			}
			return
		case "rejected":
			log.Printf("join request %s rejected: %s", requestID, resp.GetMessage())
			return
		}
		if !waitOrDone(ctx, ticker) {
			return
		}
	}
}

func waitOrDone(ctx context.Context, ticker *time.Ticker) bool {
	select {
	case <-ctx.Done():
		return false
	case <-ticker.C:
		return true
	}
}

func (srv *NodeAgentServer) applyApprovedNodeID(nodeID string) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return
	}
	srv.stateMu.Lock()
	srv.nodeID = nodeID
	srv.state.NodeID = nodeID
	srv.state.RequestID = ""
	srv.joinRequestID = ""
	srv.stateMu.Unlock()
	if err := srv.saveState(); err != nil {
		log.Printf("warn: persist approved node id: %v", err)
	}
}

func parseNodeAgentLabels() map[string]string {
	raw := strings.TrimSpace(os.Getenv("NODE_AGENT_LABELS"))
	if raw == "" {
		return nil
	}
	labels := make(map[string]string)
	pairs := strings.Split(raw, ",")
	for _, pair := range pairs {
		if pair = strings.TrimSpace(pair); pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" || value == "" {
			continue
		}
		labels[key] = value
	}
	if len(labels) == 0 {
		return nil
	}
	return labels
}

func convertNodeAgentUnits(units []*nodeagentpb.UnitStatus) []*clustercontrollerpb.NodeUnitStatus {
	if len(units) == 0 {
		return nil
	}
	out := make([]*clustercontrollerpb.NodeUnitStatus, 0, len(units))
	for _, unit := range units {
		if unit == nil {
			continue
		}
		out = append(out, &clustercontrollerpb.NodeUnitStatus{
			Name:    unit.GetName(),
			State:   unit.GetState(),
			Details: unit.GetDetails(),
		})
	}
	return out
}

type operation struct {
	id       string
	kind     string
	profiles []string

	mu          sync.Mutex
	subscribers map[chan *nodeagentpb.OperationEvent]struct{}
	lastEvent   *nodeagentpb.OperationEvent
}

func (op *operation) subscribe() (chan *nodeagentpb.OperationEvent, *nodeagentpb.OperationEvent) {
	ch := make(chan *nodeagentpb.OperationEvent, 4)
	op.mu.Lock()
	op.subscribers[ch] = struct{}{}
	last := op.lastEvent
	op.mu.Unlock()
	return ch, last
}

func (op *operation) unsubscribe(ch chan *nodeagentpb.OperationEvent) {
	op.mu.Lock()
	delete(op.subscribers, ch)
	op.mu.Unlock()
}

func (op *operation) broadcast(evt *nodeagentpb.OperationEvent) {
	op.mu.Lock()
	op.lastEvent = evt
	subs := make([]chan *nodeagentpb.OperationEvent, 0, len(op.subscribers))
	for ch := range op.subscribers {
		subs = append(subs, ch)
	}
	op.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- evt:
		default:
		}
	}
}

func (op *operation) newEvent(phase clustercontrollerpb.OperationPhase, message string, percent int32, done bool, errStr string) *nodeagentpb.OperationEvent {
	return &nodeagentpb.OperationEvent{
		OperationId: op.id,
		Phase:       phase,
		Message:     message,
		Percent:     percent,
		Done:        done,
		Error:       errStr,
	}
}
