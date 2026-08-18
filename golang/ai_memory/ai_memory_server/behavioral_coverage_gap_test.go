package main

import (
	"context"
	"strings"
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	bpb "github.com/globulario/services/golang/ai_memory/behavioral_memorypb"
)

// This file covers #249 gap 3: repeated ungoverned action checks are measured
// but were not learnable. CheckAction persisted the row and bumped a counter,
// while GeneratePromotionCandidate learned only from outcomes — so the system
// could report "I governed nothing" indefinitely and never turn that into
// review material.
//
// The architectural law the fix must not break:
//
//	observation != authority
//	candidate   != principle
//
// A coverage gap becomes a QUESTION for a human. It never becomes a rule.

// wipeCheck issues one ungoverned check of a fixed shape. Targets differ on
// purpose: the same action against different hosts is one gap.
func wipeCheck(t *testing.T, h *behavioralHandler, target string) *bpb.ActionCheck {
	t.Helper()
	return checkAction(t, h, &bpb.CheckActionRequest{
		ActionType:        "scylla.data.wipe",
		Target:            target,
		CurrentConditions: []string{"condition.scylla.cluster_healthy"},
		AgentId:           "ai-executor",
	})
}

// TestUngovernedCheckIsStampedWithACoverageTheme — without a stable theme the
// row is only a counter increment and no amount of repetition can ever be
// grouped into a reviewable observation.
func TestUngovernedCheckIsStampedWithACoverageTheme(t *testing.T) {
	_, h := newGovHandler()
	ac := wipeCheck(t, h, "globule-ryzen")

	if ac.GetGoverned() {
		t.Fatal("precondition failed: no principle is promoted, so this check must be ungoverned")
	}
	if ac.GetTheme() == "" {
		t.Fatal("an ungoverned check carries no theme — the coverage gap is unlearnable")
	}
	if !strings.HasPrefix(ac.GetTheme(), "coverage.") {
		t.Errorf("theme %q is not marked as derived; a derived grouping key must stay "+
			"distinguishable from a domain-authored outcome theme", ac.GetTheme())
	}
}

// TestRepeatedUngovernedChecksShareOneTheme — the gap must accumulate. If the
// theme varied per host, a problem present on every node would be the LEAST
// likely to reach a repeat threshold.
func TestRepeatedUngovernedChecksShareOneTheme(t *testing.T) {
	st, h := newGovHandler()
	for _, host := range []string{"globule-ryzen", "globule-nuc", "globule-dell"} {
		wipeCheck(t, h, host)
	}

	first := wipeCheck(t, h, "globule-hp-01")
	gaps, err := st.ListUngovernedActionChecksByTheme(
		context.Background(), testProject, testDomain, first.GetTheme())
	if err != nil {
		t.Fatalf("list ungoverned checks: %v", err)
	}
	if len(gaps) != 4 {
		t.Errorf("got %d checks under one theme, want 4 — the same action against "+
			"different targets is one coverage gap, not four", len(gaps))
	}
}

