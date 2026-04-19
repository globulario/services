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

func TestNodeJoinWorkflow(t *testing.T) {
	loader := v1alpha1.NewLoader()
	def, err := loader.LoadFile("../definitions/node.join.yaml")
	if err != nil {
		t.Fatalf("load definition: %v", err)
	}

	// Track installation order and parallelism.
	var mu sync.Mutex
	var installLog []string
	running := 0
	maxConcurrent := 0

	trackStart := func(name string) {
		mu.Lock()
		running++
		if running > maxConcurrent {
			maxConcurrent = running
		}
		mu.Unlock()
	}
	trackEnd := func(name string) {
		mu.Lock()
		running--
		installLog = append(installLog, name)
		mu.Unlock()
	}

	router := NewRouter()

	// node-agent actions with tracking.
	RegisterNodeAgentActions(router, NodeAgentConfig{
		FetchAndInstall: func(ctx context.Context, pkg PackageRef) error {
			trackStart(pkg.Name)
			time.Sleep(5 * time.Millisecond) // simulate install
			trackEnd(pkg.Name)
			return nil
		},
		IsServiceActive: func(name string) bool {
			return true // all pre-reqs active
		},
		SyncInstalledState: func(ctx context.Context) error {
			return nil
		},
		ProbeInfraHealth: func(ctx context.Context, probeName string) bool {
			return true // all infra healthy in test
		},
		NodeID: "node-1",
	})

	// controller actions.
	RegisterControllerActions(router, ControllerConfig{
		SetBootstrapPhase: func(ctx context.Context, nodeID, phase string) error {
			t.Logf("phase: %s → %s", nodeID, phase)
			return nil
		},
		EmitEvent: func(ctx context.Context, eventType string, data map[string]any) error {
			t.Logf("event: %s %v", eventType, data)
			return nil
		},
	})
	RegisterNodeVerificationActions(router, NodeVerificationConfig{})
	RegisterControllerVerificationActions(router, ControllerVerificationConfig{})

	eng := &Engine{
		Router: router,
		OnStepDone: func(run *Run, step *StepState) {
			t.Logf("step %s: %s (%s)", step.ID, step.Status, step.FinishedAt.Sub(step.StartedAt).Round(time.Millisecond))
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	run, err := eng.Execute(ctx, def, map[string]any{
		"cluster_id":    "test-cluster",
		"node_id":       "node-1",
		"node_hostname": "test-node",
		"node_ip":       "10.0.0.20",
	})
	elapsed := time.Since(start)

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

	// Count results.
	succeeded := 0
	for _, st := range run.Steps {
		if st.Status == StepSucceeded {
			succeeded++
		}
	}

	t.Logf("Workflow completed in %s", elapsed.Round(time.Millisecond))
	t.Logf("Steps: %d succeeded out of %d total", succeeded, len(run.Steps))
	t.Logf("Packages installed: %d", len(installLog))
	t.Logf("Max concurrent installs: %d", maxConcurrent)

	// Verify all steps succeeded.
	if succeeded != len(run.Steps) {
		t.Errorf("expected all %d steps to succeed, got %d", len(run.Steps), succeeded)
	}

	// Verify parallelism happened (tier 3 has 8 foundational services).
	if maxConcurrent < 2 {
		t.Errorf("expected parallel installs (maxConcurrent >= 2), got %d", maxConcurrent)
	}

	// Verify dependency ordering: envoy before gateway, scylladb before foundational.
	envoyIdx := -1
	gatewayIdx := -1
	scyllaIdx := -1
	dnsIdx := -1
	for i, name := range installLog {
		switch name {
		case "envoy":
			envoyIdx = i
		case "gateway":
			gatewayIdx = i
		case "scylladb":
			scyllaIdx = i
		case "dns":
			dnsIdx = i
		}
	}
	if envoyIdx >= 0 && gatewayIdx >= 0 && envoyIdx >= gatewayIdx {
		t.Errorf("envoy (idx=%d) should install before gateway (idx=%d)", envoyIdx, gatewayIdx)
	}
	if scyllaIdx >= 0 && dnsIdx >= 0 && scyllaIdx >= dnsIdx {
		t.Errorf("scylladb (idx=%d) should install before dns (idx=%d)", scyllaIdx, dnsIdx)
	}

	// Print the install order for visual inspection.
	t.Logf("Install order: %s", strings.Join(installLog, " → "))
}

func TestNodeJoinWithInstallFailure(t *testing.T) {
	loader := v1alpha1.NewLoader()
	def, err := loader.LoadFile("../definitions/node.join.yaml")
	if err != nil {
		t.Fatalf("load definition: %v", err)
	}

	router := NewRouter()
	RegisterNodeAgentActions(router, NodeAgentConfig{
		FetchAndInstall: func(ctx context.Context, pkg PackageRef) error {
			if pkg.Name == "scylladb" {
				return fmt.Errorf("scylla-server.service is masked")
			}
			return nil
		},
		IsServiceActive: func(name string) bool { return true },
		NodeID:          "node-1",
	})
	RegisterControllerActions(router, ControllerConfig{})
	RegisterNodeVerificationActions(router, NodeVerificationConfig{})
	RegisterControllerVerificationActions(router, ControllerVerificationConfig{})

	eng := &Engine{Router: router}
	run, err := eng.Execute(context.Background(), def, map[string]any{
		"cluster_id":    "test",
		"node_id":       "n1",
		"node_hostname": "h1",
		"node_ip":       "10.0.0.20",
	})

	// Should fail because ScyllaDB install fails.
	if err == nil {
		t.Fatal("expected error from ScyllaDB failure")
	}
	if run.Status != RunFailed {
		t.Errorf("expected FAILED, got %s", run.Status)
	}
	if !strings.Contains(err.Error(), "scylladb") {
		t.Errorf("expected error to mention scylladb, got: %v", err)
	}

	// Steps after scylladb should not have run.
	for _, id := range []string{"install_foundational", "install_workloads", "mark_converged"} {
		st := run.Steps[id]
		if st.Status != StepPending {
			t.Errorf("step %s should be PENDING (blocked by scylladb), got %s", id, st.Status)
		}
	}
}
