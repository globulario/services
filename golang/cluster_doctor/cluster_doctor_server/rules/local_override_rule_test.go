package rules

// local_override_rule_test.go — Unit tests for the local/official package
// identity lane doctor rules.
//
// Tests cover:
//   localOverrideActive:
//     1. No local versions → no findings
//     2. One local version suffix → WARN finding fires with correct remediation
//     3. Multiple packages, only one local → one finding
//     4. Nil snapshot → no panic, no findings
//
//   officialIdentitySealed:
//     5. No matching findings → silent
//     6. Official publisher checksum mismatch → ERROR finding with remediation
//     7. Non-official publisher mismatch → silent (not our concern)

import (
	"strings"
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// ── localOverrideActive ───────────────────────────────────────────────────────

func TestLocalOverrideActive_NoLocalVersions_NoFinding(t *testing.T) {
	snap := &collector.Snapshot{
		RepositoryVersionIndex: map[string]map[string]bool{
			"storage": {"1.2.43": true, "1.2.44": true},
			"dns":     {"1.2.10": true},
		},
	}
	findings := (localOverrideActive{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("expected no findings for official-only versions, got %d: %+v", len(findings), findings)
	}
}

func TestLocalOverrideActive_LocalVersion_WarnFires(t *testing.T) {
	snap := &collector.Snapshot{
		RepositoryVersionIndex: map[string]map[string]bool{
			"storage": {"1.2.43+local.ryzen.1": true, "1.2.43": true},
		},
	}
	findings := (localOverrideActive{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for local version, got %d: %+v", len(findings), findings)
	}
	f := findings[0]
	if f.Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Errorf("expected WARN severity, got %v", f.Severity)
	}
	if f.InvariantID != "package.local_override_active" {
		t.Errorf("wrong invariant_id: %q", f.InvariantID)
	}
	if !strings.Contains(f.Summary, "storage") {
		t.Errorf("summary should mention package name, got %q", f.Summary)
	}
	if !strings.Contains(f.Summary, "1.2.43+local.ryzen.1") {
		t.Errorf("summary should mention version, got %q", f.Summary)
	}
	if len(f.Remediation) < 2 {
		t.Errorf("expected at least 2 remediation steps, got %d", len(f.Remediation))
	}
	// Remediation must mention both promote-local and override-remove paths.
	allSteps := ""
	for _, s := range f.Remediation {
		allSteps += s.GetCliCommand() + s.GetDescription()
	}
	if !strings.Contains(allSteps, "pkg override remove") {
		t.Errorf("remediation must mention 'pkg override remove', got: %s", allSteps)
	}
}

func TestLocalOverrideActive_MultiPackage_OnlyLocalFires(t *testing.T) {
	snap := &collector.Snapshot{
		RepositoryVersionIndex: map[string]map[string]bool{
			"storage": {"1.2.43": true},                          // clean
			"dns":     {"1.2.10-dev.fix1": true, "1.2.10": true}, // local version present
			"gateway": {"2.0.1": true},                           // clean
		},
	}
	findings := (localOverrideActive{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding (dns only), got %d: %+v", len(findings), findings)
	}
	if !strings.Contains(findings[0].EntityRef, "dns") {
		t.Errorf("finding should reference dns, got: %q", findings[0].EntityRef)
	}
}

func TestLocalOverrideActive_NilSnapshot_NoPanic(t *testing.T) {
	findings := (localOverrideActive{}).Evaluate(nil, testConfig())
	if len(findings) != 0 {
		t.Errorf("nil snapshot must produce no findings, got %d", len(findings))
	}
}

// ── publisherNamespaceCollision ───────────────────────────────────────────────

func TestPublisherNamespaceCollision_CoreAndLocalSamePackage_WarnFires(t *testing.T) {
	snap := &collector.Snapshot{
		RepositoryPublisherIndex: map[string]map[string]map[string]bool{
			"gateway": {
				"core@globular.io":    {"1.2.257": true},
				"local@globule-ryzen": {"1.2.257": true},
			},
		},
	}
	findings := (publisherNamespaceCollision{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 publisher collision finding, got %d: %+v", len(findings), findings)
	}
	f := findings[0]
	if f.InvariantID != "package.publisher_namespace_collision" {
		t.Fatalf("wrong invariant_id: %q", f.InvariantID)
	}
	if f.Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Fatalf("expected WARN severity, got %v", f.Severity)
	}
	if !strings.Contains(f.Summary, "core@globular.io") || !strings.Contains(f.Summary, "local@globule-ryzen") {
		t.Fatalf("summary must name both publishers, got %q", f.Summary)
	}
	if f.InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_FAIL {
		t.Fatalf("expected failing invariant status, got %v", f.InvariantStatus)
	}
}

func TestPublisherNamespaceCollision_ThirdPartyOnly_NoFinding(t *testing.T) {
	snap := &collector.Snapshot{
		RepositoryPublisherIndex: map[string]map[string]map[string]bool{
			"demo": {
				"team-a@example.com": {"1.0.0": true},
				"team-b@example.com": {"1.0.0": true},
			},
		},
	}
	findings := (publisherNamespaceCollision{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Fatalf("third-party publisher reuse should stay silent, got %+v", findings)
	}
}

func TestPublisherNamespaceCollision_NoRepositorySignal_NoFinding(t *testing.T) {
	findings := (publisherNamespaceCollision{}).Evaluate(&collector.Snapshot{}, testConfig())
	if len(findings) != 0 {
		t.Fatalf("nil repository publisher index must produce no findings, got %+v", findings)
	}
}

// ── officialIdentitySealed ────────────────────────────────────────────────────

func TestOfficialIdentitySealed_NoFindings_Silent(t *testing.T) {
	snap := &collector.Snapshot{
		RepositoryFindings: nil,
	}
	findings := (officialIdentitySealed{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("no repo findings → no doctor findings, got %d", len(findings))
	}
}

func TestOfficialIdentitySealed_OfficialChecksumMismatch_ErrorFires(t *testing.T) {
	snap := &collector.Snapshot{
		RepositoryFindings: []*collector.RepositoryFindingSnapshot{
			{
				Kind:          "REPO_FIND_PUBLISHED_CHECKSUM_MISMATCH",
				PublisherID:   "core@globular.io",
				Name:          "storage",
				Version:       "1.2.43",
				Platform:      "linux_amd64",
				CurrentState:  "PUBLISHED",
				ExpectedState: "PUBLISHED",
				Reason:        "digest mismatch",
			},
		},
	}
	findings := (officialIdentitySealed{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for official checksum mismatch, got %d", len(findings))
	}
	f := findings[0]
	if f.Severity != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Errorf("expected ERROR severity, got %v", f.Severity)
	}
	if f.InvariantID != "package.official_identity_sealed" {
		t.Errorf("wrong invariant_id: %q", f.InvariantID)
	}
	if !strings.Contains(f.Summary, "SEALED") {
		t.Errorf("summary should contain 'SEALED', got %q", f.Summary)
	}
	if len(f.Remediation) < 2 {
		t.Errorf("expected at least 2 remediation steps, got %d", len(f.Remediation))
	}
}

func TestOfficialIdentitySealed_NonOfficialMismatch_Silent(t *testing.T) {
	snap := &collector.Snapshot{
		RepositoryFindings: []*collector.RepositoryFindingSnapshot{
			{
				Kind:        "REPO_FIND_PUBLISHED_CHECKSUM_MISMATCH",
				PublisherID: "local@ryzen", // not core@globular.io
				Name:        "storage",
				Version:     "1.2.43+local.ryzen.1",
			},
		},
	}
	findings := (officialIdentitySealed{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("non-official publisher mismatch must not trigger identity seal finding, got %d", len(findings))
	}
}

// ── localOverrideStale ────────────────────────────────────────────────────────

func TestLocalOverrideStale_NoActiveOverrides_NoFinding(t *testing.T) {
	snap := &collector.Snapshot{
		ActiveLocalOverrides:   map[string]*cluster_controllerpb.LocalOverride{},
		RepositoryBuildIDIndex: map[string]bool{"some-build-id": true},
	}
	findings := (localOverrideStale{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("expected no findings for empty overrides, got %d", len(findings))
	}
}

func TestLocalOverrideStale_NilSnapshot_NoPanic(t *testing.T) {
	findings := (localOverrideStale{}).Evaluate(nil, testConfig())
	if len(findings) != 0 {
		t.Errorf("nil snapshot must produce no findings, got %d", len(findings))
	}
}

func TestLocalOverrideStale_LocalBuildMissing_WarnFires(t *testing.T) {
	snap := &collector.Snapshot{
		ActiveLocalOverrides: map[string]*cluster_controllerpb.LocalOverride{
			"storage": {
				ServiceName:    "storage",
				PublisherID:    "local@ryzen",
				Version:        "1.2.43+local.ryzen.1",
				BuildID:        "missing-build-id-001",
				BasedOnVersion: "1.2.43",
				CreatedAtUnixS: time.Now().Unix(),
			},
		},
		// RepositoryBuildIDIndex does NOT contain missing-build-id-001
		RepositoryBuildIDIndex: map[string]bool{"other-build-id": true},
	}
	findings := (localOverrideStale{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 stale finding, got %d: %+v", len(findings), findings)
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Errorf("expected WARN severity, got %v", findings[0].Severity)
	}
	if !strings.Contains(findings[0].Summary, "no longer resolvable") {
		t.Errorf("summary should mention resolvability, got: %q", findings[0].Summary)
	}
}

func TestLocalOverrideStale_BOMMovedToNewerVersion_WarnFires(t *testing.T) {
	snap := &collector.Snapshot{
		ActiveLocalOverrides: map[string]*cluster_controllerpb.LocalOverride{
			"storage": {
				ServiceName:    "storage",
				Version:        "1.2.43+local.ryzen.1",
				BuildID:        "local-build-01",
				BasedOnVersion: "1.2.43",
				CreatedAtUnixS: time.Now().Unix(),
				OfficialSnapshot: &cluster_controllerpb.LocalOverrideSnapshot{
					Version: "1.2.43",
				},
			},
		},
		// Repository now has 1.2.44 (newer than based_on 1.2.43)
		RepositoryVersionIndex: map[string]map[string]bool{
			"storage": {"1.2.44": true, "1.2.43+local.ryzen.1": true},
		},
		RepositoryBuildIDIndex: map[string]bool{"local-build-01": true},
	}
	findings := (localOverrideStale{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 stale finding for BOM drift, got %d: %+v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Summary, "BOM has moved") {
		t.Errorf("summary should mention BOM drift, got: %q", findings[0].Summary)
	}
}

func TestLocalOverrideStale_NodesBuildIDDisagree_WarnFires(t *testing.T) {
	snap := &collector.Snapshot{
		ActiveLocalOverrides: map[string]*cluster_controllerpb.LocalOverride{
			"storage": {
				ServiceName:    "storage",
				Version:        "1.2.43+local.ryzen.1",
				BuildID:        "local-build-01",
				BasedOnVersion: "1.2.43",
				CreatedAtUnixS: time.Now().Unix(),
			},
		},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{
			"node-a": {InstalledBuildIds: map[string]string{"storage": "local-build-01"}},
			"node-b": {InstalledBuildIds: map[string]string{"storage": "local-build-99"}}, // different
		},
		RepositoryBuildIDIndex: map[string]bool{"local-build-01": true},
	}
	findings := (localOverrideStale{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for node disagreement, got %d: %+v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Summary, "disagree") {
		t.Errorf("summary should mention disagreement, got: %q", findings[0].Summary)
	}
}

func TestLocalOverrideStale_OverrideExpired_WarnFires(t *testing.T) {
	oldTimestamp := time.Now().Add(-8 * 24 * time.Hour).Unix() // 8 days ago > 7-day threshold
	snap := &collector.Snapshot{
		ActiveLocalOverrides: map[string]*cluster_controllerpb.LocalOverride{
			"storage": {
				ServiceName:    "storage",
				Version:        "1.2.43+local.ryzen.1",
				BuildID:        "local-build-01",
				BasedOnVersion: "1.2.43",
				CreatedAtUnixS: oldTimestamp,
			},
		},
		RepositoryBuildIDIndex: map[string]bool{"local-build-01": true},
	}
	findings := (localOverrideStale{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for expired override, got %d: %+v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Summary, "days old") {
		t.Errorf("summary should mention age, got: %q", findings[0].Summary)
	}
}

func TestLocalOverrideStale_CleanOverride_NoFinding(t *testing.T) {
	snap := &collector.Snapshot{
		ActiveLocalOverrides: map[string]*cluster_controllerpb.LocalOverride{
			"storage": {
				ServiceName:    "storage",
				Version:        "1.2.43+local.ryzen.1",
				BuildID:        "local-build-01",
				BasedOnVersion: "1.2.43",
				CreatedAtUnixS: time.Now().Add(-1 * time.Hour).Unix(), // fresh
				OfficialSnapshot: &cluster_controllerpb.LocalOverrideSnapshot{
					Version: "1.2.43",
				},
			},
		},
		RepositoryBuildIDIndex: map[string]bool{"local-build-01": true},
		RepositoryVersionIndex: map[string]map[string]bool{
			"storage": {"1.2.43": true, "1.2.43+local.ryzen.1": true}, // no newer official
		},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{
			"node-a": {InstalledBuildIds: map[string]string{"storage": "local-build-01"}},
			"node-b": {InstalledBuildIds: map[string]string{"storage": "local-build-01"}}, // agree
		},
	}
	findings := (localOverrideStale{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("clean override must produce no findings, got %d: %+v", len(findings), findings)
	}
}

// ── isLocalVersionSuffix ──────────────────────────────────────────────────────

func TestIsLocalVersionSuffix(t *testing.T) {
	cases := []struct {
		version string
		want    bool
	}{
		{"1.2.43+local.ryzen.1", true},
		{"1.2.43-dev.fix1", true},
		{"1.2.43-hotfix.cert", true},
		{"1.2.43+dev.1", true},
		{"1.2.43+hotfix.auth", true},
		{"1.2.43", false},
		{"1.2.43-rc1", false},
		{"2.0.0", false},
		{"", false},
	}
	for _, c := range cases {
		got := isLocalVersionSuffix(c.version)
		if got != c.want {
			t.Errorf("isLocalVersionSuffix(%q) = %v, want %v", c.version, got, c.want)
		}
	}
}
