// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.rules.kind_mismatch
// @awareness file_role=doctor_rule_per_node_per_package_kind_mismatch_blocking_dispatch
// @awareness enforces=globular.platform:invariant.repository.metadata_is_authority
// @awareness risk=high
package rules

// kind_mismatch.go — DIAGNOSTIC ONLY. Fires one finding per
// {node, package} pair where the controller's desired kind does
// not match the artifact kind published in the repository. The
// drift reconciler blocks dispatch for these packages, so
// without operator action they NEVER converge — the per-node
// scope makes the operator-facing fix obvious: correct desired
// for one node, or re-publish the artifact with the right kind.
//
// MUST NOT auto-correct kind on either side. Package kind is
// authoritative truth from the canonical kind registry; an
// auto-rewrite would mean the registry is no longer the
// authority, breaking
// repository.metadata_is_authority.

import (
	"fmt"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// kindMismatchStaleness is the maximum age of a kind mismatch record before
// the doctor considers it resolved. The drift reconciler refreshes records on
// every pass (~30 s), so any record older than this means the mismatch was
// corrected and the package has since been dispatched successfully.
const kindMismatchStaleness = 15 * time.Minute

// --- package.kind_mismatch ---------------------------------------------------
//
// Fires one finding per {node, package} pair where the controller's desired
// kind does not match the artifact kind published in the repository. The drift
// reconciler blocks dispatch for these packages, so they will NEVER converge
// until an operator corrects the desired state or re-publishes the artifact
// with the correct kind.
//
// This is a per-node, per-package companion to desired.kind_mismatch (which is
// a cluster-level aggregate from the Prometheus counter). It gives operators
// the specific package and node affected rather than just a count.

type packageKindMismatch struct{}

func (packageKindMismatch) ID() string       { return "package.kind_mismatch" }
func (packageKindMismatch) Category() string { return "control_plane" }
func (packageKindMismatch) Scope() string    { return "cluster" }

func (p packageKindMismatch) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	if len(snap.KindMismatches) == 0 {
		return nil
	}

	cutoff := time.Now().Add(-kindMismatchStaleness)
	var findings []Finding

	for _, rec := range snap.KindMismatches {
		if rec.DetectedAtUnix == 0 {
			continue
		}
		detectedAt := time.Unix(rec.DetectedAtUnix, 0)
		if detectedAt.Before(cutoff) {
			// Record is stale — mismatch was resolved; reconciler stopped refreshing it.
			continue
		}

		entityRef := rec.NodeID + "/" + rec.PkgName
		findings = append(findings, Finding{
			FindingID:   FindingID(p.ID(), entityRef, rec.DesiredKind+":"+rec.RepoKind),
			InvariantID: p.ID(),
			Severity:    cluster_doctorpb.Severity_SEVERITY_ERROR,
			Category:    p.Category(),
			EntityRef:   entityRef,
			Summary: fmt.Sprintf(
				"Package %q on node %s has kind mismatch: desired %s but repository publishes %s — dispatch permanently blocked",
				rec.PkgName, rec.NodeID, rec.DesiredKind, rec.RepoKind),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("etcd", "/globular/controller/kind_mismatches/"+rec.NodeID+"/"+rec.PkgName,
					map[string]string{
						"node_id":      rec.NodeID,
						"pkg_name":     rec.PkgName,
						"desired_kind": rec.DesiredKind,
						"repo_kind":    rec.RepoKind,
						"detected_at":  detectedAt.UTC().Format(time.RFC3339),
					}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, fmt.Sprintf("Identify the authoritative kind for %q from the package spec", rec.PkgName),
					fmt.Sprintf("globular pkg info %s", rec.PkgName)),
				step(2, fmt.Sprintf("If the desired kind %s is wrong: update the desired state with the correct kind (%s)", rec.DesiredKind, rec.RepoKind),
					fmt.Sprintf("globular deploy %s --kind %s", rec.PkgName, rec.RepoKind)),
				step(3, fmt.Sprintf("If the repo kind %s is wrong: re-publish the artifact with kind %s", rec.RepoKind, rec.DesiredKind),
					"globular pkg build && globular pkg publish"),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}

	return findings
}
