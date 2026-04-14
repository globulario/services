// preflight.go — pre-execution validation for compiled workflows.
//
// Before a workflow runs, ValidatePreflight walks every step, hook, and
// nested sub-step to verify that each (actor, action) pair has a registered
// handler in the Router. If any action is unresolvable, the workflow is
// rejected with a detailed error listing every missing handler.
//
// This prevents silent step skipping — a pattern where the engine executes
// a workflow, an action has no handler, the step fails or returns empty
// output, and downstream steps proceed with missing data (which caused
// the "reconcile clean despite 23 drift items" bug).
//
// Call ValidatePreflight from Engine.ExecuteCompiled before creating the Run.
package engine

import (
	"fmt"
	"strings"

	"github.com/globulario/services/golang/workflow/compiler"
	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// PreflightError describes actions referenced by a workflow that have no
// registered handler. Each entry is a (step_id, actor, action) triple.
type PreflightError struct {
	Missing []PreflightMissing
}

// PreflightMissing is a single unresolvable action in a workflow.
type PreflightMissing struct {
	StepID string
	Actor  string
	Action string
}

func (e *PreflightError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "workflow preflight: %d action(s) have no registered handler:\n", len(e.Missing))
	for _, m := range e.Missing {
		fmt.Fprintf(&b, "  step=%s actor=%s action=%s\n", m.StepID, m.Actor, m.Action)
	}
	return b.String()
}

// ValidatePreflight checks that every (actor, action) pair referenced in the
// compiled workflow can be resolved by the Router. Returns nil if all actions
// are resolvable. Returns a *PreflightError listing all missing handlers.
//
// The check is conservative: if an actor has a RegisterFallback handler
// (used for remote dispatch), its actions are considered resolvable because
// the remote actor will validate them at execution time.
func ValidatePreflight(cw *compiler.CompiledWorkflow, router *Router) error {
	if cw == nil || router == nil {
		return nil
	}

	var missing []PreflightMissing

	// Check all top-level steps.
	for _, step := range cw.Steps {
		missing = checkStep(step, router, missing)
	}

	// Check onFailure / onSuccess hooks.
	if cw.OnFailure != nil {
		if !canResolve(router, cw.OnFailure.Actor, cw.OnFailure.Action) {
			missing = append(missing, PreflightMissing{
				StepID: "onFailure",
				Actor:  cw.OnFailure.Actor,
				Action: cw.OnFailure.Action,
			})
		}
	}
	if cw.OnSuccess != nil {
		if !canResolve(router, cw.OnSuccess.Actor, cw.OnSuccess.Action) {
			missing = append(missing, PreflightMissing{
				StepID: "onSuccess",
				Actor:  cw.OnSuccess.Actor,
				Action: cw.OnSuccess.Action,
			})
		}
	}

	if len(missing) > 0 {
		return &PreflightError{Missing: missing}
	}
	return nil
}

// checkStep validates a single step and its nested sub-steps recursively.
func checkStep(step *compiler.CompiledStep, router *Router, missing []PreflightMissing) []PreflightMissing {
	if step == nil {
		return missing
	}

	// Main action.
	if step.Actor != "" && step.Action != "" {
		if !canResolve(router, step.Actor, step.Action) {
			missing = append(missing, PreflightMissing{
				StepID: step.ID,
				Actor:  step.Actor,
				Action: step.Action,
			})
		}
	}

	// Verification action.
	if step.Verification != nil && step.Verification.Actor != "" {
		if !canResolve(router, step.Verification.Actor, step.Verification.Action) {
			missing = append(missing, PreflightMissing{
				StepID: step.ID + "/verify",
				Actor:  step.Verification.Actor,
				Action: step.Verification.Action,
			})
		}
	}

	// Compensation action.
	if step.Compensation != nil && step.Compensation.Enabled && step.Compensation.Actor != "" {
		if !canResolve(router, step.Compensation.Actor, step.Compensation.Action) {
			missing = append(missing, PreflightMissing{
				StepID: step.ID + "/compensate",
				Actor:  step.Compensation.Actor,
				Action: step.Compensation.Action,
			})
		}
	}

	// onFailure hook on this step.
	if step.OnFailure != nil {
		if !canResolve(router, step.OnFailure.Actor, step.OnFailure.Action) {
			missing = append(missing, PreflightMissing{
				StepID: step.ID + "/onFailure",
				Actor:  step.OnFailure.Actor,
				Action: step.OnFailure.Action,
			})
		}
	}

	// Nested sub-steps (foreach patterns have an embedded sub-workflow).
	if step.SubSteps != nil {
		for _, sub := range step.SubSteps.Steps {
			missing = checkStep(sub, router, missing)
		}
		if step.SubSteps.OnFailure != nil {
			if !canResolve(router, step.SubSteps.OnFailure.Actor, step.SubSteps.OnFailure.Action) {
				missing = append(missing, PreflightMissing{
					StepID: step.ID + "/foreach/onFailure",
					Actor:  step.SubSteps.OnFailure.Actor,
					Action: step.SubSteps.OnFailure.Action,
				})
			}
		}
	}

	return missing
}

// canResolve checks if the router can handle this (actor, action) pair.
// Returns true if there's a direct handler OR a fallback for the actor
// (fallback = remote dispatch, validated at execution time by the remote actor).
func canResolve(router *Router, actor, action string) bool {
	_, ok := router.Resolve(v1alpha1.ActorType(actor), action)
	return ok
}
