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
