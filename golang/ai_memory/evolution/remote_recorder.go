package evolution

import (
	"context"
	"fmt"
	"sync"
	"time"

	behavioral "github.com/globulario/services/golang/ai_memory/behavioral/api"
	behavioralpb "github.com/globulario/services/golang/ai_memory/behavioral_memorypb"
	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
	"google.golang.org/grpc"
)

// RemoteRecorder implements the deliberately narrow BehavioralRecorder over the
// normal authenticated BehavioralMemoryService connection. It intentionally has
// no PromotePrinciple method: simulation ingestion cannot acquire promotion
// authority merely because it can reach the service.
//
// It also has no address field. Behavioral Memory has exactly one owner and
// exactly one way to find it — Globular service discovery. A caller-supplied
// endpoint would be a second routing authority, letting an operator or agent
// point production ingestion at a stale or unintended instance. Tests inject a
// fake through the BehavioralRecorder interface on SimulationIngestor instead;
// that seam carries no production routing.
type RemoteRecorder struct {
	Timeout time.Duration

	mu sync.Mutex
	cc *grpc.ClientConn
}

func NewRemoteRecorder(timeout time.Duration) *RemoteRecorder {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &RemoteRecorder{Timeout: timeout}
}

func (r *RemoteRecorder) conn() (*grpc.ClientConn, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cc != nil {
		return r.cc, nil
	}
	addr := config.ResolveServiceAddr("ai_memory.AiMemoryService", "")
	if addr == "" {
		return nil, fmt.Errorf(
			"behavioral-memory endpoint not resolvable through service discovery; " +
				"refusing to ingest without the discovered owner",
		)
	}
	opts, err := globular.InternalDialOptions()
	if err != nil {
		return nil, fmt.Errorf("behavioral-memory dial options: %w", err)
	}
	cc, err := grpc.Dial(addr, opts...) //nolint:staticcheck // preserves Globular target resolution semantics
	if err != nil {
		return nil, fmt.Errorf("behavioral-memory dial: %w", err)
	}
	r.cc = cc
	return cc, nil
}

func (r *RemoteRecorder) Close() error {
	r.mu.Lock()
	cc := r.cc
	r.cc = nil
	r.mu.Unlock()
	if cc != nil {
		return cc.Close()
	}
	return nil
}

