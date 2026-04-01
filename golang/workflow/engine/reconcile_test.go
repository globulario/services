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

func loadReconcileDef(t *testing.T) *v1alpha1.WorkflowDefinition {
	t.Helper()
	loader := v1alpha1.NewLoader()
	def, err := loader.LoadFile("../definitions/cluster.reconcile.yaml")
	if err != nil {
		t.Fatalf("load definition: %v", err)
	}
	return def
}

func reconcileInputs() map[string]any {
	return map[string]any{
		"cluster_id":      "test-cluster",
		"scope":           "cluster",
		"max_remediations": 50,
	}
}

type reconcileTestOpts struct {
	driftItems      []any
	remediationItems []any
	chooseWorkflow  func(ctx context.Context, item map[string]any) (map[string]any, error)
	startChild      func(ctx context.Context, name string, inputs map[string]any) (string, error)
	waitChild       func(ctx context.Context, runID string) (map[string]any, error)
}

func newReconcileRouter(t *testing.T, opts reconcileTestOpts) *Router {
	t.Helper()
	router := NewRouter()

	var mu sync.Mutex
	var events []string
	record := func(e string) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	}

	RegisterReconcileControllerActions(router, ReconcileControllerConfig{
		ScanDrift: func(ctx context.Context, clusterID, scope string, includeNodes []any) ([]any, error) {
			record("scan_drift")
			return opts.driftItems, nil
		},
		ClassifyDrift: func(ctx context.Context, driftReport []any, maxRem int) ([]any, error) {
			record("classify_drift")
			return opts.remediationItems, nil
		},
		FinalizeClean: func(ctx context.Context, clusterID string) error {
			record("finalize_clean")
			return nil
		},
		MarkItemStarted: func(ctx context.Context, item map[string]any) error {
			record(fmt.Sprintf("item_started:%v", item["type"]))
			return nil
		},
		ChooseWorkflow: opts.chooseWorkflow,
		MarkItemTerminal: func(ctx context.Context, item, childResult map[string]any) error {
			record("item_terminal")
			return nil
		},
		MarkItemFailed: func(ctx context.Context, item map[string]any) error {
			record("item_failed")
			return nil
		},
		AggregateResults: func(ctx context.Context) (map[string]any, error) {
			record("aggregate")
			return map[string]any{"status": "ok"}, nil
		},
		Finalize: func(ctx context.Context, aggregate map[string]any) error {
			record("finalize")
			return nil
		},
		MarkFailed:    func(ctx context.Context) error { record("mark_failed"); return nil },
		EmitCompleted: func(ctx context.Context) error { record("emit_completed"); return nil },
	})

	RegisterWorkflowServiceActions(router, WorkflowServiceConfig{
		StartChild:        opts.startChild,
		WaitChildTerminal: opts.waitChild,
	})

	t.Cleanup(func() {
		mu.Lock()
		t.Logf("Reconcile events: %s", strings.Join(events, " → "))
		mu.Unlock()
	})

	return router
}

