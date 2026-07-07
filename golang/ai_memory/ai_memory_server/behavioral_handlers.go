// Behavioral-memory gRPC surface.
//
// This file is the thin transport layer for the BehavioralMemoryService. It
// translates protobuf request/response messages (behavioral_memorypb) to and
// from the kernel's Go-native boundary (behavioral/api) and delegates all logic
// to the in-process kernel (behavioral/core).
//
// PR-2 mapped the five ingestion RPCs, PR-3 the four governance RPCs, PR-4 the
// three runtime RPCs (ResolveGovernedContext, CheckAction, RecordOutcome). All 12
// are now mapped. Keeping this surface separate from the AiMemoryService `server`
// struct preserves the clean extraction path.
package main

import (
	"context"
	"errors"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/core"
	"github.com/globulario/services/golang/ai_memory/behavioral/domain"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
	bpb "github.com/globulario/services/golang/ai_memory/behavioral_memorypb"
	cluster_operator "github.com/globulario/services/golang/ai_memory/domains/cluster_operator"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// behavioralSeedProject is the canonical behavioral-memory project under which
// domain-pack seed (cluster_operator catalogs + proposed principles) is loaded.
const behavioralSeedProject = "globular-services"

// behavioralRegistry builds the domain registry for the behavioral kernel with
// the cluster_operator pack registered. A bad embedded seed is a build-time bug;
// it is logged and the kernel continues with an empty registry rather than
// crashing the whole ai-memory service.
func behavioralRegistry() *domain.Registry {
	reg := domain.NewRegistry()
	if pack, err := cluster_operator.New(); err == nil {
		reg.Register(pack)
	} else {
		logger.Error("cluster_operator pack failed to load — behavioral runtime will have no domain catalogs", "err", err)
	}
	return reg
}

// behavioralHandler implements bpb.BehavioralMemoryServiceServer by delegating to
// the behavioral-memory kernel.
type behavioralHandler struct {
	core api.Core
}

// newBehavioralHandler builds the handler over the given store. PR-2 wires the
// Scylla store (from the shared session) in setupGrpcService; tests pass an
// in-memory store. The domain registry is empty until the cluster_operator pack
// lands (PR-5).
func newBehavioralHandler(st store.Store) *behavioralHandler {
	if st == nil {
		st = store.Unconfigured{}
	}
	return &behavioralHandler{core: core.New(st, behavioralRegistry())}
}

// registerBehavioralService registers the BehavioralMemoryService on the given
// gRPC server, backed by the given store.
func registerBehavioralService(gs *grpc.Server, st store.Store) {
	bpb.RegisterBehavioralMemoryServiceServer(gs, newBehavioralHandler(st))
}

// behavioralErr maps a kernel error to a gRPC status. ErrUnimplemented →
// Unimplemented (dark RPCs); a structured GovernanceError → its taxonomy-mapped
// code carrying the COMPLETE self-describing message (Priority 8/1 legibility);
// everything else → Internal.
func behavioralErr(op string, err error) error {
	if errors.Is(err, api.ErrUnimplemented) {
		return status.Errorf(codes.Unimplemented,
			"BehavioralMemoryService.%s: not implemented yet (lands in a later PR)", op)
	}
	var ge *api.GovernanceError
	if errors.As(err, &ge) {
		return status.Errorf(govCodeToGRPC(ge.Code),
			"BehavioralMemoryService.%s: %s", op, ge.Error())
	}
	return status.Errorf(codes.Internal, "BehavioralMemoryService.%s: %v", op, err)
}

// govCodeToGRPC maps the transport-agnostic governance ErrorCode taxonomy to gRPC
// status codes. This mapping lives in the handler (not the kernel) so the kernel
// stays free of grpc dependencies and cleanly extractable.
func govCodeToGRPC(c api.ErrorCode) codes.Code {
	switch c {
	case api.CodeMissingRequiredFields, api.CodeUnknownField, api.CodeInvalidFieldType,
		api.CodeInvalidEnumValue, api.CodeInvalidReferenceFormat:
		return codes.InvalidArgument
	case api.CodeReferenceNotFound:
		return codes.NotFound
	case api.CodeAuthorityNotMapped, api.CodeEvidenceNotObservable, api.CodeEvidencePostHoc,
		api.CodeEvidenceStale, api.CodeContradictionDetected, api.CodeRequiredTestsMissing,
		api.CodeApproverRequired, api.CodePromotionContractUnsatisfied:
		return codes.FailedPrecondition
	case api.CodeUnsafeOperationRefused:
		return codes.PermissionDenied
	default:
		return codes.Internal
	}
}

// ── Implemented (PR-2): ingestion half ────────────────────────────────────────

func (h *behavioralHandler) RecordSignal(ctx context.Context, req *bpb.RecordSignalRequest) (*bpb.RecordSignalResponse, error) {
	resp, err := h.core.RecordSignal(ctx, &api.RecordSignalRequest{Signal: pbToSignal(req.GetSignal())})
	if err != nil {
		return nil, behavioralErr("RecordSignal", err)
	}
	return &bpb.RecordSignalResponse{SignalId: resp.SignalID, Status: apiGovStatusToPB(resp.Status)}, nil
}

func (h *behavioralHandler) ExtractClaim(ctx context.Context, req *bpb.ExtractClaimRequest) (*bpb.ExtractClaimResponse, error) {
	claims := make([]api.Claim, 0, len(req.GetClaims()))
	for _, c := range req.GetClaims() {
		claims = append(claims, pbToClaim(c))
	}
	resp, err := h.core.ExtractClaim(ctx, &api.ExtractClaimRequest{
		SignalID: req.GetSignalId(), Project: req.GetProject(), Domain: api.DomainRef(req.GetDomain()), Claims: claims,
	})
	if err != nil {
		return nil, behavioralErr("ExtractClaim", err)
	}
	return &bpb.ExtractClaimResponse{ClaimIds: resp.ClaimIDs}, nil
}

func (h *behavioralHandler) RecordEvidence(ctx context.Context, req *bpb.RecordEvidenceRequest) (*bpb.RecordEvidenceResponse, error) {
	resp, err := h.core.RecordEvidence(ctx, &api.RecordEvidenceRequest{Evidence: pbToEvidence(req.GetEvidence())})
	if err != nil {
		return nil, behavioralErr("RecordEvidence", err)
	}
	return &bpb.RecordEvidenceResponse{EvidenceId: resp.EvidenceID}, nil
}

func (h *behavioralHandler) MapAuthority(ctx context.Context, req *bpb.MapAuthorityRequest) (*bpb.MapAuthorityResponse, error) {
	resp, err := h.core.MapAuthority(ctx, &api.MapAuthorityRequest{
		TargetKind:  req.GetTargetKind(),
		TargetID:    req.GetTargetId(),
		Project:     req.GetProject(),
		Domain:      api.DomainRef(req.GetDomain()),
		Authorities: toAuthorityRefs(req.GetAuthorityIds()),
	})
	if err != nil {
		return nil, behavioralErr("MapAuthority", err)
	}
	return &bpb.MapAuthorityResponse{Status: apiGovStatusToPB(resp.Status)}, nil
}

func (h *behavioralHandler) RecordContradiction(ctx context.Context, req *bpb.RecordContradictionRequest) (*bpb.RecordContradictionResponse, error) {
	resp, err := h.core.RecordContradiction(ctx, &api.RecordContradictionRequest{Contradiction: pbToContradiction(req.GetContradiction())})
	if err != nil {
		return nil, behavioralErr("RecordContradiction", err)
	}
	return &bpb.RecordContradictionResponse{ContradictionId: resp.ContradictionID}, nil
}

func (h *behavioralHandler) RegisterCondition(ctx context.Context, req *bpb.RegisterConditionRequest) (*bpb.RegisterConditionResponse, error) {
	resp, err := h.core.RegisterCondition(ctx, &api.RegisterConditionRequest{Condition: pbToCondition(req.GetCondition())})
	if err != nil {
		return nil, behavioralErr("RegisterCondition", err)
	}
	return &bpb.RegisterConditionResponse{ConditionId: resp.ConditionID, Status: apiGovStatusToPB(resp.Status)}, nil
}

func (h *behavioralHandler) RunContradictionCheck(ctx context.Context, req *bpb.RunContradictionCheckRequest) (*bpb.RunContradictionCheckResponse, error) {
	resp, err := h.core.RunContradictionCheck(ctx, &api.RunContradictionCheckRequest{
		PrincipleID: req.GetPrincipleId(),
		Project:     req.GetProject(),
		Domain:      api.DomainRef(req.GetDomain()),
		Actor:       req.GetActor(),
	})
	if err != nil {
		return nil, behavioralErr("RunContradictionCheck", err)
	}
	return &bpb.RunContradictionCheckResponse{
		ContradictionChecked: resp.ContradictionChecked,
		OpenContradictionIds: resp.OpenContradictionIDs,
	}, nil
}

// ── Implemented (PR-3): governance half ───────────────────────────────────────

func (h *behavioralHandler) ProposePrinciple(ctx context.Context, req *bpb.ProposePrincipleRequest) (*bpb.ProposePrincipleResponse, error) {
	resp, err := h.core.ProposePrinciple(ctx, &api.ProposePrincipleRequest{Principle: pbToPrinciple(req.GetPrinciple())})
	if err != nil {
		return nil, behavioralErr("ProposePrinciple", err)
	}
	return &bpb.ProposePrincipleResponse{PrincipleId: resp.PrincipleID, Status: apiGovStatusToPB(resp.Status)}, nil
}

func (h *behavioralHandler) PromotePrinciple(ctx context.Context, req *bpb.PromotePrincipleRequest) (*bpb.PromotePrincipleResponse, error) {
	resp, err := h.core.PromotePrinciple(ctx, &api.PromotePrincipleRequest{
		PrincipleID: req.GetPrincipleId(), Project: req.GetProject(), Domain: api.DomainRef(req.GetDomain()),
		Approver: req.GetApprover(), ApprovedBy: req.GetApprovedBy(), ApprovalReason: req.GetApprovalReason(), Actor: req.GetActor(),
	})
	if err != nil {
		return nil, behavioralErr("PromotePrinciple", err)
	}
	return &bpb.PromotePrincipleResponse{
		Decision: apiPromotionDecisionToPB(resp.Decision),
		Status:   apiGovStatusToPB(resp.Status),
		Record:   promotionDecisionToPB(&resp.Record),
	}, nil
}

func (h *behavioralHandler) RevokePrinciple(ctx context.Context, req *bpb.RevokePrincipleRequest) (*bpb.RevokePrincipleResponse, error) {
	resp, err := h.core.RevokePrinciple(ctx, &api.RevokePrincipleRequest{
		PrincipleID: req.GetPrincipleId(), Project: req.GetProject(), Domain: api.DomainRef(req.GetDomain()),
		Action: req.GetAction(), SupersededBy: req.GetSupersededBy(), Reason: req.GetReason(),
		NarrowedScope: req.GetNarrowedScope(), Actor: req.GetActor(),
	})
	if err != nil {
		return nil, behavioralErr("RevokePrinciple", err)
	}
	return &bpb.RevokePrincipleResponse{Status: apiGovStatusToPB(resp.Status)}, nil
}

func (h *behavioralHandler) ExplainPrinciple(ctx context.Context, req *bpb.ExplainPrincipleRequest) (*bpb.ExplainPrincipleResponse, error) {
	resp, err := h.core.ExplainPrinciple(ctx, &api.ExplainPrincipleRequest{
		PrincipleID: req.GetPrincipleId(), Project: req.GetProject(), Domain: api.DomainRef(req.GetDomain()),
	})
	if err != nil {
		return nil, behavioralErr("ExplainPrinciple", err)
	}
	out := &bpb.ExplainPrincipleResponse{Principle: principleToPB(&resp.Principle), Explanation: resp.Explanation}
	for i := range resp.Evidence {
		out.Evidence = append(out.Evidence, evidenceToPB(&resp.Evidence[i]))
	}
	for i := range resp.Authorities {
		out.Authorities = append(out.Authorities, authorityToPB(&resp.Authorities[i]))
	}
	for i := range resp.Conditions {
		out.Conditions = append(out.Conditions, conditionToPB(&resp.Conditions[i]))
	}
	for i := range resp.Contradictions {
		out.Contradictions = append(out.Contradictions, contradictionToPB(&resp.Contradictions[i]))
	}
	for i := range resp.PromotionHistory {
		out.PromotionHistory = append(out.PromotionHistory, promotionDecisionToPB(&resp.PromotionHistory[i]))
	}
	for i := range resp.RevocationRules {
		out.RevocationRules = append(out.RevocationRules, revocationRuleToPB(&resp.RevocationRules[i]))
	}
	return out, nil
}

// ── Implemented (PR-4): runtime decision support ──────────────────────────────

func (h *behavioralHandler) ResolveGovernedContext(ctx context.Context, req *bpb.ResolveGovernedContextRequest) (*bpb.ResolveGovernedContextResponse, error) {
	resp, err := h.core.ResolveGovernedContext(ctx, &api.ResolveGovernedContextRequest{
		Project: req.GetProject(), Domain: api.DomainRef(req.GetDomain()), Goal: req.GetGoal(),
		Conditions: toConditionRefs(req.GetConditions()), EntityRef: req.GetEntityRef(), Scope: req.GetScope(), Limit: req.GetLimit(),
	})
	if err != nil {
		return nil, behavioralErr("ResolveGovernedContext", err)
	}
	return &bpb.ResolveGovernedContextResponse{Context: governedContextToPB(&resp.Context)}, nil
}

func (h *behavioralHandler) CheckAction(ctx context.Context, req *bpb.CheckActionRequest) (*bpb.CheckActionResponse, error) {
	resp, err := h.core.CheckAction(ctx, &api.CheckActionRequest{
		Project: req.GetProject(), Domain: api.DomainRef(req.GetDomain()),
		ActionType: req.GetActionType(), Target: req.GetTarget(),
		CurrentConditions: toConditionRefs(req.GetCurrentConditions()), Scope: req.GetScope(), AgentID: req.GetAgentId(),
		ProvidedEvidenceRefs: req.GetProvidedEvidenceRefs(), HumanApproval: req.GetHumanApproval(),
	})
	if err != nil {
		return nil, behavioralErr("CheckAction", err)
	}
	return &bpb.CheckActionResponse{Result: actionCheckToPB(&resp.Result)}, nil
}

func (h *behavioralHandler) GetGovernanceCoverage(ctx context.Context, req *bpb.GetGovernanceCoverageRequest) (*bpb.GetGovernanceCoverageResponse, error) {
	resp, err := h.core.GetGovernanceCoverage(ctx, &api.GetGovernanceCoverageRequest{
		Project: req.GetProject(), Domain: api.DomainRef(req.GetDomain()),
	})
	if err != nil {
		return nil, behavioralErr("GetGovernanceCoverage", err)
	}
	c := resp.Coverage
	return &bpb.GetGovernanceCoverageResponse{
		Total: c.Total, Governed: c.Governed, Ungoverned: c.Ungoverned, CoverageRatio: c.Ratio,
	}, nil
}

func (h *behavioralHandler) RecordOutcome(ctx context.Context, req *bpb.RecordOutcomeRequest) (*bpb.RecordOutcomeResponse, error) {
	resp, err := h.core.RecordOutcome(ctx, &api.RecordOutcomeRequest{Outcome: pbToOutcome(req.GetOutcome())})
	if err != nil {
		return nil, behavioralErr("RecordOutcome", err)
	}
	return &bpb.RecordOutcomeResponse{OutcomeId: resp.OutcomeID}, nil
}

func (h *behavioralHandler) GeneratePromotionCandidate(ctx context.Context, req *bpb.GeneratePromotionCandidateRequest) (*bpb.GeneratePromotionCandidateResponse, error) {
	resp, err := h.core.GeneratePromotionCandidate(ctx, &api.GeneratePromotionCandidateRequest{
		Project: req.GetProject(), Domain: api.DomainRef(req.GetDomain()), Theme: req.GetTheme(), MinRepeats: req.GetMinRepeats(),
		DraftPrinciple: pbToPrinciple(req.GetDraftPrinciple()), Actor: req.GetActor(), Rationale: req.GetRationale(),
		SupportingEvidenceIDs: req.GetSupportingEvidenceIds(),
	})
	if err != nil {
		return nil, behavioralErr("GeneratePromotionCandidate", err)
	}
	return &bpb.GeneratePromotionCandidateResponse{
		Candidate:    promotionCandidateToPB(&resp.Candidate),
		OutcomeCount: resp.OutcomeCount,
	}, nil
}

func (h *behavioralHandler) ListPromotionCandidates(ctx context.Context, req *bpb.ListPromotionCandidatesRequest) (*bpb.ListPromotionCandidatesResponse, error) {
	resp, err := h.core.ListPromotionCandidates(ctx, &api.ListPromotionCandidatesRequest{
		Project: req.GetProject(), Domain: api.DomainRef(req.GetDomain()), Theme: req.GetTheme(),
		Status: pbPromotionCandidateStatusToAPI(req.GetStatus()), Limit: req.GetLimit(),
	})
	if err != nil {
		return nil, behavioralErr("ListPromotionCandidates", err)
	}
	out := &bpb.ListPromotionCandidatesResponse{}
	for i := range resp.Candidates {
		out.Candidates = append(out.Candidates, promotionCandidateToPB(&resp.Candidates[i]))
	}
	return out, nil
}

func (h *behavioralHandler) GenerateReconciliationReport(ctx context.Context, req *bpb.GenerateReconciliationReportRequest) (*bpb.GenerateReconciliationReportResponse, error) {
	resp, err := h.core.GenerateReconciliationReport(ctx, &api.GenerateReconciliationReportRequest{
		Project: req.GetProject(), Domain: api.DomainRef(req.GetDomain()), PromotionCandidateID: req.GetPromotionCandidateId(),
		Theme: req.GetTheme(), AWGInvariantIDs: req.GetAwgInvariantIds(), AWGFailureModeIDs: req.GetAwgFailureModeIds(),
		AWGTestIDs: req.GetAwgTestIds(), RuntimeRelevant: req.GetRuntimeRelevant(), Actor: req.GetActor(),
	})
	if err != nil {
		return nil, behavioralErr("GenerateReconciliationReport", err)
	}
	return &bpb.GenerateReconciliationReportResponse{Report: reconciliationReportToPB(&resp.Report)}, nil
}

func (h *behavioralHandler) ListReconciliationReports(ctx context.Context, req *bpb.ListReconciliationReportsRequest) (*bpb.ListReconciliationReportsResponse, error) {
	resp, err := h.core.ListReconciliationReports(ctx, &api.ListReconciliationReportsRequest{
		Project: req.GetProject(), Domain: api.DomainRef(req.GetDomain()), Theme: req.GetTheme(),
		PromotionCandidateID: req.GetPromotionCandidateId(), Limit: req.GetLimit(),
	})
	if err != nil {
		return nil, behavioralErr("ListReconciliationReports", err)
	}
	out := &bpb.ListReconciliationReportsResponse{}
	for i := range resp.Reports {
		out.Reports = append(out.Reports, reconciliationReportToPB(&resp.Reports[i]))
	}
	return out, nil
}

// Compile-time assertion that the handler satisfies the generated server interface.
var _ bpb.BehavioralMemoryServiceServer = (*behavioralHandler)(nil)

// ── proto → api translation (PR-2 implemented RPCs) ───────────────────────────

func pbToSignal(s *bpb.Signal) api.Signal {
	if s == nil {
		return api.Signal{}
	}
	return api.Signal{
		ID:             s.GetId(),
		Project:        s.GetProject(),
		Domain:         api.DomainRef(s.GetDomain()),
		Kind:           pbSignalKindToAPI(s.GetKind()),
		SourceKind:     s.GetSourceKind(),
		SourceRef:      s.GetSourceRef(),
		EntityRef:      s.GetEntityRef(),
		Scope:          s.GetScope(),
		ClusterID:      s.GetClusterId(),
		ConditionRef:   s.GetConditionRef(),
		Severity:       s.GetSeverity(),
		AuthorityLevel: pbObservationAuthorityToAPI(s.GetAuthorityLevel()),
		ObservedAt:     s.GetObservedAt(),
		Payload:        s.GetPayload(),
		Confidence:     s.GetConfidence(),
		Status:         pbGovStatusToAPI(s.GetStatus()),
		Provenance:     api.Provenance{AgentID: s.GetAgentId(), MemoryID: s.GetMemoryId(), CreatedAt: s.GetCreatedAt()},
		Metadata:       s.GetMetadata(),
	}
}

func pbToClaim(c *bpb.Claim) api.Claim {
	if c == nil {
		return api.Claim{}
	}
	return api.Claim{
		ID:            c.GetId(),
		Project:       c.GetProject(),
		Domain:        api.DomainRef(c.GetDomain()),
		SignalID:      c.GetSignalId(),
		Statement:     c.GetStatement(),
		SubjectEntity: c.GetSubjectEntity(),
		Predicate:     c.GetPredicate(),
		ObjectValue:   c.GetObjectValue(),
		TimeRef:       c.GetTimeRef(),
		Status:        pbGovStatusToAPI(c.GetStatus()),
		Confidence:    c.GetConfidence(),
		SourceID:      c.GetSourceId(),
		Provenance:    api.Provenance{CreatedAt: c.GetCreatedAt(), UpdatedAt: c.GetUpdatedAt()},
		Metadata:      c.GetMetadata(),
	}
}

func pbToEvidence(e *bpb.Evidence) api.Evidence {
	if e == nil {
		return api.Evidence{}
	}
	return api.Evidence{
		ID:             e.GetId(),
		Project:        e.GetProject(),
		Domain:         api.DomainRef(e.GetDomain()),
		TargetKind:     e.GetTargetKind(),
		TargetID:       e.GetTargetId(),
		Kind:           e.GetEvidenceKind(),
		Lane:           pbLaneToAPI(e.GetLane()),
		Result:         e.GetResult(),
		ProbeRef:       e.GetProbeRef(),
		SourceKind:     e.GetSourceKind(),
		SourceRef:      e.GetSourceRef(),
		EntityRef:      e.GetEntityRef(),
		ClusterID:      e.GetClusterId(),
		ConditionRef:   e.GetConditionRef(),
		Severity:       e.GetSeverity(),
		AuthorityLevel: pbObservationAuthorityToAPI(e.GetAuthorityLevel()),
		ObservedAt:     e.GetObservedAt(),
		Payload:        e.GetPayload(),
		ObservedFrom:   e.GetObservedFrom(),
		Satisfies:      toRequiredEvidenceRefs(e.GetSatisfies()),
		Provenance:     api.Provenance{SourceRef: e.GetProvenance(), CreatedAt: e.GetCreatedAt()},
		Metadata:       e.GetMetadata(),
	}
}

func pbToContradiction(c *bpb.Contradiction) api.Contradiction {
	if c == nil {
		return api.Contradiction{}
	}
	return api.Contradiction{
		ID:         c.GetId(),
		Project:    c.GetProject(),
		Domain:     api.DomainRef(c.GetDomain()),
		Kind:       c.GetKind(),
		LeftRef:    c.GetLeftRef(),
		RightRef:   c.GetRightRef(),
		Resolution: c.GetResolution(),
		Note:       c.GetNote(),
		CreatedAt:  c.GetCreatedAt(),
		Metadata:   c.GetMetadata(),
	}
}

func toAuthorityRefs(in []string) []api.AuthorityRef {
	out := make([]api.AuthorityRef, len(in))
	for i, v := range in {
		out[i] = api.AuthorityRef(v)
	}
	return out
}

func toRequiredEvidenceRefs(in []string) []api.RequiredEvidenceRef {
	out := make([]api.RequiredEvidenceRef, len(in))
	for i, v := range in {
		out[i] = api.RequiredEvidenceRef(v)
	}
	return out
}

// ── enum translation (explicit; never rely on String() name overlap) ──────────

func pbGovStatusToAPI(s bpb.GovernanceStatus) api.GovernanceStatus {
	switch s {
	case bpb.GovernanceStatus_RAW_SIGNAL:
		return api.StatusRawSignal
	case bpb.GovernanceStatus_EXTRACTED_CLAIM:
		return api.StatusExtractedClaim
	case bpb.GovernanceStatus_CANDIDATE_FACT:
		return api.StatusCandidateFact
	case bpb.GovernanceStatus_EVIDENCE_LINKED:
		return api.StatusEvidenceLinked
	case bpb.GovernanceStatus_AUTHORITY_MAPPED:
		return api.StatusAuthorityMapped
	case bpb.GovernanceStatus_CONDITION_SCOPED:
		return api.StatusConditionScoped
	case bpb.GovernanceStatus_CONTRADICTION_TESTED:
		return api.StatusContradictionTested
	case bpb.GovernanceStatus_PROPOSED_PRINCIPLE:
		return api.StatusProposedPrinciple
	case bpb.GovernanceStatus_PROMOTED_PRINCIPLE:
		return api.StatusPromotedPrinciple
	case bpb.GovernanceStatus_REVOKED:
		return api.StatusRevoked
	case bpb.GovernanceStatus_SUPERSEDED:
		return api.StatusSuperseded
	case bpb.GovernanceStatus_NARROWED:
		return api.StatusNarrowed
	default:
		return api.StatusUnspecified
	}
}

func apiGovStatusToPB(s api.GovernanceStatus) bpb.GovernanceStatus {
	switch s {
	case api.StatusRawSignal:
		return bpb.GovernanceStatus_RAW_SIGNAL
	case api.StatusExtractedClaim:
		return bpb.GovernanceStatus_EXTRACTED_CLAIM
	case api.StatusCandidateFact:
		return bpb.GovernanceStatus_CANDIDATE_FACT
	case api.StatusEvidenceLinked:
		return bpb.GovernanceStatus_EVIDENCE_LINKED
	case api.StatusAuthorityMapped:
		return bpb.GovernanceStatus_AUTHORITY_MAPPED
	case api.StatusConditionScoped:
		return bpb.GovernanceStatus_CONDITION_SCOPED
	case api.StatusContradictionTested:
		return bpb.GovernanceStatus_CONTRADICTION_TESTED
	case api.StatusProposedPrinciple:
		return bpb.GovernanceStatus_PROPOSED_PRINCIPLE
	case api.StatusPromotedPrinciple:
		return bpb.GovernanceStatus_PROMOTED_PRINCIPLE
	case api.StatusRevoked:
		return bpb.GovernanceStatus_REVOKED
	case api.StatusSuperseded:
		return bpb.GovernanceStatus_SUPERSEDED
	case api.StatusNarrowed:
		return bpb.GovernanceStatus_NARROWED
	default:
		return bpb.GovernanceStatus_GOVERNANCE_STATUS_UNSPECIFIED
	}
}

func pbSignalKindToAPI(k bpb.SignalKind) api.SignalKind {
	switch k {
	case bpb.SignalKind_SIGNAL_OBSERVED_RUNTIME_FACT:
		return api.SignalObservedRuntimeFact
	case bpb.SignalKind_SIGNAL_AGENT_INTERPRETATION:
		return api.SignalAgentInterpretation
	case bpb.SignalKind_SIGNAL_HUMAN_CORRECTION:
		return api.SignalHumanCorrection
	case bpb.SignalKind_SIGNAL_AUTOMATED_HEALTH:
		return api.SignalAutomatedHealth
	case bpb.SignalKind_SIGNAL_HISTORICAL_MEMORY:
		return api.SignalHistoricalMemory
	case bpb.SignalKind_SIGNAL_PROMOTED_PRINCIPLE:
		return api.SignalPromotedPrinciple
	default:
		return api.SignalKindUnspecified
	}
}

func pbLaneToAPI(l bpb.EvidenceLaneMode) api.EvidenceLane {
	switch l {
	case bpb.EvidenceLaneMode_EVIDENCE_LANE_STATIC_ONLY:
		return api.LaneStaticOnly
	case bpb.EvidenceLaneMode_EVIDENCE_LANE_RUNTIME_REQUIRED:
		return api.LaneRuntimeRequired
	case bpb.EvidenceLaneMode_EVIDENCE_LANE_HYBRID:
		return api.LaneHybrid
	default:
		return api.LaneUnspecified
	}
}

// ── PR-3 governance converters ────────────────────────────────────────────────

func refSlice[T ~string](in []T) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = string(v)
	}
	return out
}

