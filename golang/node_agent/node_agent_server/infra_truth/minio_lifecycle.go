package infra_truth

import (
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// deriveMinioLifecycle computes the lifecycle FSM observation from the four truth
// sources. It is pure and deterministic. state_age is 0 in Phase 3 (no persisted
// transition history yet).
//
// MinIO has no gradual "join" like an etcd learner — a node either renders the
// correct pool topology or it forms an isolated/divergent store. So the error
// states are STALLED (critical config: split-brain or loopback — needs the owner
// fixed, never more time) and DEGRADED (live but write/read quorum lost — the
// pool can recover when peers return).
func deriveMinioLifecycle(
	installed bool,
	desired *MinioDesired,
	rendered *MinioRenderedConfig,
	runtime *MinioRuntimeState,
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
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_NOT_PRESENT, "MinIO package is not installed")
	}
	if rendered == nil || !rendered.Present {
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_PACKAGE_INSTALLED, "config /var/lib/globular/minio/minio.env not rendered")
	}

	hasCritical := hasSeverity(violations, SeverityCritical)
	daemonActive := runtime != nil && runtime.DaemonActive

	// A daemon running on top of a critically-invalid topology (split-brain
	// standalone-in-cluster, loopback endpoint, or drive-count mismatch) is
	// STALLED — "active" must never mask a topology that diverges from the pool.
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
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_CONFIG_ATTESTED, "config attested; "+minioUnitName+" unit is not active")
	}

	if runtime == nil || !runtime.Live {
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_DAEMON_STARTING, "daemon active; health endpoint not answering yet")
	}

	// Live but the pool has lost write quorum: uploads fail. Recoverable when
	// peers/drives return, so DEGRADED, not STALLED.
	if !runtime.WriteQuorum {
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_DEGRADED,
			"MinIO is live but the pool has lost write quorum — uploads will fail until enough drives/peers return")
	}

	// Write quorum but no read quorum is rare (more drives down than the read set
	// tolerates); still serving writes but reads are impaired.
	if !runtime.ReadQuorum {
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_DEGRADED,
			"MinIO has write quorum but lost read quorum — some reads will fail until drives/peers return")
	}

	// Live, write+read quorum. Downgrade to DEGRADED if there are non-critical
	// (ERROR) config violations.
	if hasSeverity(violations, SeverityError) {
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_DEGRADED, "member is serving but has unresolved config errors")
	}
	return set(cluster_controllerpb.InfraLifecycleState_INFRA_MEMBER_READY, "")
}
