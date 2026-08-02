package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/versionutil"
)

// helpers for building test fixtures

func installedPkg(version, buildID string) *node_agentpb.InstalledPackage {
	return &node_agentpb.InstalledPackage{
		Version:  version,
		BuildId:  buildID,
		Metadata: map[string]string{"entrypoint_checksum": "abc123"},
	}
}

func installedPkgNoChecksum(version, buildID string) *node_agentpb.InstalledPackage {
	return &node_agentpb.InstalledPackage{
		Version: version,
		BuildId: buildID,
	}
}

func alwaysActive(_ context.Context, _ string) (bool, error)   { return true, nil }
func alwaysInactive(_ context.Context, _ string) (bool, error) { return false, nil }
func alwaysLoaded(_ context.Context, _ string) (bool, error)   { return true, nil }
func alwaysUnloaded(_ context.Context, _ string) (bool, error) { return false, nil }

func stubEntrypoint(t *testing.T, name, entrypoint string) {
	t.Helper()
	oldBase := versionutil.BaseDir()
	versionutil.SetBaseDir(t.TempDir())
	t.Cleanup(func() { versionutil.SetBaseDir(oldBase) })
	if err := versionutil.WriteEntrypoint(name, entrypoint); err != nil {
		t.Fatalf("write entrypoint sidecar: %v", err)
	}
}

// stubServicePolicyDir overrides the RBAC policy-marker probe for the duration
// of a test, restoring it on cleanup. A converged service has its policy dir;
// an out-of-band Day-0 install does not.
func stubServicePolicyDir(t *testing.T, present bool) {
	t.Helper()
	prev := servicePolicyDirPresentFunc
	servicePolicyDirPresentFunc = func(string) bool { return present }
	t.Cleanup(func() { servicePolicyDirPresentFunc = prev })
}

// TestInstallPackageSkipsOnlyWhenRuntimeActive — happy path: version matches,
// unit is active, entrypoint_checksum present, RBAC policy deployed → skip is
// allowed.
func TestInstallPackageSkipsOnlyWhenRuntimeActive(t *testing.T) {
	stubServicePolicyDir(t, true)
	result, reason := canSkipInstallPackage(
		context.Background(),
		"myservice", "SERVICE", "1.2.3", "", "", "",
		installedPkg("1.2.3", ""),
		alwaysActive,
		alwaysLoaded,
	)
	if result != installSkipAllowed {
		t.Fatalf("expected installSkipAllowed, got %d (%s)", result, reason)
	}
}

// TestInstallPackageDoesNotSkipWhenPolicyDirAbsent — a SERVICE whose RBAC policy
// directory was never deployed (installed out-of-band by the Day-0 installer;
// install_payload never ran) must NOT be skipped even when version, build_id,
// checksum, and an active unit all match. Skipping would leave the authz
// resolver with zero permission mappings and deny every role-based RPC (the
// v1.2.267 empty-resolver incident: repository GetRepositoryStatus /
// ListRepositoryFindings PermissionDenied, degrading cluster-doctor).
func TestInstallPackageDoesNotSkipWhenPolicyDirAbsent(t *testing.T) {
	stubServicePolicyDir(t, false)
	result, reason := canSkipInstallPackage(
		context.Background(),
		"myservice", "SERVICE", "1.2.3", "", "", "",
		installedPkg("1.2.3", ""),
		alwaysActive,
		alwaysLoaded,
	)
	if result != installSkipDeniedVersion {
		t.Fatalf("expected installSkipDeniedVersion when policy dir absent, got %d (%s)", result, reason)
	}
}

// TestInstallPackageDoesNotSkipWhenChecksumMissing — unit is active and version
// matches but entrypoint_checksum absent → must re-apply to register proof.
func TestInstallPackageDoesNotSkipWhenChecksumMissing(t *testing.T) {
	result, reason := canSkipInstallPackage(
		context.Background(),
		"myservice", "SERVICE", "1.2.3", "", "", "",
		installedPkgNoChecksum("1.2.3", ""),
		alwaysActive,
		alwaysLoaded,
	)
	if result != installSkipDeniedVersion {
		t.Fatalf("expected installSkipDeniedVersion, got %d (%s)", result, reason)
	}
}

// TestInstallPackageDoesNotSkipWhenUnitInactive — version matches but unit is
// loaded+inactive → must repair, not skip.
func TestInstallPackageDoesNotSkipWhenUnitInactive(t *testing.T) {
	result, reason := canSkipInstallPackage(
		context.Background(),
		"myservice", "SERVICE", "1.2.3", "", "", "",
		installedPkg("1.2.3", ""),
		alwaysInactive,
		alwaysLoaded,
	)
	if result != installSkipDeniedInactive {
		t.Fatalf("expected installSkipDeniedInactive, got %d (%s)", result, reason)
	}
}

