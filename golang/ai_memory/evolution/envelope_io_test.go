package evolution

import (
	"path/filepath"
	"testing"
)

func TestEnvelopeRoundTripYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "change.yaml")
	e := NewChangeEnvelope("chg-roundtrip", ChangeFeature, "exercise lifecycle", "source-sha", RiskHigh)
	e.Stage = StageCandidate
	e.CandidateRepository = "globulario/services"
	e.CandidateRevision = "candidate-sha"
	e.RequiredScenarios = []ScenarioRequirement{{Name: "scenario-a", Required: true}}
	if err := SaveChangeEnvelope(path, e); err != nil {
		t.Fatal(err)
	}
	got, err := LoadChangeEnvelope(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != e.ID || got.CandidateRevision != e.CandidateRevision {
		t.Fatalf("round trip mismatch: %+v", got)
	}
}

func TestMarkProvenOnlyAfterRequiredProofClosure(t *testing.T) {
	e := NewChangeEnvelope("chg-proof", ChangeSimulationRepair, "repair", "source-sha", RiskCritical)
	e.Stage = StageCandidate
	e.CandidateRepository = "globulario/services"
	e.CandidateRevision = "candidate-sha"
	e.RequiredScenarios = []ScenarioRequirement{{Name: "scenario-a", Required: true}}
	if e.MarkProvenIfComplete() {
		t.Fatal("marked proven without proof")
	}
	e.AddOrReplaceProof(ProofRecord{
		Scenario: "scenario-a", CandidateRepository: "globulario/services",
		CandidateRevision: "candidate-sha", Result: "PASS", ProofEligible: true,
	})
	if !e.MarkProvenIfComplete() || e.Stage != StageProven {
		t.Fatalf("expected PROVEN after closure, got %s", e.Stage)
	}
}

func TestAddOrReplaceProofDoesNotDoubleCountSameCandidateScenario(t *testing.T) {
	e := ChangeEnvelope{}
	e.AddOrReplaceProof(ProofRecord{Scenario: "scenario-a", CandidateRevision: "sha", Result: "FAIL"})
	e.AddOrReplaceProof(ProofRecord{Scenario: "scenario-a", CandidateRevision: "sha", Result: "PASS"})
	if len(e.Proofs) != 1 || e.Proofs[0].Result != "PASS" {
		t.Fatalf("proof replacement failed: %+v", e.Proofs)
	}
}
