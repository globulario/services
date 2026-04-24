package compiler

import (
	"context"
	"testing"
	"time"

	"github.com/globulario/services/golang/workflow/v1alpha1"
)

func TestCompileNodeBootstrap(t *testing.T) {
	loader := v1alpha1.NewLoader()
	def, err := loader.LoadFile("../definitions/node.bootstrap.yaml")
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	cw, diags, err := Compile(context.Background(), def)
	if err != nil {
		t.Fatalf("compile: %v (diags: %v)", err, diags)
	}
	if HasErrors(diags) {
		t.Fatalf("unexpected errors: %v", diags)
	}

	if cw.Name != "node.bootstrap" {
		t.Errorf("name = %q, want node.bootstrap", cw.Name)
	}
	if len(cw.Steps) != 7 {
		t.Errorf("steps = %d, want 7", len(cw.Steps))
	}
	if len(cw.TopoOrder) != 7 {
		t.Errorf("topo_order = %d, want 7", len(cw.TopoOrder))
	}
	if len(cw.EntryPoints) == 0 {
		t.Error("expected at least one entry point")
	}
	if cw.SourceHash == "" {
		t.Error("expected non-empty source hash")
	}

	// Verify mark_infra_preparing is an entry point (no dependencies).
	found := false
	for _, ep := range cw.EntryPoints {
		if ep == "mark_infra_preparing" {
			found = true
		}
	}
	if !found {
		t.Errorf("mark_infra_preparing should be an entry point, got %v", cw.EntryPoints)
	}

	// Verify maybe_wait_etcd_unit has a when condition.
	step := cw.Steps["maybe_wait_etcd_unit"]
	if step == nil {
		t.Fatal("step maybe_wait_etcd_unit not found")
	}
	if step.When == nil {
		t.Error("maybe_wait_etcd_unit should have a when condition")
	}

	// Verify retry is compiled.
	if step.Retry.MaxAttempts != 60 {
		t.Errorf("maybe_wait_etcd_unit retry = %d, want 60", step.Retry.MaxAttempts)
	}
	if step.Retry.Backoff != 5*time.Second {
		t.Errorf("maybe_wait_etcd_unit backoff = %v, want 5s", step.Retry.Backoff)
	}

	// Verify hooks compiled.
	if cw.OnFailure == nil {
		t.Error("expected onFailure hook")
	}
	if cw.OnSuccess == nil {
		t.Error("expected onSuccess hook")
	}

	t.Logf("Compiled %s: %d steps, %d entry points, topo: %v",
		cw.Name, len(cw.Steps), len(cw.EntryPoints), cw.TopoOrder)
}

func TestCompileNodeJoin(t *testing.T) {
	loader := v1alpha1.NewLoader()
	def, err := loader.LoadFile("../definitions/node.join.yaml")
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	cw, diags, err := Compile(context.Background(), def)
	if err != nil {
		t.Fatalf("compile: %v (diags: %v)", err, diags)
	}

	if cw.Name != "node.join" {
		t.Errorf("name = %q, want node.join", cw.Name)
	}
	if len(cw.Steps) < 10 {
		t.Errorf("expected 10+ steps, got %d", len(cw.Steps))
	}

	// verify_prerequisites should be entry point.
	found := false
	for _, ep := range cw.EntryPoints {
		if ep == "verify_prerequisites" {
			found = true
		}
	}
	if !found {
		t.Errorf("verify_prerequisites should be entry point, got %v", cw.EntryPoints)
	}

	// mark_converged depends on report_installed.
	mc := cw.Steps["mark_converged"]
	if mc == nil {
		t.Fatal("mark_converged not found")
	}
	foundDep := false
	for _, d := range mc.DependsOn {
		if d == "report_installed" {
			foundDep = true
		}
	}
	if !foundDep {
		t.Errorf("mark_converged should depend on report_installed, deps=%v", mc.DependsOn)
	}

	t.Logf("Compiled %s: %d steps, topo: %v", cw.Name, len(cw.Steps), cw.TopoOrder)
}

func TestCompileDay0Bootstrap(t *testing.T) {
	loader := v1alpha1.NewLoader()
	def, err := loader.LoadFile("../definitions/day0.bootstrap.yaml")
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	cw, diags, err := Compile(context.Background(), def)
	if err != nil {
		t.Fatalf("compile: %v (diags: %v)", err, diags)
	}

	if cw.Name != "day0.bootstrap" {
		t.Errorf("name = %q, want day0.bootstrap", cw.Name)
	}
	if len(cw.Steps) != 22 {
		t.Errorf("steps = %d, want 22", len(cw.Steps))
	}

	// enable_bootstrap_window has no dependencies → entry point.
	found := false
	for _, ep := range cw.EntryPoints {
		if ep == "enable_bootstrap_window" {
			found = true
		}
	}
	if !found {
		t.Errorf("enable_bootstrap_window should be entry point, got %v", cw.EntryPoints)
	}

	t.Logf("Compiled %s: %d steps, %d entry points", cw.Name, len(cw.Steps), len(cw.EntryPoints))
}

