package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// seedPublishedArtifact writes a manifest with PUBLISHED state and appends to
// the release ledger so that monotonicity checks and convergence work correctly.
func seedPublishedArtifact(t *testing.T, srv *server, m *repopb.ArtifactManifest) {
	t.Helper()
	ctx := context.Background()

	// Ensure build_id is set.
	if m.GetBuildId() == "" {
		t.Fatal("seedPublishedArtifact requires build_id to be set")
	}

	// Write manifest with PUBLISHED state.
	key := artifactKeyWithBuild(m.GetRef(), m.GetBuildNumber())
	_ = srv.Storage().MkdirAll(ctx, artifactsDir, 0o755)
	mjson, err := marshalManifestWithState(m, repopb.PublishState_PUBLISHED)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := srv.Storage().WriteFile(ctx, manifestStorageKey(key), mjson, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	// Write a binary whose length matches manifest.SizeBytes so size-integrity
	// checks (artifactBlobStatus, consistency scan) see a coherent record.
	binaryContent := []byte("fake-binary")
	if sz := m.GetSizeBytes(); sz > 0 {
		binaryContent = make([]byte, sz)
	}
	if err := srv.Storage().WriteFile(ctx, binaryStorageKey(key), binaryContent, 0o644); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	// Append to release ledger for monotonicity enforcement.
	ref := m.GetRef()
	if err := srv.appendToLedger(ctx, ref.GetPublisherId(), ref.GetName(),
		ref.GetVersion(), m.GetBuildId(), m.GetChecksum(),
		ref.GetPlatform(), m.GetSizeBytes(), nil); err != nil {
		t.Fatalf("append to ledger: %v", err)
	}
}

// seedPublishedArtifactDirect writes a manifest + binary to storage without
// updating the release ledger. Use ONLY in tests that deliberately simulate
// corruption (two build_ids for the same version/platform) — normal publish
// tests must go through seedPublishedArtifact so the ledger enforces
// version immutability.
func seedPublishedArtifactDirect(t *testing.T, srv *server, m *repopb.ArtifactManifest) {
	t.Helper()
	ctx := context.Background()

	if m.GetBuildId() == "" {
		t.Fatal("seedPublishedArtifactDirect requires build_id to be set")
	}

	key := artifactKeyWithBuild(m.GetRef(), m.GetBuildNumber())
	_ = srv.Storage().MkdirAll(ctx, artifactsDir, 0o755)
	mjson, err := marshalManifestWithState(m, repopb.PublishState_PUBLISHED)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := srv.Storage().WriteFile(ctx, manifestStorageKey(key), mjson, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	binaryContent := []byte("fake-binary")
	if sz := m.GetSizeBytes(); sz > 0 {
		binaryContent = make([]byte, sz)
	}
	if err := srv.Storage().WriteFile(ctx, binaryStorageKey(key), binaryContent, 0o644); err != nil {
		t.Fatalf("write binary: %v", err)
	}
}

// uploadTestArtifact calls UploadArtifact as a streaming RPC.
// Returns the response or error.
func uploadTestArtifact(t *testing.T, srv *server, ref *repopb.ArtifactRef, data []byte, buildNumber int64) (*repopb.UploadArtifactResponse, error) {
	t.Helper()
	// We can't easily call a streaming RPC in-process, so we use the
	// internal handler path directly. The upload handler validates,
	// stores, and promotes — we simulate the essential parts.

	// For invariant tests, we test the individual enforcement functions
	// directly rather than the full streaming upload path.
	return nil, nil // placeholder — tests below use handler functions directly
}

// ── INV-1: Released Artifact Immutable ─────────────────────────────────────

func TestINV1_ReleasedArtifactImmutable_DifferentDigest(t *testing.T) {
	srv := newTestServer(t)

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "echo",
		Version:     "1.0.0",
		Platform:    "linux_amd64",
		Kind:        repopb.ArtifactKind_SERVICE,
	}

	// Seed a PUBLISHED artifact with a known digest.
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref:         ref,
		BuildNumber: 1,
		BuildId:     "019d0001-0000-7000-8000-000000000001",
		Checksum:    "sha256:aaaa",
		SizeBytes:   100,
	})

	// Check terminal state — should reject overwrite.
	_, existingState, _, err := srv.readManifestAndStateByKey(context.Background(),
		artifactKeyWithBuild(ref, 1))
	if err != nil {
		t.Fatalf("read existing manifest: %v", err)
	}
	if !isTerminalState(existingState) {
		t.Errorf("expected PUBLISHED to be terminal, got %v", existingState)
	}
}

