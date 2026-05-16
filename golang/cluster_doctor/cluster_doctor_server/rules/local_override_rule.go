package rules

// local_override_rule.go — Doctor invariants for local/dev package identity lanes.
//
// Two invariants:
//
//   package.local_override_active
//     WARN: one or more artifacts in the repository carry a local/dev/hotfix
//     version suffix (e.g. 1.2.43+local.ryzen.1). These are workbench builds
//     that should not permanently replace official stable packages. Operators
//     should promote the fix through the official release pipeline or remove
//     the local override.
//
//   package.official_identity_sealed
//     Maps the repository finding REPO_FIND_PUBLISHED_CHECKSUM_MISMATCH for
//     official-publisher artifacts to a clear "identity sealed" finding, making
//     the identity conflict visible in doctor reports with an explicit remediation.
//
// Both rules degrade gracefully when snapshot data is unavailable (nil map).

import (
	"fmt"
	"strings"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// ── package.local_override_active ────────────────────────────────────────────

type localOverrideActive struct{}

func (localOverrideActive) ID() string       { return "package.local_override_active" }
func (localOverrideActive) Category() string { return "repository" }
func (localOverrideActive) Scope() string    { return "cluster" }

func (localOverrideActive) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	if snap == nil || len(snap.RepositoryVersionIndex) == 0 {
		return nil
	}

	var findings []Finding
	for pkgName, versions := range snap.RepositoryVersionIndex {
		for ver := range versions {
			if !isLocalVersionSuffix(ver) {
				continue
			}
			entityRef := fmt.Sprintf("%s@%s", pkgName, ver)
			findings = append(findings, Finding{
				FindingID:   FindingID("package.local_override_active", entityRef, ver),
				InvariantID: "package.local_override_active",
				Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
				Category:    "repository",
				EntityRef:   entityRef,
				Summary: fmt.Sprintf(
					"local package override active: %s version %s is a local/dev/hotfix build — "+
						"it must not permanently replace the official stable artifact",
					pkgName, ver),
				Evidence: []*cluster_doctorpb.Evidence{
					kvEvidence("repository", "RepositoryVersionIndex", map[string]string{
						"package": pkgName,
						"version": ver,
					}),
				},
				Remediation: []*cluster_doctorpb.RemediationStep{
					step(1,
						"Review why this local build is present. If testing is complete, "+
							"promote the fix through the official release pipeline:",
						"globular release promote-local "+pkgName+" --from-build <local-build-id> --as-version <new-version>"),
					step(2,
						"Or remove the local override if it is no longer needed:",
						"globular pkg override remove "+pkgName),
				},
				InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
			})
		}
	}
	return findings
}

// ── package.official_identity_sealed ─────────────────────────────────────────

// officialIdentitySealed surfaces repository checksum-mismatch findings for
// official-publisher artifacts as a distinct "identity sealed" doctor finding
// with explicit remediation guidance. This complements the generic
// repository.published_checksum_mismatch finding with identity-lane context.
type officialIdentitySealed struct{}

func (officialIdentitySealed) ID() string       { return "package.official_identity_sealed" }
func (officialIdentitySealed) Category() string { return "repository" }
func (officialIdentitySealed) Scope() string    { return "cluster" }

func (officialIdentitySealed) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	if snap == nil || len(snap.RepositoryFindings) == 0 {
		return nil
	}

	const officialPublisher = "core@globular.io"
	var findings []Finding

	for _, rf := range snap.RepositoryFindings {
		if rf == nil {
			continue
		}
		if rf.Kind != "REPO_FIND_PUBLISHED_CHECKSUM_MISMATCH" {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(rf.PublisherID), officialPublisher) {
			continue
		}
		entityRef := fmt.Sprintf("%s/%s@%s", rf.PublisherID, rf.Name, rf.Version)
		findings = append(findings, Finding{
			FindingID:   FindingID("package.official_identity_sealed", entityRef, rf.Reason),
			InvariantID: "package.official_identity_sealed",
			Severity:    cluster_doctorpb.Severity_SEVERITY_ERROR,
			Category:    "repository",
			EntityRef:   entityRef,
			Summary: fmt.Sprintf(
				"official identity conflict: %s/%s@%s is SEALED — "+
					"the stored artifact has a different digest than the official stable release. "+
					"Official stable artifacts are immutable.",
				rf.PublisherID, rf.Name, rf.Version),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("repository", "ListRepositoryFindings", map[string]string{
					"publisher":      rf.PublisherID,
					"package":        rf.Name,
					"version":        rf.Version,
					"platform":       rf.Platform,
					"current_state":  rf.CurrentState,
					"expected_state": rf.ExpectedState,
					"reason":         rf.Reason,
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1,
					"DO NOT overwrite the stored artifact digest. Investigate the root cause "+
						"(unauthorized local build uploaded as official stable?).",
					""),
				step(2,
					"If the stored artifact is corrupt, restore from the official GitHub release:",
					fmt.Sprintf("globular pkg publish --force --file <official-%s.tgz> --repository <repo>", rf.Name)),
				step(3,
					"If a local fix was mistakenly published as official stable, remove it "+
						"and use a local identity lane instead:",
					fmt.Sprintf("globular pkg publish --channel local --based-on %s@%s --file <local-%s.tgz>", rf.Name, rf.Version, rf.Name)),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return findings
}

// isLocalVersionSuffix mirrors the server-side hasLocalVersionSuffix without
// importing the repository package.
func isLocalVersionSuffix(version string) bool {
	lower := strings.ToLower(version)
	return strings.Contains(lower, "+local.") ||
		strings.Contains(lower, "-dev.") ||
		strings.Contains(lower, "-hotfix.") ||
		strings.Contains(lower, "+dev.") ||
		strings.Contains(lower, "+hotfix.")
}