func toConditionRefs(in []string) []api.ConditionRef {
	out := make([]api.ConditionRef, len(in))
	for i, v := range in {
		out[i] = api.ConditionRef(v)
	}
	return out
}

func toForbiddenMoveRefs(in []string) []api.ForbiddenMoveRef {
	out := make([]api.ForbiddenMoveRef, len(in))
	for i, v := range in {
		out[i] = api.ForbiddenMoveRef(v)
	}
	return out
}

func pbToPrinciple(p *bpb.Principle) api.Principle {
	if p == nil {
		return api.Principle{}
	}
	return api.Principle{
		ID:                   p.GetId(),
		Project:              p.GetProject(),
		Domain:               api.DomainRef(p.GetDomain()),
		Title:                p.GetTitle(),
		AppliesWhen:          toConditionRefs(p.GetAppliesWhen()),
		Authorities:          toAuthorityRefs(p.GetAuthorities()),
		RequiredEvidence:     toRequiredEvidenceRefs(p.GetRequiredEvidence()),
		ForbiddenMoves:       toForbiddenMoveRefs(p.GetForbiddenMoves()),
		RecommendedAction:    p.GetRecommendedAction(),
		RiskLevel:            p.GetRiskLevel(),
		RevocationRule:       p.GetRevocationRule(),
		PromotionReason:      p.GetPromotionReason(),
		Status:               pbGovStatusToAPI(p.GetStatus()),
		SupersededBy:         p.GetSupersededBy(),
		Version:              p.GetVersion(),
		ProposedBy:           p.GetProposedBy(),
		PromotedBy:           p.GetPromotedBy(),
		PromotionDecisionID:  p.GetPromotionDecisionId(),
		RevocationRuleID:     p.GetRevocationRuleId(),
		NarrowedBy:           p.GetNarrowedBy(),
		ContradictionChecked: p.GetContradictionChecked(),
		ApprovedBy:           p.GetApprovedBy(),
		ApprovalReason:       p.GetApprovalReason(),
		ApprovedAt:           p.GetApprovedAt(),
		SourceRefs:           p.GetSourceRefs(),
		GeneratedFrom:        p.GetGeneratedFrom(),
		Provenance:           api.Provenance{CreatedAt: p.GetCreatedAt(), UpdatedAt: p.GetUpdatedAt()},
		Metadata:             p.GetMetadata(),
	}
}

