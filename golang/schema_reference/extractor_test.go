package schema_reference

import (
	"os"
	"path/filepath"
	"testing"
)

// Extractor tests operate on tiny synthetic Go files written into a
// tempdir so they are independent of the real repo's pragma coverage.
// A future PR that adds / removes pragmas in production code does not
// need to update these tests.

func writeTempGo(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

func TestExtractFileHappyPath(t *testing.T) {
	dir := t.TempDir()
	path := writeTempGo(t, dir, "a.go", `package x

// Foo is the state blob. Important things happen here.
//
// +globular:schema:key="/globular/foo/{name}"
// +globular:schema:writer="globular-x"
// +globular:schema:readers="globular-y,globular-z"
// +globular:schema:description="The foo record."
// +globular:schema:invariants="Must be non-empty; meta.generation monotonic."
type Foo struct {
	N string
}
`)
	entries, errs := ExtractFile(path)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	e := entries[0]
	if e.KeyPattern != "/globular/foo/{name}" {
		t.Errorf("KeyPattern = %q", e.KeyPattern)
	}
	if e.Writer != "globular-x" {
		t.Errorf("Writer = %q", e.Writer)
	}
	if len(e.Readers) != 2 || e.Readers[0] != "globular-y" || e.Readers[1] != "globular-z" {
		t.Errorf("Readers = %v", e.Readers)
	}
	if e.Description != "The foo record." {
		t.Errorf("Description = %q", e.Description)
	}
	if e.TypeName != "Foo" {
		t.Errorf("TypeName = %q", e.TypeName)
	}
	if e.SourceFile != path {
		t.Errorf("SourceFile = %q, want %q", e.SourceFile, path)
	}
	if e.SourceLine <= 0 {
		t.Errorf("SourceLine = %d, want >0", e.SourceLine)
	}
}

func TestExtractFileMissingRequiredFields(t *testing.T) {
	dir := t.TempDir()
	// Missing writer → should error.
	path := writeTempGo(t, dir, "b.go", `package x
// +globular:schema:key="/globular/nowriter"
type NoWriter struct{}
`)
	_, errs := ExtractFile(path)
	if len(errs) == 0 {
		t.Fatal("expected error for missing writer, got none")
	}
}

func TestExtractFileOrphanPragma(t *testing.T) {
	dir := t.TempDir()
	// Pragma block not followed by a type declaration.
	path := writeTempGo(t, dir, "c.go", `package x
// +globular:schema:key="/globular/orphan"
// +globular:schema:writer="globular-x"
var notAType = 42
`)
	entries, errs := ExtractFile(path)
	if len(entries) != 0 {
		t.Errorf("orphan pragma should not produce an entry, got %d", len(entries))
	}
	if len(errs) == 0 {
		t.Error("expected orphan pragma error, got none")
	}
}

func TestExtractTreeSortAndDedup(t *testing.T) {
	dir := t.TempDir()
	writeTempGo(t, dir, "z.go", `package x
// +globular:schema:key="/globular/z/{name}"
// +globular:schema:writer="globular-x"
type Z struct{}
`)
	writeTempGo(t, dir, "a.go", `package x
// +globular:schema:key="/globular/a/{name}"
// +globular:schema:writer="globular-x"
type A struct{}
`)
	res, errs := ExtractTree(dir)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(res.Entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(res.Entries))
	}
	// Sorted by KeyPattern → a before z.
	if res.Entries[0].KeyPattern != "/globular/a/{name}" {
		t.Errorf("sort order wrong: first = %q", res.Entries[0].KeyPattern)
	}
	if res.Source != "schema-extractor" {
		t.Errorf("Source = %q", res.Source)
	}
}

func TestExtractTreeDuplicateKeyPattern(t *testing.T) {
	dir := t.TempDir()
	writeTempGo(t, dir, "one.go", `package x
// +globular:schema:key="/globular/dup"
// +globular:schema:writer="globular-a"
type One struct{}
`)
	writeTempGo(t, dir, "two.go", `package x
// +globular:schema:key="/globular/dup"
// +globular:schema:writer="globular-b"
type Two struct{}
`)
	_, errs := ExtractTree(dir)
	if len(errs) == 0 {
		t.Fatal("expected duplicate key_pattern error, got none")
	}
}

func TestExtractTreeSkipsTestAndPbFiles(t *testing.T) {
	dir := t.TempDir()
	// These should be skipped.
	writeTempGo(t, dir, "foo_test.go", `package x
// +globular:schema:key="/globular/ignore1"
// +globular:schema:writer="globular-x"
type Ignored1 struct{}
`)
	writeTempGo(t, dir, "gen.pb.go", `package x
// +globular:schema:key="/globular/ignore2"
// +globular:schema:writer="globular-x"
type Ignored2 struct{}
`)
	// This should be picked up.
	writeTempGo(t, dir, "real.go", `package x
// +globular:schema:key="/globular/real"
// +globular:schema:writer="globular-x"
type Real struct{}
`)
	res, errs := ExtractTree(dir)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(res.Entries) != 1 || res.Entries[0].KeyPattern != "/globular/real" {
		t.Errorf("expected only /globular/real, got %+v", res.Entries)
	}
}

func TestExtractFileRepeatedPragmaFieldJoins(t *testing.T) {
	// Repeated fields (e.g. two invariants lines) must be joined, not
	// the last-writer-wins. This is the explicit extension path for
	// types with multiple invariants.
	dir := t.TempDir()
	path := writeTempGo(t, dir, "multi.go", `package x
// +globular:schema:key="/globular/multi"
// +globular:schema:writer="globular-x"
// +globular:schema:invariants="First rule."
// +globular:schema:invariants="Second rule."
type Multi struct{}
`)
	entries, errs := ExtractFile(path)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if entries[0].Invariants != "First rule.; Second rule." {
		t.Errorf("joined invariants wrong: %q", entries[0].Invariants)
	}
}
