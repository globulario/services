package main

import (
	"context"
	"strings"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// Tests for the pure-function parts of DescribePackage: key parsing,
// name matching, desired-state JSON unmarshal, and the version sort.
// These run without etcd / MinIO / a live server, so they lock in
// the contract even when the full test suite is not exercised.

func TestExtractNodeIDFromKey(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"/globular/nodes/eb9a2dac-05b0-52ac-9002-99d8ffd35902/packages/SERVICE/cluster-controller", "eb9a2dac-05b0-52ac-9002-99d8ffd35902"},
		{"/globular/nodes/node-1/packages/INFRASTRUCTURE/etcd", "node-1"},
		{"/globular/nodes/n/packages/COMMAND/ffmpeg", "n"},
		// Malformed / missing prefix → empty.
		{"/globular/resources/foo", ""},
		{"nodes/foo/packages/bar", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := extractNodeIDFromKey(tc.in); got != tc.want {
			t.Errorf("extractNodeIDFromKey(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestExtractInfraPublisher(t *testing.T) {
	cases := []struct {
		key, name, want string
	}{
		{"/globular/resources/InfrastructureRelease/core@globular.io/etcd", "etcd", "core@globular.io"},
		{"/globular/resources/InfrastructureRelease/acme/minio", "minio", "acme"},
		// Nested publisher path (infrastructure releases shouldn't nest,
		// but the parser must still not crash).
		{"/globular/resources/InfrastructureRelease/pub/a/b/scylla", "scylla", "pub/a/b"},
		// Wrong prefix → empty.
		{"/foo/bar/etcd", "etcd", ""},
	}
	for _, tc := range cases {
		if got := extractInfraPublisher(tc.key, tc.name); got != tc.want {
			t.Errorf("extractInfraPublisher(%q, %q) = %q, want %q", tc.key, tc.name, got, tc.want)
		}
	}
}

func TestNameMatchesAny(t *testing.T) {
	cands := []string{"claude", "claude_svc"}
	cases := []struct {
		have string
		want bool
	}{
		{"claude", true},
		{"CLAUDE", true}, // case-insensitive
		{"claude_svc", true},
		{"claude-svc", false}, // not in candidates
		{"other", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := nameMatchesAny(tc.have, cands); got != tc.want {
			t.Errorf("nameMatchesAny(%q) = %v, want %v", tc.have, got, tc.want)
		}
	}
}

func TestParseDesiredServiceVersion(t *testing.T) {
	body := []byte(`{
		"meta": {"generation": 42},
		"spec": {"service_name": "cluster-controller", "version": "1.2.3"}
	}`)
	got := parseDesiredServiceVersion(body)
	if got == nil {
		t.Fatal("expected desired, got nil")
	}
	if got.GetVersion() != "1.2.3" || got.GetGeneration() != 42 || !got.GetPresent() {
		t.Errorf("parseDesiredServiceVersion: %+v", got)
	}

	// Missing version → nil (document exists but is incomplete; treat as absent).
	nv := []byte(`{"meta":{"generation":1},"spec":{"service_name":"x"}}`)
	if got := parseDesiredServiceVersion(nv); got != nil {
		t.Errorf("empty version should return nil, got %+v", got)
	}
	// Malformed JSON → nil.
	if got := parseDesiredServiceVersion([]byte("not json")); got != nil {
		t.Errorf("malformed JSON should return nil, got %+v", got)
	}
}

func TestParseDesiredInfraRelease(t *testing.T) {
	body := []byte(`{"meta":{"generation":7},"spec":{"version":"0.5.0"}}`)
	got := parseDesiredInfraRelease(body, "core@globular.io")
	if got == nil {
		t.Fatal("expected desired, got nil")
	}
	if got.GetVersion() != "0.5.0" || got.GetGeneration() != 7 {
		t.Errorf("parseDesiredInfraRelease: version/generation wrong: %+v", got)
	}
	if got.GetPublisher() != "core@globular.io" {
		t.Errorf("publisher not propagated: %q", got.GetPublisher())
	}
	if !got.GetPresent() {
		t.Errorf("present should be true when version is set")
	}
}

func TestSortSemverDesc(t *testing.T) {
	cases := []struct {
		in, want []string
	}{
		{[]string{"1.0.0", "2.0.0", "0.9.0"}, []string{"2.0.0", "1.0.0", "0.9.0"}},
		{[]string{"1.2.3", "1.2.10", "1.2.4"}, []string{"1.2.10", "1.2.4", "1.2.3"}}, // numeric, not lex
		{[]string{""}, []string{""}},
		{[]string{}, []string{}},
	}
	for _, tc := range cases {
		cp := append([]string{}, tc.in...)
		sortSemverDesc(cp)
		if !slicesEqual(cp, tc.want) {
			t.Errorf("sortSemverDesc(%v) = %v, want %v", tc.in, cp, tc.want)
		}
	}
}

func TestCompareVersionsNumeric(t *testing.T) {
	// The comparator MUST be numeric (1.2.10 > 1.2.9) not lexicographic.
	// A regression here would break "latest version" detection.
	if compareVersions("1.2.10", "1.2.9") <= 0 {
		t.Errorf("1.2.10 should be > 1.2.9 (numeric compare)")
	}
	if compareVersions("2.0.0", "1.99.99") <= 0 {
		t.Errorf("2.0.0 should be > 1.99.99")
	}
	if compareVersions("1.0", "1.0.0") != 0 {
		t.Errorf("1.0 should equal 1.0.0 (trailing zero)")
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestWalkCatalogForKindNormalization is a regression guard for the xds/gateway
// pre-v1.2.7 mismatch. Old artifacts had kind=SERVICE in the manifest; walkCatalogFor
// must report storedKind=SERVICE and effectiveKind=INFRASTRUCTURE so DescribePackage
// can surface the operator-visible warning.
func TestWalkCatalogForKindNormalization(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	// Seed xds with the old (wrong) SERVICE kind — simulates pre-v1.2.7 artifacts.
	seedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: &repopb.ArtifactRef{
			Name:        "xds",
			Version:     "1.0.0",
			PublisherId: "core@globular.io",
			Kind:        repopb.ArtifactKind_SERVICE,
		},
	})

	cat := srv.walkCatalogFor(ctx, []string{"xds"}, "")

	if cat.storedKind != repopb.ArtifactKind_SERVICE {
		t.Errorf("storedKind = %v, want SERVICE", cat.storedKind)
	}
	if cat.kind != repopb.ArtifactKind_INFRASTRUCTURE {
		t.Errorf("effectiveKind = %v, want INFRASTRUCTURE", cat.kind)
	}
}

// TestWalkCatalogForNoNormalizationForCorrectArtifact verifies that a correctly
// published artifact (kind already=INFRASTRUCTURE) does not trigger normalization.
func TestWalkCatalogForNoNormalizationForCorrectArtifact(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	seedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: &repopb.ArtifactRef{
			Name:        "xds",
			Version:     "1.2.7",
			PublisherId: "core@globular.io",
			Kind:        repopb.ArtifactKind_INFRASTRUCTURE,
		},
	})

	cat := srv.walkCatalogFor(ctx, []string{"xds"}, "")

	if cat.kind != repopb.ArtifactKind_INFRASTRUCTURE {
		t.Errorf("effectiveKind = %v, want INFRASTRUCTURE", cat.kind)
	}
	// storedKind should equal effectiveKind — no normalization occurred.
	if cat.storedKind != cat.kind {
		t.Errorf("storedKind %v != effectiveKind %v for correctly-published artifact", cat.storedKind, cat.kind)
	}
}

// TestBuildSourceKindNormalized verifies that buildSource encodes the
// kind normalization marker when stored and effective differ.
func TestBuildSourceKindNormalized(t *testing.T) {
	src := buildSource(repopb.ArtifactKind_SERVICE, repopb.ArtifactKind_INFRASTRUCTURE)
	if !strings.Contains(src, "; kind-normalized: ") {
		t.Errorf("expected kind-normalized marker, got %q", src)
	}
	if !strings.Contains(src, "SERVICE") || !strings.Contains(src, "INFRASTRUCTURE") {
		t.Errorf("expected stored and effective kind in source, got %q", src)
	}
}

// TestBuildSourceCleanWhenKindMatches verifies that buildSource returns the
// plain "live-aggregator" string when no normalization occurred.
func TestBuildSourceCleanWhenKindMatches(t *testing.T) {
	src := buildSource(repopb.ArtifactKind_INFRASTRUCTURE, repopb.ArtifactKind_INFRASTRUCTURE)
	if src != "live-aggregator" {
		t.Errorf("expected clean source, got %q", src)
	}
	// UNSPECIFIED stored kind (catalog miss) → also clean.
	src2 := buildSource(repopb.ArtifactKind_ARTIFACT_KIND_UNSPECIFIED, repopb.ArtifactKind_INFRASTRUCTURE)
	if src2 != "live-aggregator" {
		t.Errorf("expected clean source for unspecified stored, got %q", src2)
	}
}

// ── pickLatestInstallable ───────────────────────────────────────────────────
//
// Regression suite for the bug where `globular repository explain-package`
// printed a YANKED local-override version as `latest` while the resolver
// and doctor correctly excluded it. The fix routes `walkCatalogFor`'s
// latest-version selection through repopb.IsInstallableByPin via the pure
// helper pickLatestInstallable. These tests pin the contract of that
// helper. Caught live on globule-ryzen 2026-06-03 — Phase 38 cleanup.

// TestPickLatestInstallable_PublishedWins — happy path: a single PUBLISHED
// version is picked.
func TestPickLatestInstallable_PublishedWins(t *testing.T) {
	versionsDesc := []string{"1.2.149", "1.2.148", "1.2.147"}
	installable := map[string]bool{
		"1.2.149": true,
		"1.2.148": true,
		"1.2.147": true,
	}
	if got := pickLatestInstallable(versionsDesc, installable); got != "1.2.149" {
		t.Errorf("expected 1.2.149, got %q", got)
	}
}

// TestPickLatestInstallable_DeprecatedAllowed — DEPRECATED is installable
// per repopb.IsInstallableByPin, so a DEPRECATED version is a valid
// `latest`. This test asserts the helper preserves IsInstallableByPin
// semantics rather than independently re-judging deprecated.
func TestPickLatestInstallable_DeprecatedAllowed(t *testing.T) {
	versionsDesc := []string{"1.2.149", "1.2.148"}
	installableForDeprecated := repopb.IsInstallableByPin(repopb.PublishState_DEPRECATED)
	installable := map[string]bool{
		"1.2.149": installableForDeprecated, // simulate DEPRECATED
		"1.2.148": true,                     // PUBLISHED
	}
	got := pickLatestInstallable(versionsDesc, installable)
	if installableForDeprecated {
		if got != "1.2.149" {
			t.Errorf("DEPRECATED is installable-by-pin; expected 1.2.149, got %q", got)
		}
	} else {
		if got != "1.2.148" {
			t.Errorf("DEPRECATED not installable-by-pin; expected 1.2.148, got %q", got)
		}
	}
}

// TestPickLatestInstallable_YankedLocalSkipped — the original bug:
// a YANKED local-override version is highest by semver, but must not
// be picked as latest. The highest INSTALLABLE official version wins.
func TestPickLatestInstallable_YankedLocalSkipped(t *testing.T) {
	versionsDesc := []string{
		"1.2.149+local.phase38.1780502467", // YANKED — must skip
		"1.2.149",                          // PUBLISHED — winner
		"1.2.148",                          // PUBLISHED
		"1.2.131",                          // PUBLISHED
	}
	installable := map[string]bool{
		"1.2.149+local.phase38.1780502467": false,
		"1.2.149":                          true,
		"1.2.148":                          true,
		"1.2.131":                          true,
	}
	if got := pickLatestInstallable(versionsDesc, installable); got != "1.2.149" {
		t.Errorf("YANKED local must be skipped; expected 1.2.149, got %q", got)
	}
}

// TestPickLatestInstallable_OfficialOverHigherYanked — explicit version of
// the Phase 38 scenario: highest semver is YANKED, lower official PUBLISHED
// wins.
func TestPickLatestInstallable_OfficialOverHigherYanked(t *testing.T) {
	versionsDesc := []string{"2.0.0", "1.9.0"}
	installable := map[string]bool{
		"2.0.0": false, // YANKED — disqualified despite being highest
		"1.9.0": true,  // PUBLISHED
	}
	if got := pickLatestInstallable(versionsDesc, installable); got != "1.9.0" {
		t.Errorf("higher-versioned YANKED must lose to lower PUBLISHED; expected 1.9.0, got %q", got)
	}
}

// TestPickLatestInstallable_AllNonInstallable — defensive: when no version
// is installable, helper returns "". Callers should treat that as "no
// installable version" rather than fall back to a non-installable one.
func TestPickLatestInstallable_AllNonInstallable(t *testing.T) {
	versionsDesc := []string{"1.2.149", "1.2.148"}
	installable := map[string]bool{}
	if got := pickLatestInstallable(versionsDesc, installable); got != "" {
		t.Errorf("no installable version → expected empty, got %q", got)
	}
}

// TestPickLatestInstallable_EmptyInputs — guard against nil / empty edge
// cases (defensive — should never panic).
func TestPickLatestInstallable_EmptyInputs(t *testing.T) {
	if got := pickLatestInstallable(nil, nil); got != "" {
		t.Errorf("nil inputs → expected empty, got %q", got)
	}
	if got := pickLatestInstallable([]string{}, map[string]bool{}); got != "" {
		t.Errorf("empty inputs → expected empty, got %q", got)
	}
	if got := pickLatestInstallable([]string{"1.0.0"}, nil); got != "" {
		t.Errorf("nil map → expected empty, got %q", got)
	}
}

// TestPickLatestInstallable_IsInstallableByPinContract pins the
// IsInstallableByPin contract so that if it is ever broadened or narrowed,
// this test surfaces the change and forces a deliberate decision on how
// walkCatalogFor's latest selection should respond.
func TestPickLatestInstallable_IsInstallableByPinContract(t *testing.T) {
	cases := []struct {
		name       string
		state      repopb.PublishState
		expectPass bool
	}{
		{"PUBLISHED", repopb.PublishState_PUBLISHED, true},
		{"DEPRECATED", repopb.PublishState_DEPRECATED, true},
		{"YANKED", repopb.PublishState_YANKED, false},
		{"QUARANTINED", repopb.PublishState_QUARANTINED, false},
		{"REVOKED", repopb.PublishState_REVOKED, false},
		{"FAILED", repopb.PublishState_FAILED, false},
		{"ORPHANED", repopb.PublishState_ORPHANED, false},
		{"CORRUPTED", repopb.PublishState_CORRUPTED, false},
		{"ARCHIVED", repopb.PublishState_ARCHIVED, false},
		{"STAGING", repopb.PublishState_STAGING, false},
		{"VERIFIED", repopb.PublishState_VERIFIED, false},
		{"UNSPECIFIED", repopb.PublishState_PUBLISH_STATE_UNSPECIFIED, false},
	}
	for _, c := range cases {
		got := repopb.IsInstallableByPin(c.state)
		if got != c.expectPass {
			t.Errorf("%s: IsInstallableByPin = %v, want %v", c.name, got, c.expectPass)
		}
	}
}