func (r *RemoteRecorder) RecordSignal(ctx context.Context, req *behavioral.RecordSignalRequest) (*behavioral.RecordSignalResponse, error) {
	cc, err := r.conn()
	if err != nil {
		return nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()
	resp, err := behavioralpb.NewBehavioralMemoryServiceClient(cc).RecordSignal(callCtx, &behavioralpb.RecordSignalRequest{Signal: evolutionSignalToPB(req.Signal)})
	if err != nil {
		return nil, fmt.Errorf("record simulation signal: %w", err)
	}
	return &behavioral.RecordSignalResponse{SignalID: resp.GetSignalId(), Status: req.Signal.Status}, nil
}

func (r *RemoteRecorder) RecordEvidence(ctx context.Context, req *behavioral.RecordEvidenceRequest) (*behavioral.RecordEvidenceResponse, error) {
	cc, err := r.conn()
	if err != nil {
		return nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()
	resp, err := behavioralpb.NewBehavioralMemoryServiceClient(cc).RecordEvidence(callCtx, &behavioralpb.RecordEvidenceRequest{Evidence: evolutionEvidenceToPB(req.Evidence)})
	if err != nil {
		return nil, fmt.Errorf("record simulation evidence: %w", err)
	}
	return &behavioral.RecordEvidenceResponse{EvidenceID: resp.GetEvidenceId()}, nil
}

func (r *RemoteRecorder) RecordOutcome(ctx context.Context, req *behavioral.RecordOutcomeRequest) (*behavioral.RecordOutcomeResponse, error) {
	cc, err := r.conn()
	if err != nil {
		return nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()
	o := req.Outcome
	resp, err := behavioralpb.NewBehavioralMemoryServiceClient(cc).RecordOutcome(callCtx, &behavioralpb.RecordOutcomeRequest{Outcome: &behavioralpb.Outcome{
		Id: o.ID, Project: o.Project, Domain: string(o.Domain), ActionCheckId: o.ActionCheckID,
		PrincipleIds: o.PrincipleIDs, EvidenceIds: o.EvidenceIDs, Status: o.Status,
		Severe: o.Severe, HumanMarked: o.HumanMarked, IncidentId: o.IncidentID,
		Theme: o.Theme, Note: o.Note, AgentId: o.AgentID, CreatedAt: o.CreatedAt,
		SupportsPrinciples: o.SupportsPrinciples, WeakensPrinciples: o.WeakensPrinciples,
		Metadata: o.Metadata,
	}})
	if err != nil {
		return nil, fmt.Errorf("record simulation outcome: %w", err)
	}
	return &behavioral.RecordOutcomeResponse{OutcomeID: resp.GetOutcomeId()}, nil
}

func evolutionSignalToPB(s behavioral.Signal) *behavioralpb.Signal {
	return &behavioralpb.Signal{
		Id: s.ID, Project: s.Project, Domain: string(s.Domain), Kind: evolutionSignalKindToPB(s.Kind),
		SourceKind: s.SourceKind, SourceRef: s.SourceRef, EntityRef: s.EntityRef, Scope: s.Scope,
		ObservedAt: s.ObservedAt, Payload: s.Payload, Confidence: s.Confidence,
		AgentId: s.Provenance.AgentID, MemoryId: s.Provenance.MemoryID,
		Status: evolutionGovernanceStatusToPB(s.Status), CreatedAt: s.Provenance.CreatedAt,
		Metadata: s.Metadata, ClusterId: s.ClusterID, ConditionRef: s.ConditionRef,
		Severity: s.Severity, AuthorityLevel: evolutionAuthorityLevelToPB(s.AuthorityLevel),
	}
}

func evolutionEvidenceToPB(e behavioral.Evidence) *behavioralpb.Evidence {
	return &behavioralpb.Evidence{
		Id: e.ID, Project: e.Project, Domain: string(e.Domain), TargetKind: e.TargetKind,
		TargetId: e.TargetID, EvidenceKind: e.Kind, Lane: evolutionEvidenceLaneToPB(e.Lane),
		Result: e.Result, ProbeRef: e.ProbeRef, ObservedAt: e.ObservedAt, Payload: e.Payload,
		Provenance: e.Provenance.SourceRef, CreatedAt: e.Provenance.CreatedAt, Metadata: e.Metadata,
		ObservedFrom: e.ObservedFrom, Satisfies: evolutionRefsToStrings(e.Satisfies), SourceKind: e.SourceKind,
		SourceRef: e.SourceRef, EntityRef: e.EntityRef, ClusterId: e.ClusterID,
		ConditionRef: e.ConditionRef, Severity: e.Severity,
		AuthorityLevel: evolutionAuthorityLevelToPB(e.AuthorityLevel), ActionRef: e.ActionRef,
	}
}

func evolutionGovernanceStatusToPB(s behavioral.GovernanceStatus) behavioralpb.GovernanceStatus {
	switch s {
	case behavioral.StatusRawSignal:
		return behavioralpb.GovernanceStatus_RAW_SIGNAL
	case behavioral.StatusExtractedClaim:
		return behavioralpb.GovernanceStatus_EXTRACTED_CLAIM
	case behavioral.StatusCandidateFact:
		return behavioralpb.GovernanceStatus_CANDIDATE_FACT
	case behavioral.StatusEvidenceLinked:
		return behavioralpb.GovernanceStatus_EVIDENCE_LINKED
	case behavioral.StatusAuthorityMapped:
		return behavioralpb.GovernanceStatus_AUTHORITY_MAPPED
	case behavioral.StatusConditionScoped:
		return behavioralpb.GovernanceStatus_CONDITION_SCOPED
	case behavioral.StatusContradictionTested:
		return behavioralpb.GovernanceStatus_CONTRADICTION_TESTED
	case behavioral.StatusProposedPrinciple:
		return behavioralpb.GovernanceStatus_PROPOSED_PRINCIPLE
	case behavioral.StatusPromotedPrinciple:
		return behavioralpb.GovernanceStatus_PROMOTED_PRINCIPLE
	case behavioral.StatusRevoked:
		return behavioralpb.GovernanceStatus_REVOKED
	case behavioral.StatusSuperseded:
		return behavioralpb.GovernanceStatus_SUPERSEDED
	case behavioral.StatusNarrowed:
		return behavioralpb.GovernanceStatus_NARROWED
	default:
		return behavioralpb.GovernanceStatus_GOVERNANCE_STATUS_UNSPECIFIED
	}
}

func evolutionSignalKindToPB(k behavioral.SignalKind) behavioralpb.SignalKind {
	switch k {
	case behavioral.SignalObservedRuntimeFact:
		return behavioralpb.SignalKind_SIGNAL_OBSERVED_RUNTIME_FACT
	case behavioral.SignalAgentInterpretation:
		return behavioralpb.SignalKind_SIGNAL_AGENT_INTERPRETATION
	case behavioral.SignalHumanCorrection:
		return behavioralpb.SignalKind_SIGNAL_HUMAN_CORRECTION
	case behavioral.SignalAutomatedHealth:
		return behavioralpb.SignalKind_SIGNAL_AUTOMATED_HEALTH
	case behavioral.SignalHistoricalMemory:
		return behavioralpb.SignalKind_SIGNAL_HISTORICAL_MEMORY
	case behavioral.SignalPromotedPrinciple:
		return behavioralpb.SignalKind_SIGNAL_PROMOTED_PRINCIPLE
	default:
		return behavioralpb.SignalKind_SIGNAL_KIND_UNSPECIFIED
	}
}

func evolutionEvidenceLaneToPB(l behavioral.EvidenceLane) behavioralpb.EvidenceLaneMode {
	switch l {
	case behavioral.LaneStaticOnly:
		return behavioralpb.EvidenceLaneMode_EVIDENCE_LANE_STATIC_ONLY
	case behavioral.LaneRuntimeRequired:
		return behavioralpb.EvidenceLaneMode_EVIDENCE_LANE_RUNTIME_REQUIRED
	case behavioral.LaneHybrid:
		return behavioralpb.EvidenceLaneMode_EVIDENCE_LANE_HYBRID
	default:
		return behavioralpb.EvidenceLaneMode_EVIDENCE_LANE_MODE_UNSPECIFIED
	}
}

func evolutionAuthorityLevelToPB(l behavioral.ObservationAuthorityLevel) behavioralpb.ObservationAuthorityLevel {
	switch l {
	case behavioral.ObservationAuthorityInterpretation:
		return behavioralpb.ObservationAuthorityLevel_OBSERVATION_AUTHORITY_LEVEL_INTERPRETATION
	case behavioral.ObservationAuthorityEventStream:
		return behavioralpb.ObservationAuthorityLevel_OBSERVATION_AUTHORITY_LEVEL_EVENT_STREAM
	case behavioral.ObservationAuthorityDiagnostic:
		return behavioralpb.ObservationAuthorityLevel_OBSERVATION_AUTHORITY_LEVEL_DIAGNOSTIC_CLAIM
	case behavioral.ObservationAuthorityDerived:
		return behavioralpb.ObservationAuthorityLevel_OBSERVATION_AUTHORITY_LEVEL_DERIVED_EVIDENCE
	case behavioral.ObservationAuthorityTruthPlane:
		return behavioralpb.ObservationAuthorityLevel_OBSERVATION_AUTHORITY_LEVEL_TRUTH_PLANE
	default:
		return behavioralpb.ObservationAuthorityLevel_OBSERVATION_AUTHORITY_LEVEL_UNSPECIFIED
	}
}

func evolutionRefsToStrings[T ~string](in []T) []string {
	out := make([]string, len(in))
	for idx, value := range in {
		out[idx] = string(value)
	}
	return out
}
