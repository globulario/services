package main

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/domain"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
	bpb "github.com/globulario/services/golang/ai_memory/behavioral_memorypb"
	cluster_operator "github.com/globulario/services/golang/ai_memory/domains/cluster_operator"
)

const (
	pEtcdQuorum  = "principle.cluster.preserve_quorum_before_restart_under_etcd_pressure"
	condEtcd     = "condition.cluster.etcd.nospace_alarm"
	fmRestart    = "forbidden.cluster.restart_before_quorum_check"
	evEtcdAlarm  = "evidence.cluster.etcd.alarm_status"
	clusterDomain = cluster_operator.DomainName // "cluster_operator" == testDomain
)

// loadClusterPack loads the cluster_operator pack into a fresh store+handler and
// returns them. Seeds catalogs + PROPOSED principles under testProject.
func loadClusterPack(t *testing.T) (*store.MemoryStore, *behavioralHandler) {
	t.Helper()
	st := store.NewMemoryStore()
	h := newBehavioralHandler(st)
	pack, err := cluster_operator.New()
	if err != nil {
		t.Fatalf("pack New: %v", err)
	}
	if _, err := domain.LoadCatalogs(context.Background(), st, testProject, pack); err != nil {
		t.Fatalf("LoadCatalogs: %v", err)
	}
	return st, h
}

// promoteClusterPrinciple satisfies the gate for a loaded seed principle and
// promotes it (with human approval for high/irreversible risk), asserting ALLOWED.
func promoteClusterPrinciple(t *testing.T, st *store.MemoryStore, h *behavioralHandler, principleID string) {
	t.Helper()
	ctx := context.Background()
	// Mark the contradiction check (a deliberate operator act) and add evidence.
	p, err := st.GetPrinciple(ctx, testProject, clusterDomain, principleID)
	if err != nil {
		t.Fatalf("GetPrinciple %q: %v", principleID, err)
	}
	p.ContradictionChecked = true
	if err := st.CreatePrinciple(ctx, p); err != nil {
		t.Fatalf("update principle: %v", err)
	}
	if _, err := h.RecordEvidence(ctx, &bpb.RecordEvidenceRequest{Evidence: &bpb.Evidence{
		Project: testProject, Domain: clusterDomain, TargetKind: "principle", TargetId: principleID,
		EvidenceKind: "probe", Result: "pass",
	}}); err != nil {
		t.Fatalf("RecordEvidence: %v", err)
	}
	resp, err := h.PromotePrinciple(ctx, &bpb.PromotePrincipleRequest{
		PrincipleId: principleID, Project: testProject, Domain: clusterDomain, ApprovedBy: "operator-dave",
	})
	if err != nil {
		t.Fatalf("PromotePrinciple: %v", err)
	}
	if resp.GetDecision() != bpb.PromotionDecision_PROMOTION_ALLOWED {
		t.Fatalf("promote decision=%v (%s), want ALLOWED", resp.GetDecision(), resp.GetRecord().GetReason())
	}
}

// Load writes authority/condition catalog rows + PROPOSED principles.
func TestClusterPackLoadsCatalogs(t *testing.T) {
	st, _ := loadClusterPack(t)
	ctx := context.Background()
	if _, err := st.GetAuthority(ctx, testProject, clusterDomain, "authority.cluster.etcd.member_health"); err != nil {
		t.Errorf("authority not loaded: %v", err)
	}
	if _, err := st.GetCondition(ctx, testProject, clusterDomain, condEtcd); err != nil {
		t.Errorf("condition not loaded: %v", err)
	}
	p, err := st.GetPrinciple(ctx, testProject, clusterDomain, pEtcdQuorum)
	if err != nil {
		t.Fatalf("seed principle not loaded: %v", err)
	}
	if p.Status != api.StatusProposedPrinciple {
		t.Errorf("seed principle status = %q, want PROPOSED_PRINCIPLE", p.Status)
	}
	if len(p.SourceRefs) == 0 || len(p.GeneratedFrom) == 0 || p.ProposedBy == "" {
		t.Errorf("seed principle lineage/provenance dropped: %+v", p)
	}
}

