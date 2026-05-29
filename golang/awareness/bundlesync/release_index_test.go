package bundlesync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadReleaseIndex_BOMSchemaV2 pins the canonical production shape: the
// repository-index schema v2 with a packages array containing an
// AWARENESS_BUNDLE entry. Before this loader supported BOM, every cluster
// running v2 failed `globular awareness install` with
// "release-index ...: no usable version/build_id" even when the bundle pin
// was healthy.
func TestLoadReleaseIndex_BOMSchemaV2(t *testing.T) {
	body := []byte(`{
		"schema_version": "globular.repository.index/v2",
		"platform_release": "1.2.120",
		"packages": [
			{"name": "ai-executor", "kind": "service", "version": "1.2.120", "build_id": "ignored-svc-id"},
			{"name": "globular-awareness-bundle", "kind": "awareness_bundle", "version": "1.2.120", "build_id": "78b1ff01-95bd-4e97-82a7-55ce1323c1fc"}
		]
	}`)
	ri, err := parseReleaseIndex("release-index.json", body)
	if err != nil {
		t.Fatalf("parseReleaseIndex: %v", err)
	}
	if ri.Version != "1.2.120" {
		t.Errorf("Version = %q, want 1.2.120", ri.Version)
	}
	if ri.BuildID != "78b1ff01-95bd-4e97-82a7-55ce1323c1fc" {
		t.Errorf("BuildID = %q, want 78b1ff01-95bd-4e97-82a7-55ce1323c1fc", ri.BuildID)
	}
}

// TestLoadReleaseIndex_BOMNameMatchWinsOverDocumentOrder confirms the selection
// rule when multiple AWARENESS_BUNDLE packages are present: the exact-name
// match (BundleName) wins over document order. Without this guarantee, a
// downgrade-during-rollout could pick a stale entry that happened to be earlier
// in the array.
func TestLoadReleaseIndex_BOMNameMatchWinsOverDocumentOrder(t *testing.T) {
	body := []byte(`{
		"packages": [
			{"name": "stale-mirror-bundle", "kind": "AWARENESS_BUNDLE", "version": "1.2.99", "build_id": "stale"},
			{"name": "globular-awareness-bundle", "kind": "AWARENESS_BUNDLE", "version": "1.2.120", "build_id": "real"}
		]
	}`)
	ri, err := parseReleaseIndex("release-index.json", body)
	if err != nil {
		t.Fatalf("parseReleaseIndex: %v", err)
	}
	if ri.Version != "1.2.120" || ri.BuildID != "real" {
		t.Errorf("got (%q, %q), want (1.2.120, real); selection rule must prefer BundleName match over document order", ri.Version, ri.BuildID)
	}
}

// TestLoadReleaseIndex_BOMFallsBackToFirstWhenNoNameMatch verifies the
// fallback when no entry exact-name matches BundleName: pick the first
// AWARENESS_BUNDLE entry by document order, skipping ones with empty Version.
func TestLoadReleaseIndex_BOMFallsBackToFirstWhenNoNameMatch(t *testing.T) {
	body := []byte(`{
		"packages": [
			{"name": "skipped-empty-version", "kind": "AWARENESS_BUNDLE", "version": "", "build_id": "ignored"},
			{"name": "first-with-version", "kind": "AWARENESS_BUNDLE", "version": "1.2.50", "build_id": "first"},
			{"name": "second-with-version", "kind": "AWARENESS_BUNDLE", "version": "1.2.60", "build_id": "second"}
		]
	}`)
	ri, err := parseReleaseIndex("release-index.json", body)
	if err != nil {
		t.Fatalf("parseReleaseIndex: %v", err)
	}
	if ri.Version != "1.2.50" || ri.BuildID != "first" {
		t.Errorf("got (%q, %q), want (1.2.50, first); fallback rule must pick first AWARENESS_BUNDLE with non-empty Version in document order", ri.Version, ri.BuildID)
	}
}

