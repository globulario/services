// runner.go implements the ComputeRunnerService RPCs. In v1, the runner
// is embedded in the compute service for single-node execution. It handles
// staging, execution, heartbeats, and atomic output commit.
//
// The runner MUST NOT mutate cluster desired state directly. It only
// executes explicitly assigned units and reports results via CommitComputeOutput.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/globulario/services/golang/compute/computepb"
	"github.com/globulario/services/golang/compute/compute_runnerpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ─── Stage ───────────────────────────────────────────────────────────────────

func (srv *server) StageComputeUnit(ctx context.Context, req *compute_runnerpb.StageComputeUnitRequest) (*compute_runnerpb.StageComputeUnitResponse, error) {
	if req.UnitId == "" || req.JobId == "" {
		return nil, fmt.Errorf("unit_id and job_id are required")
	}
	if req.Definition == nil {
		return nil, fmt.Errorf("definition is required")
	}

	// Create execution directory.
	root := req.ExecutionRoot
	if root == "" {
		root = "/var/lib/globular/compute"
	}
	stagingPath := filepath.Join(root, "jobs", req.JobId, "units", req.UnitId)
	if err := os.MkdirAll(stagingPath, 0750); err != nil {
		return nil, fmt.Errorf("create staging dir: %w", err)
	}

	// Update unit state to STAGING.
	unit, err := getUnit(ctx, req.JobId, req.UnitId)
	if err != nil {
		return nil, fmt.Errorf("get unit: %w", err)
	}
	if unit != nil {
		unit.State = computepb.UnitState_UNIT_STAGING
		if err := putUnit(ctx, unit); err != nil {
			slog.Warn("compute runner: failed to update unit state", "unit_id", req.UnitId, "err", err)
		}
	}

	// Fetch declared input ObjectRefs from MinIO into the staging directory.
	// Inputs come from the unit (which inherits from job spec).
	var inputRefs []*computepb.ObjectRef
	if req.Unit != nil && len(req.Unit.InputRefs) > 0 {
		inputRefs = req.Unit.InputRefs
	} else if req.JobSpec != nil && len(req.JobSpec.InputRefs) > 0 {
		inputRefs = req.JobSpec.InputRefs
	}
	var warnings []string
	if len(inputRefs) > 0 {
		if err := fetchInputRefs(ctx, stagingPath, inputRefs); err != nil {
			return nil, fmt.Errorf("stage inputs: %w", err)
		}
		slog.Info("compute runner: inputs staged from MinIO",
			"unit_id", req.UnitId, "count", len(inputRefs))
	} else {
		warnings = append(warnings, "no input refs declared — staging directory has no inputs")
	}

	slog.Info("compute runner: unit staged",
		"unit_id", req.UnitId, "job_id", req.JobId,
		"staging_path", stagingPath)

	return &compute_runnerpb.StageComputeUnitResponse{
		Staged:      true,
		StagingPath: stagingPath,
		Warnings:    warnings,
	}, nil
}

// ─── Run ─────────────────────────────────────────────────────────────────────

