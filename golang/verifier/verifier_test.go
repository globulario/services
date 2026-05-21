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
		InstalledSha256:    t.DesiredEntrypointChecksum,
		RunningPid:         4242,
		RunningExePath:     "/usr/lib/globular/bin/" + t.Service,
		RunningExeSha256:   t.DesiredEntrypointChecksum,
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
		Service:                   "foo",
		NodeID:                    "ryzen",
		DesiredVersion:            "1.2.0",
		DesiredBuildID:            "build-uuid-foo-v1",
		DesiredEntrypointChecksum: hashA,
		// DesiredPackageDigest is intentionally a DIFFERENT hash so any
		// future test that accidentally compares binary against tarball
		// fails loud instead of accidentally matching.
		DesiredPackageDigest: hashC,
		RuntimeNeeded:        true,
		ApplyTime:            time.Unix(1700000000, 0),
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

// ─────────────────────────────────────────────────────────────────────
// Hash-schema regression tests (v1.2.57).
//
// v1.2.56 shipped a false-positive package.installed_binary_hash_mismatch
// finding on every service because the verifier compared the binary on
// disk to the package tarball digest. These tests pin the rule that
// binary-vs-binary is the only legitimate comparison the verifier makes
// against InstalledSha256 / RunningExeSha256, and that
// DesiredPackageDigest is kept separately for future tarball audits.
//
// Exact observed shape from ryzen at 2026-05-21T00:36:01Z:
//
//   package_digest        = 5969ba6fa0b52d3d0066ba2df6a550cd55ba136de4bf5ed05faff82a78480f79
//   entrypoint_checksum   = 97c72402ea8cc361d083385680bb7c831e525128f4945bed15a00ad4defd547d
//   installed_sha256      = 97c72402ea8cc361d083385680bb7c831e525128f4945bed15a00ad4defd547d
//
// Expected verdict: clean. (v1.2.56 raised installed_binary_hash_mismatch.)
// ─────────────────────────────────────────────────────────────────────

const (
	observedPackageDigest      = "5969ba6fa0b52d3d0066ba2df6a550cd55ba136de4bf5ed05faff82a78480f79"
	observedEntrypointChecksum = "97c72402ea8cc361d083385680bb7c831e525128f4945bed15a00ad4defd547d"
)

func TestVerifyTarget_ObservedShape_NoFalsePositive(t *testing.T) {
	// Reproduces the exact ryzen 2026-05-21T00:36:01Z case. The binary
	// on disk matches the entrypoint_checksum; the package tarball
	// digest is unrelated. v1.2.56 raised installed_binary_hash_mismatch
	// here; v1.2.57 must not.
	tgt := Target{
		Service:                   "file",
		NodeID:                    "ryzen",
		DesiredVersion:            "1.2.56",
		DesiredBuildID:            "019e486f-ffb6-7b58-9087-c41946233781",
		DesiredEntrypointChecksum: observedEntrypointChecksum,
		DesiredPackageDigest:      observedPackageDigest, // different value — must NOT be compared against the binary
		RuntimeNeeded:             true,
		ApplyTime:                 time.Now().Add(-time.Hour),
	}
	ev := Evidence{Proof: &node_agentpb.ServiceRuntimeProof{
		ServiceName:        "file",
		NodeId:             "ryzen",
		InstalledPath:      "/usr/lib/globular/bin/file_server",
		InstalledSha256:    observedEntrypointChecksum, // binary matches entrypoint_checksum
		RunningPid:         204587,
		RunningExePath:     "/usr/lib/globular/bin/file_server",
		RunningExeSha256:   observedEntrypointChecksum,
		ProcessStartTime:   timestamppb.New(time.Now().Add(-time.Minute)),
		SystemdActiveState: "active",
		SystemdSubState:    "running",
	}}
	v := VerifyTarget(tgt, ev, time.Now())
	if findingsContain(v.Findings, FindingInstalledBinaryHashMismatch) {
		t.Fatalf("v1.2.56 false-positive must not return: findings=%+v", v.Findings)
	}
	if findingsContain(v.Findings, FindingRunningBinaryHashMismatch) {
		t.Fatalf("running-vs-installed comparison must agree when both equal entrypoint_checksum: %+v", v.Findings)
	}
}

