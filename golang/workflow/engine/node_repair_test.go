package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/globulario/services/golang/workflow/v1alpha1"
)

func loadNodeRepairDef(t *testing.T) *v1alpha1.WorkflowDefinition {
	t.Helper()
	loader := v1alpha1.NewLoader()
	def, err := loader.LoadFile("../definitions/node.repair.yaml")
	if err != nil {
		t.Fatalf("load definition: %v", err)
	}
	return def
}

func nodeRepairInputs(nodeID, reason string) map[string]any {
	return map[string]any{
		"cluster_id":      "test-cluster",
		"node_id":         nodeID,
		"reason":          reason,
		"isolate_first":   true,
		"target_packages": []any{"envoy", "gateway"},
	}
}

type nodeRepairTestOpts struct {
	collectFacts    func(ctx context.Context, nodeID string, pkgs []any) (map[string]any, error)
	classify        func(ctx context.Context, nodeID string, diagnosis map[string]any) (map[string]any, error)
	repairPackages  func(ctx context.Context, nodeID string, plan map[string]any) (map[string]any, error)
	restartRepaired func(ctx context.Context, nodeID string, result map[string]any) error
	verifyRuntime   func(ctx context.Context, nodeID string, plan map[string]any) error
	syncState       func(ctx context.Context, nodeID string) error
}

func defaultNodeRepairOpts() nodeRepairTestOpts {
	return nodeRepairTestOpts{
		collectFacts: func(ctx context.Context, nodeID string, pkgs []any) (map[string]any, error) {
			return map[string]any{"node_id": nodeID, "status": "degraded", "packages": pkgs}, nil
		},
		classify: func(ctx context.Context, nodeID string, diagnosis map[string]any) (map[string]any, error) {
			return map[string]any{"action": "reinstall", "packages": []any{"envoy"}}, nil
		},
		repairPackages: func(ctx context.Context, nodeID string, plan map[string]any) (map[string]any, error) {
			return map[string]any{"repaired": true, "packages_fixed": 1}, nil
		},
		restartRepaired: func(ctx context.Context, nodeID string, result map[string]any) error { return nil },
		verifyRuntime:   func(ctx context.Context, nodeID string, plan map[string]any) error { return nil },
		syncState:       func(ctx context.Context, nodeID string) error { return nil },
	}
}

func newNodeRepairRouter(t *testing.T, opts nodeRepairTestOpts) *Router {
	t.Helper()
	router := NewRouter()

	var mu sync.Mutex
	var events []string
	record := func(e string) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	}

	RegisterNodeRepairControllerActions(router, NodeRepairControllerConfig{
		MarkStarted: func(ctx context.Context, nodeID, reason string) error {
			record("mark_started:" + nodeID)
			return nil
		},
		Classify:      opts.classify,
		IsolateNode:   func(ctx context.Context, nodeID string, plan map[string]any) error { record("isolate:" + nodeID); return nil },
		RejoinNode:    func(ctx context.Context, nodeID string) error { record("rejoin:" + nodeID); return nil },
		MarkRecovered: func(ctx context.Context, nodeID string) error { record("recovered:" + nodeID); return nil },
		MarkFailed:    func(ctx context.Context, nodeID string) error { record("failed:" + nodeID); return nil },
		EmitRecovered: func(ctx context.Context, nodeID string) error { record("emit_recovered:" + nodeID); return nil },
	})

	RegisterNodeRepairAgentActions(router, NodeRepairAgentConfig{
		CollectRepairFacts:      opts.collectFacts,
		RepairPackages:          opts.repairPackages,
		RestartRepairedServices: opts.restartRepaired,
		VerifyRepairRuntime:     opts.verifyRuntime,
		SyncInstalledState:      opts.syncState,
	})

	// Register verification actions required by sync_installed_state step.
	RegisterNodeVerificationActions(router, NodeVerificationConfig{})

	return router
}

