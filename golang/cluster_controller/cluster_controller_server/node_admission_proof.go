// @awareness namespace=globular.platform
// @awareness component=cluster_controller.node_admission
// @awareness file_role=join_proof_validation_before_cluster_admission
// @awareness implements=globular.platform:intent.join.token.validation
// @awareness implements=globular.platform:intent.security.tokens_certificates_keys.cluster_trust_contract
// @awareness implements=globular.platform:intent.cluster.membership.earned_trust
// @awareness risk=critical
package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// NodeAdmissionProof captures the evidence collected during a single
// EvaluateNodeAdmissionProof call. Each field represents one verifiable
// claim. Callers should not construct this directly — use the result from
// EvaluateNodeAdmissionProof.
type NodeAdmissionProof struct {
	// NotBlocked is true when the node is not in a hard-block state
	// (bootstrap_failed, quarantined, removed, rejected).
	NotBlocked bool
	// IdentityPresent is true when node.Identity.Hostname is non-empty.
	IdentityPresent bool
	// AgentReachable is true when node.AgentEndpoint is non-empty.
	AgentReachable bool
	// BuildPresent is true when the node reports a node-agent version or build_id.
	// Informational — never blocks admission.
	BuildPresent bool
	// HasJoinPlan is true for v2 nodes where a stored JoinPlanJSON exists
	// in the controller's join request records. Legacy v1 nodes have false.
	HasJoinPlan bool
	// IdentityMatchesPlan is true when the node's reported hostname matches
	// the JoinPlan.ExpectedNodeIdentity.Hostname. Only meaningful when HasJoinPlan.
	IdentityMatchesPlan bool
	// JoinIDMatch is true when the JoinPlan.JoinID matches the stored join
	// request ID. Only meaningful when HasJoinPlan.
	JoinIDMatch bool
	// ProfilesConsistent is true when node.Profiles aligns with the
	// JoinPlan.AssignedProfiles (or the join request profiles for v1 nodes).
	ProfilesConsistent bool
	// EtcdRequired is true when EtcdMemberIntent.Member=true for this node.
	EtcdRequired bool
	// EtcdProofOK is true when EtcdJoinPhase is not "failed". Required for
	// admission when EtcdRequired=true. Does not require verified status.
	EtcdProofOK bool
	// EtcdFullyVerified is true when EtcdJoinPhase==verified. Required for
	// the active transition.
	EtcdFullyVerified bool
	// ScyllaRequired is true when ScyllaIntent.Member=true for this node.
	ScyllaRequired bool
	// ScyllaProofOK is true when ScyllaJoinPhase is not "failed". Required
	// for admission when ScyllaRequired=true.
	ScyllaProofOK bool
	// ScyllaFullyVerified is true when ScyllaJoinPhase==verified. Required
	// for the active transition.
	ScyllaFullyVerified bool
	// ObjectstoreRequired is true when ObjectStoreIntent.Member=true.
	ObjectstoreRequired bool
	// ObjectstoreGenerationOK is true when the node's topology_generation
	// matches the cluster's current ObjectStoreGeneration. Required for the
	// active transition when ObjectstoreRequired=true. Does not block base
	// admission.
	ObjectstoreGenerationOK bool
}

// NodeAdmissionProofResult is the verdict from EvaluateNodeAdmissionProof.
type NodeAdmissionProofResult struct {
	// OK is true when the node has sufficient proof to advance from
	// admission_pending to admitted. This is the base admission gate.
	OK bool
	// ActiveOK is true when the node has sufficient proof to advance from
	// admitted to active. Requires OK=true plus full runtime convergence.
	ActiveOK bool
	// Reason is a human-readable explanation when OK=false. Empty when OK=true.
	Reason string
	// Proof captures the individual check results.
	Proof NodeAdmissionProof
	// Details carries machine-readable key/value evidence for the operator.
	Details map[string]string
	// CheckedAt is the time of evaluation.
	CheckedAt time.Time
}

