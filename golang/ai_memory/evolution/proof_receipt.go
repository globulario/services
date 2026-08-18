package evolution

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ProofOccurrenceReceipt is a trusted runner's independent statement that it
// executed one obligation and observed one result.
//
// It exists because re-hashing evidence proves the wrong thing. Recomputing a
// digest shows that an envelope and a set of files agree with each other, which
// is consistency, not authenticity: an author who writes the files, computes
// their digests, and records matching PASS entries satisfies every such check
// while nothing was ever executed. Evidence integrity is not evidence
// provenance.
//
// The receipt closes that by originating outside the object being judged. The
// runner — not the envelope, not the agent asking for the run — binds the
// candidate, the frozen plan, the obligation, the invocation, the observed
// process result and the evidence digests, and signs over the canonical form.
// The envelope may then carry the receipt, but it cannot manufacture one.
type ProofOccurrenceReceipt struct {
	SchemaVersion int `json:"schema_version" yaml:"schema_version"`

	ChangeID            string `json:"change_id" yaml:"change_id"`
	CandidateRepository string `json:"candidate_repository" yaml:"candidate_repository"`
	CandidateRevision   string `json:"candidate_revision" yaml:"candidate_revision"`
	PlanDigest          string `json:"plan_digest" yaml:"plan_digest"`

	// ObligationKind is "test" or "scenario"; ObligationName is the frozen
	// requirement name, and ObligationRef the frozen command or scenario path.
	ObligationKind string `json:"obligation_kind" yaml:"obligation_kind"`
	ObligationName string `json:"obligation_name" yaml:"obligation_name"`
	ObligationRef  string `json:"obligation_ref" yaml:"obligation_ref"`

	InvocationID       string `json:"invocation_id" yaml:"invocation_id"`
	SimulationRevision string `json:"simulation_revision,omitempty" yaml:"simulation_revision,omitempty"`

	// Result and ProofEligible are what the runner observed, not what the
	// envelope would like to record.
	Result         string `json:"result" yaml:"result"`
	ProofEligible  bool   `json:"proof_eligible" yaml:"proof_eligible"`
	EvidenceDigest string `json:"evidence_digest" yaml:"evidence_digest"`

	ObservedAt string `json:"observed_at" yaml:"observed_at"`

	// RunnerID is the signing key's fingerprint. It answers "which runner", and
	// is redundant with the signature only when the key is already trusted.
	RunnerID  string `json:"runner_id" yaml:"runner_id"`
	Signature string `json:"signature" yaml:"signature"`
}

const ProofReceiptSchemaVersion = 1

// signingBytes is the canonical form the signature covers. Signature and any
// future non-attested field are excluded, so a receipt cannot be re-signed over
// a different meaning by moving a value between fields.
func (r ProofOccurrenceReceipt) signingBytes() ([]byte, error) {
	attested := struct {
		SchemaVersion       int    `json:"schema_version"`
		ChangeID            string `json:"change_id"`
		CandidateRepository string `json:"candidate_repository"`
		CandidateRevision   string `json:"candidate_revision"`
		PlanDigest          string `json:"plan_digest"`
		ObligationKind      string `json:"obligation_kind"`
		ObligationName      string `json:"obligation_name"`
		ObligationRef       string `json:"obligation_ref"`
		InvocationID        string `json:"invocation_id"`
		SimulationRevision  string `json:"simulation_revision"`
		Result              string `json:"result"`
		ProofEligible       bool   `json:"proof_eligible"`
		EvidenceDigest      string `json:"evidence_digest"`
		ObservedAt          string `json:"observed_at"`
		RunnerID            string `json:"runner_id"`
	}{
		SchemaVersion: r.SchemaVersion, ChangeID: r.ChangeID,
		CandidateRepository: r.CandidateRepository, CandidateRevision: r.CandidateRevision,
		PlanDigest: r.PlanDigest, ObligationKind: r.ObligationKind,
		ObligationName: r.ObligationName, ObligationRef: r.ObligationRef,
		InvocationID: r.InvocationID, SimulationRevision: r.SimulationRevision,
		Result: r.Result, ProofEligible: r.ProofEligible,
		EvidenceDigest: r.EvidenceDigest, ObservedAt: r.ObservedAt, RunnerID: r.RunnerID,
	}
	return json.Marshal(attested)
}

