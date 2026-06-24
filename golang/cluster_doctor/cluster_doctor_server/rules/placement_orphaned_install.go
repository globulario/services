package rules

// placement_orphaned_install.go — Doctor rule that surfaces the "orphaned
// install" class: a package that is INSTALLED on a node whose profiles do not
// authorize it under the component catalog placement map.
//
// Such an install can never converge. The release reconciler profile-skips it
// and (after E1/E2) the drift-reconciler refuses to dispatch it — so without
// this finding it would sit as silent, permanent drift or, before E2, an apply
// hamster-wheel. This rule is the operator-facing, TERMINAL verdict:
// orphaned install, operator action required.
//
// Authority boundary: the controller decides NOT to dispatch (a convergence
// decision); the cross-layer health VERDICT lives here, because cluster-doctor
// is the only canonical answerer for cross-layer health state
// (invariant: cluster_doctor.is_the_authority_for_health_state_queries).
//
// Placement authority is component_catalog.ProfilePackages — generated from and
// consistency-tested against the controller's catalog, i.e. the SAME authority
// platform-upgrade evaluate and the release reconciler use (E1). Reusing it
// here keeps doctor and controller from disagreeing (no second law book).

import (
	"fmt"
	"sort"
	"strings"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/component_catalog"
)

type placementInstalledPackageOrphaned struct{}

func (placementInstalledPackageOrphaned) ID() string       { return "placement.installed_package_orphaned" }
func (placementInstalledPackageOrphaned) Category() string { return "placement" }
func (placementInstalledPackageOrphaned) Scope() string    { return "node" }

func (r placementInstalledPackageOrphaned) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	var findings []Finding
	for _, node := range snap.Nodes {
		nodeID := node.GetNodeId()
		health := snap.NodeHealths[nodeID]
		if health == nil {
			// Installed-state for this node could not be observed. Reduced-harvest
			// honesty: absence of data is UNKNOWN, never a definitive verdict — so
			// emit nothing rather than fabricate an orphan finding.
			continue
		}
		nodeProfiles := node.GetProfiles()

		// Placement authority: the set of packages the catalog authorizes on
		// this node's profiles (inheritance-expanded).
		placeable := map[string]bool{}
		for _, p := range component_catalog.PackagesForProfiles(nodeProfiles) {
			placeable[p] = true
		}

		// Deterministic iteration so findings are stable across runs.
		installed := health.GetInstalledVersions()
		names := make([]string, 0, len(installed))
		for name := range installed {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, rawName := range names {
			name := strings.ToLower(strings.TrimSpace(rawName))
			if name == "" {
				continue
			}
			required := component_catalog.ProfilesForPackage(name)
			if len(required) == 0 {
				// Not in the catalog placement map at all. "Unknown / not
				// catalog-tracked" is a DISTINCT condition from a profile orphan;
				// do not conflate the two. Skip here (covered elsewhere).
				continue
			}
			if placeable[name] {
				continue // legitimately authorized on this node
			}

			installedVer := strings.TrimSpace(installed[rawName])
			if installedVer == "" {
				installedVer = "<unknown>"
			}
			findings = append(findings, Finding{
				FindingID:       FindingID("placement.installed_package_orphaned", nodeID, name),
				InvariantID:     "placement.installed_package_orphaned",
				Severity:        cluster_doctorpb.Severity_SEVERITY_WARN,
				Category:        "placement",
				EntityRef:       nodeID + "/" + name,
				Summary: fmt.Sprintf(
					"Orphaned install: %q@%s is installed on node %s but the node's profiles %v do not authorize it (catalog requires one of %v) — it can never converge here; operator action required.",
					name, installedVer, nodeID, component_catalog.NormalizeProfiles(nodeProfiles), required),
				InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
				Evidence: []*cluster_doctorpb.Evidence{
					kvEvidence("cluster_controller", "GetClusterHealthV1 (installed versions) + component_catalog placement map", map[string]string{
						"node_id":           nodeID,
						"package":           name,
						"installed_version": installedVer,
						"catalog_profiles":  strings.Join(required, ","),
						"node_profiles":     strings.Join(component_catalog.NormalizeProfiles(nodeProfiles), ","),
						"terminal":          "true",
						"non_dispatchable":  "true",
						"forbidden_fix":     "do NOT add the required profile to the node just to silence this — decide intent first (retire the desired record, or uninstall the orphaned binary)",
					}),
				},
				Remediation: []*cluster_doctorpb.RemediationStep{
					step(1, fmt.Sprintf("If %s does not belong on this node, retire its desired record", name),
						fmt.Sprintf("globular services desired remove %s", name)),
					step(2, fmt.Sprintf("If the node SHOULD host %s, add a matching profile (one of %v) and reconcile", name, required),
						"globular cluster reconcile"),
					step(3, fmt.Sprintf("Optionally remove the stale binary after retiring desired (node-agent uninstall of %s)", name), ""),
				},
			})
		}
	}
	return findings
}
