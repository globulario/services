package rules

// package_version_authority_test.go — Unit tests for the
// repository.package_version_authority doctor rule.
//
// All tests inject desiredVersionsReader so no live etcd is required.
// Each test restores the original reader on cleanup.

import (
	"context"
	"strings"
	"testing"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// withDesiredVersions injects a fake desired-state reader for the duration
// of the test, restoring the original on cleanup.
func withDesiredVersions(t *testing.T, fn func(context.Context) map[string]desiredVersionEntry) {
	t.Helper()
	prev := desiredVersionsReader
	desiredVersionsReader = fn
	t.Cleanup(func() { desiredVersionsReader = prev })
}

// ─────────────────────────────────────────────────────────────────────────
// Degraded-mode: nil RepositoryVersionIndex → no findings (false positive
// suppression when the collector lost its repository client).
// ─────────────────────────────────────────────────────────────────────────

func TestPVA_NilVersionIndex_NoFinding(t *testing.T) {
	withDesiredVersions(t, func(context.Context) map[string]desiredVersionEntry {
		return map[string]desiredVersionEntry{
			"/globular/resources/DesiredService/dns": {Name: "dns", Version: "1.2.44"},
		}
	})

	snap := &collector.Snapshot{
		// RepositoryVersionIndex nil → collector had no signal
	}
	findings := (packageVersionAuthority{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("nil RepositoryVersionIndex must produce no findings; got %+v", findings)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Empty desired state → no findings.
// ─────────────────────────────────────────────────────────────────────────

func TestPVA_EmptyDesired_NoFinding(t *testing.T) {
	withDesiredVersions(t, func(context.Context) map[string]desiredVersionEntry { return nil })

	snap := &collector.Snapshot{
		RepositoryVersionIndex: map[string]map[string]bool{
			"dns": {"1.2.44": true},
		},
	}
	findings := (packageVersionAuthority{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("with no desired-state, rule must emit no findings; got %+v", findings)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Happy path: desired version is in the repository → no finding.
// ─────────────────────────────────────────────────────────────────────────

func TestPVA_DesiredVersionPresent_NoFinding(t *testing.T) {
	withDesiredVersions(t, func(context.Context) map[string]desiredVersionEntry {
		return map[string]desiredVersionEntry{
			"/globular/resources/DesiredService/dns": {Name: "dns", Version: "1.2.44"},
		}
	})

	snap := &collector.Snapshot{
		RepositoryVersionIndex: map[string]map[string]bool{
			"dns": {"1.2.44": true, "1.2.43": true},
		},
	}
	findings := (packageVersionAuthority{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("desired version present in repository: expected no finding, got %+v", findings)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Core violation: repository knows the package but not the desired version.
// This is the platform_release stamp scenario (e.g. storage desired 1.2.52
// but only 1.2.43 was ever built).
// ─────────────────────────────────────────────────────────────────────────

func TestPVA_DesiredVersionMissing_FindingFires(t *testing.T) {
	withDesiredVersions(t, func(context.Context) map[string]desiredVersionEntry {
		return map[string]desiredVersionEntry{
			"/globular/resources/DesiredService/storage": {Name: "storage", Version: "1.2.52"},
		}
	})

	snap := &collector.Snapshot{
		RepositoryVersionIndex: map[string]map[string]bool{
			// Repository has storage, but only 1.2.43 — never built 1.2.52.
			"storage": {"1.2.43": true},
		},
	}
	findings := (packageVersionAuthority{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected exactly 1 finding for missing version, got %d: %+v", len(findings), findings)
	}
	f := findings[0]
	if f.InvariantID != "repository.package_version_authority" {
		t.Errorf("wrong invariant_id: %q", f.InvariantID)
	}
	if f.Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Errorf("expected CRITICAL severity, got %v", f.Severity)
	}
	if !strings.Contains(f.Summary, "VersionAuthorityViolation") {
		t.Errorf("expected summary to carry VersionAuthorityViolation, got %q", f.Summary)
	}
	if !strings.Contains(f.Summary, "storage") {
		t.Errorf("expected summary to mention package name, got %q", f.Summary)
	}
	if !strings.Contains(f.Summary, "1.2.52") {
		t.Errorf("expected summary to mention desired version, got %q", f.Summary)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Package never published → NO finding.
//
// If the repository has zero entries for the package, this is a separate
// concern (undeployed service, endpoint_missing). The version authority
// rule must stay silent — it only fires when the repo knows the package
// but doesn't have the specific version.
// ─────────────────────────────────────────────────────────────────────────

func TestPVA_PackageNeverPublished_NoFinding(t *testing.T) {
	withDesiredVersions(t, func(context.Context) map[string]desiredVersionEntry {
		return map[string]desiredVersionEntry{
			"/globular/resources/DesiredService/newservice": {Name: "newservice", Version: "1.0.0"},
		}
	})

	snap := &collector.Snapshot{
		RepositoryVersionIndex: map[string]map[string]bool{
			// "newservice" is completely absent — never published any version.
			"dns": {"1.2.44": true},
		},
	}
	findings := (packageVersionAuthority{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("package never published must not trigger version authority finding; got %+v", findings)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Multiple packages: one OK, one violating → exactly one finding.
// ─────────────────────────────────────────────────────────────────────────

func TestPVA_MultiplePackages_OnlyViolatingFires(t *testing.T) {
	withDesiredVersions(t, func(context.Context) map[string]desiredVersionEntry {
		return map[string]desiredVersionEntry{
			"/globular/resources/DesiredService/dns":     {Name: "dns", Version: "1.2.44"},
			"/globular/resources/DesiredService/storage": {Name: "storage", Version: "1.2.52"},
		}
	})

	snap := &collector.Snapshot{
		RepositoryVersionIndex: map[string]map[string]bool{
			"dns":     {"1.2.44": true},    // dns OK
			"storage": {"1.2.43": true},    // storage missing 1.2.52
		},
	}
	findings := (packageVersionAuthority{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected exactly 1 finding (storage only), got %d: %+v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Summary, "storage") {
		t.Errorf("finding should be for storage, got: %q", findings[0].Summary)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Remediation steps are present and well-formed.
// ─────────────────────────────────────────────────────────────────────────

func TestPVA_Finding_HasRemediationSteps(t *testing.T) {
	withDesiredVersions(t, func(context.Context) map[string]desiredVersionEntry {
		return map[string]desiredVersionEntry{
			"/globular/resources/DesiredService/dns": {Name: "dns", Version: "9.9.9"},
		}
	})

	snap := &collector.Snapshot{
		RepositoryVersionIndex: map[string]map[string]bool{
			"dns": {"1.2.44": true},
		},
	}
	findings := (packageVersionAuthority{}).Evaluate(snap, testConfig())
	if len(findings) == 0 {
		t.Fatal("expected a finding")
	}
	f := findings[0]
	if len(f.Remediation) == 0 {
		t.Error("finding must carry at least one remediation step")
	}
	if len(f.Evidence) == 0 {
		t.Error("finding must carry at least one evidence entry")
	}
}
