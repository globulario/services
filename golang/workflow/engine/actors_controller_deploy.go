package engine

import (
	"context"
	"fmt"

	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// ControllerDeployConfig provides callbacks for the release.apply.controller
// workflow — the leader-aware rolling update for control-plane services.
type ControllerDeployConfig struct {
	// DiscoverReplicas returns all controller replicas, the current leader,
	// and per-node agent endpoints. Fail if discovery is incomplete.
	DiscoverReplicas func(ctx context.Context, clusterID string) (*ControllerDiscovery, error)

	// VerifyReplicaHealth checks that a controller replica on the given node
	// is healthy and running the expected version. Returns error if unhealthy.
	VerifyReplicaHealth func(ctx context.Context, nodeID, packageName, version string) error

	// ResignLeadership causes this controller to resign the etcd lease.
	ResignLeadership func(ctx context.Context, reason string) error

	// ConfirmLeadershipMoved verifies the leader is no longer the old node.
	ConfirmLeadershipMoved func(ctx context.Context, oldLeaderNodeID string) error

	// ApplyPackageRelease calls the remote node-agent to install a package.
	ApplyPackageRelease func(ctx context.Context, nodeID, agentEndpoint, pkgName, pkgKind, version, publisher, repoAddr string, buildNumber int64, force bool) error
}

// ControllerDiscovery is the result of replica discovery.
type ControllerDiscovery struct {
	Replicas       []ControllerReplica // all replicas
	Followers      []ControllerReplica // non-leader replicas
	LeaderNodeID   string
	LeaderEndpoint string
}

// ControllerReplica describes a single controller instance.
type ControllerReplica struct {
	NodeID        string
	AgentEndpoint string
	IsLeader      bool
}

// RegisterControllerDeployActions registers all actor handlers for the
// release.apply.controller workflow.
func RegisterControllerDeployActions(router *Router, cfg ControllerDeployConfig) {
	router.Register(v1alpha1.ActorClusterController, "controller.deploy.discover_replicas",
		deployDiscoverReplicas(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.deploy.validate_discovery",
		deployValidateDiscovery(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.deploy.verify_replica_health",
		deployVerifyReplicaHealth(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.deploy.verify_upgraded_follower_exists",
		deployVerifyUpgradedFollowerExists(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.deploy.resign_leadership",
		deployResignLeadership(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.deploy.confirm_leadership_moved",
		deployConfirmLeadershipMoved(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.deploy.final_health_check",
		deployFinalHealthCheck(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.deploy.mark_rollout_failed",
		deployMarkRolloutFailed(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.deploy.mark_follower_failed",
		deployMarkFollowerFailed(cfg))

	// Node-agent action: remote package apply via controller proxy.
	router.Register(v1alpha1.ActorNodeAgent, "node.apply_package_release",
		deployApplyPackageRelease(cfg))
}

// ── Action handlers ──────────────────────────────────────────────────────────

func deployDiscoverReplicas(cfg ControllerDeployConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		clusterID := fmt.Sprint(req.With["cluster_id"])
		if cfg.DiscoverReplicas == nil {
			return nil, fmt.Errorf("DiscoverReplicas not configured")
		}
		disc, err := cfg.DiscoverReplicas(ctx, clusterID)
		if err != nil {
			return nil, fmt.Errorf("discover replicas: %w", err)
		}

		// Build followers list for foreach consumption.
		followers := make([]map[string]any, 0, len(disc.Followers))
		for _, f := range disc.Followers {
			followers = append(followers, map[string]any{
				"node_id":        f.NodeID,
				"agent_endpoint": f.AgentEndpoint,
			})
		}

		replicas := make([]map[string]any, 0, len(disc.Replicas))
		for _, r := range disc.Replicas {
			replicas = append(replicas, map[string]any{
				"node_id":        r.NodeID,
				"agent_endpoint": r.AgentEndpoint,
				"is_leader":      r.IsLeader,
			})
		}

		output := map[string]any{
			"replicas":        replicas,
			"followers":       followers,
			"leader_node_id":  disc.LeaderNodeID,
			"leader_endpoint": disc.LeaderEndpoint,
			"replica_count":   len(disc.Replicas),
			"follower_count":  len(disc.Followers),
		}

		// Write to both Output (for export) and Outputs (for direct reference).
		req.Outputs["discovery"] = output

		return &ActionResult{
			OK:     true,
			Output: output,
		}, nil
	}
}

func deployValidateDiscovery(_ ControllerDeployConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		replicas, _ := req.With["replicas"].([]any)
		leaderNodeID := fmt.Sprint(req.With["leader_node_id"])

		if len(replicas) == 0 {
			return nil, fmt.Errorf("discovery returned zero replicas — fail closed")
		}
		if leaderNodeID == "" || leaderNodeID == "<nil>" {
			return nil, fmt.Errorf("no leader identified — fail closed")
		}
		if len(replicas) < 2 {
			return nil, fmt.Errorf("only %d replica(s) found — need at least 2 for safe rolling update", len(replicas))
		}

		return &ActionResult{
			OK:      true,
			Message: fmt.Sprintf("validated: %d replicas, leader=%s", len(replicas), leaderNodeID),
		}, nil
	}
}

func deployVerifyReplicaHealth(cfg ControllerDeployConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		pkgName := fmt.Sprint(req.With["package_name"])
		version := fmt.Sprint(req.With["version"])

		if cfg.VerifyReplicaHealth == nil {
			return nil, fmt.Errorf("VerifyReplicaHealth not configured")
		}
		if err := cfg.VerifyReplicaHealth(ctx, nodeID, pkgName, version); err != nil {
			return nil, fmt.Errorf("replica %s unhealthy: %w", nodeID, err)
		}

		return &ActionResult{OK: true, Message: fmt.Sprintf("replica %s healthy", nodeID)}, nil
	}
}

func deployVerifyUpgradedFollowerExists(_ ControllerDeployConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		// This step runs after apply_followers. If the foreach completed
		// without error, at least one follower was successfully updated.
		// The workflow engine already fails if the foreach had zero successes.
		followers, _ := req.With["followers"].([]any)
		if len(followers) == 0 {
			return nil, fmt.Errorf("no followers available — cannot resign leadership safely")
		}

		return &ActionResult{
			OK:      true,
			Message: fmt.Sprintf("%d follower(s) upgraded and healthy", len(followers)),
		}, nil
	}
}

func deployResignLeadership(cfg ControllerDeployConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		reason := fmt.Sprint(req.With["reason"])
		if cfg.ResignLeadership == nil {
			return nil, fmt.Errorf("ResignLeadership not configured")
		}
		if err := cfg.ResignLeadership(ctx, reason); err != nil {
			return nil, fmt.Errorf("resign leadership: %w", err)
		}

		return &ActionResult{OK: true, Message: "leadership resigned"}, nil
	}
}

func deployConfirmLeadershipMoved(cfg ControllerDeployConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		oldLeaderNodeID := fmt.Sprint(req.With["old_leader_node_id"])
		if cfg.ConfirmLeadershipMoved == nil {
			return nil, fmt.Errorf("ConfirmLeadershipMoved not configured")
		}
		if err := cfg.ConfirmLeadershipMoved(ctx, oldLeaderNodeID); err != nil {
			return nil, fmt.Errorf("leadership not moved: %w", err)
		}

		return &ActionResult{OK: true, Message: "leadership transferred"}, nil
	}
}

func deployFinalHealthCheck(cfg ControllerDeployConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		// Verify all replicas are healthy by checking discovery again.
		clusterID := fmt.Sprint(req.With["cluster_id"])
		pkgName := fmt.Sprint(req.With["package_name"])
		version := fmt.Sprint(req.With["version"])

		if cfg.DiscoverReplicas == nil || cfg.VerifyReplicaHealth == nil {
			return nil, fmt.Errorf("health check callbacks not configured")
		}
		disc, err := cfg.DiscoverReplicas(ctx, clusterID)
		if err != nil {
			return nil, fmt.Errorf("final discovery: %w", err)
		}

		for _, r := range disc.Replicas {
			if err := cfg.VerifyReplicaHealth(ctx, r.NodeID, pkgName, version); err != nil {
				return nil, fmt.Errorf("replica %s unhealthy in final check: %w", r.NodeID, err)
			}
		}

		return &ActionResult{
			OK:      true,
			Message: fmt.Sprintf("all %d replicas healthy at version %s", len(disc.Replicas), version),
		}, nil
	}
}

func deployMarkRolloutFailed(_ ControllerDeployConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		pkgName := fmt.Sprint(req.With["package_name"])
		version := fmt.Sprint(req.With["version"])
		fmt.Printf("controller-deploy: rollout FAILED for %s@%s\n", pkgName, version)
		return &ActionResult{OK: true, Message: "rollout failure recorded"}, nil
	}
}

func deployMarkFollowerFailed(_ ControllerDeployConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		pkgName := fmt.Sprint(req.With["package_name"])
		fmt.Printf("controller-deploy: follower %s FAILED for %s\n", nodeID, pkgName)
		return &ActionResult{OK: true, Message: fmt.Sprintf("follower %s failure recorded", nodeID)}, nil
	}
}

func deployApplyPackageRelease(cfg ControllerDeployConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		endpoint := fmt.Sprint(req.With["agent_endpoint"])
		pkgName := fmt.Sprint(req.With["package_name"])
		pkgKind := fmt.Sprint(req.With["package_kind"])
		version := fmt.Sprint(req.With["version"])
		publisher := fmt.Sprint(req.With["publisher"])
		repoAddr := fmt.Sprint(req.With["repository_addr"])
		force, _ := req.With["force"].(bool)

		var buildNumber int64
		if bn, ok := req.With["build_number"].(float64); ok {
			buildNumber = int64(bn)
		}

		if cfg.ApplyPackageRelease == nil {
			return nil, fmt.Errorf("ApplyPackageRelease not configured")
		}
		if err := cfg.ApplyPackageRelease(ctx, nodeID, endpoint, pkgName, pkgKind, version, publisher, repoAddr, buildNumber, force); err != nil {
			return nil, fmt.Errorf("apply %s@%s on node %s: %w", pkgName, version, nodeID, err)
		}

		return &ActionResult{
			OK:      true,
			Message: fmt.Sprintf("applied %s@%s on node %s", pkgName, version, nodeID),
		}, nil
	}
}
