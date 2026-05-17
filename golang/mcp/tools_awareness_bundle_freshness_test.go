package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/awareness/bundlesync"
)

// ── Phase B.1 freshness tool tests ───────────────────────────────────────────
//
// Acceptance:
//   - Fresh manifest + matching release-index → AWARENESS_READY
//   - Stale build_id (same version)            → AWARENESS_BUNDLE_STALE
//   - Mismatched version                       → AWARENESS_BUNDLE_MISMATCH
//   - Schema unsupported                       → AWARENESS_BUNDLE_SCHEMA_UNSUPPORTED
//   - No release-index present                 → AWARENESS_BUNDLE_VERIFY_FAILED
//   - No manifest present                      → AWARENESS_BUNDLE_MISSING
//   - Tool is allowlisted for aggregator       → present in allowlist as READ_ONLY
//   - Tool rejects path-like arguments         → ARG_REJECTED, no leak

// setupReleaseIndex writes a release-index.json for the test and points
// releaseIndexPath at it. Cleanup restores the prior path.
func setupReleaseIndex(t *testing.T, dir string, ri bundlesync.ReleaseIndex) string {
	t.Helper()
	path := filepath.Join(dir, "release-index.json")
	data, err := json.MarshalIndent(ri, "", "  ")
	if err != nil {
		t.Fatalf("marshal release-index: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write release-index: %v", err)
	}
	prev := releaseIndexPath
	releaseIndexPath = path
	t.Cleanup(func() { releaseIndexPath = prev })
	return path
}

// stubLocalBinaryInfo overrides localBinaryInfo for the test duration.
func stubLocalBinaryInfo(t *testing.T, lb *bundlesync.LocalBinaryInfo) {
	t.Helper()
	prev := localBinaryInfo
	localBinaryInfo = func() *bundlesync.LocalBinaryInfo { return lb }
	t.Cleanup(func() { localBinaryInfo = prev })
}

func newFreshnessTestServer(t *testing.T) *server {
	t.Helper()
	cfg := defaultConfig()
	// Disable groups requiring live network/services.
	cfg.ToolGroups.Cluster = false
	cfg.ToolGroups.Doctor = false
	cfg.ToolGroups.NodeAgent = false
	cfg.ToolGroups.Repository = false
	cfg.ToolGroups.Backup = false
	cfg.ToolGroups.RBAC = false
	cfg.ToolGroups.Resource = false
	cfg.ToolGroups.File = false
	cfg.ToolGroups.Composed = false
	cfg.ToolGroups.CLI = false
	cfg.ToolGroups.Governor = false
	cfg.ToolGroups.Memory = false
	cfg.ToolGroups.Skills = false
	cfg.ToolGroups.Workflow = false
	cfg.ToolGroups.Etcd = false
	cfg.ToolGroups.Title = false
	cfg.ToolGroups.Frontend = false
	cfg.ToolGroups.Proto = false
	cfg.ToolGroups.HTTPDiag = false
	cfg.ToolGroups.Monitoring = false
	cfg.ToolGroups.Browser = false
	cfg.ToolGroups.AIExecutor = false
	cfg.ToolGroups.Awareness = false
	cfg.ToolGroups.Aggregator = false

	s := newServer(cfg)
	registerAwarenessBundleServeTools(s)
	return s
}

// 1. Fresh bundle + matching release-index → AWARENESS_READY.
func TestFreshnessStatusReady(t *testing.T) {
	bundleDir := setupActiveBundleDir(t)
	manifest, _ := installFakeBundle(t, bundleDir)

	releaseDir := t.TempDir()
	setupReleaseIndex(t, releaseDir, bundlesync.ReleaseIndex{
		Version: manifest.Version,
		BuildID: manifest.BuildID,
	})
	stubLocalBinaryInfo(t, &bundlesync.LocalBinaryInfo{}) // skip correlation

	s := newFreshnessTestServer(t)
	res, err := s.callTool(context.Background(), "mcp.awareness_freshness_status", nil)
	if err != nil {
		t.Fatalf("tool error: %v", err)
	}
	m := res.(map[string]interface{})
	if m["state"] != bundlesync.StateAwarenessReady {
		t.Errorf("state = %v, want AWARENESS_READY", m["state"])
	}
	if ok, _ := m["ok"].(bool); !ok {
		t.Errorf("ok = %v, want true", m["ok"])
	}
	if vMatch, _ := m["version_matches_release"].(bool); !vMatch {
		t.Errorf("version_matches_release = %v, want true", m["version_matches_release"])
	}
	if bMatch, _ := m["build_id_matches_release"].(bool); !bMatch {
		t.Errorf("build_id_matches_release = %v, want true", m["build_id_matches_release"])
	}
}