func (srv *server) RunComputeUnit(ctx context.Context, req *compute_runnerpb.RunComputeUnitRequest) (*compute_runnerpb.RunComputeUnitResponse, error) {
	if req.UnitId == "" || req.JobId == "" {
		return nil, fmt.Errorf("unit_id and job_id are required")
	}
	if req.Definition == nil {
		return nil, fmt.Errorf("definition is required")
	}

	executionID := fmt.Sprintf("exec-%s-%d", req.UnitId, time.Now().UnixMilli())

	// Update unit state to RUNNING.
	unit, err := getUnit(ctx, req.JobId, req.UnitId)
	if err != nil {
		return nil, fmt.Errorf("get unit: %w", err)
	}
	if unit != nil {
		unit.State = computepb.UnitState_UNIT_RUNNING
		unit.StartTime = timestamppb.Now()
		if err := putUnit(ctx, unit); err != nil {
			slog.Warn("compute runner: failed to update unit state", "unit_id", req.UnitId, "err", err)
		}
	}

	// Also update job state to RUNNING.
	job, err := getJob(ctx, req.JobId)
	if err == nil && job != nil && job.State == computepb.JobState_JOB_ADMITTED {
		job.State = computepb.JobState_JOB_RUNNING
		job.UpdatedAt = timestamppb.Now()
		_ = putJob(ctx, job)
	}

	// Execute the entrypoint asynchronously with lease renewal.
	leaseID := clientv3.LeaseID(req.EtcdLeaseId)
	go srv.executeUnit(req, executionID, leaseID)

	slog.Info("compute runner: unit execution started",
		"unit_id", req.UnitId, "job_id", req.JobId,
		"execution_id", executionID,
		"entrypoint", req.Definition.Entrypoint)

	return &compute_runnerpb.RunComputeUnitResponse{
		Accepted:    true,
		ExecutionId: executionID,
	}, nil
}

// executeUnit runs the declared entrypoint with heartbeats, lease renewal,
// and cancellation support. Updates state on completion.
func (srv *server) executeUnit(req *compute_runnerpb.RunComputeUnitRequest, executionID string, leaseID clientv3.LeaseID) {
	bgCtx := context.Background()
	entrypoint := req.Definition.Entrypoint
	stagingPath := req.StagingPath
	if stagingPath == "" {
		stagingPath = filepath.Join("/var/lib/globular/compute/jobs", req.JobId, "units", req.UnitId)
	}

	// Start lease renewal if we have a lease.
	if leaseID != 0 {
		cancelRenewal, err := startLeaseRenewal(leaseID)
		if err != nil {
			slog.Warn("compute runner: lease renewal failed to start", "err", err)
		} else {
			defer cancelRenewal()
			defer revokeUnitLease(leaseID)
		}
	}

	// Create a cancellable context for the process.
	execCtx, execCancel := context.WithCancel(bgCtx)
	defer execCancel()

	// Set up execution environment.
	cmd := exec.CommandContext(execCtx, entrypoint)
	cmd.Dir = stagingPath
	cmd.Env = append(os.Environ(),
		"COMPUTE_JOB_ID="+req.JobId,
		"COMPUTE_UNIT_ID="+req.UnitId,
		"COMPUTE_EXECUTION_ID="+executionID,
		"COMPUTE_STAGING_PATH="+stagingPath,
	)

	// Capture output.
	outputPath := filepath.Join(stagingPath, "output")
	_ = os.MkdirAll(outputPath, 0750)
	stdoutFile, _ := os.Create(filepath.Join(stagingPath, "stdout.log"))
	stderrFile, _ := os.Create(filepath.Join(stagingPath, "stderr.log"))
	if stdoutFile != nil {
		cmd.Stdout = stdoutFile
		defer stdoutFile.Close()
	}
	if stderrFile != nil {
		cmd.Stderr = stderrFile
		defer stderrFile.Close()
	}

	slog.Info("compute runner: executing entrypoint",
		"unit_id", req.UnitId, "entrypoint", entrypoint, "dir", stagingPath)

	// Register for cancellation.
	srv.runningUnitsMu.Lock()
	srv.runningUnits[req.UnitId] = &runningUnit{cmd: cmd, cancel: execCancel, leaseID: leaseID}
	srv.runningUnitsMu.Unlock()
	defer func() {
		srv.runningUnitsMu.Lock()
		delete(srv.runningUnits, req.UnitId)
		srv.runningUnitsMu.Unlock()
	}()

	// Start the process.
	if err := cmd.Start(); err != nil {
		srv.handleExecutionFailure(bgCtx, req, -1, err)
		return
	}

	// Wait for process in a goroutine.
	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()

	// Heartbeat loop.
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case err := <-waitCh:
			// Process completed.
			exitCode := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				} else {
					exitCode = -1
				}
			}
			srv.handleExecutionComplete(bgCtx, req, stagingPath, exitCode, err)
			return

		case <-ticker.C:
			// Send heartbeat.
			_ = putHeartbeat(bgCtx, req.JobId, req.UnitId, 0.0)

		case <-execCtx.Done():
			// Cancelled externally.
			slog.Info("compute runner: unit cancelled", "unit_id", req.UnitId)
			return
		}
	}
}

