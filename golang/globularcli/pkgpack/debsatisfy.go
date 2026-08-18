package pkgpack

// Consumer-side satisfiability proof for bundled .deb files.
//
// The producer gate (debprovenance.go) proves the bundle contains the bytes it
// declares. This proves those bytes can actually install on the declared target
// platform. The two are independent and both are required: on 2026-08-14 the
// shipped deb was perfectly satisfiable ON THE BUILDER, which is exactly why
// nobody noticed it was also unreproducible. Wrong bytes can be perfectly
// installable, and right bytes can be uninstallable.
//
// The libnss-resolve case is decidable with no cluster involved:
//
//	bundle:   libnss-resolve = X
//	requires: systemd-resolved = X   (exact equality)
//	policy:   systemd-resolved is on the never-bundle list
//	target:   systemd-resolved = B
//
// There is no solution unless B == X. That is arithmetic, and it belongs at
// build time rather than in a joining node's dpkg output.
//
// Resolution universe: the declared baseline's `provides` (what the base image
// supplies) plus every deb bundled in the same artifact (what we ship
// alongside). A dependency resolving in neither is UNKNOWN — and UNKNOWN fails
// closed, because an unprovable dependency must not improve the verdict.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// PlatformBaseline is the declared target platform a release must install onto.
type PlatformBaseline struct {
	ID              string            `json:"id"`
	OS              string            `json:"os"`
	Image           string            `json:"image"`
	Arch            string            `json:"arch"`
	ArchiveSnapshot string            `json:"archive_snapshot"`
	ExpectedSystemd string            `json:"expected_systemd"`
	Provides        map[string]string `json:"provides"`
}

// LoadPlatformBaseline reads the declared baseline. A missing or unreadable
// baseline is an error, never a skip: assembly must not proceed by deciding the
// target platform is unknown.
func LoadPlatformBaseline(path string) (*PlatformBaseline, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read platform baseline %s: %w", path, err)
	}
	var b PlatformBaseline
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil, fmt.Errorf("parse platform baseline %s: %w", path, err)
	}
	if b.ID == "" || len(b.Provides) == 0 {
		return nil, fmt.Errorf("platform baseline %s declares no id or no provides", path)
	}
	return &b, nil
}

// DepVerdict is the result for one dependency clause.
type DepVerdict struct {
	Raw        string
	Name       string
	Op         string
	Version    string
	ResolvedBy string // "baseline" | "bundle" | ""
	Available  string
	Satisfied  bool
	Reason     string
}

// SatisfiabilityResult is the per-deb consumer verdict.
type SatisfiabilityResult struct {
	Package   string
	Verdicts  []DepVerdict
	Satisfied bool
}

// depClause matches "name", "name (>= 1.2)", "name:any (= 1.2-3)".
var depClause = regexp.MustCompile(`^([a-zA-Z0-9][a-zA-Z0-9+.\-]*)(?::[a-zA-Z0-9]+)?\s*(?:\(\s*(<<|<=|=|>=|>>|<|>)\s*([^)]+)\))?$`)

// CheckSatisfiable evaluates one deb's dependencies against the baseline plus
// the set of packages bundled alongside it.
func CheckSatisfiable(prov *DebProvenance, baseline *PlatformBaseline, bundled map[string]string) *SatisfiabilityResult {
	res := &SatisfiabilityResult{Package: prov.Package, Satisfied: true}

	all := append(append([]string{}, prov.PreDepends...), prov.Depends...)
	for _, raw := range all {
		// An alternation ("a | b") is satisfied if any branch is.
		branches := strings.Split(raw, "|")
		var branchVerdicts []DepVerdict
		ok := false
		for _, br := range branches {
			v := evalClause(strings.TrimSpace(br), baseline, bundled)
			branchVerdicts = append(branchVerdicts, v)
			if v.Satisfied {
				ok = true
				break
			}
		}
		chosen := branchVerdicts[len(branchVerdicts)-1]
		if ok {
			chosen = branchVerdicts[len(branchVerdicts)-1]
		}
		chosen.Raw = strings.TrimSpace(raw)
		if !ok {
			res.Satisfied = false
		}
		chosen.Satisfied = ok
		res.Verdicts = append(res.Verdicts, chosen)
	}
	return res
}