// TestCoverageGapAloneCanQueueACandidate is the acceptance case from #249:
//
//	same ungoverned action/condition occurs N times
//	 -> coverage gap remains visible
//	 -> domain supplies an explicit candidate draft
//	 -> candidate is queued with exact supporting check ids
//	 -> nothing is promoted automatically
//
// Note there are ZERO outcomes here. Before the fix this was refused, which is
// precisely the reported defect: a gap that never produces a result can never
// produce a candidate either, so it stays invisible forever.
func TestCoverageGapAloneCanQueueACandidate(t *testing.T) {
	st, h := newGovHandler()
	ctx := context.Background()

	var checkIDs []string
	for _, host := range []string{"globule-ryzen", "globule-nuc", "globule-dell"} {
		checkIDs = append(checkIDs, wipeCheck(t, h, host).GetId())
	}
	theme := wipeCheck(t, h, "globule-lenovo").GetTheme()
	checkIDs = append(checkIDs, "")

	resp, err := h.GeneratePromotionCandidate(ctx, &bpb.GeneratePromotionCandidateRequest{
		Project: testProject, Domain: testDomain, Theme: theme,
		MinRepeats: 3, Actor: "operator-dave",
		DraftPrinciple: &bpb.Principle{
			Title:            "Never wipe Scylla data without verified authoritative health",
			AppliesWhen:      []string{"condition.scylla.cluster_healthy"},
			Authorities:      []string{"authority.cluster.infra_probe.scylla"},
			RequiredEvidence: []string{"required.cluster.scylla.health_probe"},
			ForbiddenMoves:   []string{"forbidden.scylla.data.wipe"},
			RiskLevel:        "irreversible",
			RevocationRule:   "revoke when a safe, reversible wipe path exists",
			PromotionReason:  "repeated ungoverned wipe checks show no rule reaches this action",
		},
		SupportingEvidenceIds: []string{"ev.scylla.health_probe"},
	})
	if err != nil {
		t.Fatalf("a repeated coverage gap must be able to queue a candidate: %v", err)
	}

	c := resp.GetCandidate()
	if c.GetStatus() != bpb.PromotionCandidateStatus_PROMOTION_CANDIDATE_STATUS_QUEUED {
		t.Errorf("status = %v, want QUEUED", c.GetStatus())
	}
	// Exact supporting ids — the issue asks for citation, not a bare count.
	if len(c.GetSupportingActionCheckIds()) != 4 {
		t.Errorf("candidate cites %d action checks, want 4: %v",
			len(c.GetSupportingActionCheckIds()), c.GetSupportingActionCheckIds())
	}
	for _, want := range checkIDs {
		if want == "" {
			continue
		}
		if !containsStr(c.GetSupportingActionCheckIds(), want) {
			t.Errorf("check %s is not cited by the candidate it helped motivate", want)
		}
	}
	if len(c.GetSupportingOutcomeIds()) != 0 {
		t.Errorf("candidate claims %d supporting outcomes, but none were recorded — "+
			"a pre-action check must never be relabelled as a result",
			len(c.GetSupportingOutcomeIds()))
	}
	if !strings.Contains(c.GetSummary(), "coverage gap") {
		t.Errorf("summary %q does not say the support was a coverage gap; a reviewer "+
			"must be able to tell 'this keeps failing' from 'nothing governs this'",
			c.GetSummary())
	}

	// Nothing is promoted. The draft stays PROPOSED and no principle exists.
	if c.GetDraftPrinciple().GetStatus() != bpb.GovernanceStatus_PROPOSED_PRINCIPLE {
		t.Errorf("draft status = %v, want PROPOSED_PRINCIPLE", c.GetDraftPrinciple().GetStatus())
	}
	promoted, err := st.ListPrincipleIDsByCondition(ctx, testProject, testDomain, "condition.scylla.cluster_healthy")
	if err != nil {
		t.Fatalf("list promoted: %v", err)
	}
	if len(promoted) != 0 {
		t.Errorf("%d principle(s) became promoted from a coverage gap — learning must "+
			"improve the candidate queue, never manufacture authority", len(promoted))
	}
}

// TestCoverageGapStillRequiresExplicitEvidence is the boundary that keeps this
// honest. An ungoverned check proves a gap EXISTS. It proves nothing about what
// the rule should be. If absence of governance could supply its own
// justification, the system would author authority out of its own silence.
func TestCoverageGapStillRequiresExplicitEvidence(t *testing.T) {
	_, h := newGovHandler()
	for _, host := range []string{"a", "b", "c"} {
		wipeCheck(t, h, host)
	}
	theme := wipeCheck(t, h, "d").GetTheme()

	_, err := h.GeneratePromotionCandidate(context.Background(), &bpb.GeneratePromotionCandidateRequest{
		Project: testProject, Domain: testDomain, Theme: theme,
		MinRepeats: 3, Actor: "operator-dave",
		DraftPrinciple: &bpb.Principle{
			Title: "T", AppliesWhen: []string{"condition.scylla.cluster_healthy"},
			Authorities: []string{"auth.a"}, RequiredEvidence: []string{"req.a"},
			RiskLevel: "high", RevocationRule: "r", PromotionReason: "p",
		},
		// No SupportingEvidenceIds, and no outcomes to derive any from.
	})
	if err == nil {
		t.Fatal("a coverage gap with no supplied evidence produced a candidate — " +
			"repeated absence of governance must not become its own justification")
	}
	if !strings.Contains(err.Error(), "supporting evidence is required") {
		t.Errorf("error = %v, want the explicit-evidence refusal", err)
	}
}

// TestGovernedChecksDoNotFeedCoverageGaps — a check a promoted principle
// already reached is not a gap. Counting it would keep proposing rules for
// ground the system already governs.
func TestGovernedChecksDoNotFeedCoverageGaps(t *testing.T) {
	st, h := newGovHandler()
	ctx := context.Background()

	ac := wipeCheck(t, h, "globule-ryzen")
	governed := api.ActionCheck{
		ID: "governed-1", Project: testProject, Domain: api.DomainRef(testDomain),
		ActionType: "scylla.data.wipe", Theme: ac.GetTheme(), Governed: true,
	}
	if err := st.RecordActionCheck(ctx, &governed); err != nil {
		t.Fatalf("record governed check: %v", err)
	}

	gaps, err := st.ListUngovernedActionChecksByTheme(ctx, testProject, testDomain, ac.GetTheme())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, g := range gaps {
		if g.Governed {
			t.Errorf("governed check %s counted as a coverage gap", g.ID)
		}
	}
	if len(gaps) != 1 {
		t.Errorf("got %d gaps, want 1 (the governed check must not inflate support)", len(gaps))
	}
}
