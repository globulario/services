package api

// This file holds the kernel's domain types. They are Go-native (not generated
// protobuf) so the kernel boundary is independent of any wire format. The gRPC
// layer translates between these and behavioral_memorypb messages.
//
// Domain-isolation rule: no field here may name a cluster-specific system or
// concept. Cluster specifics are carried as opaque refs (Domain, AppliesWhen,
// Authorities, ForbiddenMoves, …). `Metadata` is an extension hatch only — it
// must never become the real schema for authority, evidence, conditions,
// contradictions, promotion status, or forbidden moves.

// Provenance is the common record-origin stamp carried by governed records.
type Provenance struct {
	AgentID   string
	SourceRef string
	MemoryID  string // link to ai_memory.memories, when derived from a memory
	CreatedAt int64
	UpdatedAt int64
}

// Signal is raw operational input.
type Signal struct {
	ID             string
	Project        string
	Domain         DomainRef
	Kind           SignalKind
	SourceKind     string // log|metric|probe|agent|human|test|status
	SourceRef      string
	EntityRef      string
	Scope          string // generic scope id (e.g. cluster id)
	ClusterID      string
	ConditionRef   string
	Severity       string
	AuthorityLevel ObservationAuthorityLevel
	ObservedAt     int64
	Payload        string
	Confidence     float32
	Status         GovernanceStatus
	Provenance     Provenance
	Metadata       map[string]string
}

// Claim is a structured statement derived from a signal.
type Claim struct {
	ID            string
	Project       string
	Domain        DomainRef
	SignalID      string
	Statement     string
	SubjectEntity string
	Predicate     string
	ObjectValue   string
	TimeRef       int64
	Status        GovernanceStatus
	Confidence    float32
	SourceID      string
	Provenance    Provenance
	Metadata      map[string]string
}

// Evidence supports a claim or a principle.
type Evidence struct {
	ID             string
	Project        string
	Domain         DomainRef
	TargetKind     string // signal|claim|principle
	TargetID       string // → bm:supportedBy (inverse): the signal/claim/principle this supports
	Kind           string // test_result|probe|metric|log|snapshot|human
	Lane           EvidenceLane
	Result         string // pass|fail|stale|unknown
	ProbeRef       string
	SourceKind     string
	SourceRef      string
	EntityRef      string
	ClusterID      string
	ConditionRef   string
	Severity       string
	AuthorityLevel ObservationAuthorityLevel
	ObservedAt     int64
	Payload        string
	Provenance     Provenance
	// RDF-readiness relations (first-class, never in Metadata):
	ObservedFrom string                // → bm:observedFrom: signal/source id this evidence came from
	Satisfies    []RequiredEvidenceRef // → bm:satisfies: required-evidence slots fulfilled
	Metadata     map[string]string
}

// Authority identifies the owner of a runtime truth (catalog entry).
type Authority struct {
	ID             string
	Project        string
	Domain         DomainRef
	Title          string
	Governs        string
	OwnerKind      string // datastore|service_rpc|human|proxy|dns
	ReadPath       string
	WritePath      string
	IdentitySource string
	Status         GovernanceStatus
	// RDF-readiness relation: → bm:governs. Targets governed by this authority
	// (domain/condition/principle refs).
	GovernsRefs []string
	Metadata    map[string]string
}

// Condition is a runtime condition that scopes a rule (catalog entry).
type Condition struct {
	ID         string
	Project    string
	Domain     DomainRef
	Title      string
	DetectSpec string // how it is evaluated (probe ref + predicate)
	Severity   string
	Status     GovernanceStatus
	Metadata   map[string]string
}

// Contradiction records a conflict that must be resolved before promotion.
type Contradiction struct {
	ID         string
	Project    string
	Domain     DomainRef
	Kind       string // claim_vs_claim|evidence_stale|authority_narrower|rule_conflict
	LeftRef    string
	RightRef   string
	Resolution string // open|resolved|superseded
	Note       string
	CreatedAt  int64
	Metadata   map[string]string
}

