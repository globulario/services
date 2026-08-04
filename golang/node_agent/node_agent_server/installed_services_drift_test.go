// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.installed_services_drift_test
// @awareness file_role=guards_hardcoded_package_lists_against_packages_specs_drift
// @awareness risk=high
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// packagesDirEnv names the explicit path to a globulario/packages checkout.
// CI sets it (.github/workflows/ci.yml, "Enforce package-classification
// parity"); local runs may set it too. When unset we fall back to the sibling
// checkout convention this repo already relies on for go.work (../Globular,
// ../globular-installer) and that ci.yml reproduces by checking packages out
// next to services in the workspace.
const packagesDirEnv = "GLOBULAR_PACKAGES_DIR"

// TestCommandAndSkipUnitListsMatchSpecs asserts that the two hardcoded
// package-classification maps in this package — commandPackages
// (grpc_workflow_skip.go) and skipSystemdUnits (installed_services.go) —
// match the canonical set of *_cmd.yaml and infrastructure *_service.yaml
// specs in the sibling globulario/packages repo.
//
// The trap: both maps are hand-maintained mirrors of an external truth
// source. Every time a new *_cmd.yaml ships, both maps must be edited or
// the node-agent silently misclassifies the new package (treats it as a
// missing systemd unit, or emits a SERVICE/unknown phantom). We hit this
// on claude-cmd before this test existed.
//
// THIS GUARD WAS INERT until 2026-08-03. It searched for
// "<ancestor>/packages/specs", but the packages repo stores specs at
// metadata/<package>/specs/*.yaml. The lookup never matched, so the test took
// a t.Skipf path — reporting the same green as a passing run while asserting
// nothing, in CI as well as locally. Two rules follow, and both are
// load-bearing:
//
//  1. NO t.Skip. Not when the packages repo is absent, not for any reason. A
//     guard that can excuse itself will eventually excuse itself permanently
//     and nobody will notice. Missing source is a FAILURE, and the failure
//     message says how to supply it.
//  2. The test logs how many spec files it actually consumed, so a green run
//     carries evidence that it asserted rather than merely executed.
//
// It also runs as its own hard-gate CI step. The broad "Unit Tests" step is
// continue-on-error (some tests need etcd/ScyllaDB), so a parity failure
// there would not block — the second half of the same inertia.
//
// The structural fix is to extract a single shared catalog package both
// the controller and node-agent import; until then this test is the
// guard. Tracked under meta-principle
// code_must_not_mirror_external_enumerations and
// candidate.invariant.meta.single_derivation_path_must_reach_its_consumer.
func TestCommandAndSkipUnitListsMatchSpecs(t *testing.T) {
	root := packagesRepoRoot(t)

	cmdNames, infraNames, scanned, err := readPackageSpecs(root)
	if err != nil {
		t.Fatalf("read canonical package specs under %s: %v", root, err)
	}

	// Zero discovered specs means the layout moved again. Without this guard
	// an empty canonical set would make every projection entry look like a
	// phantom — a confusing failure — or, if the maps were ever emptied too,
	// would pass vacuously.
	if scanned == 0 {
		t.Fatalf("discovered 0 spec files under %s/metadata/*/specs — the packages layout moved again; "+
			"fix readPackageSpecs rather than skipping the parity check", root)
	}
	if len(cmdNames) == 0 {
		t.Fatalf("discovered %d spec files under %s/metadata/*/specs but 0 *_cmd.yaml — "+
			"command specs cannot legitimately be empty", scanned, root)
	}
	if len(infraNames) == 0 {
		t.Fatalf("discovered %d spec files under %s/metadata/*/specs but 0 infrastructure *_service.yaml — "+
			"infrastructure specs cannot legitimately be empty", scanned, root)
	}

	// Proof that this run asserted. Read this line before trusting a PASS.
	t.Logf("parity source %s: scanned %d spec files, consumed %d *_cmd.yaml and %d infrastructure *_service.yaml",
		root, scanned, len(cmdNames), len(infraNames))

	// commandPackages must contain exactly the *_cmd.yaml bare names with
	// underscores normalized to hyphens. The map already has globular-cli
	// while the spec is named globular_cli_cmd.yaml — that's the
	// normalization this assertion encodes.
	expectedCommand := map[string]bool{}
	for _, n := range cmdNames {
		expectedCommand[strings.ReplaceAll(n, "_", "-")] = true
	}
	assertSetEquals(t, "commandPackages", commandPackages, expectedCommand)

	// skipSystemdUnits must contain every infrastructure *_service.yaml
	// bare name AND every *_cmd.yaml with a "-cmd" suffix appended. SERVICE
	// packages such as mcp must not appear here: otherwise loadSystemdUnits
	// skips the real unit and heartbeat never reports the service installed.
	expectedSkip := map[string]bool{}
	for _, n := range cmdNames {
		expectedSkip[strings.ReplaceAll(n, "_", "-")+"-cmd"] = true
	}
	for _, n := range infraNames {
		expectedSkip[strings.ReplaceAll(n, "_", "-")] = true
	}
	assertSetEquals(t, "skipSystemdUnits", skipSystemdUnits, expectedSkip)
}

