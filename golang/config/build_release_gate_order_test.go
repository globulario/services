package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPlatformFloorGatePrecedesPublish pins the ordering contract for the
// platform-floor gate in scripts/build-local-release.sh:
//
//	a gate must precede the irreversible step it guards.
//
// REGRESSION (2026-08-05). The script used to `cp` the assembled bundle into
// dist/ and only THEN run scripts/check-glibc-floor.sh. When the gate failed it
// printed:
//
//	✗ RELEASE REJECTED — binaries exceed the supported glibc floor.
//	  The bundle is left in place for inspection but MUST NOT be published.
//
// while the bundle was already sitting in dist/, byte-identical in placement to
// a good release. Nothing downstream can tell a "rejected" artifact from an
// accepted one — the docker simulation picked one up and installed it. A gate
// that runs after the irreversible step is not a gate; it is a log line.
//
// This test reads the real script and asserts the gate stage appears before
// BOTH irreversible stages (pack, publish). It is deliberately structural: the
// full build takes ~30 minutes and needs docker plus a base bundle, so running
// it for real is not a unit test. Structure is what regressed, and structure is
// what this pins.
func TestPlatformFloorGatePrecedesPublish(t *testing.T) {
	root := servicesRepoRootForScripts(t)
	path := filepath.Join(root, "scripts", "build-local-release.sh")

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("build-local-release.sh not readable at %s: %v", path, err)
	}
	script := string(raw)

	// Anchors. Each must be unique so an index comparison is meaningful — if a
	// refactor duplicates a stage this test must fail loudly rather than
	// silently compare the wrong occurrence.
	anchors := map[string]string{
		"gate":    `echo "── Verifying platform floor ──"`,
		"pack":    `step "Pack tarball"`,
		"publish": `step "Publish to $OUT_DIR"`,
	}

	idx := map[string]int{}
	for name, anchor := range anchors {
		first := strings.Index(script, anchor)
		if first < 0 {
			t.Fatalf("anchor %q not found in build-local-release.sh (looked for %q).\n"+
				"If the stage was renamed, update this test AND confirm the gate still "+
				"precedes publication — do not just re-point the anchor.", name, anchor)
		}
		if last := strings.LastIndex(script, anchor); last != first {
			t.Fatalf("anchor %q appears more than once (offsets %d and %d); "+
				"ordering cannot be established from a duplicated stage", name, first, last)
		}
		idx[name] = first
	}

	if idx["gate"] > idx["pack"] {
		t.Errorf("platform-floor gate runs AFTER the tarball is packed "+
			"(gate@%d > pack@%d): a rejected bundle would already be materialized",
			idx["gate"], idx["pack"])
	}
	if idx["gate"] > idx["publish"] {
		t.Errorf("platform-floor gate runs AFTER publish (gate@%d > publish@%d): "+
			"this is the exact 1.2.296 defect — the script declares RELEASE REJECTED "+
			"while the artifact is already in dist/", idx["gate"], idx["publish"])
	}

	// The gate must also be fail-closed, not merely early: a missing checker has
	// to be a hard error. A gate that silently skips when its checker cannot be
	// found fails OPEN, which is the same class of defect one level down.
	if !strings.Contains(script, "platform-floor checker missing or not executable") {
		t.Error("the platform-floor gate no longer hard-errors on a missing checker; " +
			"a gate that skips when its checker is absent fails open")
	}
}
