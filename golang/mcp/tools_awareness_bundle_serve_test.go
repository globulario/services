package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/awareness/bundlesync"
)

// ── Phase B acceptance tests ──────────────────────────────────────────────────
//
// 1. mcp.awareness_bundle_manifest returns manifest for the local active bundle.
// 2. It reports missing state cleanly if no bundle exists.
// 3. mcp.awareness_bundle_stream streams bytes, not base64 JSON.
// 4. Streamed bytes hash to the manifest sha256.
// 5. Stream tool refuses path traversal or arbitrary file selection.
// 6. Stream tool is read-only and allowlisted for aggregator remote calls.
// 7. Tests cover present bundle, missing bundle, bad manifest, and stream/hash match.
//
// The serve tools read activeBundleDir/{manifest.json,bundle.tar.gz}. We swap
// activeBundleDir to a t.TempDir() per test using setupActiveBundleDir.

// setupActiveBundleDir overrides activeBundleDir for the duration of the test
// and serializes against other tests that override it. Test cleanup restores
// the previous value.
func setupActiveBundleDir(t *testing.T) string {
	t.Helper()
	activeBundleTestMu.Lock()
	prev := activeBundleDir
	dir := t.TempDir()
	activeBundleDir = dir
	t.Cleanup(func() {
		activeBundleDir = prev
		activeBundleTestMu.Unlock()
	})
	return dir
}

// installFakeBundle writes a minimal valid (gzip)tar bundle + manifest into
// the active bundle dir and returns the manifest claim and the actual bytes
// of the bundle file. The manifest sha256 is computed from the actual bytes
// so happy-path tests can verify hash match end-to-end.
func installFakeBundle(t *testing.T, dir string) (manifest bundlesync.Manifest, bundleBytes []byte) {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	body := []byte("fake graph.db content " + t.Name())
	hdr := &tar.Header{
		Name:     "graph.db",
		Mode:     0644,
		Size:     int64(len(body)),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("tar header: %v", err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatalf("tar body: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gz close: %v", err)
	}
	bundleBytes = buf.Bytes()

	bundlePath := filepath.Join(dir, activeBundleFilename)
	if err := os.WriteFile(bundlePath, bundleBytes, 0644); err != nil {
		t.Fatalf("write bundle: %v", err)
	}

	h := sha256.Sum256(bundleBytes)
	manifest = bundlesync.Manifest{
		Name:          bundlesync.BundleName,
		Version:       "v1.2.30",
		BuildID:       "abc123",
		SchemaVersion: "awareness.bundle.v1",
		SHA256:        hex.EncodeToString(h[:]),
		SizeBytes:     int64(len(bundleBytes)),
		CreatedAt:     "2026-05-09T00:00:00Z",
		SourceNodeID:  "node-test",
	}
	mb, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, activeManifestFile), mb, 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return manifest, bundleBytes
}

// newServeTestServer registers Phase-B tools on a freshly built server and
// returns it. Disables every tool group except awareness so we don't pull in
// network-dependent groups.
func newServeTestServer(t *testing.T) *server {
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
	cfg.ToolGroups.Awareness = false  // we register only the serve subset below
	cfg.ToolGroups.Aggregator = false

	s := newServer(cfg)
	registerAwarenessBundleServeTools(s)
	return s
}

// ── Test 1 + 2: manifest present and missing ─────────────────────────────────

// 1. Present bundle: tool returns the manifest fields verbatim and reports
// AWARENESS_READY with a fresh sha256 that matches.
func TestAwarenessBundleManifestPresent(t *testing.T) {
	dir := setupActiveBundleDir(t)
	manifest, bundleBytes := installFakeBundle(t, dir)
	s := newServeTestServer(t)

	res, err := s.callTool(context.Background(), "mcp.awareness_bundle_manifest", nil)
	if err != nil {
		t.Fatalf("tool error: %v", err)
	}
	m, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", res)
	}

	if m["state"] != bundlesync.StateAwarenessReady {
		t.Errorf("state = %v, want AWARENESS_READY", m["state"])
	}
	if m["sha256"] != manifest.SHA256 {
		t.Errorf("sha256 = %v, want %v", m["sha256"], manifest.SHA256)
	}
	if size, ok := m["size_bytes"].(int64); !ok || size != int64(len(bundleBytes)) {
		t.Errorf("size_bytes = %v, want %d", m["size_bytes"], len(bundleBytes))
	}
	mf, _ := m["manifest"].(*bundlesync.Manifest)
	if mf == nil {
		t.Fatal("manifest field missing or wrong type")
	}
	if mf.Version != manifest.Version {
		t.Errorf("manifest.version = %q, want %q", mf.Version, manifest.Version)
	}
	if mf.BuildID != manifest.BuildID {
		t.Errorf("manifest.build_id = %q, want %q", mf.BuildID, manifest.BuildID)
	}
	if mf.Name != bundlesync.BundleName {
		t.Errorf("manifest.name = %q, want %q", mf.Name, bundlesync.BundleName)
	}
}

