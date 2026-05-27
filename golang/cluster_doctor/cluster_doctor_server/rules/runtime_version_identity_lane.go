package rules

import (
	"fmt"
	"strings"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// runtimeVersionIdentityLane fires when a node reports a local/dev/hotfix
// runtime version without an active local override record for that package.
// This catches undeclared local identity lanes leaking into runtime.
type runtimeVersionIdentityLane struct{}

func (runtimeVersionIdentityLane) ID() string       { return "service.runtime_version_identity_lane" }
func (runtimeVersionIdentityLane) Category() string { return "repository" }
func (runtimeVersionIdentityLane) Scope() string    { return "node" }

func (runtimeVersionIdentityLane) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	if snap == nil || len(snap.NodeHealths) == 0 {
		return nil
	}
	// Nil means override read failed; degrade gracefully to avoid false incidents.
	if snap.ActiveLocalOverrides == nil {
		return nil
	}

	var findings []Finding
	for nodeID, health := range snap.NodeHealths {
		if health == nil {
			continue
		}
		nodeKinds := snap.NodePackageKinds[nodeID]
		for name, version := range health.GetInstalledVersions() {
			canon := normalizeInstalledName(name)
			if canon == "" || strings.TrimSpace(version) == "" {
				continue
			}
			if packageIsCommand(canon, nodeKinds) {
				continue
			}
			if !isLocalVersionSuffix(version) {
				continue
			}
			ov, ok := snap.ActiveLocalOverrides[canon]
			if ok && ov != nil && strings.TrimSpace(ov.Version) == strings.TrimSpace(version) {
				continue
			}

			findings = append(findings, Finding{
				FindingID:       FindingID("service.runtime_version_identity_lane", nodeID+"/"+canon, version),
				InvariantID:     "service.runtime_version_identity_lane",
				Severity:        cluster_doctorpb.Severity_SEVERITY_WARN,
				Category:        "repository",
				EntityRef:       fmt.Sprintf("%s/%s", nodeID, canon),
				Summary:         fmt.Sprintf("node %s reports local runtime version %s for %s, but no matching active local override exists", nodeID, version, canon),
				InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
				Evidence: []*cluster_doctorpb.Evidence{
					kvEvidence("cluster_controller", "GetClusterHealthV1", map[string]string{
						"node_id":           nodeID,
						"package":           canon,
						"installed_version": version,
					}),
					kvEvidence("cluster_controller", "ListLocalOverrides", map[string]string{
						"package":          canon,
						"override_present": fmt.Sprintf("%t", ok),
					}),
				},
				Remediation: []*cluster_doctorpb.RemediationStep{
					step(1, "Declare a matching local override for this package/version if this is intentional.", "globular pkg override set "+canon+" --version "+version+" --build-id <build-id>"),
					step(2, "Or remove the undeclared local build by restoring official desired state and reconciling.", "globular node reconcile"),
				},
			})
		}
	}
	return findings
}

// runtimeVersionOverrideDivergence fires when an active local override exists
// but a node reports a different local runtime identity (version/build_id).
type runtimeVersionOverrideDivergence struct{}

func (runtimeVersionOverrideDivergence) ID() string       { return "service.runtime_version_override_divergence" }
func (runtimeVersionOverrideDivergence) Category() string { return "repository" }
func (runtimeVersionOverrideDivergence) Scope() string    { return "node" }

func (runtimeVersionOverrideDivergence) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	if snap == nil || len(snap.NodeHealths) == 0 || len(snap.ActiveLocalOverrides) == 0 {
		return nil
	}

	var findings []Finding
	for nodeID, health := range snap.NodeHealths {
		if health == nil {
			continue
		}
		nodeKinds := snap.NodePackageKinds[nodeID]
		for pkg, ov := range snap.ActiveLocalOverrides {
			if ov == nil {
				continue
			}
			canon := normalizeInstalledName(pkg)
			if canon == "" || packageIsCommand(canon, nodeKinds) {
				continue
			}
			installedVer, ok := health.GetInstalledVersions()[canon]
			if !ok || strings.TrimSpace(installedVer) == "" || !isLocalVersionSuffix(installedVer) {
				continue
			}

			var reasons []string
			if strings.TrimSpace(ov.Version) != "" && strings.TrimSpace(installedVer) != strings.TrimSpace(ov.Version) {
				reasons = append(reasons, fmt.Sprintf("version mismatch: node=%s override=%s", installedVer, ov.Version))
			}
			installedBID := strings.TrimSpace(health.GetInstalledBuildIds()[canon])
			overrideBID := strings.TrimSpace(ov.BuildID)
			if overrideBID != "" && installedBID != "" && installedBID != overrideBID {
				reasons = append(reasons, fmt.Sprintf("build_id mismatch: node=%s override=%s", min8str(installedBID), min8str(overrideBID)))
			}
			if len(reasons) == 0 {
				continue
			}

			findings = append(findings, Finding{
				FindingID:       FindingID("service.runtime_version_override_divergence", nodeID+"/"+canon, strings.Join(reasons, "|")),
				InvariantID:     "service.runtime_version_override_divergence",
				Severity:        cluster_doctorpb.Severity_SEVERITY_WARN,
				Category:        "repository",
				EntityRef:       fmt.Sprintf("%s/%s", nodeID, canon),
				Summary:         fmt.Sprintf("node %s runtime local identity diverges from active override for %s: %s", nodeID, canon, strings.Join(reasons, "; ")),
				InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
				Evidence: []*cluster_doctorpb.Evidence{
					kvEvidence("cluster_controller", "GetClusterHealthV1", map[string]string{
						"node_id":             nodeID,
						"package":             canon,
						"installed_version":   installedVer,
						"installed_build_id":  installedBID,
						"override_version":    ov.Version,
						"override_build_id":   overrideBID,
						"override_service":    ov.ServiceName,
						"override_publisher":  ov.PublisherID,
						"override_created_by": ov.CreatedBy,
					}),
				},
				Remediation: []*cluster_doctorpb.RemediationStep{
					step(1, "Reconcile nodes to apply the active local override consistently.", "globular node reconcile"),
					step(2, "If this runtime identity is intended, update the override record to the actually deployed local build.", "globular pkg override set "+canon+" --version "+installedVer+" --build-id <build-id>"),
				},
			})
		}
	}
	return findings
}
