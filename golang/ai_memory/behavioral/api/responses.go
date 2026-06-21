package api

// Response types for the Core boundary.

// RecordSignalResponse returns the stored signal id and its ladder status.
type RecordSignalResponse struct {
	SignalID string
	Status   GovernanceStatus
}

// ExtractClaimResponse returns the ids of the claims created.
type ExtractClaimResponse struct {
	ClaimIDs []string
}

// RecordEvidenceResponse returns the stored evidence id.
type RecordEvidenceResponse struct {
	EvidenceID string
}

// MapAuthorityResponse returns the resulting ladder status of the target.
type MapAuthorityResponse struct {
	Status GovernanceStatus
}

// RecordContradictionResponse returns the stored contradiction id.
type RecordContradictionResponse struct {
	ContradictionID string
}

// ProposePrincipleResponse returns the candidate principle id and status.
type ProposePrincipleResponse struct {
	PrincipleID string
	Status      GovernanceStatus
}

// PromotePrincipleResponse returns the gate verdict and resulting status.
type PromotePrincipleResponse struct {
	Decision PromotionDecision
	Status   GovernanceStatus
	Record   PromotionDecisionRecord
}

// RevokePrincipleResponse returns the resulting status of the principle.
type RevokePrincipleResponse struct {
	Status GovernanceStatus
}

// ExplainPrincipleResponse returns a principle with its full governance provenance.
type ExplainPrincipleResponse struct {
	Principle        Principle
	Evidence         []Evidence
	Authorities      []Authority
	Conditions       []Condition
	Contradictions   []Contradiction
	PromotionHistory []PromotionDecisionRecord
	RevocationRules  []RevocationRule
	Explanation      string
}

// ResolveGovernedContextResponse returns the operator-brain bundle.
type ResolveGovernedContextResponse struct {
	Context GovernedContext
}

// CheckActionResponse returns the pre-action gate result.
type CheckActionResponse struct {
	Result ActionCheck
}

// RecordOutcomeResponse returns the stored outcome id.
type RecordOutcomeResponse struct {
	OutcomeID string
}

// GeneratePromotionCandidateResponse returns the queued review candidate and
// the supporting outcome count used to justify it.
type GeneratePromotionCandidateResponse struct {
	Candidate    PromotionCandidate
	OutcomeCount int32
}

// ListPromotionCandidatesResponse returns queued promotion candidates.
type ListPromotionCandidatesResponse struct {
	Candidates []PromotionCandidate
}

// GenerateReconciliationReportResponse returns the stored advisory report.
type GenerateReconciliationReportResponse struct {
	Report ReconciliationReport
}

// ListReconciliationReportsResponse returns stored advisory reports.
type ListReconciliationReportsResponse struct {
	Reports []ReconciliationReport
}
