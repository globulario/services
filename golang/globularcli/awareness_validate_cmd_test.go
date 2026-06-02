package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helpers ─────────────────────────────────────────────────────────────────

// writeYAML writes a file under a fresh tempdir-rooted "docs/awareness"
// and returns the absolute repo root + the relative path of the written
// file. Simplifies the per-test "set up a tiny awareness tree" pattern.
func writeYAML(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return root
}

// runValidateOn is a thin wrapper so tests stay focused on inputs/outputs.
func runValidateOn(t *testing.T, root string) *validateReport {
	t.Helper()
	r, err := runValidate(root, []string{
		filepath.Join(root, "docs/awareness"),
		filepath.Join(root, "docs/intent"),
	})
	if err != nil {
		t.Fatalf("runValidate: %v", err)
	}
	return r
}

func hasFinding(r *validateReport, check string) bool {
	for _, f := range r.Findings {
		if f.Check == check {
			return true
		}
	}
	return false
}

func findingsByCheck(r *validateReport, check string) []validateFinding {
	var out []validateFinding
	for _, f := range r.Findings {
		if f.Check == check {
			out = append(out, f)
		}
	}
	return out
}

// ─── Tests ───────────────────────────────────────────────────────────────

// 1. Dangling related_invariants → error.
func TestValidate_DanglingInvariantRef(t *testing.T) {
	root := writeYAML(t, map[string]string{
		"docs/awareness/invariants.yaml": `
invariants:
  - id: known.invariant.one
    title: known
`,
		"docs/awareness/failure_modes.yaml": `
failure_modes:
  - id: some.failure
    title: x
    related_invariants:
      - known.invariant.one
      - missing.invariant.two
`,
	})
	r := runValidateOn(t, root)
	fs := findingsByCheck(r, "dangling_invariant_ref")
	if len(fs) != 1 {
		t.Fatalf("want 1 dangling_invariant_ref, got %d (all=%v)", len(fs), r.Findings)
	}
	if fs[0].Ref != "missing.invariant.two" {
		t.Errorf("ref: want missing.invariant.two, got %q", fs[0].Ref)
	}
	if fs[0].Severity != "error" {
		t.Errorf("severity: want error, got %q", fs[0].Severity)
	}
}

// 2. Dangling related_failure_modes → error.
func TestValidate_DanglingFailureModeRef(t *testing.T) {
	root := writeYAML(t, map[string]string{
		"docs/awareness/failure_modes.yaml": `
failure_modes:
  - id: known.failure.one
    title: known
`,
		"docs/awareness/invariants.yaml": `
invariants:
  - id: some.invariant
    title: x
    related_failure_modes:
      - known.failure.one
      - missing.failure.two
`,
	})
	r := runValidateOn(t, root)
	fs := findingsByCheck(r, "dangling_failure_mode_ref")
	if len(fs) != 1 {
		t.Fatalf("want 1 dangling_failure_mode_ref, got %d", len(fs))
	}
	if fs[0].Ref != "missing.failure.two" {
		t.Errorf("ref: want missing.failure.two, got %q", fs[0].Ref)
	}
}

// 3. Missing source file (expressed_by points at nonexistent .go) → error.
func TestValidate_MissingSourceFile(t *testing.T) {
	root := writeYAML(t, map[string]string{
		"docs/intent/x.yaml": `
id: foo.intent
level: principle
title: x
expressed_by:
  - golang/exists/exists.go
  - golang/does_not_exist/missing.go
`,
		"golang/exists/exists.go": "package exists\n",
	})
	r := runValidateOn(t, root)
	fs := findingsByCheck(r, "missing_source_file")
	if len(fs) != 1 {
		t.Fatalf("want 1 missing_source_file, got %d", len(fs))
	}
	if fs[0].Ref != "golang/does_not_exist/missing.go" {
		t.Errorf("ref: want missing path, got %q", fs[0].Ref)
	}
}

// 3b. Legacy paths prefixed with "services/" must ALSO be accepted when
//     the actual file lives at the canonical root. This mirrors real entries
//     like "services/golang/dependency/modes.go" in the live YAMLs.
func TestValidate_MissingSourceFile_AcceptsLegacyServicesPrefix(t *testing.T) {
	root := writeYAML(t, map[string]string{
		"docs/intent/x.yaml": `
id: foo.intent
level: principle
title: x
expressed_by:
  - services/golang/legacy/path.go
`,
		"golang/legacy/path.go": "package legacy\n",
	})
	r := runValidateOn(t, root)
	if hasFinding(r, "missing_source_file") {
		t.Errorf("services/ prefix should resolve to repo root; got findings: %v", r.Findings)
	}
}

// 4. Missing reference file in an ImplementationPattern.
func TestValidate_MissingReferenceFile_InImplementationPattern(t *testing.T) {
	root := writeYAML(t, map[string]string{
		"docs/awareness/implementation_patterns/p.yaml": `
id: globular.pattern.x
class: ImplementationPattern
label: x
when_to_use: [creating x]
reference_files:
  - path: golang/echo/echo_client/echo_client.go
    role: canonical
  - path: golang/does_not_exist/foo.go
    role: nope
`,
		"golang/echo/echo_client/echo_client.go": "package echo_client\n",
	})
	r := runValidateOn(t, root)
	fs := findingsByCheck(r, "missing_source_file")
	if len(fs) != 1 {
		t.Fatalf("want 1 missing reference file, got %d (findings=%v)", len(fs), r.Findings)
	}
	if !strings.Contains(fs[0].Ref, "does_not_exist") {
		t.Errorf("ref unexpected: %v", fs[0].Ref)
	}
}

