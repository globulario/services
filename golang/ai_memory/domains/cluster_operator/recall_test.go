package cluster_operator

import "testing"

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