func principleToPB(p *api.Principle) *bpb.Principle {
	return &bpb.Principle{
		Id:                   p.ID,
		Project:              p.Project,
		Domain:               string(p.Domain),
		Title:                p.Title,
		AppliesWhen:          refSlice(p.AppliesWhen),
		Authorities:          refSlice(p.Authorities),
		RequiredEvidence:     refSlice(p.RequiredEvidence),
		ForbiddenMoves:       refSlice(p.ForbiddenMoves),
		RecommendedAction:    p.RecommendedAction,
		RiskLevel:            p.RiskLevel,
		RevocationRule:       p.RevocationRule,
		PromotionReason:      p.PromotionReason,
		Status:               apiGovStatusToPB(p.Status),
		SupersededBy:         p.SupersededBy,
		Version:              p.Version,
		ProposedBy:           p.ProposedBy,
		PromotedBy:           p.PromotedBy,
		CreatedAt:            p.Provenance.CreatedAt,
		UpdatedAt:            p.Provenance.UpdatedAt,
		PromotionDecisionId:  p.PromotionDecisionID,
		RevocationRuleId:     p.RevocationRuleID,
		NarrowedBy:           p.NarrowedBy,
		ContradictionChecked: p.ContradictionChecked,
		ApprovedBy:           p.ApprovedBy,
		ApprovalReason:       p.ApprovalReason,
		ApprovedAt:           p.ApprovedAt,
		SourceRefs:           p.SourceRefs,
		GeneratedFrom:        p.GeneratedFrom,
		Metadata:             p.Metadata,
	}
}

