package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/globulario/awareness/bundlesync"
)

// ── mcp.awareness_freshness_status ───────────────────────────────────────────
//
// Reads the release-index and the active awareness manifest, runs
// bundlesync.CheckAwarenessFreshness, and returns a structured verdict
// suitable for the Day-1 ladder, dashboards, and the aggregator's per-node
// view.
//
// What this tool is NOT:
//
//   - It does not stream the bundle (that's mcp.awareness_bundle_stream).
//   - It does not rebuild or generate any graph (Phase A/B contract).
//   - It does not pull, install, or sync (Phase C territory).
//
// What this tool IS:
//
//   - A pure-read freshness verdict for an operator/agent to ask
//     "should I be marking this node AWARENESS_READY?"

// releaseIndexPath is the canonical location of release-index.json.
// Variable (not const) so tests can swap it for a t.TempDir() fixture.
var releaseIndexPath = "/var/lib/globular/release-index.json"

// localBinaryInfo is hooked here so tests can inject a deterministic value
// without touching runtime/debug. In production we read whatever the build
// pipeline embedded; missing fields are not a hard failure.
var localBinaryInfo = readLocalBinaryInfo

// registerAwarenessBundleFreshnessTool registers mcp.awareness_freshness_status.
// Wired by registerAwarenessBundleServeTools so it ships with the Phase-B group.
func registerAwarenessBundleFreshnessTool(s *server) {
	s.register(toolDef{
		Name: "mcp.awareness_freshness_status",
		Description: `Returns the freshness verdict for the awareness bundle on this node.
Combines: release-index match (version/build_id), schema support, optional local-binary
correlation. Read-only — does NOT stream the bundle, generate a graph, or trigger sync.

State mapping:
  - AWARENESS_READY                       — bundle matches release-index and verifies
  - AWARENESS_BUNDLE_MISSING              — no manifest installed (cold-bootstrap safe state)
  - AWARENESS_BUNDLE_STALE                — same release line, older build_id
  - AWARENESS_BUNDLE_MISMATCH             — bundle version differs from release-index
  - AWARENESS_BUNDLE_SCHEMA_UNSUPPORTED   — bundle schema is newer than this binary supports
  - AWARENESS_BUNDLE_VERIFY_FAILED        — manifest unreadable or release-index missing`,
		InputSchema: inputSchema{Type: "object"},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if violation := rejectPathLikeArgs(args); violation != nil {
			return violation, nil
		}

		out := map[string]interface{}{
			"served_at":          time.Now().UTC().Format(time.RFC3339),
			"release_index_path": releaseIndexPath,
			"manifest_path":      filepath.Join(activeBundleDir, activeManifestFile),
		}

		// Load release-index first; without it we cannot decide freshness.
		ri, riErr := loadReleaseIndex(releaseIndexPath)
		if riErr != nil {
			out["state"] = bundlesync.StateAwarenessBundleVerifyFailed
			out["reason"] = riErr.Error()
			return out, nil
		}
		out["release_index"] = ri

		// Load the active manifest. Missing manifest → AWARENESS_BUNDLE_MISSING
		// is the cold-bootstrap state and not an error.
		manifestPath := filepath.Join(activeBundleDir, activeManifestFile)
		var manifest *bundlesync.Manifest
		if _, statErr := os.Stat(manifestPath); errors.Is(statErr, os.ErrNotExist) {
			out["state"] = bundlesync.StateAwarenessBundleMissing
			out["reason"] = "no manifest installed; cold-bootstrap safe state"
			return out, nil
		}
		loaded, err := bundlesync.LoadManifest(manifestPath)
		if err != nil {
			out["state"] = bundlesync.StateAwarenessBundleVerifyFailed
			out["reason"] = fmt.Sprintf("manifest unreadable: %v", err)
			return out, nil
		}
		manifest = loaded

		// Local binary info is best-effort; empty fields are not failures.
		lb := localBinaryInfo()

		report := bundlesync.CheckAwarenessFreshness(manifest, ri, lb)

		out["state"] = report.State
		out["reason"] = report.Reason
		out["ok"] = report.OK
		out["manifest"] = manifest
		out["version_matches_release"] = report.VersionMatchesRelease
		out["build_id_matches_release"] = report.BuildIDMatchesRelease
		out["schema_supported"] = report.SchemaSupported
		out["graph_hash_present"] = report.GraphHashPresent
		out["source_commit_present"] = report.SourceCommitPresent

		if lb != nil && (lb.Version != "" || lb.BuildID != "") {
			out["local_binary"] = map[string]interface{}{
				"version":        lb.Version,
				"build_id":       lb.BuildID,
				"version_match":  report.LocalBinaryVersionMatch,
				"build_id_match": report.LocalBinaryBuildIDMatch,
			}
		}
		return out, nil
	})
}

