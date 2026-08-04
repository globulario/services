package main

// behavioral_governance.go is the doctor's half of the governed remediation
// path: it asks the behavioral governor whether an action may be dispatched, and
// afterwards records what the action achieved against the decision that allowed
// it.
//
// The briefing for this path is explicit that execution is not proof of repair
// (globular.repair.doctor_finding_requires_remediation: "Confirm convergence
// before declaring the finding resolved"). That is why the check happens BEFORE
// dispatch and the outcome is recorded only AFTER fresh verification — a
// governor consulted after the fact annotates history, and an outcome written at
// dispatch would record an intention as a result.

import (
	"context"
	"fmt"
	"strings"
	"sync"

	observation "github.com/globulario/services/golang/ai_memory/domains/cluster_operator/observation"
	"github.com/globulario/services/golang/remediation"
	"github.com/globulario/services/golang/workflow/engine"
)

// behavioralGovernor is the narrow slice of the governance client this service
// needs, so tests substitute a fake without a connection.
type behavioralGovernor interface {
	CheckAction(context.Context, observation.ActionContext) (observation.GateDecision, error)
	RecordOutcome(context.Context, observation.OutcomeRecord) (string, error)
	// The learning loop's reader — see behavioral_learning.go. Queues a
	// repeated theme for human review; never promotes.
	GeneratePromotionCandidate(context.Context, observation.CandidateRequest) (observation.CandidateResult, error)
}

// conditionForInvariant maps a doctor invariant to the behavioral condition that
// scopes principles about it.
//
// An explicit table, not a derivation. A principle applies to a NAMED condition;
// inferring one by transforming an invariant id would silently engage or
// disengage governance whenever either naming convention shifted, and the
// failure would be invisible — actions would simply stop being governed.
//
// An unmapped invariant yields no condition, which makes the check ungoverned
// rather than wrongly governed. That is the safe direction: an ungoverned action
// keeps its existing safety gates and is counted as a coverage gap.
var conditionForInvariant = map[string]string{
	"node.systemd.units_running":   "condition.cluster.node.systemd_unit_not_running",
	"cluster.desired_state.absent": "condition.cluster.service.desired_observed_mismatch",
}

// gateRemediation answers whether one remediation may be dispatched.
//
// Returns an error when the governor could not decide. The caller (the workflow
// execute actor) treats that as a refusal — never as consent.
func (s *ClusterDoctorServer) gateRemediation(ctx context.Context, req engine.GateRequest) (engine.GateVerdict, error) {
	if s.behavioralGovernor == nil {
		// No governor wired: not governed, not refused. The doctor's own
		// executor gates (risk class, approval token, unit allowlist) still run.
		return engine.GateVerdict{Governed: false, Allowed: true, Status: "ungoverned"}, nil
	}

	// Replay: a resumed or retried run must not mint a second decision for the
	// same action. Returning the first one keeps the audit trail describing one
	// action once, and keeps the ActionCheck id that later evidence cites stable.
	//
	// ONLY for a durable workflow run. Memoization identifies a single durable
	// ATTEMPT, and an autonomous healer dispatch has no WorkflowRunID — so every
	// retry for one finding/step produced the identical key "|<finding>|<step>".
	// The lookup below sits BEFORE CheckAction, so a memo hit returns without
	// re-resolving evidence or principles: a first needs_evidence verdict was
	// replayed for the doctor's whole process lifetime, and the governed branch
	// could never be reached no matter how much evidence later arrived. The
	// mirror case is as bad — an earlier allow would be reused after the
	// governing evidence or principles changed.
	//
	// Autonomous requests are therefore evaluated fresh every dispatch and never
	// written to the memo. Durable runs keep their intended idempotence.
	memoizable := strings.TrimSpace(req.WorkflowRunID) != ""
	key := gateKey(req)
	if memoizable {
		if v, ok := s.lookupGateVerdict(key); ok {
			return v, nil
		}
	}

	cond := conditionForInvariant[req.InvariantID]
	var conditions []string
	if cond != "" {
		conditions = []string{cond}
	}

	dec, err := s.behavioralGovernor.CheckAction(ctx, observation.ActionContext{
		Project:    behavioralProject,
		Domain:     behavioralDomain,
		ActionKind: req.ActionKind,
		Target:     req.EntityRef,
		Conditions: conditions,
		// Subject identity, read from the resolved finding by the caller.
		ClusterID:   req.ClusterID,
		InvariantID: req.InvariantID,
		EntityRef:   req.EntityRef,
		FindingID:   req.FindingID,
		// The action this decision is about, so post-action evidence can be
		// bound back to it.
		WorkflowRunID: req.WorkflowRunID,
		// The operator approval the doctor already holds, if any. Passed so the
		// governor can see it — it does NOT replace the executor's own approval
		// gate, which still runs on the dispatch path.
		HumanApproval: req.ApprovalToken,
	})
	if err != nil {
		return engine.GateVerdict{}, fmt.Errorf("behavioral governor unavailable: %w", err)
	}

	v := engine.GateVerdict{
		ActionCheckID: dec.ActionCheckID,
		Governed:      dec.Governed,
		Allowed:       dec.Allowed,
		Status:        dec.Status,
		Reason:        dec.Reason,
		PrincipleIDs:  dec.PrincipleIDs,
	}
	if !dec.Governed {
		// Visible, because an unmeasured gate reads as a working one. A coverage
		// gap is a fact about the SYSTEM, not about this action.
		behavioralCoverageGapNotify(req, dec.ActionCheckID)
	}
	if memoizable {
		s.rememberGateVerdict(key, v)
	}
	return v, nil
}

