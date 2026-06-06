// @awareness namespace=globular.platform
// @awareness component=platform_controller.workflow_definitions_load
// @awareness file_role=regression_tests_for_workflow_definitions_load_and_threading
// @awareness enforces=globular.platform:invariant.workflow.definitions_must_thread_required_verification_fields
// @awareness protects=globular.platform:failure_mode.workflow.stale_definitions_block_expected_sha256_propagation
// @awareness risk=critical
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withWorkflowDefinitionsDir overrides the package-level workflowDefinitionsDir
// for the lifetime of a test and restores it on cleanup. The variable is
// package-private; tests in this package can swap it directly.
func withWorkflowDefinitionsDir(t *testing.T, dir string) {
	t.Helper()
	prev := workflowDefinitionsDir
	workflowDefinitionsDir = dir
	t.Cleanup(func() { workflowDefinitionsDir = prev })
}

// fixtureWorkflows is the synthetic set used by the loader tests below. It is
// intentionally NOT the production workflow list — the loader's contract is
// "return every *.yaml under workflowDefinitionsDir", so a test fixture
// proves the contract independent of which workflows ship today.
var fixtureWorkflows = []string{
	"alpha.one",
	"beta.two",
	"gamma.three",
	"delta.four",
}

// TestLoadCoreWorkflowsFromDisk_AllPresent confirms that when every fixture
// file is present in workflowDefinitionsDir, loadCoreWorkflowsFromDisk
// returns the full map keyed by name (without the ".yaml" suffix). Baseline
// correctness for the directory-scan loader.
func TestLoadCoreWorkflowsFromDisk_AllPresent(t *testing.T) {
	dir := t.TempDir()
	withWorkflowDefinitionsDir(t, dir)

	for _, name := range fixtureWorkflows {
		path := filepath.Join(dir, name+".yaml")
		if err := os.WriteFile(path, []byte("name: "+name+"\nsteps: []\n"), 0o644); err != nil {
			t.Fatalf("write fixture %s: %v", path, err)
		}
	}

	defs := loadCoreWorkflowsFromDisk()
	if len(defs) != len(fixtureWorkflows) {
		t.Fatalf("loaded %d definitions, want %d (all fixtures present)", len(defs), len(fixtureWorkflows))
	}
	for _, name := range fixtureWorkflows {
		if _, ok := defs[name]; !ok {
			t.Errorf("missing %q from loaded definitions", name)
		}
	}
}

// TestLoadCoreWorkflowsFromDisk_MissingFilesSkipped confirms that the loader
// returns only the *.yaml files it actually finds — not a fixed list. Adding
// or removing a YAML in workflowDefinitionsDir is the supported operator
// motion; the loader must reflect what is on disk.
func TestLoadCoreWorkflowsFromDisk_MissingFilesSkipped(t *testing.T) {
	dir := t.TempDir()
	withWorkflowDefinitionsDir(t, dir)

	present := fixtureWorkflows[:len(fixtureWorkflows)/2]
	for _, name := range present {
		path := filepath.Join(dir, name+".yaml")
		if err := os.WriteFile(path, []byte("name: "+name+"\nsteps: []\n"), 0o644); err != nil {
			t.Fatalf("write fixture %s: %v", path, err)
		}
	}

	defs := loadCoreWorkflowsFromDisk()
	if len(defs) != len(present) {
		t.Fatalf("loaded %d definitions, want %d (half present)", len(defs), len(present))
	}
	for _, name := range present {
		if _, ok := defs[name]; !ok {
			t.Errorf("missing %q from loaded definitions (should be present)", name)
		}
	}
	for _, name := range fixtureWorkflows[len(fixtureWorkflows)/2:] {
		if _, ok := defs[name]; ok {
			t.Errorf("found %q in loaded definitions (should be absent)", name)
		}
	}
}

