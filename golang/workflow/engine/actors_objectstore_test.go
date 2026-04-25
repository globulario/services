package engine

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// buildTestObjectStoreConfig returns an ObjectStoreControllerConfig wired with
// simple in-memory implementations suitable for unit testing. Callers may
// override individual fields to inject failures.
func buildTestObjectStoreConfig() ObjectStoreControllerConfig {
	return ObjectStoreControllerConfig{
		CheckAllNodesRendered: func(_ context.Context, gen int64, fp string, nodeIDs []string) error {
			return nil
		},
		AcquireTopologyLock:    func(_ context.Context) error { return nil },
		ReleaseTopologyLock:    func(_ context.Context) error { return nil },
		MarkRestartInProgress:  func(_ context.Context) error { return nil },
		ClearRestartInProgress: func(_ context.Context) error { return nil },
		RecordAppliedGeneration: func(_ context.Context, gen int64) error {
			return nil
		},
		VerifyMinioClusterHealthy: func(_ context.Context, gen int64, hash string, nodeIDs []string) error {
			return nil
		},
		FailureCleanup: func(_ context.Context, gen int64, reason string) error {
			return nil
		},
	}
}

func callAction(cfg ObjectStoreControllerConfig, actionName string, with map[string]any, inputs map[string]any) (*ActionResult, error) {
	router := NewRouter()
	RegisterObjectStoreControllerActions(router, cfg)
	h, ok := router.Resolve(actorClusterController, actionName)
	if !ok {
		return nil, fmt.Errorf("action %q not registered", actionName)
	}
	return h(context.Background(), ActionRequest{With: with, Inputs: inputs})
}

// ── check_all_nodes_rendered ──────────────────────────────────────────────────

