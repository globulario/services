package main

import (
	"strings"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/evidence"
)

// evidenceToProvenance maps a doctor proto Evidence entry to the generic
// evidence.Provenance used by the trust classifier. The Source is inferred
// from the writer service so consumers can override granular sources later
// (e.g. distinguishing controller-snapshot from verifier-attestation calls).
func evidenceToProvenance(ev *cluster_doctorpb.Evidence) evidence.Provenance {
	if ev == nil {
		return evidence.Provenance{}
	}
	p := evidence.Provenance{
		WriterID: ev.GetSourceService(),
		Source:   inferEvidenceSource(ev.GetSourceService(), ev.GetSourceRpc()),
	}
	if ts := ev.GetTimestamp(); ts != nil {
		p.ObservedAt = ts.AsTime()
	}
	if cid, ok := ev.GetKeyValues()["correlation_id"]; ok {
		p.CorrelationID = cid
	}
	return p
}

// inferEvidenceSource picks the most specific evidence.Source given a writer
// service name and RPC. Unknown writers fall through to SourceInferred so
// the classifier downgrades them aggressively — silence is not freshness.
func inferEvidenceSource(service, rpc string) evidence.Source {
	s := strings.ToLower(strings.TrimSpace(service))
	r := strings.ToLower(strings.TrimSpace(rpc))
	switch {
	case s == "etcd" || strings.Contains(r, "etcd"):
		return evidence.SourceEtcdContract
	case s == "workflow" || strings.HasPrefix(r, "workflow"):
		return evidence.SourceWorkflowReceipt
	case s == "verifier" || strings.Contains(r, "verifier") || strings.Contains(r, "attest"):
		return evidence.SourceVerifierAttestation
	case s == "cluster_controller" || s == "cluster-controller":
		return evidence.SourceControllerSnapshot
	case s == "node_agent" || s == "node-agent":
		return evidence.SourceServiceLog
	case strings.Contains(s, "prometheus") || strings.Contains(s, "telemetry") || strings.Contains(s, "metric"):
		return evidence.SourceTelemetry
	case s == "operator" || s == "user":
		return evidence.SourceOperatorInput
	case s == "":
		return evidence.Source("")
	default:
		return evidence.SourceInferred
	}
}

// findingEvidenceTrust classifies each Evidence entry on a finding and
// returns the worst trust level across all entries. A finding with no
// evidence is Untrusted (silence is not freshness).
func findingEvidenceTrust(f rules.Finding, now time.Time) evidence.TrustLevel {
	if len(f.Evidence) == 0 {
		return evidence.TrustUntrusted
	}
	levels := make([]evidence.TrustLevel, 0, len(f.Evidence))
	for _, ev := range f.Evidence {
		levels = append(levels, evidence.Classify(evidenceToProvenance(ev), now))
	}
	return evidence.Worst(levels...)
}
