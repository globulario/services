// workflow_actions.go registers compute workflow action handlers with the
// workflow engine. These are the actor implementations for the compute.job.submit,
// compute.unit.execute, and compute.job.aggregate workflow definitions.
//
// All orchestration goes through workflows — these handlers perform only
// state reads, state mutations, and dispatches. They do not embed hidden
// lifecycle logic.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/globulario/services/golang/compute/computepb"
	"github.com/globulario/services/golang/compute/compute_runnerpb"
	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/v1alpha1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const ActorCompute v1alpha1.ActorType = "compute"

// RegisterComputeActions registers all compute workflow action handlers.
func RegisterComputeActions(router *engine.Router, srv *server) {
	// ── Job submission actions ────────────────────────────────────────
	router.Register(ActorCompute, "compute.load_job", computeLoadJob(srv))
	router.Register(ActorCompute, "compute.validate_job_definition", computeValidateJobDefinition(srv))
	router.Register(ActorCompute, "compute.admit_job", computeAdmitJob(srv))
	router.Register(ActorCompute, "compute.create_single_unit", computeCreateSingleUnit(srv))
	router.Register(ActorCompute, "compute.mark_job_failed", computeMarkJobFailed(srv))

	// ── Unit execution actions ───────────────────────────────────────
	router.Register(ActorCompute, "compute.choose_node", computeChooseNode(srv))
	router.Register(ActorCompute, "compute.mark_unit_assigned", computeMarkUnitAssigned(srv))
	router.Register(ActorCompute, "compute.stage_unit", computeStageUnit(srv))
	router.Register(ActorCompute, "compute.run_unit", computeRunUnit(srv))
	router.Register(ActorCompute, "compute.await_unit_terminal", computeAwaitUnitTerminal(srv))
	router.Register(ActorCompute, "compute.mark_unit_failed", computeMarkUnitFailed(srv))

	// ── Aggregation actions ──────────────────────────────────────────
	router.Register(ActorCompute, "compute.assess_unit_outcomes", computeAssessUnitOutcomes(srv))
	router.Register(ActorCompute, "compute.create_result", computeCreateResult(srv))
	router.Register(ActorCompute, "compute.finalize_job", computeFinalizeJob(srv))
}

// ─── Job submission handlers ─────────────────────────────────────────────────

func computeLoadJob(srv *server) engine.ActionHandler {
	return func(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
		jobID, _ := req.With["job_id"].(string)
		if jobID == "" {
			return nil, fmt.Errorf("job_id is required")
		}
		job, err := getJob(ctx, jobID)
		if err != nil {
			return nil, fmt.Errorf("load job: %w", err)
		}
		if job == nil {
			return nil, fmt.Errorf("job %s not found", jobID)
		}
		return &engine.ActionResult{
			OK:     true,
			Output: map[string]any{"job_id": jobID, "state": job.State.String()},
		}, nil
	}
}

func computeValidateJobDefinition(srv *server) engine.ActionHandler {
	return func(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
		jobID, _ := req.With["job_id"].(string)
		job, err := getJob(ctx, jobID)
		if err != nil || job == nil {
			return nil, fmt.Errorf("job %s not found", jobID)
		}
		def, err := getDefinition(ctx, job.Spec.DefinitionName, job.Spec.DefinitionVersion)
		if err != nil {
			return nil, fmt.Errorf("lookup definition: %w", err)
		}
		if def == nil {
			return nil, fmt.Errorf("definition %s@%s not found",
				job.Spec.DefinitionName, job.Spec.DefinitionVersion)
		}
		return &engine.ActionResult{
			OK:     true,
			Output: map[string]any{"definition_name": def.Name, "version": def.Version},
		}, nil
	}
}

func computeAdmitJob(srv *server) engine.ActionHandler {
	return func(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
		jobID, _ := req.With["job_id"].(string)
		job, err := getJob(ctx, jobID)
		if err != nil || job == nil {
			return nil, fmt.Errorf("job %s not found", jobID)
		}
		job.State = computepb.JobState_JOB_ADMITTED
		job.UpdatedAt = timestamppb.Now()
		if err := putJob(ctx, job); err != nil {
			return nil, fmt.Errorf("update job: %w", err)
		}
		slog.Info("compute workflow: job admitted", "job_id", jobID)
		return &engine.ActionResult{OK: true}, nil
	}
}

