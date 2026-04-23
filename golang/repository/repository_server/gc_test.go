package main

import (
	"context"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// ── gcEligibleStates ──────────────────────────────────────────────────────

func TestGCEligibleStates_DoesNotIncludeModeration(t *testing.T) {
	// Moderation/security states must NEVER be touched by GC.
	for _, s := range []repopb.PublishState{
		repopb.PublishState_YANKED,
		repopb.PublishState_QUARANTINED,
		repopb.PublishState_REVOKED,
		repopb.PublishState_CORRUPTED,
		repopb.PublishState_ARCHIVED, // already done
	} {
		if _, ok := gcEligibleStates[s]; ok {
			t.Errorf("state %s must NOT be in gcEligibleStates — moderation/terminal states are off-limits for GC", s)
		}
	}
}

func TestGCEligibleStates_IncludesExpectedStates(t *testing.T) {
	// GC must be able to archive PUBLISHED, DEPRECATED, VERIFIED, FAILED, ORPHANED.
	for _, s := range []repopb.PublishState{
		repopb.PublishState_PUBLISHED,
		repopb.PublishState_DEPRECATED,
		repopb.PublishState_VERIFIED,
		repopb.PublishState_FAILED,
		repopb.PublishState_ORPHANED,
	} {
		if _, ok := gcEligibleStates[s]; !ok {
			t.Errorf("state %s should be in gcEligibleStates", s)
		}
	}
}

// ── ARCHIVED state machine ────────────────────────────────────────────────

func TestArchivedState_ValidTransitionSources(t *testing.T) {
	// These states may transition to ARCHIVED.
	for _, from := range []repopb.PublishState{
		repopb.PublishState_PUBLISHED,
		repopb.PublishState_DEPRECATED,
		repopb.PublishState_VERIFIED,
		repopb.PublishState_FAILED,
		repopb.PublishState_ORPHANED,
	} {
		if !repopb.ValidStateTransition(from, repopb.PublishState_ARCHIVED) {
			t.Errorf("%s → ARCHIVED should be a valid transition", from)
		}
	}
}

func TestArchivedState_InvalidSources(t *testing.T) {
	// Moderation/terminal states may NOT transition to ARCHIVED.
	for _, from := range []repopb.PublishState{
		repopb.PublishState_YANKED,
		repopb.PublishState_QUARANTINED,
		repopb.PublishState_REVOKED,
		repopb.PublishState_CORRUPTED,
	} {
		if repopb.ValidStateTransition(from, repopb.PublishState_ARCHIVED) {
			t.Errorf("%s → ARCHIVED should be invalid (moderation state)", from)
		}
	}
}

func TestArchivedState_OnlyRevocationAllowedOut(t *testing.T) {
	// From ARCHIVED, only REVOKED is allowed (admin purge path).
	if !repopb.ValidStateTransition(repopb.PublishState_ARCHIVED, repopb.PublishState_REVOKED) {
		t.Error("ARCHIVED → REVOKED should be allowed (admin purge)")
	}
	// No other exits.
	for _, to := range []repopb.PublishState{
		repopb.PublishState_PUBLISHED,
		repopb.PublishState_DEPRECATED,
		repopb.PublishState_YANKED,
		repopb.PublishState_QUARANTINED,
		repopb.PublishState_VERIFIED,
		repopb.PublishState_ARCHIVED,
	} {
		if repopb.ValidStateTransition(repopb.PublishState_ARCHIVED, to) {
			t.Errorf("ARCHIVED → %s should be invalid (one-way state)", to)
		}
	}
}

func TestArchivedState_IsDiscoveryHidden(t *testing.T) {
	if !repopb.IsDiscoveryHidden(repopb.PublishState_ARCHIVED) {
		t.Error("ARCHIVED should be hidden from discovery")
	}
}

func TestArchivedState_IsNotDownloadBlocked(t *testing.T) {
	// ARCHIVED is hidden but download is NOT blocked — owners/admins can retrieve it.
	if repopb.IsDownloadBlocked(repopb.PublishState_ARCHIVED) {
		t.Error("ARCHIVED should NOT block downloads — owners/admins may still retrieve the binary")
	}
}

func TestArchiveUnreachableArtifacts_DuplicateDigestBypassesRetention(t *testing.T) {
	srv := newTestServer(t)
	srv.GCRetentionWindow = 3
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
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref:         ref,
		BuildNumber: 67,
		BuildId:     "019d0001-0000-7000-8000-000000000067",
		Checksum:    "sha256:same-content",
		SizeBytes:   100,
	})

	resp, err := srv.ArchiveUnreachableArtifacts(context.Background(), &repopb.ArchiveUnreachableArtifactsRequest{DryRun: true})
	if err != nil {
		t.Fatalf("ArchiveUnreachableArtifacts: %v", err)
	}
	if resp.GetArchivedCount() != 1 {
		t.Fatalf("expected one duplicate archived in dry-run, got %d", resp.GetArchivedCount())
	}
	archived := resp.GetArchived()[0]
	if archived.GetKey() != artifactKeyWithBuild(ref, 1) {
		t.Fatalf("expected build 1 duplicate to be archived, got %q", archived.GetKey())
	}
	if archived.GetReason() != "duplicate_digest" {
		t.Fatalf("expected duplicate_digest reason, got %q", archived.GetReason())
	}
}
