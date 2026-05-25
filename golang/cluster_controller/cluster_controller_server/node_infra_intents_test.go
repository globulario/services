package main

import (
	"encoding/json"
	"testing"
	"time"
)

// ── Test 1: storage profile alone does not make a new v2 node RF eligible ─────

func TestInfraIntent_StorageProfileAloneNotRFEligible(t *testing.T) {
	// A new node assigned the storage profile should have ScyllaIntent.Member=true
	// but ScyllaIntent.RFEligible=false — profile is a capability label, not proof.
	node := &nodeState{
		Status:             "converging",
		BootstrapPhase:     BootstrapAdmitted,
		JoinLifecyclePhase: JoinPhaseBootstrapping,
		Profiles:           []string{"storage"},
		ScyllaIntent:       initialScyllaIntentForProfiles([]string{"storage"}),
	}
	if node.ScyllaIntent == nil {
		t.Fatal("storage profile must produce a ScyllaIntent")
	}
	if !node.ScyllaIntent.Member {
		t.Error("ScyllaIntent.Member should be true for storage profile")
	}
	if node.ScyllaIntent.RFEligible {
		t.Error("ScyllaIntent.RFEligible must be false for a newly joining node")
	}
	if IsNodeVerifiedStorageEligible(node) {
		t.Error("storage-profiled node with RFEligible=false must not be RF eligible")
	}
	reason := nodeStorageEligibilityReason(node)
	if reason != "lifecycle:bootstrapping" && reason != "scylla_intent:rf_not_eligible" {
		t.Errorf("expected lifecycle or intent exclusion reason, got %q", reason)
	}
}

// ── Test 2: Member=true + RFEligible=true + healthy lifecycle → eligible ──────

func TestInfraIntent_MemberAndRFEligibleWithAdmittedLifecycle(t *testing.T) {
	node := &nodeState{
		Status:             "ready",
		BootstrapPhase:     BootstrapWorkloadReady,
		JoinLifecyclePhase: JoinPhaseAdmitted,
		Profiles:           []string{"storage", "control-plane"},
		ScyllaJoinPhase:    ScyllaJoinVerified,
		ScyllaJoinStartedAt: time.Now().Add(-10 * time.Minute),
		ScyllaIntent: &ScyllaIntent{
			Member:     true,
			RFEligible: true,
		},
	}
	if !IsNodeVerifiedStorageEligible(node) {
		t.Errorf("admitted node with Member+RFEligible=true must be eligible, reason=%q",
			nodeStorageEligibilityReason(node))
	}
}

// ── Test 3: legacy node (nil ScyllaIntent) remains eligible ───────────────────

func TestInfraIntent_LegacyNodeNilIntentRemainEligible(t *testing.T) {
	node := makeStorageNode("legacy-node")
	// Ensure no intents are set — legacy node
	node.ScyllaIntent = nil
	node.EtcdMemberIntent = nil
	node.ObjectStoreIntent = nil

	if !IsNodeVerifiedStorageEligible(node) {
		t.Errorf("legacy node with nil ScyllaIntent must keep existing behavior, reason=%q",
			nodeStorageEligibilityReason(node))
	}
}

// ── Test 4: ObjectStoreIntent does not affect RF ──────────────────────────────

func TestInfraIntent_ObjectStoreIntentDoesNotAffectRF(t *testing.T) {
	// A node with an ObjectStoreIntent (storage profile) but no ScyllaIntent
	// should behave the same as a legacy node for RF purposes.
	node := makeStorageNode("minio-only-node")
	node.ScyllaIntent = nil
	node.ObjectStoreIntent = &ObjectStoreIntent{
		Member:             true,
		TopologyGeneration: 7,
	}
	// Without ScyllaIntent, RF eligibility falls through to legacy checks.
	if !IsNodeVerifiedStorageEligible(node) {
		t.Errorf("node with ObjectStoreIntent but nil ScyllaIntent must remain eligible (legacy), reason=%q",
			nodeStorageEligibilityReason(node))
	}
}

// ── Test 5: EtcdMemberIntent can be stored without changing RF ────────────────

func TestInfraIntent_EtcdMemberIntentDoesNotAffectRF(t *testing.T) {
	node := makeStorageNode("etcd-node")
	node.EtcdMemberIntent = &EtcdMemberIntent{
		Member: true,
		Voter:  true,
	}
	// EtcdMemberIntent should not affect storage/RF eligibility.
	if !IsNodeVerifiedStorageEligible(node) {
		t.Errorf("EtcdMemberIntent must not affect RF eligibility, reason=%q",
			nodeStorageEligibilityReason(node))
	}
}

// ── Test 6: JoinPlan issuance does not set RFEligible=true ───────────────────

func TestInfraIntent_JoinPlanDoesNotSetRFEligible(t *testing.T) {
	srv := newJoinAuthServer(t)
	req := &JoinAuthorizationRequest{
		JoinToken: "tok-v2",
		Identity:  NodePlanIdentity{Hostname: "intent-node-01", IPs: []string{"10.0.3.1"}},
		Nonce:     "nonce-intent-1",
	}
	resp, err := srv.requestJoinAuthorizationCore(req)
	if err != nil || !resp.Allowed {
		t.Fatalf("expected allowed, err=%v denied=%q", err, resp.DeniedReason)
	}
	node := srv.state.Nodes[resp.Plan.AssignedNodeID]
	if node == nil {
		t.Fatal("node not found after JoinPlan issuance")
	}
	if node.ScyllaIntent != nil && node.ScyllaIntent.RFEligible {
		t.Error("JoinPlan issuance must not set ScyllaIntent.RFEligible=true")
	}
}