// packagesRepoRoot resolves the globulario/packages checkout root. It fails
// the test — never skips — when the source cannot be resolved, because an
// unavailable canonical source means this guard cannot do its job and must
// say so loudly.
func packagesRepoRoot(t *testing.T) string {
	t.Helper()

	if dir := strings.TrimSpace(os.Getenv(packagesDirEnv)); dir != "" {
		if err := checkPackagesRoot(dir); err != nil {
			t.Fatalf("%s=%q is not a usable globulario/packages checkout: %v", packagesDirEnv, dir, err)
		}
		return dir
	}

	found, tried := findSiblingPackagesRoot(t)
	if found != "" {
		return found
	}

	t.Fatalf("cannot locate the canonical globulario/packages checkout.\n"+
		"Set %s=/path/to/packages, or clone globulario/packages next to this repo.\n"+
		"Tried: %s\n"+
		"This check does NOT skip: parity between commandPackages/skipSystemdUnits and the\n"+
		"canonical specs is unverifiable without the source, and an unverified guard is a failure.",
		packagesDirEnv, strings.Join(tried, "\n       "))
	return ""
}

// findSiblingPackagesRoot walks up from CWD looking for a sibling "packages"
// checkout. Returns the root and every path it probed (for the failure
// message).
func findSiblingPackagesRoot(t *testing.T) (root string, tried []string) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := cwd
	for i := 0; i < 12; i++ {
		candidate := filepath.Join(filepath.Dir(dir), "packages")
		tried = append(tried, candidate)
		if checkPackagesRoot(candidate) == nil {
			return candidate, tried
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", tried
}

// checkPackagesRoot verifies dir looks like a globulario/packages checkout:
// it must hold a readable metadata/ directory, which is where per-package
// specs live (metadata/<package>/specs/*.yaml).
func checkPackagesRoot(dir string) error {
	metadata := filepath.Join(dir, "metadata")
	st, err := os.Stat(metadata)
	if err != nil {
		return fmt.Errorf("stat %s: %w", metadata, err)
	}
	if !st.IsDir() {
		return fmt.Errorf("%s is not a directory", metadata)
	}
	if _, err := os.ReadDir(metadata); err != nil {
		return fmt.Errorf("read %s: %w", metadata, err)
	}
	return nil
}

// readPackageSpecs walks <root>/metadata/<package>/specs/ and returns the
// bare names of *_cmd.yaml files, the bare names of *_service.yaml files
// carrying kind=infrastructure, and the total number of spec files scanned.
// "Bare name" means the filename minus the _cmd.yaml or _service.yaml
// suffix. We scan a single `kind:` line rather than pulling in a YAML
// dependency — the spec files are author-maintained, the field is
// invariant, and `kind:` cannot collide with any other top-level marker.
func readPackageSpecs(root string) (cmdNames, infraNames []string, scanned int, err error) {
	metadata := filepath.Join(root, "metadata")
	walkErr := filepath.WalkDir(metadata, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		// Only files directly inside a directory named "specs" are canonical
		// specs; metadata/<package>/ also holds unrelated files.
		if filepath.Base(filepath.Dir(path)) != "specs" {
			return nil
		}
		name := d.Name()
		switch {
		case strings.HasSuffix(name, "_cmd.yaml"):
			scanned++
			cmdNames = append(cmdNames, strings.TrimSuffix(name, "_cmd.yaml"))
		case strings.HasSuffix(name, "_service.yaml"):
			scanned++
			data, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			if isInfrastructureSpec(data) {
				infraNames = append(infraNames, strings.TrimSuffix(name, "_service.yaml"))
			}
		}
		return nil
	})
	if walkErr != nil {
		return nil, nil, 0, walkErr
	}
	sort.Strings(cmdNames)
	sort.Strings(infraNames)
	return cmdNames, infraNames, scanned, nil
}

func isInfrastructureSpec(data []byte) bool {
	for _, line := range strings.Split(string(data), "\n") {
		trim := strings.TrimSpace(line)
		if !strings.HasPrefix(trim, "kind:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(trim, "kind:"))
		value = strings.Trim(value, `"' `)
		if strings.EqualFold(value, "infrastructure") {
			return true
		}
	}
	return false
}

func assertSetEquals(t *testing.T, label string, got, want map[string]bool) {
	t.Helper()
	var extra, missing []string
	for k := range got {
		if !want[k] {
			extra = append(extra, k)
		}
	}
	for k := range want {
		if !got[k] {
			missing = append(missing, k)
		}
	}
	sort.Strings(extra)
	sort.Strings(missing)
	if len(extra) > 0 {
		t.Errorf("%s contains %d phantom entries not present in the canonical packages specs: %v — remove them or add the spec", label, len(extra), extra)
	}
	if len(missing) > 0 {
		t.Errorf("%s is missing %d entries present in the canonical packages specs: %v — add to the map", label, len(missing), missing)
	}
}