// ── Implemented: governance legibility (P4 discovery + P6 amend) ──────────────

func (h *behavioralHandler) ListAuthorities(ctx context.Context, req *bpb.ListAuthoritiesRequest) (*bpb.ListAuthoritiesResponse, error) {
	resp, err := h.core.ListAuthorities(ctx, &api.ListAuthoritiesRequest{
		Project: req.GetProject(), Domain: api.DomainRef(req.GetDomain()), Limit: req.GetLimit(),
	})
	if err != nil {
		return nil, behavioralErr("ListAuthorities", err)
	}
	out := make([]*bpb.Authority, 0, len(resp.Authorities))
	for i := range resp.Authorities {
		out = append(out, authorityToPB(&resp.Authorities[i]))
	}
	return &bpb.ListAuthoritiesResponse{Authorities: out}, nil
}

func (h *behavioralHandler) ListConditions(ctx context.Context, req *bpb.ListConditionsRequest) (*bpb.ListConditionsResponse, error) {
	resp, err := h.core.ListConditions(ctx, &api.ListConditionsRequest{
		Project: req.GetProject(), Domain: api.DomainRef(req.GetDomain()), Limit: req.GetLimit(),
	})
	if err != nil {
		return nil, behavioralErr("ListConditions", err)
	}
	out := make([]*bpb.Condition, 0, len(resp.Conditions))
	for i := range resp.Conditions {
		out = append(out, conditionToPB(&resp.Conditions[i]))
	}
	return &bpb.ListConditionsResponse{Conditions: out}, nil
}

