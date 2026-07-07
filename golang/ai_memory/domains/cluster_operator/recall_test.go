package cluster_operator

import (
	"strings"
	"testing"
)

// TestRecallSeedEntries asserts the embedded operational-knowledge recall
// artifact is present, parses, and every entry is well-formed — so ai-memory's
// startup self-seed has real, valid content to load.
func TestRecallSeedEntries(t *testing.T) {
	entries, err := RecallSeedEntries()
	if err != nil {
		t.Fatalf("RecallSeedEntries: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no embedded recall entries — opscompile must regenerate generated/recall.generated.yaml")
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if e.ID == "" || e.Type == "" || e.Title == "" || e.SeedSHA256 == "" {
			t.Errorf("malformed recall entry: %+v", e)
		}
		if seen[e.ID] {
			t.Errorf("duplicate recall entry id: %s", e.ID)
		}
		seen[e.ID] = true
	}
	t.Logf("embedded recall entries: %d", len(entries))
}

// TestRecallSeedContentNormalized guards the seed-normalization half of the
// idempotency invariant (ai-memory decision 45f7fc31): the compiled recall
// content must use REAL newlines, never escaped-content drift. Duplicate seed
// rows in the live store originated from re-imports whose content flipped between
// real newlines and literal "\n" / "\\n"; each representation hashes differently,
// re-triggering an upsert. If the embedded artifact ever regressed to escaped
// content, that drift would return. The block-scalar corpus has zero backslash-n
// today, so any occurrence is a compile-step regression, not legitimate content.
func TestRecallSeedContentNormalized(t *testing.T) {
	entries, err := RecallSeedEntries()
	if err != nil {
		t.Fatalf("RecallSeedEntries: %v", err)
	}
	// The 2-char escape sequences that signal double-encoded content.
	badSeqs := []string{`\n`, `\t`, `\\n`, `\\t`}
	for _, e := range entries {
		for _, bad := range badSeqs {
			if strings.Contains(e.Content, bad) {
				t.Errorf("entry %s content contains escaped sequence %q — content must use real newlines (seed-normalization drift)", e.ID, bad)
			}
		}
	}
}
