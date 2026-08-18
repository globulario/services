package evolution

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// provenFixture is a candidate whose required test and required scenario both
// carry a PASS record with complete evidence identity. Every test below starts
// from a genuinely PROVEN envelope and removes exactly one thing.
// provenFixture builds a PROVEN candidate. planMutators run before the candidate
// is bound, because changing an obligation afterwards invalidates the frozen
// plan digest and the envelope would then fail for that reason instead.
func provenFixture(t *testing.T, planMutators ...func(*ChangeEnvelope)) ChangeEnvelope {
	t.Helper()
	e := NewChangeEnvelope("chg-evidence", ChangeSimulationRepair, "repair", "source-sha", RiskCritical)
	e.RequiredTests = []TestRequirement{{
		Name:     "unit",
		Command:  []string{"go", "test", "./..."},
		Required: true,
	}}
	e.RequiredScenarios = []ScenarioRequirement{{Name: "chaos", Path: "tests/scenarios/chaos.yaml", Required: true}}
	for _, mutate := range planMutators {
		mutate(&e)
	}
	if err := e.BindCandidate("globulario/services", "candidate-sha"); err != nil {
		t.Fatal(err)
	}
	e.Tests = []TestRecord{{
		Name:                "unit",
		CandidateRepository: "globulario/services",
		CandidateRevision:   "candidate-sha",
		PlanDigest:          e.PlanDigest,
		Command:             []string{"go", "test", "./..."},
		Result:              "PASS",
		InvocationID:        "inv-t1",
		EvidenceRef:         "evidence/tests/unit.log",
		Digest:              "sha256:unit-evidence",
	}}
	e.Proofs = []ProofRecord{{
		Scenario:            "chaos",
		CandidateRepository: "globulario/services",
		CandidateRevision:   "candidate-sha",
		Repository:          "globulario/globular-quickstart",
		SimulationRevision:  "sim-sha",
		PlanDigest:          e.PlanDigest,
		InvocationID:        "inv-1",
		Result:              "PASS",
		ProofEligible:       true,
		ProofRef:            "evidence/inv-1/scenario-proof.json",
		EvidenceRef:         "evidence/inv-1/evidence.json",
		Digest:              "sha256:chaos-evidence",
	}}
	e.Stage = StageProven
	if err := e.Validate(); err != nil {
		t.Fatalf("fixture should be PROVEN: %v", err)
	}
	return e
}

func TestValidEvidencePermitsProven(t *testing.T) {
	e := provenFixture(t)
	if err := e.Validate(); err != nil {
		t.Fatalf("complete evidence identity rejected: %v", err)
	}
	if !e.MarkProvenIfComplete() || e.Stage != StageProven {
		t.Fatalf("expected PROVEN, got %s", e.Stage)
	}
}

func TestPassWithoutEvidenceReferenceCannotBeProven(t *testing.T) {
	t.Run("test record", func(t *testing.T) {
		e := provenFixture(t)
		e.Tests[0].EvidenceRef = ""
		assertNotProvable(t, e, "no evidence_ref")
	})
	t.Run("scenario proof_ref", func(t *testing.T) {
		e := provenFixture(t)
		e.Proofs[0].ProofRef = ""
		assertNotProvable(t, e, "no proof_ref")
	})
	t.Run("scenario evidence_ref", func(t *testing.T) {
		e := provenFixture(t)
		e.Proofs[0].EvidenceRef = ""
		assertNotProvable(t, e, "no evidence_ref")
	})
}

func TestPassWithoutDigestCannotBeProven(t *testing.T) {
	t.Run("test record", func(t *testing.T) {
		e := provenFixture(t)
		e.Tests[0].Digest = "   "
		assertNotProvable(t, e, "no evidence digest")
	})
	t.Run("scenario proof", func(t *testing.T) {
		e := provenFixture(t)
		e.Proofs[0].Digest = ""
		assertNotProvable(t, e, "no proof/evidence digest")
	})
}

// assertNotProvable states the whole property in one place: a PROVEN envelope
// in this shape must fail validation, must not be reachable by reconciliation,
// and must not be persistable.
func assertNotProvable(t *testing.T, e ChangeEnvelope, wantErr string) {
	t.Helper()
	err := e.Validate()
	if err == nil {
		t.Fatal("expected PROVEN to be refused without evidence identity")
	}
	if !strings.Contains(err.Error(), wantErr) {
		t.Fatalf("expected error mentioning %q, got %v", wantErr, err)
	}
	reconciled := e
	if reconciled.MarkProvenIfComplete() {
		t.Fatal("reconciliation restored PROVEN without evidence identity")
	}
	if reconciled.Stage != StageCandidate {
		t.Fatalf("expected downgrade to CANDIDATE, got %s", reconciled.Stage)
	}
	if err := SaveChangeEnvelope(filepath.Join(t.TempDir(), "change.yaml"), e); err == nil {
		t.Fatal("persisted a PROVEN envelope with no evidence identity")
	}
}

