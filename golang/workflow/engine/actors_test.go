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
	skipped := 0
	for _, st := range run.Steps {
		if st.Status == StepSucceeded {
			succeeded++
		}
		if st.Status == StepSkipped {
			skipped++
		}
	}

	t.Logf("Workflow completed in %s", elapsed.Round(time.Millisecond))
	t.Logf("Steps: %d succeeded out of %d total", succeeded, len(run.Steps))
	t.Logf("Packages installed: %d", len(installLog))
	t.Logf("Max concurrent installs: %d", maxConcurrent)

	// Verify all non-conditional steps succeeded. Media steps are intentionally
	// skipped when node_profiles does not include media-server.
	if succeeded+skipped != len(run.Steps) {
		t.Errorf("expected all %d steps to succeed or skip, got succeeded=%d skipped=%d", len(run.Steps), succeeded, skipped)
	}
	for _, id := range []string{"install_media_workloads", "install_media_commands"} {
		if st := run.Steps[id]; st == nil || st.Status != StepSkipped {
			t.Errorf("expected %s to be skipped on non-media node, got %#v", id, st)
		}
	}
	if st := run.Steps["report_installed"]; st == nil || st.Status != StepSucceeded {
		t.Errorf("expected report_installed to succeed after skipped media steps, got %#v", st)
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

// TestControllerMarkFailed_SetsBootstrapFailed ensures that the
// controller.bootstrap.mark_failed action advances the node's bootstrap state
// to bootstrap_failed via SetBootstrapPhase. Without this call the node stays
// at whatever mid-join phase it occupied when the workflow failed (e.g.
// xds_ready) and neither the phase timeout nor the recovery mechanism can
// re-trigger the join workflow in time.
func TestControllerMarkFailed_SetsBootstrapFailed(t *testing.T) {
	var gotNodeID, gotPhase string
	handler := controllerMarkFailed(ControllerConfig{
		SetBootstrapPhase: func(_ context.Context, nodeID, phase string) error {
			gotNodeID = nodeID
			gotPhase = phase
			return nil
		},
		EmitEvent: func(_ context.Context, _ string, _ map[string]any) error { return nil },
	})

	if _, err := handler(context.Background(), ActionRequest{
		With:   map[string]any{"reason": "join_workflow_failed"},
		Inputs: map[string]any{"node_id": "n1"},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotNodeID != "n1" {
		t.Errorf("SetBootstrapPhase node_id: want n1, got %q", gotNodeID)
	}
	if gotPhase != "bootstrap_failed" {
		t.Errorf("SetBootstrapPhase phase: want bootstrap_failed, got %q", gotPhase)
	}
}