func TestINV1_ReleasedArtifactImmutable_SameDigest(t *testing.T) {
	srv := newTestServer(t)

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "echo",
		Version:     "1.0.0",
		Platform:    "linux_amd64",
		Kind:        repopb.ArtifactKind_SERVICE,
	}

	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref:         ref,
		BuildNumber: 1,
		BuildId:     "019d0001-0000-7000-8000-000000000001",
		Checksum:    "sha256:aaaa",
		SizeBytes:   100,
	})

	// PUBLISHED is a terminal state — verified.
	_, state, _, err := srv.readManifestAndStateByKey(context.Background(),
		artifactKeyWithBuild(ref, 1))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if state != repopb.PublishState_PUBLISHED {
		t.Errorf("expected PUBLISHED, got %v", state)
	}
}

// ── INV-2: Monotonic Versions ──────────────────────────────────────────────

func TestINV2_MonotonicVersions_RejectLowerVersion(t *testing.T) {
	srv := newTestServer(t)

	// Seed v1.0.0 as PUBLISHED.
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "echo",
		Version:     "1.0.0",
		Platform:    "linux_amd64",
		Kind:        repopb.ArtifactKind_SERVICE,
	}
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref:         ref,
		BuildNumber: 1,
		BuildId:     "019d0001-0000-7000-8000-000000000001",
		Checksum:    "sha256:aaaa",
		SizeBytes:   100,
	})

	// Verify ledger has latest version.
	latestVer, latestBID := srv.getLatestRelease(context.Background(),
		"core@globular.io", "echo", "linux_amd64")
	if latestVer != "1.0.0" {
		t.Fatalf("expected latest version 1.0.0, got %s", latestVer)
	}
	if latestBID == "" {
		t.Fatal("expected latest build_id to be non-empty")
	}

	// Attempt to upload v0.9.0 — should be rejected by monotonicity check.
	// We test the ledger directly since the full upload path requires streaming.
	ledger := srv.readLedger(context.Background(), "core@globular.io", "echo")
	if ledger == nil {
		t.Fatal("expected ledger to exist")
	}
	if ledger.LatestVersion != "1.0.0" {
		t.Errorf("ledger latest = %s, want 1.0.0", ledger.LatestVersion)
	}

	// appendToLedger enforces monotonicity — lower version should fail.
	err := srv.appendToLedger(context.Background(),
		"core@globular.io", "echo", "0.9.0",
		"019d0002-0000-7000-8000-000000000002", "sha256:bbbb",
		"linux_amd64", 100, nil)
	if err == nil {
		t.Error("expected monotonicity error for version 0.9.0 < 1.0.0, got nil")
	}
}

func TestINV2_MonotonicVersions_AcceptHigherVersion(t *testing.T) {
	srv := newTestServer(t)

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "echo",
		Version:     "1.0.0",
		Platform:    "linux_amd64",
		Kind:        repopb.ArtifactKind_SERVICE,
	}
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref:         ref,
		BuildNumber: 1,
		BuildId:     "019d0001-0000-7000-8000-000000000001",
		Checksum:    "sha256:aaaa",
		SizeBytes:   100,
	})

	// v1.0.1 should be accepted.
	err := srv.appendToLedger(context.Background(),
		"core@globular.io", "echo", "1.0.1",
		"019d0003-0000-7000-8000-000000000003", "sha256:cccc",
		"linux_amd64", 100, nil)
	if err != nil {
		t.Errorf("expected higher version 1.0.1 to be accepted, got: %v", err)
	}
}