// TestInstallPackageDoesNotSkipWhenUnitMissing — version matches but unit file
// is gone → must reinstall.
func TestInstallPackageDoesNotSkipWhenUnitMissing(t *testing.T) {
	result, reason := canSkipInstallPackage(
		context.Background(),
		"myservice", "SERVICE", "1.2.3", "", "", "",
		installedPkg("1.2.3", ""),
		alwaysInactive,
		alwaysUnloaded,
	)
	if result != installSkipDeniedUnitGone {
		t.Fatalf("expected installSkipDeniedUnitGone, got %d (%s)", result, reason)
	}
}

// TestInstallPackageDoesNotCrossKindSkip — existing record is from a different
// kind; caller passes nil (exact-kind lookup found nothing) → must install.
func TestInstallPackageDoesNotCrossKindSkip(t *testing.T) {
	// Simulate: controller says kind=INFRASTRUCTURE, but the installed-state
	// was recorded under SERVICE.  Caller queries only INFRASTRUCTURE → nil.
	result, reason := canSkipInstallPackage(
		context.Background(),
		"scylladb", "INFRASTRUCTURE", "5.4.0", "", "", "",
		nil, // exact-kind lookup returned nothing
		alwaysActive,
		alwaysLoaded,
	)
	if result != installSkipDeniedNoRecord {
		t.Fatalf("expected installSkipDeniedNoRecord, got %d (%s)", result, reason)
	}
}

// TestCommandPackageSkipUsesBinaryProofOnly — command packages have no unit;
// version match is sufficient proof.
func TestCommandPackageSkipUsesBinaryProofOnly(t *testing.T) {
	prevExists := commandBinaryExistsFunc
	prevPath := commandBinaryPathFunc
	prevChecksum := binaryChecksumFunc
	commandBinaryExistsFunc = func(string) bool { return true }
	commandBinaryPathFunc = func(string) string { return "/tmp/restic" }
	binaryChecksumFunc = func(string) (string, error) { return "deadbeef", nil }
	t.Cleanup(func() {
		commandBinaryExistsFunc = prevExists
		commandBinaryPathFunc = prevPath
		binaryChecksumFunc = prevChecksum
	})

	result, reason := canSkipInstallPackage(
		context.Background(),
		"restic", "COMMAND", "0.16.0", "", "", "",
		installedPkg("0.16.0", ""),
		// isActive/isLoaded should never be called for command packages,
		// but pass always-inactive to prove we don't care about their output.
		alwaysInactive,
		alwaysUnloaded,
	)
	if result != installSkipAllowed {
		t.Fatalf("expected installSkipAllowed for command package, got %d (%s)", result, reason)
	}
}

func TestCommandPackageWithNoUnitProducesNoFinding(t *testing.T) {
	TestCommandPackageSkipUsesBinaryProofOnly(t)
}

// TestScyllaInactiveDoesNotReturnSuccess — scylladb maps to scylla-server.service;
// when that unit is inactive the install must not be skipped.
func TestScyllaInactiveDoesNotReturnSuccess(t *testing.T) {
	result, reason := canSkipInstallPackage(
		context.Background(),
		"scylladb", "INFRASTRUCTURE", "5.4.0", "", "", "",
		installedPkg("5.4.0", ""),
		alwaysInactive,
		alwaysLoaded,
	)
	if result != installSkipDeniedInactive {
		t.Fatalf("expected installSkipDeniedInactive for inactive scylladb, got %d (%s)", result, reason)
	}
	// Confirm correct unit name is in the reason string.
	if reason == "" {
		t.Fatal("expected non-empty reason")
	}
}

func TestInstallPackageDoesNotSkipWhenBuildIDMissing(t *testing.T) {
	result, _ := canSkipInstallPackage(
		context.Background(),
		"myservice", "SERVICE", "1.2.3", "", "", "build-123",
		installedPkg("1.2.3", ""),
		alwaysActive,
		alwaysLoaded,
	)
	if result != installSkipDeniedVersion {
		t.Fatalf("expected installSkipDeniedVersion for missing build_id, got %d", result)
	}
}

func TestInstallPackageDoesNotSkipWhenChecksumMismatch(t *testing.T) {
	pkg := installedPkg("1.2.3", "")
	pkg.Checksum = "sha256:aaaa"
	result, _ := canSkipInstallPackage(
		context.Background(),
		"myservice", "SERVICE", "1.2.3", "sha256:bbbb", "", "",
		pkg,
		alwaysActive,
		alwaysLoaded,
	)
	if result != installSkipDeniedVersion {
		t.Fatalf("expected installSkipDeniedVersion for checksum mismatch, got %d", result)
	}
}

