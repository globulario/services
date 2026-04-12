// handlers.go implements the ComputeService RPC handlers for the v1 core
// single-unit job path. All orchestration goes through workflows — these
// handlers perform admission, state mutation, and query operations only.
package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/globulario/services/golang/compute/computepb"
	"github.com/gocql/gocql"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ─── Definitions ─────────────────────────────────────────────────────────────

func (srv *server) RegisterComputeDefinition(ctx context.Context, req *computepb.RegisterComputeDefinitionRequest) (*computepb.RegisterComputeDefinitionResponse, error) {
	def := req.GetDefinition()
	if def == nil {
		return nil, fmt.Errorf("definition is required")
	}
	if def.Name == "" {
		return nil, fmt.Errorf("definition name is required")
	}
	if def.Version == "" {
		return nil, fmt.Errorf("definition version is required")
	}
	if def.Entrypoint == "" {
		return nil, fmt.Errorf("definition entrypoint is required")
	}
	if def.RuntimeType == computepb.RuntimeType_RUNTIME_TYPE_UNSPECIFIED {
		return nil, fmt.Errorf("definition runtime_type is required")
	}

	// Default kind to SINGLE_NODE if not specified.
	if def.Kind == computepb.ComputeDefinitionKind_COMPUTE_DEFINITION_KIND_UNSPECIFIED {
		def.Kind = computepb.ComputeDefinitionKind_SINGLE_NODE
	}

	// Default determinism and idempotency.
	if def.DeterminismLevel == computepb.DeterminismLevel_DETERMINISM_LEVEL_UNSPECIFIED {
		def.DeterminismLevel = computepb.DeterminismLevel_NON_DETERMINISTIC
	}
	if def.IdempotencyMode == computepb.IdempotencyMode_IDEMPOTENCY_MODE_UNSPECIFIED {
		def.IdempotencyMode = computepb.IdempotencyMode_SAFE_RETRY
	}

	if err := putDefinition(ctx, def); err != nil {
		return nil, fmt.Errorf("store definition: %w", err)
	}

	slog.Info("compute: definition registered",
		"name", def.Name, "version", def.Version, "kind", def.Kind.String())

	return &computepb.RegisterComputeDefinitionResponse{Definition: def}, nil
}

func (srv *server) GetComputeDefinition(ctx context.Context, req *computepb.GetComputeDefinitionRequest) (*computepb.GetComputeDefinitionResponse, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if req.Version == "" {
		return nil, fmt.Errorf("version is required")
	}

	def, err := getDefinition(ctx, req.Name, req.Version)
	if err != nil {
		return nil, err
	}
	if def == nil {
		return nil, fmt.Errorf("definition %s@%s not found", req.Name, req.Version)
	}
	return &computepb.GetComputeDefinitionResponse{Definition: def}, nil
}

func (srv *server) ListComputeDefinitions(ctx context.Context, req *computepb.ListComputeDefinitionsRequest) (*computepb.ListComputeDefinitionsResponse, error) {
	defs, err := listDefinitions(ctx, req.NamePrefix)
	if err != nil {
		return nil, err
	}
	return &computepb.ListComputeDefinitionsResponse{Definitions: defs}, nil
}

func (srv *server) ValidateComputeDefinition(ctx context.Context, req *computepb.ValidateComputeDefinitionRequest) (*computepb.ValidateComputeDefinitionResponse, error) {
	def := req.GetDefinition()
	if def == nil {
		return &computepb.ValidateComputeDefinitionResponse{
			Valid:  false,
			Errors: []string{"definition is nil"},
		}, nil
	}

	var errors []string
	var warnings []string

	if def.Name == "" {
		errors = append(errors, "name is required")
	}
	if def.Version == "" {
		errors = append(errors, "version is required")
	}
	if def.Entrypoint == "" {
		errors = append(errors, "entrypoint is required")
	}
	if def.RuntimeType == computepb.RuntimeType_RUNTIME_TYPE_UNSPECIFIED {
		errors = append(errors, "runtime_type is required")
	}
	if def.VerifyStrategy == nil {
		warnings = append(warnings, "no verification strategy declared — output will be UNVERIFIED")
	}

	return &computepb.ValidateComputeDefinitionResponse{
		Valid:    len(errors) == 0,
		Errors:   errors,
		Warnings: warnings,
	}, nil
}

// ─── Jobs ────────────────────────────────────────────────────────────────────

