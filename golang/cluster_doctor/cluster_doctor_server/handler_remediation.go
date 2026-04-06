package main

import (
	"context"
	"log/slog"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ExecuteRemediation runs a structured RemediationAction attached to a
// previously-seen finding. See executor.go for the allowlist/blocklist
// enforcement and projection-clauses.md Clause 8 for the contract.
//
// Flow:
//  1. Look up finding by ID in the last-report cache.
//  2. Select remediation step by index.
//  3. Hard blocklist check — never auto-executable types reject immediately.
//  4. Risk + path checks — requires approval? Is token valid?
//  5. Execute (or dry-run) via the ActionExecutor.
//  6. Write audit record.
func (s *ClusterDoctorServer) ExecuteRemediation(ctx context.Context, req *cluster_doctorpb.ExecuteRemediationRequest) (*cluster_doctorpb.ExecuteRemediationResponse, error) {
	// Remediation is a side-effecting operation — leader only.
	if !s.isAuthoritative.Load() {
		return nil, status.Error(codes.FailedPrecondition, "not leader: remediation requires the authoritative doctor instance")
	}
	findingID := req.GetFindingId()
	if findingID == "" {
		return nil, status.Error(codes.InvalidArgument, "finding_id is required")
	}

	// Look up finding in the last-report cache.
	s.lastFindingsMu.RLock()
	cached := make([]rules.Finding, len(s.lastFindings))
	copy(cached, s.lastFindings)
	s.lastFindingsMu.RUnlock()

	f, ok := rules.FindByID(cached, findingID)
	if !ok {
		return nil, status.Errorf(codes.NotFound,
			"finding %s not found in last snapshot; call GetClusterReport first",
			findingID)
	}

	// Select the step.
	steps := f.Remediation
	idx := int(req.GetStepIndex())
	if idx < 0 || idx >= len(steps) {
		return nil, status.Errorf(codes.InvalidArgument,
			"step_index %d out of range (finding has %d steps)", idx, len(steps))
	}
	step := steps[idx]
	action := step.GetAction()
	if action == nil {
		return &cluster_doctorpb.ExecuteRemediationResponse{
			Executed: false,
			Status:   "rejected",
			Reason:   "remediation step has no structured action — only text guidance available",
		}, nil
	}

	subject := callerSubject(ctx)
	audit := RemediationAudit{
		FindingID:  findingID,
		StepIndex:  req.GetStepIndex(),
		ActionType: action.GetActionType().String(),
		Risk:       action.GetRisk().String(),
		DryRun:     req.GetDryRun(),
		Subject:    subject,
		Params:     action.GetParams(),
	}

	// Hard blocklist — never run.
	if blocked, reason := hardBlocked(action); blocked {
		audit.Rejected = true
		audit.Reason = reason
		id := auditRemediation(ctx, audit)
		return &cluster_doctorpb.ExecuteRemediationResponse{
			Executed: false,
			Status:   "rejected",
			Reason:   reason,
			AuditId:  id,
		}, nil
	}

	// Approval check.
	if needs, reason := requiresApproval(action); needs {
		if req.GetApprovalToken() == "" {
			audit.Rejected = true
			audit.Reason = reason
			id := auditRemediation(ctx, audit)
			return &cluster_doctorpb.ExecuteRemediationResponse{
				Executed: false,
				Status:   "rejected",
				Reason:   reason,
				AuditId:  id,
			}, nil
		}
		// Verify the token. Today this is a placeholder — a fuller impl
		// would check a signed JWT or an operator-issued one-time token.
		if !isValidApprovalToken(ctx, req.GetApprovalToken(), findingID, subject) {
			audit.Rejected = true
			audit.Reason = "approval_token invalid"
			id := auditRemediation(ctx, audit)
			return &cluster_doctorpb.ExecuteRemediationResponse{
				Executed: false,
				Status:   "rejected",
				Reason:   "approval_token invalid",
				AuditId:  id,
			}, nil
		}
	}

	// Execute (or dry-run).
	output, err := s.executor.Execute(ctx, action, req.GetDryRun())
	if err != nil {
		audit.Reason = err.Error()
		id := auditRemediation(ctx, audit)
		slog.Warn("remediation execute failed",
			"finding_id", findingID,
			"action_type", action.GetActionType().String(),
			"err", err,
		)
		return &cluster_doctorpb.ExecuteRemediationResponse{
			Executed: false,
			Status:   "rejected",
			Reason:   err.Error(),
			AuditId:  id,
		}, nil
	}

	audit.Executed = !req.GetDryRun()
	id := auditRemediation(ctx, audit)
	stat := "executed"
	if req.GetDryRun() {
		stat = "dry_run_ok"
	}
	slog.Info("remediation executed",
		"finding_id", findingID,
		"action_type", action.GetActionType().String(),
		"dry_run", req.GetDryRun(),
		"subject", subject,
	)
	return &cluster_doctorpb.ExecuteRemediationResponse{
		Executed: audit.Executed,
		Status:   stat,
		Output:   output,
		AuditId:  id,
	}, nil
}

// callerSubject extracts the calling principal's identity for audit logging.
// Falls back to "unknown" if auth context is unavailable. Callers should
// have passed through the standard gRPC auth interceptor.
func callerSubject(ctx context.Context) string {
	// Placeholder — in a fuller impl this would read from the auth context
	// injected by the interceptor chain. For now, return a static "system"
	// label; the audit log also records token presence.
	_ = ctx
	return "system"
}

// isValidApprovalToken verifies the operator-issued approval token. For the
// initial rollout this accepts any non-empty token but records it in the
// audit log. A fuller implementation would verify JWT + scope + expiry.
func isValidApprovalToken(ctx context.Context, token, findingID, subject string) bool {
	_ = ctx
	_ = findingID
	_ = subject
	return token != ""
}
