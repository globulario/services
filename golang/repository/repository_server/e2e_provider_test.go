package main

// e2e_provider_test.go — End-to-end validation of the provider-neutral
// import path: LOCAL_DIR source with asset_path-only → SyncFromUpstream →
// repository catalog → DownloadArtifact → blob-cache refill provenance.
//
// These tests use a real LOCAL_DIR provider with actual files on disk,
// exercising the full path from release-index.json through provider.OpenArtifact
// to importUpstreamArtifact and manifest verification.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/repository/upstream"
)

// createLocalDirSource creates a LOCAL_DIR release stream with a real
// release-index.json and a real .tgz file (just test bytes, not a valid archive).
func createLocalDirSource(t *testing.T, tag, pkgName, pkgVersion string) (string, string) {
	t.Helper()
	root := t.TempDir()

	// Create the package archive (fake .tgz — just bytes for digest testing).
	pkgContent := []byte("fake-package-binary-content-for-" + pkgName)
	h := sha256.Sum256(pkgContent)
	pkgDigest := "sha256:" + hex.EncodeToString(h[:])
	filename := pkgName + "_" + pkgVersion + "_linux_amd64.tgz"

	// Place the artifact at an asset_path location.
	pkgDir := filepath.Join(root, "packages", pkgName, pkgVersion)
	os.MkdirAll(pkgDir, 0o755)
	os.WriteFile(filepath.Join(pkgDir, filename), pkgContent, 0o644)

	assetPath := filepath.Join("packages", pkgName, pkgVersion, filename)

	// Create release-index.json with asset_path only (no asset_url).
	changed := true
	idx := map[string]interface{}{
		"schema_version":   SchemaVersionV1,
		"release_tag":      tag,
		"globular_version": pkgVersion,
		"publisher":        "core@globular.io",
		"packages": []map[string]interface{}{
			{
				"name":                pkgName,
				"kind":                "SERVICE",
				"publisher":           "core@globular.io",
				"version":             pkgVersion,
				"build_number":        1,
				"build_id":            "e2e-1",
				"channel":             "stable",
				"platform":            "linux_amd64",
				"filename":            filename,
				"package_digest":      pkgDigest,
				"asset_path":          assetPath,
				"asset_url":           "", // intentionally empty
				"release_tag":         tag,
				"origin_release":      tag,
				"changed_in_release":  changed,
				"entrypoint_checksum": "",
			},
		},
	}
	idxData, _ := json.MarshalIndent(idx, "", "  ")

	relDir := filepath.Join(root, "releases", tag)
	os.MkdirAll(relDir, 0o755)
	os.WriteFile(filepath.Join(relDir, "release-index.json"), idxData, 0o644)

	return root, pkgDigest
}

