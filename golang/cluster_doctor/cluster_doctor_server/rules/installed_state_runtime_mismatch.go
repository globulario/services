package rules

import (
	"encoding/json"
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

const activatingDriftGrace = 2 * time.Minute

var activatingDriftGraceByPackage = map[string]time.Duration{
	// etcd may take longer to become active during member checks/replays.
	"etcd": 5 * time.Minute,
}

func activatingGraceForPackage(pkg string) time.Duration {
	if d, ok := activatingDriftGraceByPackage[pkg]; ok && d > 0 {
		return d
	}
	return activatingDriftGrace
}

func (installedStateRuntimeMismatch) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	var findings []Finding
	now := time.Now()

	// Opt-in packages whose runtime state is gated by an explicit cluster
	// switch. "Installed but inactive" is the correct state for these when
	// the gating spec is disabled — firing on them is a false positive.
	//
	// keepalived is gated by /globular/ingress/v1/spec.mode. When the spec
	// is "disabled" (Day-0 bootstrap default, or explicit operator decision
	// after `globular cluster network ...`), keepalived MUST NOT be running.
	// The doctor used to fire on every cluster that hadn't yet configured a
	// VIP, surfacing as ERROR installed_state_runtime_mismatch even though
	// the cluster was healthy as designed.
	ingressDisabled := IngressIsDisabled(snap)

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
		driftAge := snap.NodeDriftAge[nodeID]
		inHashDrift := driftAge > 0

		lastSeen := node.GetLastSeen().AsTime()
		age := now.Sub(lastSeen)
		stale := lastSeen.IsZero() || age > 3*time.Minute

		unitsByName := make(map[string]string, len(inv.GetUnits()))
		for _, u := range inv.GetUnits() {
			unitsByName[strings.ToLower(strings.TrimSpace(u.GetName()))] = strings.ToLower(strings.TrimSpace(u.GetState()))
		}

		nodeKinds := snap.NodePackageKinds[nodeID] // may be nil
		minioNonMember := nodeIsMinioNonMember(nodeID, snap)

		for name, version := range health.GetInstalledVersions() {
			canon := normalizeInstalledName(name)
			if canon == "" || strings.TrimSpace(version) == "" {
				continue
			}
			if packageIsCommand(canon, nodeKinds) {
				continue
			}
			// Ingress-gated: keepalived is "installed but inactive" by
			// design when the ingress spec is disabled. Skip — not a mismatch.
			if canon == "keepalived" && ingressDisabled {
				continue
			}
			// MinIO non-member: minio and sidekick are installed but inactive
			// by design on nodes that are not part of the MinIO pool.
			if (canon == "minio" || canon == "sidekick") && minioNonMember {
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
			case state == "activating" && inHashDrift && driftAge <= activatingGraceForPackage(canon):
				// During a fresh desired->applied convergence wave, units may
				// legitimately report "activating" for a short window.
				// Keep the mismatch signal for stuck activations, but suppress
				// immediate rollout noise.
				continue
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

// packageIsCommand returns true when the package has no systemd unit to check.
//
// Static list is checked first and is authoritative for known command packages.
// This handles nodes where mc/restic/etc were installed before the kind sidecar
// was introduced: their etcd entry is under INFRASTRUCTURE (not COMMAND), so
// an etcd-first check would return false and fire a spurious incident.
//
// For new packages not in the static list, etcd kind "COMMAND" is the fallback.
// This list should NOT grow; new command packages must be added to the workflow
// spec with kind=COMMAND so the etcd path covers them automatically.
func packageIsCommand(name string, nodeKinds map[string]string) bool {
	// Static list wins — authoritative for pre-kind-sidecar installs.
	switch name {
	case "rclone", "restic", "mc", "sctool", "etcdctl", "ffmpeg",
		"globular-cli", "cli", "sha256sum", "yt-dlp", "claude":
		return true
	}
	// Dynamic fallback: trust etcd COMMAND kind for newer packages.
	if kind, ok := nodeKinds[name]; ok {
		return kind == "COMMAND"
	}
	return false
}

// packageUnit returns the systemd unit name for a non-command package.
// Most packages follow the globular-{name}.service convention.
// Infrastructure packages installed as OS/native packages are listed explicitly.
func packageUnit(name string) string {
	switch name {
	case "scylladb":
		return "scylla-server.service"
	case "keepalived":
		return "keepalived.service"
	case "scylla-manager":
		return "globular-scylla-manager.service"
	case "scylla-manager-agent":
		return "globular-scylla-manager-agent.service"
	default:
		return "globular-" + name + ".service"
	}
}

// IngressIsDisabled returns true when the cluster's ingress spec at
// /globular/ingress/v1/spec is explicitly disabled. Conservative on
// failure: if the spec is missing, malformed, or unreadable, returns
// false so existing fail-open behavior is preserved. The caller MUST NOT
// gate ERROR-severity rules on a "disabled" determination derived from
// failure to read — only on a confirmed `mode: "disabled"`.
//
// Exported so the render package can apply the same waiver to drift
// items (otherwise the keepalived-inactive drift fires on every
// Day-0 cluster even though the rules package has already waived it).
func IngressIsDisabled(snap *collector.Snapshot) bool {
	if snap == nil || !snap.IngressSpecPresent {
		return false
	}
	raw := strings.TrimSpace(snap.IngressSpecRaw)
	if raw == "" {
		return false
	}
	var spec struct {
		Mode             string `json:"mode"`
		ExplicitDisabled bool   `json:"explicit_disabled"`
	}
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(spec.Mode), "disabled") || spec.ExplicitDisabled
}
