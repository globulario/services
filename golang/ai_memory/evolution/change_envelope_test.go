package evolution

import "testing"

func TestProvenRequiresProofForExactCandidate(t *testing.T) {
	e := NewChangeEnvelope("chg-1", ChangeSimulationRepair, "repair stale authority", "source-sha", RiskHigh)
	e.RequiredScenarios = []ScenarioRequirement{{Name: "controller-zombie-after-lease-loss", Path: "tests/scenarios/controller-zombie-after-lease-loss.yaml", Required: true}}
	if err := e.BindCandidate("globulario/services", "candidate-sha"); err != nil {
		t.Fatal(err)
	}
	e.Stage = StageProven
	e.Proofs = []ProofRecord{{
		Scenario:            "controller-zombie-after-lease-loss",
		CandidateRepository: "globulario/services",
		CandidateRevision:   "other-sha",
		PlanDigest:          e.PlanDigest,
		Result:              "PASS",
		ProofEligible:       true,
		Repository:          "globulario/globular-quickstart",
		SimulationRevision:  "sim-sha",
		InvocationID:        "inv-1",
		ProofRef:            "runs/inv-1/scenario-proof.json",
		EvidenceRef:         "runs/inv-1/evidence.json",
		Digest:              "sha256:proof",
	}}
	if err := e.Validate(); err == nil {
		t.Fatal("expected candidate revision mismatch")
	}
	e.Proofs[0].CandidateRevision = "candidate-sha"
	if err := e.Validate(); err != nil {
		t.Fatalf("valid proof rejected: %v", err)
	}
}

func TestAdmissionMustBindCandidateAndPlan(t *testing.T) {
	e := NewChangeEnvelope("chg-2", ChangeFeature, "new feature", "source-sha", RiskMedium)
	e.RequiredTests = []TestRequirement{{
		Name: "unit", Command: []string{"go", "test"}, Required: true,
	}}
	if err := e.BindCandidate("globulario/services", "candidate-sha"); err != nil {
		t.Fatal(err)
	}
	e.Stage = StageAdmitted
	e.Admission = AdmissionRecord{Status: "ACCEPT", Revision: "wrong-sha", PlanDigest: e.PlanDigest}
	if err := e.Validate(); err == nil {
		t.Fatal("expected admission revision mismatch")
	}
	e.Admission.Revision = "candidate-sha"
	e.Admission.PlanDigest = "sha256:wrong-plan"
	if err := e.Validate(); err == nil {
		t.Fatal("expected admission plan mismatch")
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

func TestCandidatePlanDigestRejectsShrinkingProofObligations(t *testing.T) {
	e := NewChangeEnvelope("chg-plan-freeze", ChangeSimulationRepair, "repair", "source", RiskCritical)
	e.RequiredScenarios = []ScenarioRequirement{{Name: "scenario-a", Path: "tests/scenarios/scenario-a.yaml", Required: true}}
	if err := e.BindCandidate("globulario/services", "candidate-sha"); err != nil {
		t.Fatal(err)
	}
	frozen := e.PlanDigest
	e.RequiredScenarios = nil
	if err := e.Validate(); err == nil {
		t.Fatal("expected proof-plan mutation to invalidate candidate")
	}
	if e.PlanDigest != frozen {
		t.Fatal("proof-plan mutation must not silently rewrite frozen plan digest")
	}
}
