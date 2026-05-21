package verifier

// verifier_test.go — Phase 9 of the Diagnostic Honesty Refactor.
//
// Pins the contract of VerifyTarget + AggregateResult by reproducing
// every signature failure pattern the brief calls out:
//
//   - old binary on disk but inventory says installed
//   - new binary on disk but old PID running
//   - partial rollout marked converged (release-level finding)
//   - fallback active without alarm
//   - duplicate systemd Type= (caught in deploy package; here we
//     verify the systemd.effective_config_drift finding when the
//     effective unit disagrees with rendered)
//   - cross-node webroot drift
//
// All checks are pure: tests construct ServiceRuntimeProof messages
// directly. No node-agent process or systemctl is exercised.

import (
	"strings"
	"testing"
	"time"

	"github.com/globulario/services/golang/crossnodedrift"
	"github.com/globulario/services/golang/fallback"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	hashA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	hashB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	hashC = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
)

func proofMatching(t Target, opts ...func(*node_agentpb.ServiceRuntimeProof)) *node_agentpb.ServiceRuntimeProof {
	p := &node_agentpb.ServiceRuntimeProof{
		ServiceName:        t.Service,
		NodeId:             t.NodeID,
		ExpectedBuildId:    t.DesiredBuildID,
		ExpectedVersion:    t.DesiredVersion,
		InstalledPath:      "/usr/lib/globular/bin/" + t.Service,
		InstalledSha256:    t.DesiredHash,
		RunningPid:         4242,
		RunningExePath:     "/usr/lib/globular/bin/" + t.Service,
		RunningExeSha256:   t.DesiredHash,
		SystemdActiveState: "active",
		SystemdSubState:    "running",
		SystemdUnitPath:    "/etc/systemd/system/globular-" + t.Service + ".service",
		EffectiveType:      "simple",
		EffectiveExecStart: "{ path=/usr/lib/globular/bin/" + t.Service + " ; argv[]=" + t.Service + " }",
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

func targetFoo() Target {
	return Target{
		Service:        "foo",
		NodeID:         "ryzen",
		DesiredVersion: "1.2.0",
		DesiredBuildID: "build-uuid-foo-v1",
		DesiredHash:    hashA,
		RuntimeNeeded:  true,
		ApplyTime:      time.Unix(1700000000, 0),
	}
}

// ─────────────────────────────────────────────────────────────────────
// Happy path — every proof agrees → ProofRuntimeVerified, no findings.
// ─────────────────────────────────────────────────────────────────────

func TestVerifyTarget_AllAgree_RuntimeVerified(t *testing.T) {
	tgt := targetFoo()
	ev := Evidence{Proof: proofMatching(tgt, func(p *node_agentpb.ServiceRuntimeProof) {
		// Make the running PID newer than the apply time.
		p.ProcessStartTime = timestamppb.New(tgt.ApplyTime.Add(time.Minute))
	})}
	v := VerifyTarget(tgt, ev, time.Now())
	if v.ProofStatus != ProofRuntimeVerified {
		t.Fatalf("ProofStatus=%q want=%q (findings=%+v)", v.ProofStatus, ProofRuntimeVerified, v.Findings)
	}
	if len(v.Findings) != 0 {
		t.Errorf("expected no findings on the happy path; got %+v", v.Findings)
	}
}

// ─────────────────────────────────────────────────────────────────────
// Brief signature case 1: old binary on disk while inventory says
// installed at the new version. Installed-vs-desired hash disagrees.
// ─────────────────────────────────────────────────────────────────────

func TestVerifyTarget_OldBinaryOnDisk_InstalledHashMismatch(t *testing.T) {
	tgt := targetFoo()
	ev := Evidence{Proof: proofMatching(tgt, func(p *node_agentpb.ServiceRuntimeProof) {
		p.InstalledSha256 = hashB // disk holds wrong artifact
		p.RunningExeSha256 = hashB
	})}
	v := VerifyTarget(tgt, ev, time.Now())
	if v.ProofStatus != ProofMismatch {
		t.Fatalf("ProofStatus=%q want=%q", v.ProofStatus, ProofMismatch)
	}
	if !findingsContain(v.Findings, FindingInstalledBinaryHashMismatch) {
		t.Errorf("missing %s finding; got %+v", FindingInstalledBinaryHashMismatch, v.Findings)
	}
}

// ─────────────────────────────────────────────────────────────────────
// Brief signature case 2: new binary on disk but old PID still serving.
// Installed-vs-running hashes disagree.
// ─────────────────────────────────────────────────────────────────────

func TestVerifyTarget_NewBinaryOldPid_RunningBinaryHashMismatch(t *testing.T) {
	tgt := targetFoo()
	ev := Evidence{Proof: proofMatching(tgt, func(p *node_agentpb.ServiceRuntimeProof) {
		// Disk holds the new binary…
		p.InstalledSha256 = hashA
		// …but the live PID is running the old one.
		p.RunningExeSha256 = hashB
	})}
	v := VerifyTarget(tgt, ev, time.Now())
	if v.ProofStatus != ProofMismatch {
		t.Fatalf("ProofStatus=%q want=%q", v.ProofStatus, ProofMismatch)
	}
	if !findingsContain(v.Findings, FindingRunningBinaryHashMismatch) {
		t.Errorf("missing %s finding; got %+v", FindingRunningBinaryHashMismatch, v.Findings)
	}
}

// ─────────────────────────────────────────────────────────────────────
// Old PID after upgrade — process_start_time predates the apply.
// ─────────────────────────────────────────────────────────────────────

func TestVerifyTarget_ProcessOlderThanApply_OldPidAfterUpgrade(t *testing.T) {
	tgt := targetFoo()
	ev := Evidence{Proof: proofMatching(tgt, func(p *node_agentpb.ServiceRuntimeProof) {
		// PID started BEFORE the controller commanded the apply.
		p.ProcessStartTime = timestamppb.New(tgt.ApplyTime.Add(-time.Hour))
	})}
	v := VerifyTarget(tgt, ev, time.Now())
	if !findingsContain(v.Findings, FindingOldPidAfterUpgrade) {
		t.Errorf("missing %s finding; got %+v", FindingOldPidAfterUpgrade, v.Findings)
	}
	if v.ProofStatus != ProofMismatch {
		t.Errorf("ProofStatus=%q want=%q", v.ProofStatus, ProofMismatch)
	}
}

// ─────────────────────────────────────────────────────────────────────
// Runtime version mismatch — live process /version disagrees.
// ─────────────────────────────────────────────────────────────────────

func TestVerifyTarget_RuntimeVersionDiffers_VersionMismatch(t *testing.T) {
	tgt := targetFoo()
	ev := Evidence{Proof: proofMatching(tgt, func(p *node_agentpb.ServiceRuntimeProof) {
		p.RuntimeVersion = "1.1.0" // desired is 1.2.0
		p.ProcessStartTime = timestamppb.New(tgt.ApplyTime.Add(time.Minute))
	})}
	v := VerifyTarget(tgt, ev, time.Now())
	if !findingsContain(v.Findings, FindingRunningVersionMismatch) {
		t.Errorf("missing %s finding; got %+v", FindingRunningVersionMismatch, v.Findings)
	}
}

// ─────────────────────────────────────────────────────────────────────
// Brief signature case: duplicate systemd Type= shipped → effective
// disagrees with rendered. Phase 5b's drift detector raises the finding,
// the verifier surfaces it.
// ─────────────────────────────────────────────────────────────────────

func TestVerifyTarget_RenderedVsEffective_TypeDriftRaisesFinding(t *testing.T) {
	tgt := targetFoo()
	ev := Evidence{
		Proof: proofMatching(tgt, func(p *node_agentpb.ServiceRuntimeProof) {
			p.EffectiveType = "forking" // rendered is simple
			p.ProcessStartTime = timestamppb.New(tgt.ApplyTime.Add(time.Minute))
		}),
		RenderedUnit: `[Unit]
Description=Globular foo
[Service]
Type=simple
ExecStart=/usr/lib/globular/bin/foo
`,
	}
	v := VerifyTarget(tgt, ev, time.Now())
	if !findingsContain(v.Findings, FindingSystemdEffectiveConfigDrift) {
		t.Errorf("missing %s finding; got %+v", FindingSystemdEffectiveConfigDrift, v.Findings)
	}
}

// ─────────────────────────────────────────────────────────────────────
// Partial-proof case: errors[] non-empty and no harder finding. The
// verifier degrades to ProofUnknown + runtime_identity_unproven.
// ─────────────────────────────────────────────────────────────────────

func TestVerifyTarget_ErrorsOnly_DegradesUnknown(t *testing.T) {
	tgt := targetFoo()
	ev := Evidence{Proof: proofMatching(tgt, func(p *node_agentpb.ServiceRuntimeProof) {
		p.ProcessStartTime = timestamppb.New(tgt.ApplyTime.Add(time.Minute))
		p.Errors = []string{"runtime_version probe not implemented"}
	})}
	v := VerifyTarget(tgt, ev, time.Now())
	if v.ProofStatus != ProofUnknown {
		t.Errorf("ProofStatus=%q want=%q (degraded-only)", v.ProofStatus, ProofUnknown)
	}
	if !findingsContain(v.Findings, FindingRuntimeIdentityUnproven) {
		t.Errorf("missing %s finding; got %+v", FindingRuntimeIdentityUnproven, v.Findings)
	}
}

// ─────────────────────────────────────────────────────────────────────
// No proof captured → runtime_identity_unproven, ProofUnknown.
// ─────────────────────────────────────────────────────────────────────

func TestVerifyTarget_NoProof_Unknown(t *testing.T) {
	tgt := targetFoo()
	v := VerifyTarget(tgt, Evidence{}, time.Now())
	if v.ProofStatus != ProofUnknown {
		t.Errorf("ProofStatus=%q want=%q", v.ProofStatus, ProofUnknown)
	}
	if !findingsContain(v.Findings, FindingRuntimeIdentityUnproven) {
		t.Errorf("missing %s finding; got %+v", FindingRuntimeIdentityUnproven, v.Findings)
	}
}

// ─────────────────────────────────────────────────────────────────────
// AggregateResult — partial rollout: 4 verified, 1 mismatched.
// Counts roll up and individual findings survive.
// ─────────────────────────────────────────────────────────────────────

func TestAggregateResult_PartialRollout_SummaryCounts(t *testing.T) {
	now := time.Now()
	verdicts := []Verdict{
		{Target: Target{Service: "foo", NodeID: "n1"}, ProofStatus: ProofRuntimeVerified},
		{Target: Target{Service: "foo", NodeID: "n2"}, ProofStatus: ProofRuntimeVerified},
		{Target: Target{Service: "foo", NodeID: "n3"}, ProofStatus: ProofInstalledVerified},
		{Target: Target{Service: "foo", NodeID: "n4"}, ProofStatus: ProofInventoryClaim},
		{Target: Target{Service: "foo", NodeID: "n5"}, ProofStatus: ProofMismatch, Findings: []Finding{
			{ID: FindingRunningBinaryHashMismatch, Severity: SeverityCritical},
		}},
	}
	r := AggregateResult(verdicts, nil, nil, now)
	if r.Summary.TotalTargets != 5 {
		t.Errorf("Total=%d want 5", r.Summary.TotalTargets)
	}
	if r.Summary.Verified != 2 {
		t.Errorf("Verified=%d want 2", r.Summary.Verified)
	}
	if r.Summary.InstalledOnly != 1 {
		t.Errorf("InstalledOnly=%d want 1", r.Summary.InstalledOnly)
	}
	if r.Summary.InventoryOnly != 1 {
		t.Errorf("InventoryOnly=%d want 1", r.Summary.InventoryOnly)
	}
	if r.Summary.Mismatched != 1 {
		t.Errorf("Mismatched=%d want 1", r.Summary.Mismatched)
	}
}

// ─────────────────────────────────────────────────────────────────────
// AggregateResult — fallback active surfaces as a CrossFinding even
// when every per-target verdict is clean. The brief case: "fallback
// active without alarm" gets the alarm.
// ─────────────────────────────────────────────────────────────────────

func TestAggregateResult_FallbackActive_SurfacesAsCrossFinding(t *testing.T) {
	fbs := []fallback.Active{
		{
			Service: "repository", Dependency: "scylladb", Mode: "minio_read",
			PrimaryError: "context deadline exceeded",
			NodeID:       "ryzen",
			Since:        time.Now().Add(-3 * time.Minute),
		},
	}
	r := AggregateResult(nil, fbs, nil, time.Now())
	if len(r.CrossFindings) != 1 {
		t.Fatalf("CrossFindings=%d want 1", len(r.CrossFindings))
	}
	if r.CrossFindings[0].ID != FindingSilentFallbackActive {
		t.Errorf("finding id=%q want=%q", r.CrossFindings[0].ID, FindingSilentFallbackActive)
	}
	if r.CrossFindings[0].Evidence["dependency"] != "scylladb" {
		t.Errorf("evidence dependency=%q want scylladb", r.CrossFindings[0].Evidence["dependency"])
	}
	if r.Summary.FallbacksActive != 1 {
		t.Errorf("FallbacksActive=%d want 1", r.Summary.FallbacksActive)
	}
}

// ─────────────────────────────────────────────────────────────────────
// AggregateResult — Phase 7 cross-node drift verdict from a single
// path class surfaces as one CrossFinding, summary counts a class.
// ─────────────────────────────────────────────────────────────────────

func TestAggregateResult_CrossNodeDrift_SurfacesAsCrossFinding(t *testing.T) {
	dvs := []crossnodedrift.DriftVerdict{
		{
			PathClass: "webroot",
			Path:      "index.html",
			Status:    crossnodedrift.DriftStatusDrift,
			FindingID: crossnodedrift.FindingID,
			Drifts:    []string{"ryzen: present hash=abcd", "nuc: absent"},
		},
		{
			PathClass: "rendered_systemd_units",
			Path:      "globular-foo.service",
			Status:    crossnodedrift.DriftStatusConsistent,
		},
	}
	r := AggregateResult(nil, nil, dvs, time.Now())
	if len(r.CrossFindings) != 1 {
		t.Fatalf("CrossFindings=%d want 1 (only the drift entry; consistent should be skipped)", len(r.CrossFindings))
	}
	if r.CrossFindings[0].ID != FindingCrossNodeFileDrift {
		t.Errorf("finding id=%q want=%q", r.CrossFindings[0].ID, FindingCrossNodeFileDrift)
	}
	if r.Summary.DriftedClasses != 1 {
		t.Errorf("DriftedClasses=%d want 1", r.Summary.DriftedClasses)
	}
}

// ─────────────────────────────────────────────────────────────────────
// AggregateResult — authority_undefined drift verdicts are surfaced
// with their own finding id, NOT collapsed into cross_node_file_drift.
// ─────────────────────────────────────────────────────────────────────

func TestAggregateResult_AuthorityUndefined_PreservesFindingId(t *testing.T) {
	dvs := []crossnodedrift.DriftVerdict{
		{
			PathClass: "mystery",
			Path:      "x",
			Status:    crossnodedrift.DriftStatusAuthorityUndefined,
			FindingID: crossnodedrift.FindingAuthorityUndefined,
		},
	}
	r := AggregateResult(nil, nil, dvs, time.Now())
	if len(r.CrossFindings) != 1 {
		t.Fatalf("CrossFindings=%d want 1", len(r.CrossFindings))
	}
	if r.CrossFindings[0].ID != FindingAuthorityUndefined {
		t.Errorf("finding id=%q want=%q", r.CrossFindings[0].ID, FindingAuthorityUndefined)
	}
}

// ─────────────────────────────────────────────────────────────────────
// EtcdKeyForVerification — pin the path layout from the brief.
// ─────────────────────────────────────────────────────────────────────

func TestEtcdKeyForVerification_LayoutMatchesBrief(t *testing.T) {
	got := EtcdKeyForVerification("ryzen", "foo")
	want := "/globular/verification/runtime/ryzen/foo"
	if got != want {
		t.Errorf("EtcdKey=%q want=%q", got, want)
	}
}

// ─────────────────────────────────────────────────────────────────────
// Multiple findings stack: one target with both running-hash drift and
// runtime version drift. Both must surface and ProofStatus is mismatch.
// ─────────────────────────────────────────────────────────────────────

func TestVerifyTarget_MultipleFindings_StackAndCapAtMismatch(t *testing.T) {
	tgt := targetFoo()
	ev := Evidence{Proof: proofMatching(tgt, func(p *node_agentpb.ServiceRuntimeProof) {
		p.RunningExeSha256 = hashB        // hash drift
		p.RuntimeVersion = "0.9.0"        // version drift
	})}
	v := VerifyTarget(tgt, ev, time.Now())
	if v.ProofStatus != ProofMismatch {
		t.Errorf("ProofStatus=%q want=%q", v.ProofStatus, ProofMismatch)
	}
	want := []string{FindingRunningBinaryHashMismatch, FindingRunningVersionMismatch}
	for _, id := range want {
		if !findingsContain(v.Findings, id) {
			t.Errorf("missing %s finding; got %+v", id, v.Findings)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────
// Buildup — Verdict.Reason includes every finding id when mismatch.
// Doctor surfaces this in operator UIs.
// ─────────────────────────────────────────────────────────────────────

func TestVerifyTarget_ReasonNamesFindings(t *testing.T) {
	tgt := targetFoo()
	ev := Evidence{Proof: proofMatching(tgt, func(p *node_agentpb.ServiceRuntimeProof) {
		p.RunningExeSha256 = hashB
	})}
	v := VerifyTarget(tgt, ev, time.Now())
	if !strings.Contains(v.Reason, FindingRunningBinaryHashMismatch) {
		t.Errorf("Reason=%q must name the finding", v.Reason)
	}
}

// Helpers ─────────────────────────────────────────────────────────────

func findingsContain(fs []Finding, id string) bool {
	for _, f := range fs {
		if f.ID == id {
			return true
		}
	}
	return false
}
