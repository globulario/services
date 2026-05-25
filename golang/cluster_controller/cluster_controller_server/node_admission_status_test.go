package main

import (
	"testing"
	"time"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// evalFor builds a NodeAdmissionProofResult for a given node+state pair.
func evalFor(state *controllerState, node *nodeState) NodeAdmissionProofResult {
	return EvaluateNodeAdmissionProof(state, node)
}

// statusFor is shorthand: evaluate and build the AdmissionProofStatus.
func statusFor(state *controllerState, node *nodeState) *AdmissionProofStatus {
	return buildNodeAdmissionStatus(evalFor(state, node))
}

// ── Test 1: admission_pending node exposes last failed proof reason ────────────

func TestAdmissionStatus_PendingNodeExposesReason(t *testing.T) {
	// A node with an empty agent endpoint should be blocked and must explain why.
	node := baseNode("sn1", "node-01", []string{"core"})
	node.AgentEndpoint = "" // agent not yet reachable

	jr := v2JoinRequest("sn1", "node-01", []string{"core"})
	state := admissionState(jr, 0)

	s := statusFor(state, node)
	if s.OK {
		t.Fatal("expected admission failure for unreachable agent")
	}
	if s.Reason == "" {
		t.Error("status.Reason must be non-empty for a failed admission")
	}
	if s.FindingID == "" {
		t.Error("status.FindingID must be set for a failed admission")
	}
	if s.CheckedAt.IsZero() {
		t.Error("status.CheckedAt must be stamped")
	}
}

// ── Test 2: wrong join_id appears in status details ───────────────────────────

func TestAdmissionStatus_WrongJoinIDInDetails(t *testing.T) {
	jr := v2JoinRequest("sn2", "node-02", []string{"core"})
	jr.RequestID = "tampered-id" // plan.JoinID = "jid-sn2", record.RequestID = "tampered-id"

	node := baseNode("sn2", "node-02", []string{"core"})
	state := admissionState(jr, 0)

	s := statusFor(state, node)
	if s.OK {
		t.Fatal("tampered join_id must block admission")
	}
	if s.JoinIDMatch {
		t.Error("status.JoinIDMatch must be false when join_id is tampered")
	}
	if s.FindingID != FindingAdmissionJoinIDMismatch {
		t.Errorf("want finding %q, got %q", FindingAdmissionJoinIDMismatch, s.FindingID)
	}
	if _, ok := s.Details["join_id_mismatch"]; !ok {
		t.Error("details must include join_id_mismatch key")
	}
}

// ── Test 3: identity mismatch appears in status details ───────────────────────

func TestAdmissionStatus_IdentityMismatchInDetails(t *testing.T) {
	jr := v2JoinRequest("sn3", "node-03", []string{"core"}) // plan for node-03
	node := baseNode("sn3", "node-WRONG", []string{"core"}) // but reports as node-WRONG
	state := admissionState(jr, 0)

	s := statusFor(state, node)
	if s.OK {
		t.Fatal("identity mismatch must block admission")
	}
	if s.IdentityMatchesPlan {
		t.Error("status.IdentityMatchesPlan must be false")
	}
	if s.FindingID != FindingAdmissionIdentityMismatch {
		t.Errorf("want finding %q, got %q", FindingAdmissionIdentityMismatch, s.FindingID)
	}
	if _, ok := s.Details["identity_mismatch"]; !ok {
		t.Error("details must include identity_mismatch key")
	}
}

// ── Test 4: missing etcd proof appears in status details ──────────────────────

func TestAdmissionStatus_EtcdProofInDetails(t *testing.T) {
	jr := v2JoinRequest("sn4", "node-04", []string{"core", "control-plane"})
	node := baseNode("sn4", "node-04", []string{"core", "control-plane"})
	node.EtcdMemberIntent = &EtcdMemberIntent{Member: true}
	node.EtcdJoinPhase = EtcdJoinFailed
	node.EtcdJoinError = "member add timed out"
	state := admissionState(jr, 0)

	s := statusFor(state, node)
	if s.OK {
		t.Fatal("etcd failure must block admission")
	}
	if s.EtcdProofOK {
		t.Error("status.EtcdProofOK must be false")
	}
	if !s.EtcdRequired {
		t.Error("status.EtcdRequired must be true")
	}
	if s.FindingID != FindingAdmissionEtcdUnverified {
		t.Errorf("want finding %q, got %q", FindingAdmissionEtcdUnverified, s.FindingID)
	}
	if _, ok := s.Details["etcd_join"]; !ok {
		t.Error("details must include etcd_join key")
	}
	msg := admissionOperatorMessage(s.FindingID)
	if msg == "" {
		t.Error("operator message must be non-empty for etcd finding")
	}
}

// ── Test 5: missing Scylla proof appears in status details ────────────────────

func TestAdmissionStatus_ScyllaProofInDetails(t *testing.T) {
	jr := v2JoinRequest("sn5", "node-05", []string{"core", "storage"})
	node := baseNode("sn5", "node-05", []string{"core", "storage"})
	node.ScyllaIntent = &ScyllaIntent{Member: true}
	node.ScyllaJoinPhase = ScyllaJoinFailed
	node.ScyllaJoinError = "gossip ring bootstrap failed"
	state := admissionState(jr, 0)

	s := statusFor(state, node)
	if s.OK {
		t.Fatal("scylla failure must block admission")
	}
	if s.ScyllaProofOK {
		t.Error("status.ScyllaProofOK must be false")
	}
	if s.FindingID != FindingAdmissionScyllaUnverified {
		t.Errorf("want finding %q, got %q", FindingAdmissionScyllaUnverified, s.FindingID)
	}
	if _, ok := s.Details["scylla_join"]; !ok {
		t.Error("details must include scylla_join key")
	}
}

// ── Test 6: objectstore generation mismatch appears in status details ─────────

func TestAdmissionStatus_ObjectstoreGenerationInDetails(t *testing.T) {
	jr := v2JoinRequest("sn6", "node-06", []string{"core", "storage"})
	node := baseNode("sn6", "node-06", []string{"core", "storage"})
	node.ObjectStoreIntent = &ObjectStoreIntent{Member: true, TopologyGeneration: 1}
	state := admissionState(jr, 5) // cluster at gen 5, node at gen 1

	s := statusFor(state, node)
	// Objectstore generation mismatch does NOT block base admission.
	if !s.OK {
		t.Errorf("objectstore gen mismatch must not block base admission: %s", s.Reason)
	}
	// But ActiveOK must be false.
	if s.ActiveOK {
		t.Error("ActiveOK must be false when objectstore generation mismatches")
	}
	if s.ObjectstoreGenerationOK {
		t.Error("status.ObjectstoreGenerationOK must be false on mismatch")
	}
	if _, ok := s.Details["objectstore_generation"]; !ok {
		t.Error("details must include objectstore_generation key")
	}
}

// ── Test 7: active-ready node reports OK and ActiveOK ─────────────────────────

func TestAdmissionStatus_ActiveNodeReportsOKAndActiveOK(t *testing.T) {
	jr := v2JoinRequest("sn7", "node-07", []string{"core", "control-plane", "storage"})
	node := baseNode("sn7", "node-07", []string{"core", "control-plane", "storage"})
	node.JoinLifecyclePhase = JoinPhaseAdmitted
	node.BootstrapPhase = BootstrapWorkloadReady
	node.EtcdMemberIntent = &EtcdMemberIntent{Member: true, Voter: true}
	node.EtcdJoinPhase = EtcdJoinVerified
	node.ScyllaIntent = &ScyllaIntent{Member: true}
	node.ScyllaJoinPhase = ScyllaJoinVerified
	node.ObjectStoreIntent = &ObjectStoreIntent{Member: true, TopologyGeneration: 3}
	state := admissionState(jr, 3) // generation matches

	s := statusFor(state, node)
	if !s.OK {
		t.Errorf("fully admitted node must have OK=true: %s", s.Reason)
	}
	if !s.ActiveOK {
		t.Errorf("fully converged node must have ActiveOK=true (etcd=%v scylla=%v objstore=%v bootstrap=%v)",
			s.EtcdFullyVerified, s.ScyllaFullyVerified, s.ObjectstoreGenerationOK,
			node.BootstrapPhase)
	}
	if s.FindingID != "" {
		t.Errorf("active-ready node must have empty FindingID, got %q", s.FindingID)
	}
}

// ── Test 8: legacy node reports legacy compatibility mode ─────────────────────

func TestAdmissionStatus_LegacyNodeReportsCompatMode(t *testing.T) {
	node := &nodeState{
		NodeID:             "legacy-sn8",
		Identity:           storedIdentity{Hostname: "legacy-node"},
		AgentEndpoint:      "10.0.0.8:11000",
		Profiles:           []string{"core"},
		JoinLifecyclePhase: "", // legacy
	}
	state := admissionState(nil, 0)

	// The evaluator is not called for legacy nodes by the handler.
	// admissionStatusMetadata must return "legacy_compat" and the note.
	meta := admissionStatusMetadata(node)
	if meta["admission_path"] != "legacy_compat" {
		t.Errorf("legacy node must report admission_path=legacy_compat, got %q", meta["admission_path"])
	}
	if meta["admission_note"] == "" {
		t.Error("legacy node must include admission_note")
	}

	// Verify the label function directly.
	label := admissionPathLabel(node)
	if label != "legacy_compat" {
		t.Errorf("admissionPathLabel: want legacy_compat, got %q", label)
	}

	// The evaluator itself does not crash on a legacy node but returns OK=false
	// (it can't find a join record for a node with lifecycle phase "").
	result := EvaluateNodeAdmissionProof(state, node)
	_ = result // no panic, no assertion beyond non-panic
}

// ── Test 9: reason-change logging avoids repeated identical spam ──────────────

func TestAdmissionStatus_ReasonChangeLogSuppression(t *testing.T) {
	// admissionShouldLog must return true only when the reason changes.
	reason1 := "identity does not match JoinPlan"
	reason2 := "agent_endpoint empty"

	// First evaluation: no previous reason → should log.
	if !admissionShouldLog("", reason1) {
		t.Error("first evaluation (empty prev) must trigger a log")
	}
	// Same reason again → must NOT trigger a log.
	if admissionShouldLog(reason1, reason1) {
		t.Error("identical reason must not trigger a log (duplicate suppression)")
	}
	// Different reason → must trigger a log.
	if !admissionShouldLog(reason1, reason2) {
		t.Error("reason change must trigger a log")
	}
	// Cleared reason (node admitted) → trigger a log.
	if !admissionShouldLog(reason1, "") {
		t.Error("clearing reason (admission success) must trigger a log")
	}
}

// ── Test 10: doctor findings map to proof failure reasons ─────────────────────

func TestAdmissionStatus_DoctorFindingsMappedCorrectly(t *testing.T) {
	type findingCase struct {
		name       string
		buildState func() (*nodeState, *controllerState)
		wantFinding string
		wantMsg    string
	}

	cases := []findingCase{
		{
			name: "agent_unreachable",
			buildState: func() (*nodeState, *controllerState) {
				node := baseNode("df1", "node-df1", []string{"core"})
				node.AgentEndpoint = ""
				jr := v2JoinRequest("df1", "node-df1", []string{"core"})
				return node, admissionState(jr, 0)
			},
			wantFinding: FindingAdmissionAgentUnreachable,
			wantMsg:     "node admission pending: node-agent not reachable (empty endpoint)",
		},
		{
			name: "missing_join_plan",
			buildState: func() (*nodeState, *controllerState) {
				node := baseNode("df2", "node-df2", []string{"core"})
				node.JoinLifecyclePhase = JoinPhaseAdmissionPending
				// No join record at all
				return node, admissionState(nil, 0)
			},
			wantFinding: FindingAdmissionMissingJoinPlan,
			wantMsg:     "node admission pending: JoinPlan missing",
		},
		{
			name: "identity_mismatch",
			buildState: func() (*nodeState, *controllerState) {
				jr := v2JoinRequest("df3", "node-df3", []string{"core"})
				node := baseNode("df3", "wrong-host", []string{"core"})
				return node, admissionState(jr, 0)
			},
			wantFinding: FindingAdmissionIdentityMismatch,
			wantMsg:     "node admission pending: reported identity does not match JoinPlan",
		},
		{
			name: "join_id_mismatch",
			buildState: func() (*nodeState, *controllerState) {
				jr := v2JoinRequest("df4", "node-df4", []string{"core"})
				jr.RequestID = "tampered"
				node := baseNode("df4", "node-df4", []string{"core"})
				return node, admissionState(jr, 0)
			},
			wantFinding: FindingAdmissionJoinIDMismatch,
			wantMsg:     "node admission pending: join_id does not match stored authorization",
		},
		{
			name: "etcd_unverified",
			buildState: func() (*nodeState, *controllerState) {
				jr := v2JoinRequest("df5", "node-df5", []string{"core", "control-plane"})
				node := baseNode("df5", "node-df5", []string{"core", "control-plane"})
				node.EtcdMemberIntent = &EtcdMemberIntent{Member: true}
				node.EtcdJoinPhase = EtcdJoinFailed
				return node, admissionState(jr, 0)
			},
			wantFinding: FindingAdmissionEtcdUnverified,
			wantMsg:     "node admission pending: etcd membership not verified",
		},
		{
			name: "scylla_unverified",
			buildState: func() (*nodeState, *controllerState) {
				jr := v2JoinRequest("df6", "node-df6", []string{"core", "storage"})
				node := baseNode("df6", "node-df6", []string{"core", "storage"})
				node.ScyllaIntent = &ScyllaIntent{Member: true}
				node.ScyllaJoinPhase = ScyllaJoinFailed
				return node, admissionState(jr, 0)
			},
			wantFinding: FindingAdmissionScyllaUnverified,
			wantMsg:     "node admission pending: Scylla membership not verified",
		},
		{
			name: "objectstore_gen_mismatch_active_block",
			buildState: func() (*nodeState, *controllerState) {
				jr := v2JoinRequest("df7", "node-df7", []string{"core", "storage"})
				node := baseNode("df7", "node-df7", []string{"core", "storage"})
				node.JoinLifecyclePhase = JoinPhaseAdmitted
				node.BootstrapPhase = BootstrapWorkloadReady
				node.ObjectStoreIntent = &ObjectStoreIntent{Member: true, TopologyGeneration: 1}
				return node, admissionState(jr, 9) // mismatch
			},
			wantFinding: FindingAdmissionObjectstoreGenerationMismatch,
			wantMsg:     "node active blocked: objectstore topology generation mismatch",
		},
		{
			name: "active_bootstrap_not_ready",
			buildState: func() (*nodeState, *controllerState) {
				jr := v2JoinRequest("df8", "node-df8", []string{"core"})
				node := baseNode("df8", "node-df8", []string{"core"})
				node.JoinLifecyclePhase = JoinPhaseAdmitted
				node.BootstrapPhase = BootstrapEtcdJoining // not yet workload_ready
				return node, admissionState(jr, 0)
			},
			wantFinding: FindingActiveBootstrapNotReady,
			wantMsg:     "node active blocked: workload_ready bootstrap phase not yet reached",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node, state := tc.buildState()
			result := evalFor(state, node)
			s := buildNodeAdmissionStatus(result)

			// For objectstore and active-bootstrap cases, the block is on ActiveOK,
			// not on OK. Use admissionFindingIDForActive for those.
			var gotFinding string
			if tc.wantFinding == FindingAdmissionObjectstoreGenerationMismatch ||
				tc.wantFinding == FindingActiveBootstrapNotReady {
				gotFinding = admissionFindingIDForActive(result, node)
			} else {
				gotFinding = s.FindingID
			}

			if gotFinding != tc.wantFinding {
				t.Errorf("finding: want %q, got %q (result.OK=%v reason=%q)",
					tc.wantFinding, gotFinding, result.OK, result.Reason)
			}
			msg := admissionOperatorMessage(tc.wantFinding)
			if msg != tc.wantMsg {
				t.Errorf("operator message: want %q, got %q", tc.wantMsg, msg)
			}
		})
	}
}

// ── Bonus: admissionStatusMetadata produces well-formed output ────────────────

func TestAdmissionStatus_MetadataShape(t *testing.T) {
	jr := v2JoinRequest("sm1", "node-sm1", []string{"core"})
	node := baseNode("sm1", "node-sm1", []string{"core"})
	node.JoinLifecyclePhase = JoinPhaseAdmissionPending
	state := admissionState(jr, 0)

	// Simulate what the handler does: evaluate + store.
	result := EvaluateNodeAdmissionProof(state, node)
	node.LastAdmissionProof = buildNodeAdmissionStatus(result)
	node.LastAdmissionProof.CheckedAt = time.Now()

	meta := admissionStatusMetadata(node)
	for _, key := range []string{"admission_path", "admission_ok", "admission_plan"} {
		if meta[key] == "" {
			t.Errorf("metadata key %q must be set", key)
		}
	}
	if meta["admission_path"] != "admission_pending" {
		t.Errorf("want admission_path=admission_pending, got %q", meta["admission_path"])
	}
	if meta["admission_plan"] != "v2" {
		t.Errorf("want admission_plan=v2, got %q", meta["admission_plan"])
	}
}