// Test 1: Checksum drift repaired successfully.
func TestNodeRepair_ChecksumDriftSuccess(t *testing.T) {
	opts := defaultNodeRepairOpts()
	opts.classify = func(ctx context.Context, nodeID string, diagnosis map[string]any) (map[string]any, error) {
		return map[string]any{"action": "reinstall", "reason": "checksum_mismatch"}, nil
	}

	router := newNodeRepairRouter(t, opts)
	eng := &Engine{Router: router}
	def := loadNodeRepairDef(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	run, err := eng.Execute(ctx, def, nodeRepairInputs("node-1", "checksum_drift"))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if run.Status != RunSucceeded {
		t.Errorf("expected SUCCEEDED, got %s (error: %s)", run.Status, run.Error)
		for id, st := range run.Steps {
			if st.Status == StepFailed {
				t.Errorf("  step %s FAILED: %s", id, st.Error)
			}
		}
	}

	// Verify full chain ran.
	expectedSteps := []string{
		"mark_repair_started", "diagnose_node", "classify_failure",
		"maybe_isolate", "repair_packages", "restart_repaired_services",
		"verify_runtime", "sync_installed_state", "rejoin_node", "mark_recovered",
	}
	for _, stepID := range expectedSteps {
		st := run.Steps[stepID]
		if st == nil {
			t.Errorf("step %s not found", stepID)
			continue
		}
		if st.Status != StepSucceeded {
			t.Errorf("step %s: expected SUCCEEDED, got %s", stepID, st.Status)
		}
	}
}

// Test 2: Runtime failure repaired by restart only (repair_packages is no-op).
func TestNodeRepair_RuntimeFailureRestartOnly(t *testing.T) {
	opts := defaultNodeRepairOpts()
	opts.classify = func(ctx context.Context, nodeID string, diagnosis map[string]any) (map[string]any, error) {
		return map[string]any{"action": "restart_only"}, nil
	}
	opts.repairPackages = func(ctx context.Context, nodeID string, plan map[string]any) (map[string]any, error) {
		// No actual repair needed — packages are fine, just need restart.
		return map[string]any{"repaired": false, "action": "restart_only"}, nil
	}

	router := newNodeRepairRouter(t, opts)
	eng := &Engine{Router: router}
	def := loadNodeRepairDef(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	run, err := eng.Execute(ctx, def, nodeRepairInputs("node-2", "runtime_failure"))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if run.Status != RunSucceeded {
		t.Errorf("expected SUCCEEDED, got %s (error: %s)", run.Status, run.Error)
	}
}

// Test 3: Corrupted package requires reinstall.
func TestNodeRepair_CorruptedPackageReinstall(t *testing.T) {
	var mu sync.Mutex
	var actions []string
	record := func(a string) {
		mu.Lock()
		actions = append(actions, a)
		mu.Unlock()
	}

	opts := defaultNodeRepairOpts()
	opts.repairPackages = func(ctx context.Context, nodeID string, plan map[string]any) (map[string]any, error) {
		record("repair:" + nodeID)
		return map[string]any{"repaired": true, "reinstalled": []string{"envoy"}}, nil
	}
	opts.restartRepaired = func(ctx context.Context, nodeID string, result map[string]any) error {
		record("restart:" + nodeID)
		return nil
	}
	opts.verifyRuntime = func(ctx context.Context, nodeID string, plan map[string]any) error {
		record("verify:" + nodeID)
		return nil
	}
	opts.syncState = func(ctx context.Context, nodeID string) error {
		record("sync:" + nodeID)
		return nil
	}

	router := newNodeRepairRouter(t, opts)
	eng := &Engine{Router: router}
	def := loadNodeRepairDef(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	run, err := eng.Execute(ctx, def, nodeRepairInputs("node-3", "corrupted_package"))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if run.Status != RunSucceeded {
		t.Errorf("expected SUCCEEDED, got %s", run.Status)
	}

	mu.Lock()
	t.Logf("Actions: %s", strings.Join(actions, " → "))
	for _, prefix := range []string{"repair:", "restart:", "verify:", "sync:"} {
		found := false
		for _, a := range actions {
			if strings.HasPrefix(a, prefix) {
				found = true
			}
		}
		if !found {
			t.Errorf("expected action with prefix %q", prefix)
		}
	}
	mu.Unlock()
}

// Test 4: Unrecoverable node → workflow fails, node remains isolated.
func TestNodeRepair_UnrecoverableFails(t *testing.T) {
	opts := defaultNodeRepairOpts()
	opts.repairPackages = func(ctx context.Context, nodeID string, plan map[string]any) (map[string]any, error) {
		return nil, fmt.Errorf("unrecoverable: disk read-only filesystem")
	}

	router := newNodeRepairRouter(t, opts)
	eng := &Engine{Router: router}
	def := loadNodeRepairDef(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	run, err := eng.Execute(ctx, def, nodeRepairInputs("node-4", "disk_failure"))
	if err == nil {
		t.Fatal("expected error from unrecoverable failure")
	}
	if run.Status != RunFailed {
		t.Errorf("expected FAILED, got %s", run.Status)
	}

	// rejoin_node and mark_recovered should NOT have run.
	for _, stepID := range []string{"rejoin_node", "mark_recovered"} {
		st := run.Steps[stepID]
		if st != nil && st.Status == StepSucceeded {
			t.Errorf("step %s should NOT have succeeded (node is unrecoverable)", stepID)
		}
	}

	t.Logf("Status: %s, Error: %s", run.Status, run.Error)
}

// Test 5: Skip isolation when isolate_first=false.
func TestNodeRepair_SkipIsolation(t *testing.T) {
	opts := defaultNodeRepairOpts()
	router := newNodeRepairRouter(t, opts)
	eng := &Engine{Router: router}
	def := loadNodeRepairDef(t)

	inputs := nodeRepairInputs("node-5", "minor_drift")
	inputs["isolate_first"] = false

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	run, err := eng.Execute(ctx, def, inputs)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if run.Status != RunSucceeded {
		t.Errorf("expected SUCCEEDED, got %s", run.Status)
	}

	// maybe_isolate should be SKIPPED.
	if st := run.Steps["maybe_isolate"]; st != nil && st.Status != StepSkipped {
		t.Errorf("maybe_isolate should be SKIPPED when isolate_first=false, got %s", st.Status)
	}
}
