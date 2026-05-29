package bundlesync

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// LoadReleaseIndex reads /var/lib/globular/release-index.json (or whatever path
// the caller supplies) and returns the (Version, BuildID) pair pinning the
// active awareness bundle.
//
// Three on-disk shapes are accepted, in priority order:
//
//  1. The canonical BOM produced by the repository / CI release pipeline:
//     {"schema_version": "globular.repository.index/v{1,2}",
//      "packages": [{"kind": "AWARENESS_BUNDLE", "name": "globular-awareness-bundle",
//                    "version": "...", "build_id": "..."}, ...]}.
//     The first entry whose kind matches "AWARENESS_BUNDLE" (case-insensitive)
//     AND whose name matches BundleName wins. If no exact-name match exists,
//     the first AWARENESS_BUNDLE entry in document order wins. Entries with
//     an empty Version are skipped — they cannot pin a bundle.
//
//  2. A flat shape used by older tooling and tests: {"version": "...", "build_id": "..."}.
//
//  3. A nested {"active": {"version": "...", "build_id": "..."}} shape used by
//     some dev tooling.
//
// Order matters: BOM is the source of truth on real clusters. Previously the
// CLI install path tried only flat/nested, so a BOM-shaped release-index.json
// (the production default since schema v2) failed with
// "release-index ...: no usable version/build_id" even when a healthy
// AWARENESS_BUNDLE entry was present. That broke `globular awareness install`
// against every cluster running the v2 schema.
//
// Failure mode is explicit: if none of the three shapes yield a usable
// (Version, BuildID) pair, the returned error names what was missing so the
// operator can fix the release-index rather than guess.
func LoadReleaseIndex(path string) (*ReleaseIndex, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read release-index %s: %w", path, err)
	}
	return parseReleaseIndex(path, data)
}

// parseReleaseIndex is the pure-function form of LoadReleaseIndex.
// Exported via the public entry point only — tests in this package use it
// directly to exercise each shape without writing to a tempfile.
func parseReleaseIndex(path string, data []byte) (*ReleaseIndex, error) {
	if ri, ok := releaseIndexFromBOM(data); ok {
		return ri, nil
	}
	var flat ReleaseIndex
	if err := json.Unmarshal(data, &flat); err == nil && flat.Version != "" {
		return &flat, nil
	}
	var nested struct {
		Active *ReleaseIndex `json:"active"`
	}
	if err := json.Unmarshal(data, &nested); err == nil && nested.Active != nil && nested.Active.Version != "" {
		return nested.Active, nil
	}
	return nil, fmt.Errorf("release-index %s: no usable version/build_id (no AWARENESS_BUNDLE package, no flat version, no active.version)", path)
}

// releaseIndexFromBOM extracts the awareness bundle entry from a BOM-shaped
// release-index. Returns ok=false when no AWARENESS_BUNDLE entry is present
// or the JSON is not BOM-shaped, letting the caller fall back to the flat or
// nested shapes.
func releaseIndexFromBOM(data []byte) (*ReleaseIndex, bool) {
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
	var first *ReleaseIndex
	for _, p := range bom.Packages {
		if !strings.EqualFold(strings.TrimSpace(p.Kind), "AWARENESS_BUNDLE") {
			continue
		}
		if p.Version == "" {
			continue
		}
		entry := &ReleaseIndex{Version: p.Version, BuildID: p.BuildID}
		if strings.EqualFold(p.Name, BundleName) {
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
