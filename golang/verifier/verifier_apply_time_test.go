package verifier

// verifier_apply_time_test.go — regression tests for the per-node ApplyTime
// architecture fix.
//
// Root cause: serviceReleaseToTarget used ServiceRelease.Status.LastTransitionUnixMs
// as ApplyTime. When a new node joins and the controller adds it to the
// ServiceRelease.Status.Nodes list, LastTransitionUnixMs bumps for ALL nodes —
// including existing healthy nodes. This caused process_start_time < new_ApplyTime
// → bootstrap_ordering_skew → health FAIL on nodes that were perfectly healthy.
//
// Fix: resolvePerNodeInstallInfo in verification.go looks up
// InstalledPackage.InstalledUnix for the specific (nodeID, service) pair.
// That timestamp is stable — it only changes when that specific node installs
// a new version.
//
// These tests exercise VerifyTarget directly with crafted Target.ApplyTime and
// Target.ApplyTimeSource values (no etcd, no mock needed — the verifier is pure).

import (
	"testing"
	"time"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// T1 is the reference "installed" time. T2 = T1 + 10 minutes simulates a
// release-level LastTransitionUnixMs bump caused by a new node joining.
var (
	applyTimeT1 = time.Unix(1700000000, 0)
	applyTimeT2 = applyTimeT1.Add(10 * time.Minute)
)

// makeProofStartedAt returns a minimal ServiceRuntimeProof for a Globular-managed
// binary (path inside /usr/lib/globular/bin/) with the process started at startTime.
func makeProofStartedAt(svc, hash string, startTime time.Time) *node_agentpb.ServiceRuntimeProof {
	return &node_agentpb.ServiceRuntimeProof{
		ServiceName:        svc,
		InstalledPath:      "/usr/lib/globular/bin/" + svc,
		InstalledSha256:    hash,
		RunningExePath:     "/usr/lib/globular/bin/" + svc,
		RunningExeSha256:   hash,
		RunningPid:         1234,
		SystemdActiveState: "active",
		SystemdSubState:    "running",
		ProcessStartTime:   timestamppb.New(startTime),
	}
}

// TestApplyTime_NodeJoin_NoStaleProcessOnExistingNode is the primary regression
// for the node-join false-positive.
//
// Scenario: ryzen had mcp installed at T1. Process started at T1+2s (normal boot).
// A new node joins at T2; this bumps LastTransitionUnixMs on the ServiceRelease
// to T2. Old code: ApplyTime=T2 → process_start_time(T1+2s) < T2-30s → FAIL.
// New code: ApplyTime=T1 (from InstalledPackage.InstalledUnix) → T1+2s > T1-30s → no finding.
func TestApplyTime_NodeJoin_NoStaleProcessOnExistingNode(t *testing.T) {
	processStart := applyTimeT1.Add(2 * time.Second)
	tgt := Target{
		Service:                   "mcp",
		NodeID:                    "ryzen",
		DesiredVersion:            "1.2.0",
		DesiredBuildID:            "build-uuid-mcp-v1",
		DesiredEntrypointChecksum: hashA,
		RuntimeNeeded:             true,
		ApplyTime:                 applyTimeT1, // per-node installed_unix, NOT the bumped release timestamp
		ApplyTimeSource:           "installed_package.installed_unix",
		IsFirstInstall:            false,
	}
	ev := Evidence{
		Proof: makeProofStartedAt("mcp", hashA, processStart),
	}
	now := applyTimeT2.Add(time.Minute) // evaluate after the join
	v := VerifyTarget(tgt, ev, now)

	for _, f := range v.Findings {
		if f.ID == FindingBootstrapOrderingSkew || f.ID == FindingOldPidAfterUpgrade {
			t.Errorf("unexpected stale-process finding %q (severity=%s) — node-join must not trigger stale-process on existing healthy node", f.ID, f.Severity)
		}
	}
	if v.ProofStatus == ProofMismatch {
		t.Errorf("ProofStatus=mismatch; want runtime_verified or better (findings=%+v)", v.Findings)
	}
}

// TestApplyTime_RealUpgrade_OldProcessDetected verifies that a genuine upgrade
// that left an old PID running is caught at critical severity.
//
// Scenario: Process started at T1 (old version). A real upgrade happened at T2
// (InstalledUnix=T2, IsFirstInstall=false). Process was never restarted.
func TestApplyTime_RealUpgrade_OldProcessDetected(t *testing.T) {
	processStart := applyTimeT1 // old process, predates the upgrade
	tgt := Target{
		Service:                   "mcp",
		NodeID:                    "ryzen",
		DesiredVersion:            "1.3.0",
		DesiredBuildID:            "build-uuid-mcp-v2",
		DesiredEntrypointChecksum: hashA,
		RuntimeNeeded:             true,
		ApplyTime:                 applyTimeT2,                        // new InstalledUnix after upgrade
		ApplyTimeSource:           "installed_package.installed_unix", // from new install record
		IsFirstInstall:            false,                              // this is an upgrade
	}
	ev := Evidence{
		Proof: makeProofStartedAt("mcp", hashA, processStart),
	}
	now := applyTimeT2.Add(5 * time.Minute)
	v := VerifyTarget(tgt, ev, now)

	found := false
	for _, f := range v.Findings {
		if f.ID == FindingOldPidAfterUpgrade && f.Severity == SeverityCritical {
			found = true
			// Verify apply_time_source appears in evidence.
			if src := f.Evidence["apply_time_source"]; src != "installed_package.installed_unix" {
				t.Errorf("apply_time_source in evidence = %q; want installed_package.installed_unix", src)
			}
		}
	}
	if !found {
		t.Errorf("expected %s at critical severity; findings=%+v", FindingOldPidAfterUpgrade, v.Findings)
	}
}

// TestApplyTime_ReleasePatchDoesNotAffectRuntimeIdentity verifies that a release
// metadata update (e.g. a label change with the same binary) at T2 does not cause
// the verifier to flag an existing healthy process that started at T1+1s.
//
// The key: ApplyTime must come from InstalledUnix (T1), not from the release
// transition time (T2). The verifier receives the already-resolved T1 and must
// not fire any stale-process finding.
func TestApplyTime_ReleasePatchDoesNotAffectRuntimeIdentity(t *testing.T) {
	// The binary and version are unchanged; only a metadata field bumped
	// LastTransitionUnixMs in the release. The collector resolves T1 from
	// InstalledUnix and the verifier sees T1 as ApplyTime.
	processStart := applyTimeT1.Add(time.Second)
	tgt := Target{
		Service:                   "auth",
		NodeID:                    "ryzen",
		DesiredVersion:            "1.1.0",
		DesiredBuildID:            "build-uuid-auth-v1",
		DesiredEntrypointChecksum: hashA,
		RuntimeNeeded:             true,
		ApplyTime:                 applyTimeT1, // from InstalledUnix, not T2 (release transition ignored)
		ApplyTimeSource:           "installed_package.installed_unix",
		IsFirstInstall:            false,
	}
	ev := Evidence{
		Proof: makeProofStartedAt("auth", hashA, processStart),
	}
	now := applyTimeT2.Add(2 * time.Minute)
	v := VerifyTarget(tgt, ev, now)

	for _, f := range v.Findings {
		if f.ID == FindingBootstrapOrderingSkew || f.ID == FindingOldPidAfterUpgrade {
			t.Errorf("release metadata patch must not flag existing healthy process: got finding %q", f.ID)
		}
	}
}

// TestApplyTime_FallbackVisible_NoFalsePositiveWhenProcessFresh verifies that
// when the installed-state is missing and we fall back to the release-level
// timestamp, a fresh process (started after ApplyTime) does not trigger any
// stale-process finding. Also confirms apply_time_source appears in evidence
// if a stale-process finding does fire.
func TestApplyTime_FallbackVisible_NoFalsePositiveWhenProcessFresh(t *testing.T) {
	// Process started after the fallback ApplyTime — should be clean.
	processStart := applyTimeT1.Add(5 * time.Second)
	tgt := Target{
		Service:                   "dns",
		NodeID:                    "nuc",
		DesiredVersion:            "1.0.0",
		DesiredBuildID:            "build-uuid-dns-v1",
		DesiredEntrypointChecksum: hashA,
		RuntimeNeeded:             true,
		ApplyTime:                 applyTimeT1,
		ApplyTimeSource:           "release.last_transition_fallback",
		IsFirstInstall:            false,
	}
	ev := Evidence{
		Proof: makeProofStartedAt("dns", hashA, processStart),
	}
	now := applyTimeT1.Add(10 * time.Minute)
	v := VerifyTarget(tgt, ev, now)

	for _, f := range v.Findings {
		if f.ID == FindingBootstrapOrderingSkew || f.ID == FindingOldPidAfterUpgrade {
			t.Errorf("fresh process with fallback ApplyTime must not trigger stale-process finding: got %q", f.ID)
		}
	}
}

// TestApplyTime_InstalledUnixPreservedOnMetadataUpdate verifies the verifier's
// own logic: when ApplyTime=T1 (from InstalledUnix) and a process starts at
// T1+1s, no stale-process finding fires — even if UpdatedUnix would be T2.
// The collector already resolved T1 and the verifier must use it faithfully.
func TestApplyTime_InstalledUnixPreservedOnMetadataUpdate(t *testing.T) {
	processStart := applyTimeT1.Add(time.Second)
	tgt := Target{
		Service:                   "rbac",
		NodeID:                    "ryzen",
		DesiredVersion:            "1.0.5",
		DesiredBuildID:            "build-uuid-rbac-v1",
		DesiredEntrypointChecksum: hashA,
		RuntimeNeeded:             true,
		ApplyTime:                 applyTimeT1, // InstalledUnix; UpdatedUnix=T2 is NOT passed to verifier
		ApplyTimeSource:           "installed_package.installed_unix",
		IsFirstInstall:            false,
	}
	ev := Evidence{
		Proof: makeProofStartedAt("rbac", hashA, processStart),
	}
	now := applyTimeT2.Add(time.Minute)
	v := VerifyTarget(tgt, ev, now)

	for _, f := range v.Findings {
		if f.ID == FindingBootstrapOrderingSkew || f.ID == FindingOldPidAfterUpgrade {
			t.Errorf("verifier must use InstalledUnix (T1) not UpdatedUnix (T2): got finding %q", f.ID)
		}
	}
	if v.ProofStatus == ProofMismatch {
		t.Errorf("ProofStatus=mismatch; want runtime_verified (findings=%+v)", v.Findings)
	}
}

// TestApplyTime_RealIdentityChange_OldPidCaught verifies that when a real
// identity change bumps InstalledUnix to T2, an old PID (started at T1) is
// correctly caught at critical severity.
func TestApplyTime_RealIdentityChange_OldPidCaught(t *testing.T) {
	processStart := applyTimeT1 // old process never restarted after upgrade
	tgt := Target{
		Service:                   "workflow",
		NodeID:                    "ryzen",
		DesiredVersion:            "2.0.0",
		DesiredBuildID:            "build-uuid-workflow-v2",
		DesiredEntrypointChecksum: hashA,
		RuntimeNeeded:             true,
		ApplyTime:                 applyTimeT2, // new InstalledUnix after real upgrade
		ApplyTimeSource:           "installed_package.installed_unix",
		IsFirstInstall:            false,
	}
	ev := Evidence{
		Proof: makeProofStartedAt("workflow", hashA, processStart),
	}
	now := applyTimeT2.Add(5 * time.Minute)
	v := VerifyTarget(tgt, ev, now)

	found := false
	for _, f := range v.Findings {
		if f.ID == FindingOldPidAfterUpgrade && f.Severity == SeverityCritical {
			found = true
			if src := f.Evidence["apply_time_source"]; src != "installed_package.installed_unix" {
				t.Errorf("apply_time_source = %q; want installed_package.installed_unix", src)
			}
		}
	}
	if !found {
		t.Errorf("expected %s at critical severity for real upgrade with old PID; findings=%+v",
			FindingOldPidAfterUpgrade, v.Findings)
	}
}
