package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/evidencedigest"
	"github.com/globulario/services/golang/evidence"
	"github.com/globulario/services/golang/remediation"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// approvalReplayStore tracks single-use approval-token jtis. Backed by
// etcd so the single-use guarantee survives doctor restart and leader
// failover — see approval_replay_etcd.go. The doctor is leader-only
// (see ExecuteRemediation guard) but a leadership transition between
// mint and validate must not let a token be replayed against the new
// leader.
var approvalReplayStore security.ReplayStore = newEtcdReplayStore()

const autoRemediationCooldown = 60 * time.Second

var autoRemediationCooldownByTarget sync.Map // key -> time.Time

func autoRemediationCooldownKey(findingID string, stepIndex uint32, actionType cluster_doctorpb.ActionType) string {
	return fmt.Sprintf("%s|%s|%d", findingID, actionType.String(), stepIndex)
}

func allowAutoRemediationNow(key string, now time.Time) (bool, time.Duration) {
	if v, ok := autoRemediationCooldownByTarget.Load(key); ok {
		last := v.(time.Time)
		if elapsed := now.Sub(last); elapsed < autoRemediationCooldown {
			return false, autoRemediationCooldown - elapsed
		}
	}
	autoRemediationCooldownByTarget.Store(key, now)
	return true, 0
}

// ExecuteRemediation runs a structured RemediationAction attached to a
// previously-seen finding. See executor.go for the allowlist/blocklist
// enforcement and projection-clauses.md Clause 8 for the contract.
//
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
		FindingID:      findingID,
		CorrelationID:  correlationIDFromContext(ctx, findingID, req.GetStepIndex()),
		WorkflowRunID:  workflowRunIDFromContext(ctx),
		InvariantID:    f.InvariantID,
		FindingSummary: f.Summary,
		EvidenceDigest: digestFindingEvidence(f.Evidence),
		StepIndex:      req.GetStepIndex(),
		ActionType:     action.GetActionType().String(),
		Risk:           action.GetRisk().String(),
		DryRun:         req.GetDryRun(),
		Subject:        subject,
		Params:         action.GetParams(),
	}

	// Runtime policy guardrail: autonomous remediation must carry explicit
	// invariant identity and evidence so every action is causally attributable.
	if f.InvariantID == "" || len(f.Evidence) == 0 {
		reason := "remediation requires finding invariant_id and evidence (policy guardrail)"
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

	// Evidence trust gate: stale or untrusted evidence must not authorize
	// privileged remediation. Dry-runs are allowed through so operators can
	// inspect what would happen. See
	// docs/intent/evidence.provenance_trust_levels.yaml.
	trust := findingEvidenceTrust(f, time.Now())
	audit.EvidenceTrust = string(trust)
	if !req.GetDryRun() && !evidence.AuthorizesRemediation(trust) {
		reason := fmt.Sprintf("remediation blocked: evidence trust=%s (re-collect evidence before retry)", trust)
		audit.Rejected = true
		audit.Reason = reason
		id := auditRemediation(ctx, audit)
		slog.Warn("remediation blocked on weak evidence",
			"finding_id", findingID,
			"trust", trust,
			"action_type", action.GetActionType().String(),
		)
		return &cluster_doctorpb.ExecuteRemediationResponse{
			Executed: false,
			Status:   "rejected",
			Reason:   reason,
			AuditId:  id,
		}, nil
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

	gateKey := remediationGateKey(findingID, req.GetStepIndex(), action.GetActionType())

	// If this action was repeatedly cooldown-rejected, keep autonomy bounded
	// until an operator provides an approval token.
	if gate, ok := remediationGateGet(gateKey); ok && gate.Escalated && req.GetApprovalToken() == "" {
		reason := "auto-remediation escalated after repeated cooldown rejections; operator approval required"
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
	needsApproval, reason := requiresApproval(action)
	if needsApproval {
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
		// Verify the signed approval token. The token is bound to this
		// finding, action class, target entity, and evidence digest so it
		// cannot authorize a different action or be replayed against later
		// evidence. See docs/intent/remediation.token_contract.yaml.
		expect := security.ApprovalExpectation{
			ActionClass: action.GetActionType().String(),
			Target:      approvalTargetForFinding(f),
			Generation:  audit.EvidenceDigest,
			FindingID:   findingID,
		}
		validatedClaims, vErr := security.ValidateApprovalToken(req.GetApprovalToken(), expect, approvalReplayStore)
		if vErr == nil && validatedClaims != nil {
			audit.TokenJTI = validatedClaims.ID
		}
		if vErr != nil {
			rejectReason := "approval_token invalid: " + vErr.Error()
			audit.Rejected = true
			audit.Reason = rejectReason
			id := auditRemediation(ctx, audit)
			slog.Warn("remediation approval token rejected",
				"finding_id", findingID,
				"action_type", action.GetActionType().String(),
				"subject", subject,
				"err", vErr,
			)
			return &cluster_doctorpb.ExecuteRemediationResponse{
				Executed: false,
				Status:   "rejected",
				Reason:   rejectReason,
				AuditId:  id,
			}, nil
		}
	}

	// Bounded autonomy: even low-risk auto-executable actions must be rate-limited
	// to prevent tight remediation loops when a finding persists. The policy
	// is cluster-wide and action-class aware — see
	// docs/intent/remediation.failure_rate_policy.yaml.
	if !needsApproval && req.GetApprovalToken() == "" && !req.GetDryRun() {
		policy := loadFailureRatePolicy(ctx)
		class := remediation.NormalizeActionClass(action.GetActionType().String())
		classPolicy := policy.For(class)
		recentFailures := countRecentFailedActionAttempts(
			ctx,
			f.InvariantID,
			audit.EvidenceDigest,
			action.GetActionType().String(),
			time.Now().Add(-classPolicy.Window),
			500,
		)
		if ok, breakerReason := policy.Allow(class, recentFailures); !ok {
			reason := fmt.Sprintf("auto-remediation escalated: %s — operator approval required", breakerReason)
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

		if ok, wait := allowAutoRemediationNow(gateKey, time.Now()); !ok {
			gate := remediationGateRecordCooldownRejection(gateKey, time.Now())
			reason := "auto-remediation cooldown active (" + wait.Round(time.Second).String() + " remaining)"
			if gate.Escalated {
				reason = "auto-remediation escalated after repeated cooldown rejections; operator approval required"
			}
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
	if req.GetApprovalToken() != "" && !req.GetDryRun() {
		remediationGateClear(gateKey)
	}
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

// digestFindingEvidence delegates to the shared package so the
// server-side audit digest and the CLI mint-approval --generation
// always produce identical bytes for identical input.
func digestFindingEvidence(evidence []*cluster_doctorpb.Evidence) string {
	return evidencedigest.Of(evidence)
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

// approvalTargetForFinding returns the stable target identifier the approval
// token must bind to. Prefers the finding's EntityRef (node/service) so an
// operator approval cannot be replayed against a different entity that
// happens to surface the same finding id later.
func approvalTargetForFinding(f rules.Finding) string {
	if strings.TrimSpace(f.EntityRef) != "" {
		return f.EntityRef
	}
	return f.FindingID
}
