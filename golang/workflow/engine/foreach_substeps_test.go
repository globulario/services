package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/globulario/services/golang/workflow/compiler"
	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// buildForeachSubStepsDef creates an inline definition with a foreach step
// that has nested sub-steps (install → verify → sync per node).
func buildForeachSubStepsDef() *v1alpha1.WorkflowDefinition {
	return &v1alpha1.WorkflowDefinition{
		APIVersion: v1alpha1.APIVersion,
		Kind:       v1alpha1.Kind,
		Metadata:   v1alpha1.WorkflowMetadata{Name: "test-foreach-substeps"},
		Spec: v1alpha1.WorkflowDefinitionSpec{
			Strategy: v1alpha1.ExecutionStrategy{Mode: v1alpha1.StrategySingle},
			Defaults: map[string]any{
				"nodes": []any{"node-a", "node-b"},
			},
			Steps: []v1alpha1.WorkflowStepSpec{
				{
					ID:       "apply_per_node",
					Foreach:  &v1alpha1.ScalarString{Raw: "$.nodes"},
					ItemName: &v1alpha1.ScalarString{Raw: "target"},
					Export:   &v1alpha1.ScalarString{Raw: "results"},
					Steps: []v1alpha1.WorkflowStepSpec{
						{
							ID:     "install",
							Actor:  v1alpha1.ActorNodeAgent,
							Action: "node.install_package",
							With: map[string]any{
								"package_name": "envoy",
								"version":      "1.35.3",
								"kind":         "INFRASTRUCTURE",
							},
						},
						{
							ID:        "verify",
							Actor:     v1alpha1.ActorNodeAgent,
							Action:    "node.verify_package_installed",
							DependsOn: []string{"install"},
							With: map[string]any{
								"package_name": "envoy",
								"version":      "1.35.3",
								"desired_hash": "abc123",
							},
						},
						{
							ID:        "sync",
							Actor:     v1alpha1.ActorNodeAgent,
							Action:    "node.sync_installed_package_state",
							DependsOn: []string{"verify"},
							With: map[string]any{
								"package_name": "envoy",
								"version":      "1.35.3",
								"desired_hash": "abc123",
							},
						},
					},
				},
				{
					ID:        "finalize",
					Actor:     v1alpha1.ActorClusterController,
					Action:    "controller.release.finalize_direct_apply",
					DependsOn: []string{"apply_per_node"},
				},
			},
		},
	}
}