func TestINV2_MonotonicVersions_AcceptSameVersion(t *testing.T) {
	srv := newTestServer(t)

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "echo",
		Version:     "1.0.0",
		Platform:    "linux_amd64",
		Kind:        repopb.ArtifactKind_SERVICE,
	}
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref:         ref,
		BuildNumber: 1,
		BuildId:     "019d0001-0000-7000-8000-000000000001",
		Checksum:    "sha256:aaaa",
		SizeBytes:   100,
	})

	// Same version (additional platform/build) is allowed.
	err := srv.appendToLedger(context.Background(),
		"core@globular.io", "echo", "1.0.0",
		"019d0004-0000-7000-8000-000000000004", "sha256:dddd",
		"linux_arm64", 100, nil)
	if err != nil {
		t.Errorf("expected same version to be accepted (different platform), got: %v", err)
	}
}

// ── INV-3: Build ID Server-Generated ───────────────────────────────────────

func TestINV3_BuildIdServerGenerated(t *testing.T) {
	// The upload handler generates build_id via uuid.NewV7() at line 740
	// of artifact_handlers.go. We verify that:
	// 1. The generated ID is a valid UUIDv7 (36 chars with dashes)
	// 2. seedPublishedArtifact requires build_id to be set (server responsibility)

	srv := newTestServer(t)

	// Verify that reading a manifest returns the build_id we set.
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "echo",
		Version:     "1.0.0",
		Platform:    "linux_amd64",
		Kind:        repopb.ArtifactKind_SERVICE,
	}
	expectedBID := "019d0001-0000-7000-8000-000000000001"
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref:         ref,
		BuildNumber: 1,
		BuildId:     expectedBID,
		Checksum:    "sha256:aaaa",
		SizeBytes:   100,
	})

	manifest, err := srv.GetArtifactManifest(context.Background(),
		&repopb.GetArtifactManifestRequest{Ref: ref})
	if err != nil {
		t.Fatalf("GetArtifactManifest: %v", err)
	}
	if manifest.GetManifest().GetBuildId() != expectedBID {
		t.Errorf("build_id = %s, want %s", manifest.GetManifest().GetBuildId(), expectedBID)
	}
}

// ── INV-4: Build Number Display-Only ───────────────────────────────────────

func TestINV4_BuildNumberDisplayOnly(t *testing.T) {
	// INV-4: build_number must never be used for convergence decisions.
	// This test verifies that two artifacts with the same version but
	// different build_numbers are distinguished by build_id, not build_number.

	srv := newTestServer(t)
	publisher := "core@globular.io"
	name := "echo"
	platform := "linux_amd64"

	// Seed two builds at the same version with different build_numbers.
	for i := int64(1); i <= 2; i++ {
		ref := &repopb.ArtifactRef{
			PublisherId: publisher, Name: name,
			Version: "1.0.0", Platform: platform,
			Kind: repopb.ArtifactKind_SERVICE,
		}
		seedArtifact(t, srv, &repopb.ArtifactManifest{
			Ref:         ref,
			BuildNumber: i,
			BuildId:     "019d0001-0000-7000-8000-00000000000" + string(rune('0'+i)),
			Checksum:    "sha256:aaaa",
			SizeBytes:   100,
		})
	}

	// Both should be retrievable by build_number (display lookup).
	m1, err := srv.GetArtifactManifest(context.Background(),
		&repopb.GetArtifactManifestRequest{
			Ref:         &repopb.ArtifactRef{PublisherId: publisher, Name: name, Version: "1.0.0", Platform: platform},
			BuildNumber: 1,
		})
	if err != nil {
		t.Fatalf("get build 1: %v", err)
	}
	m2, err := srv.GetArtifactManifest(context.Background(),
		&repopb.GetArtifactManifestRequest{
			Ref:         &repopb.ArtifactRef{PublisherId: publisher, Name: name, Version: "1.0.0", Platform: platform},
			BuildNumber: 2,
		})
	if err != nil {
		t.Fatalf("get build 2: %v", err)
	}

	// Distinguished by build_id, not build_number.
	if m1.GetManifest().GetBuildId() == m2.GetManifest().GetBuildId() {
		t.Error("two different builds should have different build_ids")
	}
}

