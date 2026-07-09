package api

// discovery.go — request/response types for the governance-legibility discovery
// (P4) and amend (P6) operations. See docs/design/governance-tools-legibility.md.

// ListAuthoritiesRequest lists the authority catalog for a domain.
type ListAuthoritiesRequest struct {
	Project string
	Domain  DomainRef
	Limit   int32 // 0 = server default (unbounded)
}

// ListAuthoritiesResponse carries the authority catalog rows.
type ListAuthoritiesResponse struct {
	Authorities []Authority
}

// ListConditionsRequest lists the condition catalog for a domain.
type ListConditionsRequest struct {
	Project string
	Domain  DomainRef
	Limit   int32
}

// ListConditionsResponse carries the condition catalog rows.
type ListConditionsResponse struct {
	Conditions []Condition
}

// ResolveRefRequest resolves a single canonical ref within a domain.
type ResolveRefRequest struct {
	Project string
	Domain  DomainRef
	Ref     string
}

// ResolveRefResponse reports whether the ref resolved and to what kind.
type ResolveRefResponse struct {
	Resolved  bool
	Kind      string // "authority" | "condition" | ""
	Authority *Authority
	Condition *Condition
}

// AmendProposalRequest edits a PROPOSED principle in place. Empty scalar fields
// leave the corresponding value unchanged; ref add/remove lists are set-merged.
type AmendProposalRequest struct {
	Project             string
	Domain              DomainRef
	ID                  string
	Actor               string
	AddAuthorityRefs    []string
	RemoveAuthorityRefs []string
	AddConditionRefs    []string
	RemoveConditionRefs []string
	AddEvidenceRefs     []string
	RemoveEvidenceRefs  []string
	RiskLevel           string
	RevocationRule      string
	PromotionReason     string
}

// AmendProposalResponse reports the amended principle's new state.
type AmendProposalResponse struct {
	PrincipleID        string
	Status             GovernanceStatus
	Version            int32
	ContradictionReset bool // a prior contradiction check was invalidated by the edit
}