func computeCreateSingleUnit(srv *server) engine.ActionHandler {
	return func(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
		jobID, _ := req.With["job_id"].(string)
		job, err := getJob(ctx, jobID)
		if err != nil || job == nil {
			return nil, fmt.Errorf("job %s not found", jobID)
		}

		// Check if unit already exists (idempotency).
		units, _ := listUnits(ctx, jobID)
		if len(units) > 0 {
			return &engine.ActionResult{
				OK:     true,
				Output: map[string]any{"unit_id": units[0].UnitId},
			}, nil
		}

		unitID := fmt.Sprintf("unit-%d", time.Now().UnixMilli())
		unit := &computepb.ComputeUnit{
			UnitId:    unitID,
			JobId:     jobID,
			State:     computepb.UnitState_UNIT_PENDING,
			InputRefs: job.Spec.InputRefs,
			Attempt:   1,
		}
		if err := putUnit(ctx, unit); err != nil {
			return nil, fmt.Errorf("store unit: %w", err)
		}
		slog.Info("compute workflow: unit created", "job_id", jobID, "unit_id", unitID)
		return &engine.ActionResult{
			OK:     true,
			Output: map[string]any{"unit_id": unitID},
		}, nil
	}
}

func computeMarkJobFailed(srv *server) engine.ActionHandler {
	return func(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
		jobID, _ := req.With["job_id"].(string)
		if jobID == "" {
			return &engine.ActionResult{OK: true, Message: "no job_id to mark failed"}, nil
		}
		job, err := getJob(ctx, jobID)
		if err != nil || job == nil {
			return &engine.ActionResult{OK: true, Message: "job not found"}, nil
		}
		job.State = computepb.JobState_JOB_FAILED
		job.FailureMessage = "workflow execution failed"
		job.UpdatedAt = timestamppb.Now()
		_ = putJob(ctx, job)
		slog.Warn("compute workflow: job marked failed", "job_id", jobID)
		return &engine.ActionResult{OK: true}, nil
	}
}

// ─── Unit execution handlers ─────────────────────────────────────────────────

func computeChooseNode(srv *server) engine.ActionHandler {
	return func(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
		// Discover all nodes running the compute service via etcd.
		endpoints := resolveComputeEndpoints()
		if len(endpoints) == 0 {
			return nil, fmt.Errorf("no compute service instances available")
		}

		// V1: pick the first available endpoint.
		// Future: query node capabilities, profiles, load for scheduling.
		chosen := endpoints[0]
		slog.Info("compute workflow: node chosen",
			"endpoint", chosen, "candidates", len(endpoints))
		return &engine.ActionResult{
			OK: true,
			Output: map[string]any{
				"node_id":         chosen, // endpoint doubles as node identifier for dispatch
				"runner_endpoint": chosen,
			},
		}, nil
	}
}

func computeMarkUnitAssigned(srv *server) engine.ActionHandler {
	return func(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
		jobID, _ := req.With["job_id"].(string)
		unitID, _ := req.With["unit_id"].(string)
		nodeID, _ := req.With["node_id"].(string)
		runnerEndpoint, _ := req.With["runner_endpoint"].(string)

		unit, err := getUnit(ctx, jobID, unitID)
		if err != nil || unit == nil {
			return nil, fmt.Errorf("unit %s not found", unitID)
		}
		unit.State = computepb.UnitState_UNIT_ASSIGNED
		unit.NodeId = nodeID
		if err := putUnit(ctx, unit); err != nil {
			return nil, fmt.Errorf("update unit: %w", err)
		}
		return &engine.ActionResult{
			OK:     true,
			Output: map[string]any{"runner_endpoint": runnerEndpoint},
		}, nil
	}
}

func computeStageUnit(srv *server) engine.ActionHandler {
	return func(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
		jobID, _ := req.With["job_id"].(string)
		unitID, _ := req.With["unit_id"].(string)
		runnerEndpoint, _ := req.With["runner_endpoint"].(string)

		unit, err := getUnit(ctx, jobID, unitID)
		if err != nil || unit == nil {
			return nil, fmt.Errorf("unit %s not found", unitID)
		}

		job, _ := getJob(ctx, jobID)
		if job == nil {
			return nil, fmt.Errorf("job %s not found", jobID)
		}
		def, _ := getDefinition(ctx, job.Spec.DefinitionName, job.Spec.DefinitionVersion)
		if def == nil {
			return nil, fmt.Errorf("definition not found")
		}

		stageReq := computeRunnerStageRequest(unitID, jobID, def, unit, job.Spec)

		// Dispatch to remote runner via gRPC.
		client, conn, err := runnerClient(runnerEndpoint)
		if err != nil {
			return nil, fmt.Errorf("dial runner at %s: %w", runnerEndpoint, err)
		}
		defer conn.Close()

		resp, err := client.StageComputeUnit(ctx, &stageReq)
		if err != nil {
			return nil, fmt.Errorf("remote stage unit: %w", err)
		}

		slog.Info("compute workflow: unit staged remotely",
			"unit_id", unitID, "endpoint", runnerEndpoint,
			"staging_path", resp.StagingPath)

		return &engine.ActionResult{
			OK:     true,
			Output: map[string]any{"staging_path": resp.StagingPath},
		}, nil
	}
}

