// Command affectedtests derives the Go packages affected by a set of changed
// files and requires their test suites to pass before a release may proceed.
//
// Why this exists: on 2026-08-05 two commits landed with tests already failing
// in the package they changed, and one of them (c98310b8) was built into
// distro 1.2.298 and deployed to a five-node cluster.
//
//	c98310b8  TestNodeJoinProfilePlacementSkipsUnauthorizedPackages
//	a93a85b   TestJoinScript_TarballDownloadPrefersGateway
//
// Neither was a product regression — both were assertions superseded by a
// deliberate decision — but nothing in the path from "changed package" to
// "released artifact" ever ran the suite that would have said so. That path
// must not exist.
//
// The gate reports an explicit census rather than a bare pass/fail, for the
// same reason `sensei check` now does: "affected tests passed" is ambiguous
// between "the suites ran and were green" and "no suites ran at all".
package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// Exemption records a changed file deliberately excluded from package
// resolution. Exemptions are legitimate but must be COUNTED AND NAMED — an
// uncounted exemption is indistinguishable from a file the resolver lost.
type Exemption struct {
	Path   string
	Reason string
}

// PkgResult is the outcome of one package's suite.
type PkgResult struct {
	Package string
	// Built reports whether the package compiled. A compile failure must never
	// be reported as "0 failing tests": zero failures with zero tests run is
	// the signature of a package that never built.
	Built  bool
	Passed bool
	Output string
}

// Census is the full accounting of one gate run. Every changed file lands in
// exactly one of: GoFiles (resolved to a package), Exempt, or Unmappable.
type Census struct {
	ChangedFiles []string
	GoFiles      []string
	Exempt       []Exemption
	Unmappable   []string
	Packages     []string
	Executed     []string
	Results      []PkgResult

	// DiscoveryFailed is set when the command that enumerates changed files
	// errored. An empty result from a dead discovery command must never be read
	// as "nothing changed" — that is the failure mode where the gate reports
	// success precisely because it learned nothing.
	DiscoveryFailed bool
	DiscoveryError  string
}

func (c *Census) Passed() []PkgResult {
	var out []PkgResult
	for _, r := range c.Results {
		if r.Built && r.Passed {
			out = append(out, r)
		}
	}
	return out
}

func (c *Census) Failed() []PkgResult {
	var out []PkgResult
	for _, r := range c.Results {
		if !r.Built || !r.Passed {
			out = append(out, r)
		}
	}
	return out
}

// exemptReason classifies a changed path that legitimately maps to no Go
// package. Returns "" when the path is not exempt.
//
// Kept deliberately narrow: anything not matched here and not a .go file is
// still counted, just not as a test trigger. The dangerous shape would be a
// broad "if we can't map it, skip it" rule, which is how a renamed package
// silently stops being tested.
func exemptReason(path string) string {
	switch {
	case strings.HasSuffix(path, ".md"), strings.HasSuffix(path, ".txt"):
		return "documentation"
	case strings.HasPrefix(path, "docs/"):
		return "documentation tree"
	case strings.Contains(path, "/generated/"), strings.HasPrefix(path, "generated/"):
		return "generated output"
	case strings.HasSuffix(path, ".yaml"), strings.HasSuffix(path, ".yml"),
		strings.HasSuffix(path, ".json"):
		return "declarative data"
	}
	return ""
}

// ResolvePackages maps changed repo-relative paths onto Go package directories.
//
// dirExists reports whether a repo-relative directory is present in the CURRENT
// tree; it is injected so the resolver is testable without a filesystem and so
// deleted/renamed files are handled explicitly rather than by accident.
//
// Deletions and renames are the subtle case. `git diff --name-only` reports the
// OLD path of a deleted or renamed file, and that path's directory may no
// longer exist. Two outcomes, both explicit:
//
//   - the directory still exists → the package survived the rename; test it.
//   - the directory is gone → the package itself was removed or moved. That is
//     reported as unmappable, NOT silently dropped, because "the package I
//     would have tested no longer exists" is a fact a release gate must state
//     out loud rather than absorb.
func ResolvePackages(changed []string, dirExists func(string) bool) Census {
	c := Census{ChangedFiles: append([]string(nil), changed...)}
	seen := map[string]bool{}

	for _, path := range changed {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if !strings.HasSuffix(path, ".go") {
			if reason := exemptReason(path); reason != "" {
				c.Exempt = append(c.Exempt, Exemption{Path: path, Reason: reason})
				continue
			}
			// Not a Go file and not a declared exemption: it changes the build
			// but triggers no package. Count it as exempt with an explicit
			// reason so it never looks like a resolution failure.
			c.Exempt = append(c.Exempt, Exemption{Path: path, Reason: "non-Go file, no package mapping"})
			continue
		}

		c.GoFiles = append(c.GoFiles, path)
		dir := filepath.Dir(path)
		if dir == "." || dir == "" {
			c.Unmappable = append(c.Unmappable, path)
			continue
		}
		if !dirExists(dir) {
			c.Unmappable = append(c.Unmappable, path)
			continue
		}
		if !seen[dir] {
			seen[dir] = true
			c.Packages = append(c.Packages, dir)
		}
	}
	sort.Strings(c.Packages)
	return c
}

