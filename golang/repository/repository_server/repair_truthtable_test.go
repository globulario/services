package main

// repair_truthtable_test.go — completes the repair-decision truth table with the
// two cases the existing tests left open. These LOCK already-correct master
// behavior against a future "optimization" reintroducing a metadata-only skip.
//
//   PUBLISHED + blob absent + NO upstream  → explicit refusal, never success/skip.
//   PUBLISHED + blob present + digest wrong → corrupt, never "already healthy".
//
// Scope is strictly these two rows. No build_id / reinstall / CAS-actor concerns.

import (
	"bytes"
	"context"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// TestRepairArtifact_MissingBlob_NoUpstream_RefusesNeverSucceeds locks the
// "unrecoverable" row: with the blob absent and no upstream to re-import from,
// a REAL (non-dry-run) repair must fail closed — never skipped_ok, never
// repair_blob, and it must not mutate state to pretend the artifact is healthy.
func TestRepairArtifact_MissingBlob_NoUpstream_RefusesNeverSucceeds(t *testing.T) {
	srv := newTestServer(t) // no upstream source configured
	ctx := context.Background()
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "echo",
		Version: "1.0.0", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: ref, BuildNumber: 1, BuildId: "v1",
		Checksum: "sha256:abc", SizeBytes: 100,
	})
	key := artifactKeyWithBuild(ref, 1)
	if err := srv.Storage().Remove(ctx, binaryStorageKey(key)); err != nil {
		t.Fatalf("delete blob: %v", err)
	}

	resp, err := srv.RepairArtifact(ctx, &repopb.RepairArtifactRequest{Ref: ref, BuildNumber: 1})
	if err != nil {
		t.Fatalf("RepairArtifact: %v", err)
	}
	// Must NOT falsely claim success or "nothing to do".
	if a := resp.GetAction(); a == "skipped_ok" || a == "repair_blob" || a == "repair_checksum_mismatch" {
		t.Fatalf("no-upstream repair must fail closed, got success-shaped action=%q detail=%q", a, resp.GetDetail())
	}
	if resp.GetAction() != "failed" {
		t.Fatalf("expected explicit action=failed (unrecoverable without upstream), got %q (detail=%q)", resp.GetAction(), resp.GetDetail())
	}

	// No metadata mutation pretending repair succeeded: re-verify must still be
	// broken + not installable.
	v, err := srv.VerifyArtifact(ctx, &repopb.VerifyArtifactRequest{Ref: ref, BuildNumber: 1})
	if err != nil {
		t.Fatalf("VerifyArtifact (post): %v", err)
	}
	if v.GetStatus() != repopb.ArtifactVerifyStatus_ARTIFACT_VERIFY_BROKEN_MISSING_BLOB {
		t.Fatalf("post-failed-repair status expected BROKEN_MISSING_BLOB, got %s", v.GetStatus())
	}
	if v.GetInstallable() {
		t.Error("a failed repair must not leave the artifact installable")
	}
}

// TestRepairArtifact_CorruptBlob_NeverHealthy locks the "corrupt" row with a TRUE
// digest mismatch (correct size, wrong bytes): verification must classify it as
// checksum mismatch (never healthy/installable), and repair must never call it
// skipped_ok — it must offer to re-import, never silently accept the corrupt blob.
func TestRepairArtifact_CorruptBlob_NeverHealthy(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "echo",
		Version: "1.0.0", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: ref, BuildNumber: 1, BuildId: "v1",
		Checksum: "sha256:abc", SizeBytes: 100,
	})
	key := artifactKeyWithBuild(ref, 1)
	// Correct size (100), wrong content → digest mismatch (not a size mismatch).
	if err := srv.Storage().WriteFile(ctx, binaryStorageKey(key), bytes.Repeat([]byte{0x42}, 100), 0o644); err != nil {
		t.Fatalf("corrupt blob: %v", err)
	}

	// Verification: corrupt is never healthy / installable.
	v, err := srv.VerifyArtifact(ctx, &repopb.VerifyArtifactRequest{Ref: ref, BuildNumber: 1, VerifyDigest: true})
	if err != nil {
		t.Fatalf("VerifyArtifact: %v", err)
	}
	if v.GetStatus() != repopb.ArtifactVerifyStatus_ARTIFACT_VERIFY_BROKEN_CHECKSUM_MISMATCH {
		t.Fatalf("corrupt blob expected BROKEN_CHECKSUM_MISMATCH, got %s (reason=%s)", v.GetStatus(), v.GetReason())
	}
	if v.GetInstallable() {
		t.Error("corrupt blob must not be installable")
	}

	// Repair dry-run: never skipped_ok; must offer to re-import the corrupt blob.
	resp, err := srv.RepairArtifact(ctx, &repopb.RepairArtifactRequest{Ref: ref, BuildNumber: 1, DryRun: true})
	if err != nil {
		t.Fatalf("RepairArtifact dry-run: %v", err)
	}
	if resp.GetAction() == "skipped_ok" {
		t.Fatalf("corrupt blob dry-run must NOT be skipped_ok (never already-healthy); detail=%q", resp.GetDetail())
	}
	if resp.GetAction() != "would_repair_checksum_mismatch" {
		t.Fatalf("expected action=would_repair_checksum_mismatch, got %q", resp.GetAction())
	}
}