// EvaluateNodeAdmissionProof assesses whether node has presented sufficient
// runtime evidence to advance lifecycle phases past admission_pending.
//
// v2 nodes (those with a stored signed JoinPlan) must correlate their
// reported identity with the plan's expected identity before the controller
// admits them into the cluster. A heartbeat alone is not sufficient.
//
// v1 nodes (RequestJoin path, no JoinPlan) use weaker checks: hostname
// non-empty and agent reachable.
//
// Legacy nodes (empty JoinLifecyclePhase) must not be passed to this
// function — they are managed by the BootstrapPhase path and return
// OK=false so the caller can detect and skip them.
func EvaluateNodeAdmissionProof(state *controllerState, node *nodeState) NodeAdmissionProofResult {
	now := time.Now()
	if node == nil {
		return NodeAdmissionProofResult{Reason: "nil node", CheckedAt: now}
	}
	details := make(map[string]string)
	var proof NodeAdmissionProof

	// ── Hard blocks ────────────────────────────────────────────────────────────
	switch node.JoinLifecyclePhase {
	case JoinPhaseQuarantined:
		return NodeAdmissionProofResult{
			Reason:    "node is quarantined; requires operator intervention",
			Details:   map[string]string{"lifecycle_phase": string(node.JoinLifecyclePhase)},
			CheckedAt: now,
		}
	case JoinPhaseRemoved, JoinPhaseRemoving:
		return NodeAdmissionProofResult{
			Reason:    "node has been removed from the cluster",
			Details:   map[string]string{"lifecycle_phase": string(node.JoinLifecyclePhase)},
			CheckedAt: now,
		}
	case JoinPhaseRejected:
		return NodeAdmissionProofResult{
			Reason:    "join request was rejected",
			Details:   map[string]string{"lifecycle_phase": string(node.JoinLifecyclePhase)},
			CheckedAt: now,
		}
	}
	if node.BootstrapPhase == BootstrapFailed {
		errDetail := node.BootstrapError
		if errDetail == "" {
			errDetail = "see bootstrap_error field"
		}
		return NodeAdmissionProofResult{
			Reason:    "node bootstrap failed",
			Details:   map[string]string{"bootstrap_error": errDetail},
			CheckedAt: now,
		}
	}
	proof.NotBlocked = true

	// ── Identity ───────────────────────────────────────────────────────────────
	proof.IdentityPresent = strings.TrimSpace(node.Identity.Hostname) != ""
	if !proof.IdentityPresent {
		details["identity"] = "hostname empty"
	}

	// ── Agent reachability ─────────────────────────────────────────────────────
	proof.AgentReachable = strings.TrimSpace(node.AgentEndpoint) != ""
	if !proof.AgentReachable {
		details["agent_endpoint"] = "empty"
	}

	// ── Build presence (informational) ─────────────────────────────────────────
	if v := node.InstalledVersions["node-agent"]; v != "" {
		proof.BuildPresent = true
		details["node_agent_version"] = v
	} else if bid := node.InstalledBuildIDs["node-agent"]; bid != "" {
		proof.BuildPresent = true
		details["node_agent_build_id"] = bid
	}

	// ── JoinPlan correlation ───────────────────────────────────────────────────
	// Find the current join request for this node. A node can have stale records
	// from failed or retried authorizations, so selection must be deterministic.
	joinReq := latestJoinRequestForNode(state, node.NodeID)

	proof.HasJoinPlan = joinReq != nil && len(joinReq.JoinPlanJSON) > 0

	switch {
	case proof.HasJoinPlan:
		// v2 path: full JoinPlan correlation.
		var plan JoinPlan
		if err := json.Unmarshal(joinReq.JoinPlanJSON, &plan); err != nil {
			details["join_plan_parse"] = "failed: " + err.Error()
			// Treat parse failure as no-plan: identity/join_id checks fail.
		} else {
			// Identity: node's reported hostname must match plan's expected hostname.
			if strings.EqualFold(
				strings.TrimSpace(plan.ExpectedNodeIdentity.Hostname),
				strings.TrimSpace(node.Identity.Hostname),
			) {
				proof.IdentityMatchesPlan = true
			} else {
				details["identity_mismatch"] = fmt.Sprintf(
					"plan expects %q, node reports %q",
					plan.ExpectedNodeIdentity.Hostname,
					node.Identity.Hostname,
				)
			}
			// JoinID: plan.JoinID must equal the join request's RequestID.
			if plan.JoinID == joinReq.RequestID {
				proof.JoinIDMatch = true
			} else {
				details["join_id_mismatch"] = fmt.Sprintf(
					"plan join_id=%q request_id=%q",
					plan.JoinID,
					joinReq.RequestID,
				)
			}
			// Profiles: node's assigned profiles must be consistent with the plan.
			if profilesConsistent(node.Profiles, plan.AssignedProfiles) {
				proof.ProfilesConsistent = true
			} else {
				details["profiles_mismatch"] = fmt.Sprintf(
					"node=%v plan=%v",
					node.Profiles,
					plan.AssignedProfiles,
				)
			}
		}

	case joinReq != nil:
		// v1 path: join request exists but no JoinPlan (legacy RequestJoin).
		// Hostname/identity were already validated at request time (preflight).
		proof.IdentityMatchesPlan = true // no plan to contradict
		proof.JoinIDMatch = true         // no plan join_id to verify
		proof.ProfilesConsistent = len(node.Profiles) > 0
		if !proof.ProfilesConsistent {
			details["profiles"] = "empty; not yet assigned"
		}

	default:
		// No join request found. This node arrived via auto-registration or the
		// join record expired. Cannot perform identity correlation.
		// IdentityMatchesPlan and JoinIDMatch stay false.
		details["join_request"] = "not found for node_id=" + node.NodeID
	}

	// ── Etcd member proof ──────────────────────────────────────────────────────
	if node.EtcdMemberIntent != nil && node.EtcdMemberIntent.Member {
		proof.EtcdRequired = true
		switch node.EtcdJoinPhase {
		case EtcdJoinFailed, EtcdJoinRejoinFailed:
			proof.EtcdProofOK = false
			errDetail := node.EtcdJoinError
			if errDetail == "" {
				errDetail = "etcd join failed (see node state)"
			}
			details["etcd_join"] = errDetail
		default:
			proof.EtcdProofOK = true
		}
		proof.EtcdFullyVerified = node.EtcdJoinPhase == EtcdJoinVerified
	}

	// ── Scylla member proof ────────────────────────────────────────────────────
	if node.ScyllaIntent != nil && node.ScyllaIntent.Member {
		proof.ScyllaRequired = true
		switch node.ScyllaJoinPhase {
		case ScyllaJoinFailed:
			proof.ScyllaProofOK = false
			errDetail := node.ScyllaJoinError
			if errDetail == "" {
				errDetail = "scylla join failed (see node state)"
			}
			details["scylla_join"] = errDetail
		default:
			proof.ScyllaProofOK = true
		}
		proof.ScyllaFullyVerified = node.ScyllaJoinPhase == ScyllaJoinVerified
	}

	// ── Objectstore member proof ───────────────────────────────────────────────
	// Objectstore generation mismatch blocks active but NOT base admission.
	if node.ObjectStoreIntent != nil && node.ObjectStoreIntent.Member {
		proof.ObjectstoreRequired = true
		var clusterGen uint64
		if state != nil {
			clusterGen = uint64(state.ObjectStoreGeneration)
		}
		nodeGen := node.ObjectStoreIntent.TopologyGeneration
		// Generation 0 means "not yet joined any topology" — not a mismatch.
		proof.ObjectstoreGenerationOK = nodeGen > 0 && nodeGen == clusterGen
		if !proof.ObjectstoreGenerationOK {
			details["objectstore_generation"] = fmt.Sprintf(
				"node=%d cluster=%d",
				nodeGen,
				clusterGen,
			)
		}
	}

	// ── Compute OK (admission gate) ────────────────────────────────────────────
	// A v2 node must have all three plan checks pass. A v1/legacy node needs
	// only identity presence and agent reachability.
	planOK := !proof.HasJoinPlan ||
		(proof.IdentityMatchesPlan && proof.JoinIDMatch && proof.ProfilesConsistent)

	// If no join request was found AND the node has a lifecycle phase set (v2),
	// block admission: we have no way to verify identity.
	noJoinRecordBlock := joinReq == nil && node.JoinLifecyclePhase != ""

	ok := proof.NotBlocked &&
		proof.IdentityPresent &&
		proof.AgentReachable &&
		planOK &&
		!noJoinRecordBlock &&
		(!proof.EtcdRequired || proof.EtcdProofOK) &&
		(!proof.ScyllaRequired || proof.ScyllaProofOK)

	// ── Compute ActiveOK (full runtime gate) ───────────────────────────────────
	activeOK := ok &&
		bootstrapPhaseReady(node.BootstrapPhase) &&
		(!proof.EtcdRequired || proof.EtcdFullyVerified) &&
		(!proof.ScyllaRequired || proof.ScyllaFullyVerified) &&
		(!proof.ObjectstoreRequired || proof.ObjectstoreGenerationOK)

	// ── Build operator reason ──────────────────────────────────────────────────
	reason := ""
	if !ok {
		var parts []string
		if !proof.IdentityPresent {
			parts = append(parts, "hostname empty")
		}
		if !proof.AgentReachable {
			parts = append(parts, "agent_endpoint empty")
		}
		if proof.HasJoinPlan && !proof.IdentityMatchesPlan {
			parts = append(parts, "identity does not match JoinPlan")
		}
		if proof.HasJoinPlan && !proof.JoinIDMatch {
			parts = append(parts, "join_id mismatch")
		}
		if proof.HasJoinPlan && !proof.ProfilesConsistent {
			parts = append(parts, "profiles inconsistent with JoinPlan")
		}
		if noJoinRecordBlock {
			parts = append(parts, "no join record found for node (cannot verify identity)")
		}
		if proof.EtcdRequired && !proof.EtcdProofOK {
			parts = append(parts, "etcd join failed")
		}
		if proof.ScyllaRequired && !proof.ScyllaProofOK {
			parts = append(parts, "scylla join failed")
		}
		if len(parts) == 0 {
			parts = append(parts, "admission proof incomplete")
		}
		reason = strings.Join(parts, "; ")
	}

	return NodeAdmissionProofResult{
		OK:        ok,
		ActiveOK:  activeOK,
		Reason:    reason,
		Proof:     proof,
		Details:   details,
		CheckedAt: now,
	}
}

func latestJoinRequestForNode(state *controllerState, nodeID string) *joinRequestRecord {
	if state == nil || nodeID == "" {
		return nil
	}
	var latest *joinRequestRecord
	for _, jr := range state.JoinRequests {
		if jr == nil || jr.AssignedNodeID != nodeID {
			continue
		}
		if latest == nil || jr.RequestedAt.After(latest.RequestedAt) {
			latest = jr
			continue
		}
		if jr.RequestedAt.Equal(latest.RequestedAt) && jr.RequestID > latest.RequestID {
			latest = jr
		}
	}
	return latest
}

// profilesConsistent reports whether nodeProfiles and planProfiles contain
// the same set of profiles (order-independent).
func profilesConsistent(nodeProfiles, planProfiles []string) bool {
	if len(nodeProfiles) != len(planProfiles) {
		return false
	}
	planSet := make(map[string]bool, len(planProfiles))
	for _, p := range planProfiles {
		planSet[strings.TrimSpace(p)] = true
	}
	for _, p := range nodeProfiles {
		if !planSet[strings.TrimSpace(p)] {
			return false
		}
	}
	return true
}