func TestForeachWithSubSteps_AllSucceed(t *testing.T) {
	var mu sync.Mutex
	var actions []string
	record := func(a string) {
		mu.Lock()
		actions = append(actions, a)
		mu.Unlock()
	}

	router := NewRouter()
	RegisterNodeDirectApplyActions(router, NodeDirectApplyConfig{
		InstallPackage: func(ctx context.Context, name, version, kind, buildID, desiredHash, expectedSha256 string, buildNumber int64) error {
			record(fmt.Sprintf("install:%s", name))
			return nil
		},
		VerifyPackageInstalled: func(ctx context.Context, name, version, hash string) error {
			record(fmt.Sprintf("verify:%s", name))
			return nil
		},
		SyncInstalledPackage: func(ctx context.Context, name, version, hash, kind, buildID string) error {
			record(fmt.Sprintf("sync:%s", name))
			return nil
		},
	})
	RegisterReleaseControllerActions(router, ReleaseControllerConfig{
		FinalizeDirectApply: func(ctx context.Context, releaseID string, aggregate map[string]any) error {
			record("finalize")
			return nil
		},
	})

	eng := &Engine{Router: router}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	run, err := eng.Execute(ctx, buildForeachSubStepsDef(), nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if run.Status != RunSucceeded {
		t.Errorf("expected SUCCEEDED, got %s (error: %s)", run.Status, run.Error)
		for id, st := range run.Steps {
			if st.Status == StepFailed {
				t.Errorf("  step %s: %s", id, st.Error)
			}
		}
	}

	// Verify each node got install → verify → sync.
	mu.Lock()
	defer mu.Unlock()

	installCount := 0
	verifyCount := 0
	syncCount := 0
	for _, a := range actions {
		if strings.HasPrefix(a, "install:") {
			installCount++
		}
		if strings.HasPrefix(a, "verify:") {
			verifyCount++
		}
		if strings.HasPrefix(a, "sync:") {
			syncCount++
		}
	}
	if installCount != 2 {
		t.Errorf("expected 2 installs, got %d", installCount)
	}
	if verifyCount != 2 {
		t.Errorf("expected 2 verifies, got %d", verifyCount)
	}
	if syncCount != 2 {
		t.Errorf("expected 2 syncs, got %d", syncCount)
	}

	// Verify results exported.
	results, ok := run.Outputs["results"].([]any)
	if !ok {
		t.Fatalf("expected results in outputs, got %T", run.Outputs["results"])
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	// Verify finalize ran.
	found := false
	for _, a := range actions {
		if a == "finalize" {
			found = true
		}
	}
	if !found {
		t.Error("expected finalize action to run")
	}

	t.Logf("Actions: %s", strings.Join(actions, " → "))
}

func TestForeachWithSubSteps_PartialFailure(t *testing.T) {
	var mu sync.Mutex
	installCount := 0

	router := NewRouter()
	RegisterNodeDirectApplyActions(router, NodeDirectApplyConfig{
		InstallPackage: func(ctx context.Context, name, version, kind, buildID, desiredHash, expectedSha256 string, buildNumber int64) error {
			mu.Lock()
			installCount++
			n := installCount
			mu.Unlock()
			if n == 2 { // second node fails
				return fmt.Errorf("download failed: connection refused")
			}
			return nil
		},
		VerifyPackageInstalled: func(ctx context.Context, name, version, hash string) error { return nil },
		SyncInstalledPackage:   func(ctx context.Context, name, version, hash, kind, buildID string) error { return nil },
	})
	RegisterReleaseControllerActions(router, ReleaseControllerConfig{
		FinalizeDirectApply: func(ctx context.Context, releaseID string, aggregate map[string]any) error {
			return nil
		},
	})

	eng := &Engine{Router: router}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	run, err := eng.Execute(ctx, buildForeachSubStepsDef(), nil)

	// Should fail because one item failed.
	if err == nil {
		t.Fatal("expected error from partial failure")
	}
	if run.Status != RunFailed {
		t.Errorf("expected FAILED, got %s", run.Status)
	}

	// The foreach step output should show 1 succeeded, 1 failed.
	apnState := run.Steps["apply_per_node"]
	if apnState == nil {
		t.Fatal("apply_per_node step not found")
	}
	if apnState.Output == nil {
		t.Fatal("apply_per_node has no output")
	}
	succeeded, _ := apnState.Output["succeeded"].(int)
	failed, _ := apnState.Output["failed"].(int)
	if succeeded != 1 || failed != 1 {
		t.Errorf("expected 1 succeeded + 1 failed, got %d succeeded + %d failed", succeeded, failed)
	}

	t.Logf("Status: %s, Error: %s", run.Status, run.Error)
}

func TestForeachWithSubSteps_EmptyCollection(t *testing.T) {
	router := NewRouter()
	RegisterNodeDirectApplyActions(router, NodeDirectApplyConfig{})
	RegisterReleaseControllerActions(router, ReleaseControllerConfig{})

	eng := &Engine{Router: router}

	def := buildForeachSubStepsDef()
	run, err := eng.Execute(context.Background(), def, map[string]any{
		"nodes": []any{}, // empty
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if run.Status != RunSucceeded {
		t.Errorf("expected SUCCEEDED, got %s", run.Status)
	}

	// The foreach step should be SKIPPED.
	apnState := run.Steps["apply_per_node"]
	if apnState == nil {
		t.Fatal("apply_per_node step not found")
	}
	if apnState.Status != StepSkipped {
		t.Errorf("expected SKIPPED for empty collection, got %s", apnState.Status)
	}
}

func TestForeachWithSubSteps_ChildStatesVisibleInParent(t *testing.T) {
	router := NewRouter()
	RegisterNodeDirectApplyActions(router, NodeDirectApplyConfig{
		InstallPackage:         func(ctx context.Context, name, version, kind, buildID, desiredHash, expectedSha256 string, buildNumber int64) error { return nil },
		VerifyPackageInstalled: func(ctx context.Context, name, version, hash string) error { return nil },
		SyncInstalledPackage:   func(ctx context.Context, name, version, hash, kind, buildID string) error { return nil },
	})
	RegisterReleaseControllerActions(router, ReleaseControllerConfig{})

	eng := &Engine{Router: router}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	run, err := eng.Execute(ctx, buildForeachSubStepsDef(), nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	// Child step states should be visible as "apply_per_node[0].install", etc.
	expectedPrefixes := []string{
		"apply_per_node[0].",
		"apply_per_node[1].",
	}
	for _, prefix := range expectedPrefixes {
		found := false
		for id := range run.Steps {
			if strings.HasPrefix(id, prefix) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected child step with prefix %q in parent run", prefix)
		}
	}

	t.Logf("Total steps (including child): %d", len(run.Steps))
	for id, st := range run.Steps {
		t.Logf("  %s: %s", id, st.Status)
	}
}

// buildParallelForeachDef returns a definition with strategy.mode=parallel on the foreach step.
// Each item sleeps for delayMs before completing via a "node.slow_action" handler.
func buildParallelForeachDef(concurrency int) *v1alpha1.WorkflowDefinition {
	n := concurrency
	concurrencyScalar := &v1alpha1.ScalarInt{Value: &n}
	return &v1alpha1.WorkflowDefinition{
		APIVersion: v1alpha1.APIVersion,
		Kind:       v1alpha1.Kind,
		Metadata:   v1alpha1.WorkflowMetadata{Name: "test-parallel-foreach"},
		Spec: v1alpha1.WorkflowDefinitionSpec{
			Strategy: v1alpha1.ExecutionStrategy{Mode: v1alpha1.StrategySingle},
			Defaults: map[string]any{
				"items": []any{"a", "b", "c", "d"},
			},
			Steps: []v1alpha1.WorkflowStepSpec{
				{
					ID:       "process_items",
					Foreach:  &v1alpha1.ScalarString{Raw: "$.items"},
					ItemName: &v1alpha1.ScalarString{Raw: "item"},
					Export:   &v1alpha1.ScalarString{Raw: "results"},
					Strategy: &v1alpha1.ExecutionStrategy{
						Mode:        v1alpha1.StrategyMode("parallel"),
						Concurrency: concurrencyScalar,
					},
					Steps: []v1alpha1.WorkflowStepSpec{
						{
							ID:     "work",
							Actor:  v1alpha1.ActorNodeAgent,
							Action: "node.slow_action",
						},
					},
				},
			},
		},
	}
}

func TestForeachWithSubSteps_ParallelExecution_FasterThanSerial(t *testing.T) {
	// Each item takes ~50ms. With 4-item parallelism, total should be ~50ms not ~200ms.
	const itemDelay = 50 * time.Millisecond
	var mu sync.Mutex
	activeCount := 0
	maxConcurrent := 0

	router := NewRouter()
	router.Register(v1alpha1.ActorNodeAgent, "node.slow_action", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		mu.Lock()
		activeCount++
		if activeCount > maxConcurrent {
			maxConcurrent = activeCount
		}
		mu.Unlock()

		time.Sleep(itemDelay)

		mu.Lock()
		activeCount--
		mu.Unlock()
		return &ActionResult{OK: true}, nil
	})

	eng := &Engine{Router: router}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()
	run, err := eng.Execute(ctx, buildParallelForeachDef(4), nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if run.Status != RunSucceeded {
		t.Errorf("expected SUCCEEDED, got %s", run.Status)
	}

	// 4 items × 50ms serial = 200ms. Parallel should be <130ms (50ms + margin).
	if elapsed > 150*time.Millisecond {
		t.Errorf("parallel foreach took %s, expected <150ms (4 items × 50ms should overlap)", elapsed)
	}
	t.Logf("Elapsed: %s, maxConcurrent: %d", elapsed, maxConcurrent)
	if maxConcurrent < 2 {
		t.Errorf("expected at least 2 concurrent executions, got %d", maxConcurrent)
	}
}

func TestForeachWithSubSteps_ParallelPartialFailure_OthersComplete(t *testing.T) {
	// Item "b" fails immediately. With parallel mode, items a, c, d should still complete.
	var mu sync.Mutex
	completed := map[string]bool{}

	router := NewRouter()
	router.Register(v1alpha1.ActorNodeAgent, "node.slow_action", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		item, _ := req.Inputs["item"].(string)
		if item == "b" {
			return nil, fmt.Errorf("NATIVE_LIBRARY_DEPENDENCY_MISSING: libodbc.so.2 not found on node-b")
		}
		mu.Lock()
		completed[item] = true
		mu.Unlock()
		return &ActionResult{OK: true}, nil
	})

	eng := &Engine{Router: router}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	run, err := eng.Execute(ctx, buildParallelForeachDef(4), nil)

	// Should fail due to item "b".
	if err == nil {
		t.Fatal("expected error from partial failure")
	}
	if run.Status != RunFailed {
		t.Errorf("expected FAILED, got %s", run.Status)
	}

	// Items a, c, d should have completed despite b's failure.
	mu.Lock()
	defer mu.Unlock()
	for _, item := range []string{"a", "c", "d"} {
		if !completed[item] {
			t.Errorf("item %q did not complete — was blocked by item b's failure", item)
		}
	}
	t.Logf("Completed items: %v", completed)
}

func TestForeachWithSubSteps_ItemInputsAvailable(t *testing.T) {
	var receivedInputs []map[string]any
	var mu sync.Mutex

	router := NewRouter()

	// Use a raw compiled workflow to have full control.
	cw := &compiler.CompiledWorkflow{
		Name: "test-item-inputs",
		Steps: map[string]*compiler.CompiledStep{
			"per_node": {
				ID: "per_node",
				Foreach: &compiler.ValueExpr{
					Raw:    "$.nodes",
					IsExpr: true,
				},
				ItemName: "target",
				SubSteps: &compiler.CompiledWorkflow{
					Name: "per_node_sub",
					Steps: map[string]*compiler.CompiledStep{
						"check": {
							ID:     "check",
							Actor:  "cluster-controller",
							Action: "controller.check",
						},
					},
					TopoOrder: []string{"check"},
				},
			},
		},
		TopoOrder: []string{"per_node"},
	}

	router.Register(v1alpha1.ActorClusterController, "controller.check", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		mu.Lock()
		// Copy the inputs we care about.
		snap := map[string]any{
			"item":       req.Inputs["item"],
			"item_index": req.Inputs["item_index"],
			"target":     req.Inputs["target"],
			"node_id":    req.Inputs["node_id"],
		}
		receivedInputs = append(receivedInputs, snap)
		mu.Unlock()
		return &ActionResult{OK: true}, nil
	})

	eng := &Engine{Router: router}
	run, err := eng.ExecuteCompiled(context.Background(), cw, map[string]any{
		"nodes": []any{"node-x", "node-y"},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if run.Status != RunSucceeded {
		t.Errorf("expected SUCCEEDED, got %s (error: %s)", run.Status, run.Error)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(receivedInputs) != 2 {
		t.Fatalf("expected 2 invocations, got %d", len(receivedInputs))
	}

	// First item.
	if receivedInputs[0]["item"] != "node-x" {
		t.Errorf("item[0] = %v, want node-x", receivedInputs[0]["item"])
	}
	if receivedInputs[0]["target"] != "node-x" {
		t.Errorf("target[0] = %v, want node-x", receivedInputs[0]["target"])
	}
	if receivedInputs[0]["node_id"] != "node-x" {
		t.Errorf("node_id[0] = %v, want node-x", receivedInputs[0]["node_id"])
	}
	if receivedInputs[0]["item_index"] != 0 {
		t.Errorf("item_index[0] = %v, want 0", receivedInputs[0]["item_index"])
	}

	// Second item.
	if receivedInputs[1]["item"] != "node-y" {
		t.Errorf("item[1] = %v, want node-y", receivedInputs[1]["item"])
	}
	if receivedInputs[1]["item_index"] != 1 {
		t.Errorf("item_index[1] = %v, want 1", receivedInputs[1]["item_index"])
	}
}
