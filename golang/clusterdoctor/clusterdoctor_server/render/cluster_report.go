package render

import (
	"sort"

	clusterdoctorpb "github.com/globulario/services/golang/clusterdoctor/clusterdoctorpb"
	"github.com/globulario/services/golang/clusterdoctor/clusterdoctor_server/collector"
	"github.com/globulario/services/golang/clusterdoctor/clusterdoctor_server/rules"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const topIssueCount = 5

// ClusterReport builds a ClusterReport proto from a snapshot and findings.
func ClusterReport(snap *collector.Snapshot, findings []rules.Finding, version string) *clusterdoctorpb.ClusterReport {
	protoFindings := toProtoFindings(findings)
	sortFindingsBySeverity(protoFindings)

	counts := countsByCategory(protoFindings)
	topIDs := topIssueIDs(protoFindings, topIssueCount)
	overall := overallStatus(protoFindings)

	return &clusterdoctorpb.ClusterReport{
		Header:             buildHeader(snap, version),
		OverallStatus:      overall,
		Findings:           protoFindings,
		CountsByCategory:   counts,
		TopIssueIds:        topIDs,
	}
}

func buildHeader(snap *collector.Snapshot, version string) *clusterdoctorpb.ReportHeader {
	h := &clusterdoctorpb.ReportHeader{
		GeneratedAt:    timestamppb.New(snap.GeneratedAt),
		SnapshotId:     snap.SnapshotID,
		GlobularVersion: version,
		DataSources:    snap.DataSources,
		DataIncomplete: snap.DataIncomplete,
	}
	for _, de := range snap.DataErrors {
		h.DataErrors = append(h.DataErrors, &clusterdoctorpb.Evidence{
			SourceService: de.Service,
			SourceRpc:     de.RPC,
			KeyValues:     map[string]string{"error": de.Err.Error()},
			Timestamp:     timestamppb.New(snap.GeneratedAt),
		})
	}
	return h
}

func toProtoFindings(findings []rules.Finding) []*clusterdoctorpb.Finding {
	out := make([]*clusterdoctorpb.Finding, 0, len(findings))
	for _, f := range findings {
		out = append(out, f.ToProto())
	}
	return out
}

func sortFindingsBySeverity(findings []*clusterdoctorpb.Finding) {
	sort.Slice(findings, func(i, j int) bool {
		return findings[i].GetSeverity() > findings[j].GetSeverity()
	})
}

func countsByCategory(findings []*clusterdoctorpb.Finding) map[string]uint32 {
	counts := make(map[string]uint32)
	for _, f := range findings {
		if f.GetInvariantStatus() == clusterdoctorpb.InvariantStatus_INVARIANT_FAIL {
			counts[f.GetCategory()]++
		}
	}
	return counts
}

func topIssueIDs(findings []*clusterdoctorpb.Finding, n int) []string {
	var ids []string
	for _, f := range findings {
		if f.GetInvariantStatus() == clusterdoctorpb.InvariantStatus_INVARIANT_FAIL {
			ids = append(ids, f.GetFindingId())
			if len(ids) >= n {
				break
			}
		}
	}
	return ids
}

func overallStatus(findings []*clusterdoctorpb.Finding) clusterdoctorpb.ClusterStatus {
	status := clusterdoctorpb.ClusterStatus_CLUSTER_HEALTHY
	for _, f := range findings {
		if f.GetInvariantStatus() != clusterdoctorpb.InvariantStatus_INVARIANT_FAIL {
			continue
		}
		switch f.GetSeverity() {
		case clusterdoctorpb.Severity_SEVERITY_CRITICAL:
			return clusterdoctorpb.ClusterStatus_CLUSTER_CRITICAL
		case clusterdoctorpb.Severity_SEVERITY_ERROR:
			if status < clusterdoctorpb.ClusterStatus_CLUSTER_DEGRADED {
				status = clusterdoctorpb.ClusterStatus_CLUSTER_DEGRADED
			}
		case clusterdoctorpb.Severity_SEVERITY_WARN:
			if status < clusterdoctorpb.ClusterStatus_CLUSTER_DEGRADED {
				status = clusterdoctorpb.ClusterStatus_CLUSTER_DEGRADED
			}
		}
	}
	return status
}