// TestLoadCoreWorkflowsFromDisk_IgnoresNonYamlFiles confirms that files
// without the ".yaml" extension and subdirectories are skipped, so dropping
// a README or backup file into workflowDefinitionsDir does not poison the
// seed.
func TestLoadCoreWorkflowsFromDisk_IgnoresNonYamlFiles(t *testing.T) {
	dir := t.TempDir()
	withWorkflowDefinitionsDir(t, dir)

	if err := os.WriteFile(filepath.Join(dir, "real.yaml"), []byte("name: real\nsteps: []\n"), 0o644); err != nil {
		t.Fatalf("write real fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("notes"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "backup.yaml.bak"), []byte("ignored"), 0o644); err != nil {
		t.Fatalf("write backup: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}

	defs := loadCoreWorkflowsFromDisk()
	if len(defs) != 1 {
		t.Fatalf("loaded %d definitions, want 1 (only real.yaml)", len(defs))
	}
	if _, ok := defs["real"]; !ok {
		t.Errorf("expected key %q in loaded definitions", "real")
	}
}

// TestLoadCoreWorkflowsFromDisk_EmptyDirReturnsEmpty confirms that an empty
// workflowDefinitionsDir returns an empty map (no error, no panic). This
// shape is what happens on a fresh node before install-day0 has placed the
// YAMLs — controller must not crash, just defer seeding.
func TestLoadCoreWorkflowsFromDisk_EmptyDirReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	withWorkflowDefinitionsDir(t, dir)

	defs := loadCoreWorkflowsFromDisk()
	if len(defs) != 0 {
		t.Fatalf("loaded %d definitions from empty dir, want 0", len(defs))
	}
}

// TestLoadCoreWorkflowsFromDisk_NonexistentDirReturnsEmpty is the symmetric
// safety case: the configured dir does not exist at all. The loader must
// degrade silently (readFileIfExists handles os.IsNotExist), not error.
func TestLoadCoreWorkflowsFromDisk_NonexistentDirReturnsEmpty(t *testing.T) {
	withWorkflowDefinitionsDir(t, filepath.Join(t.TempDir(), "does-not-exist"))

	defs := loadCoreWorkflowsFromDisk()
	if len(defs) != 0 {
		t.Fatalf("loaded %d definitions from nonexistent dir, want 0", len(defs))
	}
}

// TestCoreWorkflowDefinitions_ThreadExpectedSha256 is the headline
// regression. It pins workflow.definitions_must_thread_required_verification_fields
// against the real on-disk definitions in golang/workflow/definitions/.
//
// For release.apply.package and release.apply.controller, the definition
// MUST:
//
//   1. Declare a verification-field input (either resolved_entrypoint_checksum
//      or expected_sha256) in its inputs: block.
//   2. Thread that field into a node.install_package step's with: block as
//      expected_sha256.
//
// Without these, the controller's expected_sha256 propagation chain breaks
// at the workflow definition layer — the controller resolves the manifest
// checksum, asks the workflow to thread it, the YAML drops it on the floor,
// and node-agent honestly writes installed_unverified. This is the v1.2.119
// failure shape captured in
// failure_mode.workflow.stale_definitions_block_expected_sha256_propagation.
func TestCoreWorkflowDefinitions_ThreadExpectedSha256(t *testing.T) {
	repoRoot := findRepoRoot(t)
	definitionsDir := filepath.Join(repoRoot, "golang", "workflow", "definitions")

	cases := []struct {
		file       string
		inputField string // the verification-field name expected in inputs
	}{
		{"release.apply.package.yaml", "resolved_entrypoint_checksum"},
		{"release.apply.controller.yaml", "expected_sha256"},
	}

	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			path := filepath.Join(definitionsDir, c.file)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			content := string(data)

			// Property 1: inputs: block declares the verification field.
			// The on-disk YAML shape is:
			//   inputs:
			//     <field>: { type: string, default: "" }
			// We require the field name to appear in the file. Stricter
			// parsing would be tighter but brittle to comment changes;
			// substring match is sufficient because the failure_mode is
			// "field dropped entirely from the YAML".
			if !strings.Contains(content, c.inputField+":") {
				t.Errorf("%s does not declare %q as an input — verification chain broken at definition layer",
					c.file, c.inputField)
			}

			// Property 2: install_package step threads the field as
			// expected_sha256. The on-disk YAML shape is:
			//   expected_sha256: $.<inputField>
			// or simply:
			//   expected_sha256: $.expected_sha256
			// Either way, the substring "expected_sha256: $." must appear.
			if !strings.Contains(content, "expected_sha256: $.") {
				t.Errorf("%s does not thread a verification field into expected_sha256 — install_package will receive empty",
					c.file)
			}

			// Stronger: the threaded value must reference the inputField
			// declared above (either directly $.expected_sha256 or
			// $.resolved_entrypoint_checksum). Catches a future drift
			// where someone declares one field but threads another.
			want := "expected_sha256: $." + c.inputField
			if !strings.Contains(content, want) {
				t.Errorf("%s expected_sha256 threading does not reference declared input %q — found inputs and threading but they do not align",
					c.file, c.inputField)
			}
		})
	}
}

// findRepoRoot walks up from the test's working directory looking for the
// services repo root (identified by the docs/awareness directory). Returns
// the absolute path. Used to locate golang/workflow/definitions/ from the
// cluster_controller_server test working directory.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := cwd
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "docs", "awareness")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not find services repo root walking up from %s", cwd)
	return ""
}