// 5. Duplicate ID across files → error.
func TestValidate_DuplicateID(t *testing.T) {
	root := writeYAML(t, map[string]string{
		"docs/awareness/invariants.yaml": `
invariants:
  - id: dup.invariant
    title: first
`,
		"docs/awareness/echo_service_invariants.yaml": `
invariants:
  - id: dup.invariant
    title: second
  - id: only.in.echo
    title: x
`,
	})
	r := runValidateOn(t, root)
	fs := findingsByCheck(r, "duplicate_id")
	if len(fs) != 1 {
		t.Fatalf("want 1 duplicate_id, got %d", len(fs))
	}
	if fs[0].EntityID != "dup.invariant" {
		t.Errorf("entity id: want dup.invariant, got %q", fs[0].EntityID)
	}
	if !strings.Contains(fs[0].Message, "invariants.yaml") || !strings.Contains(fs[0].Message, "echo_service_invariants.yaml") {
		t.Errorf("message should name both source files: %s", fs[0].Message)
	}
}

// 6. Clean tree → zero findings.
func TestValidate_CleanTreeNoFindings(t *testing.T) {
	root := writeYAML(t, map[string]string{
		"docs/awareness/invariants.yaml": `
invariants:
  - id: inv.one
    title: x
`,
		"docs/awareness/failure_modes.yaml": `
failure_modes:
  - id: fm.one
    title: x
    related_invariants: [inv.one]
`,
		"docs/intent/foo.yaml": `
id: foo.intent
level: principle
title: x
expressed_by:
  - golang/foo/foo.go
related_invariants: [inv.one]
`,
		"golang/foo/foo.go": "package foo\n",
	})
	r := runValidateOn(t, root)
	if len(r.Findings) != 0 {
		t.Errorf("clean tree should have 0 findings, got %d: %v", len(r.Findings), r.Findings)
	}
}

// 7. Reference is to a non-.go file (e.g. operators/cluster-doctor.md) →
//    not flagged. The validator's file-existence check is scoped to Go
//    source paths; docs/awareness YAML cross-refs follow different shapes.
func TestValidate_NonGoReferences_NotFlagged(t *testing.T) {
	root := writeYAML(t, map[string]string{
		"docs/intent/foo.yaml": `
id: foo.intent
level: principle
title: x
expressed_by:
  - operators/cluster-doctor.md
  - awareness/subsystem_boundaries.yaml
`,
	})
	r := runValidateOn(t, root)
	if hasFinding(r, "missing_source_file") {
		t.Errorf("non-.go paths must not be checked; findings=%v", r.Findings)
	}
}

// 8. ImplementationPattern with file-path reference that DOES exist passes.
func TestValidate_ImplementationPattern_ResolvedReference(t *testing.T) {
	root := writeYAML(t, map[string]string{
		"docs/awareness/implementation_patterns/p.yaml": `
id: globular.pattern.x
class: ImplementationPattern
label: x
when_to_use: [creating x]
reference_files:
  - path: golang/echo/echo_client/echo_client.go
    role: canonical
`,
		"golang/echo/echo_client/echo_client.go": "package echo_client\n",
	})
	r := runValidateOn(t, root)
	if hasFinding(r, "missing_source_file") {
		t.Errorf("resolved reference should not flag; findings=%v", r.Findings)
	}
}

// 9. Bad YAML in one file emits a parse-warning but does NOT abort the
//    scan — other files must still be validated.
func TestValidate_BadYAMLDoesNotAbortScan(t *testing.T) {
	root := writeYAML(t, map[string]string{
		"docs/awareness/invariants.yaml": `
invariants:
  - id: x
    title: x
`,
		"docs/awareness/bad.yaml": `: : not yaml`,
		"docs/awareness/failure_modes.yaml": `
failure_modes:
  - id: fm.one
    title: x
    related_invariants: [missing.one]
`,
	})
	r := runValidateOn(t, root)
	if !hasFinding(r, "yaml_parse_failed") {
		t.Errorf("bad yaml should emit yaml_parse_failed warning")
	}
	if !hasFinding(r, "dangling_invariant_ref") {
		t.Errorf("scan should continue past bad yaml; missing.one ref should still surface as dangling")
	}
}

// 10. JSON output is parseable + counts match findings.
func TestValidate_JSONStructureIsParseable(t *testing.T) {
	root := writeYAML(t, map[string]string{
		"docs/awareness/invariants.yaml": `
invariants:
  - id: only.one
    title: x
`,
	})
	r := runValidateOn(t, root)
	if r.RepoRoot == "" {
		t.Errorf("repo_root field should be populated")
	}
	if len(r.Scanned) == 0 {
		t.Errorf("scanned_dirs should list scanned files")
	}
	// Counts map must mirror the findings list exactly.
	gotCounts := map[string]int{}
	for _, f := range r.Findings {
		gotCounts[f.Check]++
	}
	if len(gotCounts) != len(r.Counts) {
		t.Errorf("counts map vs actual finding counts diverged: %v vs %v", r.Counts, gotCounts)
	}
}
