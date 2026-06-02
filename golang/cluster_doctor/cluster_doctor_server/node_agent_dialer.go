// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.node_agent_dialer
// @awareness file_role=typed_node_agent_transport_for_remediation_actions
// @awareness implements=globular.platform:intent.remediation.must_go_through_workflow
// @awareness implements=globular.platform:intent.autonomy.remediation_is_bounded_and_escalates
// @awareness risk=high
package main

// The FileDelete method on controllerNodeAgentDialer is intentionally a
// stub returning an error. There is no NodeAgentService.DeleteFile RPC
// in the proto and Patch C Milestone 3 deliberately did NOT add one — a
// generic file-delete RPC would broaden the auto-mutation surface
// beyond cache cleanup. The only typed delete the healer can dispatch
// is DELETE_CACHE_ARTIFACT, whose path is constructed inside the
// node-agent (not passed by the caller).
//
// If a future agent considers implementing FileDelete: read
// docs/design/auto-healing-path-unification-patch-c.md first.

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
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
	var fallback string
	for _, n := range resp.GetNodes() {
		if n.GetNodeId() == nodeID {
			found = n.GetAgentEndpoint()
			fallback = fallbackEndpointFromNodeRecord(n)
			break
		}
	}
	if found == "" {
		return "", fmt.Errorf("node %s has no agent_endpoint", nodeID)
	}
	// Prefer direct IP endpoint when the registered endpoint host is non-IP.
	if fallback != "" {
		found = fallback
	}
	d.mu.Lock()
	d.agentEndpoint[nodeID] = found
	d.mu.Unlock()
	return found, nil
}

func fallbackEndpointFromNodeRecord(n *cluster_controllerpb.NodeRecord) string {
	endpoint := strings.TrimSpace(n.GetAgentEndpoint())
	host, port, err := net.SplitHostPort(endpoint)
	if err != nil || port == "" {
		return ""
	}
	if ip := net.ParseIP(host); ip != nil {
		return ""
	}
	candidates := []string{
		strings.TrimSpace(n.GetIdentity().GetAdvertiseIp()),
	}
	for _, ip := range n.GetIdentity().GetIps() {
		candidates = append(candidates, strings.TrimSpace(ip))
	}
	for _, c := range candidates {
		ip := net.ParseIP(c)
		if ip == nil || ip.IsLoopback() || ip.To4() == nil {
			continue
		}
		return net.JoinHostPort(c, port)
	}
	return ""
}

// dialAgent returns an authenticated gRPC connection to a node-agent.
// Caller is responsible for closing the returned connection.
func (d *controllerNodeAgentDialer) dialAgent(ctx context.Context, nodeID string) (*grpc.ClientConn, error) {
	endpoint, err := d.resolveEndpoint(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	target := config.ResolveDialTarget(endpoint)
	conn, err := grpc.NewClient(target.Address,
		grpc.WithTransportCredentials(buildClientTLSCreds(target.ServerName)),
	)
	if err != nil {
		return nil, fmt.Errorf("dial node-agent %s at %s: %w", nodeID, target.Address, err)
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
// its own allowlist check, but Patch C Milestone 3 deliberately avoided
// that path: a generic delete RPC broadens the mutation surface beyond
// cache cleanup. Use DELETE_CACHE_ARTIFACT (typed RPC) instead.
func (d *controllerNodeAgentDialer) FileDelete(ctx context.Context, nodeID, path string) error {
	return fmt.Errorf("file_delete not yet supported: node-agent lacks DeleteFile RPC (path=%s on %s)", path, nodeID)
}

// DeleteCacheArtifact dials the target node-agent and invokes the typed
// node_agent.DeleteCacheArtifact RPC. The node-agent owns path
// construction inside /var/lib/globular/staging/ and re-validates the
// publisher/package inputs server-side. The caller (executor) has already
// validated against isValidPackageIdentifier; this method is the
// transport.
func (d *controllerNodeAgentDialer) DeleteCacheArtifact(ctx context.Context, nodeID, publisherID, packageName string) (string, error) {
	conn, err := d.dialAgent(ctx, nodeID)
	if err != nil {
		return "", fmt.Errorf("dial node %s: %w", nodeID, err)
	}
	defer conn.Close()

	client := node_agentpb.NewNodeAgentServiceClient(conn)
	resp, err := client.DeleteCacheArtifact(ctx, &node_agentpb.DeleteCacheArtifactRequest{
		PackageName: packageName,
		PublisherId: publisherID,
	})
	if err != nil {
		return "", fmt.Errorf("DeleteCacheArtifact RPC on %s: %w", nodeID, err)
	}
	if !resp.GetOk() {
		// Surface the node-agent's rejection (e.g., path-escape detection)
		// as an error so the gate's audit captures it and the failure-rate
		// policy can escalate after repeated rejections.
		return "", fmt.Errorf("node-agent rejected DeleteCacheArtifact (publisher=%s package=%s): %s",
			publisherID, packageName, resp.GetMessage())
	}
	return fmt.Sprintf("deleted cache: publisher=%s package=%s path=%s on %s",
		publisherID, packageName, resp.GetPath(), nodeID), nil
}
