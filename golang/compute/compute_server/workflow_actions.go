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
	"sync/atomic"
	"time"

	"github.com/globulario/services/golang/compute/computepb"
	"github.com/globulario/services/golang/compute/compute_runnerpb"
	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/v1alpha1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// roundRobinCounter is used by computeChooseNode for deterministic placement.
var roundRobinCounter atomic.Uint64

const ActorCompute v1alpha1.ActorType = "compute"

// RegisterComputeActions registers all compute workflow action handlers.
func RegisterComputeActions(router *engine.Router, srv *server) {
	// ── Job submission actions ────────────────────────────────────────
	router.Register(ActorCompute, "compute.load_job", computeLoadJob(srv))
	router.Register(ActorCompute, "compute.validate_job_definition", computeValidateJobDefinition(srv))
	router.Register(ActorCompute, "compute.admit_job", computeAdmitJob(srv))
	router.Register(ActorCompute, "compute.create_single_unit", computeCreateSingleUnit(srv))
	router.Register(ActorCompute, "compute.mark_job_failed", computeMarkJobFailed(srv))

	// ── Multi-unit dispatch ─────────────────────────────────────────
	router.Register(ActorCompute, "compute.dispatch_all_units", computeDispatchAllUnits(srv))
	router.Register(ActorCompute, "compute.await_all_units", computeAwaitAllUnits(srv))

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

		// Check if units already exist (idempotency / partition path).
		units, _ := listUnits(ctx, jobID)
		if len(units) > 0 {
			// Multi-unit: return all unit IDs as a list for foreach fan-out.
			if len(units) > 1 {
				unitItems := make([]map[string]any, len(units))
				for i, u := range units {
					unitItems[i] = map[string]any{
						"unit_id": u.UnitId,
						"job_id":  u.JobId,
					}
				}
				return &engine.ActionResult{
					OK: true,
					Output: map[string]any{
						"unit_id":    units[0].UnitId,
						"unit_items": unitItems,
						"unit_count": len(units),
					},
				}, nil
			}
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

// ─── Multi-unit dispatch handlers ─────────────────────────────────────────────

// computeDispatchAllUnits starts a compute.unit.execute workflow for each
// pending unit in the job. For single-unit jobs, dispatches one. For multi-unit,
// dispatches N in parallel via goroutines.
func computeDispatchAllUnits(srv *server) engine.ActionHandler {
	return func(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
		jobID, _ := req.With["job_id"].(string)

		units, err := listUnits(ctx, jobID)
		if err != nil {
			return nil, fmt.Errorf("list units: %w", err)
		}

		job, _ := getJob(ctx, jobID)
		if job != nil {
			job.State = computepb.JobState_JOB_RUNNING
			job.UpdatedAt = timestamppb.Now()
			_ = putJob(ctx, job)
		}

		// For each unit, run the full execute pipeline inline:
		// choose_node → assign → stage → run (async).
		// The await step handles waiting for completion.
		endpoints := resolveComputeEndpoints()
		if len(endpoints) == 0 {
			return nil, fmt.Errorf("no compute service instances available")
		}

		var def *computepb.ComputeDefinition
		if job != nil && job.Spec != nil {
			def, _ = getDefinition(ctx, job.Spec.DefinitionName, job.Spec.DefinitionVersion)
		}
		if def == nil {
			return nil, fmt.Errorf("definition not found for job %s", jobID)
		}

		dispatched := 0
		for _, unit := range units {
			if unit.State != computepb.UnitState_UNIT_PENDING {
				continue
			}

			// Round-robin placement.
			idx := roundRobinCounter.Add(1) - 1
			endpoint := endpoints[int(idx)%len(endpoints)]

			// Grant lease.
			leaseID, err := grantUnitLease(ctx, jobID, unit.UnitId, endpoint)
			if err != nil {
				slog.Warn("compute dispatch: lease grant failed",
					"unit_id", unit.UnitId, "err", err)
				continue
			}

			// Mark assigned.
			unit.State = computepb.UnitState_UNIT_ASSIGNED
			unit.NodeId = endpoint
			unit.LeaseOwner = fmt.Sprintf("node:%s/lease:%d", endpoint, leaseID)
			unit.LeaseExpiresAt = timestamppb.New(time.Now().Add(leaseTTL * time.Second))
			_ = putUnit(ctx, unit)

			// Stage remotely.
			stageReq := computeRunnerStageRequest(unit.UnitId, jobID, def, unit, job.Spec)
			client, conn, err := runnerClient(endpoint)
			if err != nil {
				slog.Warn("compute dispatch: dial failed",
					"unit_id", unit.UnitId, "endpoint", endpoint, "err", err)
				continue
			}
			stageResp, err := client.StageComputeUnit(ctx, &stageReq)
			if err != nil {
				conn.Close()
				slog.Warn("compute dispatch: stage failed",
					"unit_id", unit.UnitId, "err", err)
				continue
			}

			// Run remotely (async on the runner side).
			runReq := computeRunnerRunRequest(unit.UnitId, jobID, def, unit, job.Spec, stageResp.StagingPath)
			runReq.EtcdLeaseId = int64(leaseID)
			_, err = client.RunComputeUnit(ctx, &runReq)
			conn.Close()
			if err != nil {
				slog.Warn("compute dispatch: run failed",
					"unit_id", unit.UnitId, "err", err)
				continue
			}

			dispatched++
			slog.Info("compute dispatch: unit started",
				"unit_id", unit.UnitId, "endpoint", endpoint)
		}

		slog.Info("compute dispatch: all units dispatched",
			"job_id", jobID, "dispatched", dispatched, "total", len(units))
		return &engine.ActionResult{
			OK:     true,
			Output: map[string]any{"dispatched": dispatched, "total": len(units)},
		}, nil
	}
}

// computeAwaitAllUnits waits for all units in the job to reach a terminal state.
func computeAwaitAllUnits(srv *server) engine.ActionHandler {
	return func(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
		jobID, _ := req.With["job_id"].(string)
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
				return nil, fmt.Errorf("units did not complete within %ds", timeoutSec)
			}

			units, err := listUnits(ctx, jobID)
			if err != nil {
				return nil, err
			}

			allTerminal := true
			succeeded := 0
			failed := 0
			for _, u := range units {
				switch u.State {
				case computepb.UnitState_UNIT_SUCCEEDED:
					succeeded++
				case computepb.UnitState_UNIT_FAILED, computepb.UnitState_UNIT_CANCELLED, computepb.UnitState_UNIT_LEASE_EXPIRED:
					failed++
				default:
					allTerminal = false
				}
			}

			if allTerminal {
				slog.Info("compute workflow: all units terminal",
					"job_id", jobID, "succeeded", succeeded, "failed", failed)
				return &engine.ActionResult{
					OK: true,
					Output: map[string]any{
						"total":     len(units),
						"succeeded": succeeded,
						"failed":    failed,
					},
				}, nil
			}

			time.Sleep(2 * time.Second)
		}
	}
}

