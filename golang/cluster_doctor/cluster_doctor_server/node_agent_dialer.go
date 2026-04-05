package main

import (
	"context"
	"fmt"
	"sync"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"google.golang.org/grpc"
)

// controllerNodeAgentDialer resolves a node's agent endpoint through the
// cluster-controller's ListNodes RPC, then dials it with TLS for systemctl
// / file operations. Caches the endpoint per node_id to avoid re-resolving
// on every action.
type controllerNodeAgentDialer struct {
	cc cluster_controllerpb.ClusterControllerServiceClient

	mu            sync.Mutex
	agentEndpoint map[string]string // node_id → "host:port"
}

func newControllerNodeAgentDialer(cc cluster_controllerpb.ClusterControllerServiceClient) *controllerNodeAgentDialer {
	return &controllerNodeAgentDialer{
		cc:            cc,
		agentEndpoint: make(map[string]string),
	}
}

// resolveEndpoint looks up a node's agent endpoint via ListNodes. Cached
// indefinitely since agent endpoints rarely change; on dial failure, the
// caller can invalidate and re-resolve.
func (d *controllerNodeAgentDialer) resolveEndpoint(ctx context.Context, nodeID string) (string, error) {
	d.mu.Lock()
	if ep, ok := d.agentEndpoint[nodeID]; ok {
		d.mu.Unlock()
		return ep, nil
	}
	d.mu.Unlock()

	resp, err := d.cc.ListNodes(ctx, &cluster_controllerpb.ListNodesRequest{})
	if err != nil {
		return "", fmt.Errorf("ListNodes: %w", err)
	}
	var found string
	for _, n := range resp.GetNodes() {
		if n.GetNodeId() == nodeID {
			found = n.GetAgentEndpoint()
			break
		}
	}
	if found == "" {
		return "", fmt.Errorf("node %s has no agent_endpoint", nodeID)
	}
	d.mu.Lock()
	d.agentEndpoint[nodeID] = found
	d.mu.Unlock()
	return found, nil
}

// dialAgent returns an authenticated gRPC connection to a node-agent.
// Caller is responsible for closing the returned connection.
func (d *controllerNodeAgentDialer) dialAgent(ctx context.Context, nodeID string) (*grpc.ClientConn, error) {
	endpoint, err := d.resolveEndpoint(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	conn, err := grpc.NewClient(endpoint,
		grpc.WithTransportCredentials(buildClientTLSCreds()),
	)
	if err != nil {
		return nil, fmt.Errorf("dial node-agent %s at %s: %w", nodeID, endpoint, err)
	}
	return conn, nil
}

// SystemctlAction implements NodeAgentDialer.SystemctlAction by calling the
// node-agent's ControlService RPC. Requires the unit to be globular-* per
// node-agent's own validation.
func (d *controllerNodeAgentDialer) SystemctlAction(ctx context.Context, nodeID, unit, verb string) (string, error) {
	conn, err := d.dialAgent(ctx, nodeID)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	client := node_agentpb.NewNodeAgentServiceClient(conn)
	resp, err := client.ControlService(ctx, &node_agentpb.ControlServiceRequest{
		Unit:   unit,
		Action: verb,
	})
	if err != nil {
		return "", fmt.Errorf("ControlService %s %s on %s: %w", verb, unit, nodeID, err)
	}
	if !resp.GetOk() {
		return resp.GetMessage(), fmt.Errorf("ControlService returned not-ok: %s", resp.GetMessage())
	}
	return fmt.Sprintf("%s %s on %s: state=%s", verb, unit, nodeID, resp.GetState()), nil
}

// FileDelete is not yet supported by node-agent — no generic file-delete
// RPC exists. For now, returns an explanatory error so the executor can
// report it cleanly. A future PR can add NodeAgentService.DeleteFile with
// its own allowlist check.
func (d *controllerNodeAgentDialer) FileDelete(ctx context.Context, nodeID, path string) error {
	return fmt.Errorf("file_delete not yet supported: node-agent lacks DeleteFile RPC (path=%s on %s)", path, nodeID)
}
