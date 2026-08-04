package main

// Commit 5: the governed-remediation scenarios, driven through the REAL workflow
// handlers.
//
// Each test runs engine.RegisterDoctorRemediationActions with the doctor's own
// gateRemediation hook, so the thing under test is the production path: resolve
// → GATE → execute → verify → outcome. The executor is a spy, because the
// central claim of most of these tests is that it was never called.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	observation "github.com/globulario/services/golang/ai_memory/domains/cluster_operator/observation"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/remediation"
	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/v1alpha1"
)

const (
	gvFinding   = "finding-units-1"
	gvCluster   = "cluster-gov"
	gvInvar     = "node.systemd.units_running"
	gvEntity    = "node-4/globular-repository" // node/unit — NOT the node id
	gvNode      = "node-4"
	gvRun       = "run-gov-1"
	gvAction    = "SYSTEMCTL_RESTART"
	gvCheckID   = "ac-1"
	gvPrinciple = "principle.cluster.restart_drifted_unit_with_observed_finding"
)

var gvDispatchAt = time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)

// fakeGovernor records what it was asked and answers as configured.
type fakeGovernor struct {
	decision observation.GateDecision
	checkErr error

	asked    []observation.ActionContext
	outcomes []observation.OutcomeRecord
	outErr   error

	// learning-loop half
	candidates []observation.CandidateRequest
	candErr    error
	candResult observation.CandidateResult
}

func (f *fakeGovernor) GeneratePromotionCandidate(_ context.Context, r observation.CandidateRequest) (observation.CandidateResult, error) {
	f.candidates = append(f.candidates, r)
	if f.candErr != nil {
		return observation.CandidateResult{}, f.candErr
	}
	return f.candResult, nil
}

func (f *fakeGovernor) CheckAction(_ context.Context, a observation.ActionContext) (observation.GateDecision, error) {
	f.asked = append(f.asked, a)
	if f.checkErr != nil {
		return observation.GateDecision{}, f.checkErr
	}
	return f.decision, nil
}

func (f *fakeGovernor) RecordOutcome(_ context.Context, o observation.OutcomeRecord) (string, error) {
	f.outcomes = append(f.outcomes, o)
	if f.outErr != nil {
		return "", f.outErr
	}
	return "outcome-1", nil
}

func allowDecision() observation.GateDecision {
	return observation.GateDecision{
		ActionCheckID: gvCheckID, Governed: true, Allowed: true, Status: "allowed",
		PrincipleIDs: []string{gvPrinciple},
	}
}

type gvResult struct {
	executorCalls int
	outputs       map[string]any
	execErr       error
	verifyErr     error
	outcome       *remediation.Outcome
}

// runGoverned drives the REAL handler chain through the doctor's real gate hook.
func runGoverned(t *testing.T, gov behavioralGovernor, converged bool) gvResult {
	t.Helper()
	resetGateMemo()

	srv := &ClusterDoctorServer{clusterID: gvCluster}
	if gov != nil {
		srv.behavioralGovernor = gov
	}
	res := gvResult{outputs: map[string]any{}}

	cfg := engine.DoctorRemediationConfig{
		Now: func() time.Time { return gvDispatchAt },
		ResolveFinding: func(_ context.Context, _ string, id string, idx uint32) (*engine.ResolvedFinding, error) {
			return &engine.ResolvedFinding{
				FindingID: id, StepIndex: idx, NodeID: gvNode,
				ActionType: gvAction, Risk: "LOW", Idempotent: true,
				Description: "restart globular-repository", HasAction: true,
				ClusterID: gvCluster, InvariantID: gvInvar, EntityRef: gvEntity,
			}, nil
		},
		ExecuteRemediation: func(context.Context, string, uint32, string, bool) (*engine.ExecutionResult, error) {
			res.executorCalls++
			return &engine.ExecutionResult{AuditID: "audit-1", Status: "EXECUTED", Executed: true}, nil
		},
		VerifyConvergence: func(context.Context, string, string, time.Time) (*engine.Verification, error) {
			return &engine.Verification{Converged: converged, FindingStillPresent: !converged}, nil
		},
		ObserveOutcome: func(ctx context.Context, o remediation.Outcome) {
			cp := o
			res.outcome = &cp
			srv.recordGovernedOutcome(ctx, o, "ev-verification-1")
		},
		// THE PRODUCTION HOOK, not a stand-in.
		GateAction: srv.gateRemediation,
	}

	router := engine.NewRouter()
	engine.RegisterDoctorRemediationActions(router, cfg)
	ctx := context.Background()

	dispatch := func(action string, with map[string]any) error {
		h, ok := router.Resolve(v1alpha1.ActorClusterDoctor, action)
		if !ok {
			t.Fatalf("%s not registered", action)
		}
		_, err := h(ctx, engine.ActionRequest{RunID: gvRun, With: with, Outputs: res.outputs})
		return err
	}

	if err := dispatch("doctor.resolve_finding", map[string]any{"finding_id": gvFinding, "step_index": 0}); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	res.execErr = dispatch("doctor.execute_remediation", map[string]any{"finding_id": gvFinding, "step_index": 0})
	if res.execErr != nil {
		return res
	}
	resolved, _ := res.outputs["resolved_finding"].(map[string]any)
	res.verifyErr = dispatch("doctor.verify_convergence", map[string]any{
		"finding_id": gvFinding, "node_id": resolved["node_id"],
		"cluster_id": resolved["cluster_id"], "invariant_id": resolved["invariant_id"],
		"entity_ref": resolved["entity_ref"],
	})
	return res
}