func TestVerifyTarget_RealBinaryMismatch_StillFires(t *testing.T) {
	// Negative test: when the binary on disk genuinely differs from
	// the desired entrypoint_checksum, the finding MUST fire. The fix
	// reduces noise; it does not weaken diagnostic honesty.
	tgt := Target{
		Service:                   "file",
		NodeID:                    "ryzen",
		DesiredVersion:            "1.2.56",
		DesiredEntrypointChecksum: observedEntrypointChecksum,
		DesiredPackageDigest:      observedPackageDigest,
		RuntimeNeeded:             true,
		ApplyTime:                 time.Now().Add(-time.Hour),
	}
	ev := Evidence{Proof: &node_agentpb.ServiceRuntimeProof{
		ServiceName:        "file",
		NodeId:             "ryzen",
		InstalledPath:      "/usr/lib/globular/bin/file_server",
		InstalledSha256:    hashB, // wrong bytes on disk
		RunningExeSha256:   hashB,
		ProcessStartTime:   timestamppb.New(time.Now().Add(-time.Minute)),
		SystemdActiveState: "active",
	}}
	v := VerifyTarget(tgt, ev, time.Now())
	if !findingsContain(v.Findings, FindingInstalledBinaryHashMismatch) {
		t.Errorf("real binary mismatch must fire installed_binary_hash_mismatch; got %+v", v.Findings)
	}
}

func TestVerifyTarget_PackageDigestNotComparedToBinary(t *testing.T) {
	// Stronger pin: even with a totally hostile package_digest value
	// (the same hash byte-equal to hashB which would otherwise be a
	// "wrong binary" hash), the verifier MUST NOT use it for the binary
	// comparison. Binary-vs-binary is the only comparison this surface
	// makes.
	tgt := Target{
		Service:                   "x",
		NodeID:                    "n",
		DesiredEntrypointChecksum: hashA,
		DesiredPackageDigest:      hashB, // a value byte-equal to what an old buggy verifier would have used
		RuntimeNeeded:             true,
		ApplyTime:                 time.Now().Add(-time.Hour),
	}
	ev := Evidence{Proof: &node_agentpb.ServiceRuntimeProof{
		ServiceName:        "x",
		InstalledSha256:    hashA, // binary correct (matches entrypoint)
		RunningExeSha256:   hashA,
		ProcessStartTime:   timestamppb.New(time.Now().Add(-time.Minute)),
		SystemdActiveState: "active",
	}}
	v := VerifyTarget(tgt, ev, time.Now())
	if findingsContain(v.Findings, FindingInstalledBinaryHashMismatch) {
		t.Fatalf("verifier compared binary against DesiredPackageDigest — schema regression: %+v", v.Findings)
	}
}

// ─────────────────────────────────────────────────────────────────────
// bootstrap_ordering_skew classification — first install vs upgrade.
// ─────────────────────────────────────────────────────────────────────

func TestVerifyTarget_FirstInstallProcessOlderThanApply_BootstrapOrderingSkew(t *testing.T) {
	// On a fresh install, install.sh starts services before the
	// controller bootstrap records the apply. The verifier must NOT
	// raise old_pid_after_upgrade (critical) for this expected
	// sequencing; it raises bootstrap_ordering_skew (info-only) instead
	// so the verdict can still reach runtime_verified when the binary
	// identity is otherwise proven (v1.2.59 brief Part 3).
	tgt := targetFoo()
	tgt.IsFirstInstall = true
	ev := Evidence{Proof: proofMatching(tgt, func(p *node_agentpb.ServiceRuntimeProof) {
		p.ProcessStartTime = timestamppb.New(tgt.ApplyTime.Add(-time.Hour))
	})}
	v := VerifyTarget(tgt, ev, time.Now())
	if findingsContain(v.Findings, FindingOldPidAfterUpgrade) {
		t.Errorf("first install must NOT fire old_pid_after_upgrade; got %+v", v.Findings)
	}
	if !findingsContain(v.Findings, FindingBootstrapOrderingSkew) {
		t.Errorf("first install with process older than apply must fire bootstrap_ordering_skew; got %+v", v.Findings)
	}
	for _, f := range v.Findings {
		if f.ID == FindingBootstrapOrderingSkew && f.Severity != SeverityInfo {
			t.Errorf("bootstrap_ordering_skew on first install must be info; got %q", f.Severity)
		}
	}
	if v.ProofStatus != ProofRuntimeVerified {
		t.Errorf("first install + binary identity verified must reach runtime_verified despite bootstrap skew; got %q (findings=%+v)",
			v.ProofStatus, v.Findings)
	}
}

