package main

import "time"

// RemoteErrorKind classifies errors from aggregator remote operations.
type RemoteErrorKind string

const (
	ErrMCPUnreachable      RemoteErrorKind = "MCP_UNREACHABLE"
	ErrMCPTLSUntrusted     RemoteErrorKind = "MCP_TLS_UNTRUSTED"
	ErrMCPToolNotAllowed   RemoteErrorKind = "MCP_TOOL_NOT_ALLOWED"
	ErrMCPToolNotFound     RemoteErrorKind = "MCP_TOOL_NOT_FOUND"
	ErrMCPTimeout          RemoteErrorKind = "MCP_TIMEOUT"
	ErrMCPResponseInvalid  RemoteErrorKind = "MCP_RESPONSE_INVALID"
	ErrNodeNotRegistered   RemoteErrorKind = "NODE_NOT_REGISTERED"
	ErrNodeClusterIDMismatch RemoteErrorKind = "NODE_CLUSTER_ID_MISMATCH"
)

// MCPTrustLevel describes how strongly the remote node's identity was verified.
type MCPTrustLevel string

const (
	MCPTrustVerified   MCPTrustLevel = "VERIFIED"   // cert chained to cluster CA and SAN matched
	MCPTrustUnverified MCPTrustLevel = "UNVERIFIED" // reachable but TLS identity not verified
	MCPTrustNone       MCPTrustLevel = "NONE"       // not reached
)

// MCPNodeEntry is the registry record for a node's MCP endpoint.
// Stored in etcd at /globular/mcp/nodes/<node-id> when published by the node-agent.
// The aggregator falls back to deriving the URL from cluster node records when
// no etcd entry exists.
type MCPNodeEntry struct {
	NodeID                 string    `json:"node_id"`
	Hostname               string    `json:"hostname"`
	IP                     string    `json:"ip"`
	MCPURL                 string    `json:"mcp_url"`
	MCPPort                int       `json:"mcp_port"`
	ClusterID              string    `json:"cluster_id,omitempty"`
	ReleaseVersion         string    `json:"release_version,omitempty"`
	BuildID                string    `json:"build_id,omitempty"`
	AwarenessBundleVersion string    `json:"awareness_bundle_version,omitempty"`
	LastSeen               time.Time `json:"last_seen"`
	Status                 string    `json:"status"`
}

// RemoteNodeResult holds the outcome of a single remote aggregator operation.
type RemoteNodeResult struct {
	NodeID       string          `json:"node_id"`
	MCPURL       string          `json:"mcp_url"`
	MCPReachable bool            `json:"mcp_reachable"`
	TLSTrust     MCPTrustLevel   `json:"tls_trust"`
	ElapsedMs    int64           `json:"elapsed_ms"`
	Result       interface{}     `json:"result,omitempty"`
	ErrorKind    RemoteErrorKind `json:"error_kind,omitempty"`
	Error        string          `json:"error,omitempty"`
	Warning      string          `json:"warning,omitempty"`
}

// ClusterSnapshotSummary is the compact rollup included in cluster-level responses.
type ClusterSnapshotSummary struct {
	TotalNodes     int `json:"total_nodes"`
	MCPReachable   int `json:"mcp_reachable"`
	MCPUnreachable int `json:"mcp_unreachable"`
}

// aggregatorMCPPort is the canonical default HTTP port for the node-local MCP server.
//
// IMPORTANT — port number authority:
//   - 10260 is the canonical MCP port across the codebase (config.go HTTPListenAddr,
//     awareness/runtime/collector.go, awareness/evidence/normalizer.go,
//     awareness/cli/mcp_cmds.go, etc.).
//   - The aggregator design doc had examples showing 10060 — that was a typo;
//     do NOT use 10060 anywhere.
//   - Per-node overrides come from etcd at /globular/mcp/nodes/<node-id> and are
//     loaded by loadMCPNodeOverrides; nodes that write a different port to that
//     key will be reached on the override port. The aggregator registry prefers
//     the etcd override over this derived default.
//
// This constant is ONLY used to derive an MCP URL when no etcd override exists.
const aggregatorMCPPort = 10260
