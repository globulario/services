package main

import "testing"

// TestPlatformMatchesTarget pins the platform filter's contract: arch-specific
// entries must equal the target; noarch entries must match every target;
// empty platform is grandfathered for legacy entries.
//
// Regression: the v1.2.117 awareness-bundle import surfaced
// DesiredBuildIdOrphaned because the BOM declared platform="noarch" but the
// sync entry loop did strict equality against the consumer's targetPlatform
// (linux_amd64), silently dropping the entry. See
// docs/intent/release.noarch_artifacts_are_first_class_in_sync.yaml and
// invariant repository.sync_must_not_silently_drop_noarch_artifacts.
func TestPlatformMatchesTarget(t *testing.T) {
	cases := []struct {
		name           string
		entryPlatform  string
		targetPlatform string
		want           bool
	}{
		// arch-specific entries
		{"exact_amd64_match", "linux_amd64", "linux_amd64", true},
		{"exact_arm64_match", "linux_arm64", "linux_arm64", true},
		{"arch_mismatch_amd64_target_arm64_entry", "linux_arm64", "linux_amd64", false},
		{"arch_mismatch_arm64_target_amd64_entry", "linux_amd64", "linux_arm64", false},

		// noarch — the load-bearing case
		{"noarch_on_amd64_target", "noarch", "linux_amd64", true},
		{"noarch_on_arm64_target", "noarch", "linux_arm64", true},
		{"noarch_case_insensitive_NoArch", "NoArch", "linux_amd64", true},
		{"noarch_case_insensitive_NOARCH", "NOARCH", "linux_amd64", true},
		{"noarch_with_whitespace", "  noarch  ", "linux_amd64", true},

		// empty entry platform — legacy grandfathering
		{"empty_entry_matches_any_target", "", "linux_amd64", true},
		{"whitespace_entry_matches_any_target", "   ", "linux_amd64", true},

		// noarch is not a magic prefix
		{"noarch_typo_is_not_match", "noarchx", "linux_amd64", false},
		{"noarch_with_suffix_is_not_match", "noarch_64", "linux_amd64", false},

		// Mismatched cases are rejected even when the entry contains substrings.
		{"prefix_only_is_not_match", "linux", "linux_amd64", false},
		{"suffix_only_is_not_match", "amd64", "linux_amd64", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := platformMatchesTarget(tc.entryPlatform, tc.targetPlatform)
			if got != tc.want {
				t.Errorf("platformMatchesTarget(%q, %q) = %v, want %v",
					tc.entryPlatform, tc.targetPlatform, got, tc.want)
			}
		})
	}
}

// TestSyncFromUpstream_NoArchEntryNotFilteredByPlatform — the platform
// filter MUST accept a noarch entry into the per-entry sync loop. This is
// the predicate test; the full sync-import path is covered by integration
// tests with a fake upstream provider.
func TestSyncFromUpstream_NoArchEntryNotFilteredByPlatform(t *testing.T) {
	if !platformMatchesTarget("noarch", "linux_amd64") {
		t.Fatal("noarch entry must be accepted by the platform filter on linux_amd64 — " +
			"this is the BOM-level rule for architecture-independent artifacts " +
			"(awareness bundle, signed manifests, configuration sets)")
	}
}

// TestSyncFromUpstream_ArchSpecificEntryStillFilteredOnMismatch — the
// noarch rule does NOT broaden filtering for arch-specific entries. A
// linux_arm64 entry must still be skipped by a linux_amd64 consumer.
// Regression guard against an over-eager "accept everything" fix.
func TestSyncFromUpstream_ArchSpecificEntryStillFilteredOnMismatch(t *testing.T) {
	if platformMatchesTarget("linux_arm64", "linux_amd64") {
		t.Fatal("arch-specific arm64 entry must NOT be accepted on linux_amd64 — " +
			"the noarch fix only specializes the filter for the noarch case; " +
			"arch-specific filtering remains strict")
	}
}

// TestSyncFromUpstream_AwarenessBundleNoArchImportsCleanly — the exact
// shape of the awareness-bundle BOM entry: kind=awareness_bundle, platform=noarch.
// Both pieces must combine to a positive filter verdict so the entry reaches
// processSyncEntry.
func TestSyncFromUpstream_AwarenessBundleNoArchImportsCleanly(t *testing.T) {
	if !platformMatchesTarget("noarch", "linux_amd64") {
		t.Fatal("awareness bundle entry (platform=noarch) must pass the platform filter")
	}
	if !platformMatchesTarget("noarch", "linux_arm64") {
		t.Fatal("awareness bundle entry (platform=noarch) must pass the platform filter on every target arch")
	}
}
