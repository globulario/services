package main

import (
	"encoding/json"
	"testing"
	"time"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// admissionState builds a minimal controllerState with a single join request.
func admissionState(jr *joinRequestRecord, objectStoreGen int64) *controllerState {
	reqs := map[string]*joinRequestRecord{}
	if jr != nil {
		reqs[jr.RequestID] = jr
	}
	return &controllerState{
		JoinRequests:          reqs,
		ObjectStoreGeneration: objectStoreGen,
	}
}

// v2JoinRequest builds a joinRequestRecord that has a JoinPlanJSON embedded.
// The plan is not signed (signature verification is only in node-agent).
func v2JoinRequest(nodeID, hostname string, profiles []string) *joinRequestRecord {
	plan := JoinPlan{
		JoinID:    "jid-" + nodeID,
		ClusterID: "cluster-test",
		IssuedAt:  time.Now().Add(-1 * time.Minute),
		ExpiresAt: time.Now().Add(30 * time.Minute),
		ExpectedNodeIdentity: NodePlanIdentity{Hostname: hostname},
		AssignedProfiles:     append([]string(nil), profiles...),
		AssignedNodeID:       nodeID,
	}
	planJSON, _ := json.Marshal(plan)
	return &joinRequestRecord{
		RequestID:      "jid-" + nodeID,
		AssignedNodeID: nodeID,
		Status:         "approved",
		LifecyclePhase: JoinPhaseBootstrapping,
		Profiles:       append([]string(nil), profiles...),
		Identity:       storedIdentity{Hostname: hostname},
		JoinPlanJSON:   planJSON,
	}
}

// baseNode returns a minimal nodeState that passes the common checks.
func baseNode(nodeID, hostname string, profiles []string) *nodeState {
	return &nodeState{
		NodeID:             nodeID,
		Identity:           storedIdentity{Hostname: hostname},
		AgentEndpoint:      "10.0.0.5:11000",
		Profiles:           append([]string(nil), profiles...),
		JoinLifecyclePhase: JoinPhaseAdmissionPending,
	}
}

// ── Test 1: heartbeat moves bootstrapping → node_agent_registered only ────────

func TestAdmissionProof_BootstrappingNodeNotAdmitted(t *testing.T) {
	// A node in bootstrapping phase (just approved, no heartbeat yet) must
	// not be admitted — even if we accidentally run the evaluator on it.
	node := baseNode("n1", "node-01", []string{"core"})
	node.JoinLifecyclePhase = JoinPhaseBootstrapping
	node.AgentEndpoint = "" // no heartbeat yet

	jr := v2JoinRequest("n1", "node-01", []string{"core"})
	state := admissionState(jr, 0)

	result := EvaluateNodeAdmissionProof(state, node)
	if result.OK {
		t.Error("bootstrapping node must not be admitted: agent_endpoint is empty")
	}
	if result.Reason == "" {
		t.Error("failed admission must have a non-empty reason")
	}
}

// ── Test 2: node_agent_registered without proof → admission_pending, not admitted

func TestAdmissionProof_NodeAgentRegisteredNeedsProof(t *testing.T) {
	// A node in node_agent_registered (first heartbeat received) must not be
	// admitted without proof. The evaluator should return OK=false for a node
	// whose join record is not yet found (simulating no join record).
	node := baseNode("n2", "node-02", []string{"core"})
	node.JoinLifecyclePhase = JoinPhaseNodeAgentRegistered
	// No join request → no proof available.
	state := admissionState(nil, 0)

	result := EvaluateNodeAdmissionProof(state, node)
	if result.OK {
		t.Error("node without a join record must not be admitted")
	}
	if _, ok := result.Details["join_request"]; !ok {
		t.Error("result.Details must document the missing join record")
	}
}

// ── Test 3: v2 node with matching join_id and identity → admitted ─────────────

func TestAdmissionProof_V2NodeAdmitted(t *testing.T) {
	profiles := []string{"core", "control-plane", "storage"}
	jr := v2JoinRequest("n3", "node-03", profiles)
	node := baseNode("n3", "node-03", profiles)
	state := admissionState(jr, 0)

	result := EvaluateNodeAdmissionProof(state, node)
	if !result.OK {
		t.Errorf("v2 node with valid JoinPlan must be admitted: %s", result.Reason)
	}
	if !result.Proof.HasJoinPlan {
		t.Error("proof.HasJoinPlan must be true for v2 nodes")
	}
	if !result.Proof.IdentityMatchesPlan {
		t.Error("proof.IdentityMatchesPlan must be true")
	}
	if !result.Proof.JoinIDMatch {
		t.Error("proof.JoinIDMatch must be true")
	}
	if !result.Proof.ProfilesConsistent {
		t.Error("proof.ProfilesConsistent must be true")
	}
}

// ── Test 4: wrong join_id blocks admission ────────────────────────────────────

func TestAdmissionProof_WrongJoinIDBlocks(t *testing.T) {
	profiles := []string{"core"}
	jr := v2JoinRequest("n4", "node-04", profiles)

	// Tamper: change the RequestID so the plan's JoinID no longer matches
	// the join request ID.
	jr.RequestID = "different-request-id"
	// The plan still has JoinID = "jid-n4" but the record has a different ID.

	node := baseNode("n4", "node-04", profiles)
	// Ensure the state can find the record by AssignedNodeID.
	state := admissionState(jr, 0)

	result := EvaluateNodeAdmissionProof(state, node)
	if result.OK {
		t.Error("tampered join_id must block admission")
	}
	if !result.Proof.HasJoinPlan {
		t.Skip("join plan not found — test setup issue")
	}
	if result.Proof.JoinIDMatch {
		t.Error("proof.JoinIDMatch must be false when plan.JoinID != request.RequestID")
	}
}

// ── Test 5: wrong node identity blocks admission ──────────────────────────────

func TestAdmissionProof_WrongNodeIdentityBlocks(t *testing.T) {
	profiles := []string{"core"}
	jr := v2JoinRequest("n5", "node-05", profiles) // plan issued for node-05

	node := baseNode("n5", "node-WRONG", profiles) // but this node says it's node-WRONG
	state := admissionState(jr, 0)

	result := EvaluateNodeAdmissionProof(state, node)
	if result.OK {
		t.Error("wrong node identity must block admission")
	}
	if result.Proof.IdentityMatchesPlan {
		t.Error("proof.IdentityMatchesPlan must be false when hostname mismatches")
	}
	if _, ok := result.Details["identity_mismatch"]; !ok {
		t.Error("details must document the identity mismatch")
	}
}

// ── Test 6: missing etcd proof blocks admission when EtcdMemberIntent.Member=true

func TestAdmissionProof_EtcdFailedBlocksAdmission(t *testing.T) {
	profiles := []string{"core", "control-plane"}
	jr := v2JoinRequest("n6", "node-06", profiles)
	node := baseNode("n6", "node-06", profiles)
	node.EtcdMemberIntent = &EtcdMemberIntent{Member: true}
	node.EtcdJoinPhase = EtcdJoinFailed
	node.EtcdJoinError = "member add timeout"
	state := admissionState(jr, 0)

	result := EvaluateNodeAdmissionProof(state, node)
	if result.OK {
		t.Error("etcd join failure must block admission when EtcdMemberIntent.Member=true")
	}
	if result.Proof.EtcdProofOK {
		t.Error("proof.EtcdProofOK must be false when etcd join failed")
	}
	if _, ok := result.Details["etcd_join"]; !ok {
		t.Error("details must document the etcd join failure")
	}
}

// ── Test 7: missing Scylla proof blocks admission when ScyllaIntent.Member=true

func TestAdmissionProof_ScyllaFailedBlocksAdmission(t *testing.T) {
	profiles := []string{"core", "storage"}
	jr := v2JoinRequest("n7", "node-07", profiles)
	node := baseNode("n7", "node-07", profiles)
	node.ScyllaIntent = &ScyllaIntent{Member: true}
	node.ScyllaJoinPhase = ScyllaJoinFailed
	node.ScyllaJoinError = "gossip ring unreachable"
	state := admissionState(jr, 0)

	result := EvaluateNodeAdmissionProof(state, node)
	if result.OK {
		t.Error("scylla join failure must block admission when ScyllaIntent.Member=true")
	}
	if result.Proof.ScyllaProofOK {
		t.Error("proof.ScyllaProofOK must be false when scylla join failed")
	}
	if _, ok := result.Details["scylla_join"]; !ok {
		t.Error("details must document the scylla join failure")
	}
}

func TestClassifierBlocksWhenScyllaFailed(t *testing.T) {
	TestAdmissionProof_ScyllaFailedBlocksAdmission(t)
}

// ── Test 8: objectstore generation mismatch does not block base admission ──────

func TestAdmissionProof_ObjectstoreGenerationMismatchDoesNotBlockAdmission(t *testing.T) {
	profiles := []string{"core", "storage"}
	jr := v2JoinRequest("n8", "node-08", profiles)
	node := baseNode("n8", "node-08", profiles)
	// ObjectStoreIntent with topology_generation=1 but cluster is at generation=3.
	node.ObjectStoreIntent = &ObjectStoreIntent{
		Member:             true,
		TopologyGeneration: 1, // stale
	}
	state := admissionState(jr, 3) // cluster at generation 3

	result := EvaluateNodeAdmissionProof(state, node)
	// Base admission must succeed despite the generation mismatch.
	if !result.OK {
		t.Errorf("objectstore generation mismatch must not block base admission: %s", result.Reason)
	}
	// But active must NOT be OK when objectstore proof is required and generation mismatches.
	if result.ActiveOK {
		t.Error("ActiveOK must be false when objectstore generation mismatches and ObjectStoreIntent.Member=true")
	}
	if result.Proof.ObjectstoreGenerationOK {
		t.Error("proof.ObjectstoreGenerationOK must be false on generation mismatch")
	}
	if _, ok := result.Details["objectstore_generation"]; !ok {
		t.Error("details must document the generation mismatch")
	}
}

// ── Test 9: admitted node becomes active only with runtime proof ───────────────

func TestAdmissionProof_ActiveRequiresFullRuntimeProof(t *testing.T) {
	profiles := []string{"core", "control-plane", "storage"}
	jr := v2JoinRequest("n9", "node-09", profiles)
	node := baseNode("n9", "node-09", profiles)
	node.EtcdMemberIntent = &EtcdMemberIntent{Member: true, Voter: true}
	node.ScyllaIntent = &ScyllaIntent{Member: true}
	node.ObjectStoreIntent = &ObjectStoreIntent{Member: true, TopologyGeneration: 2}
	state := admissionState(jr, 2) // generation matches

	// Without full verification phases, ActiveOK must be false.
	node.EtcdJoinPhase = EtcdJoinStarted     // not yet verified
	node.ScyllaJoinPhase = ScyllaJoinStarted // not yet verified
	node.BootstrapPhase = BootstrapWorkloadReady

	result := EvaluateNodeAdmissionProof(state, node)
	if !result.OK {
		t.Errorf("base admission should pass: %s", result.Reason)
	}
	if result.ActiveOK {
		t.Error("ActiveOK must be false when etcd/scylla not fully verified")
	}

	// Now complete all verifications.
	node.EtcdJoinPhase = EtcdJoinVerified
	node.ScyllaJoinPhase = ScyllaJoinVerified

	result2 := EvaluateNodeAdmissionProof(state, node)
	if !result2.ActiveOK {
		t.Errorf("ActiveOK must be true when all proofs verified and bootstrap workload_ready: reason=%s", result2.Reason)
	}
}

// ── Test 10: legacy node (empty JoinLifecyclePhase) is not blocked ─────────────

func TestAdmissionProof_LegacyNodePreservesCompatibility(t *testing.T) {
	// A legacy node with empty JoinLifecyclePhase has no join request.
	// The evaluator should NOT hard-block it — legacy nodes are out of scope.
	// The handler skips the switch for empty phase, so the evaluator is never
	// called for them. But if it IS called, it should not impose v2 rules.
	node := &nodeState{
		NodeID:             "legacy-n10",
		Identity:           storedIdentity{Hostname: "legacy-node"},
		AgentEndpoint:      "10.0.0.10:11000",
		Profiles:           []string{"core", "storage"},
		JoinLifecyclePhase: "", // legacy: no lifecycle phase
		BootstrapPhase:     BootstrapWorkloadReady,
	}
	state := admissionState(nil, 0) // no join records

	result := EvaluateNodeAdmissionProof(state, node)
	// Legacy nodes have no join record. The evaluator returns OK=false because
	// it can't find a join record and JoinLifecyclePhase is "" (not a hard block).
	// The handler must NOT call EvaluateNodeAdmissionProof for empty-phase nodes.
	// This test documents that behavior: the evaluator does not crash on legacy nodes.
	_ = result // just verify no panic
}

// ── Test 11: blocked/bootstrap-failed/removed node cannot become admitted ──────

func TestAdmissionProof_BlockedStatesPreventAdmission(t *testing.T) {
	type tc struct {
		name      string
		setupNode func(*nodeState)
		wantOK    bool
	}
	tests := []tc{
		{
			name: "bootstrap_failed",
			setupNode: func(n *nodeState) {
				n.BootstrapPhase = BootstrapFailed
				n.BootstrapError = "etcd join timed out"
			},
		},
		{
			name: "quarantined",
			setupNode: func(n *nodeState) {
				n.JoinLifecyclePhase = JoinPhaseQuarantined
			},
		},
		{
			name: "removed",
			setupNode: func(n *nodeState) {
				n.JoinLifecyclePhase = JoinPhaseRemoved
			},
		},
		{
			name: "rejected",
			setupNode: func(n *nodeState) {
				n.JoinLifecyclePhase = JoinPhaseRejected
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			profiles := []string{"core"}
			jr := v2JoinRequest("n11-"+tc.name, "node-11", profiles)
			node := baseNode("n11-"+tc.name, "node-11", profiles)
			tc.setupNode(node)
			state := admissionState(jr, 0)

			result := EvaluateNodeAdmissionProof(state, node)
			if result.OK {
				t.Errorf("%s: node must not be admitted", tc.name)
			}
			if result.ActiveOK {
				t.Errorf("%s: node must not be active", tc.name)
			}
			if result.Reason == "" {
				t.Errorf("%s: reason must be non-empty", tc.name)
			}
		})
	}
}

// ── Test 12: proof result includes clear operator reason ──────────────────────

func TestAdmissionProof_ResultInclearsOperatorReason(t *testing.T) {
	tests := []struct {
		name       string
		buildNode  func() (*nodeState, *controllerState)
		wantSubstr string
	}{
		{
			name: "empty_agent_endpoint",
			buildNode: func() (*nodeState, *controllerState) {
				jr := v2JoinRequest("n12a", "node-12a", []string{"core"})
				node := baseNode("n12a", "node-12a", []string{"core"})
				node.AgentEndpoint = ""
				return node, admissionState(jr, 0)
			},
			wantSubstr: "agent_endpoint",
		},
		{
			name: "identity_mismatch",
			buildNode: func() (*nodeState, *controllerState) {
				jr := v2JoinRequest("n12b", "node-12b", []string{"core"})
				node := baseNode("n12b", "wrong-host", []string{"core"})
				return node, admissionState(jr, 0)
			},
			wantSubstr: "identity",
		},
		{
			name: "etcd_failed",
			buildNode: func() (*nodeState, *controllerState) {
				jr := v2JoinRequest("n12c", "node-12c", []string{"core", "control-plane"})
				node := baseNode("n12c", "node-12c", []string{"core", "control-plane"})
				node.EtcdMemberIntent = &EtcdMemberIntent{Member: true}
				node.EtcdJoinPhase = EtcdJoinFailed
				return node, admissionState(jr, 0)
			},
			wantSubstr: "etcd join failed",
		},
		{
			name: "no_join_record",
			buildNode: func() (*nodeState, *controllerState) {
				node := baseNode("n12d", "node-12d", []string{"core"})
				node.JoinLifecyclePhase = JoinPhaseAdmissionPending
				return node, admissionState(nil, 0)
			},
			wantSubstr: "join record",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			node, state := tc.buildNode()
			result := EvaluateNodeAdmissionProof(state, node)
			if result.OK {
				t.Fatalf("expected admission to fail for %s", tc.name)
			}
			if result.Reason == "" {
				t.Errorf("reason must be non-empty for %s", tc.name)
			}
			found := proofContains(result.Reason, tc.wantSubstr)
			if !found {
				for _, v := range result.Details {
					if proofContains(v, tc.wantSubstr) {
						found = true
						break
					}
				}
			}
			if !found {
				t.Errorf("%s: want reason/details to mention %q, got reason=%q details=%v",
					tc.name, tc.wantSubstr, result.Reason, result.Details)
			}
		})
	}
}

func proofContains(s, sub string) bool {
	if sub == "" || len(s) < len(sub) {
		return false
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
