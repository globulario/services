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

func TestExecuteNodeBootstrap(t *testing.T) {
	loader := v1alpha1.NewLoader()
	def, err := loader.LoadFile("../definitions/node.bootstrap.yaml")
	if err != nil {
		t.Fatalf("load definition: %v", err)
	}

	// Track which actions were called and in what order.
	var mu = make(chan string, 50)
	record := func(action string) {
		mu <- action
	}

	router := NewRouter()

	// Register mock handlers for all actions in the definition.
	router.Register(v1alpha1.ActorClusterController, "controller.bootstrap.set_phase", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		phase := req.With["phase"]
		record(fmt.Sprintf("set_phase:%v", phase))
		return &ActionResult{OK: true, Output: map[string]any{"phase": phase}}, nil
	})

	router.Register(v1alpha1.ActorClusterController, "controller.bootstrap.wait_condition", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		cond := req.With["condition"]
		record(fmt.Sprintf("wait_condition:%v", cond))
		return &ActionResult{OK: true, Output: map[string]any{"condition": cond, "satisfied": true}}, nil
	})

	router.Register(v1alpha1.ActorClusterController, "controller.bootstrap.mark_failed", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		record("mark_failed")
		return &ActionResult{OK: true}, nil
	})

	router.Register(v1alpha1.ActorClusterController, "controller.bootstrap.emit_ready", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		record("emit_ready")
		return &ActionResult{OK: true}, nil
	})

	// Condition evaluator that understands "contains(inputs.node_profiles, 'X')".
	evalCond := func(ctx context.Context, expr string, inputs, outputs map[string]any) (bool, error) {
		if strings.HasPrefix(expr, "contains(inputs.node_profiles,") {
			profiles, ok := inputs["node_profiles"].([]any)
			if !ok {
				return false, nil
			}
			// Extract the value from contains(inputs.node_profiles, 'value')
			parts := strings.SplitN(expr, "'", 3)
			if len(parts) < 2 {
				return false, nil
			}
			target := parts[1]
			for _, p := range profiles {
				if fmt.Sprint(p) == target {
					return true, nil
				}
			}
			return false, nil
		}
		return true, nil
	}

	eng := &Engine{
		Router:   router,
		EvalCond: evalCond,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	run, err := eng.Execute(ctx, def, map[string]any{
		"cluster_id":    "test-cluster",
		"node_id":       "node-1",
		"node_hostname": "test-node",
		"node_profiles": []any{"control-plane", "gateway"},
	})

	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if run.Status != RunSucceeded {
		t.Errorf("expected SUCCEEDED, got %s (error: %s)", run.Status, run.Error)
	}

	// Collect actions.
	close(mu)
	var actions []string
	for a := range mu {
		actions = append(actions, a)
	}

	// Verify set_phase:infra_preparing was called (may run in parallel with
	// maybe_wait_etcd_unit since both have no dependencies).
	foundInfraPhase := false
	for _, a := range actions {
		if a == "set_phase:infra_preparing" {
			foundInfraPhase = true
		}
	}
	if !foundInfraPhase {
		t.Errorf("expected set_phase:infra_preparing in actions: %v", actions)
	}

	// Verify wait conditions were called (storage_verified is skipped because
	// control-plane+gateway profiles don't include minio/scylla/storage).
	expectedConditions := map[string]bool{
		"wait_condition:node_has_etcd_unit": false,
		"wait_condition:etcd_join_verified": false,
		"wait_condition:xds_active":         false,
		"wait_condition:envoy_active":       false,
	}
	for _, a := range actions {
		if _, ok := expectedConditions[a]; ok {
			expectedConditions[a] = true
		}
	}
	for cond, found := range expectedConditions {
		if !found {
			t.Errorf("expected condition %s to be called", cond)
		}
	}

	// Verify set_phase:workload_ready happened last (before hooks).
	found := false
	for _, a := range actions {
		if a == "set_phase:workload_ready" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected set_phase:workload_ready in actions: %v", actions)
	}

	// Verify onSuccess hook was called.
	foundEmit := false
	for _, a := range actions {
		if a == "emit_ready" {
			foundEmit = true
		}
	}
	if !foundEmit {
		t.Errorf("expected emit_ready hook to be called")
	}

	t.Logf("Actions executed: %v", actions)
	t.Logf("Steps: %d total", len(run.Steps))
	for id, st := range run.Steps {
		t.Logf("  %s: %s (attempts=%d)", id, st.Status, st.Attempt)
	}
}

