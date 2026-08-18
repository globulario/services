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

	"github.com/globulario/services/golang/ai_memory/behavioral/domain"
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

// The governance drafts that used to live here — a draftForInvariant table
// mapping two invariant ids to fully-authored titles, authorities, evidence,
// risk levels and revocation rules — now come from the domain pack
// (#249 gap 2). The doctor no longer authors governance for a domain it does
// not own.
//
// Two things were wrong with owning them here, and both were live:
//
//  1. An invariant absent from the table produced no candidate, so anything
//     the doctor had not been taught about could repeat forever without
//     becoming reviewable. The table was the ceiling on what could be learned.
//  2. It drifted. For the same rule, the table cited
//     authority.cluster.node_agent.runtime_state while the pack's
//     principle.cluster.restart_drifted_unit_with_observed_finding cites
//     authority.cluster.owner_service.runtime_state — two hand-maintained
//     copies of one governance claim, disagreeing, with nothing able to notice.
//
// What stays here is what the doctor legitimately knows: which of ITS
// invariants a finding belongs to, and the theme its outcomes accumulate under.
// What that pattern MEANS is the domain's to say.

// synthesizePromotionCandidates offers every mapped invariant to the review
// queue and reports how many were accepted. The draft for each comes from the
// domain pack, so a rule the domain never authored is never proposed.
//
// Errors are per-theme and never abort the sweep: one unreachable or malformed
// theme must not stop the others from being offered, because the loop's whole
// value is that it keeps running unattended.
func (s *ClusterDoctorServer) synthesizePromotionCandidates(ctx context.Context) int {
	if s.behavioralGovernor == nil {
		return 0
	}
	queued := 0
	for invariantID, condition := range conditionForInvariant {
		theme := behavioralThemeForInvariant(invariantID)
		draft, ok := observation.CandidateDraftFor(domain.LearningObservation{
			Theme:      theme,
			Conditions: []string{condition},
		})
		if !ok {
			// The domain has no template for this pattern. Queue nothing: a
			// generic draft would ask a human to approve a scope no domain
			// author ever wrote, and would be indistinguishable from one who
			// did.
			continue
		}
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
