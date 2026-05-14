package main

// awareness_verify_probe_test.go — end-to-end verification harness for the
// 2026-05-14 awareness bundle publish work. Reads the BOM file that the
// verification pass produced on disk and runs it through both
// ValidateReleaseIndex (structural) and ValidateReleaseIndexForInstall
// (strict install gate). Skipped if the file isn't present, so it stays
// hermetic in CI but provides a real-file regression hook for the next
// person running the same verification.

import (
	"encoding/json"
	"os"
	"testing"
)

const verifyBOMPath = "/tmp/awareness-verify/release-index-bom.json"

func TestAwarenessVerify_BOMShapePassesInstallValidation(t *testing.T) {
	data, err := os.ReadFile(verifyBOMPath)
	if err != nil {
		t.Skipf("%s not present (skipping live-verify probe): %v", verifyBOMPath, err)
	}
	var idx releaseIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		t.Fatalf("unmarshal BOM: %v", err)
	}

	// Strict install validation requires build_number > 0 and a
	// non-numeric build_id. The verify BOM was authored without
	// build_number to demonstrate the kind-acceptance fix; backfill the
	// required fields here so we exercise the AWARENESS_BUNDLE kind path,
	// not the build_number rejection path.
	for _, p := range idx.Packages {
		if p.BuildNumber == 0 {
			p.BuildNumber = 1
		}
		if p.ArtifactSha256 == "" && p.PackageDigest != "" {
			p.ArtifactSha256 = p.PackageDigest
		}
		changed := true
		p.ChangedInRelease = &changed
	}

	if err := ValidateReleaseIndex(&idx); err != nil {
		t.Fatalf("BOM should be structurally valid: %v", err)
	}
	if err := ValidateReleaseIndexForInstall(&idx); err != nil {
		t.Fatalf("BOM should pass strict install validation (kind=AWARENESS_BUNDLE): %v", err)
	}
}
