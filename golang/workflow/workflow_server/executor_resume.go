// executor_resume.go implements workflow run resumption after executor crash.
//
// When an orphaned run is claimed by the orphan scanner, ResumeRun loads
// the persisted run/step state from ScyllaDB, determines which steps are
// completed vs in-progress, and re-executes from the appropriate point.
//
// Resume semantics (from HA-control-plane-design.md):
//   - SUCCEEDED/FAILED/SKIPPED steps: skip (already terminal)
//   - RUNNING step: re-execute from the beginning (actor callbacks are idempotent)
//   - PENDING steps: execute normally
//   - Terminal hooks: check run status before replaying
//   - Child workflows: synchronous, complete before StartChild returns — no duplication risk
package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/v1alpha1"
	"github.com/globulario/services/golang/workflow/workflowpb"
)

// resumeOrphanedRun is called by the orphan scanner to resume a run
// whose executor died. It looks up the cluster_id from the run record.
func (srv *server) resumeOrphanedRun(ctx context.Context, runID string) error {
	// Find the cluster_id for this run by scanning workflow_runs.
	var clusterID string
	if err := srv.session.Query(`
		SELECT cluster_id FROM workflow.workflow_runs
		WHERE id = ? LIMIT 1 ALLOW FILTERING`, runID,
	).Scan(&clusterID); err != nil {
		return fmt.Errorf("lookup cluster_id for run %s: %w", runID, err)
	}
	if clusterID == "" {
		return fmt.Errorf("run %s not found in workflow_runs", runID)
	}
	// Resume with no actor endpoints — the workflow will fail on actor
	// dispatch, but this is correct: the controller/doctor needs to
	// re-register their actor routers for the run to actually succeed.
	// For now, this handles the terminal state cleanup and logging.
	return srv.ResumeRun(ctx, clusterID, runID, nil)
}

// ResumeRun resumes an orphaned workflow run. It loads the persisted state,
// rebuilds the engine with completed steps pre-set, and re-executes.
//
// This is called by the orphan scanner after claiming a stale lease.
func (srv *server) ResumeRun(ctx context.Context, clusterID, runID string, actorEndpoints map[string]string) error {
	if srv.session == nil {
		return fmt.Errorf("ScyllaDB not available for run state loading")
	}

	// ── 1. Check run is still in EXECUTING state ─────────────────────────
	run, err := srv.loadRunByID(clusterID, runID)
	if err != nil {
		return fmt.Errorf("load run: %w", err)
	}
	if run.Status == workflowpb.RunStatus_RUN_STATUS_SUCCEEDED ||
		run.Status == workflowpb.RunStatus_RUN_STATUS_FAILED ||
		run.Status == workflowpb.RunStatus_RUN_STATUS_CANCELED ||
		run.Status == workflowpb.RunStatus_RUN_STATUS_ROLLED_BACK {
		// Already terminal — release the lease and skip.
		slog.Info("resume: run already terminal, skipping",
			"run_id", runID, "status", run.Status.String())
		return nil
	}

	// ── 2. Load persisted steps ──────────────────────────────────────────
	steps, err := srv.loadSteps(clusterID, runID)
	if err != nil {
		return fmt.Errorf("load steps: %w", err)
	}

	// Build a map of step_key → terminal status for completed steps.
	completedSteps := make(map[string]engine.StepStatus)
	for _, step := range steps {
		switch step.Status {
		case workflowpb.StepStatus_STEP_STATUS_SUCCEEDED:
			completedSteps[step.StepKey] = engine.StepSucceeded
		case workflowpb.StepStatus_STEP_STATUS_FAILED:
			completedSteps[step.StepKey] = engine.StepFailed
		case workflowpb.StepStatus_STEP_STATUS_SKIPPED:
			completedSteps[step.StepKey] = engine.StepSkipped
		// RUNNING and PENDING steps will be re-executed.
		}
	}

	slog.Info("resume: loading workflow for re-execution",
		"run_id", runID,
		"workflow", run.WorkflowName,
		"completed_steps", len(completedSteps),
		"total_recorded_steps", len(steps))

	// ── 3. Load definition ───────────────────────────────────────────────
	defYAML, err := config.GetClusterConfig("workflows/" + run.WorkflowName + ".yaml")
	if err != nil || defYAML == nil {
		return fmt.Errorf("load definition %s: %w", run.WorkflowName, err)
	}
	loader := v1alpha1.NewLoader()
	def, err := loader.LoadBytes(defYAML)
	if err != nil {
		return fmt.Errorf("parse definition %s: %w", run.WorkflowName, err)
	}

	// ── 4. Rebuild router with actor endpoints ───────────────────────────
	dispatcher := newActorDispatcher(actorEndpoints)
	defer dispatcher.close()

	router := engine.NewRouter()
	engine.RegisterWorkflowServiceActions(router, engine.WorkflowServiceConfig{})
	for actorType := range actorEndpoints {
		at := actorType
		router.RegisterFallback(v1alpha1.ActorType(at), dispatcher.makeHandler(at))
	}

	// ── 5. Build engine with pre-completed steps and resume mode ─────────
	eng := &engine.Engine{
		Router: router,
		// PreCompleted tells the engine which steps are already done.
		// The engine skips these during DAG execution.
		PreCompleted: completedSteps,
		// IsResume enables policy-driven resume: steps with resume_policy
		// metadata are checked (verify_effect, pause, fail) before re-execution.
		IsResume: true,
		OnStepDone: func(r *engine.Run, step *engine.StepState) {
			slog.Info("resume: step done",
				"run_id", runID, "step", step.ID, "status", string(step.Status))
		},
	}

	// ── 6. Reconstruct inputs from the original run ──────────────────────
	// The original inputs are not persisted in workflow_runs (they're in
	// the correlation context). For resume, we pass empty inputs — the
	// completed steps already have their outputs in the engine, and
	// remaining steps will get their inputs from the DAG.
	//
	// TODO: persist inputs_json in workflow_runs for full resume fidelity.
	inputs := make(map[string]any)

	// ── 7. Execute (engine skips completed steps) ────────────────────────
	_, execErr := eng.Execute(ctx, def, inputs)

	if execErr != nil {
		slog.Warn("resume: re-execution failed",
			"run_id", runID, "err", execErr)
		// Record failure.
		srv.FinishRun(ctx, &workflowpb.FinishRunRequest{
			Id:           runID,
			ClusterId:    clusterID,
			Status:       workflowpb.RunStatus_RUN_STATUS_FAILED,
			Summary:      "resumed after executor crash, failed on re-execution",
			ErrorMessage: execErr.Error(),
		})
	} else {
		slog.Info("resume: re-execution succeeded", "run_id", runID)
		srv.FinishRun(ctx, &workflowpb.FinishRunRequest{
			Id:        runID,
			ClusterId: clusterID,
			Status:    workflowpb.RunStatus_RUN_STATUS_SUCCEEDED,
			Summary:   "resumed after executor crash, completed successfully",
		})
	}

	return nil
}
