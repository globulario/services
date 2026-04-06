package render

import (
	"sort"
	"time"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const topIssueCount = 5

// ReportSourceName is the stable `source` string stamped into every
// ReportHeader. Callers use this to tell cluster-doctor's reports apart
// from other report-emitting surfaces (ai-watcher, workflow service).
const ReportSourceName = "cluster-doctor"

// Freshness bundles the provenance metadata that buildHeader stamps
// into every report. It mirrors the ReportHeader freshness fields
// one-for-one so the render layer does not have to know about cache
// internals. The server handler fills this from its collector call.
type Freshness struct {
	CacheHit bool
	CacheTTL time.Duration
	Mode     cluster_doctorpb.FreshnessMode
}

// ClusterReport builds a ClusterReport proto from a snapshot and findings.
func ClusterReport(snap *collector.Snapshot, findings []rules.Finding, version string, fresh Freshness) *cluster_doctorpb.ClusterReport {
	protoFindings := toProtoFindings(findings)
	sortFindingsBySeverity(protoFindings)

	counts := countsByCategory(protoFindings)
	topIDs := topIssueIDs(protoFindings, topIssueCount)
	overall := overallStatus(protoFindings)

	return &cluster_doctorpb.ClusterReport{
		Header:             buildHeader(snap, version, fresh),
		OverallStatus:      overall,
		Findings:           protoFindings,
		CountsByCategory:   counts,
		TopIssueIds:        topIDs,
	}
}

func buildHeader(snap *collector.Snapshot, version string, fresh Freshness) *cluster_doctorpb.ReportHeader {
	// Age is computed server-side from the snapshot's own observed_at
	// so callers do not have to reason about clock skew between their
	// clock and the doctor's. If GeneratedAt is zero (shouldn't happen
	// in practice) we report 0 rather than a giant wall-clock delta.
	var ageSeconds int64
	if !snap.GeneratedAt.IsZero() {
		ageSeconds = int64(time.Since(snap.GeneratedAt).Seconds())
		if ageSeconds < 0 {
			ageSeconds = 0
		}
	}
	h := &cluster_doctorpb.ReportHeader{
		GeneratedAt:        timestamppb.New(snap.GeneratedAt),
		SnapshotId:         snap.SnapshotID,
		GlobularVersion:    version,
		DataSources:        snap.DataSources,
		DataIncomplete:     snap.DataIncomplete,
		Source:             ReportSourceName,
		ObservedAt:         timestamppb.New(snap.GeneratedAt),
		SnapshotAgeSeconds: ageSeconds,
		CacheHit:           fresh.CacheHit,
		CacheTtlSeconds:    int64(fresh.CacheTTL.Seconds()),
		FreshnessMode:      fresh.Mode,
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