// ── Test 7: admitted node with RFEligible=false does not count ────────────────

func TestInfraIntent_AdmittedWithRFEligibleFalseExcluded(t *testing.T) {
	node := &nodeState{
		Status:             "ready",
		BootstrapPhase:     BootstrapWorkloadReady,
		JoinLifecyclePhase: JoinPhaseAdmitted,
		Profiles:           []string{"storage"},
		ScyllaJoinPhase:    ScyllaJoinVerified,
		ScyllaJoinStartedAt: time.Now().Add(-5 * time.Minute),
		ScyllaIntent: &ScyllaIntent{
			Member:     true,
			RFEligible: false, // admitted but not yet RF-proven
		},
	}
	if IsNodeVerifiedStorageEligible(node) {
		t.Error("admitted node with ScyllaIntent.RFEligible=false must not count toward RF")
	}
	reason := nodeStorageEligibilityReason(node)
	if reason != "scylla_intent:rf_not_eligible" {
		t.Errorf("expected scylla_intent:rf_not_eligible reason, got %q", reason)
	}
}

// ── Test 8: active node with RFEligible=true and healthy Scylla counts ────────

func TestInfraIntent_ActiveWithRFEligibleTrueAndHealthyScyllaEligible(t *testing.T) {
	node := &nodeState{
		Status:             "ready",
		BootstrapPhase:     BootstrapWorkloadReady,
		JoinLifecyclePhase: JoinPhaseActive,
		Profiles:           []string{"storage", "control-plane"},
		ScyllaJoinPhase:    ScyllaJoinVerified,
		ScyllaJoinStartedAt: time.Now().Add(-30 * time.Minute),
		ScyllaIntent: &ScyllaIntent{
			Member:              true,
			RFEligible:          true,
			Group0VoterVerified: true,
		},
	}
	if !IsNodeVerifiedStorageEligible(node) {
		t.Errorf("active node with RFEligible=true + healthy Scylla must be eligible, reason=%q",
			nodeStorageEligibilityReason(node))
	}
}

// ── Test 9: blocked/bootstrapping node with RFEligible=true still excluded ───

func TestInfraIntent_BlockedNodeWithRFEligibleTrueStillExcluded(t *testing.T) {
	for _, phase := range []JoinLifecyclePhase{
		JoinPhaseBootstrapping,
		JoinPhaseNodeAgentRegistered,
		JoinPhaseAdmissionPending,
		JoinPhaseBlocked,
	} {
		node := &nodeState{
			Status:             "converging",
			BootstrapPhase:     BootstrapWorkloadReady,
			JoinLifecyclePhase: phase,
			Profiles:           []string{"storage"},
			ScyllaIntent: &ScyllaIntent{
				Member:     true,
				RFEligible: true, // set, but lifecycle gate must fire first
			},
		}
		if IsNodeVerifiedStorageEligible(node) {
			t.Errorf("phase %q: node with RFEligible=true but non-admitted lifecycle must not be eligible", phase)
		}
		reason := nodeStorageEligibilityReason(node)
		if reason == "" {
			t.Errorf("phase %q: expected exclusion reason", phase)
		}
	}
}

// ── Test 10: serialization round-trip preserves all intent fields ─────────────

func TestInfraIntent_SerializationRoundTrip(t *testing.T) {
	node := &nodeState{
		NodeID:  "round-trip-node",
		Status:  "ready",
		Profiles: []string{"storage", "control-plane"},
		EtcdMemberIntent: &EtcdMemberIntent{
			Member:     true,
			Voter:      true,
			Generation: 42,
			Reason:     "etcd join complete",
		},
		ScyllaIntent: &ScyllaIntent{
			Member:              true,
			RFEligible:          true,
			Group0VoterVerified: true,
			Generation:          7,
			Reason:              "scylla verified",
		},
		ObjectStoreIntent: &ObjectStoreIntent{
			Member:             true,
			TopologyGeneration: 3,
			Reason:             "minio pool joined",
		},
	}

	b, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got nodeState
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.EtcdMemberIntent == nil || !got.EtcdMemberIntent.Voter || got.EtcdMemberIntent.Generation != 42 {
		t.Errorf("EtcdMemberIntent not preserved: %+v", got.EtcdMemberIntent)
	}
	if got.ScyllaIntent == nil || !got.ScyllaIntent.RFEligible || !got.ScyllaIntent.Group0VoterVerified || got.ScyllaIntent.Generation != 7 {
		t.Errorf("ScyllaIntent not preserved: %+v", got.ScyllaIntent)
	}
	if got.ObjectStoreIntent == nil || !got.ObjectStoreIntent.Member || got.ObjectStoreIntent.TopologyGeneration != 3 {
		t.Errorf("ObjectStoreIntent not preserved: %+v", got.ObjectStoreIntent)
	}
}
