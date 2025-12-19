package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/supervisor"
	nodeagentpb "github.com/globulario/services/golang/nodeagent/nodeagentpb"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var defaultPort = "11000"

// NodeAgentServer implements the simplified node executor API.
type NodeAgentServer struct {
	nodeagentpb.UnimplementedNodeAgentServiceServer

	mu                 sync.Mutex
	operations         map[string]*operation
	joinToken          string
	bootstrapToken     string
	controllerEndpoint string
	agentVersion       string
}

func NewNodeAgentServer() *NodeAgentServer {
	return &NodeAgentServer{
		operations:         make(map[string]*operation),
		joinToken:          strings.TrimSpace(os.Getenv("NODE_AGENT_JOIN_TOKEN")),
		bootstrapToken:     strings.TrimSpace(os.Getenv("NODE_AGENT_BOOTSTRAP_TOKEN")),
		controllerEndpoint: strings.TrimSpace(os.Getenv("NODE_AGENT_CONTROLLER_ENDPOINT")),
		agentVersion:       getEnv("NODE_AGENT_VERSION", "v0.1.0"),
	}
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

	return &nodeagentpb.JoinClusterResponse{
		NodeId:  uuid.NewString(),
		Status:  "pending",
		Message: "pending approval",
	}, nil
}

func (srv *NodeAgentServer) GetInventory(ctx context.Context, _ *nodeagentpb.GetInventoryRequest) (*nodeagentpb.GetInventoryResponse, error) {
	hostname, _ := os.Hostname()
	resp := &nodeagentpb.GetInventoryResponse{
		Inventory: &nodeagentpb.Inventory{
			Identity: &clustercontrollerpb.NodeIdentity{
				Hostname:     hostname,
				Domain:       os.Getenv("NODE_AGENT_DOMAIN"),
				Ips:          gatherIPs(),
				Os:           runtime.GOOS,
				Arch:         runtime.GOARCH,
				AgentVersion: srv.agentVersion,
			},
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

	total := len(plan.GetUnitActions())
	for idx, action := range plan.GetUnitActions() {
		percent := percentForStep(idx, total)
		op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_RUNNING, fmt.Sprintf("executing %s %s", action.GetAction(), action.GetUnitName()), percent, false, ""))
		output, err := supervisor.ApplyUnitAction(ctx, action.GetUnitName(), action.GetAction())
		if err != nil {
			msg := fmt.Sprintf("%s failed: %v", action.GetUnitName(), err)
			if len(output) > 0 {
				msg = fmt.Sprintf("%s - %s", msg, strings.TrimSpace(output))
			}
			op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_FAILED, msg, percent, true, err.Error()))
			return
		}
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
	return []*nodeagentpb.UnitStatus{}
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