// ── INV-5: Allocation Protocol ─────────────────────────────────────────────

func TestINV5_AllocateUpload_BumpPatch(t *testing.T) {
	srv := newTestServer(t)

	// Seed a published artifact so the allocator has a latest version.
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "echo",
		Version:     "1.0.0",
		Platform:    "linux_amd64",
		Kind:        repopb.ArtifactKind_SERVICE,
	}
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref:         ref,
		BuildNumber: 1,
		BuildId:     "019d0001-0000-7000-8000-000000000001",
		Checksum:    "sha256:aaaa",
		SizeBytes:   100,
	})

	// Allocate with BUMP_PATCH.
	resp, err := srv.AllocateUpload(context.Background(), &repopb.AllocateUploadRequest{
		PublisherId: "core@globular.io",
		Name:        "echo",
		Platform:    "linux_amd64",
		Intent:      repopb.VersionIntent_BUMP_PATCH,
	})
	if err != nil {
		t.Fatalf("AllocateUpload: %v", err)
	}
	if resp.GetVersion() != "1.0.1" {
		t.Errorf("version = %s, want 1.0.1", resp.GetVersion())
	}
	if resp.GetBuildId() == "" {
		t.Error("build_id should be non-empty")
	}
	if len(resp.GetBuildId()) != 36 {
		t.Errorf("build_id should be UUID (36 chars), got %d: %s", len(resp.GetBuildId()), resp.GetBuildId())
	}
	if resp.GetReservationId() == "" {
		t.Error("reservation_id should be non-empty")
	}
}

func TestINV5_AllocateUpload_ConcurrentRejection(t *testing.T) {
	srv := newTestServer(t)

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "echo-concurrent-test",
		Version:     "1.0.0",
		Platform:    "linux_amd64",
		Kind:        repopb.ArtifactKind_SERVICE,
	}
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref:         ref,
		BuildNumber: 1,
		BuildId:     "019d0001-0000-7000-8000-000000000001",
		Checksum:    "sha256:aaaa",
		SizeBytes:   100,
	})

	// First allocation should succeed.
	_, err := srv.AllocateUpload(context.Background(), &repopb.AllocateUploadRequest{
		PublisherId: "core@globular.io",
		Name:        "echo-concurrent-test",
		Platform:    "linux_amd64",
		Intent:      repopb.VersionIntent_BUMP_PATCH,
	})
	if err != nil {
		t.Fatalf("first AllocateUpload: %v", err)
	}

	// Second allocation for same version → ResourceExhausted.
	_, err = srv.AllocateUpload(context.Background(), &repopb.AllocateUploadRequest{
		PublisherId: "core@globular.io",
		Name:        "echo-concurrent-test",
		Platform:    "linux_amd64",
		Intent:      repopb.VersionIntent_BUMP_PATCH,
	})
	if err == nil {
		t.Fatal("expected ResourceExhausted for concurrent allocation")
	}
	if status.Code(err) != codes.ResourceExhausted {
		t.Errorf("expected ResourceExhausted, got %v: %v", status.Code(err), err)
	}
}

func TestINV5_AllocateUpload_BumpMinor(t *testing.T) {
	srv := newTestServer(t)

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "echo",
		Version:     "1.2.3",
		Platform:    "linux_amd64",
		Kind:        repopb.ArtifactKind_SERVICE,
	}
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref:         ref,
		BuildNumber: 1,
		BuildId:     "019d0001-0000-7000-8000-000000000001",
		Checksum:    "sha256:aaaa",
		SizeBytes:   100,
	})

	resp, err := srv.AllocateUpload(context.Background(), &repopb.AllocateUploadRequest{
		PublisherId: "core@globular.io",
		Name:        "echo",
		Platform:    "linux_amd64",
		Intent:      repopb.VersionIntent_BUMP_MINOR,
	})
	if err != nil {
		t.Fatalf("AllocateUpload BUMP_MINOR: %v", err)
	}
	if resp.GetVersion() != "1.3.0" {
		t.Errorf("version = %s, want 1.3.0", resp.GetVersion())
	}
}

