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
	"errors"
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
	sess := srv.getSession()
	if sess == nil {
		return fmt.Errorf("ScyllaDB not available for orphan run lookup")
	}
	var clusterID string
	if err := sess.Query(`
		SELECT cluster_id FROM workflow.workflow_runs
		WHERE id = ? LIMIT 1 ALLOW FILTERING`, runID,
	).Scan(&clusterID); err != nil {
		return fmt.Errorf("lookup cluster_id for run %s: %w", runID, err)
	}
	if clusterID == "" {
		return fmt.Errorf("run %s not found in workflow_runs", runID)
	}
	// Resolve the controller endpoint so the resumed run can dispatch actor
	// callbacks. The controller proxies all actor types (node-agent, installer,
	// repository, workflow-service). Falls back to nil (probe-only mode) when
	// the controller is not yet discoverable — the orphan scanner will retry on
	// the next cycle once the controller re-registers in etcd.
	actorEndpoints := resolveOrphanActorEndpoints()
	return srv.ResumeRun(ctx, clusterID, runID, actorEndpoints)
}

// resolveOrphanActorEndpoints looks up the cluster-controller's direct gRPC
// address so orphaned runs can dispatch actor callbacks.
//
// IMPORTANT: must NOT use config.ResolveServiceAddr — that function routes
// through meshRouteAddrs which rewrites the port to 443 (Envoy). Envoy does
// not serve WorkflowActorService and returns text/html, causing every actor
// dispatch to fail with "unexpected HTTP status code received from server: 200".
//
// Resolution order:
//  1. ResolveLocalServiceAddr — returns the local controller's direct port when
//     this workflow node is co-located with a controller instance (common case).
//  2. ResolveControllerDirectAddr — reads the leader's config from etcd directly,
//     bypassing mesh routing. Used when the local node has no controller.
//
// Returns nil when the controller is not yet discoverable (probe-only mode).
func resolveOrphanActorEndpoints() map[string]string {
	addr := config.ResolveLocalServiceAddr("cluster_controller.ClusterControllerService")
	if addr == "" {
		addr = config.ResolveControllerDirectAddr()
	}
	if addr == "" {
		return nil
	}
	return map[string]string{
		"cluster-controller": addr,
		"node-agent":         addr,
		"installer":          addr,
		"repository":         addr,
		"workflow-service":   addr,
	}
}

// ResumeRun resumes an orphaned workflow run. It loads the persisted state,
// rebuilds the engine with completed steps pre-set, and re-executes.
//
// This is called by the orphan scanner after claiming a stale lease.
func (srv *server) ResumeRun(ctx context.Context, clusterID, runID string, actorEndpoints map[string]string) error {
	if srv.getSession() == nil {
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
		case workflowpb.StepStatus_STEP_STATUS_SKIPPED:
			completedSteps[step.StepKey] = engine.StepSkipped
		case workflowpb.StepStatus_STEP_STATUS_FAILED:
			completedSteps[step.StepKey] = engine.StepFailed
		}
	}

	// NOTE: we intentionally do NOT short-circuit when allTerminalSuccess is
	// true. "All recorded steps succeeded" does not mean "workflow is complete":
	// steps that never started have no ScyllaDB record, so they are absent from
	// completedSteps. Bypassing the engine here would skip those steps AND the
	// onSuccess hook, leaving etcd cleanup state (restart_in_progress, locks,
	// applied_generation) un-written. The engine skips pre-completed steps
	// safely via PreCompleted, so re-executing with a full engine is always
	// correct and never double-applies idempotent effects.
	slog.Info("resume: loading workflow for re-execution",
		"run_id", runID,
		"workflow", run.WorkflowName,
		"completed_steps", len(completedSteps),
		"total_recorded_steps", len(steps))

	// ── 3. Load definition: etcd (core) → MinIO (service-owned) ─────────
	// Mirror ExecuteWorkflow's lookup so resume sees the same definitions
	// as fresh dispatch. Core workflows like release.apply.package live in
	// etcd under /globular/workflows/; only service-owned definitions fall
	// back to MinIO. Going MinIO-only here would resurrect the "load
	// definition X: %!w(<nil>)" symptom — a malformed wrap that hid the
	// real cause (etcd has the def, MinIO does not).
	var defYAML []byte
	if v1alpha1.EtcdFetcher != nil {
		if b, ferr := v1alpha1.EtcdFetcher(run.WorkflowName); ferr == nil && len(b) > 0 {
			defYAML = b
		}
	}
	if len(defYAML) == 0 {
		b, ferr := config.GetClusterConfig("workflows/" + run.WorkflowName + ".yaml")
		if ferr != nil {
			return fmt.Errorf("load definition %s: %w", run.WorkflowName, ferr)
		}
		defYAML = b
	}
	if len(defYAML) == 0 {
		return fmt.Errorf("workflow definition %q not found (checked etcd and MinIO)", run.WorkflowName)
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
		// When called from the orphan scanner (actorEndpoints == nil), a
		// PreflightError means the remote actors haven't re-registered their
		// handlers yet (e.g. controller restarted alongside the workflow
		// service). Do NOT fail the run permanently — return the error so
		// the orphan scanner retries on the next cycle once actors are back.
		var pfErr *engine.PreflightError
		if errors.As(execErr, &pfErr) && len(actorEndpoints) == 0 {
			slog.Info("resume: preflight missing handlers in probe mode, will retry",
				"run_id", runID, "missing", len(pfErr.Missing))
			return execErr
		}

		// For all other errors (infrastructure failures, or preflight failures
		// when we did have endpoints), fail the run immediately so the
		// controller can re-dispatch with a fresh correlation ID.
		slog.Warn("resume: re-execution failed, releasing run",
			"run_id", runID, "err", execErr)
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