// TestE2E_LocalDir_AssetPathImport exercises the full path:
// LOCAL_DIR + asset_path → provider.OpenArtifact → import → catalog.
func TestE2E_LocalDir_AssetPathImport(t *testing.T) {
	root, expectedDigest := createLocalDirSource(t, "v1.0.84", "echo", "1.0.84")

	// Create repository server with local storage.
	srv := newTestServer(t)
	ctx := context.Background()

	// Create provider and opts.
	provider, err := upstream.NewSource(upstream.TypeLocalDir)
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	opts := upstream.SourceOpts{
		LocalRoot:         root,
		IndexPathTemplate: "releases/{tag}/release-index.json",
	}

	// Fetch and parse release index via provider.
	indexData, err := provider.GetReleaseIndex(ctx, opts, "v1.0.84")
	if err != nil {
		t.Fatalf("get release index: %v", err)
	}

	idx, err := parseReleaseIndex(indexData)
	if err != nil {
		t.Fatalf("parse release index: %v", err)
	}

	if len(idx.Packages) != 1 {
		t.Fatalf("expected 1 package, got %d", len(idx.Packages))
	}
	entry := idx.Packages[0]

	// Verify asset_url is empty and asset_path is set.
	if entry.AssetURL != "" {
		t.Fatalf("test setup: asset_url should be empty, got %q", entry.AssetURL)
	}
	if entry.AssetPath == "" {
		t.Fatalf("test setup: asset_path should be set")
	}

	// Run processSyncEntry with real import (not dry-run).
	src := &repopb.UpstreamSource{
		Name:    "e2e-local",
		Enabled: true,
	}
	result := srv.processSyncEntry(ctx, entry, src, provider, opts, "v1.0.84", false, "")

	if result.Status != repopb.UpstreamSyncStatus_SYNC_IMPORTED {
		t.Fatalf("expected SYNC_IMPORTED, got %s: %s", result.Status, result.Detail)
	}

	// ── Verify: manifest is in the repository catalog ────────────────────
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "echo",
		Version:     "1.0.84",
		Platform:    "linux_amd64",
	}
	key := artifactKeyWithBuild(ref, 1)
	_, state, manifest, readErr := srv.readManifestAndStateByKey(ctx, key)
	if readErr != nil {
		t.Fatalf("manifest not found in catalog: %v", readErr)
	}
	if state != repopb.PublishState_PUBLISHED {
		t.Fatalf("expected PUBLISHED, got %s", state)
	}
	if manifest.GetChecksum() != expectedDigest {
		t.Fatalf("checksum mismatch: expected %s, got %s", expectedDigest, manifest.GetChecksum())
	}
	if manifest.GetBuildId() != "e2e-1" {
		t.Fatalf("build_id: expected e2e-1, got %s", manifest.GetBuildId())
	}

	// ── Verify: binary is in storage ─────────────────────────────────────
	binKey := binaryStorageKey(key)
	binReader, openErr := srv.Storage().Open(ctx, binKey)
	if openErr != nil {
		t.Fatalf("binary not found in storage: %v", openErr)
	}
	binReader.Close()

	// ── Verify: upstream_import provenance has path: locator ─────────────
	ui := manifest.GetUpstreamImport()
	if ui == nil {
		t.Fatal("upstream_import should be set")
	}
	if ui.GetSourceName() != "e2e-local" {
		t.Fatalf("source_name: expected e2e-local, got %q", ui.GetSourceName())
	}
	// asset_url should have "path:" prefix since there was no HTTP URL.
	assetURL := ui.GetAssetUrl()
	if assetURL == "" {
		t.Fatal("provenance asset_url should not be empty")
	}
	if len(assetURL) < 5 || assetURL[:5] != "path:" {
		t.Fatalf("provenance asset_url should start with 'path:' for LOCAL_DIR, got %q", assetURL)
	}

	// ── Verify: delete binary then check refill provenance is available ──
	// (Full refill requires etcd for source lookup, but we verify the
	// provenance record has enough info to locate the artifact.)
	if delErr := srv.Storage().Remove(ctx, binKey); delErr != nil {
		t.Fatalf("delete binary: %v", delErr)
	}
	// Binary is gone.
	if _, openErr := srv.Storage().Open(ctx, binKey); openErr == nil {
		t.Fatal("binary should be deleted")
	}
	// Manifest still exists with provenance.
	_, _, manifest2, _ := srv.readManifestAndStateByKey(ctx, key)
	ui2 := manifest2.GetUpstreamImport()
	if ui2.GetSourceName() == "" || ui2.GetAssetUrl() == "" || ui2.GetChecksum() == "" {
		t.Fatal("provenance must have source_name, asset_url, and checksum for refill")
	}
}

// TestE2E_LocalDir_AssetPathImport_IdempotentReimport verifies that
// importing the same package twice from LOCAL_DIR is idempotent (SKIPPED).
func TestE2E_LocalDir_AssetPathImport_IdempotentReimport(t *testing.T) {
	root, _ := createLocalDirSource(t, "v1.0.84", "rbac", "1.0.84")

	srv := newTestServer(t)
	ctx := context.Background()
	provider, _ := upstream.NewSource(upstream.TypeLocalDir)
	opts := upstream.SourceOpts{
		LocalRoot:         root,
		IndexPathTemplate: "releases/{tag}/release-index.json",
	}
	indexData, err := provider.GetReleaseIndex(ctx, opts, "v1.0.84")
	if err != nil {
		t.Fatalf("get index: %v", err)
	}
	idx, err := parseReleaseIndex(indexData)
	if err != nil {
		t.Fatalf("parse index: %v", err)
	}
	if len(idx.Packages) == 0 {
		t.Fatal("no packages in index")
	}
	src := &repopb.UpstreamSource{Name: "e2e-local", Enabled: true}

	// First import.
	r1 := srv.processSyncEntry(ctx, idx.Packages[0], src, provider, opts, "v1.0.84", false, "")
	if r1.Status != repopb.UpstreamSyncStatus_SYNC_IMPORTED {
		t.Fatalf("first import: expected IMPORTED, got %s: %s", r1.Status, r1.Detail)
	}

	// Second import — should be SKIPPED.
	r2 := srv.processSyncEntry(ctx, idx.Packages[0], src, provider, opts, "v1.0.84", false, "")
	if r2.Status != repopb.UpstreamSyncStatus_SYNC_SKIPPED {
		t.Fatalf("second import: expected SKIPPED, got %s: %s", r2.Status, r2.Detail)
	}
}