func (h *behavioralHandler) ResolveRef(ctx context.Context, req *bpb.ResolveRefRequest) (*bpb.ResolveRefResponse, error) {
	resp, err := h.core.ResolveRef(ctx, &api.ResolveRefRequest{
		Project: req.GetProject(), Domain: api.DomainRef(req.GetDomain()), Ref: req.GetRef(),
	})
	if err != nil {
		return nil, behavioralErr("ResolveRef", err)
	}
	out := &bpb.ResolveRefResponse{Resolved: resp.Resolved, Kind: resp.Kind}
	if resp.Authority != nil {
		out.Authority = authorityToPB(resp.Authority)
	}
	if resp.Condition != nil {
		out.Condition = conditionToPB(resp.Condition)
	}
	return out, nil
}

func (h *behavioralHandler) AmendProposal(ctx context.Context, req *bpb.AmendProposalRequest) (*bpb.AmendProposalResponse, error) {
	resp, err := h.core.AmendProposal(ctx, &api.AmendProposalRequest{
		Project: req.GetProject(), Domain: api.DomainRef(req.GetDomain()), ID: req.GetId(), Actor: req.GetActor(),
		AddAuthorityRefs: req.GetAddAuthorityRefs(), RemoveAuthorityRefs: req.GetRemoveAuthorityRefs(),
		AddConditionRefs: req.GetAddConditionRefs(), RemoveConditionRefs: req.GetRemoveConditionRefs(),
		AddEvidenceRefs: req.GetAddEvidenceRefs(), RemoveEvidenceRefs: req.GetRemoveEvidenceRefs(),
		RiskLevel: req.GetRiskLevel(), RevocationRule: req.GetRevocationRule(), PromotionReason: req.GetPromotionReason(),
	})
	if err != nil {
		return nil, behavioralErr("AmendProposal", err)
	}
	return &bpb.AmendProposalResponse{
		PrincipleId: resp.PrincipleID, Status: apiGovStatusToPB(resp.Status),
		Version: resp.Version, ContradictionReset: resp.ContradictionReset,
	}, nil
}

