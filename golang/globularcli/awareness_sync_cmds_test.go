package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/globulario/services/golang/awareness/bundlesync"
)

// ── Phase C.5 CLI tests ───────────────────────────────────────────────────────
//
// These tests do NOT spin a network. The pull/sync commands hit the
// bundlesync.PullBundle code that already has its own httptest-backed tests
// in the bundlesync package; here we only verify:
//   - all 5 commands are registered under awarenessCmd
//   - common flags parse and resolve to sensible defaults
//   - status, verify, install can run end-to-end against on-disk fixtures
//   - all commands respect --json output mode

// resetFlags clears the package-level config struct between test runs.
func resetFlags(t *testing.T) {
	t.Helper()
	awarenessSyncCfg.from = ""
	awarenessSyncCfg.outDir = ""
	awarenessSyncCfg.manifestPath = ""
	awarenessSyncCfg.releaseIndex = ""
	awarenessSyncCfg.bundleRoot = ""
	awarenessSyncCfg.caPath = ""
	awarenessSyncCfg.json = false
	awarenessSyncCfg.timeoutSec = 0
	awarenessSyncCfg.expectVersion = ""
	awarenessSyncCfg.expectBuildID = ""
}

// installableBundle writes a (gzip)tar containing graph.db + a sidecar
// manifest into dir. Returns the bundle path, manifest path, and manifest.
func installableBundle(t *testing.T, dir, version, buildID string) (string, string, bundlesync.Manifest) {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	body := []byte("fake graph for " + version + "/" + buildID)
	hdr := &tar.Header{Name: "graph.db", Mode: 0644, Size: int64(len(body)), Typeflag: tar.TypeReg}
	tw.WriteHeader(hdr)
	tw.Write(body)
	tw.Close()
	gz.Close()
	data := buf.Bytes()

	bundlePath := filepath.Join(dir, "bundle.tar.gz")
	if err := os.WriteFile(bundlePath, data, 0644); err != nil {
		t.Fatalf("write bundle: %v", err)
	}
	h := sha256.Sum256(data)
	m := bundlesync.Manifest{
		Name:          bundlesync.BundleName,
		Version:       version,
		BuildID:       buildID,
		SchemaVersion: "awareness.bundle.v1",
		SHA256:        hex.EncodeToString(h[:]),
		SizeBytes:     int64(len(data)),
	}
	mb, _ := json.MarshalIndent(m, "", "  ")
	manifestPath := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(manifestPath, mb, 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return bundlePath, manifestPath, m
}

func writeReleaseIndex(t *testing.T, dir string, ri bundlesync.ReleaseIndex) string {
	t.Helper()
	p := filepath.Join(dir, "release-index.json")
	data, _ := json.MarshalIndent(ri, "", "  ")
	if err := os.WriteFile(p, data, 0644); err != nil {
		t.Fatalf("write release index: %v", err)
	}
	return p
}

// 1. All 5 commands registered.
func TestAwarenessSyncCommandsRegistered(t *testing.T) {
	want := []string{"status", "pull", "verify", "install", "sync"}
	for _, name := range want {
		var found bool
		for _, c := range awarenessCmd.Commands() {
			if c.Use == name || strings.HasPrefix(c.Use, name+" ") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("command %q not registered under awarenessCmd", name)
		}
	}
}

// 2. status command — fresh bundle reports AWARENESS_READY in JSON.
func TestAwarenessStatusFresh(t *testing.T) {
	resetFlags(t)
	bundleRoot := t.TempDir()

	versionedDir := filepath.Join(bundleRoot, "installed", "v1.2.30", "abc123")
	if err := os.MkdirAll(versionedDir, 0755); err != nil {
		t.Fatalf("mkdir versioned: %v", err)
	}
	m := bundlesync.Manifest{
		Name: bundlesync.BundleName, Version: "v1.2.30", BuildID: "abc123",
		SchemaVersion: "awareness.bundle.v1", SHA256: "f00d",
	}
	mb, _ := json.MarshalIndent(m, "", "  ")
	os.WriteFile(filepath.Join(versionedDir, "manifest.json"), mb, 0644)
	os.WriteFile(filepath.Join(versionedDir, "graph.db"), []byte("g"), 0644)
	os.Symlink(versionedDir, filepath.Join(bundleRoot, "current"))

	indexDir := t.TempDir()
	riPath := writeReleaseIndex(t, indexDir, bundlesync.ReleaseIndex{Version: "v1.2.30", BuildID: "abc123"})

	awarenessSyncCfg.bundleRoot = bundleRoot
	awarenessSyncCfg.releaseIndex = riPath
	awarenessSyncCfg.json = true

	var out bytes.Buffer
	awarenessStatusCmd.SetOut(&out)
	awarenessStatusCmd.SetErr(&out)
	awarenessStatusCmd.SetArgs([]string{})
	if err := awarenessStatusCmd.RunE(awarenessStatusCmd, nil); err != nil {
		t.Fatalf("status RunE: %v", err)
	}

	var rep statusReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("json: %v\noutput=%s", err, out.String())
	}
	if rep.State != bundlesync.StateAwarenessReady {
		t.Errorf("state=%s, want AWARENESS_READY (out=%s)", rep.State, out.String())
	}
	if !rep.OK {
		t.Errorf("ok=false; want true")
	}
}

