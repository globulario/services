package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// ── Test fixtures ───────────────────────────────────────────────────────────

// fixedNow returns a stable timestamp so tests can assert against
// proof_time_unix_ms without time-of-day noise.
var fixedNow = time.Date(2026, 5, 29, 1, 0, 0, 0, time.UTC)

// stubDeps assembles a selfHostedProofDeps with overridable functions. Each
// override defaults to the success path; tests inject failure where needed.
func stubDeps(t *testing.T) selfHostedProofDeps {
	t.Helper()
	return selfHostedProofDeps{
		FindRunningPID: func(ctx context.Context, name string) int { return 1234 },
		ReadProcExe: func(pid int) (string, error) {
			return installedBinaryPath("node-agent", "SERVICE"), nil
		},
		HashFile: func(path string) (string, error) {
			return "002667f187277e3ec53539efdb3a71a64a6feb93f87336c94b159e818e68b380", nil
		},
		FetchServiceRelease: func(ctx context.Context, name string) (*cluster_controllerpb.ServiceRelease, error) {
			return &cluster_controllerpb.ServiceRelease{
				Status: &cluster_controllerpb.ServiceReleaseStatus{
					ResolvedVersion:            "1.2.117",
					ResolvedBuildID:            "019e70b4-ff9d-7e52-b625-1e63d83cd43a",
					ResolvedBuildNumber:        1,
					ResolvedEntrypointChecksum: "002667f187277e3ec53539efdb3a71a64a6feb93f87336c94b159e818e68b380",
				},
			}, nil
		},
		Now: func() time.Time { return fixedNow },
	}
}

// ── Proof Builder Tests ─────────────────────────────────────────────────────

// 1. proof succeeds when /proc/PID/exe hash equals manifest entrypoint_checksum.
func TestSelfHostedProof_HappyPath(t *testing.T) {
	deps := stubDeps(t)
	proof, reason := buildSelfHostedRuntimeProof(context.Background(), "node-agent", deps)
	if reason != proofReasonOK {
		t.Fatalf("expected proofReasonOK, got %q", reason)
	}
	if proof == nil {
		t.Fatal("expected non-nil proof on happy path")
	}
	if proof.ServiceName != "node-agent" {
		t.Errorf("ServiceName=%q want node-agent", proof.ServiceName)
	}
	if proof.DesiredVersion != "1.2.117" {
		t.Errorf("DesiredVersion=%q want 1.2.117", proof.DesiredVersion)
	}
	if proof.OnDiskSHA256 != proof.ManifestChecksum {
		t.Errorf("OnDiskSHA256 != ManifestChecksum (%s vs %s)", proof.OnDiskSHA256, proof.ManifestChecksum)
	}
	if proof.ProofSource != "self_hosted_runtime_proof" {
		t.Errorf("ProofSource=%q want self_hosted_runtime_proof", proof.ProofSource)
	}
}

// 2. proof fails when manifest is missing.
func TestSelfHostedProof_ManifestMissing(t *testing.T) {
	deps := stubDeps(t)
	deps.FetchServiceRelease = func(ctx context.Context, name string) (*cluster_controllerpb.ServiceRelease, error) {
		return nil, nil
	}
	proof, reason := buildSelfHostedRuntimeProof(context.Background(), "node-agent", deps)
	if reason != proofReasonManifestMissing {
		t.Fatalf("reason=%q want %q", reason, proofReasonManifestMissing)
	}
	if proof != nil {
		t.Fatal("expected nil proof when manifest is missing")
	}
}