// Sign is the runner's operation. The key belongs to the runner; an agent that
// can call this with its own key has already crossed the boundary this exists
// to draw, which is why LoadRunnerKey refuses agent-writable key material.
func (r ProofOccurrenceReceipt) Sign(priv ed25519.PrivateKey) (ProofOccurrenceReceipt, error) {
	r.SchemaVersion = ProofReceiptSchemaVersion
	r.RunnerID = RunnerIdentity(priv.Public().(ed25519.PublicKey))
	msg, err := r.signingBytes()
	if err != nil {
		return ProofOccurrenceReceipt{}, fmt.Errorf("canonicalize receipt: %w", err)
	}
	r.Signature = hex.EncodeToString(ed25519.Sign(priv, msg))
	return r, nil
}

// VerifySignature checks the receipt against a set of trusted runner keys. It
// needs no evidence files, so a reviewer far from the original run can still
// establish who attested what.
func (r ProofOccurrenceReceipt) VerifySignature(trusted []ed25519.PublicKey) error {
	if len(trusted) == 0 {
		return fmt.Errorf("no trusted runner keys configured; attestation cannot be established")
	}
	if strings.TrimSpace(r.Signature) == "" {
		return fmt.Errorf("receipt carries no signature")
	}
	sig, err := hex.DecodeString(r.Signature)
	if err != nil {
		return fmt.Errorf("receipt signature is not decodable: %w", err)
	}
	msg, err := r.signingBytes()
	if err != nil {
		return err
	}
	for _, pub := range trusted {
		if RunnerIdentity(pub) != r.RunnerID {
			continue
		}
		if ed25519.Verify(pub, msg, sig) {
			return nil
		}
		return fmt.Errorf("receipt signature does not verify against runner %s", r.RunnerID)
	}
	return fmt.Errorf("receipt names runner %s, which is not a trusted runner", r.RunnerID)
}

// RunnerIdentity is the stable fingerprint of a runner's public key.
func RunnerIdentity(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return "runner:" + hex.EncodeToString(sum[:8])
}

// LoadRunnerKey reads a runner signing key and refuses material the calling
// identity's group or the world can write.
//
// The privilege boundary is the whole point. Putting the receipt in another
// directory achieves nothing if the same agent can write it, and a runner that
// signs files the agent prepared has only moved the self-assertion sideways.
// This check is necessary, not sufficient: full separation means the key is
// owned by a distinct OS identity the agent cannot read at all, which is a
// deployment property this function can only partially observe.
func LoadRunnerKey(path string) (ed25519.PrivateKey, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("read runner key: %w", err)
	}
	if mode := info.Mode().Perm(); mode&0o077 != 0 {
		return nil, fmt.Errorf(
			"runner key %s is group/world accessible (mode %04o); a key the agent can read cannot attest against it",
			path, mode,
		)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read runner key: %w", err)
	}
	seed, err := hex.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		return nil, fmt.Errorf("runner key is not hex-encoded seed: %w", err)
	}
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("runner key must be a %d-byte ed25519 seed, got %d", ed25519.SeedSize, len(seed))
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

// LoadTrustedRunnerKeys reads the pinned public keys an envelope's attestations
// are verified against. This is governance configuration, not envelope content:
// a claim cannot nominate the authority that validates it.
func LoadTrustedRunnerKeys(path string) ([]ed25519.PublicKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read trusted runner keys: %w", err)
	}
	var keys []ed25519.PublicKey
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		b, err := hex.DecodeString(line)
		if err != nil {
			return nil, fmt.Errorf("trusted runner key is not hex: %w", err)
		}
		if len(b) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("trusted runner key must be %d bytes, got %d", ed25519.PublicKeySize, len(b))
		}
		keys = append(keys, ed25519.PublicKey(b))
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("no trusted runner keys in %s", path)
	}
	return keys, nil
}

