// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.etcd_rejoin
// @awareness file_role=preflight_validator_for_destructive_etcd_data_dir_wipe_and_rejoin
// @awareness enforces=globular.platform:invariant.destructive_actions.require_explicit_guard
// @awareness risk=critical
package main

// etcd_rejoin.go — pure preflight validator. Returns
// EtcdRejoinPrechecks{Valid: true} only when ALL four conditions
// hold: node exists in cluster state, EtcdJoinPhase is
// rejoin_required, other healthy members exist (quorum safe), and
// the node-agent heartbeat is recent.
//
// MUST NOT mutate any state. The caller (the rejoin RPC handler)
// only proceeds with the destructive data-dir wipe when Valid()
// returns true. Loosening any precondition here — especially
// "other healthy members exist" — risks wiping the last working
// etcd data directory and losing the cluster.

import (
	"fmt"
	"time"
)

// EtcdRejoinPrechecks holds the result of pre-flight validation before
// authorising a destructive etcd data-directory wipe and rejoin.
type EtcdRejoinPrechecks struct {
	NodeFound      bool  // the node exists in cluster state
	InRejoinState  bool  // EtcdJoinPhase == rejoin_required
	NotSoleMember  bool  // other healthy etcd members exist (quorum safe)
	AgentReachable bool  // node-agent heartbeat is recent (< 2 min)
	Error          error // first failed precondition, or nil
}

// Valid returns true only if all preconditions passed.
func (p EtcdRejoinPrechecks) Valid() bool {
	return p.NodeFound && p.InRejoinState && p.NotSoleMember && p.AgentReachable
}

// validateEtcdRejoinPreconditions checks all preconditions before allowing a
// destructive etcd data-dir wipe and rejoin. No state is modified.
//
// allNodes is the full set of cluster nodes, used to count other healthy
// etcd members so we can refuse the operation if quorum would be lost.
func validateEtcdRejoinPreconditions(node *nodeState, allNodes []*nodeState) EtcdRejoinPrechecks {
	if node == nil {
		return EtcdRejoinPrechecks{Error: fmt.Errorf("node not found")}
	}
	checks := EtcdRejoinPrechecks{NodeFound: true}

	// Must be in rejoin_required state — confirms detection has fired.
	checks.InRejoinState = node.EtcdJoinPhase == EtcdJoinRejoinRequired
	if !checks.InRejoinState {
		checks.Error = fmt.Errorf("node %s is not in rejoin_required state (current: %s); "+
			"only nodes that the controller has classified as permanently stuck may be repaired",
			node.Identity.Hostname, node.EtcdJoinPhase)
		return checks
	}

	// Refuse if this node is the sole healthy etcd member — wiping its data dir
	// would destroy cluster quorum with no recovery path.
	healthyPeers := 0
	for _, n := range allNodes {
		if n == nil || n.NodeID == node.NodeID {
			continue
		}
		if n.EtcdJoinPhase == EtcdJoinVerified && nodeHasEtcdRunning(n) {
			healthyPeers++
		}
	}
	checks.NotSoleMember = healthyPeers > 0
	if !checks.NotSoleMember {
		checks.Error = fmt.Errorf("node %s is the sole healthy etcd member; "+
			"wiping its data directory would destroy cluster quorum; "+
			"restore at least one other etcd member before repairing this node",
			node.Identity.Hostname)
		return checks
	}

	// Node-agent must be reachable (recent heartbeat) so the workflow can
	// deliver the wipe-and-rejoin commands to the node.
	checks.AgentReachable = !node.LastSeen.IsZero() && time.Since(node.LastSeen) < 2*time.Minute
	if !checks.AgentReachable {
		var ago string
		if node.LastSeen.IsZero() {
			ago = "never"
		} else {
			ago = time.Since(node.LastSeen).Round(time.Second).String() + " ago"
		}
		checks.Error = fmt.Errorf("node %s agent is not reachable (last seen: %s); "+
			"the repair workflow cannot be delivered without an active agent connection",
			node.Identity.Hostname, ago)
	}

	return checks
}

// markEtcdRejoinInProgress validates preconditions and transitions a node to
// EtcdJoinRejoinInProgress, signalling that a repair workflow is running.
// The caller must hold the controller state lock and persist state afterward.
//
// This is the controller-side entry point for the
// "globular node repair-etcd --node <hostname> --wipe-local-etcd" command.
func markEtcdRejoinInProgress(node *nodeState, allNodes []*nodeState) error {
	checks := validateEtcdRejoinPreconditions(node, allNodes)
	if !checks.Valid() {
		return checks.Error
	}
	node.EtcdJoinPhase = EtcdJoinRejoinInProgress
	node.EtcdJoinError = ""
	return nil
}
