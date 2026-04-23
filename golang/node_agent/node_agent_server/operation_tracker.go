package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (srv *NodeAgentServer) BootstrapFirstNode(ctx context.Context, req *node_agentpb.BootstrapFirstNodeRequest) (*node_agentpb.BootstrapFirstNodeResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	profiles := append([]string(nil), req.GetProfiles()...)
	if len(profiles) == 0 {
		profiles = []string{"control-plane", "gateway"}
	}

	bindAddr := strings.TrimSpace(req.GetControllerBind())
	if bindAddr == "" {
		return nil, status.Error(codes.InvalidArgument, "controller bind address is required")
	}
	host, port, err := net.SplitHostPort(bindAddr)
	if err != nil || strings.TrimSpace(port) == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid controller bind address %q", bindAddr)
	}
	host = strings.TrimSpace(host)
	// If bind host is wildcard/empty, use node advertised host (routable).
	if host == "" || host == "0.0.0.0" || host == "::" {
		if advHost, _, splitErr := net.SplitHostPort(strings.TrimSpace(srv.advertisedAddr)); splitErr == nil && advHost != "" {
			host = advHost
		}
	}
	if host == "" {
		return nil, status.Error(codes.InvalidArgument, "cannot derive routable controller endpoint host")
	}
	controllerEndpoint := net.JoinHostPort(host, port)

	srv.controllerEndpoint = controllerEndpoint
	srv.state.ControllerEndpoint = controllerEndpoint
	if err := srv.saveState(); err != nil {
		log.Printf("warn: persist controller endpoint: %v", err)
	}

	// Self-register the bootstrap node SYNCHRONOUSLY.
	// The caller must know whether registration actually succeeded before
	// printing "success" and proceeding to seed.
	if err := srv.selfRegisterBootstrapNode(ctx, profiles); err != nil {
		return nil, status.Errorf(codes.Internal, "bootstrap self-registration failed: %v", err)
	}

	return &node_agentpb.BootstrapFirstNodeResponse{
		OperationId: "bootstrap",
		JoinToken:   srv.joinToken,
		Message:     fmt.Sprintf("bootstrap complete; node registered as %s", srv.nodeID),
	}, nil
}

// selfRegisterBootstrapNode registers the first node with the controller
// by issuing RequestJoin + ApproveJoin. Returns an error if registration
// fails so the caller can report the real reason.
func (srv *NodeAgentServer) selfRegisterBootstrapNode(ctx context.Context, profiles []string) error {
	// Wait for the controller to be ready (it may still be starting up).
	var connectErr error
	for attempt := 0; attempt < 15; attempt++ {
		if connectErr = srv.ensureControllerClient(ctx); connectErr == nil {
			break
		}
		log.Printf("bootstrap: waiting for controller (attempt %d/15): %v", attempt+1, connectErr)
		time.Sleep(3 * time.Second)
	}
	if srv.controllerClient == nil {
		return fmt.Errorf("could not connect to controller at %s: %w", srv.controllerEndpoint, connectErr)
	}

	// Already registered (e.g. re-run of bootstrap).
	if srv.nodeID != "" {
		log.Printf("bootstrap: node already registered as %s", srv.nodeID)
		return nil
	}

	// Validate that we have a join token.
	if srv.joinToken == "" {
		return fmt.Errorf("no join token configured (set NODE_AGENT_JOIN_TOKEN env var or ensure controller seeds a Day-0 token)")
	}

	labels := srv.joinRequestLabels()
	joinResp, err := srv.controllerClient.RequestJoin(ctx, &cluster_controllerpb.RequestJoinRequest{
		JoinToken:    srv.joinToken,
		Identity:     srv.buildNodeIdentity(),
		Labels:       labels,
		Capabilities: buildNodeCapabilities(),
	})
	if err != nil {
		return fmt.Errorf("RequestJoin failed (token=%q): %w", srv.joinToken, err)
	}
	requestID := joinResp.GetRequestId()

	// Auto-approve the bootstrap node.
	approveResp, err := srv.controllerClient.ApproveJoin(ctx, &cluster_controllerpb.ApproveJoinRequest{
		RequestId: requestID,
		Profiles:  profiles,
	})
	if err != nil {
		return fmt.Errorf("ApproveJoin failed (requestID=%s): %w", requestID, err)
	}

	nodeID := approveResp.GetNodeId()
	srv.applyApprovedNodeID(nodeID)
	log.Printf("bootstrap: node self-registered as %s", nodeID)

	// Store node-scoped identity token if provided.
	if token := approveResp.GetNodeToken(); token != "" {
		if err := srv.storeNodeToken(token, approveResp.GetNodePrincipal()); err != nil {
			log.Printf("bootstrap: failed to store node token: %v", err)
		}
	}

	return nil
}

func (srv *NodeAgentServer) getOperation(id string) *operation {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	return srv.operations[id]
}

func (srv *NodeAgentServer) WatchOperation(req *node_agentpb.WatchOperationRequest, stream node_agentpb.NodeAgentService_WatchOperationServer) error {
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

type operation struct {
	id       string
	kind     string
	profiles []string

	mu          sync.Mutex
	subscribers map[chan *node_agentpb.OperationEvent]struct{}
	lastEvent   *node_agentpb.OperationEvent
}

func (op *operation) subscribe() (chan *node_agentpb.OperationEvent, *node_agentpb.OperationEvent) {
	ch := make(chan *node_agentpb.OperationEvent, 4)
	op.mu.Lock()
	op.subscribers[ch] = struct{}{}
	last := op.lastEvent
	op.mu.Unlock()
	return ch, last
}

func (op *operation) unsubscribe(ch chan *node_agentpb.OperationEvent) {
	op.mu.Lock()
	delete(op.subscribers, ch)
	op.mu.Unlock()
}

func (op *operation) broadcast(evt *node_agentpb.OperationEvent) {
	op.mu.Lock()
	op.lastEvent = evt
	subs := make([]chan *node_agentpb.OperationEvent, 0, len(op.subscribers))
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

func (op *operation) newEvent(phase cluster_controllerpb.OperationPhase, message string, percent int32, done bool, errStr string) *node_agentpb.OperationEvent {
	return &node_agentpb.OperationEvent{
		OperationId: "bootstrap",
		Phase:       phase,
		Message:     message,
		Percent:     percent,
		Done:        done,
		Error:       errStr,
	}
}
