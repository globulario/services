package main

import "strings"

// activeDay1JoinNode reports whether a node is still in the Day-1 bootstrap
// lane. While any node is in this lane, cluster-wide drift remediation must be
// scoped to the joining node(s); otherwise a membership change can fan out
// unrelated package re-apply/restart work onto already-serving nodes.
func activeDay1JoinNode(node *nodeState) bool {
	if node == nil {
		return false
	}
	switch node.BootstrapPhase {
	case BootstrapAdmitted,
		BootstrapInfraPreparing,
		BootstrapEtcdJoining,
		BootstrapEtcdReady,
		BootstrapXdsReady,
		BootstrapEnvoyReady,
		BootstrapAwarenessReady,
		BootstrapStorageJoining:
		return true
	case BootstrapWorkloadReady:
		return strings.EqualFold(node.Status, "converging") ||
			strings.TrimSpace(node.AppliedServicesHash) == ""
	default:
		return false
	}
}
