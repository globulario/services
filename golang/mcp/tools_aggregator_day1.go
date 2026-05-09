package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// ── mcp.day1_classify_node ────────────────────────────────────────────────────
//
// Cluster-level Day-1 classifier: fuses four inputs into a single verdict.
//
//   1. Registry node info       — does the cluster know about this node?
//   2. Remote MCP trust         — can we reach it AND verify its identity?
//   3. Remote local verdict     — what does the node say about itself?
//                                 (awareness.day1_classify_node, executed remotely)
//   4. Etcd desired state       — what should be running on this node?
//                                 (/globular/resources/DesiredService/*)
//
// Aggregator-only invariants enforced here:
//   - MCP unreachable          → BLOCK / MCP_UNREACHABLE
//   - MCP reachable but UNTRUSTED → BLOCK / MCP_REACHABLE_BUT_UNTRUSTED
//                                  (we never inherit a PASS from a node whose
//                                   identity we cannot verify; the remote
//                                   verdict is included in the response for
//                                   transparency, but the aggregator's own
//                                   verdict overrides it.)
//   - Remote verdict BLOCK     → BLOCK / inherit primary blocker
//   - Remote verdict UNKNOWN   → UNKNOWN
//   - Remote verdict PASS,
//     trust=VERIFIED, registry  → PASS
//
// This tool answers "may I treat node-X as DAY1_COMPLETE for the cluster's
// purposes?" — a strictly stricter question than what the local classifier
// can answer alone.

const aggDesiredServicePrefix = "/globular/resources/DesiredService/"

// registerAggregatorDay1Tool is wired into registerAggregatorTools — but kept
// in its own file so the implementation stays focused on the cluster-level
// fusion logic.
func registerAggregatorDay1Tool(s *server) {
	s.register(toolDef{
		Name: "mcp.day1_classify_node",
		Description: `Aggregator-level Day-1 verdict for a single node. Combines:
1. registry lookup (is this node known to the cluster?)
2. MCP reachability + TLS trust level (verified via cluster CA)
3. the node's own awareness.day1_classify_node result (called remotely)
4. cluster-level desired-service state from etcd

The aggregator never inherits PASS from an unverifiable node — an UNVERIFIED
TLS trust always blocks DAY1_COMPLETE, even if the node itself reports healthy.
Use mcp.remote_call awareness.day1_classify_node when you only want the local
view; this tool produces the cluster view.`,
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

		out := map[string]interface{}{
			"node_id":      nodeID,
			"collected_at": time.Now().UTC().Format(time.RFC3339),
		}

		// Step 1 — registry lookup.
		node, regErr := validateNodeTarget(ctx, s.clients, nodeID)
		if regErr != nil {
			applyNodeNotRegistered(out, regErr)
			return out, nil
		}
		out["registry_entry"] = node
		out["mcp_url"] = node.MCPURL

		// Step 4 (early) — attach desired-service inventory regardless of MCP outcome.
		desired, desiredErr := readDesiredServiceNames(ctx)
		if desiredErr == nil {
			out["desired_services"] = desired
		} else {
			out["desired_services_error"] = desiredErr.Error()
		}

		// Step 2 — reachability + trust.
		start := time.Now()
		reachable, trust, _, pingErr := pingRemoteMCP(ctx, node.MCPURL)
		out["mcp_reachable"] = reachable
		out["tls_trust"] = trust
		out["elapsed_ms"] = time.Since(start).Milliseconds()
		if pingErr != nil {
			out["error_kind"] = ErrMCPUnreachable
			out["error"] = pingErr.Error()
		}

		// Apply the aggregator-only invariants on reachability + trust.
		// Returns true when the verdict is final (don't call remote tool).
		if final := applyTrustGate(out, nodeID, reachable, trust); final {
			return out, nil
		}

		// Step 3 — call the remote awareness.day1_classify_node tool.
		remoteResult, remoteTrust, remoteErr := callRemoteTool(
			ctx, node.MCPURL, "awareness.day1_classify_node", nil,
		)
		applyRemoteVerdict(out, remoteResult, remoteTrust, remoteErr)
		return out, nil
	})
}

// applyNodeNotRegistered finalizes the verdict when the registry lookup fails.
func applyNodeNotRegistered(out map[string]interface{}, err error) {
	out["aggregator_verdict"] = "BLOCK"
	out["aggregator_classification"] = "NODE_NOT_REGISTERED"
	out["primary_blocker"] = err.Error()
	out["error_kind"] = ErrNodeNotRegistered
	out["error"] = err.Error()
}