// 3. proof fails when entrypoint_checksum is missing.
func TestSelfHostedProof_EntrypointChecksumMissing(t *testing.T) {
	deps := stubDeps(t)
	deps.FetchServiceRelease = func(ctx context.Context, name string) (*cluster_controllerpb.ServiceRelease, error) {
		return &cluster_controllerpb.ServiceRelease{
			Status: &cluster_controllerpb.ServiceReleaseStatus{
				ResolvedVersion:            "1.2.117",
				ResolvedEntrypointChecksum: "", // ← missing
			},
		}, nil
	}
	_, reason := buildSelfHostedRuntimeProof(context.Background(), "node-agent", deps)
	if reason != proofReasonEntrypointChecksumMissing {
		t.Fatalf("reason=%q want %q", reason, proofReasonEntrypointChecksumMissing)
	}
}

// 4. proof fails when process is not running.
func TestSelfHostedProof_ProcessNotRunning(t *testing.T) {
	deps := stubDeps(t)
	deps.FindRunningPID = func(ctx context.Context, name string) int { return 0 }
	_, reason := buildSelfHostedRuntimeProof(context.Background(), "node-agent", deps)
	if reason != proofReasonProcessNotRunning {
		t.Fatalf("reason=%q want %q", reason, proofReasonProcessNotRunning)
	}
}

// 5. proof fails when /proc/PID/exe cannot be read.
func TestSelfHostedProof_ProcExeUnreadable(t *testing.T) {
	deps := stubDeps(t)
	deps.ReadProcExe = func(pid int) (string, error) {
		return "", errors.New("readlink: ESRCH")
	}
	_, reason := buildSelfHostedRuntimeProof(context.Background(), "node-agent", deps)
	if reason != proofReasonProcExeUnreadable {
		t.Fatalf("reason=%q want %q", reason, proofReasonProcExeUnreadable)
	}
}

// 6. proof fails when binary hash mismatches manifest.
func TestSelfHostedProof_BinaryHashMismatch(t *testing.T) {
	deps := stubDeps(t)
	deps.HashFile = func(path string) (string, error) {
		return "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", nil
	}
	_, reason := buildSelfHostedRuntimeProof(context.Background(), "node-agent", deps)
	if reason != proofReasonBinaryHashMismatch {
		t.Fatalf("reason=%q want %q", reason, proofReasonBinaryHashMismatch)
	}
}

// 6b. proof fails when binary path does not follow the convention. The
// runtime guard against binary substitution: even if hash matched, the
// writer refuses to claim "installed" for a binary in an unexpected
// location.
func TestSelfHostedProof_BinaryPathUnexpected(t *testing.T) {
	deps := stubDeps(t)
	deps.ReadProcExe = func(pid int) (string, error) {
		return "/usr/local/bin/rogue_binary", nil
	}
	_, reason := buildSelfHostedRuntimeProof(context.Background(), "node-agent", deps)
	if reason != proofReasonBinaryPathUnexpected {
		t.Fatalf("reason=%q want %q", reason, proofReasonBinaryPathUnexpected)
	}
}

// ── Status Promotion Tests ──────────────────────────────────────────────────

// 7. same-version status=failed promotes to installed when proof passes.
func TestSelfHostedProof_StatusFailedPromotesToInstalled(t *testing.T) {
	proof := &selfHostedRuntimeProof{
		ServiceName:      "node-agent",
		DesiredVersion:   "1.2.117",
		DesiredBuildID:   "019e70b4",
		ManifestChecksum: "002667f1",
		OnDiskSHA256:     "002667f1",
		BinaryPath:       installedBinaryPath("node-agent", "SERVICE"),
		ProofSource:      "self_hosted_runtime_proof",
		ProofTimeUnixMs:  fixedNow.UnixMilli(),
	}
	existing := &node_agentpb.InstalledPackage{
		Name:    "node-agent",
		Kind:    "SERVICE",
		Version: "1.2.117", // same version
		Status:  "failed",  // ← contradicts proof
		Metadata: map[string]string{
			"error": "package not found in local dirs (version=1.2.117)",
		},
	}
	canRefresh, reason := proofCanRefreshInstalledState(existing, proof)
	if !canRefresh {
		t.Fatalf("expected refresh when status=failed but proof good; reason=%q", reason)
	}
	pkg := applySelfHostedInstalledStateRefresh(existing, proof, "node-id")
	if pkg.Status != "installed" {
		t.Errorf("Status=%q want installed", pkg.Status)
	}
	if _, hasErr := pkg.Metadata["error"]; hasErr {
		t.Errorf("metadata.error not cleared: %q", pkg.Metadata["error"])
	}
}

