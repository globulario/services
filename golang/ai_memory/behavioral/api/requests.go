package api

// Request types for the Core boundary. Each mirrors the corresponding gRPC
// request message but in Go-native form, so the kernel never depends on the
// wire format.

// RecordSignalRequest captures raw operational input.
type RecordSignalRequest struct {
	Signal Signal
}

// ExtractClaimRequest turns one signal into structured claim(s).
type ExtractClaimRequest struct {
	SignalID string
	Project  string
	Domain   DomainRef
	Claims   []Claim
}

// RecordEvidenceRequest attaches evidence to a claim or principle.
type RecordEvidenceRequest struct {
	Evidence Evidence
}

// MapAuthorityRequest maps authority refs onto a claim or principle.
type MapAuthorityRequest struct {
	TargetKind  string // claim|principle
	TargetID    string
	Project     string
	Domain      DomainRef
	Authorities []AuthorityRef
}

// RecordContradictionRequest records a conflict.
type RecordContradictionRequest struct {
	Contradiction Contradiction
}

// ProposePrincipleRequest creates a candidate principle.
type ProposePrincipleRequest struct {
	Principle Principle
}

// PromotePrincipleRequest runs the promotion gate for a candidate.
type PromotePrincipleRequest struct {
	PrincipleID    string
	Project        string
	Domain         DomainRef
	Approver       string // deprecated alias of ApprovedBy
	ApprovedBy     string // explicit human approval (high/irreversible promotions)
	ApprovalReason string
	Actor          string // who attempted the promotion
}

// RevokePrincipleRequest revokes, narrows, or supersedes a promoted principle.
type RevokePrincipleRequest struct {
	PrincipleID   string
	Project       string
	Domain        DomainRef
	Action        string // REVOKED|SUPERSEDED|NARROWED
	SupersededBy  string // required when Action = SUPERSEDED
	Reason        string
	NarrowedScope string // required when Action = NARROWED
	Actor         string
}

// ExplainPrincipleRequest asks for the full provenance of a principle.
type ExplainPrincipleRequest struct {
	PrincipleID string
	Project     string
	Domain      DomainRef
}

// ResolveGovernedContextRequest is the operator-brain retrieval input.
type ResolveGovernedContextRequest struct {
	Project    string
	Domain     DomainRef
	Goal       string
	Conditions []ConditionRef
	EntityRef  string
	Scope      string
	Limit      int32
}

// CheckActionRequest is the pre-action gate input.
type CheckActionRequest struct {
	Project           string
	Domain            DomainRef
	ActionType        string
	Target            string
	CurrentConditions []ConditionRef
	Scope             string
	AgentID           string
	// ProvidedEvidenceRefs are required-evidence refs (or evidence ids) the agent
	// declares it already holds. Evaluated alongside already-recorded evidence;
	// CheckAction never runs a probe.
	ProvidedEvidenceRefs []string
	HumanApproval        string // approver, satisfies needs_human_approval
}

// RecordOutcomeRequest records what happened after an action.
type RecordOutcomeRequest struct {
	Outcome Outcome
}

// GeneratePromotionCandidateRequest asks the kernel to turn a repeated outcome
// theme into a review-queue item. The repeated-pattern check is automatic; the
// governance fields remain explicit human/agent input.
type GeneratePromotionCandidateRequest struct {
	Project               string
	Domain                DomainRef
	Theme                 string
	MinRepeats            int32
	DraftPrinciple        Principle
	Actor                 string
	Rationale             string
	SupportingEvidenceIDs []string
}

// ListPromotionCandidatesRequest lists review-queue items, optionally scoped by
// theme and/or queue status.
type ListPromotionCandidatesRequest struct {
	Project string
	Domain  DomainRef
	Theme   string
	Status  PromotionCandidateStatus
	Limit   int32
}

// GenerateReconciliationReportRequest creates an advisory bridge report between
// behavioral-memory and AWG.
type GenerateReconciliationReportRequest struct {
	Project              string
	Domain               DomainRef
	PromotionCandidateID string
	Theme                string
	AWGInvariantIDs      []string
	AWGFailureModeIDs    []string
	AWGTestIDs           []string
	RuntimeRelevant      bool
	Actor                string
}

// ListReconciliationReportsRequest lists stored reconciliation reports.
type ListReconciliationReportsRequest struct {
	Project              string
	Domain               DomainRef
	Theme                string
	PromotionCandidateID string
	Limit                int32
}
