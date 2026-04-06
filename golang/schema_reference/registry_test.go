package schema_reference

import (
	"testing"
)

// testRegistry returns a Registry backed by a small hand-built result.
// Avoids the embedded JSON so these tests don't drift when annotations
// change in production code.
func testRegistry(t *testing.T) *Registry {
	t.Helper()
	data := []byte(`{
		"source": "schema-extractor",
		"generated_at_unix": 1700000000,
		"entries": [
			{"key_pattern":"/globular/a","writer":"w-a","type_name":"Alpha","description":"first"},
			{"key_pattern":"/globular/b","writer":"w-b","type_name":"Beta","description":"second with alpha mention"},
			{"key_pattern":"/globular/c","writer":"w-c","type_name":"Gamma","description":"third"}
		]
	}`)
	reg, err := NewRegistryFromJSON(data)
	if err != nil {
		t.Fatalf("NewRegistryFromJSON: %v", err)
	}
	return reg
}

func TestRegistryLookupByKey(t *testing.T) {
	reg := testRegistry(t)
	if e := reg.LookupByKey("/globular/b"); e == nil || e.TypeName != "Beta" {
		t.Errorf("LookupByKey(/globular/b) = %v", e)
	}
	if e := reg.LookupByKey("/nope"); e != nil {
		t.Errorf("LookupByKey(/nope) should be nil, got %v", e)
	}
}

func TestRegistryLookupByType(t *testing.T) {
	reg := testRegistry(t)
	// Case-insensitive.
	if e := reg.LookupByType("BETA"); e == nil || e.KeyPattern != "/globular/b" {
		t.Errorf("LookupByType(BETA) = %v", e)
	}
	if e := reg.LookupByType("beta"); e == nil {
		t.Errorf("LookupByType(beta) should hit")
	}
	if e := reg.LookupByType("zeta"); e != nil {
		t.Errorf("LookupByType(zeta) should be nil")
	}
}

func TestRegistrySearch(t *testing.T) {
	reg := testRegistry(t)
	// "alpha" matches the Alpha type and the Beta description.
	hits := reg.Search("alpha")
	if len(hits) != 2 {
		t.Fatalf("Search(alpha) got %d, want 2", len(hits))
	}
	// Empty query returns nothing — scoped query discipline (clause 5).
	if hits := reg.Search("   "); hits != nil {
		t.Errorf("Search(whitespace) should return nil")
	}
	// Non-matching query returns empty slice.
	if hits := reg.Search("nonsense"); len(hits) != 0 {
		t.Errorf("Search(nonsense) got %d", len(hits))
	}
}

func TestRegistryFreshnessFields(t *testing.T) {
	// Every response (entries/source/generated_at) must survive a
	// round trip through the registry. These are the Clause 4 fields
	// callers stamp onto their responses.
	reg := testRegistry(t)
	if reg.Source() != "schema-extractor" {
		t.Errorf("Source() = %q", reg.Source())
	}
	if reg.GeneratedAtUnix() != 1700000000 {
		t.Errorf("GeneratedAtUnix() = %d", reg.GeneratedAtUnix())
	}
	if len(reg.Entries()) != 3 {
		t.Errorf("Entries() len = %d", len(reg.Entries()))
	}
}

// TestEmbeddedRegistryIsValid is a smoke test: the embedded schema.json
// must always parse cleanly. If someone hand-edits it and breaks JSON,
// this fails fast — long before a downstream caller hits the panic.
func TestEmbeddedRegistryIsValid(t *testing.T) {
	reg := DefaultRegistry()
	// Force load via any method.
	_ = reg.Entries()
	if reg.Source() != "schema-extractor" {
		t.Errorf("embedded source = %q, want schema-extractor", reg.Source())
	}
	// The embedded index should have at least a few entries once the
	// extractor has been run in this repo.
	if len(reg.Entries()) == 0 {
		t.Log("warning: embedded registry is empty — run `go run ./tools/schema-extractor` to populate")
	}
}
