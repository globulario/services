package main

import (
	"testing"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/evidence"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestInferEvidenceSource_InstanceQualifiedNodeAgent(t *testing.T) {
	for _, service := range []string{"node_agent@n1", "node-agent@n2"} {
		if got := inferEvidenceSource(service, "GetInventory"); got != evidence.SourceServiceLog {
			t.Fatalf("inferEvidenceSource(%q) = %q, want %q", service, got, evidence.SourceServiceLog)
		}
	}
}

func scopedHarvestEvidence(now time.Time) []*cluster_doctorpb.Evidence {
	return []*cluster_doctorpb.Evidence{
		{
			SourceService: "node_agent@n1",
			SourceRpc:     "GetInventory",
			Timestamp:     timestamppb.New(now),
			KeyValues: map[string]string{
				"node_id":   "n1",
				"unit_name": "globular-torrent.service",
			},
		},
		{
			SourceService: "cluster_doctor",
			SourceRpc:     "reduced_harvest",
			Timestamp:     timestamppb.New(now),
			KeyValues: map[string]string{
				"missing_sources": "workflow.ListWorkflowSummaries",
			},
		},
	}
}

func TestFindingEvidenceTrust_AllowsConclusiveScopedReducedHarvest(t *testing.T) {
	now := time.Now()
	f := rules.Finding{
		InvariantID:     "node.systemd.units_running",
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		Evidence:        scopedHarvestEvidence(now),
	}
	if got := findingEvidenceTrust(f, now); got != evidence.TrustAuthoritative {
		t.Fatalf("trust = %q, want AUTHORITATIVE for fresh target evidence with only unrelated harvest failures", got)
	}
}

func TestFindingEvidenceTrust_RejectsCompromisedReducedHarvest(t *testing.T) {
	now := time.Now()
	f := rules.Finding{
		InvariantID:     "node.systemd.units_running",
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_UNKNOWN,
		CheckError:      "verdict downgraded to UNKNOWN: target GetInventory unavailable",
		Evidence:        scopedHarvestEvidence(now),
	}
	if got := findingEvidenceTrust(f, now); got != evidence.TrustUntrusted {
		t.Fatalf("trust = %q, want UNTRUSTED when target evidence closure failed", got)
	}
}

func TestFindingEvidenceTrust_RejectsFullHarvestUnknownVerdict(t *testing.T) {
	now := time.Now()
	f := rules.Finding{
		InvariantID:     "node.systemd.units_running",
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_UNKNOWN,
		Evidence: []*cluster_doctorpb.Evidence{{
			SourceService: "node_agent@n1",
			SourceRpc:     "GetInventory",
			Timestamp:     timestamppb.New(now),
		}},
	}
	if got := findingEvidenceTrust(f, now); got != evidence.TrustUntrusted {
		t.Fatalf("trust = %q, want UNTRUSTED: fresh evidence cannot turn UNKNOWN into mutation authority", got)
	}
}

func TestFindingEvidenceTrust_RejectsFailWithCheckError(t *testing.T) {
	now := time.Now()
	f := rules.Finding{
		InvariantID:     "node.systemd.units_running",
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		CheckError:      "inventory response was incomplete",
		Evidence: []*cluster_doctorpb.Evidence{{
			SourceService: "node_agent@n1",
			SourceRpc:     "GetInventory",
			Timestamp:     timestamppb.New(now),
		}},
	}
	if got := findingEvidenceTrust(f, now); got != evidence.TrustUntrusted {
		t.Fatalf("trust = %q, want UNTRUSTED: CheckError must override a nominal FAIL", got)
	}
}