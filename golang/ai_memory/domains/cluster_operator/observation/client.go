package observation

import (
	"context"
	"fmt"
	"time"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	behavioralpb "github.com/globulario/services/golang/ai_memory/behavioral_memorypb"
	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
	"google.golang.org/grpc"
)

// RecordBundle writes a governed observation bundle into behavioral-memory.
// It is strictly best-effort: callers decide whether to ignore the error.
func RecordBundle(ctx context.Context, bundle Bundle) error {
	if bundle.Signal.Project == "" || bundle.Signal.Domain == "" {
		return fmt.Errorf("observation bundle requires project and domain")
	}
	addr := config.ResolveServiceAddr("ai_memory.AiMemoryService", "")
	if addr == "" {
		return fmt.Errorf("behavioral-memory endpoint not resolvable")
	}
	opts, err := globular.InternalDialOptions()
	if err != nil {
		return fmt.Errorf("behavioral-memory dial options: %w", err)
	}
	cc, err := grpc.Dial(addr, opts...)
	if err != nil {
		return fmt.Errorf("behavioral-memory dial: %w", err)
	}
	defer cc.Close()

	client := behavioralpb.NewBehavioralMemoryServiceClient(cc)
	callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	sigPB := signalToPB(bundle.Signal)
	rsp, err := client.RecordSignal(callCtx, &behavioralpb.RecordSignalRequest{Signal: sigPB})
	if err != nil {
		return fmt.Errorf("record signal: %w", err)
	}
	signalID := rsp.GetSignalId()
	for _, e := range bundle.Evidence {
		ev := e
		if ev.TargetKind == "" {
			ev.TargetKind = "signal"
		}
		if ev.TargetID == "" {
			ev.TargetID = signalID
		}
		if ev.ObservedFrom == "" {
			ev.ObservedFrom = signalID
		}
		if _, err := client.RecordEvidence(callCtx, &behavioralpb.RecordEvidenceRequest{Evidence: evidenceToPB(ev)}); err != nil {
			return fmt.Errorf("record evidence %s: %w", ev.ID, err)
		}
	}
	return nil
}

func signalToPB(s api.Signal) *behavioralpb.Signal {
	return &behavioralpb.Signal{
		Id:             s.ID,
		Project:        s.Project,
		Domain:         string(s.Domain),
		Kind:           signalKindToPB(s.Kind),
		SourceKind:     s.SourceKind,
		SourceRef:      s.SourceRef,
		EntityRef:      s.EntityRef,
		Scope:          s.Scope,
		ObservedAt:     s.ObservedAt,
		Payload:        s.Payload,
		Confidence:     s.Confidence,
		AgentId:        s.Provenance.AgentID,
		MemoryId:       s.Provenance.MemoryID,
		Status:         governanceStatusToPB(s.Status),
		CreatedAt:      s.Provenance.CreatedAt,
		Metadata:       s.Metadata,
		ClusterId:      s.ClusterID,
		ConditionRef:   s.ConditionRef,
		Severity:       s.Severity,
		AuthorityLevel: authorityLevelToPB(s.AuthorityLevel),
	}
}

func evidenceToPB(e api.Evidence) *behavioralpb.Evidence {
	return &behavioralpb.Evidence{
		Id:             e.ID,
		Project:        e.Project,
		Domain:         string(e.Domain),
		TargetKind:     e.TargetKind,
		TargetId:       e.TargetID,
		EvidenceKind:   e.Kind,
		Lane:           evidenceLaneToPB(e.Lane),
		Result:         e.Result,
		ProbeRef:       e.ProbeRef,
		ObservedAt:     e.ObservedAt,
		Payload:        e.Payload,
		Provenance:     e.Provenance.SourceRef,
		CreatedAt:      e.Provenance.CreatedAt,
		Metadata:       e.Metadata,
		ObservedFrom:   e.ObservedFrom,
		Satisfies:      refsToStrings(e.Satisfies),
		SourceKind:     e.SourceKind,
		SourceRef:      e.SourceRef,
		EntityRef:      e.EntityRef,
		ClusterId:      e.ClusterID,
		ConditionRef:   e.ConditionRef,
		Severity:       e.Severity,
		AuthorityLevel: authorityLevelToPB(e.AuthorityLevel),
	}
}

