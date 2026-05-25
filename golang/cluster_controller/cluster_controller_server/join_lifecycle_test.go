package main

import (
	"context"
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// ── Lifecycle mapping ─────────────────────────────────────────────────────────

func TestNormalizeJoinLifecyclePhase_LegacyMapping(t *testing.T) {
	cases := []struct {
		input string
		want  JoinLifecyclePhase
	}{
		{"pending", JoinPhaseRequested},
		{"approved", JoinPhaseAuthorized},
		{"blocked", JoinPhaseBlocked},
		{"rejected", JoinPhaseRejected},
		{"converging", JoinPhaseConverging},
		{"ready", JoinPhaseActive},
		{"", ""},
		{"unknown_string", ""},
	}
	for _, c := range cases {
		got := normalizeJoinLifecyclePhase(c.input)
		if got != c.want {
			t.Errorf("normalizeJoinLifecyclePhase(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestNormalizeJoinLifecyclePhase_TypedPassThrough(t *testing.T) {
	for _, p := range []JoinLifecyclePhase{
		JoinPhaseRequested, JoinPhaseAuthorized, JoinPhaseBootstrapping,
		JoinPhaseNodeAgentRegistered, JoinPhaseAdmissionPending,
		JoinPhaseAdmitted, JoinPhaseConverging, JoinPhaseActive,
		JoinPhaseBlocked, JoinPhaseRejected, JoinPhaseQuarantined,
		JoinPhaseRemoving, JoinPhaseRemoved, JoinPhaseStaleGhost,
	} {
		got := normalizeJoinLifecyclePhase(string(p))
		if got != p {
			t.Errorf("normalizeJoinLifecyclePhase(%q) = %q, want %q (round-trip failed)", p, got, p)
		}
	}
}

func TestEffectiveLifecyclePhase_PrefersTypedField(t *testing.T) {
	jr := &joinRequestRecord{
		Status:         "pending",
		LifecyclePhase: JoinPhaseAuthorized,
	}
	got := effectiveLifecyclePhase(jr)
	if got != JoinPhaseAuthorized {
		t.Errorf("effectiveLifecyclePhase = %q, want join_authorized (typed field should win)", got)
	}
}

func TestEffectiveLifecyclePhase_FallsBackToLegacyStatus(t *testing.T) {
	jr := &joinRequestRecord{Status: "approved"}
	got := effectiveLifecyclePhase(jr)
	if got != JoinPhaseAuthorized {
		t.Errorf("effectiveLifecyclePhase = %q, want join_authorized (legacy fallback)", got)
	}
}

func TestEffectiveLifecyclePhase_NilRecord(t *testing.T) {
	if got := effectiveLifecyclePhase(nil); got != "" {
		t.Errorf("effectiveLifecyclePhase(nil) = %q, want empty", got)
	}
}

// ── Phase helpers ─────────────────────────────────────────────────────────────

func TestJoinLifecyclePhase_EligibleForClusterDecisions(t *testing.T) {
	eligible := []JoinLifecyclePhase{
		JoinPhaseAdmitted, JoinPhaseConverging, JoinPhaseActive,
	}
	notEligible := []JoinLifecyclePhase{
		JoinPhaseRequested, JoinPhaseAuthorized, JoinPhaseBootstrapping,
		JoinPhaseNodeAgentRegistered, JoinPhaseAdmissionPending,
		JoinPhaseBlocked, JoinPhaseRejected, JoinPhaseQuarantined,
		JoinPhaseRemoving, JoinPhaseRemoved, JoinPhaseStaleGhost,
	}
	for _, p := range eligible {
		if !p.EligibleForClusterDecisions() {
			t.Errorf("phase %q should be eligible", p)
		}
	}
	for _, p := range notEligible {
		if p.EligibleForClusterDecisions() {
			t.Errorf("phase %q should NOT be eligible", p)
		}
	}
}

func TestJoinLifecyclePhase_Terminal(t *testing.T) {
	terminal := []JoinLifecyclePhase{
		JoinPhaseRejected, JoinPhaseRemoved, JoinPhaseQuarantined, JoinPhaseStaleGhost,
	}
	nonTerminal := []JoinLifecyclePhase{
		JoinPhaseRequested, JoinPhaseAuthorized, JoinPhaseBootstrapping,
		JoinPhaseAdmitted, JoinPhaseConverging, JoinPhaseActive, JoinPhaseBlocked,
	}
	for _, p := range terminal {
		if !p.Terminal() {
			t.Errorf("phase %q should be terminal", p)
		}
	}
	for _, p := range nonTerminal {
		if p.Terminal() {
			t.Errorf("phase %q should NOT be terminal", p)
		}
	}
}

// ── RequestJoinAuthorization lifecycle transitions ────────────────────────────

func TestRequestJoinAuthorization_StartsAsJoinRequested(t *testing.T) {
	srv := newJoinAuthServer(t)
	req := &JoinAuthorizationRequest{
		JoinToken: "tok-v2",
		Identity:  NodePlanIdentity{Hostname: "node-lifecycle-01", IPs: []string{"10.0.1.1"}},
		Nonce:     "nonce-lc1",
	}
	resp, err := srv.requestJoinAuthorizationCore(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Allowed {
		t.Fatalf("expected allowed, denied: %q", resp.DeniedReason)
	}
	jr := srv.state.JoinRequests[resp.JoinID]
	if jr == nil {
		t.Fatal("join request not found")
	}
	if jr.LifecyclePhase != JoinPhaseAuthorized {
		t.Errorf("after authorization: LifecyclePhase = %q, want join_authorized", jr.LifecyclePhase)
	}
}

func TestRequestJoinAuthorization_PreflightFailureSetsBlocked(t *testing.T) {
	srv := newJoinAuthServer(t)
	// Use a loopback IP to trigger preflight failure.
	req := &JoinAuthorizationRequest{
		JoinToken: "tok-v2",
		Identity:  NodePlanIdentity{Hostname: "node-blocked", IPs: []string{"127.0.0.1"}},
		Nonce:     "nonce-blocked",
	}
	resp, err := srv.requestJoinAuthorizationCore(req)
	if err != nil {
		t.Fatalf("unexpected gRPC error: %v", err)
	}
	if resp.Allowed {
		t.Fatal("expected allowed=false for preflight failure")
	}
	// Find the blocked join request — it was stored before preflight was evaluated.
	var foundJR *joinRequestRecord
	for _, jr := range srv.state.JoinRequests {
		if jr.Identity.Hostname == "node-blocked" {
			foundJR = jr
			break
		}
	}
	if foundJR == nil {
		t.Fatal("blocked join request not found in state")
	}
	if foundJR.LifecyclePhase != JoinPhaseBlocked {
		t.Errorf("blocked request: LifecyclePhase = %q, want blocked", foundJR.LifecyclePhase)
	}
}

func TestRequestJoinAuthorization_PlanIssuanceDoesNotAdmitNode(t *testing.T) {
	srv := newJoinAuthServer(t)
	req := &JoinAuthorizationRequest{
		JoinToken: "tok-v2",
		Identity:  NodePlanIdentity{Hostname: "node-notadmitted", IPs: []string{"10.0.1.2"}},
		Nonce:     "nonce-notadmitted",
	}
	resp, err := srv.requestJoinAuthorizationCore(req)
	if err != nil || !resp.Allowed {
		t.Fatalf("expected allowed, err=%v denied=%q", err, resp.DeniedReason)
	}
	// The node must exist in state (created by approveJoinRecordLocked),
	// but must not yet be admitted or active.
	node := srv.state.Nodes[resp.Plan.AssignedNodeID]
	if node == nil {
		t.Fatal("node state not found after authorization")
	}
	if node.JoinLifecyclePhase.EligibleForClusterDecisions() {
		t.Errorf("node lifecycle %q should not be cluster-eligible after JoinPlan issuance", node.JoinLifecyclePhase)
	}
	if node.JoinLifecyclePhase != JoinPhaseBootstrapping {
		t.Errorf("node lifecycle = %q, want bootstrapping", node.JoinLifecyclePhase)
	}
}

// ── Status messages ───────────────────────────────────────────────────────────

func TestStatusMessage_LifecyclePhaseDrivesMessage(t *testing.T) {
	cases := []struct {
		phase   JoinLifecyclePhase
		wantSub string
	}{
		{JoinPhaseRequested, "awaiting authorization"},
		{JoinPhaseAuthorized, "signed JoinPlan issued"},
		{JoinPhaseBootstrapping, "bootstrapping"},
		{JoinPhaseNodeAgentRegistered, "node-agent registered"},
		{JoinPhaseAdmissionPending, "evaluating admission"},
		{JoinPhaseAdmitted, "desired state"},
		{JoinPhaseConverging, "converging"},
		{JoinPhaseActive, "active"},
		{JoinPhaseBlocked, "blocked"},
		{JoinPhaseRejected, "rejected"},
	}
	for _, c := range cases {
		jr := &joinRequestRecord{LifecyclePhase: c.phase}
		msg := jr.statusMessage()
		found := false
		for i := 0; i < len(msg)-len(c.wantSub)+1; i++ {
			if msg[i:i+len(c.wantSub)] == c.wantSub {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("phase %q: statusMessage() = %q, want substring %q", c.phase, msg, c.wantSub)
		}
	}
}

func TestStatusMessage_BlockedIncludesReason(t *testing.T) {
	jr := &joinRequestRecord{
		LifecyclePhase: JoinPhaseBlocked,
		Reason:         "hostname conflict",
	}
	msg := jr.statusMessage()
	if msg != "blocked: hostname conflict" {
		t.Errorf("statusMessage() = %q, want \"blocked: hostname conflict\"", msg)
	}
}

// ── RF eligibility ─────────────────────────────────────────────────────────────

func TestRFEligibility_JoinAuthorizedNodeExcluded(t *testing.T) {
	node := &nodeState{
		Status:             "converging",
		BootstrapPhase:     BootstrapAdmitted,
		JoinLifecyclePhase: JoinPhaseBootstrapping,
	}
	if IsNodeVerifiedStorageEligible(node) {
		t.Error("bootstrapping node must not be RF eligible")
	}
	reason := nodeStorageEligibilityReason(node)
	if reason == "" {
		t.Error("expected a lifecycle exclusion reason")
	}
}

func TestRFEligibility_AdmittedNodeEligible(t *testing.T) {
	node := &nodeState{
		Status:             "ready",
		BootstrapPhase:     BootstrapWorkloadReady,
		JoinLifecyclePhase: JoinPhaseAdmitted,
	}
	reason := nodeStorageEligibilityReason(node)
	if reason != "" {
		t.Errorf("admitted node should be eligible, got reason=%q", reason)
	}
}

func TestRFEligibility_LegacyNodeRemainsEligible(t *testing.T) {
	// A legacy node with empty JoinLifecyclePhase must not be excluded.
	node := &nodeState{
		Status:             "ready",
		BootstrapPhase:     BootstrapWorkloadReady,
		JoinLifecyclePhase: "",
	}
	reason := nodeStorageEligibilityReason(node)
	if reason != "" {
		t.Errorf("legacy node (empty lifecycle) should be eligible, got reason=%q", reason)
	}
}

func TestRFEligibility_NodeAgentRegisteredNotEligible(t *testing.T) {
	for _, phase := range []JoinLifecyclePhase{
		JoinPhaseNodeAgentRegistered,
		JoinPhaseAdmissionPending,
		JoinPhaseAuthorized,
		JoinPhaseRequested,
	} {
		node := &nodeState{
			Status:             "converging",
			BootstrapPhase:     BootstrapAdmitted,
			JoinLifecyclePhase: phase,
		}
		if IsNodeVerifiedStorageEligible(node) {
			t.Errorf("phase %q must not be RF eligible", phase)
		}
	}
}

// ── First heartbeat advances lifecycle ────────────────────────────────────────

func TestHeartbeat_BootstrappingAdvancesToNodeAgentRegistered(t *testing.T) {
	state := newControllerState()
	state.JoinTokens["tok-hb"] = &joinTokenRecord{
		Token:     "tok-hb",
		ExpiresAt: time.Now().Add(time.Hour),
		MaxUses:   5,
	}
	srv := newTestServer(t, state)

	// Manually inject a bootstrapping node (simulates a just-authorized join).
	nodeID := "node-hb-01"
	state.Nodes[nodeID] = &nodeState{
		NodeID:             nodeID,
		Identity:           storedIdentity{Hostname: "hb-node", Ips: []string{"10.0.2.1"}},
		Status:             "converging",
		BootstrapPhase:     BootstrapAdmitted,
		JoinLifecyclePhase: JoinPhaseBootstrapping,
		Profiles:           []string{"core"},
		Metadata:           make(map[string]string),
	}

	// Simulate first heartbeat from the node-agent.
	_, err := srv.ReportNodeStatus(context.Background(), &cluster_controllerpb.ReportNodeStatusRequest{
		Status: &cluster_controllerpb.NodeStatus{
			NodeId:        nodeID,
			AgentEndpoint: "10.0.2.1:11000",
			Identity: &cluster_controllerpb.NodeIdentity{
				Hostname: "hb-node",
				Ips:      []string{"10.0.2.1"},
			},
		},
	})
	if err != nil {
		t.Fatalf("ReportNodeStatus error: %v", err)
	}

	node := srv.state.Nodes[nodeID]
	if node == nil {
		t.Fatal("node not found after heartbeat")
	}
	if node.JoinLifecyclePhase != JoinPhaseNodeAgentRegistered &&
		node.JoinLifecyclePhase != JoinPhaseAdmissionPending {
		t.Errorf("after first heartbeat: JoinLifecyclePhase = %q, want node_agent_registered or admission_pending",
			node.JoinLifecyclePhase)
	}
}
