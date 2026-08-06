// audit-package-archives reports which published package artifacts the
// node-agent's archive admission gate would reject, and why.
//
// It calls the installer's own planner (actions.ValidateArtifactArchive), not a
// reimplementation, so a clean result is evidence about the real extraction
// path rather than about a parallel test harness.
//
// Run it BEFORE enabling the fail-closed gate in production: an artifact that
// this command rejects will fail to install on every node once the gate is
// live. Findings are grouped by package so each one can be classified as a
// genuine invalid path, harmless historical debris, or a validator false
// positive — and repaired or republished before merge.
//
// Usage:
//
//	audit-package-archives [--json] [--fail-on-findings] <path-or-dir>...
//
// Each path may be a .tgz artifact or a directory that is scanned for
// *.tgz. The package name is derived from the conventional artifact filename
// <name>_<version>_<platform>.tgz and only affects the config/ and policy/
// destination roots.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/actions"
)

type artifactReport struct {
	Artifact   string                     `json:"artifact"`
	Package    string                     `json:"package"`
	Version    string                     `json:"version"`
	Error      string                     `json:"error,omitempty"`
	Violations []actions.ArchiveViolation `json:"violations,omitempty"`
}

func main() {
	asJSON := flag.Bool("json", false, "emit the report as JSON")
	failOnFindings := flag.Bool("fail-on-findings", false, "exit non-zero when any artifact would be rejected")
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: audit-package-archives [--json] [--fail-on-findings] <path-or-dir>...")
		os.Exit(2)
	}

	artifacts, err := collectArtifacts(flag.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, "audit-package-archives:", err)
		os.Exit(2)
	}
	if len(artifacts) == 0 {
		fmt.Fprintln(os.Stderr, "audit-package-archives: no .tgz artifacts found")
		os.Exit(2)
	}

	reports := make([]artifactReport, 0, len(artifacts))
	for _, artifact := range artifacts {
		pkg, version := packageIdentityFromFilename(artifact)
		report := artifactReport{Artifact: artifact, Package: pkg, Version: version}
		violations, err := actions.ValidateArtifactArchive(artifact, pkg)
		if err != nil {
			report.Error = err.Error()
		}
		report.Violations = violations
		reports = append(reports, report)
	}
	sort.Slice(reports, func(i, j int) bool { return reports[i].Artifact < reports[j].Artifact })

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(reports); err != nil {
			fmt.Fprintln(os.Stderr, "audit-package-archives:", err)
			os.Exit(2)
		}
	} else {
		printText(reports)
	}

	rejected := 0
	for _, r := range reports {
		if len(r.Violations) > 0 || r.Error != "" {
			rejected++
		}
	}
	if rejected > 0 && *failOnFindings {
		os.Exit(1)
	}
}

func printText(reports []artifactReport) {
	mappedTotal, unmappedTotal, rejected, unreadable := 0, 0, 0, 0
	byReason := map[string]int{}

	for _, r := range reports {
		if r.Error != "" {
			unreadable++
			fmt.Printf("UNREADABLE  %s\n            %s\n", filepath.Base(r.Artifact), r.Error)
			continue
		}
		if len(r.Violations) == 0 {
			fmt.Printf("clean       %s\n", filepath.Base(r.Artifact))
			continue
		}
		rejected++
		fmt.Printf("REJECTED    %s (package=%s version=%s, %d finding(s))\n",
			filepath.Base(r.Artifact), r.Package, r.Version, len(r.Violations))
		for _, v := range r.Violations {
			scope := "unmapped/inert"
			if v.Mapped {
				scope = "MAPPED/would-extract"
				mappedTotal++
			} else {
				unmappedTotal++
			}
			byReason[v.Reason]++
			fmt.Printf("            [%s] %s\n              entry:  %s\n              detail: %s\n",
				scope, v.Reason, v.Entry, v.Detail)
		}
	}

	fmt.Printf("\n── summary ──────────────────────────────────────────────\n")
	fmt.Printf("artifacts scanned : %d\n", len(reports))
	fmt.Printf("clean             : %d\n", len(reports)-rejected-unreadable)
	fmt.Printf("would be rejected : %d\n", rejected)
	if unreadable > 0 {
		fmt.Printf("unreadable        : %d\n", unreadable)
	}
	fmt.Printf("findings (mapped) : %d   ← would have been written to disk\n", mappedTotal)
	fmt.Printf("findings (inert)  : %d   ← ignored by the current installer\n", unmappedTotal)
	if len(byReason) > 0 {
		reasons := make([]string, 0, len(byReason))
		for reason := range byReason {
			reasons = append(reasons, reason)
		}
		sort.Strings(reasons)
		fmt.Printf("by reason         :\n")
		for _, reason := range reasons {
			fmt.Printf("  %-28s %d\n", reason, byReason[reason])
		}
	}
	if rejected == 0 && unreadable == 0 {
		fmt.Printf("\nRESULT: clean — the fail-closed gate admits every scanned artifact.\n")
	} else {
		fmt.Printf("\nRESULT: %d artifact(s) must be repaired or republished before the gate is enabled.\n", rejected+unreadable)
	}
}

func collectArtifacts(paths []string) ([]string, error) {
	var out []string
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			out = append(out, p)
			continue
		}
		err = filepath.WalkDir(p, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(path, ".tgz") {
				out = append(out, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(out)
	return out, nil
}

// packageIdentityFromFilename parses the conventional artifact filename
// <name>_<version>_<platform>.tgz. Versions may themselves contain underscores
// (minio's RELEASE.2025-09-07T16-13-09Z form does not, but scylla builds have),
// so the name is taken as everything before the FIRST underscore and the
// version as everything between the first and last.
func packageIdentityFromFilename(artifact string) (pkg, version string) {
	base := strings.TrimSuffix(filepath.Base(artifact), ".tgz")
	first := strings.Index(base, "_")
	last := strings.LastIndex(base, "_")
	if first < 0 || last <= first {
		return base, ""
	}
	return base[:first], base[first+1 : last]
}
