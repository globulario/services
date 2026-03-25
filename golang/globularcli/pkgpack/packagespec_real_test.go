package pkgpack

import (
	"os"
	"path/filepath"
	"testing"
)

// TestParseRealSpecs validates that all generated specs in the repo parse
// and pass validation without errors.
func TestParseRealSpecs(t *testing.T) {
	specDir := filepath.Join("..", "..", "..", "generated", "specs")
	entries, err := os.ReadDir(specDir)
	if err != nil {
		t.Skipf("generated/specs not found: %v", err)
	}

	var parsed, failed int
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		path := filepath.Join(specDir, e.Name())
		t.Run(e.Name(), func(t *testing.T) {
			spec, err := ParseSpec(path)
			if err != nil {
				t.Fatalf("ParseSpec: %v", err)
			}
			parsed++

			errs := ValidateSpec(spec, path)
			for _, e := range errs {
				t.Errorf("validation: %v", e)
				failed++
			}
		})
	}
	t.Logf("parsed %d specs, %d validation errors", parsed, failed)
}
