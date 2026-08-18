package evolution

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The attack this whole mechanism exists to stop.
//
// An author who never runs anything can still write evidence files, compute
// their digests, invent an invocation id, and record matching PASS entries.
// Every integrity check passes, because the envelope and the files genuinely do
// agree — that is consistency, not authenticity. Re-hashing author-controlled
// bytes proves what the bytes are, never why anyone should believe them.
func TestForgedEnvelopeCannotReachProven(t *testing.T) {
	dir := t.TempDir()

	// The forger writes its own "evidence" and digests it honestly.
	testLog := filepath.Join(dir, "unit.log")
	if err := os.WriteFile(testLog, []byte("ok  fabricated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	proofJSON := filepath.Join(dir, "scenario-proof.json")
	evidenceJSON := filepath.Join(dir, "evidence.json")
	for _, f := range []string{proofJSON, evidenceJSON} {
		if err := os.WriteFile(f, []byte(`{"fabricated":true}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	testDigest, err := DigestFiles(testLog)
	if err != nil {
		t.Fatal(err)
	}
	scenarioDigest, err := DigestFiles(proofJSON, evidenceJSON)
	if err != nil {
		t.Fatal(err)
	}

	e := NewChangeEnvelope("chg-forged", ChangeSimulationRepair, "repair", "source-sha", RiskCritical)
	e.RequiredTests = []TestRequirement{{
		Name: "unit", Command: []string{"go", "test", "./..."}, Required: true,
	}}
	e.RequiredScenarios = []ScenarioRequirement{{
		Name: "chaos", Path: "tests/scenarios/chaos.yaml", Required: true,
	}}
	if err := e.BindCandidate("globulario/services", "candidate-sha"); err != nil {
		t.Fatal(err)
	}
	e.Tests = []TestRecord{{
		Name: "unit", CandidateRepository: "globulario/services",
		CandidateRevision: "candidate-sha", PlanDigest: e.PlanDigest,
		InvocationID: "inv-i-made-this-up", Command: []string{"go", "test", "./..."},
		Result: "PASS", EvidenceRef: testLog, Digest: testDigest,
	}}
	e.Proofs = []ProofRecord{{
		Scenario: "chaos", CandidateRepository: "globulario/services",
		CandidateRevision: "candidate-sha", Repository: CanonicalSimulationRepository,
		SimulationRevision: "sim-sha", PlanDigest: e.PlanDigest,
		InvocationID: "inv-i-made-this-up-too", Result: "PASS", ProofEligible: true,
		ProofRef: proofJSON, EvidenceRef: evidenceJSON, Digest: scenarioDigest,
	}}

	// Everything the envelope can check about itself is satisfied.
	if err := e.VerifyEvidenceArtifacts(); err != nil {
		t.Fatalf("the forgery is self-consistent by construction, so this must pass: %v", err)
	}
	status := e.ProofStatus()
	if !status.ObligationsComplete {
		t.Fatalf("the forgery declares complete obligations: %+v", status)
	}

	// And it still cannot certify.
	if e.MarkProvenIfComplete() {
		t.Fatal("a fabricated envelope reached PROVEN — evidence integrity was mistaken for provenance")
	}
	e.Stage = StageProven
	err = e.Validate()
	if err == nil {
		t.Fatal("a fabricated envelope validated as PROVEN")
	}
	if !strings.Contains(err.Error(), "attestation") {
		t.Fatalf("refusal did not name the missing attestation: %v", err)
	}
	// The status says precisely which occurrences are unattested, so an operator
	// sees "complete but unproven" rather than a bare failure.
	if len(status.AttestationUnproven) != 2 {
		t.Fatalf("both occurrences should be reported unattested: %+v", status.AttestationUnproven)
	}
}

// A receipt signed by a key nobody trusts is not attestation either.
func TestReceiptFromUntrustedRunnerIsRefused(t *testing.T) {
	e := provenFixture(t)

	// Re-sign every occurrence with a rogue key, then verify against a different
	// trusted set — the shape of an agent that generated its own "runner".
	roguePath, _ := newTestRunner(t)
	attestFixture(t, &e, roguePath)
	_, honestTrusted := newTestRunner(t)

	err := e.VerifyAttestations(honestTrusted)
	if err == nil {
		t.Fatal("a receipt signed by an untrusted key was accepted as attestation")
	}
	if !strings.Contains(err.Error(), "not a trusted runner") {
		t.Fatalf("expected an untrusted-runner refusal, got %v", err)
	}
}

// A genuine receipt for one obligation must not be reusable on another.
func TestReceiptCannotBeMovedBetweenObligations(t *testing.T) {
	e := provenFixture(t)
	if len(e.Tests) == 0 || len(e.Proofs) == 0 {
		t.Skip("fixture shape changed")
	}
	stolen := *e.Proofs[0].Receipt
	e.Tests[0].Receipt = &stolen

	keyPath, trusted := newTestRunner(t)
	_ = keyPath
	if err := e.VerifyAttestations(trusted); err == nil {
		t.Fatal("a scenario receipt was accepted as attestation for a test obligation")
	}
}

// Tampering with an attested value must invalidate the signature.
func TestReceiptSignatureCoversTheBindings(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := ProofOccurrenceReceipt{
		ChangeID: "chg-1", CandidateRepository: "globulario/services",
		CandidateRevision: "cand", PlanDigest: "sha256:plan",
		ObligationKind: "test", ObligationName: "unit", ObligationRef: "go test",
		InvocationID: "inv-1", Result: "PASS", ProofEligible: true,
		EvidenceDigest: "sha256:x", ObservedAt: "2026-08-18T00:00:00Z",
	}.Sign(priv)
	if err != nil {
		t.Fatal(err)
	}
	trusted := []ed25519.PublicKey{pub}
	if err := receipt.VerifySignature(trusted); err != nil {
		t.Fatalf("a freshly signed receipt must verify: %v", err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*ProofOccurrenceReceipt)
	}{
		{"result", func(r *ProofOccurrenceReceipt) { r.Result = "FAIL" }},
		{"candidate revision", func(r *ProofOccurrenceReceipt) { r.CandidateRevision = "other" }},
		{"evidence digest", func(r *ProofOccurrenceReceipt) { r.EvidenceDigest = "sha256:other" }},
		{"obligation name", func(r *ProofOccurrenceReceipt) { r.ObligationName = "other" }},
		{"invocation", func(r *ProofOccurrenceReceipt) { r.InvocationID = "inv-2" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tampered := receipt
			tc.mutate(&tampered)
			if err := tampered.VerifySignature(trusted); err == nil {
				t.Fatalf("changing %s did not invalidate the signature", tc.name)
			}
		})
	}
}

// A key the agent can read is not a privilege boundary.
func TestRunnerKeyMustNotBeAgentReadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; filesystem permissions are not enforced")
	}
	keyPath, _ := newTestRunner(t)
	if err := os.Chmod(keyPath, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRunnerKey(keyPath); err == nil {
		t.Fatal("a group/world-readable runner key was accepted as a signing identity")
	}
}