// bindsRecord checks a receipt describes the occurrence the record claims.
//
// The signature establishes that a trusted runner said something. This
// establishes that what it said is what this record is using it for — otherwise
// a genuine receipt for one obligation could be pasted onto another.
func (r ProofOccurrenceReceipt) bindsRecord(
	e ChangeEnvelope, kind, name, ref, invocationID, result, evidenceDigest string,
) error {
	for _, f := range []struct{ what, got, want string }{
		{"change id", r.ChangeID, e.ID},
		{"candidate repository", r.CandidateRepository, e.CandidateRepository},
		{"candidate revision", r.CandidateRevision, e.CandidateRevision},
		{"plan digest", r.PlanDigest, e.PlanDigest},
		{"obligation kind", r.ObligationKind, kind},
		{"obligation name", r.ObligationName, name},
		{"obligation ref", r.ObligationRef, ref},
		{"invocation id", r.InvocationID, invocationID},
		{"result", r.Result, result},
		{"evidence digest", r.EvidenceDigest, evidenceDigest},
	} {
		if strings.TrimSpace(f.got) == "" {
			return fmt.Errorf("attestation for %s %q names no %s", kind, name, f.what)
		}
		if f.got != f.want {
			return fmt.Errorf(
				"attestation for %s %q reports %s %q, but the record claims %q",
				kind, name, f.what, f.got, f.want,
			)
		}
	}
	return nil
}

// VerifyAttestations checks every certifying record carries a receipt signed by
// a trusted runner and bound to that exact occurrence.
//
// Signature verification needs no evidence files, so this stays portable: a
// reviewer far from the original run can still establish who attested what,
// against which candidate, under which frozen plan.
func (e ChangeEnvelope) VerifyAttestations(trusted []ed25519.PublicKey) error {
	for _, requirement := range e.RequiredTests {
		if !requirement.Required {
			continue
		}
		for _, record := range e.Tests {
			if record.Name != requirement.Name || record.Result != "PASS" {
				continue
			}
			if record.Receipt == nil {
				return fmt.Errorf("required test %q has no runner attestation", requirement.Name)
			}
			if err := record.Receipt.bindsRecord(e, "test", record.Name,
				strings.Join(requirement.Command, " "), record.InvocationID,
				record.Result, record.Digest); err != nil {
				return err
			}
			if err := record.Receipt.VerifySignature(trusted); err != nil {
				return fmt.Errorf("required test %q: %w", requirement.Name, err)
			}
		}
	}
	for _, requirement := range e.RequiredScenarios {
		if !requirement.Required {
			continue
		}
		for _, proof := range e.Proofs {
			if proof.Scenario != requirement.Name || proof.Result != "PASS" || !proof.ProofEligible {
				continue
			}
			if proof.Receipt == nil {
				return fmt.Errorf("required scenario %q has no runner attestation", requirement.Name)
			}
			if err := proof.Receipt.bindsRecord(e, "scenario", proof.Scenario,
				requirement.Path, proof.InvocationID, proof.Result, proof.Digest); err != nil {
				return err
			}
			if err := proof.Receipt.VerifySignature(trusted); err != nil {
				return fmt.Errorf("required scenario %q: %w", requirement.Name, err)
			}
		}
	}
	return nil
}

// AttestationUnproven lists obligations whose certifying record carries no
// runner attestation. Portable: it reports the absence of a receipt, which is
// checkable without any key.
func (e ChangeEnvelope) AttestationUnproven() []string {
	var unproven []string
	for _, requirement := range e.RequiredTests {
		if !requirement.Required {
			continue
		}
		for _, record := range e.Tests {
			if record.Name == requirement.Name && record.Result == "PASS" && record.Receipt == nil {
				unproven = append(unproven, "test:"+requirement.Name)
			}
		}
	}
	for _, requirement := range e.RequiredScenarios {
		if !requirement.Required {
			continue
		}
		for _, proof := range e.Proofs {
			if proof.Scenario == requirement.Name && proof.Result == "PASS" &&
				proof.ProofEligible && proof.Receipt == nil {
				unproven = append(unproven, "scenario:"+requirement.Name)
			}
		}
	}
	return unproven
}
