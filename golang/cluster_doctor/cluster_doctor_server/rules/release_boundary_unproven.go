package rules

// release_boundary_unproven.go — PR-19: surface release boundaries that are not
// fully PROVEN as operator-facing doctor findings.
//
// This is a diagnostic only (cluster_doctor.observer_only_never_writes_etcd):
// it reads the release-boundary reports the collector produced via the shared
// boundarycheck verifier and turns any non-PROVEN verdict into a Finding. It
// proposes NO remediation step that mutates state — proving a boundary is the
// operator's signal, not a trigger for auto-repair.
//
// Verdict → finding mapping (PROVEN emits nothing — the boundary is whole):
//   FAILED         → INVARIANT_FAIL    (real drift: installed/runtime ≠ published)
//   INDETERMINATE  → INVARIANT_UNKNOWN (CheckError; missing/ambiguous evidence)
//   NOT_APPLICABLE → INVARIANT_UNKNOWN (CheckError; wrapper/unhashable, never OK)

import (
	"fmt"
	"sort"
	"strings"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/release_boundary"
)

type releaseBoundaryUnproven struct{}

func (releaseBoundaryUnproven) ID() string       { return "release.boundary_unproven" }
func (releaseBoundaryUnproven) Category() string { return "artifact" }
func (releaseBoundaryUnproven) Scope() string    { return "node" }

func (releaseBoundaryUnproven) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	if len(snap.ReleaseBoundaryReports) == 0 {
		return nil
	}

	// Deterministic order for stable finding output.
	keys := make([]string, 0, len(snap.ReleaseBoundaryReports))
	for k := range snap.ReleaseBoundaryReports {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var findings []Finding
	for _, k := range keys {
		rb := snap.ReleaseBoundaryReports[k]
		if rb == nil || rb.Report.Verdict == release_boundary.VerdictProven {
			continue // PROVEN (or absent) → the boundary is whole; no finding.
		}
		findings = append(findings, releaseBoundaryFinding(rb))
	}
	return findings
}

func releaseBoundaryFinding(rb *collector.ReleaseBoundaryReport) Finding {
	rep := rb.Report

	severity := cluster_doctorpb.Severity_SEVERITY_WARN
	status := cluster_doctorpb.InvariantStatus_INVARIANT_UNKNOWN
	checkError := ""
	switch rep.Verdict {
	case release_boundary.VerdictFailed:
		// A reachable truth source proved a mismatch — real, actionable drift.
		severity = cluster_doctorpb.Severity_SEVERITY_ERROR
		status = cluster_doctorpb.InvariantStatus_INVARIANT_FAIL
	case release_boundary.VerdictIndeterminate:
		// Missing/ambiguous evidence — indeterminate, must not count as FAIL.
		severity = cluster_doctorpb.Severity_SEVERITY_WARN
		status = cluster_doctorpb.InvariantStatus_INVARIANT_UNKNOWN
		checkError = "release boundary indeterminate — evidence incomplete"
	case release_boundary.VerdictNotApplicable:
		// Wrapper/unhashable package — not verifiable, but never reported OK.
		severity = cluster_doctorpb.Severity_SEVERITY_INFO
		status = cluster_doctorpb.InvariantStatus_INVARIANT_UNKNOWN
		checkError = "release boundary not applicable (wrapper/unhashable package)"
	}

	// Per-assertion evidence (A0..A4 verdicts + reasons).
	ev := map[string]string{
		"service":  rep.ServiceName,
		"node":     rep.NodeName,
		"verdict":  string(rep.Verdict),
		"build_id": rep.BuildID,
		"checksum": rep.Checksum,
	}
	if rb.ProvenanceGitSHA != "" {
		ev["provenance_git_sha"] = rb.ProvenanceGitSHA
	}
	var failed []string
	for _, a := range rep.Assertions {
		ev[string(a.ID)] = string(a.Verdict)
		ev[string(a.ID)+"_reason"] = a.Reason
		if a.Verdict != release_boundary.VerdictProven {
			failed = append(failed, fmt.Sprintf("%s=%s", a.ID, a.Verdict))
		}
	}
	for src, msg := range rb.CollectionErrors {
		ev["collection_error."+src] = msg
	}

	summary := fmt.Sprintf("Release boundary for %s on %s is %s (%s). "+
		"The published artifact (build_id %s) is not fully proven to match what is installed and running.",
		rep.ServiceName, rep.NodeName, rep.Verdict, strings.Join(failed, ", "), shortID(rep.BuildID))

	return Finding{
		FindingID:       FindingID("release.boundary_unproven", rep.NodeName, rep.ServiceName),
		InvariantID:     "release.boundary_unproven",
		Severity:        severity,
		Category:        "artifact",
		EntityRef:       rep.ServiceName + "@" + rep.NodeName,
		Summary:         summary,
		Evidence:        []*cluster_doctorpb.Evidence{kvEvidence("cluster_doctor", "release_verify_boundary", ev)},
		InvariantStatus: status,
		CheckError:      checkError,
	}
}

func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}
