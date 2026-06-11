// @awareness namespace=globular.platform
// @awareness component=platform_services_mobility.nodeagent_controller
// @awareness file_role=production_node_agent_controller_calls_generated_grpc_client
// @awareness risk=medium
package mobility

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// NodeRecord is the subset of cluster-controller's NodeRecord we need.
// We deliberately do NOT depend on the cluster_controller_server
// package — its types may evolve and we only need two fields.
type NodeRecord struct {
	NodeID        string
	AgentEndpoint string // host:port the node-agent listens on
}

// NodeAgentControllerImpl is the production NodeAgentController. It
// holds a node-ID → AgentEndpoint mapping plus the TLS material needed
// to dial each node-agent.
type NodeAgentControllerImpl struct {
	mu       sync.Mutex
	nodes    map[string]NodeRecord
	tlsCfg   *tls.Config
	connPool map[string]*grpc.ClientConn // endpoint → conn
}

// NewNodeAgentControllerImpl constructs a controller using the
// cluster's standard service cert and CA bundle for mTLS.
//
// caPath is typically /var/lib/globular/pki/ca.crt.
// certPath / keyPath are typically the cluster's service.crt /
// service.key.
//
// The nodes slice seeds the node-ID → AgentEndpoint map; callers can
// also call SetNodes later if the topology changes.
func NewNodeAgentControllerImpl(nodes []NodeRecord, caPath, certPath, keyPath string) (*NodeAgentControllerImpl, error) {
	caBytes, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("read CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caBytes) {
		return nil, fmt.Errorf("parse CA bundle from %s", caPath)
	}
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("load client cert: %w", err)
	}
	tlsCfg := &tls.Config{
		RootCAs:      pool,
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
	n := &NodeAgentControllerImpl{
		nodes:    map[string]NodeRecord{},
		tlsCfg:   tlsCfg,
		connPool: map[string]*grpc.ClientConn{},
	}
	n.SetNodes(nodes)
	return n, nil
}

// SetNodes replaces the node-ID → AgentEndpoint map. Safe to call
// while migrations are in flight; the map lookup is locked.
func (n *NodeAgentControllerImpl) SetNodes(nodes []NodeRecord) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.nodes = make(map[string]NodeRecord, len(nodes))
	for _, r := range nodes {
		n.nodes[r.NodeID] = r
	}
}

// Close drops every cached connection. Safe to call concurrently with
// in-flight RPCs; subsequent RPCs reconnect on demand.
func (n *NodeAgentControllerImpl) Close() {
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, c := range n.connPool {
		_ = c.Close()
	}
	n.connPool = map[string]*grpc.ClientConn{}
}

func (n *NodeAgentControllerImpl) endpoint(nodeID string) (string, error) {
	n.mu.Lock()
	rec, ok := n.nodes[nodeID]
	n.mu.Unlock()
	if !ok {
		return "", fmt.Errorf("node %q not in topology", nodeID)
	}
	if rec.AgentEndpoint == "" {
		return "", fmt.Errorf("node %q has empty AgentEndpoint", nodeID)
	}
	return rec.AgentEndpoint, nil
}

func (n *NodeAgentControllerImpl) client(ctx context.Context, nodeID string) (node_agentpb.NodeAgentServiceClient, error) {
	endpoint, err := n.endpoint(nodeID)
	if err != nil {
		return nil, err
	}

	n.mu.Lock()
	conn := n.connPool[endpoint]
	n.mu.Unlock()
	if conn != nil {
		return node_agentpb.NewNodeAgentServiceClient(conn), nil
	}

	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, err = grpc.DialContext(dialCtx, endpoint,
		grpc.WithTransportCredentials(credentials.NewTLS(n.tlsCfg)),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("dial node-agent %s: %w", endpoint, err)
	}
	n.mu.Lock()
	n.connPool[endpoint] = conn
	n.mu.Unlock()
	return node_agentpb.NewNodeAgentServiceClient(conn), nil
}

