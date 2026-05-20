package main

// release_proof_status_test.go — Phase 4 (Diagnostic Honesty Refactor).
//
// Pins the contract of decideNodeRolloutProof and aggregateRolloutProof.
// The strict promise: a release at AVAILABLE whose floor is below
// installed_verified MUST carry the rollout.partial_not_converged finding.
// 4-of-5 inventory-claim nodes is PARTIAL, not CONVERGED.

import (
	"reflect"
	"sort"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

func ip(version, checksum, buildID string) *node_agentpb.InstalledPackage {
	return &node_agentpb.InstalledPackage{
		Version:  version,
		Checksum: checksum,
		BuildId:  buildID,
	}
}

// ─────────────────────────────────────────────────────────────────────────
// decideNodeRolloutProof — per-node verdict.
// ─────────────────────────────────────────────────────────────────────────

func TestDecideNodeRolloutProof_NoInstalled_ProofMissing(t *testing.T) {
	v := decideNodeRolloutProof("1.2.0", "abc", "bid-a", nil, true, false)
	if v.ProofStatus != RolloutProofUnknown {
		t.Errorf("ProofStatus=%q want=%q", v.ProofStatus, RolloutProofUnknown)
	}
	if v.FindingID != FindingRolloutProofMissing {
		t.Errorf("FindingID=%q want=%q", v.FindingID, FindingRolloutProofMissing)
	}
}

func TestDecideNodeRolloutProof_HashMismatch_Mismatch(t *testing.T) {
	v := decideNodeRolloutProof(
		"1.2.0", "sha256:aaaa", "bid-a",
		ip("1.2.0", "sha256:bbbb", "bid-a"),
		true, true,
	)
	if v.ProofStatus != RolloutProofMismatch {
		t.Errorf("ProofStatus=%q want=%q", v.ProofStatus, RolloutProofMismatch)
	}
	if v.FindingID != FindingRolloutInstalledHashMismatch {
		t.Errorf("FindingID=%q want=%q", v.FindingID, FindingRolloutInstalledHashMismatch)
	}
}

func TestDecideNodeRolloutProof_BuildIdMismatch_Mismatch(t *testing.T) {
	v := decideNodeRolloutProof(
		"1.2.0", "", "bid-desired",
		ip("1.2.0", "", "bid-installed"),
		true, true,
	)
	if v.ProofStatus != RolloutProofMismatch {
		t.Errorf("ProofStatus=%q want=%q", v.ProofStatus, RolloutProofMismatch)
	}
	if v.FindingID != FindingRolloutInstalledBuildIdMismatch {
		t.Errorf("FindingID=%q want=%q", v.FindingID, FindingRolloutInstalledBuildIdMismatch)
	}
}

func TestDecideNodeRolloutProof_VersionMismatch_Mismatch(t *testing.T) {
	v := decideNodeRolloutProof(
		"1.2.0", "", "",
		ip("1.1.0", "", ""),
		true, true,
	)
	if v.ProofStatus != RolloutProofMismatch {
		t.Errorf("ProofStatus=%q want=%q", v.ProofStatus, RolloutProofMismatch)
	}
	if v.FindingID != FindingRolloutInstalledVersionMismatch {
		t.Errorf("FindingID=%q want=%q", v.FindingID, FindingRolloutInstalledVersionMismatch)
	}
}

func TestDecideNodeRolloutProof_HashAndBuildMatch_RuntimeActive_InstalledVerified(t *testing.T) {
	v := decideNodeRolloutProof(
		"1.2.0", "sha256:aaaa", "bid-a",
		ip("1.2.0", "sha256:aaaa", "bid-a"),
		true, true,
	)
	if v.ProofStatus != RolloutProofInstalledVerified {
		t.Errorf("ProofStatus=%q want=%q", v.ProofStatus, RolloutProofInstalledVerified)
	}
	if v.FindingID != "" {
		t.Errorf("FindingID=%q want empty for successful verification", v.FindingID)
	}
}

func TestDecideNodeRolloutProof_BuildMatch_RuntimeNotNeeded_InstalledVerified(t *testing.T) {
	// COMMAND-kind packages (mc, restic, rclone) don't need a running unit.
	v := decideNodeRolloutProof(
		"1.2.0", "", "bid-a",
		ip("1.2.0", "", "bid-a"),
		false, false,
	)
	if v.ProofStatus != RolloutProofInstalledVerified {
		t.Errorf("runtime not needed: ProofStatus=%q want=%q", v.ProofStatus, RolloutProofInstalledVerified)
	}
}

func TestDecideNodeRolloutProof_HashMatch_RuntimeNotActive_PartialNotConverged(t *testing.T) {
	// Phase 4's signature failure mode: new binary on disk, old PID
	// (or no PID) actually running.
	v := decideNodeRolloutProof(
		"1.2.0", "sha256:aaaa", "bid-a",
		ip("1.2.0", "sha256:aaaa", "bid-a"),
		true, false,
	)
	if v.ProofStatus != RolloutProofMismatch {
		t.Errorf("ProofStatus=%q want=%q", v.ProofStatus, RolloutProofMismatch)
	}
	if v.FindingID != FindingRolloutPartialNotConverged {
		t.Errorf("FindingID=%q want=%q", v.FindingID, FindingRolloutPartialNotConverged)
	}
}

func TestDecideNodeRolloutProof_NoHashNoBuild_InventoryClaim(t *testing.T) {
	// Pre-Phase-1 release: no desired hash or build_id to compare against.
	// We can only take the node-agent's word for it.
	v := decideNodeRolloutProof(
		"1.2.0", "", "",
		ip("1.2.0", "", ""),
		true, true,
	)
	if v.ProofStatus != RolloutProofInventoryClaim {
		t.Errorf("ProofStatus=%q want=%q", v.ProofStatus, RolloutProofInventoryClaim)
	}
	if v.FindingID != "" {
		t.Errorf("FindingID=%q want empty (claim only, no drift)", v.FindingID)
	}
}

// Hash normalization parity: sha256: prefix and case must be stripped on
// both sides before comparison.
func TestDecideNodeRolloutProof_HashNormalization(t *testing.T) {
	v := decideNodeRolloutProof(
		"1.2.0", "SHA256:ABCD", "",
		ip("1.2.0", "sha256:abcd", ""),
		false, false,
	)
	if v.ProofStatus != RolloutProofInstalledVerified {
		t.Errorf("case+prefix normalization broken: ProofStatus=%q", v.ProofStatus)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// aggregateRolloutProof — release-level roll-up.
// ─────────────────────────────────────────────────────────────────────────

func TestAggregateRolloutProof_AllVerified_NoFinding(t *testing.T) {
	verdicts := []NodeRolloutProofVerdict{
		{ProofStatus: RolloutProofInstalledVerified},
		{ProofStatus: RolloutProofInstalledVerified},
		{ProofStatus: RolloutProofInstalledVerified},
	}
	agg := aggregateRolloutProof(verdicts, true)
	if agg.ProofStatus != RolloutProofInstalledVerified {
		t.Errorf("ProofStatus=%q want=%q", agg.ProofStatus, RolloutProofInstalledVerified)
	}
	if len(agg.Findings) != 0 {
		t.Errorf("findings should be empty; got %v", agg.Findings)
	}
}

func TestAggregateRolloutProof_PartialRollout_PartialNotConvergedFinding(t *testing.T) {
	// The brief's signature test: 4-of-5 installed claim but only 1-of-5
	// runtime verified. Floor must be inventory_claim and the finding
	// rollout.partial_not_converged must be emitted at release scope.
	verdicts := []NodeRolloutProofVerdict{
		{ProofStatus: RolloutProofInstalledVerified},
		{ProofStatus: RolloutProofInventoryClaim},
		{ProofStatus: RolloutProofInventoryClaim},
		{ProofStatus: RolloutProofInventoryClaim},
		{ProofStatus: RolloutProofInventoryClaim},
	}
	agg := aggregateRolloutProof(verdicts, true)
	if agg.ProofStatus != RolloutProofInventoryClaim {
		t.Errorf("ProofStatus=%q want=%q (floor across nodes)", agg.ProofStatus, RolloutProofInventoryClaim)
	}
	found := false
	for _, f := range agg.Findings {
		if f == FindingRolloutPartialNotConverged {
			found = true
		}
	}
	if !found {
		t.Errorf("findings should include %q; got %v", FindingRolloutPartialNotConverged, agg.Findings)
	}
}

func TestAggregateRolloutProof_MismatchFloor_KeepsPerNodeFinding(t *testing.T) {
	// One node has a real drift (hash mismatch) — the per-node finding
	// must roll up to the release alongside partial_not_converged.
	verdicts := []NodeRolloutProofVerdict{
		{ProofStatus: RolloutProofInstalledVerified},
		{ProofStatus: RolloutProofMismatch, FindingID: FindingRolloutInstalledHashMismatch},
	}
	agg := aggregateRolloutProof(verdicts, true)
	if agg.ProofStatus != RolloutProofMismatch {
		t.Errorf("ProofStatus=%q want=%q (mismatch beats verified)", agg.ProofStatus, RolloutProofMismatch)
	}
	sort.Strings(agg.Findings)
	want := []string{FindingRolloutInstalledHashMismatch, FindingRolloutPartialNotConverged}
	sort.Strings(want)
	if !reflect.DeepEqual(agg.Findings, want) {
		t.Errorf("findings=%v want=%v", agg.Findings, want)
	}
}

func TestAggregateRolloutProof_NotAtAvailable_NoPartialFinding(t *testing.T) {
	// While the release is still PENDING/RESOLVED, inventory_claim is
	// expected (the workflow hasn't finished). Don't spam the operator
	// with partial_not_converged during normal progression.
	verdicts := []NodeRolloutProofVerdict{
		{ProofStatus: RolloutProofInventoryClaim},
		{ProofStatus: RolloutProofInventoryClaim},
	}
	agg := aggregateRolloutProof(verdicts, false)
	if agg.ProofStatus != RolloutProofInventoryClaim {
		t.Errorf("ProofStatus=%q want=%q", agg.ProofStatus, RolloutProofInventoryClaim)
	}
	for _, f := range agg.Findings {
		if f == FindingRolloutPartialNotConverged {
			t.Errorf("finding %q must NOT be emitted while release is below AVAILABLE", FindingRolloutPartialNotConverged)
		}
	}
}

func TestAggregateRolloutProof_EmptyVerdicts_Unknown(t *testing.T) {
	agg := aggregateRolloutProof(nil, true)
	if agg.ProofStatus != RolloutProofUnknown {
		t.Errorf("ProofStatus=%q want=unknown for empty verdicts", agg.ProofStatus)
	}
	if len(agg.Findings) != 0 {
		t.Errorf("findings should be empty; got %v", agg.Findings)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// rolloutProofRank ordering — pinning the sort.
// ─────────────────────────────────────────────────────────────────────────

func TestRolloutProofRank_Order(t *testing.T) {
	if !(rolloutProofRank(RolloutProofRuntimeVerified) >
		rolloutProofRank(RolloutProofInstalledVerified) &&
		rolloutProofRank(RolloutProofInstalledVerified) >
			rolloutProofRank(RolloutProofInventoryClaim) &&
		rolloutProofRank(RolloutProofInventoryClaim) >
			rolloutProofRank(RolloutProofUnknown) &&
		rolloutProofRank(RolloutProofUnknown) >
			rolloutProofRank(RolloutProofMismatch)) {
		t.Errorf("rank ordering wrong: runtime=%d installed=%d inv=%d unknown=%d mismatch=%d",
			rolloutProofRank(RolloutProofRuntimeVerified),
			rolloutProofRank(RolloutProofInstalledVerified),
			rolloutProofRank(RolloutProofInventoryClaim),
			rolloutProofRank(RolloutProofUnknown),
			rolloutProofRank(RolloutProofMismatch))
	}
}

func TestRolloutProofMin_PicksWeaker(t *testing.T) {
	if rolloutProofMin(RolloutProofInstalledVerified, RolloutProofInventoryClaim) != RolloutProofInventoryClaim {
		t.Error("min picked stronger")
	}
	if rolloutProofMin(RolloutProofMismatch, RolloutProofInstalledVerified) != RolloutProofMismatch {
		t.Error("mismatch must dominate")
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Wire-up: statusPatch → applyPatchToSvcStatus persists ProofStatus and
// Findings on the "nodes" path. This is the seam where the per-node verdicts
// computed by detectServiceDrift actually reach etcd, so an integration test
// at the patch layer catches a future regression where someone removes the
// new fields from the patch shape.
// ─────────────────────────────────────────────────────────────────────────

func TestApplyPatchToSvcStatus_NodesPath_PersistsProofStatusAndFindings(t *testing.T) {
	s := &cluster_controllerpb.ServiceReleaseStatus{}
	p := statusPatch{
		SetFields:    "nodes",
		Phase:        cluster_controllerpb.ReleasePhaseAvailable,
		Nodes:        []*cluster_controllerpb.NodeReleaseStatus{{NodeID: "n1", ProofStatus: RolloutProofInventoryClaim}},
		ProofStatus:  RolloutProofInventoryClaim,
		Findings:     []string{FindingRolloutPartialNotConverged},
		LastTransitionUnixMs: 1,
	}
	if !applyPatchToSvcStatus(s, p) {
		t.Fatal("applyPatchToSvcStatus returned false; expected mutation")
	}
	if s.ProofStatus != RolloutProofInventoryClaim {
		t.Errorf("status.ProofStatus=%q want=%q", s.ProofStatus, RolloutProofInventoryClaim)
	}
	if len(s.Findings) != 1 || s.Findings[0] != FindingRolloutPartialNotConverged {
		t.Errorf("status.Findings=%v want=[%s]", s.Findings, FindingRolloutPartialNotConverged)
	}
}

func TestApplyPatchToSvcStatus_PhasePath_DoesNotClobberProofStatus(t *testing.T) {
	// A phase-only patch (e.g. resolved→applying transition) should leave
	// ProofStatus / Findings untouched. Otherwise every workflow tick would
	// wipe the rollout proof verdict the drift detector just wrote.
	s := &cluster_controllerpb.ServiceReleaseStatus{
		Phase:       cluster_controllerpb.ReleasePhaseAvailable,
		ProofStatus: RolloutProofInstalledVerified,
		Findings:    []string{FindingRolloutPartialNotConverged},
	}
	p := statusPatch{
		SetFields: "phase",
		Phase:     cluster_controllerpb.ReleasePhaseResolved,
	}
	if !applyPatchToSvcStatus(s, p) {
		t.Fatal("applyPatchToSvcStatus returned false")
	}
	if s.ProofStatus != RolloutProofInstalledVerified {
		t.Errorf("phase patch clobbered ProofStatus: %q", s.ProofStatus)
	}
	if len(s.Findings) != 1 {
		t.Errorf("phase patch clobbered Findings: %v", s.Findings)
	}
}
