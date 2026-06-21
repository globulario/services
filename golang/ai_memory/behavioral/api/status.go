// Package api defines the protocol-shaped, transport-agnostic boundary of the
// behavioral-memory kernel.
//
// Every public operation accepts a request object and returns a response object
// (see Core), so the kernel can be promoted to a standalone gRPC service later
// without changing the domain model. This package is the single source of the
// kernel's Go vocabulary; the gRPC layer (behavioral_memorypb) translates to and
// from these types at the edge.
//
// Generic-kernel rule: nothing in this package — or anywhere under behavioral/ —
// may reference cluster-specific systems (etcd, ScyllaDB, MinIO, Envoy, DNS) or
// import Globular cluster packages. Cluster-specific concepts arrive only as
// opaque string refs (DomainRef, ConditionRef, AuthorityRef, …) resolved through
// the domain registry. Cluster logic lives under domains/cluster_operator/ (a
// later PR), never here.
package api

// GovernanceStatus is the promotion ladder a fact climbs from raw signal to
// promoted principle. Only forward transitions are valid, except the terminal
// revocation set (StatusRevoked / StatusSuperseded / StatusNarrowed).
//
// Names/semantics mirror the awareness-graph (AWG) governance model so the two
// systems can share a kernel later.
type GovernanceStatus string

const (
	StatusUnspecified         GovernanceStatus = ""
	StatusRawSignal           GovernanceStatus = "RAW_SIGNAL"
	StatusExtractedClaim      GovernanceStatus = "EXTRACTED_CLAIM"
	StatusCandidateFact       GovernanceStatus = "CANDIDATE_FACT"
	StatusEvidenceLinked      GovernanceStatus = "EVIDENCE_LINKED"
	StatusAuthorityMapped     GovernanceStatus = "AUTHORITY_MAPPED"
	StatusConditionScoped     GovernanceStatus = "CONDITION_SCOPED"
	StatusContradictionTested GovernanceStatus = "CONTRADICTION_TESTED"
	StatusProposedPrinciple   GovernanceStatus = "PROPOSED_PRINCIPLE"
	StatusPromotedPrinciple   GovernanceStatus = "PROMOTED_PRINCIPLE"
	StatusRevoked             GovernanceStatus = "REVOKED"
	StatusSuperseded          GovernanceStatus = "SUPERSEDED"
	StatusNarrowed            GovernanceStatus = "NARROWED"
)

// EvidenceLane is which burden of proof a piece of evidence (or a required-
// evidence slot) satisfies. Mirrors AWG EvidenceLaneMode.
type EvidenceLane string

const (
	LaneUnspecified     EvidenceLane = ""
	LaneStaticOnly      EvidenceLane = "STATIC_ONLY"
	LaneRuntimeRequired EvidenceLane = "RUNTIME_REQUIRED"
	LaneHybrid          EvidenceLane = "HYBRID"
)

// PromotionDecision is the verdict of the promotion gate. Mirrors AWG
// PromotionDecision.
type PromotionDecision string

const (
	PromotionUnspecified    PromotionDecision = ""
	PromotionAllowed        PromotionDecision = "ALLOWED"
	PromotionBlocked        PromotionDecision = "BLOCKED"
	PromotionReviewRequired PromotionDecision = "REVIEW_REQUIRED"
)

// PromotionCandidateStatus is the human-review queue state for outcome-derived
// promotion candidates. These are NOT promoted principles; they are review work.
type PromotionCandidateStatus string

const (
	PromotionCandidateStatusUnspecified  PromotionCandidateStatus = ""
	PromotionCandidateStatusQueued       PromotionCandidateStatus = "QUEUED"
	PromotionCandidateStatusReviewed     PromotionCandidateStatus = "REVIEWED"
	PromotionCandidateStatusDismissed    PromotionCandidateStatus = "DISMISSED"
	PromotionCandidateStatusMaterialized PromotionCandidateStatus = "MATERIALIZED"
)

// SignalKind keeps observed-fact / interpretation / correction / automated-health
// distinct so they are never collapsed into one untyped note.
type SignalKind string

const (
	SignalKindUnspecified     SignalKind = ""
	SignalObservedRuntimeFact SignalKind = "OBSERVED_RUNTIME_FACT"
	SignalAgentInterpretation SignalKind = "AGENT_INTERPRETATION"
	SignalHumanCorrection     SignalKind = "HUMAN_CORRECTION"
	SignalAutomatedHealth     SignalKind = "AUTOMATED_HEALTH"
	SignalHistoricalMemory    SignalKind = "HISTORICAL_MEMORY"
	SignalPromotedPrinciple   SignalKind = "PROMOTED_PRINCIPLE"
)

// ObservationAuthorityLevel keeps source trust explicit for governed
// observation intake. Different inputs may all be useful without carrying equal
// authority.
type ObservationAuthorityLevel string

const (
	ObservationAuthorityUnspecified    ObservationAuthorityLevel = ""
	ObservationAuthorityInterpretation ObservationAuthorityLevel = "INTERPRETATION"
	ObservationAuthorityEventStream    ObservationAuthorityLevel = "EVENT_STREAM"
	ObservationAuthorityDiagnostic     ObservationAuthorityLevel = "DIAGNOSTIC_CLAIM"
	ObservationAuthorityDerived        ObservationAuthorityLevel = "DERIVED_EVIDENCE"
	ObservationAuthorityTruthPlane     ObservationAuthorityLevel = "TRUTH_PLANE"
)

// Opaque reference types. The kernel never interprets their contents — a
// ConditionRef like "condition.cluster.etcd.nospace_alarm" is resolved through
// the domain registry, never parsed here. This is what keeps cluster specifics
// out of the generic kernel.
type (
	// DomainRef identifies a domain pack, e.g. "cluster_operator".
	DomainRef string
	// ConditionRef references a Condition catalog entry by id.
	ConditionRef string
	// AuthorityRef references an Authority catalog entry by id.
	AuthorityRef string
	// ForbiddenMoveRef references a ForbiddenMove catalog entry by id.
	ForbiddenMoveRef string
	// RequiredEvidenceRef references a RequiredEvidence catalog entry by id.
	RequiredEvidenceRef string
)
