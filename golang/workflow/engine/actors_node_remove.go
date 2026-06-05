// @awareness namespace=globular.platform
// @awareness component=platform_workflow.actors_node_remove
// @awareness file_role=node_remove_workflow_actions_lift_of_hidden_workflow
// @awareness implements=globular.platform:intent.workflow.source_of_operational_truth
// @awareness implements=globular.platform:invariant.workflow.every_state_mutation_belongs_to_a_workflow_instance
// @awareness protects=globular.platform:failure_mode.hidden_workflow.controller_remove_node_inline_preflight_and_drain
// @awareness protects=globular.platform:failure_mode.hidden_workflow.controller_node_removal_requests_queue_consumer
// @awareness risk=high
package engine

import (
	"context"
	"fmt"
	"log"

	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// ──────────────────────────────────────────────────────────────────────────
// node.remove controller actions
// ──────────────────────────────────────────────────────────────────────────
//
// node.remove is the workflow-native replacement for the previously-inline
// RemoveNode RPC + processNodeRemovalRequests queue consumer
// (cluster_controller_server/handlers_node.go and node_removal_requests.go).
//
// Both call sites — operator-initiated RPC and queued removal request —
// now dispatch this single workflow per (node_id) target. The 8 declared
// steps each carry their own actor binding, idempotency declaration, and
// resume policy. A controller crash mid-removal has a deterministic
// resume point per step receipt.
//
// The RPC handler captures the node's mutable state (ScyllaHostID,
// NodeIPs, Hostname, AgentEndpoint) under the server lock BEFORE dispatch,
// and passes them as workflow inputs. The workflow steps operate on
// those inputs without re-reading state.Nodes.

// NodeRemovePreflightViolation names a single topology safety violation
// returned by the preflight step.
type NodeRemovePreflightViolation struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// NodeRemoveControllerConfig wires the controller's typed helpers into the
// node.remove workflow action handlers. Each closure delegates to an
// existing controller method so the canonical RPC contracts and locking
// boundaries are preserved.
type NodeRemoveControllerConfig struct {
	// Preflight runs topology safety checks. Returns the violations list
	// (empty when safe). The handler decides whether to block based on
	// the `force` input.
	Preflight func(ctx context.Context, nodeID string) ([]NodeRemovePreflightViolation, error)

	// RemoveEtcdMembership prunes the node from etcd cluster membership
	// if it was an etcd member. No-op for non-etcd nodes.
	RemoveEtcdMembership func(ctx context.Context, nodeID string) error

	// DrainNode best-effort stops node-agent services. Errors are
	// returned to the handler, which gates final fail/pass on the
	// `force` input.
	DrainNode func(ctx context.Context, nodeID, opID, agentEndpoint string) error

	// DeleteState removes the node from controller in-memory state and
	// persists. Atomic from the workflow's perspective.
	DeleteState func(ctx context.Context, nodeID string) error

	// PublishScyllaHosts re-publishes the Scylla seed host list after
	// the node has been removed from membership.
	PublishScyllaHosts func(ctx context.Context) error

	// CleanupEtcdPrefixes deletes per-node etcd prefixes (packages,
	// ingress status, plans, convergence outcomes, controller actions).
	// Returns the total number of keys deleted across all prefixes.
	CleanupEtcdPrefixes func(ctx context.Context, nodeID string) (int64, error)

	// CleanupReleaseStatus purges the node from ServiceRelease /
	// InfrastructureRelease status objects. Returns the number of
	// release objects modified.
	CleanupReleaseStatus func(ctx context.Context, nodeID string) (int, error)

	// RemoveFromScyllaRing removes the node from the ScyllaDB gossip
	// ring via a healthy peer. Non-fatal: errors are recorded in the
	// step receipt but the workflow continues.
	RemoveFromScyllaRing func(ctx context.Context, nodeID, scyllaHostID string, nodeIPs []string) error
}

// RegisterNodeRemoveControllerActions registers the eight controller-side
// actions the node.remove workflow YAML declares.
func RegisterNodeRemoveControllerActions(router *Router, cfg NodeRemoveControllerConfig) {
	router.Register(v1alpha1.ActorClusterController, "controller.node_remove.preflight",
		nodeRemovePreflight(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.node_remove.remove_etcd_membership",
		nodeRemoveEtcdMembership(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.node_remove.drain_node",
		nodeRemoveDrainNode(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.node_remove.delete_state",
		nodeRemoveDeleteState(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.node_remove.publish_scylla_hosts",
		nodeRemovePublishScyllaHosts(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.node_remove.cleanup_etcd_prefixes",
		nodeRemoveCleanupEtcdPrefixes(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.node_remove.cleanup_release_status",
		nodeRemoveCleanupReleaseStatus(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.node_remove.remove_scylla_ring",
		nodeRemoveScyllaRing(cfg))
}

func nodeRemovePreflight(cfg NodeRemoveControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		if cfg.Preflight == nil {
			return nil, fmt.Errorf("node_remove.preflight: handler not wired")
		}
		nodeID, _ := req.With["node_id"].(string)
		if nodeID == "" {
			return nil, fmt.Errorf("node_remove.preflight: node_id is required")
		}
		force, _ := req.With["force"].(bool)
		violations, err := cfg.Preflight(ctx, nodeID)
		if err != nil {
			return nil, fmt.Errorf("preflight: %w", err)
		}
		if len(violations) > 0 && !force {
			// Force the workflow to fail with an actionable message.
			msgs := make([]string, 0, len(violations))
			for _, v := range violations {
				msgs = append(msgs, v.Message)
			}
			return nil, fmt.Errorf("topology safety violation — use force=true to override: %v", msgs)
		}
		log.Printf("actor[controller]: node_remove.preflight node=%s violations=%d force=%v",
			nodeID, len(violations), force)
		return &ActionResult{
			OK: true,
			Output: map[string]any{
				"violation_count": len(violations),
				"force_override":  force && len(violations) > 0,
			},
		}, nil
	}
}

func nodeRemoveEtcdMembership(cfg NodeRemoveControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		if cfg.RemoveEtcdMembership == nil {
			return nil, fmt.Errorf("node_remove.remove_etcd_membership: handler not wired")
		}
		nodeID, _ := req.With["node_id"].(string)
		if nodeID == "" {
			return nil, fmt.Errorf("node_remove.remove_etcd_membership: node_id is required")
		}
		if err := cfg.RemoveEtcdMembership(ctx, nodeID); err != nil {
			return nil, fmt.Errorf("remove etcd membership: %w", err)
		}
		log.Printf("actor[controller]: node_remove.remove_etcd_membership node=%s", nodeID)
		return &ActionResult{OK: true}, nil
	}
}

func nodeRemoveDrainNode(cfg NodeRemoveControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		if cfg.DrainNode == nil {
			return nil, fmt.Errorf("node_remove.drain_node: handler not wired")
		}
		nodeID, _ := req.With["node_id"].(string)
		opID, _ := req.With["op_id"].(string)
		agentEndpoint, _ := req.With["agent_endpoint"].(string)
		force, _ := req.With["force"].(bool)
		if nodeID == "" || agentEndpoint == "" {
			return nil, fmt.Errorf("node_remove.drain_node: node_id + agent_endpoint required")
		}
		drainErr := cfg.DrainNode(ctx, nodeID, opID, agentEndpoint)
		if drainErr != nil && !force {
			return nil, fmt.Errorf("drain failed (use force=true to override): %w", drainErr)
		}
		out := map[string]any{}
		if drainErr != nil {
			out["drain_error"] = drainErr.Error()
			log.Printf("actor[controller]: node_remove.drain_node node=%s err=%v force_override=true",
				nodeID, drainErr)
		} else {
			log.Printf("actor[controller]: node_remove.drain_node node=%s OK", nodeID)
		}
		return &ActionResult{OK: true, Output: out}, nil
	}
}

func nodeRemoveDeleteState(cfg NodeRemoveControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		if cfg.DeleteState == nil {
			return nil, fmt.Errorf("node_remove.delete_state: handler not wired")
		}
		nodeID, _ := req.With["node_id"].(string)
		if nodeID == "" {
			return nil, fmt.Errorf("node_remove.delete_state: node_id is required")
		}
		if err := cfg.DeleteState(ctx, nodeID); err != nil {
			return nil, fmt.Errorf("delete state: %w", err)
		}
		log.Printf("actor[controller]: node_remove.delete_state node=%s", nodeID)
		return &ActionResult{OK: true}, nil
	}
}

func nodeRemovePublishScyllaHosts(cfg NodeRemoveControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		if cfg.PublishScyllaHosts == nil {
			return nil, fmt.Errorf("node_remove.publish_scylla_hosts: handler not wired")
		}
		if err := cfg.PublishScyllaHosts(ctx); err != nil {
			return nil, fmt.Errorf("publish scylla hosts: %w", err)
		}
		log.Printf("actor[controller]: node_remove.publish_scylla_hosts done")
		return &ActionResult{OK: true}, nil
	}
}

func nodeRemoveCleanupEtcdPrefixes(cfg NodeRemoveControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		if cfg.CleanupEtcdPrefixes == nil {
			return nil, fmt.Errorf("node_remove.cleanup_etcd_prefixes: handler not wired")
		}
		nodeID, _ := req.With["node_id"].(string)
		if nodeID == "" {
			return nil, fmt.Errorf("node_remove.cleanup_etcd_prefixes: node_id is required")
		}
		deleted, err := cfg.CleanupEtcdPrefixes(ctx, nodeID)
		if err != nil {
			return nil, fmt.Errorf("cleanup etcd prefixes: %w", err)
		}
		log.Printf("actor[controller]: node_remove.cleanup_etcd_prefixes node=%s deleted=%d",
			nodeID, deleted)
		return &ActionResult{
			OK: true,
			Output: map[string]any{
				"keys_deleted": deleted,
			},
		}, nil
	}
}

func nodeRemoveCleanupReleaseStatus(cfg NodeRemoveControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		if cfg.CleanupReleaseStatus == nil {
			return nil, fmt.Errorf("node_remove.cleanup_release_status: handler not wired")
		}
		nodeID, _ := req.With["node_id"].(string)
		if nodeID == "" {
			return nil, fmt.Errorf("node_remove.cleanup_release_status: node_id is required")
		}
		cleaned, err := cfg.CleanupReleaseStatus(ctx, nodeID)
		if err != nil {
			return nil, fmt.Errorf("cleanup release status: %w", err)
		}
		log.Printf("actor[controller]: node_remove.cleanup_release_status node=%s releases=%d",
			nodeID, cleaned)
		return &ActionResult{
			OK: true,
			Output: map[string]any{
				"releases_cleaned": cleaned,
			},
		}, nil
	}
}

func nodeRemoveScyllaRing(cfg NodeRemoveControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		if cfg.RemoveFromScyllaRing == nil {
			return nil, fmt.Errorf("node_remove.remove_scylla_ring: handler not wired")
		}
		nodeID, _ := req.With["node_id"].(string)
		scyllaHostID, _ := req.With["scylla_host_id"].(string)
		var nodeIPs []string
		if raw, ok := req.With["node_ips"].([]any); ok {
			for _, v := range raw {
				if s, ok := v.(string); ok {
					nodeIPs = append(nodeIPs, s)
				}
			}
		}
		ringErr := cfg.RemoveFromScyllaRing(ctx, nodeID, scyllaHostID, nodeIPs)
		// Non-fatal: a ring-removal failure does not fail the workflow.
		// The on_error: continue declaration in YAML and the step's
		// receipt make the warning visible.
		if ringErr != nil {
			log.Printf("actor[controller]: node_remove.remove_scylla_ring node=%s WARN: %v",
				nodeID, ringErr)
			return &ActionResult{
				OK: true,
				Output: map[string]any{
					"warning": ringErr.Error(),
				},
			}, nil
		}
		log.Printf("actor[controller]: node_remove.remove_scylla_ring node=%s OK", nodeID)
		return &ActionResult{OK: true}, nil
	}
}