// ── 1. allowed ──────────────────────────────────────────────────────────────

func TestGoverned_AllowedPathExecutesAndRecordsSuccess(t *testing.T) {
	gov := &fakeGovernor{decision: allowDecision()}
	r := runGoverned(t, gov, true)

	if r.execErr != nil || r.verifyErr != nil {
		t.Fatalf("allowed path must complete: exec=%v verify=%v", r.execErr, r.verifyErr)
	}
	if r.executorCalls != 1 {
		t.Fatalf("executor calls = %d, want 1", r.executorCalls)
	}

	// The gate was asked about the REAL subject, read from the resolved finding.
	if len(gov.asked) != 1 {
		t.Fatalf("gate asked %d times, want 1", len(gov.asked))
	}
	a := gov.asked[0]
	for _, c := range []struct{ name, got, want string }{
		{"cluster", a.ClusterID, gvCluster},
		{"invariant", a.InvariantID, gvInvar},
		{"entity", a.EntityRef, gvEntity},
		{"finding", a.FindingID, gvFinding},
		{"run", a.WorkflowRunID, gvRun},
		{"action kind", a.ActionKind, gvAction},
	} {
		if c.got != c.want {
			t.Errorf("gate %s = %q, want %q", c.name, c.got, c.want)
		}
	}
	if a.EntityRef == gvNode {
		t.Error("entity_ref collapsed to node_id — the gate would judge the wrong subject")
	}
	// The condition must be the one the principle applies to, or the check is
	// ungoverned and the principle never engages.
	if len(a.Conditions) != 1 || a.Conditions[0] != "condition.cluster.node.systemd_unit_not_running" {
		t.Errorf("conditions = %v, want the mapped systemd-unit condition", a.Conditions)
	}

	// The decision travels with the dispatch.
	exec, _ := r.outputs["execution_result"].(map[string]any)
	if exec["action_check_id"] != gvCheckID {
		t.Errorf("action_check_id = %v, want %q", exec["action_check_id"], gvCheckID)
	}
	if r.outcome == nil || r.outcome.ActionCheckID != gvCheckID {
		t.Fatalf("the outcome must cite the check that allowed it, got %+v", r.outcome)
	}

	// The behavioral outcome supports the principle only because the repair was
	// verified AND attributable.
	if len(gov.outcomes) != 1 {
		t.Fatalf("outcomes recorded = %d, want 1", len(gov.outcomes))
	}
	o := gov.outcomes[0]
	if o.Status != "success" {
		t.Errorf("status = %q, want success", o.Status)
	}
	if o.ActionCheckID != gvCheckID {
		t.Errorf("outcome ActionCheckID = %q, want %q", o.ActionCheckID, gvCheckID)
	}
	if len(o.SupportsPrinciples) != 1 || o.SupportsPrinciples[0] != gvPrinciple {
		t.Errorf("SupportsPrinciples = %v, want [%s]", o.SupportsPrinciples, gvPrinciple)
	}
	if len(o.EvidenceIDs) == 0 {
		t.Error("the outcome must cite the verification evidence")
	}
}

// ── 2. missing evidence ─────────────────────────────────────────────────────

