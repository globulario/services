// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.rules
// @awareness file_role=local_operator_override_detection_rule
// @awareness risk=medium
package rules

// local_override_rule.go — Doctor invariants for local/dev package identity lanes.
//
// Three invariants:
//
//   package.local_override_active
//     WARN: one or more artifacts in the repository carry a local/dev/hotfix
//     version suffix (e.g. 1.2.43+local.ryzen.1). These are workbench builds
//     that should not permanently replace official stable packages. Operators
//     should promote the fix through the official release pipeline or remove
//     the local override.
//
//   package.local_override_stale
//     WARN: a local override is stale when:
//       (a) the local build is no longer resolvable in the repository,
//       (b) the official BOM has moved to a newer version than based_on,
//       (c) nodes are running different build_ids for the overridden package, or
//       (d) the override record is older than the configured staleness threshold.
//     Any one of these conditions fires the finding, reporting all reasons found.
//
//   package.official_identity_sealed
//     Maps the repository finding REPO_FIND_PUBLISHED_CHECKSUM_MISMATCH for
//     official-publisher artifacts to a clear "identity sealed" finding, making
//     the identity conflict visible in doctor reports with an explicit remediation.
//
// All rules degrade gracefully when snapshot data is unavailable (nil map).

import (
	"fmt"
	"strings"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// overrideStalenessThreshold is the default age after which an override is
// considered stale regardless of BOM drift. 7 days captures weekend hotfixes
// that were never promoted; 0 disables the age check.
const overrideStalenessThreshold = 7 * 24 * time.Hour

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

// ── package.local_override_stale ─────────────────────────────────────────────

type localOverrideStale struct{}

func (localOverrideStale) ID() string       { return "package.local_override_stale" }
func (localOverrideStale) Category() string { return "repository" }
func (localOverrideStale) Scope() string    { return "cluster" }

func (localOverrideStale) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	if snap == nil || len(snap.ActiveLocalOverrides) == 0 {
		return nil
	}

	var findings []Finding
	now := time.Now()

	for pkgName, ov := range snap.ActiveLocalOverrides {
		if ov == nil {
			continue
		}

		var reasons []string

		// (a) Local build no longer resolvable in repository
		if snap.RepositoryBuildIDIndex != nil && !snap.RepositoryBuildIDIndex[ov.BuildID] {
			reasons = append(reasons, fmt.Sprintf(
				"local build %s... is no longer resolvable in the repository", min8str(ov.BuildID)))
		}

		// (b) Official BOM has moved: a newer non-local version exists for this package
		basedOn := ""
		if ov.OfficialSnapshot != nil {
			basedOn = ov.OfficialSnapshot.Version
		}
		if basedOn == "" {
			basedOn = ov.BasedOnVersion
		}
		if snap.RepositoryVersionIndex != nil && basedOn != "" {
			if versions, ok := snap.RepositoryVersionIndex[pkgName]; ok {
				for v := range versions {
					if isLocalVersionSuffix(v) {
						continue
					}
					if isNewerVersion(v, basedOn) {
						reasons = append(reasons, fmt.Sprintf(
							"official BOM has moved: repository now has %s, override based on %s",
							v, basedOn))
						break
					}
				}
			}
		}

		// (c) Nodes running different build_ids for this package
		if len(snap.NodeHealths) > 1 {
			seen := make(map[string][]string) // build_id → []nodeID
			for nodeID, nh := range snap.NodeHealths {
				if nh == nil {
					continue
				}
				if bid, ok := nh.InstalledBuildIds[pkgName]; ok && bid != "" {
					seen[bid] = append(seen[bid], nodeID)
				}
			}
			if len(seen) > 1 {
				var parts []string
				for bid, nodes := range seen {
					parts = append(parts, fmt.Sprintf("%s... on %v", min8str(bid), nodes))
				}
				reasons = append(reasons, "nodes disagree on installed build_id: "+strings.Join(parts, "; "))
			}
		}

		// (d) Override record older than staleness threshold
		if overrideStalenessThreshold > 0 && ov.CreatedAtUnixS > 0 {
			age := now.Sub(time.Unix(ov.CreatedAtUnixS, 0))
			if age > overrideStalenessThreshold {
				reasons = append(reasons, fmt.Sprintf(
					"override is %.0f days old (threshold: %.0f days)",
					age.Hours()/24, overrideStalenessThreshold.Hours()/24))
			}
		}

		if len(reasons) == 0 {
			continue
		}

		entityRef := fmt.Sprintf("%s@%s", pkgName, ov.Version)
		var evidenceKV []map[string]string
		for i, r := range reasons {
			evidenceKV = append(evidenceKV, map[string]string{
				"reason": fmt.Sprintf("[%d] %s", i+1, r),
			})
		}
		var evidences []*cluster_doctorpb.Evidence
		for _, kv := range evidenceKV {
			evidences = append(evidences, kvEvidence("override", "ActiveLocalOverrides", kv))
		}

		findings = append(findings, Finding{
			FindingID:   FindingID("package.local_override_stale", entityRef, strings.Join(reasons, "|")),
			InvariantID: "package.local_override_stale",
			Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:    "repository",
			EntityRef:   entityRef,
			Summary: fmt.Sprintf(
				"local override for %s (version %s, build_id %s...) is stale: %s",
				pkgName, ov.Version, min8str(ov.BuildID), strings.Join(reasons, "; ")),
			Evidence: evidences,
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1,
					"Promote the fix through the official release pipeline:",
					"globular release promote-local "+pkgName+" --from-build <local-build-id> --as-version <new-version>"),
				step(2,
					"Or remove the stale override to restore the official build:",
					"globular pkg override remove "+pkgName),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return findings
}

// isNewerVersion returns true if candidate is a strictly newer semver-like
// version string than baseline. Compares only non-local (official) versions.
// Uses simple string prefix comparison on the semver numeric portion; a full
// semver library would be overkill here given versions follow a single format.
func isNewerVersion(candidate, baseline string) bool {
	if candidate == "" || baseline == "" {
		return false
	}
	// Strip leading "v" if present
	c := strings.TrimPrefix(candidate, "v")
	b := strings.TrimPrefix(baseline, "v")
	// Only compare the base version (before any prerelease/build metadata)
	c = strings.SplitN(c, "+", 2)[0]
	c = strings.SplitN(c, "-", 2)[0]
	b = strings.SplitN(b, "+", 2)[0]
	b = strings.SplitN(b, "-", 2)[0]
	return c > b // lexicographic; works for 1.2.43 vs 1.2.44 since padding is consistent
}

// min8str returns at most 8 chars of s, for compact display in summaries.
func min8str(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:8]
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