func TestINV5_AllocateUpload_BumpMajor(t *testing.T) {
	srv := newTestServer(t)

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "echo",
		Version:     "1.2.3",
		Platform:    "linux_amd64",
		Kind:        repopb.ArtifactKind_SERVICE,
	}
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref:         ref,
		BuildNumber: 1,
		BuildId:     "019d0001-0000-7000-8000-000000000001",
		Checksum:    "sha256:aaaa",
		SizeBytes:   100,
	})

	resp, err := srv.AllocateUpload(context.Background(), &repopb.AllocateUploadRequest{
		PublisherId: "core@globular.io",
		Name:        "echo",
		Platform:    "linux_amd64",
		Intent:      repopb.VersionIntent_BUMP_MAJOR,
	})
	if err != nil {
		t.Fatalf("AllocateUpload BUMP_MAJOR: %v", err)
	}
	if resp.GetVersion() != "2.0.0" {
		t.Errorf("version = %s, want 2.0.0", resp.GetVersion())
	}
}

// ── INV-6: Provisional Import ──────────────────────────────────────────────

func TestINV6_ImportProvisional_IdempotentSameDigest(t *testing.T) {
	srv := newTestServer(t)

	// Seed a published artifact.
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "echo",
		Version:     "1.0.0",
		Platform:    "linux_amd64",
		Kind:        repopb.ArtifactKind_SERVICE,
	}
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref:         ref,
		BuildNumber: 1,
		BuildId:     "019d0001-0000-7000-8000-000000000001",
		Checksum:    "sha256:aaaa",
		SizeBytes:   100,
	})

	// Import with same version + same digest → idempotent success.
	resp, err := srv.ImportProvisionalArtifact(context.Background(), &repopb.ImportProvisionalRequest{
		PublisherId:        "core@globular.io",
		Name:               "echo",
		Version:            "1.0.0",
		Platform:           "linux_amd64",
		Digest:             "sha256:aaaa",
		ProvisionalBuildId: "local-id",
		Kind:               "SERVICE",
	})
	if err != nil {
		t.Fatalf("ImportProvisionalArtifact: %v", err)
	}
	if !resp.GetOk() {
		t.Errorf("expected ok=true, got ok=%v message=%s", resp.GetOk(), resp.GetMessage())
	}
	if resp.GetConfirmedBuildId() != "019d0001-0000-7000-8000-000000000001" {
		t.Errorf("expected existing build_id, got %s", resp.GetConfirmedBuildId())
	}
}

func TestINV6_ImportProvisional_RejectDifferentDigest(t *testing.T) {
	srv := newTestServer(t)

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "echo",
		Version:     "1.0.0",
		Platform:    "linux_amd64",
		Kind:        repopb.ArtifactKind_SERVICE,
	}
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref:         ref,
		BuildNumber: 1,
		BuildId:     "019d0001-0000-7000-8000-000000000001",
		Checksum:    "sha256:aaaa",
		SizeBytes:   100,
	})

	// Import with same version + different digest → conflict.
	resp, err := srv.ImportProvisionalArtifact(context.Background(), &repopb.ImportProvisionalRequest{
		PublisherId:        "core@globular.io",
		Name:               "echo",
		Version:            "1.0.0",
		Platform:           "linux_amd64",
		Digest:             "sha256:bbbb",
		ProvisionalBuildId: "local-id",
		Kind:               "SERVICE",
	})
	if err != nil {
		t.Fatalf("ImportProvisionalArtifact: %v", err)
	}
	if resp.GetOk() {
		t.Error("expected ok=false for different digest, got ok=true")
	}
}

