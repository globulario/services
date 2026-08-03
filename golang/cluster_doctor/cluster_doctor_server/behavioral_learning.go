package main

// behavioral_learning.go closes the read half of the learning loop.
//
// The write half already worked: the doctor observes a finding, the workflow
// acts, and the result is recorded as an outcome under a stable theme. Nothing
// ever read those outcomes back, so the accumulated experience sat in ScyllaDB
// and changed nothing. Observed 2026-08-01 on a freshly-bootstrapped cluster:
// 103 evidence rows, 94 signals, 6 action checks, 2 outcomes — and 0 promotion
// candidates, because GeneratePromotionCandidate's only caller was an MCP tool
// no scheduled job ever invoked.
//
// This file turns repeated outcomes into a QUEUED PROMOTION CANDIDATE: a
// human-review entry. It does NOT promote, and must never learn to. Promotion
// stays a human decision (behavioral/core: "never auto-promotes", enforced in
// the service and restated in four files) because a rule that starts governing
// actions without anyone agreeing to it is indistinguishable from drift with
// better bookkeeping.
//
// What the loop produces is therefore a HINT — "this failed N times, here is the
// rule I would propose, promote it if you agree" — the honest output of a system
// that has observed something but has no authority to conclude it.
//
// Leader-only: the synthesizer runs inside the healer loop's isAuthoritative
// gate (globular.repair.doctor_finding_requires_remediation — only the elected
// leader may authorize remediation). Every doctor racing to queue the same
// candidate would multiply one observation into N review items.

import (
	"context"
	"errors"
	"fmt"

	observation "github.com/globulario/services/golang/ai_memory/domains/cluster_operator/observation"
)

// candidateMinRepeats is how many outcomes on one theme justify asking a human
// to look.
//
// Two is deliberate and low. The candidate is a QUESTION, not a conclusion — the
// cost of a premature question is one review, while the cost of a missed pattern
// is the cluster repeating a failure nobody was ever told about. A threshold
// tuned to eliminate false positives would be tuned to stay silent.
const candidateMinRepeats = 2

// candidateActor identifies this synthesizer in the audit trail. A reviewer must
// be able to tell a machine-drafted candidate from a human-authored one without
// reading its content.
const candidateActor = "cluster_doctor.learning_loop"

// draftForInvariant is the governance draft proposed when an invariant's
// remediations keep producing outcomes.
//
// An explicit table, for the same reason conditionForInvariant is one: every ref
// here must resolve in the authority/condition/evidence catalogs, and a derived
// id would silently stop resolving the moment either naming convention shifted.
// The failure would be invisible — candidates would simply stop being generated,
// and an empty review queue reads identically to a healthy cluster.
//
// An invariant absent from this table produces no candidate. That is the safe
// direction: no hint is honest, whereas a hint drafted from guessed refs would
// ask a human to approve a rule whose scope nobody established.
var draftForInvariant = map[string]observation.CandidateDraft{
	"node.systemd.units_running": {
		Title:             "Restart a drifted Globular unit only on an observed finding",
		AppliesWhen:       []string{"condition.cluster.node.systemd_unit_not_running"},
		Authorities:       []string{"authority.cluster.node_agent.runtime_state"},
		RequiredEvidence:  []string{"evidence.doctor.finding_observed"},
		RecommendedAction: "Restart the drifted globular-* unit through the node agent's supervisor path, and only after the finding that names it has been observed.",
		RiskLevel:         "low",
		PromotionReason:   "remediation of this invariant repeated across runs; the action is deterministic, idempotent, and already constrained by the executor's globular-* unit allowlist",
		RevocationRule:    "revoke if restarting a drifted unit stops resolving the finding, or if the executor's unit allowlist is widened beyond globular-*",
	},
	"cluster.desired_state.absent": {
		Title:             "Do not converge a service whose desired state is absent",
		AppliesWhen:       []string{"condition.cluster.service.desired_observed_mismatch"},
		Authorities:       []string{"authority.cluster.cluster_controller.runtime_state"},
		RequiredEvidence:  []string{"evidence.cluster.owner_service.desired_state"},
		RecommendedAction: "Resolve desired state from the controller before acting; absent desired state is a question for the owner, not a licence to converge toward whatever is currently observed.",
		RiskLevel:         "high",
		PromotionReason:   "repeated findings in which observed state was treated as authoritative because desired state could not be resolved",
		RevocationRule:    "revoke if the controller gains an authoritative absent-desired-state representation that makes the distinction unnecessary",
	},
}

// synthesizePromotionCandidates offers every drafted invariant to the review
// queue and reports how many were accepted.
//
// Errors are per-theme and never abort the sweep: one unreachable or malformed
// theme must not stop the others from being offered, because the loop's whole
// value is that it keeps running unattended.
func (s *ClusterDoctorServer) synthesizePromotionCandidates(ctx context.Context) int {
	if s.behavioralGovernor == nil {
		return 0
	}
	queued := 0
	for invariantID, draft := range draftForInvariant {
		theme := behavioralThemeForInvariant(invariantID)
		res, err := s.behavioralGovernor.GeneratePromotionCandidate(ctx, observation.CandidateRequest{
			Project:    behavioralProject,
			Domain:     behavioralDomain,
			Theme:      theme,
			MinRepeats: candidateMinRepeats,
			Actor:      candidateActor,
			Rationale: fmt.Sprintf(
				"remediation outcomes for %q repeated on this cluster; queued for human review by the doctor learning loop",
				invariantID),
			Draft: draft,
		})
		switch {
		case errors.Is(err, observation.ErrInsufficientSupport):
			// The normal case. Silent by design — see ErrInsufficientSupport.
			continue
		case err != nil:
			behavioralCandidateFailedNotify(theme, err)
			continue
		}
		queued++
		behavioralCandidateQueuedNotify(invariantID, res)
	}
	return queued
}

// behavioralThemeForInvariant mirrors the theme recordGovernedOutcome writes.
//
// These two must agree, or the synthesizer reads an empty theme forever while
// outcomes accumulate under a different key — a silent break that produces no
// error anywhere. Centralized here rather than duplicated as a string literal.
func behavioralThemeForInvariant(invariantID string) string {
	return "remediation." + invariantID
}

var behavioralCandidateQueuedNotify = func(invariantID string, res observation.CandidateResult) {
	logger.Info("learning: promotion candidate queued for human review — NOT promoted",
		"invariant_id", invariantID,
		"candidate_id", res.CandidateID,
		"status", res.Status,
		"repeat_count", res.RepeatCount,
		"outcome_count", res.OutcomeCount,
	)
}

var behavioralCandidateFailedNotify = func(theme string, err error) {
	// Visible and harmless: a lost candidate degrades learning, never the
	// remediation verdict the operator already has.
	logger.Warn("learning: promotion candidate not queued; hint lost, cluster behavior unaffected",
		"theme", theme,
		"err", err,
	)
}