// 2. Missing bundle: tool reports AWARENESS_BUNDLE_MISSING cleanly.
// No error returned; this is the cold-bootstrap safe state.
func TestAwarenessBundleManifestMissing(t *testing.T) {
	_ = setupActiveBundleDir(t) // empty dir; no manifest, no bundle
	s := newServeTestServer(t)

	res, err := s.callTool(context.Background(), "mcp.awareness_bundle_manifest", nil)
	if err != nil {
		t.Fatalf("tool error: %v", err)
	}
	m := res.(map[string]interface{})
	if m["state"] != bundlesync.StateAwarenessBundleMissing {
		t.Errorf("state = %v, want AWARENESS_BUNDLE_MISSING", m["state"])
	}
	if _, present := m["manifest"]; present {
		t.Errorf("manifest field must be omitted when missing, got %v", m["manifest"])
	}
}

// 7a. Bad manifest (malformed JSON) → AWARENESS_BUNDLE_VERIFY_FAILED.
// The tool must not panic, must not stream, and must surface the parse error.
func TestAwarenessBundleManifestMalformed(t *testing.T) {
	dir := setupActiveBundleDir(t)
	if err := os.WriteFile(filepath.Join(dir, activeManifestFile), []byte("{not json"), 0644); err != nil {
		t.Fatalf("write bad manifest: %v", err)
	}
	s := newServeTestServer(t)

	res, _ := s.callTool(context.Background(), "mcp.awareness_bundle_manifest", nil)
	m := res.(map[string]interface{})
	if m["state"] != bundlesync.StateAwarenessBundleVerifyFailed {
		t.Errorf("state = %v, want AWARENESS_BUNDLE_VERIFY_FAILED", m["state"])
	}
	if _, ok := m["error"]; !ok {
		t.Errorf("expected error field for malformed manifest; got %v", m)
	}
}

// ── Test 3 + 4: stream tool returns metadata + URL; HTTP handler streams bytes
//                that hash to the manifest sha256 ──────────────────────────────