// 2. Stale build_id (same version) → AWARENESS_BUNDLE_STALE, NOT READY.
// This is the "bundle behind on CI build" case.
func TestFreshnessStatusStaleOnBuildIDDrift(t *testing.T) {
	bundleDir := setupActiveBundleDir(t)
	manifest, _ := installFakeBundle(t, bundleDir)

	releaseDir := t.TempDir()
	setupReleaseIndex(t, releaseDir, bundlesync.ReleaseIndex{
		Version: manifest.Version,
		BuildID: "newer-build-456",
	})
	stubLocalBinaryInfo(t, &bundlesync.LocalBinaryInfo{})

	s := newFreshnessTestServer(t)
	res, _ := s.callTool(context.Background(), "mcp.awareness_freshness_status", nil)
	m := res.(map[string]interface{})

	if m["state"] != bundlesync.StateAwarenessBundleStale {
		t.Errorf("state = %v, want AWARENESS_BUNDLE_STALE", m["state"])
	}
	if ok, _ := m["ok"].(bool); ok {
		t.Errorf("ok = true; stale bundle must not pass")
	}
	if vMatch, _ := m["version_matches_release"].(bool); !vMatch {
		t.Errorf("version_matches_release should be true (only build_id drifted)")
	}
	if bMatch, _ := m["build_id_matches_release"].(bool); bMatch {
		t.Errorf("build_id_matches_release should be false")
	}
}

// 3. Different version → AWARENESS_BUNDLE_MISMATCH.
func TestFreshnessStatusMismatchOnVersionDrift(t *testing.T) {
	bundleDir := setupActiveBundleDir(t)
	manifest, _ := installFakeBundle(t, bundleDir)

	releaseDir := t.TempDir()
	setupReleaseIndex(t, releaseDir, bundlesync.ReleaseIndex{
		Version: "v9.9.99",
		BuildID: manifest.BuildID,
	})
	stubLocalBinaryInfo(t, &bundlesync.LocalBinaryInfo{})

	s := newFreshnessTestServer(t)
	res, _ := s.callTool(context.Background(), "mcp.awareness_freshness_status", nil)
	m := res.(map[string]interface{})

	if m["state"] != bundlesync.StateAwarenessBundleMismatch {
		t.Errorf("state = %v, want AWARENESS_BUNDLE_MISMATCH", m["state"])
	}
}

// 4. Unsupported schema is preserved through the tool — distinct from VERIFY_FAILED.
func TestFreshnessStatusSchemaUnsupported(t *testing.T) {
	bundleDir := setupActiveBundleDir(t)
	manifest, _ := installFakeBundle(t, bundleDir)

	// Rewrite the manifest with an unsupported schema.
	manifest.SchemaVersion = "awareness.bundle.v99"
	mb, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(bundleDir, activeManifestFile), mb, 0644); err != nil {
		t.Fatalf("rewrite manifest: %v", err)
	}

	releaseDir := t.TempDir()
	setupReleaseIndex(t, releaseDir, bundlesync.ReleaseIndex{
		Version: manifest.Version,
		BuildID: manifest.BuildID,
	})
	stubLocalBinaryInfo(t, &bundlesync.LocalBinaryInfo{})

	s := newFreshnessTestServer(t)
	res, _ := s.callTool(context.Background(), "mcp.awareness_freshness_status", nil)
	m := res.(map[string]interface{})

	if m["state"] != bundlesync.StateAwarenessBundleSchemaUnsupported {
		t.Errorf("state = %v, want AWARENESS_BUNDLE_SCHEMA_UNSUPPORTED", m["state"])
	}
}

