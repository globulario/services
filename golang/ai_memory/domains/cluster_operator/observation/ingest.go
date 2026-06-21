package observation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	ai_watcherpb "github.com/globulario/services/golang/ai_watcher/ai_watcherpb"
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

const (
	SourceKindClusterDoctorFinding  = "cluster_doctor_finding"
	SourceKindClusterDoctorEvidence = "cluster_doctor_evidence"
	SourceKindInfraProbeTruthPlane  = "infra_probe_truth_plane"
	SourceKindAIWatcherIncident     = "ai_watcher_incident"
)

type Bundle struct {
	Signal   api.Signal
	Evidence []api.Evidence
}

func stableID(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:8])
}

func marshalPayload(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

func doctorSeverity(s cluster_doctorpb.Severity) string {
	switch s {
	case cluster_doctorpb.Severity_SEVERITY_INFO:
		return "info"
	case cluster_doctorpb.Severity_SEVERITY_WARN:
		return "warning"
	case cluster_doctorpb.Severity_SEVERITY_ERROR:
		return "error"
	case cluster_doctorpb.Severity_SEVERITY_CRITICAL:
		return "critical"
	default:
		return "unknown"
	}
}

func headerObservedAt(h *cluster_doctorpb.ReportHeader) int64 {
	if h == nil || h.GetObservedAt() == nil {
		return 0
	}
	return h.GetObservedAt().GetSeconds()
}

func FromDoctorFinding(project string, domain api.DomainRef, clusterID string, header *cluster_doctorpb.ReportHeader, finding *cluster_doctorpb.Finding) Bundle {
	if finding == nil {
		return Bundle{}
	}
	signalID := "signal.doctor." + stableID(project, string(domain), clusterID, finding.GetFindingId(), finding.GetInvariantId())
	sig := api.Signal{
		ID:             signalID,
		Project:        project,
		Domain:         domain,
		Kind:           api.SignalAutomatedHealth,
		SourceKind:     SourceKindClusterDoctorFinding,
		SourceRef:      finding.GetFindingId(),
		EntityRef:      finding.GetEntityRef(),
		Scope:          clusterID,
		ClusterID:      clusterID,
		ConditionRef:   finding.GetInvariantId(),
		Severity:       doctorSeverity(finding.GetSeverity()),
		AuthorityLevel: api.ObservationAuthorityDiagnostic,
		ObservedAt:     headerObservedAt(header),
		Payload:        finding.GetSummary(),
		Status:         api.StatusRawSignal,
		Metadata: map[string]string{
			"finding_id":         finding.GetFindingId(),
			"invariant_id":       finding.GetInvariantId(),
			"doctor_category":    finding.GetCategory(),
			"doctor_source":      header.GetSource(),
			"doctor_snapshot_id": header.GetSnapshotId(),
		},
	}
	out := Bundle{Signal: sig}
	for i, ev := range finding.GetEvidence() {
		out.Evidence = append(out.Evidence, api.Evidence{
			ID:             fmt.Sprintf("%s.evidence.%d", signalID, i),
			Project:        project,
			Domain:         domain,
			TargetKind:     "signal",
			TargetID:       signalID,
			Kind:           SourceKindClusterDoctorEvidence,
			Lane:           api.LaneRuntimeRequired,
			Result:         "claim",
			SourceKind:     SourceKindClusterDoctorEvidence,
			SourceRef:      finding.GetFindingId(),
			EntityRef:      finding.GetEntityRef(),
			ClusterID:      clusterID,
			ConditionRef:   finding.GetInvariantId(),
			Severity:       doctorSeverity(finding.GetSeverity()),
			AuthorityLevel: api.ObservationAuthorityDerived,
			ObservedAt:     ev.GetTimestamp().GetSeconds(),
			Payload:        marshalPayload(ev),
			ObservedFrom:   signalID,
			Provenance:     api.Provenance{SourceRef: ev.GetSourceRpc(), CreatedAt: ev.GetTimestamp().GetSeconds()},
		})
	}
	return out
}

func probeSeverity(p *cluster_controllerpb.InfraProbeResult) string {
	if p == nil {
		return "unknown"
	}
	for _, v := range p.GetViolations() {
		s := strings.ToLower(v.GetSeverity())
		switch s {
		case "critical":
			return s
		case "error":
			if s != "critical" {
				return s
			}
		case "warn":
			return "warning"
		}
	}
	if len(p.GetErrors()) > 0 {
		return "error"
	}
	if p.GetHealthy() && p.GetConfigValid() {
		return "info"
	}
	return "warning"
}

func probeCondition(p *cluster_controllerpb.InfraProbeResult) string {
	if p == nil {
		return ""
	}
	if vs := p.GetViolations(); len(vs) > 0 {
		return vs[0].GetId()
	}
	return ""
}

func FromInfraProbe(project string, domain api.DomainRef, clusterID string, probe *cluster_controllerpb.InfraProbeResult) Bundle {
	if probe == nil {
		return Bundle{}
	}
	sourceRef := fmt.Sprintf("%s:%s:%d", probe.GetComponent(), probe.GetNodeId(), probe.GetProbedAtUnix())
	signalID := "signal.probe." + stableID(project, string(domain), clusterID, sourceRef)
	sig := api.Signal{
		ID:             signalID,
		Project:        project,
		Domain:         domain,
		Kind:           api.SignalObservedRuntimeFact,
		SourceKind:     SourceKindInfraProbeTruthPlane,
		SourceRef:      sourceRef,
		EntityRef:      probe.GetNodeId() + "/" + probe.GetComponent(),
		Scope:          clusterID,
		ClusterID:      clusterID,
		ConditionRef:   probeCondition(probe),
		Severity:       probeSeverity(probe),
		AuthorityLevel: api.ObservationAuthorityTruthPlane,
		ObservedAt:     probe.GetProbedAtUnix(),
		Payload:        probe.GetSummary(),
		Status:         api.StatusRawSignal,
		Metadata: map[string]string{
			"component":      probe.GetComponent(),
			"node_id":        probe.GetNodeId(),
			"probe_stale":    fmt.Sprintf("%t", probe.GetProbeStale()),
			"probe_age_secs": fmt.Sprintf("%d", probe.GetProbeAgeSeconds()),
			"probe_errors":   strings.Join(probe.GetErrors(), ";"),
		},
	}
	return Bundle{
		Signal: sig,
		Evidence: []api.Evidence{{
			ID:             signalID + ".evidence.0",
			Project:        project,
			Domain:         domain,
			TargetKind:     "signal",
			TargetID:       signalID,
			Kind:           "probe",
			Lane:           api.LaneRuntimeRequired,
			Result:         "observed",
			ProbeRef:       probe.GetComponent(),
			SourceKind:     SourceKindInfraProbeTruthPlane,
			SourceRef:      sourceRef,
			EntityRef:      probe.GetNodeId() + "/" + probe.GetComponent(),
			ClusterID:      clusterID,
			ConditionRef:   probeCondition(probe),
			Severity:       probeSeverity(probe),
			AuthorityLevel: api.ObservationAuthorityTruthPlane,
			ObservedAt:     probe.GetProbedAtUnix(),
			Payload:        marshalPayload(probe),
			ObservedFrom:   signalID,
			Provenance:     api.Provenance{SourceRef: sourceRef, CreatedAt: probe.GetProbedAtUnix()},
		}},
	}
}

func watcherSeverity(inc *ai_watcherpb.Incident) string {
	if inc == nil {
		return "unknown"
	}
	if s := strings.ToLower(strings.TrimSpace(inc.GetMetadata()["severity"])); s != "" {
		return s
	}
	switch inc.GetTier() {
	case ai_watcherpb.PermissionTier_REQUIRE_APPROVAL:
		return "critical"
	case ai_watcherpb.PermissionTier_AUTO_REMEDIATE:
		return "error"
	default:
		return "warning"
	}
}

func FromWatcherIncident(project string, domain api.DomainRef, clusterID string, inc *ai_watcherpb.Incident) Bundle {
	if inc == nil {
		return Bundle{}
	}
	entityRef := inc.GetMetadata()["entity_ref"]
	if entityRef == "" {
		entityRef = inc.GetTriggerEvent()
	}
	sig := api.Signal{
		ID:             "signal.watcher." + stableID(project, string(domain), clusterID, inc.GetId()),
		Project:        project,
		Domain:         domain,
		Kind:           api.SignalAutomatedHealth,
		SourceKind:     SourceKindAIWatcherIncident,
		SourceRef:      inc.GetId(),
		EntityRef:      entityRef,
		Scope:          clusterID,
		ClusterID:      clusterID,
		ConditionRef:   inc.GetTriggerEvent(),
		Severity:       watcherSeverity(inc),
		AuthorityLevel: api.ObservationAuthorityEventStream,
		ObservedAt:     inc.GetDetectedAt(),
		Payload:        inc.GetDiagnosis(),
		Status:         api.StatusRawSignal,
		Metadata: map[string]string{
			"incident_id":     inc.GetId(),
			"trigger_event":   inc.GetTriggerEvent(),
			"watcher_status":  inc.GetStatus().String(),
			"watcher_tier":    inc.GetTier().String(),
			"proposed_action": inc.GetProposedAction(),
			"action_taken":    inc.GetActionTaken(),
			"result":          inc.GetResult(),
		},
	}
	return Bundle{Signal: sig}
}