func (srv *server) SubmitComputeJob(ctx context.Context, req *computepb.SubmitComputeJobRequest) (*computepb.SubmitComputeJobResponse, error) {
	spec := req.GetSpec()
	if spec == nil {
		return nil, fmt.Errorf("spec is required")
	}
	if spec.DefinitionName == "" {
		return nil, fmt.Errorf("definition_name is required")
	}
	if spec.DefinitionVersion == "" {
		return nil, fmt.Errorf("definition_version is required")
	}

	// Validate definition exists.
	def, err := getDefinition(ctx, spec.DefinitionName, spec.DefinitionVersion)
	if err != nil {
		return nil, fmt.Errorf("lookup definition: %w", err)
	}
	if def == nil {
		return nil, fmt.Errorf("definition %s@%s not found", spec.DefinitionName, spec.DefinitionVersion)
	}

	// Create job.
	jobID := gocql.TimeUUID().String()
	now := timestamppb.Now()

	job := &computepb.ComputeJob{
		JobId:     jobID,
		Spec:      spec,
		State:     computepb.JobState_JOB_PENDING,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := putJob(ctx, job); err != nil {
		return nil, fmt.Errorf("store job: %w", err)
	}

	// For v1 single-unit path: create one unit immediately.
	unitID := gocql.TimeUUID().String()
	unit := &computepb.ComputeUnit{
		UnitId:    unitID,
		JobId:     jobID,
		State:     computepb.UnitState_UNIT_PENDING,
		InputRefs: spec.InputRefs,
		Attempt:   1,
	}

	if err := putUnit(ctx, unit); err != nil {
		return nil, fmt.Errorf("store unit: %w", err)
	}

	// Transition job to ADMITTED.
	job.State = computepb.JobState_JOB_ADMITTED
	job.UpdatedAt = timestamppb.Now()
	if err := putJob(ctx, job); err != nil {
		return nil, fmt.Errorf("update job state: %w", err)
	}

	slog.Info("compute: job submitted",
		"job_id", jobID,
		"unit_id", unitID,
		"definition", spec.DefinitionName+"@"+spec.DefinitionVersion)

	// Dispatch via workflow engine — the workflow service orchestrates the
	// full job lifecycle and calls back to this service for each action.
	go srv.executeViaWorkflow(def, job, unit)

	return &computepb.SubmitComputeJobResponse{Job: job}, nil
}

func (srv *server) GetComputeJob(ctx context.Context, req *computepb.GetComputeJobRequest) (*computepb.GetComputeJobResponse, error) {
	if req.JobId == "" {
		return nil, fmt.Errorf("job_id is required")
	}
	job, err := getJob(ctx, req.JobId)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, fmt.Errorf("job %s not found", req.JobId)
	}
	return &computepb.GetComputeJobResponse{Job: job}, nil
}

func (srv *server) ListComputeJobs(ctx context.Context, req *computepb.ListComputeJobsRequest) (*computepb.ListComputeJobsResponse, error) {
	jobs, err := listJobs(ctx)
	if err != nil {
		return nil, err
	}
	// Apply state filter if specified.
	if req.StateFilter != computepb.JobState_JOB_STATE_UNSPECIFIED {
		filtered := make([]*computepb.ComputeJob, 0)
		for _, j := range jobs {
			if j.State == req.StateFilter {
				filtered = append(filtered, j)
			}
		}
		jobs = filtered
	}
	return &computepb.ListComputeJobsResponse{Jobs: jobs}, nil
}

func (srv *server) CancelComputeJob(ctx context.Context, req *computepb.CancelComputeJobRequest) (*computepb.CancelComputeJobResponse, error) {
	if req.JobId == "" {
		return nil, fmt.Errorf("job_id is required")
	}
	job, err := getJob(ctx, req.JobId)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, fmt.Errorf("job %s not found", req.JobId)
	}

	// Only cancel non-terminal jobs.
	if job.State == computepb.JobState_JOB_COMPLETED ||
		job.State == computepb.JobState_JOB_FAILED ||
		job.State == computepb.JobState_JOB_CANCELLED {
		return &computepb.CancelComputeJobResponse{Job: job}, nil
	}

	job.State = computepb.JobState_JOB_CANCELLED
	job.FailureMessage = req.Reason
	job.UpdatedAt = timestamppb.Now()

	if err := putJob(ctx, job); err != nil {
		return nil, fmt.Errorf("update job: %w", err)
	}

	slog.Info("compute: job cancelled", "job_id", req.JobId, "reason", req.Reason)
	return &computepb.CancelComputeJobResponse{Job: job}, nil
}

func (srv *server) GetComputeResult(ctx context.Context, req *computepb.GetComputeResultRequest) (*computepb.GetComputeResultResponse, error) {
	if req.JobId == "" {
		return nil, fmt.Errorf("job_id is required")
	}
	result, err := getResult(ctx, req.JobId)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("result for job %s not found", req.JobId)
	}
	return &computepb.GetComputeResultResponse{Result: result}, nil
}

// ─── Units ───────────────────────────────────────────────────────────────────

func (srv *server) ListComputeUnits(ctx context.Context, req *computepb.ListComputeUnitsRequest) (*computepb.ListComputeUnitsResponse, error) {
	if req.JobId == "" {
		return nil, fmt.Errorf("job_id is required")
	}
	units, err := listUnits(ctx, req.JobId)
	if err != nil {
		return nil, err
	}
	if req.StateFilter != computepb.UnitState_UNIT_STATE_UNSPECIFIED {
		filtered := make([]*computepb.ComputeUnit, 0)
		for _, u := range units {
			if u.State == req.StateFilter {
				filtered = append(filtered, u)
			}
		}
		units = filtered
	}
	return &computepb.ListComputeUnitsResponse{Units: units}, nil
}

func (srv *server) GetComputeUnit(ctx context.Context, req *computepb.GetComputeUnitRequest) (*computepb.GetComputeUnitResponse, error) {
	if req.UnitId == "" {
		return nil, fmt.Errorf("unit_id is required")
	}
	// Need job_id to look up unit — scan all jobs for now (v1 simplification).
	// In production, we'd have a secondary index or the caller provides job_id.
	return nil, fmt.Errorf("GetComputeUnit requires job context — use ListComputeUnits with job_id instead")
}

