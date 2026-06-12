package engine

import (
	"context"
	"errors"
	"testing"
)

// ── helpers ──────────────────────────────────────────────────────────────

func makeReq(with map[string]any) ActionRequest {
	return ActionRequest{
		RunID:  "test-run",
		StepID: "test-step",
		With:   with,
	}
}

func stubConfig() NodeRemoveControllerConfig {
	return NodeRemoveControllerConfig{
		Preflight: func(ctx context.Context, nodeID string) ([]NodeRemovePreflightViolation, error) {
			return nil, nil
		},
		RemoveEtcdMembership: func(ctx context.Context, nodeID string) error {
			return nil
		},
		DrainNode: func(ctx context.Context, nodeID, opID, agentEndpoint string) error {
			return nil
		},
		DeleteState: func(ctx context.Context, nodeID string) error {
			return nil
		},
		PublishScyllaHosts: func(ctx context.Context) error {
			return nil
		},
		CleanupEtcdPrefixes: func(ctx context.Context, nodeID string) (int64, error) {
			return 0, nil
		},
		CleanupReleaseStatus: func(ctx context.Context, nodeID string) (int, error) {
			return 0, nil
		},
		RemoveFromScyllaRing: func(ctx context.Context, nodeID, scyllaHostID string, nodeIPs []string) error {
			return nil
		},
	}
}

// ── preflight tests ──────────────────────────────────────────────────────