func TestStepSkippedWhenConditionFalse(t *testing.T) {
	loader := v1alpha1.NewLoader()
	def, err := loader.LoadFile("../definitions/node.bootstrap.yaml")
	if err != nil {
		t.Fatalf("load definition: %v", err)
	}

	router := NewRouter()
	router.Register(v1alpha1.ActorClusterController, "controller.bootstrap.set_phase", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		return &ActionResult{OK: true}, nil
	})
	router.Register(v1alpha1.ActorClusterController, "controller.bootstrap.wait_condition", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		return &ActionResult{OK: true}, nil
	})
	router.Register(v1alpha1.ActorClusterController, "controller.bootstrap.emit_ready", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		return &ActionResult{OK: true}, nil
	})

	// No profiles → all conditional steps should be skipped.
	eng := &Engine{
		Router: router,
		EvalCond: func(ctx context.Context, expr string, inputs, outputs map[string]any) (bool, error) {
			return false, nil // all conditions false
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	run, err := eng.Execute(ctx, def, map[string]any{
		"cluster_id":    "test",
		"node_id":       "n1",
		"node_hostname": "h1",
		"node_profiles": []any{},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if run.Status != RunSucceeded {
		t.Errorf("expected SUCCEEDED, got %s", run.Status)
	}

	// All conditional steps should be SKIPPED.
	for id, st := range run.Steps {
		if strings.HasPrefix(id, "maybe_") && st.Status != StepSkipped {
			t.Errorf("step %s should be SKIPPED, got %s", id, st.Status)
		}
	}
}

func TestRetryOnFailure(t *testing.T) {
	// Simple inline definition with retry.
	def := &v1alpha1.WorkflowDefinition{
		APIVersion: v1alpha1.APIVersion,
		Kind:       v1alpha1.Kind,
		Metadata:   v1alpha1.WorkflowMetadata{Name: "test-retry"},
		Spec: v1alpha1.WorkflowDefinitionSpec{
			Strategy: v1alpha1.ExecutionStrategy{Mode: v1alpha1.StrategySingle},
			Steps: []v1alpha1.WorkflowStepSpec{
				{
					ID:     "flaky",
					Actor:  v1alpha1.ActorInstaller,
					Action: "installer.do_thing",
					Retry: &v1alpha1.RetryPolicy{
						MaxAttempts: 3,
						Backoff:     &v1alpha1.ScalarString{Raw: "10ms"},
					},
				},
			},
		},
	}

	attempts := 0
	router := NewRouter()
	router.Register(v1alpha1.ActorInstaller, "installer.do_thing", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		attempts++
		if attempts < 3 {
			return nil, fmt.Errorf("transient failure %d", attempts)
		}
		return &ActionResult{OK: true, Output: map[string]any{"attempts": attempts}}, nil
	})

	eng := &Engine{Router: router}
	ctx := context.Background()

	run, err := eng.Execute(ctx, def, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if run.Status != RunSucceeded {
		t.Errorf("expected SUCCEEDED, got %s", run.Status)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
	st := run.Steps["flaky"]
	if st.Attempt != 3 {
		t.Errorf("expected step attempt=3, got %d", st.Attempt)
	}
}

func TestParallelSteps(t *testing.T) {
	// Two steps with no dependencies should run in parallel.
	def := &v1alpha1.WorkflowDefinition{
		APIVersion: v1alpha1.APIVersion,
		Kind:       v1alpha1.Kind,
		Metadata:   v1alpha1.WorkflowMetadata{Name: "test-parallel"},
		Spec: v1alpha1.WorkflowDefinitionSpec{
			Strategy: v1alpha1.ExecutionStrategy{Mode: v1alpha1.StrategySingle},
			Steps: []v1alpha1.WorkflowStepSpec{
				{ID: "a", Actor: v1alpha1.ActorInstaller, Action: "installer.slow"},
				{ID: "b", Actor: v1alpha1.ActorInstaller, Action: "installer.slow"},
				{ID: "c", Actor: v1alpha1.ActorInstaller, Action: "installer.fast", DependsOn: []string{"a", "b"}},
			},
		},
	}

	var mu sync.Mutex
	running := 0
	maxConcurrent := 0

	router := NewRouter()
	router.Register(v1alpha1.ActorInstaller, "installer.slow", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		mu.Lock()
		running++
		if running > maxConcurrent {
			maxConcurrent = running
		}
		mu.Unlock()
		time.Sleep(50 * time.Millisecond)
		mu.Lock()
		running--
		mu.Unlock()
		return &ActionResult{OK: true}, nil
	})
	router.Register(v1alpha1.ActorInstaller, "installer.fast", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		return &ActionResult{OK: true}, nil
	})

	eng := &Engine{Router: router}
	run, err := eng.Execute(context.Background(), def, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if run.Status != RunSucceeded {
		t.Errorf("expected SUCCEEDED, got %s", run.Status)
	}
	if maxConcurrent < 2 {
		t.Errorf("expected parallel execution (maxConcurrent >= 2), got %d", maxConcurrent)
	}
}
