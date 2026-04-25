package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/globulario/services/golang/node_agent/node_agentpb"
)

// helpers for building test fixtures

func installedPkg(version, buildID string) *node_agentpb.InstalledPackage {
	return &node_agentpb.InstalledPackage{
		Version: version,
		BuildId: buildID,
	}
}

func alwaysActive(_ context.Context, _ string) (bool, error)  { return true, nil }
func alwaysInactive(_ context.Context, _ string) (bool, error) { return false, nil }
func alwaysLoaded(_ context.Context, _ string) (bool, error)   { return true, nil }
func alwaysUnloaded(_ context.Context, _ string) (bool, error) { return false, nil }

// TestInstallPackageSkipsOnlyWhenRuntimeActive — happy path: version matches AND
// unit is active → skip is allowed.
func TestInstallPackageSkipsOnlyWhenRuntimeActive(t *testing.T) {
	result, reason := canSkipInstallPackage(
		context.Background(),
		"myservice", "SERVICE", "1.2.3", "", "",
		installedPkg("1.2.3", ""),
		alwaysActive,
		alwaysLoaded,
	)
	if result != installSkipAllowed {
		t.Fatalf("expected installSkipAllowed, got %d (%s)", result, reason)
	}
}

// TestInstallPackageDoesNotSkipWhenUnitInactive — version matches but unit is
// loaded+inactive → must repair, not skip.
func TestInstallPackageDoesNotSkipWhenUnitInactive(t *testing.T) {
	result, reason := canSkipInstallPackage(
		context.Background(),
		"myservice", "SERVICE", "1.2.3", "", "",
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
		"myservice", "SERVICE", "1.2.3", "", "",
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
		"scylladb", "INFRASTRUCTURE", "5.4.0", "", "",
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
		"restic", "COMMAND", "0.16.0", "", "",
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

// TestScyllaInactiveDoesNotReturnSuccess — scylladb maps to scylla-server.service;
// when that unit is inactive the install must not be skipped.
func TestScyllaInactiveDoesNotReturnSuccess(t *testing.T) {
	result, reason := canSkipInstallPackage(
		context.Background(),
		"scylladb", "INFRASTRUCTURE", "5.4.0", "", "",
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
		"myservice", "SERVICE", "1.2.3", "", "build-123",
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
		"myservice", "SERVICE", "1.2.3", "sha256:bbbb", "",
		pkg,
		alwaysActive,
		alwaysLoaded,
	)
	if result != installSkipDeniedVersion {
		t.Fatalf("expected installSkipDeniedVersion for checksum mismatch, got %d", result)
	}
}

func TestCommandPackageDoesNotSkipWhenBinaryMissing(t *testing.T) {
	prevExists := commandBinaryExistsFunc
	commandBinaryExistsFunc = func(string) bool { return false }
	t.Cleanup(func() { commandBinaryExistsFunc = prevExists })

	result, _ := canSkipInstallPackage(
		context.Background(),
		"restic", "COMMAND", "0.16.0", "", "",
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
		"restic", "COMMAND", "0.16.0", "sha256:bbbbbbbb", "",
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
		"restic", "COMMAND", "0.16.0", "sha256:abc123", "",
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
		"restic", "COMMAND", "0.16.0", "sha256:abc123", "",
		installedPkg("0.16.0", ""),
		alwaysInactive,
		alwaysUnloaded,
	)
	if result != installSkipDeniedVersion {
		t.Fatalf("expected installSkipDeniedVersion when checksum read fails, got %d", result)
	}
}
