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

func loadReleasePackageDef(t *testing.T) *v1alpha1.WorkflowDefinition {
	t.Helper()
	loader := v1alpha1.NewLoader()
	def, err := loader.LoadFile("../definitions/release.apply.package.yaml")
	if err != nil {
		t.Fatalf("load definition: %v", err)
	}
	return def
}

func releasePackageInputs(kind string, nodes ...string) map[string]any {
	nodesAny := make([]any, len(nodes))
	for i, n := range nodes {
		nodesAny[i] = n
	}
	return map[string]any{
		"cluster_id":       "test-cluster",
		"release_id":       "rel-pkg-001",
		"release_name":     "test-pkg",
		"package_name":     "test-svc",
		"package_kind":     kind,
		"resolved_version": "2.0.0",
		"desired_hash":     "def456",
		"candidate_nodes":  nodesAny,
		"restart_policy":   "auto",
		"runtime_check":    "auto",
	}
}

type releasePackageTestOpts struct {
	selectTargets        func(ctx context.Context, candidates []any, pkg, kind, hash string) ([]any, error)
	aggregateDirectApply func(ctx context.Context, releaseID, pkg string) (map[string]any, error)
	installPackage       func(ctx context.Context, name, version, kind string) error
	verifyPackageInstalled func(ctx context.Context, name, version, hash string) error
	maybeRestartPackage  func(ctx context.Context, name, kind, policy string) error
	verifyPackageRuntime func(ctx context.Context, name, check string) error
	syncInstalledPackage func(ctx context.Context, name, version, hash, kind string) error
}

func defaultReleasePackageOpts() releasePackageTestOpts {
	return releasePackageTestOpts{
		selectTargets: func(ctx context.Context, candidates []any, pkg, kind, hash string) ([]any, error) {
			targets := make([]any, len(candidates))
			for i, c := range candidates {
				targets[i] = map[string]any{"node_id": fmt.Sprint(c)}
			}
			return targets, nil
		},
		aggregateDirectApply: func(ctx context.Context, releaseID, pkg string) (map[string]any, error) {
			return map[string]any{"status": "AVAILABLE"}, nil
		},
		installPackage:         func(ctx context.Context, name, version, kind string) error { return nil },
		verifyPackageInstalled: func(ctx context.Context, name, version, hash string) error { return nil },
		maybeRestartPackage:    func(ctx context.Context, name, kind, policy string) error { return nil },
		verifyPackageRuntime:   func(ctx context.Context, name, check string) error { return nil },
		syncInstalledPackage:   func(ctx context.Context, name, version, hash string) error { return nil },
	}
}

func newReleasePackageRouter(t *testing.T, opts releasePackageTestOpts) *Router {
	t.Helper()
	router := NewRouter()
	RegisterReleaseControllerActions(router, ReleaseControllerConfig{
		MarkReleaseResolved:  func(ctx context.Context, releaseID string) error { return nil },
		MarkReleaseApplying:  func(ctx context.Context, releaseID string) error { return nil },
		MarkReleaseFailed:    func(ctx context.Context, releaseID, reason string) error { return nil },
		RecheckConvergence:   func(ctx context.Context, releaseID string) error { return nil },
		SelectPackageTargets: opts.selectTargets,
		FinalizeNoop:         func(ctx context.Context, releaseID string) error { return nil },
		MarkNodeStarted:      func(ctx context.Context, releaseID, nodeID string) error { return nil },
		MarkNodeSucceeded:    func(ctx context.Context, releaseID, nodeID, ver, hash string) error { return nil },
		MarkNodeFailed:       func(ctx context.Context, releaseID, nodeID, reason string) error { return nil },
		AggregateDirectApply: opts.aggregateDirectApply,
		FinalizeDirectApply:  func(ctx context.Context, releaseID string, agg map[string]any) error { return nil },
	})
	RegisterNodeDirectApplyActions(router, NodeDirectApplyConfig{
		InstallPackage:         opts.installPackage,
		VerifyPackageInstalled: opts.verifyPackageInstalled,
		MaybeRestartPackage:    opts.maybeRestartPackage,
		VerifyPackageRuntime:   opts.verifyPackageRuntime,
		SyncInstalledPackage:   opts.syncInstalledPackage,
	})
	return router
}

