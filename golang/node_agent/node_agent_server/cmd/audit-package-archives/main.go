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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/actions"
)

type artifactReport struct {
	Node       string                     `json:"node"`
	Artifact   string                     `json:"artifact"`
	SHA256     string                     `json:"sha256,omitempty"`
	Package    string                     `json:"package"`
	Version    string                     `json:"version"`
	Error      string                     `json:"error,omitempty"`
	Violations []actions.ArchiveViolation `json:"violations,omitempty"`
}

func main() {
	asJSON := flag.Bool("json", false, "emit the report as JSON")
	failOnFindings := flag.Bool("fail-on-findings", false, "exit non-zero when any artifact would be rejected")
	node := flag.String("node", "", "node label for this report (default: hostname) — lets per-node reports be merged and deduplicated by digest")
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

	label := strings.TrimSpace(*node)
	if label == "" {
		if hostname, err := os.Hostname(); err == nil {
			label = hostname
		} else {
			label = "unknown-node"
		}
	}

	reports := make([]artifactReport, 0, len(artifacts))
	for _, artifact := range artifacts {
		pkg, version := packageIdentityFromFilename(artifact)
		report := artifactReport{Node: label, Artifact: artifact, Package: pkg, Version: version}
		if digest, err := sha256File(artifact); err == nil {
			report.SHA256 = digest
		} else {
			report.Error = err.Error()
		}
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
	printDigestSummary(reports)

	if rejected == 0 && unreadable == 0 {
		fmt.Printf("\nRESULT: clean — the fail-closed gate admits every scanned artifact.\n")
	} else {
		fmt.Printf("\nRESULT: %d artifact(s) must be repaired or republished before the gate is enabled.\n", rejected+unreadable)
	}
}

// printDigestSummary collapses proof by archive content while keeping placement
// visible. The same package copied to five nodes is ONE artifact inspected, not
// five independent proofs — counting copies would overstate coverage. But two
// DIFFERENT digests for one package+version is node drift, which is worth more
// attention than either copy on its own.
func printDigestSummary(reports []artifactReport) {
	type placement struct{ node, path string }
	byDigest := map[string][]placement{}
	identityOf := map[string]string{}
	digestsPerIdentity := map[string]map[string]struct{}{}
	nodes := map[string]struct{}{}

	for _, r := range reports {
		if r.SHA256 == "" {
			continue
		}
		byDigest[r.SHA256] = append(byDigest[r.SHA256], placement{r.Node, r.Artifact})
		identity := r.Package + "@" + r.Version
		identityOf[r.SHA256] = identity
		if digestsPerIdentity[identity] == nil {
			digestsPerIdentity[identity] = map[string]struct{}{}
		}
		digestsPerIdentity[identity][r.SHA256] = struct{}{}
		nodes[r.Node] = struct{}{}
	}
	if len(byDigest) == 0 {
		return
	}

	copies := 0
	for _, places := range byDigest {
		copies += len(places)
	}
	fmt.Printf("unique archives   : %d   (from %d cop(ies) across %d node(s))\n",
		len(byDigest), copies, len(nodes))

	// Drift: one package identity carrying more than one distinct digest.
	var drifted []string
	for identity, digests := range digestsPerIdentity {
		if len(digests) > 1 {
			drifted = append(drifted, identity)
		}
	}
	if len(drifted) == 0 {
		return
	}
	sort.Strings(drifted)
	fmt.Printf("\nDIGEST DRIFT — one package identity with more than one archive content:\n")
	for _, identity := range drifted {
		fmt.Printf("  %s\n", identity)
		for digest, places := range byDigest {
			if identityOf[digest] != identity {
				continue
			}
			sort.Slice(places, func(i, j int) bool { return places[i].node < places[j].node })
			for _, p := range places {
				fmt.Printf("    %s  %s:%s\n", digest[:16], p.node, p.path)
			}
		}
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
// <name>_<version>_<os>_<arch>.tgz.
//
// The platform is TWO underscore-separated fields (linux_amd64), so the version
// is everything between the first underscore and the last two — taking
// everything up to the final underscore would report "255.4_linux" for
// libnss-resolve_255.4_linux_amd64.tgz. Versions may themselves contain
// underscores, so the middle is rejoined rather than assumed to be one field.
func packageIdentityFromFilename(artifact string) (pkg, version string) {
	base := strings.TrimSuffix(filepath.Base(artifact), ".tgz")
	parts := strings.Split(base, "_")
	switch {
	case len(parts) >= 4:
		return parts[0], strings.Join(parts[1:len(parts)-2], "_")
	case len(parts) >= 2:
		return parts[0], strings.Join(parts[1:], "_")
	default:
		return base, ""
	}
}

// sha256File is the archive identity used to deduplicate proof across nodes.
// The same package copied to five nodes is one artifact to inspect, not five
// independent proofs — but the per-node paths are retained so placement drift
// stays visible.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