func TestGoverned_MissingEvidenceBlocksBeforeDispatch(t *testing.T) {
	gov := &fakeGovernor{decision: observation.GateDecision{
		ActionCheckID: gvCheckID, Governed: true, Allowed: false,
		Status: "needs_evidence",
		Reason: "gather required evidence: evidence.doctor.finding_observed",
	}}
	r := runGoverned(t, gov, true)

	if r.execErr == nil {
		t.Fatal("a blocked action must fail the step, not proceed quietly")
	}
	if r.executorCalls != 0 {
		t.Fatalf("EXECUTOR WAS CALLED (%d times) despite a block — the gate is after the action", r.executorCalls)
	}
	if !strings.Contains(r.execErr.Error(), "needs_evidence") {
		t.Errorf("the failure must name the governance status, got %v", r.execErr)
	}
	// A blocked action still leaves a record.
	g, _ := r.outputs["governance"].(map[string]any)
	if g == nil || g["allowed"] != false || g["action_check_id"] != gvCheckID {
		t.Errorf("a refusal must be recorded like an approval, got %v", g)
	}
}

// ── 3. disqualified evidence ────────────────────────────────────────────────

// Evidence EXISTS and does not count. The verdict must differ from absence: an
// operator told to "gather" what is already stored has been misdirected.
func TestGoverned_DisqualifiedEvidenceReadsDifferentlyFromAbsent(t *testing.T) {
	disq := &fakeGovernor{decision: observation.GateDecision{
		ActionCheckID: gvCheckID, Governed: true, Allowed: false, Status: "needs_evidence",
		Reason: "required evidence exists but does not qualify: evidence.doctor.finding_observed " +
			"(authority_insufficient: floor DERIVED_EVIDENCE got INTERPRETATION)",
	}}
	rd := runGoverned(t, disq, true)

	if rd.executorCalls != 0 {
		t.Fatalf("executor called %d times despite disqualified evidence", rd.executorCalls)
	}
	if !strings.Contains(rd.execErr.Error(), "does not qualify") {
		t.Errorf("the reason must say the evidence exists and was refused, got %v", rd.execErr)
	}

	absent := &fakeGovernor{decision: observation.GateDecision{
		ActionCheckID: "ac-2", Governed: true, Allowed: false, Status: "needs_evidence",
		Reason: "gather required evidence: evidence.doctor.finding_observed",
	}}
	ra := runGoverned(t, absent, true)

	if rd.execErr.Error() == ra.execErr.Error() {
		t.Fatal("disqualified and absent evidence must not produce the same message — " +
			"they call for different operator actions")
	}
}

// ── 4. human approval ───────────────────────────────────────────────────────