// Principle is a governed behavioral rule. The "Good" shape from the design:
// no cluster-typed fields, only generic refs.
type Principle struct {
	ID                string
	Project           string
	Domain            DomainRef
	Title             string
	AppliesWhen       []ConditionRef
	Authorities       []AuthorityRef
	RequiredEvidence  []RequiredEvidenceRef
	ForbiddenMoves    []ForbiddenMoveRef
	RecommendedAction string
	RiskLevel         string // info|low|high|irreversible
	RevocationRule    string // free-text WHEN-to-revoke (see RevocationRuleID for the node ref)
	PromotionReason   string
	Status            GovernanceStatus
	SupersededBy      string // → bm:supersededBy: principle id that supersedes this one
	Version           int32
	ProposedBy        string // actor (not a node ref)
	PromotedBy        string // actor who promoted (not a node ref)
	Provenance        Provenance
	// RDF-readiness relations (first-class, never in Metadata):
	PromotionDecisionID string // → bm:promotedBy: PromotionDecision node id
	RevocationRuleID    string // → bm:revokedBy: RevocationRule node id
	NarrowedBy          string // → bm:narrowedBy: principle id that narrowed this one
	// Promotion-gate inputs (first-class, never in Metadata):
	ContradictionChecked bool   // a contradiction check was performed before promotion
	ApprovedBy           string // human approver for high-risk/irreversible promotion
	ApprovalReason       string
	ApprovedAt           int64
	// Seed/compiler lineage (first-class, never in Metadata). Populated by the
	// Operational Knowledge Compiler (PR-5A); the seam exists now so PR-5A is
	// additive (no principles-table migration).
	SourceRefs    []string // → bm:sourceRef: provenance source links
	GeneratedFrom []string // → bm:generatedFrom: OperationalKnowledgeEntry ids
	Metadata      map[string]string
}

// ForbiddenMove is an action shape ruled out under a condition (catalog entry).
type ForbiddenMove struct {
	ID                string
	Project           string
	Domain            DomainRef
	Title             string
	Summary           string
	Reason            string
	ActionType        string
	TargetPattern     string
	RelatedPrinciples []string
	Status            GovernanceStatus
	Metadata          map[string]string
}

// RequiredEvidence is an evidence slot a principle requires before it applies.
type RequiredEvidence struct {
	ID        string
	Project   string
	Domain    DomainRef
	Title     string
	Lane      EvidenceLane
	ProbeRef  string
	Predicate string
	AppliesTo []string // principle ids
	Metadata  map[string]string
}

// Outcome records what happened after an action.
type Outcome struct {
	ID            string
	Project       string
	Domain        DomainRef
	ActionCheckID string   // → bm:resultedFrom: the ActionCheck this outcome followed
	PrincipleIDs  []string // neutral association (all principles referenced)
	EvidenceIDs   []string
	Status        string // success|failure|blocked|reverted
	Severe        bool
	HumanMarked   bool
	IncidentID    string
	Theme         string // grouping key for promotion proposals
	Note          string
	AgentID       string
	CreatedAt     int64
	// RDF-readiness relations (first-class, never in Metadata): directional
	// outcome→principle edges, kept distinct from the neutral PrincipleIDs.
	SupportsPrinciples []string // → bm:supportsPrinciple
	WeakensPrinciples  []string // → bm:weakensPrinciple
	Metadata           map[string]string
}

// PromotionCandidate is a human-review queue item derived from repeated
// runtime outcomes. It captures an explicit principle draft plus the supporting
// repeated outcomes/evidence that made it review-worthy. It is NOT a principle
// row and never implies auto-promotion.
type PromotionCandidate struct {
	ID                      string
	Project                 string
	Domain                  DomainRef
	Theme                   string
	Status                  PromotionCandidateStatus
	Title                   string
	Summary                 string
	Rationale               string
	SupportingOutcomeIDs    []string
	SupportingEvidenceIDs   []string
	RepeatCount             int32
	DraftPrinciple          Principle
	GeneratedBy             string
	CreatedAt               int64
	UpdatedAt               int64
	MaterializedPrincipleID string
	Metadata                map[string]string
}

