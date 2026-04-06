package engine

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// TestEmbeddedWorkflowsHaveRegisteredActions asserts that every
// (actor, action) pair referenced by the workflow YAML files shipped
// in this repo has a Go handler registered with the engine Router.
//
// The goal is to catch a whole class of bugs at test time instead of
// at runtime:
//
//   - renamed an action in Go? the YAML still references the old name
//     → this test fails.
//   - added a new step in YAML? forgot to register the handler
//     → this test fails.
//   - changed actor constant? mismatched actor/action key
//     → this test fails.
//
// Without this, the first sign of trouble is a workflow run failing
// in production with "no handler for actor=X action=Y". That is much
// too late — connective-tissue bugs should be compile/test-time bugs.
func TestEmbeddedWorkflowsHaveRegisteredActions(t *testing.T) {
	// Locate YAMLs relative to this test file. We intentionally do NOT
	// go:embed them here: this test proves the on-disk YAMLs and the
	// in-binary Go handlers agree.
	yamlPaths := []string{
		// workflow/definitions/
		mustResolveYAML(t, "../definitions/cluster.reconcile.yaml"),
		mustResolveYAML(t, "../definitions/day0.bootstrap.yaml"),
		mustResolveYAML(t, "../definitions/node.bootstrap.yaml"),
		mustResolveYAML(t, "../definitions/node.join.yaml"),
		mustResolveYAML(t, "../definitions/node.repair.yaml"),
		mustResolveYAML(t, "../definitions/release.apply.package.yaml"),
		mustResolveYAML(t, "../definitions/release.apply.infrastructure.yaml"),
		mustResolveYAML(t, "../definitions/release.remove.package.yaml"),
		// cluster_doctor ships its remediation workflow in-binary; the
		// YAML lives next to the Go source and is the single source of
		// truth for that workflow.
		mustResolveYAML(t, "../../cluster_doctor/cluster_doctor_server/workflow_remediate_doctor_finding.yaml"),
	}

	// Build the union of all actions registered across every actor.
	router := NewRouter()
	registerAllActorsWithStubs(router)

	loader := v1alpha1.NewLoader()
	missing := map[string]string{} // "actor::action" -> YAML file
	for _, p := range yamlPaths {
		def, err := loader.LoadFile(p)
		if err != nil {
			t.Fatalf("load %s: %v", p, err)
		}
		for _, step := range flattenSteps(def.Spec.Steps) {
			if step.Actor == "" || step.Action == "" {
				// A control-flow-only step (e.g. a foreach group with no
				// action of its own). These dispatch their children but
				// do not themselves need a handler.
				continue
			}
			if _, ok := router.Resolve(step.Actor, step.Action); !ok {
				key := string(step.Actor) + "::" + step.Action
				if _, dup := missing[key]; !dup {
					missing[key] = filepath.Base(p)
				}
			}
		}
		// onFailure/onSuccess hooks also reference actions.
		for _, h := range []*v1alpha1.WorkflowHook{def.Spec.OnFailure, def.Spec.OnSuccess} {
			if h == nil || h.Actor == "" || h.Action == "" {
				continue
			}
			if _, ok := router.Resolve(h.Actor, h.Action); !ok {
				key := string(h.Actor) + "::" + h.Action
				if _, dup := missing[key]; !dup {
					missing[key] = filepath.Base(p) + " (hook)"
				}
			}
		}
	}

	if len(missing) > 0 {
		keys := make([]string, 0, len(missing))
		for k := range missing {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		t.Errorf("workflow YAMLs reference %d unregistered actions (handler missing or renamed):", len(missing))
		for _, k := range keys {
			t.Errorf("  %s  (first seen in %s)", k, missing[k])
		}
		t.Errorf("Fix by either registering the handler in workflow/engine/actors*.go or updating the YAML to match the registered action name.")
	}
}

// flattenSteps walks into nested foreach-group steps so we validate
// every leaf that actually gets dispatched.
func flattenSteps(steps []v1alpha1.WorkflowStepSpec) []v1alpha1.WorkflowStepSpec {
	var out []v1alpha1.WorkflowStepSpec
	for _, s := range steps {
		out = append(out, s)
		if len(s.Steps) > 0 {
			out = append(out, flattenSteps(s.Steps)...)
		}
	}
	return out
}

// mustResolveYAML fails the test if the YAML file is missing — this
// catches accidental deletions before they bite in production.
func mustResolveYAML(t *testing.T, rel string) string {
	t.Helper()
	if _, err := os.Stat(rel); err != nil {
		t.Fatalf("workflow YAML missing at %s: %v", rel, err)
	}
	return rel
}

// registerAllActorsWithStubs fills the Router with every action known
// to the engine package. We pass zero-valued Config structs because
// the Register* functions only reference cfg fields from inside closures
// — registration itself never dereferences them, so nil function fields
// are safe and we don't need to invent stub implementations here.
func registerAllActorsWithStubs(router *Router) {
	RegisterNodeAgentActions(router, NodeAgentConfig{})
	RegisterControllerActions(router, ControllerConfig{})
	RegisterInstallerActions(router, InstallerConfig{})
	RegisterRepositoryActions(router, RepositoryConfig{})
	RegisterReleaseControllerActions(router, ReleaseControllerConfig{})
	RegisterNodeDirectApplyActions(router, NodeDirectApplyConfig{})
	RegisterReconcileControllerActions(router, ReconcileControllerConfig{})
	RegisterWorkflowServiceActions(router, WorkflowServiceConfig{})
	RegisterDoctorRemediationActions(router, DoctorRemediationConfig{})
	RegisterNodeRepairControllerActions(router, NodeRepairControllerConfig{})
	RegisterNodeRepairAgentActions(router, NodeRepairAgentConfig{})
}