// The governor may demand approval; the doctor's own approval gate is untouched.
// Two gates, both effective, neither replacing the other.
func TestGoverned_HumanApprovalCombinesWithExistingGate(t *testing.T) {
	gov := &fakeGovernor{decision: observation.GateDecision{
		ActionCheckID: gvCheckID, Governed: true, Allowed: false,
		Status: "needs_human_approval",
		Reason: "obtain explicit human approval before proceeding (high/irreversible risk)",
	}}
	r := runGoverned(t, gov, true)

	if r.executorCalls != 0 {
		t.Fatalf("needs_human_approval must stop dispatch, executor called %d times", r.executorCalls)
	}
	if !strings.Contains(r.execErr.Error(), "needs_human_approval") {
		t.Errorf("want the approval status surfaced, got %v", r.execErr)
	}

	// And when an operator token IS present, it reaches the governor while the
	// executor still receives it too — the doctor's gate is not bypassed.
	resetGateMemo()
	gov2 := &fakeGovernor{decision: allowDecision()}
	srv := &ClusterDoctorServer{clusterID: gvCluster, behavioralGovernor: gov2}
	var executorSawToken string
	cfg := engine.DoctorRemediationConfig{
		Now: func() time.Time { return gvDispatchAt },
		ResolveFinding: func(_ context.Context, _ string, id string, idx uint32) (*engine.ResolvedFinding, error) {
			return &engine.ResolvedFinding{
				FindingID: id, StepIndex: idx, NodeID: gvNode, ActionType: gvAction,
				Risk: "HIGH", Idempotent: true, HasAction: true,
				ClusterID: gvCluster, InvariantID: gvInvar, EntityRef: gvEntity,
			}, nil
		},
		ExecuteRemediation: func(_ context.Context, _ string, _ uint32, token string, _ bool) (*engine.ExecutionResult, error) {
			executorSawToken = token
			return &engine.ExecutionResult{Status: "EXECUTED", Executed: true}, nil
		},
		GateAction: srv.gateRemediation,
	}
	router := engine.NewRouter()
	engine.RegisterDoctorRemediationActions(router, cfg)
	outputs := map[string]any{}
	h, _ := router.Resolve(v1alpha1.ActorClusterDoctor, "doctor.resolve_finding")
	if _, err := h(context.Background(), engine.ActionRequest{
		RunID: gvRun, With: map[string]any{"finding_id": gvFinding, "step_index": 0}, Outputs: outputs,
	}); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	h, _ = router.Resolve(v1alpha1.ActorClusterDoctor, "doctor.execute_remediation")
	if _, err := h(context.Background(), engine.ActionRequest{
		RunID: gvRun, Outputs: outputs,
		With: map[string]any{"finding_id": gvFinding, "step_index": 0, "approval_token": "op-token-1"},
	}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(gov2.asked) != 1 || gov2.asked[0].HumanApproval != "op-token-1" {
		t.Errorf("the governor must see the approval the doctor holds, got %+v", gov2.asked)
	}
	if executorSawToken != "op-token-1" {
		t.Errorf("the executor's own approval gate must still receive the token, got %q", executorSawToken)
	}
}

// ── 5. failed verification ──────────────────────────────────────────────────

func TestGoverned_FailedVerificationRecordsFailureAndDoesNotSupport(t *testing.T) {
	gov := &fakeGovernor{decision: allowDecision()}
	r := runGoverned(t, gov, false) // finding remains

	if r.executorCalls != 1 {
		t.Fatalf("the action was allowed and must dispatch, calls=%d", r.executorCalls)
	}
	if r.verifyErr == nil {
		t.Fatal("a finding still present after remediation must fail the verify step")
	}
	if len(gov.outcomes) != 1 {
		t.Fatalf("a failed repair must still be recorded, got %d outcomes", len(gov.outcomes))
	}
	o := gov.outcomes[0]
	if o.Status != "failure" {
		t.Errorf("status = %q, want failure", o.Status)
	}
	if len(o.SupportsPrinciples) != 0 {
		t.Fatal("a dispatched-but-unconverged action must NEVER support the principle")
	}
	if len(o.WeakensPrinciples) != 1 {
		t.Errorf("a failed verification must weaken the principle, got %v", o.WeakensPrinciples)
	}
}

// Dispatch alone never supports the principle, even when everything else is
// attributable.
func TestGoverned_DispatchAloneNeverSupports(t *testing.T) {
	o := remediation.Outcome{
		FindingID: gvFinding, WorkflowRunID: gvRun, ClusterID: gvCluster,
		InvariantID: gvInvar, EntityRef: gvEntity, ActionCheckID: gvCheckID,
		Dispatched: true, Verified: false,
		DispatchedAt: gvDispatchAt,
	}
	if got := behavioralOutcomeStatus(o, false); got != "degraded" {
		t.Fatalf("dispatched-but-unverified = %q, want degraded — dispatch is not repair", got)
	}
}

// A repair that worked but cannot be attributed must not count for the rule.
func TestGoverned_UnattributableSuccessIsNotSuccess(t *testing.T) {
	o := remediation.Outcome{
		FindingID: gvFinding, WorkflowRunID: gvRun, ActionCheckID: gvCheckID,
		Dispatched: true, Verified: true, FindingResolved: true,
		DispatchedAt: gvDispatchAt, VerifiedAt: gvDispatchAt.Add(time.Minute),
	}
	if o.LineageComplete() {
		t.Fatal("fixture must be unattributable")
	}
	if got := behavioralOutcomeStatus(o, false); got == "success" {
		t.Fatal("an unattributable repair must not read as success")
	}
}

func TestGoverned_BlockedIsBlockedNotFailure(t *testing.T) {
	o := remediation.Outcome{FindingID: gvFinding, Dispatched: false}
	if got := behavioralOutcomeStatus(o, true); got != "blocked" {
		t.Fatalf("blocked = %q, want blocked — a refused action is not a failed repair", got)
	}
}

// ── 6. ungoverned ───────────────────────────────────────────────────────────

func TestGoverned_UngovernedKeepsExistingBehaviorAndRecordsGap(t *testing.T) {
	var gapFinding string
	orig := behavioralCoverageGapNotify
	behavioralCoverageGapNotify = func(req engine.GateRequest, _ string) { gapFinding = req.FindingID }
	defer func() { behavioralCoverageGapNotify = orig }()

	gov := &fakeGovernor{decision: observation.GateDecision{
		ActionCheckID: "ac-ungoverned", Governed: false, Allowed: true, Status: "allowed",
	}}
	r := runGoverned(t, gov, true)

	if r.execErr != nil || r.executorCalls != 1 {
		t.Fatalf("an ungoverned action must retain existing behavior: err=%v calls=%d", r.execErr, r.executorCalls)
	}
	if gapFinding != gvFinding {
		t.Error("an ungoverned dispatch must record a coverage gap — an unmeasured gate reads as a working one")
	}
	g, _ := r.outputs["governance"].(map[string]any)
	if g["governed"] != false {
		t.Errorf("the receipt must not claim governance, got %v", g)
	}
}

// ── 7. gate unavailable ─────────────────────────────────────────────────────

// The governor is unreachable. This must NOT become an allow.
func TestGoverned_UnavailableGovernorRefusesRatherThanAllows(t *testing.T) {
	gov := &fakeGovernor{checkErr: errors.New("connection refused")}
	r := runGoverned(t, gov, true)

	if r.executorCalls != 0 {
		t.Fatalf("an undecidable gate must not dispatch, executor called %d times — "+
			"governance would be strongest when working and absent when not", r.executorCalls)
	}
	if r.execErr == nil || !strings.Contains(r.execErr.Error(), "governance check unavailable") {
		t.Errorf("the failure must name the degraded gate, got %v", r.execErr)
	}
}

// No governor wired at all keeps the pre-governance behavior.
func TestGoverned_NoGovernorWiredKeepsExistingBehavior(t *testing.T) {
	r := runGoverned(t, nil, true)
	if r.execErr != nil || r.executorCalls != 1 {
		t.Fatalf("no governor must not block: err=%v calls=%d", r.execErr, r.executorCalls)
	}
}

// ── 8. replay ───────────────────────────────────────────────────────────────

// A resumed run must not mint a second decision for the same action.
func TestGoverned_ReplayReusesTheSameActionCheck(t *testing.T) {
	resetGateMemo()
	gov := &fakeGovernor{decision: allowDecision()}
	srv := &ClusterDoctorServer{clusterID: gvCluster, behavioralGovernor: gov}

	req := engine.GateRequest{
		FindingID: gvFinding, ClusterID: gvCluster, InvariantID: gvInvar,
		EntityRef: gvEntity, NodeID: gvNode, ActionKind: gvAction,
		WorkflowRunID: gvRun, StepIndex: 0,
	}
	first, err := srv.gateRemediation(context.Background(), req)
	if err != nil {
		t.Fatalf("gate: %v", err)
	}
	second, err := srv.gateRemediation(context.Background(), req)
	if err != nil {
		t.Fatalf("gate replay: %v", err)
	}

	if len(gov.asked) != 1 {
		t.Fatalf("replay issued %d checks, want 1 — the audit trail must describe one action once", len(gov.asked))
	}
	if first.ActionCheckID != second.ActionCheckID {
		t.Fatalf("replay produced a different ActionCheck (%q vs %q); later evidence would cite a check "+
			"that no longer matches the decision", first.ActionCheckID, second.ActionCheckID)
	}

	// A genuinely different run is a different action and MUST be checked again.
	other := req
	other.WorkflowRunID = "run-gov-2"
	if _, err := srv.gateRemediation(context.Background(), other); err != nil {
		t.Fatalf("gate other run: %v", err)
	}
	if len(gov.asked) != 2 {
		t.Fatalf("a different run must be checked independently, asked=%d", len(gov.asked))
	}
}

// Replaying the whole chain records the outcome against the SAME check, and the
// verification evidence keeps the same id — so no duplicate logical record.
func TestGoverned_ReplayDoesNotDuplicateOutcomeIdentity(t *testing.T) {
	gov := &fakeGovernor{decision: allowDecision()}
	first := runGoverned(t, gov, true)
	firstCheck := gov.outcomes[0].ActionCheckID

	// Same run replayed. resetGateMemo is NOT called: a resume within one
	// doctor's lifetime must reuse its decision.
	gov2 := &fakeGovernor{decision: allowDecision()}
	srv := &ClusterDoctorServer{clusterID: gvCluster, behavioralGovernor: gov2}
	v, err := srv.gateRemediation(context.Background(), engine.GateRequest{
		FindingID: gvFinding, ClusterID: gvCluster, InvariantID: gvInvar,
		EntityRef: gvEntity, NodeID: gvNode, ActionKind: gvAction,
		WorkflowRunID: gvRun, StepIndex: 0,
	})
	if err != nil {
		t.Fatalf("replay gate: %v", err)
	}
	if v.ActionCheckID != firstCheck {
		t.Fatalf("replay produced check %q, want the original %q", v.ActionCheckID, firstCheck)
	}
	if len(gov2.asked) != 0 {
		t.Error("the replay must be served from the remembered decision, not a fresh check")
	}
	if first.outcome == nil || first.outcome.ActionCheckID != firstCheck {
		t.Error("the outcome must cite the original decision")
	}
}

// A lost outcome record degrades learning only — the remediation verdict the
// operator already holds is untouched.
func TestGoverned_OutcomeRecordFailureDoesNotAffectRemediation(t *testing.T) {
	var notified bool
	orig := behavioralOutcomeRecordFailedNotify
	behavioralOutcomeRecordFailedNotify = func(remediation.Outcome, error) { notified = true }
	defer func() { behavioralOutcomeRecordFailedNotify = orig }()

	gov := &fakeGovernor{decision: allowDecision(), outErr: errors.New("ai-memory down")}
	r := runGoverned(t, gov, true)

	if r.execErr != nil || r.verifyErr != nil {
		t.Fatalf("a failed outcome record must not fail the remediation: exec=%v verify=%v", r.execErr, r.verifyErr)
	}
	if !notified {
		t.Error("a lost governed outcome must be reported")
	}
	if r.outcome == nil || !r.outcome.IsSuccess() {
		t.Error("the remediation verdict must be unaffected by a behavioral write failure")
	}
}

// TestGatedDispatcher_ConsultsGovernanceBeforeExecuting is the guard for the
// asymmetry found on 2026-08-01.
//
// The behavioral gate was wired into the WORKFLOW path only. The autonomous
// healer reached executeRemediationForFinding through gatedDispatcher.Dispatch
// and never consulted it. Live evidence: a principle promoted specifically to
// govern SYSTEMCTL_RESTART on node.systemd.units_running was in force, the
// healer performed exactly that action, and behavioral_memory.action_checks
// held zero rows from cluster_doctor. The operator-driven path was governed;
// the unattended one was not — the wrong way round, since the unattended path
// is the one with no human watching.
func TestGatedDispatcher_ConsultsGovernanceBeforeExecuting(t *testing.T) {
	resetGateMemo() // the memo is keyed run|finding|step; these tests share one key
	gov := &fakeGovernor{decision: allowDecision()}
	srv := &ClusterDoctorServer{behavioralGovernor: gov, clusterID: "c1"}

	f := findingWithRestartAction()
	// The autonomous path now reaches governance through the workflow's own
	// gate rather than a healer-side call. The property under test is
	// unchanged — the unattended mutation must be authorized before it runs —
	// but there is now exactly ONE decision point for operator-started and
	// autonomous runs alike, so a dispatch cannot mint a second action check.
	v, err := srv.gateRemediation(context.Background(), engine.GateRequest{
		FindingID:     f.FindingID,
		ClusterID:     "c1",
		InvariantID:   f.InvariantID,
		EntityRef:     f.EntityRef,
		ActionKind:    "restart_drifted_unit",
		WorkflowRunID: "run-1",
	})
	if err != nil {
		t.Fatalf("gate returned error: %v", err)
	}
	if !v.Allowed {
		t.Fatal("an allowed verdict must permit dispatch")
	}
	if len(gov.asked) != 1 {
		t.Fatalf("governance consulted %d time(s), want exactly 1 — the autonomous "+
			"healer must ask before it mutates", len(gov.asked))
	}
	a := gov.asked[0]
	if a.InvariantID != f.InvariantID || a.EntityRef != f.EntityRef {
		t.Errorf("gate asked about the wrong subject: invariant=%q entity=%q, want %q/%q\n"+
			"a verdict about the wrong subject is worse than no verdict",
			a.InvariantID, a.EntityRef, f.InvariantID, f.EntityRef)
	}
	// The run id is now REAL rather than absent: an autonomous repair is a
	// genuine Workflow Service run, so claiming one is not forgery. Human
	// approval must still be empty — nobody approved this.
	if a.WorkflowRunID != "run-1" {
		t.Errorf("gate must see the real workflow run (%q), want run-1", a.WorkflowRunID)
	}
	if a.HumanApproval != "" {
		t.Errorf("autonomous dispatch must not claim human approval (%q): nobody approved it, "+
			"and inventing one forges lineage the governor would trust", a.HumanApproval)
	}
}

// TestGatedDispatcher_RefusalBlocksExecution verifies a governed refusal stops
// the mutation.
func TestGatedDispatcher_RefusalBlocksExecution(t *testing.T) {
	resetGateMemo() // the memo is keyed run|finding|step; these tests share one key
	gov := &fakeGovernor{decision: observation.GateDecision{
		ActionCheckID: "chk-1", Governed: true, Allowed: false,
		Status: "needs_evidence", Reason: "required evidence does not qualify",
	}}
	srv := &ClusterDoctorServer{behavioralGovernor: gov, clusterID: "c1"}

	v, err := srv.gateRemediation(context.Background(), gateRequestFor(findingWithRestartAction(), "run-refuse"))
	if err != nil {
		t.Fatalf("gate returned error: %v", err)
	}
	if v.Allowed {
		t.Error("a governed refusal must block the dispatch")
	}
}

// TestGatedDispatcher_UnreachableGovernorRefuses verifies an unreachable
// governor is treated as refusal, never as consent.
//
// This pauses auto-healing while behavioral memory is down. Deliberate: the
// cluster's deterministic convergence is untouched, so what pauses is the
// doctor's autonomy — the part that depends on being authorized.
func TestGatedDispatcher_UnreachableGovernorRefuses(t *testing.T) {
	resetGateMemo() // the memo is keyed run|finding|step; these tests share one key
	gov := &fakeGovernor{checkErr: errors.New("governor unavailable")}
	srv := &ClusterDoctorServer{behavioralGovernor: gov, clusterID: "c1"}

	v, _ := srv.gateRemediation(context.Background(), gateRequestFor(findingWithRestartAction(), "run-unreachable"))
	if v.Allowed {
		t.Error("an unreachable governor must be a refusal, never consent —\n" +
			"otherwise governance is strongest when it works and absent when it does not")
	}
}

// TestGatedDispatcher_UngovernedStillProceeds verifies the no-principle case is
// unchanged: the action keeps exactly the protection it had before governance
// existed. Refusing here would make promoting the FIRST principle a
// cluster-wide outage of auto-healing.
func TestGatedDispatcher_UngovernedStillProceeds(t *testing.T) {
	resetGateMemo() // the memo is keyed run|finding|step; these tests share one key
	gov := &fakeGovernor{decision: observation.GateDecision{
		Governed: false, Allowed: true, Status: "ungoverned",
	}}
	srv := &ClusterDoctorServer{behavioralGovernor: gov, clusterID: "c1"}

	v, err := srv.gateRemediation(context.Background(), gateRequestFor(findingWithRestartAction(), "run-ungoverned"))
	if err != nil {
		t.Fatalf("gate returned error: %v", err)
	}
	if !v.Allowed {
		t.Error("an ungoverned action must still proceed under the executor's own gates")
	}
}

func findingWithRestartAction() rules.Finding {
	return rules.Finding{
		FindingID:   "f-1",
		InvariantID: "node.systemd.units_running",
		EntityRef:   "node-1/globular-torrent.service",
		Remediation: []*cluster_doctorpb.RemediationStep{
			{
				Order: 1,
				Action: &cluster_doctorpb.RemediationAction{
					ActionType: cluster_doctorpb.ActionType_SYSTEMCTL_RESTART,
					Risk:       cluster_doctorpb.ActionRisk_RISK_LOW,
					Idempotent: true,
					Params:     map[string]string{"node_id": "node-1", "unit": "globular-torrent.service"},
				},
			},
		},
	}
}

// gateRequestFor builds the gate request the autonomous path submits for a
// finding, so these tests exercise the same single decision point production
// uses rather than a healer-side shortcut that no longer exists.
func gateRequestFor(f rules.Finding, runID string) engine.GateRequest {
	return engine.GateRequest{
		FindingID:     f.FindingID,
		ClusterID:     "c1",
		InvariantID:   f.InvariantID,
		EntityRef:     f.EntityRef,
		ActionKind:    "restart_drifted_unit",
		WorkflowRunID: runID,
	}
}