func promotionDecisionToPB(d *api.PromotionDecisionRecord) *bpb.PromotionDecisionRecord {
	return &bpb.PromotionDecisionRecord{
		Id:                     d.ID,
		Project:                d.Project,
		Domain:                 string(d.Domain),
		PrincipleId:            d.PrincipleID,
		Decision:               apiPromotionDecisionToPB(d.Decision),
		Verdict:                d.Verdict,
		MissingEvidence:        d.MissingEvidence,
		BlockedByForbidden:     d.BlockedByForbidden,
		Reviewer:               d.Reviewer,
		Reason:                 d.Reason,
		CreatedAt:              d.CreatedAt,
		UnresolvedAuthority:    d.UnresolvedAuthority,
		UnresolvedConditions:   d.UnresolvedConditions,
		BlockingContradictions: d.BlockingContradictions,
		RiskLevel:              d.RiskLevel,
		ReviewRequired:         d.ReviewRequired,
		ApprovedBy:             d.ApprovedBy,
		PromotionReason:        d.PromotionReason,
		Actor:                  d.Actor,
		Metadata:               d.Metadata,
		SatisfactionSteps:      apiSatisfactionStepsToPB(d.SatisfactionSteps),
		SatisfactionSummary:    d.SatisfactionSummary,
	}
}

// apiSatisfactionStepsToPB maps the kernel's satisfaction recipe to protobuf.
func apiSatisfactionStepsToPB(steps []api.SatisfactionStep) []*bpb.SatisfactionStep {
	if len(steps) == 0 {
		return nil
	}
	out := make([]*bpb.SatisfactionStep, 0, len(steps))
	for _, s := range steps {
		out = append(out, &bpb.SatisfactionStep{
			Requirement:    s.Requirement,
			Satisfied:      s.Satisfied,
			Detail:         s.Detail,
			HowToSatisfy:   s.HowToSatisfy,
			NextOperations: s.NextOperations,
		})
	}
	return out
}

func revocationRuleToPB(r *api.RevocationRule) *bpb.RevocationRule {
	return &bpb.RevocationRule{
		Id:               r.ID,
		Project:          r.Project,
		Domain:           string(r.Domain),
		PrincipleId:      r.PrincipleID,
		Condition:        r.Condition,
		Action:           r.Action,
		Note:             r.Note,
		CreatedAt:        r.CreatedAt,
		RevocationReason: r.RevocationReason,
		Actor:            r.Actor,
		SupersededBy:     r.SupersededBy,
		NarrowedScope:    r.NarrowedScope,
		Metadata:         r.Metadata,
	}
}

func evidenceToPB(e *api.Evidence) *bpb.Evidence {
	return &bpb.Evidence{
		Id:             e.ID,
		Project:        e.Project,
		Domain:         string(e.Domain),
		TargetKind:     e.TargetKind,
		TargetId:       e.TargetID,
		EvidenceKind:   e.Kind,
		Lane:           apiLaneToPB(e.Lane),
		Result:         e.Result,
		ProbeRef:       e.ProbeRef,
		ObservedAt:     e.ObservedAt,
		Payload:        e.Payload,
		Provenance:     e.Provenance.SourceRef,
		CreatedAt:      e.Provenance.CreatedAt,
		ObservedFrom:   e.ObservedFrom,
		Satisfies:      refSlice(e.Satisfies),
		Metadata:       e.Metadata,
		SourceKind:     e.SourceKind,
		SourceRef:      e.SourceRef,
		EntityRef:      e.EntityRef,
		ClusterId:      e.ClusterID,
		ConditionRef:   e.ConditionRef,
		Severity:       e.Severity,
		AuthorityLevel: apiObservationAuthorityToPB(e.AuthorityLevel),
	}
}

func authorityToPB(a *api.Authority) *bpb.Authority {
	return &bpb.Authority{
		Id:             a.ID,
		Project:        a.Project,
		Domain:         string(a.Domain),
		Title:          a.Title,
		Governs:        a.Governs,
		OwnerKind:      a.OwnerKind,
		ReadPath:       a.ReadPath,
		WritePath:      a.WritePath,
		IdentitySource: a.IdentitySource,
		Status:         apiGovStatusToPB(a.Status),
		GovernsRefs:    a.GovernsRefs,
		Metadata:       a.Metadata,
	}
}

func conditionToPB(c *api.Condition) *bpb.Condition {
	return &bpb.Condition{
		Id:         c.ID,
		Project:    c.Project,
		Domain:     string(c.Domain),
		Title:      c.Title,
		DetectSpec: c.DetectSpec,
		Severity:   c.Severity,
		Status:     apiGovStatusToPB(c.Status),
		Metadata:   c.Metadata,
	}
}

// pbToCondition converts an inbound proto Condition to the api type. Status is
// left to the core (RegisterCondition defaults it) so the catalog entry's
// lifecycle is owned by the kernel, not the caller.
func pbToCondition(c *bpb.Condition) api.Condition {
	if c == nil {
		return api.Condition{}
	}
	return api.Condition{
		ID:         c.GetId(),
		Project:    c.GetProject(),
		Domain:     api.DomainRef(c.GetDomain()),
		Title:      c.GetTitle(),
		DetectSpec: c.GetDetectSpec(),
		Severity:   c.GetSeverity(),
		Metadata:   c.GetMetadata(),
	}
}

func contradictionToPB(c *api.Contradiction) *bpb.Contradiction {
	return &bpb.Contradiction{
		Id:         c.ID,
		Project:    c.Project,
		Domain:     string(c.Domain),
		Kind:       c.Kind,
		LeftRef:    c.LeftRef,
		RightRef:   c.RightRef,
		Resolution: c.Resolution,
		Note:       c.Note,
		CreatedAt:  c.CreatedAt,
		Metadata:   c.Metadata,
	}
}

func apiPromotionDecisionToPB(d api.PromotionDecision) bpb.PromotionDecision {
	switch d {
	case api.PromotionAllowed:
		return bpb.PromotionDecision_PROMOTION_ALLOWED
	case api.PromotionBlocked:
		return bpb.PromotionDecision_PROMOTION_BLOCKED
	case api.PromotionReviewRequired:
		return bpb.PromotionDecision_PROMOTION_REVIEW_REQUIRED
	default:
		return bpb.PromotionDecision_PROMOTION_DECISION_UNSPECIFIED
	}
}

func apiLaneToPB(l api.EvidenceLane) bpb.EvidenceLaneMode {
	switch l {
	case api.LaneStaticOnly:
		return bpb.EvidenceLaneMode_EVIDENCE_LANE_STATIC_ONLY
	case api.LaneRuntimeRequired:
		return bpb.EvidenceLaneMode_EVIDENCE_LANE_RUNTIME_REQUIRED
	case api.LaneHybrid:
		return bpb.EvidenceLaneMode_EVIDENCE_LANE_HYBRID
	default:
		return bpb.EvidenceLaneMode_EVIDENCE_LANE_MODE_UNSPECIFIED
	}
}