func evalClause(clause string, baseline *PlatformBaseline, bundled map[string]string) DepVerdict {
	v := DepVerdict{Raw: clause}
	m := depClause.FindStringSubmatch(clause)
	if m == nil {
		v.Reason = "unparseable dependency clause"
		return v
	}
	v.Name, v.Op, v.Version = m[1], m[2], strings.TrimSpace(m[3])

	available, from := "", ""
	if ver, ok := bundled[v.Name]; ok {
		available, from = ver, "bundle"
	} else if ver, ok := baseline.Provides[v.Name]; ok {
		available, from = ver, "baseline"
	}

	if from == "" {
		// Neither shipped nor supplied by the declared target. We cannot prove
		// this installs offline, so we do not claim it does.
		v.Reason = "not provided by the declared baseline and not bundled — UNKNOWN, refused (an unprovable dependency cannot improve the verdict)"
		return v
	}
	v.ResolvedBy, v.Available = from, available

	if v.Op == "" {
		v.Satisfied = true
		return v
	}
	sat, err := dpkgCompare(available, v.Op, v.Version)
	if err != nil {
		v.Reason = fmt.Sprintf("version comparison failed: %v", err)
		return v
	}
	v.Satisfied = sat
	if !sat {
		v.Reason = fmt.Sprintf("%s provides %s, dependency requires %s %s", from, available, v.Op, v.Version)
	}
	return v
}

// dpkgCompare defers to dpkg for Debian version ordering rather than
// reimplementing epoch/tilde/alphanumeric comparison and getting it subtly
// wrong.
func dpkgCompare(have, op, want string) (bool, error) {
	switch op {
	case "<":
		op = "<<"
	case ">":
		op = ">>"
	}
	cmd := exec.Command("dpkg", "--compare-versions", have, op, want)
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return false, nil // ran fine, comparison is false
		}
		return false, err
	}
	return true, nil
}

// FormatProvenanceRecord renders the per-deb record for build output. Both
// halves in one place: where the bytes came from, and whether they can install.
func FormatProvenanceRecord(p *DebProvenance, s *SatisfiabilityResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "  deb %s\n", p.Package)
	fmt.Fprintf(&b, "    version          %s\n", p.Version)
	fmt.Fprintf(&b, "    architecture     %s\n", p.Architecture)
	fmt.Fprintf(&b, "    sha256           %s\n", p.SHA256)
	fmt.Fprintf(&b, "    declared_source  %s\n", p.DeclaredSource)
	fmt.Fprintf(&b, "    source_repo      %s\n", p.SourceRepo)
	fmt.Fprintf(&b, "    source_revision  %s\n", p.SourceRevision)
	fmt.Fprintf(&b, "    source_blob      %s\n", p.SourceBlobOID)
	if len(p.PreDepends) > 0 {
		fmt.Fprintf(&b, "    pre-depends      %s\n", strings.Join(p.PreDepends, ", "))
	}
	if len(p.Depends) > 0 {
		fmt.Fprintf(&b, "    depends          %s\n", strings.Join(p.Depends, ", "))
	}
	verdict := "SATISFIABLE"
	if !s.Satisfied {
		verdict = "UNSATISFIABLE"
	}
	fmt.Fprintf(&b, "    satisfiability   %s\n", verdict)
	for _, v := range s.Verdicts {
		if v.Satisfied {
			continue
		}
		fmt.Fprintf(&b, "      REFUSED %s: %s\n", v.Raw, v.Reason)
	}
	return b.String()
}
