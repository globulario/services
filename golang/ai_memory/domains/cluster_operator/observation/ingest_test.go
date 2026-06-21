package observation

import (
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	ai_watcherpb "github.com/globulario/services/golang/ai_watcher/ai_watcherpb"
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestFromDoctorFindingPreservesDiagnosticAuthority(t *testing.T) {
	b := FromDoctorFinding("globular-services", api.DomainRef("cluster_operator"), "c-1", &cluster_doctorpb.ReportHeader{
		Source: "cluster-doctor", SnapshotId: "snap-1", ObservedAt: timestamppb.Now(),
	}, &cluster_doctorpb.Finding{
		FindingId: "finding-1", InvariantId: "policy.embed.stale_grant", Severity: cluster_doctorpb.Severity_SEVERITY_ERROR,
		EntityRef: "cluster_controller", Summary: "stale grant detected",
		Evidence: []*cluster_doctorpb.Evidence{{SourceService: "cluster_doctor", SourceRpc: "ClusterReport", Timestamp: timestamppb.Now()}},
	})
	if b.Signal.AuthorityLevel != api.ObservationAuthorityDiagnostic {
		t.Fatalf("signal authority=%q, want diagnostic", b.Signal.AuthorityLevel)
	}
	if len(b.Evidence) != 1 || b.Evidence[0].AuthorityLevel != api.ObservationAuthorityDerived {
		t.Fatalf("doctor evidence authority=%v", b.Evidence)
	}
	if b.Evidence[0].TargetKind != "signal" || b.Evidence[0].TargetID != b.Signal.ID {
		t.Fatalf("doctor evidence target=%q/%q", b.Evidence[0].TargetKind, b.Evidence[0].TargetID)
	}
}

func TestFromInfraProbePreservesTruthPlaneAuthority(t *testing.T) {
	b := FromInfraProbe("globular-services", api.DomainRef("cluster_operator"), "c-1", &cluster_controllerpb.InfraProbeResult{
		Component: "scylladb", NodeId: "node-a", Healthy: false, ConfigValid: false, Summary: "group0 unavailable",
		ProbedAtUnix: 42, Violations: []*cluster_controllerpb.InfraViolation{{Id: "scylla.group0.quorum_loss", Severity: "CRITICAL"}},
	})
	if b.Signal.AuthorityLevel != api.ObservationAuthorityTruthPlane {
		t.Fatalf("signal authority=%q, want truth-plane", b.Signal.AuthorityLevel)
	}
	if len(b.Evidence) != 1 || b.Evidence[0].AuthorityLevel != api.ObservationAuthorityTruthPlane {
		t.Fatalf("probe evidence authority=%v", b.Evidence)
	}
	if b.Signal.ConditionRef != "scylla.group0.quorum_loss" {
		t.Fatalf("condition_ref=%q", b.Signal.ConditionRef)
	}
}

func TestFromWatcherIncidentPreservesEventStreamAuthority(t *testing.T) {
	b := FromWatcherIncident("globular-services", api.DomainRef("cluster_operator"), "c-1", &ai_watcherpb.Incident{
		Id: "inc-1", TriggerEvent: "cluster.scylla.group0.error", Diagnosis: "event storm", DetectedAt: 99,
		Tier: ai_watcherpb.PermissionTier_AUTO_REMEDIATE, Metadata: map[string]string{"entity_ref": "node-a/scylladb"},
	})
	if b.Signal.AuthorityLevel != api.ObservationAuthorityEventStream {
		t.Fatalf("signal authority=%q, want event-stream", b.Signal.AuthorityLevel)
	}
	if b.Signal.SourceKind != SourceKindAIWatcherIncident {
		t.Fatalf("source_kind=%q", b.Signal.SourceKind)
	}
	if len(b.Evidence) != 0 {
		t.Fatalf("watcher incident should not emit evidence by default")
	}
}
