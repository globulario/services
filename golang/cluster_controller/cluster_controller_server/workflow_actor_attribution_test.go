// @awareness namespace=globular.platform
// @awareness component=platform_controller.workflow_actor_attribution
// @awareness file_role=regression_tests_for_workflow_step_actor_attribution
// @awareness enforces=globular.platform:invariant.four_layer.workflow_actor_attribution_required
// @awareness enforces=globular.platform:invariant.workflow.every_state_mutation_belongs_to_a_workflow_instance
// @awareness risk=critical
package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestMarkNodeSucceeded_AttributedToActorNotWorkflowService pins the actor-
// attribution contract for the mark_node_succeeded step across every
// release.apply.*.yaml definition. The step writes per-node state into a
// ServiceRelease/InfrastructureRelease — a Layer-3 mutation. Per
// four_layer.workflow_actor_attribution_required, the writing actor must
// be cluster-controller (the controller owns desired-state writes,
// including release-status patches), NOT workflow-service.
//
// The regression shape this test catches: a future YAML edit that changes
// `actor: cluster-controller` to `actor: workflow-service` for any
// mark_node_succeeded step, which would attribute the write to the
// workflow service's identity (violating both invariants above).
func TestMarkNodeSucceeded_AttributedToActorNotWorkflowService(t *testing.T) {
	repoRoot := findRepoRoot(t)
	definitionsDir := filepath.Join(repoRoot, "golang", "workflow", "definitions")

	// Every YAML whose name starts with release.apply must, for any step
	// using action controller.release.mark_node_succeeded, name
	// actor: cluster-controller.
	entries, err := os.ReadDir(definitionsDir)
	if err != nil {
		t.Fatalf("read definitions dir: %v", err)
	}

	found := 0
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "release.apply.") || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(definitionsDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		content := string(data)

		// Locate every mark_node_succeeded step block. The YAML pattern is:
		//   actor: <name>
		//   action: controller.release.mark_node_succeeded
		// We assert that the immediately-preceding actor line names
		// cluster-controller (with either underscore or hyphen — the
		// YAMLs use both forms).
		actionRe := regexp.MustCompile(`(?m)^[[:space:]]+actor:[[:space:]]+([a-z][a-z_\-]+)[[:space:]]*$\n[[:space:]]+action:[[:space:]]+controller\.release\.mark_node_succeeded[[:space:]]*$`)
		matches := actionRe.FindAllStringSubmatch(content, -1)
		if len(matches) == 0 {
			continue
		}
		for _, m := range matches {
			actor := m[1]
			normalized := strings.ReplaceAll(actor, "_", "-")
			if normalized != "cluster-controller" {
				t.Errorf("%s: mark_node_succeeded attributed to actor=%q (normalized=%q) — must be cluster-controller per four_layer.workflow_actor_attribution_required",
					e.Name(), actor, normalized)
			}
			if normalized == "workflow-service" {
				t.Errorf("%s: CRITICAL — mark_node_succeeded attributed to workflow-service; the workflow service is a router, not a writer (violates workflow.every_state_mutation_belongs_to_a_workflow_instance forbidden_fix actor_substitutes_workflow_service_identity_for_step_actor)",
					e.Name())
			}
			found++
		}
	}
	if found == 0 {
		t.Fatalf("found zero mark_node_succeeded steps in release.apply.*.yaml — the test pattern is stale or no such step exists")
	}
}

// TestReleaseApplyWorkflows_NoStateMutationAttributedToWorkflowService is
// the broader sibling test: scan every release.apply.*.yaml and assert no
// step using a state-mutation action (controller.*, node.*, repository.*)
// is attributed to actor: workflow-service.
//
// The workflow service is the ROUTER between actors. Its job is to
// dispatch each step to the actor named in the YAML — never to substitute
// its own identity. A workflow-service-attributed mutation is the exact
// forbidden_fix shape captured by
// actor_substitutes_workflow_service_identity_for_step_actor.
func TestReleaseApplyWorkflows_NoStateMutationAttributedToWorkflowService(t *testing.T) {
	repoRoot := findRepoRoot(t)
	definitionsDir := filepath.Join(repoRoot, "golang", "workflow", "definitions")

	entries, err := os.ReadDir(definitionsDir)
	if err != nil {
		t.Fatalf("read definitions dir: %v", err)
	}

	// Pattern: actor: <X> ... action: controller.<...> | node.<...> | repository.<...> | compute.<...>
	// Stateful action namespaces — these mutate cluster state and MUST be
	// attributed to the owning actor, never to workflow-service.
	stateMutationActionRe := regexp.MustCompile(`(?m)^[[:space:]]+actor:[[:space:]]+(workflow[_\-]service)[[:space:]]*$\n[[:space:]]+action:[[:space:]]+(controller|node|repository|compute)\.`)

	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "release.apply.") || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(definitionsDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if matches := stateMutationActionRe.FindAllStringSubmatch(string(data), -1); len(matches) > 0 {
			for _, m := range matches {
				t.Errorf("%s: actor=%q dispatching state-mutation action namespace %q — workflow service must not substitute its identity for the owning actor",
					e.Name(), m[1], m[2])
			}
		}
	}
}
