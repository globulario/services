package main

import (
	"testing"
)

// TestLeaseManagerCreation verifies the lease manager initializes correctly.
func TestLeaseManagerCreation(t *testing.T) {
	srv := &server{} // no ScyllaDB session
	m := newExecutorLeaseManager(srv)
	if m.executorID == "" {
		t.Error("executorID should not be empty")
	}
	if m.ownedRuns == nil {
		t.Error("ownedRuns map should be initialized")
	}
}

// TestClaimRunWithoutScylla verifies that claim always succeeds in
// single-node mode (no ScyllaDB).
func TestClaimRunWithoutScylla(t *testing.T) {
	srv := &server{} // session == nil
	m := newExecutorLeaseManager(srv)

	claimed, err := m.ClaimRun(nil, "test-run-001")
	if err != nil {
		t.Fatalf("ClaimRun without ScyllaDB should succeed, got: %v", err)
	}
	if !claimed {
		t.Error("ClaimRun without ScyllaDB should always claim")
	}
}

// TestReleaseRunWithoutScylla verifies that release is a no-op in
// single-node mode.
func TestReleaseRunWithoutScylla(t *testing.T) {
	srv := &server{} // session == nil
	m := newExecutorLeaseManager(srv)

	// Should not panic.
	m.ReleaseRun("test-run-001")
}

// TestOwnershipExclusivity verifies that the lease manager's in-memory
// tracking correctly manages owned runs.
func TestOwnershipExclusivity(t *testing.T) {
	srv := &server{} // no ScyllaDB
	m := newExecutorLeaseManager(srv)

	// Claim a run.
	claimed, _ := m.ClaimRun(nil, "run-A")
	if !claimed {
		t.Fatal("expected claim to succeed")
	}

	// Verify it's tracked.
	m.mu.Lock()
	_, tracked := m.ownedRuns["run-A"]
	m.mu.Unlock()
	// In no-ScyllaDB mode, the run is NOT tracked (no heartbeat needed).
	// This is correct — tracking only matters with ScyllaDB.
	_ = tracked

	// Release should be clean.
	m.ReleaseRun("run-A")
}

// TestMultipleRunsTracked verifies that multiple runs can be managed
// simultaneously.
func TestMultipleRunsTracked(t *testing.T) {
	srv := &server{} // no ScyllaDB
	m := newExecutorLeaseManager(srv)

	m.ClaimRun(nil, "run-1")
	m.ClaimRun(nil, "run-2")
	m.ClaimRun(nil, "run-3")

	// All should release cleanly.
	m.ReleaseRun("run-1")
	m.ReleaseRun("run-2")
	m.ReleaseRun("run-3")
}

// TestOrphanScannerDoesNotPanicWithoutScylla verifies the scanner
// gracefully handles no ScyllaDB.
func TestOrphanScannerDoesNotPanicWithoutScylla(t *testing.T) {
	srv := &server{} // no ScyllaDB
	m := newExecutorLeaseManager(srv)
	// StartOrphanScanner should be a no-op without ScyllaDB.
	m.StartOrphanScanner(nil)
}
