package opsknowledge

import (
	"strings"
	"testing"
)

// validEntry builds an entry that passes every rule.
func validEntry() Entry {
	return Entry{
		ID:    "ops.day-1.example.thing",
		Type:  TypeReference,
		Title: "Test entry",
		Tags:  []string{StageDay1, "example"},
		AppliesWhen: AppliesWhen{
			ClusterPhases:   []string{StageDay1, StageDay2},
			ServicesPresent: []string{"cluster-controller"},
			ServicesHealthy: []string{"cluster-controller"},
		},
		Content: "non-empty content",
		Provenance: Provenance{
			Source:    ProvenanceSourceSeed,
			Immutable: true,
		},
	}
}

// validRefs builds Refs that allow link checks to succeed.
func validRefs() *Refs {
	return &Refs{
		InvariantIDs:   map[string]bool{"some.invariant": true},
		FailureModeIDs: map[string]bool{"some.failure": true},
		RunbookPaths:   map[string]bool{"runbooks/some-runbook.yaml": true},
		SeenEntryIDs:   map[string]string{},
	}
}

func validFile(entries ...Entry) *File {
	return &File{
		SchemaVersion: 1,
		FileKind:      FileKindStage,
		Metadata:      Metadata{Title: "Test", Description: "Test description"},
		Entries:       entries,
		Path:          "/test/path.yaml",
	}
}

func findingCodes(fs []Finding) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.Code
	}
	return out
}

func TestValidate_HappyPath(t *testing.T) {
	f := validFile(validEntry())
	findings := Validate(f, validRefs())
	if HasErrors(findings) {
		t.Fatalf("expected no errors, got: %v", findingCodes(findings))
	}
}

func TestValidate_FileLevel(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(*File)
		wantCode string
	}{
		{"schema_version_zero", func(f *File) { f.SchemaVersion = 0 }, "schema_version_unsupported"},
		{"schema_version_two", func(f *File) { f.SchemaVersion = 2 }, "schema_version_unsupported"},
		{"file_kind_invalid", func(f *File) { f.FileKind = "bogus" }, "file_kind_invalid"},
		{"metadata_title_missing", func(f *File) { f.Metadata.Title = "" }, "metadata_title_missing"},
		{"no_entries", func(f *File) { f.Entries = nil }, "no_entries"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := validFile(validEntry())
			tc.mutate(f)
			findings := Validate(f, validRefs())
			if !containsCode(findings, tc.wantCode) {
				t.Fatalf("expected code %q in findings, got: %v", tc.wantCode, findingCodes(findings))
			}
		})
	}
}

func TestValidate_EntryLevel(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(*Entry)
		wantCode string
	}{
		{"id_missing", func(e *Entry) { e.ID = "" }, "id_missing"},
		{"id_namespace_invalid", func(e *Entry) { e.ID = "wrong.prefix.foo" }, "id_namespace_invalid"},
		{"type_missing", func(e *Entry) { e.Type = "" }, "type_missing"},
		{"type_invalid", func(e *Entry) { e.Type = "BOGUS" }, "type_invalid"},
		{"title_missing", func(e *Entry) { e.Title = "" }, "title_missing"},
		{"tags_missing", func(e *Entry) { e.Tags = nil }, "tags_missing"},
		{"tag_first_must_be_lifecycle", func(e *Entry) { e.Tags = []string{"not-a-stage"} }, "tag_first_must_be_lifecycle"},
		{"applies_when_cluster_phases_empty", func(e *Entry) { e.AppliesWhen.ClusterPhases = nil }, "applies_when_cluster_phases_empty"},
		{"applies_when_cluster_phase_invalid", func(e *Entry) { e.AppliesWhen.ClusterPhases = []string{"day-99"} }, "applies_when_cluster_phase_invalid"},
		{"services_healthy_not_in_present", func(e *Entry) {
			e.AppliesWhen.ServicesHealthy = []string{"unknown-service"}
		}, "services_healthy_not_in_present"},
		{"content_missing", func(e *Entry) { e.Content = "   \n  " }, "content_missing"},
		{"content_too_large", func(e *Entry) {
			e.Content = strings.Repeat("x", MaxEntryContentBytes+1)
		}, "content_too_large"},
		{"provenance_source_invalid", func(e *Entry) { e.Provenance.Source = "human" }, "provenance_source_invalid"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := validEntry()
			tc.mutate(&e)
			f := validFile(e)
			findings := Validate(f, validRefs())
			if !containsCode(findings, tc.wantCode) {
				t.Fatalf("expected code %q in findings, got: %v", tc.wantCode, findingCodes(findings))
			}
		})
	}
}

func TestValidate_ProvenanceSourceUnsetIsWarn(t *testing.T) {
	e := validEntry()
	e.Provenance.Source = ""
	f := validFile(e)
	findings := Validate(f, validRefs())
	// Must NOT count as an error — the build tool stamps it.
	if HasErrors(findings) {
		t.Fatalf("provenance.source unset should be a WARN only, got errors: %v", findingCodes(findings))
	}
	if !containsCode(findings, "provenance_source_unset") {
		t.Fatalf("expected provenance_source_unset warning, got: %v", findingCodes(findings))
	}
}

