package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// registerAggregatorTools registers the MCP aggregator tool group.
// These tools let an agent query multiple node-local MCP servers through one
// control-plane surface without modifying any existing local MCP tools.
//
// Boundary:
//   node-local MCP  = what do I see here?
//   aggregator MCP  = what does the cluster show across nodes?
//   awareness       = what does that mean?
func registerAggregatorTools(s *server) {

	// ── mcp.cluster_nodes ───────────────────────────────────────────────────
	s.register(toolDef{
		Name:        "mcp.cluster_nodes",
		Description: "Lists all cluster nodes that have (or are expected to have) an MCP endpoint. Returns node_id, hostname, ip, mcp_url, last_seen, and status for each node. Use this to discover which nodes you can target with mcp.remote_ping or mcp.remote_call.",
		InputSchema: inputSchema{Type: "object"},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		nodes, err := listMCPNodes(ctx, s.clients)
		if err != nil {
			return nil, fmt.Errorf("mcp.cluster_nodes: %w", err)
		}
		return map[string]interface{}{
			"collected_at": time.Now().UTC().Format(time.RFC3339),
			"total":        len(nodes),
			"nodes":        nodes,
		}, nil
	})

	// ── mcp.remote_ping ─────────────────────────────────────────────────────
	s.register(toolDef{
		Name:        "mcp.remote_ping",
		Description: "Checks whether the MCP endpoint on a specific node is reachable. Returns mcp_reachable, tls_trust level, and elapsed_ms. A UNVERIFIED trust level means the node responded but its TLS certificate could not be verified against the cluster CA.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"node_id": {Type: "string", Description: "Node ID to probe (from mcp.cluster_nodes)"},
			},
			Required: []string{"node_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		nodeID := getStr(args, "node_id")
		if nodeID == "" {
			return nil, fmt.Errorf("node_id is required")
		}

		node, err := validateNodeTarget(ctx, s.clients, nodeID)
		if err != nil {
			return map[string]interface{}{
				"node_id":      nodeID,
				"mcp_reachable": false,
				"tls_trust":    MCPTrustNone,
				"error_kind":   ErrNodeNotRegistered,
				"error":        err.Error(),
			}, nil
		}

		reachable, trust, elapsedMs, pingErr := pingRemoteMCP(ctx, node.MCPURL)
		result := map[string]interface{}{
			"node_id":       nodeID,
			"mcp_url":       node.MCPURL,
			"mcp_reachable": reachable,
			"tls_trust":     trust,
			"elapsed_ms":    elapsedMs,
		}
		if pingErr != nil {
			result["error_kind"] = ErrMCPUnreachable
			result["error"] = pingErr.Error()
		}
		if trust == MCPTrustUnverified {
			result["warning"] = "MCP endpoint reached but node identity was not fully verified against the cluster CA"
		}
		return result, nil
	})

	// ── mcp.remote_call ─────────────────────────────────────────────────────
	s.register(toolDef{
		Name: "mcp.remote_call",
		Description: `Calls a single read-only tool on a remote node's local MCP server.
Only tools in the phase-1 allowlist may be called remotely.
Forbidden tools (etcd.put, etcd.delete, workflow.execute, etc.) are rejected before any remote call is made.

Returns the remote tool result wrapped in a node-scoped envelope with tls_trust and elapsed_ms.`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"node_id": {Type: "string", Description: "Target node ID (from mcp.cluster_nodes)"},
				"tool":    {Type: "string", Description: "Local MCP tool name to call on the remote node (e.g. awareness.bundle_status)"},
				"args":    {Type: "object", Description: "Arguments to pass to the remote tool (optional)"},
			},
			Required: []string{"node_id", "tool"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		nodeID := getStr(args, "node_id")
		tool := getStr(args, "tool")

		if nodeID == "" {
			return nil, fmt.Errorf("node_id is required")
		}
		if tool == "" {
			return nil, fmt.Errorf("tool is required")
		}

		// Policy check — never make a network call for forbidden tools.
		if !IsRemoteToolAllowed(tool) {
			return map[string]interface{}{
				"node_id":    nodeID,
				"tool":       tool,
				"allowed":    false,
				"error_kind": ErrMCPToolNotAllowed,
				"error":      fmt.Sprintf("tool %q is not in the remote-call allowlist (safety=%s)", tool, ClassifyRemoteToolSafety(tool)),
			}, nil
		}

		node, err := validateNodeTarget(ctx, s.clients, nodeID)
		if err != nil {
			return map[string]interface{}{
				"node_id":    nodeID,
				"error_kind": ErrNodeNotRegistered,
				"error":      err.Error(),
			}, nil
		}

		toolArgs, _ := args["args"].(map[string]interface{})

		start := time.Now()
		result, trust, callErr := callRemoteTool(ctx, node.MCPURL, tool, toolArgs)
		elapsed := time.Since(start).Milliseconds()

		envelope := map[string]interface{}{
			"node_id":    nodeID,
			"mcp_url":    node.MCPURL,
			"tool":       tool,
			"tls_trust":  trust,
			"elapsed_ms": elapsed,
		}
		if callErr != nil {
			envelope["mcp_reachable"] = false
			envelope["error_kind"] = classifyCallError(callErr)
			envelope["error"] = callErr.Error()
		} else {
			envelope["mcp_reachable"] = true
			envelope["result"] = result
		}
		if trust == MCPTrustUnverified {
			envelope["warning"] = "Remote node TLS identity not verified against cluster CA"
		}
		return envelope, nil
	})

	// ── mcp.remote_snapshot ─────────────────────────────────────────────────
	s.register(toolDef{
		Name:        "mcp.remote_snapshot",
		Description: "Collects a standard multi-tool snapshot from a remote node by calling a curated set of read-only local tools: awareness bundle status, runtime errors, Day-1 verdict, node inventory, installed packages, and PKI status. Returns partial results if individual tools fail.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"node_id": {Type: "string", Description: "Target node ID (from mcp.cluster_nodes)"},
			},
			Required: []string{"node_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		nodeID := getStr(args, "node_id")
		if nodeID == "" {
			return nil, fmt.Errorf("node_id is required")
		}

		node, err := validateNodeTarget(ctx, s.clients, nodeID)
		if err != nil {
			return map[string]interface{}{
				"node_id":       nodeID,
				"mcp_reachable": false,
				"error_kind":    ErrNodeNotRegistered,
				"error":         err.Error(),
			}, nil
		}

		start := time.Now()
		snapshot, trust := collectRemoteSnapshot(ctx, node.MCPURL)
		elapsed := time.Since(start).Milliseconds()

		result := map[string]interface{}{
			"node_id":       nodeID,
			"mcp_url":       node.MCPURL,
			"mcp_reachable": true,
			"tls_trust":     trust,
			"elapsed_ms":    elapsed,
			"collected_at":  time.Now().UTC().Format(time.RFC3339),
			"snapshot":      snapshot,
		}
		if trust == MCPTrustUnverified {
			result["warning"] = "Remote node TLS identity not verified against cluster CA"
		}
		return result, nil
	})

	// ── mcp.compare_nodes ───────────────────────────────────────────────────
	s.register(toolDef{
		Name: "mcp.compare_nodes",
		Description: `Compares selected nodes across one or more aspects to surface drift.
Supported aspects: "release" (installed versions), "awareness_bundle" (bundle version/build_id), "packages" (all installed packages), "pki" (certificate status).
Returns per-node data and a diff summary listing nodes that differ from the first responding node.`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"node_ids": {
					Type:        "array",
					Description: "Node IDs to compare (from mcp.cluster_nodes). Min 2.",
					Items:       &propSchema{Type: "string"},
				},
				"aspects": {
					Type:        "array",
					Description: `Aspects to compare. Valid: "release", "awareness_bundle", "packages", "pki". Default: all.`,
					Items:       &propSchema{Type: "string"},
				},
			},
			Required: []string{"node_ids"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		nodeIDs := getStrSliceArg(args, "node_ids")
		if len(nodeIDs) < 2 {
			return nil, fmt.Errorf("node_ids must contain at least 2 node IDs")
		}

		aspects := getStrSliceArg(args, "aspects")
		if len(aspects) == 0 {
			aspects = []string{"release", "awareness_bundle", "packages", "pki"}
		}

		// Map aspect → tool to call.
		aspectTool := map[string]string{
			"release":          "nodeagent_list_installed_packages",
			"awareness_bundle": "awareness.bundle_status",
			"packages":         "nodeagent_list_installed_packages",
			"pki":              "nodeagent_get_certificate_status",
		}

		// Collect per-node data for each aspect concurrently.
		type nodeAspectResult struct {
			nodeID string
			aspect string
			data   interface{}
			err    string
		}

		sem := make(chan struct{}, aggMaxConcurrent)
		var mu sync.Mutex
		var wg sync.WaitGroup
		results := make([]nodeAspectResult, 0, len(nodeIDs)*len(aspects))

		for _, nid := range nodeIDs {
			node, err := validateNodeTarget(ctx, s.clients, nid)
			if err != nil {
				mu.Lock()
				for _, asp := range aspects {
					results = append(results, nodeAspectResult{nodeID: nid, aspect: asp, err: err.Error()})
				}
				mu.Unlock()
				continue
			}

			for _, asp := range aspects {
				tool, ok := aspectTool[asp]
				if !ok {
					mu.Lock()
					results = append(results, nodeAspectResult{nodeID: nid, aspect: asp, err: fmt.Sprintf("unknown aspect %q", asp)})
					mu.Unlock()
					continue
				}

				wg.Add(1)
				go func(n *MCPNodeEntry, aspect, toolName string) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()

					data, _, callErr := callRemoteTool(ctx, n.MCPURL, toolName, nil)
					r := nodeAspectResult{nodeID: n.NodeID, aspect: aspect}
					if callErr != nil {
						r.err = callErr.Error()
					} else {
						r.data = data
					}
					mu.Lock()
					results = append(results, r)
					mu.Unlock()
				}(node, asp, tool)
			}
		}
		wg.Wait()

		// Organize results by aspect then by node. We also track which
		// (aspect, node) pairs had errors so the comparison logic below
		// can skip them — comparing an error result against real data
		// would emit a "differs" finding that is actually a missing-data
		// signal in disguise, exactly the harvest_and_yield principle
		// violation we previously hit in INC-2026-0004.
		byAspect := make(map[string]map[string]interface{})
		erroredPairs := make(map[string]map[string]bool) // aspect -> nodeID -> true
		for _, r := range results {
			if byAspect[r.aspect] == nil {
				byAspect[r.aspect] = make(map[string]interface{})
				erroredPairs[r.aspect] = make(map[string]bool)
			}
			if r.err != "" {
				byAspect[r.aspect][r.nodeID] = map[string]interface{}{"error": r.err}
				erroredPairs[r.aspect][r.nodeID] = true
			} else {
				byAspect[r.aspect][r.nodeID] = r.data
			}
		}

		// Build diff: for each aspect, find nodes that differ from the
		// reference. Skip (aspect, node) pairs where either side had an
		// error — those are MISSING-DATA cases, not drift. Track them
		// separately so the caller can see what could not be compared.
		diffs := make([]map[string]interface{}, 0)
		skipped := make([]map[string]interface{}, 0)
		for _, asp := range aspects {
			nodeData := byAspect[asp]
			if len(nodeData) < 2 {
				continue
			}
			refNodeID := nodeIDs[0]
			refErrored := erroredPairs[asp][refNodeID]
			refData := nodeData[refNodeID]
			for _, nid := range nodeIDs[1:] {
				if refErrored || erroredPairs[asp][nid] {
					reason := "target node errored"
					if refErrored {
						reason = "reference node errored"
					}
					skipped = append(skipped, map[string]interface{}{
						"aspect":    asp,
						"node_id":   nid,
						"reference": refNodeID,
						"reason":    reason,
					})
					continue
				}
				other := nodeData[nid]
				if !deepEqual(refData, other) {
					diffs = append(diffs, map[string]interface{}{
						"aspect":    asp,
						"node_id":   nid,
						"reference": refNodeID,
						"mismatch":  true,
						"finding":   fmt.Sprintf("%s: %s differs from %s", asp, nid, refNodeID),
					})
				}
			}
		}

		// Compute completeness markers — the harvest_and_yield principle
		// asks every aggregating response to declare its completeness so
		// consumers can decide whether to trust drift findings as
		// authoritative or as best-effort. fallback_must_degrade_semantics
		// also applies: returning the same drift_count shape regardless
		// of harvest would let callers mistake a partial sweep for a
		// complete one.
		totalPairs := len(nodeIDs) * len(aspects)
		erroredCount := 0
		for _, perNode := range erroredPairs {
			for range perNode {
				erroredCount++
			}
		}
		harvestPct := 100
		if totalPairs > 0 {
			harvestPct = (totalPairs - erroredCount) * 100 / totalPairs
		}
		partial := erroredCount > 0

		return map[string]interface{}{
			"collected_at": time.Now().UTC().Format(time.RFC3339),
			"node_ids":     nodeIDs,
			"aspects":      aspects,
			"per_node":     byAspect,
			"diffs":        diffs,
			"drift_count":  len(diffs),
			// Harvest/yield envelope. partial=true means at least one
			// (aspect, node) pair could not be fetched; drift_count and
			// diffs reflect only the pairs that succeeded. harvest_pct
			// names the fraction of the requested (node × aspect)
			// universe that contributed to the comparison.
			"partial":       partial,
			"harvest_pct":   harvestPct,
			"skipped_pairs": skipped,
			"errored_count": erroredCount,
		}, nil
	})

	// ── mcp.cluster_snapshot ────────────────────────────────────────────────
	s.register(toolDef{
		Name:        "mcp.cluster_snapshot",
		Description: "Returns a compact cluster-wide MCP view: total nodes, reachable/unreachable counts, and a per-node reachability+trust summary. Unreachable nodes produce partial results, not a total failure. Use mcp.remote_snapshot for drill-down on a specific node.",
		InputSchema: inputSchema{Type: "object"},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		nodes, err := listMCPNodes(ctx, s.clients)
		if err != nil {
			return nil, fmt.Errorf("mcp.cluster_snapshot: %w", err)
		}

		sem := make(chan struct{}, aggMaxConcurrent)
		var mu sync.Mutex
		var wg sync.WaitGroup
		nodeResults := make([]RemoteNodeResult, 0, len(nodes))

		for _, node := range nodes {
			wg.Add(1)
			go func(n MCPNodeEntry) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				reachable, trust, elapsed, pingErr := pingRemoteMCP(ctx, n.MCPURL)
				r := RemoteNodeResult{
					NodeID:       n.NodeID,
					MCPURL:       n.MCPURL,
					MCPReachable: reachable,
					TLSTrust:     trust,
					ElapsedMs:    elapsed,
				}
				if pingErr != nil {
					r.ErrorKind = ErrMCPUnreachable
					r.Error = pingErr.Error()
				}
				if trust == MCPTrustUnverified {
					r.Warning = "TLS identity not verified"
				}
				mu.Lock()
				nodeResults = append(nodeResults, r)
				mu.Unlock()
			}(node)
		}
		wg.Wait()

		summary := ClusterSnapshotSummary{TotalNodes: len(nodeResults)}
		for _, r := range nodeResults {
			if r.MCPReachable {
				summary.MCPReachable++
			} else {
				summary.MCPUnreachable++
			}
		}

		return map[string]interface{}{
			"collected_at": time.Now().UTC().Format(time.RFC3339),
			"partial":      summary.MCPUnreachable > 0,
			"summary":      summary,
			"nodes":        nodeResults,
		}, nil
	})
}

// ── helpers ───────────────────────────────────────────────────────────────────

func classifyCallError(err error) RemoteErrorKind {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case containsAny(msg, string(ErrMCPToolNotFound)):
		return ErrMCPToolNotFound
	case containsAny(msg, "timeout", "deadline"):
		return ErrMCPTimeout
	case containsAny(msg, string(ErrMCPResponseInvalid)):
		return ErrMCPResponseInvalid
	default:
		return ErrMCPUnreachable
	}
}

// getStrSliceArg extracts a []string from an interface{} slice arg.
func getStrSliceArg(args map[string]interface{}, key string) []string {
	v, ok := args[key]
	if !ok {
		return nil
	}
	raw, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

// deepEqual compares two interface{} values by marshaling to JSON.
// Used for aspect comparison in mcp.compare_nodes.
func deepEqual(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	ja, err1 := marshalCompact(a)
	jb, err2 := marshalCompact(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return ja == jb
}

func marshalCompact(v interface{}) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
