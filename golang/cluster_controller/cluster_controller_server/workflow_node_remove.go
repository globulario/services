// @awareness namespace=globular.platform
// @awareness component=platform_controller.workflow_node_remove
// @awareness file_role=controller_side_handlers_and_dispatch_for_node_remove_workflow
// @awareness implements=globular.platform:invariant.workflow.every_state_mutation_belongs_to_a_workflow_instance
// @awareness protects=globular.platform:failure_mode.hidden_workflow.controller_remove_node_inline_preflight_and_drain
// @awareness protects=globular.platform:failure_mode.hidden_workflow.controller_node_removal_requests_queue_consumer
// @awareness risk=high
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/globulario/services/golang/workflow/engine"
)

// workflow_node_remove.go — controller-side handlers and dispatch helper
// for the node.remove workflow. Lifts the previously-inline RemoveNode
// RPC + processNodeRemovalRequests queue consumer (failure_modes
// hidden_workflow.controller_remove_node_inline_preflight_and_drain and
// hidden_workflow.controller_node_removal_requests_queue_consumer) into
// a declarative workflow with per-step durable receipts.

// buildNodeRemoveControllerConfig assembles the engine.NodeRemoveControllerConfig
// the workflow needs. Each closure delegates to an existing controller
// helper so the canonical typed-RPC contracts and locking boundaries
// (srv.lock / srv.unlock) are preserved.
func (srv *server) buildNodeRemoveControllerConfig() engine.NodeRemoveControllerConfig {
	return engine.NodeRemoveControllerConfig{
		Preflight: func(ctx context.Context, nodeID string) ([]engine.NodeRemovePreflightViolation, error) {
			srv.lock("node-remove-preflight")
			defer srv.unlock()
			raw := srv.topologyPreflightForRemove(nodeID)
			out := make([]engine.NodeRemovePreflightViolation, 0, len(raw))
			for _, v := range raw {
				out = append(out, engine.NodeRemovePreflightViolation{
					Code:    v.Kind,
					Message: v.Message,
				})
			}
			return out, nil
		},

		RemoveEtcdMembership: func(ctx context.Context, nodeID string) error {
			return srv.removeNodeEtcdMembership(ctx, nodeID)
		},

		DrainNode: func(ctx context.Context, nodeID, opID, agentEndpoint string) error {
			// drainNode operates on the *nodeState pointer; look it up
			// from current state. If the node has already been deleted
			// (e.g. previous workflow attempt crashed after delete_state
			// but before drain_node), report success — there's nothing
			// to drain.
			srv.lock("node-remove-drain-lookup")
			node := srv.state.Nodes[nodeID]
			srv.unlock()
			if node == nil {
				log.Printf("node-remove: drain_node — node %s already absent from state, treating as drained", nodeID)
				return nil
			}
			return srv.drainNode(ctx, node, opID)
		},

		DeleteState: func(ctx context.Context, nodeID string) error {
			srv.lock("node-remove-delete-state")
			defer srv.unlock()
			node, exists := srv.state.Nodes[nodeID]
			if !exists {
				// Idempotent: a re-run finds the node already deleted.
				return nil
			}
			// Clean MinioPoolNodes: remove the node's IP so stale entries
			// don't persist after removal. filterEligiblePoolIPsLocked
			// filters at publish time, but cleaning here keeps the state
			// consistent and avoids stale-ref drift on controller restart.
			if len(srv.state.MinioPoolNodes) > 0 && node != nil {
				nodeIP := node.StableIP(srv.clusterVIP())
				if nodeIP == "" {
					nodeIP = node.PrimaryIP()
				}
				if nodeIP != "" {
					var kept []string
					for _, ip := range srv.state.MinioPoolNodes {
						if ip != nodeIP {
							kept = append(kept, ip)
						}
					}
					if len(kept) != len(srv.state.MinioPoolNodes) {
						log.Printf("node-remove: cleaned MinioPoolNodes: removed %s for node %s", nodeIP, nodeID)
						srv.state.MinioPoolNodes = kept
					}
				}
			}
			delete(srv.state.Nodes, nodeID)
			if err := srv.persistStateLocked(true); err != nil {
				return fmt.Errorf("persist node removal: %w", err)
			}
			return nil
		},

		PublishScyllaHosts: func(ctx context.Context) error {
			srv.publishScyllaHostsIfNeeded(ctx)
			return nil
		},

		CleanupEtcdPrefixes: func(ctx context.Context, nodeID string) (int64, error) {
			prefixes := []string{
				fmt.Sprintf("/globular/nodes/%s/", nodeID),
				fmt.Sprintf("/globular/ingress/v1/status/%s", nodeID),
				fmt.Sprintf("globular/plans/v1/nodes/%s/", nodeID),
				fmt.Sprintf("globular/cluster/v1/observed_hash_services/%s", nodeID),
				fmt.Sprintf("globular/cluster/v1/applied_hash_services/%s", nodeID),
				fmt.Sprintf("globular/cluster/v1/fail_count_services/%s", nodeID),
				// Convergence outcome records (package install/block history).
				// Stale BLOCKED_MISSING_NATIVE_DEP records here permanently
				// prevent convergence on rejoin — they must be wiped.
				fmt.Sprintf("/globular/convergence/nodes/%s/", nodeID),
				// Convergence action records written by the controller.
				fmt.Sprintf("/globular/convergence/actions/controller/%s/", nodeID),
			}
			var total int64
			for _, p := range prefixes {
				total += srv.cleanNodeEtcdPrefix(ctx, nodeID, p)
			}
			return total, nil
		},

		CleanupReleaseStatus: func(ctx context.Context, nodeID string) (int, error) {
			return srv.cleanNodeFromReleases(ctx, nodeID), nil
		},

		RemoveFromScyllaRing: func(ctx context.Context, nodeID, scyllaHostID string, nodeIPs []string) error {
			return srv.removeNodeFromScyllaRing(ctx, nodeID, scyllaHostID, nodeIPs)
		},
	}
}

