package main

import (
	"context"
	"fmt"
	"log"
	"time"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// scyllaRemoveNodeKey is the installed_build_ids key the node agent uses to
// report the local ScyllaDB host UUID. Must match the constant in the node agent.
const scyllaRemoveNodeKey = "scylla:host_id"

// removeNodeFromScyllaRing calls "nodetool removenode <hostID>" on any healthy
// ScyllaDB peer, removing a dead node from the gossip ring.
//
// It finds the first peer that:
//   - has scylla-server.service active
//   - is NOT the node being removed
//   - has a reachable agent endpoint
//
// Returns nil if no ScyllaDB peers exist (single-node cluster or ScyllaDB not
// deployed) — the caller should treat that as a no-op, not an error.
// Returns an error only if a peer was found but the removenode call failed.
func (srv *server) removeNodeFromScyllaRing(ctx context.Context, removedNodeID, scyllaHostID string) error {
	if scyllaHostID == "" {
		log.Printf("scylla-ring-remove: no ScyllaDB host ID for node %s — skipping ring removal (nodetool removenode must be run manually)", removedNodeID)
		return nil
	}

	// Find a healthy ScyllaDB peer (not the node being removed).
	srv.lock("scylla-ring-remove:find-peer")
	var peerEndpoint string
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
		break
	}
	srv.unlock()

	if peerEndpoint == "" {
		log.Printf("scylla-ring-remove: no healthy ScyllaDB peer found — skipping ring removal for host %s (run nodetool removenode %s manually from a ring member)", scyllaHostID, scyllaHostID)
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
