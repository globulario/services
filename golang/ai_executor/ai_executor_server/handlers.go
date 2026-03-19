package main

import (
	"context"
	"time"

	"github.com/globulario/services/golang/ai_executor/ai_executorpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (srv *server) ProcessIncident(ctx context.Context, req *ai_executorpb.ProcessIncidentRequest) (*ai_executorpb.ProcessIncidentResponse, error) {
	if req.GetIncidentId() == "" {
		return nil, status.Error(codes.InvalidArgument, "incident_id is required")
	}

	srv.statsMu.Lock()
	srv.stats.IncidentsProcessed++
	srv.statsMu.Unlock()

	logger.Info("processing incident",
		"incident_id", req.GetIncidentId(),
		"rule_id", req.GetRuleId(),
		"tier", req.GetTier(),
		"trigger", req.GetTriggerEventName(),
		"batch_size", len(req.GetEventBatch()),
	)

	// Step 1: Diagnose.
	diagnosis, err := srv.diagnoser.diagnose(ctx, req)
	if err != nil {
		logger.Error("diagnosis failed", "incident_id", req.GetIncidentId(), "err", err)
		return nil, status.Errorf(codes.Internal, "diagnosis failed: %v", err)
	}

	srv.statsMu.Lock()
	srv.stats.DiagnosesCompleted++
	srv.statsMu.Unlock()

	// Step 2: Create durable job.
	job := srv.jobStore.createJob(req.GetIncidentId(), req.GetRuleId(), req.GetTier(), diagnosis)

	logger.Info("diagnosis complete",
		"incident_id", req.GetIncidentId(),
		"root_cause", diagnosis.GetRootCause(),
		"confidence", diagnosis.GetConfidence(),
		"proposed_action", diagnosis.GetProposedAction(),
		"job_state", job.GetState().String(),
	)

	// Step 3: Act based on tier.
	var action *ai_executorpb.RemediationAction

	switch req.GetTier() {
	case 0: // OBSERVE — record only
		srv.jobStore.updateState(req.GetIncidentId(), ai_executorpb.JobState_JOB_SUCCEEDED)
		srv.notifier.notify(ctx, buildNotification(job, NotifyResolved))

	case 1: // AUTO_REMEDIATE — execute now
		action = srv.executeJob(ctx, job)

	case 2: // REQUIRE_APPROVAL — notify and wait
		srv.notifier.notify(ctx, buildNotification(job, NotifyApprovalRequired))
	}

	return &ai_executorpb.ProcessIncidentResponse{
		Diagnosis: diagnosis,
		Action:    action,
	}, nil
}

// executeJob runs the proposed action for a job and records the outcome.
func (srv *server) executeJob(ctx context.Context, job *ai_executorpb.Job) *ai_executorpb.RemediationAction {
	srv.jobStore.markExecuting(job.GetIncidentId())

	action := srv.remediator.execute(ctx, job.GetDiagnosis(), job.GetTier())

	if action.GetStatus() == ai_executorpb.ActionStatus_ACTION_SUCCEEDED {
		srv.jobStore.markResult(job.GetIncidentId(), true, "executed successfully", "")
		srv.statsMu.Lock()
		srv.stats.ActionsExecuted++
		srv.statsMu.Unlock()
		srv.notifier.notify(ctx, buildNotification(job, NotifyResolved))
	} else if action.GetStatus() == ai_executorpb.ActionStatus_ACTION_FAILED {
		srv.jobStore.markResult(job.GetIncidentId(), false, "", action.GetError())
		srv.statsMu.Lock()
		srv.stats.ActionsFailed++
		srv.statsMu.Unlock()
		srv.notifier.notify(ctx, buildNotification(job, NotifyFailed))
	} else {
		srv.jobStore.updateState(job.GetIncidentId(), ai_executorpb.JobState_JOB_SUCCEEDED)
	}

	// Track recent actions.
	srv.recentActionsMu.Lock()
	srv.recentActions = append(srv.recentActions, action)
	if len(srv.recentActions) > 100 {
		srv.recentActions = srv.recentActions[len(srv.recentActions)-100:]
	}
	srv.recentActionsMu.Unlock()

	return action
}

// ApproveAction approves a Tier 3 pending action and triggers execution.
func (srv *server) ApproveAction(ctx context.Context, req *ai_executorpb.ApproveActionRequest) (*ai_executorpb.ApproveActionResponse, error) {
	if req.GetIncidentId() == "" {
		return nil, status.Error(codes.InvalidArgument, "incident_id is required")
	}

	job, err := srv.jobStore.approve(req.GetIncidentId(), req.GetApprovedBy())
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "%v", err)
	}

	logger.Info("action approved",
		"incident_id", req.GetIncidentId(),
		"approved_by", req.GetApprovedBy(),
	)

	// Execute the approved action.
	srv.executeJob(ctx, job)

	// Reload job with final state.
	job = srv.jobStore.getJob(req.GetIncidentId())

	return &ai_executorpb.ApproveActionResponse{Job: job}, nil
}