func governanceStatusToPB(s api.GovernanceStatus) behavioralpb.GovernanceStatus {
	switch s {
	case api.StatusRawSignal:
		return behavioralpb.GovernanceStatus_RAW_SIGNAL
	case api.StatusExtractedClaim:
		return behavioralpb.GovernanceStatus_EXTRACTED_CLAIM
	case api.StatusCandidateFact:
		return behavioralpb.GovernanceStatus_CANDIDATE_FACT
	case api.StatusEvidenceLinked:
		return behavioralpb.GovernanceStatus_EVIDENCE_LINKED
	case api.StatusAuthorityMapped:
		return behavioralpb.GovernanceStatus_AUTHORITY_MAPPED
	case api.StatusConditionScoped:
		return behavioralpb.GovernanceStatus_CONDITION_SCOPED
	case api.StatusContradictionTested:
		return behavioralpb.GovernanceStatus_CONTRADICTION_TESTED
	case api.StatusProposedPrinciple:
		return behavioralpb.GovernanceStatus_PROPOSED_PRINCIPLE
	case api.StatusPromotedPrinciple:
		return behavioralpb.GovernanceStatus_PROMOTED_PRINCIPLE
	case api.StatusRevoked:
		return behavioralpb.GovernanceStatus_REVOKED
	case api.StatusSuperseded:
		return behavioralpb.GovernanceStatus_SUPERSEDED
	case api.StatusNarrowed:
		return behavioralpb.GovernanceStatus_NARROWED
	default:
		return behavioralpb.GovernanceStatus_GOVERNANCE_STATUS_UNSPECIFIED
	}
}

func signalKindToPB(k api.SignalKind) behavioralpb.SignalKind {
	switch k {
	case api.SignalObservedRuntimeFact:
		return behavioralpb.SignalKind_SIGNAL_OBSERVED_RUNTIME_FACT
	case api.SignalAgentInterpretation:
		return behavioralpb.SignalKind_SIGNAL_AGENT_INTERPRETATION
	case api.SignalHumanCorrection:
		return behavioralpb.SignalKind_SIGNAL_HUMAN_CORRECTION
	case api.SignalAutomatedHealth:
		return behavioralpb.SignalKind_SIGNAL_AUTOMATED_HEALTH
	case api.SignalHistoricalMemory:
		return behavioralpb.SignalKind_SIGNAL_HISTORICAL_MEMORY
	case api.SignalPromotedPrinciple:
		return behavioralpb.SignalKind_SIGNAL_PROMOTED_PRINCIPLE
	default:
		return behavioralpb.SignalKind_SIGNAL_KIND_UNSPECIFIED
	}
}

func evidenceLaneToPB(l api.EvidenceLane) behavioralpb.EvidenceLaneMode {
	switch l {
	case api.LaneStaticOnly:
		return behavioralpb.EvidenceLaneMode_EVIDENCE_LANE_STATIC_ONLY
	case api.LaneRuntimeRequired:
		return behavioralpb.EvidenceLaneMode_EVIDENCE_LANE_RUNTIME_REQUIRED
	case api.LaneHybrid:
		return behavioralpb.EvidenceLaneMode_EVIDENCE_LANE_HYBRID
	default:
		return behavioralpb.EvidenceLaneMode_EVIDENCE_LANE_MODE_UNSPECIFIED
	}
}

func authorityLevelToPB(l api.ObservationAuthorityLevel) behavioralpb.ObservationAuthorityLevel {
	switch l {
	case api.ObservationAuthorityInterpretation:
		return behavioralpb.ObservationAuthorityLevel_OBSERVATION_AUTHORITY_LEVEL_INTERPRETATION
	case api.ObservationAuthorityEventStream:
		return behavioralpb.ObservationAuthorityLevel_OBSERVATION_AUTHORITY_LEVEL_EVENT_STREAM
	case api.ObservationAuthorityDiagnostic:
		return behavioralpb.ObservationAuthorityLevel_OBSERVATION_AUTHORITY_LEVEL_DIAGNOSTIC_CLAIM
	case api.ObservationAuthorityDerived:
		return behavioralpb.ObservationAuthorityLevel_OBSERVATION_AUTHORITY_LEVEL_DERIVED_EVIDENCE
	case api.ObservationAuthorityTruthPlane:
		return behavioralpb.ObservationAuthorityLevel_OBSERVATION_AUTHORITY_LEVEL_TRUTH_PLANE
	default:
		return behavioralpb.ObservationAuthorityLevel_OBSERVATION_AUTHORITY_LEVEL_UNSPECIFIED
	}
}

func refsToStrings[T ~string](in []T) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = string(v)
	}
	return out
}
