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
	// rendered_generation >= targetGeneration AND the expected state fingerprint
	// to etcd. Returns a non-nil error (suitable for retry) when any node has
	// not yet rendered or rendered a different topology.
	CheckAllNodesRendered func(ctx context.Context, targetGeneration int64, expectedFingerprint string, poolNodeIDs []string) error

	// AcquireTopologyLock acquires the distributed objectstore topology lock.
	// Uses a lease-backed key so the lock auto-expires if the controller crashes.
	// Also recovers stale locks older than lockStaleTTL.
	AcquireTopologyLock func(ctx context.Context) error

	// ReleaseTopologyLock releases the distributed objectstore topology lock.
	ReleaseTopologyLock func(ctx context.Context) error

	// MarkRestartInProgress sets the restart_in_progress flag in etcd.
	MarkRestartInProgress func(ctx context.Context) error

	// ClearRestartInProgress clears the restart_in_progress flag in etcd.
	ClearRestartInProgress func(ctx context.Context) error

	// RecordAppliedGeneration writes the successfully applied topology
	// generation to etcd and records a JSON summary with status=succeeded.
	RecordAppliedGeneration func(ctx context.Context, generation int64) error

	// VerifyMinioClusterHealthy probes all pool nodes to confirm that:
	//   - globular-minio.service is active on every pool node
	//   - the MinIO health endpoint responds on the cluster endpoint
	//   - desired generation still matches targetGeneration (no concurrent topology change)
	//   - desired volumes_hash still matches expectedVolumesHash
	//
	// Returns a retriable error when any check fails (workflow retries).
	VerifyMinioClusterHealthy func(ctx context.Context, targetGeneration int64, expectedVolumesHash string, poolNodeIDs []string) error

	// VerifyRuntimeScope checks that the cluster is safe to proceed with a
	// coordinated MinIO restart. It must run after the topology lock is held
	// but before any stop/start actions, so that:
	//   - no active globular-minio.service is found on nodes outside poolNodeIDs;
	//   - all desired pool nodes have a reachable agent endpoint.
	//
	// Returns a non-nil error (blocking, not retriable) when:
	//   - MinIO is active on a node not in poolNodeIDs (split-brain risk); or
	//   - a desired pool node has no agent endpoint (topology apply cannot proceed).
	// The error message includes violating node IDs and IPs.
	VerifyRuntimeScope func(ctx context.Context, poolNodeIDs []string) error

	// FailureCleanup is called by onFailure to atomically clean up all
	// in-progress state: release the topology lock, clear restart_in_progress,
	// and write a last_restart_result record with status=failed.
	FailureCleanup func(ctx context.Context, generation int64, reason string) error
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
	router.Register(v1alpha1.ActorClusterController, "controller.objectstore.verify_minio_cluster_healthy", objectstoreVerifyMinioClusterHealthy(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.objectstore.verify_runtime_scope", objectstoreVerifyRuntimeScope(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.objectstore.failure_cleanup", objectstoreFailureCleanup(cfg))
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

		expectedFingerprint := extractString(req.With, "expected_state_fingerprint")
		if expectedFingerprint == "" {
			expectedFingerprint = extractString(req.Inputs, "expected_state_fingerprint")
		}

		poolNodeIDs := extractStringSlice(req.With, "pool_node_ids")
		if len(poolNodeIDs) == 0 {
			poolNodeIDs = extractStringSlice(req.Inputs, "pool_node_ids")
		}
		if len(poolNodeIDs) == 0 {
			return nil, fmt.Errorf("check_all_nodes_rendered: pool_node_ids is required")
		}

		if cfg.CheckAllNodesRendered != nil {
			if err := cfg.CheckAllNodesRendered(ctx, gen, expectedFingerprint, poolNodeIDs); err != nil {
				return nil, fmt.Errorf("nodes have not rendered generation %d: %w", gen, err)
			}
		}

		log.Printf("actor[controller/objectstore]: all pool nodes rendered generation %d", gen)
		return &ActionResult{OK: true, Output: map[string]any{"generation": gen, "nodes_ready": len(poolNodeIDs)}}, nil
	}
}

func objectstoreVerifyMinioClusterHealthy(cfg ObjectStoreControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		gen := extractInt64(req.With, "target_generation")
		if gen == 0 {
			gen = extractInt64(req.Inputs, "target_generation")
		}
		if gen == 0 {
			return nil, fmt.Errorf("verify_minio_cluster_healthy: target_generation is required")
		}

		expectedHash := extractString(req.With, "expected_volumes_hash")
		if expectedHash == "" {
			expectedHash = extractString(req.Inputs, "expected_volumes_hash")
		}

		poolNodeIDs := extractStringSlice(req.With, "pool_node_ids")
		if len(poolNodeIDs) == 0 {
			poolNodeIDs = extractStringSlice(req.Inputs, "pool_node_ids")
		}
		if len(poolNodeIDs) == 0 {
			return nil, fmt.Errorf("verify_minio_cluster_healthy: pool_node_ids is required")
		}

		if cfg.VerifyMinioClusterHealthy != nil {
			if err := cfg.VerifyMinioClusterHealthy(ctx, gen, expectedHash, poolNodeIDs); err != nil {
				return nil, fmt.Errorf("minio cluster not healthy after restart: %w", err)
			}
		}

		log.Printf("actor[controller/objectstore]: minio cluster healthy, generation=%d", gen)
		return &ActionResult{OK: true, Output: map[string]any{"generation": gen, "healthy": true}}, nil
	}
}

func objectstoreVerifyRuntimeScope(cfg ObjectStoreControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		poolNodeIDs := extractStringSlice(req.With, "pool_node_ids")
		if len(poolNodeIDs) == 0 {
			poolNodeIDs = extractStringSlice(req.Inputs, "pool_node_ids")
		}
		if len(poolNodeIDs) == 0 {
			return nil, fmt.Errorf("verify_runtime_scope: pool_node_ids is required")
		}

		if cfg.VerifyRuntimeScope != nil {
			if err := cfg.VerifyRuntimeScope(ctx, poolNodeIDs); err != nil {
				return nil, fmt.Errorf("runtime scope violation: %w", err)
			}
		}

		log.Printf("actor[controller/objectstore]: runtime scope verified (%d pool nodes)", len(poolNodeIDs))
		return &ActionResult{OK: true, Output: map[string]any{"pool_nodes_verified": len(poolNodeIDs)}}, nil
	}
}

func objectstoreFailureCleanup(cfg ObjectStoreControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		gen := extractInt64(req.With, "target_generation")
		if gen == 0 {
			gen = extractInt64(req.Inputs, "target_generation")
		}
		reason := extractString(req.With, "reason")
		if reason == "" {
			reason = extractString(req.Inputs, "reason")
		}
		if reason == "" {
			reason = "workflow_failed"
		}

		if cfg.FailureCleanup != nil {
			if err := cfg.FailureCleanup(ctx, gen, reason); err != nil {
				return nil, fmt.Errorf("failure cleanup: %w", err)
			}
		}

		log.Printf("actor[controller/objectstore]: failure cleanup done (gen=%d reason=%s)", gen, reason)
		return &ActionResult{OK: true, Message: "failure cleanup complete"}, nil
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

func extractString(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
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
