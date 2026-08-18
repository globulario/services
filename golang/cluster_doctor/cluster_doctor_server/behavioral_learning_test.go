package main

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/domain"
	observation "github.com/globulario/services/golang/ai_memory/domains/cluster_operator/observation"
)

// TestLearningLoop_NeverPromotes is the load-bearing test of this file.
//
// The learning loop exists to queue a question, not to answer it. The behavioral
// service refuses to auto-promote, but a caller that reached for PromotePrinciple
// would be asking it to — so the doctor must never acquire that reach in the
// first place. This asserts on the narrow governor interface: if someone widens
// it to include promotion, this fails and they have to justify it.
func TestLearningLoop_NeverPromotes(t *testing.T) {
	gov := &fakeGovernor{}
	var iface behavioralGovernor = gov

	if _, ok := iface.(interface {
		PromotePrinciple(context.Context, string) (string, error)
	}); ok {
		t.Fatal("the doctor's governor interface must NOT expose promotion:\n" +
			"promotion is a human decision (behavioral/core: \"never auto-promotes\").\n" +
			"A loop that can promote its own candidates is drift with better bookkeeping.")
	}
}

// TestSynthesizePromotionCandidates_QueuesDraftedInvariants verifies the loop
// offers every drafted invariant with the review metadata a human needs.
func TestSynthesizePromotionCandidates_QueuesDraftedInvariants(t *testing.T) {
	gov := &fakeGovernor{candResult: observation.CandidateResult{
		CandidateID: "cand-1", Status: "QUEUED", RepeatCount: 3, OutcomeCount: 3,
	}}
	s := &ClusterDoctorServer{behavioralGovernor: gov}

	queued := s.synthesizePromotionCandidates(context.Background())

	if queued != len(conditionForInvariant) {
		t.Errorf("queued = %d, want %d (one per mapped invariant)", queued, len(conditionForInvariant))
	}
	if len(gov.candidates) != len(conditionForInvariant) {
		t.Fatalf("offered %d candidates, want %d", len(gov.candidates), len(conditionForInvariant))
	}
	for _, c := range gov.candidates {
		if c.MinRepeats != candidateMinRepeats {
			t.Errorf("min_repeats = %d, want %d", c.MinRepeats, candidateMinRepeats)
		}
		if c.Actor != candidateActor {
			t.Errorf("actor = %q, want %q — a machine-drafted candidate must be "+
				"distinguishable from a human-authored one", c.Actor, candidateActor)
		}
	}
}

// TestSynthesizePromotionCandidates_ThemeMatchesOutcomeWriter is the silent-break
// guard.
//
// recordGovernedOutcome writes outcomes under one theme and this loop reads them
// back under another. If the two ever diverge, the reader finds an empty theme
// forever while outcomes pile up under a different key — and NOTHING errors. The
// queue just stays empty, which is indistinguishable from a healthy cluster.
func TestSynthesizePromotionCandidates_ThemeMatchesOutcomeWriter(t *testing.T) {
	const invariant = "node.systemd.units_running"

	// What the writer records (behavioral_governance.go recordGovernedOutcome).
	written := behavioralThemeForInvariant(invariant)

	gov := &fakeGovernor{candResult: observation.CandidateResult{CandidateID: "c"}}
	s := &ClusterDoctorServer{behavioralGovernor: gov}
	s.synthesizePromotionCandidates(context.Background())

	// Match on the DRAFT the domain supplies for this invariant's condition, so
	// the assertion proves the right draft travelled under the right theme —
	// not merely that some candidate carried the string.
	want, ok := observation.CandidateDraftFor(domain.LearningObservation{
		Theme:      written,
		Conditions: []string{conditionForInvariant[invariant]},
	})
	if !ok {
		t.Fatalf("no domain template for %s", invariant)
	}
	var read string
	for _, c := range gov.candidates {
		if c.Draft.Title == want.Title {
			read = c.Theme
		}
	}
	if read != written {
		t.Errorf("reader theme %q != writer theme %q\n"+
			"these must agree or the loop reads an empty theme forever with no error", read, written)
	}
}

// TestSynthesizePromotionCandidates_InsufficientSupportIsSilent verifies the
// common case stays quiet.
//
// Most themes never repeat. If under-supported themes surfaced as warnings, the
// loop's output would be mostly noise and operators would learn to ignore the
// one line that matters.
func TestSynthesizePromotionCandidates_InsufficientSupportIsSilent(t *testing.T) {
	gov := &fakeGovernor{candErr: fmt.Errorf("%w: x", observation.ErrInsufficientSupport)}
	s := &ClusterDoctorServer{behavioralGovernor: gov}

	warned := 0
	orig := behavioralCandidateFailedNotify
	behavioralCandidateFailedNotify = func(string, error) { warned++ }
	defer func() { behavioralCandidateFailedNotify = orig }()

	if queued := s.synthesizePromotionCandidates(context.Background()); queued != 0 {
		t.Errorf("queued = %d, want 0", queued)
	}
	if warned != 0 {
		t.Errorf("warned %d times on insufficient support; want 0 — this is the normal case", warned)
	}
}