func TestVerifyTarget_UpgradeProcessOlderThanApply_StillCritical(t *testing.T) {
	// Same timing, but it's an upgrade — restart didn't take, this IS
	// stale bytes serving. Must stay critical.
	tgt := targetFoo()
	tgt.IsFirstInstall = false
	ev := Evidence{Proof: proofMatching(tgt, func(p *node_agentpb.ServiceRuntimeProof) {
		p.ProcessStartTime = timestamppb.New(tgt.ApplyTime.Add(-time.Hour))
	})}
	v := VerifyTarget(tgt, ev, time.Now())
	if !findingsContain(v.Findings, FindingOldPidAfterUpgrade) {
		t.Errorf("upgrade with process older than apply must fire old_pid_after_upgrade; got %+v", v.Findings)
	}
	for _, f := range v.Findings {
		if f.ID == FindingOldPidAfterUpgrade && f.Severity != SeverityCritical {
			t.Errorf("old_pid_after_upgrade must be critical; got %q", f.Severity)
		}
	}
}

// Sub-second skew between ExecMainStartTimestamp and ApplyTime is normal
// (systemd's start timestamp has only second precision; the controller
// writes ApplyTime at the end of the apply RPC). The verifier must not
// fire old_pid_after_upgrade for a process whose start time lands within
// applyGraceWindow of ApplyTime.
func TestVerifyTarget_UpgradeProcessWithinGraceWindow_NoFinding(t *testing.T) {
	tgt := targetFoo()
	tgt.IsFirstInstall = false
	ev := Evidence{Proof: proofMatching(tgt, func(p *node_agentpb.ServiceRuntimeProof) {
		// 250 ms before apply — well inside the grace window.
		p.ProcessStartTime = timestamppb.New(tgt.ApplyTime.Add(-250 * time.Millisecond))
	})}
	v := VerifyTarget(tgt, ev, time.Now())
	if findingsContain(v.Findings, FindingOldPidAfterUpgrade) {
		t.Errorf("sub-second skew must NOT fire old_pid_after_upgrade; got %+v", v.Findings)
	}
	if findingsContain(v.Findings, FindingBootstrapOrderingSkew) {
		t.Errorf("sub-second skew on upgrade must NOT fire bootstrap_ordering_skew either; got %+v", v.Findings)
	}
}

// Wrapper-package gate: keepalived, scylladb and other packages that
// ship only a no-op placeholder MUST NOT raise installed-vs-desired or
// running-vs-installed binary mismatches. The real binary is OS-supplied
// and we cannot meaningfully verify its hash.
func TestVerifyTarget_WrapsUpstreamBinary_SkipsBinaryHashChecks(t *testing.T) {
	tgt := targetFoo()
	tgt.WrapsUpstreamBinary = true
	// Force a state that would otherwise fire BOTH binary-hash findings:
	// desired says one SHA, installed says another, running says a third.
	tgt.DesiredEntrypointChecksum = strings.Repeat("a", 64)
	ev := Evidence{Proof: proofMatching(tgt, func(p *node_agentpb.ServiceRuntimeProof) {
		p.InstalledSha256 = strings.Repeat("b", 64)
		p.RunningExeSha256 = strings.Repeat("c", 64)
		p.ProcessStartTime = timestamppb.New(tgt.ApplyTime.Add(time.Minute))
	})}
	v := VerifyTarget(tgt, ev, time.Now())

	// Neither binary-identity finding may fire.
	if findingsContain(v.Findings, FindingInstalledBinaryHashMismatch) {
		t.Errorf("wrapper package must NOT fire %s; got %+v",
			FindingInstalledBinaryHashMismatch, v.Findings)
	}
	if findingsContain(v.Findings, FindingRunningBinaryHashMismatch) {
		t.Errorf("wrapper package must NOT fire %s; got %+v",
			FindingRunningBinaryHashMismatch, v.Findings)
	}

	// The wrapper-explained info finding MUST fire so operators see why.
	if !findingsContain(v.Findings, FindingPackageWrapsUpstreamBinary) {
		t.Errorf("wrapper package must surface %s as an explicit info marker; got %+v",
			FindingPackageWrapsUpstreamBinary, v.Findings)
	}
	for _, f := range v.Findings {
		if f.ID == FindingPackageWrapsUpstreamBinary && f.Severity != SeverityInfo {
			t.Errorf("%s must be info severity; got %q", FindingPackageWrapsUpstreamBinary, f.Severity)
		}
	}
}

