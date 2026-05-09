package main

import (
	"strings"
	"testing"
)

// TestValidateCatalog_LiveCatalogPasses guards the production catalog
// against the bijection invariant: every profile in ProfileCapabilities
// has at least one component, and every component-referenced profile is
// defined. If this test fails, the live catalog is internally inconsistent
// and a Day-1 join would silently install an under-specified set.
func TestValidateCatalog_LiveCatalogPasses(t *testing.T) {
	if err := ValidateCatalog(); err != nil {
		t.Fatalf("live catalog failed bijection invariant: %v", err)
	}
}

// TestValidateCatalog_EmptyProfileRejected — a profile defined in
// ProfileCapabilities but with zero components claiming it must be
// rejected at startup. The whole point of this invariant is "a profile
// with no services is not a profile."
func TestValidateCatalog_EmptyProfileRejected(t *testing.T) {
	origCatalog := catalog
	origProfileCaps := ProfileCapabilities
	defer func() {
		catalog = origCatalog
		ProfileCapabilities = origProfileCaps
	}()

	// Inject a profile that no component claims.
	ProfileCapabilities = map[string][]Capability{
		"core":         {CapConfigStore},
		"orphan-prof":  {CapConfigStore}, // defined but no component lists it
	}
	catalog = []*Component{
		{Name: "etcd", Unit: "globular-etcd.service", Kind: KindInfrastructure, Profiles: []string{"core"}},
	}

	err := ValidateCatalog()
	if err == nil {
		t.Fatal("expected ValidateCatalog to reject empty profile, got nil")
	}
	if !strings.Contains(err.Error(), "orphan-prof") {
		t.Fatalf("error message must name the offending profile, got: %v", err)
	}
	if !strings.Contains(err.Error(), "no components") {
		t.Fatalf("error message must explain the reason, got: %v", err)
	}
}

// TestValidateCatalog_UndefinedProfileRejected — a component referencing
// a profile that isn't in ProfileCapabilities must be rejected. Catches
// typos and stale references after a profile is renamed/removed.
func TestValidateCatalog_UndefinedProfileRejected(t *testing.T) {
	origCatalog := catalog
	origProfileCaps := ProfileCapabilities
	defer func() {
		catalog = origCatalog
		ProfileCapabilities = origProfileCaps
	}()

	ProfileCapabilities = map[string][]Capability{
		"core": {CapConfigStore},
	}
	catalog = []*Component{
		{Name: "etcd", Unit: "globular-etcd.service", Kind: KindInfrastructure, Profiles: []string{"core", "ghost-profile"}},
	}

	err := ValidateCatalog()
	if err == nil {
		t.Fatal("expected ValidateCatalog to reject undefined profile reference, got nil")
	}
	if !strings.Contains(err.Error(), "ghost-profile") {
		t.Fatalf("error message must name the offending profile, got: %v", err)
	}
	if !strings.Contains(err.Error(), "etcd") {
		t.Fatalf("error message must name the offending component, got: %v", err)
	}
}

// TestResolveNodeIntent_EmptyProfilesIsError — every node must have a
// profile. Silently coercing empty to ["core"] (the prior behavior)
// hid the "node joined without a profile" bug from operators and
// shipped an under-specified install set. Now it must error.
func TestResolveNodeIntent_EmptyProfilesIsError(t *testing.T) {
	cases := [][]string{
		nil,
		{},
		{"", " ", "\t"}, // whitespace-only — normalizeProfiles drops these
	}
	for i, profiles := range cases {
		intent, err := ResolveNodeIntent("test-node", profiles, nil, nil)
		if err == nil {
			t.Fatalf("case %d: expected error for empty profiles, got intent=%+v", i, intent)
		}
		if intent != nil {
			t.Fatalf("case %d: expected nil intent on error, got %+v", i, intent)
		}
		if !strings.Contains(err.Error(), "no profiles assigned") {
			t.Fatalf("case %d: error must explain the cause, got: %v", i, err)
		}
	}
}

// TestResolveNodeIntent_ValidProfilesStillWork — a sanity check that the
// new error path doesn't break the happy path.
func TestResolveNodeIntent_ValidProfilesStillWork(t *testing.T) {
	intent, err := ResolveNodeIntent("test-node", []string{"core"}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error for valid profile: %v", err)
	}
	if intent == nil {
		t.Fatal("intent must not be nil on success")
	}
	if len(intent.Profiles) == 0 {
		t.Fatal("intent.Profiles must not be empty after successful resolve")
	}
}
