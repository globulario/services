package rules

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// readBundleManifest loads the manifest.json that lives at the root of
// an installed awareness bundle directory.
func readBundleManifest(bundleDir string) (*bundleManifest, string, error) {
	manifestPath := filepath.Join(bundleDir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, manifestPath, fmt.Errorf("read manifest: %w", err)
	}
	var m bundleManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, manifestPath, fmt.Errorf("parse manifest: %w", err)
	}
	return &m, manifestPath, nil
}