// nodeRemoveInputs is the input bag dispatchNodeRemove builds at RPC time.
// Captured under srv.lock so the workflow receives a consistent snapshot.
type nodeRemoveInputs struct {
	NodeID         string
	Hostname       string
	NodeIPs        []string
	ScyllaHostID   string
	AgentEndpoint  string
	Force          bool
	Drain          bool
	OpID           string
}

// dispatchNodeRemove starts a node.remove workflow run for the given target
// and waits for terminal status. Called by both the RemoveNode RPC handler
// and the node-removal-request queue consumer (single call site shared
// between them — the queue consumer just passes Force=true, Drain=false).
func (srv *server) dispatchNodeRemove(ctx context.Context, in nodeRemoveInputs) (runID, status, errMsg string, err error) {
	router := engine.NewRouter()
	engine.RegisterNodeRemoveControllerActions(router, srv.buildNodeRemoveControllerConfig())

	nodeIPsAny := make([]any, 0, len(in.NodeIPs))
	for _, ip := range in.NodeIPs {
		nodeIPsAny = append(nodeIPsAny, ip)
	}
	inputs := map[string]any{
		"cluster_id":     srv.cfg.ClusterDomain,
		"node_id":        in.NodeID,
		"op_id":          in.OpID,
		"hostname":       in.Hostname,
		"node_ips":       nodeIPsAny,
		"scylla_host_id": in.ScyllaHostID,
		"agent_endpoint": in.AgentEndpoint,
		"force":          in.Force,
		"drain":          in.Drain,
	}
	// correlation_id: per-node, time-suffixed so concurrent requests for
	// the same node collapse to one run within a 1-second window. The
	// workflow's mode=single ensures only one instance progresses;
	// duplicates dedupe naturally.
	corrID := fmt.Sprintf("node-remove-%s-%d", in.NodeID, time.Now().Unix())

	resp, runErr := srv.executeWorkflowCentralized(ctx, "node.remove", corrID, inputs, router)
	if runErr != nil {
		return "", "", "", fmt.Errorf("dispatch node.remove: %w", runErr)
	}
	return resp.RunId, resp.Status, resp.Error, nil
}