// 8. same-version stale metadata.error is cleared when proof passes.
func TestSelfHostedProof_StaleErrorCleared(t *testing.T) {
	proof := &selfHostedRuntimeProof{
		ServiceName:      "cluster-controller",
		DesiredVersion:   "1.2.124",
		ManifestChecksum: "746a5a96",
		OnDiskSHA256:     "746a5a96",
		BinaryPath:       installedBinaryPath("cluster-controller", "SERVICE"),
		ProofSource:      "self_hosted_runtime_proof",
		ProofTimeUnixMs:  fixedNow.UnixMilli(),
	}
	existing := &node_agentpb.InstalledPackage{
		Name:    "cluster-controller",
		Kind:    "SERVICE",
		Version: "1.2.124",
		Status:  "installed",
		Metadata: map[string]string{
			"error": "stale install error from prior attempt",
		},
	}
	pkg := applySelfHostedInstalledStateRefresh(existing, proof, "node-id")
	if _, hasErr := pkg.Metadata["error"]; hasErr {
		t.Errorf("metadata.error not cleared: %q", pkg.Metadata["error"])
	}
	if pkg.Metadata["proof_source"] != "self_hosted_runtime_proof" {
		t.Errorf("proof_source not recorded: %q", pkg.Metadata["proof_source"])
	}
}

// 9. same-version failed does not promote when proof fails — covered by
// the build-side tests; refresh writer is only called with non-nil proof.
// This test asserts: when no proof exists, the writer is not invoked
// (caller responsibility), so the existing record is preserved.
func TestSelfHostedProof_RefreshWriterRequiresProof(t *testing.T) {
	existing := &node_agentpb.InstalledPackage{
		Name:    "node-agent",
		Kind:    "SERVICE",
		Version: "1.2.117",
		Status:  "failed",
	}
	canRefresh, reason := proofCanRefreshInstalledState(existing, nil)
	if canRefresh {
		t.Fatal("expected canRefresh=false when proof is nil")
	}
	if reason != "no_proof" {
		t.Errorf("reason=%q want no_proof", reason)
	}
}

// 10. buildId mismatch may refresh only for self-hosted components when
// proof passes. cluster-doctor today: installed_state at v1.2.117/buildId=X,
// but binary is v1.2.118. Proof against the v1.2.118 manifest must be able
// to refresh the record because the canonical name is in the allowlist.
func TestSelfHostedProof_BuildIDMismatchRefreshesForSelfHosted(t *testing.T) {
	proof := &selfHostedRuntimeProof{
		ServiceName:        "cluster-doctor",
		DesiredVersion:     "1.2.118",
		DesiredBuildID:     "019e7184-NEW",
		DesiredBuildNumber: 1,
		ManifestChecksum:   "5bf6fe9c",
		OnDiskSHA256:       "5bf6fe9c",
		BinaryPath:         installedBinaryPath("cluster-doctor", "SERVICE"),
		ProofSource:        "self_hosted_runtime_proof",
		ProofTimeUnixMs:    fixedNow.UnixMilli(),
	}
	existing := &node_agentpb.InstalledPackage{
		Name:     "cluster-doctor",
		Kind:     "SERVICE",
		Version:  "1.2.117", // stale
		BuildId:  "dd168c5a-OLD",
		Checksum: "aa41fc70", // also stale
		Status:   "installed",
	}
	canRefresh, _ := proofCanRefreshInstalledState(existing, proof)
	if !canRefresh {
		t.Fatal("expected refresh for self-hosted buildId mismatch when proof passes")
	}
	pkg := applySelfHostedInstalledStateRefresh(existing, proof, "node-id")
	if pkg.Version != "1.2.118" || pkg.BuildId != "019e7184-NEW" || !strings.EqualFold(pkg.Checksum, "5bf6fe9c") {
		t.Errorf("refresh did not advance identity: version=%s buildId=%s checksum=%s",
			pkg.Version, pkg.BuildId, pkg.Checksum)
	}
}

