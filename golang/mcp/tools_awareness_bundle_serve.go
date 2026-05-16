package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/globulario/awareness/bundlesync"
)

// ── Phase B: serve the locally-installed awareness bundle to the cluster ─────
//
// Two MCP tools and one HTTP handler. All read-only; all serve EXACTLY the
// bundle pointed at by /var/lib/globular/awareness/current — never an
// arbitrary path, never a sibling file, never anything chosen by the caller.
//
// Tools:
//   mcp.awareness_bundle_manifest — returns Manifest + state (no bundle bytes)
//   mcp.awareness_bundle_stream   — returns metadata + stream URL/path
//
// HTTP handler:
//   GET /awareness/bundle  → application/octet-stream of the active bundle
//
// Boundary kept here:
//   - We do NOT extract, install, or modify any path. Phase B is "serve only."
//   - We do NOT accept any path-like argument from callers. The active bundle
//     is the only thing servable; arbitrary file selection is structurally
//     impossible (no input parameter exists for it).
//   - Any path-like argument the caller injects (path, file, name, target)
//     is rejected with a clear error so a misbehaving client can't pretend
//     this is a generic file-fetch tool.

// activeBundleLayout describes the canonical on-disk bundle layout the
// serve tools read from. Variables (not constants) so tests can swap them.
var (
	activeBundleDir      = "/var/lib/globular/awareness/current"
	activeBundleFilename = "bundle.tar.gz"
	activeManifestFile   = "manifest.json"
)

// awarenessBundleStreamPath is the HTTPS path the stream tool advertises.
// It is NOT user-controlled — handler resolves it to the active bundle file.
const awarenessBundleStreamPath = "/awareness/bundle"

// awarenessBundleManifestPath serves the manifest as JSON over plain HTTPS so
// pullers don't need a JSON-RPC client. Same content as the
// mcp.awareness_bundle_manifest tool, just served at a fixed URL.
const awarenessBundleManifestPath = "/awareness/manifest"

// pathLikeArgKeys are caller-supplied arg names that COULD be exploited to
// select a different file. We reject them all so Phase B's "serve only the
// active bundle" contract is enforced even against a malicious or buggy client.
var pathLikeArgKeys = []string{"path", "file", "name", "target", "src", "source"}

// registerAwarenessBundleServeTools registers the two Phase-B serve tools.
// Independent from registerAwarenessTools so it can be wired in MCP-only
// (no awareness graph required to serve a bundle).
func registerAwarenessBundleServeTools(s *server) {
	registerAwarenessBundleManifestTool(s)
	registerAwarenessBundleStreamTool(s)
	registerAwarenessBundleFreshnessTool(s)
}

// ── mcp.awareness_bundle_manifest ─────────────────────────────────────────────

func registerAwarenessBundleManifestTool(s *server) {
	s.register(toolDef{
		Name: "mcp.awareness_bundle_manifest",
		Description: `Returns the manifest of the awareness bundle currently installed on this node.
Read-only. Serves only /var/lib/globular/awareness/current — never an arbitrary path.
When no bundle is installed, returns state=AWARENESS_BUNDLE_MISSING (this is the
cold-bootstrap safe state; an empty cluster has no source to pull from yet).`,
		InputSchema: inputSchema{Type: "object"},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if violation := rejectPathLikeArgs(args); violation != nil {
			return violation, nil
		}

		st, m, _, sha, size, err := snapshotActiveBundle()
		if err != nil {
			return manifestErrorPayload(st, err), nil
		}
		out := map[string]interface{}{
			"state":          st,
			"size_bytes":     size,
			"sha256":         sha,
			"served_at":      time.Now().UTC().Format(time.RFC3339),
			"node_advertise": s.cfg.HTTPAdvertiseHost,
		}
		// Only include the manifest field when we actually have one. A typed
		// nil *bundlesync.Manifest stored in interface{} is not == nil for
		// callers; omitting the field keeps the contract clean.
		if m != nil {
			out["manifest"] = m
		}
		return out, nil
	})
}

// ── mcp.awareness_bundle_stream ───────────────────────────────────────────────

