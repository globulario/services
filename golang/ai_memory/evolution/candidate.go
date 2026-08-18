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

// BindCandidate establishes the exact implementation revision and freezes the
// complete proof plan under PlanDigest. Rebinding a different candidate clears
// all candidate-derived evidence so no proof leaks across revisions. Changing
// contracts/tests/scenarios after binding makes the envelope invalid; re-plan
// explicitly before proof instead of silently shrinking obligations.
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

	candidateChanged := e.CandidateRepository != repository || e.CandidateRevision != revision
	e.CandidateRepository = repository
	e.CandidateRevision = revision
	e.Stage = StageCandidate

	planDigest, err := e.IdentityDigest()
	if err != nil {
		return fmt.Errorf("calculate candidate plan digest: %w", err)
	}
	planChanged := e.PlanDigest != "" && e.PlanDigest != planDigest
	if candidateChanged || planChanged {
		e.Tests = nil
		e.Proofs = nil
		e.Admission = AdmissionRecord{}
		e.Release = ReleaseRecord{}
		e.ProductionVerification = nil
		e.Learning = nil
		e.BlockedReason = ""
	}
	e.PlanDigest = planDigest
	return e.Validate()
}

type ProofStatus struct {
	Stage                ChangeStage `json:"stage"`
	CandidateRepository  string      `json:"candidate_repository,omitempty"`
	CandidateRevision    string      `json:"candidate_revision,omitempty"`
	PlanDigest           string      `json:"plan_digest,omitempty"`
	MissingRequiredTests []string    `json:"missing_required_tests,omitempty"`
	FailedRequiredTests  []string    `json:"failed_required_tests,omitempty"`
	MissingScenarios     []string    `json:"missing_required_scenarios,omitempty"`
	FailedScenarios      []string    `json:"failed_required_scenarios,omitempty"`
	// SimulatorIdentityUnproven lists required scenarios whose proof could not
	// say which simulator repository produced it. Not a failure — an explicit
	// decision for the admission authority.
	SimulatorIdentityUnproven []string `json:"simulator_identity_unproven,omitempty"`
	// ObligationsComplete reports that every declared obligation has a PASS
	// record. It is deliberately separate from ProofComplete: an envelope may
	// describe complete evidence while none of it is independently attested.
	ObligationsComplete bool `json:"obligations_complete"`
	// AttestationUnproven lists certifying occurrences with no runner receipt.
	AttestationUnproven []string `json:"attestation_unproven,omitempty"`
	// AdmissionUnverified reports that the admission recorded here is a claim
	// referencing a decision taken elsewhere, which this process cannot check.
	AdmissionUnverified   bool   `json:"admission_unverified,omitempty"`
	ProofComplete         bool   `json:"proof_complete"`
	NextAuthorityBoundary string `json:"next_authority_boundary"`
}

func (e ChangeEnvelope) ProofStatus() ProofStatus {
	status := ProofStatus{
		Stage:                 e.Stage,
		CandidateRepository:   e.CandidateRepository,
		CandidateRevision:     e.CandidateRevision,
		PlanDigest:            e.PlanDigest,
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
			passed = record.Result == "PASS" &&
				record.PlanDigest == e.PlanDigest &&
				stringSlicesEqual(record.Command, requirement.Command)
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
			passed = proof.Result == "PASS" && proof.ProofEligible && proof.PlanDigest == e.PlanDigest
			break
		}
		if !found {
			status.MissingScenarios = append(status.MissingScenarios, requirement.Name)
		} else if !passed {
			status.FailedScenarios = append(status.FailedScenarios, requirement.Name)
		}
	}
	status.SimulatorIdentityUnproven = e.SimulatorIdentityUnproven()
	status.AttestationUnproven = e.AttestationUnproven()
	status.ObligationsComplete = len(status.MissingRequiredTests) == 0 &&
		len(status.FailedRequiredTests) == 0 &&
		len(status.MissingScenarios) == 0 &&
		len(status.FailedScenarios) == 0
	status.AdmissionUnverified = e.Admission != (AdmissionRecord{})
	candidate := e
	candidate.Stage = StageProven
	status.ProofComplete = candidate.Validate() == nil
	if status.ProofComplete {
		status.NextAuthorityBoundary = "sensei_admission"
	}
	// The pointer stops here. Everything past sensei_admission is owned by an
	// authority this framework cannot reach, so naming a later boundary would
	// imply this process had established the earlier one.
	return status
}