// IsNodeReachable returns true if the node-agent responds to a Ping
// (we use ControlService with action=status as a cheap reachability
// check — ControlService is allow-listed and idempotent).
func (n *NodeAgentControllerImpl) IsNodeReachable(ctx context.Context, nodeID string) (bool, error) {
	c, err := n.client(ctx, nodeID)
	if err != nil {
		// Return the error so callers can distinguish config/credential
		// failures from genuine node unreachability.
		return false, fmt.Errorf("IsNodeReachable: node %s client error: %w", nodeID, err)
	}
	rctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	// Cheap probe — query a benign unit. The node-agent accepts any
	// allow-listed unit name; we use globular-node-agent.service which
	// is always present on a node where the node-agent is running.
	_, err = c.ControlService(rctx, &node_agentpb.ControlServiceRequest{
		Unit:   "globular-node-agent.service",
		Action: "status",
	})
	if err != nil {
		return false, nil
	}
	return true, nil
}

// IsServiceBinaryInstalled queries the node-agent's installed-packages
// list for the named service. The match is case-insensitive on the
// short name (e.g. "ai-memory" matches the package named
// "ai-memory" or "ai_memory").
func (n *NodeAgentControllerImpl) IsServiceBinaryInstalled(ctx context.Context, nodeID, serviceName string) (bool, error) {
	c, err := n.client(ctx, nodeID)
	if err != nil {
		return false, err
	}
	rctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	resp, err := c.ListInstalledPackages(rctx, &node_agentpb.ListInstalledPackagesRequest{})
	if err != nil {
		return false, fmt.Errorf("ListInstalledPackages: %w", err)
	}
	short := shortServiceName(serviceName)
	for _, pkg := range resp.GetPackages() {
		if matchesPackageName(pkg.GetName(), short) {
			return true, nil
		}
	}
	return false, nil
}

// StartService asks the node-agent to start the systemd unit owning
// `serviceName`. The unit is conventionally globular-<short-name>.service.
func (n *NodeAgentControllerImpl) StartService(ctx context.Context, nodeID, serviceName string) error {
	return n.controlUnit(ctx, nodeID, serviceName, "start")
}

// StopService is the symmetric operation. systemd's graceful-stop
// period gives in-flight requests time to complete.
func (n *NodeAgentControllerImpl) StopService(ctx context.Context, nodeID, serviceName string) error {
	return n.controlUnit(ctx, nodeID, serviceName, "stop")
}

func (n *NodeAgentControllerImpl) controlUnit(ctx context.Context, nodeID, serviceName, action string) error {
	c, err := n.client(ctx, nodeID)
	if err != nil {
		return err
	}
	rctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	unit := unitNameFor(serviceName)
	resp, err := c.ControlService(rctx, &node_agentpb.ControlServiceRequest{
		Unit:   unit,
		Action: action,
	})
	if err != nil {
		return fmt.Errorf("%s %s on node %s: %w", action, unit, nodeID, err)
	}
	if resp != nil && !resp.GetOk() {
		return fmt.Errorf("%s %s on node %s: %s", action, unit, nodeID, resp.GetMessage())
	}
	return nil
}

// shortServiceName normalizes "ai_memory.AiMemoryService" or
// "ai-memory" to "ai-memory" for unit-name + package-name matching.
func shortServiceName(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	if dot := strings.Index(s, "."); dot >= 0 {
		s = s[:dot]
	}
	return strings.ReplaceAll(s, "_", "-")
}

// unitNameFor produces the systemd unit name. We follow Globular's
// convention of "globular-<short>.service" for everything except a
// small allow-list of upstream-named units; the node-agent's
// isAllowedUnit gate enforces the same allow-list server-side.
func unitNameFor(serviceName string) string {
	short := shortServiceName(serviceName)
	switch short {
	case "scylladb":
		return "scylla-server.service"
	case "keepalived":
		return "keepalived.service"
	}
	return "globular-" + short + ".service"
}

func matchesPackageName(stored, want string) bool {
	s := strings.ToLower(strings.TrimSpace(stored))
	s = strings.ReplaceAll(s, "_", "-")
	return s == want
}