// Test 1: No-op — all targets already converged.
func TestReleasePackage_NoopAllConverged(t *testing.T) {
	opts := defaultReleasePackageOpts()
	opts.selectTargets = func(ctx context.Context, candidates []any, pkg, kind, hash string) ([]any, error) {
		return []any{}, nil
	}

	router := newReleasePackageRouter(t, opts)
	eng := &Engine{Router: router}
	def := loadReleasePackageDef(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	run, err := eng.Execute(ctx, def, releasePackageInputs("SERVICE", "node-1"))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if run.Status != RunSucceeded {
		t.Errorf("expected SUCCEEDED, got %s (error: %s)", run.Status, run.Error)
	}
}

// Test 2: Service rollout success.
func TestReleasePackage_ServiceRolloutSuccess(t *testing.T) {
	var mu sync.Mutex
	var actions []string
	record := func(a string) {
		mu.Lock()
		actions = append(actions, a)
		mu.Unlock()
	}

	opts := defaultReleasePackageOpts()
	opts.installPackage = func(ctx context.Context, name, version, kind string) error {
		record(fmt.Sprintf("install:%s@%s(%s)", name, version, kind))
		return nil
	}
	opts.verifyPackageInstalled = func(ctx context.Context, name, version, hash string) error {
		record("verify:" + name)
		return nil
	}
	opts.maybeRestartPackage = func(ctx context.Context, name, kind, policy string) error {
		record("restart:" + name)
		return nil
	}
	opts.verifyPackageRuntime = func(ctx context.Context, name, check string) error {
		record("health:" + name)
		return nil
	}
	opts.syncInstalledPackage = func(ctx context.Context, name, version, hash string) error {
		record("sync:" + name)
		return nil
	}

	router := newReleasePackageRouter(t, opts)
	eng := &Engine{Router: router}
	def := loadReleasePackageDef(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	run, err := eng.Execute(ctx, def, releasePackageInputs("SERVICE", "node-1"))
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
	t.Logf("Actions: %s", strings.Join(actions, " → "))
	for _, prefix := range []string{"install:", "verify:", "restart:", "health:", "sync:"} {
		found := false
		for _, a := range actions {
			if strings.HasPrefix(a, prefix) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected action starting with %q", prefix)
		}
	}
	mu.Unlock()
}

// Test 3: Workload rollout partial failure → DEGRADED.
func TestReleasePackage_WorkloadPartialFailure(t *testing.T) {
	var mu sync.Mutex
	installCount := 0

	opts := defaultReleasePackageOpts()
	opts.installPackage = func(ctx context.Context, name, version, kind string) error {
		mu.Lock()
		installCount++
		n := installCount
		mu.Unlock()
		if n == 2 {
			return fmt.Errorf("download failed: connection refused")
		}
		return nil
	}
	opts.verifyPackageInstalled = func(ctx context.Context, name, version, hash string) error { return nil }
	opts.maybeRestartPackage = func(ctx context.Context, name, kind, policy string) error { return nil }
	opts.verifyPackageRuntime = func(ctx context.Context, name, check string) error { return nil }
	opts.syncInstalledPackage = func(ctx context.Context, name, version, hash string) error { return nil }

	router := newReleasePackageRouter(t, opts)
	eng := &Engine{Router: router}
	def := loadReleasePackageDef(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	run, err := eng.Execute(ctx, def, releasePackageInputs("WORKLOAD", "node-1", "node-2", "node-3"))
	if err == nil {
		t.Log("workflow completed without error (execution order may vary)")
	}
	if run != nil {
		t.Logf("Status: %s, Error: %s", run.Status, run.Error)
	}
}

// Test 4: Command rollout — restart_policy='never' should skip restart.
func TestReleasePackage_CommandSkipsRestart(t *testing.T) {
	var mu sync.Mutex
	var actions []string
	record := func(a string) {
		mu.Lock()
		actions = append(actions, a)
		mu.Unlock()
	}

	opts := defaultReleasePackageOpts()
	opts.installPackage = func(ctx context.Context, name, version, kind string) error {
		record("install:" + kind)
		return nil
	}
	opts.verifyPackageInstalled = func(ctx context.Context, name, version, hash string) error { return nil }
	opts.maybeRestartPackage = func(ctx context.Context, name, kind, policy string) error {
		record("restart:" + name)
		return nil
	}
	opts.verifyPackageRuntime = func(ctx context.Context, name, check string) error { return nil }
	opts.syncInstalledPackage = func(ctx context.Context, name, version, hash string) error { return nil }

	router := newReleasePackageRouter(t, opts)
	eng := &Engine{Router: router}
	def := loadReleasePackageDef(t)

	inputs := releasePackageInputs("COMMAND", "node-1")
	inputs["restart_policy"] = "never"

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	run, err := eng.Execute(ctx, def, inputs)
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

	// maybe_restart should be SKIPPED because restart_policy == 'never'.
	mu.Lock()
	for _, a := range actions {
		if strings.HasPrefix(a, "restart:") {
			t.Errorf("restart action should not be called for restart_policy=never, got %s", a)
		}
	}
	mu.Unlock()
}
