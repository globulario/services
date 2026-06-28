package pkgpack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHyphenatedPackagesUseHyphenatedStateDirsInSystemdMetadata(t *testing.T) {
	metadataRoot := filepath.Join("..", "..", "..", "..", "packages", "metadata")
	entries, err := os.ReadDir(metadataRoot)
	if err != nil {
		t.Skipf("packages metadata unavailable: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pkgDir := filepath.Join(metadataRoot, entry.Name())
		packageJSONPath := filepath.Join(pkgDir, "package.json")
		raw, err := os.ReadFile(packageJSONPath)
		if err != nil {
			continue
		}

		var manifest struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(raw, &manifest); err != nil {
			t.Fatalf("parse %s: %v", packageJSONPath, err)
		}
		if !strings.Contains(manifest.Name, "-") {
			continue
		}

		unitDir := filepath.Join(pkgDir, "systemd")
		unitEntries, err := os.ReadDir(unitDir)
		if err != nil {
			continue
		}

		underscoreDir := "{{.StateDir}}/" + strings.ReplaceAll(manifest.Name, "-", "_")
		for _, unitEntry := range unitEntries {
			if unitEntry.IsDir() || filepath.Ext(unitEntry.Name()) != ".service" {
				continue
			}

			unitPath := filepath.Join(unitDir, unitEntry.Name())
			unitText, err := os.ReadFile(unitPath)
			if err != nil {
				t.Fatalf("read %s: %v", unitPath, err)
			}
			if strings.Contains(string(unitText), underscoreDir) {
				t.Errorf("%s uses underscore state dir %q for hyphenated package %q", unitPath, underscoreDir, manifest.Name)
			}
		}
	}
}
