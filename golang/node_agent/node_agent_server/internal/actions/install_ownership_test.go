package actions

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestInstallOwnership_DuplicateDispatchSuppressedWhileRunning(t *testing.T) {
	ActionStateDir = t.TempDir()
	t.Cleanup(func() { ActionStateDir = "/var/lib/globular" })

	first, err := AcquireInstallOwnership(AcquireInstallOwnershipRequest{
		NodeID:        "node-1",
		PackageID:     "envoy",
		TargetBuildID: "build-a",
		TransactionID: "txn-1",
	})
	if err != nil {
		t.Fatalf("acquire first: %v", err)
	}
	if first.State != InstallOwnershipStateRunning {
		t.Fatalf("first state = %q, want running", first.State)
	}

	_, err = AcquireInstallOwnership(AcquireInstallOwnershipRequest{
		NodeID:        "node-1",
		PackageID:     "envoy",
		TargetBuildID: "build-a",
		TransactionID: "txn-2",
	})
	var busyErr *InstallOwnershipBusyError
	if !errors.As(err, &busyErr) {
		t.Fatalf("expected busy error, got %v", err)
	}
	rec, err := loadInstallOwnership("node-1", "envoy")
	if err != nil {
		t.Fatalf("load ownership: %v", err)
	}
	if rec.TransactionID != "txn-1" {
		t.Fatalf("owner transaction = %q, want txn-1", rec.TransactionID)
	}
	if rec.ConflictCount != 1 {
		t.Fatalf("conflict_count = %d, want 1", rec.ConflictCount)
	}
	if rec.CooldownUntilUnix == 0 {
		t.Fatal("cooldown_until_unix must be set after duplicate suppression")
	}
}

func TestInstallOwnership_DifferentBuildAllowedOnlyAfterClose(t *testing.T) {
	ActionStateDir = t.TempDir()
	t.Cleanup(func() { ActionStateDir = "/var/lib/globular" })

	if _, err := AcquireInstallOwnership(AcquireInstallOwnershipRequest{
		NodeID:        "node-1",
		PackageID:     "envoy",
		TargetBuildID: "build-a",
		TransactionID: "txn-1",
	}); err != nil {
		t.Fatalf("acquire first: %v", err)
	}
	_, err := AcquireInstallOwnership(AcquireInstallOwnershipRequest{
		NodeID:        "node-1",
		PackageID:     "envoy",
		TargetBuildID: "build-b",
		TransactionID: "txn-2",
	})
	var busyErr *InstallOwnershipBusyError
	if !errors.As(err, &busyErr) {
		t.Fatalf("expected busy error for concurrent different build, got %v", err)
	}

	if err := CloseInstallOwnership("node-1", "envoy", "build-a", "txn-1", InstallOwnershipStateCommitted, "", 0); err != nil {
		t.Fatalf("close first: %v", err)
	}
	second, err := AcquireInstallOwnership(AcquireInstallOwnershipRequest{
		NodeID:        "node-1",
		PackageID:     "envoy",
		TargetBuildID: "build-b",
		TransactionID: "txn-2",
	})
	if err != nil {
		t.Fatalf("acquire second after close: %v", err)
	}
	if second.TargetBuildID != "build-b" {
		t.Fatalf("target_build_id = %q, want build-b", second.TargetBuildID)
	}
}