// 3. status command — missing bundle reports AWARENESS_BUNDLE_MISSING cleanly.
func TestAwarenessStatusMissing(t *testing.T) {
	resetFlags(t)
	bundleRoot := t.TempDir()
	indexDir := t.TempDir()
	riPath := writeReleaseIndex(t, indexDir, bundlesync.ReleaseIndex{Version: "v1.2.30", BuildID: "abc123"})

	awarenessSyncCfg.bundleRoot = bundleRoot
	awarenessSyncCfg.releaseIndex = riPath
	awarenessSyncCfg.json = true

	var out bytes.Buffer
	awarenessStatusCmd.SetOut(&out)
	awarenessStatusCmd.SetErr(&out)
	if err := awarenessStatusCmd.RunE(awarenessStatusCmd, nil); err != nil {
		t.Fatalf("status RunE: %v", err)
	}
	var rep statusReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("json: %v", err)
	}
	if rep.State != bundlesync.StateAwarenessBundleMissing {
		t.Errorf("state=%s, want AWARENESS_BUNDLE_MISSING", rep.State)
	}
}

// 4. verify command — happy path.
func TestAwarenessVerifyPasses(t *testing.T) {
	resetFlags(t)
	scratch := t.TempDir()
	bp, mp, m := installableBundle(t, scratch, "v1.2.30", "abc123")
	indexDir := t.TempDir()
	riPath := writeReleaseIndex(t, indexDir, bundlesync.ReleaseIndex{Version: m.Version, BuildID: m.BuildID})

	awarenessSyncCfg.releaseIndex = riPath
	awarenessSyncCfg.manifestPath = mp
	awarenessSyncCfg.json = true

	var out bytes.Buffer
	awarenessVerifyCmd.SetOut(&out)
	awarenessVerifyCmd.SetErr(&out)
	if err := awarenessVerifyCmd.RunE(awarenessVerifyCmd, []string{bp}); err != nil {
		t.Fatalf("verify RunE: %v", err)
	}
	var res bundlesync.VerifyResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("json: %v\nout=%s", err, out.String())
	}
	if !res.OK {
		t.Errorf("verify OK=false; state=%s reason=%s", res.State, res.Reason)
	}
}

// 5. install command — happy path.
func TestAwarenessInstallSucceeds(t *testing.T) {
	resetFlags(t)
	scratch := t.TempDir()
	bp, mp, m := installableBundle(t, scratch, "v1.2.30", "abc123")
	bundleRoot := t.TempDir()
	indexDir := t.TempDir()
	riPath := writeReleaseIndex(t, indexDir, bundlesync.ReleaseIndex{Version: m.Version, BuildID: m.BuildID})

	awarenessSyncCfg.bundleRoot = bundleRoot
	awarenessSyncCfg.releaseIndex = riPath
	awarenessSyncCfg.manifestPath = mp
	awarenessSyncCfg.json = true

	var out bytes.Buffer
	awarenessInstallCmd.SetOut(&out)
	awarenessInstallCmd.SetErr(&out)
	if err := awarenessInstallCmd.RunE(awarenessInstallCmd, []string{bp}); err != nil {
		t.Fatalf("install RunE: %v\nout=%s", err, out.String())
	}
	var res bundlesync.InstallResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("json: %v\nout=%s", err, out.String())
	}
	if !res.OK {
		t.Errorf("install OK=false; state=%s reason=%s", res.State, res.Reason)
	}
	target, _ := os.Readlink(filepath.Join(bundleRoot, "current"))
	want := filepath.Join(bundleRoot, "installed", m.Version, m.BuildID)
	if target != want {
		t.Errorf("current → %s, want %s", target, want)
	}
}

// 6. install command — verify failure (wrong sha256 in manifest) returns error
// and current symlink does not exist.
func TestAwarenessInstallVerifyFailure(t *testing.T) {
	resetFlags(t)
	scratch := t.TempDir()
	bp, mp, m := installableBundle(t, scratch, "v1.2.30", "abc123")
	// Tamper the manifest's sha256.
	m.SHA256 = "deadbeef00000000000000000000000000000000000000000000000000000000"
	mb, _ := json.MarshalIndent(m, "", "  ")
	os.WriteFile(mp, mb, 0644)

	bundleRoot := t.TempDir()
	indexDir := t.TempDir()
	riPath := writeReleaseIndex(t, indexDir, bundlesync.ReleaseIndex{Version: m.Version, BuildID: m.BuildID})

	awarenessSyncCfg.bundleRoot = bundleRoot
	awarenessSyncCfg.releaseIndex = riPath
	awarenessSyncCfg.manifestPath = mp

	var out bytes.Buffer
	awarenessInstallCmd.SetOut(&out)
	awarenessInstallCmd.SetErr(&out)
	if err := awarenessInstallCmd.RunE(awarenessInstallCmd, []string{bp}); err == nil {
		t.Fatal("install RunE did not return an error for tampered sha256")
	}
	if _, err := os.Lstat(filepath.Join(bundleRoot, "current")); err == nil {
		t.Error("current symlink must not exist after verify failure")
	}
}

