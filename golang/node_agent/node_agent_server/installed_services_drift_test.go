// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.installed_services_drift_test
// @awareness file_role=guards_hardcoded_package_lists_against_packages_specs_drift
// @awareness risk=high
package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestCommandAndSkipUnitListsMatchSpecs asserts that the two hardcoded
// package-classification maps in this package — commandPackages
// (grpc_workflow_skip.go) and skipSystemdUnits (installed_services.go) —
// match the canonical set of *_cmd.yaml and infrastructure *_service.yaml
// files in the sibling globulario/packages repo.
//
// The trap: both maps are hand-maintained mirrors of an external truth
// source. Every time a new *_cmd.yaml ships, both maps must be edited or
// the node-agent silently misclassifies the new package (treats it as a
// missing systemd unit, or emits a SERVICE/unknown phantom). We hit this
// on claude-cmd before this test existed.
//
// The packages repo is checked out by both CI workflows
// (.github/workflows/release.yml and ci.yml). When it's not findable —
// typically a local dev tree without the sibling clone — the test is
// skipped so devs aren't blocked, but CI runs always exercise it.
//
// The structural fix is to extract a single shared catalog package both
// the controller and node-agent import; until then this test is the
// guard. Tracked under meta-principle
// code_must_not_mirror_external_enumerations.
func TestCommandAndSkipUnitListsMatchSpecs(t *testing.T) {
	specsDir := findPackagesSpecsDir(t)
	if specsDir == "" {
		t.Skipf("packages/specs not findable from %s — skipping drift check (CI runs always have it via .github/workflows/*.yml)", currentWD(t))
	}

	cmdNames, infraNames, err := readPackageSpecs(specsDir)
	if err != nil {
		t.Fatalf("read packages/specs in %s: %v", specsDir, err)
	}

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
	// bare name AND every *_cmd.yaml with a "-cmd" suffix appended.
	// Documented carve-outs (packages whose spec.kind disagrees with how
	// node-agent classifies them) are added explicitly so the test fails
	// loudly when someone changes the spec OR removes the carve-out.
	expectedSkip := map[string]bool{
		// mcp is kind=service in its spec but is one of the 4 services
		// outside the release pipeline (CLAUDE.md). node-agent treats it
		// as infrastructure-like for the phantom-mask reason documented
		// in installed_services.go. Spec change tracked separately.
		"mcp": true,
	}
	for _, n := range cmdNames {
		expectedSkip[strings.ReplaceAll(n, "_", "-")+"-cmd"] = true
	}
	for _, n := range infraNames {
		expectedSkip[strings.ReplaceAll(n, "_", "-")] = true
	}
	assertSetEquals(t, "skipSystemdUnits", skipSystemdUnits, expectedSkip)
}

// findPackagesSpecsDir walks up from CWD looking for a sibling
// globulario/packages/specs directory. Returns "" when not found.
func findPackagesSpecsDir(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := cwd
	for i := 0; i < 12; i++ {
		candidate := filepath.Join(filepath.Dir(dir), "packages", "specs")
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// readPackageSpecs returns the bare names of *_cmd.yaml files and the bare
// names of *_service.yaml files that carry metadata.kind=infrastructure.
// "Bare name" means the filename minus the _cmd.yaml or _service.yaml
// suffix. We scan a single `kind:` line rather than pulling in a YAML
// dependency — the spec files are author-maintained, the field is
// invariant, and `kind:` cannot collide with any other top-level marker.
func readPackageSpecs(dir string) (cmdNames, infraNames []string, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		switch {
		case strings.HasSuffix(name, "_cmd.yaml"):
			cmdNames = append(cmdNames, strings.TrimSuffix(name, "_cmd.yaml"))
		case strings.HasSuffix(name, "_service.yaml"):
			data, rerr := os.ReadFile(filepath.Join(dir, name))
			if rerr != nil {
				return nil, nil, rerr
			}
			if isInfrastructureSpec(data) {
				infraNames = append(infraNames, strings.TrimSuffix(name, "_service.yaml"))
			}
		}
	}
	sort.Strings(cmdNames)
	sort.Strings(infraNames)
	return cmdNames, infraNames, nil
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
		t.Errorf("%s contains %d phantom entries not present in packages/specs: %v", label, len(extra), extra)
	}
	if len(missing) > 0 {
		t.Errorf("%s is missing %d entries present in packages/specs: %v — add to the map", label, len(missing), missing)
	}
}

func currentWD(t *testing.T) string {
	t.Helper()
	cwd, _ := os.Getwd()
	return cwd
}
