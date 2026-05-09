package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// etcdMCPNodePrefix is where nodes publish their MCP endpoint metadata.
// Node-agent writes this key; aggregator reads it preferentially over derived URLs.
const etcdMCPNodePrefix = "/globular/mcp/nodes/"

// listMCPNodes returns all known cluster nodes with their MCP endpoint info.
// It calls the cluster controller for the authoritative node list, then
// enriches each entry from etcd /globular/mcp/nodes/<node-id> if present.
func listMCPNodes(ctx context.Context, pool *clientPool) ([]MCPNodeEntry, error) {
	conn, err := pool.get(ctx, controllerEndpoint())
	if err != nil {
		return nil, fmt.Errorf("aggregator registry: controller unreachable: %w", err)
	}
	client := cluster_controllerpb.NewClusterControllerServiceClient(conn)

	callCtx, cancel := context.WithTimeout(authCtx(ctx), 8*time.Second)
	defer cancel()

	resp, err := client.ListNodes(callCtx, &cluster_controllerpb.ListNodesRequest{})
	if err != nil {
		return nil, fmt.Errorf("aggregator registry: ListNodes: %w", err)
	}

	// Load etcd overrides (best-effort; failures don't block the result).
	overrides := loadMCPNodeOverrides(ctx)

	return mergeNodeOverrides(resp.GetNodes(), overrides), nil
}

// mergeNodeOverrides merges controller-derived node records with per-node etcd
// overrides. Etcd is the authority for actual MCP listen address — when an
// override exists it wins entirely (so e.g. a non-canonical MCP port published
// by the node is preserved end-to-end). Extracted from listMCPNodes so override
// preference can be unit-tested without etcd or the controller.
func mergeNodeOverrides(nodes []*cluster_controllerpb.NodeRecord, overrides map[string]MCPNodeEntry) []MCPNodeEntry {
	out := make([]MCPNodeEntry, 0, len(nodes))
	for _, n := range nodes {
		nodeID := n.GetNodeId()
		if ov, ok := overrides[nodeID]; ok {
			// Etcd override wins. Carry over status/last_seen from controller
			// when the override doesn't provide them, but never override the
			// MCP URL/port/IP — those are the whole point of the override.
			if ov.Status == "" {
				ov.Status = n.GetStatus()
			}
			if ov.LastSeen.IsZero() {
				if ts := n.GetLastSeen(); ts != nil {
					ov.LastSeen = time.Unix(ts.GetSeconds(), int64(ts.GetNanos()))
				}
			}
			out = append(out, ov)
			continue
		}
		out = append(out, deriveMCPNodeEntry(n))
	}
	return out
}

// getMCPNode looks up a single node by ID. Returns NODE_NOT_REGISTERED error
// when the node doesn't exist in the cluster registry.
func getMCPNode(ctx context.Context, pool *clientPool, nodeID string) (*MCPNodeEntry, error) {
	nodes, err := listMCPNodes(ctx, pool)
	if err != nil {
		return nil, err
	}
	for i := range nodes {
		if nodes[i].NodeID == nodeID {
			return &nodes[i], nil
		}
	}
	return nil, fmt.Errorf("%s: node %q", ErrNodeNotRegistered, nodeID)
}

// validateNodeTarget returns the MCPNodeEntry for nodeID or an error with a RemoteErrorKind prefix.
func validateNodeTarget(ctx context.Context, pool *clientPool, nodeID string) (*MCPNodeEntry, error) {
	return getMCPNode(ctx, pool, nodeID)
}

// ── helpers ──────────────────────────────────────────────────────────────────

// deriveMCPNodeEntry builds an MCPNodeEntry from a cluster controller NodeRecord.
func deriveMCPNodeEntry(n *cluster_controllerpb.NodeRecord) MCPNodeEntry {
	hostname := ""
	var ips []string
	if id := n.GetIdentity(); id != nil {
		hostname = id.GetHostname()
		ips = id.GetIps()
	}

	ip := extractIP(n.GetAgentEndpoint())
	if ip == "" && len(ips) > 0 {
		ip = ips[0]
	}

	lastSeen := time.Time{}
	if ts := n.GetLastSeen(); ts != nil {
		lastSeen = time.Unix(ts.GetSeconds(), int64(ts.GetNanos()))
	}

	entry := MCPNodeEntry{
		NodeID:   n.GetNodeId(),
		Hostname: hostname,
		IP:       ip,
		MCPPort:  aggregatorMCPPort,
		LastSeen: lastSeen,
		Status:   n.GetStatus(),
	}
	entry.MCPURL = buildMCPURL(ip, aggregatorMCPPort)
	return entry
}

// extractIP parses the IP portion from an address like "10.0.0.8:11000".
func extractIP(addr string) string {
	if addr == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// Not host:port — try to use as-is if it looks like an IP.
		if net.ParseIP(addr) != nil {
			return addr
		}
		return ""
	}
	return host
}

// buildMCPURL constructs the MCP HTTPS URL for a given IP and port.
func buildMCPURL(ip string, port int) string {
	if ip == "" {
		return ""
	}
	return fmt.Sprintf("https://%s:%d", ip, port)
}

// loadMCPNodeOverrides reads /globular/mcp/nodes/* from etcd and returns
// any published MCPNodeEntry records keyed by node_id. Non-fatal on error.
func loadMCPNodeOverrides(ctx context.Context) map[string]MCPNodeEntry {
	out := make(map[string]MCPNodeEntry)

	cli, err := config.GetEtcdClient()
	if err != nil {
		return out
	}

	readCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	resp, err := cli.Get(readCtx, etcdMCPNodePrefix, clientv3.WithPrefix())
	if err != nil {
		return out
	}

	for _, kv := range resp.Kvs {
		key := strings.TrimPrefix(string(kv.Key), etcdMCPNodePrefix)
		if key == "" {
			continue
		}
		var entry MCPNodeEntry
		if err := json.Unmarshal(kv.Value, &entry); err == nil {
			out[key] = entry
		}
	}
	return out
}
