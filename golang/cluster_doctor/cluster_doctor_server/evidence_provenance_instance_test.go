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

// The three cases below assert that an inconclusive verdict cannot authorize a
// mutation. That is verdict closure, not evidence trust: the evidence in each is
// fresh and authoritative, and it is the *finding* that is not conclusive. They
// are therefore asked of rules.RemediationEvidenceClosure, which owns that
// question, rather than of findingEvidenceTrust, which answers only how good the
// evidence is. Merging the two made a fresh finding report UNTRUSTED and left
// callers unable to tell which gate refused them.
func TestRemediationClosure_RejectsCompromisedReducedHarvest(t *testing.T) {
	now := time.Now()
	f := rules.Finding{
		InvariantID:     "node.systemd.units_running",
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_UNKNOWN,
		CheckError:      "verdict downgraded to UNKNOWN: target GetInventory unavailable",
		Evidence:        scopedHarvestEvidence(now),
	}
	if eligible, why := rules.RemediationEvidenceClosure(f); eligible {
		t.Fatal("a finding whose target evidence closure failed authorized a mutation")
	} else if why == "" {
		t.Fatal("refusal gave no reason")
	}
	// The evidence itself is fresh, so evidence trust must not be the refusing gate.
	if got := findingEvidenceTrust(f, now); got != evidence.TrustAuthoritative {
		t.Fatalf("evidence trust = %q, want AUTHORITATIVE: the verdict is what is inconclusive, not the evidence", got)
	}
}

func TestRemediationClosure_RejectsFullHarvestUnknownVerdict(t *testing.T) {
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
	if eligible, _ := rules.RemediationEvidenceClosure(f); eligible {
		t.Fatal("fresh evidence turned an UNKNOWN verdict into mutation authority")
	}
	if got := findingEvidenceTrust(f, now); got != evidence.TrustAuthoritative {
		t.Fatalf("evidence trust = %q, want AUTHORITATIVE: the evidence is fresh; the verdict is UNKNOWN", got)
	}
}

func TestRemediationClosure_RejectsFailWithCheckError(t *testing.T) {
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
	if eligible, _ := rules.RemediationEvidenceClosure(f); eligible {
		t.Fatal("a CheckError did not override a nominal FAIL")
	}
	if got := findingEvidenceTrust(f, now); got != evidence.TrustAuthoritative {
		t.Fatalf("evidence trust = %q, want AUTHORITATIVE: the evidence is fresh; the check errored", got)
	}
}