// Heuristic detection: when the installed binary is outside the
// Globular-managed bin directories, treat as a wrapper even if the
// Target's explicit WrapsUpstreamBinary flag is false. This is the
// load-bearing detection path while the manifest's Entrypoints list
// isn't reliably populated.
func TestVerifyTarget_WrapsUpstreamBinary_DetectedFromInstalledPath(t *testing.T) {
	tgt := targetFoo()
	tgt.WrapsUpstreamBinary = false // not explicitly flagged
	tgt.DesiredEntrypointChecksum = strings.Repeat("a", 64)
	ev := Evidence{Proof: proofMatching(tgt, func(p *node_agentpb.ServiceRuntimeProof) {
		// Heuristic trigger: binary outside /usr/lib/globular/bin/
		p.InstalledPath = "/usr/sbin/keepalived"
		p.InstalledSha256 = strings.Repeat("b", 64)
		p.RunningExeSha256 = strings.Repeat("c", 64)
		p.ProcessStartTime = timestamppb.New(tgt.ApplyTime.Add(time.Minute))
	})}
	v := VerifyTarget(tgt, ev, time.Now())
	if findingsContain(v.Findings, FindingInstalledBinaryHashMismatch) {
		t.Errorf("binary at /usr/sbin/ must be detected as upstream wrapper; got installed_binary_hash_mismatch")
	}
	if !findingsContain(v.Findings, FindingPackageWrapsUpstreamBinary) {
		t.Errorf("heuristic must emit wraps_upstream_binary info finding")
	}
}

func TestInstalledPathIsUpstream(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/usr/lib/globular/bin/dns_server", false}, // Globular-managed
		{"/usr/local/lib/globular/bin/file_server", false},
		{"/usr/sbin/keepalived", true},   // OS package
		{"/usr/bin/scylla", true},        // OS wrapper script
		{"/opt/scylladb/libexec/scylla", true},
		{"", false}, // empty path is conservative — don't false-positive
	}
	for _, c := range cases {
		if got := installedPathIsUpstream(c.path); got != c.want {
			t.Errorf("installedPathIsUpstream(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

// Non-wrapper packages with a real binary drift MUST still fire the
// critical findings — guard against the wrapper gate accidentally
// silencing legitimate drift.
func TestVerifyTarget_NonWrapperPackage_StillFiresBinaryHashMismatch(t *testing.T) {
	tgt := targetFoo()
	tgt.WrapsUpstreamBinary = false
	tgt.DesiredEntrypointChecksum = strings.Repeat("a", 64)
	ev := Evidence{Proof: proofMatching(tgt, func(p *node_agentpb.ServiceRuntimeProof) {
		p.InstalledSha256 = strings.Repeat("b", 64) // drift
		p.RunningExeSha256 = strings.Repeat("b", 64)
		p.ProcessStartTime = timestamppb.New(tgt.ApplyTime.Add(time.Minute))
	})}
	v := VerifyTarget(tgt, ev, time.Now())
	if !findingsContain(v.Findings, FindingInstalledBinaryHashMismatch) {
		t.Errorf("non-wrapper drift must still fire %s; got %+v",
			FindingInstalledBinaryHashMismatch, v.Findings)
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
