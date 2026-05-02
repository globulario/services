package main

// blob_integrity.go — Canonical artifact presence verification.
//
// INVARIANT: An artifact is "present" if and only if:
//   1. Metadata exists (ScyllaDB row or manifest file).
//   2. The binary blob exists at binaryStorageKey(artifactKeyWithBuild(...)).
//
// Neither alone is sufficient. This file provides the single source of truth
// for artifact presence checks across all code paths (sync, import, download,
// consistency scan).

import (
	"context"
	"io/fs"
	"log/slog"
	"strings"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// artifactBlobStatus checks whether the binary blob for a given artifact
// exists in object storage AND, when expectedSize > 0, that its size matches.
// Returns (present, reason) where reason is a stable token suitable for logs:
//
//	"ok"             — blob exists (and size matches expectedSize if provided)
//	"missing_blob"   — Stat returned an error (object not in storage)
//	"size_mismatch"  — Stat succeeded but fi.Size() != expectedSize
//	"nil_ref"        — ref was nil (programmer error)
//
// This is the ONLY function that should be used to decide whether an
// artifact can be skipped during import or sync. Never trust metadata alone.
// Bucket listing or cached directory state must NOT be used here — only an
// exact object Stat against binaryStorageKey(artifactKeyWithBuild(...)).
func (srv *server) artifactBlobStatus(ctx context.Context, ref *repopb.ArtifactRef, buildNumber int64, expectedSize int64) (present bool, reason string) {
	if ref == nil {
		return false, "nil_ref"
	}

	key := artifactKeyWithBuild(ref, buildNumber)
	blobKey := binaryStorageKey(key)

	fi, err := srv.Storage().Stat(ctx, blobKey)
	if err != nil {
		return false, "missing_blob"
	}
	if expectedSize > 0 && fi.Size() != expectedSize {
		return false, "size_mismatch"
	}
	return true, "ok"
}

// digestEqual compares two digest strings, normalizing the "sha256:" prefix.
// Both "sha256:abc123" and "abc123" are treated as equivalent.
// Empty digests never match (returns false).
func digestEqual(a, b string) bool {
	a = normalizeDigest(a)
	b = normalizeDigest(b)
	if a == "" || b == "" {
		return false
	}
	return a == b
}

// normalizeDigest strips the "sha256:" prefix, trims whitespace, and lowercases.
// Returns "" for empty input.
func normalizeDigest(d string) string {
	d = strings.TrimSpace(d)
	d = strings.ToLower(d)
	d = strings.TrimPrefix(d, "sha256:")
	return d
}

// canonicalDigest returns the digest in canonical "sha256:<hex>" form.
// Returns "" for empty input.
func canonicalDigest(d string) string {
	n := normalizeDigest(d)
	if n == "" {
		return ""
	}
	return "sha256:" + n
}

// artifactBlobPresent is a convenience wrapper that returns true if the blob
// exists and matches expected size. Use this in skip-decision paths.
func (srv *server) artifactBlobPresent(ctx context.Context, ref *repopb.ArtifactRef, buildNumber int64, expectedSize int64) bool {
	present, _ := srv.artifactBlobStatus(ctx, ref, buildNumber, expectedSize)
	return present
}

// logBlobSkipDecision logs the full fingerprint of a skip / repair decision.
// Decision is one of "skip" | "reimport" | "repair_blob"; reason is the value
// returned by artifactBlobStatus (e.g. "ok", "missing_blob", "size_mismatch").
func logBlobSkipDecision(source, publisher, name, version, platform, buildID string, buildNumber int64, digest, blobKey, decision, reason string) {
	slog.Info("blob-integrity: skip decision",
		"source", source,
		"publisher", publisher,
		"name", name,
		"version", version,
		"platform", platform,
		"build_id", buildID,
		"build_number", buildNumber,
		"digest", truncDigest(digest),
		"blob_key", blobKey,
		"decision", decision,
		"reason", reason,
	)
}

// blobKeyForRef builds the blob storage key for logging purposes.
func blobKeyForRef(ref *repopb.ArtifactRef, buildNumber int64) string {
	if ref == nil {
		return ""
	}
	return binaryStorageKey(artifactKeyWithBuild(ref, buildNumber))
}

// statToFileInfo is a helper — Storage().Stat returns fs.FileInfo but we
// only need it for size checks. If Stat is not available, use Exists.
var _ fs.FileInfo // ensure import
