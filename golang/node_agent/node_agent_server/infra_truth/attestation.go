package infra_truth

import (
	"fmt"
	"strings"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// remediationOwner is the standard remediation string. It enforces the rule
// "config files are artifacts, not authority": the fix targets the OWNER that
// generated the bad config (the controller-provided desired state / the package
// post-install renderer), never a manual edit of /etc/scylla/scylla.yaml.
const remediationOwner = "Repair the owner that generated this config: the ScyllaDB post-install renderer and the controller-provided desired state (seeds/listen address). Do NOT hand-edit /etc/scylla/scylla.yaml as the permanent fix — a render will overwrite it."

// AttestScyllaConfig runs every attestation rule against the rendered config and
// returns the violations found (empty == config valid). It attests nothing when
// the config file is absent — that is a lifecycle fact (not yet rendered), not a
// config violation.
//
// runtime may be nil (e.g. before the daemon is up, or in pure static tests). It
// is consulted only by rules whose predicted harm is an empirical runtime fact —
// notably the self-only-seed "isolated ring" prediction, which is refuted once the
// node is observed as a converged member of a multi-node ring. A nil runtime means
// "no runtime proof", so those rules fire exactly as the static config implies.
func AttestScyllaConfig(desired *InfraDesiredState, rendered *ScyllaRenderedConfig, runtime *ScyllaRuntimeState) []*cluster_controllerpb.InfraViolation {
	if rendered == nil || !rendered.Present {
		return nil
	}

	var v []*cluster_controllerpb.InfraViolation
	add := func(viol *cluster_controllerpb.InfraViolation) {
		if viol != nil {
			v = append(v, viol)
		}
	}

	// Cluster-facing addresses must not be loopback. listen_address and the
	// broadcast addresses additionally must not be the unspecified address
	// (0.0.0.0/::) — a cluster member must advertise a routable address.
	add(attestNoLoopback("listen_address", rendered.ListenAddress, true))
	add(attestNoLoopback("rpc_address", rendered.RPCAddress, false))
	add(attestNoLoopback("broadcast_address", rendered.BroadcastAddress, true))
	add(attestNoLoopback("broadcast_rpc_address", rendered.BroadcastRPCAddress, true))
	// api_address is intentionally allowed to be loopback (local admin REST API).

	add(attestClusterName(desired, rendered))
	add(attestSeeds(desired, rendered, runtime))
	add(attestAddressMatchesLocalNode(desired, rendered))

	return v
}

// attestNoLoopback returns a CRITICAL violation if value is a loopback address
// (or the unspecified address when forbidUnspecified is set). An empty value is
// not attested here — emptiness of a required address is config-completeness,
// handled elsewhere. Returns nil when the address is acceptable.
func attestNoLoopback(field, value string, forbidUnspecified bool) *cluster_controllerpb.InfraViolation {
	val := stripQuotes(value)
	if val == "" {
		return nil
	}
	switch {
	case isLoopback(val):
		return newViolation(
			"scylla.loopback_forbidden",
			SeverityCritical,
			fmt.Sprintf("%s is a loopback address (%s) — a cluster member must not advertise loopback; peers can never reach it", field, val),
			fmt.Sprintf("%s=%s", field, val),
			remediationOwner,
		)
	case forbidUnspecified && isUnspecified(val):
		return newViolation(
			"scylla.loopback_forbidden",
			SeverityCritical,
			fmt.Sprintf("%s is the unspecified address (%s) — a cluster member must advertise a concrete routable address", field, val),
			fmt.Sprintf("%s=%s", field, val),
			remediationOwner,
		)
	}
	return nil
}

// attestClusterName requires a non-empty cluster name that matches desired.
func attestClusterName(desired *InfraDesiredState, rendered *ScyllaRenderedConfig) *cluster_controllerpb.InfraViolation {
	if rendered.ClusterName == "" {
		return newViolation(
			"scylla.config_valid",
			SeverityError,
			"cluster_name is empty — ScyllaDB will refuse to form/join a cluster",
			"cluster_name=\"\"",
			remediationOwner,
		)
	}
	if desired != nil && desired.ExpectedClusterName != "" && rendered.ClusterName != desired.ExpectedClusterName {
		return newViolation(
			"scylla.config_valid",
			SeverityError,
			fmt.Sprintf("cluster_name %q does not match desired %q — a mismatched node cannot join the ring", rendered.ClusterName, desired.ExpectedClusterName),
			fmt.Sprintf("rendered=%q desired=%q", rendered.ClusterName, desired.ExpectedClusterName),
			remediationOwner,
		)
	}
	return nil
}

// attestSeeds checks the seed list against bootstrap intent. A joining node must
// have at least one expected non-self seed; a self-only seed list is only valid
// for an explicit first-node bootstrap.
//
// The self-only-seed rule predicts a *future* event ("it WILL bootstrap an
// isolated single-node ring"). That prediction must be reconciled against runtime
// truth: once the node is observed as a converged member of a multi-node ring
// (operation_mode NORMAL with a live non-self gossip peer), the bootstrap already
// happened and did NOT isolate the node — keeping it an ERROR is a false positive
// that drags a healthy member to DEGRADED and can trigger needless remediation of
// a working ring. In that case the rule is downgraded to an INFO seed-hygiene note
// rather than silenced, because a future wiped restart with self-only seeds would
// still re-isolate the node. Before bootstrap (runtime nil / not yet NORMAL) the
// rule stays an ERROR so a config that would isolate a fresh node is caught early
// (infra.config_must_be_attested_before_start).
func attestSeeds(desired *InfraDesiredState, rendered *ScyllaRenderedConfig, runtime *ScyllaRuntimeState) *cluster_controllerpb.InfraViolation {
	if len(rendered.Seeds) == 0 {
		return newViolation(
			"scylla.config_valid",
			SeverityError,
			"seeds is empty — ScyllaDB has no contact point to discover the ring",
			"seeds=\"\"",
			remediationOwner,
		)
	}
	if desired == nil {
		return nil
	}

	self := ""
	if len(desired.ExpectedListenAddresses) > 0 {
		self = desired.ExpectedListenAddresses[0]
	}
	hasNonSelfSeed := hasNonSelf(rendered.Seeds, self)

	if desired.BootstrapIntent == BootstrapJoining && !hasNonSelfSeed {
		if scyllaProvenEstablishedMember(desired, runtime) {
			return newViolation(
				"scylla.config_valid",
				SeverityInfo,
				fmt.Sprintf("seeds contains only self (%s); the node is already a NORMAL member of a multi-node ring so it is not isolated now, but a wiped restart with self-only seeds would re-bootstrap an isolated ring — add a non-self seed for restart safety", strings.Join(rendered.Seeds, ",")),
				fmt.Sprintf("seeds=%s bootstrap_intent=%s operation_mode=%s observed_peers=%s", strings.Join(rendered.Seeds, ","), desired.BootstrapIntent, runtime.OperationMode, strings.Join(runtime.ObservedPeers, ",")),
				remediationOwner,
			)
		}
		return newViolation(
			"scylla.config_valid",
			SeverityError,
			fmt.Sprintf("seeds contains only self (%s) but this is a joining node — it will bootstrap an isolated single-node ring instead of joining the cluster", strings.Join(rendered.Seeds, ",")),
			fmt.Sprintf("seeds=%s bootstrap_intent=%s", strings.Join(rendered.Seeds, ","), desired.BootstrapIntent),
			remediationOwner,
		)
	}
	return nil
}

// scyllaProvenEstablishedMember reports whether runtime truth proves this node is
// already a converged member of a MULTI-node ring: operation_mode NORMAL with at
// least one live non-self gossip peer. This is the empirical refutation of the
// "joining node will bootstrap an isolated single-node ring" prediction. A node
// that genuinely isolated itself also reports NORMAL, but with only itself in live
// gossip — that is NOT established membership, so the violation must stand. A nil
// runtime (no proof yet) is never "established".
func scyllaProvenEstablishedMember(desired *InfraDesiredState, runtime *ScyllaRuntimeState) bool {
	if runtime == nil {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(runtime.OperationMode), "NORMAL") {
		return false
	}
	self := ""
	if desired != nil && len(desired.ExpectedListenAddresses) > 0 {
		self = desired.ExpectedListenAddresses[0]
	}
	return hasNonSelf(runtime.ObservedPeers, self)
}

// attestAddressMatchesLocalNode verifies the rendered listen address is one of
// the addresses Globular expects this node to advertise. A non-loopback mismatch
// means the node will advertise the wrong identity to the ring.
func attestAddressMatchesLocalNode(desired *InfraDesiredState, rendered *ScyllaRenderedConfig) *cluster_controllerpb.InfraViolation {
	if desired == nil || len(desired.ExpectedListenAddresses) == 0 {
		return nil
	}
	la := stripQuotes(rendered.ListenAddress)
	if la == "" || isLoopback(la) || isUnspecified(la) {
		return nil // emptiness/loopback are reported by the dedicated rules
	}
	for _, exp := range desired.ExpectedListenAddresses {
		if stripQuotes(exp) == la {
			return nil
		}
	}
	return newViolation(
		"scylla.config_valid",
		SeverityError,
		fmt.Sprintf("listen_address %s is not one of this node's expected addresses %s — the node will advertise the wrong identity", la, strings.Join(desired.ExpectedListenAddresses, ",")),
		fmt.Sprintf("listen_address=%s expected=%s", la, strings.Join(desired.ExpectedListenAddresses, ",")),
		remediationOwner,
	)
}
