package main

// drift_entrypoint_checksum_test.go — Phase 37.
//
// Pins the binary-identity drift contract documented in the inline
// drift-reconciler comment:
//
//   "(version, build_number) match alone is NOT sufficient proof of
//    convergence — the installed_state can claim a buildId/buildNumber
//    via the convergence committer while the binary on disk is still
//    old. The entrypoint checksum is the hard binary identity proof."
//
// Caught live on globule-ryzen 2026-06-03: Phase 32's partial repair
// left installed_state.buildId pointing at the new artifact but the
// binary on disk was never swapped (entrypoint_checksum still old).
// Pre-Phase-37 drift comparison missed this because it only compared
// (version, buildNumber). These tests pin the entrypoint_checksum
// arm of the comparison.

import (
	"strings"
	"testing"
)

func TestEntrypointChecksumDriftPresent_BothEmpty_NoDrift(t *testing.T) {
	// Legacy artifact with no proof recorded on either side. The
	// verifier owns this case via runtime_identity_unproven — drift
	// reconciler stays out of speculation here.
	if entrypointChecksumDriftPresent("", "") {
		t.Fatal("both-empty should not report drift")
	}
}

func TestEntrypointChecksumDriftPresent_OneSideEmpty_NoDrift(t *testing.T) {
	// Cannot confidently compare — one side missing proof. The
	// verifier surfaces missing proof; drift-reconciler doesn't
	// dispatch speculatively on incomplete data.
	if entrypointChecksumDriftPresent("sha256:abc", "") {
		t.Error("desired-only proof should not report drift")
	}
	if entrypointChecksumDriftPresent("", "sha256:abc") {
		t.Error("installed-only proof should not report drift")
	}
}

func TestEntrypointChecksumDriftPresent_Equal_NoDrift(t *testing.T) {
	// The healthy case: both sides agree on the binary identity.
	for _, c := range []struct{ d, i string }{
		{"abc123", "abc123"},
		{"sha256:abc123", "abc123"},     // prefix on one side
		{"ABC123", "abc123"},             // case variance
		{"  sha256:abc123  ", "abc123"}, // whitespace variance
	} {
		if entrypointChecksumDriftPresent(c.d, c.i) {
			t.Errorf("desired=%q installed=%q should NOT be drift (normalized equal)", c.d, c.i)
		}
	}
}

func TestEntrypointChecksumDriftPresent_Differ_Drift(t *testing.T) {
	// THE Phase 37 case. Different binary checksums = drift, even when
	// the buildId/version layer says "converged."
	if !entrypointChecksumDriftPresent(
		"e9434387f92fd3a19fc399fa8d2a9b2f7097f151d0027b4a7d20cccfe22556c8", // CI v1.2.143
		"20d5bfff12f4ee2fd25bedaebb95740d80b51137b7345f176977cffea47d35ec", // phantom on disk
	) {
		t.Fatal("CI vs phantom checksums must be classified as drift — this is the Phase 37 root case")
	}
}

func TestNormalizeChecksum_StripsPrefixAndLowercases(t *testing.T) {
	// shortChecksum + normalizeChecksum are used in log messages; pin
	// the normalization so operators see consistent forms.
	cases := []struct {
		in   string
		want string
	}{
		{"sha256:ABC123", "abc123"},
		{"  sha256:abc123  ", "abc123"},
		{"ABC123", "abc123"},
		{"abc123", "abc123"},
		{"", ""},
	}
	for _, c := range cases {
		if got := normalizeChecksum(c.in); got != c.want {
			t.Errorf("normalizeChecksum(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestShortChecksum_TruncatesTo16(t *testing.T) {
	if got := shortChecksum("sha256:abcdef0123456789aaaaaaaa"); got != "abcdef0123456789" {
		t.Errorf("shortChecksum truncation failed: %q", got)
	}
	if got := shortChecksum("short"); got != "short" {
		t.Errorf("shortChecksum should pass-through short input: %q", got)
	}
}

// ── End-to-end shape: the live INC pattern ─────────────────────────────────
// installed_state.buildId == desired.buildID
// AND installed_state.buildNumber == desired.buildNumber
// AND installed_state.entrypoint_checksum != desired.entrypoint_checksum
// →   drift IS reported.

func TestDriftReconciler_BuildIdMatchesButBinaryDiffers_IsDrift(t *testing.T) {
	// Direct shape test using the exact identity values observed live.
	desired := desiredVersionInfo{
		version:            "1.2.143",
		buildNumber:        2,
		buildID:            "019e8da6-42a7-7201-b858-4bf26d76e67c",
		entrypointChecksum: "e9434387f92fd3a19fc399fa8d2a9b2f7097f151d0027b4a7d20cccfe22556c8",
	}
	installed := installedInfo{
		version:            "1.2.143",
		buildNumber:        2,
		status:             "installed",
		buildID:            "019e8da6-42a7-7201-b858-4bf26d76e67c",
		entrypointChecksum: "20d5bfff12f4ee2fd25bedaebb95740d80b51137b7345f176977cffea47d35ec",
	}
	if !entrypointChecksumDriftPresent(desired.entrypointChecksum, installed.entrypointChecksum) {
		t.Fatal("the live Phase 37 case (buildId+buildNumber match, binary differs) must be classified as drift")
	}
}

func TestDriftReconciler_BuildIdMatchesAndBinaryMatches_NoDrift(t *testing.T) {
	// Inverse: when the install really did happen, drift must NOT
	// be reported. This is the post-Phase-37-reinstall steady state.
	desired := desiredVersionInfo{
		version:            "1.2.143",
		buildNumber:        2,
		entrypointChecksum: "e9434387f92fd3a19fc399fa8d2a9b2f7097f151d0027b4a7d20cccfe22556c8",
	}
	installed := installedInfo{
		version:            "1.2.143",
		buildNumber:        2,
		entrypointChecksum: "e9434387f92fd3a19fc399fa8d2a9b2f7097f151d0027b4a7d20cccfe22556c8",
	}
	if entrypointChecksumDriftPresent(desired.entrypointChecksum, installed.entrypointChecksum) {
		t.Fatal("matching binary checksums must not be drift")
	}
}

func TestDriftReconciler_ShortChecksumInLogMessage(t *testing.T) {
	// Log messages embed shortChecksum() values so operators can
	// quickly tell which checksums diverged. Pin format characteristics
	// so a refactor doesn't accidentally break the operator UX.
	s := shortChecksum("e9434387f92fd3a19fc399fa8d2a9b2f7097f151d0027b4a7d20cccfe22556c8")
	if len(s) != 16 {
		t.Errorf("shortChecksum returned %d chars; expected 16 for full-length hex", len(s))
	}
	if !strings.HasPrefix(s, "e9434387") {
		t.Errorf("shortChecksum prefix lost: got %q", s)
	}
}