func TestInstallOwnership_PartialRecoveryBlocksNormalInstallButAllowsRecovery(t *testing.T) {
	ActionStateDir = t.TempDir()
	t.Cleanup(func() { ActionStateDir = "/var/lib/globular" })

	rec := &InstallOwnershipRecord{
		InstallKey:    installOwnershipKey("node-1", "envoy", "build-a"),
		NodeID:        "node-1",
		PackageID:     "envoy",
		TargetBuildID: "build-a",
		TransactionID: "txn-old",
		State:         InstallOwnershipStatePartialInstallRecovery,
		LastResult:    InstallOwnershipStatePartialInstallRecovery,
		AcquiredUnix:  time.Now().Unix(),
	}
	if err := writeInstallOwnership(rec); err != nil {
		t.Fatalf("seed partial recovery: %v", err)
	}

	_, err := AcquireInstallOwnership(AcquireInstallOwnershipRequest{
		NodeID:        "node-1",
		PackageID:     "envoy",
		TargetBuildID: "build-a",
		TransactionID: "txn-normal",
	})
	var recoveryErr *InstallOwnershipRecoveryRequiredError
	if !errors.As(err, &recoveryErr) {
		t.Fatalf("expected recovery-required error, got %v", err)
	}

	recoveryRec, err := AcquireInstallOwnership(AcquireInstallOwnershipRequest{
		NodeID:        "node-1",
		PackageID:     "envoy",
		TargetBuildID: "build-a",
		TransactionID: "txn-recovery",
		RecoveryMode:  true,
	})
	if err != nil {
		t.Fatalf("acquire recovery: %v", err)
	}
	if recoveryRec.Mode != InstallOwnershipModeRecovery {
		t.Fatalf("mode = %q, want recovery", recoveryRec.Mode)
	}
	if recoveryRec.State != InstallOwnershipStateRunning {
		t.Fatalf("state = %q, want running", recoveryRec.State)
	}
}

func TestInstallOwnership_RejectedDispatchDoesNotCreateStormRecords(t *testing.T) {
	ActionStateDir = t.TempDir()
	t.Cleanup(func() { ActionStateDir = "/var/lib/globular" })

	if _, err := AcquireInstallOwnership(AcquireInstallOwnershipRequest{
		NodeID:        "node-1",
		PackageID:     "envoy",
		TargetBuildID: "build-a",
		TransactionID: "txn-1",
	}); err != nil {
		t.Fatalf("acquire first: %v", err)
	}
	for i := 0; i < 3; i++ {
		_, _ = AcquireInstallOwnership(AcquireInstallOwnershipRequest{
			NodeID:        "node-1",
			PackageID:     "envoy",
			TargetBuildID: "build-a",
			TransactionID: "txn-dup-" + string(rune('a'+i)),
		})
	}
	matches, err := filepath.Glob(filepath.Join(installOwnershipRoot(), "*.json"))
	if err != nil {
		t.Fatalf("glob ownership files: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("ownership file count = %d, want 1", len(matches))
	}
	rec, err := loadInstallOwnership("node-1", "envoy")
	if err != nil {
		t.Fatalf("load ownership: %v", err)
	}
	if rec.ConflictCount != 3 {
		t.Fatalf("conflict_count = %d, want 3", rec.ConflictCount)
	}
}

func TestInstallOwnership_CooldownBlocksImmediateReacquireForSameKey(t *testing.T) {
	ActionStateDir = t.TempDir()
	t.Cleanup(func() { ActionStateDir = "/var/lib/globular" })

	if _, err := AcquireInstallOwnership(AcquireInstallOwnershipRequest{
		NodeID:        "node-1",
		PackageID:     "envoy",
		TargetBuildID: "build-a",
		TransactionID: "txn-1",
	}); err != nil {
		t.Fatalf("acquire first: %v", err)
	}
	if err := CloseInstallOwnership("node-1", "envoy", "build-a", "txn-1", InstallOwnershipStateReleased, "backend pressure", time.Minute); err != nil {
		t.Fatalf("close first: %v", err)
	}
	_, err := AcquireInstallOwnership(AcquireInstallOwnershipRequest{
		NodeID:        "node-1",
		PackageID:     "envoy",
		TargetBuildID: "build-a",
		TransactionID: "txn-2",
	})
	var cooldownErr *InstallOwnershipCooldownError
	if !errors.As(err, &cooldownErr) {
		t.Fatalf("expected cooldown error, got %v", err)
	}
}