// TestLoadReleaseIndex_FlatShape preserves backwards compatibility with the
// flat shape used by older tooling and tests.
func TestLoadReleaseIndex_FlatShape(t *testing.T) {
	body := []byte(`{"version":"1.2.0","build_id":"flat-build-id"}`)
	ri, err := parseReleaseIndex("release-index.json", body)
	if err != nil {
		t.Fatalf("parseReleaseIndex: %v", err)
	}
	if ri.Version != "1.2.0" || ri.BuildID != "flat-build-id" {
		t.Errorf("flat shape lost: got (%q, %q)", ri.Version, ri.BuildID)
	}
}

// TestLoadReleaseIndex_NestedActiveShape preserves backwards compatibility
// with the {"active": {...}} shape some dev tooling emits.
func TestLoadReleaseIndex_NestedActiveShape(t *testing.T) {
	body := []byte(`{"active":{"version":"1.2.10","build_id":"nested-build-id"}}`)
	ri, err := parseReleaseIndex("release-index.json", body)
	if err != nil {
		t.Fatalf("parseReleaseIndex: %v", err)
	}
	if ri.Version != "1.2.10" || ri.BuildID != "nested-build-id" {
		t.Errorf("nested shape lost: got (%q, %q)", ri.Version, ri.BuildID)
	}
}

// TestLoadReleaseIndex_V2WithNoAwarenessBundleEntry verifies the error message
// is specific. The previous "no usable version/build_id" message did not
// distinguish "BOM present but no AWARENESS_BUNDLE" from "not BOM-shaped at all"
// — operators ended up guessing whether to fix release-index or the bundle.
func TestLoadReleaseIndex_V2WithNoAwarenessBundleEntry(t *testing.T) {
	body := []byte(`{
		"schema_version": "globular.repository.index/v2",
		"packages": [
			{"name": "ai-executor", "kind": "service", "version": "1.2.120", "build_id": "svc"},
			{"name": "globular-mcp", "kind": "service", "version": "1.2.120", "build_id": "svc2"}
		]
	}`)
	_, err := parseReleaseIndex("release-index.json", body)
	if err == nil {
		t.Fatal("expected error: v2 schema without AWARENESS_BUNDLE entry must not return a usable pin")
	}
	if !strings.Contains(err.Error(), "no AWARENESS_BUNDLE") {
		t.Errorf("error message must distinguish missing-AWARENESS_BUNDLE from missing-flat-version; got %q", err.Error())
	}
}

// TestLoadReleaseIndex_FileMissing surfaces ENOENT cleanly rather than as a
// JSON parse error.
func TestLoadReleaseIndex_FileMissing(t *testing.T) {
	_, err := LoadReleaseIndex(filepath.Join(t.TempDir(), "absent.json"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "read release-index") {
		t.Errorf("missing-file error should mention the read step; got %q", err.Error())
	}
}

// TestLoadReleaseIndex_RealProductionFixture loads the byte-for-byte shape
// observed on the running cluster on 2026-05-29. If anyone tightens the
// loader, this test catches the regression that this whole patch was written
// to fix.
func TestLoadReleaseIndex_RealProductionFixture(t *testing.T) {
	body := []byte(`{
		"schema_version": "globular.repository.index/v2",
		"platform_release": "1.2.120",
		"release_tag": "v1.2.120",
		"publisher": "globulario",
		"generated_at": "2026-05-29T22:25:28Z",
		"package_digest_algorithm": "sha256",
		"force_full_rebuild": false,
		"packages": [
			{
				"name": "globular-awareness-bundle",
				"kind": "awareness_bundle",
				"version": "1.2.120",
				"build_number": 372,
				"build_id": "78b1ff01-95bd-4e97-82a7-55ce1323c1fc",
				"platform": "noarch",
				"publisher": "core@globular.io"
			}
		]
	}`)
	dir := t.TempDir()
	path := filepath.Join(dir, "release-index.json")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	ri, err := LoadReleaseIndex(path)
	if err != nil {
		t.Fatalf("LoadReleaseIndex: %v", err)
	}
	if ri.Version != "1.2.120" || ri.BuildID != "78b1ff01-95bd-4e97-82a7-55ce1323c1fc" {
		t.Errorf("production fixture lost: got (%q, %q)", ri.Version, ri.BuildID)
	}
}
