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

	// Step 1: Diagnose — gather evidence and determine root cause.
	diagnosis, err := srv.diagnoser.diagnose(ctx, req)
	if err != nil {
		logger.Error("diagnosis failed", "incident_id", req.GetIncidentId(), "err", err)
		return nil, status.Errorf(codes.Internal, "diagnosis failed: %v", err)
	}

	srv.statsMu.Lock()
	srv.stats.DiagnosesCompleted++
	srv.statsMu.Unlock()

	// Cache diagnosis.
	srv.diagnosesMu.Lock()
	srv.diagnoses[req.GetIncidentId()] = diagnosis
	srv.diagnosesMu.Unlock()

	logger.Info("diagnosis complete",
		"incident_id", req.GetIncidentId(),
		"root_cause", diagnosis.GetRootCause(),
		"confidence", diagnosis.GetConfidence(),
		"proposed_action", diagnosis.GetProposedAction(),
	)

	// Step 2: Remediate based on tier.
	action := srv.remediator.execute(ctx, diagnosis, req.GetTier())

	// Track action.
	if action.GetStatus() == ai_executorpb.ActionStatus_ACTION_SUCCEEDED {
		srv.statsMu.Lock()
		srv.stats.ActionsExecuted++
		srv.statsMu.Unlock()
	} else if action.GetStatus() == ai_executorpb.ActionStatus_ACTION_FAILED {
		srv.statsMu.Lock()
		srv.stats.ActionsFailed++
		srv.statsMu.Unlock()
	}

	srv.recentActionsMu.Lock()
	srv.recentActions = append(srv.recentActions, action)
	if len(srv.recentActions) > 100 {
		srv.recentActions = srv.recentActions[len(srv.recentActions)-100:]
	}
	srv.recentActionsMu.Unlock()

	return &ai_executorpb.ProcessIncidentResponse{
		Diagnosis: diagnosis,
		Action:    action,
	}, nil
}

func (srv *server) GetDiagnosis(_ context.Context, req *ai_executorpb.GetDiagnosisRequest) (*ai_executorpb.GetDiagnosisResponse, error) {
	srv.diagnosesMu.RLock()
	d, ok := srv.diagnoses[req.GetIncidentId()]
	srv.diagnosesMu.RUnlock()

	if !ok {
		return nil, status.Errorf(codes.NotFound, "no diagnosis for incident %s", req.GetIncidentId())
	}
	return &ai_executorpb.GetDiagnosisResponse{Diagnosis: d}, nil
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

	// Return most recent first.
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
