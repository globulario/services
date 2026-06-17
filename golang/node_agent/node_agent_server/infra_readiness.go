package main

import (
	"context"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/node_agent/node_agent_server/infra_truth"
	"github.com/globulario/services/golang/workflow/engine"
)

// infraReadinessProbeTimeout bounds the readiness lookup. The probe is served
// from the warm cache in the common case, so this only matters on a cold cache.
const infraReadinessProbeTimeout = 3 * time.Second

// isServiceReady is the workflow engine's IsServiceActive callback. The default
// (engine.DefaultIsServiceActive) treats a service as "ready to accept traffic"
// when `systemctl is-active` reports active — which is blind to the runtime
// states the infra truth plane detects (etcd without a leader, MinIO split-brain,
// an Envoy LDS wedge: the unit is active but the component is not serving).
//
// For the components covered by the truth plane this OVERRIDES the systemd-only
// verdict in the two CONFIDENT directions only:
//   - lifecycle STALLED  → NOT ready (the active-but-broken case is-active hides)
//   - lifecycle MEMBER_READY → ready
//
// For every other state — warming up (DAEMON_STARTING/JOINING), DEGRADED, not
// installed, or the probe could not observe — it falls back to the systemd check.
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
// component and whether the probe was confident enough to decide it. decided is
// false for non-infra services and for any inconclusive/transient lifecycle, so
// the caller falls back to engine.DefaultIsServiceActive.
func (srv *NodeAgentServer) infraReadiness(name string) (ready bool, decided bool) {
	var probe func(context.Context, bool) *cluster_controllerpb.InfraProbeResult
	switch strings.ToLower(strings.TrimSpace(name)) {
	case infra_truth.ComponentScylla:
		probe = srv.scyllaInfraProbe
	case infra_truth.ComponentEtcd:
		probe = srv.etcdInfraProbe
	case infra_truth.ComponentMinio:
		probe = srv.minioInfraProbe
	case infra_truth.ComponentEnvoy:
		probe = srv.envoyInfraProbe
	default:
		return false, false
	}

	ctx, cancel := context.WithTimeout(context.Background(), infraReadinessProbeTimeout)
	defer cancel()
	res := probe(ctx, false) // cached; cheap unless the cache is cold
	if res == nil || !res.GetInstalled() {
		return false, false // not installed / no result → defer to the systemd check
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
