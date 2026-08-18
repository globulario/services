package evolution

import (
	"context"
	"testing"

	behavioral "github.com/globulario/services/golang/ai_memory/behavioral/api"
)

type fakeRecorder struct{ signals, evidence, outcomes int }

func (f *fakeRecorder) RecordSignal(_ context.Context, _ *behavioral.RecordSignalRequest) (*behavioral.RecordSignalResponse, error) {
	f.signals++
	return &behavioral.RecordSignalResponse{SignalID: "sig-1"}, nil
}
func (f *fakeRecorder) RecordEvidence(_ context.Context, _ *behavioral.RecordEvidenceRequest) (*behavioral.RecordEvidenceResponse, error) {
	f.evidence++
	return &behavioral.RecordEvidenceResponse{EvidenceID: "ev"}, nil
}
func (f *fakeRecorder) RecordOutcome(_ context.Context, _ *behavioral.RecordOutcomeRequest) (*behavioral.RecordOutcomeResponse, error) {
	f.outcomes++
	return &behavioral.RecordOutcomeResponse{OutcomeID: "out-1"}, nil
}

func validLearning() SimulationLearning {
	return SimulationLearning{
		LearningSchemaVersion: 1,
		CreatedAt:             "2026-08-17T20:00:00Z",
		Source:                "globular-quickstart-simulation",
		Scenario:              "x",
		Suite:                 "resilience",
		Result:                "FAIL",
		SourceRevision:        "sim-sha",
		Change: ChangeBinding{
			ID:                  "chg-1",
			CandidateRepository: "globulario/services",
			CandidateRevision:   "cand-sha",
			PlanDigest:          "sha256:plan",
			SimulationRevision:  "sim-sha",
		},
		Proof:           SimulationProof{Claim: "stale authority is fenced", Determinism: Determinism{Replayable: true, Seed: "seed-1"}},
		CandidatePolicy: CandidatePolicy{LearningEnabled: true, CandidateTypes: []string{"failure_mode", "scenario"}, MayCreateCandidates: true, MayPromote: false},
		Authority:       SimulationAuthority{ProductionAuthoritative: false, PromotionRequired: true},
		EvidenceRef:     "evidence.json",
		ProofRef:        "scenario-proof.json",
	}
}

func TestSimulationLearningRejectsAuthorityEscalation(t *testing.T) {
	l := validLearning()
	l.Authority.ProductionAuthoritative = true
	if err := l.Validate(); err == nil {
		t.Fatal("expected authority rejection")
	}
	l = validLearning()
	l.CandidatePolicy.MayPromote = true
	if err := l.Validate(); err == nil {
		t.Fatal("expected promotion rejection")
	}
}

func TestSimulationLearningBindsChangeRevision(t *testing.T) {
	l := validLearning()
	l.Change.SimulationRevision = "other"
	if err := l.Validate(); err == nil {
		t.Fatal("expected simulation revision mismatch")
	}
}

func TestSimulationLearningRequiresFrozenPlan(t *testing.T) {
	l := validLearning()
	l.Change.PlanDigest = ""
	if err := l.Validate(); err == nil {
		t.Fatal("expected missing plan digest rejection")
	}
}

func TestIngestRecordsButDoesNotPromote(t *testing.T) {
	f := &fakeRecorder{}
	r, err := (SimulationIngestor{Recorder: f}).Ingest(context.Background(), validLearning())
	if err != nil {
		t.Fatal(err)
	}
	if f.signals != 1 || f.evidence != 2 || f.outcomes != 1 {
		t.Fatalf("unexpected writes: %+v", f)
	}
	if r.OutcomeID == "" || len(r.CandidateHints) != 2 {
		t.Fatalf("unexpected result: %+v", r)
	}
}
