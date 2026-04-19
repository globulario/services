package adapter

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/v1alpha1"
)

func TestRemoteHandlerSuccess(t *testing.T) {
	// Set up a local node-agent with a mock install handler.
	nodeRouter := engine.NewRouter()
	nodeRouter.Register(v1alpha1.ActorNodeAgent, "node.install_packages", func(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
		return &engine.ActionResult{
			OK:      true,
			Message: "installed 3 packages",
			Output:  map[string]any{"installed": 3},
		}, nil
	})

	executor := NewStepExecutor(nodeRouter, "node-1")
	transport := NewMemoryTransport()
	transport.RegisterNode("node-1", executor)

	// Create remote handler and invoke it like the engine would.
	rh := NewRemoteHandler(transport, "node-1")

	result, err := rh.Handle(context.Background(), engine.ActionRequest{
		RunID:  "run-123",
		StepID: "install_mesh",
		Actor:  v1alpha1.ActorNodeAgent,
		Action: "node.install_packages",
		With:   map[string]any{"packages": []any{"etcd", "minio", "xds"}},
		Inputs: map[string]any{"node_id": "node-1"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.OK {
		t.Errorf("expected OK, got: %s", result.Message)
	}
	if result.Output["installed"] != 3 {
		t.Errorf("expected 3 installed, got %v", result.Output["installed"])
	}
}

func TestRemoteHandlerFailure(t *testing.T) {
	nodeRouter := engine.NewRouter()
	nodeRouter.Register(v1alpha1.ActorNodeAgent, "node.install_packages", func(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
		return nil, fmt.Errorf("download failed: connection refused")
	})

	executor := NewStepExecutor(nodeRouter, "node-1")
	transport := NewMemoryTransport()
	transport.RegisterNode("node-1", executor)

	rh := NewRemoteHandler(transport, "node-1")
	result, err := rh.Handle(context.Background(), engine.ActionRequest{
		RunID:  "run-456",
		StepID: "install_mesh",
		Actor:  v1alpha1.ActorNodeAgent,
		Action: "node.install_packages",
		Inputs: map[string]any{"node_id": "node-1"},
	})

	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if result.OK {
		t.Error("expected failure")
	}
	if !strings.Contains(result.Message, "DOWNLOAD_FAILED") {
		t.Errorf("expected DOWNLOAD_FAILED classification, got: %s", result.Message)
	}
}

func TestRemoteHandlerNoHandler(t *testing.T) {
	// Node has no handlers registered.
	nodeRouter := engine.NewRouter()
	executor := NewStepExecutor(nodeRouter, "node-1")
	transport := NewMemoryTransport()
	transport.RegisterNode("node-1", executor)

	rh := NewRemoteHandler(transport, "node-1")
	result, err := rh.Handle(context.Background(), engine.ActionRequest{
		RunID:  "run-789",
		StepID: "unknown_step",
		Actor:  v1alpha1.ActorNodeAgent,
		Action: "node.nonexistent_action",
		Inputs: map[string]any{"node_id": "node-1"},
	})

	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if result.OK {
		t.Error("expected failure for missing handler")
	}
	if !strings.Contains(result.Message, "INVALID_INPUT") {
		t.Errorf("expected INVALID_INPUT, got: %s", result.Message)
	}
}

func TestRemoteHandlerNodeNotFound(t *testing.T) {
	transport := NewMemoryTransport()
	rh := NewRemoteHandler(transport, "missing-node")

	_, err := rh.Handle(context.Background(), engine.ActionRequest{
		RunID:  "run-abc",
		StepID: "step-1",
		Actor:  v1alpha1.ActorNodeAgent,
		Action: "node.install_packages",
		Inputs: map[string]any{"node_id": "missing-node"},
	})

	if err == nil {
		t.Fatal("expected error for missing node")
	}
	if !strings.Contains(err.Error(), "missing-node") {
		t.Errorf("expected error to mention node, got: %v", err)
	}
}

func TestRemoteHandlerOverridesNodeID(t *testing.T) {
	// Handler is created for node-1, but inputs specify node-2.
	nodeRouter := engine.NewRouter()
	nodeRouter.Register(v1alpha1.ActorNodeAgent, "node.verify_services_active", func(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
		return &engine.ActionResult{OK: true}, nil
	})

	executor := NewStepExecutor(nodeRouter, "node-2")
	transport := NewMemoryTransport()
	transport.RegisterNode("node-2", executor)

	rh := NewRemoteHandler(transport, "node-1") // default is node-1

	result, err := rh.Handle(context.Background(), engine.ActionRequest{
		RunID:  "run-def",
		StepID: "verify",
		Actor:  v1alpha1.ActorNodeAgent,
		Action: "node.verify_services_active",
		Inputs: map[string]any{"node_id": "node-2"}, // override
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.OK {
		t.Errorf("expected OK, got: %s", result.Message)
	}
}

func TestExecutorCancel(t *testing.T) {
	nodeRouter := engine.NewRouter()
	nodeRouter.Register(v1alpha1.ActorNodeAgent, "node.install_packages", func(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
		// Simulate long-running install.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(10 * time.Second):
			return &engine.ActionResult{OK: true}, nil
		}
	})

	executor := NewStepExecutor(nodeRouter, "node-1")

	// Start execution in background.
	var result *ResultEvent
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		result = executor.Execute(context.Background(), ExecuteStepRequest{
			Identity: ExecutionIdentity{
				RunID:   "run-cancel",
				StepID:  "slow_install",
				Attempt: 1,
				NodeID:  "node-1",
			},
			Actor:  "node-agent",
			Action: "node.install_packages",
			Inputs: map[string]any{},
		})
	}()

	// Give it a moment to start.
	time.Sleep(50 * time.Millisecond)

	// Cancel.
	ok := executor.Cancel(CancelStepRequest{
		Identity: ExecutionIdentity{
			RunID:   "run-cancel",
			StepID:  "slow_install",
			Attempt: 1,
		},
		Reason: "user requested",
	})
	if !ok {
		t.Error("expected cancel to succeed")
	}

	wg.Wait()

	if result == nil {
		t.Fatal("expected result")
	}
	if result.Status != StatusFailed {
		t.Errorf("expected FAILED, got %s", result.Status)
	}
	if result.Error == nil || result.Error.ErrorClass != ErrCancelled {
		t.Errorf("expected CANCELLED error class, got %v", result.Error)
	}
}

func TestEndToEndWithEngine(t *testing.T) {
	// Full integration: engine dispatches to remote node-agent via adapter.
	nodeRouter := engine.NewRouter()

	var installLog []string
	var mu sync.Mutex
	nodeRouter.Register(v1alpha1.ActorNodeAgent, "node.install_packages", func(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
		pkgs, _ := req.With["packages"].([]any)
		for _, p := range pkgs {
			mu.Lock()
			installLog = append(installLog, fmt.Sprint(p))
			mu.Unlock()
		}
		return &engine.ActionResult{OK: true, Output: map[string]any{"installed": len(pkgs)}}, nil
	})
	nodeRouter.Register(v1alpha1.ActorNodeAgent, "node.verify_services_active", func(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
		return &engine.ActionResult{OK: true}, nil
	})
	nodeRouter.Register(v1alpha1.ActorNodeAgent, "node.sync_installed_state", func(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
		return &engine.ActionResult{OK: true}, nil
	})
	nodeRouter.Register(v1alpha1.ActorNodeAgent, "node.probe_infra_health", func(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
		return &engine.ActionResult{OK: true}, nil
	})
	engine.RegisterNodeVerificationActions(nodeRouter, engine.NodeVerificationConfig{})

	executor := NewStepExecutor(nodeRouter, "node-1")
	transport := NewMemoryTransport()
	transport.RegisterNode("node-1", executor)

	// Engine router uses remote handlers for all node-agent actions.
	engineRouter := engine.NewRouter()
	RegisterRemoteNodeAgent(engineRouter, transport, "node-1")

	// Also register controller actions locally (they run on controller).
	engineRouter.Register(v1alpha1.ActorClusterController, "controller.bootstrap.set_phase", func(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
		return &engine.ActionResult{OK: true}, nil
	})
	engineRouter.Register(v1alpha1.ActorClusterController, "controller.bootstrap.mark_failed", func(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
		return &engine.ActionResult{OK: true}, nil
	})
	engineRouter.Register(v1alpha1.ActorClusterController, "controller.bootstrap.emit_ready", func(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
		return &engine.ActionResult{OK: true}, nil
	})
	engineRouter.Register(v1alpha1.ActorClusterController, "controller.bootstrap.wait_condition", func(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
		return &engine.ActionResult{OK: true}, nil
	})
	engine.RegisterControllerVerificationActions(engineRouter, engine.ControllerVerificationConfig{})

	// Load and execute a real workflow definition.
	loader := v1alpha1.NewLoader()
	def, err := loader.LoadFile("../definitions/node.join.yaml")
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	eng := &engine.Engine{Router: engineRouter}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	run, err := eng.Execute(ctx, def, map[string]any{
		"cluster_id":    "test",
		"node_id":       "node-1",
		"node_hostname": "test-host",
		"node_ip":       "10.0.0.1",
	})

	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if run.Status != engine.RunSucceeded {
		t.Errorf("expected SUCCEEDED, got %s (error: %s)", run.Status, run.Error)
		for id, st := range run.Steps {
			if st.Status == engine.StepFailed {
				t.Errorf("  step %s FAILED: %s", id, st.Error)
			}
		}
	}

	mu.Lock()
	count := len(installLog)
	mu.Unlock()
	if count == 0 {
		t.Error("expected packages to be installed via remote handler")
	}

	t.Logf("Installed %d packages via remote adapter: %s",
		count, strings.Join(installLog, ", "))
}

func TestErrorClassification(t *testing.T) {
	tests := []struct {
		err      error
		expected StepErrorClass
	}{
		{context.DeadlineExceeded, ErrTimeout},
		{context.Canceled, ErrCancelled},
		{fmt.Errorf("file not found"), ErrNotFound},
		{fmt.Errorf("permission denied"), ErrPermissionDenied},
		{fmt.Errorf("download failed: 503"), ErrDownloadFailed},
		{fmt.Errorf("checksum mismatch"), ErrVerifyFailed},
		{fmt.Errorf("transient network error"), ErrTransient},
		{fmt.Errorf("something unexpected"), ErrInternal},
	}

	for _, tt := range tests {
		got := classifyError(tt.err)
		if got != tt.expected {
			t.Errorf("classifyError(%q) = %s, want %s", tt.err, got, tt.expected)
		}
	}
}