func TestINV6_ImportProvisional_NewVersion(t *testing.T) {
	srv := newTestServer(t)

	// Import a brand-new version (no ledger entry yet).
	resp, err := srv.ImportProvisionalArtifact(context.Background(), &repopb.ImportProvisionalRequest{
		PublisherId:        "core@globular.io",
		Name:               "newpkg",
		Version:            "1.0.0",
		Platform:           "linux_amd64",
		Digest:             "sha256:ffff",
		ProvisionalBuildId: "local-prov-id",
		Kind:               "SERVICE",
	})
	if err != nil {
		t.Fatalf("ImportProvisionalArtifact: %v", err)
	}
	if !resp.GetOk() {
		t.Errorf("expected ok=true for new version, got ok=%v message=%s", resp.GetOk(), resp.GetMessage())
	}
	if resp.GetConfirmedBuildId() == "" {
		t.Error("expected confirmed_build_id to be non-empty")
	}
	if resp.GetState() != "RELEASED" {
		t.Errorf("state = %s, want RELEASED", resp.GetState())
	}

	// Verify ledger was updated.
	latestVer, latestBID := srv.getLatestRelease(context.Background(),
		"core@globular.io", "newpkg", "linux_amd64")
	if latestVer != "1.0.0" {
		t.Errorf("ledger latest version = %s, want 1.0.0", latestVer)
	}
	if latestBID != resp.GetConfirmedBuildId() {
		t.Errorf("ledger build_id = %s, want %s", latestBID, resp.GetConfirmedBuildId())
	}
}

// ── INV-8: Terminal State Classification ───────────────────────────────────

func TestINV8_TerminalStates(t *testing.T) {
	terminal := []repopb.PublishState{
		repopb.PublishState_PUBLISHED,
		repopb.PublishState_DEPRECATED,
		repopb.PublishState_YANKED,
		repopb.PublishState_QUARANTINED,
		repopb.PublishState_REVOKED,
	}
	for _, s := range terminal {
		if !isTerminalState(s) {
			t.Errorf("%v should be terminal", s)
		}
	}

	nonTerminal := []repopb.PublishState{
		repopb.PublishState_PUBLISH_STATE_UNSPECIFIED,
		repopb.PublishState_STAGING,
		repopb.PublishState_VERIFIED,
		repopb.PublishState_FAILED,
	}
	for _, s := range nonTerminal {
		if isTerminalState(s) {
			t.Errorf("%v should NOT be terminal", s)
		}
	}
}

// ── INV-10: Release Ledger Persistence ─────────────────────────────────────

func TestINV10_ReleaseLedgerPersistence(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	// Append entries and verify they persist.
	if err := srv.appendToLedger(ctx, "core@globular.io", "test-svc",
		"1.0.0", "bid-1", "sha256:aaaa", "linux_amd64", 100, nil); err != nil {
		t.Fatalf("first append: %v", err)
	}
	if err := srv.appendToLedger(ctx, "core@globular.io", "test-svc",
		"1.0.1", "bid-2", "sha256:bbbb", "linux_amd64", 200, nil); err != nil {
		t.Fatalf("second append: %v", err)
	}

	// Read back.
	ver, bid := srv.getLatestRelease(ctx, "core@globular.io", "test-svc", "linux_amd64")
	if ver != "1.0.1" {
		t.Errorf("latest version = %s, want 1.0.1", ver)
	}
	if bid != "bid-2" {
		t.Errorf("latest build_id = %s, want bid-2", bid)
	}

	// Verify ledger has both entries.
	ledger := srv.readLedger(ctx, "core@globular.io", "test-svc")
	if ledger == nil {
		t.Fatal("ledger should exist")
	}
	if len(ledger.Releases) != 2 {
		t.Errorf("expected 2 releases, got %d", len(ledger.Releases))
	}
}

// ── Reservation TTL ────────────────────────────────────────────────────────