// 5. No release-index → AWARENESS_BUNDLE_VERIFY_FAILED with a clear reason.
// We cannot decide freshness without authority.
func TestFreshnessStatusMissingReleaseIndex(t *testing.T) {
	_ = setupActiveBundleDir(t) // bundle setup not relevant — we never reach it
	prev := releaseIndexPath
	releaseIndexPath = filepath.Join(t.TempDir(), "no-such-release-index.json")
	t.Cleanup(func() { releaseIndexPath = prev })

	stubLocalBinaryInfo(t, &bundlesync.LocalBinaryInfo{})

	s := newFreshnessTestServer(t)
	res, _ := s.callTool(context.Background(), "mcp.awareness_freshness_status", nil)
	m := res.(map[string]interface{})

	if m["state"] != bundlesync.StateAwarenessBundleVerifyFailed {
		t.Errorf("state = %v, want AWARENESS_BUNDLE_VERIFY_FAILED", m["state"])
	}
	if _, ok := m["reason"]; !ok {
		t.Errorf("reason field missing")
	}
}

// 6. No manifest → AWARENESS_BUNDLE_MISSING (cold-bootstrap state).
func TestFreshnessStatusMissingManifest(t *testing.T) {
	_ = setupActiveBundleDir(t) // empty dir, no manifest

	releaseDir := t.TempDir()
	setupReleaseIndex(t, releaseDir, bundlesync.ReleaseIndex{
		Version: "v1.2.30", BuildID: "abc123",
	})
	stubLocalBinaryInfo(t, &bundlesync.LocalBinaryInfo{})

	s := newFreshnessTestServer(t)
	res, _ := s.callTool(context.Background(), "mcp.awareness_freshness_status", nil)
	m := res.(map[string]interface{})

	if m["state"] != bundlesync.StateAwarenessBundleMissing {
		t.Errorf("state = %v, want AWARENESS_BUNDLE_MISSING", m["state"])
	}
}

// 7. Local binary version drift fails freshness even when manifest matches release.
func TestFreshnessStatusLocalBinaryDrift(t *testing.T) {
	bundleDir := setupActiveBundleDir(t)
	manifest, _ := installFakeBundle(t, bundleDir)

	releaseDir := t.TempDir()
	setupReleaseIndex(t, releaseDir, bundlesync.ReleaseIndex{
		Version: manifest.Version,
		BuildID: manifest.BuildID,
	})
	// Local binary is on a different version than the bundle.
	stubLocalBinaryInfo(t, &bundlesync.LocalBinaryInfo{
		Version: "v1.2.31",
		BuildID: manifest.BuildID,
	})

	s := newFreshnessTestServer(t)
	res, _ := s.callTool(context.Background(), "mcp.awareness_freshness_status", nil)
	m := res.(map[string]interface{})

	if m["state"] != bundlesync.StateAwarenessBundleMismatch {
		t.Errorf("state = %v, want AWARENESS_BUNDLE_MISMATCH for binary/bundle drift", m["state"])
	}
	lb, ok := m["local_binary"].(map[string]interface{})
	if !ok {
		t.Fatalf("local_binary missing from response")
	}
	if vMatch, _ := lb["version_match"].(bool); vMatch {
		t.Errorf("local_binary.version_match should be false")
	}
}

// 8. Tool is in the aggregator allowlist as READ_ONLY.
func TestFreshnessStatusAllowlisted(t *testing.T) {
	if !IsRemoteToolAllowed("mcp.awareness_freshness_status") {
		t.Error("mcp.awareness_freshness_status must be in the aggregator allowlist")
	}
	if ClassifyRemoteToolSafety("mcp.awareness_freshness_status") != "READ_ONLY" {
		t.Errorf("safety = %q, want READ_ONLY", ClassifyRemoteToolSafety("mcp.awareness_freshness_status"))
	}
}

// 9. Tool rejects path-like arguments — defense-in-depth, even though it
// reads only canonical paths.
func TestFreshnessStatusRejectsPathLikeArgs(t *testing.T) {
	bundleDir := setupActiveBundleDir(t)
	installFakeBundle(t, bundleDir)
	releaseDir := t.TempDir()
	setupReleaseIndex(t, releaseDir, bundlesync.ReleaseIndex{Version: "v1.2.30", BuildID: "abc123"})
	stubLocalBinaryInfo(t, &bundlesync.LocalBinaryInfo{})

	s := newFreshnessTestServer(t)
	res, _ := s.callTool(context.Background(), "mcp.awareness_freshness_status",
		map[string]interface{}{"path": "/etc/passwd"})
	m := res.(map[string]interface{})
	if m["error_kind"] != "ARG_REJECTED" {
		t.Errorf("error_kind = %v, want ARG_REJECTED", m["error_kind"])
	}
	for _, leak := range []string{"manifest", "release_index", "ok"} {
		if _, ok := m[leak]; ok {
			t.Errorf("rejected response leaks %q", leak)
		}
	}
}

