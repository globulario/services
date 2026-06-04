// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.rules.package_version_authority
// @awareness file_role=doctor_rule_detecting_desired_version_that_repository_never_built
// @awareness enforces=globular.platform:invariant.repository.metadata_is_authority
// @awareness risk=high
package rules

// package_version_authority.go — DIAGNOSTIC ONLY. Detects the
// version-authority violation where desired state pins a
// package version the repository has no build for (the
// gen-version.sh / CI version-stamp regression class). Without
// this rule the reconciler install-storms forever against a
// non-existent version because the build_id-orphan rule stays
// silent (no build, no orphan to detect).
//
// MUST NOT rewrite desired state to "fix" the version. The fix
// is upstream: re-publish the package at the right version, or
// roll desired back via the typed controller RPC. Doctor
// auto-correction would mask the CI bug.

// package_version_authority.go — Doctor rule that catches version-authority
// violations: desired state requests a package version that the repository
// has never built or published.
//
// Root cause this rule detects:
//
//	When gen-version.sh or the CI pipeline incorrectly stamps an unchanged
//	package with platform_release (e.g. storage gets "1.2.52" instead of its
//	own "1.2.43"), the controller writes that wrong version to desired state.
//	The repository never built storage@1.2.52 — the reconciler then
//	install-storms forever against a version that doesn't exist.
//
// This rule is complementary to repository.desired_build_ids_resolve:
//   - desired_build_ids_resolve fires when a desired build_id is orphaned
//   - package_version_authority fires when a desired *version* doesn't exist
//     at all in the repository (no build for that version, not just a missing
//     build_id pin)
//
// Both rules are needed: a version that was never built has no build_id to
// orphan, so the build_id rule stays silent while convergence fails.

import (
	"fmt"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

type packageVersionAuthority struct{}

func (packageVersionAuthority) ID() string       { return "repository.package_version_authority" }
func (packageVersionAuthority) Category() string { return "repository" }
func (packageVersionAuthority) Scope() string    { return "cluster" }

func (packageVersionAuthority) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	// Two inputs, both read straight from the Snapshot:
	//  (a) Snapshot.DesiredVersionIndex (populated by the collector via
	//      cluster_controller.GetDesiredState).
	//  (b) Snapshot.RepositoryVersionIndex (populated from
	//      repository.ListArtifacts).
	//
	// No RPCs at evaluation time. The four-layer authority contract
	// requires that "what's desired?" flow out of the controller's
	// typed RPC, never via direct etcd scan
	// (invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage).
	//
	// Degraded-mode: nil RepositoryVersionIndex or nil
	// DesiredVersionIndex → no signal → no findings. This prevents
	// false alarms when either upstream client lost connectivity.
	if snap.RepositoryVersionIndex == nil {
		return nil
	}

	desired := snap.DesiredVersionIndex
	if desired == nil || len(desired) == 0 {
		return nil
	}

	var findings []Finding
	for ref, entry := range desired {
		name := entry.Name
		ver := entry.Version
		if name == "" || ver == "" {
			continue
		}
		// If the package has no published artifacts at all, the repository
		// version index has no entry for it — that is a separate concern
		// (repository.endpoint_missing or undeployed service). Only flag when
		// the repository DOES have the package but not that specific version.
		knownVersions, packageKnown := snap.RepositoryVersionIndex[name]
		if !packageKnown {
			// Repository has zero artifacts for this package — not a version
			// authority violation per se (package may never have been published).
			continue
		}
		if knownVersions[ver] {
			// Repository has this version — no finding.
			continue
		}

		// The repository knows this package but not this version. This is the
		// version-authority failure: desired state was stamped with a version
		// that was never built or published (e.g. platform_release instead of
		// the package's own BOM version).
		known := make([]string, 0, len(knownVersions))
		for v := range knownVersions {
			known = append(known, v)
		}
		findings = append(findings, Finding{
			FindingID:       FindingID("repository.package_version_authority", name, ver),
			InvariantID:     "repository.package_version_authority",
			Severity:        cluster_doctorpb.Severity_SEVERITY_CRITICAL,
			Category:        "repository",
			EntityRef:       ref,
			Summary: fmt.Sprintf(
				"VersionAuthorityViolation: desired %s@%s not in repository — "+
					"repository has versions %v but not %s; "+
					"likely caused by platform_release stamp on an unchanged package",
				name, ver, known, ver),
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("repository", "ListArtifacts (version index)", map[string]string{
					"package":             name,
					"desired_version":     ver,
					"desired_ref":         ref,
					"repository_versions": fmt.Sprintf("%v", known),
					"hint":                "desired version was never built — check gen-version.sh or BOM for platform_release stamp on unchanged packages",
					"forbidden_fix":       "do NOT delete the desired state — roll forward to a version the repository has",
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Identify the correct version from the BOM", "cat /var/lib/globular/release-index.json | jq '.packages[] | select(.name==\""+name+"\") | {version, origin_release, changed_in_release}'"),
				step(2, "Roll desired forward to the correct BOM version", "globular services desired set "+name+" <correct-version-from-bom>"),
				step(3, "Verify the repository has that version", "globular repository scan --package "+name),
			},
		})
	}
	return findings
}

