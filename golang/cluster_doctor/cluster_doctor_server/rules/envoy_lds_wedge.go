// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.envoy_lds_wedge_detector
// @awareness file_role=diagnostic_only_detector_for_envoy_lds_handshake_wedge
// @awareness enforces=globular.platform:invariant.envoy.lds_progress_required_for_http_mesh_readiness
// @awareness relates_to=globular.platform:failure_mode.envoy.lds_update_attempt_zero_despite_cds_progress
// @awareness risk=medium
package rules

// envoy_lds_wedge.go — Phase 28.
//
// Diagnostic-only doctor rule that fires when the Envoy data-plane has
// taken at least one CDS update but never attempted an LDS update. In
// that state the mesh has clusters but no listeners; port 443 stays
// unbound; HTTP routing through Envoy is dead even though
// `systemctl is-active globular-envoy.service` reports active.
//
// The rule MUST NOT restart, kill, reload, or otherwise touch Envoy or
// xDS — it only converts the Prometheus signal into a structured
// Finding that the operator (or a future, separately-reviewed
// remediator) can act on. The actual root cause for the LDS-stays-at-0
// pattern observed in INC docs/awareness/reports/envoy_lds_cds_wedge.md
// is a restart-storm upstream (workflow re-dispatching
// node.maybe_restart_package against envoy) that SIGTERMs Envoy before
// the LDS handshake can complete. This rule does not attempt to fix
// that upstream loop; it surfaces the data-plane symptom so an
// operator knows the mesh is wedged.

import (
	"fmt"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// envoyLDSWedge classifies the (CDS-progresses, LDS-zero) state as a
// critical data-plane failure.
type envoyLDSWedge struct{}

func (envoyLDSWedge) ID() string       { return "envoy.lds_wedge" }
func (envoyLDSWedge) Category() string { return "data_plane" }
func (envoyLDSWedge) Scope() string    { return "cluster" }

// Evaluate consumes the Prometheus-fed snapshot. It is a no-op when
// the required metrics are absent (cluster has no Prometheus, or the
// scrape window hasn't captured Envoy yet) and when CDS has not yet
// progressed (Envoy is still in cold init — too early to tell).
func (envoyLDSWedge) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	if snap.PromMetrics == nil {
		return nil
	}

	cdsSuccess, cdsOK := snap.PromMetrics["envoy_cds_update_success"]
	ldsAttempt, ldsAttemptOK := snap.PromMetrics["envoy_lds_update_attempt"]
	ldsSuccess, ldsSuccessOK := snap.PromMetrics["envoy_lds_update_success"]
	ldsRejected, _ := snap.PromMetrics["envoy_lds_update_rejected"]

	// Cannot evaluate without the two essential counters.
	if !cdsOK || !ldsAttemptOK {
		return nil
	}

	// Envoy is still in cold init — CDS hasn't happened yet either.
	// Reporting now would just be noise during the normal startup window.
	if cdsSuccess == 0 {
		return nil
	}

	// LDS handshake has been attempted at least once — the wedge condition
	// from invariant envoy.lds_progress_required_for_http_mesh_readiness
	// is not present. Optionally surface healthy state as INFO when LDS
	// has also succeeded; otherwise stay silent.
	if ldsAttempt > 0 {
		if ldsSuccessOK && ldsSuccess > 0 {
			return []Finding{{
				FindingID:   FindingID("envoy.lds_progress_ok", "cluster", "envoy"),
				InvariantID: "envoy.lds_progress_required_for_http_mesh_readiness",
				Severity:    cluster_doctorpb.Severity_SEVERITY_INFO,
				Category:    "data_plane",
				EntityRef:   "envoy",
				Summary: fmt.Sprintf(
					"Envoy data-plane LDS healthy — cds_success=%.0f, lds_attempt=%.0f, lds_success=%.0f",
					cdsSuccess, ldsAttempt, ldsSuccess),
				Evidence: []*cluster_doctorpb.Evidence{kvEvidence("prometheus", "envoy_lds_progress", map[string]string{
					"cds_update_success":  fmt.Sprintf("%.0f", cdsSuccess),
					"lds_update_attempt":  fmt.Sprintf("%.0f", ldsAttempt),
					"lds_update_success":  fmt.Sprintf("%.0f", ldsSuccess),
					"lds_update_rejected": fmt.Sprintf("%.0f", ldsRejected),
					"timestamp":           snap.PromTS.UTC().Format(time.RFC3339),
				})},
				InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_PASS,
			}}
		}
		return nil
	}

	// CDS has progressed but LDS has never even been attempted. This is
	// the exact failure_mode anchored as
	//   envoy.lds_update_attempt_zero_despite_cds_progress
	// and the inverse of the readiness invariant
	//   envoy.lds_progress_required_for_http_mesh_readiness.
	// Surface as CRITICAL with structured evidence so an operator can
	// trace it. Remediation is NOT this rule's job — see the report at
	// docs/awareness/reports/envoy_lds_cds_wedge.md for the upstream
	// restart-storm cause and the safe operator workaround.
	return []Finding{{
		FindingID:   FindingID("envoy.lds_wedge", "cluster", "envoy"),
		InvariantID: "envoy.lds_progress_required_for_http_mesh_readiness",
		Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
		Category:    "data_plane",
		EntityRef:   "envoy",
		Summary: fmt.Sprintf(
			"Envoy mesh WEDGED — CDS has applied %.0f update(s) but LDS update_attempt is 0; port 443 will not bind, HTTP mesh is down",
			cdsSuccess),
		Evidence: []*cluster_doctorpb.Evidence{kvEvidence("prometheus", "envoy_lds_wedge", map[string]string{
			"cds_update_success":   fmt.Sprintf("%.0f", cdsSuccess),
			"lds_update_attempt":   fmt.Sprintf("%.0f", ldsAttempt),
			"lds_update_success":   fmt.Sprintf("%.0f", ldsSuccess),
			"lds_update_rejected":  fmt.Sprintf("%.0f", ldsRejected),
			"prom_query_cds":       "max(envoy_cluster_manager_cds_update_success)",
			"prom_query_lds":       "max(envoy_listener_manager_lds_update_attempt)",
			"timestamp":            snap.PromTS.UTC().Format(time.RFC3339),
			"failure_mode_anchor":  "envoy.lds_update_attempt_zero_despite_cds_progress",
			"invariant_anchor":     "envoy.lds_progress_required_for_http_mesh_readiness",
			"see_also":             "docs/awareness/reports/envoy_lds_cds_wedge.md",
			"auto_clear_condition": "lds_update_attempt > 0",
			"do_not_auto_remediate": "true — this is a diagnostic-only rule; restart loops can deepen the wedge",
		})},
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
	}}
}