func pbObservationAuthorityToAPI(l bpb.ObservationAuthorityLevel) api.ObservationAuthorityLevel {
	switch l {
	case bpb.ObservationAuthorityLevel_OBSERVATION_AUTHORITY_LEVEL_INTERPRETATION:
		return api.ObservationAuthorityInterpretation
	case bpb.ObservationAuthorityLevel_OBSERVATION_AUTHORITY_LEVEL_EVENT_STREAM:
		return api.ObservationAuthorityEventStream
	case bpb.ObservationAuthorityLevel_OBSERVATION_AUTHORITY_LEVEL_DIAGNOSTIC_CLAIM:
		return api.ObservationAuthorityDiagnostic
	case bpb.ObservationAuthorityLevel_OBSERVATION_AUTHORITY_LEVEL_DERIVED_EVIDENCE:
		return api.ObservationAuthorityDerived
	case bpb.ObservationAuthorityLevel_OBSERVATION_AUTHORITY_LEVEL_TRUTH_PLANE:
		return api.ObservationAuthorityTruthPlane
	default:
		return api.ObservationAuthorityUnspecified
	}
}

func apiObservationAuthorityToPB(l api.ObservationAuthorityLevel) bpb.ObservationAuthorityLevel {
	switch l {
	case api.ObservationAuthorityInterpretation:
		return bpb.ObservationAuthorityLevel_OBSERVATION_AUTHORITY_LEVEL_INTERPRETATION
	case api.ObservationAuthorityEventStream:
		return bpb.ObservationAuthorityLevel_OBSERVATION_AUTHORITY_LEVEL_EVENT_STREAM
	case api.ObservationAuthorityDiagnostic:
		return bpb.ObservationAuthorityLevel_OBSERVATION_AUTHORITY_LEVEL_DIAGNOSTIC_CLAIM
	case api.ObservationAuthorityDerived:
		return bpb.ObservationAuthorityLevel_OBSERVATION_AUTHORITY_LEVEL_DERIVED_EVIDENCE
	case api.ObservationAuthorityTruthPlane:
		return bpb.ObservationAuthorityLevel_OBSERVATION_AUTHORITY_LEVEL_TRUTH_PLANE
	default:
		return bpb.ObservationAuthorityLevel_OBSERVATION_AUTHORITY_LEVEL_UNSPECIFIED
	}
}

// ── PR-4 runtime converters ───────────────────────────────────────────────────

func pbToOutcome(o *bpb.Outcome) api.Outcome {
	if o == nil {
		return api.Outcome{}
	}
	return api.Outcome{
		ID:                 o.GetId(),
		Project:            o.GetProject(),
		Domain:             api.DomainRef(o.GetDomain()),
		ActionCheckID:      o.GetActionCheckId(),
		PrincipleIDs:       o.GetPrincipleIds(),
		EvidenceIDs:        o.GetEvidenceIds(),
		Status:             o.GetStatus(),
		Severe:             o.GetSevere(),
		HumanMarked:        o.GetHumanMarked(),
		IncidentID:         o.GetIncidentId(),
		Theme:              o.GetTheme(),
		Note:               o.GetNote(),
		AgentID:            o.GetAgentId(),
		CreatedAt:          o.GetCreatedAt(),
		SupportsPrinciples: o.GetSupportsPrinciples(),
		WeakensPrinciples:  o.GetWeakensPrinciples(),
		Metadata:           o.GetMetadata(),
	}
}

func outcomeToPB(o *api.Outcome) *bpb.Outcome {
	return &bpb.Outcome{
		Id:                 o.ID,
		Project:            o.Project,
		Domain:             string(o.Domain),
		ActionCheckId:      o.ActionCheckID,
		PrincipleIds:       o.PrincipleIDs,
		EvidenceIds:        o.EvidenceIDs,
		Status:             o.Status,
		Severe:             o.Severe,
		HumanMarked:        o.HumanMarked,
		IncidentId:         o.IncidentID,
		Theme:              o.Theme,
		Note:               o.Note,
		AgentId:            o.AgentID,
		CreatedAt:          o.CreatedAt,
		SupportsPrinciples: o.SupportsPrinciples,
		WeakensPrinciples:  o.WeakensPrinciples,
		Metadata:           o.Metadata,
	}
}

func promotionCandidateToPB(c *api.PromotionCandidate) *bpb.PromotionCandidate {
	if c == nil {
		return nil
	}
	return &bpb.PromotionCandidate{
		Id:                      c.ID,
		Project:                 c.Project,
		Domain:                  string(c.Domain),
		Theme:                   c.Theme,
		Status:                  apiPromotionCandidateStatusToPB(c.Status),
		Title:                   c.Title,
		Summary:                 c.Summary,
		Rationale:               c.Rationale,
		SupportingOutcomeIds:    c.SupportingOutcomeIDs,
		SupportingEvidenceIds:   c.SupportingEvidenceIDs,
		RepeatCount:             c.RepeatCount,
		DraftPrinciple:          principleToPB(&c.DraftPrinciple),
		GeneratedBy:             c.GeneratedBy,
		CreatedAt:               c.CreatedAt,
		UpdatedAt:               c.UpdatedAt,
		MaterializedPrincipleId: c.MaterializedPrincipleID,
		Metadata:                c.Metadata,
	}
}

func reconciliationReportToPB(r *api.ReconciliationReport) *bpb.ReconciliationReport {
	if r == nil {
		return nil
	}
	return &bpb.ReconciliationReport{
		Id:                        r.ID,
		Project:                   r.Project,
		Domain:                    string(r.Domain),
		PromotionCandidateId:      r.PromotionCandidateID,
		Theme:                     r.Theme,
		AwgInvariantIds:           r.AWGInvariantIDs,
		AwgFailureModeIds:         r.AWGFailureModeIDs,
		AwgTestIds:                r.AWGTestIDs,
		Findings:                  r.Findings,
		Summary:                   r.Summary,
		OutcomeCount:              r.OutcomeCount,
		FailureCount:              r.FailureCount,
		SuccessCount:              r.SuccessCount,
		SevereCount:               r.SevereCount,
		ProposedAwgInvariantIds:   r.ProposedAWGInvariantIDs,
		ProposedAwgFailureModeIds: r.ProposedAWGFailureModeIDs,
		ProposedAwgTestIds:        r.ProposedAWGTestIDs,
		ProposedBehavioralTheme:   r.ProposedBehavioralTheme,
		Actor:                     r.Actor,
		CreatedAt:                 r.CreatedAt,
		Metadata:                  r.Metadata,
	}
}

func pbPromotionCandidateStatusToAPI(s bpb.PromotionCandidateStatus) api.PromotionCandidateStatus {
	switch s {
	case bpb.PromotionCandidateStatus_PROMOTION_CANDIDATE_STATUS_QUEUED:
		return api.PromotionCandidateStatusQueued
	case bpb.PromotionCandidateStatus_PROMOTION_CANDIDATE_STATUS_REVIEWED:
		return api.PromotionCandidateStatusReviewed
	case bpb.PromotionCandidateStatus_PROMOTION_CANDIDATE_STATUS_DISMISSED:
		return api.PromotionCandidateStatusDismissed
	case bpb.PromotionCandidateStatus_PROMOTION_CANDIDATE_STATUS_MATERIALIZED:
		return api.PromotionCandidateStatusMaterialized
	default:
		return api.PromotionCandidateStatusUnspecified
	}
}