// loadReleaseIndex reads the canonical release-index.json. Tolerant of fields
// the binary doesn't recognize — only Version/BuildID are required for
// freshness; the rest of the file (other releases, signatures, etc.) is
// ignored.
//
// Three shapes are accepted, in priority order:
//
//  1. The canonical BOM produced by the repository / CI release pipeline
//     ({"schema_version": "globular.repository.index/v{1,2}", "packages":
//     [{"kind": "AWARENESS_BUNDLE", "version": ..., "build_id": ...}, ...]}).
//     We pick the AWARENESS_BUNDLE entry; if multiple exist, the first one
//     whose name matches bundlesync.BundleName wins, otherwise the first
//     AWARENESS_BUNDLE entry by document order.
//
//  2. A flat shape used by older tooling and tests: {"version": ..., "build_id": ...}.
//
//  3. A nested {"active": {"version": ..., "build_id": ...}} shape some
//     dev tooling emits.
//
// Order matters: the BOM is the source of truth on real clusters, and
// previously a BOM-shaped file failed both flat and nested parses, so the
// freshness verdict was AWARENESS_BUNDLE_VERIFY_FAILED on every node that
// had a normal release-index.json even after a successful publish.
func loadReleaseIndex(path string) (*bundlesync.ReleaseIndex, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read release-index %s: %w", path, err)
	}
	if ri, ok := releaseIndexFromBOM(data); ok {
		return ri, nil
	}
	// Flat shape.
	var flat bundlesync.ReleaseIndex
	if err := json.Unmarshal(data, &flat); err == nil && flat.Version != "" {
		return &flat, nil
	}
	// Nested {"active": {...}} shape.
	var nested struct {
		Active *bundlesync.ReleaseIndex `json:"active"`
	}
	if err := json.Unmarshal(data, &nested); err == nil && nested.Active != nil && nested.Active.Version != "" {
		return nested.Active, nil
	}
	return nil, fmt.Errorf("release-index %s: no usable version/build_id", path)
}

// releaseIndexFromBOM extracts the awareness bundle entry from a BOM-shaped
// release-index. Returns ok=false when no AWARENESS_BUNDLE entry is present,
// letting the caller fall back to the flat/nested shapes.
func releaseIndexFromBOM(data []byte) (*bundlesync.ReleaseIndex, bool) {
	var bom struct {
		Packages []struct {
			Name    string `json:"name"`
			Kind    string `json:"kind"`
			Version string `json:"version"`
			BuildID string `json:"build_id"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(data, &bom); err != nil || len(bom.Packages) == 0 {
		return nil, false
	}
	var first *bundlesync.ReleaseIndex
	for _, p := range bom.Packages {
		if !strings.EqualFold(strings.TrimSpace(p.Kind), "AWARENESS_BUNDLE") {
			continue
		}
		if p.Version == "" {
			continue
		}
		entry := &bundlesync.ReleaseIndex{Version: p.Version, BuildID: p.BuildID}
		if strings.EqualFold(p.Name, bundlesync.BundleName) {
			return entry, true
		}
		if first == nil {
			first = entry
		}
	}
	if first != nil {
		return first, true
	}
	return nil, false
}

// readLocalBinaryInfo returns the running binary's release identity from
// runtime/debug build info. ldflags-injected versions don't always land in
// debug.BuildInfo settings; missing fields are returned as empty strings,
// and CheckAwarenessFreshness treats empty fields as "skip the correlation."
func readLocalBinaryInfo() *bundlesync.LocalBinaryInfo {
	out := &bundlesync.LocalBinaryInfo{}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return out
	}

	for _, s := range info.Settings {
		if s.Key == "vcs.revision" && out.BuildID == "" {
			out.BuildID = s.Value
		}
	}
	if strings.TrimSpace(out.BuildID) == "" {
		out.BuildID = ""
	}
	return out
}
