package render

import (
	"sort"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const topIssueCount = 5

// ClusterReport builds a ClusterReport proto from a snapshot and findings.
func ClusterReport(snap *collector.Snapshot, findings []rules.Finding, version string) *cluster_doctorpb.ClusterReport {
	protoFindings := toProtoFindings(findings)
	sortFindingsBySeverity(protoFindings)

	counts := countsByCategory(protoFindings)
	topIDs := topIssueIDs(protoFindings, topIssueCount)
	overall := overallStatus(protoFindings)

	return &cluster_doctorpb.ClusterReport{
		Header:             buildHeader(snap, version),
		OverallStatus:      overall,
		Findings:           protoFindings,
		CountsByCategory:   counts,
		TopIssueIds:        topIDs,
	}
}

func buildHeader(snap *collector.Snapshot, version string) *cluster_doctorpb.ReportHeader {
	h := &cluster_doctorpb.ReportHeader{
		GeneratedAt:    timestamppb.New(snap.GeneratedAt),
		SnapshotId:     snap.SnapshotID,
		GlobularVersion: version,
		DataSources:    snap.DataSources,
		DataIncomplete: snap.DataIncomplete,
	}
	for _, de := range snap.DataErrors {
		h.DataErrors = append(h.DataErrors, &cluster_doctorpb.Evidence{
			SourceService: de.Service,
			SourceRpc:     de.RPC,
			KeyValues:     map[string]string{"error": de.Err.Error()},
			Timestamp:     timestamppb.New(snap.GeneratedAt),
		})
	}
	return h
}

func toProtoFindings(findings []rules.Finding) []*cluster_doctorpb.Finding {
	out := make([]*cluster_doctorpb.Finding, 0, len(findings))
	for _, f := range findings {
		out = append(out, f.ToProto())
	}
	return out
}

func sortFindingsBySeverity(findings []*cluster_doctorpb.Finding) {
	sort.Slice(findings, func(i, j int) bool {
		return findings[i].GetSeverity() > findings[j].GetSeverity()
	})
}

func countsByCategory(findings []*cluster_doctorpb.Finding) map[string]uint32 {
	counts := make(map[string]uint32)
	for _, f := range findings {
		if f.GetInvariantStatus() == cluster_doctorpb.InvariantStatus_INVARIANT_FAIL {
			counts[f.GetCategory()]++
		}
	}
	return counts
}

func topIssueIDs(findings []*cluster_doctorpb.Finding, n int) []string {
	var ids []string
	for _, f := range findings {
		if f.GetInvariantStatus() == cluster_doctorpb.InvariantStatus_INVARIANT_FAIL {
			ids = append(ids, f.GetFindingId())
			if len(ids) >= n {
				break
			}
		}
	}
	return ids
}

func overallStatus(findings []*cluster_doctorpb.Finding) cluster_doctorpb.ClusterStatus {
	status := cluster_doctorpb.ClusterStatus_CLUSTER_HEALTHY
	for _, f := range findings {
		if f.GetInvariantStatus() != cluster_doctorpb.InvariantStatus_INVARIANT_FAIL {
			continue
		}
		switch f.GetSeverity() {
		case cluster_doctorpb.Severity_SEVERITY_CRITICAL:
			return cluster_doctorpb.ClusterStatus_CLUSTER_CRITICAL
		case cluster_doctorpb.Severity_SEVERITY_ERROR:
			if status < cluster_doctorpb.ClusterStatus_CLUSTER_DEGRADED {
				status = cluster_doctorpb.ClusterStatus_CLUSTER_DEGRADED
			}
		case cluster_doctorpb.Severity_SEVERITY_WARN:
			if status < cluster_doctorpb.ClusterStatus_CLUSTER_DEGRADED {
				status = cluster_doctorpb.ClusterStatus_CLUSTER_DEGRADED
			}
		}
	}
	return status
}