// gateKey identifies one governed action. Run + finding + step: the same run
// retrying the same step is the same action, a different run is not.
func gateKey(req engine.GateRequest) string {
	return fmt.Sprintf("%s|%s|%d", req.WorkflowRunID, req.FindingID, req.StepIndex)
}

// Gate memo. Process-local: it makes a replay within one doctor's lifetime reuse
// its decision. A doctor restart between attempts will issue a fresh check —
// truthfully, since the earlier decision is no longer in reach. Durable
// idempotence would need the check keyed by action identity in the store, which
// is a behavioral-memory change, not a doctor one.
var (
	gateMemoMu sync.Mutex
	gateMemo   = map[string]engine.GateVerdict{}
)

func (s *ClusterDoctorServer) lookupGateVerdict(key string) (engine.GateVerdict, bool) {
	gateMemoMu.Lock()
	defer gateMemoMu.Unlock()
	v, ok := gateMemo[key]
	return v, ok
}

func (s *ClusterDoctorServer) rememberGateVerdict(key string, v engine.GateVerdict) {
	gateMemoMu.Lock()
	defer gateMemoMu.Unlock()
	gateMemo[key] = v
}

// resetGateMemo is a test seam.
func resetGateMemo() {
	gateMemoMu.Lock()
	defer gateMemoMu.Unlock()
	gateMemo = map[string]engine.GateVerdict{}
}

var behavioralCoverageGapNotify = func(req engine.GateRequest, checkID string) {
	logger.Info("remediation dispatched UNGOVERNED — no applicable promoted principle",
		"finding_id", req.FindingID,
		"invariant_id", req.InvariantID,
		"entity_ref", req.EntityRef,
		"action_kind", req.ActionKind,
		"action_check_id", checkID,
	)
}

// ── post-action outcome ─────────────────────────────────────────────────────

// behavioralOutcomeStatus maps a remediation result to the governed vocabulary.
//
// The mapping is total and deliberately conservative at the top: only a
// dispatched, verified, resolved AND fully attributable remediation is SUCCESS.
func behavioralOutcomeStatus(o remediation.Outcome, blocked bool) string {
	switch {
	case blocked:
		// A refused action is not a failed repair. Recording it as failure would
		// teach the system that the principle does not work, when in fact it
		// worked exactly as intended.
		return "blocked"
	case !o.Dispatched:
		return "blocked"
	case !o.Verified:
		// Dispatched, no verification. Not success — dispatch is not repair —
		// and not failure either, because nothing was checked.
		return "degraded"
	case o.FindingResolved && o.LineageComplete():
		return "success"
	case o.FindingResolved:
		// The repair worked but cannot be attributed. It must not count as
		// evidence FOR the principle, so it is not success.
		return "degraded"
	default:
		return "failure"
	}
}

// recordGovernedOutcome links the verified result back to the decision that
// allowed it.
//
// Only a verified, attributable success may SUPPORT the principle. Dispatch
// alone never does: that is the whole content of the truth-consistency contract,
// and a governor that learned from dispatches would be reinforcing principles on
// the strength of actions nobody checked.
//
// A failed verification WEAKENS rather than revokes. Revocation is a governed,
// human-gated decision; one failure is evidence, not a verdict on the rule.
func (s *ClusterDoctorServer) recordGovernedOutcome(ctx context.Context, o remediation.Outcome, evidenceID string) {
	if s.behavioralGovernor == nil || o.ActionCheckID == "" {
		// Nothing to link to. An ungoverned remediation still records its
		// evidence through the recorder path; it just has no decision to cite.
		return
	}
	status := behavioralOutcomeStatus(o, false)

	rec := observation.OutcomeRecord{
		Project:       behavioralProject,
		Domain:        behavioralDomain,
		ActionCheckID: o.ActionCheckID,
		PrincipleIDs:  s.principlesForCheck(o.ActionCheckID),
		Status:        status,
		// Shared with the learning loop's reader — see behavioralThemeForInvariant.
		Theme: behavioralThemeForInvariant(o.InvariantID),
		Note:  o.Reason(),
		Metadata: map[string]string{
			"workflow_run_id": o.WorkflowRunID,
			"finding_id":      o.FindingID,
			"invariant_id":    o.InvariantID,
			"entity_ref":      o.EntityRef,
			"cluster_id":      o.ClusterID,
		},
	}
	if evidenceID != "" {
		rec.EvidenceIDs = []string{evidenceID}
	}
	switch status {
	case "success":
		rec.SupportsPrinciples = rec.PrincipleIDs
	case "failure":
		rec.WeakensPrinciples = rec.PrincipleIDs
	}

	if _, err := s.behavioralGovernor.RecordOutcome(ctx, rec); err != nil {
		// Visible and harmless: a lost outcome degrades learning, never the
		// remediation verdict the operator already has.
		behavioralOutcomeRecordFailedNotify(o, err)
	}
}

// principlesForCheck returns the principles the decision was made against.
func (s *ClusterDoctorServer) principlesForCheck(checkID string) []string {
	gateMemoMu.Lock()
	defer gateMemoMu.Unlock()
	for _, v := range gateMemo {
		if v.ActionCheckID == checkID {
			return v.PrincipleIDs
		}
	}
	return nil
}

var behavioralOutcomeRecordFailedNotify = func(o remediation.Outcome, err error) {
	logger.Warn("governed outcome not recorded; learning degraded, remediation verdict unaffected",
		"action_check_id", o.ActionCheckID,
		"finding_id", o.FindingID,
		"status", string(o.Status()),
		"err", err,
	)
}
