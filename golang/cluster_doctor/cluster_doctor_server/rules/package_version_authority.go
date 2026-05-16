package rules

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
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/config"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type packageVersionAuthority struct{}

func (packageVersionAuthority) ID() string       { return "repository.package_version_authority" }
func (packageVersionAuthority) Category() string { return "repository" }
func (packageVersionAuthority) Scope() string    { return "cluster" }

// desiredVersionsReader is the indirection that lets tests inject desired-state
// without going through etcd. Production code uses readDesiredVersions.
var desiredVersionsReader = readDesiredVersions

func (packageVersionAuthority) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	// Two inputs:
	//  (a) the set of (name, version) pairs the cluster currently desires
	//  (b) the set of (name, version) pairs the repository can serve
	//
	// (a) is collected via desiredVersionsReader (etcd, overrideable in tests).
	// (b) is snap.RepositoryVersionIndex, built from ListArtifacts PUBLISHED.
	//
	// Degraded-mode: nil RepositoryVersionIndex → no signal → no findings.
	// This prevents false alarms when the collector lost its repository client.
	if snap.RepositoryVersionIndex == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	desired := desiredVersionsReader(ctx)
	if len(desired) == 0 {
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

// desiredVersionEntry holds (name, version) extracted from a desired-state record.
type desiredVersionEntry struct {
	Name    string
	Version string
}

// readDesiredVersions scans all desired-state etcd prefixes and returns a map
// etcd-ref → (name, version). Best-effort: returns empty map on etcd error.
func readDesiredVersions(ctx context.Context) map[string]desiredVersionEntry {
	out := map[string]desiredVersionEntry{}
	cli, err := config.GetEtcdClient()
	if err != nil {
		return out
	}
	prefixes := []string{
		"/globular/resources/ServiceDesiredVersion/",
		"/globular/resources/InfrastructureRelease/",
		"/globular/resources/DesiredService/",
		"/globular/resources/ServiceRelease/",
	}
	type genericRec struct {
		Metadata *struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Spec *struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"spec"`
		Status *struct {
			InstalledVersion string `json:"installed_version"`
		} `json:"status"`
	}
	for _, prefix := range prefixes {
		resp, getErr := cli.Get(ctx, prefix, clientv3.WithPrefix(), clientv3.WithLimit(500))
		if getErr != nil {
			continue
		}
		for _, kv := range resp.Kvs {
			var rec genericRec
			if json.Unmarshal(kv.Value, &rec) != nil {
				continue
			}
			ref := string(kv.Key)
			name := ""
			ver := ""
			if rec.Metadata != nil && rec.Metadata.Name != "" {
				name = rec.Metadata.Name
			}
			if rec.Spec != nil {
				if rec.Spec.Name != "" {
					name = rec.Spec.Name
				}
				if rec.Spec.Version != "" {
					ver = rec.Spec.Version
				}
			}
			if name == "" || ver == "" {
				continue
			}
			out[ref] = desiredVersionEntry{Name: name, Version: ver}
		}
	}
	return out
}