// handleExecutionComplete updates unit/job state after a process exits.
func (srv *server) handleExecutionComplete(ctx context.Context, req *compute_runnerpb.RunComputeUnitRequest, stagingPath string, exitCode int, execErr error) {
	unit, getErr := getUnit(ctx, req.JobId, req.UnitId)
	if getErr != nil || unit == nil {
		slog.Error("compute runner: could not retrieve unit after execution",
			"unit_id", req.UnitId, "err", getErr)
		return
	}

	unit.EndTime = timestamppb.Now()
	unit.ExitStatus = int32(exitCode)

	if exitCode == 0 {
		outputRef, uploadErr := uploadOutput(ctx, stagingPath, req.JobId, req.UnitId)
		if uploadErr != nil {
			slog.Warn("compute runner: output upload failed (non-fatal)",
				"unit_id", req.UnitId, "err", uploadErr)
		}
		unit.OutputRef = outputRef
		if outputRef != nil {
			unit.Checksum = outputRef.Sha256
		}
		unit.State = computepb.UnitState_UNIT_SUCCEEDED
		unit.ObservedProgress = 1.0
		slog.Info("compute runner: unit succeeded",
			"unit_id", req.UnitId, "job_id", req.JobId,
			"has_output", outputRef != nil)
	} else {
		unit.State = computepb.UnitState_UNIT_FAILED
		unit.FailureClass = computepb.FailureClass_EXECUTION_NONZERO_EXIT
		unit.FailureReason = fmt.Sprintf("exit code %d: %v", exitCode, execErr)
		slog.Warn("compute runner: unit failed",
			"unit_id", req.UnitId, "job_id", req.JobId,
			"exit_code", exitCode, "err", execErr)
	}

	if putErr := putUnit(ctx, unit); putErr != nil {
		slog.Error("compute runner: failed to persist unit result",
			"unit_id", req.UnitId, "err", putErr)
	}

	srv.finalizeJob(ctx, req.JobId, unit)
}

// handleExecutionFailure marks a unit as failed when the process can't start.
func (srv *server) handleExecutionFailure(ctx context.Context, req *compute_runnerpb.RunComputeUnitRequest, exitCode int, err error) {
	unit, _ := getUnit(ctx, req.JobId, req.UnitId)
	if unit == nil {
		return
	}
	unit.State = computepb.UnitState_UNIT_FAILED
	unit.ExitStatus = int32(exitCode)
	unit.FailureClass = computepb.FailureClass_EXECUTION_NONZERO_EXIT
	unit.FailureReason = fmt.Sprintf("start failed: %v", err)
	unit.EndTime = timestamppb.Now()
	_ = putUnit(ctx, unit)
	srv.finalizeJob(ctx, req.JobId, unit)
}