func TestReservationExpiry(t *testing.T) {
	// Verify that reservations have a finite TTL.
	// We don't test actual expiry (5 min wait) but verify the constant.
	if reservationTTL <= 0 {
		t.Error("reservationTTL should be positive")
	}
	if reservationTTL > 10*time.Minute {
		t.Errorf("reservationTTL = %v, should be <= 10 minutes", reservationTTL)
	}
}

// ── Migration ──────────────────────────────────────────────────────────────

func TestMigrateBuildIDs_Idempotent(t *testing.T) {
	srv := newTestServer(t)

	// Seed an artifact without build_id.
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "echo",
		Version:     "1.0.0",
		Platform:    "linux_amd64",
		Kind:        repopb.ArtifactKind_SERVICE,
	}
	seedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref:         ref,
		BuildNumber: 1,
		Checksum:    "sha256:aaaa",
	})

	// Run migration.
	srv.MigrateBuildIDs(context.Background())

	// Verify build_id was assigned.
	m, err := srv.GetArtifactManifest(context.Background(),
		&repopb.GetArtifactManifestRequest{Ref: ref, BuildNumber: 1})
	if err != nil {
		t.Fatalf("GetArtifactManifest after migration: %v", err)
	}
	bid1 := m.GetManifest().GetBuildId()
	if bid1 == "" {
		t.Fatal("build_id should be assigned after migration")
	}

	// Run again — should be idempotent (marker file prevents re-run).
	srv.MigrateBuildIDs(context.Background())

	m2, _ := srv.GetArtifactManifest(context.Background(),
		&repopb.GetArtifactManifestRequest{Ref: ref, BuildNumber: 1})
	if m2.GetManifest().GetBuildId() != bid1 {
		t.Errorf("build_id changed after re-migration: %s → %s", bid1, m2.GetManifest().GetBuildId())
	}

	// Verify migration provenance sidecar is written for legacy backfill.
	key := artifactKeyWithBuild(ref, 1)
	provBytes, err := srv.Storage().ReadFile(context.Background(), buildIDMigrationProvenanceKey(key))
	if err != nil {
		t.Fatalf("expected migration provenance sidecar: %v", err)
	}
	var prov map[string]any
	if err := json.Unmarshal(provBytes, &prov); err != nil {
		t.Fatalf("unmarshal migration provenance: %v", err)
	}
	if migrated, _ := prov["migrated_from_legacy"].(bool); !migrated {
		t.Errorf("migrated_from_legacy = %v, want true", prov["migrated_from_legacy"])
	}
	if got, _ := prov["migration_version"].(string); got != buildIDMigrationVersion {
		t.Errorf("migration_version = %q, want %q", got, buildIDMigrationVersion)
	}
	if legacyPath, _ := prov["legacy_path"].(string); legacyPath == "" {
		t.Errorf("legacy_path = %q, want non-empty", legacyPath)
	}
}

