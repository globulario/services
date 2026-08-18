package evolution

import (
	"path/filepath"
	"testing"
)

func TestEnvelopeRoundTripYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "change.yaml")
	e := NewChangeEnvelope("chg-roundtrip", ChangeFeature, "exercise lifecycle", "source-sha", RiskHigh)
	e.RequiredScenarios = []ScenarioRequirement{{Name: "scenario-a", Required: true}}
	if err := e.BindCandidate("globulario/services", "candidate-sha"); err != nil {
		t.Fatal(err)
	}
	if err := SaveChangeEnvelope(path, e); err != nil {
		t.Fatal(err)
	}
	got, err := LoadChangeEnvelope(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != e.ID || got.CandidateRevision != e.CandidateRevision || got.PlanDigest != e.PlanDigest {
		t.Fatalf("round trip mismatch: %+v", got)
	}
}

func TestMarkProvenOnlyAfterRequiredTestAndScenarioClosure(t *testing.T) {
	e := NewChangeEnvelope("chg-proof", ChangeSimulationRepair, "repair", "source-sha", RiskCritical)
	e.RequiredScenarios = []ScenarioRequirement{{Name: "scenario-a", Required: true}}
	e.RequiredTests = []TestRequirement{{
		Name:       "go-test-evolution",
		Repository: "globulario/services",
		Command:    []string{"go", "test", "./ai_memory/evolution"},
		Required:   true,
	}}
	if err := e.BindCandidate("globulario/services", "candidate-sha"); err != nil {
		t.Fatal(err)
	}
	if e.MarkProvenIfComplete() {
		t.Fatal("marked proven without proof")
	}
	e.AddOrReplaceProof(ProofRecord{
		Scenario:            "scenario-a",
		CandidateRepository: "globulario/services",
		CandidateRevision:   "candidate-sha",
		PlanDigest:          e.PlanDigest,
		Result:              "PASS",
		ProofEligible:       true,
	})
	if e.MarkProvenIfComplete() {
		t.Fatal("marked proven while required test was missing")
	}
	e.AddOrReplaceTest(TestRecord{
		Name:                "go-test-evolution",
		CandidateRepository: "globulario/services",
		CandidateRevision:   "candidate-sha",
		PlanDigest:          e.PlanDigest,
		Command:             []string{"go", "test", "./ai_memory/evolution"},
		Result:              "PASS",
	})
	if !e.MarkProvenIfComplete() || e.Stage != StageProven {
		t.Fatalf("expected PROVEN after test + scenario closure, got %s", e.Stage)
	}
}

func TestRequiredTestRejectsSubstitutedCommand(t *testing.T) {
	e := NewChangeEnvelope("chg-command", ChangeFeature, "feature", "source-sha", RiskHigh)
	e.RequiredTests = []TestRequirement{{
		Name:     "real-test",
		Command:  []string{"go", "test", "./..."},
		Required: true,
	}}
	if err := e.BindCandidate("globulario/services", "candidate-sha"); err != nil {
		t.Fatal(err)
	}
	e.Stage = StageProven
	e.Tests = []TestRecord{{
		Name:              "real-test",
		CandidateRevision: "candidate-sha",
		PlanDigest:        e.PlanDigest,
		Command:           []string{"true"},
		Result:            "PASS",
	}}
	if err := e.Validate(); err == nil {
		t.Fatal("expected substituted command to be rejected")
	}
}

func TestAddOrReplaceProofDoesNotDoubleCountSameCandidateScenario(t *testing.T) {
	e := ChangeEnvelope{}
	e.AddOrReplaceProof(ProofRecord{Scenario: "scenario-a", CandidateRevision: "sha", PlanDigest: "plan", Result: "FAIL"})
	e.AddOrReplaceProof(ProofRecord{Scenario: "scenario-a", CandidateRevision: "sha", PlanDigest: "plan", Result: "PASS"})
	if len(e.Proofs) != 1 || e.Proofs[0].Result != "PASS" {
		t.Fatalf("proof replacement failed: %+v", e.Proofs)
	}
}

func TestAddOrReplaceTestDoesNotDoubleCountSameCandidateTest(t *testing.T) {
	e := ChangeEnvelope{}
	e.AddOrReplaceTest(TestRecord{Name: "go-test", CandidateRevision: "sha", PlanDigest: "plan", Result: "FAIL"})
	e.AddOrReplaceTest(TestRecord{Name: "go-test", CandidateRevision: "sha", PlanDigest: "plan", Result: "PASS"})
	if len(e.Tests) != 1 || e.Tests[0].Result != "PASS" {
		t.Fatalf("test replacement failed: %+v", e.Tests)
	}
}
