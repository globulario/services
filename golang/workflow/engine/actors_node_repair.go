package engine

import (
	"context"
	"fmt"
	"log"

	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// --------------------------------------------------------------------------
// Node repair controller actions (node.repair workflow)
// --------------------------------------------------------------------------

// NodeRepairControllerConfig provides dependencies for node repair orchestration.
type NodeRepairControllerConfig struct {
	MarkStarted       func(ctx context.Context, nodeID, reason string) error
	ValidateReference func(ctx context.Context, referenceNodeID string) error
	Classify          func(ctx context.Context, nodeID string, diagnosis map[string]any) (map[string]any, error)
	IsolateNode       func(ctx context.Context, nodeID string, repairPlan map[string]any) error
	RejoinNode        func(ctx context.Context, nodeID string) error
	MarkRecovered     func(ctx context.Context, nodeID string) error
	MarkFailed        func(ctx context.Context, nodeID string) error
	EmitRecovered     func(ctx context.Context, nodeID string) error
}

// RegisterNodeRepairControllerActions registers node repair controller handlers.
func RegisterNodeRepairControllerActions(router *Router, cfg NodeRepairControllerConfig) {
	router.Register(v1alpha1.ActorClusterController, "controller.node_repair.mark_started", nodeRepairMarkStarted(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.node_repair.validate_reference", nodeRepairValidateReference(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.node_repair.classify", nodeRepairClassify(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.node_repair.isolate_node", nodeRepairIsolateNode(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.node_repair.rejoin_node", nodeRepairRejoinNode(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.node_repair.mark_recovered", nodeRepairMarkRecovered(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.node_repair.mark_failed", nodeRepairMarkFailed(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.node_repair.emit_recovered", nodeRepairEmitRecovered(cfg))
}

func nodeRepairMarkStarted(cfg NodeRepairControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		reason := fmt.Sprint(req.With["reason"])
		log.Printf("actor[controller]: node_repair started %s: %s", nodeID, reason)
		if cfg.MarkStarted != nil {
			if err := cfg.MarkStarted(ctx, nodeID, reason); err != nil {
				return nil, fmt.Errorf("mark repair started: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func nodeRepairValidateReference(cfg NodeRepairControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		refID := fmt.Sprint(req.With["reference_node_id"])
		log.Printf("actor[controller]: validating reference node %s", refID)
		if cfg.ValidateReference != nil {
			if err := cfg.ValidateReference(ctx, refID); err != nil {
				return nil, fmt.Errorf("validate reference node: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func nodeRepairClassify(cfg NodeRepairControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		diagnosis, _ := req.With["diagnosis"].(map[string]any)
		if diagnosis == nil {
			if d, ok := req.Outputs["diagnosis"].(map[string]any); ok {
				diagnosis = d
			}
		}
		if cfg.Classify != nil {
			plan, err := cfg.Classify(ctx, nodeID, diagnosis)
			if err != nil {
				return nil, fmt.Errorf("classify failure: %w", err)
			}
			req.Outputs["repair_plan"] = plan
			return &ActionResult{OK: true, Output: plan}, nil
		}
		plan := map[string]any{"action": "reinstall", "node_id": nodeID}
		req.Outputs["repair_plan"] = plan
		return &ActionResult{OK: true, Output: plan}, nil
	}
}

func nodeRepairIsolateNode(cfg NodeRepairControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		repairPlan, _ := req.With["repair_plan"].(map[string]any)
		if repairPlan == nil {
			if rp, ok := req.Outputs["repair_plan"].(map[string]any); ok {
				repairPlan = rp
			}
		}
		log.Printf("actor[controller]: isolating node %s for repair", nodeID)
		if cfg.IsolateNode != nil {
			if err := cfg.IsolateNode(ctx, nodeID, repairPlan); err != nil {
				return nil, fmt.Errorf("isolate node: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func nodeRepairRejoinNode(cfg NodeRepairControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		log.Printf("actor[controller]: rejoining node %s after repair", nodeID)
		if cfg.RejoinNode != nil {
			if err := cfg.RejoinNode(ctx, nodeID); err != nil {
				return nil, fmt.Errorf("rejoin node: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func nodeRepairMarkRecovered(cfg NodeRepairControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		log.Printf("actor[controller]: node %s RECOVERED", nodeID)
		if cfg.MarkRecovered != nil {
			if err := cfg.MarkRecovered(ctx, nodeID); err != nil {
				return nil, fmt.Errorf("mark recovered: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func nodeRepairMarkFailed(cfg NodeRepairControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		log.Printf("actor[controller]: node %s repair FAILED", nodeID)
		if cfg.MarkFailed != nil {
			if err := cfg.MarkFailed(ctx, nodeID); err != nil {
				return nil, fmt.Errorf("mark repair failed: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func nodeRepairEmitRecovered(cfg NodeRepairControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		if cfg.EmitRecovered != nil {
			if err := cfg.EmitRecovered(ctx, nodeID); err != nil {
				return nil, fmt.Errorf("emit recovered: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

// --------------------------------------------------------------------------
// Node repair agent actions (node.repair workflow)
// --------------------------------------------------------------------------

// NodeRepairAgentConfig provides dependencies for node-agent repair operations.
type NodeRepairAgentConfig struct {
	CollectRepairFacts      func(ctx context.Context, nodeID string, targetPackages []any) (map[string]any, error)
	RepairPackages          func(ctx context.Context, nodeID string, repairPlan map[string]any) (map[string]any, error)
	RestartRepairedServices func(ctx context.Context, nodeID string, repairResult map[string]any) error
	VerifyRepairRuntime     func(ctx context.Context, nodeID string, repairPlan map[string]any) error
	SyncInstalledState      func(ctx context.Context, nodeID string) error
	RotateServiceCerts      func(ctx context.Context, nodeID string) error
}

// RegisterNodeRepairAgentActions registers node-agent repair action handlers.
func RegisterNodeRepairAgentActions(router *Router, cfg NodeRepairAgentConfig) {
	router.Register(v1alpha1.ActorNodeAgent, "node.collect_repair_facts", nodeCollectRepairFacts(cfg))
	router.Register(v1alpha1.ActorNodeAgent, "node.repair_packages", nodeRepairPackages(cfg))
	router.Register(v1alpha1.ActorNodeAgent, "node.restart_repaired_services", nodeRestartRepairedServices(cfg))
	router.Register(v1alpha1.ActorNodeAgent, "node.verify_repair_runtime", nodeVerifyRepairRuntime(cfg))
	router.Register(v1alpha1.ActorNodeAgent, "node.sync_installed_state", nodeRepairSyncInstalledState(cfg))
	router.Register(v1alpha1.ActorNodeAgent, "node.rotate_service_certs", nodeRotateServiceCerts(cfg))
	// node.verify_installed_state_synced is registered in actors_verification.go
}

func nodeCollectRepairFacts(cfg NodeRepairAgentConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		targetPkgs, _ := req.With["target_packages"].([]any)
		if cfg.CollectRepairFacts != nil {
			facts, err := cfg.CollectRepairFacts(ctx, nodeID, targetPkgs)
			if err != nil {
				return nil, fmt.Errorf("collect repair facts: %w", err)
			}
			req.Outputs["diagnosis"] = facts
			return &ActionResult{OK: true, Output: facts}, nil
		}
		facts := map[string]any{"node_id": nodeID, "status": "degraded"}
		req.Outputs["diagnosis"] = facts
		return &ActionResult{OK: true, Output: facts}, nil
	}
}

func nodeRepairPackages(cfg NodeRepairAgentConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		repairPlan, _ := req.With["repair_plan"].(map[string]any)
		if repairPlan == nil {
			if rp, ok := req.Outputs["repair_plan"].(map[string]any); ok {
				repairPlan = rp
			}
		}
		if cfg.RepairPackages != nil {
			result, err := cfg.RepairPackages(ctx, nodeID, repairPlan)
			if err != nil {
				return nil, fmt.Errorf("repair packages: %w", err)
			}
			req.Outputs["repair_result"] = result
			return &ActionResult{OK: true, Output: result}, nil
		}
		result := map[string]any{"repaired": true, "node_id": nodeID}
		req.Outputs["repair_result"] = result
		return &ActionResult{OK: true, Output: result}, nil
	}
}

func nodeRestartRepairedServices(cfg NodeRepairAgentConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		repairResult, _ := req.With["repair_result"].(map[string]any)
		if repairResult == nil {
			if rr, ok := req.Outputs["repair_result"].(map[string]any); ok {
				repairResult = rr
			}
		}
		if cfg.RestartRepairedServices != nil {
			if err := cfg.RestartRepairedServices(ctx, nodeID, repairResult); err != nil {
				return nil, fmt.Errorf("restart repaired services: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func nodeVerifyRepairRuntime(cfg NodeRepairAgentConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		repairPlan, _ := req.With["repair_plan"].(map[string]any)
		if repairPlan == nil {
			if rp, ok := req.Outputs["repair_plan"].(map[string]any); ok {
				repairPlan = rp
			}
		}
		if cfg.VerifyRepairRuntime != nil {
			if err := cfg.VerifyRepairRuntime(ctx, nodeID, repairPlan); err != nil {
				return nil, fmt.Errorf("verify repair runtime: %w", err)
			}
		}
		return &ActionResult{OK: true, Output: map[string]any{"healthy": true}}, nil
	}
}

func nodeRepairSyncInstalledState(cfg NodeRepairAgentConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		if cfg.SyncInstalledState != nil {
			if err := cfg.SyncInstalledState(ctx, nodeID); err != nil {
				return nil, fmt.Errorf("sync installed state: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func nodeRotateServiceCerts(cfg NodeRepairAgentConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		log.Printf("actor[node-agent]: rotating service certificates for %s", nodeID)
		if cfg.RotateServiceCerts != nil {
			if err := cfg.RotateServiceCerts(ctx, nodeID); err != nil {
				return nil, fmt.Errorf("rotate service certs: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}
