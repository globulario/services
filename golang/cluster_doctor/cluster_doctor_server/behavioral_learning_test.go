package main

import (
	"context"
	"errors"
	"fmt"
	"testing"

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

	if queued != len(draftForInvariant) {
		t.Errorf("queued = %d, want %d (one per drafted invariant)", queued, len(draftForInvariant))
	}
	if len(gov.candidates) != len(draftForInvariant) {
		t.Fatalf("offered %d candidates, want %d", len(gov.candidates), len(draftForInvariant))
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

	var read string
	for _, c := range gov.candidates {
		if c.Draft.Title == draftForInvariant[invariant].Title {
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
	if len(draftForInvariant) < 2 {
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

	if calls != len(draftForInvariant) {
		t.Errorf("attempted %d themes, want %d — a failure aborted the sweep", calls, len(draftForInvariant))
	}
	if queued != len(draftForInvariant)-1 {
		t.Errorf("queued = %d, want %d (all but the failing one)", queued, len(draftForInvariant)-1)
	}
}

// TestDraftForInvariant_DraftsAreComplete guards the fields the behavioral
// service validates.
//
// An incomplete draft is rejected at the far end, which surfaces as a warning
// nobody reads and a queue that stays empty. Catching it here keeps the failure
// at test time rather than at 3am in a cluster.
func TestDraftForInvariant_DraftsAreComplete(t *testing.T) {
	validRisk := map[string]bool{"info": true, "low": true, "high": true, "irreversible": true}

	for invariantID, d := range draftForInvariant {
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

// TestDraftedInvariantsAreGovernable is the coverage-consistency check.
//
// A drafted invariant that is not in conditionForInvariant can be promoted and
// still never govern anything: CheckAction scopes principles by condition, so
// the promoted rule would sit inert while the operator believes it took effect.
// That is worse than no candidate at all.
func TestDraftedInvariantsAreGovernable(t *testing.T) {
	for invariantID, d := range draftForInvariant {
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
