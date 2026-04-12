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
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
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
		nodes := resolveComputeNodes()
		if len(nodes) == 0 {
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

			// Round-robin placement across distinct nodes.
			idx := roundRobinCounter.Add(1) - 1
			node := nodes[int(idx)%len(nodes)]
			endpoint := node.Address

			// Grant lease.
			leaseID, err := grantUnitLease(ctx, jobID, unit.UnitId, node.NodeID)
			if err != nil {
				slog.Warn("compute dispatch: lease grant failed",
					"unit_id", unit.UnitId, "err", err)
				continue
			}

			// Mark assigned with explicit node identity.
			unit.State = computepb.UnitState_UNIT_ASSIGNED
			unit.NodeId = node.NodeID
			unit.LeaseOwner = fmt.Sprintf("node:%s/lease:%d", node.NodeID, leaseID)
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
// Failed units are retried if the retry policy allows (bounded by max attempts).
func computeAwaitAllUnits(srv *server) engine.ActionHandler {
	return func(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
		jobID, _ := req.With["job_id"].(string)
		timeoutSec := 3600
		if ts, ok := req.With["timeout_seconds"].(float64); ok {
			timeoutSec = int(ts)
		}

		// Load definition for retry policy.
		var def *computepb.ComputeDefinition
		if job, _ := getJob(ctx, jobID); job != nil && job.Spec != nil {
			def, _ = getDefinition(ctx, job.Spec.DefinitionName, job.Spec.DefinitionVersion)
		}

		// Track which (unit, attempt) pairs we've retried to avoid double-retry
		// but allow re-evaluation after a new attempt fails.
		retriedUnits := map[string]bool{}

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
			retried := 0
			for _, u := range units {
				switch u.State {
				case computepb.UnitState_UNIT_SUCCEEDED:
					succeeded++
				case computepb.UnitState_UNIT_FAILED, computepb.UnitState_UNIT_CANCELLED, computepb.UnitState_UNIT_LEASE_EXPIRED:
					// Check retry policy for failed units.
					retryKey := fmt.Sprintf("%s:%d", u.UnitId, u.Attempt)
					if def != nil && !retriedUnits[retryKey] {
						decision := shouldRetryUnit(def, u)
						logRetryDecision(u, decision)
						if decision.ShouldRetry {
							retriedUnits[retryKey] = true
							go retryUnit(ctx, srv, def, jobID, u)
							retried++
							allTerminal = false
							continue
						}
					}
					failed++
				default:
					allTerminal = false
				}
			}

			if allTerminal {
				slog.Info("compute workflow: all units terminal",
					"job_id", jobID, "succeeded", succeeded,
					"failed", failed, "retried", retried)
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

// retryUnit re-dispatches a failed unit with an incremented attempt counter.
func retryUnit(ctx context.Context, srv *server, def *computepb.ComputeDefinition, jobID string, unit *computepb.ComputeUnit) {
	slog.Info("compute retry: re-dispatching unit",
		"unit_id", unit.UnitId, "job_id", jobID,
		"attempt", unit.Attempt+1,
		"previous_failure", unit.FailureClass.String())

	// Increment attempt and reset state.
	unit.Attempt++
	unit.State = computepb.UnitState_UNIT_PENDING
	unit.FailureClass = computepb.FailureClass_FAILURE_CLASS_UNSPECIFIED
	unit.FailureReason = ""
	unit.ExitStatus = 0
	unit.OutputRef = nil
	unit.Checksum = ""
	unit.StartTime = nil
	unit.EndTime = nil
	unit.ObservedProgress = 0
	if err := putUnit(ctx, unit); err != nil {
		slog.Error("compute retry: failed to reset unit", "unit_id", unit.UnitId, "err", err)
		return
	}

	// Pick a (possibly different) node for the retry.
	nodes := resolveComputeNodes()
	if len(nodes) == 0 {
		slog.Error("compute retry: no compute nodes available")
		unit.State = computepb.UnitState_UNIT_FAILED
		unit.FailureReason = "no compute nodes for retry"
		_ = putUnit(ctx, unit)
		return
	}
	idx := roundRobinCounter.Add(1) - 1
	node := nodes[int(idx)%len(nodes)]

	// Grant lease.
	leaseID, err := grantUnitLease(ctx, jobID, unit.UnitId, node.NodeID)
	if err != nil {
		slog.Error("compute retry: lease grant failed", "unit_id", unit.UnitId, "err", err)
		unit.State = computepb.UnitState_UNIT_FAILED
		unit.FailureReason = fmt.Sprintf("retry lease grant failed: %v", err)
		_ = putUnit(ctx, unit)
		return
	}

	unit.State = computepb.UnitState_UNIT_ASSIGNED
	unit.NodeId = node.NodeID
	unit.LeaseOwner = fmt.Sprintf("node:%s/lease:%d", node.NodeID, leaseID)
	unit.LeaseExpiresAt = timestamppb.New(time.Now().Add(leaseTTL * time.Second))
	_ = putUnit(ctx, unit)

	// Get job spec for stage/run requests.
	job, _ := getJob(ctx, jobID)
	if job == nil {
		return
	}

	// Stage.
	stageReq := computeRunnerStageRequest(unit.UnitId, jobID, def, unit, job.Spec)
	client, conn, err := runnerClient(node.Address)
	if err != nil {
		slog.Error("compute retry: dial failed", "unit_id", unit.UnitId, "err", err)
		unit.State = computepb.UnitState_UNIT_FAILED
		unit.FailureClass = classifyRunFailure(err)
		unit.FailureReason = fmt.Sprintf("retry dial failed: %v", err)
		_ = putUnit(ctx, unit)
		return
	}
	stageResp, err := client.StageComputeUnit(ctx, &stageReq)
	if err != nil {
		conn.Close()
		unit.State = computepb.UnitState_UNIT_FAILED
		unit.FailureClass = classifyStageFailure(err)
		unit.FailureReason = fmt.Sprintf("retry stage failed: %v", err)
		_ = putUnit(ctx, unit)
		return
	}

	// Run.
	runReq := computeRunnerRunRequest(unit.UnitId, jobID, def, unit, job.Spec, stageResp.StagingPath)
	runReq.EtcdLeaseId = int64(leaseID)
	_, err = client.RunComputeUnit(ctx, &runReq)
	conn.Close()
	if err != nil {
		unit.State = computepb.UnitState_UNIT_FAILED
		unit.FailureClass = classifyRunFailure(err)
		unit.FailureReason = fmt.Sprintf("retry run failed: %v", err)
		_ = putUnit(ctx, unit)
		return
	}

	slog.Info("compute retry: unit re-dispatched",
		"unit_id", unit.UnitId, "node", node.Address, "attempt", unit.Attempt)
}

// ─── Unit execution handlers ─────────────────────────────────────────────────

func computeChooseNode(srv *server) engine.ActionHandler {
	return func(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
		jobID, _ := req.With["job_id"].(string)

		// Discover all nodes running the compute service.
		nodes := resolveComputeNodes()
		if len(nodes) == 0 {
			return nil, fmt.Errorf("no compute service instances available")
		}

		// Filter by definition's allowed_node_profiles if specified.
		if jobID != "" {
			if job, _ := getJob(ctx, jobID); job != nil && job.Spec != nil {
				if def, _ := getDefinition(ctx, job.Spec.DefinitionName, job.Spec.DefinitionVersion); def != nil {
					if len(def.AllowedNodeProfiles) > 0 {
						filtered := filterByProfiles(nodes, def.AllowedNodeProfiles)
						if len(filtered) == 0 {
							slog.Warn("compute workflow: no nodes match required profiles, using all",
								"required", def.AllowedNodeProfiles, "available", len(nodes))
						} else {
							slog.Info("compute workflow: profile filter applied",
								"required", def.AllowedNodeProfiles,
								"before", len(nodes), "after", len(filtered))
							nodes = filtered
						}
					}
				}
			}
		}

		// Round-robin across eligible nodes for spread placement.
		idx := roundRobinCounter.Add(1) - 1
		chosen := nodes[int(idx)%len(nodes)]
		slog.Info("compute workflow: node chosen",
			"address", chosen.Address,
			"hostname", chosen.Hostname,
			"profiles", chosen.Profiles,
			"candidates", len(nodes))
		return &engine.ActionResult{
			OK: true,
			Output: map[string]any{
				"node_id":         chosen.NodeID,
				"runner_endpoint": chosen.Address,
				"node_hostname":   chosen.Hostname,
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

		// Collect outputs and checksums from all succeeded units.
		var allChecksums []string
		var lastOutputRef *computepb.ObjectRef
		succeededCount := 0

		var def *computepb.ComputeDefinition
		if job != nil && job.Spec != nil {
			def, _ = getDefinition(ctx, job.Spec.DefinitionName, job.Spec.DefinitionVersion)
		}

		// Run verification on each succeeded unit and collect aggregate checksums.
		allVerified := true
		var worstTrust computepb.ResultTrustLevel
		for _, u := range units {
			if u.State != computepb.UnitState_UNIT_SUCCEEDED {
				continue
			}
			succeededCount++
			if u.Checksum != "" {
				allChecksums = append(allChecksums, u.Checksum)
			}
			lastOutputRef = u.OutputRef

			// Per-unit verification.
			if def != nil {
				stagingPath := "/var/lib/globular/compute/jobs/" + jobID + "/units/" + u.UnitId
				vr := verifyOutput(def, stagingPath, u)
				if !vr.Passed {
					allVerified = false
				}
				if vr.TrustLevel > worstTrust {
					worstTrust = vr.TrustLevel
				}
				slog.Info("compute workflow: unit verification",
					"job_id", jobID, "unit_id", u.UnitId,
					"passed", vr.Passed, "trust", vr.TrustLevel.String())
			}
		}

		// Aggregate trust level: use the weakest verification across units.
		trustLevel := worstTrust
		if trustLevel == computepb.ResultTrustLevel_RESULT_TRUST_LEVEL_UNSPECIFIED {
			trustLevel = computepb.ResultTrustLevel_UNVERIFIED
		}

		// For multi-unit jobs, upload an aggregate manifest to MinIO.
		var aggregateRef *computepb.ObjectRef
		if succeededCount > 1 {
			manifest := buildAggregateManifest(jobID, units)
			ref, err := uploadAggregateManifest(ctx, jobID, manifest)
			if err != nil {
				slog.Warn("compute workflow: aggregate manifest upload failed",
					"job_id", jobID, "err", err)
			} else {
				aggregateRef = ref
			}
		}

		resultRef := aggregateRef
		if resultRef == nil {
			resultRef = lastOutputRef
		}

		result := &computepb.ComputeResult{
			JobId:       jobID,
			ResultRef:   resultRef,
			TrustLevel:  trustLevel,
			Checksums:   allChecksums,
			CompletedAt: timestamppb.Now(),
		}
		if err := putResult(ctx, result); err != nil {
			return nil, fmt.Errorf("store result: %w", err)
		}

		slog.Info("compute workflow: aggregate result created",
			"job_id", jobID, "units_succeeded", succeededCount,
			"trust_level", trustLevel.String(),
			"checksums", len(allChecksums),
			"has_manifest", aggregateRef != nil)

		return &engine.ActionResult{
			OK: true,
			Output: map[string]any{
				"trust_level":         trustLevel.String(),
				"verification_passed": allVerified,
				"units_succeeded":     succeededCount,
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

// ─── Aggregate manifest ──────────────────────────────────────────────────────

type aggregateManifest struct {
	JobID       string              `json:"job_id"`
	CompletedAt string              `json:"completed_at"`
	UnitCount   int                 `json:"unit_count"`
	Units       []unitManifestEntry `json:"units"`
}

type unitManifestEntry struct {
	UnitID      string `json:"unit_id"`
	PartitionID string `json:"partition_id,omitempty"`
	NodeID      string `json:"node_id"`
	State       string `json:"state"`
	OutputURI   string `json:"output_uri,omitempty"`
	Checksum    string `json:"checksum,omitempty"`
	SizeBytes   uint64 `json:"size_bytes,omitempty"`
}

func buildAggregateManifest(jobID string, units []*computepb.ComputeUnit) *aggregateManifest {
	m := &aggregateManifest{
		JobID:       jobID,
		CompletedAt: time.Now().UTC().Format(time.RFC3339),
		UnitCount:   len(units),
	}
	for _, u := range units {
		entry := unitManifestEntry{
			UnitID:      u.UnitId,
			PartitionID: u.PartitionId,
			NodeID:      u.NodeId,
			State:       u.State.String(),
		}
		if u.OutputRef != nil {
			entry.OutputURI = u.OutputRef.Uri
			entry.Checksum = u.OutputRef.Sha256
			entry.SizeBytes = u.OutputRef.SizeBytes
		}
		m.Units = append(m.Units, entry)
	}
	return m
}

func uploadAggregateManifest(ctx context.Context, jobID string, manifest *aggregateManifest) (*computepb.ObjectRef, error) {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal manifest: %w", err)
	}

	// Write manifest to a staging dir and upload via MinIO.
	stagingDir := fmt.Sprintf("/var/lib/globular/compute/jobs/%s/aggregate", jobID)
	outputDir := stagingDir + "/output"
	if err := os.MkdirAll(outputDir, 0750); err != nil {
		return nil, err
	}
	if err := os.WriteFile(outputDir+"/manifest.json", data, 0644); err != nil {
		return nil, err
	}

	return uploadOutput(ctx, stagingDir, jobID, "aggregate")
}