// Test 1: Clean cluster — no drift, finalize_clean path.
func TestReconcile_CleanCluster(t *testing.T) {
	opts := reconcileTestOpts{
		driftItems:       []any{},
		remediationItems: []any{},
		chooseWorkflow: func(ctx context.Context, item map[string]any) (map[string]any, error) {
			return nil, fmt.Errorf("should not be called")
		},
	}

	router := newReconcileRouter(t, opts)
	eng := &Engine{Router: router}
	def := loadReconcileDef(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	run, err := eng.Execute(ctx, def, reconcileInputs())
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

	// short_circuit_clean should have run.
	if st := run.Steps["short_circuit_clean"]; st != nil && st.Status != StepSucceeded {
		t.Errorf("short_circuit_clean should be SUCCEEDED, got %s", st.Status)
	}

	// dispatch_remediations should be SKIPPED.
	if st := run.Steps["dispatch_remediations"]; st != nil && st.Status != StepSkipped {
		t.Errorf("dispatch_remediations should be SKIPPED, got %s", st.Status)
	}
}

// Test 2: One missing package → launches release.apply.package.
func TestReconcile_OneMissingPackage(t *testing.T) {
	var mu sync.Mutex
	var childWorkflows []string

	opts := reconcileTestOpts{
		driftItems: []any{
			map[string]any{"type": "missing_package", "package": "dns", "node": "node-1"},
		},
		remediationItems: []any{
			map[string]any{"type": "missing_package", "package": "dns", "node": "node-1"},
		},
		chooseWorkflow: func(ctx context.Context, item map[string]any) (map[string]any, error) {
			return map[string]any{
				"workflow_name": "release.apply.package",
				"inputs": map[string]any{
					"package_name": item["package"],
					"node":         item["node"],
				},
			}, nil
		},
		startChild: func(ctx context.Context, name string, inputs map[string]any) (string, error) {
			mu.Lock()
			childWorkflows = append(childWorkflows, name)
			mu.Unlock()
			return "child-run-001", nil
		},
		waitChild: func(ctx context.Context, runID string) (map[string]any, error) {
			return map[string]any{"status": "SUCCEEDED", "run_id": runID}, nil
		},
	}

	router := newReconcileRouter(t, opts)
	eng := &Engine{Router: router}
	def := loadReconcileDef(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	run, err := eng.Execute(ctx, def, reconcileInputs())
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

	mu.Lock()
	if len(childWorkflows) != 1 {
		t.Errorf("expected 1 child workflow, got %d", len(childWorkflows))
	} else if childWorkflows[0] != "release.apply.package" {
		t.Errorf("expected release.apply.package, got %s", childWorkflows[0])
	}
	mu.Unlock()
}

// Test 3: One degraded node → launches node.repair.
func TestReconcile_OneDegradedNode(t *testing.T) {
	var mu sync.Mutex
	var childWorkflows []string

	opts := reconcileTestOpts{
		driftItems: []any{
			map[string]any{"type": "node_degraded", "node": "node-3"},
		},
		remediationItems: []any{
			map[string]any{"type": "node_degraded", "node": "node-3"},
		},
		chooseWorkflow: func(ctx context.Context, item map[string]any) (map[string]any, error) {
			return map[string]any{
				"workflow_name": "node.repair",
				"inputs": map[string]any{
					"node_id": item["node"],
					"reason":  "degraded",
				},
			}, nil
		},
		startChild: func(ctx context.Context, name string, inputs map[string]any) (string, error) {
			mu.Lock()
			childWorkflows = append(childWorkflows, name)
			mu.Unlock()
			return "child-run-002", nil
		},
		waitChild: func(ctx context.Context, runID string) (map[string]any, error) {
			return map[string]any{"status": "SUCCEEDED"}, nil
		},
	}

	router := newReconcileRouter(t, opts)
	eng := &Engine{Router: router}
	def := loadReconcileDef(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	run, err := eng.Execute(ctx, def, reconcileInputs())
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if run.Status != RunSucceeded {
		t.Errorf("expected SUCCEEDED, got %s (error: %s)", run.Status, run.Error)
	}

	mu.Lock()
	if len(childWorkflows) != 1 || childWorkflows[0] != "node.repair" {
		t.Errorf("expected [node.repair], got %v", childWorkflows)
	}
	mu.Unlock()
}

// Test 4: Mixed drift → multiple child workflows.
func TestReconcile_MixedDrift(t *testing.T) {
	var mu sync.Mutex
	var childWorkflows []string

	workflowMap := map[string]string{
		"missing_package": "release.apply.package",
		"wrong_version":   "release.apply.package",
		"node_degraded":   "node.repair",
	}

	opts := reconcileTestOpts{
		driftItems: []any{
			map[string]any{"type": "missing_package", "package": "dns"},
			map[string]any{"type": "wrong_version", "package": "envoy"},
			map[string]any{"type": "node_degraded", "node": "node-5"},
		},
		remediationItems: []any{
			map[string]any{"type": "missing_package", "package": "dns"},
			map[string]any{"type": "wrong_version", "package": "envoy"},
			map[string]any{"type": "node_degraded", "node": "node-5"},
		},
		chooseWorkflow: func(ctx context.Context, item map[string]any) (map[string]any, error) {
			driftType := fmt.Sprint(item["type"])
			wfName, ok := workflowMap[driftType]
			if !ok {
				return nil, fmt.Errorf("unknown drift type: %s", driftType)
			}
			return map[string]any{
				"workflow_name": wfName,
				"inputs":        item,
			}, nil
		},
		startChild: func(ctx context.Context, name string, inputs map[string]any) (string, error) {
			mu.Lock()
			childWorkflows = append(childWorkflows, name)
			mu.Unlock()
			return fmt.Sprintf("child-%d", len(childWorkflows)), nil
		},
		waitChild: func(ctx context.Context, runID string) (map[string]any, error) {
			return map[string]any{"status": "SUCCEEDED"}, nil
		},
	}

	router := newReconcileRouter(t, opts)
	eng := &Engine{Router: router}
	def := loadReconcileDef(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	run, err := eng.Execute(ctx, def, reconcileInputs())
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

	mu.Lock()
	if len(childWorkflows) != 3 {
		t.Errorf("expected 3 child workflows, got %d: %v", len(childWorkflows), childWorkflows)
	}
	// Count by type.
	releaseCt, repairCt := 0, 0
	for _, wf := range childWorkflows {
		switch wf {
		case "release.apply.package":
			releaseCt++
		case "node.repair":
			repairCt++
		}
	}
	if releaseCt != 2 {
		t.Errorf("expected 2 release.apply.package, got %d", releaseCt)
	}
	if repairCt != 1 {
		t.Errorf("expected 1 node.repair, got %d", repairCt)
	}
	mu.Unlock()

	t.Logf("Child workflows launched: %v", childWorkflows)
}
