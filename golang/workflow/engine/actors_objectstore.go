package engine

import (
	"context"
	"fmt"
	"log"

	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// ObjectStoreControllerConfig provides dependencies for objectstore topology
// workflow steps that run on the cluster controller.
type ObjectStoreControllerConfig struct {
	// CheckAllNodesRendered returns nil when all pool nodes have written
	// rendered_generation >= targetGeneration to etcd. Returns a non-nil
	// error (suitable for retry) when any node has not yet rendered.
	CheckAllNodesRendered func(ctx context.Context, targetGeneration int64, poolNodeIDs []string) error

	// AcquireTopologyLock acquires the distributed objectstore topology lock.
	// Returns an error if the lock cannot be acquired (already held).
	AcquireTopologyLock func(ctx context.Context) error

	// ReleaseTopologyLock releases the distributed objectstore topology lock.
	ReleaseTopologyLock func(ctx context.Context) error

	// MarkRestartInProgress sets the restart_in_progress flag in etcd.
	MarkRestartInProgress func(ctx context.Context) error

	// ClearRestartInProgress clears the restart_in_progress flag in etcd.
	ClearRestartInProgress func(ctx context.Context) error

	// RecordAppliedGeneration writes the successfully applied topology
	// generation to etcd and records a JSON summary.
	RecordAppliedGeneration func(ctx context.Context, generation int64) error
}

// RegisterObjectStoreControllerActions registers all cluster-controller actor
// handlers for the objectstore topology workflow.
func RegisterObjectStoreControllerActions(router *Router, cfg ObjectStoreControllerConfig) {
	router.Register(v1alpha1.ActorClusterController, "controller.objectstore.check_all_nodes_rendered", objectstoreCheckAllNodesRendered(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.objectstore.acquire_topology_lock", objectstoreAcquireTopologyLock(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.objectstore.release_topology_lock", objectstoreReleaseTopologyLock(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.objectstore.mark_restart_in_progress", objectstoreMarkRestartInProgress(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.objectstore.clear_restart_in_progress", objectstoreClearRestartInProgress(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.objectstore.record_applied_generation", objectstoreRecordAppliedGeneration(cfg))
}

// ── action implementations ────────────────────────────────────────────────────

func objectstoreCheckAllNodesRendered(cfg ObjectStoreControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		gen := extractInt64(req.With, "target_generation")
		if gen == 0 {
			gen = extractInt64(req.Inputs, "target_generation")
		}
		if gen == 0 {
			return nil, fmt.Errorf("check_all_nodes_rendered: target_generation is required")
		}

		poolNodeIDs := extractStringSlice(req.With, "pool_node_ids")
		if len(poolNodeIDs) == 0 {
			poolNodeIDs = extractStringSlice(req.Inputs, "pool_node_ids")
		}
		if len(poolNodeIDs) == 0 {
			return nil, fmt.Errorf("check_all_nodes_rendered: pool_node_ids is required")
		}

		if cfg.CheckAllNodesRendered != nil {
			if err := cfg.CheckAllNodesRendered(ctx, gen, poolNodeIDs); err != nil {
				return nil, fmt.Errorf("nodes have not rendered generation %d: %w", gen, err)
			}
		}

		log.Printf("actor[controller/objectstore]: all pool nodes rendered generation %d", gen)
		return &ActionResult{OK: true, Output: map[string]any{"generation": gen, "nodes_ready": len(poolNodeIDs)}}, nil
	}
}

func objectstoreAcquireTopologyLock(cfg ObjectStoreControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		if cfg.AcquireTopologyLock != nil {
			if err := cfg.AcquireTopologyLock(ctx); err != nil {
				return nil, fmt.Errorf("acquire objectstore topology lock: %w", err)
			}
		}
		log.Printf("actor[controller/objectstore]: topology lock acquired")
		return &ActionResult{OK: true, Message: "topology lock acquired"}, nil
	}
}

func objectstoreReleaseTopologyLock(cfg ObjectStoreControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		if cfg.ReleaseTopologyLock != nil {
			if err := cfg.ReleaseTopologyLock(ctx); err != nil {
				return nil, fmt.Errorf("release objectstore topology lock: %w", err)
			}
		}
		log.Printf("actor[controller/objectstore]: topology lock released")
		return &ActionResult{OK: true, Message: "topology lock released"}, nil
	}
}

func objectstoreMarkRestartInProgress(cfg ObjectStoreControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		if cfg.MarkRestartInProgress != nil {
			if err := cfg.MarkRestartInProgress(ctx); err != nil {
				return nil, fmt.Errorf("mark restart in progress: %w", err)
			}
		}
		return &ActionResult{OK: true, Message: "restart in progress"}, nil
	}
}

func objectstoreClearRestartInProgress(cfg ObjectStoreControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		if cfg.ClearRestartInProgress != nil {
			if err := cfg.ClearRestartInProgress(ctx); err != nil {
				return nil, fmt.Errorf("clear restart in progress: %w", err)
			}
		}
		return &ActionResult{OK: true, Message: "restart complete"}, nil
	}
}

func objectstoreRecordAppliedGeneration(cfg ObjectStoreControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		gen := extractInt64(req.With, "target_generation")
		if gen == 0 {
			gen = extractInt64(req.Inputs, "target_generation")
		}
		if gen == 0 {
			return nil, fmt.Errorf("record_applied_generation: target_generation is required")
		}

		if cfg.RecordAppliedGeneration != nil {
			if err := cfg.RecordAppliedGeneration(ctx, gen); err != nil {
				return nil, fmt.Errorf("record applied generation %d: %w", gen, err)
			}
		}

		log.Printf("actor[controller/objectstore]: recorded applied_generation=%d", gen)
		return &ActionResult{OK: true, Output: map[string]any{"applied_generation": gen}}, nil
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func extractInt64(m map[string]any, key string) int64 {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch x := v.(type) {
	case int64:
		return x
	case float64:
		return int64(x)
	case int:
		return int64(x)
	}
	return 0
}

func extractStringSlice(m map[string]any, key string) []string {
	v, ok := m[key]
	if !ok {
		return nil
	}
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}
