package main

import (
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/node_agent/node_agent_server/infra_truth"
	"github.com/globulario/services/golang/workflow/engine"
)

// infraReadinessMaxAge bounds how stale a cached infra probe may be and still be
// trusted as a confident readiness verdict. Beyond it we fall back to the systemd
// check rather than assert PASS/FAIL on stale evidence.
const infraReadinessMaxAge = 2 * time.Minute

// isServiceReady is the workflow engine's IsServiceActive callback. The default
// (engine.DefaultIsServiceActive) treats a service as "ready to accept traffic"
// when `systemctl is-active` reports active — which is blind to the runtime
// states the infra truth plane detects (etcd active without a leader, MinIO
// split-brain, an Envoy LDS wedge: the unit is active but the component is not
// serving).
//
// For the components covered by the truth plane this OVERRIDES the systemd-only
// verdict in the two CONFIDENT directions only:
//   - lifecycle STALLED  → NOT ready (the active-but-broken case is-active hides)
//   - lifecycle MEMBER_READY → ready
//
// For every other state — warming up (DAEMON_STARTING/JOINING), DEGRADED, not
// installed, or no fresh cached probe — it falls back to the systemd check.
// Readiness is therefore never made stricter on transient or uncertain evidence,
// so the change adds the ability to fence a definitively-broken component without
// introducing new convergence stalls. Closes infra.process_active_is_not_health
// at the workflow readiness gate.
func (srv *NodeAgentServer) isServiceReady(name string) bool {
	if ready, decided := srv.infraReadiness(name); decided {
		return ready
	}
	return engine.DefaultIsServiceActive(name)
}

// infraReadiness returns the truth-plane readiness verdict for an infrastructure
// component and whether the cached probe was confident enough to decide it.
// decided is false for non-infra services and for any inconclusive/transient
// lifecycle, so the caller falls back to engine.DefaultIsServiceActive.
func (srv *NodeAgentServer) infraReadiness(name string) (ready bool, decided bool) {
	component := infraComponentForService(name)
	if component == "" {
		return false, false
	}
	res := srv.cachedInfraProbe(component)
	if res == nil || !res.GetInstalled() {
		return false, false // no fresh truth → defer to the systemd check
	}

	switch res.GetLifecycle().GetState() {
	case cluster_controllerpb.InfraLifecycleState_INFRA_STALLED:
		// Active but definitively not serving — wedge / corrupt / split-brain /
		// critically-invalid config. Block readiness; this is the case the bare
		// systemd check hides.
		return false, true
	case cluster_controllerpb.InfraLifecycleState_INFRA_MEMBER_READY:
		return true, true
	default:
		// Warming up, degraded, or unobserved — do not make readiness stricter on
		// transient/uncertain evidence; fall back to the systemd check.
		return false, false
	}
}

// infraComponentStalled reports whether the cached infra probe for a component
// shows the STALLED lifecycle — active but definitively not serving (an Envoy LDS
// wedge, etcd CORRUPT / no quorum, MinIO split-brain, or a critically-invalid
// config). Cache-only: a missing or stale entry returns false so a caller never
// blocks or fails on absent evidence (it keeps the systemd-only behaviour).
func (srv *NodeAgentServer) infraComponentStalled(component string) bool {
	res := srv.cachedInfraProbe(component)
	return res != nil && res.GetInstalled() &&
		res.GetLifecycle().GetState() == cluster_controllerpb.InfraLifecycleState_INFRA_STALLED
}

// envoyDataPlaneStalled reports whether the local Envoy data plane is STALLED —
// an LDS wedge (CDS applied but listeners never load) or a critically-invalid
// bootstrap: active but not serving traffic. STALLED-only, so it fences a wedged
// mesh without false-negating while Envoy is still warming up.
func (srv *NodeAgentServer) envoyDataPlaneStalled() bool {
	return srv.infraComponentStalled(infra_truth.ComponentEnvoy)
}

// cachedInfraProbe returns the most recent cached probe for a component WITHOUT
// triggering a live probe. Readiness is a hot path and must never block on a
// native-API call — the background refresher (startInfraProbeRefresher) keeps the
// cache warm. A missing or stale (> infraReadinessMaxAge) entry returns nil so
// the caller falls back to the systemd check rather than assert a verdict on
// absent or stale evidence.
func (srv *NodeAgentServer) cachedInfraProbe(component string) *cluster_controllerpb.InfraProbeResult {
	srv.ensureInfraTruth()
	res, at, ok := srv.infraProbeCache.Get(component)
	if !ok || res == nil {
		return nil
	}
	if time.Since(at) > infraReadinessMaxAge {
		return nil
	}
	return res
}

// infraComponentForService maps a workflow service name to its infra truth-plane
// component, or "" when the service is not covered by the truth plane.
func infraComponentForService(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case infra_truth.ComponentScylla:
		return infra_truth.ComponentScylla
	case infra_truth.ComponentEtcd:
		return infra_truth.ComponentEtcd
	case infra_truth.ComponentMinio:
		return infra_truth.ComponentMinio
	case infra_truth.ComponentEnvoy:
		return infra_truth.ComponentEnvoy
	default:
		return ""
	}
}
