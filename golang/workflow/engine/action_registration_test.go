package engine

import (
	"context"
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
		mustResolveYAML(t, "../definitions/remediate.doctor.finding.yaml"),
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
	// WH-4: verification handlers for resume-policy dispatch.
	RegisterNodeVerificationActions(router, NodeVerificationConfig{})
	RegisterControllerVerificationActions(router, ControllerVerificationConfig{})
}

// --------------------------------------------------------------------------
// Actor capability parity tests
// --------------------------------------------------------------------------
//
// These tests verify the reverse direction from
// TestEmbeddedWorkflowsHaveRegisteredActions: every actor that claims to own
// actions in the central registry MUST register the same set locally.
//
// Without this, the central registry can drift from actor implementations:
// a registration function adds an action, but the actor service never wires
// the corresponding handler — and the mismatch only surfaces at runtime
// when the workflow service dispatches a callback the actor doesn't handle.

// actorRegistrationSet maps each actor type to its local registration
// function. When centralized execution is live, each actor's
// WorkflowActorService will build a Router using exactly these functions.
// This test proves the local set matches the central set.
var actorRegistrationSet = map[string]func(r *Router){
	string(v1alpha1.ActorClusterDoctor): func(r *Router) {
		RegisterDoctorRemediationActions(r, DoctorRemediationConfig{})
	},
	string(v1alpha1.ActorClusterController): func(r *Router) {
		RegisterControllerActions(r, ControllerConfig{})
		RegisterReleaseControllerActions(r, ReleaseControllerConfig{})
		RegisterReconcileControllerActions(r, ReconcileControllerConfig{})
		RegisterNodeRepairControllerActions(r, NodeRepairControllerConfig{})
		RegisterControllerVerificationActions(r, ControllerVerificationConfig{})
		// installer and repository are controller-owned actors but register
		// under their own actor type — tested separately below.
	},
	string(v1alpha1.ActorNodeAgent): func(r *Router) {
		RegisterNodeAgentActions(r, NodeAgentConfig{})
		RegisterNodeDirectApplyActions(r, NodeDirectApplyConfig{})
		RegisterNodeRepairAgentActions(r, NodeRepairAgentConfig{})
		RegisterNodeVerificationActions(r, NodeVerificationConfig{})
	},
	string(v1alpha1.ActorWorkflowService): func(r *Router) {
		RegisterWorkflowServiceActions(r, WorkflowServiceConfig{})
	},
	string(v1alpha1.ActorInstaller): func(r *Router) {
		RegisterInstallerActions(r, InstallerConfig{})
	},
	string(v1alpha1.ActorRepository): func(r *Router) {
		RegisterRepositoryActions(r, RepositoryConfig{})
	},
}

// TestActorCapabilityParity verifies that every action the central
// registry declares for an actor is also present in that actor's local
// registration. This is a hard requirement — see
// docs/centralized-workflow-execution.md §4.
func TestActorCapabilityParity(t *testing.T) {
	// Build the central (union) registry.
	central := NewRouter()
	registerAllActorsWithStubs(central)
	centralActions := central.RegisteredActions()

	for actorName, registerFn := range actorRegistrationSet {
		t.Run(actorName, func(t *testing.T) {
			// Build the actor's local registry.
			local := NewRouter()
			registerFn(local)
			localActions := local.RegisteredActions()

			// Every action in the central registry for this actor must
			// exist in the local registry.
			centralSet := toSet(centralActions[actorName])
			localSet := toSet(localActions[actorName])

			var missing []string
			for action := range centralSet {
				if !localSet[action] {
					missing = append(missing, action)
				}
			}
			if len(missing) > 0 {
				sort.Strings(missing)
				t.Errorf("actor %q has %d actions in central registry but missing from local registration:", actorName, len(missing))
				for _, a := range missing {
					t.Errorf("  %s::%s", actorName, a)
				}
				t.Errorf("The actor's WorkflowActorService will reject these at runtime. Fix by registering the handler in the actor's local router.")
			}
		})
	}
}