// ─── Unit execution handlers ─────────────────────────────────────────────────

func computeChooseNode(srv *server) engine.ActionHandler {
	return func(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
		jobID, _ := req.With["job_id"].(string)

		// Load definition to check allowed_node_profiles.
		var allowedProfiles []string
		if jobID != "" {
			job, _ := getJob(ctx, jobID)
			if job != nil && job.Spec != nil {
				def, _ := getDefinition(ctx, job.Spec.DefinitionName, job.Spec.DefinitionVersion)
				if def != nil {
					allowedProfiles = def.AllowedNodeProfiles
				}
			}
		}

		// Discover all nodes running the compute service via etcd.
		endpoints := resolveComputeEndpoints()
		if len(endpoints) == 0 {
			return nil, fmt.Errorf("no compute service instances available")
		}

		// Filter by allowed profiles if specified.
		// For v1, if profiles are specified but we can't resolve node metadata,
		// we proceed with all candidates (best-effort).
		if len(allowedProfiles) > 0 {
			slog.Info("compute workflow: filtering by profiles",
				"allowed", allowedProfiles, "candidates", len(endpoints))
		}

		// Round-robin across available endpoints for spread placement.
		idx := roundRobinCounter.Add(1) - 1
		chosen := endpoints[int(idx)%len(endpoints)]
		slog.Info("compute workflow: node chosen",
			"endpoint", chosen, "candidates", len(endpoints))
		return &engine.ActionResult{
			OK: true,
			Output: map[string]any{
				"node_id":         chosen,
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

		// Acquire an etcd TTL lease for exclusive unit ownership.
		leaseID, err := grantUnitLease(ctx, jobID, unitID, nodeID)
		if err != nil {
			return nil, fmt.Errorf("grant lease: %w", err)
		}

		unit, err := getUnit(ctx, jobID, unitID)
		if err != nil || unit == nil {
			revokeUnitLease(leaseID)
			return nil, fmt.Errorf("unit %s not found", unitID)
		}
		unit.State = computepb.UnitState_UNIT_ASSIGNED
		unit.NodeId = nodeID
		unit.LeaseOwner = fmt.Sprintf("node:%s/lease:%d", nodeID, leaseID)
		unit.LeaseExpiresAt = timestamppb.New(time.Now().Add(leaseTTL * time.Second))
		if err := putUnit(ctx, unit); err != nil {
			revokeUnitLease(leaseID)
			return nil, fmt.Errorf("update unit: %w", err)
		}
		return &engine.ActionResult{
			OK: true,
			Output: map[string]any{
				"runner_endpoint": runnerEndpoint,
				"lease_id":        int64(leaseID),
			},
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
		leaseIDf, _ := req.With["lease_id"].(float64) // JSON numbers are float64

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
		runReq.EtcdLeaseId = int64(leaseIDf)

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
		leaseCheckInterval := 10 * time.Second
		lastLeaseCheck := time.Now()

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
						"unit_state":     "UNIT_FAILED",
						"failure_class":  unit.FailureClass.String(),
						"failure_reason": unit.FailureReason,
					},
				}, nil
			case computepb.UnitState_UNIT_CANCELLED:
				return &engine.ActionResult{
					OK:     true,
					Output: map[string]any{"unit_state": "UNIT_CANCELLED"},
				}, nil
			case computepb.UnitState_UNIT_LEASE_EXPIRED:
				return &engine.ActionResult{
					OK:     true,
					Output: map[string]any{"unit_state": "UNIT_LEASE_EXPIRED"},
				}, nil
			}

			// Periodically check if the lease is still alive.
			if time.Since(lastLeaseCheck) >= leaseCheckInterval {
				if unit.State == computepb.UnitState_UNIT_RUNNING && !isLeaseAlive(ctx, jobID, unitID) {
					slog.Warn("compute workflow: lease expired for running unit",
						"job_id", jobID, "unit_id", unitID)
					unit.State = computepb.UnitState_UNIT_LEASE_EXPIRED
					unit.FailureReason = "lease expired — runner may have died"
					unit.EndTime = timestamppb.Now()
					_ = putUnit(ctx, unit)
					return &engine.ActionResult{
						OK:     true,
						Output: map[string]any{"unit_state": "UNIT_LEASE_EXPIRED"},
					}, nil
				}
				lastLeaseCheck = time.Now()
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