func registerAwarenessBundleStreamTool(s *server) {
	s.register(toolDef{
		Name: "mcp.awareness_bundle_stream",
		Description: `Returns the metadata + stream location for the awareness bundle currently
installed on this node. Bytes are served via HTTPS at /awareness/bundle as
application/octet-stream — NOT base64 in JSON — so callers can pull large
bundles without buffering the whole thing.

Read-only and allowlisted for aggregator remote calls. Serves ONLY the active
bundle; any path-like argument is rejected. The streamed file is guaranteed
to hash to the manifest sha256.`,
		InputSchema: inputSchema{Type: "object"},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if violation := rejectPathLikeArgs(args); violation != nil {
			return violation, nil
		}

		st, m, _, sha, size, err := snapshotActiveBundle()
		if err != nil {
			return manifestErrorPayload(st, err), nil
		}

		// Build a stream URL the caller can pull directly. We always emit the
		// path; the URL is built only when we know the advertise host.
		streamURL := ""
		if s.cfg.HTTPAdvertiseHost != "" {
			scheme := "https"
			if !s.cfg.HTTPUseTLS {
				scheme = "http"
			}
			port := streamPort(s.cfg.HTTPListenAddr)
			streamURL = fmt.Sprintf("%s://%s:%d%s", scheme, s.cfg.HTTPAdvertiseHost, port, awarenessBundleStreamPath)
		}

		out := map[string]interface{}{
			"state":        st,
			"stream_path":  awarenessBundleStreamPath,
			"stream_url":   streamURL,
			"size_bytes":   size,
			"sha256":       sha,
			"content_type": "application/octet-stream",
			"served_at":    time.Now().UTC().Format(time.RFC3339),
		}
		if m != nil {
			out["manifest"] = m
		}
		return out, nil
	})
}

// ── HTTP handler (wired in transport_http.go) ────────────────────────────────

// awarenessBundleHTTPHandler streams the active bundle as application/octet-stream.
// GET only. Any other method returns 405. Any I/O error returns 500.
//
// This handler is the only path that returns bundle bytes. It does not accept
// any path/query parameter that could select a different file: the served
// path is resolved from activeBundleDir/activeBundleFilename and nothing else.
func (s *server) awarenessBundleHTTPHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	bundlePath := filepath.Join(activeBundleDir, activeBundleFilename)
	f, err := os.Open(bundlePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "AWARENESS_BUNDLE_MISSING", http.StatusNotFound)
			return
		}
		log.Printf("awareness bundle stream: open %s: %v", bundlePath, err)
		http.Error(w, "bundle unreadable", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		log.Printf("awareness bundle stream: stat %s: %v", bundlePath, err)
		http.Error(w, "bundle unreadable", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	w.Header().Set("X-Awareness-Bundle-File", activeBundleFilename)
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}

	if _, err := io.Copy(w, f); err != nil {
		// Connection may have closed mid-stream; can't change status now,
		// just log.
		log.Printf("awareness bundle stream: copy: %v", err)
	}
}

// awarenessManifestHTTPHandler serves the active manifest as JSON. GET only.
// Empty body with 404 when no manifest is installed (cold-bootstrap state) —
// callers (pullers) can check status code without parsing.
//
// This endpoint exists alongside mcp.awareness_bundle_manifest so a Phase-C
// puller can reach it with a plain HTTP client. Both surfaces serve the
// same manifest file; the JSON-RPC tool is for AI agents, the HTTP path is
// for tooling.
func (s *server) awarenessManifestHTTPHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	manifestPath := filepath.Join(activeBundleDir, activeManifestFile)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "AWARENESS_BUNDLE_MISSING", http.StatusNotFound)
			return
		}
		log.Printf("awareness manifest serve: read %s: %v", manifestPath, err)
		http.Error(w, "manifest unreadable", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.Write(data)
}

// ── helpers ──────────────────────────────────────────────────────────────────