// DenyAction denies a Tier 3 pending action.
func (srv *server) DenyAction(ctx context.Context, req *ai_executorpb.DenyActionRequest) (*ai_executorpb.DenyActionResponse, error) {
	if req.GetIncidentId() == "" {
		return nil, status.Error(codes.InvalidArgument, "incident_id is required")
	}

	job, err := srv.jobStore.deny(req.GetIncidentId(), req.GetDeniedBy(), req.GetReason())
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "%v", err)
	}

	logger.Info("action denied",
		"incident_id", req.GetIncidentId(),
		"denied_by", req.GetDeniedBy(),
		"reason", req.GetReason(),
	)

	srv.notifier.notify(ctx, buildNotification(job, NotifyDenied))

	return &ai_executorpb.DenyActionResponse{Job: job}, nil
}

// RetryAction retries a failed action.
func (srv *server) RetryAction(ctx context.Context, req *ai_executorpb.RetryActionRequest) (*ai_executorpb.RetryActionResponse, error) {
	if req.GetIncidentId() == "" {
		return nil, status.Error(codes.InvalidArgument, "incident_id is required")
	}

	job := srv.jobStore.getJob(req.GetIncidentId())
	if job == nil {
		return nil, status.Errorf(codes.NotFound, "job not found: %s", req.GetIncidentId())
	}
	if job.State != ai_executorpb.JobState_JOB_FAILED {
		return nil, status.Errorf(codes.FailedPrecondition, "job not in failed state: %s", job.State)
	}

	logger.Info("retrying action", "incident_id", req.GetIncidentId(), "attempt", job.Attempts+1)

	srv.executeJob(ctx, job)
	job = srv.jobStore.getJob(req.GetIncidentId())

	return &ai_executorpb.RetryActionResponse{Job: job}, nil
}

func (srv *server) GetJob(_ context.Context, req *ai_executorpb.GetJobRequest) (*ai_executorpb.GetJobResponse, error) {
	job := srv.jobStore.getJob(req.GetIncidentId())
	if job == nil {
		return nil, status.Errorf(codes.NotFound, "job not found: %s", req.GetIncidentId())
	}
	return &ai_executorpb.GetJobResponse{Job: job}, nil
}

func (srv *server) ListJobs(_ context.Context, req *ai_executorpb.ListJobsRequest) (*ai_executorpb.ListJobsResponse, error) {
	jobs := srv.jobStore.listJobs(req.GetStateFilter(), int(req.GetLimit()))
	return &ai_executorpb.ListJobsResponse{Jobs: jobs}, nil
}

func (srv *server) GetDiagnosis(_ context.Context, req *ai_executorpb.GetDiagnosisRequest) (*ai_executorpb.GetDiagnosisResponse, error) {
	job := srv.jobStore.getJob(req.GetIncidentId())
	if job == nil || job.Diagnosis == nil {
		return nil, status.Errorf(codes.NotFound, "no diagnosis for incident %s", req.GetIncidentId())
	}
	return &ai_executorpb.GetDiagnosisResponse{Diagnosis: job.Diagnosis}, nil
}

func (srv *server) GetStatus(_ context.Context, _ *ai_executorpb.GetStatusRequest) (*ai_executorpb.GetStatusResponse, error) {
	srv.statsMu.Lock()
	stats := srv.stats
	srv.statsMu.Unlock()

	return &ai_executorpb.GetStatusResponse{
		IncidentsProcessed: stats.IncidentsProcessed,
		DiagnosesCompleted: stats.DiagnosesCompleted,
		ActionsExecuted:    stats.ActionsExecuted,
		ActionsFailed:      stats.ActionsFailed,
		UptimeSeconds:      int64(time.Since(srv.startedAt).Seconds()),
	}, nil
}

func (srv *server) ListActions(_ context.Context, req *ai_executorpb.ListActionsRequest) (*ai_executorpb.ListActionsResponse, error) {
	srv.recentActionsMu.RLock()
	defer srv.recentActionsMu.RUnlock()

	limit := int(req.GetLimit())
	if limit <= 0 || limit > len(srv.recentActions) {
		limit = len(srv.recentActions)
	}

	start := len(srv.recentActions) - limit
	actions := make([]*ai_executorpb.RemediationAction, limit)
	for i := range limit {
		actions[i] = srv.recentActions[start+limit-1-i]
	}

	return &ai_executorpb.ListActionsResponse{Actions: actions}, nil
}

func (srv *server) Stop(_ context.Context, _ *ai_executorpb.StopRequest) (*ai_executorpb.StopResponse, error) {
	return &ai_executorpb.StopResponse{}, srv.StopService()
}