// Reconcile returns the reasons this gate must fail. Empty slice means the
// release may proceed.
//
// Every condition here is one where a naive implementation would report
// success: they are all shapes of "nothing went wrong because nothing
// happened".
func Reconcile(c *Census) []string {
	var reasons []string

	// A dead discovery command yields an empty change set, which looks exactly
	// like a clean tree. This must be checked FIRST — every count below is
	// meaningless if the input was never gathered.
	if c.DiscoveryFailed {
		reasons = append(reasons, fmt.Sprintf(
			"change discovery failed (%s) — an empty result from a failed discovery is not evidence of no changes",
			c.DiscoveryError))
		return reasons
	}

	if len(c.Unmappable) > 0 {
		reasons = append(reasons, fmt.Sprintf(
			"%d changed Go file(s) could not be mapped to a package: %s",
			len(c.Unmappable), strings.Join(c.Unmappable, ", ")))
	}

	// Discovered but not executed. This is the gap the whole gate exists to
	// close: a package can be resolved and then never run, and the run would
	// still end with zero failures.
	executed := map[string]bool{}
	for _, p := range c.Executed {
		executed[p] = true
	}
	var skipped []string
	for _, p := range c.Packages {
		if !executed[p] {
			skipped = append(skipped, p)
		}
	}
	if len(skipped) > 0 {
		reasons = append(reasons, fmt.Sprintf(
			"%d affected package(s) were discovered but never executed: %s",
			len(skipped), strings.Join(skipped, ", ")))
	}

	// Compile failures are reported separately from test failures so that
	// "0 failing tests" can never disguise "the package never built".
	var didNotBuild, failedTests []string
	for _, r := range c.Results {
		switch {
		case !r.Built:
			didNotBuild = append(didNotBuild, r.Package)
		case !r.Passed:
			failedTests = append(failedTests, r.Package)
		}
	}
	if len(didNotBuild) > 0 {
		reasons = append(reasons, fmt.Sprintf(
			"%d affected package(s) failed to COMPILE: %s",
			len(didNotBuild), strings.Join(didNotBuild, ", ")))
	}
	if len(failedTests) > 0 {
		reasons = append(reasons, fmt.Sprintf(
			"%d affected package(s) have failing tests: %s",
			len(failedTests), strings.Join(failedTests, ", ")))
	}

	// Go files changed but nothing resolved and nothing exempted: the resolver
	// produced an empty answer from a non-empty input.
	if len(c.GoFiles) > 0 && len(c.Packages) == 0 && len(c.Unmappable) == 0 {
		reasons = append(reasons, fmt.Sprintf(
			"%d Go file(s) changed but no packages resolved — the resolver returned an empty answer for a non-empty input",
			len(c.GoFiles)))
	}

	return reasons
}

// Format renders the census the gate must always print, whether it passes or
// fails. A gate that only speaks when it fails cannot be distinguished from one
// that never ran.
func Format(c *Census) string {
	var b strings.Builder
	fmt.Fprintf(&b, "    changed files discovered   %d\n", len(c.ChangedFiles))
	fmt.Fprintf(&b, "    changed Go files           %d\n", len(c.GoFiles))
	fmt.Fprintf(&b, "    exempted (counted)         %d\n", len(c.Exempt))
	fmt.Fprintf(&b, "    unmappable                 %d\n", len(c.Unmappable))
	fmt.Fprintf(&b, "    affected packages resolved %d\n", len(c.Packages))
	fmt.Fprintf(&b, "    package suites executed    %d\n", len(c.Executed))
	fmt.Fprintf(&b, "    package suites passed      %d\n", len(c.Passed()))
	fmt.Fprintf(&b, "    package suites failed      %d\n", len(c.Failed()))
	byReason := map[string]int{}
	for _, e := range c.Exempt {
		byReason[e.Reason]++
	}
	reasons := make([]string, 0, len(byReason))
	for r := range byReason {
		reasons = append(reasons, r)
	}
	sort.Strings(reasons)
	for _, r := range reasons {
		fmt.Fprintf(&b, "      exempt: %-28s %d\n", r, byReason[r])
	}
	return b.String()
}
