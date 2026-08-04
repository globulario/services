package main

import (
	"context"
	"sync"
	"testing"

	observation "github.com/globulario/services/golang/ai_memory/domains/cluster_operator/observation"
	"github.com/globulario/services/golang/workflow/engine"
)

// F11: an autonomous dispatch carries no WorkflowRunID, so gateKey collapsed to
// "|<findingID>|<step>" — one stable key for the doctor's whole process life.
// The memo lookup sits BEFORE CheckAction, so a hit returns without re-resolving
// evidence or principles: a first needs_evidence verdict was replayed forever
// and the governed branch could never be reached however much evidence later
// arrived. The mirror case is equally wrong — an earlier allow reused after the
// governing evidence or principles changed.

type scriptedGovernor struct {
	mu       sync.Mutex
	calls    int
	verdicts []observation.GateDecision // consumed in order; last repeats
}

func (g *scriptedGovernor) CheckAction(context.Context, observation.ActionContext) (observation.GateDecision, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	i := g.calls
	g.calls++
	if i >= len(g.verdicts) {
		i = len(g.verdicts) - 1
	}
	return g.verdicts[i], nil
}
func (g *scriptedGovernor) RecordOutcome(context.Context, observation.OutcomeRecord) (string, error) {
	return "", nil
}
func (g *scriptedGovernor) GeneratePromotionCandidate(context.Context, observation.CandidateRequest) (observation.CandidateResult, error) {
	return observation.CandidateResult{}, nil
}
func (g *scriptedGovernor) count() int { g.mu.Lock(); defer g.mu.Unlock(); return g.calls }

func needsEvidence() observation.GateDecision {
	return observation.GateDecision{ActionCheckID: "chk-1", Governed: true, Allowed: false, Status: "needs_evidence"}
}
func allowed() observation.GateDecision {
	return observation.GateDecision{ActionCheckID: "chk-2", Governed: true, Allowed: true, Status: "allowed"}
}

func autonomousReq() engine.GateRequest {
	return engine.GateRequest{FindingID: "f-1", StepIndex: 0, InvariantID: "node.systemd.units_running", EntityRef: "n/u"}
}
func durableReq(runID string) engine.GateRequest {
	r := autonomousReq()
	r.WorkflowRunID = runID
	return r
}

// THE regression: needs_evidence must not be cached for autonomous dispatch.
func TestGovernanceMemo_AutonomousReevaluatesAfterEvidenceArrives(t *testing.T) {
	resetGateMemo()
	g := &scriptedGovernor{verdicts: []observation.GateDecision{needsEvidence(), allowed()}}
	srv := &ClusterDoctorServer{behavioralGovernor: g}

	v1, err := srv.gateRemediation(context.Background(), autonomousReq())
	if err != nil || v1.Allowed {
		t.Fatalf("first autonomous check must be needs_evidence; got %+v err=%v", v1, err)
	}
	// Evidence arrives; the governor would now allow.
	v2, err := srv.gateRemediation(context.Background(), autonomousReq())
	if err != nil {
		t.Fatalf("second check: %v", err)
	}
	if !v2.Allowed {
		t.Error("autonomous dispatch must be RE-EVALUATED after evidence arrives; " +
			"a memoized needs_evidence makes the governed branch permanently unreachable")
	}
	if g.count() != 2 {
		t.Errorf("governor must be consulted on every autonomous dispatch; calls=%d", g.count())
	}
}

// An earlier allow must not survive a change in evidence or principles.
func TestGovernanceMemo_AutonomousAllowNotReusedAfterChange(t *testing.T) {
	resetGateMemo()
	g := &scriptedGovernor{verdicts: []observation.GateDecision{allowed(), needsEvidence()}}
	srv := &ClusterDoctorServer{behavioralGovernor: g}
	if v, _ := srv.gateRemediation(context.Background(), autonomousReq()); !v.Allowed {
		t.Fatal("first check should allow")
	}
	if v, _ := srv.gateRemediation(context.Background(), autonomousReq()); v.Allowed {
		t.Error("an earlier allow must not be reused once governance changes")
	}
}

// An autonomous request must never create a memo entry.
func TestGovernanceMemo_AutonomousNeverMemoized(t *testing.T) {
	resetGateMemo()
	g := &scriptedGovernor{verdicts: []observation.GateDecision{needsEvidence()}}
	srv := &ClusterDoctorServer{behavioralGovernor: g}
	_, _ = srv.gateRemediation(context.Background(), autonomousReq())
	if _, ok := srv.lookupGateVerdict(gateKey(autonomousReq())); ok {
		t.Error("autonomous verdicts must not be stored in the durable-run memo")
	}
}

// Durable runs keep their idempotence — one decision per attempt.
func TestGovernanceMemo_DurableRunRemainsIdempotent(t *testing.T) {
	resetGateMemo()
	g := &scriptedGovernor{verdicts: []observation.GateDecision{allowed(), needsEvidence()}}
	srv := &ClusterDoctorServer{behavioralGovernor: g}
	v1, _ := srv.gateRemediation(context.Background(), durableReq("run-A"))
	v2, _ := srv.gateRemediation(context.Background(), durableReq("run-A"))
	if v1.ActionCheckID != v2.ActionCheckID || !v2.Allowed {
		t.Errorf("a replayed durable run must reuse its decision; got %+v then %+v", v1, v2)
	}
	if g.count() != 1 {
		t.Errorf("durable replay must consult the governor once; calls=%d", g.count())
	}
}

// Distinct durable runs evaluate independently.
func TestGovernanceMemo_DistinctRunsEvaluateIndependently(t *testing.T) {
	resetGateMemo()
	g := &scriptedGovernor{verdicts: []observation.GateDecision{allowed(), needsEvidence()}}
	srv := &ClusterDoctorServer{behavioralGovernor: g}
	if v, _ := srv.gateRemediation(context.Background(), durableReq("run-A")); !v.Allowed {
		t.Fatal("run-A should allow")
	}
	if v, _ := srv.gateRemediation(context.Background(), durableReq("run-B")); v.Allowed {
		t.Error("run-B must be evaluated independently of run-A")
	}
	if g.count() != 2 {
		t.Errorf("two distinct runs => two checks; calls=%d", g.count())
	}
}

// Concurrency: autonomous dispatches must be race-safe and all re-evaluated.
func TestGovernanceMemo_ConcurrentAutonomousRaceSafe(t *testing.T) {
	resetGateMemo()
	g := &scriptedGovernor{verdicts: []observation.GateDecision{allowed()}}
	srv := &ClusterDoctorServer{behavioralGovernor: g}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, _ = srv.gateRemediation(context.Background(), autonomousReq()) }()
	}
	wg.Wait()
	if g.count() != 16 {
		t.Errorf("every autonomous dispatch must be evaluated; calls=%d want 16", g.count())
	}
}

// Concurrency: durable-run replays stay single-decision.
func TestGovernanceMemo_ConcurrentDurableRunSingleDecision(t *testing.T) {
	resetGateMemo()
	g := &scriptedGovernor{verdicts: []observation.GateDecision{allowed()}}
	srv := &ClusterDoctorServer{behavioralGovernor: g}
	var wg sync.WaitGroup
	ids := make([]string, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			v, _ := srv.gateRemediation(context.Background(), durableReq("run-C"))
			ids[i] = v.ActionCheckID
		}(i)
	}
	wg.Wait()
	for _, id := range ids {
		if id != ids[0] {
			t.Fatalf("durable run must yield one stable ActionCheckID; got %v", ids)
		}
	}
}