func TestInstallPackageSkipsChecksumOnlyDriftWhenBuildIDAndEntrypointMatch(t *testing.T) {
	const (
		binarySHA       = "241c1702f0ed1c0dba31339abaab422906a4295cc92640f5b832c131ee385767"
		convergenceHash = "cfd6de59b3ecdb00f5b8430d1d8cc5cc6c00e8e88468dfb96d47f2fbe9425212"
		buildID         = "019f0000-1111-7222-8333-444455556666"
	)
	pkg := installedPkg("1.35.3", buildID)
	pkg.Checksum = binarySHA
	pkg.Metadata["entrypoint_checksum"] = binarySHA

	result, reason := canSkipInstallPackage(
		context.Background(),
		"envoy", "INFRASTRUCTURE", "1.35.3", convergenceHash, binarySHA, buildID,
		pkg,
		alwaysActive,
		alwaysLoaded,
	)
	if result != installSkipAllowed {
		t.Fatalf("expected installSkipAllowed for checksum-only drift with matching build_id+entrypoint, got %d (%s)", result, reason)
	}
}

func TestInstallPackageDoesNotSkipWhenEntrypointChecksumMismatches(t *testing.T) {
	const (
		oldBinarySHA = "241c1702f0ed1c0dba31339abaab422906a4295cc92640f5b832c131ee385767"
		newBinarySHA = "cfd6de59b3ecdb00f5b8430d1d8cc5cc6c00e8e88468dfb96d47f2fbe9425212"
		buildID      = "019f0000-1111-7222-8333-444455556666"
	)
	pkg := installedPkg("1.35.3", buildID)
	pkg.Checksum = oldBinarySHA
	pkg.Metadata["entrypoint_checksum"] = oldBinarySHA

	result, reason := canSkipInstallPackage(
		context.Background(),
		"envoy", "INFRASTRUCTURE", "1.35.3", newBinarySHA, newBinarySHA, buildID,
		pkg,
		alwaysActive,
		alwaysLoaded,
	)
	if result != installSkipDeniedVersion {
		t.Fatalf("expected installSkipDeniedVersion for entrypoint checksum mismatch, got %d (%s)", result, reason)
	}
}

func TestPackageUnitKeepalivedUsesOSUnit(t *testing.T) {
	if got := packageUnit("keepalived"); got != "keepalived.service" {
		t.Fatalf("packageUnit(keepalived) = %q, want keepalived.service", got)
	}
}

func TestInstallPackageSkipsKeepalivedWhenIngressOwnsRuntime(t *testing.T) {
	stubEntrypoint(t, "keepalived", "none")
	const (
		convergenceHash = "9a18c1b5b8f7a4d3c2e1f00112233445566778899aabbccddeeff0011223344"
		buildID         = "019f0000-aaaa-7bbb-8ccc-111122223333"
	)
	pkg := installedPkgNoChecksum("2.2.8", buildID)

	result, reason := canSkipInstallPackage(
		context.Background(),
		"keepalived", "INFRASTRUCTURE", "2.2.8", convergenceHash, "", buildID,
		pkg,
		alwaysInactive,
		alwaysLoaded,
	)
	if result != installSkipAllowed {
		t.Fatalf("expected keepalived install skip when package identity matches and ingress owns runtime, got %d (%s)", result, reason)
	}
}

func TestCommandPackageDoesNotSkipWhenBinaryMissing(t *testing.T) {
	prevExists := commandBinaryExistsFunc
	commandBinaryExistsFunc = func(string) bool { return false }
	t.Cleanup(func() { commandBinaryExistsFunc = prevExists })

	result, _ := canSkipInstallPackage(
		context.Background(),
		"restic", "COMMAND", "0.16.0", "", "", "",
		installedPkg("0.16.0", ""),
		alwaysInactive,
		alwaysUnloaded,
	)
	if result != installSkipDeniedUnitGone {
		t.Fatalf("expected installSkipDeniedUnitGone when command binary missing, got %d", result)
	}
}

func TestCommandPackageDoesNotSkipWhenBinaryChecksumMismatch(t *testing.T) {
	prevExists := commandBinaryExistsFunc
	prevPath := commandBinaryPathFunc
	prevChecksum := binaryChecksumFunc
	commandBinaryExistsFunc = func(string) bool { return true }
	commandBinaryPathFunc = func(string) string { return "/tmp/restic" }
	binaryChecksumFunc = func(string) (string, error) { return "aaaaaaaa", nil }
	t.Cleanup(func() {
		commandBinaryExistsFunc = prevExists
		commandBinaryPathFunc = prevPath
		binaryChecksumFunc = prevChecksum
	})

	result, _ := canSkipInstallPackage(
		context.Background(),
		"restic", "COMMAND", "0.16.0", "", "sha256:bbbbbbbb", "",
		installedPkg("0.16.0", ""),
		alwaysInactive,
		alwaysUnloaded,
	)
	if result != installSkipDeniedVersion {
		t.Fatalf("expected installSkipDeniedVersion when command checksum mismatches, got %d", result)
	}
}