func TestCompileReleaseInfra(t *testing.T) {
	loader := v1alpha1.NewLoader()
	def, err := loader.LoadFile("../definitions/release.apply.infrastructure.yaml")
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	cw, diags, err := Compile(context.Background(), def)
	if err != nil {
		t.Fatalf("compile: %v (diags: %v)", err, diags)
	}

	if cw.Name != "release.apply.infrastructure" {
		t.Errorf("name = %q, want release.apply.infrastructure", cw.Name)
	}
	// New YAML uses strategy: single (foreach is per-step, not per-workflow).
	if cw.Strategy.Mode != "single" {
		t.Errorf("strategy mode = %q, want single", cw.Strategy.Mode)
	}

	// apply_per_node has foreach + nested sub-steps.
	apn := cw.Steps["apply_per_node"]
	if apn == nil {
		t.Fatal("apply_per_node not found")
	}
	if apn.Foreach == nil {
		t.Error("apply_per_node should have foreach")
	}
	if apn.SubSteps == nil {
		t.Fatal("apply_per_node should have sub-steps")
	}
	if len(apn.SubSteps.Steps) != 7 {
		t.Errorf("apply_per_node sub-steps = %d, want 7", len(apn.SubSteps.Steps))
	}
	// Verify sub-DAG has topo order.
	if len(apn.SubSteps.TopoOrder) != 7 {
		t.Errorf("sub-steps topo order = %d, want 7", len(apn.SubSteps.TopoOrder))
	}

	t.Logf("Compiled %s: %d top-level steps, apply_per_node has %d sub-steps",
		cw.Name, len(cw.Steps), len(apn.SubSteps.Steps))
}

func TestCompileValidationErrors(t *testing.T) {
	// Missing name.
	def := &v1alpha1.WorkflowDefinition{
		APIVersion: v1alpha1.APIVersion,
		Kind:       v1alpha1.Kind,
		Spec: v1alpha1.WorkflowDefinitionSpec{
			Steps: []v1alpha1.WorkflowStepSpec{
				{ID: "a", Actor: "installer", Action: "do"},
				{ID: "a", Actor: "installer", Action: "do"}, // duplicate
			},
		},
	}

	_, diags, err := Compile(context.Background(), def)
	if err == nil {
		t.Fatal("expected error for invalid definition")
	}
	if !HasErrors(diags) {
		t.Fatal("expected error diagnostics")
	}

	foundDup := false
	foundName := false
	for _, d := range diags {
		if d.Code == "duplicate_id" {
			foundDup = true
		}
		if d.Code == "required" && d.Path == "metadata.name" {
			foundName = true
		}
	}
	if !foundDup {
		t.Error("expected duplicate_id diagnostic")
	}
	if !foundName {
		t.Error("expected missing name diagnostic")
	}
}

func TestCompileCycleDetection(t *testing.T) {
	def := &v1alpha1.WorkflowDefinition{
		APIVersion: v1alpha1.APIVersion,
		Kind:       v1alpha1.Kind,
		Metadata:   v1alpha1.WorkflowMetadata{Name: "cycle-test"},
		Spec: v1alpha1.WorkflowDefinitionSpec{
			Steps: []v1alpha1.WorkflowStepSpec{
				{ID: "a", Actor: "x", Action: "do", DependsOn: []string{"b"}},
				{ID: "b", Actor: "x", Action: "do", DependsOn: []string{"a"}},
			},
		},
	}

	_, diags, err := Compile(context.Background(), def)
	if err == nil {
		t.Fatal("expected error for cycle")
	}
	foundCycle := false
	for _, d := range diags {
		if d.Code == "cycle_detected" {
			foundCycle = true
		}
	}
	if !foundCycle {
		t.Errorf("expected cycle_detected diagnostic, got %v", diags)
	}
}

func TestMustCompilePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic from MustCompile with nil def")
		}
	}()
	MustCompile(nil)
}

func TestSourceHashDeterministic(t *testing.T) {
	loader := v1alpha1.NewLoader()
	def, _ := loader.LoadFile("../definitions/node.bootstrap.yaml")

	cw1, _, _ := Compile(context.Background(), def)
	cw2, _, _ := Compile(context.Background(), def)

	if cw1.SourceHash != cw2.SourceHash {
		t.Errorf("hash not deterministic: %s != %s", cw1.SourceHash, cw2.SourceHash)
	}
}
