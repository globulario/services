// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.rules.infra_probe_required
// @awareness file_role=diagnostic_honesty_for_missing_infra_probe_could_not_observe_vs_observed_failure
// @awareness implements=globular.platform:intent.doctor.findings_are_operator_language
// @awareness implements=globular.platform:intent.degraded_is_explicit_not_hidden
// @awareness risk=high
package rules

// infra_probe_required.go — shared builder for the four
// "<component>.probe_required_when_installed" rules (scylladb, etcd, minio,
// envoy). Centralised so the diagnostic-honesty contract lives in ONE place and
// the four rules cannot drift apart (the exact class of bug that bit mcp's
// binary name across six files).
//
// CONTRACT — two availability dimensions must stay distinct
// (meta.harvest_and_yield_are_distinct_availability_dimensions):
//
//   - "could not observe" — the infra probe was not collected because the
//     collector's GetInfraProbe sub-fetch errored (dial failure / source error),
//     or the node-agent binary predates GetInfraProbe (capability gap). The
//     verdict is INDETERMINATE, not failed. Emitting ERROR / INVARIANT_FAIL here
//     asserts a failure on evidence we never collected — it reads "infra is
//     broken" when the truth is "I couldn't look", and alarms operators about
//     healthy infrastructure (observed live 2026-06-22: a single context-canceled
//     harvest produced 5 CRITICAL/ERROR probe-required findings against a fully
//     healthy node). These are WARN + INVARIANT_UNKNOWN with CheckError set, so
//     aggregators never count them as FAIL (see Finding.CheckError contract).
//
//   - "observed nothing despite a complete harvest" — the harvest for this node
//     succeeded (no collector error, capability present) yet an installed
//     component still produced no probe. That is a genuine source failure and
//     stays ERROR / INVARIANT_FAIL.
//
// It NEVER goes silent: a missing probe for an installed component always
// produces a finding (degraded_is_explicit_not_hidden — "could not see" must
// never be indistinguishable from "healthy"). The fix is the epistemic LEVEL of
// the finding, not its presence.

import (
	"fmt"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
)

// infraProbeRequiredFinding builds the probe-required finding for an installed
// node that produced no infra probe. component is the rule/category prefix
// ("scylla", "etcd", "minio", "envoy"); displayName is the operator-facing name
// ("ScyllaDB", "etcd", "MinIO", "Envoy").
func infraProbeRequiredFinding(snap *collector.Snapshot, component, displayName, nid string) Finding {
	capMissing := snap.InfraProbeCapabilityMissing[nid]
	hadErr := snap.HadError("node_agent@"+nid, "GetInfraProbe")

	// Default: complete harvest but no probe — a real source failure.
	sev := cluster_doctorpb.Severity_SEVERITY_ERROR
	status := cluster_doctorpb.InvariantStatus_INVARIANT_FAIL
	checkErr := ""
	reason := "node-agent returned no infra probe despite a complete harvest (source returned nothing)"
	remediation := "Ensure the node-agent is reachable and up to date so it can answer GetInfraProbe."

	switch {
	case capMissing:
		// Old binary that predates GetInfraProbe — expected during rollout.
		sev = cluster_doctorpb.Severity_SEVERITY_WARN
		status = cluster_doctorpb.InvariantStatus_INVARIANT_UNKNOWN
		checkErr = "node-agent predates GetInfraProbe (capability missing)"
		reason = "node-agent binary predates GetInfraProbe (capability missing) — upgrade the node-agent to enable infra truth-plane visibility; status INDETERMINATE, not failed"
		remediation = "Upgrade the node-agent so it implements GetInfraProbe."
	case hadErr:
		// The harvest itself errored — we could not observe, so we cannot claim failure.
		sev = cluster_doctorpb.Severity_SEVERITY_WARN
		status = cluster_doctorpb.InvariantStatus_INVARIANT_UNKNOWN
		checkErr = "infra probe harvest failed (dial or source error)"
		reason = "infra probe could not be collected this sweep (dial failure or source error) — status INDETERMINATE, not failed"
		remediation = "Ensure the node-agent is reachable so it can answer GetInfraProbe; the verdict is unknown until a probe is collected."
	}

	id := component + ".probe_required_when_installed"
	return Finding{
		FindingID:   FindingID(id, nid, ""),
		InvariantID: id,
		Severity:    sev,
		Category:    component,
		EntityRef:   nid,
		Summary:     fmt.Sprintf("Node %s has %s installed but produced no infra probe: %s", nid, displayName, reason),
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("node_agent", "GetInfraProbe", map[string]string{
				"node_id":             nid,
				"capability_missing":  fmt.Sprintf("%t", capMissing),
				"collector_had_error": fmt.Sprintf("%t", hadErr),
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1, remediation, ""),
		},
		InvariantStatus: status,
		CheckError:      checkErr,
	}
}
