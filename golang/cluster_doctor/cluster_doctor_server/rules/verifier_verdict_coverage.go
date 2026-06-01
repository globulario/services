// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.rules
// @awareness file_role=verifier_verdict_coverage_rule
// @awareness implements=globular.platform:intent.runtime.identity_requires_verification
// @awareness risk=high
package rules

// verifier_verdict_coverage.go — invariant that detects installed services
// with no verifier verdict.
//
// Root cause of INC-2026-0008: when a ServiceRelease is transiently FAILED
// (e.g. the repository hasn't synced yet after a platform-upgrade), the
// catch-up pass in runVerification used to skip the service because it had no
// DesiredServiceTarget. The verifier wrote no verdict. After the 5-minute
// grace window expired the health handler showed proof:UNVERIFIED even though
// the service was running correctly.
//
// v1.2.87 fixed the catch-up pass to build a minimal target from the installed
// package when no desired target exists. This invariant is the regression gate:
// if any installed SERVICE-kind package produces no verifier verdict in the
// current sweep, it fires — giving immediate visibility instead of a silent
// degradation that only surfaces in the health UI.

import (
	"fmt"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/verifier"
)

type verifierVerdictCoverage struct{}

func (verifierVerdictCoverage) ID() string       { return "verifier.verdict_coverage" }
func (verifierVerdictCoverage) Category() string { return "diagnostic" }
func (verifierVerdictCoverage) Scope() string    { return "service" }

func (verifierVerdictCoverage) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	if snap == nil || snap.VerifierResult == nil {
		return nil
	}

	// Build the set of (nodeID, service) pairs that DO have a verdict this sweep.
	covered := make(map[string]bool)
	for _, v := range snap.VerifierResult.Verdicts {
		covered[v.Target.NodeID+"/"+v.Target.Service] = true
	}

	// For every node's installed SERVICE-kind packages, check coverage.
	// We only check SERVICE kind — INFRASTRUCTURE and COMMAND packages either
	// have no systemd unit (COMMAND) or are managed by the infra release
	// pipeline which has its own path. Wrapping is handled by the existing
	// verifier logic.
	bootstrapNodes := bootstrappingNodeSet(snap)

	var out []Finding
	for nodeID, kinds := range snap.NodePackageKinds {
		if bootstrapNodes[nodeID] {
			continue // node still bootstrapping; verdicts are expected to be absent
		}
		for svcName, kind := range kinds {
			if kind != "SERVICE" {
				continue
			}
			key := nodeID + "/" + svcName
			if covered[key] {
				continue // verdict present — good
			}
			out = append(out, Finding{
				FindingID:   FindingID("verifier.verdict_coverage.missing", key, svcName),
				InvariantID: "verifier.verdict_coverage",
				Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
				Category:    "diagnostic.runtime",
				EntityRef:   key,
				Summary: fmt.Sprintf("[%s] %s: no verifier verdict produced this sweep — "+
					"ServiceRelease may be FAILED or catch-up pass skipped this service",
					shortNodeID(nodeID), svcName),
				Evidence: []*cluster_doctorpb.Evidence{
					kvEvidence("verifier", "verdict_coverage", map[string]string{
						"node_id":      nodeID,
						"service":      svcName,
						"kind":         kind,
						"covered_keys": fmt.Sprintf("%d", len(covered)),
						"finding":      verifier.FindingRuntimeIdentityUnproven,
					}),
				},
				InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
			})
		}
	}
	return out
}
