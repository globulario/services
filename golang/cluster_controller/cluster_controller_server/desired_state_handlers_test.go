package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// funcBodySource returns the source text of the named method/function in file.
func funcBodySource(t *testing.T, file, funcName string) string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", file, err)
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	src := string(raw)
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil || fn.Name.Name != funcName || fn.Body == nil {
			continue
		}
		start := fset.Position(fn.Body.Pos()).Offset
		end := fset.Position(fn.Body.End()).Offset
		if start < 0 || end > len(src) || start >= end {
			t.Fatalf("bad offsets for %s: %d..%d", funcName, start, end)
		}
		return src[start:end]
	}
	t.Fatalf("function %q not found in %s", funcName, file)
	return ""
}

// TestImportInstalledToDesiredMustNotKeyArtifactLookupOnBuildNumber pins the
// contract that the desired-state import must not propagate the INSTALLED
// record's build_number into the DesiredService it upserts.
//
// Why this matters. build_number is a display-only monotonic counter --
// pkgpack/manifest.go states outright "NOT used in convergence" -- and the two
// sides of the system number it independently:
//
//	locally-built bundle : build_number = 1785443290 (timestamp-style)
//	repository at publish: build_number = 1          (its own sequence)
//
// upsertOne feeds DesiredService.BuildNumber to validateArtifactInRepo, which
// calls GetArtifactManifest(ref, buildNumber). A non-zero value forces an
// EXACT-key manifest lookup (readManifestWithFallback -> artifactKeyWithBuild),
// which can never match when the two sides disagree. Observed on a clean Day-0
// install of 1.2.279: all 24 services were rejected with
//
//	artifact event@1.2.279 (build 1785443290) not found in repository
//
// leaving desired state permanently empty, which in turn pinned
// release_boundary A0..A3 at INDETERMINATE for every service.
//
// Passing 0 selects readManifestWithFallback's documented path, "resolve to the
// latest (highest) PUBLISHED build number first", and upsertOne then persists
// the repository's authoritative build_id -- the real convergence identity.
//
// Guards invariant:desired.keyed_by_kind_and_name (desired records are keyed by
// (kind, name), not by build number) and
// invariant:meta.identity_computation_must_be_invariant (identity must not vary
// by which writer observed it).
//
// SCOPE: this is a structural guard on the call site, not an end-to-end proof.
// importInstalledToDesired requires a live resource store and repository client,
// so there is no unit-level harness for it in this package (see the note in
// phantom_guard_test.go:151). This test catches reintroduction of the exact
// defect; it does not prove the repository resolves correctly -- that is
// covered by the live seed check in the release runbook.
func TestImportInstalledToDesiredMustNotKeyArtifactLookupOnBuildNumber(t *testing.T) {
	body := funcBodySource(t, "desired_state_handlers.go", "importInstalledToDesired")

	// Locate the DesiredService literal handed to upsertOne.
	const marker = "srv.upsertOne(ctx, &cluster_controllerpb.DesiredService{"
	idx := strings.Index(body, marker)
	if idx < 0 {
		t.Fatalf("upsertOne call with a DesiredService literal not found in "+
			"importInstalledToDesired; the guard needs updating.\nbody:\n%s", body)
	}
	rest := body[idx+len(marker):]
	closing := strings.Index(rest, "}")
	if closing < 0 {
		t.Fatal("unterminated DesiredService literal after upsertOne")
	}
	literal := rest[:closing]

	if strings.Contains(literal, "BuildNumber") {
		t.Errorf("importInstalledToDesired passes BuildNumber to upsertOne:\n"+
			"  DesiredService{%s}\n"+
			"build_number is display-only and is numbered independently by the "+
			"local bundle and the repository, so a non-zero value makes "+
			"validateArtifactInRepo do an exact-key lookup that cannot match — "+
			"every service is skipped NotFound and desired state stays empty. "+
			"Leave it 0 so the repository resolves the latest PUBLISHED build.",
			literal)
	}

	// The rationale must stay attached to the call site: a future edit that
	// re-adds the field should have to delete an explicit explanation first.
	if !strings.Contains(body, "display-only") {
		t.Error("the comment explaining why BuildNumber is omitted has been " +
			"removed from importInstalledToDesired; keep it so the omission " +
			"does not read as an oversight and get 'fixed' back in")
	}
}