// TestYAMLActionsReferenceKnownActors ensures every actor type used in YAML
// definitions maps to a known actor registration set. If a YAML references
// actor "foo" but no registration function exists for it, the action can
// never be dispatched.
func TestYAMLActionsReferenceKnownActors(t *testing.T) {
	yamlPaths := collectAllYAMLPaths(t)
	loader := v1alpha1.NewLoader()
	unknown := map[string]string{} // actor → first YAML file

	for _, p := range yamlPaths {
		def, err := loader.LoadFile(p)
		if err != nil {
			t.Fatalf("load %s: %v", p, err)
		}
		for _, step := range flattenSteps(def.Spec.Steps) {
			if step.Actor == "" {
				continue
			}
			actor := string(step.Actor)
			if _, ok := actorRegistrationSet[actor]; !ok {
				if _, dup := unknown[actor]; !dup {
					unknown[actor] = filepath.Base(p)
				}
			}
		}
	}

	if len(unknown) > 0 {
		keys := make([]string, 0, len(unknown))
		for k := range unknown {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		t.Errorf("%d actor types used in YAML but missing from actorRegistrationSet:", len(unknown))
		for _, k := range keys {
			t.Errorf("  actor %q (first seen in %s)", k, unknown[k])
		}
		t.Errorf("Add a registration function for each actor to actorRegistrationSet in action_registration_test.go.")
	}
}

// TestFallbackResolveExactTakesPrecedence verifies the Router dispatch
// contract: exact (actor, action) matches take precedence over fallback
// handlers, and fallbacks reject unknown actions.
func TestFallbackResolveExactTakesPrecedence(t *testing.T) {
	router := NewRouter()

	exactCalled := false
	fallbackCalled := false

	router.Register(v1alpha1.ActorClusterDoctor, "doctor.resolve_finding", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		exactCalled = true
		return &ActionResult{OK: true}, nil
	})
	router.RegisterFallback(v1alpha1.ActorClusterDoctor, func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		fallbackCalled = true
		return &ActionResult{OK: true}, nil
	})

	// Exact match should win.
	h, ok := router.Resolve(v1alpha1.ActorClusterDoctor, "doctor.resolve_finding")
	if !ok {
		t.Fatal("expected exact handler to resolve")
	}
	h(context.Background(), ActionRequest{})
	if !exactCalled {
		t.Error("exact handler was not called")
	}
	if fallbackCalled {
		t.Error("fallback should not have been called for exact match")
	}

	// Unknown action should fall through to fallback.
	exactCalled = false
	fallbackCalled = false
	h, ok = router.Resolve(v1alpha1.ActorClusterDoctor, "doctor.unknown_action")
	if !ok {
		t.Fatal("expected fallback handler to resolve for unknown action")
	}
	h(context.Background(), ActionRequest{})
	if !fallbackCalled {
		t.Error("fallback handler was not called for unknown action")
	}
	if exactCalled {
		t.Error("exact handler should not have been called for unknown action")
	}

	// No fallback for a different actor should return not-found.
	_, ok = router.Resolve(v1alpha1.ActorNodeAgent, "agent.some_action")
	if ok {
		t.Error("expected no handler for actor without registration or fallback")
	}
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

func collectAllYAMLPaths(t *testing.T) []string {
	t.Helper()
	// All workflow definitions live in one canonical directory:
	// golang/workflow/definitions/. No service-private copies.
	return []string{
		mustResolveYAML(t, "../definitions/cluster.reconcile.yaml"),
		mustResolveYAML(t, "../definitions/day0.bootstrap.yaml"),
		mustResolveYAML(t, "../definitions/node.bootstrap.yaml"),
		mustResolveYAML(t, "../definitions/node.join.yaml"),
		mustResolveYAML(t, "../definitions/node.repair.yaml"),
		mustResolveYAML(t, "../definitions/release.apply.package.yaml"),
		mustResolveYAML(t, "../definitions/release.apply.infrastructure.yaml"),
		mustResolveYAML(t, "../definitions/release.remove.package.yaml"),
		mustResolveYAML(t, "../definitions/remediate.doctor.finding.yaml"),
	}
}

func toSet(items []string) map[string]bool {
	s := make(map[string]bool, len(items))
	for _, item := range items {
		s[item] = true
	}
	return s
}