func computeRunUnit(srv *server) engine.ActionHandler {
	return func(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
		jobID, _ := req.With["job_id"].(string)
		unitID, _ := req.With["unit_id"].(string)
		stagingPath, _ := req.With["staging_path"].(string)
		runnerEndpoint, _ := req.With["runner_endpoint"].(string)

		job, _ := getJob(ctx, jobID)
		if job == nil {
			return nil, fmt.Errorf("job %s not found", jobID)
		}
		def, _ := getDefinition(ctx, job.Spec.DefinitionName, job.Spec.DefinitionVersion)
		if def == nil {
			return nil, fmt.Errorf("definition not found")
		}
		unit, _ := getUnit(ctx, jobID, unitID)
		if unit == nil {
			return nil, fmt.Errorf("unit %s not found", unitID)
		}

		runReq := computeRunnerRunRequest(unitID, jobID, def, unit, job.Spec, stagingPath)

		// Dispatch to remote runner via gRPC.
		client, conn, err := runnerClient(runnerEndpoint)
		if err != nil {
			return nil, fmt.Errorf("dial runner at %s: %w", runnerEndpoint, err)
		}
		defer conn.Close()

		resp, err := client.RunComputeUnit(ctx, &runReq)
		if err != nil {
			return nil, fmt.Errorf("remote run unit: %w", err)
		}

		slog.Info("compute workflow: unit running remotely",
			"unit_id", unitID, "endpoint", runnerEndpoint,
			"execution_id", resp.ExecutionId)

		return &engine.ActionResult{
			OK:     true,
			Output: map[string]any{"execution_id": resp.ExecutionId},
		}, nil
	}
}

func computeAwaitUnitTerminal(srv *server) engine.ActionHandler {
	return func(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
		jobID, _ := req.With["job_id"].(string)
		unitID, _ := req.With["unit_id"].(string)
		timeoutSec := 3600

		if ts, ok := req.With["timeout_seconds"].(float64); ok {
			timeoutSec = int(ts)
		}

		deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
		for {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if time.Now().After(deadline) {
				return nil, fmt.Errorf("unit %s did not reach terminal state within %ds", unitID, timeoutSec)
			}

			unit, err := getUnit(ctx, jobID, unitID)
			if err != nil {
				return nil, err
			}
			if unit == nil {
				return nil, fmt.Errorf("unit %s not found", unitID)
			}

			switch unit.State {
			case computepb.UnitState_UNIT_SUCCEEDED:
				return &engine.ActionResult{
					OK: true,
					Output: map[string]any{
						"unit_state": "UNIT_SUCCEEDED",
					},
				}, nil
			case computepb.UnitState_UNIT_FAILED:
				return &engine.ActionResult{
					OK: true,
					Output: map[string]any{
						"unit_state":    "UNIT_FAILED",
						"failure_class": unit.FailureClass.String(),
						"failure_reason": unit.FailureReason,
					},
				}, nil
			case computepb.UnitState_UNIT_CANCELLED:
				return &engine.ActionResult{
					OK:     true,
					Output: map[string]any{"unit_state": "UNIT_CANCELLED"},
				}, nil
			}

			time.Sleep(2 * time.Second)
		}
	}
}

func computeMarkUnitFailed(srv *server) engine.ActionHandler {
	return func(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
		jobID, _ := req.With["job_id"].(string)
		unitID, _ := req.With["unit_id"].(string)

		if unitID == "" || jobID == "" {
			return &engine.ActionResult{OK: true, Message: "no unit to mark failed"}, nil
		}
		unit, _ := getUnit(ctx, jobID, unitID)
		if unit != nil {
			unit.State = computepb.UnitState_UNIT_FAILED
			unit.FailureReason = "workflow execution failed"
			unit.EndTime = timestamppb.Now()
			_ = putUnit(ctx, unit)
		}
		return &engine.ActionResult{OK: true}, nil
	}
}

// ─── Aggregation handlers ────────────────────────────────────────────────────