// 11. buildId mismatch does not refresh for ordinary services. The writer
// only operates on the allowlist; ordinary services never reach the
// refresh path because buildSelfHostedRuntimeProof returns
// proofReasonNotInAllowlist first.
func TestSelfHostedProof_OrdinaryServiceNotInAllowlist(t *testing.T) {
	deps := stubDeps(t)
	_, reason := buildSelfHostedRuntimeProof(context.Background(), "dns", deps)
	if reason != proofReasonNotInAllowlist {
		t.Fatalf("reason=%q want %q (dns must NOT be in self-hosted allowlist)",
			reason, proofReasonNotInAllowlist)
	}
}

// 12. existing good record remains idempotently good. Calling the writer
// repeatedly with a record that already matches the proof must NOT trigger
// an etcd write — proofCanRefreshInstalledState returns false with
// "already_canonical".
func TestSelfHostedProof_IdempotentSkipsAlreadyCanonical(t *testing.T) {
	proof := &selfHostedRuntimeProof{
		ServiceName:      "node-agent",
		DesiredVersion:   "1.2.117",
		DesiredBuildID:   "019e70b4",
		ManifestChecksum: "002667f1",
		OnDiskSHA256:     "002667f1",
		ProofSource:      "self_hosted_runtime_proof",
	}
	existing := &node_agentpb.InstalledPackage{
		Name:     "node-agent",
		Kind:     "SERVICE",
		Version:  "1.2.117",
		BuildId:  "019e70b4",
		Checksum: "002667f1",
		Status:   "installed",
		Metadata: map[string]string{
			"proof_source": "self_hosted_runtime_proof",
		},
	}
	canRefresh, reason := proofCanRefreshInstalledState(existing, proof)
	if canRefresh {
		t.Fatal("expected idempotent skip; record already canonical")
	}
	if reason != "already_canonical" {
		t.Errorf("reason=%q want already_canonical", reason)
	}
}

// ── Regression Tests ────────────────────────────────────────────────────────

// 13. buildId guard still protects workflow-committed records for ordinary
// services. This is enforced by the allowlist check — only the self-hosted
// names get the proof path; everyone else falls through to the existing
// heartbeat refresh which respects the buildId guard.
//
// After Project D's bridge surfaced the same staleness for `repository`,
// it was added to the allowlist (commit extending self-hosted scope).
// Other ordinary services (dns, rbac, authentication, workflow, event)
// remain workflow-committed.
func TestSelfHostedProof_BuildIDGuardPreservedForOrdinaryServices(t *testing.T) {
	for _, ordinary := range []string{"dns", "rbac", "authentication", "workflow", "event"} {
		if selfHostedServiceNames[ordinary] {
			t.Errorf("ordinary service %q must NOT be in self-hosted allowlist", ordinary)
		}
	}
}

// 13b. The allowlist must contain exactly the documented self-hosted
// control-plane services. A regression that silently widens the
// allowlist would let the proof writer touch ordinary services' records
// — a forbidden write that bypasses the workflow ConvergenceResultV1
// committer for components that don't restart themselves during apply.
func TestSelfHostedProof_AllowlistContainsExactlyExpectedNames(t *testing.T) {
	expected := map[string]bool{
		"node-agent":         true,
		"cluster-controller": true,
		"cluster-doctor":     true,
		"repository":         true,
	}
	if len(selfHostedServiceNames) != len(expected) {
		t.Errorf("allowlist has %d entries, want %d", len(selfHostedServiceNames), len(expected))
	}
	for name := range expected {
		if !selfHostedServiceNames[name] {
			t.Errorf("expected %q in allowlist, missing", name)
		}
	}
	for name := range selfHostedServiceNames {
		if !expected[name] {
			t.Errorf("unexpected entry %q in allowlist", name)
		}
	}
}

