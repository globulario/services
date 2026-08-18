package evolution

import "testing"

func TestBindCandidateClearsEvidenceWhenRevisionChanges(t *testing.T) {
	e := NewChangeEnvelope("chg-bind", ChangeFeature, "feature", "source", RiskHigh)
	if err := e.BindCandidate("globulario/services", "sha-a"); err != nil {
		t.Fatal(err)
	}
	e.Tests = []TestRecord{{Name: "test", CandidateRevision: "sha-a", Result: "PASS"}}
	e.Proofs = []ProofRecord{{Scenario: "scenario", CandidateRevision: "sha-a", Result: "PASS", ProofEligible: true}}
	if err := e.BindCandidate("globulario/services", "sha-b"); err != nil {
		t.Fatal(err)
	}
	if len(e.Tests) != 0 || len(e.Proofs) != 0 || e.Stage != StageCandidate {
		t.Fatalf("stale evidence survived candidate rebind: %+v", e)
	}
}

func TestBindCandidateRefusesAfterAdmission(t *testing.T) {
	e := NewChangeEnvelope("chg-admitted", ChangeFeature, "feature", "source", RiskHigh)
	e.Stage = StageAdmitted
	e.CandidateRepository = "globulario/services"
	e.CandidateRevision = "sha-a"
	e.Admission = AdmissionRecord{Status: "ACCEPT", Revision: "sha-a"}
	if err := e.BindCandidate("globulario/services", "sha-b"); err == nil {
		t.Fatal("expected admitted candidate to be immutable")
	}
}

func TestProofStatusNamesMissingEvidenceAndAdmissionBoundary(t *testing.T) {
	e := NewChangeEnvelope("chg-status", ChangeSimulationRepair, "repair", "source", RiskCritical)
	if err := e.BindCandidate("globulario/services", "sha"); err != nil {
		t.Fatal(err)
	}
	e.RequiredTests = []TestRequirement{{Name: "unit", Command: []string{"go", "test"}, Required: true}}
	e.RequiredScenarios = []ScenarioRequirement{{Name: "chaos", Required: true}}
	status := e.ProofStatus()
	if len(status.MissingRequiredTests) != 1 || len(status.MissingScenarios) != 1 || status.ProofComplete {
		t.Fatalf("unexpected incomplete status: %+v", status)
	}
	e.Tests = []TestRecord{{Name: "unit", CandidateRevision: "sha", Command: []string{"go", "test"}, Result: "PASS"}}
	e.Proofs = []ProofRecord{{Scenario: "chaos", CandidateRevision: "sha", Result: "PASS", ProofEligible: true}}
	e.Stage = StageProven
	status = e.ProofStatus()
	if !status.ProofComplete || status.NextAuthorityBoundary != "sensei_admission" {
		t.Fatalf("unexpected complete status: %+v", status)
	}
}
