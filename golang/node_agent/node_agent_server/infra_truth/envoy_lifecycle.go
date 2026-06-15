package infra_truth

import (
	"fmt"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// deriveEnvoyLifecycle computes the lifecycle FSM observation for the Envoy data
// plane. It is pure and deterministic. state_age is 0 in Phase 4 (no persisted
// transition history yet).
//
// The headline failure is the LDS wedge — CDS has applied at least one update but
// LDS update_attempt is still 0. In that state Envoy has clusters but no
// listeners; port 443 never binds and the HTTP mesh is dead even though
// `systemctl is-active globular-envoy.service` reports active. It maps to STALLED
// (needs the owner/operator, not more time — and per the existing incident the
// root cause is an upstream restart storm, so it must NOT be auto-restarted). It
// pins invariant envoy.lds_progress_required_for_http_mesh_readiness.
func deriveEnvoyLifecycle(
	installed bool,
	rendered *EnvoyRenderedConfig,
	runtime *EnvoyRuntimeState,
	violations []*cluster_controllerpb.InfraViolation,
	now int64,
) *cluster_controllerpb.InfraLifecycleObservation {
	obs := &cluster_controllerpb.InfraLifecycleObservation{
		ObservedAtUnix:  now,
		StateAgeSeconds: 0,
	}
	set := func(s cluster_controllerpb.InfraLifecycleState, blocking string) *cluster_controllerpb.InfraLifecycleObservation {
		obs.State = s
		obs.StateLabel = lifecycleLabel(s)
		obs.BlockingReason = blocking
		return obs
	}

	if !installed {
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_NOT_PRESENT, "Envoy package is not installed")
	}
	if rendered == nil || !rendered.Present {
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_PACKAGE_INSTALLED, "bootstrap /run/globular/envoy/envoy-bootstrap.json not written yet (gateway writes it on startup)")
	}

	hasCritical := hasSeverity(violations, SeverityCritical)
	daemonActive := runtime != nil && runtime.DaemonActive

	// A daemon running on a critically-invalid bootstrap (no ads/lds config, ADS
	// cluster undefined) is STALLED — "active" must never mask a dead mesh.
	if hasCritical {
		reason := firstCriticalMessage(violations)
		if reason == "" {
			reason = "bootstrap has a critical violation"
		}
		if daemonActive {
			reason = "daemon is active but " + reason
		}
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_STALLED, reason)
	}

	if !daemonActive {
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_CONFIG_ATTESTED, "bootstrap attested; "+envoyUnitName+" unit is not active")
	}

	if runtime == nil || !runtime.AdminReachable {
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_DAEMON_STARTING, "daemon active; admin API not answering yet")
	}

	// THE LDS WEDGE — CDS progressed but LDS was never attempted. Mirrors the
	// Prometheus-fed envoy.lds_wedge rule, observed here per-node from the admin
	// API. STALLED, not DEGRADED: it cannot clear on its own and auto-restart can
	// deepen it.
	if runtime.CDSUpdateSuccess > 0 && runtime.LDSUpdateAttempt == 0 {
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_STALLED,
			fmt.Sprintf("Envoy mesh WEDGED — CDS applied %d update(s) but LDS update_attempt is 0; port 443 will not bind, HTTP mesh is down. Likely an upstream restart storm — do NOT auto-restart.", runtime.CDSUpdateSuccess))
	}

	// Admin up but xDS has not delivered cluster config yet — still warming.
	if runtime.CDSUpdateSuccess == 0 {
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_LOCAL_API_READY, "admin reachable; xDS has not delivered any cluster config yet (warming)")
	}

	// CDS and LDS have both progressed. Rejected listener config is a real degrade.
	if runtime.LDSUpdateRejected > 0 {
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_DEGRADED,
			fmt.Sprintf("Envoy rejected %d LDS update(s) — xDS is sending invalid listener config", runtime.LDSUpdateRejected))
	}

	// LDS attempted but no listener is active — port 443 is not actually serving.
	if runtime.ActiveListeners == 0 {
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_DEGRADED,
			"LDS has been attempted but no listener is active — port 443 is not bound")
	}

	if hasSeverity(violations, SeverityError) {
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_DEGRADED, "data plane is serving but the bootstrap has unresolved config errors")
	}

	// Clusters and listeners active, but /ready not yet 200 (server still draining
	// or initializing): impaired but not wedged.
	if !runtime.Ready {
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_DEGRADED,
			fmt.Sprintf("clusters and listeners active but /ready is not 200 (server_state=%s)", runtime.ServerState))
	}

	return set(cluster_controllerpb.InfraLifecycleState_INFRA_MEMBER_READY, "")
}