func computeAssessUnitOutcomes(srv *server) engine.ActionHandler {
	return func(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
		jobID, _ := req.With["job_id"].(string)
		units, err := listUnits(ctx, jobID)
		if err != nil {
			return nil, fmt.Errorf("list units: %w", err)
		}

		succeeded := 0
		failed := 0
		for _, u := range units {
			switch u.State {
			case computepb.UnitState_UNIT_SUCCEEDED:
				succeeded++
			case computepb.UnitState_UNIT_FAILED:
				failed++
			}
		}

		return &engine.ActionResult{
			OK: true,
			Output: map[string]any{
				"total":           len(units),
				"succeeded":       succeeded,
				"failed":          failed,
				"requires_repair": false, // v1: no repair
			},
		}, nil
	}
}

func computeCreateResult(srv *server) engine.ActionHandler {
	return func(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
		jobID, _ := req.With["job_id"].(string)

		job, _ := getJob(ctx, jobID)
		units, _ := listUnits(ctx, jobID)

		// Run verification on the first succeeded unit (v1: single unit).
		var vResult verificationResult
		for _, u := range units {
			if u.State != computepb.UnitState_UNIT_SUCCEEDED {
				continue
			}
			if job != nil && job.Spec != nil {
				def, _ := getDefinition(ctx, job.Spec.DefinitionName, job.Spec.DefinitionVersion)
				if def != nil {
					stagingPath := "/var/lib/globular/compute/jobs/" + jobID + "/units/" + u.UnitId
					vResult = verifyOutput(def, stagingPath, u)
					slog.Info("compute workflow: verification completed",
						"job_id", jobID, "unit_id", u.UnitId,
						"passed", vResult.Passed,
						"trust_level", vResult.TrustLevel.String())
				}
			}
			break
		}
		if vResult.TrustLevel == computepb.ResultTrustLevel_RESULT_TRUST_LEVEL_UNSPECIFIED {
			vResult.TrustLevel = computepb.ResultTrustLevel_UNVERIFIED
			vResult.Passed = true
		}

		result := &computepb.ComputeResult{
			JobId:       jobID,
			TrustLevel:  vResult.TrustLevel,
			Checksums:   vResult.Checksums,
			Metadata:    verificationMetadataToStruct(vResult.Metadata),
			CompletedAt: timestamppb.Now(),
		}
		if err := putResult(ctx, result); err != nil {
			return nil, fmt.Errorf("store result: %w", err)
		}

		return &engine.ActionResult{
			OK: true,
			Output: map[string]any{
				"trust_level":         vResult.TrustLevel.String(),
				"verification_passed": vResult.Passed,
				"verification_msg":    vResult.Message,
			},
		}, nil
	}
}

func computeFinalizeJob(srv *server) engine.ActionHandler {
	return func(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
		jobID, _ := req.With["job_id"].(string)
		verificationPassed := true
		if vp, ok := req.With["verification_passed"].(bool); ok {
			verificationPassed = vp
		}

		job, err := getJob(ctx, jobID)
		if err != nil || job == nil {
			return nil, fmt.Errorf("job %s not found", jobID)
		}

		units, _ := listUnits(ctx, jobID)
		allSucceeded := true
		for _, u := range units {
			if u.State != computepb.UnitState_UNIT_SUCCEEDED {
				allSucceeded = false
				break
			}
		}

		if allSucceeded && verificationPassed {
			job.State = computepb.JobState_JOB_COMPLETED
		} else if allSucceeded && !verificationPassed {
			job.State = computepb.JobState_JOB_FAILED
			job.FailureMessage = "verification failed"
		} else {
			job.State = computepb.JobState_JOB_FAILED
			job.FailureMessage = "one or more units failed"
		}
		job.UpdatedAt = timestamppb.Now()
		if err := putJob(ctx, job); err != nil {
			return nil, fmt.Errorf("finalize job: %w", err)
		}

		slog.Info("compute workflow: job finalized",
			"job_id", jobID, "state", job.State.String())
		return &engine.ActionResult{OK: true, Output: map[string]any{"state": job.State.String()}}, nil
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func computeRunnerStageRequest(unitID, jobID string, def *computepb.ComputeDefinition, unit *computepb.ComputeUnit, spec *computepb.ComputeJobSpec) compute_runnerpb.StageComputeUnitRequest {
	return compute_runnerpb.StageComputeUnitRequest{
		UnitId:     unitID,
		JobId:      jobID,
		Definition: def,
		Unit:       unit,
		JobSpec:    spec,
	}
}

func computeRunnerRunRequest(unitID, jobID string, def *computepb.ComputeDefinition, unit *computepb.ComputeUnit, spec *computepb.ComputeJobSpec, stagingPath string) compute_runnerpb.RunComputeUnitRequest {
	return compute_runnerpb.RunComputeUnitRequest{
		UnitId:      unitID,
		JobId:       jobID,
		Definition:  def,
		Unit:        unit,
		JobSpec:     spec,
		StagingPath: stagingPath,
	}
}
