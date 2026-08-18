package evolution

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// newTestRunner returns a signing key path and the matching trusted public key.
// The key is written 0600 because LoadRunnerKey refuses group/world-accessible
// material — the check exists so a "runner key" the agent can read is rejected
// rather than quietly accepted.
func newTestRunner(t *testing.T) (keyPath string, trusted []ed25519.PublicKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keyPath = filepath.Join(t.TempDir(), "runner.key")
	if err := os.WriteFile(keyPath, []byte(hex.EncodeToString(priv.Seed())), 0o600); err != nil {
		t.Fatal(err)
	}
	return keyPath, []ed25519.PublicKey{pub}
}

// attestFixture signs the certifying records of an already-complete envelope, so
// tests about other properties do not each have to run a real runner.
func attestFixture(t *testing.T, e *ChangeEnvelope, keyPath string) {
	t.Helper()
	key, err := LoadRunnerKey(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for i := range e.Tests {
		rec := &e.Tests[i]
		if rec.Result != "PASS" {
			continue
		}
		var command string
		for _, req := range e.RequiredTests {
			if req.Name == rec.Name {
				command = joinCommand(req.Command)
			}
		}
		r, err := ProofOccurrenceReceipt{
			ChangeID: e.ID, CandidateRepository: e.CandidateRepository,
			CandidateRevision: e.CandidateRevision, PlanDigest: e.PlanDigest,
			ObligationKind: "test", ObligationName: rec.Name, ObligationRef: command,
			InvocationID: rec.InvocationID, Result: rec.Result, ProofEligible: true,
			EvidenceDigest: rec.Digest, ObservedAt: now,
		}.Sign(key)
		if err != nil {
			t.Fatal(err)
		}
		rec.Receipt = &r
	}
	for i := range e.Proofs {
		pr := &e.Proofs[i]
		if pr.Result != "PASS" || !pr.ProofEligible {
			continue
		}
		var path string
		for _, req := range e.RequiredScenarios {
			if req.Name == pr.Scenario {
				path = req.Path
			}
		}
		r, err := ProofOccurrenceReceipt{
			ChangeID: e.ID, CandidateRepository: e.CandidateRepository,
			CandidateRevision: e.CandidateRevision, PlanDigest: e.PlanDigest,
			ObligationKind: "scenario", ObligationName: pr.Scenario, ObligationRef: path,
			InvocationID: pr.InvocationID, SimulationRevision: pr.SimulationRevision,
			Result: pr.Result, ProofEligible: pr.ProofEligible,
			EvidenceDigest: pr.Digest, ObservedAt: now,
		}.Sign(key)
		if err != nil {
			t.Fatal(err)
		}
		pr.Receipt = &r
	}
}

func joinCommand(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += " "
		}
		out += p
	}
	return out
}

// testRunnerKey is the one-liner form for tests that call a runner directly.
func testRunnerKey(t *testing.T) string {
	t.Helper()
	keyPath, _ := newTestRunner(t)
	return keyPath
}
