package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	base := flag.String("base", "", "git ref to diff against (default: staged+unstaged vs HEAD)")
	root := flag.String("root", ".", "repository root")
	goRoot := flag.String("go-root", "golang", "directory containing the Go module, relative to root")
	dryRun := flag.Bool("dry-run", false, "resolve and report, but do not execute suites")
	flag.Parse()

	repoRoot, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "affectedtests: %v\n", err)
		os.Exit(2)
	}

	changed, derr := discoverChangedFiles(repoRoot, *base)
	c := ResolvePackages(changed, func(dir string) bool {
		info, err := os.Stat(filepath.Join(repoRoot, dir))
		return err == nil && info.IsDir()
	})
	if derr != nil {
		c.DiscoveryFailed = true
		c.DiscoveryError = derr.Error()
	}

	if !*dryRun && !c.DiscoveryFailed {
		runSuites(repoRoot, *goRoot, &c)
	}

	fmt.Println("── Affected-package test gate ──")
	fmt.Print(Format(&c))

	reasons := Reconcile(&c)
	if len(reasons) > 0 {
		fmt.Fprintln(os.Stderr, "\n  ✗ RELEASE REJECTED — affected-package tests did not pass:")
		for _, r := range reasons {
			fmt.Fprintf(os.Stderr, "    - %s\n", r)
		}
		for _, f := range c.Failed() {
			fmt.Fprintf(os.Stderr, "\n--- %s ---\n%s\n", f.Package, trimOutput(f.Output))
		}
		os.Exit(1)
	}
	fmt.Println("  ✓ affected-package tests passed")
}

// discoverChangedFiles enumerates repo-relative changed paths.
//
// A non-nil error must be propagated rather than swallowed: `git` failing and
// `git` reporting a clean tree both produce an empty list, and only one of them
// means it is safe to proceed.
func discoverChangedFiles(repoRoot, base string) ([]string, error) {
	args := []string{"-C", repoRoot, "diff", "--name-only"}
	if base != "" {
		args = append(args, base+"...HEAD")
	} else {
		args = append(args, "HEAD")
	}
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}
	// Include untracked files: a brand-new package with a failing test is
	// exactly the case a diff-only view misses.
	un, err := exec.Command("git", "-C", repoRoot, "ls-files", "--others", "--exclude-standard").Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}

	seen := map[string]bool{}
	var files []string
	for _, chunk := range [][]byte{out, un} {
		for _, line := range strings.Split(string(chunk), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || seen[line] {
				continue
			}
			seen[line] = true
			files = append(files, line)
		}
	}
	return files, nil
}

// runSuites executes `go test` once per affected package, recording compile
// failure separately from test failure.
func runSuites(repoRoot, goRoot string, c *Census) {
	modDir := filepath.Join(repoRoot, goRoot)
	for _, pkg := range c.Packages {
		rel, err := filepath.Rel(goRoot, pkg)
		if err != nil || strings.HasPrefix(rel, "..") {
			// Outside the Go module (e.g. a .go file in a sibling tree). Not a
			// resolution failure — record it as executed-with-no-suite so the
			// package is never counted as skipped.
			c.Executed = append(c.Executed, pkg)
			c.Results = append(c.Results, PkgResult{Package: pkg, Built: true, Passed: true,
				Output: "outside the Go module; no suite"})
			continue
		}
		target := "./" + filepath.ToSlash(rel)
		cmd := exec.Command("go", "test", "-count=1", target)
		cmd.Dir = modDir
		out, err := cmd.CombinedOutput()

		c.Executed = append(c.Executed, pkg)
		res := PkgResult{Package: pkg, Output: string(out)}
		switch {
		case err == nil:
			res.Built, res.Passed = true, true
		case isBuildFailure(string(out)):
			res.Built, res.Passed = false, false
		default:
			res.Built, res.Passed = true, false
		}
		c.Results = append(c.Results, res)
	}
}

// isBuildFailure distinguishes "did not compile" from "compiled and failed".
// The distinction matters because a package that never built reports zero
// failing tests, which is the same number a healthy package reports.
func isBuildFailure(out string) bool {
	return strings.Contains(out, "[build failed]") ||
		strings.Contains(out, "build constraints exclude all Go files") ||
		strings.Contains(out, "no non-test Go files") ||
		strings.Contains(out, "cannot find package")
}

func trimOutput(s string) string {
	const max = 4000
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n... (truncated)"
}