// Seed loading does NOT bypass the promotion gate.
func TestClusterSeedPromotionDoesNotBypassGate(t *testing.T) {
	st, h := loadClusterPack(t)
	// Freshly seeded principle has no evidence + no contradiction check → BLOCKED.
	resp, err := h.PromotePrinciple(context.Background(), &bpb.PromotePrincipleRequest{
		PrincipleId: pEtcdQuorum, Project: testProject, Domain: clusterDomain, ApprovedBy: "op",
	})
	if err != nil {
		t.Fatalf("PromotePrinciple: %v", err)
	}
	if resp.GetDecision() != bpb.PromotionDecision_PROMOTION_BLOCKED {
		t.Fatalf("seed promote decision=%v, want BLOCKED (gate not bypassed)", resp.GetDecision())
	}
	if resp.GetRecord().GetId() == "" {
		t.Error("blocked seed promotion must still record a decision")
	}
	got, _ := st.GetPrinciple(context.Background(), testProject, clusterDomain, pEtcdQuorum)
	if got.Status == api.StatusPromotedPrinciple {
		t.Error("seed principle promoted despite blocked gate")
	}
}

// A promoted cluster principle drives CheckAction → blocked for a matching
// forbidden move.
func TestClusterCheckActionBlockedForbidden(t *testing.T) {
	st, h := loadClusterPack(t)
	promoteClusterPrinciple(t, st, h, pEtcdQuorum)
	// promoted → indexed by condEtcd
	ac := checkAction(t, h, &bpb.CheckActionRequest{
		Domain: clusterDomain, ActionType: fmRestart, CurrentConditions: []string{condEtcd},
	})
	if ac.GetStatus() != "blocked" {
		t.Errorf("status=%q, want blocked (forbidden move %q)", ac.GetStatus(), fmRestart)
	}
}

// CheckAction → needs_evidence for an unsatisfied cluster required-evidence ref.
func TestClusterCheckActionNeedsEvidence(t *testing.T) {
	st, h := loadClusterPack(t)
	promoteClusterPrinciple(t, st, h, pEtcdQuorum)
	// non-forbidden action, no provided evidence → the 3 etcd evidence refs missing.
	ac := checkAction(t, h, &bpb.CheckActionRequest{
		Domain: clusterDomain, ActionType: "inspect", CurrentConditions: []string{condEtcd},
	})
	if ac.GetStatus() != "needs_evidence" {
		t.Fatalf("status=%q, want needs_evidence", ac.GetStatus())
	}
	if !containsStr(ac.GetMissingEvidence(), evEtcdAlarm) {
		t.Errorf("missing_evidence = %v, want to include %q", ac.GetMissingEvidence(), evEtcdAlarm)
	}
}

// ResolveGovernedContext surfaces cluster authorities/evidence/forbidden moves +
// generative behavior.
func TestClusterResolveReturnsClusterBundle(t *testing.T) {
	st, h := loadClusterPack(t)
	promoteClusterPrinciple(t, st, h, pEtcdQuorum)
	resp, err := h.ResolveGovernedContext(context.Background(), &bpb.ResolveGovernedContextRequest{
		Project: testProject, Domain: clusterDomain, Conditions: []string{condEtcd},
	})
	if err != nil {
		t.Fatalf("ResolveGovernedContext: %v", err)
	}
	c := resp.GetContext()
	if len(c.GetApplicablePrinciples()) != 1 {
		t.Fatalf("applicable principles = %d, want 1", len(c.GetApplicablePrinciples()))
	}
	p := c.GetApplicablePrinciples()[0]
	if !containsStr(p.GetForbiddenMoves(), fmRestart) {
		t.Errorf("principle forbidden moves = %v, want %q", p.GetForbiddenMoves(), fmRestart)
	}
	if !containsStr(p.GetAuthorities(), "authority.cluster.etcd.member_health") {
		t.Errorf("principle authorities = %v, want etcd member_health", p.GetAuthorities())
	}
	if len(c.GetRequiredEvidence()) == 0 {
		t.Error("bundle has no required evidence refs")
	}
	if len(c.GetForbiddenMoves()) == 0 {
		t.Error("bundle has no forbidden move refs")
	}
	if c.GetRecommendedBehavior() == "" {
		t.Error("bundle has no generative recommended behavior")
	}
}

func containsStr(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}