func TestCommandPackageSkipWhenBinaryChecksumMatches(t *testing.T) {
	prevExists := commandBinaryExistsFunc
	prevPath := commandBinaryPathFunc
	prevChecksum := binaryChecksumFunc
	commandBinaryExistsFunc = func(string) bool { return true }
	commandBinaryPathFunc = func(string) string { return "/tmp/restic" }
	binaryChecksumFunc = func(string) (string, error) { return "abc123", nil }
	t.Cleanup(func() {
		commandBinaryExistsFunc = prevExists
		commandBinaryPathFunc = prevPath
		binaryChecksumFunc = prevChecksum
	})

	result, reason := canSkipInstallPackage(
		context.Background(),
		"restic", "COMMAND", "0.16.0", "", "sha256:abc123", "",
		installedPkg("0.16.0", ""),
		alwaysInactive,
		alwaysUnloaded,
	)
	if result != installSkipAllowed {
		t.Fatalf("expected installSkipAllowed for matching command checksum, got %d (%s)", result, reason)
	}
}

func TestCommandPackageDoesNotSkipWhenChecksumReadFails(t *testing.T) {
	prevExists := commandBinaryExistsFunc
	prevPath := commandBinaryPathFunc
	prevChecksum := binaryChecksumFunc
	commandBinaryExistsFunc = func(string) bool { return true }
	commandBinaryPathFunc = func(string) string { return "/tmp/restic" }
	binaryChecksumFunc = func(string) (string, error) { return "", fmt.Errorf("checksum read failed") }
	t.Cleanup(func() {
		commandBinaryExistsFunc = prevExists
		commandBinaryPathFunc = prevPath
		binaryChecksumFunc = prevChecksum
	})

	result, _ := canSkipInstallPackage(
		context.Background(),
		"restic", "COMMAND", "0.16.0", "", "sha256:abc123", "",
		installedPkg("0.16.0", ""),
		alwaysInactive,
		alwaysUnloaded,
	)
	if result != installSkipDeniedVersion {
		t.Fatalf("expected installSkipDeniedVersion when checksum read fails, got %d", result)
	}
}

// TestBuildIDSkipChecksumOK — regression for INC-2026-0019: ApplyPackageRelease
// was skipping reinstall when build_id matched even if the installed binary
// checksum differed from the manifest's expected_sha256 (binary replaced
// out-of-band via globular deploy or local build). The guard must deny skip
// when both hashes are present and disagree.
func TestBuildIDSkipChecksumOK(t *testing.T) {
	cases := []struct {
		name      string
		installed string
		expected  string
		wantOK    bool
	}{
		{"match", "sha256:aaaa", "sha256:aaaa", true},
		{"match_no_prefix", "aaaa", "aaaa", true},
		{"mismatch", "sha256:aaaa", "sha256:bbbb", false},
		{"mismatch_mixed_prefix", "aaaa", "sha256:bbbb", false},
		{"no_expected", "sha256:aaaa", "", true},
		{"no_installed", "", "sha256:bbbb", true},
		{"both_empty", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildIDSkipChecksumOK(tc.installed, tc.expected)
			if got != tc.wantOK {
				t.Errorf("buildIDSkipChecksumOK(%q, %q) = %v, want %v",
					tc.installed, tc.expected, got, tc.wantOK)
			}
		})
	}
}

func TestInstalledBinaryChecksumForSkipPrefersEntrypointMetadata(t *testing.T) {
	pkg := &node_agentpb.InstalledPackage{
		Checksum: "convergence-hash",
		Metadata: map[string]string{
			"entrypoint_checksum": "binary-hash",
		},
	}
	if got := installedBinaryChecksumForSkip(pkg); got != "binary-hash" {
		t.Fatalf("installedBinaryChecksumForSkip = %q, want metadata entrypoint checksum", got)
	}
}

// TestNewCommandPackagesAreRecognized verifies that all packages added to
// commandPackages in v1.2.64+ are treated as binary-only (no systemd unit).
// Missing entries cause the skip check to try a unit lookup, which always
// fails and triggers unnecessary reinstalls.
func TestNewCommandPackagesAreRecognized(t *testing.T) {
	newEntries := []string{"sha256sum", "yt-dlp", "claude", "codex", "globular-cli"}
	for _, name := range newEntries {
		if !isCommandPackage(name) {
			t.Errorf("isCommandPackage(%q) = false, want true — missing from commandPackages map", name)
		}
		unit := packageUnit(name)
		if unit != "" {
			t.Errorf("packageUnit(%q) = %q, want empty string — COMMAND packages must have no unit", name, unit)
		}
	}
}