func TestValidate_LinkIntegrity(t *testing.T) {
	t.Run("invariant link to existing id passes", func(t *testing.T) {
		e := validEntry()
		e.Links.AwarenessInvariants = []string{"some.invariant"}
		findings := Validate(validFile(e), validRefs())
		if HasErrors(findings) {
			t.Fatalf("unexpected errors: %v", findingCodes(findings))
		}
	})
	t.Run("invariant link to missing id fails", func(t *testing.T) {
		e := validEntry()
		e.Links.AwarenessInvariants = []string{"nonexistent.invariant"}
		findings := Validate(validFile(e), validRefs())
		if !containsCode(findings, "link_invariant_not_found") {
			t.Fatalf("expected link_invariant_not_found, got: %v", findingCodes(findings))
		}
	})
	t.Run("failure_mode link to missing id fails", func(t *testing.T) {
		e := validEntry()
		e.Links.AwarenessFailureModes = []string{"nonexistent.failure"}
		findings := Validate(validFile(e), validRefs())
		if !containsCode(findings, "link_failure_mode_not_found") {
			t.Fatalf("expected link_failure_mode_not_found, got: %v", findingCodes(findings))
		}
	})
	t.Run("runbook link with prefix accepted", func(t *testing.T) {
		e := validEntry()
		e.Links.Runbooks = []string{"runbooks/some-runbook.yaml"}
		findings := Validate(validFile(e), validRefs())
		if HasErrors(findings) {
			t.Fatalf("unexpected errors: %v", findingCodes(findings))
		}
	})
	t.Run("runbook link without prefix accepted (auto-prefixed)", func(t *testing.T) {
		e := validEntry()
		e.Links.Runbooks = []string{"some-runbook.yaml"}
		findings := Validate(validFile(e), validRefs())
		if HasErrors(findings) {
			t.Fatalf("unexpected errors: %v", findingCodes(findings))
		}
	})
	t.Run("runbook link to missing path fails", func(t *testing.T) {
		e := validEntry()
		e.Links.Runbooks = []string{"runbooks/missing.yaml"}
		findings := Validate(validFile(e), validRefs())
		if !containsCode(findings, "link_runbook_not_found") {
			t.Fatalf("expected link_runbook_not_found, got: %v", findingCodes(findings))
		}
	})
}

func TestValidate_DuplicateIDsAcrossFiles(t *testing.T) {
	refs := validRefs()
	f1 := validFile(validEntry())
	f1.Path = "/file1.yaml"
	f2 := validFile(validEntry())
	f2.Path = "/file2.yaml"

	findings1 := Validate(f1, refs)
	if HasErrors(findings1) {
		t.Fatalf("file1 should pass: %v", findingCodes(findings1))
	}
	findings2 := Validate(f2, refs) // same id, second file
	if !containsCode(findings2, "id_duplicate") {
		t.Fatalf("expected id_duplicate in file2, got: %v", findingCodes(findings2))
	}
}

func TestHashEntry_DeterministicAcrossKeyOrder(t *testing.T) {
	a := validEntry()
	b := validEntry()
	// Set fields in a way that exercises map ordering — links + tags.
	a.Tags = []string{StageDay1, "alpha", "beta"}
	a.Links = Links{AwarenessInvariants: []string{"x", "y"}}
	b.Tags = []string{StageDay1, "alpha", "beta"}
	b.Links = Links{AwarenessInvariants: []string{"x", "y"}}

	hA, err := HashEntry(a)
	if err != nil {
		t.Fatal(err)
	}
	hB, err := HashEntry(b)
	if err != nil {
		t.Fatal(err)
	}
	if hA != hB {
		t.Fatalf("identical entries produced different hashes: %s vs %s", hA, hB)
	}
}

func TestHashEntry_IgnoresSeedVersionAndSeedSHA256(t *testing.T) {
	a := validEntry()
	b := validEntry()
	b.Provenance.SeedVersion = "1.0.0"
	b.Provenance.SeedSHA256 = "deadbeef"

	hA, err := HashEntry(a)
	if err != nil {
		t.Fatal(err)
	}
	hB, err := HashEntry(b)
	if err != nil {
		t.Fatal(err)
	}
	if hA != hB {
		t.Fatalf("seed_version/seed_sha256 must NOT affect hash: %s vs %s", hA, hB)
	}
}

func TestHashEntry_DifferentContentDifferentHash(t *testing.T) {
	a := validEntry()
	b := validEntry()
	b.Content = "different content"
	hA, err := HashEntry(a)
	if err != nil {
		t.Fatal(err)
	}
	hB, err := HashEntry(b)
	if err != nil {
		t.Fatal(err)
	}
	if hA == hB {
		t.Fatalf("different content must produce different hashes; got %s for both", hA)
	}
}

// ── helpers ─────────────────────────────────────────────────────────────────

func containsCode(fs []Finding, code string) bool {
	for _, f := range fs {
		if f.Code == code {
			return true
		}
	}
	return false
}