// finalizeJob transitions the job to a terminal state based on unit outcomes
// and verification results. For multi-unit jobs, only finalizes when ALL
// units have reached terminal state — otherwise the workflow's
// await_all_units + aggregate steps handle finalization.
func (srv *server) finalizeJob(ctx context.Context, jobID string, unit *computepb.ComputeUnit) {
	job, err := getJob(ctx, jobID)
	if err != nil || job == nil {
		return
	}

	// Check if there are other non-terminal units — if so, don't finalize.
	// The workflow aggregate step handles multi-unit finalization.
	allUnits, _ := listUnits(ctx, jobID)
	if len(allUnits) > 1 {
		for _, u := range allUnits {
			if u.UnitId == unit.UnitId {
				continue
			}
			switch u.State {
			case computepb.UnitState_UNIT_SUCCEEDED, computepb.UnitState_UNIT_FAILED,
				computepb.UnitState_UNIT_CANCELLED, computepb.UnitState_UNIT_LEASE_EXPIRED:
				// terminal
			default:
				// Non-terminal unit still exists — defer to workflow aggregation.
				slog.Info("compute runner: deferring job finalization (other units still running)",
					"job_id", jobID, "unit_id", unit.UnitId,
					"pending_unit", u.UnitId, "pending_state", u.State.String())
				return
			}
		}
	}

	now := timestamppb.Now()

	if unit.State == computepb.UnitState_UNIT_SUCCEEDED {
		// Run verification if the definition declares a strategy.
		var vResult verificationResult
		if job.Spec != nil {
			def, _ := getDefinition(ctx, job.Spec.DefinitionName, job.Spec.DefinitionVersion)
			if def != nil {
				stagingPath := filepath.Join("/var/lib/globular/compute/jobs", jobID, "units", unit.UnitId)
				vResult = verifyOutput(def, stagingPath, unit)
				slog.Info("compute: verification completed",
					"job_id", jobID, "passed", vResult.Passed,
					"trust_level", vResult.TrustLevel.String(),
					"message", vResult.Message)
			}
		}
		if vResult.TrustLevel == computepb.ResultTrustLevel_RESULT_TRUST_LEVEL_UNSPECIFIED {
			vResult.TrustLevel = computepb.ResultTrustLevel_UNVERIFIED
			vResult.Passed = true
		}

		result := &computepb.ComputeResult{
			JobId:       jobID,
			ResultRef:   unit.OutputRef,
			TrustLevel:  vResult.TrustLevel,
			Checksums:   vResult.Checksums,
			Metadata:    verificationMetadataToStruct(vResult.Metadata),
			CompletedAt: now,
		}
		if putErr := putResult(ctx, result); putErr != nil {
			slog.Error("compute: failed to store result", "job_id", jobID, "err", putErr)
		}

		// Terminal state depends on verification: if verification was declared
		// and failed, the job fails even though execution succeeded.
		if vResult.Passed {
			job.State = computepb.JobState_JOB_COMPLETED
		} else {
			job.State = computepb.JobState_JOB_FAILED
			job.FailureMessage = "verification failed: " + vResult.Message
		}
	} else {
		job.State = computepb.JobState_JOB_FAILED
		job.FailureMessage = unit.FailureReason
	}

	job.UpdatedAt = now
	if putErr := putJob(ctx, job); putErr != nil {
		slog.Error("compute: failed to finalize job", "job_id", jobID, "err", putErr)
	}

	slog.Info("compute: job finalized",
		"job_id", jobID, "state", job.State.String())
}

// ─── Heartbeat ───────────────────────────────────────────────────────────────

func (srv *server) ReportComputeHeartbeat(ctx context.Context, req *compute_runnerpb.ReportComputeHeartbeatRequest) (*compute_runnerpb.ReportComputeHeartbeatResponse, error) {
	if req.UnitId == "" || req.JobId == "" {
		return nil, fmt.Errorf("unit_id and job_id are required")
	}

	// Update unit progress.
	unit, err := getUnit(ctx, req.JobId, req.UnitId)
	if err != nil {
		return nil, fmt.Errorf("get unit: %w", err)
	}
	if unit == nil {
		return &compute_runnerpb.ReportComputeHeartbeatResponse{Ok: false, ShouldCancel: true}, nil
	}

	// Check if job was cancelled.
	job, _ := getJob(ctx, req.JobId)
	if job != nil && job.State == computepb.JobState_JOB_CANCELLED {
		return &compute_runnerpb.ReportComputeHeartbeatResponse{Ok: true, ShouldCancel: true}, nil
	}

	unit.ObservedProgress = req.Progress
	if err := putUnit(ctx, unit); err != nil {
		slog.Warn("compute runner: heartbeat persist failed", "unit_id", req.UnitId, "err", err)
	}

	return &compute_runnerpb.ReportComputeHeartbeatResponse{Ok: true}, nil
}

