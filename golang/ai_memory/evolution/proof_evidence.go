package evolution

import (
	"fmt"
	"strings"
)

// ValidateEvidenceIdentity requires every required proof used by a PROVEN
// envelope to name immutable evidence and carry a content digest. It validates
// identity metadata, not local file existence, so envelopes remain portable to
// independent admission/review environments.
func (e ChangeEnvelope) ValidateEvidenceIdentity() error {
	if !stageAtLeast(e.Stage, StageProven) {
		return nil
	}

	for _, requirement := range e.RequiredTests {
		if !requirement.Required {
			continue
		}
		matched := false
		for _, record := range e.Tests {
			if record.Name != requirement.Name ||
				record.CandidateRevision != e.CandidateRevision ||
				record.PlanDigest != e.PlanDigest ||
				record.Result != "PASS" ||
				!stringSlicesEqual(record.Command, requirement.Command) {
				continue
			}
			matched = true
			if strings.TrimSpace(record.EvidenceRef) == "" {
				return fmt.Errorf("required test %q has no evidence_ref", requirement.Name)
			}
			if strings.TrimSpace(record.Digest) == "" {
				return fmt.Errorf("required test %q has no evidence digest", requirement.Name)
			}
			break
		}
		if !matched {
			return fmt.Errorf("required test %q has no content-addressed PASS evidence", requirement.Name)
		}
	}

	for _, requirement := range e.RequiredScenarios {
		if !requirement.Required {
			continue
		}
		matched := false
		for _, proof := range e.Proofs {
			if proof.Scenario != requirement.Name ||
				proof.CandidateRevision != e.CandidateRevision ||
				proof.PlanDigest != e.PlanDigest ||
				proof.Result != "PASS" ||
				!proof.ProofEligible {
				continue
			}
			matched = true
			if strings.TrimSpace(proof.ProofRef) == "" {
				return fmt.Errorf("required scenario %q has no proof_ref", requirement.Name)
			}
			if strings.TrimSpace(proof.EvidenceRef) == "" {
				return fmt.Errorf("required scenario %q has no evidence_ref", requirement.Name)
			}
			if strings.TrimSpace(proof.Digest) == "" {
				return fmt.Errorf("required scenario %q has no proof/evidence digest", requirement.Name)
			}
			break
		}
		if !matched {
			return fmt.Errorf("required scenario %q has no content-addressed PASS proof", requirement.Name)
		}
	}
	return nil
}

// evidenceArtifacts is the ordered, unambiguous artifact list a record's digest
// is taken over. Both accessors must stay in lockstep with the runners that
// produce the digest, so verification recomputes exactly what was recorded.
func (r TestRecord) evidenceArtifacts() []string {
	return nonEmpty(r.EvidenceRef)
}

func (p ProofRecord) evidenceArtifacts() []string {
	return nonEmpty(p.ProofRef, p.EvidenceRef)
}

func nonEmpty(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	return out
}

// VerifyEvidenceArtifacts re-derives every recorded digest from the artifacts on
// disk. ValidateEvidenceIdentity is deliberately portable and checks identity
// metadata only; this is the local counterpart, used where the artifacts are
// actually reachable. Deleting or editing a proof artifact after the fact makes
// the recorded digest unreproducible, and an unreproducible digest is not proof.
func (e ChangeEnvelope) VerifyEvidenceArtifacts() error {
	if !stageAtLeast(e.Stage, StageProven) {
		return nil
	}
	for _, record := range e.Tests {
		if record.Result != "PASS" || !isRequiredTest(e.RequiredTests, record.Name) {
			continue
		}
		if err := verifyRecordedDigest(
			fmt.Sprintf("required test %q", record.Name),
			record.Digest,
			record.evidenceArtifacts(),
		); err != nil {
			return err
		}
	}
	for _, proof := range e.Proofs {
		if proof.Result != "PASS" || !proof.ProofEligible || !isRequiredScenario(e.RequiredScenarios, proof.Scenario) {
			continue
		}
		if err := verifyRecordedDigest(
			fmt.Sprintf("required scenario %q", proof.Scenario),
			proof.Digest,
			proof.evidenceArtifacts(),
		); err != nil {
			return err
		}
	}
	return nil
}

func verifyRecordedDigest(subject, recorded string, artifacts []string) error {
	if len(artifacts) == 0 {
		return fmt.Errorf("%s names no evidence artifact", subject)
	}
	actual, err := DigestFiles(artifacts...)
	if err != nil {
		return fmt.Errorf("%s evidence is no longer readable: %w", subject, err)
	}
	if actual != recorded {
		return fmt.Errorf(
			"%s evidence digest %s does not match recorded %s; the proof artifact changed after it was recorded",
			subject,
			actual,
			recorded,
		)
	}
	return nil
}

func isRequiredTest(requirements []TestRequirement, name string) bool {
	for _, requirement := range requirements {
		if requirement.Name == name {
			return requirement.Required
		}
	}
	return false
}

func isRequiredScenario(requirements []ScenarioRequirement, name string) bool {
	for _, requirement := range requirements {
		if requirement.Name == name {
			return requirement.Required
		}
	}
	return false
}
