package main

import (
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/node_agent/node_agent_server/infra_truth"
)

// seedInfraProbe puts a crafted probe result in the cache so infraReadiness reads
// it (rather than running a live probe).
func seedInfraProbe(srv *NodeAgentServer, component string, installed bool, state cluster_controllerpb.InfraLifecycleState) {
	srv.infraProbeCache.Put(component, &cluster_controllerpb.InfraProbeResult{
		Component: component,
		Installed: installed,
		Healthy:   state == cluster_controllerpb.InfraLifecycleState_INFRA_MEMBER_READY,
		Lifecycle: &cluster_controllerpb.InfraLifecycleObservation{State: state},
	}, time.Now())
}

// infraReadiness must OVERRIDE the systemd verdict only in the two confident
// directions (STALLED → not ready, MEMBER_READY → ready) and stay undecided for
// every transient/uncertain state so the caller falls back to is-active — adding
// the ability to fence a definitively-broken component without new stalls.
func TestInfraReadiness_DecisionMatrix(t *testing.T) {
	srv := &NodeAgentServer{}
	srv.ensureInfraTruth()

	// Non-infra service → undecided (caller falls back to systemd).
	if _, decided := srv.infraReadiness("gateway"); decided {
		t.Error("non-infra service must be undecided")
	}

	// STALLED (Envoy LDS wedge / etcd CORRUPT / MinIO split-brain) → not ready,
	// decided. This is the active-but-broken case the bare is-active check hides.
	seedInfraProbe(srv, infra_truth.ComponentEnvoy, true, cluster_controllerpb.InfraLifecycleState_INFRA_STALLED)
	if ready, decided := srv.infraReadiness("envoy"); !decided || ready {
		t.Errorf("STALLED envoy: got ready=%v decided=%v, want ready=false decided=true", ready, decided)
	}

	// MEMBER_READY → ready, decided.
	seedInfraProbe(srv, infra_truth.ComponentEtcd, true, cluster_controllerpb.InfraLifecycleState_INFRA_MEMBER_READY)
	if ready, decided := srv.infraReadiness("etcd"); !decided || !ready {
		t.Errorf("MEMBER_READY etcd: got ready=%v decided=%v, want ready=true decided=true", ready, decided)
	}

	// DEGRADED → undecided: do not make readiness stricter on transient evidence;
	// fall back to the systemd check (no new convergence stall).
	seedInfraProbe(srv, infra_truth.ComponentMinio, true, cluster_controllerpb.InfraLifecycleState_INFRA_DEGRADED)
	if _, decided := srv.infraReadiness("minio"); decided {
		t.Error("DEGRADED minio must be undecided (fall back to systemd)")
	}

	// Warming up (DAEMON_STARTING) → undecided.
	seedInfraProbe(srv, infra_truth.ComponentEtcd, true, cluster_controllerpb.InfraLifecycleState_INFRA_DAEMON_STARTING)
	if _, decided := srv.infraReadiness("etcd"); decided {
		t.Error("DAEMON_STARTING etcd must be undecided (still warming up)")
	}

	// Not installed → undecided (defer to the systemd check).
	seedInfraProbe(srv, infra_truth.ComponentScylla, false, cluster_controllerpb.InfraLifecycleState_INFRA_NOT_PRESENT)
	if _, decided := srv.infraReadiness("scylladb"); decided {
		t.Error("not-installed scylladb must be undecided")
	}
}
