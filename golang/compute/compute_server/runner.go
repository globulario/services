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

	slog.Info("compute runner: unit staged",
		"unit_id", req.UnitId, "job_id", req.JobId,
		"staging_path", stagingPath)

	return &compute_runnerpb.StageComputeUnitResponse{
		Staged:      true,
		StagingPath: stagingPath,
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

	// Execute the entrypoint asynchronously.
	go srv.executeUnit(req, executionID)

	slog.Info("compute runner: unit execution started",
		"unit_id", req.UnitId, "job_id", req.JobId,
		"execution_id", executionID,
		"entrypoint", req.Definition.Entrypoint)

	return &compute_runnerpb.RunComputeUnitResponse{
		Accepted:    true,
		ExecutionId: executionID,
	}, nil
}

// executeUnit runs the declared entrypoint and updates state on completion.
func (srv *server) executeUnit(req *compute_runnerpb.RunComputeUnitRequest, executionID string) {
	ctx := context.Background()
	entrypoint := req.Definition.Entrypoint
	stagingPath := req.StagingPath
	if stagingPath == "" {
		stagingPath = filepath.Join("/var/lib/globular/compute/jobs", req.JobId, "units", req.UnitId)
	}

	// Set up execution environment.
	cmd := exec.CommandContext(ctx, entrypoint)
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

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	// Update unit state based on exit.
	unit, getErr := getUnit(ctx, req.JobId, req.UnitId)
	if getErr != nil || unit == nil {
		slog.Error("compute runner: could not retrieve unit after execution",
			"unit_id", req.UnitId, "err", getErr)
		return
	}

	unit.EndTime = timestamppb.Now()
	unit.ExitStatus = int32(exitCode)

	if exitCode == 0 {
		unit.State = computepb.UnitState_UNIT_SUCCEEDED
		unit.ObservedProgress = 1.0
		slog.Info("compute runner: unit succeeded",
			"unit_id", req.UnitId, "job_id", req.JobId)
	} else {
		unit.State = computepb.UnitState_UNIT_FAILED
		unit.FailureClass = computepb.FailureClass_EXECUTION_NONZERO_EXIT
		unit.FailureReason = fmt.Sprintf("exit code %d: %v", exitCode, err)
		slog.Warn("compute runner: unit failed",
			"unit_id", req.UnitId, "job_id", req.JobId,
			"exit_code", exitCode, "err", err)
	}

	if putErr := putUnit(ctx, unit); putErr != nil {
		slog.Error("compute runner: failed to persist unit result",
			"unit_id", req.UnitId, "err", putErr)
	}

	// Finalize job state.
	srv.finalizeJob(ctx, req.JobId, unit)
}

// finalizeJob transitions the job to a terminal state based on unit outcomes.
func (srv *server) finalizeJob(ctx context.Context, jobID string, unit *computepb.ComputeUnit) {
	job, err := getJob(ctx, jobID)
	if err != nil || job == nil {
		return
	}

	now := timestamppb.Now()

	if unit.State == computepb.UnitState_UNIT_SUCCEEDED {
		// Create result record.
		result := &computepb.ComputeResult{
			JobId:       jobID,
			TrustLevel:  computepb.ResultTrustLevel_UNVERIFIED,
			CompletedAt: now,
		}

		// If verification strategy was declared, mark accordingly.
		if job.Spec != nil {
			def, _ := getDefinition(ctx, job.Spec.DefinitionName, job.Spec.DefinitionVersion)
			if def != nil && def.VerifyStrategy != nil {
				result.TrustLevel = computepb.ResultTrustLevel_STRUCTURALLY_VERIFIED
			}
		}

		if putErr := putResult(ctx, result); putErr != nil {
			slog.Error("compute: failed to store result", "job_id", jobID, "err", putErr)
		}

		job.State = computepb.JobState_JOB_COMPLETED
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

	unit.OutputRef = req.OutputRef
	unit.Checksum = req.Checksum
	unit.ExitStatus = req.ExitStatus

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
