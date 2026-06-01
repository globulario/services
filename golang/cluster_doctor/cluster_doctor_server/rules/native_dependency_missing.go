// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.rules
// @awareness file_role=native_dependency_missing_detection_rule
// @awareness enforces=globular.platform:invariant.state.installed_not_catalog
// @awareness risk=high
package rules

import (
	"fmt"
	"strings"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// nativeDependencyMissing surfaces findings when a service package is installed
// and its systemd unit is in "failed" state because of a known missing native
// shared library. It uses static knowledge of which packages require which
// OS-level libraries, which is populated from package.json native_dependencies
// declarations during build time. For now, the mapping is maintained here and
// should be kept in sync with packages/metadata/*/package.json.
//
// Severity is ERROR: the service cannot start and the cluster is degraded.
type nativeDependencyMissing struct{}

func (nativeDependencyMissing) ID() string       { return "package.native_dependency_missing" }
func (nativeDependencyMissing) Category() string { return "convergence" }
func (nativeDependencyMissing) Scope() string    { return "node" }

// knownNativeDeps maps package name → (missing library → OS package that provides it).
// Populated from packages/metadata/*/package.json native_dependencies declarations.
var knownNativeDeps = map[string]map[string]string{
	"sql": {
		"libodbc.so.2": "unixodbc",
	},
}

func (nativeDependencyMissing) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	var findings []Finding

	for _, node := range snap.Nodes {
		nodeID := node.GetNodeId()
		if nodeID == "" {
			continue
		}
		health := snap.NodeHealths[nodeID]
		if health == nil {
			continue
		}
		inv := snap.Inventories[nodeID]
		if inv == nil {
			continue
		}

		unitsByName := make(map[string]string, len(inv.GetUnits()))
		for _, u := range inv.GetUnits() {
			unitsByName[strings.ToLower(strings.TrimSpace(u.GetName()))] = strings.ToLower(strings.TrimSpace(u.GetState()))
		}

		for name, version := range health.GetInstalledVersions() {
			if strings.TrimSpace(version) == "" {
				continue
			}
			canon := normalizeInstalledName(name)
			nativeDeps, ok := knownNativeDeps[canon]
			if !ok {
				continue // no known native deps for this package
			}

			unit := packageUnit(canon)
			state := unitsByName[strings.ToLower(unit)]
			if state != "failed" {
				continue // unit is not in a failure state — no finding needed
			}

			var missingLibs []string
			var providers []string
			for lib, pkg := range nativeDeps {
				missingLibs = append(missingLibs, lib)
				providers = append(providers, pkg)
			}

			findings = append(findings, Finding{
				FindingID:       FindingID("package.native_dependency_missing", nodeID, canon),
				InvariantID:     "package.native_dependency_missing",
				Severity:        cluster_doctorpb.Severity_SEVERITY_ERROR,
				Category:        "convergence",
				EntityRef:       fmt.Sprintf("%s/%s", nodeID, canon),
				Summary:         fmt.Sprintf("Package %s on node %s is crash-looping: binary requires missing native libraries %v", canon, nodeID, missingLibs),
				InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
				Evidence: []*cluster_doctorpb.Evidence{
					kvEvidence("node_agent", "GetInventory", map[string]string{
						"node_id":          nodeID,
						"package":          canon,
						"unit":             unit,
						"unit_state":       state,
						"missing_libs":     strings.Join(missingLibs, ", "),
						"os_packages":      strings.Join(providers, ", "),
					}),
				},
				Remediation: []*cluster_doctorpb.RemediationStep{
					step(1, fmt.Sprintf("Install the missing OS packages on node %s: sudo apt install -y %s", nodeID, strings.Join(providers, " ")), ""),
					step(2, fmt.Sprintf("Restart the service: sudo systemctl restart %s", unit), ""),
					step(3, "Verify the service is active and the workflow retries", "globular cluster get-doctor-report"),
				},
			})
		}
	}

	return findings
}
