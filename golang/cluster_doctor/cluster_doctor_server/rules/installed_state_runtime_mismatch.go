package rules

import (
	"fmt"
	"strings"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

type installedStateRuntimeMismatch struct{}

func (installedStateRuntimeMismatch) ID() string       { return "installed_state_runtime_mismatch" }
func (installedStateRuntimeMismatch) Category() string { return "convergence" }
func (installedStateRuntimeMismatch) Scope() string    { return "node" }

func (installedStateRuntimeMismatch) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	var findings []Finding
	now := time.Now()

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

		lastSeen := node.GetLastSeen().AsTime()
		age := now.Sub(lastSeen)
		stale := lastSeen.IsZero() || age > 3*time.Minute

		unitsByName := make(map[string]string, len(inv.GetUnits()))
		for _, u := range inv.GetUnits() {
			unitsByName[strings.ToLower(strings.TrimSpace(u.GetName()))] = strings.ToLower(strings.TrimSpace(u.GetState()))
		}

		for name, version := range health.GetInstalledVersions() {
			canon := normalizeInstalledName(name)
			if canon == "" || strings.TrimSpace(version) == "" {
				continue
			}
			if commandPackage(canon) {
				continue
			}
			unit := packageUnit(canon)
			state, ok := unitsByName[strings.ToLower(unit)]

			mismatch := false
			reason := ""
			switch {
			case stale:
				mismatch = true
				reason = fmt.Sprintf("runtime status stale (last seen %s ago)", age.Round(time.Second))
			case !ok:
				mismatch = true
				reason = fmt.Sprintf("runtime unit missing (%s)", unit)
			case state != "active":
				mismatch = true
				reason = fmt.Sprintf("runtime unit state=%s (%s)", state, unit)
			}
			if !mismatch {
				continue
			}

			sev := cluster_doctorpb.Severity_SEVERITY_WARN
			if node.GetStatus() != "ready" {
				sev = cluster_doctorpb.Severity_SEVERITY_ERROR
			}
			key := canon + ":" + unit
			findings = append(findings, Finding{
				FindingID:       FindingID("installed_state_runtime_mismatch", nodeID, key),
				InvariantID:     "installed_state_runtime_mismatch",
				Severity:        sev,
				Category:        "convergence",
				EntityRef:       fmt.Sprintf("%s/%s", nodeID, canon),
				Summary:         fmt.Sprintf("Package %s on node %s has installed_state=%s but runtime not converged: %s", canon, nodeID, version, reason),
				InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
				Evidence: []*cluster_doctorpb.Evidence{
					kvEvidence("cluster_controller", "GetClusterHealthV1+GetInventory", map[string]string{
						"node_id":           nodeID,
						"package":           canon,
						"installed_version": version,
						"unit":              unit,
						"runtime_state":     state,
						"reason":            reason,
					}),
				},
				Remediation: []*cluster_doctorpb.RemediationStep{
					step(1, fmt.Sprintf("Dispatch release workflow repair for %s on node %s", canon, nodeID), "globular node reconcile"),
					step(2, fmt.Sprintf("Inspect unit logs: journalctl -u %s -n 100", unit), ""),
					step(3, "Verify convergence after workflow retry", "globular cluster get-doctor-report"),
				},
			})
		}
	}

	return findings
}

func normalizeInstalledName(name string) string {
	n := strings.TrimSpace(strings.ToLower(name))
	n = strings.ReplaceAll(n, "_", "-")
	return n
}

func commandPackage(name string) bool {
	switch name {
	case "rclone", "restic", "mc", "sctool", "etcdctl", "ffmpeg", "globular-cli":
		return true
	default:
		return false
	}
}

func packageUnit(name string) string {
	switch name {
	case "scylladb":
		return "scylla-server.service"
	case "scylla-manager":
		return "globular-scylla-manager.service"
	case "scylla-manager-agent":
		return "globular-scylla-manager-agent.service"
	default:
		return "globular-" + name + ".service"
	}
}
