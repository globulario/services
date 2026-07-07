package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/core"
	bmdomain "github.com/globulario/services/golang/ai_memory/behavioral/domain"
	"github.com/globulario/services/golang/ai_memory/behavioral/rdf"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
	behavioral_rdf "github.com/globulario/services/golang/ai_memory/behavioral_rdf"
	cluster_operator "github.com/globulario/services/golang/ai_memory/domains/cluster_operator"
)

// Integration test for the ScyllaStore adapter against a REAL ScyllaDB.
//
// It is skipped unless BEHAVIORAL_SCYLLA_HOSTS is set (e.g.
// BEHAVIORAL_SCYLLA_HOSTS=127.0.0.1 go test ./ai_memory/ai_memory_server -run Integration).
// NOT RUN in the default CI/dev environment here because no Scylla container is
// available; the in-memory store tests (behavioral_handlers_test.go) cover the
// same ingestion logic deterministically. This test verifies the CQL itself —
// schema DDL, INSERT/SELECT round-trips, the evidence_by_target batch, and the
// governs_refs set-add — which the in-memory store cannot.
func TestScyllaStoreIngestionIntegration(t *testing.T) {
	hostsEnv := os.Getenv("BEHAVIORAL_SCYLLA_HOSTS")
	if hostsEnv == "" {
		t.Skip("BEHAVIORAL_SCYLLA_HOSTS not set — skipping ScyllaDB integration test (no container in this environment)")
	}
	hosts := strings.Split(hostsEnv, ",")
	ctx := context.Background()

	srv := &server{ScyllaHosts: hosts, ScyllaPort: 9042}
	if err := srv.applyBehavioralSchema(ctx); err != nil {
		t.Fatalf("applyBehavioralSchema: %v", err)
	}

	session, err := newSchemaSession(hosts, 9042, len(hosts))
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	defer session.Close()
	st := store.NewScyllaStore(session)

	const project, domain = "globular-services", "cluster_operator"
	sig := &api.Signal{ID: "it-sig-1", Project: project, Domain: domain, Kind: api.SignalObservedRuntimeFact, Status: api.StatusRawSignal}
	if err := st.PutSignal(ctx, sig); err != nil {
		t.Fatalf("PutSignal: %v", err)
	}
	if got, err := st.GetSignal(ctx, project, domain, "it-sig-1"); err != nil || got.Kind != api.SignalObservedRuntimeFact {
		t.Fatalf("GetSignal: got %+v err %v", got, err)
	}

	claim := &api.Claim{ID: "it-claim-1", Project: project, Domain: domain, SignalID: "it-sig-1", Status: api.StatusExtractedClaim}
	if err := st.PutClaim(ctx, claim); err != nil {
		t.Fatalf("PutClaim: %v", err)
	}
	ev := &api.Evidence{ID: "it-ev-1", Project: project, Domain: domain, TargetKind: "claim", TargetID: "it-claim-1", Lane: api.LaneRuntimeRequired, Result: "pass"}
	if err := st.PutEvidence(ctx, ev); err != nil {
		t.Fatalf("PutEvidence: %v", err)
	}
	if list, err := st.ListEvidenceForTarget(ctx, project, domain, "it-claim-1"); err != nil || len(list) != 1 {
		t.Fatalf("ListEvidenceForTarget: got %d err %v", len(list), err)
	}
	if err := st.UpdateClaimStatus(ctx, project, domain, "it-claim-1", api.StatusEvidenceLinked, 1); err != nil {
		t.Fatalf("UpdateClaimStatus: %v", err)
	}
	if err := st.AddAuthorityGoverns(ctx, project, domain, "authority.cluster.etcd.member_health", api.CanonicalURI(api.KindClaim, "it-claim-1"), 1); err != nil {
		t.Fatalf("AddAuthorityGoverns: %v", err)
	}
	if a, err := st.GetAuthority(ctx, project, domain, "authority.cluster.etcd.member_health"); err != nil || len(a.GovernsRefs) != 1 {
		t.Fatalf("GetAuthority: got %+v err %v", a, err)
	}
	contra := &api.Contradiction{ID: "it-con-1", Project: project, Domain: domain, Kind: "claim_vs_claim", LeftRef: "it-claim-1", RightRef: "it-claim-1", Resolution: "open"}
	if err := st.PutContradiction(ctx, contra); err != nil {
		t.Fatalf("PutContradiction: %v", err)
	}
	if got, err := st.GetContradiction(ctx, project, domain, "it-con-1"); err != nil || got.Kind != "claim_vs_claim" {
		t.Fatalf("GetContradiction: got %+v err %v", got, err)
	}
	// contradictions_by_target index maintained.
	if list, err := st.ListContradictionsForTarget(ctx, project, domain, "it-claim-1"); err != nil || len(list) == 0 {
		t.Fatalf("ListContradictionsForTarget: got %d err %v", len(list), err)
	}

	// ── PR-3 governance round-trip (principles / promotion / revocation) ───────
	princ := &api.Principle{
		ID: "it-princ-1", Project: project, Domain: domain, Title: "etcd nospace",
		AppliesWhen:   []api.ConditionRef{"cond.nospace"},
		Authorities:   []api.AuthorityRef{"authority.cluster.etcd.member_health"},
		RiskLevel:     "low", RevocationRule: "narrow if semantics change",
		PromotionReason: "repeated incidents", ProposedBy: "agent", ContradictionChecked: true,
		Status: api.StatusProposedPrinciple, Version: 1,
		SourceRefs: []string{"seed:incidents/inc-1"}, GeneratedFrom: []string{"opsknowledge:runbook.etcd_nospace"},
	}
	if err := st.CreatePrinciple(ctx, princ); err != nil {
		t.Fatalf("CreatePrinciple: %v", err)
	}
	got, err := st.GetPrinciple(ctx, project, domain, "it-princ-1")
	if err != nil || got.Status != api.StatusProposedPrinciple || len(got.AppliesWhen) != 1 {
		t.Fatalf("GetPrinciple round-trip: got %+v err %v", got, err)
	}
	// PR-5A lineage seam round-trips (source_refs / generated_from columns).
	if len(got.SourceRefs) != 1 || len(got.GeneratedFrom) != 1 {
		t.Fatalf("lineage fields did not round-trip: source_refs=%v generated_from=%v", got.SourceRefs, got.GeneratedFrom)
	}
	// blocked decision persisted.
	blocked := &api.PromotionDecisionRecord{ID: "it-dec-blocked", Project: project, Domain: domain, PrincipleID: "it-princ-1", Decision: api.PromotionBlocked, Reason: "no evidence", MissingEvidence: []string{"req.alarm"}, RiskLevel: "low", CreatedAt: 1}
	if err := st.RecordPromotionDecision(ctx, blocked); err != nil {
		t.Fatalf("RecordPromotionDecision blocked: %v", err)
	}
	if d, err := st.GetPromotionDecision(ctx, project, domain, "it-dec-blocked"); err != nil || d.Decision != api.PromotionBlocked {
		t.Fatalf("GetPromotionDecision blocked: got %+v err %v", d, err)
	}
	// allowed decision persisted + principle promoted.
	allowed := &api.PromotionDecisionRecord{ID: "it-dec-allowed", Project: project, Domain: domain, PrincipleID: "it-princ-1", Decision: api.PromotionAllowed, Verdict: "all gate checks passed", RiskLevel: "low", CreatedAt: 2}
	if err := st.RecordPromotionDecision(ctx, allowed); err != nil {
		t.Fatalf("RecordPromotionDecision allowed: %v", err)
	}
	if err := st.UpdatePrincipleStatus(ctx, project, domain, "it-princ-1", api.StatusPromotedPrinciple, 2); err != nil {
		t.Fatalf("UpdatePrincipleStatus: %v", err)
	}
	if got, _ := st.GetPrinciple(ctx, project, domain, "it-princ-1"); got.Status != api.StatusPromotedPrinciple {
		t.Fatalf("principle not promoted: %q", got.Status)
	}
	// revocation persisted without deleting + principle status updated.
	rule := &api.RevocationRule{ID: "it-rev-1", Project: project, Domain: domain, PrincipleID: "it-princ-1", Action: "REVOKED", RevocationReason: "superseded by runbook", Actor: "op", CreatedAt: 3}
	if err := st.RecordRevocationRule(ctx, rule); err != nil {
		t.Fatalf("RecordRevocationRule: %v", err)
	}
	if r, err := st.GetRevocationRule(ctx, project, domain, "it-rev-1"); err != nil || r.Action != "REVOKED" {
		t.Fatalf("GetRevocationRule: got %+v err %v", r, err)
	}
	if err := st.UpdatePrincipleStatus(ctx, project, domain, "it-princ-1", api.StatusRevoked, 3); err != nil {
		t.Fatalf("UpdatePrincipleStatus revoke: %v", err)
	}
	if got, err := st.GetPrinciple(ctx, project, domain, "it-princ-1"); err != nil || got.Status != api.StatusRevoked {
		t.Fatalf("principle not revoked-in-place: got %+v err %v", got, err)
	}

	// ── PR-4 runtime store round-trip (condition index / action_checks / outcomes) ──
	idx := &api.Principle{ID: "it-princ-idx", Project: project, Domain: domain, AppliesWhen: []api.ConditionRef{"cond.x"}, RiskLevel: "low", Status: api.StatusPromotedPrinciple}
	if err := st.CreatePrinciple(ctx, idx); err != nil {
		t.Fatalf("CreatePrinciple idx: %v", err)
	}
	if err := st.IndexPromotedPrinciple(ctx, idx); err != nil {
		t.Fatalf("IndexPromotedPrinciple: %v", err)
	}
	if ids, err := st.ListPrincipleIDsByCondition(ctx, project, domain, "cond.x"); err != nil || len(ids) != 1 {
		t.Fatalf("condition index after promote: got %v err %v", ids, err)
	}
	if err := st.DeindexPromotedPrinciple(ctx, idx); err != nil {
		t.Fatalf("DeindexPromotedPrinciple: %v", err)
	}
	if ids, err := st.ListPrincipleIDsByCondition(ctx, project, domain, "cond.x"); err != nil || len(ids) != 0 {
		t.Fatalf("condition index after revoke: got %v err %v (want empty)", ids, err)
	}

	ac := &api.ActionCheck{ID: "it-ac-1", Project: project, Domain: domain, ActionType: "restart", Status: "blocked", Allowed: false,
		ForbiddenMatched: []api.ForbiddenMoveRef{"forbid.x"}, CheckedAgainstPrinciples: []string{"it-princ-idx"}, CreatedAt: 1}
	if err := st.RecordActionCheck(ctx, ac); err != nil {
		t.Fatalf("RecordActionCheck: %v", err)
	}
	if got, err := st.GetActionCheck(ctx, project, domain, "it-ac-1"); err != nil || got.Status != "blocked" || len(got.ForbiddenMatched) != 1 {
		t.Fatalf("GetActionCheck: got %+v err %v", got, err)
	}

	o := &api.Outcome{ID: "it-out-1", Project: project, Domain: domain, ActionCheckID: "it-ac-1", Status: "failure",
		Severe: true, HumanMarked: true, IncidentID: "INC-1", Theme: "etcd.nospace", SupportsPrinciples: []string{"it-princ-1"}, CreatedAt: 2}
	if err := st.RecordOutcome(ctx, o); err != nil {
		t.Fatalf("RecordOutcome: %v", err)
	}
	if got, err := st.GetOutcome(ctx, project, domain, "it-out-1"); err != nil || got.Status != "failure" || !got.Severe {
		t.Fatalf("GetOutcome: got %+v err %v", got, err)
	}
	if list, err := st.ListOutcomesByTheme(ctx, project, domain, "etcd.nospace"); err != nil || len(list) != 1 {
		t.Fatalf("ListOutcomesByTheme: got %d err %v", len(list), err)
	}

	// ── PR-5 cluster_operator pack (load idempotent → promote via gate → CheckAction) ──
	pack, perr := cluster_operator.New()
	if perr != nil {
		t.Fatalf("cluster_operator.New: %v", perr)
	}
	const seedProj = "globular-services"
	cdom := cluster_operator.DomainName
	if _, err := bmdomain.LoadCatalogs(ctx, st, seedProj, pack); err != nil {
		t.Fatalf("LoadCatalogs (1): %v", err)
	}
	// Idempotent: re-load must not error.
	if _, err := bmdomain.LoadCatalogs(ctx, st, seedProj, pack); err != nil {
		t.Fatalf("LoadCatalogs (2, idempotent): %v", err)
	}
	if _, err := st.GetAuthority(ctx, seedProj, cdom, "authority.cluster.etcd.member_health"); err != nil {
		t.Fatalf("seed authority not loaded: %v", err)
	}
	if _, err := st.GetCondition(ctx, seedProj, cdom, "condition.cluster.etcd.nospace_alarm"); err != nil {
		t.Fatalf("seed condition not loaded: %v", err)
	}

	// P4 discovery (the schema-accurate guard). list_authorities / list_conditions
	// must enumerate the seeded catalog via the single-partition by-scope index —
	// NOT a (project,domain) prefix scan on the composite ((project,domain,id))
	// partition key, which Scylla rejects without ALLOW FILTERING. The in-memory
	// store cannot catch this class (it filters a map); only a real-Scylla list can.
	// This is the test whose absence let the ALLOW-FILTERING bug ship in v1.2.270.
	auths, err := st.ListAuthorities(ctx, seedProj, cdom, 0)
	if err != nil {
		t.Fatalf("ListAuthorities must not require ALLOW FILTERING: %v", err)
	}
	if !containsAuthorityID(auths, "authority.cluster.etcd.member_health") {
		t.Fatalf("ListAuthorities missing seeded authority (got %d rows)", len(auths))
	}
	conds, err := st.ListConditions(ctx, seedProj, cdom, 0)
	if err != nil {
		t.Fatalf("ListConditions must not require ALLOW FILTERING: %v", err)
	}
	if !containsConditionID(conds, "condition.cluster.etcd.nospace_alarm") {
		t.Fatalf("ListConditions missing seeded condition (got %d rows)", len(conds))
	}

	const pid = "principle.cluster.preserve_quorum_before_restart_under_etcd_pressure"
	seedP, err := st.GetPrinciple(ctx, seedProj, cdom, pid)
	if err != nil || seedP.Status != api.StatusProposedPrinciple {
		t.Fatalf("seed principle not PROPOSED: %+v err %v", seedP, err)
	}

	svc := core.New(st, bmdomain.NewRegistry())
	// seed promotion does NOT bypass the gate: fresh promote is BLOCKED.
	blockedResp, err := svc.PromotePrinciple(ctx, &api.PromotePrincipleRequest{PrincipleID: pid, Project: seedProj, Domain: api.DomainRef(cdom), ApprovedBy: "op"})
	if err != nil || blockedResp.Decision != api.PromotionBlocked {
		t.Fatalf("seed promote should be BLOCKED: decision=%v err %v", blockedResp.Decision, err)
	}
	// satisfy the gate, then promote (irreversible risk → needs approval).
	seedP.ContradictionChecked = true
	if err := st.CreatePrinciple(ctx, seedP); err != nil {
		t.Fatalf("mark contradiction checked: %v", err)
	}
	if _, err := svc.RecordEvidence(ctx, &api.RecordEvidenceRequest{Evidence: api.Evidence{Project: seedProj, Domain: api.DomainRef(cdom), TargetKind: "principle", TargetID: pid, Result: "pass"}}); err != nil {
		t.Fatalf("RecordEvidence: %v", err)
	}
	allowedResp, err := svc.PromotePrinciple(ctx, &api.PromotePrincipleRequest{PrincipleID: pid, Project: seedProj, Domain: api.DomainRef(cdom), ApprovedBy: "op"})
	if err != nil || allowedResp.Decision != api.PromotionAllowed {
		t.Fatalf("gated promote should be ALLOWED: decision=%v reason=%q err %v", allowedResp.Decision, allowedResp.Record.Reason, err)
	}
	// promoted → principles_by_condition entry under the etcd condition.
	if ids, err := st.ListPrincipleIDsByCondition(ctx, seedProj, cdom, "condition.cluster.etcd.nospace_alarm"); err != nil || len(ids) != 1 || ids[0] != pid {
		t.Fatalf("condition index for promoted cluster principle: got %v err %v", ids, err)
	}
	// CheckAction uses the promoted cluster condition → blocked for forbidden move.
	chk, err := svc.CheckAction(ctx, &api.CheckActionRequest{
		Project: seedProj, Domain: api.DomainRef(cdom), ActionType: "forbidden.cluster.restart_before_quorum_check",
		CurrentConditions: []api.ConditionRef{"condition.cluster.etcd.nospace_alarm"},
	})
	if err != nil || chk.Result.Status != "blocked" {
		t.Fatalf("CheckAction on promoted cluster principle: status=%q err %v", chk.Result.Status, err)
	}

	// ── PR-5A compiler-generated catalogs (loaded via the same pack) ───────────
	if _, err := st.GetAuthority(ctx, seedProj, cdom, "authority.cluster.cluster_controller.runtime_state"); err != nil {
		t.Fatalf("generated authority not loaded: %v", err)
	}
	if _, err := st.GetCondition(ctx, seedProj, cdom, "condition.cluster.recovery.in_progress"); err != nil {
		t.Fatalf("generated condition not loaded: %v", err)
	}
	const genPID = "principle.cluster.observe_plan_execute_verify_before_recovery_claim"
	genP, err := st.GetPrinciple(ctx, seedProj, cdom, genPID)
	if err != nil || genP.Status != api.StatusProposedPrinciple {
		t.Fatalf("generated principle not stored PROPOSED: %+v err %v", genP, err)
	}
	if len(genP.SourceRefs) == 0 || len(genP.GeneratedFrom) == 0 {
		t.Fatalf("generated principle lineage not persisted: %+v", genP)
	}
	// generated principle promotion is BLOCKED until the gate is satisfied.
	genResp, err := svc.PromotePrinciple(ctx, &api.PromotePrincipleRequest{PrincipleID: genPID, Project: seedProj, Domain: api.DomainRef(cdom), ApprovedBy: "op"})
	if err != nil || genResp.Decision != api.PromotionBlocked {
		t.Fatalf("generated principle promote should be BLOCKED: decision=%v err %v", genResp.Decision, err)
	}

	// ── PR-7 RDF projection over the live Scylla rows (read-only, full-scan) ───
	reader := behavioral_rdf.NewScyllaReader(session)
	bundle, err := reader.Read(ctx, rdf.ReadOptions{Project: seedProj, Domain: cdom})
	if err != nil {
		t.Fatalf("ScyllaReader.Read: %v", err)
	}
	doc := string(rdf.Project(bundle))
	// the promoted etcd principle projects with its stable URI + first-class relations
	if !strings.Contains(doc, "principle/"+pid) {
		t.Errorf("RDF projection missing principle URI for %q", pid)
	}
	if !strings.Contains(doc, "behavioral#appliesWhen>") || !strings.Contains(doc, "behavioral#governedBy>") {
		t.Error("RDF projection missing first-class principle relations")
	}
	// generated principle lineage + backfill-free etcd authority present
	if !strings.Contains(doc, "behavioral#generatedFrom>") {
		t.Error("RDF projection missing generatedFrom lineage from compiled principles")
	}
	if !strings.Contains(doc, "authority/authority.cluster.etcd.member_health") {
		t.Error("RDF projection missing seed authority subject")
	}
	// deterministic: a second projection of the same bundle is byte-identical.
	if rdf.Project(bundle) == nil || string(rdf.Project(bundle)) != doc {
		t.Error("RDF projection is not deterministic")
	}
}

func containsAuthorityID(list []api.Authority, id string) bool {
	for _, a := range list {
		if a.ID == id {
			return true
		}
	}
	return false
}

func containsConditionID(list []api.Condition, id string) bool {
	for _, c := range list {
		if c.ID == id {
			return true
		}
	}
	return false
}