func TestNodeRemovePreflight_NoViolations(t *testing.T) {
	cfg := stubConfig()
	handler := nodeRemovePreflight(cfg)
	result, err := handler(context.Background(), makeReq(map[string]any{
		"node_id": "test-node",
		"force":   false,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.OK {
		t.Fatal("expected OK=true")
	}
	if result.Output["violation_count"].(int) != 0 {
		t.Fatalf("expected 0 violations, got %v", result.Output["violation_count"])
	}
}

func TestNodeRemovePreflight_ViolationsBlockWithoutForce(t *testing.T) {
	cfg := stubConfig()
	cfg.Preflight = func(ctx context.Context, nodeID string) ([]NodeRemovePreflightViolation, error) {
		return []NodeRemovePreflightViolation{
			{Code: "storage_quorum", Message: "would drop below 3 storage nodes"},
		}, nil
	}
	handler := nodeRemovePreflight(cfg)
	_, err := handler(context.Background(), makeReq(map[string]any{
		"node_id": "test-node",
		"force":   false,
	}))
	if err == nil {
		t.Fatal("expected error for violations without force")
	}
	if !errors.Is(err, err) { // just checking it's non-nil
		t.Fatalf("unexpected error type: %v", err)
	}
}

func TestNodeRemovePreflight_ViolationsPassWithForce(t *testing.T) {
	cfg := stubConfig()
	cfg.Preflight = func(ctx context.Context, nodeID string) ([]NodeRemovePreflightViolation, error) {
		return []NodeRemovePreflightViolation{
			{Code: "storage_quorum", Message: "would drop below 3 storage nodes"},
		}, nil
	}
	handler := nodeRemovePreflight(cfg)
	result, err := handler(context.Background(), makeReq(map[string]any{
		"node_id": "test-node",
		"force":   true,
	}))
	if err != nil {
		t.Fatalf("unexpected error with force=true: %v", err)
	}
	if !result.OK {
		t.Fatal("expected OK=true with force override")
	}
	if result.Output["force_override"] != true {
		t.Fatal("expected force_override=true in output")
	}
}

func TestNodeRemovePreflight_MissingNodeID(t *testing.T) {
	cfg := stubConfig()
	handler := nodeRemovePreflight(cfg)
	_, err := handler(context.Background(), makeReq(map[string]any{}))
	if err == nil {
		t.Fatal("expected error for missing node_id")
	}
}

// ── etcd membership tests ────────────────────────────────────────────────

func TestNodeRemoveEtcdMembership_Success(t *testing.T) {
	called := false
	cfg := stubConfig()
	cfg.RemoveEtcdMembership = func(ctx context.Context, nodeID string) error {
		called = true
		if nodeID != "node-123" {
			t.Fatalf("expected node-123, got %s", nodeID)
		}
		return nil
	}
	handler := nodeRemoveEtcdMembership(cfg)
	result, err := handler(context.Background(), makeReq(map[string]any{"node_id": "node-123"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.OK || !called {
		t.Fatal("expected OK and handler called")
	}
}

func TestNodeRemoveEtcdMembership_Error(t *testing.T) {
	cfg := stubConfig()
	cfg.RemoveEtcdMembership = func(ctx context.Context, nodeID string) error {
		return errors.New("etcd unreachable")
	}
	handler := nodeRemoveEtcdMembership(cfg)
	_, err := handler(context.Background(), makeReq(map[string]any{"node_id": "node-123"}))
	if err == nil {
		t.Fatal("expected error")
	}
}

// ── drain tests ──────────────────────────────────────────────────────────

func TestNodeRemoveDrain_Success(t *testing.T) {
	cfg := stubConfig()
	cfg.DrainNode = func(ctx context.Context, nodeID, opID, agentEndpoint string) error {
		if nodeID != "n1" || agentEndpoint != "10.0.0.1:11000" {
			t.Fatalf("wrong args: %s %s", nodeID, agentEndpoint)
		}
		return nil
	}
	handler := nodeRemoveDrainNode(cfg)
	result, err := handler(context.Background(), makeReq(map[string]any{
		"node_id":        "n1",
		"op_id":          "op-1",
		"agent_endpoint": "10.0.0.1:11000",
		"force":          false,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.OK {
		t.Fatal("expected OK")
	}
}

func TestNodeRemoveDrain_FailureBlocksWithoutForce(t *testing.T) {
	cfg := stubConfig()
	cfg.DrainNode = func(ctx context.Context, nodeID, opID, agentEndpoint string) error {
		return errors.New("agent unreachable")
	}
	handler := nodeRemoveDrainNode(cfg)
	_, err := handler(context.Background(), makeReq(map[string]any{
		"node_id":        "n1",
		"op_id":          "op-1",
		"agent_endpoint": "10.0.0.1:11000",
		"force":          false,
	}))
	if err == nil {
		t.Fatal("expected error for drain failure without force")
	}
}

func TestNodeRemoveDrain_FailureContinuesWithForce(t *testing.T) {
	cfg := stubConfig()
	cfg.DrainNode = func(ctx context.Context, nodeID, opID, agentEndpoint string) error {
		return errors.New("agent unreachable")
	}
	handler := nodeRemoveDrainNode(cfg)
	result, err := handler(context.Background(), makeReq(map[string]any{
		"node_id":        "n1",
		"op_id":          "op-1",
		"agent_endpoint": "10.0.0.1:11000",
		"force":          true,
	}))
	if err != nil {
		t.Fatalf("unexpected error with force=true: %v", err)
	}
	if !result.OK {
		t.Fatal("expected OK with force override")
	}
	if result.Output["drain_error"] == nil {
		t.Fatal("expected drain_error in output")
	}
}

// ── delete state tests ───────────────────────────────────────────────────

func TestNodeRemoveDeleteState_Success(t *testing.T) {
	deleted := false
	cfg := stubConfig()
	cfg.DeleteState = func(ctx context.Context, nodeID string) error {
		deleted = true
		return nil
	}
	handler := nodeRemoveDeleteState(cfg)
	result, err := handler(context.Background(), makeReq(map[string]any{"node_id": "n1"}))
	if err != nil || !result.OK || !deleted {
		t.Fatalf("err=%v ok=%v deleted=%v", err, result.OK, deleted)
	}
}

// ── scylla ring tests ────────────────────────────────────────────────────

func TestNodeRemoveScyllaRing_NonFatalOnError(t *testing.T) {
	cfg := stubConfig()
	cfg.RemoveFromScyllaRing = func(ctx context.Context, nodeID, scyllaHostID string, nodeIPs []string) error {
		return errors.New("no healthy peer")
	}
	handler := nodeRemoveScyllaRing(cfg)
	result, err := handler(context.Background(), makeReq(map[string]any{
		"node_id":        "n1",
		"scylla_host_id": "abc",
		"node_ips":       []any{"10.0.0.1"},
	}))
	if err != nil {
		t.Fatalf("scylla ring error should be non-fatal, got: %v", err)
	}
	if !result.OK {
		t.Fatal("expected OK even on ring error")
	}
	if result.Output["warning"] == nil {
		t.Fatal("expected warning in output")
	}
}

func TestNodeRemoveScyllaRing_Success(t *testing.T) {
	cfg := stubConfig()
	handler := nodeRemoveScyllaRing(cfg)
	result, err := handler(context.Background(), makeReq(map[string]any{
		"node_id":        "n1",
		"scylla_host_id": "abc",
		"node_ips":       []any{"10.0.0.1"},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.OK {
		t.Fatal("expected OK")
	}
}

// ── cleanup tests ────────────────────────────────────────────────────────

func TestNodeRemoveCleanupEtcdPrefixes_ReturnsCount(t *testing.T) {
	cfg := stubConfig()
	cfg.CleanupEtcdPrefixes = func(ctx context.Context, nodeID string) (int64, error) {
		return 42, nil
	}
	handler := nodeRemoveCleanupEtcdPrefixes(cfg)
	result, err := handler(context.Background(), makeReq(map[string]any{"node_id": "n1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output["keys_deleted"].(int64) != 42 {
		t.Fatalf("expected 42 keys deleted, got %v", result.Output["keys_deleted"])
	}
}

func TestNodeRemoveCleanupReleaseStatus_ReturnsCount(t *testing.T) {
	cfg := stubConfig()
	cfg.CleanupReleaseStatus = func(ctx context.Context, nodeID string) (int, error) {
		return 5, nil
	}
	handler := nodeRemoveCleanupReleaseStatus(cfg)
	result, err := handler(context.Background(), makeReq(map[string]any{"node_id": "n1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output["releases_cleaned"].(int) != 5 {
		t.Fatalf("expected 5 releases, got %v", result.Output["releases_cleaned"])
	}
}

// ── registration test ────────────────────────────────────────────────────

func TestRegisterNodeRemoveControllerActions_AllRegistered(t *testing.T) {
	router := NewRouter()
	cfg := stubConfig()
	RegisterNodeRemoveControllerActions(router, cfg)

	expectedActions := []string{
		"controller.node_remove.preflight",
		"controller.node_remove.remove_etcd_membership",
		"controller.node_remove.drain_node",
		"controller.node_remove.delete_state",
		"controller.node_remove.publish_scylla_hosts",
		"controller.node_remove.cleanup_etcd_prefixes",
		"controller.node_remove.cleanup_release_status",
		"controller.node_remove.remove_scylla_ring",
	}

	for _, action := range expectedActions {
		key := "cluster-controller::" + action
		router.mu.RLock()
		_, exists := router.handlers[key]
		router.mu.RUnlock()
		if !exists {
			t.Errorf("action %q not registered", action)
		}
	}
}

// ── nil handler tests ────────────────────────────────────────────────────

func TestNodeRemoveHandlers_NilConfigErrors(t *testing.T) {
	empty := NodeRemoveControllerConfig{}
	handlers := []struct {
		name string
		fn   ActionHandler
		with map[string]any
	}{
		{"preflight", nodeRemovePreflight(empty), map[string]any{"node_id": "n1"}},
		{"etcd", nodeRemoveEtcdMembership(empty), map[string]any{"node_id": "n1"}},
		{"drain", nodeRemoveDrainNode(empty), map[string]any{"node_id": "n1", "agent_endpoint": "x"}},
		{"delete", nodeRemoveDeleteState(empty), map[string]any{"node_id": "n1"}},
		{"scylla_hosts", nodeRemovePublishScyllaHosts(empty), map[string]any{}},
		{"etcd_prefixes", nodeRemoveCleanupEtcdPrefixes(empty), map[string]any{"node_id": "n1"}},
		{"release_status", nodeRemoveCleanupReleaseStatus(empty), map[string]any{"node_id": "n1"}},
		{"scylla_ring", nodeRemoveScyllaRing(empty), map[string]any{"node_id": "n1"}},
	}
	for _, tc := range handlers {
		_, err := tc.fn(context.Background(), makeReq(tc.with))
		if err == nil {
			t.Errorf("%s: expected error for nil handler", tc.name)
		}
	}
}