func apiPromotionCandidateStatusToPB(s api.PromotionCandidateStatus) bpb.PromotionCandidateStatus {
	switch s {
	case api.PromotionCandidateStatusQueued:
		return bpb.PromotionCandidateStatus_PROMOTION_CANDIDATE_STATUS_QUEUED
	case api.PromotionCandidateStatusReviewed:
		return bpb.PromotionCandidateStatus_PROMOTION_CANDIDATE_STATUS_REVIEWED
	case api.PromotionCandidateStatusDismissed:
		return bpb.PromotionCandidateStatus_PROMOTION_CANDIDATE_STATUS_DISMISSED
	case api.PromotionCandidateStatusMaterialized:
		return bpb.PromotionCandidateStatus_PROMOTION_CANDIDATE_STATUS_MATERIALIZED
	default:
		return bpb.PromotionCandidateStatus_PROMOTION_CANDIDATE_STATUS_UNSPECIFIED
	}
}

func actionCheckToPB(a *api.ActionCheck) *bpb.ActionCheck {
	return &bpb.ActionCheck{
		Id:                       a.ID,
		Project:                  a.Project,
		Domain:                   string(a.Domain),
		ActionType:               a.ActionType,
		Target:                   a.Target,
		Conditions:               refSlice(a.Conditions),
		Allowed:                  a.Allowed,
		Status:                   a.Status,
		ViolatedPrinciples:       a.ViolatedPrinciples,
		MissingEvidence:          refSlice(a.MissingEvidence),
		UnresolvedAuthority:      refSlice(a.UnresolvedAuthority),
		ForbiddenMatched:         refSlice(a.ForbiddenMatched),
		RecommendedSteps:         a.RecommendedSteps,
		Explanation:              a.Explanation,
		AgentId:                  a.AgentID,
		CreatedAt:                a.CreatedAt,
		CheckedAgainstPrinciples: a.CheckedAgainstPrinciples,
		Metadata:                 a.Metadata,
		Governed:                 a.Governed,
	}
}

func signalToPB(s *api.Signal) *bpb.Signal {
	return &bpb.Signal{
		Id: s.ID, Project: s.Project, Domain: string(s.Domain), Kind: apiSignalKindToPB(s.Kind),
		SourceKind: s.SourceKind, SourceRef: s.SourceRef, EntityRef: s.EntityRef, Scope: s.Scope,
		ClusterId: s.ClusterID, ConditionRef: s.ConditionRef, Severity: s.Severity, AuthorityLevel: apiObservationAuthorityToPB(s.AuthorityLevel),
		ObservedAt: s.ObservedAt, Payload: s.Payload, Confidence: s.Confidence, Status: apiGovStatusToPB(s.Status),
		AgentId: s.Provenance.AgentID, MemoryId: s.Provenance.MemoryID, CreatedAt: s.Provenance.CreatedAt, Metadata: s.Metadata,
	}
}

func claimToPB(c *api.Claim) *bpb.Claim {
	return &bpb.Claim{
		Id: c.ID, Project: c.Project, Domain: string(c.Domain), SignalId: c.SignalID, Statement: c.Statement,
		SubjectEntity: c.SubjectEntity, Predicate: c.Predicate, ObjectValue: c.ObjectValue, TimeRef: c.TimeRef,
		Status: apiGovStatusToPB(c.Status), Confidence: c.Confidence, SourceId: c.SourceID,
		CreatedAt: c.Provenance.CreatedAt, UpdatedAt: c.Provenance.UpdatedAt, Metadata: c.Metadata,
	}
}

func requiredEvidenceToPB(r *api.RequiredEvidence) *bpb.RequiredEvidence {
	return &bpb.RequiredEvidence{
		Id: r.ID, Project: r.Project, Domain: string(r.Domain), Title: r.Title, Lane: apiLaneToPB(r.Lane),
		ProbeRef: r.ProbeRef, Predicate: r.Predicate, AppliesTo: r.AppliesTo, Metadata: r.Metadata,
	}
}

func forbiddenMoveToPB(f *api.ForbiddenMove) *bpb.ForbiddenMove {
	return &bpb.ForbiddenMove{
		Id: f.ID, Project: f.Project, Domain: string(f.Domain), Title: f.Title, Summary: f.Summary, Reason: f.Reason,
		ActionType: f.ActionType, TargetPattern: f.TargetPattern, RelatedPrinciples: f.RelatedPrinciples,
		Status: apiGovStatusToPB(f.Status), Metadata: f.Metadata,
	}
}

func governedContextToPB(c *api.GovernedContext) *bpb.GovernedContext {
	g := &bpb.GovernedContext{
		RelevantMemoryIds:   c.RelevantMemoryIDs,
		RecommendedBehavior: c.RecommendedBehavior,
		Confidence:          c.Confidence,
	}
	for i := range c.Signals {
		g.Signals = append(g.Signals, signalToPB(&c.Signals[i]))
	}
	for i := range c.Claims {
		g.Claims = append(g.Claims, claimToPB(&c.Claims[i]))
	}
	for i := range c.ApplicablePrinciples {
		g.ApplicablePrinciples = append(g.ApplicablePrinciples, principleToPB(&c.ApplicablePrinciples[i]))
	}
	for i := range c.MatchedConditions {
		g.MatchedConditions = append(g.MatchedConditions, conditionToPB(&c.MatchedConditions[i]))
	}
	for i := range c.RequiredEvidence {
		g.RequiredEvidence = append(g.RequiredEvidence, requiredEvidenceToPB(&c.RequiredEvidence[i]))
	}
	for i := range c.ForbiddenMoves {
		g.ForbiddenMoves = append(g.ForbiddenMoves, forbiddenMoveToPB(&c.ForbiddenMoves[i]))
	}
	for i := range c.UnresolvedAuthority {
		g.UnresolvedAuthority = append(g.UnresolvedAuthority, authorityToPB(&c.UnresolvedAuthority[i]))
	}
	for i := range c.KnownContradictions {
		g.KnownContradictions = append(g.KnownContradictions, contradictionToPB(&c.KnownContradictions[i]))
	}
	for i := range c.PriorOutcomes {
		g.PriorOutcomes = append(g.PriorOutcomes, outcomeToPB(&c.PriorOutcomes[i]))
	}
	return g
}

func apiSignalKindToPB(k api.SignalKind) bpb.SignalKind {
	switch k {
	case api.SignalObservedRuntimeFact:
		return bpb.SignalKind_SIGNAL_OBSERVED_RUNTIME_FACT
	case api.SignalAgentInterpretation:
		return bpb.SignalKind_SIGNAL_AGENT_INTERPRETATION
	case api.SignalHumanCorrection:
		return bpb.SignalKind_SIGNAL_HUMAN_CORRECTION
	case api.SignalAutomatedHealth:
		return bpb.SignalKind_SIGNAL_AUTOMATED_HEALTH
	case api.SignalHistoricalMemory:
		return bpb.SignalKind_SIGNAL_HISTORICAL_MEMORY
	case api.SignalPromotedPrinciple:
		return bpb.SignalKind_SIGNAL_PROMOTED_PRINCIPLE
	default:
		return bpb.SignalKind_SIGNAL_KIND_UNSPECIFIED
	}
}
