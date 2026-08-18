package evolution

import "testing"

func TestProvenRequiresProofForExactCandidate(t *testing.T) {
	e := NewChangeEnvelope("chg-1", ChangeSimulationRepair, "repair stale authority", "source-sha", RiskHigh)
	e.Stage = StageProven
	e.CandidateRepository = "globulario/services"
	e.CandidateRevision = "candidate-sha"
	e.RequiredScenarios = []ScenarioRequirement{{Name: "controller-zombie-after-lease-loss", Required: true}}
	e.Proofs = []ProofRecord{{Scenario: "controller-zombie-after-lease-loss", CandidateRepository: "globulario/services", CandidateRevision: "other-sha", Result: "PASS", ProofEligible: true}}
	if err := e.Validate(); err == nil {
		t.Fatal("expected candidate revision mismatch")
	}
	e.Proofs[0].CandidateRevision = "candidate-sha"
	if err := e.Validate(); err != nil {
		t.Fatalf("valid proof rejected: %v", err)
	}
}

func TestAdmissionMustBindCandidate(t *testing.T) {
	e := NewChangeEnvelope("chg-2", ChangeFeature, "new feature", "source-sha", RiskMedium)
	e.Stage = StageAdmitted
	e.CandidateRepository = "globulario/services"
	e.CandidateRevision = "candidate-sha"
	e.Admission = AdmissionRecord{Status: "ACCEPT", Revision: "wrong-sha"}
	if err := e.Validate(); err == nil {
		t.Fatal("expected admission revision mismatch")
	}
}

func TestIdentityDigestIgnoresListOrdering(t *testing.T) {
	a := NewChangeEnvelope("chg-3", ChangeIncidentRepair, "repair", "source", RiskCritical)
	a.AuthorityScope = []string{"b", "a"}
	b := a
	b.AuthorityScope = []string{"a", "b"}
	da, err := a.IdentityDigest()
	if err != nil {
		t.Fatal(err)
	}
	db, err := b.IdentityDigest()
	if err != nil {
		t.Fatal(err)
	}
	if da != db {
		t.Fatalf("identity digest changed with set ordering: %s != %s", da, db)
	}
}
