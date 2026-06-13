package infra_truth

import (
	"strings"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// lifecycleLabel maps the FSM enum to a stable human label.
func lifecycleLabel(s cluster_controllerpb.InfraLifecycleState) string {
	switch s {
	case cluster_controllerpb.InfraLifecycleState_INFRA_NOT_PRESENT:
		return "not_present"
	case cluster_controllerpb.InfraLifecycleState_INFRA_PACKAGE_INSTALLED:
		return "package_installed"
	case cluster_controllerpb.InfraLifecycleState_INFRA_CONFIG_RENDERED:
		return "config_rendered"
	case cluster_controllerpb.InfraLifecycleState_INFRA_CONFIG_ATTESTED:
		return "config_attested"
	case cluster_controllerpb.InfraLifecycleState_INFRA_DAEMON_STARTING:
		return "daemon_starting"
	case cluster_controllerpb.InfraLifecycleState_INFRA_LOCAL_API_READY:
		return "local_api_ready"
	case cluster_controllerpb.InfraLifecycleState_INFRA_CQL_READY:
		return "cql_ready"
	case cluster_controllerpb.InfraLifecycleState_INFRA_CLUSTER_CONTACTED:
		return "cluster_contacted"
	case cluster_controllerpb.InfraLifecycleState_INFRA_JOINING:
		return "joining"
	case cluster_controllerpb.InfraLifecycleState_INFRA_SCHEMA_AGREEING:
		return "schema_agreeing"
	case cluster_controllerpb.InfraLifecycleState_INFRA_MEMBER_READY:
		return "member_ready"
	case cluster_controllerpb.InfraLifecycleState_INFRA_DEGRADED:
		return "degraded"
	case cluster_controllerpb.InfraLifecycleState_INFRA_STALLED:
		return "stalled"
	default:
		return "unknown"
	}
}

// hasSeverity reports whether any violation carries the given severity.
func hasSeverity(vs []*cluster_controllerpb.InfraViolation, sev string) bool {
	for _, v := range vs {
		if v.GetSeverity() == sev {
			return true
		}
	}
	return false
}

func firstCriticalMessage(vs []*cluster_controllerpb.InfraViolation) string {
	for _, v := range vs {
		if v.GetSeverity() == SeverityCritical {
			return v.GetMessage()
		}
	}
	return ""
}

// deriveScyllaLifecycle computes the lifecycle FSM observation from the four
// truth sources. It is pure and deterministic: state_age is 0 in Phase 1 because
// no transition history is persisted yet (the field is present so consumers can
// start reading it now and we can fill it later from
// /var/lib/globular/state/infra_lifecycle.jsonl).
//
// Stall semantics: a STALLED state means the component cannot make progress on
// its own and needs operator/owner intervention — never "give it more time".
func deriveScyllaLifecycle(
	installed bool,
	desired *InfraDesiredState,
	rendered *ScyllaRenderedConfig,
	runtime *ScyllaRuntimeState,
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
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_NOT_PRESENT, "ScyllaDB package is not installed")
	}
	if rendered == nil || !rendered.Present {
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_PACKAGE_INSTALLED, "config /etc/scylla/scylla.yaml not rendered")
	}

	hasCritical := hasSeverity(violations, SeverityCritical)
	daemonActive := runtime != nil && runtime.DaemonActive

	// Rule: a daemon running on top of a critically-invalid config is STALLED —
	// "active" must never mask a broken config (partial_failure_hidden_by_global_green).
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

	// Config attested (no critical violations beyond this point).
	if !daemonActive {
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_CONFIG_ATTESTED, "config attested; scylla-server unit is not active")
	}
	if runtime == nil || !runtime.RESTAPIReady {
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_DAEMON_STARTING, "daemon active; REST API not answering yet")
	}

	mode := strings.ToUpper(strings.TrimSpace(runtime.OperationMode))
	joining := desired != nil && desired.BootstrapIntent == BootstrapJoining

	// Bootstrap streaming finished but the ring has not converged → STALLED.
	if runtime.BootstrapProgress >= 100 && runtime.GossipLive == 0 {
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_STALLED,
			"bootstrap progress is 100% but no live gossip peers — node streamed but did not converge into the ring")
	}

	if mode == "JOINING" || mode == "BOOTSTRAP" {
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_JOINING, "node is joining/bootstrapping the ring")
	}

	if !runtime.CQLReady {
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_LOCAL_API_READY, "REST API ready; CQL not yet accepting queries")
	}

	// CQL is ready. For a joining node, an empty observed-peer set means it came
	// up isolated — it answers CQL as a one-node ring instead of joining → STALLED.
	if joining && len(runtime.ObservedPeers) == 0 {
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_STALLED,
			"CQL is ready but no peers are observed on a joining node — it formed an isolated single-node ring")
	}

	if len(runtime.ObservedPeers) == 0 {
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_CQL_READY, "CQL ready; no peers contacted yet")
	}

	if mode != "" && mode != "NORMAL" {
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_SCHEMA_AGREEING, "peers contacted; operation mode is "+mode)
	}

	// NORMAL operation mode, peers observed, no critical violations → full member.
	// Downgrade to DEGRADED if there are non-critical (ERROR) violations.
	if hasSeverity(violations, SeverityError) {
		return set(cluster_controllerpb.InfraLifecycleState_INFRA_DEGRADED, "member is serving but has unresolved config errors")
	}
	return set(cluster_controllerpb.InfraLifecycleState_INFRA_MEMBER_READY, "")
}