func TestStoredProvenWithoutEvidenceLoadsAsCandidate(t *testing.T) {
	// A stale PROVEN claim on disk must not survive a load that cannot
	// substantiate it. The load demotes; it never carries the claim forward and
	// never fails open.
	path := filepath.Join(t.TempDir(), "change.yaml")
	e := provenFixture(t)
	if err := SaveChangeEnvelope(path, e); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tampered := strings.Replace(
		string(raw), "digest: sha256:unit-evidence", `digest: ""`, 1,
	)
	if tampered == string(raw) {
		t.Fatal("fixture did not contain the expected evidence digest")
	}
	if err := os.WriteFile(path, []byte(tampered), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadChangeEnvelope(path)
	if err != nil {
		t.Fatalf("load should demote rather than fail: %v", err)
	}
	if loaded.Stage != StageCandidate {
		t.Fatalf("stale PROVEN survived a load it could not substantiate: %s", loaded.Stage)
	}
}

func TestLoadStillRefusesUnrepairableEnvelope(t *testing.T) {
	// Demotion is only ever to CANDIDATE. An envelope that is invalid for a
	// reason CANDIDATE does not fix must still be a hard refusal.
	path := filepath.Join(t.TempDir(), "change.yaml")
	e := provenFixture(t)
	if err := SaveChangeEnvelope(path, e); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	broken := strings.Replace(string(raw), "plan_digest: sha256:", "plan_digest: sha256:ff", 1)
	if err := os.WriteFile(path, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadChangeEnvelope(path); err == nil {
		t.Fatal("expected a plan-digest mismatch to be refused, not demoted")
	}
}

func TestVerifyEvidenceArtifactsRejectsTamperedArtifact(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "unit.log")
	if err := os.WriteFile(log, []byte("original proof output\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	digest, err := DigestFiles(log)
	if err != nil {
		t.Fatal(err)
	}

	e := provenFixture(t)
	// Narrow to the test leg so the assertion is about digest re-derivation,
	// not about which artifacts a scenario happens to name.
	e.RequiredScenarios = nil
	e.Proofs = nil
	e.Tests[0].EvidenceRef = log
	e.Tests[0].Digest = digest
	if err := e.VerifyEvidenceArtifacts(); err != nil {
		t.Fatalf("untouched artifact rejected: %v", err)
	}

	// Replaced content, same path: identity metadata still looks complete, so
	// only re-deriving the digest can catch it.
	if err := os.WriteFile(log, []byte("substituted proof output\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err = e.VerifyEvidenceArtifacts()
	if err == nil || !strings.Contains(err.Error(), "changed after it was recorded") {
		t.Fatalf("expected tampered artifact rejection, got %v", err)
	}

	// Deleted artifact is equally fatal: an unreadable digest is not proof.
	if err := os.Remove(log); err != nil {
		t.Fatal(err)
	}
	if err := e.VerifyEvidenceArtifacts(); err == nil {
		t.Fatal("expected a deleted proof artifact to invalidate the claim")
	}
}

func TestVerifyEvidenceArtifactsIgnoresPreProvenStages(t *testing.T) {
	e := provenFixture(t)
	e.Stage = StageCandidate
	if err := e.VerifyEvidenceArtifacts(); err != nil {
		t.Fatalf("candidate stage should not require reachable artifacts: %v", err)
	}
}

func TestAdmittedEnvelopeIsNeverDemotedOnLoad(t *testing.T) {
	// Demotion exists so an unsubstantiated PROVEN claim cannot stand. It must
	// stop there. ADMITTED and later are accepted history: an envelope whose
	// evidence no longer validates at those stages is a hard refusal, because
	// silently rewriting it to CANDIDATE would erase the accepted record
	// instead of forcing a new candidate.
	path := filepath.Join(t.TempDir(), "change.yaml")
	e := provenFixture(t)
	e.Stage = StageAdmitted
	e.Admission = AdmissionRecord{
		Status:     "ACCEPT",
		Revision:   e.CandidateRevision,
		PlanDigest: e.PlanDigest,
		Ref:        "sensei://admission/chg-evidence",
		Actor:      "sensei",
		At:         "2026-08-18T00:00:00Z",
	}
	if err := SaveChangeEnvelope(path, e); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tampered := strings.Replace(string(raw), "digest: sha256:unit-evidence", `digest: ""`, 1)
	if tampered == string(raw) {
		t.Fatal("fixture did not contain the expected evidence digest")
	}
	if err := os.WriteFile(path, []byte(tampered), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadChangeEnvelope(path)
	if err == nil {
		t.Fatalf("admitted history was silently rewritten to %s", loaded.Stage)
	}
}