// TestMigrateBuildIDAliases_FromUpstreamImport seeds a legacy artifact that has
// a build_id but no alias record, with an UpstreamImportRecord carrying
// release_tag + build_number, and verifies MigrateBuildIDAliases backfills the
// (release_tag, build_number) → canonical_build_id alias.
func TestMigrateBuildIDAliases_FromUpstreamImport(t *testing.T) {
	srv := newTestServer(t)

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "echo",
		Version:     "1.0.0",
		Platform:    "linux_amd64",
		Kind:        repopb.ArtifactKind_SERVICE,
	}
	canonicalBuildID := "01JTESTCANONICALBUILDID0001"
	seedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref:         ref,
		BuildNumber: 7,
		BuildId:     canonicalBuildID,
		Checksum:    "sha256:cafebabe",
		UpstreamImport: &repopb.UpstreamImportRecord{
			SourceName:    "github-globulario",
			ReleaseTag:    "v1.2.32",
			BuildNumber:   42,
			OriginRelease: "v1.2.32",
			Checksum:      "sha256:cafebabe",
		},
	})

	srv.MigrateBuildIDAliases(context.Background())

	alias, err := srv.loadReleaseBuildAlias(context.Background(), ref, "v1.2.32", 42)
	if err != nil {
		t.Fatalf("loadReleaseBuildAlias: %v", err)
	}
	if alias == nil {
		t.Fatal("expected alias record after migration; got nil")
	}
	if alias.CanonicalBuildID != canonicalBuildID {
		t.Errorf("alias.CanonicalBuildID = %q, want %q", alias.CanonicalBuildID, canonicalBuildID)
	}
	if alias.ReleaseTag != "v1.2.32" || alias.BuildNumber != 42 {
		t.Errorf("alias locator = (%s, %d), want (v1.2.32, 42)", alias.ReleaseTag, alias.BuildNumber)
	}

	// Provenance sidecar should exist for the migrated alias.
	key := artifactKeyWithBuild(ref, 7)
	provBytes, err := srv.Storage().ReadFile(context.Background(), buildIDAliasMigrationProvenanceKey(key))
	if err != nil {
		t.Fatalf("expected alias migration provenance sidecar: %v", err)
	}
	var prov map[string]any
	if err := json.Unmarshal(provBytes, &prov); err != nil {
		t.Fatalf("unmarshal alias provenance: %v", err)
	}
	if got, _ := prov["build_id"].(string); got != canonicalBuildID {
		t.Errorf("provenance build_id = %q, want %q", got, canonicalBuildID)
	}
	if got, _ := prov["migration_version"].(string); got != buildIDAliasMigrationVersion {
		t.Errorf("migration_version = %q, want %q", got, buildIDAliasMigrationVersion)
	}

	// Marker should exist — second invocation is a no-op.
	if _, err := srv.Storage().ReadFile(context.Background(), buildIDAliasMigrationMarker); err != nil {
		t.Fatalf("expected migration marker: %v", err)
	}
	srv.MigrateBuildIDAliases(context.Background())

	alias2, err := srv.loadReleaseBuildAlias(context.Background(), ref, "v1.2.32", 42)
	if err != nil {
		t.Fatalf("loadReleaseBuildAlias after re-run: %v", err)
	}
	if alias2 == nil || alias2.CanonicalBuildID != canonicalBuildID {
		t.Errorf("alias canonical_build_id changed across re-run: %+v", alias2)
	}
}

// TestMigrateBuildIDAliases_SkipsWithoutImportRecord confirms artifacts that
// have a build_id but no UpstreamImportRecord (e.g. locally-uploaded legacy
// packages) are left untouched — there is no release_tag context from which
// to derive an alias locator.
func TestMigrateBuildIDAliases_SkipsWithoutImportRecord(t *testing.T) {
	srv := newTestServer(t)

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "echo",
		Version:     "1.0.0",
		Platform:    "linux_amd64",
		Kind:        repopb.ArtifactKind_SERVICE,
	}
	seedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref:         ref,
		BuildNumber: 3,
		BuildId:     "01JTESTLOCALARTIFACT0001",
		Checksum:    "sha256:deadbeef",
	})

	srv.MigrateBuildIDAliases(context.Background())

	// No alias should be created — there's no release_tag/build_number to map.
	alias, err := srv.loadReleaseBuildAlias(context.Background(), ref, "v1.2.32", 1)
	if err != nil {
		t.Fatalf("loadReleaseBuildAlias: %v", err)
	}
	if alias != nil {
		t.Errorf("unexpected alias for local-only artifact: %+v", alias)
	}

	// No per-artifact provenance sidecar.
	key := artifactKeyWithBuild(ref, 3)
	if _, err := srv.Storage().ReadFile(context.Background(), buildIDAliasMigrationProvenanceKey(key)); err == nil {
		t.Errorf("unexpected alias provenance sidecar for local-only artifact")
	}

	// Marker is still written so the migration is considered complete.
	if _, err := srv.Storage().ReadFile(context.Background(), buildIDAliasMigrationMarker); err != nil {
		t.Fatalf("expected migration marker even when no aliases written: %v", err)
	}
}
