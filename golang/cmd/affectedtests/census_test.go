package main

import (
	"strings"
	"testing"
)

// This gate exists because on 2026-08-05 two commits landed with tests already
// failing in the package they changed, and one was built into distro 1.2.298
// and deployed to a five-node cluster:
//
//	c98310b8  golang/node_agent/node_agent_server
//	          TestNodeJoinProfilePlacementSkipsUnauthorizedPackages
//	a93a85b   internal/gateway/handlers/cluster (Globular)
//	          TestJoinScript_TarballDownloadPrefersGateway
//
// Neither was a product regression — both were assertions superseded by a
// deliberate decision — but nothing between "changed package" and "released
// artifact" ever ran the suite that would have said so.
//
// Every test below targets a shape of "nothing went wrong because nothing
// happened", which is the only way a gate like this fails in practice.

func allDirsExist(string) bool { return true }
func noDirsExist(string) bool  { return false }
func onlyGolang(d string) bool { return strings.HasPrefix(d, "golang/") }

// TestScarOne_NodeAgentPlacementSuiteIsDiscoveredAndRun replays c98310b8: the
// commit changed component_catalog and node_agent_server, and the node_agent
// suite was the one that failed.
func TestScarOne_NodeAgentPlacementSuiteIsDiscoveredAndRun(t *testing.T) {
	c := ResolvePackages([]string{
		"golang/component_catalog/profilemap.go",
		"golang/component_catalog/node_baseline_test.go",
	}, allDirsExist)

	if len(c.Packages) != 1 || c.Packages[0] != "golang/component_catalog" {
		t.Fatalf("packages = %v, want [golang/component_catalog]", c.Packages)
	}

	// Discovered but not executed must FAIL. This is the exact hole: the
	// package was known, and its suite still never ran.
	if reasons := Reconcile(&c); len(reasons) == 0 {
		t.Fatal("a discovered-but-unexecuted package passed the gate — this is the " +
			"c98310b8 defect: the affected suite was never run and the release proceeded")
	} else if !strings.Contains(strings.Join(reasons, " "), "never executed") {
		t.Errorf("reason must name the unexecuted package; got %v", reasons)
	}

	// Executed and failing must also FAIL, with the package named.
	c.Executed = []string{"golang/component_catalog"}
	c.Results = []PkgResult{{
		Package: "golang/component_catalog", Built: true, Passed: false,
		Output: `--- FAIL: TestNodeJoinProfilePlacementSkipsUnauthorizedPackages
    core package "envoy" should be rejected by placement filter`,
	}}
	reasons := Reconcile(&c)
	if len(reasons) == 0 {
		t.Fatal("a failing affected suite passed the gate")
	}
	if !strings.Contains(strings.Join(reasons, " "), "failing tests") {
		t.Errorf("reason must say tests failed; got %v", reasons)
	}
}

// TestScarTwo_JoinScriptSuiteResolvesOutsideTheServicesModule replays a93a85b,
// which changed a package in the Globular repo. The resolver must still map it
// to a package rather than dropping it as unmappable — a file it cannot place
// is a file whose tests never run.
func TestScarTwo_JoinScriptSuiteResolvesOutsideTheServicesModule(t *testing.T) {
	c := ResolvePackages([]string{
		"internal/gateway/handlers/cluster/join_script.go",
		"internal/gateway/handlers/cluster/join_script_test.go",
	}, allDirsExist)

	if len(c.Unmappable) != 0 {
		t.Fatalf("changed Go files were unmappable: %v", c.Unmappable)
	}
	if len(c.Packages) != 1 || c.Packages[0] != "internal/gateway/handlers/cluster" {
		t.Fatalf("packages = %v, want [internal/gateway/handlers/cluster]", c.Packages)
	}
}

// TestCompileFailureIsNotZeroFailingTests is the control the directive calls
// for: a package that never built reports the same "0 failing tests" as a
// healthy one, so the gate must classify it separately and say so.
func TestCompileFailureIsNotZeroFailingTests(t *testing.T) {
	c := Census{
		GoFiles:  []string{"golang/x/x.go"},
		Packages: []string{"golang/x"},
		Executed: []string{"golang/x"},
		Results: []PkgResult{{
			Package: "golang/x", Built: false, Passed: false,
			Output: "# golang/x\n./x.go:9:2: undefined: helper\nFAIL golang/x [build failed]",
		}},
	}
	reasons := Reconcile(&c)
	joined := strings.Join(reasons, " ")
	if !strings.Contains(joined, "COMPILE") {
		t.Fatalf("a build failure must be reported as a compile failure, not as passing "+
			"or as a plain test failure; got %v", reasons)
	}
	if len(c.Passed()) != 0 {
		t.Error("a package that did not build must never be counted as passed")
	}
	if len(c.Failed()) != 1 {
		t.Error("a package that did not build must be counted as failed")
	}
	if !isBuildFailure(c.Results[0].Output) {
		t.Error("isBuildFailure must recognize the go test build-failure signature")
	}
}

