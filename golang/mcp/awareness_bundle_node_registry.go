package main

import (
	"context"

	"github.com/globulario/awareness/bundlesync"
)

// ── bundlesync.NodeRegistry adapter ──────────────────────────────────────────
//
// bundlesync's discovery layer is decoupled from MCP/etcd by design — this
// file is the bridge. It exposes the etcd-backed /globular/mcp/nodes/*
// registry (already used by the aggregator in aggregator_registry.go) as a
// bundlesync.NodeRegistry, so Phase C.4's orchestrator can call DiscoverSources
// with a real cluster registry without bundlesync importing etcd or MCP types.
//
// The adapter ONLY reads etcd overrides — controller-derived peers (from
// ListNodes) are intentionally NOT included here because the aggregator-derived
// list is the operator's view, while the bundle puller wants the publisher's
// view: a peer is a valid bundle source only if it has actively published its
// MCP endpoint, release, and bundle version. A controller-known node that has
// never published to the MCP registry is a node we shouldn't trust to serve a
// bundle.

// mcpNodesRegistry implements bundlesync.NodeRegistry by reading
// /globular/mcp/nodes/* from etcd. The lookup is cheap (single prefix Get).
type mcpNodesRegistry struct{}

// newMCPNodesRegistry returns the singleton-shaped registry. Stateless on
// purpose — etcd connection comes from config.GetEtcdClient() each time.
func newMCPNodesRegistry() *mcpNodesRegistry {
	return &mcpNodesRegistry{}
}

// ListNodes satisfies bundlesync.NodeRegistry.
func (r *mcpNodesRegistry) ListNodes(ctx context.Context) ([]bundlesync.NodeRegistryEntry, error) {
	overrides := loadMCPNodeOverrides(ctx)
	out := make([]bundlesync.NodeRegistryEntry, 0, len(overrides))
	for _, e := range overrides {
		out = append(out, mcpEntryToBundlesync(e))
	}
	return out, nil
}

// mcpEntryToBundlesync projects MCPNodeEntry onto bundlesync.NodeRegistryEntry.
// Keep this close to the source MCPNodeEntry definition so a field rename in
// one place is caught by a compile error in the other.
func mcpEntryToBundlesync(e MCPNodeEntry) bundlesync.NodeRegistryEntry {
	return bundlesync.NodeRegistryEntry{
		NodeID:                 e.NodeID,
		PeerURL:                e.MCPURL,
		ClusterID:              e.ClusterID,
		ReleaseVersion:         e.ReleaseVersion,
		BuildID:                e.BuildID,
		AwarenessBundleVersion: e.AwarenessBundleVersion,
		LastSeen:               e.LastSeen,
		Status:                 e.Status,
	}
}
