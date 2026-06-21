package api

import (
	"context"
	"errors"
)

// ErrUnimplemented is returned by kernel operations that are not yet built. The
// gRPC layer maps it to codes.Unimplemented so callers get a deterministic,
// non-panicking response. PR-1 ships every Core method returning this.
var ErrUnimplemented = errors.New("behavioral-memory: operation not implemented")

// Core is the behavioral-memory kernel boundary.
//
// Every method is request/response shaped so the kernel can be promoted to a
// standalone gRPC service without changing the domain model. Implementations are
// domain-agnostic: domain specifics arrive via opaque refs (DomainRef,
// ConditionRef, …) resolved through the domain registry, never via cluster-typed
// fields.
//
// The method set is the governance ladder plus the runtime decision-support hot
// path:
//
//	Ingestion : RecordSignal → ExtractClaim → RecordEvidence → MapAuthority → RecordContradiction
//	Governance: ProposePrinciple → PromotePrinciple → RevokePrinciple → ExplainPrinciple
//	Runtime   : ResolveGovernedContext, CheckAction, RecordOutcome
type Core interface {
	// Ingestion / the governance ladder.
	RecordSignal(ctx context.Context, req *RecordSignalRequest) (*RecordSignalResponse, error)
	ExtractClaim(ctx context.Context, req *ExtractClaimRequest) (*ExtractClaimResponse, error)
	RecordEvidence(ctx context.Context, req *RecordEvidenceRequest) (*RecordEvidenceResponse, error)
	MapAuthority(ctx context.Context, req *MapAuthorityRequest) (*MapAuthorityResponse, error)
	RecordContradiction(ctx context.Context, req *RecordContradictionRequest) (*RecordContradictionResponse, error)

	// Governance.
	ProposePrinciple(ctx context.Context, req *ProposePrincipleRequest) (*ProposePrincipleResponse, error)
	PromotePrinciple(ctx context.Context, req *PromotePrincipleRequest) (*PromotePrincipleResponse, error)
	RevokePrinciple(ctx context.Context, req *RevokePrincipleRequest) (*RevokePrincipleResponse, error)
	ExplainPrinciple(ctx context.Context, req *ExplainPrincipleRequest) (*ExplainPrincipleResponse, error)

	// Runtime decision support (the agent hot path).
	ResolveGovernedContext(ctx context.Context, req *ResolveGovernedContextRequest) (*ResolveGovernedContextResponse, error)
	CheckAction(ctx context.Context, req *CheckActionRequest) (*CheckActionResponse, error)
	RecordOutcome(ctx context.Context, req *RecordOutcomeRequest) (*RecordOutcomeResponse, error)
	GeneratePromotionCandidate(ctx context.Context, req *GeneratePromotionCandidateRequest) (*GeneratePromotionCandidateResponse, error)
	ListPromotionCandidates(ctx context.Context, req *ListPromotionCandidatesRequest) (*ListPromotionCandidatesResponse, error)
	GenerateReconciliationReport(ctx context.Context, req *GenerateReconciliationReportRequest) (*GenerateReconciliationReportResponse, error)
	ListReconciliationReports(ctx context.Context, req *ListReconciliationReportsRequest) (*ListReconciliationReportsResponse, error)
}
