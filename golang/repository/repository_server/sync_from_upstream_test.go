package main

import (
	"context"
	"strings"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

func TestProcessSyncEntrySkipsExistingDigestWithDifferentBuildNumber(t *testing.T) {
	srv := newTestServer(t)
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "workflow",
		Version:     "1.0.53",
		Platform:    "linux_amd64",
		Kind:        repopb.ArtifactKind_SERVICE,
	}
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref:         ref,
		BuildNumber: 1,
		BuildId:     "019d0001-0000-7000-8000-000000000001",
		Checksum:    "sha256:same-content",
		SizeBytes:   100,
	})

	result := srv.processSyncEntry(
		context.Background(),
		&releaseIndexEntry{
			Name:          "workflow",
			Publisher:     "core@globular.io",
			Version:       "1.0.53",
			BuildID:       "67",
			Platform:      "linux_amd64",
			PackageDigest: "sha256:same-content",
			AssetURL:      "https://example.invalid/workflow.tgz",
		},
		&repopb.UpstreamSource{Name: "test-source"},
		"v1.0.53",
		false,
		"",
	)

	if result.GetStatus() != repopb.UpstreamSyncStatus_SYNC_SKIPPED {
		t.Fatalf("expected SYNC_SKIPPED, got %s: %s", result.GetStatus().String(), result.GetDetail())
	}
	if result.GetDetail() == "" {
		t.Fatal("expected detail explaining the existing artifact")
	}
}

// ── Policy rejection tests ──────────────────────────────────────────────────

func TestCheckImportPolicy_AllowedPublishers(t *testing.T) {
	entry := &releaseIndexEntry{Name: "echo", Kind: "SERVICE"}
	src := &repopb.UpstreamSource{AllowedPublishers: []string{"trusted@globular.io"}}

	reason, rejected := checkImportPolicy(entry, "evil@attacker.io", src)
	if !rejected {
		t.Fatal("expected rejection for disallowed publisher")
	}
	if !strings.Contains(reason, "allowed_publishers") {
		t.Fatalf("expected allowed_publishers in reason, got: %s", reason)
	}

	// Allowed publisher should pass.
	_, rejected = checkImportPolicy(entry, "trusted@globular.io", src)
	if rejected {
		t.Fatal("trusted publisher should not be rejected")
	}
}

func TestCheckImportPolicy_AllowedKinds(t *testing.T) {
	entry := &releaseIndexEntry{Name: "echo", Kind: "APPLICATION"}
	src := &repopb.UpstreamSource{AllowedKinds: []string{"SERVICE", "INFRASTRUCTURE"}}

	reason, rejected := checkImportPolicy(entry, "pub", src)
	if !rejected {
		t.Fatal("expected rejection for disallowed kind")
	}
	if !strings.Contains(reason, "allowed_kinds") {
		t.Fatalf("expected allowed_kinds in reason, got: %s", reason)
	}

	entry.Kind = "SERVICE"
	_, rejected = checkImportPolicy(entry, "pub", src)
	if rejected {
		t.Fatal("SERVICE kind should be allowed")
	}
}

func TestCheckImportPolicy_AllowedChannels(t *testing.T) {
	entry := &releaseIndexEntry{Name: "echo", Kind: "SERVICE"}
	src := &repopb.UpstreamSource{
		Channel:         "dev",
		AllowedChannels: []string{"stable"},
	}

	reason, rejected := checkImportPolicy(entry, "pub", src)
	if !rejected {
		t.Fatal("expected rejection for disallowed channel")
	}
	if !strings.Contains(reason, "allowed_channels") {
		t.Fatalf("expected allowed_channels in reason, got: %s", reason)
	}
}

func TestCheckImportPolicy_RequireChecksum(t *testing.T) {
	entry := &releaseIndexEntry{Name: "echo", Kind: "SERVICE", PackageDigest: ""}
	src := &repopb.UpstreamSource{RequireChecksum: true}

	reason, rejected := checkImportPolicy(entry, "pub", src)
	if !rejected {
		t.Fatal("expected rejection for missing checksum")
	}
	if !strings.Contains(reason, "require_checksum") {
		t.Fatalf("expected require_checksum in reason, got: %s", reason)
	}

	// With checksum present, should pass.
	entry.PackageDigest = "sha256:abc123"
	_, rejected = checkImportPolicy(entry, "pub", src)
	if rejected {
		t.Fatal("should not reject when checksum is present")
	}
}

