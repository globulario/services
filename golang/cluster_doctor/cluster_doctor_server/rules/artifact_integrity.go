package rules

import (
	"fmt"
	"strings"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// artifactIntegrity surfaces findings from the per-node
// VerifyPackageIntegrity RPC as doctor invariants. The collector fetches a
// report from each node_agent; this rule walks the findings and emits one
// doctor Finding per package + invariant pair.
//
// The severity mapping follows the contract in the todo:
//
//   artifact.installed_digest_mismatch   → ERROR
//   artifact.desired_version_mismatch    → WARN
//   artifact.desired_build_mismatch      → WARN
//   artifact.cache_digest_mismatch       → WARN
//   artifact.cache_missing               → INFO
//
// Nodes that have no report (older node_agent binaries without the RPC, or
// dial failures) contribute no findings — the invariant is best-effort and
// silent when data is unavailable.
type artifactIntegrity struct{}

func (artifactIntegrity) ID() string       { return "artifact.integrity" }
func (artifactIntegrity) Category() string { return "artifact" }
func (artifactIntegrity) Scope() string    { return "node" }

func (artifactIntegrity) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	var findings []Finding
	if snap == nil || len(snap.IntegrityReports) == 0 {
		return findings
	}

	for nodeID, report := range snap.IntegrityReports {
		if report == nil {
			continue
		}
		for _, f := range report.Findings {
			severity := severityFromReport(f.Severity)
			// Remediation depends on the specific invariant.
			rem := remediationFor(f.Invariant, nodeID, f.Package, f.Kind)

			findings = append(findings, Finding{
				FindingID:       FindingID(f.Invariant, nodeID+"/"+f.Package, f.Summary),
				InvariantID:     f.Invariant,
				Severity:        severity,
				Category:        "artifact",
				EntityRef:       nodeID + "/" + f.Package,
				Summary:         fmt.Sprintf("[%s] %s/%s: %s", shortNodeID(nodeID), f.Kind, f.Package, f.Summary),
				Evidence:        evidenceFromMap(f.Invariant, f.Evidence),
				Remediation:     rem,
				InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
			})
		}
	}
	return findings
}

// shortNodeID returns the first 8 characters of a node ID for log/summary
// readability. Handles short / non-UUID node IDs without panicking.
func shortNodeID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

// severityFromReport maps the action's string severity to proto enum.
func severityFromReport(s string) cluster_doctorpb.Severity {
	switch strings.ToUpper(s) {
	case "ERROR":
		return cluster_doctorpb.Severity_SEVERITY_ERROR
	case "WARN":
		return cluster_doctorpb.Severity_SEVERITY_WARN
	case "INFO":
		return cluster_doctorpb.Severity_SEVERITY_INFO
	}
	return cluster_doctorpb.Severity_SEVERITY_WARN
}

// evidenceFromMap builds a single Evidence message from a flat map.
func evidenceFromMap(invariantID string, m map[string]string) []*cluster_doctorpb.Evidence {
	if len(m) == 0 {
		return nil
	}
	// Copy so the caller's map is not retained.
	kv := make(map[string]string, len(m))
	for k, v := range m {
		kv[k] = v
	}
	return []*cluster_doctorpb.Evidence{
		kvEvidence("node_agent", "VerifyPackageIntegrity", kv),
	}
}

// remediationFor returns canonical remediation steps per invariant ID.
func remediationFor(invariantID, nodeID, pkg, kind string) []*cluster_doctorpb.RemediationStep {
	switch invariantID {
	case "artifact.installed_digest_mismatch":
		return []*cluster_doctorpb.RemediationStep{
			step(1,
				fmt.Sprintf("Re-install %s via the release pipeline to replace the tampered/stale bytes on node %s",
					pkg, nodeID),
				fmt.Sprintf("globular services desired set %s <version> --build-number <n>", pkg)),
			step(2,
				"If the drift-reconciler does not pick it up, force a direct install via node_agent",
				fmt.Sprintf("globular services verify-integrity --package %s  # confirm, then dispatch", pkg)),
		}
	case "artifact.desired_version_mismatch", "artifact.desired_build_mismatch":
		return []*cluster_doctorpb.RemediationStep{
			step(1,
				fmt.Sprintf("Node %s has an installed %s that differs from desired state. Wait for the drift-reconciler to converge, or trigger it manually.",
					nodeID, pkg),
				"globular cluster reconcile"),
			step(2,
				"If convergence is not progressing, check cluster_controller logs for release pipeline errors",
				"journalctl -u globular-cluster-controller.service -n 200"),
		}
	case "artifact.cache_digest_mismatch":
		return []*cluster_doctorpb.RemediationStep{
			step(1,
				fmt.Sprintf("Cached artifact for %s on node %s does not match the published manifest. The fix is automatic on the next install — the new artifact.fetch validates digests before reuse.",
					pkg, nodeID),
				"globular services verify-integrity --package "+pkg),
			step(2,
				"If the mismatch persists, force a re-download by bumping the desired build or removing the cache",
				"sudo rm /var/lib/globular/staging/core@globular.io/"+pkg+"/latest.artifact"),
		}
	case "artifact.cache_missing":
		return []*cluster_doctorpb.RemediationStep{
			step(1,
				"Informational only — the next install for this package will re-fetch from the repository.",
				""),
		}
	}
	return nil
}