// stream_tool returns metadata + stream_path. Bundle bytes are NOT in the
// response (no base64). Streamed bytes from /awareness/bundle hash to the
// manifest sha256.
func TestAwarenessBundleStreamReturnsMetadataAndBytesHash(t *testing.T) {
	dir := setupActiveBundleDir(t)
	manifest, bundleBytes := installFakeBundle(t, dir)
	s := newServeTestServer(t)

	// Tool first.
	res, err := s.callTool(context.Background(), "mcp.awareness_bundle_stream", nil)
	if err != nil {
		t.Fatalf("tool error: %v", err)
	}
	m := res.(map[string]interface{})

	if m["state"] != bundlesync.StateAwarenessReady {
		t.Errorf("state = %v, want AWARENESS_READY", m["state"])
	}
	if m["stream_path"] != awarenessBundleStreamPath {
		t.Errorf("stream_path = %v, want %s", m["stream_path"], awarenessBundleStreamPath)
	}
	if m["content_type"] != "application/octet-stream" {
		t.Errorf("content_type = %v, want application/octet-stream", m["content_type"])
	}
	// CRITICAL: bytes themselves must NOT appear in the JSON response.
	for _, suspect := range []string{"bytes", "data", "content", "body", "payload", "base64"} {
		if _, ok := m[suspect]; ok {
			t.Errorf("response contains %q field — bundle bytes must not be in JSON, only via /awareness/bundle", suspect)
		}
	}
	if m["sha256"] != manifest.SHA256 {
		t.Errorf("sha256 = %v, want %v", m["sha256"], manifest.SHA256)
	}

	// HTTP handler next: GET the stream and verify hash matches manifest.
	mux := http.NewServeMux()
	mux.HandleFunc(awarenessBundleStreamPath, s.awarenessBundleHTTPHandler)
	httpSrv := httptest.NewServer(mux)
	defer httpSrv.Close()

	resp, err := http.Get(httpSrv.URL + awarenessBundleStreamPath)
	if err != nil {
		t.Fatalf("GET stream: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET stream status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/octet-stream" {
		t.Errorf("Content-Type = %q, want application/octet-stream", ct)
	}

	streamed, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read stream: %v", err)
	}
	if !bytes.Equal(streamed, bundleBytes) {
		t.Errorf("streamed bytes do not match bundle file (%d vs %d bytes)", len(streamed), len(bundleBytes))
	}

	h := sha256.Sum256(streamed)
	got := hex.EncodeToString(h[:])
	if got != manifest.SHA256 {
		t.Errorf("streamed sha256 = %s, manifest claims %s", got, manifest.SHA256)
	}
}

// HTTP handler returns 404 when no bundle is installed.
func TestAwarenessBundleStreamHTTP404WhenMissing(t *testing.T) {
	_ = setupActiveBundleDir(t) // empty
	s := newServeTestServer(t)

	mux := http.NewServeMux()
	mux.HandleFunc(awarenessBundleStreamPath, s.awarenessBundleHTTPHandler)
	httpSrv := httptest.NewServer(mux)
	defer httpSrv.Close()

	resp, err := http.Get(httpSrv.URL + awarenessBundleStreamPath)
	if err != nil {
		t.Fatalf("GET stream: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for missing bundle", resp.StatusCode)
	}
}

// HTTP handler rejects POST/PUT/DELETE with 405. Read-only invariant.
func TestAwarenessBundleStreamHTTPMethodNotAllowed(t *testing.T) {
	dir := setupActiveBundleDir(t)
	installFakeBundle(t, dir)
	s := newServeTestServer(t)

	mux := http.NewServeMux()
	mux.HandleFunc(awarenessBundleStreamPath, s.awarenessBundleHTTPHandler)
	httpSrv := httptest.NewServer(mux)
	defer httpSrv.Close()

	for _, method := range []string{"POST", "PUT", "DELETE", "PATCH"} {
		req, _ := http.NewRequest(method, httpSrv.URL+awarenessBundleStreamPath, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s: %v", method, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("%s status = %d, want 405", method, resp.StatusCode)
		}
	}
}

// ── Test 5: stream tool refuses path traversal / arbitrary selection ─────────

// Any path-like argument is rejected before the tool reads anything from disk.
// This guards against a malicious caller trying to use the tool as a
// generic file-fetch.
func TestAwarenessBundleStreamRejectsPathLikeArgs(t *testing.T) {
	dir := setupActiveBundleDir(t)
	installFakeBundle(t, dir) // valid bundle in place; rejection must NOT leak it
	s := newServeTestServer(t)

	cases := []map[string]interface{}{
		{"path": "../../etc/passwd"},
		{"file": "/etc/shadow"},
		{"name": "../bundle.tar.gz"},
		{"target": "graph.db"},
		{"src": "anything"},
		{"source": "../../../"},
	}
	for _, args := range cases {
		res, err := s.callTool(context.Background(), "mcp.awareness_bundle_stream", args)
		if err != nil {
			t.Fatalf("tool error for %v: %v", args, err)
		}
		m := res.(map[string]interface{})
		if m["error_kind"] != "ARG_REJECTED" {
			t.Errorf("args=%v: error_kind = %v, want ARG_REJECTED", args, m["error_kind"])
		}
		// Rejected response must NOT carry stream_url / stream_path /
		// manifest content — it must look nothing like a successful response.
		for _, leakField := range []string{"stream_url", "stream_path", "sha256", "manifest"} {
			if _, ok := m[leakField]; ok {
				t.Errorf("args=%v: rejected response leaks %q — must not", args, leakField)
			}
		}
	}
}

// Same protection on the manifest tool — it also takes no args.
func TestAwarenessBundleManifestRejectsPathLikeArgs(t *testing.T) {
	dir := setupActiveBundleDir(t)
	installFakeBundle(t, dir)
	s := newServeTestServer(t)

	res, _ := s.callTool(context.Background(), "mcp.awareness_bundle_manifest",
		map[string]interface{}{"path": "../../etc/passwd"})
	m := res.(map[string]interface{})
	if m["error_kind"] != "ARG_REJECTED" {
		t.Errorf("error_kind = %v, want ARG_REJECTED", m["error_kind"])
	}
}

// ── Test 6: tools allowlisted for aggregator remote calls ────────────────────

// The aggregator policy must permit both Phase-B tools so an aggregator on
// node A can ask node B for B's manifest/stream metadata. Bundle bytes flow
// over /awareness/bundle, not through the JSON-RPC call.
func TestAwarenessBundleToolsAreAllowlistedForAggregator(t *testing.T) {
	for _, tool := range []string{
		"mcp.awareness_bundle_manifest",
		"mcp.awareness_bundle_stream",
	} {
		if !IsRemoteToolAllowed(tool) {
			t.Errorf("tool %q must be in the aggregator allowlist", tool)
		}
		if ClassifyRemoteToolSafety(tool) != "READ_ONLY" {
			t.Errorf("tool %q safety = %q, want READ_ONLY", tool, ClassifyRemoteToolSafety(tool))
		}
	}
}

// ── Test 7d: stream tool with bad manifest still does not stream ─────────────
//
// If the manifest is malformed, we report VERIFY_FAILED — not a stream URL.
// The stream URL would imply "go fetch this thing" but we have no usable
// manifest for the receiver to verify against.
func TestAwarenessBundleStreamBadManifestReturnsError(t *testing.T) {
	dir := setupActiveBundleDir(t)
	if err := os.WriteFile(filepath.Join(dir, activeManifestFile), []byte("{not json"), 0644); err != nil {
		t.Fatalf("write bad manifest: %v", err)
	}
	s := newServeTestServer(t)

	res, _ := s.callTool(context.Background(), "mcp.awareness_bundle_stream", nil)
	m := res.(map[string]interface{})
	if m["state"] != bundlesync.StateAwarenessBundleVerifyFailed {
		t.Errorf("state = %v, want AWARENESS_BUNDLE_VERIFY_FAILED", m["state"])
	}
	if _, ok := m["stream_url"]; ok {
		t.Errorf("stream_url must not be present when manifest is malformed; got %v", m["stream_url"])
	}
	if _, ok := m["stream_path"]; ok {
		t.Errorf("stream_path must not be present when manifest is malformed")
	}
}
