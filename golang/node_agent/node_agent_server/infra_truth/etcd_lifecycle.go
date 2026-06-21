package infra_truth

import (
	"strings"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// etcdHasAlarm reports whether the runtime carries an alarm whose name contains
// the given token (case-insensitive), e.g. "CORRUPT" or "NOSPACE".
func etcdHasAlarm(rt *EtcdRuntimeState, token string) bool {
	if rt == nil {
		return false
	}
	for _, a := range rt.Alarms {
		if strings.Contains(strings.ToUpper(a), token) {
			return true
		}
	}
	return false
}

// deriveEtcdLifecycle computes the lifecycle FSM observation from the four truth
// sources. It is pure and deterministic. state_age is 0 in Phase 2 (no persisted
// transition history yet).
//
// Stall semantics: a STALLED state means the component cannot make progress on
// its own and needs operator/owner intervention — never "give it more time". A
// leaderless cluster or an in-progress learner is DEGRADED/JOINING, not STALLED,
// because those can still converge on their own.
func deriveEtcdLifecycle(
	installed bool,
	desired *InfraDesiredState,
	rendered *EtcdRenderedConfig,
	runtime *EtcdRuntimeState,
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
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_NOT_PRESENT, "etcd package is not installed")
	}
	if rendered == nil || !rendered.Present {
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_PACKAGE_INSTALLED, "config /var/lib/globular/config/etcd.yaml not rendered")
	}

	hasCritical := hasSeverity(violations, SeverityCritical)
	daemonActive := runtime != nil && runtime.DaemonActive

	// A daemon running on top of a critically-invalid config is STALLED — "active"
	// must never mask a broken config.
	if hasCritical {
		reason := firstCriticalMessage(violations)
		if reason == "" {
			reason = "config has a critical violation"
		}
		if daemonActive {
			reason = "daemon is active but " + reason
		}
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_STALLED, reason)
	}

	if !daemonActive {
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_CONFIG_ATTESTED, "config attested; "+etcdUnitName+" unit is not active")
	}

	// CORRUPT is a hard, operator-intervention-required state: a corrupted member
	// cannot heal itself and must be wiped+rejoined by the controller workflow.
	if etcdHasAlarm(runtime, "CORRUPT") {
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_STALLED,
			"etcd raised a CORRUPT alarm — this member's data is damaged and needs a controller-driven wipe+rejoin, never more time")
	}

	if runtime == nil || !runtime.LocalReachable {
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_DAEMON_STARTING, "daemon active; local v3 API not answering yet")
	}

	// A learner is mid-join (replicating the log before promotion to voter).
	if runtime.IsLearner {
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_JOINING, "member is a learner — replicating the raft log, not yet promoted to voter")
	}

	// Reachable but no raft leader: election in progress or quorum lost. This can
	// still recover on its own, so DEGRADED, not STALLED.
	if !runtime.HasLeader {
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_DEGRADED,
			"member is reachable but the cluster has no raft leader — election in progress or quorum lost")
	}

	// NOSPACE: the backend quota is exceeded; writes are blocked until compaction
	// and defrag free space. Serving reads but impaired.
	if etcdHasAlarm(runtime, "NOSPACE") {
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_DEGRADED,
			"etcd raised a NOSPACE alarm — backend quota exceeded; writes are blocked until compaction/defrag")
	}

	// Has leader, voter, no alarms. Downgrade to DEGRADED if there are non-critical
	// (ERROR) config violations.
	if hasSeverity(violations, SeverityError) {
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_DEGRADED, "member is serving but has unresolved config errors")
	}
	return set(cluster_controllerpb.InfraLifecycleState_INFRA_MEMBER_READY, "")
}
