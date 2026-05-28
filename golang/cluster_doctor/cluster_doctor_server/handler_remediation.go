package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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

	// Bounded autonomy: even low-risk auto-executable actions must be rate-limited
	// to prevent tight remediation loops when a finding persists.
	if !needsApproval && req.GetApprovalToken() == "" && !req.GetDryRun() {
		recentFailures := countRecentFailedActionAttempts(
			ctx,
			f.InvariantID,
			audit.EvidenceDigest,
			action.GetActionType().String(),
			time.Now().Add(-remediationFailureEscalationWindow),
			500,
		)
		if recentFailures >= remediationFailureEscalationThreshold {
			reason := fmt.Sprintf("auto-remediation escalated after %d failed attempts in %s; operator approval required", recentFailures, remediationFailureEscalationWindow)
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

func digestFindingEvidence(evidence []*cluster_doctorpb.Evidence) string {
	if len(evidence) == 0 {
		return ""
	}
	parts := make([]string, 0, len(evidence))
	for _, ev := range evidence {
		if ev == nil {
			continue
		}
		kvPairs := make([]string, 0, len(ev.GetKeyValues()))
		for k, v := range ev.GetKeyValues() {
			kvPairs = append(kvPairs, k+"="+v)
		}
		sort.Strings(kvPairs)
		timestamp := ""
		if ev.GetTimestamp() != nil {
			timestamp = fmt.Sprintf("%d", ev.GetTimestamp().GetSeconds())
		}
		parts = append(parts,
			ev.GetSourceService()+"|"+ev.GetSourceRpc()+"|"+timestamp+"|"+strings.Join(kvPairs, ","),
		)
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return "sha256:" + hex.EncodeToString(sum[:])
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
