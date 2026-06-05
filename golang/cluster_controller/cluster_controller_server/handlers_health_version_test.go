package main

// handlers_health_version_test.go — Phase 3 (Diagnostic Honesty Refactor).
//
// Pins the contract of decideVersionVerdict + the legacy versionCheckDecision
// shim. Strict-by-default: a passing claim-level check WITHOUT independent
// runtime proof is degraded, not healthy. The escape hatch
// GLOBULAR_HEALTH_LEGACY_CLAIM_OK=1 restores pre-Phase-3 semantics for
// operators still in transition.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/globulario/services/golang/verifier"
)

// ─────────────────────────────────────────────────────────────────────────
// Claim disagreement — critical mismatch regardless of proof state. The
// reason text from the legacy bool API stays compatible so existing UIs
// keep rendering the same string.
// ─────────────────────────────────────────────────────────────────────────

func TestDecideVersionVerdict_BuildIdsDiffer_VersionsDiffer_Mismatch(t *testing.T) {
	v := decideVersionVerdict(
		"1.2.0", "80ab89b1-72b5-4b2e-ba01-bbebba1cffbd",
		"1.1.5", "ffffffff-1111-2222-3333-444444444444", true, nil)
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
	v := decideVersionVerdict("1.1.5", "", "", "", false, nil)
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
		"1.2.0", "80ab89b1-72b5-4b2e-ba01-bbebba1cffbd", true, nil)
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
		"1.2.0", "ffffffff-1111-2222-3333-444444444444", true, nil)
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
		"1.2.0", "80ab89b1-72b5-4b2e-ba01-bbebba1cffbd", true, nil)
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
		v := decideVersionVerdict("1.0", "bid", "1.0", "bid", true, nil)
		if !v.Ok {
			t.Errorf("GLOBULAR_HEALTH_LEGACY_CLAIM_OK=%q should enable legacy Ok=true", val)
		}
	}
	// Falsy values keep strict behaviour.
	for _, val := range []string{"", "0", "false", "no"} {
		t.Setenv("GLOBULAR_HEALTH_LEGACY_CLAIM_OK", val)
		v := decideVersionVerdict("1.0", "bid", "1.0", "bid", true, nil)
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

// ─────────────────────────────────────────────────────────────────────────
// Phase 9 verifier verdict integration. The handler reads per-(node,
// service) verdicts from etcd and passes the matching one into the
// version-check decision. These tests pin the contract for each
// verifier.ProofStatus value the handler must consume.
// ─────────────────────────────────────────────────────────────────────────

func TestDecideVersionVerdict_VerifierRuntimeVerified_IsOk(t *testing.T) {
	t.Setenv("GLOBULAR_HEALTH_LEGACY_CLAIM_OK", "")
	proof := &verifier.Verdict{
		ProofStatus: verifier.ProofRuntimeVerified,
		Reason:      "all proofs agree",
	}
	v := decideVersionVerdict(
		"1.2.57", "80ab89b1-72b5-4b2e-ba01-bbebba1cffbd",
		"1.2.57", "80ab89b1-72b5-4b2e-ba01-bbebba1cffbd", true, proof)
	if !v.Ok {
		t.Fatalf("runtime_verified must yield Ok=true; got %+v", v)
	}
	if v.ProofStatus != "verified" {
		t.Errorf("ProofStatus=%q want=verified", v.ProofStatus)
	}
	if v.FindingID != "" {
		t.Errorf("FindingID=%q want=empty for verified verdict", v.FindingID)
	}
}

func TestDecideVersionVerdict_VerifierInstalledVerified_IsOk(t *testing.T) {
	t.Setenv("GLOBULAR_HEALTH_LEGACY_CLAIM_OK", "")
	proof := &verifier.Verdict{
		ProofStatus: verifier.ProofInstalledVerified,
	}
	v := decideVersionVerdict(
		"1.2.57", "bid", "1.2.57", "bid", true, proof)
	if !v.Ok {
		t.Fatalf("installed_verified must yield Ok=true; got %+v", v)
	}
	if v.ProofStatus != "installed_verified" {
		t.Errorf("ProofStatus=%q want=installed_verified", v.ProofStatus)
	}
	if v.FindingID != "" {
		t.Errorf("FindingID=%q want=empty for installed_verified verdict", v.FindingID)
	}
}

func TestDecideVersionVerdict_VerifierMismatch_SurfacesFinding(t *testing.T) {
	t.Setenv("GLOBULAR_HEALTH_LEGACY_CLAIM_OK", "")
	proof := &verifier.Verdict{
		ProofStatus: verifier.ProofMismatch,
		Reason:      "running binary sha differs from installed",
		Findings: []verifier.Finding{
			{ID: verifier.FindingRunningBinaryHashMismatch, Severity: verifier.SeverityCritical},
		},
	}
	v := decideVersionVerdict(
		"1.2.57", "bid", "1.2.57", "bid", true, proof)
	if v.Ok {
		t.Fatalf("mismatch verdict must yield Ok=false; got %+v", v)
	}
	if v.ProofStatus != "mismatch" {
		t.Errorf("ProofStatus=%q want=mismatch", v.ProofStatus)
	}
	if v.FindingID != verifier.FindingRunningBinaryHashMismatch {
		t.Errorf("FindingID=%q want=%q", v.FindingID, verifier.FindingRunningBinaryHashMismatch)
	}
	if !strings.Contains(v.Reason, "running binary sha differs") {
		t.Errorf("Reason should carry verifier's reason; got %q", v.Reason)
	}
}

func TestDecideVersionVerdict_VerifierUnknown_DegradesToUnverified(t *testing.T) {
	t.Setenv("GLOBULAR_HEALTH_LEGACY_CLAIM_OK", "")
	proof := &verifier.Verdict{
		ProofStatus: verifier.ProofUnknown,
		Findings: []verifier.Finding{
			{ID: verifier.FindingRuntimeIdentityUnproven, Severity: verifier.SeverityDegraded},
		},
	}
	v := decideVersionVerdict(
		"1.2.57", "bid", "1.2.57", "bid", true, proof)
	if v.Ok {
		t.Fatalf("unknown verdict must keep Ok=false; got %+v", v)
	}
	if v.FindingID != "service.runtime_identity_unproven" {
		t.Errorf("FindingID=%q want=service.runtime_identity_unproven", v.FindingID)
	}
}

// Day-0 grace: when the verifier emits runtime_identity_unproven at
// INFO severity (its own grace window — first install, fresh apply),
// the UI surface must treat the service as OK rather than red-flagging
// the operator during normal Day-0 settling. Outside the grace window
// the verifier emits at SeverityDegraded and this branch doesn't fire.
func TestDecideVersionVerdict_VerifierDay0GraceInfo_IsOk(t *testing.T) {
	t.Setenv("GLOBULAR_HEALTH_LEGACY_CLAIM_OK", "")
	proof := &verifier.Verdict{
		ProofStatus: verifier.ProofUnknown,
		Findings: []verifier.Finding{
			{ID: verifier.FindingRuntimeIdentityUnproven, Severity: verifier.SeverityInfo},
		},
	}
	v := decideVersionVerdict(
		"1.2.57", "bid", "1.2.57", "bid", true, proof)
	if !v.Ok {
		t.Fatalf("Day-0 grace verdict must yield Ok=true; got %+v", v)
	}
	if v.ProofStatus != "claim_only_day0_grace" {
		t.Errorf("ProofStatus=%q want=claim_only_day0_grace", v.ProofStatus)
	}
	if v.FindingID != "" {
		t.Errorf("FindingID=%q want=\"\" (no finding for in-grace state)", v.FindingID)
	}
}

// Fresh-install grace: when proof is nil but the installed package was
// registered within Day0UnprovenGraceWindow, the controller synthesises
// the same Day-0 verdict the verifier emits during its own grace
// window. The existing isDay0UnprovenGraceVerdict branch then maps it
// to claim_only_day0_grace (Ok=true) so the UI doesn't flicker red for
// the gap between ServiceRelease resolution and the first verifier
// sweep. Outside the grace window the strict default still applies.
func TestDecideVersionVerdictWithInstallTime_FreshInstallNoProof_IsDay0Grace(t *testing.T) {
	t.Setenv("GLOBULAR_HEALTH_LEGACY_CLAIM_OK", "")
	freshInstall := time.Now().Add(-30 * time.Second).Unix()
	v := decideVersionVerdictWithInstallTime(
		"1.2.61", "bid", "1.2.61", "bid", true, nil, freshInstall, true,
	)
	if !v.Ok {
		t.Fatalf("fresh-install grace must yield Ok=true when proof is nil; got %+v", v)
	}
	if v.ProofStatus != "claim_only_day0_grace" {
		t.Errorf("ProofStatus=%q want=claim_only_day0_grace", v.ProofStatus)
	}
	if v.FindingID != "" {
		t.Errorf("FindingID=%q want=\"\" (no finding during grace)", v.FindingID)
	}
}

func TestDecideVersionVerdictWithInstallTime_StaleInstallNoProof_StaysFail(t *testing.T) {
	t.Setenv("GLOBULAR_HEALTH_LEGACY_CLAIM_OK", "")
	staleInstall := time.Now().Add(-1 * time.Hour).Unix()
	v := decideVersionVerdictWithInstallTime(
		"1.2.61", "bid", "1.2.61", "bid", true, nil, staleInstall, true,
	)
	if v.Ok {
		t.Fatalf("install older than grace window must keep strict FAIL; got %+v", v)
	}
	if v.FindingID != "service.runtime_identity_unproven" {
		t.Errorf("FindingID=%q want=service.runtime_identity_unproven", v.FindingID)
	}
}

func TestDecideVersionVerdictWithInstallTime_NoInstallSignal_FallsThroughToStrict(t *testing.T) {
	t.Setenv("GLOBULAR_HEALTH_LEGACY_CLAIM_OK", "")
	v := decideVersionVerdictWithInstallTime(
		"1.2.61", "bid", "1.2.61", "bid", true, nil, 0, true,
	)
	if v.Ok {
		t.Fatalf("missing install timestamp must not unlock grace; got %+v", v)
	}
	if v.FindingID != "service.runtime_identity_unproven" {
		t.Errorf("FindingID=%q want=service.runtime_identity_unproven", v.FindingID)
	}
}

// TestDecideVersionVerdictWithInstallTime_InstalledStateUnobservable_PreservesGrace
// pins the round-5 authority-uncertainty fix to loadInstalledUnixForNode.
// When installed-state can't be read (etcd transient outage), passing
// installedAtTrusted=false MUST yield Day-0 grace rather than the strict
// FAIL the previous shape produced for every service at once.
func TestDecideVersionVerdictWithInstallTime_InstalledStateUnobservable_PreservesGrace(t *testing.T) {
	t.Setenv("GLOBULAR_HEALTH_LEGACY_CLAIM_OK", "")
	v := decideVersionVerdictWithInstallTime(
		"1.2.61", "bid", "1.2.61", "bid", true, nil, 0, false,
	)
	if !v.Ok {
		t.Fatalf("unobservable installed-state must preserve grace; got %+v", v)
	}
	if v.ProofStatus != "claim_only_day0_grace" {
		t.Errorf("ProofStatus=%q want=claim_only_day0_grace", v.ProofStatus)
	}
}

func TestDecideVersionVerdictWithInstallTime_ProofPresent_IgnoresInstallTime(t *testing.T) {
	t.Setenv("GLOBULAR_HEALTH_LEGACY_CLAIM_OK", "")
	freshInstall := time.Now().Add(-10 * time.Second).Unix()
	proof := &verifier.Verdict{ProofStatus: verifier.ProofMismatch,
		Reason: "running binary sha differs from installed",
		Findings: []verifier.Finding{{
			ID: verifier.FindingRunningBinaryHashMismatch, Severity: verifier.SeverityCritical,
		}},
	}
	v := decideVersionVerdictWithInstallTime(
		"1.2.61", "bid", "1.2.61", "bid", true, proof, freshInstall, true,
	)
	if v.Ok {
		t.Fatalf("real mismatch verdict must dominate fresh-install grace; got %+v", v)
	}
	if v.ProofStatus != "mismatch" {
		t.Errorf("ProofStatus=%q want=mismatch", v.ProofStatus)
	}
}

func TestDecideVersionVerdict_VerifierVerified_LosesToClaimMismatch(t *testing.T) {
	// Even with a "verified" verdict, a claim disagreement (e.g. controller
	// view of installed version differs from desired) must dominate. The
	// running_version_mismatch is a critical finding regardless of proof.
	t.Setenv("GLOBULAR_HEALTH_LEGACY_CLAIM_OK", "")
	proof := &verifier.Verdict{ProofStatus: verifier.ProofRuntimeVerified}
	v := decideVersionVerdict(
		"1.2.57", "bid-a", "1.1.0", "bid-b", true, proof)
	if v.Ok {
		t.Fatalf("claim mismatch must dominate even with verified verdict; got %+v", v)
	}
	if v.ProofStatus != "mismatch" || v.FindingID != "service.running_version_mismatch" {
		t.Errorf("expected claim-mismatch path; got ProofStatus=%q FindingID=%q",
			v.ProofStatus, v.FindingID)
	}
}

func TestPickFindingID_PrefersCriticalThenHigh(t *testing.T) {
	findings := []verifier.Finding{
		{ID: "degraded.thing", Severity: verifier.SeverityDegraded},
		{ID: "high.thing", Severity: verifier.SeverityHigh},
		{ID: "critical.thing", Severity: verifier.SeverityCritical},
	}
	if got := pickFindingID(findings, "fallback"); got != "critical.thing" {
		t.Errorf("pickFindingID prefers critical; got %q", got)
	}
	if got := pickFindingID(findings[:2], "fallback"); got != "high.thing" {
		t.Errorf("pickFindingID prefers high when no critical; got %q", got)
	}
	if got := pickFindingID(findings[:1], "fallback"); got != "degraded.thing" {
		t.Errorf("pickFindingID returns any present ID when no critical/high; got %q", got)
	}
	if got := pickFindingID(nil, "fallback"); got != "fallback" {
		t.Errorf("pickFindingID returns fallback on empty; got %q", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// loadVerifierVerdicts: prefix-Get parses every Verdict written by the
// cluster_doctor sweep and keys by canonical service name. Missing nodes
// or unparseable values must not poison the result.
// ─────────────────────────────────────────────────────────────────────────

func TestLoadVerifierVerdicts_PrefixGetParsesAllVerdicts(t *testing.T) {
	kv := newFakeKV()
	srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)
	srv.kv = kv

	write := func(node, svc string, v verifier.Verdict) {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if _, err := kv.Put(context.Background(),
			"/globular/verification/runtime/"+node+"/"+svc, string(b)); err != nil {
			t.Fatalf("put: %v", err)
		}
	}

	write("globule-ryzen", "dns", verifier.Verdict{ProofStatus: verifier.ProofRuntimeVerified})
	write("globule-ryzen", "rbac", verifier.Verdict{ProofStatus: verifier.ProofMismatch,
		Findings: []verifier.Finding{
			{ID: verifier.FindingRunningBinaryHashMismatch, Severity: verifier.SeverityCritical},
		}})
	// Different node — must NOT appear in ryzen's map.
	write("globule-nuc", "dns", verifier.Verdict{ProofStatus: verifier.ProofMismatch})
	// Garbage value — must be skipped without erroring out the loader.
	if _, err := kv.Put(context.Background(),
		"/globular/verification/runtime/globule-ryzen/badjson", "not-json"); err != nil {
		t.Fatalf("put garbage: %v", err)
	}

	got, trusted := srv.loadVerifierVerdicts(context.Background(), "globule-ryzen")
	if !trusted {
		t.Fatalf("expected trusted=true when etcd Get succeeds; got trusted=false")
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 verdicts for ryzen (dns, rbac); got %d: %+v", len(got), got)
	}
	if v := got["dns"]; v == nil || v.ProofStatus != verifier.ProofRuntimeVerified {
		t.Errorf("dns verdict missing or wrong: %+v", v)
	}
	if v := got["rbac"]; v == nil || v.ProofStatus != verifier.ProofMismatch {
		t.Errorf("rbac verdict missing or wrong: %+v", v)
	}
	if _, ok := got["badjson"]; ok {
		t.Errorf("garbage value should be skipped, not parsed")
	}
}

func TestLoadVerifierVerdicts_NoKV_ReturnsEmpty(t *testing.T) {
	srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)
	// Deliberately leave srv.kv nil. trusted=true because "no etcd configured"
	// is a known runtime state (test fixture, pre-bootstrap startup), not an
	// observation gap.
	got, trusted := srv.loadVerifierVerdicts(context.Background(), "globule-ryzen")
	if len(got) != 0 {
		t.Errorf("nil kv must yield empty map; got %d entries", len(got))
	}
	if !trusted {
		t.Errorf("nil kv must report trusted=true (known runtime state, not observation gap); got false")
	}
}

// ingressIsDisabled reports the gate that prevents keepalived from being
// flagged FAIL when the cluster has not yet configured ingress (Day-0
// default mode=disabled). Mirrors cluster_doctor's behaviour: only a
// confirmed "disabled" or explicit_disabled=true counts; any error or
// missing key returns false (fail-open).
func TestIngressIsDisabled(t *testing.T) {
	mk := func(value string) *server {
		kv := newFakeKV()
		srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)
		srv.kv = kv
		if value != "" {
			if _, err := kv.Put(context.Background(), "/globular/ingress/v1/spec", value); err != nil {
				t.Fatalf("put: %v", err)
			}
		}
		return srv
	}

	cases := []struct {
		name  string
		value string
		want  bool
	}{
		{"missing key fails open", "", false},
		{"mode disabled", `{"mode":"disabled","generation":1}`, true},
		{"mode DISABLED case insensitive", `{"mode":"DISABLED"}`, true},
		{"explicit_disabled true overrides mode", `{"mode":"active","explicit_disabled":true}`, true},
		{"mode active", `{"mode":"active","generation":2}`, false},
		{"malformed JSON fails open", `not json`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := mk(c.value)
			if got := srv.ingressIsDisabled(context.Background()); got != c.want {
				t.Errorf("ingressIsDisabled() = %v; want %v (value=%q)", got, c.want, c.value)
			}
		})
	}

	t.Run("nil kv fails open", func(t *testing.T) {
		srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)
		if srv.ingressIsDisabled(context.Background()) {
			t.Error("nil kv must yield false (fail-open)")
		}
	})
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