// ─── Cancel ──────────────────────────────────────────────────────────────────

func (srv *server) CancelComputeUnit(ctx context.Context, req *compute_runnerpb.CancelComputeUnitRequest) (*compute_runnerpb.CancelComputeUnitResponse, error) {
	slog.Info("compute runner: cancel requested", "unit_id", req.UnitId, "reason", req.Reason)

	srv.runningUnitsMu.Lock()
	ru, ok := srv.runningUnits[req.UnitId]
	srv.runningUnitsMu.Unlock()

	if !ok {
		return &compute_runnerpb.CancelComputeUnitResponse{Accepted: false}, nil
	}

	// Kill the process via context cancellation.
	ru.cancel()

	// Update unit state.
	unit, _ := getUnit(ctx, req.JobId, req.UnitId)
	if unit != nil {
		unit.State = computepb.UnitState_UNIT_CANCELLED
		unit.EndTime = timestamppb.Now()
		unit.FailureReason = "cancelled: " + req.Reason
		_ = putUnit(ctx, unit)
	}

	// Revoke lease.
	revokeUnitLease(ru.leaseID)

	slog.Info("compute runner: unit cancelled", "unit_id", req.UnitId, "reason", req.Reason)
	return &compute_runnerpb.CancelComputeUnitResponse{Accepted: true}, nil
}

// ─── Commit ──────────────────────────────────────────────────────────────────

func (srv *server) CommitComputeOutput(ctx context.Context, req *compute_runnerpb.CommitComputeOutputRequest) (*compute_runnerpb.CommitComputeOutputResponse, error) {
	if req.UnitId == "" || req.JobId == "" {
		return nil, fmt.Errorf("unit_id and job_id are required")
	}

	// Atomic output commit: update unit with output ref and mark committed.
	unit, err := getUnit(ctx, req.JobId, req.UnitId)
	if err != nil {
		return nil, fmt.Errorf("get unit: %w", err)
	}
	if unit == nil {
		return nil, fmt.Errorf("unit %s not found", req.UnitId)
	}

	unit.ExitStatus = req.ExitStatus

	// If no output ref was provided by the caller, upload the output
	// directory to MinIO and compute the checksum.
	outputRef := req.OutputRef
	checksum := req.Checksum
	if outputRef == nil && req.ExitStatus == 0 {
		stagingPath := filepath.Join("/var/lib/globular/compute/jobs", req.JobId, "units", req.UnitId)
		uploaded, err := uploadOutput(ctx, stagingPath, req.JobId, req.UnitId)
		if err != nil {
			slog.Warn("compute runner: output upload failed", "unit_id", req.UnitId, "err", err)
		} else if uploaded != nil {
			outputRef = uploaded
			checksum = uploaded.Sha256
		}
	}

	unit.OutputRef = outputRef
	unit.Checksum = checksum

	// Determine unit state from exit status — success is NOT just exit code.
	// The verification strategy from the definition determines trust level.
	if req.ExitStatus == 0 {
		unit.State = computepb.UnitState_UNIT_SUCCEEDED
	} else {
		unit.State = computepb.UnitState_UNIT_FAILED
		unit.FailureClass = computepb.FailureClass_EXECUTION_NONZERO_EXIT
		unit.FailureReason = fmt.Sprintf("exit code %d", req.ExitStatus)
	}
	unit.EndTime = timestamppb.Now()

	if err := putUnit(ctx, unit); err != nil {
		return nil, fmt.Errorf("persist unit: %w", err)
	}

	// Finalize job.
	srv.finalizeJob(ctx, req.JobId, unit)

	slog.Info("compute runner: output committed",
		"unit_id", req.UnitId, "job_id", req.JobId,
		"exit_status", req.ExitStatus)

	return &compute_runnerpb.CommitComputeOutputResponse{
		Committed:   true,
		ResultState: unit.State.String(),
	}, nil
}
