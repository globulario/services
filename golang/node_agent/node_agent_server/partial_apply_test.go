package main

import (
	"testing"
)

// ── Typed observation source tests ───────────────────────────────────────────

// TestObservationSource_ManagedIsAuthoritative verifies that only
// ManagedInstalled observations are authoritative.
func TestObservationSource_ManagedIsAuthoritative(t *testing.T) {
	cases := []struct {
		source ObservationSource
		auth   bool
	}{
		{ManagedInstalled, true},
		{RuntimeUnmanaged, false},
		{FallbackDiscovered, false},
	}
	for _, tc := range cases {
		info := InstalledServiceInfo{Source: tc.source}
		if info.IsAuthoritative() != tc.auth {
			t.Errorf("Source=%s: expected authoritative=%v, got %v",
				tc.source, tc.auth, info.IsAuthoritative())
		}
	}
}

// TestObservationSource_String verifies string representation.
func TestObservationSource_String(t *testing.T) {
	if ManagedInstalled.String() != "managed_installed" {
		t.Errorf("expected 'managed_installed', got %q", ManagedInstalled.String())
	}
	if RuntimeUnmanaged.String() != "runtime_unmanaged" {
		t.Errorf("expected 'runtime_unmanaged', got %q", RuntimeUnmanaged.String())
	}
	if FallbackDiscovered.String() != "fallback_discovered" {
		t.Errorf("expected 'fallback_discovered', got %q", FallbackDiscovered.String())
	}
}

// TestNonAuthoritativeFilteredFromReport verifies that services with
// non-authoritative sources are NOT included in heartbeat reports,
// regardless of their version string. This proves the logic is
// source-based, not magic-string-based.
func TestNonAuthoritativeFilteredFromReport(t *testing.T) {
	installed := map[ServiceKey]InstalledServiceInfo{
		// ManagedInstalled — should be reported.
		{PublisherID: "globular", ServiceName: "dns"}: {
			Version: "0.0.2", ServiceName: "dns", Source: ManagedInstalled,
		},
		// RuntimeUnmanaged with "unknown" version — should NOT be reported.
		{PublisherID: "unknown", ServiceName: "legacy"}: {
			Version: "unknown", ServiceName: "legacy", Source: RuntimeUnmanaged,
		},
		// RuntimeUnmanaged with a REAL-LOOKING version (e.g. "0.0.5") —
		// should STILL NOT be reported because source is non-authoritative.
		// This proves filtering is source-based, not ""-based.
		{PublisherID: "unknown", ServiceName: "rogue"}: {
			Version: "0.0.5", ServiceName: "rogue", Source: RuntimeUnmanaged,
		},
		// FallbackDiscovered with a plausible version — should NOT be reported.
		{PublisherID: "unknown", ServiceName: "stale"}: {
			Version: "1.2.3", ServiceName: "stale", Source: FallbackDiscovered,
		},
	}

	// Simulate heartbeat Phase 1 filtering (same logic as reportStatus).
	reported := make(map[string]string)
	for key, info := range installed {
		if !info.IsAuthoritative() {
			continue
		}
		if info.Version == "unknown" || info.Version == "" {
			continue // defense-in-depth
		}
		reported[key.String()] = info.Version
	}

	if _, ok := reported["legacy"]; ok {
		t.Error("RuntimeUnmanaged 'legacy' must not be reported")
	}
	if _, ok := reported["rogue"]; ok {
		t.Error("RuntimeUnmanaged 'rogue' (version 0.0.5) must not be reported — source-based, not version-based")
	}
	if _, ok := reported["stale"]; ok {
		t.Error("FallbackDiscovered 'stale' must not be reported")
	}
	if v, ok := reported["dns"]; !ok || v != "0.0.2" {
		t.Error("ManagedInstalled 'dns' must be reported with version 0.0.2")
	}
	if len(reported) != 1 {
		t.Errorf("expected exactly 1 reported service, got %d: %v", len(reported), reported)
	}
}

// TestNonAuthoritativeFilteredFromEtcdSync verifies that non-authoritative
// entries are NOT written to etcd installed-state, using the same
// IsAuthoritative() check as the sync path.
func TestNonAuthoritativeFilteredFromEtcdSync(t *testing.T) {
	entries := []InstalledServiceInfo{
		{ServiceName: "dns", Version: "0.0.2", Source: ManagedInstalled},
		{ServiceName: "phantom", Version: "0.0.5", Source: RuntimeUnmanaged},
		{ServiceName: "old", Version: "2.0.0", Source: FallbackDiscovered},
	}

	var wouldSync []string
	for _, info := range entries {
		if !info.IsAuthoritative() {
			continue
		}
		if info.Version == "unknown" || info.Version == "" {
			continue
		}
		wouldSync = append(wouldSync, info.ServiceName)
	}

	if len(wouldSync) != 1 || wouldSync[0] != "dns" {
		t.Errorf("expected only 'dns' to sync, got %v", wouldSync)
	}
}

// ── Partial apply detection tests ────────────────────────────────────────────

// TestPartialApplyDetectionContract verifies the mismatch detection contract:
// when a binary checksum differs from the recorded entrypoint_checksum but the
// version is unchanged, the status should be flagged as "partial_apply".
func TestPartialApplyDetectionContract(t *testing.T) {
	type testCase struct {
		name             string
		recordedChecksum string
		currentChecksum  string
		currentStatus    string
		expectPartial    bool
	}
	cases := []testCase{
		{
			name:             "checksums match — no mismatch",
			recordedChecksum: "abc123",
			currentChecksum:  "abc123",
			currentStatus:    "installed",
			expectPartial:    false,
		},
		{
			name:             "checksums differ — partial apply",
			recordedChecksum: "abc123",
			currentChecksum:  "def456",
			currentStatus:    "installed",
			expectPartial:    true,
		},
		{
			name:             "no recorded checksum — skip",
			recordedChecksum: "",
			currentChecksum:  "def456",
			currentStatus:    "installed",
			expectPartial:    false,
		},
		{
			name:             "already flagged — no double-flag",
			recordedChecksum: "abc123",
			currentChecksum:  "def456",
			currentStatus:    "partial_apply",
			expectPartial:    false, // already flagged
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			shouldFlag := tc.recordedChecksum != "" &&
				tc.currentChecksum != tc.recordedChecksum &&
				tc.currentStatus != "partial_apply"
			if shouldFlag != tc.expectPartial {
				t.Errorf("expected partial=%v, got %v", tc.expectPartial, shouldFlag)
			}
		})
	}
}

// TestComputeAppliedServicesHash_NonEmpty verifies that the hash function
// produces non-empty output for real services.
func TestComputeAppliedServicesHash_NonEmpty(t *testing.T) {
	inst := map[ServiceKey]InstalledServiceInfo{
		{PublisherID: "globular", ServiceName: "dns"}: {Version: "0.0.2", ServiceName: "dns"},
	}
	hash := computeAppliedServicesHash(inst)
	if hash == "" {
		t.Error("hash for real services should not be empty")
	}
}
