package main

// handlers_health_version_test.go — Phase 3 (Diagnostic Honesty Refactor).
//
// Pins the contract of decideVersionVerdict + the legacy versionCheckDecision
// shim. Strict-by-default: a passing claim-level check WITHOUT independent
// runtime proof is degraded, not healthy. The escape hatch
// GLOBULAR_HEALTH_LEGACY_CLAIM_OK=1 restores pre-Phase-3 semantics for
// operators still in transition.

import (
	"strings"
	"testing"
)

// ─────────────────────────────────────────────────────────────────────────
// Claim disagreement — critical mismatch regardless of proof state. The
// reason text from the legacy bool API stays compatible so existing UIs
// keep rendering the same string.
// ─────────────────────────────────────────────────────────────────────────

func TestDecideVersionVerdict_BuildIdsDiffer_VersionsDiffer_Mismatch(t *testing.T) {
	v := decideVersionVerdict(
		"1.2.0", "80ab89b1-72b5-4b2e-ba01-bbebba1cffbd",
		"1.1.5", "ffffffff-1111-2222-3333-444444444444", true)
	if v.Ok {
		t.Errorf("claim disagreement must produce Ok=false")
	}
	if v.ProofStatus != "mismatch" {
		t.Errorf("ProofStatus=%q want=mismatch", v.ProofStatus)
	}
	if v.FindingID != "service.running_version_mismatch" {
		t.Errorf("FindingID=%q want=service.running_version_mismatch", v.FindingID)
	}
	if v.ClaimOK {
		t.Errorf("ClaimOK=true; want false (versions and build_ids both differ)")
	}
	if v.Reason != "installed 1.1.5, desired 1.2.0" {
		t.Errorf("legacy reason text drifted: %q", v.Reason)
	}
}