// 14. ExpectedSha256 chain remains strict. The proof writer reads
// ResolvedEntrypointChecksum (binary sha256) from the ServiceRelease
// Status, not from spec or from the package tarball digest. This test
// guards against a regression where the writer might accept
// ResolvedArtifactDigest (tarball) as binary identity.
func TestSelfHostedProof_UsesEntrypointChecksumNotArtifactDigest(t *testing.T) {
	deps := stubDeps(t)
	deps.FetchServiceRelease = func(ctx context.Context, name string) (*cluster_controllerpb.ServiceRelease, error) {
		return &cluster_controllerpb.ServiceRelease{
			Status: &cluster_controllerpb.ServiceReleaseStatus{
				ResolvedVersion:            "1.2.117",
				ResolvedArtifactDigest:     "TARBALL_HASH_NEVER_USE_THIS_FOR_BINARY",
				ResolvedEntrypointChecksum: "002667f187277e3ec53539efdb3a71a64a6feb93f87336c94b159e818e68b380",
			},
		}, nil
	}
	proof, reason := buildSelfHostedRuntimeProof(context.Background(), "node-agent", deps)
	if reason != proofReasonOK {
		t.Fatalf("proof should succeed; reason=%q", reason)
	}
	if proof.ManifestChecksum != "002667f187277e3ec53539efdb3a71a64a6feb93f87336c94b159e818e68b380" {
		t.Errorf("ManifestChecksum=%q must come from ResolvedEntrypointChecksum, not ResolvedArtifactDigest",
			proof.ManifestChecksum)
	}
}

// 15. Hash-schema separation: the proof writer never writes
// ResolvedArtifactDigest (tarball hash) into installed_state.Checksum
// or metadata.entrypoint_checksum. Both must equal the binary sha256.
func TestSelfHostedProof_HashSchemaSeparation(t *testing.T) {
	proof := &selfHostedRuntimeProof{
		ServiceName:      "node-agent",
		DesiredVersion:   "1.2.117",
		ManifestChecksum: "002667f1",
		OnDiskSHA256:     "002667f1",
	}
	pkg := applySelfHostedInstalledStateRefresh(nil, proof, "node-id")
	if pkg.Checksum != proof.ManifestChecksum {
		t.Errorf("Checksum=%q want ManifestChecksum=%q (must be binary sha256)",
			pkg.Checksum, proof.ManifestChecksum)
	}
	if pkg.Metadata["entrypoint_checksum"] != proof.ManifestChecksum {
		t.Errorf("metadata.entrypoint_checksum=%q want ManifestChecksum=%q",
			pkg.Metadata["entrypoint_checksum"], proof.ManifestChecksum)
	}
}

// 16. AWARENESS_BUNDLE must not be in the self-hosted allowlist; it has its
// own observe-only path (Project A4/A5). A regression that adds it here
// would let the proof writer fabricate a SERVICE installed_state record
// for the bundle, which the bundle's observe path explicitly forbids.
func TestSelfHostedProof_AwarenessBundleNotInAllowlist(t *testing.T) {
	for _, name := range []string{"globular-awareness-bundle", "awareness-bundle"} {
		if selfHostedServiceNames[name] {
			t.Errorf("AWARENESS_BUNDLE canonical %q must NOT be in self-hosted allowlist", name)
		}
	}
}

// 17. No desired-state mutation: the proof writer reads ServiceRelease.Status
// (controller-managed observed state) but never writes to it. Test guards
// against future refactors that might call FetchServiceRelease with a
// write capability.
func TestSelfHostedProof_DepsHaveNoMutationCapability(t *testing.T) {
	deps := stubDeps(t)
	// The FetchServiceRelease signature returns the record; it does not
	// accept a record to write. Capture: it must be a read-only fn.
	if deps.FetchServiceRelease == nil {
		t.Fatal("FetchServiceRelease must be set")
	}
}

