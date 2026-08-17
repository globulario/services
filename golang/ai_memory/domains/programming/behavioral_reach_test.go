package programming_test

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/core"
	"github.com/globulario/services/golang/ai_memory/behavioral/domain"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
	programming "github.com/globulario/services/golang/ai_memory/domains/programming"
)

const programmingTestProject = "globular-services"

func newProgrammingBehaviorService(t *testing.T) (*core.Service, *store.MemoryStore) {
	t.Helper()
	pack, err := programming.New()
	if err != nil {
		t.Fatalf("programming.New: %v", err)
	}
	st := store.NewMemoryStore()
	if _, err := domain.LoadCatalogs(context.Background(), st, programmingTestProject, pack); err != nil {
		t.Fatalf("LoadCatalogs: %v", err)
	}
	reg := domain.NewRegistry()
	reg.Register(pack)
	return core.New(st, reg), st
}

func promoteProgrammingPrinciple(t *testing.T, svc *core.Service, id string) {
	t.Helper()
	ctx := context.Background()
	if _, err := svc.RecordEvidence(ctx, &api.RecordEvidenceRequest{Evidence: api.Evidence{
		Project:    programmingTestProject,
		Domain:     api.DomainRef(programming.DomainName),
		TargetKind: "principle",
		TargetID:   id,
		Kind:       "governance_design_evidence",
		Lane:       api.LaneStaticOnly,
		Result:     "reviewed",
		SourceKind: "test_review",
		SourceRef:  "behavioral_reach_test",
		Provenance: api.Provenance{AgentID: "test.human_reviewer"},
	}}); err != nil {
		t.Fatalf("RecordEvidence(%s): %v", id, err)
	}
	if _, err := svc.RunContradictionCheck(ctx, &api.RunContradictionCheckRequest{
		Project: programmingTestProject, Domain: api.DomainRef(programming.DomainName), PrincipleID: id, Actor: "test.human_reviewer",
	}); err != nil {
		t.Fatalf("RunContradictionCheck(%s): %v", id, err)
	}
	resp, err := svc.PromotePrinciple(ctx, &api.PromotePrincipleRequest{
		Project: programmingTestProject, Domain: api.DomainRef(programming.DomainName), PrincipleID: id,
		ApprovedBy: "test.human_reviewer", ApprovalReason: "runtime reach regression test", Actor: "test.human_reviewer",
	})
	if err != nil {
		t.Fatalf("PromotePrinciple(%s): %v", id, err)
	}
	if resp.Decision != api.PromotionAllowed || resp.Status != api.StatusPromotedPrinciple {
		t.Fatalf("promotion %s = %+v", id, resp)
	}
}

func checkProgrammingAction(t *testing.T, svc *core.Service, action string) api.ActionCheck {
	t.Helper()
	resp, err := svc.CheckAction(context.Background(), &api.CheckActionRequest{
		Project: programmingTestProject,
		Domain:  api.DomainRef(programming.DomainName),
		ActionType: action,
	})
	if err != nil {
		t.Fatalf("CheckAction(%s): %v", action, err)
	}
	return resp.Result
}

func TestSuppressFailingTestAliasHasUniversalForbiddenReach(t *testing.T) {
	svc, _ := newProgrammingBehaviorService(t)
	promoteProgrammingPrinciple(t, svc, "principle.prog.no_deploy_without_passing_tests")

	ac := checkProgrammingAction(t, svc, "skip_failing_test")
	if ac.Allowed || !ac.Governed || ac.Status != "blocked" {
		t.Fatalf("skip_failing_test verdict allowed=%v governed=%v status=%q, want false/true/blocked", ac.Allowed, ac.Governed, ac.Status)
	}
	if len(ac.ForbiddenMatched) != 1 || ac.ForbiddenMatched[0] != api.ForbiddenMoveRef("forbidden.prog.suppress_failing_test_to_go_green") {
		t.Fatalf("forbidden matches=%v", ac.ForbiddenMatched)
	}
}

func TestCommitSecretAliasHasUniversalForbiddenReach(t *testing.T) {
	svc, _ := newProgrammingBehaviorService(t)
	promoteProgrammingPrinciple(t, svc, "principle.prog.never_commit_secret")

	ac := checkProgrammingAction(t, svc, "commit_secret")
	if ac.Allowed || !ac.Governed || ac.Status != "blocked" {
		t.Fatalf("commit_secret verdict allowed=%v governed=%v status=%q, want false/true/blocked", ac.Allowed, ac.Governed, ac.Status)
	}
	if len(ac.ForbiddenMatched) != 1 || ac.ForbiddenMatched[0] != api.ForbiddenMoveRef("forbidden.prog.commit_secret_in_source") {
		t.Fatalf("forbidden matches=%v", ac.ForbiddenMatched)
	}
}

func TestAlwaysSentinelDoesNotEngageUnrelatedHighRiskRule(t *testing.T) {
	svc, _ := newProgrammingBehaviorService(t)
	promoteProgrammingPrinciple(t, svc, "principle.prog.no_deploy_without_passing_tests")

	ac := checkProgrammingAction(t, svc, "edit_readme")
	if !ac.Allowed || ac.Governed {
		t.Fatalf("unrelated action allowed=%v governed=%v, want true/false; condition.always must provide reach without engagement", ac.Allowed, ac.Governed)
	}
	if ac.Status != "allowed" {
		t.Fatalf("unrelated action status=%q want allowed", ac.Status)
	}
}