// TestDeadDiscoveryIsNotACleanTree: `git` failing and `git` reporting no
// changes both yield an empty list. Only one of them means it is safe to ship.
func TestDeadDiscoveryIsNotACleanTree(t *testing.T) {
	c := Census{DiscoveryFailed: true, DiscoveryError: "git diff: exit status 128"}
	reasons := Reconcile(&c)
	if len(reasons) == 0 {
		t.Fatal("a failed discovery command passed the gate as though the tree were clean")
	}
	if !strings.Contains(reasons[0], "not evidence of no changes") {
		t.Errorf("reason must state why an empty result is untrustworthy; got %v", reasons)
	}

	// A genuinely clean tree must still pass — otherwise the gate blocks
	// every release and would simply be turned off.
	clean := Census{}
	if reasons := Reconcile(&clean); len(reasons) != 0 {
		t.Errorf("an genuinely empty change set must pass; got %v", reasons)
	}
}

// TestDeletedOrRenamedPackageIsNamedNotDropped: git reports the OLD path for a
// deleted or renamed file, and that directory may be gone. Silently skipping it
// is how a renamed package stops being tested without anyone noticing.
func TestDeletedOrRenamedPackageIsNamedNotDropped(t *testing.T) {
	c := ResolvePackages([]string{"golang/removed_pkg/thing.go"}, noDirsExist)

	if len(c.Packages) != 0 {
		t.Fatalf("a vanished directory must not resolve to a package; got %v", c.Packages)
	}
	if len(c.Unmappable) != 1 {
		t.Fatalf("a vanished directory must be reported unmappable, not dropped; census %+v", c)
	}
	if reasons := Reconcile(&c); len(reasons) == 0 {
		t.Fatal("an unmappable changed Go file passed the gate")
	}

	// Mixed case: one package survived a rename, one did not. The survivor must
	// still be tested; the casualty must still be named.
	mixed := ResolvePackages([]string{
		"golang/node_agent/node_agent_server/heartbeat.go",
		"typescript/old/gone.go",
	}, onlyGolang)
	if len(mixed.Packages) != 1 || mixed.Packages[0] != "golang/node_agent/node_agent_server" {
		t.Errorf("surviving package must still be tested; got %v", mixed.Packages)
	}
	if len(mixed.Unmappable) != 1 {
		t.Errorf("vanished package must still be named; got %v", mixed.Unmappable)
	}
}

// TestExemptionsAreCountedAndNamed. Doc-only and generated-only changes may be
// exempt, but an uncounted exemption is indistinguishable from a file the
// resolver lost.
func TestExemptionsAreCountedAndNamed(t *testing.T) {
	c := ResolvePackages([]string{
		"README.md",
		"docs/awareness/invariants.yaml",
		"golang/generated/specs/sql_service.yaml",
		"scripts/build-local-release.sh",
	}, allDirsExist)

	if len(c.Packages) != 0 {
		t.Errorf("no Go files changed; expected no packages, got %v", c.Packages)
	}
	if len(c.Unmappable) != 0 {
		t.Errorf("exempt files must not be reported unmappable; got %v", c.Unmappable)
	}
	if len(c.Exempt) != 4 {
		t.Fatalf("all 4 non-Go files must be counted as exemptions; got %d: %+v", len(c.Exempt), c.Exempt)
	}
	for _, e := range c.Exempt {
		if strings.TrimSpace(e.Reason) == "" {
			t.Errorf("exemption for %q has no named reason", e.Path)
		}
	}
	if reasons := Reconcile(&c); len(reasons) != 0 {
		t.Errorf("a documentation-only change must pass; got %v", reasons)
	}

	// The census must print each exemption reason with its count.
	out := Format(&c)
	if !strings.Contains(out, "exempt: documentation") || !strings.Contains(out, "exempt: generated output") {
		t.Errorf("census must name exemption reasons; got:\n%s", out)
	}
}

// TestCensusReportsEveryRequiredCount pins the five counts the directive
// requires, so "affected tests passed" can never again be ambiguous between
// "suites ran green" and "no suites ran".
func TestCensusReportsEveryRequiredCount(t *testing.T) {
	c := Census{
		ChangedFiles: []string{"golang/a/a.go"},
		GoFiles:      []string{"golang/a/a.go"},
		Packages:     []string{"golang/a"},
		Executed:     []string{"golang/a"},
		Results:      []PkgResult{{Package: "golang/a", Built: true, Passed: true}},
	}
	out := Format(&c)
	for _, want := range []string{
		"changed files discovered",
		"affected packages resolved",
		"package suites executed",
		"package suites passed",
		"package suites failed",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("census missing %q; got:\n%s", want, out)
		}
	}
	if reasons := Reconcile(&c); len(reasons) != 0 {
		t.Errorf("a fully green run must pass; got %v", reasons)
	}
}

// TestResolverCannotReturnEmptyForNonEmptyGoInput guards the resolver itself:
// Go files changed, nothing resolved, nothing reported unmappable is a silent
// answer of "no work to do" from a non-empty input.
func TestResolverCannotReturnEmptyForNonEmptyGoInput(t *testing.T) {
	c := Census{GoFiles: []string{"golang/a/a.go", "golang/b/b.go"}}
	reasons := Reconcile(&c)
	if len(reasons) == 0 {
		t.Fatal("resolver returned no packages and no unmappable files for changed Go files, and the gate passed")
	}
	if !strings.Contains(strings.Join(reasons, " "), "empty answer") {
		t.Errorf("reason must call out the empty answer; got %v", reasons)
	}
}