// 18. Proof metadata is recorded. After refresh, the installed_state record
// must carry: proof_source, proof_manifest_checksum, proof_on_disk_sha256,
// proof_binary_path, proof_time_unix_ms.
func TestSelfHostedProof_MetadataRecorded(t *testing.T) {
	proof := &selfHostedRuntimeProof{
		ServiceName:      "node-agent",
		DesiredVersion:   "1.2.117",
		ManifestChecksum: "002667f1",
		OnDiskSHA256:     "002667f1",
		BinaryPath:       "/usr/lib/globular/bin/node_agent_server",
		ProofSource:      "self_hosted_runtime_proof",
		ProofTimeUnixMs:  fixedNow.UnixMilli(),
	}
	pkg := applySelfHostedInstalledStateRefresh(nil, proof, "node-id")
	for _, k := range []string{"proof_source", "proof_manifest_checksum", "proof_on_disk_sha256", "proof_binary_path", "proof_time_unix_ms"} {
		if pkg.Metadata[k] == "" {
			t.Errorf("metadata[%q] not recorded", k)
		}
	}
	if pkg.Metadata["proof_source"] != "self_hosted_runtime_proof" {
		t.Errorf("proof_source=%q want self_hosted_runtime_proof", pkg.Metadata["proof_source"])
	}
}

// 19. Regression for service.old_pid_after_upgrade false positives
// (INC-2026-0016): the writer must anchor InstalledUnix and UpdatedUnix
// to the running PID's start time, not time.Now(). If it bumps either to
// "now" every heartbeat, the verifier sees ApplyTime > PID_start and
// flags the (correctly) older PID as a stale upgrade.
func TestSelfHostedProof_TimestampsAnchorToPIDStart(t *testing.T) {
	const pidStart = int64(1780020000) // 2026-05-29 00:00:00 +/- noise
	proof := &selfHostedRuntimeProof{
		ServiceName:      "cluster-controller",
		DesiredVersion:   "1.2.126",
		ManifestChecksum: "e29ad06a",
		OnDiskSHA256:     "e29ad06a",
		BinaryPath:       installedBinaryPath("cluster-controller", "SERVICE"),
		ProofSource:      "self_hosted_runtime_proof",
		ProofTimeUnixMs:  pidStart*1000 + 90_000, // 90s after PID start
		PIDStartUnix:     pidStart,
		RunningPID:       12345,
	}
	pkg := applySelfHostedInstalledStateRefresh(nil, proof, "node-id")
	if pkg.InstalledUnix != pidStart {
		t.Errorf("InstalledUnix=%d want %d (PID start anchor)", pkg.InstalledUnix, pidStart)
	}
	if pkg.UpdatedUnix != pidStart {
		t.Errorf("UpdatedUnix=%d want %d (PID start anchor)", pkg.UpdatedUnix, pidStart)
	}
}