// applyTrustGate enforces the aggregator-only invariants on reachability and
// TLS trust. Returns true when out is finalized and the caller must NOT proceed
// to call the remote tool.
//
// Invariants enforced:
//   - !reachable                         → BLOCK / MCP_UNREACHABLE       (final)
//   - reachable && trust != VERIFIED     → BLOCK / MCP_REACHABLE_BUT_UNTRUSTED (final)
//   - reachable && trust == VERIFIED     → not final (caller proceeds)
//
// The unverified-trust case explicitly does NOT call the remote tool: we
// cannot inherit a PASS from a node we cannot verify. Including the remote
// result anyway would just give the agent a false sense that the node had
// passed somewhere.
func applyTrustGate(out map[string]interface{}, nodeID string, reachable bool, trust MCPTrustLevel) bool {
	if !reachable {
		out["aggregator_verdict"] = "BLOCK"
		out["aggregator_classification"] = "MCP_UNREACHABLE"
		out["primary_blocker"] = "MCP endpoint unreachable for node " + nodeID
		out["forbidden_actions"] = []string{
			"mark node DAY1_COMPLETE",
			"dispatch workloads to this node",
		}
		return true
	}
	if trust != MCPTrustVerified {
		out["aggregator_verdict"] = "BLOCK"
		out["aggregator_classification"] = "MCP_REACHABLE_BUT_UNTRUSTED"
		out["primary_blocker"] = "MCP endpoint reached but TLS identity not verified against cluster CA"
		out["error_kind"] = ErrMCPTLSUntrusted
		out["warning"] = "Aggregator verdict overrides any remote PASS while TLS is unverified."
		out["forbidden_actions"] = []string{
			"mark node DAY1_COMPLETE",
			"trust remote awareness.day1_classify_node verdict",
			"dispatch workloads based on this node's self-report",
		}
		return true
	}
	return false
}

// applyRemoteVerdict folds the remote awareness.day1_classify_node response
// into the aggregator's output. Honors a mid-call trust downgrade: if the
// transport had to retry with InsecureSkipVerify, we treat the response as
// untrusted and BLOCK regardless of the verdict the remote claims.
func applyRemoteVerdict(out map[string]interface{}, remoteResult interface{}, remoteTrust MCPTrustLevel, remoteErr error) {
	if remoteErr != nil {
		out["aggregator_verdict"] = "UNKNOWN"
		out["aggregator_classification"] = "REMOTE_TOOL_ERROR"
		out["primary_blocker"] = remoteErr.Error()
		out["error_kind"] = classifyCallError(remoteErr)
		out["error"] = remoteErr.Error()
		return
	}
	out["remote_verdict"] = remoteResult

	if remoteTrust == MCPTrustUnverified {
		out["aggregator_verdict"] = "BLOCK"
		out["aggregator_classification"] = "MCP_REACHABLE_BUT_UNTRUSTED"
		out["primary_blocker"] = "TLS identity downgraded during remote tool call"
		out["error_kind"] = ErrMCPTLSUntrusted
		return
	}

	remoteVerdict, remoteClass, remoteBlocker := extractRemoteVerdictFields(remoteResult)
	switch remoteVerdict {
	case "PASS":
		out["aggregator_verdict"] = "PASS"
		out["aggregator_classification"] = "DAY1_COMPLETE"
	case "UNKNOWN":
		out["aggregator_verdict"] = "UNKNOWN"
		out["aggregator_classification"] = remoteClass
		out["primary_blocker"] = remoteBlocker
	default:
		out["aggregator_verdict"] = "BLOCK"
		out["aggregator_classification"] = remoteClass
		out["primary_blocker"] = remoteBlocker
	}
}

// readDesiredServiceNames returns the names of services in the cluster's
// desired state, sorted. Uses a short timeout so the aggregator stays responsive
// when etcd is slow.
func readDesiredServiceNames(parent context.Context) ([]string, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()

	resp, err := cli.Get(ctx, aggDesiredServicePrefix, clientv3.WithPrefix(), clientv3.WithKeysOnly())
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		name := strings.TrimPrefix(string(kv.Key), aggDesiredServicePrefix)
		if name != "" {
			out = append(out, name)
		}
	}
	return out, nil
}

// extractRemoteVerdictFields reads verdict/classification/primary_blocker from
// the remote awareness.day1_classify_node response. The remote tool returns a
// JSON object that callRemoteTool already decoded; tolerate either map[string]
// or json.RawMessage shapes.
func extractRemoteVerdictFields(remote interface{}) (verdict, classification, blocker string) {
	switch v := remote.(type) {
	case map[string]interface{}:
		verdict = stringField(v, "verdict")
		classification = stringField(v, "classification")
		blocker = stringField(v, "primary_blocker")
	case json.RawMessage:
		var m map[string]interface{}
		if json.Unmarshal(v, &m) == nil {
			return extractRemoteVerdictFields(m)
		}
	}
	return verdict, classification, blocker
}

func stringField(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