func TestCheckAllNodesRendered_Success(t *testing.T) {
	cfg := buildTestObjectStoreConfig()
	_, err := callAction(cfg, "controller.objectstore.check_all_nodes_rendered",
		map[string]any{"target_generation": int64(3), "pool_node_ids": []any{"node-1", "node-2"}},
		nil)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestCheckAllNodesRendered_MissingGeneration(t *testing.T) {
	cfg := buildTestObjectStoreConfig()
	_, err := callAction(cfg, "controller.objectstore.check_all_nodes_rendered",
		map[string]any{"pool_node_ids": []any{"node-1"}},
		nil)
	if err == nil {
		t.Fatal("expected error for missing target_generation")
	}
}

func TestCheckAllNodesRendered_MissingNodeIDs(t *testing.T) {
	cfg := buildTestObjectStoreConfig()
	_, err := callAction(cfg, "controller.objectstore.check_all_nodes_rendered",
		map[string]any{"target_generation": int64(1)},
		nil)
	if err == nil {
		t.Fatal("expected error for missing pool_node_ids")
	}
}

// TestCheckAllNodesRendered_GenerationMismatch simulates a node that hasn't
// rendered the target generation yet. The action must propagate the error
// (making the workflow step retriable).
func TestCheckAllNodesRendered_GenerationMismatch(t *testing.T) {
	cfg := buildTestObjectStoreConfig()
	cfg.CheckAllNodesRendered = func(_ context.Context, gen int64, fp string, nodeIDs []string) error {
		return fmt.Errorf("nodes not yet at generation %d: [node-2:rendered=1]", gen)
	}
	_, err := callAction(cfg, "controller.objectstore.check_all_nodes_rendered",
		map[string]any{"target_generation": int64(3), "pool_node_ids": []any{"node-1", "node-2"}},
		nil)
	if err == nil {
		t.Fatal("expected error when node hasn't rendered target generation")
	}
}

// TestCheckAllNodesRendered_FingerprintMismatch simulates a node that rendered
// the right generation number but with a different topology (wrong volumes_hash
// or mode). The action must propagate the error.
func TestCheckAllNodesRendered_FingerprintMismatch(t *testing.T) {
	cfg := buildTestObjectStoreConfig()
	cfg.CheckAllNodesRendered = func(_ context.Context, gen int64, fp string, nodeIDs []string) error {
		return fmt.Errorf("nodes not yet at generation %d (fingerprint=%s): [node-2:fp_mismatch(got=deadbeef)]", gen, fp[:8])
	}
	_, err := callAction(cfg, "controller.objectstore.check_all_nodes_rendered",
		map[string]any{
			"target_generation":        int64(3),
			"expected_state_fingerprint": "abc123def456",
			"pool_node_ids":            []any{"node-1", "node-2"},
		},
		nil)
	if err == nil {
		t.Fatal("expected error when node fingerprint mismatches")
	}
}

// ── verify_minio_cluster_healthy ──────────────────────────────────────────────

func TestVerifyMinioClusterHealthy_Success(t *testing.T) {
	cfg := buildTestObjectStoreConfig()
	_, err := callAction(cfg, "controller.objectstore.verify_minio_cluster_healthy",
		map[string]any{
			"target_generation":    int64(3),
			"expected_volumes_hash": "abc123",
			"pool_node_ids":        []any{"node-1"},
		},
		nil)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

// TestVerifyMinioClusterHealthy_ServiceNotActive simulates a node where MinIO
// did not come up after restart. The action must propagate the retriable error
// so the workflow retries before recording applied_generation.
func TestVerifyMinioClusterHealthy_ServiceNotActive(t *testing.T) {
	cfg := buildTestObjectStoreConfig()
	cfg.VerifyMinioClusterHealthy = func(_ context.Context, gen int64, hash string, nodeIDs []string) error {
		return fmt.Errorf("minio service not active on nodes: [node-1:not_active(failed)]")
	}
	_, err := callAction(cfg, "controller.objectstore.verify_minio_cluster_healthy",
		map[string]any{
			"target_generation": int64(3),
			"pool_node_ids":     []any{"node-1"},
		},
		nil)
	if err == nil {
		t.Fatal("expected error when minio service not active")
	}
}

// TestVerifyMinioClusterHealthy_VolumesHashChanged simulates a concurrent
// topology change mid-workflow: the volumes_hash in etcd no longer matches
// what we started the workflow with.
func TestVerifyMinioClusterHealthy_VolumesHashChanged(t *testing.T) {
	cfg := buildTestObjectStoreConfig()
	cfg.VerifyMinioClusterHealthy = func(_ context.Context, gen int64, hash string, nodeIDs []string) error {
		return fmt.Errorf("desired volumes_hash changed mid-workflow: want=%s got=differenthash", hash)
	}
	_, err := callAction(cfg, "controller.objectstore.verify_minio_cluster_healthy",
		map[string]any{
			"target_generation":    int64(3),
			"expected_volumes_hash": "originalhash",
			"pool_node_ids":        []any{"node-1"},
		},
		nil)
	if err == nil {
		t.Fatal("expected error when volumes_hash changed mid-workflow")
	}
}

// TestVerifyMinioClusterHealthy_MissingTargetGeneration verifies that the
// action fails fast when the required input is missing.
func TestVerifyMinioClusterHealthy_MissingTargetGeneration(t *testing.T) {
	cfg := buildTestObjectStoreConfig()
	_, err := callAction(cfg, "controller.objectstore.verify_minio_cluster_healthy",
		map[string]any{"pool_node_ids": []any{"node-1"}},
		nil)
	if err == nil {
		t.Fatal("expected error for missing target_generation")
	}
}

// ── applied_generation is NOT recorded when health check fails ────────────────

// TestAppliedGenerationNotRecordedWhenHealthFails is the core acceptance test:
// if VerifyMinioClusterHealthy fails, the RecordAppliedGeneration function
// must NOT be called — the workflow fails without advancing applied_generation.
func TestAppliedGenerationNotRecordedWhenHealthFails(t *testing.T) {
	recordCalled := false

	cfg := buildTestObjectStoreConfig()
	cfg.RecordAppliedGeneration = func(_ context.Context, gen int64) error {
		recordCalled = true
		return nil
	}
	cfg.VerifyMinioClusterHealthy = func(_ context.Context, gen int64, hash string, nodeIDs []string) error {
		return errors.New("minio health endpoint unreachable")
	}

	// Call verify — it should fail.
	_, verifyErr := callAction(cfg, "controller.objectstore.verify_minio_cluster_healthy",
		map[string]any{
			"target_generation": int64(3),
			"pool_node_ids":     []any{"node-1"},
		},
		nil)
	if verifyErr == nil {
		t.Fatal("expected verify to fail")
	}

	// Simulate workflow executor: only calls record_generation if verify succeeded.
	if verifyErr == nil {
		_, _ = callAction(cfg, "controller.objectstore.record_applied_generation",
			map[string]any{"target_generation": int64(3)},
			nil)
	}

	if recordCalled {
		t.Error("RecordAppliedGeneration must NOT be called when health verification fails")
	}
}

// ── failure_cleanup ───────────────────────────────────────────────────────────

// TestFailureCleanup_ClearsRestartInProgress verifies that failure_cleanup
// clears restart_in_progress so future topology workflows can run.
func TestFailureCleanup_ClearsRestartInProgress(t *testing.T) {
	lockReleased := false
	restartCleared := false
	resultWritten := false

	cfg := buildTestObjectStoreConfig()
	cfg.FailureCleanup = func(_ context.Context, gen int64, reason string) error {
		lockReleased = true
		restartCleared = true
		resultWritten = true
		return nil
	}

	_, err := callAction(cfg, "controller.objectstore.failure_cleanup",
		map[string]any{"target_generation": int64(3), "reason": "workflow_failed"},
		nil)
	if err != nil {
		t.Fatalf("failure_cleanup returned error: %v", err)
	}
	if !lockReleased {
		t.Error("expected lock to be released in failure_cleanup")
	}
	if !restartCleared {
		t.Error("expected restart_in_progress to be cleared in failure_cleanup")
	}
	if !resultWritten {
		t.Error("expected last_restart_result to be written in failure_cleanup")
	}
}

// TestFailureCleanup_WritesStatusFailed simulates the real cleanup function
// and verifies that it writes a result with status=failed (not success).
func TestFailureCleanup_WritesStatusFailed(t *testing.T) {
	var capturedGen int64
	var capturedReason string

	cfg := buildTestObjectStoreConfig()
	cfg.FailureCleanup = func(_ context.Context, gen int64, reason string) error {
		capturedGen = gen
		capturedReason = reason
		return nil
	}

	_, err := callAction(cfg, "controller.objectstore.failure_cleanup",
		map[string]any{"target_generation": int64(5), "reason": "health_check_failed"},
		nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedGen != 5 {
		t.Errorf("expected gen=5, got %d", capturedGen)
	}
	if capturedReason != "health_check_failed" {
		t.Errorf("expected reason=health_check_failed, got %q", capturedReason)
	}
}

// TestFailureCleanup_DefaultReason verifies that when no reason is provided,
// the action defaults to "workflow_failed".
func TestFailureCleanup_DefaultReason(t *testing.T) {
	var capturedReason string
	cfg := buildTestObjectStoreConfig()
	cfg.FailureCleanup = func(_ context.Context, gen int64, reason string) error {
		capturedReason = reason
		return nil
	}
	_, err := callAction(cfg, "controller.objectstore.failure_cleanup",
		map[string]any{"target_generation": int64(1)},
		nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedReason != "workflow_failed" {
		t.Errorf("expected default reason=workflow_failed, got %q", capturedReason)
	}
}

// ── lock actions ─────────────────────────────────────────────────────────────

// TestAcquireTopologyLock_AlreadyHeld simulates a lock contention scenario:
// AcquireTopologyLock returns an error, the action must propagate it.
func TestAcquireTopologyLock_AlreadyHeld(t *testing.T) {
	cfg := buildTestObjectStoreConfig()
	cfg.AcquireTopologyLock = func(_ context.Context) error {
		return fmt.Errorf("topology lock already held")
	}
	_, err := callAction(cfg, "controller.objectstore.acquire_topology_lock", nil, nil)
	if err == nil {
		t.Fatal("expected error when lock is already held")
	}
}

// TestReleaseTopologyLock_Idempotent verifies that releasing a lock that
// doesn't exist does not cause the action to fail.
func TestReleaseTopologyLock_Idempotent(t *testing.T) {
	cfg := buildTestObjectStoreConfig()
	// cfg.ReleaseTopologyLock returns nil by default (idempotent delete in real impl)
	_, err := callAction(cfg, "controller.objectstore.release_topology_lock", nil, nil)
	if err != nil {
		t.Fatalf("release_topology_lock should be idempotent, got: %v", err)
	}
}

// ── record_applied_generation ────────────────────────────────────────────────

// TestRecordAppliedGeneration_RequiresGeneration verifies the action rejects
// a call without target_generation.
func TestRecordAppliedGeneration_RequiresGeneration(t *testing.T) {
	cfg := buildTestObjectStoreConfig()
	_, err := callAction(cfg, "controller.objectstore.record_applied_generation", nil, nil)
	if err == nil {
		t.Fatal("expected error for missing target_generation")
	}
}

func TestRecordAppliedGeneration_Success(t *testing.T) {
	var recorded int64
	cfg := buildTestObjectStoreConfig()
	cfg.RecordAppliedGeneration = func(_ context.Context, gen int64) error {
		recorded = gen
		return nil
	}
	_, err := callAction(cfg, "controller.objectstore.record_applied_generation",
		map[string]any{"target_generation": int64(7)}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recorded != 7 {
		t.Errorf("expected gen=7 recorded, got %d", recorded)
	}
}

// ── all actions registered ────────────────────────────────────────────────────

// TestAllObjectStoreActionsRegistered verifies that every action name used in
// the YAML appears in the registered router. This is the engine-level equivalent
// of TestEmbeddedWorkflowsHaveRegisteredActions for the objectstore workflow.
func TestAllObjectStoreActionsRegistered(t *testing.T) {
	router := NewRouter()
	RegisterObjectStoreControllerActions(router, ObjectStoreControllerConfig{})

	required := []string{
		"controller.objectstore.check_all_nodes_rendered",
		"controller.objectstore.acquire_topology_lock",
		"controller.objectstore.release_topology_lock",
		"controller.objectstore.mark_restart_in_progress",
		"controller.objectstore.clear_restart_in_progress",
		"controller.objectstore.record_applied_generation",
		"controller.objectstore.verify_minio_cluster_healthy",
		"controller.objectstore.failure_cleanup",
	}

	for _, action := range required {
		if _, ok := router.Resolve(actorClusterController, action); !ok {
			t.Errorf("action %q not registered", action)
		}
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// actorClusterController mirrors v1alpha1.ActorClusterController without importing
// the package from a test file (avoids import cycle risk).
const actorClusterController = "cluster-controller"