// snapshotActiveBundle reads the active manifest and computes a fresh
// sha256/size of the active bundle file. Returns:
//   - state: AWARENESS_READY | AWARENESS_BUNDLE_MISSING | AWARENESS_BUNDLE_VERIFY_FAILED
//   - manifest pointer (may be nil when missing)
//   - bundlePath (always set even when missing — useful for error messages)
//   - sha (hex sha256 of the bundle file when readable, else "")
//   - size (bytes when readable, else 0)
//   - err non-nil only when the manifest exists but is malformed or the
//     bundle file exists but cannot be hashed.
func snapshotActiveBundle() (state bundlesync.State, m *bundlesync.Manifest, bundlePath, sha string, size int64, err error) {
	manifestPath := filepath.Join(activeBundleDir, activeManifestFile)
	bundlePath = filepath.Join(activeBundleDir, activeBundleFilename)

	if _, statErr := os.Stat(manifestPath); errors.Is(statErr, os.ErrNotExist) {
		return bundlesync.StateAwarenessBundleMissing, nil, bundlePath, "", 0, nil
	}

	loaded, err := bundlesync.LoadManifest(manifestPath)
	if err != nil {
		return bundlesync.StateAwarenessBundleVerifyFailed, nil, bundlePath, "", 0, err
	}
	m = loaded

	info, statErr := os.Stat(bundlePath)
	if errors.Is(statErr, os.ErrNotExist) {
		// Manifest present but bundle file is gone — treat as missing so
		// callers know there's nothing to stream.
		return bundlesync.StateAwarenessBundleMissing, m, bundlePath, "", 0, nil
	}
	if statErr != nil {
		return bundlesync.StateAwarenessBundleVerifyFailed, m, bundlePath, "", 0, statErr
	}
	size = info.Size()

	// Hash the bundle file fresh. We don't trust manifest.sha256 to be
	// canonical truth here — the served bytes are what matters. Callers
	// receive both: the manifest hash they should verify against and the
	// hash we just computed.
	hashed, hashErr := hashFile(bundlePath)
	if hashErr != nil {
		return bundlesync.StateAwarenessBundleVerifyFailed, m, bundlePath, "", size, hashErr
	}
	sha = hashed

	return bundlesync.StateAwarenessReady, m, bundlePath, sha, size, nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// rejectPathLikeArgs returns a non-nil error envelope when the caller
// supplied any pathLikeArgKey. The serve tools take no such arguments;
// receiving one signals either misuse or a path-traversal attempt and
// must be rejected unambiguously.
func rejectPathLikeArgs(args map[string]interface{}) map[string]interface{} {
	if len(args) == 0 {
		return nil
	}
	for _, k := range pathLikeArgKeys {
		if _, present := args[k]; present {
			return map[string]interface{}{
				"state":       bundlesync.StateAwarenessBundleVerifyFailed,
				"error_kind":  "ARG_REJECTED",
				"error":       fmt.Sprintf("argument %q is not accepted: this tool serves ONLY the active awareness bundle", k),
				"rejected_arg": k,
			}
		}
	}
	return nil
}

func manifestErrorPayload(state bundlesync.State, err error) map[string]interface{} {
	out := map[string]interface{}{
		"state": state,
	}
	if err != nil {
		out["error"] = err.Error()
	}
	return out
}

// streamPort extracts the listen port from cfg.HTTPListenAddr (":10260" or
// "0.0.0.0:10260"). Falls back to the canonical aggregator MCP port when the
// addr is empty or malformed; the stream URL is best-effort metadata, never
// the only way to reach the endpoint.
func streamPort(listen string) int {
	port, ok := parsePort(listen)
	if ok {
		return port
	}
	return aggregatorMCPPort
}

func parsePort(addr string) (int, bool) {
	if addr == "" {
		return 0, false
	}
	// trim host portion: ":10260" or "0.0.0.0:10260"
	colon := -1
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			colon = i
			break
		}
	}
	if colon < 0 || colon == len(addr)-1 {
		return 0, false
	}
	p := 0
	for _, c := range addr[colon+1:] {
		if c < '0' || c > '9' {
			return 0, false
		}
		p = p*10 + int(c-'0')
	}
	if p <= 0 || p > 65535 {
		return 0, false
	}
	return p, true
}

// ── shared lock for tests that swap activeBundleDir ──────────────────────────
//
// Production calls do not contend; tests may run subtests in parallel that
// each set activeBundleDir. Expose a Mutex so tests serialize on it.
var activeBundleTestMu sync.Mutex
