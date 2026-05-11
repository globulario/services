package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// readBundleSeedVersion returns the bundle's declared version string,
// or "auto-seeded" if the manifest is unreadable. The value is stamped
// into metadata.seed_version on every row so operators can correlate
// what's in ai-memory with which bundle build wrote it.
func readBundleSeedVersion() string {
	const fallback = "auto-seeded"
	data, err := os.ReadFile(filepath.Join(opsKnowledgeBundlePath, "manifest.json"))
	if err != nil {
		return fallback
	}
	var m struct {
		Version string `json:"version"`
		BuildID string `json:"build_id"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return fallback
	}
	if m.Version == "" {
		return fallback
	}
	if m.BuildID != "" && len(m.BuildID) >= 8 {
		return m.Version + "+" + m.BuildID[:8]
	}
	return m.Version
}