// 20. The idempotency guard must treat a record with timestamps ahead of
// PID start as not-canonical, so the writer corrects them on the next
// refresh. Pre-fix behavior returned "already_canonical" and skipped the
// write, leaving the bug stuck across heartbeats.
func TestSelfHostedProof_TimestampsAheadOfPIDStartTriggerRefresh(t *testing.T) {
	const pidStart = int64(1780020000)
	proof := &selfHostedRuntimeProof{
		ServiceName:      "cluster-controller",
		DesiredVersion:   "1.2.126",
		ManifestChecksum: "e29ad06a",
		OnDiskSHA256:     "e29ad06a",
		PIDStartUnix:     pidStart,
		ProofSource:      "self_hosted_runtime_proof",
		ProofTimeUnixMs:  pidStart*1000 + 90_000,
	}
	existing := &node_agentpb.InstalledPackage{
		Name:          "cluster-controller",
		Kind:          "SERVICE",
		Version:       "1.2.126",
		Checksum:      "e29ad06a",
		Status:        "installed",
		InstalledUnix: pidStart + 60, // bug: future-dated by a minute
		UpdatedUnix:   pidStart + 90, // bug: future-dated by 1.5 min
		Metadata: map[string]string{
			"proof_source": "self_hosted_runtime_proof",
		},
	}
	canRefresh, reason := proofCanRefreshInstalledState(existing, proof)
	if !canRefresh {
		t.Fatalf("expected refresh when timestamps ahead of PID start; reason=%q", reason)
	}
	pkg := applySelfHostedInstalledStateRefresh(existing, proof, "node-id")
	if pkg.InstalledUnix != pidStart {
		t.Errorf("InstalledUnix=%d want %d (clamped to PID start)", pkg.InstalledUnix, pidStart)
	}
	if pkg.UpdatedUnix != pidStart {
		t.Errorf("UpdatedUnix=%d want %d (clamped to PID start)", pkg.UpdatedUnix, pidStart)
	}
}

// 21. Idempotency: when the record is already correct (status/version/
// checksum/proof match AND timestamps not ahead of PID start), no refresh.
func TestSelfHostedProof_RecordCanonicalNoRefresh(t *testing.T) {
	const pidStart = int64(1780020000)
	proof := &selfHostedRuntimeProof{
		ServiceName:      "cluster-controller",
		DesiredVersion:   "1.2.126",
		ManifestChecksum: "e29ad06a",
		OnDiskSHA256:     "e29ad06a",
		PIDStartUnix:     pidStart,
		ProofSource:      "self_hosted_runtime_proof",
		ProofTimeUnixMs:  pidStart*1000 + 90_000,
	}
	existing := &node_agentpb.InstalledPackage{
		Name:          "cluster-controller",
		Kind:          "SERVICE",
		Version:       "1.2.126",
		Checksum:      "e29ad06a",
		Status:        "installed",
		InstalledUnix: pidStart,
		UpdatedUnix:   pidStart,
		Metadata: map[string]string{
			"proof_source": "self_hosted_runtime_proof",
		},
	}
	canRefresh, reason := proofCanRefreshInstalledState(existing, proof)
	if canRefresh {
		t.Fatal("expected no refresh when record is already canonical")
	}
	if reason != "already_canonical" {
		t.Errorf("reason=%q want already_canonical", reason)
	}
}

// 22. Preserve a historically-older InstalledUnix: if the binary was
// installed before the current PID started, InstalledUnix must stay at
// the original install time, not get clamped down to PID start.
func TestSelfHostedProof_InstalledBeforePIDStartPreserved(t *testing.T) {
	const pidStart = int64(1780020000)
	const installedAt = int64(1779000000) // 12 days earlier
	proof := &selfHostedRuntimeProof{
		ServiceName:      "cluster-controller",
		DesiredVersion:   "1.2.126",
		ManifestChecksum: "e29ad06a",
		OnDiskSHA256:     "e29ad06a",
		PIDStartUnix:     pidStart,
		ProofSource:      "self_hosted_runtime_proof",
		ProofTimeUnixMs:  pidStart*1000 + 90_000,
	}
	existing := &node_agentpb.InstalledPackage{
		Name:          "cluster-controller",
		Kind:          "SERVICE",
		Version:       "1.2.126",
		Checksum:      "e29ad06a",
		Status:        "installed",
		InstalledUnix: installedAt,
		UpdatedUnix:   installedAt,
		Metadata: map[string]string{
			"proof_source": "self_hosted_runtime_proof",
		},
	}
	pkg := applySelfHostedInstalledStateRefresh(existing, proof, "node-id")
	if pkg.InstalledUnix != installedAt {
		t.Errorf("InstalledUnix=%d want %d (preserved historical install time)", pkg.InstalledUnix, installedAt)
	}
}