func TestCheckImportPolicy_NoRestrictions(t *testing.T) {
	entry := &releaseIndexEntry{Name: "echo", Kind: "SERVICE"}
	src := &repopb.UpstreamSource{} // no policy fields set

	_, rejected := checkImportPolicy(entry, "any-publisher", src)
	if rejected {
		t.Fatal("unrestricted source should not reject anything")
	}
}

// ── Deterministic build ID tests ────────────────────────────────────────────

func TestDeriveUpstreamBuildID_Deterministic(t *testing.T) {
	id1 := deriveUpstreamBuildID("core@globular.io", "echo", "1.0.0", "linux_amd64", "sha256:abc")
	id2 := deriveUpstreamBuildID("core@globular.io", "echo", "1.0.0", "linux_amd64", "sha256:abc")
	if id1 != id2 {
		t.Fatalf("expected deterministic build_id, got %q and %q", id1, id2)
	}
	if !strings.HasPrefix(id1, "upstream:") {
		t.Fatalf("expected upstream: prefix, got %q", id1)
	}
}

func TestDeriveUpstreamBuildID_DifferentInputs(t *testing.T) {
	id1 := deriveUpstreamBuildID("core@globular.io", "echo", "1.0.0", "linux_amd64", "sha256:abc")
	id2 := deriveUpstreamBuildID("core@globular.io", "rbac", "1.0.0", "linux_amd64", "sha256:abc")
	if id1 == id2 {
		t.Fatal("different names should produce different build IDs")
	}

	id3 := deriveUpstreamBuildID("core@globular.io", "echo", "1.0.0", "linux_amd64", "sha256:def")
	if id1 == id3 {
		t.Fatal("different digests should produce different build IDs")
	}
}

// ── Quarantine trust policy tests ───────────────────────────────────────────

func TestImportTargetState_Default(t *testing.T) {
	src := &repopb.UpstreamSource{}
	if s := importTargetState(src); s != repopb.PublishState_PUBLISHED {
		t.Fatalf("expected PUBLISHED, got %s", s)
	}
}

func TestImportTargetState_Import(t *testing.T) {
	src := &repopb.UpstreamSource{TrustPolicy: "import"}
	if s := importTargetState(src); s != repopb.PublishState_PUBLISHED {
		t.Fatalf("expected PUBLISHED, got %s", s)
	}
}

func TestImportTargetState_Quarantine(t *testing.T) {
	src := &repopb.UpstreamSource{TrustPolicy: "quarantine"}
	if s := importTargetState(src); s != repopb.PublishState_QUARANTINED {
		t.Fatalf("expected QUARANTINED, got %s", s)
	}
}

func TestImportTargetState_QuarantineCaseInsensitive(t *testing.T) {
	src := &repopb.UpstreamSource{TrustPolicy: "QUARANTINE"}
	if s := importTargetState(src); s != repopb.PublishState_QUARANTINED {
		t.Fatalf("expected QUARANTINED, got %s", s)
	}
}

// ── Credential redaction tests ──────────────────────────────────────────────

func TestResolveCredentialFromEtcd_RejectsUnsafePrefix(t *testing.T) {
	_, err := resolveCredentialFromEtcd(context.Background(), "/etc/passwd")
	if err == nil {
		t.Fatal("expected error for unsafe prefix")
	}
	if !strings.Contains(err.Error(), "/globular/credentials/") {
		t.Fatalf("expected prefix error, got: %v", err)
	}
}

func TestResolveCredentialFromEtcd_RejectsArbitraryEtcdKey(t *testing.T) {
	_, err := resolveCredentialFromEtcd(context.Background(), "/globular/system/config")
	if err == nil {
		t.Fatal("expected error for key outside credentials prefix")
	}
}

// ── ContainsFold tests ──────────────────────────────────────────────────────

func TestContainsFold(t *testing.T) {
	if !containsFold([]string{"SERVICE", "INFRASTRUCTURE"}, "service") {
		t.Fatal("expected case-insensitive match")
	}
	if containsFold([]string{"SERVICE"}, "APPLICATION") {
		t.Fatal("should not match different value")
	}
	if containsFold(nil, "anything") {
		t.Fatal("nil slice should not match")
	}
}