// ReconciliationReport is an advisory bridge artifact between behavioral-memory
// and AWG. It records detected drift/reinforcement plus proposed review links;
// it never mutates either governance surface automatically.
type ReconciliationReport struct {
	ID                        string
	Project                   string
	Domain                    DomainRef
	PromotionCandidateID      string
	Theme                     string
	AWGInvariantIDs           []string
	AWGFailureModeIDs         []string
	AWGTestIDs                []string
	Findings                  []string
	Summary                   string
	OutcomeCount              int32
	FailureCount              int32
	SuccessCount              int32
	SevereCount               int32
	ProposedAWGInvariantIDs   []string
	ProposedAWGFailureModeIDs []string
	ProposedAWGTestIDs        []string
	ProposedBehavioralTheme   string
	Actor                     string
	CreatedAt                 int64
	Metadata                  map[string]string
}

// PromotionDecisionRecord is the audit record of a promotion gate evaluation.
type PromotionDecisionRecord struct {
	ID                 string
	Project            string
	Domain             DomainRef
	PrincipleID        string
	Decision           PromotionDecision
	Verdict            string // certification-verdict detail
	MissingEvidence    []string
	BlockedByForbidden []string
	Reviewer           string
	Reason             string
	CreatedAt          int64
	// PR-3 gate detail (first-class). Blocked promotion is also memory.
	UnresolvedAuthority    []string
	UnresolvedConditions   []string
	BlockingContradictions []string
	RiskLevel              string
	ReviewRequired         bool
	ApprovedBy             string
	PromotionReason        string
	Actor                  string
	Metadata               map[string]string
}

// RevocationRule expresses when/how a principle should be revoked or narrowed.
type RevocationRule struct {
	ID          string
	Project     string
	Domain      DomainRef
	PrincipleID string
	Condition   string
	Action      string // REVOKED|SUPERSEDED|NARROWED
	Note        string
	CreatedAt   int64
	// PR-3 first-class revocation detail (never in Metadata):
	RevocationReason string
	Actor            string
	SupersededBy     string // required when Action = SUPERSEDED
	NarrowedScope    string // required when Action = NARROWED
	Metadata         map[string]string
}

// GovernedContext is the operator-brain retrieval bundle.
type GovernedContext struct {
	RelevantMemoryIDs    []string
	Signals              []Signal
	Claims               []Claim
	ApplicablePrinciples []Principle
	MatchedConditions    []Condition
	RequiredEvidence     []RequiredEvidence
	ForbiddenMoves       []ForbiddenMove
	UnresolvedAuthority  []Authority
	KnownContradictions  []Contradiction
	PriorOutcomes        []Outcome
	RecommendedBehavior  string
	Confidence           string // certainty classification
}

// ActionCheck is the result of evaluating a proposed action against governed
// memory. This is the most important runtime feature: the pre-action gate.
type ActionCheck struct {
	ID                  string
	Project             string
	Domain              DomainRef
	ActionType          string
	Target              string
	Conditions          []ConditionRef
	Allowed             bool
	Status              string                // allowed|blocked|needs_evidence|needs_authority|needs_human_approval
	ViolatedPrinciples  []string              // subset of CheckedAgainstPrinciples that this action violates
	MissingEvidence     []RequiredEvidenceRef // → bm:missingEvidence
	UnresolvedAuthority []AuthorityRef
	ForbiddenMatched    []ForbiddenMoveRef // → bm:blockedBy
	RecommendedSteps    []string
	Explanation         string
	AgentID             string
	CreatedAt           int64
	// RDF-readiness relation: → bm:checkedAgainst. Every principle considered
	// (superset of ViolatedPrinciples).
	CheckedAgainstPrinciples []string
	// Governed is true iff at least one applicable promoted principle was
	// evaluated. It separates a governed allow ("principles satisfied") from a
	// default ungoverned allow ("no applicable principle") — without it the two
	// are indistinguishable and the gate's reach cannot be measured.
	Governed bool
	Metadata map[string]string
}

// GovernanceCoverage measures how much of the action surface is actually under
// governance: how many CheckActions had an applicable promoted principle
// (Governed) vs were default-allowed for lack of one (Ungoverned).
type GovernanceCoverage struct {
	Project    string
	Domain     string
	Total      int64
	Governed   int64
	Ungoverned int64
	Ratio      float64 // Governed / Total (0 when Total == 0)
}
