package evolution

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

func NewChangeID() (string, error) {
	var entropy [6]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", fmt.Errorf("generate change id: %w", err)
	}
	return fmt.Sprintf(
		"chg-%s-%s",
		time.Now().UTC().Format("20060102T150405Z"),
		hex.EncodeToString(entropy[:]),
	), nil
}

// BindCandidate establishes the exact implementation revision under proof. It
// may be used only before Sensei admission. Rebinding a different candidate
// clears all candidate-derived evidence so no proof can leak across revisions.
func (e *ChangeEnvelope) BindCandidate(repository, revision string) error {
	repository = strings.TrimSpace(repository)
	revision = strings.TrimSpace(revision)
	if repository == "" || revision == "" {
		return fmt.Errorf("candidate repository and revision are required")
	}
	if e.Stage != StageDraft && e.Stage != StageCandidate && e.Stage != StageProven {
		return fmt.Errorf(
			"cannot rebind candidate after stage %s; create a new ChangeEnvelope",
			e.Stage,
		)
	}
	changed := e.CandidateRepository != repository || e.CandidateRevision != revision
	e.CandidateRepository = repository
	e.CandidateRevision = revision
	e.Stage = StageCandidate
	if changed {
		e.Tests = nil
		e.Proofs = nil
		e.Admission = AdmissionRecord{}
		e.Release = ReleaseRecord{}
		e.ProductionVerification = nil
		e.Learning = nil
		e.BlockedReason = ""
	}
	return e.Validate()
}

type ProofStatus struct {
	Stage                 ChangeStage `json:"stage"`
	CandidateRepository   string      `json:"candidate_repository,omitempty"`
	CandidateRevision     string      `json:"candidate_revision,omitempty"`
	MissingRequiredTests  []string    `json:"missing_required_tests,omitempty"`
	FailedRequiredTests   []string    `json:"failed_required_tests,omitempty"`
	MissingScenarios      []string    `json:"missing_required_scenarios,omitempty"`
	FailedScenarios       []string    `json:"failed_required_scenarios,omitempty"`
	ProofComplete         bool        `json:"proof_complete"`
	NextAuthorityBoundary string      `json:"next_authority_boundary"`
}

func (e ChangeEnvelope) ProofStatus() ProofStatus {
	status := ProofStatus{
		Stage: e.Stage,
		CandidateRepository: e.CandidateRepository,
		CandidateRevision: e.CandidateRevision,
		NextAuthorityBoundary: "local_and_simulation_proof",
	}
	for _, requirement := range e.RequiredTests {
		if !requirement.Required {
			continue
		}
		found := false
		passed := false
		for _, record := range e.Tests {
			if record.Name != requirement.Name || record.CandidateRevision != e.CandidateRevision {
				continue
			}
			found = true
			passed = record.Result == "PASS" && stringSlicesEqual(record.Command, requirement.Command)
			break
		}
		if !found {
			status.MissingRequiredTests = append(status.MissingRequiredTests, requirement.Name)
		} else if !passed {
			status.FailedRequiredTests = append(status.FailedRequiredTests, requirement.Name)
		}
	}
	for _, requirement := range e.RequiredScenarios {
		if !requirement.Required {
			continue
		}
		found := false
		passed := false
		for _, proof := range e.Proofs {
			if proof.Scenario != requirement.Name || proof.CandidateRevision != e.CandidateRevision {
				continue
			}
			found = true
			passed = proof.Result == "PASS" && proof.ProofEligible
			break
		}
		if !found {
			status.MissingScenarios = append(status.MissingScenarios, requirement.Name)
		} else if !passed {
			status.FailedScenarios = append(status.FailedScenarios, requirement.Name)
		}
	}
	candidate := e
	candidate.Stage = StageProven
	status.ProofComplete = candidate.Validate() == nil
	if status.ProofComplete {
		status.NextAuthorityBoundary = "sensei_admission"
	}
	if stageAtLeast(e.Stage, StageAdmitted) {
		status.NextAuthorityBoundary = "immutable_release"
	}
	if stageAtLeast(e.Stage, StageReleased) {
		status.NextAuthorityBoundary = "production_verification"
	}
	if stageAtLeast(e.Stage, StageVerified) {
		status.NextAuthorityBoundary = "behavioral_learning"
	}
	return status
}
