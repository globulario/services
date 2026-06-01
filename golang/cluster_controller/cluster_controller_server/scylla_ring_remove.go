// @awareness namespace=globular.platform
// @awareness component=platform_controller
// @awareness file_role=scylladb_ring_removal_orchestration
// @awareness implements=globular.platform:intent.quorum_safety_before_storage_mutation
// @awareness risk=high
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// scyllaRemoveNodeKey is the installed_build_ids key the node agent uses to
// report the local ScyllaDB host UUID. Must match the constant in the node agent.
const scyllaRemoveNodeKey = "scylla:host_id"

// scyllaRestPort is the ScyllaDB REST API port (not CQL, not JMX).
const scyllaRestPort = "10000"

// removeNodeFromScyllaRing removes a dead node from the ScyllaDB gossip ring.
//
// When scyllaHostID is empty (node was wiped before RemoveNode was called and
// never reported its host UUID via heartbeat), the function falls back to
// querying a healthy peer's ScyllaDB REST API to resolve the host ID from the
// departing node's IP addresses.
//
// It finds the first healthy peer that:
//   - is NOT the node being removed
//   - has scylla-server.service active
//   - has a reachable agent endpoint
//
// Returns nil if no ScyllaDB peers exist (single-node cluster or ScyllaDB not
// deployed) — the caller should treat that as a no-op, not an error.
// Returns an error only if a peer was found but the removenode call failed.
func (srv *server) removeNodeFromScyllaRing(ctx context.Context, removedNodeID, scyllaHostID string, nodeIPs []string) error {
	// Find a healthy ScyllaDB peer (not the node being removed).
	srv.lock("scylla-ring-remove:find-peer")
	var peerEndpoint string
	var peerIP string
	for id, node := range srv.state.Nodes {
		if id == removedNodeID {
			continue
		}
		if node.AgentEndpoint == "" {
			continue
		}
		if !nodeHasScyllaRunning(node) {
			continue
		}
		peerEndpoint = node.AgentEndpoint
		if len(node.Identity.Ips) > 0 {
			peerIP = node.Identity.Ips[0]
		}
		break
	}
	srv.unlock()

	if peerEndpoint == "" {
		log.Printf("scylla-ring-remove: no healthy ScyllaDB peer found — skipping ring removal for node %s (run nodetool removenode manually)", removedNodeID)
		return nil
	}

	// Resolve scyllaHostID from REST API when the node-agent never reported it
	// (e.g. node was wiped before RemoveNode was called).
	if scyllaHostID == "" && peerIP != "" && len(nodeIPs) > 0 {
		resolved, err := scyllaLookupHostIDByIP(ctx, peerIP, nodeIPs)
		if err != nil {
			log.Printf("scylla-ring-remove: REST fallback lookup failed: %v — will attempt removenode without host_id", err)
		} else if resolved != "" {
			log.Printf("scylla-ring-remove: resolved host_id=%s for node %s via REST API on peer %s", resolved, removedNodeID, peerIP)
			scyllaHostID = resolved
		}
	}

	if scyllaHostID == "" {
		log.Printf("scylla-ring-remove: host ID unknown for node %s (IPs=%v) — skipping ring removal (run nodetool removenode manually from a ring member)", removedNodeID, nodeIPs)
		return nil
	}

	log.Printf("scylla-ring-remove: calling nodetool removenode %s via peer %s", scyllaHostID, peerEndpoint)
	conn, _, err := srv.dialNodeAgentForEndpoint(peerEndpoint)
	if err != nil {
		return fmt.Errorf("scylla-ring-remove: connect to peer %s: %w", peerEndpoint, err)
	}
	defer conn.Close()

	client := node_agentpb.NewNodeAgentServiceClient(conn)
	rctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	resp, err := client.RunWorkflow(rctx, &node_agentpb.RunWorkflowRequest{
		WorkflowName: "scylla-remove-node",
		Inputs:       map[string]string{"host_id": scyllaHostID},
	})
	if err != nil {
		return fmt.Errorf("scylla-ring-remove: RunWorkflow on peer %s: %w", peerEndpoint, err)
	}
	if resp.GetStatus() != "SUCCEEDED" {
		return fmt.Errorf("scylla-ring-remove: nodetool removenode %s failed: %s", scyllaHostID, resp.GetError())
	}

	log.Printf("scylla-ring-remove: node %s (host=%s) removed from ScyllaDB ring", removedNodeID, scyllaHostID)
	return nil
}

// scyllaLookupHostIDByIP queries the ScyllaDB REST API on peerIP to find the
// host ID for any of the given candidate IPs. Uses the /storage_service/host_id
// endpoint which returns a map of endpoint→host_id for all ring members.
//
// ScyllaDB REST API: GET http://<peer>:10000/storage_service/host_id
// Response: [{"key":"<ip>","value":"<host_id>"}, ...]
func scyllaLookupHostIDByIP(ctx context.Context, peerIP string, candidateIPs []string) (string, error) {
	url := fmt.Sprintf("http://%s:%s/storage_service/host_id", peerIP, scyllaRestPort)
	rctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(rctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("GET %s: status %d: %s", url, resp.StatusCode, body)
	}

	// Response is an array of {"key": "<endpoint>", "value": "<host_id>"}.
	var entries []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return "", fmt.Errorf("decode response from %s: %w", url, err)
	}

	candidateSet := make(map[string]struct{}, len(candidateIPs))
	for _, ip := range candidateIPs {
		candidateSet[strings.TrimSpace(ip)] = struct{}{}
	}

	for _, e := range entries {
		ep := strings.TrimSpace(e.Key)
		// Strip port if present.
		if idx := strings.LastIndex(ep, ":"); idx > 0 && !strings.Contains(ep[:idx], ":") {
			ep = ep[:idx]
		}
		if _, ok := candidateSet[ep]; ok {
			return strings.TrimSpace(e.Value), nil
		}
	}

	return "", fmt.Errorf("no ring entry found for IPs %v in %d ring members", candidateIPs, len(entries))
}