// loadReleaseIndex tolerates the {"active":{...}} nested shape.
func TestLoadReleaseIndexNestedActiveShape(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "release-index.json")
	data := []byte(`{"active":{"version":"v1.2.30","build_id":"abc123"},"history":[]}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	ri, err := loadReleaseIndex(path)
	if err != nil {
		t.Fatalf("loadReleaseIndex: %v", err)
	}
	if ri.Version != "v1.2.30" || ri.BuildID != "abc123" {
		t.Errorf("got %+v, want v1.2.30/abc123", ri)
	}
}

// TestLoadReleaseIndexBOMShape pins the contract that loadReleaseIndex
// extracts the AWARENESS_BUNDLE entry out of the canonical BOM-shaped
// release-index produced by the repository / CI release pipeline. Without
// this case, every freshness check on a node that has a real
// release-index.json returned AWARENESS_BUNDLE_VERIFY_FAILED — even after
// a successful `awareness bundle publish` — because the flat shape parse
// silently fell through.
func TestLoadReleaseIndexBOMShape(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "release-index.json")
	data := []byte(`{
  "schema_version": "globular.repository.index/v2",
  "release_tag": "v2026.05.14",
  "packages": [
    {
      "name": "echo",
      "kind": "SERVICE",
      "version": "1.0.5",
      "build_id": "service-build-id-aaa",
      "platform": "linux_amd64",
      "package_digest": "sha256:0000000000000000000000000000000000000000000000000000000000000000"
    },
    {
      "name": "globular-awareness-bundle",
      "kind": "AWARENESS_BUNDLE",
      "version": "0.0.7",
      "build_id": "bundle-build-id-xyz",
      "platform": "any",
      "package_digest": "sha256:1111111111111111111111111111111111111111111111111111111111111111"
    }
  ]
}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	ri, err := loadReleaseIndex(path)
	if err != nil {
		t.Fatalf("loadReleaseIndex: %v", err)
	}
	if ri.Version != "0.0.7" || ri.BuildID != "bundle-build-id-xyz" {
		t.Errorf("got %+v, want version=0.0.7 build_id=bundle-build-id-xyz", ri)
	}
}

// TestLoadReleaseIndexBOMShapePrefersCanonicalName ensures a BOM with
// multiple AWARENESS_BUNDLE entries (e.g. a third-party bundle and the
// canonical globular-awareness-bundle) picks the canonical one — so a
// rogue or test bundle in the index can never displace the official
// freshness target.
func TestLoadReleaseIndexBOMShapePrefersCanonicalName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "release-index.json")
	data := []byte(`{
  "schema_version": "globular.repository.index/v2",
  "release_tag": "v1",
  "packages": [
    {"name": "third-party-knowledge", "kind": "AWARENESS_BUNDLE", "version": "9.9.9", "build_id": "rogue"},
    {"name": "globular-awareness-bundle", "kind": "AWARENESS_BUNDLE", "version": "0.0.7", "build_id": "official"}
  ]
}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	ri, err := loadReleaseIndex(path)
	if err != nil {
		t.Fatalf("loadReleaseIndex: %v", err)
	}
	if ri.Version != "0.0.7" || ri.BuildID != "official" {
		t.Errorf("got %+v, want canonical entry (version=0.0.7 build_id=official)", ri)
	}
}

// TestLoadReleaseIndexBOMShapeNoAwarenessFalls fails over to the flat
// shape when a BOM is present but contains no AWARENESS_BUNDLE entry. The
// existing freshness behaviour for that case is "no version found" which
// surfaces as AWARENESS_BUNDLE_VERIFY_FAILED — this test pins that the
// BOM-aware path doesn't silently fabricate a verdict.
func TestLoadReleaseIndexBOMShapeNoAwarenessFalls(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "release-index.json")
	data := []byte(`{
  "schema_version": "globular.repository.index/v2",
  "release_tag": "v1",
  "packages": [
    {"name": "echo", "kind": "SERVICE", "version": "1.0.5", "build_id": "svc"}
  ]
}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := loadReleaseIndex(path); err == nil {
		t.Fatalf("expected error when BOM has no AWARENESS_BUNDLE entry")
	}
}