func TestDecideVersionVerdict_NotInstalled_Mismatch(t *testing.T) {
	v := decideVersionVerdict("1.1.5", "", "", "", false)
	if v.Ok {
		t.Error("missing install must produce Ok=false")
	}
	if v.FindingID == "" {
		t.Error("missing install must carry a finding id")
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Claim agrees + no runtime proof = degraded (Phase 3 strict default).
// Pins the new behaviour: claim-only OK is forbidden per the brief.
// ─────────────────────────────────────────────────────────────────────────

func TestDecideVersionVerdict_BuildIdsMatch_NoProof_DegradedUnverified(t *testing.T) {
	// Ensure the legacy override is not set for this test.
	t.Setenv("GLOBULAR_HEALTH_LEGACY_CLAIM_OK", "")
	v := decideVersionVerdict(
		"1.2.0", "80ab89b1-72b5-4b2e-ba01-bbebba1cffbd",
		"1.2.0", "80ab89b1-72b5-4b2e-ba01-bbebba1cffbd", true)
	if v.Ok {
		t.Errorf("strict default: claim match + no proof must NOT be Ok; got Ok=true")
	}
	if !v.ClaimOK {
		t.Errorf("ClaimOK must be true (build_ids match)")
	}
	if v.ProofStatus != "claim_only" {
		t.Errorf("ProofStatus=%q want=claim_only", v.ProofStatus)
	}
	if v.FindingID != "service.runtime_identity_unproven" {
		t.Errorf("FindingID=%q want=service.runtime_identity_unproven", v.FindingID)
	}
	if !strings.Contains(v.Reason, "claim:OK") || !strings.Contains(v.Reason, "proof:UNVERIFIED") {
		t.Errorf("reason text must distinguish claim from proof: %q", v.Reason)
	}
	if !strings.Contains(v.Reason, "service.runtime_identity_unproven") {
		t.Errorf("reason text must name the finding id for operator triage: %q", v.Reason)
	}
}

func TestDecideVersionVerdict_BuildDriftVersionsMatch_NoProof_DegradedUnverified(t *testing.T) {
	t.Setenv("GLOBULAR_HEALTH_LEGACY_CLAIM_OK", "")
	v := decideVersionVerdict(
		"1.2.0", "80ab89b1-72b5-4b2e-ba01-bbebba1cffbd",
		"1.2.0", "ffffffff-1111-2222-3333-444444444444", true)
	if v.Ok {
		t.Error("build-drift case is still claim_only — must NOT be Ok under strict default")
	}
	if !v.ClaimOK {
		t.Error("ClaimOK must be true for build-drift case (versions match)")
	}
	if !strings.Contains(v.Reason, "build drift") {
		t.Errorf("reason must surface the build_id drift for operator triage: %q", v.Reason)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Legacy escape hatch — the env var restores pre-Phase-3 semantics so a
// fleet doesn't go entirely red the instant this change ships. To be
// removed once Phase 9 verifier is live.
// ─────────────────────────────────────────────────────────────────────────

func TestDecideVersionVerdict_LegacyOverride_RestoresClaimOkSemantics(t *testing.T) {
	t.Setenv("GLOBULAR_HEALTH_LEGACY_CLAIM_OK", "1")
	v := decideVersionVerdict(
		"1.2.0", "80ab89b1-72b5-4b2e-ba01-bbebba1cffbd",
		"1.2.0", "80ab89b1-72b5-4b2e-ba01-bbebba1cffbd", true)
	if !v.Ok {
		t.Error("legacy override must produce Ok=true on claim-only match")
	}
	if v.ProofStatus != "claim_only" {
		t.Errorf("ProofStatus=%q want=claim_only (even under legacy override)", v.ProofStatus)
	}
	// Reason text mirrors the historical empty-string contract when
	// build_ids match.
	if v.Reason != "" {
		t.Errorf("legacy override + claim match → reason should match pre-Phase-3 (\"\"); got %q", v.Reason)
	}
}

func TestDecideVersionVerdict_LegacyOverride_VariousTruthyValues(t *testing.T) {
	for _, val := range []string{"1", "true", "TRUE", "yes", "Yes"} {
		t.Setenv("GLOBULAR_HEALTH_LEGACY_CLAIM_OK", val)
		v := decideVersionVerdict("1.0", "bid", "1.0", "bid", true)
		if !v.Ok {
			t.Errorf("GLOBULAR_HEALTH_LEGACY_CLAIM_OK=%q should enable legacy Ok=true", val)
		}
	}
	// Falsy values keep strict behaviour.
	for _, val := range []string{"", "0", "false", "no"} {
		t.Setenv("GLOBULAR_HEALTH_LEGACY_CLAIM_OK", val)
		v := decideVersionVerdict("1.0", "bid", "1.0", "bid", true)
		if v.Ok {
			t.Errorf("GLOBULAR_HEALTH_LEGACY_CLAIM_OK=%q should NOT enable legacy Ok=true", val)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Legacy versionCheckDecision shim — kept for any caller that hasn't
// migrated. Its (bool, string) return mirrors decideVersionVerdict.Ok and
// .Reason exactly.
// ─────────────────────────────────────────────────────────────────────────

func TestVersionCheckDecision_LegacyShim_DelegatesToVerdict(t *testing.T) {
	t.Setenv("GLOBULAR_HEALTH_LEGACY_CLAIM_OK", "")
	cases := []struct {
		name                                                     string
		desiredVer, desiredBID, installedVer, installedBID       string
		hasInstalled                                             bool
		wantOK                                                   bool
		wantReasonContains                                       string
	}{
		{
			name:               "claim match + no proof → strict default degraded",
			desiredVer:         "1.2.0",
			desiredBID:         "bid-a",
			installedVer:       "1.2.0",
			installedBID:       "bid-a",
			hasInstalled:       true,
			wantOK:             false,
			wantReasonContains: "proof:UNVERIFIED",
		},
		{
			name:               "claim mismatch → critical",
			desiredVer:         "1.2.0",
			desiredBID:         "bid-a",
			installedVer:       "1.1.0",
			installedBID:       "bid-b",
			hasInstalled:       true,
			wantOK:             false,
			wantReasonContains: "installed 1.1.0, desired 1.2.0",
		},
		{
			name:               "not installed → fail with desired version cited",
			desiredVer:         "1.1.5",
			desiredBID:         "",
			installedVer:       "",
			installedBID:       "",
			hasInstalled:       false,
			wantOK:             false,
			wantReasonContains: "not installed (desired 1.1.5)",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, reason := versionCheckDecision(tc.desiredVer, tc.desiredBID, tc.installedVer, tc.installedBID, tc.hasInstalled)
			if ok != tc.wantOK {
				t.Errorf("ok=%v want=%v (reason=%q)", ok, tc.wantOK, reason)
			}
			if !strings.Contains(reason, tc.wantReasonContains) {
				t.Errorf("reason=%q does not contain %q", reason, tc.wantReasonContains)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Pin: a verified verdict carries empty FindingID and ProofStatus=verified.
// Phase 9 will produce this verdict once GetServiceRuntimeProof is wired
// into the health pipeline. Test the contract now so the integration can
// rely on it.
// ─────────────────────────────────────────────────────────────────────────

func TestVersionHealthVerdict_VerifiedShapeContract(t *testing.T) {
	// Construct directly to pin the shape (we can't reach "verified" without
	// proof wiring, which is Phase 9).
	v := versionHealthVerdict{
		Ok: true, Reason: "",
		ProofStatus: "verified", FindingID: "", ClaimOK: true,
	}
	if !v.Ok || v.ProofStatus != "verified" || v.FindingID != "" {
		t.Errorf("verified shape contract drifted: %+v", v)
	}
}

func TestShortBuildID(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"abc", "abc"},
		{"80ab89b1-72b5-4b2e-ba01-bbebba1cffbd", "80ab89b1"},
		{"12345678", "12345678"},
		{"123456789", "12345678"},
	}
	for _, tc := range cases {
		if got := shortBuildID(tc.in); got != tc.want {
			t.Errorf("shortBuildID(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
