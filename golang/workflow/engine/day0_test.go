package engine

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/globulario/services/golang/workflow/v1alpha1"
)

func TestDay0BootstrapWorkflow(t *testing.T) {
	loader := v1alpha1.NewLoader()
	def, err := loader.LoadFile("../definitions/day0.bootstrap.yaml")
	if err != nil {
		t.Fatalf("load definition: %v", err)
	}

	var steps []string
	record := func(name string) {
		steps = append(steps, name)
	}

	router := NewRouter()

	// Register node-agent actions (needed for infra health probes).
	RegisterNodeAgentActions(router, NodeAgentConfig{
		ProbeInfraHealth: func(ctx context.Context, probeName string) bool { return true },
	})

	// Register installer actions.
	RegisterInstallerActions(router, InstallerConfig{
		SetupTLS: func(ctx context.Context, clusterID string) error {
			record("setup_tls")
			return nil
		},
		EnableBootstrapWindow: func(ctx context.Context, ttl time.Duration) error {
			record("enable_bootstrap_window")
			return nil
		},
		DisableBootstrapWindow: func(ctx context.Context) error {
			record("disable_bootstrap_window")
			return nil
		},
		WriteBootstrapCreds: func(ctx context.Context) error {
			record("write_bootstrap_credentials")
			return nil
		},
		InstallPackage: func(ctx context.Context, name string) error {
			record(fmt.Sprintf("install:%s", name))
			return nil
		},
		InstallPackageSet: func(ctx context.Context, packages []string) error {
			record(fmt.Sprintf("install_set:%s", strings.Join(packages, ",")))
			return nil
		},
		InstallProfileSets: func(ctx context.Context, profiles []string) error {
			record(fmt.Sprintf("install_profiles:%s", strings.Join(profiles, ",")))
			return nil
		},
		ConfigureSharedStorage: func(ctx context.Context) error {
			record("configure_shared_storage")
			return nil
		},
		BootstrapDNS: func(ctx context.Context, domain string) error {
			record(fmt.Sprintf("bootstrap_dns:%s", domain))
			return nil
		},
		ValidateClusterHealth: func(ctx context.Context) error {
			record("validate_cluster_health")
			return nil
		},
		GenerateJoinToken: func(ctx context.Context) (string, error) {
			record("generate_join_token")
			return "test-token", nil
		},
		RestartServices: func(ctx context.Context, services []string) error {
			record(fmt.Sprintf("restart:%s", strings.Join(services, ",")))
			return nil
		},
		ClusterBootstrap: func(ctx context.Context, clusterID, nodeID string) error {
			record("cluster_bootstrap")
			return nil
		},
		CaptureFailureBundle: func(ctx context.Context, runID string) error {
			record("capture_failure_bundle")
			return nil
		},
	})

	// Register repository actions.
	RegisterRepositoryActions(router, RepositoryConfig{
		PublishBootstrapArtifacts: func(ctx context.Context, source string) error {
			record(fmt.Sprintf("publish_artifacts:%s", source))
			return nil
		},
	})

	// Register release controller actions (Day-0 extras).
	RegisterReleaseControllerActions(router, ReleaseControllerConfig{
		SeedDesiredFromInstalled: func(ctx context.Context, clusterID string) error {
			record("seed_desired")
			return nil
		},
		ReconcileUntilStable: func(ctx context.Context, clusterID string) error {
			record("reconcile_until_stable")
			return nil
		},
		EmitBootstrapSucceeded: func(ctx context.Context, clusterID string) error {
			record("emit_bootstrap_succeeded")
			return nil
		},
	})

	eng := &Engine{
		Router: router,
		OnStepDone: func(run *Run, step *StepState) {
			t.Logf("step %s: %s", step.ID, step.Status)
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	run, err := eng.Execute(ctx, def, map[string]any{
		"cluster_id":              "globular.internal",
		"bootstrap_node_id":      "node-0",
		"bootstrap_node_hostname": "bootstrap-host",
		"repository_address":     "10.0.0.1:443",
		"domain":                 "globular.internal",
	})

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
	t.Logf("Steps: %d succeeded out of %d total", succeeded, len(run.Steps))
	t.Logf("Actions: %s", strings.Join(steps, " → "))

	// Verify ordering where dependencies exist in the YAML.
	// Note: steps with no dependsOn run in parallel, so we only assert
	// ordering between steps that have explicit dependency chains.
	assertBefore(t, steps, "configure_shared_storage", "install:persistence")
	assertBefore(t, steps, "write_bootstrap_credentials", "install:persistence")
	assertBefore(t, steps, "install:persistence", "install_set:xds,envoy,gateway")
	assertBefore(t, steps, "bootstrap_dns:globular.internal", "validate_cluster_health")
	assertBefore(t, steps, "validate_cluster_health", "generate_join_token")
	assertBefore(t, steps, "cluster_bootstrap", "publish_artifacts:local-bootstrap-cache")
	assertBefore(t, steps, "seed_desired", "reconcile_until_stable")
}

func TestForeachStep(t *testing.T) {
	def := &v1alpha1.WorkflowDefinition{
		APIVersion: v1alpha1.APIVersion,
		Kind:       v1alpha1.Kind,
		Metadata:   v1alpha1.WorkflowMetadata{Name: "test-foreach"},
		Spec: v1alpha1.WorkflowDefinitionSpec{
			Strategy: v1alpha1.ExecutionStrategy{Mode: v1alpha1.StrategySingle},
			Defaults: map[string]any{
				"nodes": []any{"node-a", "node-b", "node-c"},
			},
			Steps: []v1alpha1.WorkflowStepSpec{
				{
					ID:      "process_each",
					Actor:   v1alpha1.ActorClusterController,
					Action:  "controller.process_node",
					Foreach: &v1alpha1.ScalarString{Raw: "$.nodes"},
					Export:  &v1alpha1.ScalarString{Raw: "processed"},
				},
				{
					ID:        "aggregate",
					Actor:     v1alpha1.ActorClusterController,
					Action:    "controller.aggregate",
					DependsOn: []string{"process_each"},
				},
			},
		},
	}

	var processedNodes []string
	router := NewRouter()
	router.Register(v1alpha1.ActorClusterController, "controller.process_node", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.Inputs["node_id"])
		processedNodes = append(processedNodes, nodeID)
		return &ActionResult{OK: true, Output: map[string]any{"node_id": nodeID, "ok": true}}, nil
	})
	router.Register(v1alpha1.ActorClusterController, "controller.aggregate", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		// Verify processed results are available in outputs.
		processed, _ := req.Outputs["processed"].([]any)
		if len(processed) != 3 {
			return nil, fmt.Errorf("expected 3 processed results, got %d", len(processed))
		}
		return &ActionResult{OK: true}, nil
	})

	eng := &Engine{Router: router}
	run, err := eng.Execute(context.Background(), def, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if run.Status != RunSucceeded {
		t.Errorf("expected SUCCEEDED, got %s (error: %s)", run.Status, run.Error)
	}
	if len(processedNodes) != 3 {
		t.Errorf("expected 3 processed nodes, got %d: %v", len(processedNodes), processedNodes)
	}
	// Verify exported results.
	processed, ok := run.Outputs["processed"].([]any)
	if !ok || len(processed) != 3 {
		t.Errorf("expected 3 exported results, got %v", run.Outputs["processed"])
	}
}

// assertBefore checks that action a appears before action b in the step log.
func assertBefore(t *testing.T, steps []string, a, b string) {
	t.Helper()
	idxA, idxB := -1, -1
	for i, s := range steps {
		if s == a && idxA < 0 {
			idxA = i
		}
		if s == b && idxB < 0 {
			idxB = i
		}
	}
	if idxA < 0 {
		t.Errorf("expected action %q in steps", a)
		return
	}
	if idxB < 0 {
		t.Errorf("expected action %q in steps", b)
		return
	}
	if idxA >= idxB {
		t.Errorf("expected %q (idx=%d) before %q (idx=%d)", a, idxA, b, idxB)
	}
}