// TestSynthesizePromotionCandidates_OneFailureDoesNotAbortSweep verifies a bad
// theme cannot silence the others. The loop's value is that it keeps running
// unattended, so a single fault must not end the sweep.
func TestSynthesizePromotionCandidates_OneFailureDoesNotAbortSweep(t *testing.T) {
	if len(conditionForInvariant) < 2 {
		t.Skip("needs at least two drafted invariants to observe a partial sweep")
	}
	calls := 0
	gov := &failOnceGovernor{inner: &fakeGovernor{
		candResult: observation.CandidateResult{CandidateID: "c"},
	}, calls: &calls}
	s := &ClusterDoctorServer{behavioralGovernor: gov}

	orig := behavioralCandidateFailedNotify
	behavioralCandidateFailedNotify = func(string, error) {}
	defer func() { behavioralCandidateFailedNotify = orig }()

	queued := s.synthesizePromotionCandidates(context.Background())

	if calls != len(conditionForInvariant) {
		t.Errorf("attempted %d themes, want %d — a failure aborted the sweep", calls, len(conditionForInvariant))
	}
	if queued != len(conditionForInvariant)-1 {
		t.Errorf("queued = %d, want %d (all but the failing one)", queued, len(conditionForInvariant)-1)
	}
}

// TestDraftForInvariant_DraftsAreComplete guards the fields the behavioral
// service validates.
//
// An incomplete draft is rejected at the far end, which surfaces as a warning
// nobody reads and a queue that stays empty. Catching it here keeps the failure
// at test time rather than at 3am in a cluster.
// domainDrafts resolves what the domain pack now supplies for every invariant
// the doctor maps. These are the drafts that actually reach a reviewer.
func domainDrafts(t *testing.T) map[string]observation.CandidateDraft {
	t.Helper()
	out := map[string]observation.CandidateDraft{}
	for invariantID, cond := range conditionForInvariant {
		d, ok := observation.CandidateDraftFor(domain.LearningObservation{
			Theme:      behavioralThemeForInvariant(invariantID),
			Conditions: []string{cond},
		})
		if !ok {
			t.Errorf("%s (condition %q): the domain supplies no template, so this "+
				"invariant can never produce a candidate", invariantID, cond)
			continue
		}
		out[invariantID] = d
	}
	return out
}

// TestDomainDraftsAreComplete carries forward the completeness contract from the
// deleted draftForInvariant table. Moving authorship into the pack must not
// lower the bar: a draft missing an authority or a revocation rule is not
// reviewable, and a rule with no revocation condition cannot be un-learned.
func TestDomainDraftsAreComplete(t *testing.T) {
	validRisk := map[string]bool{"info": true, "low": true, "high": true, "irreversible": true}

	for invariantID, d := range domainDrafts(t) {
		if d.Title == "" {
			t.Errorf("%s: title is required", invariantID)
		}
		if len(d.AppliesWhen) == 0 {
			t.Errorf("%s: applies_when (condition refs) is required", invariantID)
		}
		if len(d.Authorities) == 0 {
			t.Errorf("%s: authorities is required", invariantID)
		}
		if len(d.RequiredEvidence) == 0 {
			t.Errorf("%s: required_evidence is required", invariantID)
		}
		if d.PromotionReason == "" {
			t.Errorf("%s: promotion_reason is required", invariantID)
		}
		if d.RevocationRule == "" {
			t.Errorf("%s: revocation_rule is required — a rule with no revocation "+
				"condition cannot be un-learned", invariantID)
		}
		if !validRisk[d.RiskLevel] {
			t.Errorf("%s: risk_level %q must be info|low|high|irreversible", invariantID, d.RiskLevel)
		}
	}
}

// TestDomainDraftsAreGovernable is the coverage-consistency check, carried
// forward and now more load-bearing than before.
//
// The draft's applies_when comes from the pack, while the gate's condition comes
// from the doctor. If they diverge, the promoted principle is scoped to a
// condition the gate never declares: it sits inert while the operator believes
// it took effect. That is worse than no candidate at all, and the two sides can
// now drift independently — which is exactly why this must be asserted.
func TestDomainDraftsAreGovernable(t *testing.T) {
	for invariantID, d := range domainDrafts(t) {
		cond, ok := conditionForInvariant[invariantID]
		if !ok {
			t.Errorf("%s has a promotion draft but no conditionForInvariant mapping:\n"+
				"promoting it would produce a principle that never engages", invariantID)
			continue
		}
		found := false
		for _, w := range d.AppliesWhen {
			if w == cond {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: draft applies_when %v does not include the gate's condition %q\n"+
				"the promoted principle would be scoped to a condition the gate never declares",
				invariantID, d.AppliesWhen, cond)
		}
	}
}

// failOnceGovernor fails the first candidate call and delegates the rest.
type failOnceGovernor struct {
	inner *fakeGovernor
	calls *int
}

func (f *failOnceGovernor) CheckAction(ctx context.Context, a observation.ActionContext) (observation.GateDecision, error) {
	return f.inner.CheckAction(ctx, a)
}

func (f *failOnceGovernor) RecordOutcome(ctx context.Context, o observation.OutcomeRecord) (string, error) {
	return f.inner.RecordOutcome(ctx, o)
}

func (f *failOnceGovernor) GeneratePromotionCandidate(ctx context.Context, r observation.CandidateRequest) (observation.CandidateResult, error) {
	*f.calls++
	if *f.calls == 1 {
		return observation.CandidateResult{}, errors.New("boom")
	}
	return f.inner.GeneratePromotionCandidate(ctx, r)
}
