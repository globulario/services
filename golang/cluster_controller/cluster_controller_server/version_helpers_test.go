package main

import "testing"

func TestLookupInstalledVersionFromMap_ExactMatch(t *testing.T) {
	m := map[string]string{
		"rbac":    "1.0.0",
		"gateway": "2.0.0",
	}
	if v := lookupInstalledVersionFromMap(m, "rbac"); v != "1.0.0" {
		t.Fatalf("expected 1.0.0, got %q", v)
	}
	if v := lookupInstalledVersionFromMap(m, "gateway"); v != "2.0.0" {
		t.Fatalf("expected 2.0.0, got %q", v)
	}
}

func TestLookupInstalledVersionFromMap_PublisherPrefix(t *testing.T) {
	m := map[string]string{
		"core@globular.io/rbac": "1.5.0",
	}
	if v := lookupInstalledVersionFromMap(m, "rbac"); v != "1.5.0" {
		t.Fatalf("expected 1.5.0, got %q", v)
	}
}

func TestLookupInstalledVersionFromMap_CanonicalizesQuery(t *testing.T) {
	m := map[string]string{
		"gateway": "3.0.0",
	}
	// "globular-gateway.service" should canonicalize to "gateway".
	if v := lookupInstalledVersionFromMap(m, "globular-gateway.service"); v != "3.0.0" {
		t.Fatalf("expected 3.0.0, got %q", v)
	}
}

func TestLookupInstalledVersionFromMap_Unknown(t *testing.T) {
	m := map[string]string{
		"rbac": "1.0.0",
	}
	if v := lookupInstalledVersionFromMap(m, "nonexistent"); v != "" {
		t.Fatalf("expected empty for unknown, got %q", v)
	}
}

func TestLookupInstalledVersionFromMap_NilMap(t *testing.T) {
	if v := lookupInstalledVersionFromMap(nil, "rbac"); v != "" {
		t.Fatalf("expected empty for nil map, got %q", v)
	}
}

func TestLookupInstalledVersionFromMap_EmptyMap(t *testing.T) {
	if v := lookupInstalledVersionFromMap(map[string]string{}, "rbac"); v != "" {
		t.Fatalf("expected empty for empty map, got %q", v)
	}
}

func TestLookupInstalledVersionFromMap_TrimsWhitespace(t *testing.T) {
	m := map[string]string{
		"rbac": "  1.2.3  ",
	}
	if v := lookupInstalledVersionFromMap(m, "rbac"); v != "1.2.3" {
		t.Fatalf("expected trimmed 1.2.3, got %q", v)
	}
}