// 7. pull command — missing --from returns a clear error without making any
// network call.
func TestAwarenessPullRequiresFrom(t *testing.T) {
	resetFlags(t)
	awarenessSyncCfg.expectVersion = "v1.2.30"
	awarenessSyncCfg.expectBuildID = "abc123"

	awarenessPullCmd.SetContext(context.Background())
	if err := awarenessPullCmd.RunE(awarenessPullCmd, nil); err == nil {
		t.Fatal("pull RunE did not error when --from is missing")
	}
}

// 8. resolveExpectedRelease prefers --version+--build-id over the on-disk index.
func TestResolveExpectedReleasePrefersFlags(t *testing.T) {
	resetFlags(t)
	indexDir := t.TempDir()
	awarenessSyncCfg.releaseIndex = writeReleaseIndex(t, indexDir, bundlesync.ReleaseIndex{Version: "v0.0.1", BuildID: "old"})
	awarenessSyncCfg.expectVersion = "v9.9.99"
	awarenessSyncCfg.expectBuildID = "new"

	ri, err := resolveExpectedRelease()
	if err != nil {
		t.Fatalf("resolveExpectedRelease: %v", err)
	}
	if ri.Version != "v9.9.99" || ri.BuildID != "new" {
		t.Errorf("flag override ignored: got %+v", ri)
	}
}

// 9. resolveExpectedRelease falls back to release-index when flags are empty.
func TestResolveExpectedReleaseFromIndex(t *testing.T) {
	resetFlags(t)
	indexDir := t.TempDir()
	awarenessSyncCfg.releaseIndex = writeReleaseIndex(t, indexDir, bundlesync.ReleaseIndex{Version: "v1.2.30", BuildID: "abc123"})

	ri, err := resolveExpectedRelease()
	if err != nil {
		t.Fatalf("resolveExpectedRelease: %v", err)
	}
	if ri.Version != "v1.2.30" || ri.BuildID != "abc123" {
		t.Errorf("got %+v, want v1.2.30/abc123", ri)
	}
}

// 10. defaultManifestSidecar resolves the conventional sidecar path.
func TestDefaultManifestSidecar(t *testing.T) {
	dir := t.TempDir()
	bundlePath := filepath.Join(dir, "bundle.tar.gz")
	// When manifest.json sits next to it, that's preferred.
	os.WriteFile(filepath.Join(dir, "manifest.json"), []byte("{}"), 0644)
	got := defaultManifestSidecar(bundlePath)
	if got != filepath.Join(dir, "manifest.json") {
		t.Errorf("got %s, want sibling manifest.json", got)
	}
	// When no sibling manifest, fall back to <bundle>.manifest.json.
	dir2 := t.TempDir()
	bundlePath2 := filepath.Join(dir2, "awareness-1.2.30.tar.gz")
	got = defaultManifestSidecar(bundlePath2)
	wantSuffix := ".manifest.json"
	if !strings.HasSuffix(got, wantSuffix) {
		t.Errorf("got %s, want suffix %s", got, wantSuffix)
	}
}

// 11. loadReleaseIndexFromDisk handles both flat and {"active":{...}} shapes.
func TestLoadReleaseIndexAcceptsBothShapes(t *testing.T) {
	dir := t.TempDir()
	flat := filepath.Join(dir, "flat.json")
	os.WriteFile(flat, []byte(`{"version":"v1","build_id":"b"}`), 0644)
	if ri, err := loadReleaseIndexFromDisk(flat); err != nil || ri.Version != "v1" {
		t.Errorf("flat: %+v err=%v", ri, err)
	}

	nested := filepath.Join(dir, "nested.json")
	os.WriteFile(nested, []byte(`{"active":{"version":"v2","build_id":"b2"}}`), 0644)
	if ri, err := loadReleaseIndexFromDisk(nested); err != nil || ri.Version != "v2" {
		t.Errorf("nested: %+v err=%v", ri, err)
	}

	bad := filepath.Join(dir, "bad.json")
	os.WriteFile(bad, []byte(`{"foo":"bar"}`), 0644)
	if _, err := loadReleaseIndexFromDisk(bad); err == nil {
		t.Error("expected error for shape with no version/build_id")
	}
}
