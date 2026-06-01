// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.rules
// @awareness file_role=repository_findings_surfacing_rule
// @awareness implements=globular.platform:intent.repository.identity_doctor_reports_collisions
// @awareness risk=high
package rules

// repository_findings.go — doctor invariants that pull from the repository
// service's ListRepositoryFindings RPC. The collector populates
// snap.RepositoryFindings; this rule maps each entry to a doctor Finding
// with severity + remediation hints the operator can run directly.

import (
	"fmt"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

type repositoryFindings struct{}

func (repositoryFindings) ID() string       { return "repository.findings" }
func (repositoryFindings) Category() string { return "repository" }
func (repositoryFindings) Scope() string    { return "cluster" }

func (repositoryFindings) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	var findings []Finding
	if snap == nil || len(snap.RepositoryFindings) == 0 {
		return findings
	}
	for _, rf := range snap.RepositoryFindings {
		if rf == nil {
			continue
		}
		invariantID := mapInvariantID(rf.Kind)
		severity := mapRepoSeverity(rf.Severity)
		entityRef := rf.ArtifactKey
		if rf.NodeID != "" {
			entityRef = rf.NodeID + "/" + entityRef
		}
		summary := buildRepoSummary(rf)

		evidence := []*cluster_doctorpb.Evidence{
			kvEvidence("repository", "ListRepositoryFindings", flattenEvidence(rf)),
		}

		findings = append(findings, Finding{
			FindingID:       FindingID(invariantID, entityRef, rf.Reason),
			InvariantID:     invariantID,
			Severity:        severity,
			Category:        "repository",
			EntityRef:       entityRef,
			Summary:         summary,
			Evidence:        evidence,
			Remediation:     repositoryRemediation(rf),
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return findings
}

// mapInvariantID translates the repository kind enum string to a stable
// dotted-id used by the doctor / explain UI.
func mapInvariantID(kind string) string {
	switch kind {
	case "REPO_FIND_PUBLISHED_MISSING_BLOB":
		return "repository.published_missing_blob"
	case "REPO_FIND_PUBLISHED_CHECKSUM_MISMATCH":
		return "repository.published_checksum_mismatch"
	case "REPO_FIND_PUBLISHED_UNSIGNED_REQUIRED":
		return "repository.published_unsigned_required"
	case "REPO_FIND_REVOKED_INSTALLABLE":
		return "repository.revoked_installable"
	case "REPO_FIND_QUARANTINED_INSTALLABLE":
		return "repository.quarantined_installable"
	case "REPO_FIND_CONFIG_CONFLICT":
		return "package.config_conflict"
	case "REPO_FIND_ROLLBACK_FAILED":
		return "package.rollback_failed"
	// Phase 1 hardening: dependency-mode coherence kinds emitted by
	// evalDependencyModeCoherence in repository_findings.go.
	case "REPO_FIND_SCYLLA_DOWN_MODE_INCONSISTENT":
		return "repository.watchdog_inconsistency"
	case "REPO_FIND_MINIO_BLOCKS_REPOSITORY":
		return "repository.watchdog_inconsistency"
	case "REPO_FIND_SOURCE_CHAIN_UNAVAILABLE":
		return "repository.source_chain_unavailable"
	case "REPO_FIND_LOCAL_CACHE_CORRUPTION":
		return "repository.local_cache_corruption"
	}
	return "repository.finding"
}

func mapRepoSeverity(s string) cluster_doctorpb.Severity {
	switch s {
	case "REPO_FIND_CRITICAL":
		return cluster_doctorpb.Severity_SEVERITY_ERROR // doctor's highest severity
	case "REPO_FIND_ERROR":
		return cluster_doctorpb.Severity_SEVERITY_ERROR
	case "REPO_FIND_WARN":
		return cluster_doctorpb.Severity_SEVERITY_WARN
	case "REPO_FIND_INFO":
		return cluster_doctorpb.Severity_SEVERITY_INFO
	}
	return cluster_doctorpb.Severity_SEVERITY_WARN
}

func buildRepoSummary(rf *collector.RepositoryFindingSnapshot) string {
	id := mapInvariantID(rf.Kind)
	pkg := rf.Name
	if rf.PublisherID != "" {
		pkg = rf.PublisherID + "/" + rf.Name
	}
	if rf.Version != "" {
		pkg += "@" + rf.Version
	}
	if rf.Platform != "" {
		pkg += " [" + rf.Platform + "]"
	}
	return fmt.Sprintf("[%s] %s — %s", id, pkg, rf.Reason)
}

func flattenEvidence(rf *collector.RepositoryFindingSnapshot) map[string]string {
	out := make(map[string]string, len(rf.Evidence)+5)
	for k, v := range rf.Evidence {
		out[k] = v
	}
	out["artifact_key"] = rf.ArtifactKey
	out["current_state"] = rf.CurrentState
	out["expected_state"] = rf.ExpectedState
	if rf.NodeID != "" {
		out["node_id"] = rf.NodeID
	}
	return out
}

func repositoryRemediation(rf *collector.RepositoryFindingSnapshot) []*cluster_doctorpb.RemediationStep {
	if rf.RecommendedCommand == "" {
		return nil
	}
	desc := "Run the recommended repository / package command"
	switch mapInvariantID(rf.Kind) {
	case "repository.published_missing_blob":
		desc = "Re-import the missing blob from upstream"
	case "repository.published_checksum_mismatch":
		desc = "Repair the corrupted blob from upstream"
	case "repository.published_unsigned_required":
		desc = "Register a trusted publisher signature, or quarantine the artifact"
	case "repository.revoked_installable", "repository.quarantined_installable":
		desc = "Stamp pipeline_state coherently with publish_state"
	case "package.config_conflict":
		desc = "Resolve config-file conflicts before retrying upgrade / rollback"
	case "package.rollback_failed":
		desc = "Investigate node-agent + service health; check workflow run history"
	}
	return []*cluster_doctorpb.RemediationStep{
		step(1, desc, rf.RecommendedCommand),
	}
}
