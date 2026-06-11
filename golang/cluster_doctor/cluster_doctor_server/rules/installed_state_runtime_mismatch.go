// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.rules.installed_state_runtime_mismatch
// @awareness file_role=doctor_rule_correlating_layer3_installed_with_layer4_runtime_in_4_layer_truth_model
// @awareness implements=globular.platform:intent.runtime_observation_must_not_mutate_desired
// @awareness risk=high
package rules

// installed_state_runtime_mismatch.go — DIAGNOSTIC ONLY. Correlates
// Layer 3 (installed observed) with Layer 4 (runtime health) from
// the 4-layer truth model. A package marked installed but whose
// systemd unit is not active for longer than activatingDriftGrace
// (2 min default, 5 min for etcd) surfaces as a finding so an
// operator can dig in.
//
// MUST NOT mutate desired state. Conflating "running" with "should
// be running" would collapse Layer 1↔Layer 2 — exactly the failure
// mode the 4-layer model exists to prevent. The opt-in gates
// (e.g. keepalived requires ingress spec mode != disabled) prevent
// the rule from firing on packages that are intentionally
// "installed but inactive"; they are a correctness requirement,
// not a cosmetic suppression.

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
	// Guard: refuse to emit findings when the data source errored — "no data" must not become "no problems." See meta.absence_scope_must_be_explicit.
	if snap.HadError("cluster_controller", "GetClusterHealthV1") {
		return nil
	}

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

		// Node-wide heartbeat staleness is NOT a per-package mismatch. When the
		// node has not heartbeat within the reachability window (or is explicitly
		// "unreachable"), we cannot observe ANY package's runtime — the state is
		// UNKNOWN, not "not converged". Emitting one finding per installed package
		// here would (1) amplify a single node outage into N findings
		// (meta.diagnostic_output_must_be_bounded) and (2) misclassify unknown
		// runtime as down (fm.industry.missing_inventory_misclassified_as_down /
		// state.unknown_must_not_default_to_healthy). The node-scoped node.reachable
		// rule owns this signal and fires on the SAME condition (see
		// node_reachable.go), so suppressing here never drops coverage — it
		// deduplicates it down to a single node-level CRITICAL.
		lastSeen := node.GetLastSeen().AsTime()
		age := now.Sub(lastSeen)
		if node.GetStatus() == "unreachable" || lastSeen.IsZero() || age > cfg.HeartbeatStale {
			continue
		}

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
